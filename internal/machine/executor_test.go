package machine_test

import (
	"context"
	"errors"
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
	loader := &fakeBrowserLoader{}
	executor := machine.Executor{Browser: loader}
	req := machine.Request{
		RequestID:  "req-unsupported-capability",
		Capability: "config.read",
		Operation:  machine.OperationList,
		Input:      &machine.Input{Product: "zia", Resource: "locations"},
	}

	got, err := executor.Execute(context.Background(), req)
	if err == nil {
		t.Fatal("Executor.Execute(unsupported capability) error = nil, want MachineError")
	}
	assertMachineError(t, err, machine.ErrorKindUnsupportedCapability, machine.OperationList, "zia", "locations")
	assertResponseError(t, got, machine.ErrorKindUnsupportedCapability)
	if len(loader.calls) != 0 {
		t.Fatalf("Executor.Execute(unsupported capability) loader calls = %#v, want none", loader.calls)
	}
}

func TestExecutorRejectsUnsupportedOperationBeforeLoader(t *testing.T) {
	loader := &fakeBrowserLoader{}
	executor := machine.Executor{Browser: loader}
	req := machine.Request{
		RequestID:  "req-delete",
		Capability: machine.CapabilityResourcesRead,
		Operation:  machine.Operation("delete"),
		Input:      &machine.Input{Product: "zia", Resource: "locations"},
	}

	got, err := executor.Execute(context.Background(), req)
	if err == nil {
		t.Fatal("Executor.Execute(delete request) error = nil, want MachineError")
	}
	assertMachineError(t, err, machine.ErrorKindUnsupportedOperation, machine.Operation("delete"), "zia", "locations")
	assertResponseError(t, got, machine.ErrorKindUnsupportedOperation)
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
			name: "options",
			op:   machine.OperationList,
			input: &machine.Input{
				Product: "zia", Resource: "locations", Options: map[string]string{"raw": "true"},
			},
		},
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
			Fields:   []string{"nope"},
		},
	}

	got, err := executor.Execute(context.Background(), req)
	if err == nil {
		t.Fatal("Executor.Execute(unknown field request) error = nil, want usage MachineError")
	}
	assertMachineError(t, err, machine.ErrorKindUsage, machine.OperationList, "zia", "locations")
	assertResponseError(t, got, machine.ErrorKindUsage)
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
