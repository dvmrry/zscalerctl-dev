package machine_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/machine"
)

func TestDumpResultReturnsDefensiveCopies(t *testing.T) {
	t.Parallel()

	source := []machine.DumpResourceError{{
		Product: "zia", Resource: "locations", Operation: machine.OperationList, Kind: "list_failed",
	}}
	result := machine.NewDumpResult(4, 2, "share", source)
	source[0].Product = "mutated"

	want := []machine.DumpResourceError{{
		Product: "zia", Resource: "locations", Operation: machine.OperationList, Kind: "list_failed",
	}}
	first := result.Errors()
	if !reflect.DeepEqual(first, want) {
		t.Fatalf("DumpResult.Errors() = %#v, want %#v", first, want)
	}
	first[0].Resource = "mutated"
	if second := result.Errors(); !reflect.DeepEqual(second, want) {
		t.Fatalf("DumpResult.Errors() after caller mutation = %#v, want %#v", second, want)
	}
	if result.Records() != 4 || result.Resources() != 2 || result.Warnings() != 1 ||
		result.Redaction() != "share" || !result.Partial() {
		t.Fatalf("DumpResult summary = records:%d resources:%d warnings:%d redaction:%q partial:%t",
			result.Records(), result.Resources(), result.Warnings(), result.Redaction(), result.Partial())
	}
	empty := machine.NewDumpResult(0, 0, "standard", nil)
	if empty.Errors() == nil || empty.Warnings() != 0 || empty.Partial() {
		t.Fatalf("empty DumpResult = errors:%#v warnings:%d partial:%t, want non-nil empty/0/false",
			empty.Errors(), empty.Warnings(), empty.Partial())
	}
}

func TestDumpEngineValuesRejectDirectJSON(t *testing.T) {
	t.Parallel()

	values := []struct {
		name  string
		value any
	}{
		{
			name: "request",
			value: machine.DumpRequest{
				RequestID: "must-not-render",
				OutputDir: "must-not-render",
				Products:  []string{"must-not-render"},
				Resources: []machine.DumpResourceSelector{{Product: "zia", Resource: "must-not-render"}},
			},
		},
		{
			name: "result",
			value: machine.NewDumpResult(1, 1, "must-not-render", []machine.DumpResourceError{{
				Product: "zia", Resource: "must-not-render", Operation: machine.OperationList, Kind: "failed",
			}}),
		},
	}
	for _, tt := range values {
		body, err := json.Marshal(tt.value)
		if err == nil {
			t.Fatalf("json.Marshal(Dump %s) error = nil; body = %s, want no wire format", tt.name, body)
		}
		if strings.Contains(string(body), "must-not-render") {
			t.Fatalf("json.Marshal(Dump %s) body = %q, want no value bytes", tt.name, body)
		}
	}

	var request machine.DumpRequest
	if err := json.Unmarshal([]byte(`{"OutputDir":"must-not-render"}`), &request); err == nil {
		t.Fatalf("json.Unmarshal(DumpRequest) error = nil; request = %#v, want no wire format", request)
	}
	var result machine.DumpResult
	if err := json.Unmarshal([]byte(`{"records":1}`), &result); err == nil {
		t.Fatalf("json.Unmarshal(DumpResult) error = nil; result = %#v, want no wire format", result)
	}
}
