package diff

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/dvmrry/zscalerctl/internal/dump"
	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

const SchemaID = "zscalerctl.diff.v1"

const (
	maxManifestBytes         int64 = 1 << 20
	maxResourceBytes         int64 = 512 << 20
	maxExpandedIdentityBytes       = 8 << 10
)

var (
	ErrInvalidDump        = errors.New("invalid dump")
	ErrPartialDumpInput   = errors.New("partial dump input")
	ErrRedactionMismatch  = errors.New("redaction mode mismatch")
	ErrInvalidCatalog     = errors.New("invalid catalog")
	errTrailingJSON       = errors.New("unexpected trailing JSON value")
	errUnexpectedEndJSON  = errors.New("unexpected end of JSON input")
	errUnexpectedArrayEnd = errors.New("unexpected token after resource array")
)

type Options struct {
	Catalog           resources.ResourceCatalog
	Products          map[resources.Product]bool
	Resources         map[ResourceKey]bool
	IgnoreOperational bool
	AllowPartial      bool
}

type ResourceKey struct {
	Product resources.Product
	Name    string
}

type Progress struct {
	Product  resources.Product
	Resource string
	Done     int
	Total    int
}

type ProgressFunc func(Progress) error

type Report struct {
	Schema    string         `json:"schema"`
	Old       DumpRef        `json:"old"`
	New       DumpRef        `json:"new"`
	Summary   Summary        `json:"summary"`
	Resources []ResourceDiff `json:"resources"`
}

func (Report) OutputSafe() {}

func (r Report) HasDrift() bool {
	return r.Summary.RecordsAdded > 0 || r.Summary.RecordsRemoved > 0 || r.Summary.RecordsChanged > 0
}

// CloneReport returns a recursively independent copy of report. In particular,
// values in field changes and records are copied rather than merely copying
// the containing report structs.
func CloneReport(report Report) Report {
	out := report
	if report.Resources != nil {
		out.Resources = make([]ResourceDiff, len(report.Resources))
		for i, resource := range report.Resources {
			out.Resources[i] = cloneResourceDiff(resource)
		}
	}
	return out
}

func cloneResourceDiff(resource ResourceDiff) ResourceDiff {
	out := resource
	out.Added = cloneRecordRefs(resource.Added)
	out.Removed = cloneRecordRefs(resource.Removed)
	if resource.Changed != nil {
		out.Changed = make([]RecordChange, len(resource.Changed))
		for i, change := range resource.Changed {
			out.Changed[i] = change
			if change.Changes != nil {
				out.Changed[i].Changes = make([]FieldChange, len(change.Changes))
				for j, fieldChange := range change.Changes {
					out.Changed[i].Changes[j] = FieldChange{
						Field: fieldChange.Field,
						Old:   cloneReportAny(fieldChange.Old),
						New:   cloneReportAny(fieldChange.New),
					}
				}
			}
		}
	}
	return out
}

func cloneRecordRefs(refs []RecordRef) []RecordRef {
	if refs == nil {
		return nil
	}
	out := make([]RecordRef, len(refs))
	for i, ref := range refs {
		out[i] = ref
		out[i].Record = cloneReportMap(ref.Record)
	}
	return out
}

func cloneReportMap(value map[string]any) map[string]any {
	if value == nil {
		return nil
	}
	out := make(map[string]any, len(value))
	for key, item := range value {
		out[key] = cloneReportAny(item)
	}
	return out
}

func cloneReportAny(value any) any {
	if value == nil {
		return nil
	}
	return cloneReportValue(reflect.ValueOf(value)).Interface()
}

func cloneReportValue(value reflect.Value) reflect.Value {
	if !value.IsValid() {
		return value
	}
	switch value.Kind() {
	case reflect.Interface:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		cloned := cloneReportValue(value.Elem())
		out := reflect.New(value.Type()).Elem()
		out.Set(cloned)
		return out
	case reflect.Map:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		out := reflect.MakeMapWithSize(value.Type(), value.Len())
		iter := value.MapRange()
		for iter.Next() {
			key := cloneReportValue(iter.Key())
			item := cloneReportValue(iter.Value())
			out.SetMapIndex(key, item)
		}
		return out
	case reflect.Slice:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		out := reflect.MakeSlice(value.Type(), value.Len(), value.Len())
		for i := 0; i < value.Len(); i++ {
			out.Index(i).Set(cloneReportValue(value.Index(i)))
		}
		return out
	case reflect.Array:
		out := reflect.New(value.Type()).Elem()
		for i := 0; i < value.Len(); i++ {
			out.Index(i).Set(cloneReportValue(value.Index(i)))
		}
		return out
	case reflect.Pointer:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		out := reflect.New(value.Type().Elem())
		out.Elem().Set(cloneReportValue(value.Elem()))
		return out
	default:
		return value
	}
}

type DumpRef struct {
	Path           string `json:"path"`
	ManifestSchema string `json:"manifest_schema"`
	Redaction      string `json:"redaction"`
	Status         string `json:"status"`
	Partial        bool   `json:"partial"`
}

type Summary struct {
	ResourcesCompared  int `json:"resources_compared"`
	ResourcesWithDrift int `json:"resources_with_drift"`
	RecordsAdded       int `json:"records_added"`
	RecordsRemoved     int `json:"records_removed"`
	RecordsChanged     int `json:"records_changed"`
}

type ResourceDiff struct {
	Product  string         `json:"product"`
	Resource string         `json:"resource"`
	Identity Identity       `json:"identity"`
	Added    []RecordRef    `json:"added"`
	Removed  []RecordRef    `json:"removed"`
	Changed  []RecordChange `json:"changed"`
	Note     string         `json:"note,omitempty"`
}

func (r ResourceDiff) HasDrift() bool {
	return len(r.Added) > 0 || len(r.Removed) > 0 || len(r.Changed) > 0
}

type Identity struct {
	Mode  string `json:"mode"`
	Field string `json:"field,omitempty"`
}

type RecordRef struct {
	Key    string         `json:"key,omitempty"`
	Hash   string         `json:"hash,omitempty"`
	Record map[string]any `json:"record,omitempty"`
}

type RecordChange struct {
	Key     string        `json:"key"`
	Changes []FieldChange `json:"changes"`
}

type FieldChange struct {
	Field string `json:"field"`
	Old   any    `json:"old"`
	New   any    `json:"new"`
}

type loadedDump struct {
	ref       DumpRef
	manifest  dump.Manifest
	resources map[ResourceKey]loadedResource
}

type loadedResource struct {
	manifest dump.ManifestResource
	records  []map[string]any
}

func Compare(oldDir, newDir string, opts Options) (Report, error) {
	return CompareContext(context.Background(), oldDir, newDir, opts, nil)
}

func CompareContext(ctx context.Context, oldDir, newDir string, opts Options, progress ProgressFunc) (Report, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := checkContext(ctx); err != nil {
		return Report{}, err
	}
	catalog := opts.Catalog
	if len(catalog) == 0 {
		catalog = resources.Catalog()
	}
	catalogSpecs, err := validateCatalog(catalog)
	if err != nil {
		return Report{}, err
	}
	specs, err := selectedSpecs(ctx, catalog, opts)
	if err != nil {
		return Report{}, err
	}
	selected := make(map[ResourceKey]bool, len(specs))
	for _, spec := range specs {
		selected[ResourceKey{Product: spec.Product, Name: spec.Name}] = true
	}
	if err := checkContext(ctx); err != nil {
		return Report{}, err
	}
	oldDump, err := loadDump(ctx, oldDir, catalogSpecs, selected)
	if err != nil {
		return Report{}, err
	}
	newDump, err := loadDump(ctx, newDir, catalogSpecs, selected)
	if err != nil {
		return Report{}, err
	}
	if err := checkContext(ctx); err != nil {
		return Report{}, err
	}
	if oldDump.manifest.Redaction != newDump.manifest.Redaction {
		return Report{}, fmt.Errorf("%w: old=%s new=%s", ErrRedactionMismatch, oldDump.manifest.Redaction, newDump.manifest.Redaction)
	}
	if !opts.AllowPartial {
		if oldDump.ref.Partial {
			return Report{}, fmt.Errorf("%w: old dump %s is partial", ErrPartialDumpInput, oldDir)
		}
		if newDump.ref.Partial {
			return Report{}, fmt.Errorf("%w: new dump %s is partial", ErrPartialDumpInput, newDir)
		}
	}
	report := Report{
		Schema: SchemaID,
		Old:    oldDump.ref,
		New:    newDump.ref,
	}
	for i, spec := range specs {
		if err := checkContext(ctx); err != nil {
			return Report{}, err
		}
		key := ResourceKey{Product: spec.Product, Name: spec.Name}
		oldRes := oldDump.resources[key]
		newRes := newDump.resources[key]
		if progress != nil {
			if err := checkContext(ctx); err != nil {
				return Report{}, err
			}
			if err := progress(Progress{
				Product:  spec.Product,
				Resource: spec.Name,
				Done:     i + 1,
				Total:    len(specs),
			}); err != nil {
				return Report{}, err
			}
			if err := checkContext(ctx); err != nil {
				return Report{}, err
			}
		}
		resourceDiff, err := compareResource(ctx, spec, oldRes.records, newRes.records, opts.IgnoreOperational)
		if err != nil {
			return Report{}, err
		}
		if len(oldRes.records) == 0 && len(newRes.records) == 0 {
			continue
		}
		report.Summary.ResourcesCompared++
		if resourceDiff.HasDrift() {
			report.Summary.ResourcesWithDrift++
		}
		report.Summary.RecordsAdded += len(resourceDiff.Added)
		report.Summary.RecordsRemoved += len(resourceDiff.Removed)
		report.Summary.RecordsChanged += len(resourceDiff.Changed)
		report.Resources = append(report.Resources, resourceDiff)
	}
	return report, nil
}

type catalogValidationError struct {
	cause error
}

func (e catalogValidationError) Error() string {
	return ErrInvalidCatalog.Error()
}

func (e catalogValidationError) Unwrap() error {
	return e.cause
}

func invalidCatalogError(cause error) error {
	if cause == nil {
		cause = ErrInvalidCatalog
	} else {
		cause = errors.Join(ErrInvalidCatalog, cause)
	}
	return catalogValidationError{cause: cause}
}

func validateCatalog(catalog resources.ResourceCatalog) (map[ResourceKey]resources.ResourceSpec, error) {
	for _, spec := range catalog {
		if err := spec.Validate(); err != nil {
			return nil, invalidCatalogError(resources.ErrInvalidResourceSpec)
		}
	}
	if err := resources.AssertReadOnly(catalog...); err != nil {
		return nil, invalidCatalogError(resources.ErrMutatingOperation)
	}

	index := make(map[ResourceKey]resources.ResourceSpec, len(catalog))
	for _, spec := range catalog {
		key := ResourceKey{Product: spec.Product, Name: spec.Name}
		if _, exists := index[key]; exists {
			return nil, invalidCatalogError(nil)
		}
		index[key] = spec
	}
	return index, nil
}

func admitRecords(ctx context.Context, spec resources.ResourceSpec, mode redact.Mode, records []map[string]any) ([]map[string]any, error) {
	ctx = normalizedContext(ctx)
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	verified, err := resources.NewVerifiedProjectedRecordsFromProjectedFields(spec, mode, records)
	if err != nil {
		if ctxErr := checkContext(ctx); ctxErr != nil {
			return nil, ctxErr
		}
		return nil, invalidAdmissionError(spec, false)
	}
	if err := checkContext(ctx); err != nil {
		return nil, err
	}

	verifiedRecords := verified.Records()
	admitted := make([]map[string]any, len(verifiedRecords))
	for i := range verifiedRecords {
		if err := checkContext(ctx); err != nil {
			return nil, err
		}
		fields := verifiedRecords[i].Fields()
		projected, _, err := resources.ProjectRecordAndVerify(spec, mode, resources.NewSourceRecord(fields))
		if err != nil {
			if ctxErr := checkContext(ctx); ctxErr != nil {
				return nil, ctxErr
			}
			return nil, invalidAdmissionError(spec, false)
		}
		projectedFields := projected.Fields()
		if !reflect.DeepEqual(projectedFields, fields) {
			return nil, invalidAdmissionError(spec, true)
		}
		admitted[i] = projectedFields
	}
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	return admitted, nil
}

func invalidAdmissionError(spec resources.ResourceSpec, nonIdempotent bool) error {
	if nonIdempotent {
		return fmt.Errorf("%w: resource %s/%s contains a non-idempotent record", ErrInvalidDump, spec.Product, spec.Name)
	}
	return fmt.Errorf("%w: resource %s/%s contains a field or value that is not admitted", ErrInvalidDump, spec.Product, spec.Name)
}

func checkContext(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	return ctx.Err()
}

func normalizedContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r contextReader) Read(p []byte) (int, error) {
	if err := checkContext(r.ctx); err != nil {
		return 0, err
	}
	n, err := r.reader.Read(p)
	if ctxErr := checkContext(r.ctx); ctxErr != nil {
		return n, ctxErr
	}
	return n, err
}

type resourceReadFailure struct {
	cause error
}

func (e *resourceReadFailure) Error() string { return e.cause.Error() }
func (e *resourceReadFailure) Unwrap() error { return e.cause }

// stickyErrorReader retains the first non-EOF reader error. json.Decoder.More
// exposes only a bool and can otherwise swallow a transient peek failure before
// the following Token call retries successfully.
type stickyErrorReader struct {
	reader  io.Reader
	failure *resourceReadFailure
}

func (r *stickyErrorReader) Read(p []byte) (int, error) {
	if r.failure != nil {
		return 0, r.failure
	}
	n, err := r.reader.Read(p)
	if err != nil && !errors.Is(err, io.EOF) {
		r.failure = &resourceReadFailure{cause: err}
		return n, r.failure
	}
	return n, err
}

func selectedSpecs(ctx context.Context, catalog resources.ResourceCatalog, opts Options) ([]resources.ResourceSpec, error) {
	specs := make([]resources.ResourceSpec, 0, len(catalog))
	for _, spec := range catalog {
		if err := checkContext(ctx); err != nil {
			return nil, err
		}
		if len(opts.Products) > 0 && !opts.Products[spec.Product] {
			continue
		}
		if len(opts.Resources) > 0 && !opts.Resources[ResourceKey{Product: spec.Product, Name: spec.Name}] {
			continue
		}
		if !spec.SupportsReadOperation("list") && !spec.SupportsReadOperation("show") {
			continue
		}
		specs = append(specs, spec)
	}
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	sort.Slice(specs, func(i, j int) bool {
		if specs[i].Product != specs[j].Product {
			return specs[i].Product < specs[j].Product
		}
		return specs[i].Name < specs[j].Name
	})
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	return specs, nil
}

func loadDump(
	ctx context.Context,
	dir string,
	catalog map[ResourceKey]resources.ResourceSpec,
	selected map[ResourceKey]bool,
) (loadedDump, error) {
	ctx = normalizedContext(ctx)
	if err := checkContext(ctx); err != nil {
		return loadedDump{}, err
	}
	info, err := os.Stat(dir)
	if ctxErr := checkContext(ctx); ctxErr != nil {
		return loadedDump{}, ctxErr
	}
	if err != nil {
		return loadedDump{}, fmt.Errorf("%w: inspect %s: %v", ErrInvalidDump, dir, err)
	}
	if !info.IsDir() {
		return loadedDump{}, fmt.Errorf("%w: %s is not a directory", ErrInvalidDump, dir)
	}
	if err := checkContext(ctx); err != nil {
		return loadedDump{}, err
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		if ctxErr := checkContext(ctx); ctxErr != nil {
			return loadedDump{}, ctxErr
		}
		return loadedDump{}, fmt.Errorf("%w: open %s: %v", ErrInvalidDump, dir, err)
	}
	defer root.Close()
	if ctxErr := checkContext(ctx); ctxErr != nil {
		return loadedDump{}, ctxErr
	}

	body, err := readRootFile(ctx, root, "manifest.json", fmt.Sprintf("manifest for %s", dir), maxManifestBytes)
	if err != nil {
		return loadedDump{}, err
	}
	var manifest dump.Manifest
	if err := json.Unmarshal(body, &manifest); err != nil {
		if ctxErr := checkContext(ctx); ctxErr != nil {
			return loadedDump{}, ctxErr
		}
		return loadedDump{}, fmt.Errorf("%w: parse manifest for %s: %v", ErrInvalidDump, dir, err)
	}
	if err := checkContext(ctx); err != nil {
		return loadedDump{}, err
	}
	if manifest.Schema != dump.ManifestSchemaID {
		return loadedDump{}, fmt.Errorf("%w: unsupported manifest schema (want %s; see docs/schema/manifest.schema.json)", ErrInvalidDump, dump.ManifestSchemaID)
	}
	mode, err := redact.ParseMode(manifest.Redaction)
	if err != nil {
		return loadedDump{}, fmt.Errorf("%w: invalid redaction mode", ErrInvalidDump)
	}
	switch manifest.Status {
	case "complete", "partial":
	default:
		return loadedDump{}, fmt.Errorf("%w: invalid manifest status (want complete or partial)", ErrInvalidDump)
	}
	loaded := loadedDump{
		ref: DumpRef{
			Path:           dir,
			ManifestSchema: manifest.Schema,
			Redaction:      manifest.Redaction,
			Status:         manifest.Status,
			Partial:        manifest.Status == "partial",
		},
		manifest:  manifest,
		resources: make(map[ResourceKey]loadedResource),
	}
	seen := make(map[ResourceKey]struct{}, len(manifest.Resources))
	for _, mr := range manifest.Resources {
		if err := checkContext(ctx); err != nil {
			return loadedDump{}, err
		}
		key := ResourceKey{Product: resources.Product(mr.Product), Name: mr.Name}
		spec, ok := catalog[key]
		if !ok {
			return loadedDump{}, fmt.Errorf("%w: manifest references an unknown resource", ErrInvalidDump)
		}
		if _, duplicate := seen[key]; duplicate {
			return loadedDump{}, fmt.Errorf("%w: resource %s/%s appears more than once in manifest", ErrInvalidDump, spec.Product, spec.Name)
		}
		seen[key] = struct{}{}
		switch mr.Status {
		case "ok":
			if strings.TrimSpace(mr.Path) == "" {
				return loadedDump{}, fmt.Errorf("%w: resource %s/%s has no path", ErrInvalidDump, spec.Product, spec.Name)
			}
			if selected[key] {
				records, err := readResource(ctx, root, mr, spec)
				if err != nil {
					return loadedDump{}, err
				}
				if len(records) != mr.Records {
					return loadedDump{}, fmt.Errorf("%w: resource %s/%s manifest record count does not match resource file", ErrInvalidDump, spec.Product, spec.Name)
				}
				admitted, err := admitRecords(ctx, spec, mode, records)
				if err != nil {
					return loadedDump{}, err
				}
				loaded.resources[key] = loadedResource{manifest: mr, records: admitted}
				continue
			}
			records, err := readUnselectedResourceCount(ctx, root, mr, spec)
			if err != nil {
				return loadedDump{}, err
			}
			if records != mr.Records {
				return loadedDump{}, fmt.Errorf("%w: resource %s/%s manifest record count does not match resource file", ErrInvalidDump, spec.Product, spec.Name)
			}
		case "error":
			loaded.ref.Partial = true
		default:
			return loadedDump{}, fmt.Errorf("%w: resource %s/%s has invalid status (want ok or error)", ErrInvalidDump, spec.Product, spec.Name)
		}
	}
	return loaded, nil
}

func readResource(ctx context.Context, root *os.Root, mr dump.ManifestResource, spec resources.ResourceSpec) ([]map[string]any, error) {
	ctx = normalizedContext(ctx)
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	path, err := localResourcePath(mr, spec)
	if err != nil {
		return nil, err
	}
	body, err := readRootFile(ctx, root, path, fmt.Sprintf("resource %s/%s", spec.Product, spec.Name), maxResourceBytes)
	if err != nil {
		return nil, err
	}
	var raw any
	if err := decodeResourceValue(body, &raw); err != nil {
		if ctxErr := checkContext(ctx); ctxErr != nil {
			return nil, ctxErr
		}
		return nil, fmt.Errorf("%w: parse resource %s/%s: %v", ErrInvalidDump, spec.Product, spec.Name, err)
	}
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	var records []map[string]any
	switch value := raw.(type) {
	case []any:
		records = make([]map[string]any, 0, len(value))
		for _, item := range value {
			if err := checkContext(ctx); err != nil {
				return nil, err
			}
			record, ok := item.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("%w: resource %s/%s contains a non-object record", ErrInvalidDump, spec.Product, spec.Name)
			}
			records = append(records, record)
		}
	case map[string]any:
		records = []map[string]any{value}
	default:
		return nil, fmt.Errorf("%w: resource %s/%s payload is not an object or array", ErrInvalidDump, spec.Product, spec.Name)
	}
	return records, nil
}

func decodeResourceValue(body []byte, destination any) error {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err == nil {
		return errTrailingJSON
	} else if !errors.Is(err, io.EOF) {
		return err
	}
	return validateDecodedNumbers(reflect.ValueOf(destination).Elem())
}

func validateDecodedNumbers(value reflect.Value) error {
	if !value.IsValid() {
		return nil
	}
	if value.Kind() == reflect.Interface {
		if value.IsNil() {
			return nil
		}
		return validateDecodedNumbers(value.Elem())
	}
	if value.CanInterface() {
		if number, ok := value.Interface().(json.Number); ok {
			if _, err := number.Float64(); err != nil {
				return err
			}
			return nil
		}
	}
	switch value.Kind() {
	case reflect.Map:
		iterator := value.MapRange()
		for iterator.Next() {
			if err := validateDecodedNumbers(iterator.Value()); err != nil {
				return err
			}
		}
	case reflect.Slice, reflect.Array:
		for i := range value.Len() {
			if err := validateDecodedNumbers(value.Index(i)); err != nil {
				return err
			}
		}
	}
	return nil
}

func localResourcePath(mr dump.ManifestResource, spec resources.ResourceSpec) (string, error) {
	path := filepath.Clean(filepath.FromSlash(mr.Path))
	if !filepath.IsLocal(path) {
		return "", fmt.Errorf("%w: resource %s/%s has unsafe path", ErrInvalidDump, spec.Product, spec.Name)
	}
	return path, nil
}

// resourceJSONShape is the structural result needed to enforce the manifest's
// record count without retaining an unselected resource's decoded values.
type resourceJSONShape struct {
	records         int
	objectOrArray   bool
	nonObjectRecord bool
}

// readUnselectedResourceCount validates one unselected resource body while
// retaining at most one decoded array record. It preserves the selected path's
// file, JSON, top-level shape, and manifest-count checks without admitting or
// retaining values that cannot enter the diff report.
func readUnselectedResourceCount(
	ctx context.Context,
	root *os.Root,
	mr dump.ManifestResource,
	spec resources.ResourceSpec,
) (int, error) {
	ctx = normalizedContext(ctx)
	if err := checkContext(ctx); err != nil {
		return 0, err
	}
	path, err := localResourcePath(mr, spec)
	if err != nil {
		return 0, err
	}
	label := fmt.Sprintf("resource %s/%s", spec.Product, spec.Name)
	file, err := openRootRegularFile(ctx, root, path, label, maxResourceBytes)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	limited := &io.LimitedReader{
		R: contextReader{ctx: ctx, reader: file},
		N: maxResourceBytes + 1,
	}
	shape, parseErr := scanResourceJSON(ctx, limited)
	if parseErr != nil {
		if ctxErr := checkContext(ctx); ctxErr != nil {
			return 0, ctxErr
		}
		if !isJSONContentError(parseErr) {
			return 0, fmt.Errorf("%w: read %s: %v", ErrInvalidDump, label, parseErr)
		}
		// readRootFile establishes size and read-error precedence before parsing.
		// Drain the remaining bounded input after an early syntax error so the
		// streaming path preserves that ordering, including if the file grew after
		// its initial stat.
		if _, err := io.Copy(io.Discard, limited); err != nil {
			if ctxErr := checkContext(ctx); ctxErr != nil {
				return 0, ctxErr
			}
			return 0, fmt.Errorf("%w: read %s: %v", ErrInvalidDump, label, err)
		}
	}
	if err := checkContext(ctx); err != nil {
		return 0, err
	}
	if limited.N == 0 {
		return 0, fmt.Errorf("%w: %s is too large", ErrInvalidDump, label)
	}
	if parseErr != nil {
		// The legacy Cobra adapter intentionally preserves local invalid-dump
		// diagnostics. Replay only malformed inputs through the bounded historical
		// decoder so valid resources retain the streaming memory win while invalid
		// inputs keep their established error text.
		replayErr := replayLegacyJSONError(ctx, file, label, maxResourceBytes)
		switch {
		case errors.Is(replayErr, context.Canceled), errors.Is(replayErr, context.DeadlineExceeded):
			return 0, replayErr
		case errors.Is(replayErr, ErrInvalidDump):
			return 0, replayErr
		case replayErr != nil:
			parseErr = replayErr
		case errors.Is(parseErr, io.EOF), errors.Is(parseErr, io.ErrUnexpectedEOF):
			parseErr = errUnexpectedEndJSON
		}
		return 0, fmt.Errorf("%w: parse resource %s/%s: %v", ErrInvalidDump, spec.Product, spec.Name, parseErr)
	}
	if !shape.objectOrArray {
		return 0, fmt.Errorf("%w: resource %s/%s payload is not an object or array", ErrInvalidDump, spec.Product, spec.Name)
	}
	if shape.nonObjectRecord {
		return 0, fmt.Errorf("%w: resource %s/%s contains a non-object record", ErrInvalidDump, spec.Product, spec.Name)
	}
	return shape.records, nil
}

func isJSONContentError(err error) bool {
	var readFailure *resourceReadFailure
	if errors.As(err, &readFailure) {
		return false
	}
	if errors.Is(err, errTrailingJSON) || errors.Is(err, errUnexpectedArrayEnd) ||
		errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}
	var syntaxErr *json.SyntaxError
	if errors.As(err, &syntaxErr) {
		return true
	}
	var typeErr *json.UnmarshalTypeError
	return errors.As(err, &typeErr)
}

// replayLegacyJSONError reproduces the pre-streaming decoder diagnostic for an
// already-invalid body. It is never called for valid unselected resources.
func replayLegacyJSONError(
	ctx context.Context,
	file *os.File,
	label string,
	maxBytes int64,
) error {
	if err := checkContext(ctx); err != nil {
		return err
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		if ctxErr := checkContext(ctx); ctxErr != nil {
			return ctxErr
		}
		return fmt.Errorf("%w: read %s: %v", ErrInvalidDump, label, err)
	}
	body, err := io.ReadAll(io.LimitReader(contextReader{ctx: ctx, reader: file}, maxBytes+1))
	if err != nil {
		if ctxErr := checkContext(ctx); ctxErr != nil {
			return ctxErr
		}
		return fmt.Errorf("%w: read %s: %v", ErrInvalidDump, label, err)
	}
	if err := checkContext(ctx); err != nil {
		return err
	}
	if int64(len(body)) > maxBytes {
		return fmt.Errorf("%w: %s is too large", ErrInvalidDump, label)
	}
	var raw any
	parseErr := json.Unmarshal(body, &raw)
	if err := checkContext(ctx); err != nil {
		return err
	}
	return parseErr
}

// scanResourceJSON parses one complete JSON value and reports only its resource
// shape. Array records are decoded one at a time so context cancellation and GC
// can make progress between records.
func scanResourceJSON(ctx context.Context, reader io.Reader) (shape resourceJSONShape, err error) {
	sticky := &stickyErrorReader{reader: reader}
	decoder := json.NewDecoder(sticky)
	defer func() {
		if ctxErr := checkContext(ctx); ctxErr != nil {
			err = ctxErr
			return
		}
		if sticky.failure != nil {
			err = sticky.failure
		}
	}()
	first, err := nextJSONToken(ctx, decoder)
	if err != nil {
		return shape, err
	}
	delim, composite := first.(json.Delim)
	switch {
	case composite && delim == '{':
		shape.objectOrArray = true
		shape.records = 1
		if err := consumeJSONComposite(ctx, decoder); err != nil {
			return shape, err
		}
	case composite && delim == '[':
		shape.objectOrArray = true
		var item any
		for decoder.More() {
			item = nil
			err := decodeJSONValue(ctx, decoder, &item)
			if err != nil {
				return shape, err
			}
			if _, ok := item.(map[string]any); ok {
				shape.records++
			} else {
				shape.nonObjectRecord = true
			}
		}
		closing, err := nextJSONToken(ctx, decoder)
		if err != nil {
			return shape, err
		}
		if closing != json.Delim(']') {
			return shape, errUnexpectedArrayEnd
		}
	}

	if _, err := nextJSONToken(ctx, decoder); err == nil {
		return shape, errTrailingJSON
	} else if !errors.Is(err, io.EOF) {
		return shape, err
	}
	return shape, nil
}

func consumeJSONComposite(ctx context.Context, decoder *json.Decoder) error {
	depth := 1
	for depth > 0 {
		token, err := nextJSONToken(ctx, decoder)
		if err != nil {
			return err
		}
		delim, ok := token.(json.Delim)
		if !ok {
			continue
		}
		switch delim {
		case '{', '[':
			depth++
		case '}', ']':
			depth--
		}
	}
	return nil
}

func nextJSONToken(ctx context.Context, decoder *json.Decoder) (json.Token, error) {
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	token, err := decoder.Token()
	if ctxErr := checkContext(ctx); ctxErr != nil {
		return nil, ctxErr
	}
	return token, err
}

func decodeJSONValue(ctx context.Context, decoder *json.Decoder, value any) error {
	if err := checkContext(ctx); err != nil {
		return err
	}
	err := decoder.Decode(value)
	if ctxErr := checkContext(ctx); ctxErr != nil {
		return ctxErr
	}
	return err
}

func readRootFile(ctx context.Context, root *os.Root, name, label string, maxBytes int64) ([]byte, error) {
	ctx = normalizedContext(ctx)
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	file, err := openRootRegularFile(ctx, root, name, label, maxBytes)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	body, err := io.ReadAll(io.LimitReader(contextReader{ctx: ctx, reader: file}, maxBytes+1))
	if err != nil {
		if ctxErr := checkContext(ctx); ctxErr != nil {
			return nil, ctxErr
		}
		return nil, fmt.Errorf("%w: read %s: %v", ErrInvalidDump, label, err)
	}
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	if int64(len(body)) > maxBytes {
		return nil, fmt.Errorf("%w: %s is too large", ErrInvalidDump, label)
	}
	return body, nil
}

// openRootRegularFile opens and validates a bounded regular file below root.
// The caller owns and must close a successful result.
func openRootRegularFile(
	ctx context.Context,
	root *os.Root,
	name, label string,
	maxBytes int64,
) (*os.File, error) {
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	file, err := root.Open(name)
	if err != nil {
		if ctxErr := checkContext(ctx); ctxErr != nil {
			return nil, ctxErr
		}
		return nil, fmt.Errorf("%w: read %s: %v", ErrInvalidDump, label, err)
	}
	if ctxErr := checkContext(ctx); ctxErr != nil {
		file.Close()
		return nil, ctxErr
	}
	info, err := file.Stat()
	if ctxErr := checkContext(ctx); ctxErr != nil {
		file.Close()
		return nil, ctxErr
	}
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("%w: inspect %s: %v", ErrInvalidDump, label, err)
	}
	if !info.Mode().IsRegular() {
		file.Close()
		return nil, fmt.Errorf("%w: %s is not a regular file", ErrInvalidDump, label)
	}
	if info.Size() > maxBytes {
		file.Close()
		return nil, fmt.Errorf("%w: %s is too large", ErrInvalidDump, label)
	}
	if err := checkContext(ctx); err != nil {
		file.Close()
		return nil, err
	}
	return file, nil
}

func compareResource(ctx context.Context, spec resources.ResourceSpec, oldRecords, newRecords []map[string]any, ignoreOperational bool) (ResourceDiff, error) {
	if err := checkContext(ctx); err != nil {
		return ResourceDiff{}, err
	}
	out := ResourceDiff{
		Product:  string(spec.Product),
		Resource: spec.Name,
		Added:    []RecordRef{},
		Removed:  []RecordRef{},
		Changed:  []RecordChange{},
	}
	switch {
	case spec.EffectiveShape() == resources.ShapeSingleton:
		out.Identity = Identity{Mode: "singleton"}
		return compareSingleton(ctx, spec, oldRecords, newRecords, ignoreOperational, out)
	case spec.EffectiveGetKey() != "":
		out.Identity = Identity{Mode: "get_key", Field: spec.EffectiveGetKey()}
		return compareKeyed(ctx, spec, oldRecords, newRecords, ignoreOperational, out)
	default:
		out.Identity = Identity{Mode: "content_hash"}
		out.Note = "identity unavailable; compared by canonical content hash"
		return compareContentHash(ctx, spec, oldRecords, newRecords, out)
	}
}

func compareSingleton(ctx context.Context, spec resources.ResourceSpec, oldRecords, newRecords []map[string]any, ignoreOperational bool, out ResourceDiff) (ResourceDiff, error) {
	if err := checkContext(ctx); err != nil {
		return out, err
	}
	if len(oldRecords) > 1 || len(newRecords) > 1 {
		return out, fmt.Errorf("%w: singleton resource %s/%s has multiple records", ErrInvalidDump, spec.Product, spec.Name)
	}
	switch {
	case len(oldRecords) == 0 && len(newRecords) == 1:
		record, err := filterRecordContext(ctx, spec, newRecords[0], ignoreOperational)
		if err != nil {
			return out, err
		}
		out.Added = append(out.Added, RecordRef{Key: "singleton", Record: record})
	case len(oldRecords) == 1 && len(newRecords) == 0:
		record, err := filterRecordContext(ctx, spec, oldRecords[0], ignoreOperational)
		if err != nil {
			return out, err
		}
		out.Removed = append(out.Removed, RecordRef{Key: "singleton", Record: record})
	case len(oldRecords) == 1 && len(newRecords) == 1:
		oldRecord, err := filterRecordContext(ctx, spec, oldRecords[0], ignoreOperational)
		if err != nil {
			return out, err
		}
		newRecord, err := filterRecordContext(ctx, spec, newRecords[0], ignoreOperational)
		if err != nil {
			return out, err
		}
		changes, err := diffFieldsContext(ctx, oldRecord, newRecord)
		if err != nil {
			return out, err
		}
		if len(changes) > 0 {
			out.Changed = append(out.Changed, RecordChange{Key: "singleton", Changes: changes})
		}
	}
	return out, nil
}

func compareKeyed(ctx context.Context, spec resources.ResourceSpec, oldRecords, newRecords []map[string]any, ignoreOperational bool, out ResourceDiff) (ResourceDiff, error) {
	if err := checkContext(ctx); err != nil {
		return out, err
	}
	keyField := spec.EffectiveGetKey()
	oldByKey, err := keyedRecords(ctx, spec, keyField, oldRecords, ignoreOperational)
	if err != nil {
		return out, err
	}
	newByKey, err := keyedRecords(ctx, spec, keyField, newRecords, ignoreOperational)
	if err != nil {
		return out, err
	}
	for _, key := range sortedKeys(oldByKey, newByKey) {
		if err := checkContext(ctx); err != nil {
			return out, err
		}
		oldRecord, oldOK := oldByKey[key]
		newRecord, newOK := newByKey[key]
		switch {
		case !oldOK:
			out.Added = append(out.Added, RecordRef{Key: key, Record: newRecord})
		case !newOK:
			out.Removed = append(out.Removed, RecordRef{Key: key, Record: oldRecord})
		default:
			changes, err := diffFieldsContext(ctx, oldRecord, newRecord)
			if err != nil {
				return out, err
			}
			if len(changes) > 0 {
				out.Changed = append(out.Changed, RecordChange{Key: key, Changes: changes})
			}
		}
	}
	return out, nil
}

func keyedRecords(ctx context.Context, spec resources.ResourceSpec, keyField string, records []map[string]any, ignoreOperational bool) (map[string]map[string]any, error) {
	out := make(map[string]map[string]any, len(records))
	for i, record := range records {
		if err := checkContext(ctx); err != nil {
			return nil, err
		}
		raw, ok := record[keyField]
		if !ok || raw == nil {
			return nil, fmt.Errorf("%w: resource %s/%s record %d missing identity field %q", ErrInvalidDump, spec.Product, spec.Name, i, keyField)
		}
		key := identityString(raw)
		if key == "" {
			return nil, fmt.Errorf("%w: resource %s/%s record %d has empty identity field %q", ErrInvalidDump, spec.Product, spec.Name, i, keyField)
		}
		if _, exists := out[key]; exists {
			return nil, fmt.Errorf("%w: resource %s/%s has duplicate identity %q", ErrInvalidDump, spec.Product, spec.Name, key)
		}
		filtered, err := filterRecordContext(ctx, spec, record, ignoreOperational)
		if err != nil {
			return nil, err
		}
		out[key] = filtered
	}
	return out, nil
}

func identityString(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case json.Number:
		canonical, err := canonicalIdentityNumber(v.String())
		if err != nil {
			return v.String()
		}
		return canonical
	default:
		return fmt.Sprint(v)
	}
}

func canonicalIdentityNumber(lexeme string) (string, error) {
	canonical, err := canonicalNumber(lexeme)
	if err != nil {
		return "", err
	}
	value := canonical.String()
	if value == "0" {
		return value, nil
	}
	negative := strings.HasPrefix(value, "-")
	if negative {
		value = value[1:]
	}
	parts := strings.SplitN(value, "e", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("canonical number has no exponent")
	}
	exponentValue, ok := new(big.Int).SetString(parts[1], 10)
	if !ok {
		return "", fmt.Errorf("canonical number has an invalid exponent")
	}
	if !exponentValue.IsInt64() {
		return canonical.String(), nil
	}
	exponent64 := exponentValue.Int64()
	if exponent64 < -maxExpandedIdentityBytes || exponent64 > maxExpandedIdentityBytes {
		return canonical.String(), nil
	}
	digits := strings.ReplaceAll(parts[0], ".", "")
	if len(digits) > maxExpandedIdentityBytes {
		return canonical.String(), nil
	}
	exponent := int(exponent64)
	scale := exponent - (len(digits) - 1)
	var plain string
	switch {
	case scale >= 0:
		if len(digits)+scale > maxExpandedIdentityBytes {
			return canonical.String(), nil
		}
		plain = digits + strings.Repeat("0", scale)
	case len(digits)+scale > 0:
		point := len(digits) + scale
		plain = digits[:point] + "." + digits[point:]
	default:
		leadingZeros := -(len(digits) + scale)
		if 2+leadingZeros+len(digits) > maxExpandedIdentityBytes {
			return canonical.String(), nil
		}
		plain = "0." + strings.Repeat("0", leadingZeros) + digits
	}
	if negative && plain != "0" {
		if len(plain)+1 > maxExpandedIdentityBytes {
			return canonical.String(), nil
		}
		plain = "-" + plain
	}
	return plain, nil
}

func compareContentHash(ctx context.Context, spec resources.ResourceSpec, oldRecords, newRecords []map[string]any, out ResourceDiff) (ResourceDiff, error) {
	if err := checkContext(ctx); err != nil {
		return out, err
	}
	oldByHash, err := hashedRecords(ctx, spec, oldRecords)
	if err != nil {
		return out, err
	}
	newByHash, err := hashedRecords(ctx, spec, newRecords)
	if err != nil {
		return out, err
	}
	for _, hash := range sortedHashKeys(oldByHash, newByHash) {
		if err := checkContext(ctx); err != nil {
			return out, err
		}
		oldBucket := oldByHash[hash]
		newBucket := newByHash[hash]
		switch {
		case len(oldBucket) > len(newBucket):
			for _, record := range oldBucket[len(newBucket):] {
				if err := checkContext(ctx); err != nil {
					return out, err
				}
				out.Removed = append(out.Removed, RecordRef{Hash: hash, Record: record})
			}
		case len(newBucket) > len(oldBucket):
			for _, record := range newBucket[len(oldBucket):] {
				if err := checkContext(ctx); err != nil {
					return out, err
				}
				out.Added = append(out.Added, RecordRef{Hash: hash, Record: record})
			}
		}
	}
	sortRecordRefs(out.Added)
	sortRecordRefs(out.Removed)
	return out, nil
}

func hashedRecords(ctx context.Context, spec resources.ResourceSpec, records []map[string]any) (map[string][]map[string]any, error) {
	out := make(map[string][]map[string]any, len(records))
	for _, record := range records {
		if err := checkContext(ctx); err != nil {
			return nil, err
		}
		filtered, err := filterRecordContext(ctx, spec, record, true)
		if err != nil {
			return nil, err
		}
		hash, err := recordHash(filtered)
		if err != nil {
			return nil, err
		}
		if err := checkContext(ctx); err != nil {
			return nil, err
		}
		out[hash] = append(out[hash], filtered)
	}
	return out, nil
}

func recordHash(record map[string]any) (string, error) {
	canonical, err := canonicalHashValue(record)
	if err != nil {
		return "", fmt.Errorf("canonicalize record for content hash: %w", err)
	}
	body, err := json.Marshal(canonical)
	if err != nil {
		return "", fmt.Errorf("canonicalize record for content hash: %w", err)
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:]), nil
}

func filterRecordContext(ctx context.Context, spec resources.ResourceSpec, record map[string]any, ignoreOperational bool) (map[string]any, error) {
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	out := make(map[string]any, len(record))
	classes := fieldClasses(spec)
	for key, value := range record {
		if err := checkContext(ctx); err != nil {
			return nil, err
		}
		if ignoreOperational && classes[key] == resources.ClassOperational {
			continue
		}
		out[key] = canonicalValue(value)
	}
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	return out, nil
}

func fieldClasses(spec resources.ResourceSpec) map[string]resources.FieldClassification {
	out := make(map[string]resources.FieldClassification, len(spec.Fields))
	for _, field := range spec.Fields {
		out[field.JSONField()] = field.Classification
	}
	return out
}

func canonicalValue(value any) any {
	switch v := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, item := range v {
			out[key] = canonicalValue(item)
		}
		return out
	case []any:
		// Arrays are compared in order. Some fields encode precedence or ordered
		// membership, so reordering a list is intentionally reported as drift.
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = canonicalValue(item)
		}
		return out
	case float64:
		return normalizeFloat(v)
	case json.Number:
		return json.Number(v.String())
	default:
		return v
	}
}

func canonicalHashValue(value any) (any, error) {
	if lexeme, ok := numberLexeme(value); ok {
		return canonicalNumber(lexeme)
	}
	switch typed := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, item := range typed {
			canonical, err := canonicalHashValue(item)
			if err != nil {
				return nil, err
			}
			out[key] = canonical
		}
		return out, nil
	case []any:
		out := make([]any, len(typed))
		for i, item := range typed {
			canonical, err := canonicalHashValue(item)
			if err != nil {
				return nil, err
			}
			out[i] = canonical
		}
		return out, nil
	default:
		return typed, nil
	}
}

func numberLexeme(value any) (string, bool) {
	switch typed := value.(type) {
	case json.Number:
		return typed.String(), true
	case float64:
		if math.IsNaN(typed) || math.IsInf(typed, 0) {
			return "", false
		}
		return strconv.FormatFloat(typed, 'g', -1, 64), true
	case float32:
		if math.IsNaN(float64(typed)) || math.IsInf(float64(typed), 0) {
			return "", false
		}
		return strconv.FormatFloat(float64(typed), 'g', -1, 32), true
	case int:
		return strconv.FormatInt(int64(typed), 10), true
	case int8:
		return strconv.FormatInt(int64(typed), 10), true
	case int16:
		return strconv.FormatInt(int64(typed), 10), true
	case int32:
		return strconv.FormatInt(int64(typed), 10), true
	case int64:
		return strconv.FormatInt(typed, 10), true
	case uint:
		return strconv.FormatUint(uint64(typed), 10), true
	case uint8:
		return strconv.FormatUint(uint64(typed), 10), true
	case uint16:
		return strconv.FormatUint(uint64(typed), 10), true
	case uint32:
		return strconv.FormatUint(uint64(typed), 10), true
	case uint64:
		return strconv.FormatUint(typed, 10), true
	default:
		return "", false
	}
}

func canonicalNumber(lexeme string) (json.Number, error) {
	number := json.Number(lexeme)
	if _, err := number.Float64(); err != nil {
		return "", err
	}
	negative := strings.HasPrefix(lexeme, "-")
	if negative {
		lexeme = lexeme[1:]
	}
	exponent := new(big.Int)
	if index := strings.IndexAny(lexeme, "eE"); index >= 0 {
		if _, ok := exponent.SetString(lexeme[index+1:], 10); !ok {
			return "", fmt.Errorf("invalid number exponent")
		}
		lexeme = lexeme[:index]
	}
	fractionDigits := 0
	if index := strings.IndexByte(lexeme, '.'); index >= 0 {
		fractionDigits = len(lexeme) - index - 1
		lexeme = lexeme[:index] + lexeme[index+1:]
	}
	digits := strings.TrimLeft(lexeme, "0")
	if digits == "" {
		return json.Number("0"), nil
	}
	trailingZeros := len(digits) - len(strings.TrimRight(digits, "0"))
	if trailingZeros > 0 {
		digits = digits[:len(digits)-trailingZeros]
	}
	// The exponent can be numerically enormous even when its source lexeme is
	// short (for example 1e-1000000000). Keep exponent arithmetic proportional
	// to the input bytes and never expand it into exponent-sized storage.
	delta := int64(-fractionDigits + trailingZeros + len(digits) - 1)
	effectiveExponent := new(big.Int).Add(exponent, big.NewInt(delta))
	mantissa := digits[:1]
	if len(digits) > 1 {
		mantissa += "." + digits[1:]
	}
	if negative {
		mantissa = "-" + mantissa
	}
	return json.Number(mantissa + "e" + effectiveExponent.String()), nil
}

func normalizeFloat(value float64) any {
	if value == math.Trunc(value) && value >= math.MinInt64 && value <= math.MaxInt64 {
		return int64(value)
	}
	return value
}

func diffFieldsContext(ctx context.Context, oldRecord, newRecord map[string]any) ([]FieldChange, error) {
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	var fields []string
	seen := map[string]bool{}
	for key := range oldRecord {
		if err := checkContext(ctx); err != nil {
			return nil, err
		}
		seen[key] = true
		fields = append(fields, key)
	}
	for key := range newRecord {
		if err := checkContext(ctx); err != nil {
			return nil, err
		}
		if !seen[key] {
			fields = append(fields, key)
		}
	}
	sort.Strings(fields)
	changes := make([]FieldChange, 0)
	for _, field := range fields {
		if err := checkContext(ctx); err != nil {
			return nil, err
		}
		oldValue, oldOK := oldRecord[field]
		newValue, newOK := newRecord[field]
		if !oldOK {
			oldValue = nil
		}
		if !newOK {
			newValue = nil
		}
		if !jsonValuesEqual(oldValue, newValue) {
			changes = append(changes, FieldChange{Field: field, Old: oldValue, New: newValue})
		}
	}
	return changes, nil
}

func jsonValuesEqual(left, right any) bool {
	leftNumber, leftIsNumber := numberLexeme(left)
	rightNumber, rightIsNumber := numberLexeme(right)
	if leftIsNumber || rightIsNumber {
		if !leftIsNumber || !rightIsNumber {
			return false
		}
		leftCanonical, leftErr := canonicalNumber(leftNumber)
		rightCanonical, rightErr := canonicalNumber(rightNumber)
		return leftErr == nil && rightErr == nil && leftCanonical == rightCanonical
	}
	switch leftTyped := left.(type) {
	case map[string]any:
		rightTyped, ok := right.(map[string]any)
		if !ok || len(leftTyped) != len(rightTyped) {
			return false
		}
		for key, leftValue := range leftTyped {
			rightValue, present := rightTyped[key]
			if !present || !jsonValuesEqual(leftValue, rightValue) {
				return false
			}
		}
		return true
	case []any:
		rightTyped, ok := right.([]any)
		if !ok || len(leftTyped) != len(rightTyped) {
			return false
		}
		for i := range leftTyped {
			if !jsonValuesEqual(leftTyped[i], rightTyped[i]) {
				return false
			}
		}
		return true
	default:
		return reflect.DeepEqual(left, right)
	}
}

func sortedKeys(a, b map[string]map[string]any) []string {
	seen := map[string]bool{}
	keys := make([]string, 0, len(a)+len(b))
	for key := range a {
		seen[key] = true
		keys = append(keys, key)
	}
	for key := range b {
		if !seen[key] {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys
}

func sortedHashKeys(a, b map[string][]map[string]any) []string {
	seen := map[string]bool{}
	keys := make([]string, 0, len(a)+len(b))
	for key := range a {
		seen[key] = true
		keys = append(keys, key)
	}
	for key := range b {
		if !seen[key] {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys
}

func sortRecordRefs(refs []RecordRef) {
	sort.Slice(refs, func(i, j int) bool {
		if refs[i].Key != refs[j].Key {
			return refs[i].Key < refs[j].Key
		}
		return refs[i].Hash < refs[j].Hash
	})
}
