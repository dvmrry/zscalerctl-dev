package zscaler

import (
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/resources"
)

func TestReviewedSDKShapesMatchCatalogOrIgnoredRegistry(t *testing.T) {
	t.Parallel()

	for _, shape := range reviewedSDKShapes() {
		shape := shape
		t.Run(shape.name, func(t *testing.T) {
			t.Parallel()
			shape.assertReviewed(t)
		})
	}
}

func TestReviewedSDKShapesCoverCatalogReadResources(t *testing.T) {
	t.Parallel()

	reviewed := map[string]struct{}{}
	for _, shape := range reviewedSDKShapes() {
		if shape.resource == "" && shape.resourceName == "" {
			continue
		}
		reviewed[resourceReviewKey(shape.resource, shape.resourceName)] = struct{}{}
	}

	var missing []string
	for _, spec := range resources.Catalog() {
		if !specHasReadOperation(spec) {
			continue
		}
		key := resourceReviewKey(spec.Product, spec.Name)
		if _, ok := reviewed[key]; !ok {
			missing = append(missing, key)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Fatalf("reviewedSDKShapes() missing top-level SDK shape reviews for catalog read resources: %v", missing)
	}
}

type sdkShapeReview struct {
	name          string
	resource      resources.Product
	resourceName  string
	typ           reflect.Type
	catalogFields []string
	ignoredFields map[string]string
}

func (s sdkShapeReview) assertReviewed(t *testing.T) {
	t.Helper()

	catalog := map[string]struct{}{}
	if s.resource != "" || s.resourceName != "" {
		spec, ok := resources.FindSpec(s.resource, s.resourceName)
		if !ok {
			t.Fatalf("resources.FindSpec(%s, %s) ok = false, want true", s.resource, s.resourceName)
		}
		catalog = catalogFieldNames(spec)
	}

	classified := namesSet(s.catalogFields)
	for _, field := range s.catalogFields {
		if _, ok := catalog[field]; !ok {
			t.Errorf("%s catalog field %q missing from %s/%s", s.name, field, s.resource, s.resourceName)
		}
	}

	for field, reason := range s.ignoredFields {
		if strings.TrimSpace(reason) == "" {
			t.Errorf("%s ignored field %q has empty reason", s.name, field)
		}
		if _, ok := classified[field]; ok {
			t.Errorf("%s field %q is both catalog-classified and ignored", s.name, field)
		}
	}

	var missing []string
	exported := exportedJSONFields(s.typ)
	for _, field := range exported {
		if _, ok := classified[field]; ok {
			continue
		}
		if _, ok := s.ignoredFields[field]; ok {
			continue
		}
		missing = append(missing, field)
	}
	if len(missing) > 0 {
		t.Errorf("%s SDK fields missing catalog classification or ignore reason: %v", s.name, missing)
	}

	exportedSet := namesSet(exported)
	var stale []string
	for field := range s.ignoredFields {
		if _, ok := exportedSet[field]; !ok {
			stale = append(stale, field)
		}
	}
	sort.Strings(stale)
	if len(stale) > 0 {
		t.Errorf("%s ignored SDK fields no longer exist: %v", s.name, stale)
	}
}

func reviewedSDKShapes() []sdkShapeReview {
	// Register top-level SDK resource structs and every nested SDK helper struct
	// that reader mappings intentionally traverse. A struct-typed field ignored
	// at a parent level still needs its own review entry when its fields are
	// mapped into SourceRecord data, so SDK bumps cannot add nested fields
	// without an explicit classify-or-ignore decision.
	var shapes []sdkShapeReview
	shapes = append(shapes, reviewedSDKShapesZIA()...)
	shapes = append(shapes, reviewedSDKShapesZPA()...)
	shapes = append(shapes, reviewedSDKShapesZTW()...)
	shapes = append(shapes, reviewedSDKShapesZidentity()...)
	return shapes
}

func catalogFieldNames(spec resources.ResourceSpec) map[string]struct{} {
	fields := map[string]struct{}{}
	collectCatalogFieldNames(fields, spec.Fields)
	return fields
}

func collectCatalogFieldNames(out map[string]struct{}, fields []resources.FieldSpec) {
	for _, field := range fields {
		out[field.JSONField()] = struct{}{}
		collectCatalogFieldNames(out, field.Fields)
	}
}

func catalogFieldsFor(product resources.Product, name string) []string {
	spec, ok := resources.FindSpec(product, name)
	if !ok {
		panic("missing catalog spec " + resourceReviewKey(product, name))
	}
	fields := make([]string, 0, len(spec.Fields))
	for _, field := range spec.Fields {
		fields = append(fields, field.JSONField())
	}
	sort.Strings(fields)
	return fields
}

func exportedJSONFields(typ reflect.Type) []string {
	if typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	var fields []string
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if field.PkgPath != "" {
			continue
		}
		name := jsonFieldName(field)
		if name == "" || name == "-" {
			continue
		}
		fields = append(fields, name)
	}
	sort.Strings(fields)
	return fields
}

func jsonFieldName(field reflect.StructField) string {
	tag := field.Tag.Get("json")
	if tag == "-" {
		return "-"
	}
	if index := strings.IndexByte(tag, ','); index >= 0 {
		tag = tag[:index]
	}
	if tag != "" {
		return tag
	}
	return field.Name
}

func ignoredBecause(reason string, fields ...string) map[string]string {
	ignored := map[string]string{}
	for _, field := range fields {
		ignored[field] = reason
	}
	return ignored
}

func mergeIgnoredFields(items ...map[string]string) map[string]string {
	merged := map[string]string{}
	for _, item := range items {
		for field, reason := range item {
			merged[field] = reason
		}
	}
	return merged
}

func namesSet(names []string) map[string]struct{} {
	out := make(map[string]struct{}, len(names))
	for _, name := range names {
		out[name] = struct{}{}
	}
	return out
}

func specHasReadOperation(spec resources.ResourceSpec) bool {
	for _, op := range spec.Operations {
		if op.Capability == resources.CapabilityRead {
			return true
		}
	}
	return false
}

func resourceReviewKey(product resources.Product, name string) string {
	return string(product) + "/" + name
}
