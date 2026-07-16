package machine_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	dumpdiff "github.com/dvmrry/zscalerctl/internal/diff"
	"github.com/dvmrry/zscalerctl/internal/machine"
)

func TestDiffResultReturnsRecursiveDefensiveCopies(t *testing.T) {
	t.Parallel()

	source := dumpdiff.Report{
		Schema:  dumpdiff.SchemaID,
		Summary: dumpdiff.Summary{RecordsAdded: 1, RecordsChanged: 1},
		Resources: []dumpdiff.ResourceDiff{{
			Product:  "zia",
			Resource: "locations",
			Added: []dumpdiff.RecordRef{{
				Key: "1",
				Record: map[string]any{
					"name":   "HQ",
					"nested": []any{map[string]any{"value": "safe"}},
				},
			}},
			Removed: []dumpdiff.RecordRef{},
			Changed: []dumpdiff.RecordChange{{
				Key: "2",
				Changes: []dumpdiff.FieldChange{{
					Field: "name",
					Old:   map[string]any{"value": []any{"old"}},
					New:   map[string]any{"value": []any{"new"}},
				}},
			}},
		}},
	}
	result := machine.NewDiffResult(source)
	source.Resources[0].Added[0].Record["name"] = "mutated-source"

	first := result.Report()
	first.Resources[0].Added[0].Record["name"] = "mutated-result"
	first.Resources[0].Changed[0].Changes[0].Old.(map[string]any)["value"].([]any)[0] = "mutated-old"

	second := result.Report()
	if got := second.Resources[0].Added[0].Record["name"]; got != "HQ" {
		t.Fatalf("DiffResult.Report added record after mutation = %#v, want HQ", got)
	}
	if got := second.Resources[0].Changed[0].Changes[0].Old.(map[string]any)["value"].([]any)[0]; got != "old" {
		t.Fatalf("DiffResult.Report field change after mutation = %#v, want old", got)
	}
	if !result.HasDrift() {
		t.Fatal("DiffResult.HasDrift() = false, want true")
	}

	empty := machine.NewDiffResult(dumpdiff.Report{})
	if empty.HasDrift() || empty.Report().Resources != nil {
		t.Fatalf("empty DiffResult = drift:%t resources:%#v, want false/nil", empty.HasDrift(), empty.Report().Resources)
	}
}

func TestDiffEngineValuesRejectDirectJSON(t *testing.T) {
	t.Parallel()

	values := []struct {
		name  string
		value any
	}{
		{
			name: "request",
			value: machine.DiffRequest{
				RequestID: "must-not-render",
				OldDir:    "must-not-render",
				NewDir:    "must-not-render",
				Products:  []string{"must-not-render"},
				Resources: []machine.DiffResourceSelector{{Product: "zia", Resource: "must-not-render"}},
			},
		},
		{
			name: "result",
			value: machine.NewDiffResult(dumpdiff.Report{
				Old: dumpdiff.DumpRef{Path: "must-not-render"},
			}),
		},
	}
	for _, tt := range values {
		body, err := json.Marshal(tt.value)
		if err == nil {
			t.Fatalf("json.Marshal(Diff %s) error = nil; body = %s, want no wire format", tt.name, body)
		}
		if strings.Contains(string(body), "must-not-render") {
			t.Fatalf("json.Marshal(Diff %s) body = %q, want no value bytes", tt.name, body)
		}
	}

	var request machine.DiffRequest
	if err := json.Unmarshal([]byte(`{"OldDir":"must-not-render"}`), &request); err == nil {
		t.Fatalf("json.Unmarshal(DiffRequest) error = nil; request = %#v, want no wire format", request)
	}
	var result machine.DiffResult
	if err := json.Unmarshal([]byte(`{"schema":"must-not-render"}`), &result); err == nil {
		t.Fatalf("json.Unmarshal(DiffResult) error = nil; result = %#v, want no wire format", result)
	}

	if got, want := machine.OperationDiff, machine.Operation("diff"); !reflect.DeepEqual(got, want) {
		t.Fatalf("OperationDiff = %q, want %q", got, want)
	}
}
