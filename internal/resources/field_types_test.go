package resources_test

import (
	"testing"

	"github.com/dvmrry/zscalerctl/internal/resources"
)

// TestFieldTypesFitCatalogShape pins that a field with nested catalog fields
// is typed as an object or a list of objects, or not at all: a scalar type
// there would contradict the nested names schema list shows under it.
func TestFieldTypesFitCatalogShape(t *testing.T) {
	t.Parallel()

	var walk func(prefix string, fields []resources.FieldSpec)
	walk = func(prefix string, fields []resources.FieldSpec) {
		for _, field := range fields {
			path := prefix + field.JSONField()
			if len(field.Fields) > 0 {
				switch field.Type {
				case "", "object", "array<object>":
				default:
					t.Errorf("%s has nested fields but type %q", path, field.Type)
				}
			}
			walk(path+".", field.Fields)
		}
	}
	typed := 0
	for _, spec := range resources.Catalog() {
		walk(string(spec.Product)+"/"+spec.Name+"/", spec.Fields)
		for _, field := range spec.Fields {
			if field.Type != "" {
				typed++
			}
		}
	}
	if typed == 0 {
		t.Fatal("no catalog field has a type; is field_types_gen.go applied?")
	}
}
