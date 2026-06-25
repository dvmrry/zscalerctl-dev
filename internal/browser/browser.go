// Package browser exposes a UI-agnostic resource browsing service.
package browser

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

var (
	// ErrMissingReader reports that a Service cannot load records because no
	// reader was supplied.
	ErrMissingReader = errors.New("browser service reader is required")

	// ErrUnknownResource reports that the requested product/resource is not in
	// the service catalog.
	ErrUnknownResource = errors.New("unknown browser resource")

	// ErrUnsupportedLoad reports that a catalog entry has no list or show
	// operation suitable for browser loading.
	ErrUnsupportedLoad = errors.New("unsupported browser load operation")
)

// UnknownResourceError describes a product/resource lookup miss.
type UnknownResourceError struct {
	Product  string
	Resource string
}

func (e UnknownResourceError) Error() string {
	return fmt.Sprintf("%s: %s/%s", ErrUnknownResource, e.Product, e.Resource)
}

// Unwrap returns the sentinel error for errors.Is checks.
func (e UnknownResourceError) Unwrap() error { return ErrUnknownResource }

// RecordReader is the backend reader surface required for resource browsing.
type RecordReader interface {
	List(context.Context, resources.Product, string) ([]resources.SourceRecord, error)
	Show(context.Context, resources.Product, string) (resources.SourceRecord, error)
}

// Filter narrows catalog resources returned by Service.Resources.
type Filter struct {
	Products  []resources.Product
	Resources []string
}

// ResourceInfo describes a catalog resource without exposing CLI command
// plumbing.
type ResourceInfo struct {
	Product    string
	Name       string
	Label      string
	Operations []string
}

// Field is one projected and redacted record field.
type Field struct {
	Key   string
	Value any
}

// Record is a projected and redacted resource record with summary fields for
// browser-style lists.
type Record struct {
	ID     string
	Name   string
	Status string
	Fields []Field
}

// Service loads catalog metadata and projected records for UI-agnostic
// resource browsing.
type Service struct {
	Catalog resources.ResourceCatalog
	Reader  RecordReader
	Mode    redact.Mode
}

// Resources returns catalog resources matching filter in deterministic
// product/name order.
func (s Service) Resources(filter Filter) []ResourceInfo {
	productSet := productFilterSet(filter.Products)
	resourceSet := stringFilterSet(filter.Resources)

	out := make([]ResourceInfo, 0)
	for _, spec := range s.catalog() {
		if len(productSet) > 0 && !productSet[spec.Product] {
			continue
		}
		if len(resourceSet) > 0 && !resourceSet[spec.Name] {
			continue
		}
		out = append(out, resourceInfo(spec))
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Product != out[j].Product {
			return out[i].Product < out[j].Product
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// Load returns projected and redacted records for one resource. List-backed
// resources call Reader.List; show-backed resources call Reader.Show and return
// a single record.
func (s Service) Load(ctx context.Context, product, resource string) ([]Record, error) {
	spec, projected, err := s.loadProjected(ctx, product, resource)
	if err != nil {
		return nil, err
	}
	return projectedRecords(spec, redact.EffectiveMode(s.Mode), projected), nil
}

// LoadProjected returns projected and redacted records for one resource without
// choosing a presentation shape. It is intended for non-UI callers that already
// render resources.ProjectedRecords through their own output contract.
func (s Service) LoadProjected(ctx context.Context, product, resource string) (resources.ProjectedRecords, error) {
	_, projected, err := s.loadProjected(ctx, product, resource)
	if err != nil {
		return resources.ProjectedRecords{}, err
	}
	return projected, nil
}

func (s Service) loadProjected(
	ctx context.Context,
	product string,
	resource string,
) (resources.ResourceSpec, resources.ProjectedRecords, error) {
	if s.Reader == nil {
		return resources.ResourceSpec{}, resources.ProjectedRecords{}, ErrMissingReader
	}
	spec, ok := s.catalog().FindSpec(resources.Product(product), resource)
	if !ok {
		return resources.ResourceSpec{}, resources.ProjectedRecords{}, UnknownResourceError{
			Product:  product,
			Resource: resource,
		}
	}
	if err := resources.AssertReadOnly(spec); err != nil {
		return resources.ResourceSpec{}, resources.ProjectedRecords{}, err
	}
	mode := redact.EffectiveMode(s.Mode)
	switch {
	case spec.SupportsReadOperation("show"):
		record, err := s.Reader.Show(ctx, spec.Product, spec.Name)
		if err != nil {
			return resources.ResourceSpec{}, resources.ProjectedRecords{}, err
		}
		projected, _, err := resources.ProjectRecordAndVerify(spec, mode, record)
		if err != nil {
			return resources.ResourceSpec{}, resources.ProjectedRecords{}, err
		}
		return spec, resources.NewProjectedRecords([]resources.ProjectedRecord{projected}), nil
	case spec.SupportsReadOperation("list"):
		records, err := s.Reader.List(ctx, spec.Product, spec.Name)
		if err != nil {
			return resources.ResourceSpec{}, resources.ProjectedRecords{}, err
		}
		projected, _, err := resources.ProjectRecordsAndVerify(spec, mode, records)
		if err != nil {
			return resources.ResourceSpec{}, resources.ProjectedRecords{}, err
		}
		return spec, projected, nil
	default:
		return resources.ResourceSpec{}, resources.ProjectedRecords{},
			fmt.Errorf("%w: %s/%s", ErrUnsupportedLoad, spec.Product, spec.Name)
	}
}

func (s Service) catalog() resources.ResourceCatalog {
	if s.Catalog == nil {
		return resources.Catalog()
	}
	out := make(resources.ResourceCatalog, len(s.Catalog))
	copy(out, s.Catalog)
	return out
}

func resourceInfo(spec resources.ResourceSpec) ResourceInfo {
	return ResourceInfo{
		Product:    string(spec.Product),
		Name:       spec.Name,
		Label:      spec.Name,
		Operations: readOperationNames(spec),
	}
}

func readOperationNames(spec resources.ResourceSpec) []string {
	out := make([]string, 0, len(spec.Operations))
	for _, op := range spec.Operations {
		if op.Capability == resources.CapabilityRead {
			out = append(out, op.Name)
		}
	}
	return out
}

func projectedRecords(
	spec resources.ResourceSpec,
	mode redact.Mode,
	records resources.ProjectedRecords,
) []Record {
	projected := records.Records()
	out := make([]Record, len(projected))
	for i, record := range projected {
		out[i] = projectedRecord(spec, mode, record)
	}
	return out
}

func projectedRecord(
	spec resources.ResourceSpec,
	mode redact.Mode,
	record resources.ProjectedRecord,
) Record {
	values := record.Fields()
	return Record{
		ID:     summaryValue(values, spec.EffectiveGetKey(), "id"),
		Name:   summaryValue(values, "name", "displayName", "configuredName"),
		Status: summaryValue(values, "status", "state"),
		Fields: projectedFields(spec, mode, values),
	}
}

func projectedFields(
	spec resources.ResourceSpec,
	mode redact.Mode,
	values map[string]any,
) []Field {
	order := spec.FieldOrder(mode)
	out := make([]Field, 0, len(values))
	for _, key := range order {
		value, ok := values[key]
		if !ok {
			continue
		}
		out = append(out, Field{Key: key, Value: value})
	}
	return out
}

func summaryValue(values map[string]any, keys ...string) string {
	for _, key := range keys {
		if key == "" {
			continue
		}
		value, ok := values[key]
		if !ok {
			continue
		}
		switch v := value.(type) {
		case string:
			return v
		case fmt.Stringer:
			return v.String()
		default:
			return fmt.Sprint(v)
		}
	}
	return ""
}

func productFilterSet(products []resources.Product) map[resources.Product]bool {
	if len(products) == 0 {
		return nil
	}
	out := make(map[resources.Product]bool, len(products))
	for _, product := range products {
		if product == "" {
			continue
		}
		out[product] = true
	}
	return out
}

func stringFilterSet(values []string) map[string]bool {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string]bool, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		out[value] = true
	}
	return out
}
