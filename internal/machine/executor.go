package machine

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

const (
	// ErrorKindUsage reports an invalid machine request shape.
	ErrorKindUsage = "usage"

	// ErrorKindUnsupportedCapability reports a capability this executor does
	// not implement.
	ErrorKindUnsupportedCapability = "unsupported_capability"

	// ErrorKindUnsupportedOperation reports an operation this executor does not
	// implement for a supported capability.
	ErrorKindUnsupportedOperation = "unsupported_operation"

	// ErrorKindUnknownResource reports a product/resource that is not in the
	// projected resource catalog.
	ErrorKindUnknownResource = "unknown_resource"

	// ErrorKindLiveAccessFailed reports a sanitized resource-loading failure.
	ErrorKindLiveAccessFailed = "live_access_failed"

	// ErrorKindCanceled reports a request canceled by context.
	ErrorKindCanceled = "canceled"

	// ErrorKindDeadlineExceeded reports a request that exceeded its deadline.
	ErrorKindDeadlineExceeded = "deadline_exceeded"

	// ErrorKindInternal reports executor wiring errors.
	ErrorKindInternal = "internal"
)

// BrowserLoader is the projected-record loading surface required by Executor.
// Implementations own catalog lookup, live reads, projection, and redaction.
type BrowserLoader interface {
	ListProjected(ctx context.Context, product, resource string) (resources.ProjectedRecords, error)
	ShowProjected(ctx context.Context, product, resource string) (resources.ProjectedRecords, error)
}

// ProjectedRecordGetter is the projected-record loading surface for ID-backed
// reads. Implementations own catalog lookup, live reads, projection, and
// redaction.
type ProjectedRecordGetter interface {
	GetProjectedByID(ctx context.Context, product, resource, id string) (resources.ProjectedRecords, error)
}

// Executor executes supported machine requests through projected-record
// loaders without owning CLI routing, config loading, SDK clients, or rendering.
type Executor struct {
	Browser   BrowserLoader
	Catalog   resources.ResourceCatalog
	Redaction redact.Mode
}

// Execute validates and runs one supported read-only machine request.
func (e Executor) Execute(ctx context.Context, req Request) (Response, error) {
	resp := responseForRequest(req)
	product, resource := inputResource(req)
	if machineErr := validateRequestSemantics(req, product, resource); machineErr != nil {
		return errorResponse(resp, *machineErr)
	}
	if req.Operation == OperationManifest {
		if req.Capability != "" && req.Capability != CapabilityResourcesRead {
			return errorResponse(resp, MachineError{
				Kind:      ErrorKindUnsupportedCapability,
				Message:   fmt.Sprintf("unsupported capability %q", req.Capability),
				Operation: req.Operation,
				Product:   product,
				Resource:  resource,
			})
		}
		manifest := ManifestFromCatalog(e.catalog())
		resp.Manifest = &manifest
		resp.Meta.Count = len(manifest.Capabilities)
		return resp, nil
	}
	if req.Capability != CapabilityResourcesRead {
		return errorResponse(resp, MachineError{
			Kind:      ErrorKindUnsupportedCapability,
			Message:   fmt.Sprintf("unsupported capability %q", req.Capability),
			Operation: req.Operation,
			Product:   product,
			Resource:  resource,
		})
	}
	if !isSupportedReadOperation(req.Operation) {
		return errorResponse(resp, MachineError{
			Kind:      ErrorKindUnsupportedOperation,
			Message:   fmt.Sprintf("unsupported operation %q for %s", req.Operation, CapabilityResourcesRead),
			Operation: req.Operation,
			Product:   product,
			Resource:  resource,
		})
	}

	product, resource, recordID, missing := requiredInput(req)
	if len(missing) > 0 {
		return errorResponse(resp, MachineError{
			Kind:      ErrorKindUsage,
			Message:   "missing required input: " + strings.Join(missing, ", "),
			Missing:   missing,
			Operation: req.Operation,
			Product:   product,
			Resource:  resource,
		})
	}
	resp.Meta.Product = product
	resp.Meta.Resource = resource

	if e.Browser == nil {
		return errorResponse(resp, MachineError{
			Kind:      ErrorKindInternal,
			Message:   "browser loader is not configured",
			Operation: req.Operation,
			Product:   product,
			Resource:  resource,
		})
	}

	projected, err := e.loadProjected(ctx, req.Operation, product, resource, recordID)
	if err != nil {
		return errorResponse(resp, machineErrorFromLoadError(err, req.Operation, product, resource))
	}
	projected, err = e.narrowProjected(req, product, resource, projected)
	if err != nil {
		return errorResponse(resp, machineErrorFromLoadError(err, req.Operation, product, resource))
	}
	resp.Records = projectedRecordsToMaps(projected)
	resp.Meta.Count = len(resp.Records)
	return resp, nil
}

func isSupportedReadOperation(op Operation) bool {
	return op == OperationList || op == OperationGet || op == OperationShow
}

func requiredInput(req Request) (string, string, string, []string) {
	if req.Input == nil {
		return "", "", "", []string{"input"}
	}
	product, resource := inputResource(req)
	recordID := inputRecordID(req)
	var missing []string
	if product == "" {
		missing = append(missing, "input.product")
	}
	if resource == "" {
		missing = append(missing, "input.resource")
	}
	if req.Operation == OperationGet && recordID == "" {
		missing = append(missing, "input.record_id")
	}
	return product, resource, recordID, missing
}

func inputResource(req Request) (string, string) {
	if req.Input == nil {
		return "", ""
	}
	return strings.TrimSpace(req.Input.Product), strings.TrimSpace(req.Input.Resource)
}

func inputRecordID(req Request) string {
	if req.Input == nil {
		return ""
	}
	return strings.TrimSpace(req.Input.RecordID)
}

func (e Executor) loadProjected(
	ctx context.Context,
	op Operation,
	product string,
	resource string,
	recordID string,
) (resources.ProjectedRecords, error) {
	if op != OperationGet {
		if op == OperationShow {
			return e.Browser.ShowProjected(ctx, product, resource)
		}
		return e.Browser.ListProjected(ctx, product, resource)
	}
	getter, ok := e.Browser.(ProjectedRecordGetter)
	if !ok {
		return resources.ProjectedRecords{}, &MachineError{
			Kind:      ErrorKindInternal,
			Message:   "projected record getter is not configured",
			Operation: op,
			Product:   product,
			Resource:  resource,
		}
	}
	return getter.GetProjectedByID(ctx, product, resource, recordID)
}

func responseForRequest(req Request) Response {
	product, resource := inputResource(req)
	meta := &Meta{
		RequestID: req.RequestID,
		Product:   product,
		Resource:  resource,
		ReadOnly:  true,
		Count:     0,
	}
	return Response{
		RequestID:  req.RequestID,
		Capability: req.Capability,
		Operation:  req.Operation,
		Meta:       meta,
	}
}

func errorResponse(resp Response, machineErr MachineError) (Response, error) {
	resp.Error = &machineErr
	return resp, &machineErr
}

func projectedRecordsToMaps(records resources.ProjectedRecords) []map[string]any {
	projected := records.Records()
	out := make([]map[string]any, len(projected))
	for i, record := range projected {
		out[i] = record.Fields()
	}
	return out
}

func validateRequestSemantics(req Request, product, resource string) *MachineError {
	if req.Input == nil {
		return nil
	}
	if len(req.Input.Options) > 0 {
		return &MachineError{
			Kind:      ErrorKindUsage,
			Message:   "input.options is not supported",
			Operation: req.Operation,
			Product:   product,
			Resource:  resource,
		}
	}
	if len(req.Input.Fields) > 0 && !isSupportedReadOperation(req.Operation) {
		return &MachineError{
			Kind:      ErrorKindUsage,
			Message:   "input.fields applies to resource read operations only",
			Operation: req.Operation,
			Product:   product,
			Resource:  resource,
		}
	}
	if len(req.Input.Filters) > 0 && req.Operation != OperationList {
		return &MachineError{
			Kind:      ErrorKindUsage,
			Message:   "input.filters applies to list operation only",
			Operation: req.Operation,
			Product:   product,
			Resource:  resource,
		}
	}
	if req.Input.Search != "" && req.Operation != OperationList {
		return &MachineError{
			Kind:      ErrorKindUsage,
			Message:   "input.search applies to list operation only",
			Operation: req.Operation,
			Product:   product,
			Resource:  resource,
		}
	}
	if _, err := filtersFromInput(req.Input.Filters); err != nil {
		return &MachineError{
			Kind:      ErrorKindUsage,
			Message:   err.Error(),
			Operation: req.Operation,
			Product:   product,
			Resource:  resource,
		}
	}
	return nil
}

func (e Executor) narrowProjected(
	req Request,
	product string,
	resource string,
	projected resources.ProjectedRecords,
) (resources.ProjectedRecords, error) {
	if req.Input == nil {
		return projected, nil
	}
	if len(req.Input.Fields) == 0 && len(req.Input.Filters) == 0 && req.Input.Search == "" {
		return projected, nil
	}
	spec, ok := e.catalog().FindSpec(resources.Product(product), resource)
	if !ok {
		return resources.ProjectedRecords{},
			fmt.Errorf("%w: %s/%s", resources.ErrUnknownResource, product, resource)
	}
	filters, err := filtersFromInput(req.Input.Filters)
	if err != nil {
		return resources.ProjectedRecords{}, err
	}
	return resources.NarrowProjectedRecords(spec, e.Redaction, projected, resources.NarrowOptions{
		Fields:  fieldsFromInput(req.Input.Fields),
		Filters: filters,
		Search:  req.Input.Search,
	})
}

func fieldsFromInput(fields []string) []string {
	out := make([]string, 0, len(fields))
	for _, field := range fields {
		if trimmed := strings.TrimSpace(field); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func filtersFromInput(filters []Filter) ([]resources.ProjectedFilter, error) {
	out := make([]resources.ProjectedFilter, 0, len(filters))
	for _, filter := range filters {
		field := strings.TrimSpace(filter.Field)
		if field == "" {
			return nil, errors.New("input.filters.field is required")
		}
		projectedFilter := resources.ProjectedFilter{
			Field: field,
			Value: filter.Value,
		}
		switch strings.TrimSpace(filter.Operator) {
		case "=", "exact":
			projectedFilter.Substring = false
		case "~", "contains":
			projectedFilter.Substring = true
		default:
			return nil, fmt.Errorf("input.filters.operator %q is not supported", filter.Operator)
		}
		out = append(out, projectedFilter)
	}
	return out, nil
}

func (e Executor) catalog() resources.ResourceCatalog {
	if e.Catalog == nil {
		return resources.Catalog()
	}
	out := make(resources.ResourceCatalog, len(e.Catalog))
	copy(out, e.Catalog)
	return out
}

func machineErrorFromLoadError(err error, op Operation, product, resource string) MachineError {
	var machineErr *MachineError
	if errors.As(err, &machineErr) {
		return *machineErr
	}
	switch {
	case errors.Is(err, context.Canceled):
		return MachineError{
			Kind:      ErrorKindCanceled,
			Message:   "request canceled",
			Operation: op,
			Product:   product,
			Resource:  resource,
		}
	case errors.Is(err, context.DeadlineExceeded):
		return MachineError{
			Kind:      ErrorKindDeadlineExceeded,
			Message:   "request deadline exceeded",
			Operation: op,
			Product:   product,
			Resource:  resource,
		}
	case errors.Is(err, resources.ErrUnknownResource):
		return MachineError{
			Kind:      ErrorKindUnknownResource,
			Message:   "unknown resource",
			Operation: op,
			Product:   product,
			Resource:  resource,
		}
	case errors.Is(err, resources.ErrMissingID), errors.Is(err, resources.ErrUnknownField):
		return MachineError{
			Kind:      ErrorKindUsage,
			Message:   err.Error(),
			Operation: op,
			Product:   product,
			Resource:  resource,
		}
	case errors.Is(err, resources.ErrUnsupportedLoad), errors.Is(err, resources.ErrMutatingOperation):
		return MachineError{
			Kind:      ErrorKindUnsupportedOperation,
			Message:   "unsupported resource read operation",
			Operation: op,
			Product:   product,
			Resource:  resource,
		}
	default:
		return MachineError{
			Kind:      ErrorKindLiveAccessFailed,
			Message:   "resource read failed",
			Operation: op,
			Product:   product,
			Resource:  resource,
		}
	}
}
