package machine

import (
	"errors"

	"github.com/dvmrry/zscalerctl/internal/resources"
)

// EventKind identifies one operation lifecycle event.
type EventKind string

const (
	EventStarted   EventKind = "started"
	EventProgress  EventKind = "progress"
	EventRecord    EventKind = "record"
	EventWarning   EventKind = "warning"
	EventCompleted EventKind = "completed"
	EventFailed    EventKind = "failed"
	EventCanceled  EventKind = "canceled"
)

var errEventHasNoWireFormat = errors.New("machine event has no wire format")

// Event is one candidate, in-process operation lifecycle event.
//
// Record values have already passed projection, redaction, and verification.
// Manifest is set only on a successful manifest completion. Err is set only on
// warning, failed, or canceled events and is always machine-safe. Event is not
// a wire type; future transports must define and version their own DTOs.
type Event struct {
	Kind     EventKind
	Product  string
	Resource string

	Done  int
	Total int

	Record   *resources.ProjectedRecord
	Manifest *Manifest
	Err      *MachineError

	Records   int
	Resources int
	Warnings  int
}

// MarshalJSON rejects direct Event serialization. Transports must convert an
// Event into an explicitly versioned wire DTO instead.
func (Event) MarshalJSON() ([]byte, error) {
	return nil, errEventHasNoWireFormat
}

// EventSink receives events synchronously on the ExecuteStream caller's
// goroutine. Returning an error aborts the operation.
type EventSink func(Event) error

type eventEmitter struct {
	sink     EventSink
	started  bool
	terminal bool
}

func (e *eventEmitter) emit(event Event) *MachineError {
	if e.sink == nil {
		return &MachineError{
			Kind:    ErrorKindInternal,
			Message: "event sink is not configured",
		}
	}
	if e.terminal {
		return &MachineError{
			Kind:    ErrorKindInternal,
			Message: "event emitted after terminal event",
		}
	}
	if !e.started && event.Kind != EventStarted {
		return &MachineError{
			Kind:    ErrorKindInternal,
			Message: "first event is not started",
		}
	}
	if e.started && event.Kind == EventStarted {
		return &MachineError{
			Kind:    ErrorKindInternal,
			Message: "started event emitted more than once",
		}
	}

	if event.Kind == EventStarted {
		e.started = true
	}
	if isTerminalEvent(event.Kind) {
		e.terminal = true
	}

	err, panicked := callEventSink(e.sink, copyEventForSink(event))
	if panicked {
		return &MachineError{
			Kind:     ErrorKindInternal,
			Message:  "event sink panicked",
			Product:  event.Product,
			Resource: event.Resource,
		}
	}
	if err != nil {
		machineErr := machineErrorFromSinkError(err)
		if machineErr.Product == "" {
			machineErr.Product = event.Product
		}
		if machineErr.Resource == "" {
			machineErr.Resource = event.Resource
		}
		return &machineErr
	}
	return nil
}

func (e *eventEmitter) failAfterDelivery(
	failure MachineError,
	op Operation,
	product string,
	resource string,
) error {
	failure = machineErrorWithContext(failure, op, product, resource)
	if !e.started || e.terminal {
		return &failure
	}
	return e.finishError(EventFailed, failure)
}

func (e *eventEmitter) finishError(kind EventKind, machineErr MachineError) error {
	machineErr = copyMachineError(machineErr)
	failure := e.emit(Event{
		Kind:     kind,
		Product:  machineErr.Product,
		Resource: machineErr.Resource,
		Err:      &machineErr,
	})
	if failure != nil {
		failureWithContext := machineErrorWithContext(
			*failure,
			machineErr.Operation,
			machineErr.Product,
			machineErr.Resource,
		)
		return &failureWithContext
	}
	return &machineErr
}

func (e *eventEmitter) finishMachineError(machineErr MachineError) error {
	kind := EventFailed
	if machineErr.Kind == ErrorKindCanceled {
		kind = EventCanceled
	}
	return e.finishError(kind, machineErr)
}

func isTerminalEvent(kind EventKind) bool {
	return kind == EventCompleted || kind == EventFailed || kind == EventCanceled
}

func callEventSink(sink EventSink, event Event) (err error, panicked bool) {
	defer func() {
		if recover() != nil {
			err = nil
			panicked = true
		}
	}()
	return sink(event), false
}

func machineErrorFromSinkError(err error) MachineError {
	var pointer *MachineError
	if errors.As(err, &pointer) && pointer != nil {
		return copyMachineError(*pointer)
	}
	var value MachineError
	if errors.As(err, &value) {
		return copyMachineError(value)
	}
	return MachineError{
		Kind:    ErrorKindInternal,
		Message: "event sink failed",
	}
}

func machineErrorWithContext(
	machineErr MachineError,
	op Operation,
	product string,
	resource string,
) MachineError {
	machineErr = copyMachineError(machineErr)
	if machineErr.Kind == "" {
		machineErr.Kind = ErrorKindInternal
	}
	if machineErr.Message == "" {
		machineErr.Message = "event sink failed"
	}
	if machineErr.Operation == "" {
		machineErr.Operation = op
	}
	if machineErr.Product == "" {
		machineErr.Product = product
	}
	if machineErr.Resource == "" {
		machineErr.Resource = resource
	}
	return machineErr
}

func copyEventForSink(event Event) Event {
	out := event
	if event.Record != nil {
		record := *event.Record
		out.Record = &record
	}
	if event.Manifest != nil {
		manifest := *event.Manifest
		manifest.Capabilities = append([]Capability(nil), event.Manifest.Capabilities...)
		manifest.Schemas = append([]SchemaRef(nil), event.Manifest.Schemas...)
		if event.Manifest.Meta != nil {
			meta := *event.Manifest.Meta
			manifest.Meta = &meta
		}
		out.Manifest = &manifest
	}
	if event.Err != nil {
		machineErr := copyMachineError(*event.Err)
		out.Err = &machineErr
	}
	return out
}

func copyMachineError(machineErr MachineError) MachineError {
	machineErr.Missing = append([]string(nil), machineErr.Missing...)
	return machineErr
}
