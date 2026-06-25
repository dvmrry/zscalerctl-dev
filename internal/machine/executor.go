package machine

import (
	"context"
	"fmt"
	"strings"

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

	// ErrorKindLiveAccessFailed reports a sanitized resource-loading failure.
	ErrorKindLiveAccessFailed = "live_access_failed"

	// ErrorKindInternal reports executor wiring errors.
	ErrorKindInternal = "internal"
)

// BrowserLoader is the projected-record loading surface required by Executor.
// Implementations own catalog lookup, live reads, projection, and redaction.
type BrowserLoader interface {
	LoadProjected(ctx context.Context, product, resource string) (resources.ProjectedRecords, error)
}

// Executor executes supported machine requests through projected-record
// loaders without owning CLI routing, config loading, SDK clients, or rendering.
type Executor struct {
	Browser BrowserLoader
}

// Execute validates and runs one supported read-only machine request.
func (e Executor) Execute(ctx context.Context, req Request) (Response, error) {
	resp := responseForRequest(req)
	product, resource := inputResource(req)
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

	product, resource, missing := requiredInputResource(req)
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

	projected, err := e.Browser.LoadProjected(ctx, product, resource)
	if err != nil {
		return errorResponse(resp, MachineError{
			Kind:      ErrorKindLiveAccessFailed,
			Message:   "resource read failed",
			Operation: req.Operation,
			Product:   product,
			Resource:  resource,
		})
	}
	resp.Records = projectedRecordsToMaps(projected)
	resp.Meta.Count = len(resp.Records)
	return resp, nil
}

func isSupportedReadOperation(op Operation) bool {
	return op == OperationList || op == OperationShow
}

func requiredInputResource(req Request) (string, string, []string) {
	if req.Input == nil {
		return "", "", []string{"input"}
	}
	product, resource := inputResource(req)
	var missing []string
	if product == "" {
		missing = append(missing, "input.product")
	}
	if resource == "" {
		missing = append(missing, "input.resource")
	}
	return product, resource, missing
}

func inputResource(req Request) (string, string) {
	if req.Input == nil {
		return "", ""
	}
	return strings.TrimSpace(req.Input.Product), strings.TrimSpace(req.Input.Resource)
}

func responseForRequest(req Request) Response {
	meta := copyMeta(req.Meta)
	if meta == nil {
		meta = &Meta{}
	}
	meta.RequestID = req.RequestID
	meta.ReadOnly = true
	meta.Count = 0
	meta.Product, meta.Resource = inputResource(req)
	return Response{
		RequestID:  req.RequestID,
		Capability: req.Capability,
		Operation:  req.Operation,
		Meta:       meta,
	}
}

func copyMeta(in *Meta) *Meta {
	if in == nil {
		return nil
	}
	out := *in
	return &out
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
