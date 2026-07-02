//go:build ignore

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"strconv"
	"strings"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintf(os.Stderr, "usage: go run ./scripts/verify-machine-manifest-schema.go SCHEMA FIXTURE\n")
		os.Exit(2)
	}
	schema, err := readJSONFile(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "read schema: %v\n", err)
		os.Exit(1)
	}
	fixture, err := readJSONFile(os.Args[2])
	if err != nil {
		fmt.Fprintf(os.Stderr, "read fixture: %v\n", err)
		os.Exit(1)
	}
	var errs []error
	validateJSONSchemaSubset("$", schema, fixture, schema, &errs)
	if len(errs) > 0 {
		fmt.Fprintln(os.Stderr, "machine manifest schema validation failed:")
		for _, err := range errs {
			fmt.Fprintf(os.Stderr, "  - %v\n", err)
		}
		os.Exit(1)
	}
	fmt.Println("machine manifest schema validation: PASS")
}

func readJSONFile(path string) (map[string]any, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	var doc map[string]any
	if err := decoder.Decode(&doc); err != nil {
		return nil, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return nil, errors.New("trailing JSON values")
		}
		return nil, err
	}
	return doc, nil
}

func validateJSONSchemaSubset(path string, schema map[string]any, value any, root map[string]any, errs *[]error) {
	if ref, ok := stringValue(schema["$ref"]); ok {
		target, err := resolveLocalRef(root, ref)
		if err != nil {
			*errs = append(*errs, fmt.Errorf("%s: %v", path, err))
			return
		}
		validateJSONSchemaSubset(path, target, value, root, errs)
		return
	}

	if constValue, ok := schema["const"]; ok && !jsonValuesEqual(value, constValue) {
		*errs = append(*errs, fmt.Errorf("%s: got %v, want const %v", path, value, constValue))
	}
	if enumValues, ok := schema["enum"].([]any); ok && !containsJSONValue(enumValues, value) {
		*errs = append(*errs, fmt.Errorf("%s: got %v, want one of %v", path, value, enumValues))
	}

	if typ, ok := stringValue(schema["type"]); ok && !jsonTypeMatches(typ, value) {
		*errs = append(*errs, fmt.Errorf("%s: got JSON type %s, want %s", path, jsonTypeName(value), typ))
		return
	}

	if min, ok := schema["minimum"]; ok {
		if number, ok := value.(json.Number); ok {
			got, err := strconv.ParseFloat(number.String(), 64)
			if err != nil {
				*errs = append(*errs, fmt.Errorf("%s: invalid JSON number %q", path, number.String()))
			} else if want, ok := jsonNumberAsFloat(min); ok && got < want {
				*errs = append(*errs, fmt.Errorf("%s: got %v, want minimum %v", path, got, want))
			}
		}
	}

	if obj, ok := value.(map[string]any); ok {
		validateObject(path, schema, obj, root, errs)
	}
	if arr, ok := value.([]any); ok {
		validateArray(path, schema, arr, root, errs)
	}
}

func validateObject(path string, schema map[string]any, obj map[string]any, root map[string]any, errs *[]error) {
	props, _ := schema["properties"].(map[string]any)
	if required, ok := schema["required"].([]any); ok {
		for _, item := range required {
			name, ok := item.(string)
			if !ok {
				*errs = append(*errs, fmt.Errorf("%s: non-string required entry %v", path, item))
				continue
			}
			if _, exists := obj[name]; !exists {
				*errs = append(*errs, fmt.Errorf("%s: missing required property %q", path, name))
			}
		}
	}
	if additional, ok := schema["additionalProperties"].(bool); ok && !additional {
		for key := range obj {
			if _, known := props[key]; !known {
				*errs = append(*errs, fmt.Errorf("%s: unexpected property %q", path, key))
			}
		}
	}
	for key, rawProp := range props {
		value, exists := obj[key]
		if !exists {
			continue
		}
		prop, ok := rawProp.(map[string]any)
		if !ok {
			*errs = append(*errs, fmt.Errorf("%s.%s: property schema is not an object", path, key))
			continue
		}
		validateJSONSchemaSubset(path+"."+key, prop, value, root, errs)
	}
}

func validateArray(path string, schema map[string]any, arr []any, root map[string]any, errs *[]error) {
	items, ok := schema["items"].(map[string]any)
	if !ok {
		return
	}
	for i, value := range arr {
		validateJSONSchemaSubset(fmt.Sprintf("%s[%d]", path, i), items, value, root, errs)
	}
}

func resolveLocalRef(root map[string]any, ref string) (map[string]any, error) {
	const prefix = "#/$defs/"
	if !strings.HasPrefix(ref, prefix) {
		return nil, fmt.Errorf("unsupported $ref %q", ref)
	}
	defs, ok := root["$defs"].(map[string]any)
	if !ok {
		return nil, errors.New("root schema has no $defs object")
	}
	name := strings.TrimPrefix(ref, prefix)
	target, ok := defs[name].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("missing $defs entry %q", name)
	}
	return target, nil
}

func jsonTypeMatches(typ string, value any) bool {
	switch typ {
	case "object":
		_, ok := value.(map[string]any)
		return ok
	case "array":
		_, ok := value.([]any)
		return ok
	case "string":
		_, ok := value.(string)
		return ok
	case "boolean":
		_, ok := value.(bool)
		return ok
	case "integer":
		number, ok := value.(json.Number)
		return ok && strings.IndexAny(number.String(), ".eE") == -1
	default:
		return true
	}
}

func jsonTypeName(value any) string {
	switch value.(type) {
	case map[string]any:
		return "object"
	case []any:
		return "array"
	case string:
		return "string"
	case bool:
		return "boolean"
	case json.Number:
		return "number"
	case nil:
		return "null"
	default:
		return fmt.Sprintf("%T", value)
	}
}

func stringValue(value any) (string, bool) {
	got, ok := value.(string)
	return got, ok
}

func jsonNumberAsFloat(value any) (float64, bool) {
	switch v := value.(type) {
	case json.Number:
		got, err := strconv.ParseFloat(v.String(), 64)
		return got, err == nil
	case float64:
		return v, true
	default:
		return 0, false
	}
}

func containsJSONValue(values []any, want any) bool {
	for _, value := range values {
		if jsonValuesEqual(want, value) {
			return true
		}
	}
	return false
}

func jsonValuesEqual(left any, right any) bool {
	if leftNumber, ok := left.(json.Number); ok {
		if rightNumber, ok := right.(json.Number); ok {
			return leftNumber.String() == rightNumber.String()
		}
	}
	return reflect.DeepEqual(left, right)
}
