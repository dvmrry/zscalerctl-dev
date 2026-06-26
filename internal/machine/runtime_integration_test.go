package machine_test

import (
	"context"
	"encoding/json"
	"errors"
	"os/exec"
	"reflect"
	"strings"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/browser"
	"github.com/dvmrry/zscalerctl/internal/machine"
	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

const runtimeHarnessCanary = "abc123abc123abc123abc123abc123abc123"

func TestExecutorWithBrowserServiceProjectsAndRedactsListShowGet(t *testing.T) {
	reader := &runtimeHarnessReader{
		list: map[runtimeResourceKey][]resources.SourceRecord{
			{product: resources.ProductZIA, name: "locations"}: {
				resources.NewSourceRecord(map[string]any{
					"id":       "loc-1",
					"name":     "HQ",
					"status":   "active",
					"apiKey":   runtimeHarnessCanary,
					"password": runtimeHarnessCanary,
					"rawOnly":  runtimeHarnessCanary,
				}),
				resources.NewSourceRecord(map[string]any{
					"id":       "loc-2",
					"name":     "Branch",
					"status":   "disabled",
					"apiKey":   runtimeHarnessCanary,
					"password": runtimeHarnessCanary,
					"rawOnly":  runtimeHarnessCanary,
				}),
			},
		},
		show: map[runtimeResourceKey]resources.SourceRecord{
			{product: resources.ProductZIA, name: "advanced-settings"}: resources.NewSourceRecord(map[string]any{
				"id":       "advanced-settings",
				"name":     "Advanced Settings",
				"status":   "enabled",
				"apiKey":   runtimeHarnessCanary,
				"password": runtimeHarnessCanary,
				"rawOnly":  runtimeHarnessCanary,
			}),
		},
		get: map[runtimeResourceIDKey]resources.SourceRecord{
			{product: resources.ProductZIA, name: "locations", id: "loc-1"}: resources.NewSourceRecord(map[string]any{
				"id":       "loc-1",
				"name":     "HQ",
				"status":   "active",
				"apiKey":   runtimeHarnessCanary,
				"password": runtimeHarnessCanary,
				"rawOnly":  runtimeHarnessCanary,
			}),
		},
	}
	executor := runtimeHarnessExecutor(reader)

	tests := []struct {
		name        string
		req         machine.Request
		wantCall    string
		wantRecords []map[string]any
	}{
		{
			name: "list",
			req: machine.Request{
				RequestID:  "req-list",
				Capability: machine.CapabilityResourcesRead,
				Operation:  machine.OperationList,
				Input:      &machine.Input{Product: "zia", Resource: "locations"},
			},
			wantCall: "list:zia/locations",
			wantRecords: []map[string]any{
				{"id": "loc-1", "name": "HQ", "status": "active"},
				{"id": "loc-2", "name": "Branch", "status": "disabled"},
			},
		},
		{
			name: "show",
			req: machine.Request{
				RequestID:  "req-show",
				Capability: machine.CapabilityResourcesRead,
				Operation:  machine.OperationShow,
				Input:      &machine.Input{Product: "zia", Resource: "advanced-settings"},
			},
			wantCall: "show:zia/advanced-settings",
			wantRecords: []map[string]any{
				{"id": "advanced-settings", "name": "Advanced Settings", "status": "enabled"},
			},
		},
		{
			name: "get",
			req: machine.Request{
				RequestID:  "req-get",
				Capability: machine.CapabilityResourcesRead,
				Operation:  machine.OperationGet,
				Input:      &machine.Input{Product: "zia", Resource: "locations", RecordID: "loc-1"},
			},
			wantCall: "get:zia/locations/loc-1",
			wantRecords: []map[string]any{
				{"id": "loc-1", "name": "HQ", "status": "active"},
			},
		},
	}

	for _, tt := range tests {
		got, err := executor.Execute(context.Background(), tt.req)
		if err != nil {
			t.Fatalf("Executor.Execute(%s through browser.Service) error = %v, want nil", tt.name, err)
		}
		if !reflect.DeepEqual(got.Records, tt.wantRecords) {
			t.Fatalf("Executor.Execute(%s through browser.Service).Records = %#v, want %#v",
				tt.name, got.Records, tt.wantRecords)
		}
		assertRuntimeHarnessMeta(t, got, tt.req, len(tt.wantRecords))
		assertRuntimeHarnessResponseDoesNotLeak(t, tt.name, got)
	}

	wantCalls := []string{
		"list:zia/locations",
		"show:zia/advanced-settings",
		"get:zia/locations/loc-1",
	}
	if !reflect.DeepEqual(reader.calls, wantCalls) {
		t.Fatalf("Executor.Execute(list/show/get through browser.Service) reader calls = %#v, want %#v", reader.calls, wantCalls)
	}
}

func TestExecutorWithBrowserServiceHandlesInputAndBackendErrors(t *testing.T) {
	t.Run("missing_product", func(t *testing.T) {
		reader := &runtimeHarnessReader{}
		executor := runtimeHarnessExecutor(reader)
		req := machine.Request{
			RequestID:  "req-missing-product",
			Capability: machine.CapabilityResourcesRead,
			Operation:  machine.OperationList,
			Input:      &machine.Input{Resource: "locations"},
		}

		got, err := executor.Execute(context.Background(), req)
		machineErr := assertRuntimeHarnessMachineError(
			t,
			err,
			machine.ErrorKindUsage,
			machine.OperationList,
			"",
			"locations",
		)
		wantMissing := []string{"input.product"}
		if !reflect.DeepEqual(machineErr.Missing, wantMissing) {
			t.Fatalf("Executor.Execute(missing product through browser.Service) missing = %#v, want %#v",
				machineErr.Missing, wantMissing)
		}
		assertRuntimeHarnessResponseError(t, got, machine.ErrorKindUsage)
		if len(reader.calls) != 0 {
			t.Fatalf("Executor.Execute(missing product through browser.Service) reader calls = %#v, want none", reader.calls)
		}
	})

	t.Run("missing_resource", func(t *testing.T) {
		reader := &runtimeHarnessReader{}
		executor := runtimeHarnessExecutor(reader)
		req := machine.Request{
			RequestID:  "req-missing-resource",
			Capability: machine.CapabilityResourcesRead,
			Operation:  machine.OperationList,
			Input:      &machine.Input{Product: "zia"},
		}

		got, err := executor.Execute(context.Background(), req)
		machineErr := assertRuntimeHarnessMachineError(
			t,
			err,
			machine.ErrorKindUsage,
			machine.OperationList,
			"zia",
			"",
		)
		wantMissing := []string{"input.resource"}
		if !reflect.DeepEqual(machineErr.Missing, wantMissing) {
			t.Fatalf("Executor.Execute(missing resource through browser.Service) missing = %#v, want %#v",
				machineErr.Missing, wantMissing)
		}
		assertRuntimeHarnessResponseError(t, got, machine.ErrorKindUsage)
		if len(reader.calls) != 0 {
			t.Fatalf("Executor.Execute(missing resource through browser.Service) reader calls = %#v, want none", reader.calls)
		}
	})

	t.Run("get_missing_record_id", func(t *testing.T) {
		reader := &runtimeHarnessReader{}
		executor := runtimeHarnessExecutor(reader)
		req := machine.Request{
			RequestID:  "req-missing-record-id",
			Capability: machine.CapabilityResourcesRead,
			Operation:  machine.OperationGet,
			Input:      &machine.Input{Product: "zia", Resource: "locations"},
		}

		got, err := executor.Execute(context.Background(), req)
		machineErr := assertRuntimeHarnessMachineError(
			t,
			err,
			machine.ErrorKindUsage,
			machine.OperationGet,
			"zia",
			"locations",
		)
		wantMissing := []string{"input.record_id"}
		if !reflect.DeepEqual(machineErr.Missing, wantMissing) {
			t.Fatalf("Executor.Execute(get without record_id through browser.Service) missing = %#v, want %#v",
				machineErr.Missing, wantMissing)
		}
		assertRuntimeHarnessResponseError(t, got, machine.ErrorKindUsage)
		if len(reader.calls) != 0 {
			t.Fatalf("Executor.Execute(get without record_id through browser.Service) reader calls = %#v, want none", reader.calls)
		}
	})

	t.Run("unsupported_operation", func(t *testing.T) {
		reader := &runtimeHarnessReader{}
		executor := runtimeHarnessExecutor(reader)
		req := machine.Request{
			RequestID:  "req-delete",
			Capability: machine.CapabilityResourcesRead,
			Operation:  machine.Operation("delete"),
			Input:      &machine.Input{Product: "zia", Resource: "locations"},
		}

		got, err := executor.Execute(context.Background(), req)
		assertRuntimeHarnessMachineError(
			t,
			err,
			machine.ErrorKindUnsupportedOperation,
			machine.Operation("delete"),
			"zia",
			"locations",
		)
		assertRuntimeHarnessResponseError(t, got, machine.ErrorKindUnsupportedOperation)
		if len(reader.calls) != 0 {
			t.Fatalf("Executor.Execute(delete through browser.Service) reader calls = %#v, want none", reader.calls)
		}
	})

	t.Run("backend_error_sanitized", func(t *testing.T) {
		reader := &runtimeHarnessReader{
			listErr: errors.New("raw SDK token " + runtimeHarnessCanary + " failed"),
		}
		executor := runtimeHarnessExecutor(reader)
		req := machine.Request{
			RequestID:  "req-backend-error",
			Capability: machine.CapabilityResourcesRead,
			Operation:  machine.OperationList,
			Input:      &machine.Input{Product: "zia", Resource: "locations"},
		}

		got, err := executor.Execute(context.Background(), req)
		machineErr := assertRuntimeHarnessMachineError(
			t,
			err,
			machine.ErrorKindLiveAccessFailed,
			machine.OperationList,
			"zia",
			"locations",
		)
		if strings.Contains(machineErr.Message, runtimeHarnessCanary) || strings.Contains(machineErr.Message, "SDK") {
			t.Fatalf("Executor.Execute(backend error through browser.Service) MachineError.Message = %q, want sanitized message",
				machineErr.Message)
		}
		assertRuntimeHarnessResponseError(t, got, machine.ErrorKindLiveAccessFailed)
		assertRuntimeHarnessResponseDoesNotLeak(t, "backend_error_sanitized", got)
		wantCalls := []string{"list:zia/locations"}
		if !reflect.DeepEqual(reader.calls, wantCalls) {
			t.Fatalf("Executor.Execute(backend error through browser.Service) reader calls = %#v, want %#v",
				reader.calls, wantCalls)
		}
	})
}

func TestRuntimeHarnessUsesOnlyCoreRuntimePackages(t *testing.T) {
	cmd := exec.Command("go", "list", "-deps", "-test", "-mod=vendor", "github.com/dvmrry/zscalerctl/internal/machine")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go list -deps -test internal/machine error = %v\n%s", err, out)
	}
	for _, forbidden := range []string{
		"github.com/dvmrry/zscalerctl/internal/cli",
		"github.com/dvmrry/zscalerctl/internal/output",
		"github.com/dvmrry/zscalerctl/internal/config",
		"github.com/dvmrry/zscalerctl/internal/credentials",
		"github.com/dvmrry/zscalerctl/internal/secretref",
		"github.com/dvmrry/zscalerctl/internal/secret",
		"github.com/dvmrry/zscalerctl/internal/zscaler",
		"github.com/spf13/cobra",
		"bubbletea",
		"bubbles",
		"wails",
		"fang",
		"react",
		"vite",
		"lipgloss",
	} {
		if strings.Contains(string(out), forbidden) {
			t.Fatalf("go list -deps -test internal/machine includes %q, want only core runtime/test dependencies\n%s",
				forbidden, out)
		}
	}
}

type runtimeResourceKey struct {
	product resources.Product
	name    string
}

type runtimeResourceIDKey struct {
	product resources.Product
	name    string
	id      string
}

type runtimeHarnessReader struct {
	list    map[runtimeResourceKey][]resources.SourceRecord
	show    map[runtimeResourceKey]resources.SourceRecord
	get     map[runtimeResourceIDKey]resources.SourceRecord
	listErr error
	showErr error
	getErr  error
	calls   []string
}

func (r *runtimeHarnessReader) List(
	_ context.Context,
	product resources.Product,
	name string,
) ([]resources.SourceRecord, error) {
	r.calls = append(r.calls, "list:"+string(product)+"/"+name)
	if r.listErr != nil {
		return nil, r.listErr
	}
	return r.list[runtimeResourceKey{product: product, name: name}], nil
}

func (r *runtimeHarnessReader) Show(
	_ context.Context,
	product resources.Product,
	name string,
) (resources.SourceRecord, error) {
	r.calls = append(r.calls, "show:"+string(product)+"/"+name)
	if r.showErr != nil {
		return resources.SourceRecord{}, r.showErr
	}
	return r.show[runtimeResourceKey{product: product, name: name}], nil
}

func (r *runtimeHarnessReader) Get(
	_ context.Context,
	product resources.Product,
	name string,
	id string,
) (resources.SourceRecord, error) {
	r.calls = append(r.calls, "get:"+string(product)+"/"+name+"/"+id)
	if r.getErr != nil {
		return resources.SourceRecord{}, r.getErr
	}
	return r.get[runtimeResourceIDKey{product: product, name: name, id: id}], nil
}

func runtimeHarnessExecutor(reader *runtimeHarnessReader) machine.Executor {
	return machine.Executor{
		Browser: browser.Service{
			Catalog: runtimeHarnessCatalog(),
			Reader:  reader,
			Mode:    redact.ModeShare,
		},
	}
}

func runtimeHarnessCatalog() resources.ResourceCatalog {
	return resources.ResourceCatalog{
		runtimeHarnessSpec(resources.ProductZIA, "locations", resources.ReadOperations()),
		runtimeHarnessSpec(resources.ProductZIA, "advanced-settings", resources.ShowOperation()),
	}
}

func runtimeHarnessSpec(
	product resources.Product,
	name string,
	operations []resources.Operation,
) resources.ResourceSpec {
	return resources.ResourceSpec{
		Product:    product,
		Name:       name,
		Operations: operations,
		Fields: []resources.FieldSpec{
			{
				Name:           "id",
				Classification: resources.ClassOperational,
				AllowedModes:   runtimeHarnessAllModes(),
			},
			{
				Name:           "name",
				Classification: resources.ClassTenantConfig,
				AllowedModes:   []redact.Mode{redact.ModeStandard, redact.ModeShare},
			},
			{
				Name:           "status",
				Classification: resources.ClassOperational,
				AllowedModes:   runtimeHarnessAllModes(),
			},
			{
				Name:           "apiKey",
				Classification: resources.ClassSecret,
			},
			{
				Name:           "password",
				Classification: resources.ClassSecret,
			},
			{
				Name:           "rawOnly",
				Classification: resources.ClassSecret,
			},
		},
	}
}

func runtimeHarnessAllModes() []redact.Mode {
	return []redact.Mode{redact.ModeStandard, redact.ModeShare, redact.ModeParanoid}
}

func assertRuntimeHarnessMeta(t *testing.T, got machine.Response, req machine.Request, wantCount int) {
	t.Helper()
	if got.Meta == nil {
		t.Fatalf("Executor.Execute(%#v through browser.Service).Meta = nil, want metadata", req)
	}
	if got.Meta.RequestID != req.RequestID ||
		got.Meta.Product != req.Input.Product ||
		got.Meta.Resource != req.Input.Resource ||
		!got.Meta.ReadOnly ||
		got.Meta.Count != wantCount {
		t.Fatalf("Executor.Execute(%#v through browser.Service).Meta = %#v, want request/product/resource/read_only/count",
			req, got.Meta)
	}
}

func assertRuntimeHarnessResponseDoesNotLeak(t *testing.T, label string, got machine.Response) {
	t.Helper()
	body, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("json.Marshal(Executor.Execute(%s through browser.Service)) error = %v, want nil", label, err)
	}
	for _, forbidden := range []string{"apiKey", "password", "rawOnly", runtimeHarnessCanary} {
		if strings.Contains(string(body), forbidden) {
			t.Fatalf("json.Marshal(Executor.Execute(%s through browser.Service)) = %s, want no %q",
				label, body, forbidden)
		}
	}
}

func assertRuntimeHarnessMachineError(
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
		t.Fatalf("Executor.Execute through browser.Service error = %T %v, want *machine.MachineError", err, err)
	}
	if machineErr.Kind != wantKind ||
		machineErr.Operation != wantOperation ||
		machineErr.Product != wantProduct ||
		machineErr.Resource != wantResource {
		t.Fatalf("MachineError through browser.Service = %#v, want kind:%q operation:%q product:%q resource:%q",
			machineErr, wantKind, wantOperation, wantProduct, wantResource)
	}
	return machineErr
}

func assertRuntimeHarnessResponseError(t *testing.T, got machine.Response, wantKind string) {
	t.Helper()
	if got.Error == nil {
		t.Fatalf("Executor.Execute through browser.Service response error = nil, want MachineError")
	}
	if got.Error.Kind != wantKind {
		t.Fatalf("Executor.Execute through browser.Service response error kind = %q, want %q", got.Error.Kind, wantKind)
	}
}
