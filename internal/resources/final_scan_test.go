package resources

import (
	"strings"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/redact"
)

// TestFinalScanKeepsCatalogAllowedValues pins that the final output scan
// never replaces a value the catalog renders just because of its key. The
// scan replaces values under keys ending in a secret word (password, token,
// session, ...) as a backstop for unreviewed JSON; a reviewed field with such
// a name (ZCC refreshKerberosToken, an integer setting) must still show its
// value. A new catalog field that fails here needs a decision: a numeric
// setting goes in redact's numericSettingKeys; anything else means changing
// the key rule or classifying the field.
func TestFinalScanKeepsCatalogAllowedValues(t *testing.T) {
	t.Parallel()
	var walk func(spec ResourceSpec, fields []FieldSpec, chain []FieldSpec)
	walk = func(spec ResourceSpec, fields []FieldSpec, chain []FieldSpec) {
		for _, field := range fields {
			next := append(append([]FieldSpec(nil), chain...), field)
			for _, sample := range finalScanSamples(field) {
				doc := `{"` + field.JSONField() + `":` + sample + `}`
				for _, mode := range availabilityAlong(next).RenderableIn {
					if got, _ := redact.New(mode).ScanString(doc); got != doc {
						t.Errorf("%s/%s field %s (%s): final scan in %s turns %s into %s",
							spec.Product, spec.Name, field.JSONField(), field.Type, mode, doc, got)
					}
				}
			}
			walk(spec, field.Fields, next)
		}
	}
	for _, spec := range Catalog() {
		walk(spec, spec.Fields, nil)
	}
}

// Each reviewed numeric setting key the final scan exempts is a catalog field
// that renders as a number, so the exemption cannot outlive its reason.
func TestNumericSettingKeysAreCatalogFields(t *testing.T) {
	t.Parallel()
	rendered := map[string]bool{}
	var walk func(fields []FieldSpec, chain []FieldSpec)
	walk = func(fields []FieldSpec, chain []FieldSpec) {
		for _, field := range fields {
			next := append(append([]FieldSpec(nil), chain...), field)
			if (field.Type == "integer" || field.Type == "number") && len(availabilityAlong(next).RenderableIn) > 0 {
				rendered[field.JSONField()] = true
			}
			walk(field.Fields, next)
		}
	}
	for _, spec := range Catalog() {
		walk(spec.Fields, nil)
	}
	for _, key := range redact.NumericSettingKeys() {
		if !rendered[key] {
			t.Errorf("numeric setting key %q is not a rendered numeric catalog field", key)
		}
	}
}

// finalScanSamples returns benign JSON values of a field's type; an untyped
// field gets one of every type. The rule under test reads the key and the
// value's type only, so one benign value per type is enough; a rule that
// looked at depth or the value itself would need samples per path.
func finalScanSamples(field FieldSpec) []string {
	samples := map[string]string{
		"string": `"Example setting"`, "integer": `1`, "number": `1.5`,
		"boolean": `true`, "object": `{}`, "array": `[]`,
	}
	kind := field.Type
	if strings.HasPrefix(kind, "array") {
		kind = "array"
	}
	if sample, ok := samples[kind]; ok {
		return []string{sample}
	}
	out := make([]string, 0, len(samples))
	for _, sample := range samples {
		out = append(out, sample)
	}
	return out
}
