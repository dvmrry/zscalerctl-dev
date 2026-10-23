package zscaler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/format"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

// TestFieldTypesAreCurrent regenerates internal/resources/field_types_gen.go
// in memory and fails on any drift, so a reader or SDK change that alters
// what a field renders as changes the published schema visibly. Set
// FIELD_TYPES_WRITE=1 (`make field-types`) to rewrite the file instead.
//
// Types are observed, not declared: every resource handler's own record
// mapper runs on synthetic SDK values (sampleMapper), the records are
// projected through the catalog in standard mode, and each catalog path gets
// the JSON type of the values that reach it. Samples never leave a pointer
// or list unset, so a typed value may still be null. A path gets a type only when
// every sample that emits it agrees, so a field the reader derives from
// content, never emits, drops in projection (secrets, structured leaves), or
// takes from an interface{} SDK field gets none rather than a guess.
func TestFieldTypesAreCurrent(t *testing.T) {
	t.Parallel()

	want := renderFieldTypes(buildFieldTypes(t))
	path := filepath.Join("..", "resources", "field_types_gen.go")
	if os.Getenv("FIELD_TYPES_WRITE") == "1" {
		if err := os.WriteFile(path, want, 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
		t.Logf("wrote %s", path)
		return
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("%s is out of date; run `make field-types`", path)
	}
}

// TestFieldTypesCoverKnownShapes pins derivations a generator bug would
// break, including reader-built values that differ from the SDK field type.
func TestFieldTypesCoverKnownShapes(t *testing.T) {
	t.Parallel()

	types := buildFieldTypes(t)
	for path, want := range map[string]string{
		"zia/url-filtering-rules/id":                  "integer",
		"zia/url-filtering-rules/name":                "string",
		"zia/url-filtering-rules/enforceTimeValidity": "boolean",
		"zia/url-filtering-rules/urlCategories":       "array<string>",
		"zia/url-filtering-rules/groups":              "array<object>",
		"zia/url-filtering-rules/groups.id":           "integer",
		"zia/locations/sslScanEnabled":                "boolean",
		// The SDK holds a VM list; the reader emits its length.
		"ztw/ec-groups/ecVMs": "integer",
		// Untyped: a secret never renders.
		"zia/locations/preSharedKey": "",
		// Reader-built nested list of objects.
		"zcc/company-info/devicePostureFrequency":           "array<object>",
		"zcc/company-info/devicePostureFrequency.postureId": "integer",
	} {
		if got := types[path]; got != want {
			t.Errorf("field type %s = %q, want %q", path, got, want)
		}
	}
}

// TestFieldTypesAgreeWithReviewedSemantics checks the observed types against
// the hand-reviewed types in `schema describe`, which name a list of
// {id, name} references "array<reference>".
func TestFieldTypesAgreeWithReviewedSemantics(t *testing.T) {
	t.Parallel()

	types := buildFieldTypes(t)
	checked := 0
	for _, spec := range resources.Catalog() {
		semantics, err := resources.DescribeSemantics(spec, redact.ModeStandard)
		if err != nil {
			t.Fatalf("DescribeSemantics(%s/%s) error = %v", spec.Product, spec.Name, err)
		}
		for _, field := range semantics.Fields {
			want := string(field.Type)
			if want == "array<reference>" {
				want = "array<object>"
			}
			path := resourceReviewKey(spec.Product, spec.Name) + "/" + field.Name
			if got := types[path]; got != want {
				t.Errorf("field type %s = %q, reviewed semantics say %q", path, got, want)
			}
			checked++
		}
	}
	if checked == 0 {
		t.Fatal("no reviewed semantics fields checked")
	}
}

// recordSampler is implemented (in this file, for tests only) by the generic
// resource handlers: it maps synthetic values through the handler's own
// record mapper without calling the API.
type recordSampler interface {
	sampleRecords() []resources.SourceRecord
}

func (h listGetHandler[T]) sampleRecords() []resources.SourceRecord {
	return sampleMapper(h.sourceRecord)
}

func (h listOnlyHandler[T]) sampleRecords() []resources.SourceRecord {
	return sampleMapper(h.sourceRecord)
}

func (h singletonHandler[T]) sampleRecords() []resources.SourceRecord {
	return sampleMapper(h.sourceRecord)
}

// sampleStrings vary string inputs, since some mappers parse or branch on
// them; a path whose type depends on the input then disagrees across samples
// and stays untyped.
var sampleStrings = []string{"1", "x", "true", "2026-01-01T00:00:00Z", "SAMPLE_VALUE"}

func sampleMapper[T any](mapper func(T) resources.SourceRecord) []resources.SourceRecord {
	var out []resources.SourceRecord
	for variant := range sampleStrings {
		var item T
		fillSample(reflect.ValueOf(&item).Elem(), variant, 0)
		if record, ok := mapSample(mapper, item); ok {
			out = append(out, record)
		}
	}
	return out
}

func mapSample[T any](mapper func(T) resources.SourceRecord, item T) (record resources.SourceRecord, ok bool) {
	defer func() {
		if recover() != nil {
			ok = false
		}
	}()
	return mapper(item), true
}

// fillSample sets every exported field reachable from v: scalars to a sample
// value, lists to one element, maps to one entry. interface{} stays nil, so a
// field the SDK leaves open is never typed by a guess.
func fillSample(v reflect.Value, variant, depth int) {
	if depth > 8 || !v.CanSet() {
		return
	}
	switch v.Kind() {
	case reflect.String:
		v.SetString(sampleStrings[variant])
	case reflect.Bool:
		v.SetBool(variant%2 == 0)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v.SetInt(int64(variant + 1))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		v.SetUint(uint64(variant + 1))
	case reflect.Float32, reflect.Float64:
		v.SetFloat(1.5)
	case reflect.Pointer:
		elem := reflect.New(v.Type().Elem())
		fillSample(elem.Elem(), variant, depth+1)
		v.Set(elem)
	case reflect.Slice:
		list := reflect.MakeSlice(v.Type(), 1, 1)
		fillSample(list.Index(0), variant, depth+1)
		v.Set(list)
	case reflect.Map:
		m := reflect.MakeMapWithSize(v.Type(), 1)
		key := reflect.New(v.Type().Key()).Elem()
		fillSample(key, variant, depth+1)
		value := reflect.New(v.Type().Elem()).Elem()
		fillSample(value, variant, depth+1)
		m.SetMapIndex(key, value)
		v.Set(m)
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if v.Type().Field(i).IsExported() {
				fillSample(v.Field(i), variant, depth+1)
			}
		}
	}
}

// sampledResource is one catalog resource with its handler's sample records
// projected in standard mode.
type sampledResource struct {
	spec    resources.ResourceSpec
	records []map[string]any
}

// sampleCatalog runs every catalog resource's record mapper on synthetic
// samples. It fails when a handler cannot be sampled or yields no record.
func sampleCatalog(t *testing.T) []sampledResource {
	t.Helper()

	handlers := newResourceHandlers(sdkClient{})
	var out []sampledResource
	for _, spec := range resources.Catalog() {
		handler, ok := handlers[resourceKey{product: spec.Product, name: spec.Name}]
		if !ok {
			continue
		}
		sampler, ok := handler.(recordSampler)
		if !ok {
			t.Fatalf("%s/%s handler %T cannot be sampled", spec.Product, spec.Name, handler)
		}
		samples := sampler.sampleRecords()
		if len(samples) == 0 {
			t.Fatalf("%s/%s record mapper panicked on every sample", spec.Product, spec.Name)
		}
		projected, _, err := resources.ProjectRecordsAndVerify(spec, redact.ModeStandard, samples)
		if err != nil {
			t.Fatalf("project samples for %s/%s: %v", spec.Product, spec.Name, err)
		}
		sampled := sampledResource{spec: spec}
		for _, record := range projected.Records() {
			sampled.records = append(sampled.records, record.Fields())
		}
		out = append(out, sampled)
	}
	return out
}

func buildFieldTypes(t *testing.T) map[string]string {
	t.Helper()

	out := map[string]string{}
	for _, sampled := range sampleCatalog(t) {
		spec := sampled.spec
		seen := map[string]map[string]bool{}
		for _, record := range sampled.records {
			observeFieldTypes(seen, "", record, spec.Fields)
		}
		prefix := resourceReviewKey(spec.Product, spec.Name) + "/"
		for path, kinds := range seen {
			if len(kinds) == 1 {
				for kind := range kinds {
					out[prefix+path] = kind
				}
			}
		}
	}
	return out
}

func observeFieldTypes(seen map[string]map[string]bool, prefix string, object map[string]any, fields []resources.FieldSpec) {
	for _, field := range fields {
		value, ok := object[field.JSONField()]
		if !ok {
			continue
		}
		path := prefix + field.JSONField()
		if kind, ok := sampleJSONType(value); ok {
			if seen[path] == nil {
				seen[path] = map[string]bool{}
			}
			seen[path][kind] = true
		}
		for _, element := range sampleObjects(value) {
			observeFieldTypes(seen, path+".", element, field.Fields)
		}
	}
}

// sampleObjects returns the objects a value holds: itself, or a list's
// object elements.
func sampleObjects(value any) []map[string]any {
	if object, ok := value.(map[string]any); ok {
		return []map[string]any{object}
	}
	rv := reflect.ValueOf(value)
	if !rv.IsValid() || rv.Kind() != reflect.Slice {
		return nil
	}
	var out []map[string]any
	for i := 0; i < rv.Len(); i++ {
		if object, ok := rv.Index(i).Interface().(map[string]any); ok {
			out = append(out, object)
		}
	}
	return out
}

// sampleJSONType returns the JSON type of a projected value. A float holding
// a whole number is an integer: ZPA records pass through encoding/json, which
// decodes every number as float64, and float SDK fields are sampled as 1.5.
func sampleJSONType(value any) (string, bool) {
	if number, ok := value.(json.Number); ok {
		if strings.ContainsAny(number.String(), ".eE") {
			return "number", true
		}
		return "integer", true
	}
	rv := reflect.ValueOf(value)
	if !rv.IsValid() {
		return "", false
	}
	switch rv.Kind() {
	case reflect.String:
		return "string", true
	case reflect.Bool:
		return "boolean", true
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return "integer", true
	case reflect.Float32, reflect.Float64:
		if f := rv.Float(); f == math.Trunc(f) {
			return "integer", true
		}
		return "number", true
	case reflect.Map:
		return "object", true
	case reflect.Slice:
		elem := ""
		for i := 0; i < rv.Len(); i++ {
			kind, ok := sampleJSONType(rv.Index(i).Interface())
			if !ok || (elem != "" && kind != elem) {
				return "", false
			}
			elem = kind
		}
		if elem == "" {
			return "", false
		}
		return "array<" + elem + ">", true
	}
	return "", false
}

func renderFieldTypes(types map[string]string) []byte {
	paths := make([]string, 0, len(types))
	for path := range types {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	var b bytes.Buffer
	b.WriteString("// Code generated by TestFieldTypesAreCurrent in internal/zscaler; DO NOT EDIT.\n\n")
	b.WriteString("package resources\n\n")
	b.WriteString("// generatedFieldTypes maps product/resource/field.path to the JSON type the\n")
	b.WriteString("// reader emits for it, observed by sampling each resource's record mapper.\n")
	b.WriteString("var generatedFieldTypes = map[string]string{\n")
	for _, path := range paths {
		fmt.Fprintf(&b, "\t%q: %q,\n", path, types[path])
	}
	b.WriteString("}\n")
	formatted, err := format.Source(b.Bytes())
	if err != nil {
		panic(fmt.Sprintf("format generated field types: %v", err))
	}
	return formatted
}
