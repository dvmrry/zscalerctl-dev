// Package adapter explicitly converts between transport-owned enginewire DTOs
// and candidate in-process machine types. It is the only place where those two
// contracts are allowed to know about each other.
package adapter

import (
	"strconv"

	"github.com/dvmrry/zscalerctl/internal/enginewire"
	"github.com/dvmrry/zscalerctl/internal/machine"
)

// RequestID converts one positive wire ID to the engine's opaque request ID.
func RequestID(id enginewire.SafeInteger) string {
	return strconv.FormatUint(id.Uint64(), 10)
}

// ToCatalogRequest converts a validated catalog request defensively.
func ToCatalogRequest(frame enginewire.CatalogRequest) machine.CatalogRequest {
	return machine.CatalogRequest{RequestID: RequestID(frame.ID)}
}

// ToStatusRequest converts a validated status operation defensively.
func ToStatusRequest(id enginewire.SafeInteger, operation enginewire.Operation) machine.StatusRequest {
	return machine.StatusRequest{
		RequestID: RequestID(id),
		Operation: machine.Operation(operation),
	}
}

// ToURLLookupRequest converts and copies a validated URL batch.
func ToURLLookupRequest(frame enginewire.URLLookupRequest) machine.URLLookupRequest {
	return machine.URLLookupRequest{
		RequestID: RequestID(frame.ID),
		URLs:      append([]string{}, frame.Input.URLs...),
	}
}

// ToResourceListRequest converts the exact list DTO to a typed engine request.
func ToResourceListRequest(frame enginewire.ResourceListRequest) machine.ResourceReadRequest {
	return machine.ResourceReadRequest{
		RequestID: RequestID(frame.ID),
		Operation: machine.OperationList,
		Input: machine.ResourceReadInput{
			Product:  string(frame.Input.Product),
			Resource: frame.Input.Resource,
			Fields:   append([]string{}, frame.Input.Fields...),
			Filters:  toMachineFilters(frame.Input.Filters),
			Search:   frame.Input.Search,
		},
	}
}

// ToResourceGetRequest converts the exact get DTO to a typed engine request.
func ToResourceGetRequest(frame enginewire.ResourceGetRequest) machine.ResourceReadRequest {
	return machine.ResourceReadRequest{
		RequestID: RequestID(frame.ID),
		Operation: machine.OperationGet,
		Input: machine.ResourceReadInput{
			Product:  string(frame.Input.Product),
			Resource: frame.Input.Resource,
			RecordID: frame.Input.RecordID,
			Fields:   append([]string{}, frame.Input.Fields...),
		},
	}
}

// ToResourceShowRequest converts the exact show DTO to a typed engine request.
func ToResourceShowRequest(frame enginewire.ResourceShowRequest) machine.ResourceReadRequest {
	return machine.ResourceReadRequest{
		RequestID: RequestID(frame.ID),
		Operation: machine.OperationShow,
		Input: machine.ResourceReadInput{
			Product:  string(frame.Input.Product),
			Resource: frame.Input.Resource,
			Fields:   append([]string{}, frame.Input.Fields...),
		},
	}
}

func toMachineFilters(filters []enginewire.Filter) []machine.Filter {
	out := make([]machine.Filter, len(filters))
	for i, filter := range filters {
		out[i] = machine.Filter{
			Field:    filter.Field,
			Operator: string(filter.Operator),
			Value:    filter.Value,
		}
	}
	return out
}

// ToDumpRequest converts and copies one explicit dump request.
func ToDumpRequest(frame enginewire.DumpRequest) machine.DumpRequest {
	products := make([]string, len(frame.Input.Products))
	for i, product := range frame.Input.Products {
		products[i] = string(product)
	}
	resources := make([]machine.DumpResourceSelector, len(frame.Input.Resources))
	for i, resource := range frame.Input.Resources {
		resources[i] = machine.DumpResourceSelector{
			Product:  string(resource.Product),
			Resource: resource.Resource,
		}
	}
	return machine.DumpRequest{
		RequestID:       RequestID(frame.ID),
		OutputDir:       frame.Input.OutputDir,
		Products:        products,
		Resources:       resources,
		ContinueOnError: frame.Input.ContinueOnError,
		Force:           frame.Input.Force,
	}
}

// ToDiffRequest converts and copies one explicit diff request.
func ToDiffRequest(frame enginewire.DiffRequest) machine.DiffRequest {
	products := make([]string, len(frame.Input.Products))
	for i, product := range frame.Input.Products {
		products[i] = string(product)
	}
	resources := make([]machine.DiffResourceSelector, len(frame.Input.Resources))
	for i, resource := range frame.Input.Resources {
		resources[i] = machine.DiffResourceSelector{
			Product:  string(resource.Product),
			Resource: resource.Resource,
		}
	}
	return machine.DiffRequest{
		RequestID:         RequestID(frame.ID),
		OldDir:            frame.Input.OldDir,
		NewDir:            frame.Input.NewDir,
		Products:          products,
		Resources:         resources,
		IgnoreOperational: frame.Input.IgnoreOperational,
		AllowPartial:      frame.Input.AllowPartial,
	}
}
