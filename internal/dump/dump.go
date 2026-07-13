package dump

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
	"github.com/dvmrry/zscalerctl/internal/version"
)

const (
	dirPerm     os.FileMode = 0o700
	filePerm    os.FileMode = 0o600
	dumpWarning             = "sanitized dumps remain confidential operational data" // #nosec G101 -- static warning string, not a credential
)

var (
	ErrUnsafeOverwrite          = errors.New("refusing to overwrite existing dump file")
	ErrUnsafePath               = errors.New("unsafe dump path")
	ErrAtomicReplaceUnsupported = errors.New("atomic dump directory replacement is unsupported")
)

type safeJSON interface {
	OutputSafe()
}

// Result is the safe, projected data set written by a dump.
type Result struct {
	Entries []ResourceDump
	Errors  []ResourceError
}

type ResourceDump struct {
	Spec    resources.ResourceSpec
	Records resources.ProjectedRecords
	Record  *resources.ProjectedRecord
	Reports []resources.ProjectionReport
}

// ResourceError records a value-free per-resource dump failure.
type ResourceError struct {
	Schema    string `json:"schema"`
	Product   string `json:"product"`
	Name      string `json:"name"`
	Operation string `json:"operation"`
	Kind      string `json:"kind"`
}

// NewResourceError builds a value-free dump failure record.
func NewResourceError(product resources.Product, name string, operation string, kind string) ResourceError {
	return ResourceError{
		Schema:    ResourceErrorSchemaID,
		Product:   string(product),
		Name:      name,
		Operation: operation,
		Kind:      kind,
	}
}

// ManifestSchemaID is the versioned schema identifier written to manifest.json.
// It must match the `schema` const published in docs/schema/manifest.schema.json.
const ManifestSchemaID = "zscalerctl.dump.manifest.v2"

type Manifest struct {
	Schema      string             `json:"schema"`
	CollectedAt string             `json:"collected_at"`
	ToolVersion string             `json:"tool_version"`
	Redaction   string             `json:"redaction"`
	Warning     string             `json:"warning"`
	Status      string             `json:"status"`
	Errors      int                `json:"errors,omitempty"`
	ErrorsPath  string             `json:"errors_path,omitempty"`
	Resources   []ManifestResource `json:"resources"`
}

func (Manifest) OutputSafe() {}

type ManifestResource struct {
	Product   string `json:"product"`
	Name      string `json:"name"`
	Shape     string `json:"shape,omitempty"`
	Status    string `json:"status"`
	Path      string `json:"path,omitempty"`
	Records   int    `json:"records"`
	Operation string `json:"operation,omitempty"`
	ErrorKind string `json:"error_kind,omitempty"`
}

type RedactionReport struct {
	Schema    string           `json:"schema"`
	Redaction string           `json:"redaction"`
	Resources []ResourceReport `json:"resources"`
}

func (RedactionReport) OutputSafe() {}

type ResourceReport struct {
	Product        string   `json:"product"`
	Name           string   `json:"name"`
	Path           string   `json:"path"`
	Records        int      `json:"records"`
	IncludedFields []string `json:"included_fields,omitempty"`
	DroppedFields  []string `json:"dropped_fields,omitempty"`
	RedactedFields []string `json:"redacted_fields,omitempty"`
}

type resourceTarget struct {
	entry   ResourceDump
	product string
	name    string
	relPath string
	path    string
}

func contextOrBackground(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func checkContext(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return nil
}

func Write(dir string, mode redact.Mode, result Result) error {
	return WriteContext(context.Background(), dir, mode, result)
}

// WriteContext writes and exclusively publishes a sanitized dump directory.
func WriteContext(ctx context.Context, dir string, mode redact.Mode, result Result) error {
	return PublishContext(ctx, dir, mode, result, false)
}

// PublishContext writes a sanitized dump into a private sibling staging
// directory, validates the complete artifact, and publishes it at the directory
// boundary. When force is true, only an empty directory or a structurally valid
// zscalerctl dump may be replaced.
func PublishContext(ctx context.Context, dir string, mode redact.Mode, result Result, force bool) error {
	return publishContextWithHooks(ctx, dir, mode, result, force, publishContextHooks{})
}

type publishContextHooks struct {
	beforeStagingCleanupRelocate func(string)
	afterParentValidation        func()
}

func publishContextWithHooks(
	ctx context.Context,
	dir string,
	mode redact.Mode,
	result Result,
	force bool,
	hooks publishContextHooks,
) error {
	ctx = contextOrBackground(ctx)
	if err := checkContext(ctx); err != nil {
		return err
	}
	if strings.TrimSpace(dir) == "" {
		return fmt.Errorf("%w: missing dump directory", ErrUnsafePath)
	}
	dir = filepath.Clean(dir)
	if force {
		if err := rejectDangerousForceTargetContext(ctx, dir); err != nil {
			return err
		}
	}
	parent := filepath.Dir(dir)
	parent, err := ensureStagingParentContext(ctx, parent)
	if err != nil {
		return err
	}
	// All later pathname operations use the validated resolved parent. This
	// prevents a mutable symlink in the caller's lexical path from bypassing the
	// stable-parent namespace check after validation.
	dir = filepath.Join(parent, filepath.Base(dir))
	if hooks.afterParentValidation != nil {
		hooks.afterParentValidation()
	}
	stagingDir, err := os.MkdirTemp(parent, ".zscalerctl-staging-*")
	if err != nil {
		return fmt.Errorf("create dump staging directory: %w", err)
	}
	stagingInfo, err := os.Lstat(stagingDir)
	if err != nil {
		return fmt.Errorf("inspect dump staging directory: %w", err)
	}
	if err := os.Chmod(stagingDir, dirPerm); err != nil {
		cleanupStagingPath(stagingDir, stagingInfo, nil)
		return fmt.Errorf("chmod dump staging directory: %w", err)
	}
	stagingRoot, err := os.OpenRoot(stagingDir)
	if err != nil {
		cleanupStagingPath(stagingDir, stagingInfo, nil)
		return fmt.Errorf("open dump staging directory: %w", err)
	}
	published := false
	defer func() {
		if stagingRoot != nil {
			_ = stagingRoot.Close()
			stagingRoot = nil
		}
		if !published {
			cleanupStagingPath(stagingDir, stagingInfo, hooks.beforeStagingCleanupRelocate)
		}
	}()

	mode = redact.EffectiveMode(mode)
	if err := writeArtifactContext(ctx, stagingDir, mode, result); err != nil {
		return err
	}
	if _, err := ValidateArtifactRootContext(ctx, stagingRoot); err != nil {
		return fmt.Errorf("validate staged dump: %w", err)
	}
	currentStagingInfo, err := os.Lstat(stagingDir)
	if err != nil || !os.SameFile(stagingInfo, currentStagingInfo) {
		return fmt.Errorf("%w: dump staging directory changed before publication", ErrUnsafePath)
	}
	if closeRootBeforeRename() {
		if err := stagingRoot.Close(); err != nil {
			return fmt.Errorf("close dump staging directory before publication: %w", err)
		}
		stagingRoot = nil
	}
	if err := checkContext(ctx); err != nil {
		return err
	}
	if force {
		if err := publishReplacingDirectoryContext(ctx, stagingDir, dir, true); err != nil {
			return err
		}
	} else {
		err := publishDirectoryNoReplace(stagingDir, dir)
		if errors.Is(err, ErrUnsafeOverwrite) {
			err = publishReplacingDirectoryContext(ctx, stagingDir, dir, false)
		}
		if err != nil {
			return err
		}
	}
	published = true
	return nil
}

func writeArtifactContext(ctx context.Context, dir string, mode redact.Mode, result Result) error {
	ctx = contextOrBackground(ctx)
	if err := checkContext(ctx); err != nil {
		return err
	}
	if strings.TrimSpace(dir) == "" {
		return fmt.Errorf("%w: missing dump directory", ErrUnsafePath)
	}
	mode = redact.EffectiveMode(mode)

	targets, err := buildResourceTargetsContext(ctx, dir, result.Entries)
	if err != nil {
		return err
	}
	targetPaths := []string{filepath.Join(dir, "manifest.json"), filepath.Join(dir, "redaction_report.json")}
	if len(result.Errors) > 0 {
		targetPaths = append(targetPaths, filepath.Join(dir, "errors.ndjson"))
	}
	for _, target := range targets {
		targetPaths = append(targetPaths, target.path)
	}
	for _, path := range targetPaths {
		if err := checkContext(ctx); err != nil {
			return err
		}
		if err := rejectExistingContext(ctx, path); err != nil {
			return err
		}
	}
	if err := checkContext(ctx); err != nil {
		return err
	}
	if err := ensureDirContext(ctx, dir); err != nil {
		return err
	}

	manifest := Manifest{
		Schema:      ManifestSchemaID,
		CollectedAt: time.Now().UTC().Format(time.RFC3339),
		ToolVersion: version.Current().Version,
		Redaction:   string(mode),
		Warning:     dumpWarning,
		Status:      "complete",
		Resources:   []ManifestResource{},
	}
	if len(result.Errors) > 0 {
		manifest.Status = "partial"
		manifest.Errors = len(result.Errors)
		manifest.ErrorsPath = "errors.ndjson"
	}
	report := RedactionReport{
		Schema:    RedactionReportSchemaID,
		Redaction: string(mode),
		Resources: []ResourceReport{},
	}

	for _, target := range targets {
		if err := checkContext(ctx); err != nil {
			return err
		}
		if err := ensureDirChainContext(ctx, dir, filepath.Dir(target.relPath)); err != nil {
			return err
		}
		if err := checkContext(ctx); err != nil {
			return err
		}
		payload, recordCount := target.entry.payload()
		if err := checkContext(ctx); err != nil {
			return err
		}
		if err := writeJSONFileContext(ctx, target.path, mode, payload); err != nil {
			return err
		}
		if err := checkContext(ctx); err != nil {
			return err
		}
		manifest.Resources = append(manifest.Resources, ManifestResource{
			Product: target.product,
			Name:    target.name,
			Shape:   ManifestResourceShape(target.entry.Spec),
			Status:  "ok",
			Path:    filepath.ToSlash(target.relPath),
			Records: recordCount,
		})
		report.Resources = append(report.Resources, buildResourceReport(target.product, target.name, target.relPath, recordCount, target.entry.Reports))
	}
	for _, resourceError := range result.Errors {
		if err := checkContext(ctx); err != nil {
			return err
		}
		manifest.Resources = append(manifest.Resources, ManifestResource{
			Product:   resourceError.Product,
			Name:      resourceError.Name,
			Status:    "error",
			Operation: resourceError.Operation,
			ErrorKind: resourceError.Kind,
		})
	}

	if err := checkContext(ctx); err != nil {
		return err
	}
	if err := writeJSONFileContext(ctx, filepath.Join(dir, "redaction_report.json"), mode, report); err != nil {
		return err
	}
	if len(result.Errors) > 0 {
		if err := checkContext(ctx); err != nil {
			return err
		}
		if err := writeNDJSONFileContext(ctx, filepath.Join(dir, "errors.ndjson"), mode, result.Errors); err != nil {
			return err
		}
	}
	// Finalize the ownership marker last. A canceled or failed write may leave
	// partial files, but never a manifest that falsely declares the artifact
	// complete or makes an incomplete directory eligible for --force removal.
	if err := checkContext(ctx); err != nil {
		return err
	}
	if err := writeJSONFileContext(ctx, filepath.Join(dir, "manifest.json"), mode, manifest); err != nil {
		return err
	}
	return nil
}

func ensureStagingParentContext(ctx context.Context, parent string) (string, error) {
	if err := checkContext(ctx); err != nil {
		return "", err
	}
	if err := os.MkdirAll(parent, dirPerm); err != nil {
		return "", fmt.Errorf("create dump parent directory: %w", err)
	}
	info, err := os.Stat(parent)
	if err != nil {
		return "", fmt.Errorf("inspect dump parent directory: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%w: dump parent %s is not a directory", ErrUnsafePath, parent)
	}
	resolved, err := validateStablePublicationParent(parent)
	if err != nil {
		return "", fmt.Errorf("%w: dump parent namespace is not stable: %v", ErrUnsafePath, err)
	}
	if err := checkContext(ctx); err != nil {
		return "", err
	}
	return resolved, nil
}

func cleanupStagingPath(path string, original os.FileInfo, beforeRelocate func(string)) {
	current, err := os.Lstat(path)
	if err != nil || !os.SameFile(original, current) {
		return
	}
	if err := preflightProcessStagingCleanup(path, original); err != nil {
		return
	}
	quarantine, err := os.MkdirTemp(filepath.Dir(path), ".zscalerctl-discard-*")
	if err != nil {
		return
	}
	if err := os.Chmod(quarantine, dirPerm); err != nil {
		return
	}
	quarantinedRoot := filepath.Join(quarantine, "root")
	if beforeRelocate != nil {
		beforeRelocate(path)
	}
	if err := preflightProcessStagingCleanup(path, original); err != nil {
		return
	}
	if err := renameNoReplace(path, quarantinedRoot); err != nil {
		return
	}
	movedInfo, err := os.Lstat(quarantinedRoot)
	if err != nil || !os.SameFile(original, movedInfo) {
		_ = renameNoReplace(quarantinedRoot, path)
		return
	}
	root, err := os.OpenRoot(quarantinedRoot)
	if err != nil {
		_ = renameNoReplace(quarantinedRoot, path)
		return
	}
	clearErr := clearDumpRootContext(context.Background(), root)
	_ = root.Close()
	if clearErr != nil {
		_ = renameNoReplace(quarantinedRoot, path)
		return
	}
	// Never unlink either public discard pathname: a hostile writable parent can
	// substitute an ancestor after any identity check. The empty 0700 directory
	// skeleton contains no tenant data and is safe to leave behind.
}

func preflightProcessStagingCleanup(path string, original os.FileInfo) error {
	current, err := os.Lstat(path)
	if err != nil || !os.SameFile(original, current) || !current.IsDir() {
		return fmt.Errorf("%w: process staging root changed before cleanup", ErrUnsafePath)
	}
	return filepath.WalkDir(path, func(entryPath string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("%w: process staging path is a symlink", ErrUnsafePath)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.IsDir() && info.Mode().Perm()&0o300 != 0o300 {
			return fmt.Errorf("%w: process staging directory is not owner-writable", ErrUnsafePath)
		}
		if err := validateAbsoluteCleanupEntry(entryPath, info, info.IsDir()); err != nil {
			return err
		}
		return nil
	})
}

func publishDirectoryNoReplace(stagingDir, destination string) error {
	if err := renameNoReplace(stagingDir, destination); err != nil {
		if errors.Is(err, os.ErrExist) {
			return fmt.Errorf("%w: %s", ErrUnsafeOverwrite, destination)
		}
		return fmt.Errorf("publish dump directory: %w", err)
	}
	return nil
}

// ManifestResourceShape returns the v2 manifest encoding for a catalog shape.
// Ordinary list shape is represented by omission; singleton is explicit.
func ManifestResourceShape(spec resources.ResourceSpec) string {
	shape := spec.EffectiveShape()
	if shape == resources.ShapeList {
		return ""
	}
	return string(shape)
}

func buildResourceTargetsContext(ctx context.Context, dir string, entries []ResourceDump) ([]resourceTarget, error) {
	ctx = contextOrBackground(ctx)
	targets := make([]resourceTarget, 0, len(entries))
	for _, entry := range entries {
		if err := checkContext(ctx); err != nil {
			return nil, err
		}
		product, err := safeSegment(string(entry.Spec.Product))
		if err != nil {
			return nil, fmt.Errorf("dump product %q: %w", entry.Spec.Product, err)
		}
		name, err := safeSegment(entry.Spec.Name)
		if err != nil {
			return nil, fmt.Errorf("dump resource %q: %w", entry.Spec.Name, err)
		}
		relPath := filepath.Join("resources", product, name+".json")
		targets = append(targets, resourceTarget{
			entry:   entry,
			product: product,
			name:    name,
			relPath: relPath,
			path:    filepath.Join(dir, relPath),
		})
	}
	return targets, nil
}

func (d ResourceDump) payload() (safeJSON, int) {
	if d.Record != nil {
		return *d.Record, 1
	}
	return d.Records, d.Records.Len()
}

func buildResourceReport(
	product, name, relPath string,
	recordCount int,
	reports []resources.ProjectionReport,
) ResourceReport {
	return ResourceReport{
		Product:        product,
		Name:           name,
		Path:           filepath.ToSlash(relPath),
		Records:        recordCount,
		IncludedFields: uniqueFields(reports, func(r resources.ProjectionReport) []string { return r.IncludedFields }),
		DroppedFields:  uniqueFields(reports, func(r resources.ProjectionReport) []string { return r.DroppedFields }),
		RedactedFields: uniqueFields(reports, func(r resources.ProjectionReport) []string { return r.RedactedFields }),
	}
}

func uniqueFields(
	reports []resources.ProjectionReport,
	selectFields func(resources.ProjectionReport) []string,
) []string {
	seen := map[string]struct{}{}
	for _, report := range reports {
		for _, field := range selectFields(report) {
			seen[field] = struct{}{}
		}
	}
	fields := make([]string, 0, len(seen))
	for field := range seen {
		fields = append(fields, field)
	}
	sort.Strings(fields)
	return fields
}

func writeJSONFile(path string, mode redact.Mode, value safeJSON) error {
	return writeJSONFileContext(context.Background(), path, mode, value)
}

func writeJSONFileContext(ctx context.Context, path string, mode redact.Mode, value safeJSON) error {
	ctx = contextOrBackground(ctx)
	if err := checkContext(ctx); err != nil {
		return err
	}
	if err := rejectExistingContext(ctx, path); err != nil {
		return err
	}
	body, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal dump json: %w", err)
	}
	body = append(body, '\n')
	body = redact.New(mode).Bytes(body)
	if err := checkContext(ctx); err != nil {
		return err
	}
	return writeFileExclusiveContext(ctx, path, body)
}

func writeNDJSONFileContext(ctx context.Context, path string, mode redact.Mode, values []ResourceError) error {
	ctx = contextOrBackground(ctx)
	if err := checkContext(ctx); err != nil {
		return err
	}
	if err := rejectExistingContext(ctx, path); err != nil {
		return err
	}
	var body []byte
	for _, value := range values {
		if err := checkContext(ctx); err != nil {
			return err
		}
		line, err := json.Marshal(value)
		if err != nil {
			return fmt.Errorf("marshal dump ndjson: %w", err)
		}
		body = append(body, line...)
		body = append(body, '\n')
	}
	body = redact.New(mode).Bytes(body)
	if err := checkContext(ctx); err != nil {
		return err
	}
	return writeFileExclusiveContext(ctx, path, body)
}

func rejectExistingContext(ctx context.Context, path string) error {
	ctx = contextOrBackground(ctx)
	if err := checkContext(ctx); err != nil {
		return err
	}
	_, err := os.Lstat(path)
	if contextErr := checkContext(ctx); contextErr != nil {
		return contextErr
	}
	if err == nil {
		return fmt.Errorf("%w: %s", ErrUnsafeOverwrite, path)
	}
	if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect dump path: %w", err)
	}
	return nil
}

func writeFileExclusive(path string, body []byte) error {
	return writeFileExclusiveContext(context.Background(), path, body)
}

func writeFileExclusiveContext(ctx context.Context, path string, body []byte) error {
	ctx = contextOrBackground(ctx)
	if err := checkContext(ctx); err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := ensureDirContext(ctx, dir); err != nil {
		return err
	}
	if err := checkContext(ctx); err != nil {
		return err
	}
	// Write to a temp file in the same directory, fsync it, then atomically
	// rename it into place. A crash mid-write can then never leave a partial
	// file at the final path — which rejectExisting would otherwise treat as an
	// unsafe pre-existing output, blocking reruns to that directory.
	tmp, err := os.CreateTemp(dir, ".tmp-"+filepath.Base(path)+"-*")
	if err != nil {
		return fmt.Errorf("create dump file: %w", err)
	}
	tmpPath := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()
	if err := checkContext(ctx); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(filePerm); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("set dump file mode: %w", err)
	}
	if err := checkContext(ctx); err != nil {
		_ = tmp.Close()
		return err
	}
	// Check immediately before writing the temp file so cancellation leaves no
	// finalized or intermediate output for this file.
	if _, err := tmp.Write(body); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write dump file: %w", err)
	}
	if err := checkContext(ctx); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync dump file: %w", err)
	}
	if err := checkContext(ctx); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close dump file: %w", err)
	}
	if err := checkContext(ctx); err != nil {
		return err
	}
	if err := renameNoReplace(tmpPath, path); err != nil {
		if errors.Is(err, os.ErrExist) {
			return fmt.Errorf("%w: %s", ErrUnsafeOverwrite, path)
		}
		return fmt.Errorf("finalize dump file: %w", err)
	}
	cleanup = false
	return nil
}

func ensureDir(dir string) error {
	return ensureDirContext(context.Background(), dir)
}

func ensureDirContext(ctx context.Context, dir string) error {
	ctx = contextOrBackground(ctx)
	if err := checkContext(ctx); err != nil {
		return err
	}
	info, err := os.Lstat(dir)
	if contextErr := checkContext(ctx); contextErr != nil {
		return contextErr
	}
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("inspect dump directory: %w", err)
		}
		if err := checkContext(ctx); err != nil {
			return err
		}
		if err := os.MkdirAll(dir, dirPerm); err != nil {
			return fmt.Errorf("create dump directory: %w", err)
		}
		if err := checkContext(ctx); err != nil {
			return err
		}
		info, err = os.Lstat(dir)
		if contextErr := checkContext(ctx); contextErr != nil {
			return contextErr
		}
		if err != nil {
			return fmt.Errorf("inspect dump directory: %w", err)
		}
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%w: %s is a symlink", ErrUnsafePath, dir)
	}
	if !info.IsDir() {
		return fmt.Errorf("%w: %s is not a directory", ErrUnsafePath, dir)
	}
	if err := checkContext(ctx); err != nil {
		return err
	}
	if err := os.Chmod(dir, dirPerm); err != nil {
		return fmt.Errorf("chmod dump directory: %w", err)
	}
	if err := checkContext(ctx); err != nil {
		return err
	}
	return nil
}

func ensureDirChainContext(ctx context.Context, root string, relDir string) error {
	ctx = contextOrBackground(ctx)
	if err := checkContext(ctx); err != nil {
		return err
	}
	if err := ensureDirContext(ctx, root); err != nil {
		return err
	}
	for _, part := range strings.Split(filepath.Clean(relDir), string(os.PathSeparator)) {
		if err := checkContext(ctx); err != nil {
			return err
		}
		if part == "." || part == "" {
			continue
		}
		root = filepath.Join(root, part)
		if err := ensureDirContext(ctx, root); err != nil {
			return err
		}
	}
	return nil
}

func safeSegment(value string) (string, error) {
	if value == "" {
		return "", ErrUnsafePath
	}
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' {
			continue
		}
		return "", ErrUnsafePath
	}
	return value, nil
}
