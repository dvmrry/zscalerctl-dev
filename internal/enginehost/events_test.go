package enginehost

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/enginewire"
	"github.com/dvmrry/zscalerctl/internal/machine"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

func TestEventBridgeConvertsOnlyProvisionalDumpMetadata(t *testing.T) {
	t.Parallel()

	var provisional []provisionalFrame
	bridge := newEventBridge(machine.OperationDump, func(frame provisionalFrame) error {
		provisional = append(provisional, frame)
		return nil
	})
	records := resources.NewProjectedRecordsFromProjectedFields([]map[string]any{{"secret_canary": "must-not-be-read"}}).Records()
	warningErr := &machine.MachineError{
		Kind: "list_failed", Operation: machine.OperationList, Product: "zia", Resource: "locations",
	}
	events := []machine.Event{
		{Kind: machine.EventStarted, Total: 1},
		{Kind: machine.EventProgress, Done: 1, Total: 1, Product: "zia", Resource: "locations"},
		{Kind: machine.EventRecord, Product: "zia", Resource: "locations", Record: &records[0]},
		{Kind: machine.EventWarning, Product: "zia", Resource: "locations", Err: warningErr},
		{Kind: machine.EventCompleted, Records: 1, Resources: 0, Warnings: 1},
	}
	for _, event := range events {
		if err := bridge.Accept(event); err != nil {
			t.Fatalf("Accept(%s) error = %v", event.Kind, err)
		}
	}
	if err := bridge.Finish(nil); err != nil {
		t.Fatalf("Finish(nil) error = %v", err)
	}
	if len(provisional) != 2 {
		t.Fatalf("provisional frames = %d, want progress + warning", len(provisional))
	}
	if _, ok := provisional[0].(provisionalProgress); !ok {
		t.Fatalf("first provisional = %T, want progress", provisional[0])
	}
	warning, ok := provisional[1].(provisionalWarning)
	if !ok || warning.warning.Kind != "list_failed" || warning.warning.Resource != "locations" {
		t.Fatalf("warning = %#v", provisional[1])
	}
	wantWarnings := []enginewire.DumpFailure{{
		Product: enginewire.ProductZIA, Resource: "locations", Phase: "list", Kind: "list_failed",
	}}
	if got := bridge.Warnings(); !reflect.DeepEqual(got, wantWarnings) {
		t.Fatalf("Warnings() = %#v, want %#v", got, wantWarnings)
	}
}

func TestEventBridgeRejectsLifecycleAndTerminalClassificationMismatches(t *testing.T) {
	t.Parallel()

	bridge := newEventBridge(machine.OperationDiff, func(provisionalFrame) error { return nil })
	if err := bridge.Accept(machine.Event{Kind: machine.EventProgress, Done: 1, Total: 1, Product: "zia", Resource: "locations"}); !errors.Is(err, errInvalidEngineEvents) {
		t.Fatalf("progress before started error = %v", err)
	}

	bridge = newEventBridge(machine.OperationDiff, func(provisionalFrame) error { return nil })
	if err := bridge.Accept(machine.Event{Kind: machine.EventStarted, Total: 1}); err != nil {
		t.Fatalf("Accept(started) error = %v", err)
	}
	if err := bridge.Accept(machine.Event{Kind: machine.EventWarning, Err: &machine.MachineError{}}); !errors.Is(err, errInvalidEngineEvents) {
		t.Fatalf("diff warning error = %v", err)
	}

	bridge = newEventBridge(machine.OperationDiff, func(provisionalFrame) error { return nil })
	_ = bridge.Accept(machine.Event{Kind: machine.EventStarted, Total: 0})
	_ = bridge.Accept(machine.Event{Kind: machine.EventFailed, Err: &machine.MachineError{Kind: machine.ErrorKindInternal}})
	if err := bridge.Finish(context.Canceled); !errors.Is(err, errInvalidEngineEvents) {
		t.Fatalf("mismatched canceled return error = %v", err)
	}
}

func TestEventBridgeReturnsStaticSessionClosingError(t *testing.T) {
	t.Parallel()

	bridge := newEventBridge(machine.OperationDiff, func(provisionalFrame) error { return errSessionClosing })
	if err := bridge.Accept(machine.Event{Kind: machine.EventStarted, Total: 1}); err != nil {
		t.Fatalf("Accept(started) error = %v", err)
	}
	err := bridge.Accept(machine.Event{Kind: machine.EventProgress, Done: 1, Total: 1, Product: "zia", Resource: "locations"})
	if !isSessionClosingError(err) {
		t.Fatalf("Accept(progress) error = %v, want session-closing sentinel", err)
	}
}

func TestEventBridgeRejectsSkippedOrIncompleteProgress(t *testing.T) {
	t.Parallel()

	bridge := newEventBridge(machine.OperationDiff, func(provisionalFrame) error { return nil })
	if err := bridge.Accept(machine.Event{Kind: machine.EventStarted, Total: 2}); err != nil {
		t.Fatalf("Accept(started) error = %v", err)
	}
	if err := bridge.Accept(machine.Event{
		Kind: machine.EventProgress, Done: 2, Total: 2, Product: "zia", Resource: "locations",
	}); !errors.Is(err, errInvalidEngineEvents) {
		t.Errorf("Accept(skipped progress) error = %v, want errInvalidEngineEvents", err)
	}

	bridge = newEventBridge(machine.OperationDiff, func(provisionalFrame) error { return nil })
	_ = bridge.Accept(machine.Event{Kind: machine.EventStarted, Total: 2})
	_ = bridge.Accept(machine.Event{Kind: machine.EventProgress, Done: 1, Total: 2, Product: "zia", Resource: "locations"})
	_ = bridge.Accept(machine.Event{Kind: machine.EventCompleted, Resources: 2})
	if err := bridge.Finish(nil); !errors.Is(err, errInvalidEngineEvents) {
		t.Errorf("Finish(incomplete progress) error = %v, want errInvalidEngineEvents", err)
	}
}

func TestEventBridgeRejectsDumpResourceAccountingMismatch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		resources int
		warnings  int
	}{
		{name: "unaccounted resource", resources: 1},
		{name: "too many resources", resources: 3},
		{name: "resources plus warnings exceed total", resources: 1, warnings: 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			bridge := newEventBridge(machine.OperationDump, func(provisionalFrame) error { return nil })
			if err := bridge.Accept(machine.Event{Kind: machine.EventStarted, Total: 2}); err != nil {
				t.Fatalf("Accept(started) error = %v", err)
			}
			for current := 1; current <= 2; current++ {
				if err := bridge.Accept(machine.Event{
					Kind: machine.EventProgress, Done: current, Total: 2,
					Product: "zia", Resource: "locations",
				}); err != nil {
					t.Fatalf("Accept(progress %d) error = %v", current, err)
				}
			}
			for warning := 0; warning < tt.warnings; warning++ {
				machineErr := &machine.MachineError{
					Kind: "list_failed", Operation: machine.OperationList,
					Product: "zia", Resource: "locations",
				}
				if err := bridge.Accept(machine.Event{
					Kind: machine.EventWarning, Product: "zia", Resource: "locations", Err: machineErr,
				}); err != nil {
					t.Fatalf("Accept(warning %d) error = %v", warning, err)
				}
			}
			if err := bridge.Accept(machine.Event{
				Kind: machine.EventCompleted, Resources: tt.resources, Warnings: tt.warnings,
			}); err != nil {
				t.Fatalf("Accept(completed) error = %v", err)
			}
			if err := bridge.Finish(nil); !errors.Is(err, errInvalidEngineEvents) {
				t.Fatalf("Finish(mismatched accounting) error = %v, want errInvalidEngineEvents", err)
			}
		})
	}
}
