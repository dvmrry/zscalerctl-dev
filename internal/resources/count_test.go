package resources_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

func countFixture(t *testing.T) resources.ProjectedRecords {
	t.Helper()
	source := []resources.SourceRecord{
		resources.NewSourceRecord(map[string]any{"id": 1, "name": "a", "tags": []any{"x", "y", "x"},
			"groups": []any{map[string]any{"id": 10, "name": "G10"}, map[string]any{"id": 11, "name": "G11"}}}),
		resources.NewSourceRecord(map[string]any{"id": 2, "name": "b", "tags": []any{"x"},
			"groups": []any{map[string]any{"id": 10, "name": "G10"}}}),
		resources.NewSourceRecord(map[string]any{"id": 3, "name": "a"}),
	}
	projected, _, err := resources.ProjectRecordsAndVerify(nestedNarrowingSpec(), redact.ModeStandard, source)
	if err != nil {
		t.Fatal(err)
	}
	return projected
}

func rowsString(rows []resources.CountRow) string {
	var parts []string
	for _, row := range rows {
		var values []string
		for _, v := range row.Values {
			if v == nil {
				values = append(values, "null")
			} else {
				values = append(values, resources.FormatProjectedValue(v))
			}
		}
		parts = append(parts, strings.Join(values, "+")+"="+string(rune('0'+row.Count)))
	}
	return strings.Join(parts, " ")
}

func TestCountProjectedRecords(t *testing.T) {
	t.Parallel()

	records := countFixture(t)
	for _, tc := range []struct {
		paths []string
		want  string
	}{
		// Members per group; the record without groups counts under null.
		{[]string{"groups.id"}, "10=2 11=1 null=1"},
		// A top-level list counts per element, each element once per record.
		{[]string{"tags"}, "x=2 y=1 null=1"},
		{[]string{"name"}, "a=2 b=1"},
		// Several paths count combinations of their values.
		{[]string{"name", "groups.id"}, "a+10=1 a+11=1 a+null=1 b+10=1"},
	} {
		if got := rowsString(resources.CountProjectedRecords(records, tc.paths)); got != tc.want {
			t.Errorf("CountProjectedRecords(%v) = %q, want %q", tc.paths, got, tc.want)
		}
	}
}

func TestValidateCountByPaths(t *testing.T) {
	t.Parallel()

	spec := nestedNarrowingSpec()
	spec.Fields = append(spec.Fields, resources.FieldSpec{Name: "token", Classification: resources.ClassSecret})
	if err := resources.ValidateCountByPaths(spec, []string{"groups.id", "tags", "name"}); err != nil {
		t.Errorf("valid paths error = %v", err)
	}
	var pathErr resources.CountByPathError
	if err := resources.ValidateCountByPaths(spec, []string{"groups"}); !errors.As(err, &pathErr) || !pathErr.NotLeaf {
		t.Errorf("object path error = %v, want a not-leaf CountByPathError", err)
	}
	if err := resources.ValidateCountByPaths(spec, []string{"nope"}); !errors.Is(err, resources.ErrUnknownField) {
		t.Errorf("unknown path error = %v, want ErrUnknownField", err)
	}
	var suppressed resources.SuppressedFieldError
	err := resources.ValidateCountByPathsInMode(spec, redact.ModeShare, []string{"groups.name"})
	if !errors.As(err, &suppressed) || !strings.HasPrefix(err.Error(), "--count-by:") || !strings.Contains(err.Error(), "--redaction standard") {
		t.Errorf("share groups.name error = %v, want a --count-by suppressed-field error naming standard", err)
	}
	if err := resources.ValidateCountByPathsInMode(spec, redact.ModeStandard, []string{"token"}); !errors.Is(err, resources.ErrSuppressedField) {
		t.Errorf("secret path error = %v, want ErrSuppressedField", err)
	}
}
