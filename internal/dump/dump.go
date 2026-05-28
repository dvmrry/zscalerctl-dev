package dump

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

const (
	dirPerm     os.FileMode = 0o700
	filePerm    os.FileMode = 0o600
	dumpWarning             = "sanitized dumps remain confidential operational data"
)

var (
	ErrUnsafeOverwrite = errors.New("refusing to overwrite existing dump file")
	ErrUnsafePath      = errors.New("unsafe dump path")
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
		Schema:    "zscalerctl.dump.error.v1",
		Product:   string(product),
		Name:      name,
		Operation: operation,
		Kind:      kind,
	}
}

type Manifest struct {
	Schema     string             `json:"schema"`
	Redaction  string             `json:"redaction"`
	Warning    string             `json:"warning"`
	Status     string             `json:"status"`
	Errors     int                `json:"errors,omitempty"`
	ErrorsPath string             `json:"errors_path,omitempty"`
	Resources  []ManifestResource `json:"resources"`
}

func (Manifest) OutputSafe() {}

type ManifestResource struct {
	Product   string `json:"product"`
	Name      string `json:"name"`
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

func Write(dir string, mode redact.Mode, result Result) error {
	if strings.TrimSpace(dir) == "" {
		return fmt.Errorf("%w: missing dump directory", ErrUnsafePath)
	}
	mode = redact.EffectiveMode(mode)

	targets, err := buildResourceTargets(dir, result.Entries)
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
		if err := rejectExisting(path); err != nil {
			return err
		}
	}
	if err := ensureDir(dir); err != nil {
		return err
	}

	manifest := Manifest{
		Schema:    "zscalerctl.dump.manifest.v1",
		Redaction: string(mode),
		Warning:   dumpWarning,
		Status:    "complete",
	}
	if len(result.Errors) > 0 {
		manifest.Status = "partial"
		manifest.Errors = len(result.Errors)
		manifest.ErrorsPath = "errors.ndjson"
	}
	report := RedactionReport{Schema: "zscalerctl.redaction_report.v1", Redaction: string(mode)}

	for _, target := range targets {
		if err := ensureDirChain(dir, filepath.Dir(target.relPath)); err != nil {
			return err
		}
		if err := writeJSONFile(target.path, mode, target.entry.Records); err != nil {
			return err
		}
		recordCount := len(target.entry.Records.Records())
		manifest.Resources = append(manifest.Resources, ManifestResource{
			Product: target.product,
			Name:    target.name,
			Status:  "complete",
			Path:    filepath.ToSlash(target.relPath),
			Records: recordCount,
		})
		report.Resources = append(report.Resources, buildResourceReport(target.product, target.name, target.relPath, recordCount, target.entry.Reports))
	}
	for _, resourceError := range result.Errors {
		manifest.Resources = append(manifest.Resources, ManifestResource{
			Product:   resourceError.Product,
			Name:      resourceError.Name,
			Status:    "error",
			Operation: resourceError.Operation,
			ErrorKind: resourceError.Kind,
		})
	}

	if err := writeJSONFile(filepath.Join(dir, "manifest.json"), mode, manifest); err != nil {
		return err
	}
	if err := writeJSONFile(filepath.Join(dir, "redaction_report.json"), mode, report); err != nil {
		return err
	}
	if len(result.Errors) > 0 {
		if err := writeNDJSONFile(filepath.Join(dir, "errors.ndjson"), mode, result.Errors); err != nil {
			return err
		}
	}
	return nil
}

func buildResourceTargets(dir string, entries []ResourceDump) ([]resourceTarget, error) {
	targets := make([]resourceTarget, 0, len(entries))
	for _, entry := range entries {
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
	if err := rejectExisting(path); err != nil {
		return err
	}
	body, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal dump json: %w", err)
	}
	body = append(body, '\n')
	body = redact.New(mode).Bytes(body)
	return writeFileAtomic(path, body)
}

func writeNDJSONFile(path string, mode redact.Mode, values []ResourceError) error {
	if err := rejectExisting(path); err != nil {
		return err
	}
	var body []byte
	for _, value := range values {
		line, err := json.Marshal(value)
		if err != nil {
			return fmt.Errorf("marshal dump ndjson: %w", err)
		}
		body = append(body, line...)
		body = append(body, '\n')
	}
	body = redact.New(mode).Bytes(body)
	return writeFileAtomic(path, body)
}

func rejectExisting(path string) error {
	if _, err := os.Lstat(path); err == nil {
		return fmt.Errorf("%w: %s", ErrUnsafeOverwrite, path)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect dump path: %w", err)
	}
	return nil
}

func writeFileAtomic(path string, body []byte) error {
	if err := ensureDir(filepath.Dir(path)); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return fmt.Errorf("create temp dump file: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpName)
		}
	}()
	if err := tmp.Chmod(filePerm); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("chmod temp dump file: %w", err)
	}
	if _, err := tmp.Write(body); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp dump file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp dump file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("commit dump file: %w", err)
	}
	cleanup = false
	return nil
}

func ensureDir(dir string) error {
	if err := os.MkdirAll(dir, dirPerm); err != nil {
		return fmt.Errorf("create dump directory: %w", err)
	}
	info, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("stat dump directory: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%w: %s is not a directory", ErrUnsafePath, dir)
	}
	if err := os.Chmod(dir, dirPerm); err != nil {
		return fmt.Errorf("chmod dump directory: %w", err)
	}
	return nil
}

func ensureDirChain(root string, relDir string) error {
	if err := ensureDir(root); err != nil {
		return err
	}
	for _, part := range strings.Split(filepath.Clean(relDir), string(os.PathSeparator)) {
		if part == "." || part == "" {
			continue
		}
		root = filepath.Join(root, part)
		if err := ensureDir(root); err != nil {
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
