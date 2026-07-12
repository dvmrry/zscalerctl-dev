package machine_test

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/dvmrry/zscalerctl/internal/machine"
	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

func TestExecutorReadRoutesTypedOperations(t *testing.T) {
	tests := []struct {
		name      string
		operation machine.Operation
		input     machine.ResourceReadInput
		wantCall  string
	}{
		{
			name:      "list",
			operation: machine.OperationList,
			input:     machine.ResourceReadInput{Product: "zia", Resource: "locations"},
			wantCall:  "list:zia/locations",
		},
		{
			name:      "show",
			operation: machine.OperationShow,
			input:     machine.ResourceReadInput{Product: "zia", Resource: "advanced-settings"},
			wantCall:  "show:zia/advanced-settings",
		},
		{
			name:      "get",
			operation: machine.OperationGet,
			input: machine.ResourceReadInput{
				Product: "zia", Resource: "locations", RecordID: "123",
			},
			wantCall: "get:zia/locations/123",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projected := projectedRecordsFromFields(t, map[string]any{"id": "123", "name": "HQ"})
			loader := &fakeBrowserLoader{records: projected, getRecords: projected}
			result, err := (machine.Executor{Browser: loader}).Read(
				context.Background(),
				machine.ResourceReadRequest{
					RequestID: "typed-" + tt.name,
					Operation: tt.operation,
					Input:     tt.input,
				},
			)
			if err != nil {
				t.Fatalf("Executor.Read(%s) error = %v, want nil", tt.name, err)
			}
			if !reflect.DeepEqual(loader.calls, []string{tt.wantCall}) {
				t.Fatalf("Executor.Read(%s) loader calls = %#v, want %q", tt.name, loader.calls, tt.wantCall)
			}
			assertProjectedResultFields(t, result, []map[string]any{{"id": "123", "name": "HQ"}})
		})
	}
}

func TestExecutorReadStreamRoutesTypedRequestAndEvents(t *testing.T) {
	t.Parallel()

	projected := projectedRecordsFromFields(t,
		map[string]any{"id": "1", "name": "HQ"},
		map[string]any{"id": "2", "name": "Branch"},
	)
	loader := &fakeBrowserLoader{records: projected}
	var events []machine.Event
	err := (machine.Executor{Browser: loader}).ReadStream(
		context.Background(),
		machine.ResourceReadRequest{
			RequestID: "typed-stream",
			Operation: machine.OperationList,
			Input: machine.ResourceReadInput{
				Product: "zia", Resource: "locations",
			},
		},
		func(event machine.Event) error {
			events = append(events, event)
			return nil
		},
	)
	if err != nil {
		t.Fatalf("Executor.ReadStream(list locations) error = %v, want nil", err)
	}
	assertEventKinds(t, events, []machine.EventKind{
		machine.EventStarted,
		machine.EventRecord,
		machine.EventRecord,
		machine.EventCompleted,
	})
	if !reflect.DeepEqual(loader.calls, []string{"list:zia/locations"}) {
		t.Fatalf("Executor.ReadStream(list locations) loader calls = %#v, want one list", loader.calls)
	}
	gotRecords := []map[string]any{
		events[1].Record.Fields(),
		events[2].Record.Fields(),
	}
	wantRecords := []map[string]any{
		{"id": "1", "name": "HQ"},
		{"id": "2", "name": "Branch"},
	}
	if !reflect.DeepEqual(gotRecords, wantRecords) {
		t.Fatalf("Executor.ReadStream(list locations) records = %#v, want %#v", gotRecords, wantRecords)
	}
	if events[3].Records != 2 || events[3].Resources != 1 || events[3].Warnings != 0 {
		t.Fatalf("Executor.ReadStream(list locations) completion = %#v, want 2 records/1 resource/0 warnings", events[3])
	}
}

func TestExecutorReadStreamCopiesInputBeforeStartedEvent(t *testing.T) {
	t.Parallel()

	fields := []string{"name"}
	filters := []machine.Filter{{Field: "country", Operator: "=", Value: "US"}}
	request := machine.ResourceReadRequest{
		Operation: machine.OperationList,
		Input: machine.ResourceReadInput{
			Product:  "zia",
			Resource: "locations",
			Fields:   fields,
			Filters:  filters,
		},
	}
	loader := &fakeBrowserLoader{records: projectedRecordsFromFields(t,
		map[string]any{"id": "1", "name": "HQ", "country": "US"},
		map[string]any{"id": "2", "name": "Branch", "country": "DE"},
	)}
	catalog := resources.ResourceCatalog{
		testExecutorSpec(resources.ProductZIA, "locations", resources.ListOperations(), "id", "name", "country"),
	}
	var records []map[string]any
	err := (machine.Executor{
		Browser: loader, Catalog: catalog, Redaction: redact.ModeStandard,
	}).ReadStream(context.Background(), request, func(event machine.Event) error {
		if event.Kind == machine.EventStarted {
			fields[0] = "id"
			filters[0].Value = "DE"
		}
		if event.Kind == machine.EventRecord {
			records = append(records, event.Record.Fields())
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Executor.ReadStream(mutated caller input) error = %v, want nil", err)
	}
	want := []map[string]any{{"name": "HQ"}}
	if !reflect.DeepEqual(records, want) {
		t.Fatalf("Executor.ReadStream(mutated caller input) records = %#v, want %#v", records, want)
	}
}

func TestExecutorReadStreamRejectsNonReadOperationWithTerminalFailure(t *testing.T) {
	t.Parallel()

	loader := &fakeBrowserLoader{}
	var events []machine.Event
	err := (machine.Executor{Browser: loader}).ReadStream(
		context.Background(),
		machine.ResourceReadRequest{
			Operation: machine.OperationManifest,
			Input: machine.ResourceReadInput{
				Product: "zia", Resource: "locations",
			},
		},
		func(event machine.Event) error {
			events = append(events, event)
			return nil
		},
	)
	assertMachineError(
		t,
		err,
		machine.ErrorKindUnsupportedOperation,
		machine.OperationManifest,
		"zia",
		"locations",
	)
	assertEventKinds(t, events, []machine.EventKind{machine.EventStarted, machine.EventFailed})
	if len(loader.calls) != 0 {
		t.Fatalf("Executor.ReadStream(manifest) loader calls = %#v, want none", loader.calls)
	}
}

func TestExecutorReadStreamPreservesCancellationAndDeadlineTerminals(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		loadErr      error
		wantKind     string
		wantTerminal machine.EventKind
	}{
		{
			name:         "canceled",
			loadErr:      context.Canceled,
			wantKind:     machine.ErrorKindCanceled,
			wantTerminal: machine.EventCanceled,
		},
		{
			name:         "deadline exceeded",
			loadErr:      context.DeadlineExceeded,
			wantKind:     machine.ErrorKindDeadlineExceeded,
			wantTerminal: machine.EventFailed,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var events []machine.Event
			err := (machine.Executor{Browser: &fakeBrowserLoader{err: tt.loadErr}}).ReadStream(
				context.Background(),
				machine.ResourceReadRequest{
					Operation: machine.OperationList,
					Input: machine.ResourceReadInput{
						Product: "zia", Resource: "locations",
					},
				},
				func(event machine.Event) error {
					events = append(events, event)
					return nil
				},
			)
			assertMachineError(
				t,
				err,
				tt.wantKind,
				machine.OperationList,
				"zia",
				"locations",
			)
			assertEventKinds(t, events, []machine.EventKind{machine.EventStarted, tt.wantTerminal})
		})
	}
}

func TestExecutorReadStreamContainsSinkFailureAndPanic(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		panics bool
	}{
		{
			name: "error",
		},
		{
			name:   "panic",
			panics: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			projected := projectedRecordsFromFields(t, map[string]any{"id": "1", "name": "HQ"})
			var events []machine.Event
			err := (machine.Executor{Browser: &fakeBrowserLoader{records: projected}}).ReadStream(
				context.Background(),
				machine.ResourceReadRequest{
					Operation: machine.OperationList,
					Input: machine.ResourceReadInput{
						Product: "zia", Resource: "locations",
					},
				},
				func(event machine.Event) error {
					events = append(events, event)
					if event.Kind == machine.EventRecord {
						if tt.panics {
							panic("panic detail")
						}
						return errors.New("sink detail")
					}
					return nil
				},
			)
			machineErr := assertMachineError(
				t,
				err,
				machine.ErrorKindInternal,
				machine.OperationList,
				"zia",
				"locations",
			)
			if strings.Contains(machineErr.Message, "detail") {
				t.Fatalf("Executor.ReadStream(%s sink) message = %q, want value-free", tt.name, machineErr.Message)
			}
			assertEventKinds(t, events, []machine.EventKind{
				machine.EventStarted, machine.EventRecord, machine.EventFailed,
			})
		})
	}
}

func TestExecutorReadAppliesPostProjectionNarrowing(t *testing.T) {
	loader := &fakeBrowserLoader{records: projectedRecordsFromFields(t,
		map[string]any{"id": "1", "name": "HQ", "country": "US"},
		map[string]any{"id": "2", "name": "Branch", "country": "DE"},
	)}
	catalog := resources.ResourceCatalog{
		testExecutorSpec(resources.ProductZIA, "locations", resources.ListOperations(), "id", "name", "country"),
	}
	fields := []string{"name"}
	filters := []machine.Filter{{Field: "country", Operator: "=", Value: "US"}}
	request := machine.ResourceReadRequest{
		Operation: machine.OperationList,
		Input: machine.ResourceReadInput{
			Product:  "zia",
			Resource: "locations",
			Fields:   fields,
			Filters:  filters,
			Search:   "hq",
		},
	}

	result, err := (machine.Executor{
		Browser:   loader,
		Catalog:   catalog,
		Redaction: redact.ModeStandard,
	}).Read(context.Background(), request)
	if err != nil {
		t.Fatalf("Executor.Read(narrowed list) error = %v, want nil", err)
	}
	assertProjectedResultFields(t, result, []map[string]any{{"name": "HQ"}})
	if !reflect.DeepEqual(fields, []string{"name"}) ||
		!reflect.DeepEqual(filters, []machine.Filter{{Field: "country", Operator: "=", Value: "US"}}) {
		t.Fatalf("Executor.Read mutated caller input slices: fields=%#v filters=%#v", fields, filters)
	}
}

func TestExecutorReadPreservesEmptyResultFamily(t *testing.T) {
	result, err := (machine.Executor{Browser: &fakeBrowserLoader{}}).Read(
		context.Background(),
		machine.ResourceReadRequest{
			Operation: machine.OperationList,
			Input:     machine.ResourceReadInput{Product: "zia", Resource: "locations"},
		},
	)
	if err != nil {
		t.Fatalf("Executor.Read(empty list) error = %v, want nil", err)
	}
	if got := result.Records().Records(); got == nil || len(got) != 0 {
		t.Fatalf("Executor.Read(empty list) records = %#v, want initialized empty collection", got)
	}
}

func TestExecutorReadRejectsNonReadOperationBeforeLoader(t *testing.T) {
	loader := &fakeBrowserLoader{}
	_, err := (machine.Executor{Browser: loader}).Read(
		context.Background(),
		machine.ResourceReadRequest{
			Operation: machine.OperationManifest,
			Input:     machine.ResourceReadInput{Product: "zia", Resource: "locations"},
		},
	)
	assertMachineError(
		t,
		err,
		machine.ErrorKindUnsupportedOperation,
		machine.OperationManifest,
		"zia",
		"locations",
	)
	if len(loader.calls) != 0 {
		t.Fatalf("Executor.Read(manifest) loader calls = %#v, want none", loader.calls)
	}
}

func TestExecutorReadPreservesMachineErrorClassification(t *testing.T) {
	loader := &fakeBrowserLoader{err: errors.New("raw backend detail")}
	_, err := (machine.Executor{Browser: loader}).Read(
		context.Background(),
		machine.ResourceReadRequest{
			Operation: machine.OperationList,
			Input:     machine.ResourceReadInput{Product: "zia", Resource: "locations"},
		},
	)
	machineErr := assertMachineError(
		t,
		err,
		machine.ErrorKindLiveAccessFailed,
		machine.OperationList,
		"zia",
		"locations",
	)
	if strings.Contains(machineErr.Message, "raw backend detail") {
		t.Fatalf("Executor.Read(loader error) message = %q, want sanitized", machineErr.Message)
	}
}

func TestResourceReadResultIsDefensive(t *testing.T) {
	bools := []bool{true, false}
	floats := []float64{1.5, 2.5}
	bytesValue := []byte{1, 2, 3}
	projected := resources.NewProjectedRecordsFromProjectedFields([]map[string]any{{
		"id": "123", "name": "HQ", "ports": []int{80, 443},
		"bools": bools, "floats": floats, "bytes": bytesValue,
	}})
	result := machine.NewResourceReadResult(projected)
	bools[0] = false
	floats[0] = 9.5
	bytesValue[0] = 9

	first := result.Records().Records()
	first[0] = resources.ProjectedRecord{}
	second := result.Records().Records()
	if got := second[0].Fields(); !reflect.DeepEqual(got, map[string]any{
		"id": "123", "name": "HQ", "ports": []int{80, 443},
		"bools": []bool{true, false}, "floats": []float64{1.5, 2.5}, "bytes": []byte{1, 2, 3},
	}) {
		t.Fatalf("ResourceReadResult after returned-slice mutation = %#v, want original fields", got)
	}
	fields := second[0].Fields()
	fields["name"] = "mutated"
	fields["ports"].([]int)[0] = 8080
	fields["bools"].([]bool)[0] = false
	fields["floats"].([]float64)[0] = 9.5
	fields["bytes"].([]byte)[0] = 9
	if got := result.Records().Records()[0].Fields(); !reflect.DeepEqual(got, map[string]any{
		"id": "123", "name": "HQ", "ports": []int{80, 443},
		"bools": []bool{true, false}, "floats": []float64{1.5, 2.5}, "bytes": []byte{1, 2, 3},
	}) {
		t.Fatalf("ResourceReadResult after returned-map mutation = %#v, want original fields", got)
	}
}

func TestTypedEngineValuesRejectDirectJSON(t *testing.T) {
	values := []struct {
		name  string
		value any
	}{
		{
			name: "resource read request",
			value: machine.ResourceReadRequest{
				Operation: machine.OperationList,
				Input:     machine.ResourceReadInput{Product: "zia", Resource: "locations"},
			},
		},
		{
			name: "resource read result",
			value: machine.NewResourceReadResult(
				resources.NewProjectedRecordsFromProjectedFields([]map[string]any{{
					"name": "must-not-serialize",
				}}),
			),
		},
		{
			name: "execution settings",
			value: machine.ExecutionSettings{
				ConfigPath: "/must/not/serialize", Timeout: time.Second,
			},
		},
	}
	for _, tt := range values {
		t.Run(tt.name, func(t *testing.T) {
			body, err := json.Marshal(tt.value)
			if err == nil {
				t.Fatalf("json.Marshal(%s) error = nil; body = %s, want no wire format", tt.name, body)
			}
			if strings.Contains(string(body), "must-not-serialize") ||
				strings.Contains(string(body), "/must/not/serialize") {
				t.Fatalf("json.Marshal(%s) body = %q, want no value bytes", tt.name, body)
			}
		})
	}

	var request machine.ResourceReadRequest
	if err := json.Unmarshal([]byte(`{"Operation":"list"}`), &request); err == nil {
		t.Fatalf("json.Unmarshal(ResourceReadRequest) error = nil; request = %#v, want no wire format", request)
	}
	var result machine.ResourceReadResult
	if err := json.Unmarshal([]byte(`{"records":[]}`), &result); err == nil {
		t.Fatalf("json.Unmarshal(ResourceReadResult) error = nil; result = %#v, want no wire format", result)
	}
	var settings machine.ExecutionSettings
	if err := json.Unmarshal([]byte(`{"Profile":"default"}`), &settings); err == nil {
		t.Fatalf("json.Unmarshal(ExecutionSettings) error = nil; settings = %#v, want no wire format", settings)
	}
}

func assertProjectedResultFields(
	t *testing.T,
	result machine.ResourceReadResult,
	want []map[string]any,
) {
	t.Helper()
	records := result.Records().Records()
	got := make([]map[string]any, len(records))
	for i, record := range records {
		got[i] = record.Fields()
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ResourceReadResult records = %#v, want %#v", got, want)
	}
}
