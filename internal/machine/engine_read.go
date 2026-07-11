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
	if !isSupportedReadOperation(req.Operation) {
		return ResourceReadResult{}, &MachineError{
			Kind:      ErrorKindUnsupportedOperation,
			Message:   fmt.Sprintf("unsupported operation %q for %s", req.Operation, CapabilityResourcesRead),
			Operation: req.Operation,
			Product:   product,
			Resource:  resource,
		}
	}

	legacyRequest := Request{
		RequestID:  req.RequestID,
		Capability: CapabilityResourcesRead,
		Operation:  req.Operation,
		Input:      &input,
	}
	emittedRecords := 0
	var result *ResourceReadResult
	err := e.ExecuteStream(ctx, legacyRequest, func(event Event) error {
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
