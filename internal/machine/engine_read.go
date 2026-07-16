package machine

import (
	"context"
	"fmt"
	"strings"
)

// Read executes one typed catalog-driven resource read through the shared
// operation-event path. It cannot execute manifest or another capability.
func (e Executor) Read(ctx context.Context, req ResourceReadRequest) (ResourceReadResult, error) {
	input := cloneResourceReadInput(req.Input)
	product := strings.TrimSpace(input.Product)
	resource := strings.TrimSpace(input.Resource)
	streamRequest := req
	streamRequest.Input = input
	emittedRecords := 0
	var result *ResourceReadResult
	err := e.readStream(ctx, streamRequest, func(event Event) error {
		switch event.Kind {
		case EventRecord:
			if event.Record == nil {
				return &MachineError{
					Kind:      ErrorKindInternal,
					Message:   "record event has no record",
					Operation: req.Operation,
					Product:   product,
					Resource:  resource,
				}
			}
			emittedRecords++
		case EventWarning:
			return &MachineError{
				Kind:      ErrorKindInternal,
				Message:   "resource read result cannot represent warning event",
				Operation: req.Operation,
				Product:   product,
				Resource:  resource,
			}
		case EventCompleted:
			if event.Manifest != nil {
				return &MachineError{
					Kind:      ErrorKindInternal,
					Message:   "resource read completed with manifest payload",
					Operation: req.Operation,
					Product:   product,
					Resource:  resource,
				}
			}
			if event.resourceResult == nil ||
				event.resourceResult.Records().Len() != emittedRecords ||
				event.Records != emittedRecords {
				return &MachineError{
					Kind:      ErrorKindInternal,
					Message:   "resource read completed with inconsistent typed result",
					Operation: req.Operation,
					Product:   product,
					Resource:  resource,
				}
			}
			result = event.resourceResult
		}
		return nil
	})
	if err != nil {
		return ResourceReadResult{}, err
	}
	if result == nil {
		return ResourceReadResult{}, &MachineError{
			Kind:      ErrorKindInternal,
			Message:   "resource read completed without typed result",
			Operation: req.Operation,
			Product:   product,
			Resource:  resource,
		}
	}
	return *result, nil
}

// ReadStream executes one typed catalog-driven resource read through the
// shared operation-event path. It exposes no generic capability selector or
// option map to callers and cannot execute manifest or another capability.
func (e Executor) ReadStream(ctx context.Context, req ResourceReadRequest, sink EventSink) error {
	req.Input = cloneResourceReadInput(req.Input)
	return e.readStream(ctx, req, sink)
}

func (e Executor) readStream(ctx context.Context, req ResourceReadRequest, sink EventSink) error {
	product := strings.TrimSpace(req.Input.Product)
	resource := strings.TrimSpace(req.Input.Resource)
	if !IsResourceReadOperation(req.Operation) {
		stream, err := StartEventStream(sink, req.Operation, product, resource, 0)
		if err != nil {
			return err
		}
		return failEventStream(
			stream,
			*unsupportedResourceReadOperationError(req.Operation, product, resource),
		)
	}
	return e.ExecuteStream(ctx, Request{
		RequestID:  req.RequestID,
		Capability: CapabilityResourcesRead,
		Operation:  req.Operation,
		Input:      &req.Input,
	}, sink)
}

func unsupportedResourceReadOperationError(
	operation Operation,
	product string,
	resource string,
) *MachineError {
	return &MachineError{
		Kind:      ErrorKindUnsupportedOperation,
		Message:   fmt.Sprintf("unsupported operation %q for %s", operation, CapabilityResourcesRead),
		Operation: operation,
		Product:   product,
		Resource:  resource,
	}
}
