package machine_test

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/machine"
	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

func TestExecutorExecuteListCallsLoaderAndReturnsProjectedRecords(t *testing.T) {
	loader := &fakeBrowserLoader{
		records: projectedRecordsFromFields(t,
			map[string]any{"id": "123", "name": "HQ"},
			map[string]any{"id": "456", "name": "Branch"},
		),
	}
	executor := machine.Executor{Browser: loader}
	req := machine.Request{
		RequestID:  "req-1",
		Capability: machine.CapabilityResourcesRead,
		Operation:  machine.OperationList,
		Input: &machine.Input{
			Product:  "zia",
			Resource: "locations",
		},
		Meta: &machine.Meta{Version: "client.v1"},
	}

	got, err := executor.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Executor.Execute(list request) error = %v, want nil", err)
	}
	wantCalls := []string{"list:zia/locations"}
	if !reflect.DeepEqual(loader.calls, wantCalls) {
		t.Fatalf("Executor.Execute(list request) loader calls = %#v, want %#v", loader.calls, wantCalls)
	}
	wantRecords := []map[string]any{
		{"id": "123", "name": "HQ"},
		{"id": "456", "name": "Branch"},
	}
	if !reflect.DeepEqual(got.Records, wantRecords) {
		t.Fatalf("Executor.Execute(list request).Records = %#v, want %#v", got.Records, wantRecords)
	}
	assertResponseEnvelope(t, got, req, 2)
	if got.Meta.Version != "" {
		t.Fatalf("Executor.Execute(list request).Meta.Version = %q, want empty server-generated metadata", got.Meta.Version)
	}
}

func TestExecutorExecuteShowCallsLoaderAndReturnsProjectedRecords(t *testing.T) {
	loader := &fakeBrowserLoader{
		records: projectedRecordsFromFields(t,
			map[string]any{"id": "settings", "name": "Advanced Settings"},
		),
	}
	executor := machine.Executor{Browser: loader}
	req := machine.Request{
		RequestID:  "req-show",
		Capability: machine.CapabilityResourcesRead,
		Operation:  machine.OperationShow,
		Input: &machine.Input{
			Product:  "zia",
			Resource: "advanced-settings",
		},
	}

	got, err := executor.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Executor.Execute(show request) error = %v, want nil", err)
	}
	wantCalls := []string{"show:zia/advanced-settings"}
	if !reflect.DeepEqual(loader.calls, wantCalls) {
		t.Fatalf("Executor.Execute(show request) loader calls = %#v, want %#v", loader.calls, wantCalls)
	}
	wantRecords := []map[string]any{
		{"id": "settings", "name": "Advanced Settings"},
	}
	if !reflect.DeepEqual(got.Records, wantRecords) {
		t.Fatalf("Executor.Execute(show request).Records = %#v, want %#v", got.Records, wantRecords)
	}
	assertResponseEnvelope(t, got, req, 1)
}

func TestExecutorExecuteGetCallsGetterAndReturnsProjectedRecord(t *testing.T) {
	loader := &fakeBrowserLoader{
		getRecords: projectedRecordsFromFields(t,
			map[string]any{"id": "123", "name": "HQ"},
		),
	}
	executor := machine.Executor{Browser: loader}
	req := machine.Request{
		RequestID:  "req-get",
		Capability: machine.CapabilityResourcesRead,
		Operation:  machine.OperationGet,
		Input: &machine.Input{
			Product:  "zia",
			Resource: "locations",
			RecordID: "123",
		},
	}

	got, err := executor.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Executor.Execute(get request) error = %v, want nil", err)
	}
	wantCalls := []string{"get:zia/locations/123"}
	if !reflect.DeepEqual(loader.calls, wantCalls) {
		t.Fatalf("Executor.Execute(get request) loader calls = %#v, want %#v", loader.calls, wantCalls)
	}
	wantRecords := []map[string]any{
		{"id": "123", "name": "HQ"},
	}
	if !reflect.DeepEqual(got.Records, wantRecords) {
		t.Fatalf("Executor.Execute(get request).Records = %#v, want %#v", got.Records, wantRecords)
	}
	assertResponseEnvelope(t, got, req, 1)
}

func TestExecutorRejectsUnsupportedCapabilityBeforeLoader(t *testing.T) {
	const capabilityCanary = "\x1b[31mcapability-canary-raw\x1b[0m"
	loader := &fakeBrowserLoader{}
	executor := machine.Executor{Browser: loader}
	req := machine.Request{
		RequestID:  "req-unsupported-capability",
		Capability: capabilityCanary,
		Operation:  machine.OperationList,
		Input:      &machine.Input{Product: "zia", Resource: "locations"},
	}

	got, err := executor.Execute(context.Background(), req)
	if err == nil {
		t.Fatal("Executor.Execute(unsupported capability) error = nil, want MachineError")
	}
	assertMachineError(t, err, machine.ErrorKindUnsupportedCapability, machine.OperationList, "zia", "locations")
	assertResponseError(t, got, machine.ErrorKindUnsupportedCapability)
	if got.Error.Message != "unsupported capability" {
		t.Fatalf("Executor.Execute(unsupported capability) message = %q, want fixed diagnostic", got.Error.Message)
	}
	assertNoValidationCanary(t, err.Error(), capabilityCanary)
	if len(loader.calls) != 0 {
		t.Fatalf("Executor.Execute(unsupported capability) loader calls = %#v, want none", loader.calls)
	}
}

func TestExecutorRejectsUnsupportedOperationBeforeLoader(t *testing.T) {
	const operationCanary = "\x1b[31moperation-canary-raw\x1b[0m"
	loader := &fakeBrowserLoader{}
	executor := machine.Executor{Browser: loader}
	req := machine.Request{
		RequestID:  "req-delete",
		Capability: machine.CapabilityResourcesRead,
		Operation:  machine.Operation(operationCanary),
		Input:      &machine.Input{Product: "zia", Resource: "locations"},
	}

	got, err := executor.Execute(context.Background(), req)
	if err == nil {
		t.Fatal("Executor.Execute(delete request) error = nil, want MachineError")
	}
	assertMachineError(t, err, machine.ErrorKindUnsupportedOperation, machine.Operation(operationCanary), "zia", "locations")
	assertResponseError(t, got, machine.ErrorKindUnsupportedOperation)
	if got.Error.Message != "unsupported operation for resources.read" {
		t.Fatalf("Executor.Execute(delete request) message = %q, want fixed diagnostic", got.Error.Message)
	}
	assertNoValidationCanary(t, err.Error(), operationCanary)
	if len(loader.calls) != 0 {
		t.Fatalf("Executor.Execute(delete request) loader calls = %#v, want none", loader.calls)
	}
}

func TestExecutorExecuteManifestReturnsCatalogManifestWithoutLoader(t *testing.T) {
	loader := &fakeBrowserLoader{}
	catalog := resources.ResourceCatalog{
		testExecutorSpec(resources.ProductZIA, "locations", resources.ReadOperations(), "id", "name"),
	}
	executor := machine.Executor{Browser: loader, Catalog: catalog}
	req := machine.Request{
		RequestID: "req-manifest",
		Operation: machine.OperationManifest,
	}

	got, err := executor.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Executor.Execute(manifest request) error = %v, want nil", err)
	}
	if got.Manifest == nil {
		t.Fatalf("Executor.Execute(manifest request).Manifest = nil, want manifest")
	}
	if got.Meta == nil || got.Meta.Count != 1 || !got.Meta.ReadOnly {
		t.Fatalf("Executor.Execute(manifest request).Meta = %#v, want read-only count 1", got.Meta)
	}
	if len(loader.calls) != 0 {
		t.Fatalf("Executor.Execute(manifest request) loader calls = %#v, want none", loader.calls)
	}
}

func TestExecutorRejectsMissingInputBeforeLoader(t *testing.T) {
	tests := []struct {
		name         string
		input        *machine.Input
		wantMissing  []string
		wantProduct  string
		wantResource string
	}{
		{
			name:        "missing_input",
			input:       nil,
			wantMissing: []string{"input"},
		},
		{
			name:         "missing_product",
			input:        &machine.Input{Resource: "locations"},
			wantMissing:  []string{"input.product"},
			wantResource: "locations",
		},
		{
			name:        "missing_resource",
			input:       &machine.Input{Product: "zia"},
			wantMissing: []string{"input.resource"},
			wantProduct: "zia",
		},
		{
			name:        "missing_product_and_resource",
			input:       &machine.Input{},
			wantMissing: []string{"input.product", "input.resource"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loader := &fakeBrowserLoader{}
			executor := machine.Executor{Browser: loader}
			req := machine.Request{
				RequestID:  "req-" + tt.name,
				Capability: machine.CapabilityResourcesRead,
				Operation:  machine.OperationList,
				Input:      tt.input,
			}

			got, err := executor.Execute(context.Background(), req)
			if err == nil {
				t.Fatalf("Executor.Execute(%s) error = nil, want MachineError", tt.name)
			}
			machineErr := assertMachineError(
				t,
				err,
				machine.ErrorKindUsage,
				machine.OperationList,
				tt.wantProduct,
				tt.wantResource,
			)
			if !reflect.DeepEqual(machineErr.Missing, tt.wantMissing) {
				t.Fatalf("Executor.Execute(%s) missing = %#v, want %#v", tt.name, machineErr.Missing, tt.wantMissing)
			}
			assertResponseError(t, got, machine.ErrorKindUsage)
			if len(loader.calls) != 0 {
				t.Fatalf("Executor.Execute(%s) loader calls = %#v, want none", tt.name, loader.calls)
			}
		})
	}
}

func TestExecutorRejectsGetMissingRecordIDBeforeLoader(t *testing.T) {
	tests := []string{"", " \t "}
	for _, recordID := range tests {
		t.Run("record_id="+recordID, func(t *testing.T) {
			loader := &fakeBrowserLoader{}
			executor := machine.Executor{Browser: loader}
			req := machine.Request{
				RequestID:  "req-get-missing-record-id",
				Capability: machine.CapabilityResourcesRead,
				Operation:  machine.OperationGet,
				Input: &machine.Input{
					Product:  "zia",
					Resource: "locations",
					RecordID: recordID,
				},
			}

			got, err := executor.Execute(context.Background(), req)
			if err == nil {
				t.Fatalf("Executor.Execute(get request record_id=%q) error = nil, want MachineError", recordID)
			}
			machineErr := assertMachineError(
				t,
				err,
				machine.ErrorKindUsage,
				machine.OperationGet,
				"zia",
				"locations",
			)
			wantMissing := []string{"input.record_id"}
			if !reflect.DeepEqual(machineErr.Missing, wantMissing) {
				t.Fatalf("Executor.Execute(get request record_id=%q) missing = %#v, want %#v",
					recordID, machineErr.Missing, wantMissing)
			}
			assertResponseError(t, got, machine.ErrorKindUsage)
			if len(loader.calls) != 0 {
				t.Fatalf("Executor.Execute(get request record_id=%q) loader calls = %#v, want none", recordID, loader.calls)
			}
		})
	}
}

func TestExecutorMapsLoaderErrorToSanitizedMachineError(t *testing.T) {
	loader := &fakeBrowserLoader{
		err: errors.New("raw SDK token leaked-token-123 transport failure"),
	}
	executor := machine.Executor{Browser: loader}
	req := machine.Request{
		RequestID:  "req-loader-error",
		Capability: machine.CapabilityResourcesRead,
		Operation:  machine.OperationList,
		Input:      &machine.Input{Product: "zia", Resource: "locations"},
	}

	got, err := executor.Execute(context.Background(), req)
	if err == nil {
		t.Fatal("Executor.Execute(loader error) error = nil, want MachineError")
	}
	machineErr := assertMachineError(t, err, machine.ErrorKindLiveAccessFailed, machine.OperationList, "zia", "locations")
	if strings.Contains(machineErr.Message, "leaked-token-123") || strings.Contains(machineErr.Message, "SDK") {
		t.Fatalf("Executor.Execute(loader error) message = %q, want sanitized message", machineErr.Message)
	}
	assertResponseError(t, got, machine.ErrorKindLiveAccessFailed)
	wantCalls := []string{"list:zia/locations"}
	if !reflect.DeepEqual(loader.calls, wantCalls) {
		t.Fatalf("Executor.Execute(loader error) loader calls = %#v, want %#v", loader.calls, wantCalls)
	}
}

func TestExecutorMapsKnownLoaderErrorsToStableMachineKinds(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantKind string
	}{
		{name: "unknown_resource", err: resources.ErrUnknownResource, wantKind: machine.ErrorKindUnknownResource},
		{name: "record_not_found", err: resources.ErrRecordNotFound, wantKind: machine.ErrorKindNotFound},
		{name: "unsupported_load", err: resources.ErrUnsupportedLoad, wantKind: machine.ErrorKindUnsupportedOperation},
		{name: "context_canceled", err: context.Canceled, wantKind: machine.ErrorKindCanceled},
		{name: "deadline_exceeded", err: context.DeadlineExceeded, wantKind: machine.ErrorKindDeadlineExceeded},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loader := &fakeBrowserLoader{err: tt.err}
			executor := machine.Executor{Browser: loader}
			req := machine.Request{
				RequestID:  "req-" + tt.name,
				Capability: machine.CapabilityResourcesRead,
				Operation:  machine.OperationList,
				Input:      &machine.Input{Product: "zia", Resource: "locations"},
			}

			got, err := executor.Execute(context.Background(), req)
			if err == nil {
				t.Fatalf("Executor.Execute(%s loader error) error = nil, want MachineError", tt.name)
			}
			assertMachineError(t, err, tt.wantKind, machine.OperationList, "zia", "locations")
			assertResponseError(t, got, tt.wantKind)
		})
	}
}

func TestExecutorGetRecordNotFoundReturnsNotFound(t *testing.T) {
	loader := &fakeBrowserLoader{
		getErr: fmt.Errorf("%w: loc-123 raw backend body", resources.ErrRecordNotFound),
	}
	executor := machine.Executor{Browser: loader}
	req := machine.Request{
		RequestID:  "req-not-found",
		Capability: machine.CapabilityResourcesRead,
		Operation:  machine.OperationGet,
		Input:      &machine.Input{Product: "zia", Resource: "locations", RecordID: "loc-123"},
	}

	got, err := executor.Execute(context.Background(), req)
	if err == nil {
		t.Fatal("Executor.Execute(get not found) error = nil, want MachineError")
	}
	machineErr := assertMachineError(t, err, machine.ErrorKindNotFound, machine.OperationGet, "zia", "locations")
	if machineErr.Message != "record not found" {
		t.Fatalf("MachineError.Message = %q, want sanitized not-found message", machineErr.Message)
	}
	if strings.Contains(machineErr.Message, "loc-123") || strings.Contains(machineErr.Message, "raw backend body") {
		t.Fatalf("MachineError.Message = %q, want no record ID or backend detail", machineErr.Message)
	}
	assertResponseError(t, got, machine.ErrorKindNotFound)
	wantCalls := []string{"get:zia/locations/loc-123"}
	if !reflect.DeepEqual(loader.calls, wantCalls) {
		t.Fatalf("Executor.Execute(get not found) loader calls = %#v, want %#v", loader.calls, wantCalls)
	}
}

func TestExecutorGetInvalidResourceIDReturnsSanitizedUsageError(t *testing.T) {
	const invalidID = "not-a-number"
	loader := &fakeBrowserLoader{
		getErr: fmt.Errorf("%w: %q", resources.ErrInvalidResourceID, invalidID),
	}
	executor := machine.Executor{Browser: loader}
	req := machine.Request{
		RequestID:  "req-invalid-id",
		Capability: machine.CapabilityResourcesRead,
		Operation:  machine.OperationGet,
		Input:      &machine.Input{Product: "zia", Resource: "locations", RecordID: invalidID},
	}

	got, err := executor.Execute(context.Background(), req)
	if err == nil {
		t.Fatal("Executor.Execute(invalid resource ID) error = nil, want MachineError")
	}
	machineErr := assertMachineError(t, err, machine.ErrorKindInvalidResourceID, machine.OperationGet, "zia", "locations")
	if machineErr.Message != "invalid resource ID" {
		t.Fatalf("MachineError.Message = %q, want sanitized invalid-resource-ID message", machineErr.Message)
	}
	if strings.Contains(machineErr.Message, invalidID) {
		t.Fatalf("MachineError.Message = %q, want no raw resource ID", machineErr.Message)
	}
	if !errors.Is(err, resources.ErrInvalidResourceID) {
		t.Fatalf("Executor.Execute(invalid resource ID) error = %v, want sanitized sentinel bridge", err)
	}
	assertResponseError(t, got, machine.ErrorKindInvalidResourceID)
}

func TestExecutorAppliesFieldsFiltersAndSearchAfterProjection(t *testing.T) {
	loader := &fakeBrowserLoader{
		records: projectedRecordsFromFields(t,
			map[string]any{"id": "1", "name": "HQ", "country": "US"},
			map[string]any{"id": "2", "name": "Branch East", "country": "US"},
			map[string]any{"id": "3", "name": "Branch West", "country": "DE"},
		),
	}
	executor := machine.Executor{
		Browser: loader,
		Catalog: resources.ResourceCatalog{
			testExecutorSpec(resources.ProductZIA, "locations", resources.ReadOperations(), "id", "name", "country"),
		},
		Redaction: redact.ModeStandard,
	}
	req := machine.Request{
		RequestID:  "req-narrow",
		Capability: machine.CapabilityResourcesRead,
		Operation:  machine.OperationList,
		Input: &machine.Input{
			Product:  "zia",
			Resource: "locations",
			Fields:   []string{"name"},
			Filters: []machine.Filter{
				{Field: "country", Operator: "=", Value: "DE"},
				{Field: "name", Operator: "~", Value: "branch"},
			},
			Search: "west",
		},
	}

	got, err := executor.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Executor.Execute(narrowed list request) error = %v, want nil", err)
	}
	wantRecords := []map[string]any{{"name": "Branch West"}}
	if !reflect.DeepEqual(got.Records, wantRecords) {
		t.Fatalf("Executor.Execute(narrowed list request).Records = %#v, want %#v", got.Records, wantRecords)
	}
}

func TestExecutorRejectsUnsupportedInputSemanticsBeforeLoader(t *testing.T) {
	tests := []struct {
		name  string
		input *machine.Input
		op    machine.Operation
	}{
		{
			name:  "filter_on_get",
			op:    machine.OperationGet,
			input: &machine.Input{Product: "zia", Resource: "locations", RecordID: "123", Filters: []machine.Filter{{Field: "name", Operator: "=", Value: "HQ"}}},
		},
		{
			name:  "search_on_show",
			op:    machine.OperationShow,
			input: &machine.Input{Product: "zia", Resource: "advanced-settings", Search: "enabled"},
		},
		{
			name:  "invalid_filter_operator",
			op:    machine.OperationList,
			input: &machine.Input{Product: "zia", Resource: "locations", Filters: []machine.Filter{{Field: "name", Operator: "!=", Value: "HQ"}}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loader := &fakeBrowserLoader{}
			executor := machine.Executor{Browser: loader}
			req := machine.Request{
				RequestID:  "req-" + tt.name,
				Capability: machine.CapabilityResourcesRead,
				Operation:  tt.op,
				Input:      tt.input,
			}

			got, err := executor.Execute(context.Background(), req)
			if err == nil {
				t.Fatalf("Executor.Execute(%s) error = nil, want usage MachineError", tt.name)
			}
			assertMachineError(t, err, machine.ErrorKindUsage, tt.op, tt.input.Product, tt.input.Resource)
			assertResponseError(t, got, machine.ErrorKindUsage)
			if len(loader.calls) != 0 {
				t.Fatalf("Executor.Execute(%s) loader calls = %#v, want none", tt.name, loader.calls)
			}
		})
	}
}

func TestExecutorRejectsInvalidFilterOperatorWithoutReflectingClientValue(t *testing.T) {
	const operatorCanary = "\x1b[31moperator-canary-raw\x1b[0m"
	newExecutor := func(loader *fakeBrowserLoader) machine.Executor {
		return machine.Executor{
			Browser: loader,
			Catalog: resources.ResourceCatalog{
				testExecutorSpec(resources.ProductZIA, "locations", resources.ReadOperations(), "id", "name"),
			},
		}
	}
	newRequest := func() machine.Request {
		return machine.Request{
			RequestID:  "req-invalid-filter-operator",
			Capability: machine.CapabilityResourcesRead,
			Operation:  machine.OperationList,
			Input: &machine.Input{
				Product:  "zia",
				Resource: "locations",
				Filters: []machine.Filter{{
					Field:    "name",
					Operator: operatorCanary,
					Value:    "HQ",
				}},
			},
		}
	}
	const wantMessage = `input.filters.operator is not supported; use "=", "exact", "~", or "contains"`

	t.Run("execute", func(t *testing.T) {
		loader := &fakeBrowserLoader{}
		got, err := newExecutor(loader).Execute(context.Background(), newRequest())
		if err == nil {
			t.Fatal("Executor.Execute(invalid filter operator) error = nil, want usage MachineError")
		}
		assertMachineError(t, err, machine.ErrorKindUsage, machine.OperationList, "zia", "locations")
		assertResponseError(t, got, machine.ErrorKindUsage)
		if got.Error.Message != wantMessage {
			t.Fatalf("Executor.Execute(invalid filter operator) message = %q, want fixed diagnostic", got.Error.Message)
		}
		assertNoValidationCanary(t, err.Error(), operatorCanary)
		if len(loader.calls) != 0 {
			t.Fatalf("Executor.Execute(invalid filter operator) loader calls = %#v, want none", loader.calls)
		}
	})

	t.Run("execute_stream_terminal", func(t *testing.T) {
		loader := &fakeBrowserLoader{}
		var events []machine.Event
		err := newExecutor(loader).ExecuteStream(context.Background(), newRequest(), func(event machine.Event) error {
			events = append(events, event)
			return nil
		})
		if err == nil {
			t.Fatal("Executor.ExecuteStream(invalid filter operator) error = nil, want usage MachineError")
		}
		assertMachineError(t, err, machine.ErrorKindUsage, machine.OperationList, "zia", "locations")
		if len(events) != 2 || events[1].Kind != machine.EventFailed || events[1].Err == nil {
			t.Fatalf("Executor.ExecuteStream(invalid filter operator) events = %#v, want started plus failed terminal", events)
		}
		if events[1].Err.Message != wantMessage {
			t.Fatalf("Executor.ExecuteStream(invalid filter operator) terminal message = %q, want fixed diagnostic", events[1].Err.Message)
		}
		assertNoValidationCanary(t, events[1].Err.Message, operatorCanary)
		if len(loader.calls) != 0 {
			t.Fatalf("Executor.ExecuteStream(invalid filter operator) loader calls = %#v, want none", loader.calls)
		}
	})

	t.Run("typed_read", func(t *testing.T) {
		loader := &fakeBrowserLoader{}
		request := machine.ResourceReadRequest{
			RequestID: "req-invalid-filter-operator",
			Operation: machine.OperationList,
			Input: machine.ResourceReadInput{
				Product:  "zia",
				Resource: "locations",
				Filters: []machine.Filter{{
					Field:    "name",
					Operator: operatorCanary,
					Value:    "HQ",
				}},
			},
		}
		_, err := newExecutor(loader).Read(context.Background(), request)
		if err == nil {
			t.Fatal("Executor.Read(invalid filter operator) error = nil, want usage MachineError")
		}
		assertMachineError(t, err, machine.ErrorKindUsage, machine.OperationList, "zia", "locations")
		if err.Error() != wantMessage {
			t.Fatalf("Executor.Read(invalid filter operator) message = %q, want fixed diagnostic", err.Error())
		}
		assertNoValidationCanary(t, err.Error(), operatorCanary)
		if len(loader.calls) != 0 {
			t.Fatalf("Executor.Read(invalid filter operator) loader calls = %#v, want none", loader.calls)
		}
	})
}

func assertNoValidationCanary(t *testing.T, got, canary string) {
	t.Helper()
	if strings.Contains(got, canary) || strings.Contains(got, "operator-canary-raw") || strings.ContainsAny(got, "\x1b") {
		t.Fatalf("validation error = %q, want no client operator value or ANSI escape", got)
	}
}

func TestExecutorDoesNotEchoClientSuppliedMeta(t *testing.T) {
	loader := &fakeBrowserLoader{
		records: projectedRecordsFromFields(t, map[string]any{"id": "123", "name": "HQ"}),
	}
	executor := machine.Executor{Browser: loader}
	req := machine.Request{
		RequestID:  "req-meta",
		Capability: machine.CapabilityResourcesRead,
		Operation:  machine.OperationList,
		Input:      &machine.Input{Product: "zia", Resource: "locations"},
		Meta: &machine.Meta{
			Version:     "client",
			RequestID:   "spoofed",
			GeneratedAt: "yesterday",
			Product:     "zpa",
			Resource:    "server-groups",
			Shape:       "singleton",
			GetKey:      "externalId",
			ReadOnly:    false,
			Count:       99,
		},
	}

	got, err := executor.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Executor.Execute(client meta request) error = %v, want nil", err)
	}
	if got.Meta == nil {
		t.Fatal("Executor.Execute(client meta request).Meta = nil, want server metadata")
	}
	if got.Meta.RequestID != "req-meta" ||
		got.Meta.Product != "zia" ||
		got.Meta.Resource != "locations" ||
		!got.Meta.ReadOnly ||
		got.Meta.Count != 1 {
		t.Fatalf("Executor.Execute(client meta request).Meta = %#v, want server-generated values", got.Meta)
	}
	if got.Meta.Version != "" || got.Meta.GeneratedAt != "" || got.Meta.Shape != "" || got.Meta.GetKey != "" {
		t.Fatalf("Executor.Execute(client meta request).Meta = %#v, want no echoed client metadata", got.Meta)
	}
}

func TestExecutorUnknownFieldSelectionIsUsageError(t *testing.T) {
	loader := &fakeBrowserLoader{
		records: projectedRecordsFromFields(t, map[string]any{"id": "123", "name": "HQ"}),
	}
	executor := machine.Executor{
		Browser: loader,
		Catalog: resources.ResourceCatalog{
			testExecutorSpec(resources.ProductZIA, "locations", resources.ReadOperations(), "id", "name"),
		},
	}
	req := machine.Request{
		RequestID:  "req-unknown-field",
		Capability: machine.CapabilityResourcesRead,
		Operation:  machine.OperationList,
		Input: &machine.Input{
			Product:  "zia",
			Resource: "locations",
			Fields:   []string{"Authorization: Bearer machine-field-canary-abcdefghijklmnopqrstuvwxyz"},
		},
	}

	got, err := executor.Execute(context.Background(), req)
	if err == nil {
		t.Fatal("Executor.Execute(unknown field request) error = nil, want usage MachineError")
	}
	assertMachineError(t, err, machine.ErrorKindUsage, machine.OperationList, "zia", "locations")
	assertResponseError(t, got, machine.ErrorKindUsage)
	if len(loader.calls) != 0 {
		t.Fatalf("Executor.Execute(unknown field request) loader calls = %#v, want none", loader.calls)
	}
	if strings.Contains(err.Error(), "machine-field-canary-abcdefghijklmnopqrstuvwxyz") {
		t.Fatalf("Executor.Execute(unknown field request) error = %q, want canary redacted", err.Error())
	}
	if !strings.Contains(err.Error(), "<REDACTED:SECRET>") {
		t.Fatalf("Executor.Execute(unknown field request) error = %q, want redaction marker", err.Error())
	}
}

func TestExecutorUnknownFilterIsUsageErrorBeforeLoader(t *testing.T) {
	loader := &fakeBrowserLoader{
		records: projectedRecordsFromFields(t, map[string]any{"id": "123", "name": "HQ"}),
	}
	executor := machine.Executor{
		Browser: loader,
		Catalog: resources.ResourceCatalog{
			testExecutorSpec(resources.ProductZIA, "locations", resources.ReadOperations(), "id", "name"),
		},
	}
	req := machine.Request{
		RequestID:  "req-unknown-filter",
		Capability: machine.CapabilityResourcesRead,
		Operation:  machine.OperationList,
		Input: &machine.Input{
			Product:  "zia",
			Resource: "locations",
			Filters: []machine.Filter{{
				Field:    "Authorization: Bearer machine-filter-canary-abcdefghijklmnopqrstuvwxyz",
				Operator: "=",
				Value:    "HQ",
			}},
		},
	}

	got, err := executor.Execute(context.Background(), req)
	if err == nil {
		t.Fatal("Executor.Execute(unknown filter request) error = nil, want usage MachineError")
	}
	assertMachineError(t, err, machine.ErrorKindUsage, machine.OperationList, "zia", "locations")
	assertResponseError(t, got, machine.ErrorKindUsage)
	if len(loader.calls) != 0 {
		t.Fatalf("Executor.Execute(unknown filter request) loader calls = %#v, want none", loader.calls)
	}
	if strings.Contains(err.Error(), "machine-filter-canary-abcdefghijklmnopqrstuvwxyz") {
		t.Fatalf("Executor.Execute(unknown filter request) error = %q, want canary redacted", err.Error())
	}
	if !strings.Contains(err.Error(), "<REDACTED:SECRET>") {
		t.Fatalf("Executor.Execute(unknown filter request) error = %q, want redaction marker", err.Error())
	}
}

func TestExecutorRejectsMissingLoader(t *testing.T) {
	executor := machine.Executor{}
	req := machine.Request{
		RequestID:  "req-no-loader",
		Capability: machine.CapabilityResourcesRead,
		Operation:  machine.OperationList,
		Input:      &machine.Input{Product: "zia", Resource: "locations"},
	}

	got, err := executor.Execute(context.Background(), req)
	if err == nil {
		t.Fatal("Executor.Execute(missing loader) error = nil, want MachineError")
	}
	assertMachineError(t, err, machine.ErrorKindInternal, machine.OperationList, "zia", "locations")
	assertResponseError(t, got, machine.ErrorKindInternal)
}

func TestExecutorRejectsGetWhenLoaderDoesNotImplementGetter(t *testing.T) {
	loader := &projectedOnlyLoader{}
	executor := machine.Executor{Browser: loader}
	req := machine.Request{
		RequestID:  "req-get-no-getter",
		Capability: machine.CapabilityResourcesRead,
		Operation:  machine.OperationGet,
		Input:      &machine.Input{Product: "zia", Resource: "locations", RecordID: "123"},
	}

	got, err := executor.Execute(context.Background(), req)
	if err == nil {
		t.Fatal("Executor.Execute(get request without getter) error = nil, want MachineError")
	}
	assertMachineError(t, err, machine.ErrorKindInternal, machine.OperationGet, "zia", "locations")
	assertResponseError(t, got, machine.ErrorKindInternal)
	if len(loader.calls) != 0 {
		t.Fatalf("Executor.Execute(get request without getter) loader calls = %#v, want none", loader.calls)
	}
}

type fakeBrowserLoader struct {
	records    resources.ProjectedRecords
	getRecords resources.ProjectedRecords
	err        error
	getErr     error
	calls      []string
}

func (l *fakeBrowserLoader) LoadProjected(
	_ context.Context,
	product string,
	resource string,
) (resources.ProjectedRecords, error) {
	return l.ListProjected(context.Background(), product, resource)
}

func (l *fakeBrowserLoader) ListProjected(
	_ context.Context,
	product string,
	resource string,
) (resources.ProjectedRecords, error) {
	l.calls = append(l.calls, "list:"+product+"/"+resource)
	if l.err != nil {
		return resources.ProjectedRecords{}, l.err
	}
	return l.records, nil
}

func (l *fakeBrowserLoader) ShowProjected(
	_ context.Context,
	product string,
	resource string,
) (resources.ProjectedRecords, error) {
	l.calls = append(l.calls, "show:"+product+"/"+resource)
	if l.err != nil {
		return resources.ProjectedRecords{}, l.err
	}
	return l.records, nil
}

func (l *fakeBrowserLoader) LoadProjectedByID(
	_ context.Context,
	product string,
	resource string,
	id string,
) (resources.ProjectedRecords, error) {
	return l.GetProjectedByID(context.Background(), product, resource, id)
}

func (l *fakeBrowserLoader) GetProjectedByID(
	_ context.Context,
	product string,
	resource string,
	id string,
) (resources.ProjectedRecords, error) {
	l.calls = append(l.calls, "get:"+product+"/"+resource+"/"+id)
	if l.getErr != nil {
		return resources.ProjectedRecords{}, l.getErr
	}
	return l.getRecords, nil
}

type projectedOnlyLoader struct {
	calls []string
}

func (l *projectedOnlyLoader) ListProjected(
	_ context.Context,
	product string,
	resource string,
) (resources.ProjectedRecords, error) {
	l.calls = append(l.calls, "list:"+product+"/"+resource)
	return resources.ProjectedRecords{}, nil
}

func (l *projectedOnlyLoader) ShowProjected(
	_ context.Context,
	product string,
	resource string,
) (resources.ProjectedRecords, error) {
	l.calls = append(l.calls, "show:"+product+"/"+resource)
	return resources.ProjectedRecords{}, nil
}

func projectedRecordsFromFields(t *testing.T, rows ...map[string]any) resources.ProjectedRecords {
	t.Helper()
	fieldSet := map[string]bool{}
	for _, row := range rows {
		for key := range row {
			fieldSet[key] = true
		}
	}
	fields := make([]resources.FieldSpec, 0, len(fieldSet))
	for key := range fieldSet {
		fields = append(fields, resources.FieldSpec{
			Name:           key,
			Classification: resources.ClassPublicProjectData,
			AllowedModes:   []redact.Mode{redact.ModeStandard},
		})
	}
	spec := resources.ResourceSpec{
		Product:    resources.ProductZIA,
		Name:       "test-resource",
		Operations: resources.ListOperations(),
		Fields:     fields,
	}
	source := make([]resources.SourceRecord, 0, len(rows))
	for _, row := range rows {
		source = append(source, resources.NewSourceRecord(row))
	}
	projected, _, err := resources.ProjectRecordsAndVerify(spec, redact.ModeStandard, source)
	if err != nil {
		t.Fatalf("ProjectRecordsAndVerify(test resource, rows=%#v) error = %v, want nil", rows, err)
	}
	return projected
}

func testExecutorSpec(
	product resources.Product,
	name string,
	operations []resources.Operation,
	fields ...string,
) resources.ResourceSpec {
	fieldSpecs := make([]resources.FieldSpec, len(fields))
	for i, field := range fields {
		fieldSpecs[i] = resources.FieldSpec{
			Name:           field,
			Classification: resources.ClassPublicProjectData,
			AllowedModes:   []redact.Mode{redact.ModeStandard},
		}
	}
	return resources.ResourceSpec{
		Product:    product,
		Name:       name,
		Operations: operations,
		Fields:     fieldSpecs,
	}
}

func assertResponseEnvelope(t *testing.T, got machine.Response, req machine.Request, wantCount int) {
	t.Helper()
	if got.RequestID != req.RequestID || got.Capability != req.Capability || got.Operation != req.Operation {
		t.Fatalf("Executor.Execute(%#v) envelope = request_id:%q capability:%q operation:%q, want request_id:%q capability:%q operation:%q",
			req, got.RequestID, got.Capability, got.Operation, req.RequestID, req.Capability, req.Operation)
	}
	if got.Error != nil {
		t.Fatalf("Executor.Execute(%#v).Error = %#v, want nil", req, got.Error)
	}
	if got.Meta == nil {
		t.Fatalf("Executor.Execute(%#v).Meta = nil, want metadata", req)
	}
	if got.Meta.RequestID != req.RequestID ||
		got.Meta.Product != req.Input.Product ||
		got.Meta.Resource != req.Input.Resource ||
		!got.Meta.ReadOnly ||
		got.Meta.Count != wantCount {
		t.Fatalf("Executor.Execute(%#v).Meta = %#v, want request/product/resource/read_only/count", req, got.Meta)
	}
}

func assertMachineError(
	t *testing.T,
	err error,
	wantKind string,
	wantOperation machine.Operation,
	wantProduct string,
	wantResource string,
) *machine.MachineError {
	t.Helper()
	var machineErr *machine.MachineError
	if !errors.As(err, &machineErr) {
		t.Fatalf("Executor.Execute error = %T %v, want *machine.MachineError", err, err)
	}
	if machineErr.Kind != wantKind ||
		machineErr.Operation != wantOperation ||
		machineErr.Product != wantProduct ||
		machineErr.Resource != wantResource {
		t.Fatalf("MachineError = %#v, want kind:%q operation:%q product:%q resource:%q",
			machineErr, wantKind, wantOperation, wantProduct, wantResource)
	}
	return machineErr
}

func assertResponseError(t *testing.T, got machine.Response, wantKind string) {
	t.Helper()
	if got.Error == nil {
		t.Fatalf("Executor.Execute error response = %#v, want MachineError", got)
	}
	if got.Error.Kind != wantKind {
		t.Fatalf("Executor.Execute response error kind = %q, want %q", got.Error.Kind, wantKind)
	}
}
