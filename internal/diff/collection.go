package diff

import (
	"context"
	"fmt"
	"strings"

	"github.com/dvmrry/zscalerctl/internal/dump"
	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

// CollectionProvenance contains the safe metadata retained by a loaded
// collection. It deliberately excludes manifest-authored free-form strings.
type CollectionProvenance struct {
	ManifestSchema string `json:"manifest_schema"`
	Redaction      string `json:"redaction"`
	Status         string `json:"status"`
	ResourceCount  int    `json:"resource_count"`
	RecordCount    int    `json:"record_count"`
}

// Collection is an immutable, in-memory view of one complete saved dump.
// Records are admitted using the dump's stored redaction mode and remain
// stable after loading, even if the source directory changes.
type Collection struct {
	mode       redact.Mode
	provenance CollectionProvenance
	specs      map[ResourceKey]resources.ResourceSpec
	resources  map[ResourceKey]resources.ProjectedRecords
}

// LoadCollection validates and loads one complete saved dump into memory.
// A nil catalog selects the built-in catalog. The catalog is copied during
// admission, and a loaded collection never reads configuration, credentials,
// providers, SDK clients, or the network.
func LoadCollection(
	ctx context.Context,
	dir string,
	catalog resources.ResourceCatalog,
) (*Collection, error) {
	ctx = normalizedContext(ctx)
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	if catalog == nil {
		catalog = resources.Catalog()
	}
	catalog = cloneResourceCatalog(catalog)
	catalogSpecs, err := validateCatalog(catalog)
	if err != nil {
		return nil, err
	}
	selected := make(map[ResourceKey]bool, len(catalogSpecs))
	for key, spec := range catalogSpecs {
		if spec.SupportsReadOperation("list") || spec.SupportsReadOperation("show") || spec.SupportsReadOperation("get") {
			selected[key] = true
		}
	}
	loaded, err := loadDumpWithBudgetOptions(ctx, dir, catalogSpecs, selected, maxCollectionBytes, true)
	if err != nil {
		return nil, err
	}
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	if loaded.ref.Partial || loaded.manifest.Status != "complete" {
		return nil, fmt.Errorf("%w: collection artifact is partial", ErrPartialDumpInput)
	}
	mode, err := redact.ParseMode(loaded.manifest.Redaction)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid redaction mode", ErrInvalidDump)
	}

	collection := &Collection{
		mode: redact.EffectiveMode(mode),
		provenance: CollectionProvenance{
			ManifestSchema: dump.ManifestSchemaID,
			Redaction:      loaded.manifest.Redaction,
			Status:         "complete",
		},
		specs:     make(map[ResourceKey]resources.ResourceSpec, len(catalogSpecs)),
		resources: make(map[ResourceKey]resources.ProjectedRecords, len(loaded.resources)),
	}
	for key, spec := range catalogSpecs {
		collection.specs[key] = cloneResourceSpec(spec)
	}
	for key, loadedResource := range loaded.resources {
		if err := checkContext(ctx); err != nil {
			return nil, err
		}
		spec, ok := collection.specs[key]
		if !ok {
			return nil, fmt.Errorf("%w: loaded resource is not in the catalog", ErrInvalidDump)
		}
		if err := validateLoadedRecords(spec, collection.mode, loadedResource.records); err != nil {
			return nil, err
		}
		projected, err := resources.NewVerifiedProjectedRecordsFromProjectedFields(
			spec,
			collection.mode,
			loadedResource.records,
		)
		if err != nil {
			return nil, invalidAdmissionError(spec, false)
		}
		collection.resources[key] = projected
		collection.provenance.ResourceCount++
		collection.provenance.RecordCount += projected.Len()
	}
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	return collection, nil
}

// Redaction reports the redaction mode used to admit this collection.
func (c *Collection) Redaction() redact.Mode {
	if c == nil {
		return redact.ModeStandard
	}
	return c.mode
}

// Provenance returns safe metadata describing this collection.
func (c *Collection) Provenance() CollectionProvenance {
	if c == nil {
		return CollectionProvenance{}
	}
	return c.provenance
}

// ListProjected returns the admitted records for a list-backed resource in
// their original stable order.
func (c *Collection) ListProjected(
	ctx context.Context,
	product string,
	resource string,
) (resources.ProjectedRecords, error) {
	spec, records, err := c.lookup(ctx, product, resource, "list")
	if err != nil {
		return resources.ProjectedRecords{}, err
	}
	if spec.EffectiveShape() == resources.ShapeSingleton && records.Len() > 1 {
		return resources.ProjectedRecords{}, fmt.Errorf("%w: singleton resource has multiple records", ErrInvalidDump)
	}
	return records, nil
}

// ShowProjected returns the admitted record for a show-backed resource in its
// original stable order.
func (c *Collection) ShowProjected(
	ctx context.Context,
	product string,
	resource string,
) (resources.ProjectedRecords, error) {
	_, records, err := c.lookup(ctx, product, resource, "show")
	if err != nil {
		return resources.ProjectedRecords{}, err
	}
	return records, nil
}

// GetProjectedByID returns one admitted record selected by the catalog's get
// key. It never performs a live read and reports an absent ID explicitly.
func (c *Collection) GetProjectedByID(
	ctx context.Context,
	product string,
	resource string,
	id string,
) (resources.ProjectedRecords, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return resources.ProjectedRecords{}, resources.ErrMissingID
	}
	spec, records, err := c.lookup(ctx, product, resource, "get")
	if err != nil {
		return resources.ProjectedRecords{}, err
	}
	keyField := spec.EffectiveGetKey()
	if !isRenderableField(spec, c.mode, keyField) {
		return resources.ProjectedRecords{}, resources.ErrUnsupportedLoad
	}
	for _, record := range records.Records() {
		if err := checkContext(ctx); err != nil {
			return resources.ProjectedRecords{}, err
		}
		value, ok := record.Value(keyField)
		if !ok || value == nil {
			continue
		}
		if identityString(value) == id {
			return resources.NewProjectedRecords([]resources.ProjectedRecord{record}), nil
		}
	}
	return resources.ProjectedRecords{}, resources.ErrRecordNotFound
}

func (c *Collection) lookup(
	ctx context.Context,
	product string,
	resource string,
	operation string,
) (resources.ResourceSpec, resources.ProjectedRecords, error) {
	ctx = normalizedContext(ctx)
	if err := checkContext(ctx); err != nil {
		return resources.ResourceSpec{}, resources.ProjectedRecords{}, err
	}
	if c == nil {
		return resources.ResourceSpec{}, resources.ProjectedRecords{}, ErrInvalidDump
	}
	key := ResourceKey{Product: resources.Product(strings.TrimSpace(product)), Name: strings.TrimSpace(resource)}
	spec, ok := c.specs[key]
	if !ok {
		return resources.ResourceSpec{}, resources.ProjectedRecords{}, resources.ErrUnknownResource
	}
	if !spec.SupportsReadOperation(operation) {
		return resources.ResourceSpec{}, resources.ProjectedRecords{}, resources.ErrUnsupportedLoad
	}
	records, ok := c.resources[key]
	if !ok {
		return resources.ResourceSpec{}, resources.ProjectedRecords{}, ErrCollectionScopeMismatch
	}
	if err := checkContext(ctx); err != nil {
		return resources.ResourceSpec{}, resources.ProjectedRecords{}, err
	}
	return spec, records, nil
}

func validateLoadedRecords(spec resources.ResourceSpec, mode redact.Mode, records []map[string]any) error {
	if isSingletonSpec(spec) && len(records) > 1 {
		return fmt.Errorf("%w: singleton resource has multiple records", ErrInvalidDump)
	}
	keyField := spec.EffectiveGetKey()
	if keyField == "" || !isRenderableField(spec, mode, keyField) {
		return nil
	}
	seen := make(map[string]struct{}, len(records))
	for _, record := range records {
		value, ok := record[keyField]
		if !ok || value == nil {
			continue
		}
		key := identityString(value)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			return fmt.Errorf("%w: resource has duplicate identity", ErrInvalidDump)
		}
		seen[key] = struct{}{}
	}
	return nil
}

func isRenderableField(spec resources.ResourceSpec, mode redact.Mode, field string) bool {
	if field == "" {
		return false
	}
	for _, rendered := range spec.FieldOrder(mode) {
		if rendered == field {
			return true
		}
	}
	return false
}

func isSingletonSpec(spec resources.ResourceSpec) bool {
	return spec.EffectiveShape() == resources.ShapeSingleton ||
		(!spec.SupportsReadOperation("list") && spec.SupportsReadOperation("show"))
}

func cloneResourceCatalog(catalog resources.ResourceCatalog) resources.ResourceCatalog {
	out := make(resources.ResourceCatalog, len(catalog))
	for i, spec := range catalog {
		out[i] = cloneResourceSpec(spec)
	}
	return out
}

func cloneResourceSpec(spec resources.ResourceSpec) resources.ResourceSpec {
	out := spec
	out.Operations = append([]resources.Operation(nil), spec.Operations...)
	out.Fields = cloneFieldSpecs(spec.Fields)
	return out
}

func cloneFieldSpecs(fields []resources.FieldSpec) []resources.FieldSpec {
	if fields == nil {
		return nil
	}
	out := make([]resources.FieldSpec, len(fields))
	for i, field := range fields {
		out[i] = field
		out[i].AllowedModes = append([]redact.Mode(nil), field.AllowedModes...)
		out[i].Fields = cloneFieldSpecs(field.Fields)
	}
	return out
}
