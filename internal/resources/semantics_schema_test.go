package resources_test

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

const resourceSemanticsSchemaFile = "resource-semantics-v1.schema.json"

// TestPublishedResourceSemanticsSchemaValidatesCatalogOutputs exercises the
// candidate schema with the actual config-free output for every catalog
// resource and every supported redaction mode. Keeping the validator here
// small and limited to the JSON Schema vocabulary used by this published
// document avoids adding a runtime dependency just for a test contract.
func TestPublishedResourceSemanticsSchemaValidatesCatalogOutputs(t *testing.T) {
	t.Parallel()

	schema := readResourceSemanticsSchema(t)
	description, ok := schema["description"].(string)
	if !ok || !strings.Contains(description, "Candidate") || !strings.Contains(description, "pilot") {
		t.Fatalf("%s description = %q, want candidate pilot wording", resourceSemanticsSchemaFile, description)
	}

	modes := []redact.Mode{
		redact.ModeStandard,
		redact.ModeShare,
		redact.ModeParanoid,
	}
	for _, spec := range resources.Catalog() {
		spec := spec
		t.Run(string(spec.Product)+"/"+spec.Name, func(t *testing.T) {
			t.Parallel()
			for _, mode := range modes {
				mode := mode
				t.Run(string(mode), func(t *testing.T) {
					t.Parallel()
					got, err := resources.DescribeSemantics(spec, mode)
					if err != nil {
						t.Fatalf("DescribeSemantics(%s/%s, %s) error = %v, want nil", spec.Product, spec.Name, mode, err)
					}
					body, err := json.Marshal(got)
					if err != nil {
						t.Fatalf("json.Marshal(DescribeSemantics(%s/%s, %s)) error = %v, want nil", spec.Product, spec.Name, mode, err)
					}
					var value any
					if err := json.Unmarshal(body, &value); err != nil {
						t.Fatalf("json.Unmarshal(DescribeSemantics(%s/%s, %s)) error = %v, want nil", spec.Product, spec.Name, mode, err)
					}
					validateResourceSemanticsSchema(t, schema, schema, value, "$")
				})
			}
		})
	}
}

// TestPublishedResourceSemanticsSchemaMatchesDTOs follows the repository's
// published-schema drift checks: every JSON-tagged field on the DTOs appears
// in the matching schema properties, and every schema property is represented
// by a DTO field.
func TestPublishedResourceSemanticsSchemaMatchesDTOs(t *testing.T) {
	t.Parallel()

	schema := readResourceSemanticsSchema(t)
	cases := []struct {
		name      string
		typ       reflect.Type
		propsPath []string
	}{
		{"Semantics", reflect.TypeOf(resources.Semantics{}), []string{"properties"}},
		{"Operation", reflect.TypeOf(resources.Operation{}), []string{"$defs", "operation", "properties"}},
		{"FieldSemantics", reflect.TypeOf(resources.FieldSemantics{}), []string{"$defs", "fieldSemantics", "properties"}},
		{"CollectionSemantics", reflect.TypeOf(resources.CollectionSemantics{}), []string{"$defs", "collectionSemantics", "properties"}},
		{"ReferenceSemantics", reflect.TypeOf(resources.ReferenceSemantics{}), []string{"$defs", "referenceSemantics", "properties"}},
		{"SemanticSource", reflect.TypeOf(resources.SemanticSource{}), []string{"$defs", "semanticSource", "properties"}},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			structFields := resourceSemanticsJSONFieldNames(tc.typ)
			schemaFields := resourceSemanticsSchemaPropertyKeys(t, schema, tc.propsPath)
			if !reflect.DeepEqual(structFields, schemaFields) {
				t.Errorf("%s: struct JSON fields and %s properties differ\n struct: %v\n schema: %v\n missing-from-schema: %v\n extra-in-schema: %v",
					tc.name,
					resourceSemanticsSchemaFile,
					structFields,
					schemaFields,
					resourceSemanticsSetDiff(structFields, schemaFields),
					resourceSemanticsSetDiff(schemaFields, structFields),
				)
			}
		})
	}
}

func readResourceSemanticsSchema(t *testing.T) map[string]any {
	t.Helper()

	body, err := os.ReadFile(filepath.Join("..", "..", "docs", "schema", resourceSemanticsSchemaFile))
	if err != nil {
		t.Fatalf("read %s: %v", resourceSemanticsSchemaFile, err)
	}
	var schema map[string]any
	if err := json.Unmarshal(body, &schema); err != nil {
		t.Fatalf("parse %s: %v", resourceSemanticsSchemaFile, err)
	}
	return schema
}

func resourceSemanticsSchemaPropertyKeys(t *testing.T, schema map[string]any, path []string) []string {
	t.Helper()

	var current any = schema
	for _, segment := range path {
		object, ok := current.(map[string]any)
		if !ok {
			t.Fatalf("%s: %q is not an object while walking %v", resourceSemanticsSchemaFile, segment, path)
		}
		var exists bool
		current, exists = object[segment]
		if !exists {
			t.Fatalf("%s: missing %q while walking %v", resourceSemanticsSchemaFile, segment, path)
		}
	}
	properties, ok := current.(map[string]any)
	if !ok {
		t.Fatalf("%s: node at %v is not an object", resourceSemanticsSchemaFile, path)
	}
	keys := make([]string, 0, len(properties))
	for key := range properties {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func resourceSemanticsJSONFieldNames(typ reflect.Type) []string {
	var names []string
	for index := 0; index < typ.NumField(); index++ {
		field := typ.Field(index)
		if field.PkgPath != "" {
			continue
		}
		tag := field.Tag.Get("json")
		if tag == "" || tag == "-" {
			continue
		}
		if comma := strings.IndexByte(tag, ','); comma >= 0 {
			tag = tag[:comma]
		}
		if tag != "" {
			names = append(names, tag)
		}
	}
	sort.Strings(names)
	return names
}

func resourceSemanticsSetDiff(a, b []string) []string {
	set := make(map[string]struct{}, len(b))
	for _, value := range b {
		set[value] = struct{}{}
	}
	var difference []string
	for _, value := range a {
		if _, ok := set[value]; !ok {
			difference = append(difference, value)
		}
	}
	return difference
}

// validateResourceSemanticsSchema implements the subset of draft 2020-12 used
// by resource-semantics-v1.schema.json: local $ref, type, const, enum,
// required, properties, additionalProperties, items, minLength, and
// minItems. It deliberately fails on the first path-specific mismatch so a
// schema or DTO drift reports the resource and mode that exposed it.
func validateResourceSemanticsSchema(t *testing.T, root, schema map[string]any, value any, path string) {
	t.Helper()

	if reference, ok := schema["$ref"].(string); ok {
		const prefix = "#/$defs/"
		if !strings.HasPrefix(reference, prefix) {
			t.Fatalf("%s: unsupported schema reference %q", path, reference)
		}
		name := strings.TrimPrefix(reference, prefix)
		defs, ok := root["$defs"].(map[string]any)
		if !ok {
			t.Fatalf("%s: schema has reference %q but no $defs object", path, reference)
		}
		definition, ok := defs[name].(map[string]any)
		if !ok {
			t.Fatalf("%s: schema reference %q is missing", path, reference)
		}
		validateResourceSemanticsSchema(t, root, definition, value, path)
		return
	}

	if expected, ok := schema["const"]; ok && !reflect.DeepEqual(expected, value) {
		t.Fatalf("%s: value %v does not equal schema const %v", path, value, expected)
	}
	if rawEnum, ok := schema["enum"].([]any); ok {
		matched := false
		for _, allowed := range rawEnum {
			if reflect.DeepEqual(allowed, value) {
				matched = true
				break
			}
		}
		if !matched {
			t.Fatalf("%s: value %v is not in schema enum %v", path, value, rawEnum)
		}
	}

	schemaType, _ := schema["type"].(string)
	switch schemaType {
	case "object":
		object, ok := value.(map[string]any)
		if !ok {
			t.Fatalf("%s: value has Go type %T, want JSON object", path, value)
		}
		if rawRequired, ok := schema["required"].([]any); ok {
			for _, rawName := range rawRequired {
				name, ok := rawName.(string)
				if !ok {
					t.Fatalf("%s: schema required entry %v is not a string", path, rawName)
				}
				if _, present := object[name]; !present {
					t.Fatalf("%s: required property %q is missing", path, name)
				}
			}
		}
		properties, _ := schema["properties"].(map[string]any)
		if additional, exists := schema["additionalProperties"]; exists {
			if allowed, isBoolean := additional.(bool); isBoolean && !allowed {
				for name := range object {
					if _, declared := properties[name]; !declared {
						t.Fatalf("%s: property %q is not declared by schema", path, name)
					}
				}
			}
		}
		for name, rawChild := range properties {
			child, ok := rawChild.(map[string]any)
			if !ok {
				t.Fatalf("%s.%s: schema property is not an object", path, name)
			}
			if childValue, present := object[name]; present {
				validateResourceSemanticsSchema(t, root, child, childValue, path+"."+name)
			}
		}

	case "array":
		array, ok := value.([]any)
		if !ok {
			t.Fatalf("%s: value has Go type %T, want JSON array", path, value)
		}
		if minimum, ok := schema["minItems"].(float64); ok && len(array) < int(minimum) {
			t.Fatalf("%s: array has %d items, want at least %d", path, len(array), int(minimum))
		}
		if rawItems, ok := schema["items"].(map[string]any); ok {
			for index, item := range array {
				validateResourceSemanticsSchema(t, root, rawItems, item, fmt.Sprintf("%s[%d]", path, index))
			}
		}

	case "string":
		stringValue, ok := value.(string)
		if !ok {
			t.Fatalf("%s: value has Go type %T, want JSON string", path, value)
		}
		if minimum, ok := schema["minLength"].(float64); ok && utf8.RuneCountInString(stringValue) < int(minimum) {
			t.Fatalf("%s: string length is %d, want at least %d", path, utf8.RuneCountInString(stringValue), int(minimum))
		}

	case "boolean":
		if _, ok := value.(bool); !ok {
			t.Fatalf("%s: value has Go type %T, want JSON boolean", path, value)
		}

	case "integer":
		number, ok := value.(float64)
		if !ok || math.Trunc(number) != number {
			t.Fatalf("%s: value %v has Go type %T, want JSON integer", path, value, value)
		}

	case "number":
		if _, ok := value.(float64); !ok {
			t.Fatalf("%s: value has Go type %T, want JSON number", path, value)
		}
	}
}
