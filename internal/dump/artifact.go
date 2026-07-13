package dump

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/dvmrry/zscalerctl/internal/redact"
)

const (
	RedactionReportSchemaID = "zscalerctl.redaction_report.v1"
	ResourceErrorSchemaID   = "zscalerctl.dump.error.v1"

	maxArtifactManifestBytes int64 = 1 << 20
	maxArtifactReportBytes   int64 = 64 << 20
	maxArtifactErrorsBytes   int64 = 64 << 20
	maxArtifactResourceBytes int64 = 512 << 20
)

var ErrInvalidArtifact = errors.New("invalid dump artifact")

// ValidatedArtifact is the reconciled metadata from one structurally valid dump.
type ValidatedArtifact struct {
	Manifest        Manifest
	RedactionReport RedactionReport
	Errors          []ResourceError
	cleanup         artifactCleanupPlan
}

type artifactCleanupPlan struct {
	files      []string
	dirs       []string
	identities map[string]os.FileInfo
}

// ValidateArtifactContext validates the complete dump artifact rooted at dir.
func ValidateArtifactContext(ctx context.Context, dir string) (ValidatedArtifact, error) {
	ctx = contextOrBackground(ctx)
	if err := checkContext(ctx); err != nil {
		return ValidatedArtifact{}, err
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return ValidatedArtifact{}, fmt.Errorf("%w: open dump root: %v", ErrInvalidArtifact, err)
	}
	defer root.Close()
	return ValidateArtifactRootContext(ctx, root)
}

// ValidateArtifactRootContext validates a dump through an already-open root.
func ValidateArtifactRootContext(ctx context.Context, root *os.Root) (ValidatedArtifact, error) {
	return validateArtifactRootContext(ctx, root, true)
}

// ValidateArtifactMetadataRootContext reconciles artifact metadata and file
// inventory through an already-open root. Callers must separately validate the
// contents and record counts of every successful resource file.
func ValidateArtifactMetadataRootContext(ctx context.Context, root *os.Root) (ValidatedArtifact, error) {
	return validateArtifactRootContext(ctx, root, false)
}

func validateArtifactRootContext(
	ctx context.Context,
	root *os.Root,
	validateResources bool,
) (ValidatedArtifact, error) {
	ctx = contextOrBackground(ctx)
	if err := checkContext(ctx); err != nil {
		return ValidatedArtifact{}, err
	}
	if root == nil {
		return ValidatedArtifact{}, fmt.Errorf("%w: nil dump root", ErrInvalidArtifact)
	}
	info, err := root.Stat(".")
	if err != nil {
		return ValidatedArtifact{}, fmt.Errorf("%w: inspect dump root: %v", ErrInvalidArtifact, err)
	}
	if !info.IsDir() {
		return ValidatedArtifact{}, fmt.Errorf("%w: dump root is not a directory", ErrInvalidArtifact)
	}

	manifestBody, err := readArtifactFile(ctx, root, "manifest.json", maxArtifactManifestBytes)
	if err != nil {
		return ValidatedArtifact{}, err
	}
	var manifest Manifest
	if err := decodeStrictJSON(manifestBody, &manifest); err != nil {
		return ValidatedArtifact{}, fmt.Errorf("%w: parse manifest: %v", ErrInvalidArtifact, err)
	}
	mode, okResources, failedResources, expectedFiles, expectedDirs, err := validateManifest(manifest)
	if err != nil {
		return ValidatedArtifact{}, err
	}

	reportBody, err := readArtifactFile(ctx, root, "redaction_report.json", maxArtifactReportBytes)
	if err != nil {
		return ValidatedArtifact{}, err
	}
	var report RedactionReport
	if err := decodeStrictJSON(reportBody, &report); err != nil {
		return ValidatedArtifact{}, fmt.Errorf("%w: parse redaction report: %v", ErrInvalidArtifact, err)
	}
	if err := validateRedactionReport(report, mode, okResources); err != nil {
		return ValidatedArtifact{}, err
	}
	if validateResources {
		if err := validateArtifactResources(ctx, root, manifest.Resources); err != nil {
			return ValidatedArtifact{}, err
		}
	}

	resourceErrors, err := validateArtifactErrors(ctx, root, manifest, failedResources)
	if err != nil {
		return ValidatedArtifact{}, err
	}
	if manifest.ErrorsPath != "" {
		expectedFiles[manifest.ErrorsPath] = struct{}{}
		addExpectedDirs(expectedDirs, manifest.ErrorsPath)
	}
	inventory, err := validateArtifactInventory(ctx, root, expectedFiles, expectedDirs)
	if err != nil {
		return ValidatedArtifact{}, err
	}
	return ValidatedArtifact{
		Manifest:        manifest,
		RedactionReport: report,
		Errors:          resourceErrors,
		cleanup:         newArtifactCleanupPlan(expectedFiles, expectedDirs, inventory),
	}, nil
}

func validateArtifactResources(
	ctx context.Context,
	root *os.Root,
	resources []ManifestResource,
) error {
	for _, resource := range resources {
		if resource.Status != "ok" {
			continue
		}
		if err := checkContext(ctx); err != nil {
			return err
		}
		key := artifactResourceKey{product: resource.Product, name: resource.Name}
		records, err := scanArtifactResource(ctx, root, resource.Path)
		if err != nil {
			return fmt.Errorf(
				"%w: validate resource %s/%s: %v",
				ErrInvalidArtifact,
				key.product,
				key.name,
				err,
			)
		}
		if records != resource.Records {
			return fmt.Errorf(
				"%w: resource %s/%s record count does not match resource file",
				ErrInvalidArtifact,
				key.product,
				key.name,
			)
		}
	}
	return nil
}

func newArtifactCleanupPlan(
	expectedFiles map[string]struct{},
	expectedDirs map[string]struct{},
	identities map[string]os.FileInfo,
) artifactCleanupPlan {
	plan := artifactCleanupPlan{
		files:      make([]string, 0, len(expectedFiles)),
		dirs:       make([]string, 0, len(expectedDirs)),
		identities: make(map[string]os.FileInfo, len(identities)),
	}
	for path := range expectedFiles {
		plan.files = append(plan.files, path)
	}
	for path := range expectedDirs {
		if path != "." {
			plan.dirs = append(plan.dirs, path)
		}
	}
	for path, info := range identities {
		plan.identities[path] = info
	}
	sort.Strings(plan.files)
	sort.Slice(plan.dirs, func(i, j int) bool {
		leftDepth := strings.Count(plan.dirs[i], "/")
		rightDepth := strings.Count(plan.dirs[j], "/")
		if leftDepth != rightDepth {
			return leftDepth > rightDepth
		}
		return plan.dirs[i] > plan.dirs[j]
	})
	return plan
}

func scanArtifactResource(ctx context.Context, root *os.Root, name string) (int, error) {
	info, err := root.Lstat(name)
	if err != nil {
		return 0, fmt.Errorf("inspect %s: %v", name, err)
	}
	if !info.Mode().IsRegular() {
		return 0, fmt.Errorf("%s is not a regular file", name)
	}
	if info.Size() > maxArtifactResourceBytes {
		return 0, fmt.Errorf("%s is too large", name)
	}
	file, err := root.Open(name)
	if err != nil {
		return 0, fmt.Errorf("open %s: %v", name, err)
	}
	defer file.Close()
	openedInfo, err := file.Stat()
	if err != nil {
		return 0, fmt.Errorf("inspect opened %s: %v", name, err)
	}
	if !openedInfo.Mode().IsRegular() || !os.SameFile(info, openedInfo) {
		return 0, fmt.Errorf("%s changed during validation", name)
	}

	limited := &io.LimitedReader{
		R: &artifactContextReader{ctx: ctx, reader: file},
		N: maxArtifactResourceBytes + 1,
	}
	decoder := json.NewDecoder(limited)
	decoder.UseNumber()
	first, err := decoder.Token()
	if err != nil {
		return 0, err
	}
	delim, ok := first.(json.Delim)
	if !ok || (delim != '[' && delim != '{') {
		return 0, errors.New("payload is not an object or array")
	}
	records := 1
	if delim == '[' {
		records = 0
		for decoder.More() {
			if err := checkContext(ctx); err != nil {
				return 0, err
			}
			item, err := decoder.Token()
			if err != nil {
				return 0, err
			}
			opening, ok := item.(json.Delim)
			if !ok || opening != '{' {
				return 0, errors.New("array contains a non-object record")
			}
			if err := consumeOpenedJSONContainer(ctx, decoder, opening); err != nil {
				return 0, err
			}
			records++
		}
		closing, err := decoder.Token()
		if err != nil {
			return 0, err
		}
		if closing != json.Delim(']') {
			return 0, errors.New("array has an invalid closing token")
		}
	} else if err := consumeOpenedJSONContainer(ctx, decoder, delim); err != nil {
		return 0, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err == nil {
		return 0, errors.New("unexpected trailing JSON value")
	} else if !errors.Is(err, io.EOF) {
		return 0, err
	}
	if limited.N == 0 {
		return 0, fmt.Errorf("%s is too large", name)
	}
	return records, checkContext(ctx)
}

func consumeOpenedJSONContainer(ctx context.Context, decoder *json.Decoder, opening json.Delim) error {
	closing := json.Delim(']')
	object := opening == '{'
	if object {
		closing = '}'
	}
	for decoder.More() {
		if err := checkContext(ctx); err != nil {
			return err
		}
		if object {
			key, err := decoder.Token()
			if err != nil {
				return err
			}
			if _, ok := key.(string); !ok {
				return errors.New("object contains a non-string key")
			}
		}
		if err := consumeJSONValue(ctx, decoder); err != nil {
			return err
		}
	}
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	if token != closing {
		return errors.New("container has an invalid closing token")
	}
	return nil
}

func consumeJSONValue(ctx context.Context, decoder *json.Decoder) error {
	if err := checkContext(ctx); err != nil {
		return err
	}
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	if opening, ok := token.(json.Delim); ok {
		if opening != '[' && opening != '{' {
			return errors.New("unexpected JSON closing token")
		}
		return consumeOpenedJSONContainer(ctx, decoder, opening)
	}
	return nil
}

type artifactContextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r *artifactContextReader) Read(buffer []byte) (int, error) {
	if err := checkContext(r.ctx); err != nil {
		return 0, err
	}
	n, err := r.reader.Read(buffer)
	if contextErr := checkContext(r.ctx); contextErr != nil {
		return n, contextErr
	}
	return n, err
}

type artifactResourceKey struct {
	product string
	name    string
}

func validateManifest(
	manifest Manifest,
) (
	redact.Mode,
	map[artifactResourceKey]ManifestResource,
	map[artifactResourceKey]ManifestResource,
	map[string]struct{},
	map[string]struct{},
	error,
) {
	if manifest.Schema != ManifestSchemaID {
		return "", nil, nil, nil, nil, fmt.Errorf(
			"%w: unsupported manifest schema (want %s; see docs/schema/manifest.schema.json)",
			ErrInvalidArtifact,
			ManifestSchemaID,
		)
	}
	if _, err := time.Parse(time.RFC3339, manifest.CollectedAt); err != nil {
		return "", nil, nil, nil, nil, fmt.Errorf("%w: invalid collected_at", ErrInvalidArtifact)
	}
	if strings.TrimSpace(manifest.ToolVersion) == "" {
		return "", nil, nil, nil, nil, fmt.Errorf("%w: missing tool_version", ErrInvalidArtifact)
	}
	mode, err := redact.ParseMode(manifest.Redaction)
	if err != nil {
		return "", nil, nil, nil, nil, fmt.Errorf("%w: invalid redaction mode", ErrInvalidArtifact)
	}
	if strings.TrimSpace(manifest.Warning) == "" {
		return "", nil, nil, nil, nil, fmt.Errorf("%w: missing warning", ErrInvalidArtifact)
	}
	if manifest.Resources == nil {
		return "", nil, nil, nil, nil, fmt.Errorf("%w: missing resources", ErrInvalidArtifact)
	}
	switch manifest.Status {
	case "complete", "partial":
	default:
		return "", nil, nil, nil, nil, fmt.Errorf(
			"%w: invalid manifest status (want complete or partial)",
			ErrInvalidArtifact,
		)
	}

	okResources := make(map[artifactResourceKey]ManifestResource)
	failedResources := make(map[artifactResourceKey]ManifestResource)
	expectedFiles := map[string]struct{}{
		"manifest.json":         {},
		"redaction_report.json": {},
	}
	expectedDirs := map[string]struct{}{".": {}}
	seen := make(map[artifactResourceKey]struct{}, len(manifest.Resources))
	for _, resource := range manifest.Resources {
		product, err := safeSegment(resource.Product)
		if err != nil || product != resource.Product {
			return "", nil, nil, nil, nil, fmt.Errorf("%w: resource has unsafe product", ErrInvalidArtifact)
		}
		name, err := safeSegment(resource.Name)
		if err != nil || name != resource.Name {
			return "", nil, nil, nil, nil, fmt.Errorf("%w: resource has unsafe name", ErrInvalidArtifact)
		}
		key := artifactResourceKey{product: product, name: name}
		if _, duplicate := seen[key]; duplicate {
			return "", nil, nil, nil, nil, fmt.Errorf(
				"%w: resource %s/%s appears more than once in manifest",
				ErrInvalidArtifact,
				product,
				name,
			)
		}
		seen[key] = struct{}{}
		if resource.Records < 0 {
			return "", nil, nil, nil, nil, fmt.Errorf(
				"%w: resource %s/%s has negative record count",
				ErrInvalidArtifact,
				product,
				name,
			)
		}
		switch resource.Status {
		case "ok":
			if resource.Shape != "" && resource.Shape != "singleton" {
				return "", nil, nil, nil, nil, fmt.Errorf(
					"%w: resource %s/%s has unsupported shape",
					ErrInvalidArtifact,
					product,
					name,
				)
			}
			expectedPath := filepath.ToSlash(filepath.Join("resources", product, name+".json"))
			if resource.Path != expectedPath {
				return "", nil, nil, nil, nil, fmt.Errorf(
					"%w: resource %s/%s has unsafe path",
					ErrInvalidArtifact,
					product,
					name,
				)
			}
			if resource.Operation != "" || resource.ErrorKind != "" {
				return "", nil, nil, nil, nil, fmt.Errorf(
					"%w: successful resource %s/%s has error metadata",
					ErrInvalidArtifact,
					product,
					name,
				)
			}
			okResources[key] = resource
			expectedFiles[resource.Path] = struct{}{}
			addExpectedDirs(expectedDirs, resource.Path)
		case "error":
			if resource.Path != "" || resource.Records != 0 || resource.Shape != "" {
				return "", nil, nil, nil, nil, fmt.Errorf(
					"%w: failed resource %s/%s has success metadata",
					ErrInvalidArtifact,
					product,
					name,
				)
			}
			if resource.Operation != "list" && resource.Operation != "show" {
				return "", nil, nil, nil, nil, fmt.Errorf(
					"%w: failed resource %s/%s has invalid operation",
					ErrInvalidArtifact,
					product,
					name,
				)
			}
			if strings.TrimSpace(resource.ErrorKind) == "" {
				return "", nil, nil, nil, nil, fmt.Errorf(
					"%w: failed resource %s/%s has no error kind",
					ErrInvalidArtifact,
					product,
					name,
				)
			}
			failedResources[key] = resource
		default:
			return "", nil, nil, nil, nil, fmt.Errorf(
				"%w: resource %s/%s has invalid status (want ok or error)",
				ErrInvalidArtifact,
				product,
				name,
			)
		}
	}

	switch manifest.Status {
	case "complete":
		if manifest.Errors != 0 || manifest.ErrorsPath != "" || len(failedResources) != 0 {
			return "", nil, nil, nil, nil, fmt.Errorf(
				"%w: complete manifest has error metadata",
				ErrInvalidArtifact,
			)
		}
	case "partial":
		if manifest.Errors <= 0 || manifest.ErrorsPath != "errors.ndjson" {
			return "", nil, nil, nil, nil, fmt.Errorf(
				"%w: partial manifest has inconsistent error metadata",
				ErrInvalidArtifact,
			)
		}
		if manifest.Errors != len(failedResources) {
			return "", nil, nil, nil, nil, fmt.Errorf(
				"%w: manifest error count does not match failed resources",
				ErrInvalidArtifact,
			)
		}
	}
	return mode, okResources, failedResources, expectedFiles, expectedDirs, nil
}

func validateRedactionReport(
	report RedactionReport,
	mode redact.Mode,
	okResources map[artifactResourceKey]ManifestResource,
) error {
	if report.Schema != RedactionReportSchemaID {
		return fmt.Errorf(
			"%w: unsupported redaction report schema (want %s)",
			ErrInvalidArtifact,
			RedactionReportSchemaID,
		)
	}
	if report.Redaction != string(mode) {
		return fmt.Errorf("%w: redaction report mode does not match manifest", ErrInvalidArtifact)
	}
	if report.Resources == nil {
		return fmt.Errorf("%w: missing redaction report resources", ErrInvalidArtifact)
	}
	seen := make(map[artifactResourceKey]struct{}, len(report.Resources))
	for _, resource := range report.Resources {
		key := artifactResourceKey{product: resource.Product, name: resource.Name}
		manifestResource, ok := okResources[key]
		if !ok {
			return fmt.Errorf(
				"%w: redaction report references non-successful resource %s/%s",
				ErrInvalidArtifact,
				resource.Product,
				resource.Name,
			)
		}
		if _, duplicate := seen[key]; duplicate {
			return fmt.Errorf(
				"%w: resource %s/%s appears more than once in redaction report",
				ErrInvalidArtifact,
				resource.Product,
				resource.Name,
			)
		}
		seen[key] = struct{}{}
		if resource.Path != manifestResource.Path || resource.Records != manifestResource.Records {
			return fmt.Errorf(
				"%w: redaction report path or record count does not match resource %s/%s",
				ErrInvalidArtifact,
				resource.Product,
				resource.Name,
			)
		}
	}
	if len(seen) != len(okResources) {
		return fmt.Errorf("%w: redaction report resource coverage does not match manifest", ErrInvalidArtifact)
	}
	return nil
}

func validateArtifactErrors(
	ctx context.Context,
	root *os.Root,
	manifest Manifest,
	failedResources map[artifactResourceKey]ManifestResource,
) ([]ResourceError, error) {
	if manifest.Status == "complete" {
		if _, err := root.Lstat("errors.ndjson"); err == nil {
			return nil, fmt.Errorf("%w: complete artifact contains errors.ndjson", ErrInvalidArtifact)
		} else if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("%w: inspect errors.ndjson: %v", ErrInvalidArtifact, err)
		}
		return nil, nil
	}
	body, err := readArtifactFile(ctx, root, manifest.ErrorsPath, maxArtifactErrorsBytes)
	if err != nil {
		return nil, err
	}
	scanner := bufio.NewScanner(bytes.NewReader(body))
	scanner.Buffer(make([]byte, 64*1024), int(maxArtifactErrorsBytes))
	resourceErrors := make([]ResourceError, 0, manifest.Errors)
	seen := make(map[artifactResourceKey]struct{}, manifest.Errors)
	for scanner.Scan() {
		if err := checkContext(ctx); err != nil {
			return nil, err
		}
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			return nil, fmt.Errorf("%w: errors.ndjson contains an empty row", ErrInvalidArtifact)
		}
		var resourceError ResourceError
		if err := decodeStrictJSON(line, &resourceError); err != nil {
			return nil, fmt.Errorf("%w: parse errors.ndjson: %v", ErrInvalidArtifact, err)
		}
		if resourceError.Schema != ResourceErrorSchemaID {
			return nil, fmt.Errorf(
				"%w: unsupported dump error schema (want %s)",
				ErrInvalidArtifact,
				ResourceErrorSchemaID,
			)
		}
		key := artifactResourceKey{product: resourceError.Product, name: resourceError.Name}
		failed, ok := failedResources[key]
		if !ok {
			return nil, fmt.Errorf(
				"%w: errors.ndjson references non-failed resource %s/%s",
				ErrInvalidArtifact,
				resourceError.Product,
				resourceError.Name,
			)
		}
		if _, duplicate := seen[key]; duplicate {
			return nil, fmt.Errorf(
				"%w: resource %s/%s appears more than once in errors.ndjson",
				ErrInvalidArtifact,
				resourceError.Product,
				resourceError.Name,
			)
		}
		seen[key] = struct{}{}
		if resourceError.Operation != failed.Operation || resourceError.Kind != failed.ErrorKind {
			return nil, fmt.Errorf(
				"%w: errors.ndjson metadata does not match resource %s/%s",
				ErrInvalidArtifact,
				resourceError.Product,
				resourceError.Name,
			)
		}
		resourceErrors = append(resourceErrors, resourceError)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("%w: read errors.ndjson: %v", ErrInvalidArtifact, err)
	}
	if len(resourceErrors) != manifest.Errors || len(seen) != len(failedResources) {
		return nil, fmt.Errorf("%w: errors.ndjson coverage does not match manifest", ErrInvalidArtifact)
	}
	return resourceErrors, nil
}

func validateArtifactInventory(
	ctx context.Context,
	root *os.Root,
	expectedFiles map[string]struct{},
	expectedDirs map[string]struct{},
) (map[string]os.FileInfo, error) {
	inventory := make(map[string]os.FileInfo, len(expectedFiles)+len(expectedDirs))
	err := fs.WalkDir(root.FS(), ".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("%w: inspect artifact path: %v", ErrInvalidArtifact, walkErr)
		}
		if err := checkContext(ctx); err != nil {
			return err
		}
		path = filepath.ToSlash(path)
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("%w: artifact path %s is a symlink", ErrInvalidArtifact, path)
		}
		if entry.IsDir() {
			if _, expectedFile := expectedFiles[path]; expectedFile {
				return fmt.Errorf("%w: artifact path %s is not a regular file", ErrInvalidArtifact, path)
			}
			if _, ok := expectedDirs[path]; !ok {
				return fmt.Errorf("%w: unexpected artifact directory %s", ErrInvalidArtifact, path)
			}
			info, err := entry.Info()
			if err != nil {
				return fmt.Errorf("%w: inspect artifact path %s: %v", ErrInvalidArtifact, path, err)
			}
			inventory[path] = info
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("%w: inspect artifact path %s: %v", ErrInvalidArtifact, path, err)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("%w: artifact path %s is not a regular file", ErrInvalidArtifact, path)
		}
		if _, ok := expectedFiles[path]; !ok {
			return fmt.Errorf("%w: unexpected artifact file %s", ErrInvalidArtifact, path)
		}
		inventory[path] = info
		return nil
	})
	if err != nil {
		return nil, err
	}
	for path := range expectedFiles {
		if _, ok := inventory[path]; !ok {
			return nil, fmt.Errorf("%w: missing artifact file %s", ErrInvalidArtifact, path)
		}
	}
	for path := range expectedDirs {
		if _, ok := inventory[path]; !ok {
			return nil, fmt.Errorf("%w: missing artifact directory %s", ErrInvalidArtifact, path)
		}
	}
	return inventory, nil
}

func addExpectedDirs(expected map[string]struct{}, filePath string) {
	for dir := filepath.ToSlash(filepath.Dir(filepath.FromSlash(filePath))); dir != "."; dir = filepath.ToSlash(filepath.Dir(dir)) {
		expected[dir] = struct{}{}
	}
}

func readArtifactFile(ctx context.Context, root *os.Root, name string, maxBytes int64) ([]byte, error) {
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	info, err := root.Lstat(name)
	if err != nil {
		return nil, fmt.Errorf("%w: inspect %s: %v", ErrInvalidArtifact, name, err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%w: %s is not a regular file", ErrInvalidArtifact, name)
	}
	if info.Size() > maxBytes {
		return nil, fmt.Errorf("%w: %s is too large", ErrInvalidArtifact, name)
	}
	body, err := root.ReadFile(name)
	if err != nil {
		return nil, fmt.Errorf("%w: read %s: %v", ErrInvalidArtifact, name, err)
	}
	if int64(len(body)) > maxBytes {
		return nil, fmt.Errorf("%w: %s is too large", ErrInvalidArtifact, name)
	}
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	return body, nil
}

func decodeStrictJSON(body []byte, destination any) error {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err == nil {
		return errors.New("unexpected trailing JSON value")
	} else if !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}
