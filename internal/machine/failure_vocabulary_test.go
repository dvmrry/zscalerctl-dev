package machine_test

import (
	"encoding/json"
	"os"
	"slices"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/enginewire"
	"github.com/dvmrry/zscalerctl/internal/machine"
)

// TestFailureReasonsMatchEveryCarrier keeps the one reason vocabulary equal
// everywhere it is published: the stdio wire's copy (enginewire imports no
// other zscalerctl package), the CLI error envelope schema, the dump error
// schema, and the stdio engine schema.
func TestFailureReasonsMatchEveryCarrier(t *testing.T) {
	t.Parallel()

	want := slices.Sorted(slices.Values(machine.FailureReasons()))
	if got := slices.Sorted(slices.Values(enginewire.FailureReasons)); !slices.Equal(got, want) {
		t.Errorf("enginewire.FailureReasons = %v, want %v", got, want)
	}
	for _, carrier := range []struct {
		path string
		// pointer walks object keys from the schema root to the reason property.
		pointer []string
	}{
		{"../../docs/schema/error.schema.json", []string{"properties", "error", "properties", "reason"}},
		{"../../docs/schema/dump-error.schema.json", []string{"properties", "reason"}},
		{"../../docs/schema/engine-stdio-v1.schema.json", []string{"$defs", "failureReason"}},
	} {
		body, err := os.ReadFile(carrier.path)
		if err != nil {
			t.Fatal(err)
		}
		var node any
		if err := json.Unmarshal(body, &node); err != nil {
			t.Fatal(err)
		}
		for _, key := range carrier.pointer {
			object, ok := node.(map[string]any)
			if !ok {
				t.Fatalf("%s: no object at %q", carrier.path, key)
			}
			node = object[key]
		}
		reason, _ := node.(map[string]any)
		var got []string
		for _, value := range reason["enum"].([]any) {
			got = append(got, value.(string))
		}
		slices.Sort(got)
		if !slices.Equal(got, want) {
			t.Errorf("%s reason enum = %v, want %v", carrier.path, got, want)
		}
	}
}
