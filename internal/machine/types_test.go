package machine_test

import (
	"encoding/json"
	"os/exec"
	"reflect"
	"strings"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/machine"
)

type safeJSON interface {
	OutputSafe()
}

func TestRequestJSONShape(t *testing.T) {
	req := machine.Request{
		RequestID:  "req-1",
		Capability: "resources.read",
		Operation:  machine.OperationList,
		Input: &machine.Input{
			Product:  "zia",
			Resource: "locations",
			Fields:   []string{"id", "name"},
			Filters: []machine.Filter{
				{Field: "name", Operator: "~", Value: "branch"},
			},
			Search: "hq",
		},
	}

	got := decodeJSONMap(t, req)
	if got["request_id"] != "req-1" ||
		got["capability"] != "resources.read" ||
		got["operation"] != "list" {
		t.Fatalf("json.Marshal(Request) = %#v, want request_id, capability, and operation", got)
	}
	if _, ok := got["product"]; ok {
		t.Fatalf("json.Marshal(Request) = %#v, want product under input only", got)
	}
	input, ok := got["input"].(map[string]any)
	if !ok {
		t.Fatalf("json.Marshal(Request) input = %T, want object; body %#v", got["input"], got)
	}
	if input["product"] != "zia" || input["resource"] != "locations" {
		t.Fatalf("json.Marshal(Request) input = %#v, want zia/locations", input)
	}
	if _, ok := got["meta"]; ok {
		t.Fatalf("json.Marshal(Request) = %#v, want empty meta omitted", got)
	}
}

func TestManifestJSONShape(t *testing.T) {
	manifest := machine.Manifest{
		Version: "machine.v1",
		Capabilities: []machine.Capability{
			{
				Name:        "resources.read",
				Title:       "Read projected resources",
				Operations:  []machine.Operation{machine.OperationList, machine.OperationShow},
				Input:       &machine.Input{Product: "zia", Resource: "locations"},
				Output:      &machine.SchemaRef{Name: "projected-records", Version: "1"},
				Description: "load projected and redacted resource records",
				Examples: []machine.Example{
					{
						Name: "list locations",
						Request: machine.Request{
							Capability: "resources.read",
							Operation:  machine.OperationList,
							Input:      &machine.Input{Product: "zia", Resource: "locations"},
						},
					},
				},
			},
		},
		Schemas: []machine.SchemaRef{{Name: "projected-records", Version: "1"}},
		Meta:    &machine.Meta{Version: "1", Count: 1},
	}

	got := decodeJSONMap(t, manifest)
	if got["version"] != "machine.v1" {
		t.Fatalf("json.Marshal(Manifest) version = %#v, want machine.v1", got["version"])
	}
	caps, ok := got["capabilities"].([]any)
	if !ok || len(caps) != 1 {
		t.Fatalf("json.Marshal(Manifest) capabilities = %#v, want one capability", got["capabilities"])
	}
	capability, ok := caps[0].(map[string]any)
	if !ok {
		t.Fatalf("json.Marshal(Manifest) capability = %T, want object", caps[0])
	}
	if capability["name"] != "resources.read" {
		t.Fatalf("json.Marshal(Manifest) capability name = %#v, want resources.read", capability["name"])
	}
}

func TestResponseJSONShape(t *testing.T) {
	resp := machine.Response{
		RequestID:  "req-1",
		Capability: "resources.read",
		Operation:  machine.OperationList,
		Records: []map[string]any{
			{"id": "123", "name": "HQ"},
		},
		Meta: &machine.Meta{Count: 1},
	}

	got := decodeJSONMap(t, resp)
	records, ok := got["records"].([]any)
	if !ok || len(records) != 1 {
		t.Fatalf("json.Marshal(Response) records = %#v, want one record", got["records"])
	}
	record, ok := records[0].(map[string]any)
	if !ok {
		t.Fatalf("json.Marshal(Response) record = %T, want object", records[0])
	}
	if !reflect.DeepEqual(record, map[string]any{"id": "123", "name": "HQ"}) {
		t.Fatalf("json.Marshal(Response) record = %#v, want projected record", record)
	}
	if _, ok := got["manifest"]; ok {
		t.Fatalf("json.Marshal(Response) = %#v, want empty manifest omitted", got)
	}
	if _, ok := got["error"]; ok {
		t.Fatalf("json.Marshal(Response) = %#v, want empty error omitted", got)
	}
}

func TestMachineErrorShapeAndErrorFallback(t *testing.T) {
	errBody := machine.MachineError{
		Kind:      "live_access_failed",
		Message:   "request failed",
		Operation: machine.OperationList,
		Product:   "zia",
		Resource:  "locations",
	}
	if got, want := errBody.Error(), "request failed"; got != want {
		t.Fatalf("MachineError.Error() = %q, want %q", got, want)
	}

	got := decodeJSONMap(t, errBody)
	want := map[string]any{
		"kind":      "live_access_failed",
		"message":   "request failed",
		"operation": "list",
		"product":   "zia",
		"resource":  "locations",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("json.Marshal(MachineError) = %#v, want %#v", got, want)
	}

	if got, want := (machine.MachineError{Kind: "usage"}).Error(), "usage"; got != want {
		t.Fatalf("MachineError{Kind: usage}.Error() = %q, want %q", got, want)
	}
	if got, want := (machine.MachineError{}).Error(), "machine error"; got != want {
		t.Fatalf("MachineError{}.Error() = %q, want %q", got, want)
	}
}

func TestSafeJSONMarkers(t *testing.T) {
	var _ safeJSON = machine.Response{}
	var _ safeJSON = machine.Manifest{}
	var _ safeJSON = machine.MachineError{}
}

func TestMachinePackageDoesNotImportCLIUIOrRendering(t *testing.T) {
	cmd := exec.Command("go", "list", "-deps", "-mod=vendor", "github.com/dvmrry/zscalerctl/internal/machine")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go list internal/machine error = %v\n%s", err, out)
	}
	for _, forbidden := range []string{
		"github.com/dvmrry/zscalerctl/internal/cli",
		"github.com/dvmrry/zscalerctl/internal/output",
		"github.com/spf13/cobra",
		"bubbletea",
		"bubbles",
		"wails",
		"react",
		"vite",
		"lipgloss",
	} {
		if strings.Contains(string(out), forbidden) {
			t.Fatalf("internal/machine deps include %q, want no CLI, UI, or rendering dependency\n%s", forbidden, out)
		}
	}
}

func decodeJSONMap(t *testing.T, value any) map[string]any {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal(%T) error = %v, want nil", value, err)
	}
	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("json.Unmarshal(json.Marshal(%T)) error = %v; body %s", value, err, body)
	}
	return got
}
