package zscaler

import (
	"testing"

	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

// TestCatalogPathsAreEmitted fails on a catalog path its reader never
// emits. Such a path is listed by `schema list` and accepted by --fields,
// --filter and --count-by, yet always comes back empty, so an agent reads
// "no value" where the field cannot be read at all. It runs every handler's
// record mapper on the synthetic samples the field type generator uses
// (sampleCatalog) and requires each field that renders in any mode, nested
// ones included, to appear in at least one sample projected in standard
// mode, so a field rendering in share or paranoid but not standard fails
// too. Secret fields render in no mode and are skipped. A path the reader no
// longer fills is removed from the catalog; one the reader never mapped is
// mapped or removed.
func TestCatalogPathsAreEmitted(t *testing.T) {
	t.Parallel()

	for _, sampled := range sampleCatalog(t) {
		key := resourceReviewKey(sampled.spec.Product, sampled.spec.Name)
		for _, path := range unemittedPaths("", sampled.spec.Fields, sampled.records) {
			t.Errorf("%s/%s: catalog path is never emitted by its reader", key, path)
		}
	}
}

// unemittedPaths returns the fields that render in some mode yet are absent
// from every object. A missing field is reported once; the fields beneath it
// are not.
func unemittedPaths(prefix string, fields []resources.FieldSpec, objects []map[string]any) []string {
	var missing []string
	for _, field := range fields {
		if !rendersInSomeMode(field) {
			continue
		}
		path := prefix + field.JSONField()
		var children []map[string]any
		present := false
		for _, object := range objects {
			value, ok := object[field.JSONField()]
			if !ok {
				continue
			}
			present = true
			children = append(children, sampleObjects(value)...)
		}
		if !present {
			missing = append(missing, path)
			continue
		}
		if len(field.Fields) > 0 {
			missing = append(missing, unemittedPaths(path+".", field.Fields, children)...)
		}
	}
	return missing
}

func rendersInSomeMode(field resources.FieldSpec) bool {
	for _, mode := range []redact.Mode{redact.ModeStandard, redact.ModeShare, redact.ModeParanoid} {
		if field.AllowedIn(mode) {
			return true
		}
	}
	return false
}
