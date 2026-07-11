package machine_test

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/machine"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

func TestExecutorExecuteStreamListEmitsOrderedProjectedRecords(t *testing.T) {
	loader := &fakeBrowserLoader{
		records: projectedRecordsFromFields(t,
			map[string]any{"id": "123", "name": "HQ"},
			map[string]any{"id": "456", "name": "Branch"},
		),
	}
	executor := machine.Executor{Browser: loader}
	req := machine.Request{
		RequestID:  "req-stream-list",
		Capability: machine.CapabilityResourcesRead,
		Operation:  machine.OperationList,
		Input:      &machine.Input{Product: "zia", Resource: "locations"},
	}

	var events []machine.Event
	err := executor.ExecuteStream(context.Background(), req, func(event machine.Event) error {
		events = append(events, event)
		return nil
	})
	if err != nil {
		t.Fatalf("Executor.ExecuteStream(list request) error = %v, want nil", err)
	}
	wantKinds := []machine.EventKind{
		machine.EventStarted,
		machine.EventRecord,
		machine.EventRecord,
		machine.EventCompleted,
	}
	assertEventKinds(t, events, wantKinds)

	wantRecords := []map[string]any{
		{"id": "123", "name": "HQ"},
		{"id": "456", "name": "Branch"},
	}
	for i, want := range wantRecords {
		event := events[i+1]
		if event.Record == nil {
			t.Fatalf("Executor.ExecuteStream(list request) event[%d].Record = nil, want record", i+1)
		}
		if got := event.Record.Fields(); !reflect.DeepEqual(got, want) {
			t.Errorf("Executor.ExecuteStream(list request) event[%d].Record = %#v, want %#v", i+1, got, want)
		}
		if event.Product != "zia" || event.Resource != "locations" {
			t.Errorf("Executor.ExecuteStream(list request) event[%d] scope = %s/%s, want zia/locations",
				i+1, event.Product, event.Resource)
		}
	}
	completed := events[len(events)-1]
	if completed.Records != 2 || completed.Resources != 1 || completed.Warnings != 0 {
		t.Errorf("Executor.ExecuteStream(list request) completed counts = records:%d resources:%d warnings:%d, want 2/1/0",
			completed.Records, completed.Resources, completed.Warnings)
	}
}

func TestExecutorExecuteReconstructionPreservesEmptyRecordSlice(t *testing.T) {
	executor := machine.Executor{Browser: &fakeBrowserLoader{}}
	req := machine.Request{
		Capability: machine.CapabilityResourcesRead,
		Operation:  machine.OperationList,
		Input:      &machine.Input{Product: "zia", Resource: "locations"},
	}

	resp, err := executor.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Executor.Execute(empty list request) error = %v, want nil", err)
	}
	if resp.Records == nil {
		t.Fatal("Executor.Execute(empty list request).Records = nil, want initialized empty slice")
	}
	if got := len(resp.Records); got != 0 {
		t.Errorf("Executor.Execute(empty list request).Records length = %d, want 0", got)
	}
}

func TestExecutorExecuteStreamManifestCarriesCompletionPayload(t *testing.T) {
	catalog := resources.ResourceCatalog{
		testExecutorSpec(resources.ProductZIA, "locations", resources.ReadOperations(), "id", "name"),
	}
	executor := machine.Executor{Browser: &fakeBrowserLoader{}, Catalog: catalog}
	req := machine.Request{RequestID: "req-stream-manifest", Operation: machine.OperationManifest}

	var events []machine.Event
	err := executor.ExecuteStream(context.Background(), req, func(event machine.Event) error {
		events = append(events, event)
		return nil
	})
	if err != nil {
		t.Fatalf("Executor.ExecuteStream(manifest request) error = %v, want nil", err)
	}
	assertEventKinds(t, events, []machine.EventKind{machine.EventStarted, machine.EventCompleted})
	completed := events[1]
	if completed.Manifest == nil {
		t.Fatal("Executor.ExecuteStream(manifest request) completed.Manifest = nil, want manifest")
	}
	if got := len(completed.Manifest.Capabilities); got != 1 {
		t.Errorf("Executor.ExecuteStream(manifest request) manifest capabilities = %d, want 1", got)
	}
}

func TestExecutorExecuteManifestPreservesEmptyCapabilities(t *testing.T) {
	emptyCatalog := resources.ResourceCatalog{}
	executor := machine.Executor{Catalog: emptyCatalog}

	resp, err := executor.Execute(context.Background(), machine.Request{Operation: machine.OperationManifest})
	if err != nil {
		t.Fatalf("Executor.Execute(empty manifest request) error = %v, want nil", err)
	}
	if resp.Manifest == nil {
		t.Fatal("Executor.Execute(empty manifest request).Manifest = nil, want manifest")
	}
	if resp.Manifest.Capabilities == nil {
		t.Fatal("Executor.Execute(empty manifest request).Manifest.Capabilities = nil, want initialized empty slice")
	}
	want := machine.ManifestFromCatalog(emptyCatalog)
	if !reflect.DeepEqual(*resp.Manifest, want) {
		t.Errorf("Executor.Execute(empty manifest request).Manifest = %#v, want %#v", *resp.Manifest, want)
	}
	body, err := json.Marshal(resp.Manifest)
	if err != nil {
		t.Fatalf("json.Marshal(empty reconstructed manifest) error = %v, want nil", err)
	}
	if !strings.Contains(string(body), `"capabilities":[]`) {
		t.Errorf("json.Marshal(empty reconstructed manifest) = %s, want capabilities empty array", body)
	}
}

func TestEventStreamOwnsTerminalDelivery(t *testing.T) {
	var events []machine.Event
	stream, err := machine.StartEventStream(func(event machine.Event) error {
		events = append(events, event)
		return nil
	}, machine.Operation("dump"), "", "", 2)
	if err != nil {
		t.Fatalf("StartEventStream() error = %v, want nil", err)
	}

	operationErr := machine.MachineError{
		Kind:      machine.ErrorKindCanceled,
		Message:   "request canceled",
		Operation: machine.OperationList,
		Product:   "zia",
		Resource:  "locations",
	}
	if err := stream.Fail(operationErr); err != nil {
		t.Fatalf("EventStream.Fail(delivered terminal) error = %v, want nil delivery error", err)
	}
	assertEventKinds(t, events, []machine.EventKind{machine.EventStarted, machine.EventCanceled})
	if events[0].Total != 2 {
		t.Errorf("EventStream started.Total = %d, want 2", events[0].Total)
	}
	if events[1].Err == nil || events[1].Err.Kind != machine.ErrorKindCanceled {
		t.Errorf("EventStream canceled error = %#v, want canceled", events[1].Err)
	}
}

func TestEventStreamRejectsTerminalBypassThroughEmit(t *testing.T) {
	var events []machine.Event
	stream, err := machine.StartEventStream(func(event machine.Event) error {
		events = append(events, event)
		return nil
	}, machine.OperationList, "zia", "locations", 0)
	if err != nil {
		t.Fatalf("StartEventStream() error = %v, want nil", err)
	}

	err = stream.Emit(machine.Event{Kind: machine.EventCompleted})
	assertMachineError(t, err, machine.ErrorKindInternal, machine.OperationList, "zia", "locations")
	assertEventKinds(t, events, []machine.EventKind{machine.EventStarted, machine.EventFailed})
	if events[1].Err == nil || events[1].Err.Message != "event stream received an invalid non-terminal event kind" {
		t.Errorf("EventStream invalid emit terminal error = %#v, want producer failure", events[1].Err)
	}
}

func TestExecutorExecuteStreamRecordFieldsAreDefensiveCopies(t *testing.T) {
	executor := machine.Executor{Browser: &fakeBrowserLoader{
		records: projectedRecordsFromFields(t, map[string]any{"name": "HQ", "ports": []int{80, 443}}),
	}}
	req := machine.Request{
		Capability: machine.CapabilityResourcesRead,
		Operation:  machine.OperationList,
		Input:      &machine.Input{Product: "zia", Resource: "locations"},
	}
	var retained *resources.ProjectedRecord
	err := executor.ExecuteStream(context.Background(), req, func(event machine.Event) error {
		if event.Kind == machine.EventRecord {
			retained = event.Record
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Executor.ExecuteStream(typed-slice record) error = %v, want nil", err)
	}
	if retained == nil {
		t.Fatal("Executor.ExecuteStream(typed-slice record) retained record = nil, want record")
	}
	fields := retained.Fields()
	fields["ports"].([]int)[0] = 8080
	ports, ok := retained.Value("ports")
	if !ok || !reflect.DeepEqual(ports, []int{80, 443}) {
		t.Errorf("retained ProjectedRecord.Value(ports) after caller mutation = %#v (present %t), want [80 443]", ports, ok)
	}
}

func TestExecutorExecuteStreamMapsFailuresToTerminalEvents(t *testing.T) {
	tests := []struct {
		name         string
		loaderErr    error
		wantKind     string
		wantTerminal machine.EventKind
	}{
		{name: "canceled", loaderErr: context.Canceled, wantKind: machine.ErrorKindCanceled, wantTerminal: machine.EventCanceled},
		{name: "deadline", loaderErr: context.DeadlineExceeded, wantKind: machine.ErrorKindDeadlineExceeded, wantTerminal: machine.EventFailed},
		{name: "backend", loaderErr: errors.New("raw backend secret"), wantKind: machine.ErrorKindLiveAccessFailed, wantTerminal: machine.EventFailed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := machine.Executor{Browser: &fakeBrowserLoader{err: tt.loaderErr}}
			req := machine.Request{
				Capability: machine.CapabilityResourcesRead,
				Operation:  machine.OperationList,
				Input:      &machine.Input{Product: "zia", Resource: "locations"},
			}
			var events []machine.Event
			err := executor.ExecuteStream(context.Background(), req, func(event machine.Event) error {
				events = append(events, event)
				return nil
			})
			machineErr := assertMachineError(t, err, tt.wantKind, machine.OperationList, "zia", "locations")
			assertEventKinds(t, events, []machine.EventKind{machine.EventStarted, tt.wantTerminal})
			terminal := events[1]
			if terminal.Err == nil || terminal.Err.Kind != tt.wantKind {
				t.Errorf("Executor.ExecuteStream(%s) terminal error = %#v, want kind %q", tt.name, terminal.Err, tt.wantKind)
			}
			if strings.Contains(machineErr.Message, "raw backend secret") {
				t.Errorf("Executor.ExecuteStream(%s) error message = %q, want sanitized", tt.name, machineErr.Message)
			}
		})
	}
}

func TestExecutorExecuteStreamSinkErrorEmitsSanitizedFailure(t *testing.T) {
	const rawSinkError = "consumer leaked secret value"
	executor := machine.Executor{Browser: &fakeBrowserLoader{
		records: projectedRecordsFromFields(t, map[string]any{"id": "123", "name": "HQ"}),
	}}
	req := machine.Request{
		Capability: machine.CapabilityResourcesRead,
		Operation:  machine.OperationList,
		Input:      &machine.Input{Product: "zia", Resource: "locations"},
	}
	var events []machine.Event
	err := executor.ExecuteStream(context.Background(), req, func(event machine.Event) error {
		events = append(events, event)
		if event.Kind == machine.EventRecord {
			return errors.New(rawSinkError)
		}
		return nil
	})

	machineErr := assertMachineError(t, err, machine.ErrorKindInternal, machine.OperationList, "zia", "locations")
	assertEventKinds(t, events, []machine.EventKind{
		machine.EventStarted,
		machine.EventRecord,
		machine.EventFailed,
	})
	if strings.Contains(machineErr.Message, rawSinkError) || strings.Contains(machineErr.Error(), rawSinkError) {
		t.Fatalf("Executor.ExecuteStream(sink error) error = %q, want no raw sink error", machineErr.Message)
	}
	terminal := events[len(events)-1]
	if terminal.Err == nil || terminal.Err.Kind != machine.ErrorKindInternal || terminal.Err.Message != "event sink failed" {
		t.Errorf("Executor.ExecuteStream(sink error) terminal error = %#v, want sanitized internal failure", terminal.Err)
	}
}

func TestExecutorExecuteStreamTypedNilMachineErrorEmitsFailure(t *testing.T) {
	executor := machine.Executor{Browser: &fakeBrowserLoader{}}
	req := machine.Request{
		Capability: machine.CapabilityResourcesRead,
		Operation:  machine.OperationList,
		Input:      &machine.Input{Product: "zia", Resource: "locations"},
	}
	var typedNil *machine.MachineError
	var events []machine.Event
	err := executor.ExecuteStream(context.Background(), req, func(event machine.Event) error {
		events = append(events, event)
		if event.Kind == machine.EventStarted {
			return typedNil
		}
		return nil
	})

	machineErr := assertMachineError(t, err, machine.ErrorKindInternal, machine.OperationList, "zia", "locations")
	assertEventKinds(t, events, []machine.EventKind{machine.EventStarted, machine.EventFailed})
	if machineErr.Message != "event sink failed" {
		t.Errorf("Executor.ExecuteStream(typed-nil sink error) message = %q, want sanitized event sink failure", machineErr.Message)
	}
	if events[1].Err == nil || events[1].Err.Kind != machine.ErrorKindInternal {
		t.Errorf("Executor.ExecuteStream(typed-nil sink error) terminal error = %#v, want internal", events[1].Err)
	}
}

func TestExecutorExecuteStreamSinkPanicEmitsSanitizedFailure(t *testing.T) {
	const panicValue = "panic containing tenant secret"
	executor := machine.Executor{Browser: &fakeBrowserLoader{
		records: projectedRecordsFromFields(t, map[string]any{"id": "123", "name": "HQ"}),
	}}
	req := machine.Request{
		Capability: machine.CapabilityResourcesRead,
		Operation:  machine.OperationList,
		Input:      &machine.Input{Product: "zia", Resource: "locations"},
	}
	var events []machine.Event
	err := executor.ExecuteStream(context.Background(), req, func(event machine.Event) error {
		events = append(events, event)
		if event.Kind == machine.EventRecord {
			panic(panicValue)
		}
		return nil
	})

	machineErr := assertMachineError(t, err, machine.ErrorKindInternal, machine.OperationList, "zia", "locations")
	assertEventKinds(t, events, []machine.EventKind{
		machine.EventStarted,
		machine.EventRecord,
		machine.EventFailed,
	})
	if strings.Contains(machineErr.Message, panicValue) {
		t.Fatalf("Executor.ExecuteStream(sink panic) error = %q, want no panic value", machineErr.Message)
	}
	terminal := events[len(events)-1]
	if terminal.Err == nil || terminal.Err.Message != "event sink panicked" {
		t.Errorf("Executor.ExecuteStream(sink panic) terminal error = %#v, want sanitized panic failure", terminal.Err)
	}
}

func TestExecutorExecuteStreamTerminalPanicDoesNotRetryTerminal(t *testing.T) {
	const panicValue = "terminal panic containing secret"
	executor := machine.Executor{Browser: &fakeBrowserLoader{
		records: projectedRecordsFromFields(t, map[string]any{"id": "123", "name": "HQ"}),
	}}
	req := machine.Request{
		Capability: machine.CapabilityResourcesRead,
		Operation:  machine.OperationList,
		Input:      &machine.Input{Product: "zia", Resource: "locations"},
	}
	var events []machine.Event
	err := executor.ExecuteStream(context.Background(), req, func(event machine.Event) error {
		events = append(events, event)
		if event.Kind == machine.EventCompleted {
			panic(panicValue)
		}
		return nil
	})

	machineErr := assertMachineError(t, err, machine.ErrorKindInternal, machine.OperationList, "zia", "locations")
	assertEventKinds(t, events, []machine.EventKind{
		machine.EventStarted,
		machine.EventRecord,
		machine.EventCompleted,
	})
	if strings.Contains(machineErr.Message, panicValue) {
		t.Fatalf("Executor.ExecuteStream(terminal panic) error = %q, want no panic value", machineErr.Message)
	}
	if got := terminalEventCount(events); got != 1 {
		t.Errorf("Executor.ExecuteStream(terminal panic) terminal events = %d, want 1", got)
	}
}

func TestExecutorExecuteStreamTerminalErrorDoesNotRetryTerminal(t *testing.T) {
	const rawSinkError = "terminal sink secret"
	executor := machine.Executor{Browser: &fakeBrowserLoader{
		records: projectedRecordsFromFields(t, map[string]any{"id": "123", "name": "HQ"}),
	}}
	req := machine.Request{
		Capability: machine.CapabilityResourcesRead,
		Operation:  machine.OperationList,
		Input:      &machine.Input{Product: "zia", Resource: "locations"},
	}
	var events []machine.Event
	err := executor.ExecuteStream(context.Background(), req, func(event machine.Event) error {
		events = append(events, event)
		if event.Kind == machine.EventCompleted {
			return errors.New(rawSinkError)
		}
		return nil
	})

	machineErr := assertMachineError(t, err, machine.ErrorKindInternal, machine.OperationList, "zia", "locations")
	assertEventKinds(t, events, []machine.EventKind{
		machine.EventStarted,
		machine.EventRecord,
		machine.EventCompleted,
	})
	if strings.Contains(machineErr.Message, rawSinkError) {
		t.Fatalf("Executor.ExecuteStream(terminal error) message = %q, want no raw sink error", machineErr.Message)
	}
	if got := terminalEventCount(events); got != 1 {
		t.Errorf("Executor.ExecuteStream(terminal error) terminal events = %d, want 1", got)
	}
}

func TestExecutorExecuteStreamCopiesTerminalErrorBeforeDelivery(t *testing.T) {
	executor := machine.Executor{Browser: &fakeBrowserLoader{err: context.Canceled}}
	req := machine.Request{
		Capability: machine.CapabilityResourcesRead,
		Operation:  machine.OperationList,
		Input:      &machine.Input{Product: "zia", Resource: "locations"},
	}
	err := executor.ExecuteStream(context.Background(), req, func(event machine.Event) error {
		if event.Kind == machine.EventCanceled {
			event.Err.Kind = "tampered"
			event.Err.Message = "tampered"
			event.Err.Missing = append(event.Err.Missing, "tampered")
		}
		return nil
	})
	assertMachineError(t, err, machine.ErrorKindCanceled, machine.OperationList, "zia", "locations")
}

func TestExecutorExecuteStreamRejectsNilSink(t *testing.T) {
	executor := machine.Executor{Browser: &fakeBrowserLoader{}}
	req := machine.Request{
		Capability: machine.CapabilityResourcesRead,
		Operation:  machine.OperationList,
		Input:      &machine.Input{Product: "zia", Resource: "locations"},
	}
	err := executor.ExecuteStream(context.Background(), req, nil)
	assertMachineError(t, err, machine.ErrorKindInternal, machine.OperationList, "zia", "locations")
}

func TestEventRejectsDirectJSONSerialization(t *testing.T) {
	records := projectedRecordsFromFields(t, map[string]any{"name": "must-not-serialize"})
	record := records.Records()[0]
	body, err := json.Marshal(machine.Event{Kind: machine.EventRecord, Record: &record})
	if err == nil {
		t.Fatalf("json.Marshal(machine.Event) error = nil; body = %s, want no wire format", body)
	}
	if strings.Contains(string(body), "must-not-serialize") {
		t.Fatalf("json.Marshal(machine.Event) body = %q, want no record bytes", body)
	}
	var decoded machine.Event
	if err := json.Unmarshal([]byte(`{"Kind":"record"}`), &decoded); err == nil {
		t.Fatalf("json.Unmarshal(machine.Event) error = nil; event = %#v, want no wire format", decoded)
	}
}

func assertEventKinds(t *testing.T, events []machine.Event, want []machine.EventKind) {
	t.Helper()
	got := make([]machine.EventKind, len(events))
	for i, event := range events {
		got[i] = event.Kind
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("event kinds = %#v, want %#v", got, want)
	}
}

func terminalEventCount(events []machine.Event) int {
	count := 0
	for _, event := range events {
		switch event.Kind {
		case machine.EventCompleted, machine.EventFailed, machine.EventCanceled:
			count++
		}
	}
	return count
}
