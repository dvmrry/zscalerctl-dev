package dump

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

// TestPublishedSchemasMatchStructs guards the hand-written JSON Schemas under
// docs/schema/ against drift: every JSON field on the dump artifact structs must
// appear in the schema's properties and vice versa. This is a field-name sync
// check (not a full JSON Schema validation), mirroring the catalog shape-review
// pattern, so new or removed struct fields cannot silently diverge from the
// published contract.
func TestPublishedSchemasMatchStructs(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		typ        reflect.Type
		schemaFile string
		propsPath  []string // JSON pointer segments to the "properties" object
	}{
		{"Manifest", reflect.TypeOf(Manifest{}), "manifest.schema.json", []string{"properties"}},
		{"ManifestResource", reflect.TypeOf(ManifestResource{}), "manifest.schema.json", []string{"$defs", "manifestResource", "properties"}},
		{"RedactionReport", reflect.TypeOf(RedactionReport{}), "redaction-report.schema.json", []string{"properties"}},
		{"ResourceReport", reflect.TypeOf(ResourceReport{}), "redaction-report.schema.json", []string{"$defs", "resourceReport", "properties"}},
		{"ResourceError", reflect.TypeOf(ResourceError{}), "dump-error.schema.json", []string{"properties"}},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			structFields := jsonFieldNames(tc.typ)
			schemaFields := schemaPropertyKeys(t, tc.schemaFile, tc.propsPath)

			if !reflect.DeepEqual(structFields, schemaFields) {
				t.Errorf("%s: struct JSON fields and %s properties differ\n struct: %v\n schema: %v\n missing-from-schema: %v\n extra-in-schema: %v",
					tc.name, tc.schemaFile, structFields, schemaFields,
					setDiff(structFields, schemaFields), setDiff(schemaFields, structFields))
			}
		})
	}
}

// TestManifestStatusValuesMatchSchemaEnums drives a real dump containing both a
// successful and a failed resource, then asserts every status value the code
// emits is a member of the corresponding schema enum. The field-name check
// above does not look at enum values, so this is what catches a code/schema
// status-vocabulary mismatch (e.g. emitting "complete" where the per-resource
// enum is ["ok","error"]).
func TestManifestStatusValuesMatchSchemaEnums(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "dump")
	entry := projectedDumpEntry(t, resources.ProductZIA, "locations", []resources.SourceRecord{
		resources.NewSourceRecord(map[string]any{"id": 1, "name": "HQ", "description": ""}),
	})
	result := Result{
		Entries: []ResourceDump{entry},
		Errors:  []ResourceError{NewResourceError(resources.ProductZIA, "rule-labels", "list", "list_failed")},
	}
	if err := Write(dir, redact.ModeStandard, result); err != nil {
		t.Fatalf("Write(partial dump) error = %v, want nil", err)
	}

	var manifest Manifest
	readJSON(t, filepath.Join(dir, "manifest.json"), &manifest)

	topEnum := schemaEnum(t, "manifest.schema.json", []string{"properties", "status"})
	if !contains(topEnum, manifest.Status) {
		t.Errorf("manifest status %q not in schema enum %v", manifest.Status, topEnum)
	}
	resEnum := schemaEnum(t, "manifest.schema.json", []string{"$defs", "manifestResource", "properties", "status"})
	for _, r := range manifest.Resources {
		if !contains(resEnum, r.Status) {
			t.Errorf("manifest resource %s/%s status %q not in schema enum %v", r.Product, r.Name, r.Status, resEnum)
		}
	}
}

func schemaEnum(t *testing.T, file string, path []string) []string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("..", "..", "docs", "schema", file))
	if err != nil {
		t.Fatalf("read %s: %v", file, err)
	}
	var doc map[string]any
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatalf("parse %s: %v", file, err)
	}
	var cur any = doc
	for _, seg := range path {
		m, ok := cur.(map[string]any)
		if !ok {
			t.Fatalf("%s: %q is not an object while walking %v", file, seg, path)
		}
		cur, ok = m[seg]
		if !ok {
			t.Fatalf("%s: missing %q while walking %v", file, seg, path)
		}
	}
	node, ok := cur.(map[string]any)
	if !ok {
		t.Fatalf("%s: node at %v is not an object", file, path)
	}
	raw, ok := node["enum"].([]any)
	if !ok {
		t.Fatalf("%s: no enum at %v", file, path)
	}
	values := make([]string, 0, len(raw))
	for _, v := range raw {
		s, ok := v.(string)
		if !ok {
			t.Fatalf("%s: non-string enum value %v at %v", file, v, path)
		}
		values = append(values, s)
	}
	return values
}

func contains(values []string, want string) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}
	return false
}

func jsonFieldNames(typ reflect.Type) []string {
	var names []string
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if field.PkgPath != "" {
			continue
		}
		tag := field.Tag.Get("json")
		if tag == "" || tag == "-" {
			continue
		}
		if idx := strings.IndexByte(tag, ','); idx >= 0 {
			tag = tag[:idx]
		}
		if tag != "" {
			names = append(names, tag)
		}
	}
	sort.Strings(names)
	return names
}

func schemaPropertyKeys(t *testing.T, file string, path []string) []string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("..", "..", "docs", "schema", file))
	if err != nil {
		t.Fatalf("read %s: %v", file, err)
	}
	var doc map[string]any
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatalf("parse %s: %v", file, err)
	}
	var cur any = doc
	for _, seg := range path {
		m, ok := cur.(map[string]any)
		if !ok {
			t.Fatalf("%s: %q is not an object while walking %v", file, seg, path)
		}
		cur, ok = m[seg]
		if !ok {
			t.Fatalf("%s: missing %q while walking %v", file, seg, path)
		}
	}
	props, ok := cur.(map[string]any)
	if !ok {
		t.Fatalf("%s: properties at %v is not an object", file, path)
	}
	keys := make([]string, 0, len(props))
	for k := range props {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func setDiff(a, b []string) []string {
	set := make(map[string]struct{}, len(b))
	for _, v := range b {
		set[v] = struct{}{}
	}
	var out []string
	for _, v := range a {
		if _, ok := set[v]; !ok {
			out = append(out, v)
		}
	}
	return out
}
