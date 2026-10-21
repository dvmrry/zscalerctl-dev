package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/output"
	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

// assertTextRedactsLikeJSON renders record, narrowed to fields as a read
// narrows it, in JSON and through all four text record renderers, and fails
// when a renderer's output shows a marker where JSON does not, or the
// reverse.
func assertTextRedactsLikeJSON(t *testing.T, label string, mode redact.Mode, record map[string]any, fields []string) {
	t.Helper()
	style := output.NewStyle(false, false)
	style.Width = 400
	renderer := output.NewRenderer(redact.New(mode))
	selected := resources.NewProjectedRecordsFromProjectedFields([]map[string]any{record}).Records()[0].Select(fields)
	body, err := json.Marshal(selected)
	if err != nil {
		t.Fatal(err)
	}
	jsonRedacted := strings.Contains(string(renderer.Redactor.Bytes(body)), "<REDACTED")
	records := resources.NewProjectedRecords([]resources.ProjectedRecord{selected})
	for name, text := range map[string]string{
		"table list":  renderRecordsTable(fields, records, renderer.ScanRecord, style).String(),
		"pretty list": renderRecordsPretty(fields, records, renderer.ScanRecord, style).String(),
		"table show":  renderRecordKeyValues(fields, selected, renderer.ScanRecord, style).String(),
		"pretty show": renderRecordPretty(fields, selected, renderer.ScanRecord, style).String(),
	} {
		if textRedacted := strings.Contains(text, "<REDACTED"); textRedacted != jsonRedacted {
			t.Errorf("%s (%s, %s, fields %v): text redacted %v, JSON redacted %v:\n%s", label, mode, name, fields, textRedacted, jsonRedacted, text)
		}
	}
}

// TestRecordTextRedactsLikeJSON pins that table and pretty output redact
// exactly what JSON output redacts: every catalog field at every depth, in
// every mode that renders it, with a benign value of each JSON type, under
// its top-level field and its dotted path.
func TestRecordTextRedactsLikeJSON(t *testing.T) {
	t.Parallel()
	samples := []any{"Example setting", json.Number("480"), json.Number("1.5"), true, map[string]any{}, []any{}}
	var walk func(spec resources.ResourceSpec, fields []resources.FieldSpec, path []string)
	walk = func(spec resources.ResourceSpec, fields []resources.FieldSpec, path []string) {
		for _, field := range fields {
			next := append(append([]string(nil), path...), field.JSONField())
			dotted := strings.Join(next, ".")
			availability, ok := resources.FieldPathAvailability(spec, dotted)
			if !ok {
				continue
			}
			for _, mode := range availability.RenderableIn {
				for _, sample := range samples {
					record := nestedRecord(next, sample)
					label := string(spec.Product) + "/" + spec.Name + " " + dotted
					assertTextRedactsLikeJSON(t, label, mode, record, []string{next[0]})
					assertTextRedactsLikeJSON(t, label, mode, record, []string{dotted})
				}
			}
			walk(spec, field.Fields, next)
		}
	}
	for _, spec := range resources.Catalog() {
		walk(spec, spec.Fields, nil)
	}
}

// Rules that read sibling context (a key/value pair naming a credential)
// must see the record as JSON output does, also through lists and when
// --fields selects only part of the pair.
func TestRecordTextRedactsSiblingContextLikeJSON(t *testing.T) {
	t.Parallel()
	const tags = "expressionJson.expressionContainers.tagContainer.tags"
	record := func(tag map[string]any) map[string]any {
		return map[string]any{"expressionJson": map[string]any{"expressionContainers": []any{
			map[string]any{"tagContainer": map[string]any{"tags": []any{tag}}},
		}}}
	}
	for _, tc := range []struct {
		tag    map[string]any
		fields []string
	}{
		{map[string]any{"key": "password", "value": pairSecret}, []string{tags + ".key", tags + ".value"}},
		{map[string]any{"key": "password", "value": pairSecret}, []string{"expressionJson"}},
		{map[string]any{"key": "password", "value": pairSecret}, []string{tags + ".value"}},
		{map[string]any{"key": pairSecret, "value": "prod"}, []string{tags + ".key"}},
		{map[string]any{"key": "Environment", "value": "prod"}, []string{tags + ".key", tags + ".value"}},
	} {
		for _, mode := range []redact.Mode{redact.ModeStandard, redact.ModeShare, redact.ModeParanoid} {
			if _, ok := resources.FieldPathAvailability(ztwWorkloadGroups(t), tags+".key"); !ok {
				t.Fatal("ztw/workload-groups tags.key is not a catalog path")
			}
			assertTextRedactsLikeJSON(t, "ztw/workload-groups tags", mode, record(tc.tag), tc.fields)
		}
	}
}

// pairSecret is a synthetic credential-shaped tag value.
const pairSecret = "Zq8vK3" + "mP7xR2wT"

func ztwWorkloadGroups(t *testing.T) resources.ResourceSpec {
	t.Helper()
	spec, ok := resources.Catalog().FindSpec(resources.ProductZTW, "workload-groups")
	if !ok {
		t.Fatal("ztw/workload-groups missing from the catalog")
	}
	return spec
}

// nestedRecord builds {path[0]: {path[1]: ... value}}.
func nestedRecord(path []string, value any) map[string]any {
	for i := len(path) - 1; i > 0; i-- {
		value = map[string]any{path[i]: value}
	}
	return map[string]any{path[0]: value}
}
