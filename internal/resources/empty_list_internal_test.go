package resources

import (
	"encoding/json"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/redact"
)

// TestProjectionRendersOnlyConfirmedEmptyLists pins the empty-list model: an
// empty list (nil or not, scalar or object) is skipped, because a mapper's
// empty slice cannot tell "sent as []" from "not sent"; only a list marked as
// returned [] by the API renders [], and only in a mode that renders it.
func TestProjectionRendersOnlyConfirmedEmptyLists(t *testing.T) {
	t.Parallel()

	spec := ResourceSpec{Product: ProductZIA, Name: "probe", Operations: ReadOperations(), Fields: []FieldSpec{
		operationalField("id", allModes()),
		tenantConfigField("tags", allModes()),
		tenantConfigField("hosts", standardOnlyMode()),
		{Name: "groups", Classification: ClassTenantConfig, AllowedModes: allModes(), Fields: []FieldSpec{
			operationalField("id", allModes()),
		}},
	}}
	projectWithReport := func(mode redact.Mode, record SourceRecord) (string, ProjectionReport) {
		t.Helper()
		projected, report, err := ProjectRecord(spec, mode, record)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := json.Marshal(projected)
		return string(body), report
	}
	project := func(mode redact.Mode, record SourceRecord) string {
		t.Helper()
		body, _ := projectWithReport(mode, record)
		return body
	}
	for _, empty := range []any{[]string(nil), []string{}, []any(nil), []any{}, []int{}, []map[string]any{}} {
		record := NewSourceRecord(map[string]any{"id": 1, "tags": empty, "groups": empty})
		got, report := projectWithReport(redact.ModeStandard, record)
		if got != `{"id":1}` {
			t.Errorf("unconfirmed empty %T projects to %s, want the keys skipped", empty, got)
		}
		if len(report.DroppedFields) != 0 {
			t.Errorf("unconfirmed empty %T: DroppedFields = %v; empty lists are skipped, not redaction drops", empty, report.DroppedFields)
		}
	}

	confirmed := NewSourceRecord(map[string]any{"id": 1, "tags": []string{}}).
		WithConfirmedEmptyList("tags").WithConfirmedEmptyList("groups").WithConfirmedEmptyList("hosts")
	if got, want := project(redact.ModeStandard, confirmed), `{"groups":[],"hosts":[],"id":1,"tags":[]}`; got != want {
		t.Errorf("confirmed empty lists in standard = %s, want %s", got, want)
	}
	if got, want := project(redact.ModeShare, confirmed), `{"groups":[],"id":1,"tags":[]}`; got != want {
		t.Errorf("confirmed empty lists in share = %s, want %s (hosts is standard-only)", got, want)
	}

	kept := NewSourceRecord(map[string]any{"id": 1, "tags": []string{"a"}}).WithConfirmedEmptyList("tags")
	if got, want := project(redact.ModeStandard, kept), `{"id":1,"tags":["a"]}`; got != want {
		t.Errorf("WithConfirmedEmptyList over a non-empty list = %s, want %s", got, want)
	}
}
