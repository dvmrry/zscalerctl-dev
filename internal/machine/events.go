package machine

import (
	"errors"
	"slices"

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
// Product and Resource are caller-controlled selectors until the operation has
// successfully resolved them through the catalog-backed projected loader.
type Event struct {
	Kind     EventKind
	Product  string
	Resource string

	Done  int
	Total int

	Record   *resources.ProjectedRecord
	Manifest *Manifest
	Err      *MachineError

	// Successful completion counters. Records counts emitted record events;
	// Resources counts successfully completed resources (including a
	// zero-record resource); Warnings counts emitted warning events. For a
	// multi-resource operation, started/progress Total carries the selected
	// resource count, so Resources+Warnings can describe a partial completion.
	Records   int
	Resources int
	Warnings  int

	// resourceResult preserves the typed immutable result for in-package adapters
	// while public event consumers continue to receive record events and counters.
	resourceResult *ResourceReadResult
}

// MarshalJSON rejects direct Event serialization. Transports must convert an
// Event into an explicitly versioned wire DTO instead.
func (Event) MarshalJSON() ([]byte, error) {
	return nil, errEventHasNoWireFormat
}

// UnmarshalJSON rejects direct Event deserialization. Transports must decode an
// explicitly versioned wire DTO and validate it before constructing events.
func (*Event) UnmarshalJSON([]byte) error {
	return errEventHasNoWireFormat
}

// EventSink receives events synchronously on the producer caller's goroutine.
// Returning an error aborts the operation.
type EventSink func(Event) error

// EventStream owns one candidate in-process event lifecycle. It emits the
// started event during construction, accepts only non-terminal events through
// Emit, and owns completion/failure delivery so producers cannot accidentally
// bypass the terminal-exactly-once rules.
//
// Fail reports only a terminal-delivery failure. When delivery succeeds, the
// producer remains responsible for returning its own operation error. This
// keeps a sanitized event payload separate from richer in-process errors whose
// sentinel identity an existing adapter may need to preserve.
type EventStream struct {
	emitter   eventEmitter
	operation Operation
	product   string
	resource  string
}

// StartEventStream starts one synchronous event lifecycle.
func StartEventStream(
	sink EventSink,
	operation Operation,
	product string,
	resource string,
	total int,
) (*EventStream, error) {
	stream := &EventStream{
		emitter:   eventEmitter{sink: sink},
		operation: operation,
		product:   product,
		resource:  resource,
	}
	if failure := stream.emitter.emit(Event{
		Kind:     EventStarted,
		Product:  product,
		Resource: resource,
		Total:    total,
	}); failure != nil {
		return nil, stream.emitter.failAfterDelivery(*failure, operation, product, resource)
	}
	return stream, nil
}

// Emit delivers one non-terminal progress, record, or warning event.
func (s *EventStream) Emit(event Event) error {
	if s == nil {
		return &MachineError{Kind: ErrorKindInternal, Message: "event stream is nil"}
	}
	if message := invalidNonTerminalEvent(event); message != "" {
		return s.failProducer(message)
	}
	s.applyDefaultScope(&event)
	if failure := s.emitter.emit(event); failure != nil {
		return s.emitter.failAfterDelivery(*failure, s.operation, event.Product, event.Resource)
	}
	return nil
}

// Complete delivers the successful terminal event. Kind may be empty or
// EventCompleted; the stream sets it to EventCompleted before delivery.
func (s *EventStream) Complete(event Event) error {
	if s == nil {
		return &MachineError{Kind: ErrorKindInternal, Message: "event stream is nil"}
	}
	if event.Kind != "" && event.Kind != EventCompleted {
		return s.failProducer("completion has a non-completed event kind")
	}
	if event.Record != nil || event.Err != nil || event.Done != 0 || event.Total != 0 {
		return s.failProducer("completion has a non-completion payload")
	}
	if event.Records < 0 || event.Resources < 0 || event.Warnings < 0 {
		return s.failProducer("completion has a negative count")
	}
	if event.Manifest != nil && (event.Records != 0 || event.Resources != 0 || event.Warnings != 0) {
		return s.failProducer("manifest completion has resource counters")
	}
	if event.Manifest != nil && event.resourceResult != nil {
		return s.failProducer("manifest completion has typed resource result")
	}
	event.Kind = EventCompleted
	s.applyDefaultScope(&event)
	if failure := s.emitter.emit(event); failure != nil {
		return s.emitter.failAfterDelivery(*failure, s.operation, event.Product, event.Resource)
	}
	return nil
}

// Fail delivers a failed or canceled terminal event. A nil return means the
// terminal event was delivered; it does not mean the producer operation
// succeeded. The caller should then return its original operation error.
func (s *EventStream) Fail(machineErr MachineError) error {
	if s == nil {
		return &MachineError{Kind: ErrorKindInternal, Message: "event stream is nil"}
	}
	machineErr = machineErrorWithContext(
		machineErr,
		s.operation,
		s.product,
		s.resource,
	)
	kind := EventFailed
	if machineErr.Kind == ErrorKindCanceled {
		kind = EventCanceled
	}
	if failure := s.emitter.emitTerminalError(kind, machineErr); failure != nil {
		return failure
	}
	return nil
}

func (s *EventStream) applyDefaultScope(event *Event) {
	if event.Product == "" {
		event.Product = s.product
	}
	if event.Resource == "" {
		event.Resource = s.resource
	}
}

func (s *EventStream) failProducer(message string) error {
	machineErr := MachineError{
		Kind:      ErrorKindInternal,
		Message:   message,
		Operation: s.operation,
		Product:   s.product,
		Resource:  s.resource,
	}
	if err := s.Fail(machineErr); err != nil {
		return err
	}
	return &machineErr
}

func invalidNonTerminalEvent(event Event) string {
	switch event.Kind {
	case EventProgress:
		if event.Done < 0 || event.Total < 0 || (event.Total > 0 && event.Done > event.Total) {
			return "progress event has invalid counters"
		}
		if event.Record != nil ||
			event.Manifest != nil ||
			event.Err != nil ||
			event.resourceResult != nil ||
			hasCompletionCounters(event) {
			return "progress event has a non-progress payload"
		}
	case EventRecord:
		if event.Record == nil {
			return "record event has no record"
		}
		if event.Manifest != nil ||
			event.Err != nil ||
			event.resourceResult != nil ||
			event.Done != 0 ||
			event.Total != 0 ||
			hasCompletionCounters(event) {
			return "record event has a non-record payload"
		}
	case EventWarning:
		if event.Err == nil {
			return "warning event has no machine error"
		}
		if event.Record != nil ||
			event.Manifest != nil ||
			event.resourceResult != nil ||
			event.Done != 0 ||
			event.Total != 0 ||
			hasCompletionCounters(event) {
			return "warning event has a non-warning payload"
		}
	default:
		return "event stream received an invalid non-terminal event kind"
	}
	return ""
}

func hasCompletionCounters(event Event) bool {
	return event.Records != 0 || event.Resources != 0 || event.Warnings != 0
}

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
	if deliveryFailure := e.emitTerminalError(EventFailed, failure); deliveryFailure != nil {
		return deliveryFailure
	}
	return &failure
}

func (e *eventEmitter) emitTerminalError(kind EventKind, machineErr MachineError) *MachineError {
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
	return nil
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
	if errors.As(err, &pointer) {
		if pointer == nil {
			return MachineError{
				Kind:    ErrorKindInternal,
				Message: "event sink failed",
			}
		}
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
	if event.resourceResult != nil {
		result := *event.resourceResult
		out.resourceResult = &result
	}
	if event.Record != nil {
		record := *event.Record
		out.Record = &record
	}
	if event.Manifest != nil {
		manifest := *event.Manifest
		manifest.Capabilities = slices.Clone(event.Manifest.Capabilities)
		manifest.Schemas = slices.Clone(event.Manifest.Schemas)
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
	machineErr.Missing = slices.Clone(machineErr.Missing)
	return machineErr
}
