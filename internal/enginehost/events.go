package enginehost

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/dvmrry/zscalerctl/internal/enginewire"
	"github.com/dvmrry/zscalerctl/internal/enginewire/adapter"
	"github.com/dvmrry/zscalerctl/internal/machine"
)

type provisionalFrame interface {
	serverFrame(id, sequence enginewire.SafeInteger) enginewire.ServerFrame
}

type provisionalProgress struct {
	current  enginewire.SafeInteger
	total    enginewire.SafeInteger
	product  enginewire.Product
	resource string
}

func (p provisionalProgress) serverFrame(id, sequence enginewire.SafeInteger) enginewire.ServerFrame {
	return enginewire.Progress{
		Type: "progress", ID: id, Sequence: sequence, Phase: "resource_started",
		Current: p.current, Total: p.total, Product: p.product, Resource: p.resource,
	}
}

type provisionalWarning struct {
	warning enginewire.DumpFailure
}

func (w provisionalWarning) serverFrame(id, sequence enginewire.SafeInteger) enginewire.ServerFrame {
	return enginewire.Warning{Type: "warning", ID: id, Sequence: sequence, Warning: w.warning}
}

type eventBridge struct {
	operation machine.Operation
	emit      func(provisionalFrame) error

	started      bool
	terminal     machine.EventKind
	terminalErr  error
	startedTotal int
	progressDone int
	completed    machine.Event
	warnings     []enginewire.DumpFailure
}

func newEventBridge(operation machine.Operation, emit func(provisionalFrame) error) *eventBridge {
	return &eventBridge{operation: operation, emit: emit}
}

func (b *eventBridge) Accept(event machine.Event) error {
	if b == nil || b.emit == nil {
		return fmt.Errorf("%w: nil event bridge", errInvalidEngineEvents)
	}
	if b.terminal != "" {
		return fmt.Errorf("%w: event after terminal", errInvalidEngineEvents)
	}
	switch event.Kind {
	case machine.EventStarted:
		if b.started || event.Total < 0 || event.Done != 0 || event.Record != nil || event.Manifest != nil || event.Err != nil ||
			event.Records != 0 || event.Resources != 0 || event.Warnings != 0 || event.Product != "" || event.Resource != "" {
			return fmt.Errorf("%w: invalid started event", errInvalidEngineEvents)
		}
		b.started = true
		b.startedTotal = event.Total
		return nil

	case machine.EventProgress:
		if !b.started || (b.operation != machine.OperationDump && b.operation != machine.OperationDiff) ||
			event.Done < 1 || event.Total < 1 || event.Done > event.Total || event.Total != b.startedTotal ||
			event.Done != b.progressDone+1 ||
			event.Record != nil || event.Manifest != nil || event.Err != nil ||
			event.Records != 0 || event.Resources != 0 || event.Warnings != 0 {
			return fmt.Errorf("%w: invalid progress event", errInvalidEngineEvents)
		}
		current, err := safeWireCount(event.Done)
		if err != nil {
			return err
		}
		total, err := safeWireCount(event.Total)
		if err != nil {
			return err
		}
		progress := provisionalProgress{
			current: current, total: total, product: enginewire.Product(event.Product), resource: event.Resource,
		}
		probe := progress.serverFrame(1, 1)
		if _, err := enginewire.MarshalServerFrame(probe); err != nil {
			return fmt.Errorf("%w: invalid progress conversion: %v", errInvalidEngineEvents, err)
		}
		if err := b.emit(progress); err != nil {
			return err
		}
		b.progressDone = event.Done
		return nil

	case machine.EventRecord:
		if !b.started || b.operation != machine.OperationDump || event.Record == nil || event.Err != nil || event.Manifest != nil ||
			event.Done != 0 || event.Total != 0 || event.Records != 0 || event.Resources != 0 || event.Warnings != 0 {
			return fmt.Errorf("%w: invalid dump record event", errInvalidEngineEvents)
		}
		// Dump values are deliberately acknowledged without conversion, sizing,
		// logging, or writer submission.
		return nil

	case machine.EventWarning:
		if !b.started || b.operation != machine.OperationDump || event.Err == nil || event.Record != nil || event.Manifest != nil ||
			event.Done != 0 || event.Total != 0 || event.Records != 0 || event.Resources != 0 || event.Warnings != 0 {
			return fmt.Errorf("%w: invalid dump warning event", errInvalidEngineEvents)
		}
		if event.Product != event.Err.Product || event.Resource != event.Err.Resource {
			return fmt.Errorf("%w: warning scope mismatch", errInvalidEngineEvents)
		}
		warning := enginewire.DumpFailure{
			Product: enginewire.Product(event.Product), Resource: event.Resource,
			Phase: string(event.Err.Operation), Kind: event.Err.Kind,
		}
		probe := provisionalWarning{warning: warning}.serverFrame(1, 1)
		if _, err := enginewire.MarshalServerFrame(probe); err != nil {
			return fmt.Errorf("%w: invalid warning conversion: %v", errInvalidEngineEvents, err)
		}
		b.warnings = append(b.warnings, warning)
		return b.emit(provisionalWarning{warning: warning})

	case machine.EventCompleted:
		if !b.started || event.Err != nil || event.Record != nil || event.Manifest != nil ||
			event.Done != 0 || event.Total != 0 || event.Product != "" || event.Resource != "" {
			return fmt.Errorf("%w: invalid completed event", errInvalidEngineEvents)
		}
		b.terminal = machine.EventCompleted
		b.completed = event
		return nil

	case machine.EventFailed, machine.EventCanceled:
		if !b.started || event.Err == nil || event.Record != nil || event.Manifest != nil ||
			event.Done != 0 || event.Total != 0 || event.Records != 0 || event.Resources != 0 || event.Warnings != 0 {
			return fmt.Errorf("%w: invalid error terminal", errInvalidEngineEvents)
		}
		copyErr := *event.Err
		copyErr.Missing = append([]string(nil), event.Err.Missing...)
		b.terminal = event.Kind
		b.terminalErr = &copyErr
		return nil
	default:
		return fmt.Errorf("%w: unsupported event kind %q", errInvalidEngineEvents, event.Kind)
	}
}

func (b *eventBridge) Finish(operationErr error) error {
	if b == nil || !b.started || b.terminal == "" {
		return fmt.Errorf("%w: missing started or terminal event", errInvalidEngineEvents)
	}
	if operationErr == nil {
		if b.terminal != machine.EventCompleted {
			return fmt.Errorf("%w: successful operation has non-completed terminal", errInvalidEngineEvents)
		}
		if b.operation == machine.OperationDump {
			if b.completed.Warnings != len(b.warnings) || b.completed.Resources < 0 ||
				len(b.warnings) > b.startedTotal ||
				b.completed.Resources != b.startedTotal-len(b.warnings) ||
				b.completed.Records < 0 || b.progressDone != b.startedTotal {
				return fmt.Errorf("%w: dump completion counters do not reconcile", errInvalidEngineEvents)
			}
		} else if b.completed.Warnings != 0 || len(b.warnings) != 0 || b.completed.Records != 0 ||
			b.completed.Resources != b.startedTotal || b.progressDone != b.startedTotal {
			return fmt.Errorf("%w: diff completion counters do not reconcile", errInvalidEngineEvents)
		}
		return nil
	}

	conversion := adapter.ToOperationError(operationErr)
	wantTerminal := machine.EventFailed
	if conversion.Canceled {
		wantTerminal = machine.EventCanceled
	}
	if b.terminal != wantTerminal || b.terminalErr == nil {
		return fmt.Errorf("%w: returned error and terminal kind disagree", errInvalidEngineEvents)
	}
	terminalConversion := adapter.ToOperationError(b.terminalErr)
	if !reflect.DeepEqual(terminalConversion, conversion) {
		return fmt.Errorf("%w: returned error and terminal classification disagree", errInvalidEngineEvents)
	}
	return nil
}

func (b *eventBridge) Warnings() []enginewire.DumpFailure {
	if b == nil || b.warnings == nil {
		return nil
	}
	return append([]enginewire.DumpFailure(nil), b.warnings...)
}

func (b *eventBridge) Completion() machine.Event {
	if b == nil {
		return machine.Event{}
	}
	return b.completed
}

func isSessionClosingError(err error) bool {
	return errors.Is(err, errSessionClosing)
}
