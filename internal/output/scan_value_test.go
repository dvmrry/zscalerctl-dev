package output_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/output"
	"github.com/dvmrry/zscalerctl/internal/redact"
)

// ScanRecord gives a record the scan JSON output gives it, siblings included.
func TestScanRecordAppliesTheJSONScanToTheWholeRecord(t *testing.T) {
	t.Parallel()
	const marker = "<REDACTED:SECRET>"
	for _, tc := range []struct {
		name   string
		record map[string]any
		want   map[string]any
	}{
		{"plain", map[string]any{"name": "Branch office"}, map[string]any{"name": "Branch office"}},
		{"secret-named key", map[string]any{"apiToken": "literal-canary"}, map[string]any{"apiToken": marker}},
		{"container under a secret key", map[string]any{"password": map[string]any{"v": "x"}}, map[string]any{"password": marker}},
		{"reviewed numeric setting", map[string]any{"policyExtension": map[string]any{"zpaAuthExpOnWinLogonSession": json.Number("480")}},
			map[string]any{"policyExtension": map[string]any{"zpaAuthExpOnWinLogonSession": json.Number("480")}}},
		{"literal under a secret word", map[string]any{"allowZdxDisableWithoutPassword": true}, map[string]any{"allowZdxDisableWithoutPassword": true}},
		{"key/value pair names a credential", map[string]any{"tags": []any{map[string]any{"key": "password", "value": "Zq8vK3mP7xR2wT"}}},
			map[string]any{"tags": []any{map[string]any{"key": "password", "value": marker}}}},
		{"top-level name/value pair names a credential", map[string]any{"name": "password", "value": "Zq8vK3mP7xR2wT"},
			map[string]any{"name": "password", "value": marker}},
		{"float renders as JSON writes it", map[string]any{"limit": 1e6}, map[string]any{"limit": json.Number("1000000")}},
	} {
		for _, mode := range []redact.Mode{redact.ModeStandard, redact.ModeShare, redact.ModeParanoid} {
			if got := output.NewRenderer(redact.New(mode)).ScanRecord(tc.record); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("%s (%s): ScanRecord(%#v) = %#v, want %#v", tc.name, mode, tc.record, got, tc.want)
			}
		}
	}
	got := output.NewRenderer(redact.New(redact.ModeStandard)).ScanRecord(map[string]any{"x": make(chan int), "y": "fine"})
	if !reflect.DeepEqual(got, map[string]any{"x": marker, "y": marker}) {
		t.Errorf("ScanRecord(unencodable) = %#v, want every member replaced", got)
	}
}
