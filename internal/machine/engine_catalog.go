package machine

import (
	"context"
	"slices"

	"github.com/dvmrry/zscalerctl/internal/resources"
)

// CatalogRequest is the typed, config-free request for projected resource
// catalog discovery. It carries no runtime settings or generic options.
type CatalogRequest struct {
	RequestID string
}

// MarshalJSON rejects direct CatalogRequest serialization.
func (CatalogRequest) MarshalJSON() ([]byte, error) {
	return nil, errEngineTypeHasNoWireFormat
}

// UnmarshalJSON rejects direct CatalogRequest deserialization.
func (*CatalogRequest) UnmarshalJSON([]byte) error {
	return errEngineTypeHasNoWireFormat
}

// CatalogResult owns a defensive snapshot of the projected resource catalog.
type CatalogResult struct {
	catalog resources.ResourceCatalog
}

// NewCatalogResult constructs a typed catalog result from trusted static
// catalog metadata.
func NewCatalogResult(catalog resources.ResourceCatalog) CatalogResult {
	return CatalogResult{catalog: cloneEngineCatalog(catalog)}
}

// Catalog returns a deep defensive copy of the catalog snapshot.
func (r CatalogResult) Catalog() resources.ResourceCatalog {
	return cloneEngineCatalog(r.catalog)
}

// MarshalJSON rejects direct CatalogResult serialization.
func (CatalogResult) MarshalJSON() ([]byte, error) {
	return nil, errEngineTypeHasNoWireFormat
}

// UnmarshalJSON rejects direct CatalogResult deserialization.
func (*CatalogResult) UnmarshalJSON([]byte) error {
	return errEngineTypeHasNoWireFormat
}

// DiscoverCatalog returns the executor's static projected catalog without
// loading config, resolving credentials, constructing an SDK client, executing
// a provider, or contacting a tenant.
func (e Executor) DiscoverCatalog(ctx context.Context, _ CatalogRequest) (CatalogResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		machineErr := machineErrorFromLoadError(err, OperationList, "", "")
		return CatalogResult{}, &machineErr
	}
	catalog := e.catalog()
	if err := resources.AssertReadOnly(catalog...); err != nil {
		return CatalogResult{}, &MachineError{
			Kind:      ErrorKindUnsupportedOperation,
			Message:   "catalog contains a mutating operation",
			Operation: OperationList,
			cause:     err,
		}
	}
	return NewCatalogResult(catalog), nil
}

func cloneEngineCatalog(catalog resources.ResourceCatalog) resources.ResourceCatalog {
	if catalog == nil {
		return resources.ResourceCatalog{}
	}
	out := make(resources.ResourceCatalog, len(catalog))
	for i, spec := range catalog {
		spec.Operations = slices.Clone(spec.Operations)
		spec.Fields = cloneEngineFields(spec.Fields)
		out[i] = spec
	}
	return out
}

func cloneEngineFields(fields []resources.FieldSpec) []resources.FieldSpec {
	if fields == nil {
		return nil
	}
	out := make([]resources.FieldSpec, len(fields))
	for i, field := range fields {
		field.AllowedModes = slices.Clone(field.AllowedModes)
		field.Fields = cloneEngineFields(field.Fields)
		out[i] = field
	}
	return out
}
