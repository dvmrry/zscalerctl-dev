package resources_test

import (
	"encoding/json"
	"errors"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/output"
	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

func TestDescribeSemanticsPilotFieldsAndModes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		product  resources.Product
		resource string
		want     map[string]resources.SemanticValueType
	}{
		{
			name:     "locations",
			product:  resources.ProductZIA,
			resource: "locations",
			want: map[string]resources.SemanticValueType{
				"id":          resources.SemanticTypeInteger,
				"name":        resources.SemanticTypeString,
				"parentId":    resources.SemanticTypeInteger,
				"country":     resources.SemanticTypeString,
				"tz":          resources.SemanticTypeString,
				"ipAddresses": resources.SemanticTypeStringList,
				"ports":       resources.SemanticTypeIntegerList,
				"profile":     resources.SemanticTypeString,
				"description": resources.SemanticTypeString,
				"ipv6Enabled": resources.SemanticTypeBoolean,
			},
		},
		{
			name:     "url-filtering-rules",
			product:  resources.ProductZIA,
			resource: "url-filtering-rules",
			want: map[string]resources.SemanticValueType{
				"id":             resources.SemanticTypeInteger,
				"name":           resources.SemanticTypeString,
				"order":          resources.SemanticTypeInteger,
				"state":          resources.SemanticTypeString,
				"action":         resources.SemanticTypeString,
				"protocols":      resources.SemanticTypeStringList,
				"urlCategories":  resources.SemanticTypeStringList,
				"requestMethods": resources.SemanticTypeStringList,
				"description":    resources.SemanticTypeString,
				"blockOverride":  resources.SemanticTypeBoolean,
				"labels":         resources.SemanticTypeReferenceList,
				"locations":      resources.SemanticTypeReferenceList,
			},
		},
		{
			name:     "rule-labels",
			product:  resources.ProductZIA,
			resource: "rule-labels",
			want: map[string]resources.SemanticValueType{
				"id":                  resources.SemanticTypeInteger,
				"name":                resources.SemanticTypeString,
				"description":         resources.SemanticTypeString,
				"lastModifiedTime":    resources.SemanticTypeInteger,
				"referencedRuleCount": resources.SemanticTypeInteger,
			},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			spec, ok := resources.FindSpec(test.product, test.resource)
			if !ok {
				t.Fatalf("FindSpec(%q, %q) ok = false, want true", test.product, test.resource)
			}
			for _, mode := range []redact.Mode{
				redact.ModeStandard,
				redact.ModeShare,
				redact.ModeParanoid,
			} {
				got, err := resources.DescribeSemantics(spec, mode)
				if err != nil {
					t.Fatalf("DescribeSemantics(%s/%s, %s) error = %v, want nil", test.product, test.resource, mode, err)
				}
				if got.Schema != resources.SemanticsSchema {
					t.Errorf("DescribeSemantics(%s/%s, %s).Schema = %q, want %q", test.product, test.resource, mode, got.Schema, resources.SemanticsSchema)
				}
				if got.Version != resources.SemanticsVersion {
					t.Errorf("DescribeSemantics(%s/%s, %s).Version = %q, want %q", test.product, test.resource, mode, got.Version, resources.SemanticsVersion)
				}
				if got.ReviewStatus != resources.SemanticsReviewed {
					t.Errorf("DescribeSemantics(%s/%s, %s).ReviewStatus = %q, want %q", test.product, test.resource, mode, got.ReviewStatus, resources.SemanticsReviewed)
				}
				if got.RedactionMode != mode {
					t.Errorf("DescribeSemantics(%s/%s, %s).RedactionMode = %q, want %q", test.product, test.resource, mode, got.RedactionMode, mode)
				}
				if !got.ReadOnly {
					t.Errorf("DescribeSemantics(%s/%s, %s).ReadOnly = false, want true", test.product, test.resource, mode)
				}
				if got.Shape != spec.EffectiveShape() {
					t.Errorf("DescribeSemantics(%s/%s, %s).Shape = %q, want %q", test.product, test.resource, mode, got.Shape, spec.EffectiveShape())
				}
				if got.GetKey != spec.EffectiveGetKey() {
					t.Errorf("DescribeSemantics(%s/%s, %s).GetKey = %q, want %q", test.product, test.resource, mode, got.GetKey, spec.EffectiveGetKey())
				}

				catalogFields := make(map[string]resources.FieldSpec, len(spec.Fields))
				for _, field := range spec.Fields {
					catalogFields[field.JSONField()] = field
				}
				seen := make(map[string]bool, len(got.Fields))
				for _, field := range got.Fields {
					catalogField, exists := catalogFields[field.Name]
					if !exists {
						t.Errorf("DescribeSemantics(%s/%s, %s) field %q is not in catalog, want an existing field", test.product, test.resource, mode, field.Name)
						continue
					}
					if seen[field.Name] {
						t.Errorf("DescribeSemantics(%s/%s, %s) field %q is duplicated, want one descriptor", test.product, test.resource, mode, field.Name)
					}
					seen[field.Name] = true
					if !catalogField.AllowedIn(mode) {
						t.Errorf("DescribeSemantics(%s/%s, %s) field %q is not allowed by the catalog mode gate, want it omitted", test.product, test.resource, mode, field.Name)
					}
					if field.Classification != catalogField.Classification {
						t.Errorf("DescribeSemantics(%s/%s, %s) field %q classification = %q, want catalog %q", test.product, test.resource, mode, field.Name, field.Classification, catalogField.Classification)
					}
					if !reflect.DeepEqual(field.AllowedModes, catalogField.AllowedModes) {
						t.Errorf("DescribeSemantics(%s/%s, %s) field %q allowed modes = %v, want catalog %v", test.product, test.resource, mode, field.Name, field.AllowedModes, catalogField.AllowedModes)
					}
					if field.Description == "" {
						t.Errorf("DescribeSemantics(%s/%s, %s) field %q description = empty, want reviewed SDK description", test.product, test.resource, mode, field.Name)
					}
					if field.Enum != nil {
						t.Errorf("DescribeSemantics(%s/%s, %s) field %q enum = %v, want nil because no closed set was verified", test.product, test.resource, mode, field.Name, field.Enum)
					}
					if field.Source.SDKPath == "" || field.Source.CatalogPath == "" {
						t.Errorf("DescribeSemantics(%s/%s, %s) field %q source = %#v, want SDK and catalog paths", test.product, test.resource, mode, field.Name, field.Source)
					}
				}

				wantNames := namesAllowedInMode(test.want, spec.Fields, mode)
				if gotNames := semanticFieldNames(got.Fields); !reflect.DeepEqual(gotNames, wantNames) {
					t.Errorf("DescribeSemantics(%s/%s, %s) field names = %v, want %v", test.product, test.resource, mode, gotNames, wantNames)
				}
				for name, wantType := range test.want {
					if field, ok := semanticField(got.Fields, name); ok && field.Type != wantType {
						t.Errorf("DescribeSemantics(%s/%s, %s) field %q type = %q, want %q", test.product, test.resource, mode, name, field.Type, wantType)
					}
				}
			}
		})
	}
}

func TestDescribeSemanticsReferenceTargetsAndCollectionOrdering(t *testing.T) {
	t.Parallel()

	spec, ok := resources.FindSpec(resources.ProductZIA, "url-filtering-rules")
	if !ok {
		t.Fatal("FindSpec(zia, url-filtering-rules) ok = false, want true")
	}
	got, err := resources.DescribeSemantics(spec, redact.ModeStandard)
	if err != nil {
		t.Fatalf("DescribeSemantics(zia/url-filtering-rules, standard) error = %v, want nil", err)
	}

	wantTargets := map[string]string{
		"labels":    "rule-labels",
		"locations": "locations",
	}
	for fieldName, wantResource := range wantTargets {
		field, ok := semanticField(got.Fields, fieldName)
		if !ok {
			t.Fatalf("DescribeSemantics(zia/url-filtering-rules, standard) field %q present = false, want true", fieldName)
		}
		if field.Reference == nil {
			t.Fatalf("DescribeSemantics(zia/url-filtering-rules, standard) field %q reference = nil, want target zia/%s", fieldName, wantResource)
		}
		if field.Reference.Product != resources.ProductZIA || field.Reference.Resource != wantResource {
			t.Errorf("DescribeSemantics(zia/url-filtering-rules, standard) field %q reference = %#v, want zia/%s", fieldName, field.Reference, wantResource)
		}
		if _, ok := resources.FindSpec(field.Reference.Product, field.Reference.Resource); !ok {
			t.Errorf("DescribeSemantics(zia/url-filtering-rules, standard) field %q reference target = %s/%s, want a catalog resource", fieldName, field.Reference.Product, field.Reference.Resource)
		}
		if field.Collection == nil {
			t.Fatalf("DescribeSemantics(zia/url-filtering-rules, standard) field %q collection = nil, want collection metadata", fieldName)
		}
		if field.Collection.Ordering != resources.CollectionOrderingUnknown {
			t.Errorf("DescribeSemantics(zia/url-filtering-rules, standard) field %q collection ordering = %q, want %q", fieldName, field.Collection.Ordering, resources.CollectionOrderingUnknown)
		}
	}
}

func TestDescribeSemanticsKnownResourceOutsidePilot(t *testing.T) {
	t.Parallel()

	spec, ok := resources.FindSpec(resources.ProductZIA, "static-ips")
	if !ok {
		t.Fatal("FindSpec(zia, static-ips) ok = false, want true")
	}
	got, err := resources.DescribeSemantics(spec, redact.ModeShare)
	if err != nil {
		t.Fatalf("DescribeSemantics(zia/static-ips, share) error = %v, want nil", err)
	}
	if got.ReviewStatus != resources.SemanticsNotReviewed {
		t.Errorf("DescribeSemantics(zia/static-ips, share).ReviewStatus = %q, want %q", got.ReviewStatus, resources.SemanticsNotReviewed)
	}
	if len(got.Fields) != 0 {
		t.Errorf("DescribeSemantics(zia/static-ips, share).Fields = %#v, want empty for unreviewed resource", got.Fields)
	}
	if got.Product != spec.Product || got.Resource != spec.Name || got.GetKey != spec.EffectiveGetKey() {
		t.Errorf("DescribeSemantics(zia/static-ips, share) identity = %s/%s get_key=%q, want %s/%s get_key=%q", got.Product, got.Resource, got.GetKey, spec.Product, spec.Name, spec.EffectiveGetKey())
	}
}

func TestDescribeSemanticsRejectsInvalidModeAndPilotIdentity(t *testing.T) {
	t.Parallel()

	spec, ok := resources.FindSpec(resources.ProductZIA, "locations")
	if !ok {
		t.Fatal("FindSpec(zia, locations) ok = false, want true")
	}
	if _, err := resources.DescribeSemantics(spec, redact.Mode("invalid")); !errors.Is(err, resources.ErrInvalidSemanticsSpec) {
		t.Errorf("DescribeSemantics(zia/locations, invalid) error = %v, want ErrInvalidSemanticsSpec", err)
	}

	invalid := spec
	invalid.GetKey = "name"
	if _, err := resources.DescribeSemantics(invalid, redact.ModeStandard); !errors.Is(err, resources.ErrInvalidSemanticsSpec) {
		t.Errorf("DescribeSemantics(zia/locations with get_key=name, standard) error = %v, want ErrInvalidSemanticsSpec", err)
	}
}

func TestDescribeSemanticsReturnsIndependentSafeSnapshots(t *testing.T) {
	t.Parallel()

	spec, ok := resources.FindSpec(resources.ProductZIA, "url-filtering-rules")
	if !ok {
		t.Fatal("FindSpec(zia, url-filtering-rules) ok = false, want true")
	}
	got, err := resources.DescribeSemantics(spec, redact.ModeStandard)
	if err != nil {
		t.Fatalf("DescribeSemantics(zia/url-filtering-rules, standard) error = %v, want nil", err)
	}
	var _ output.SafeJSON = got
	body, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("json.Marshal(DescribeSemantics(zia/url-filtering-rules, standard)) error = %v, want nil", err)
	}
	if strings.Contains(string(body), "tenant-value-canary") {
		t.Fatalf("json.Marshal(DescribeSemantics(zia/url-filtering-rules, standard)) = %s, want no tenant values", body)
	}

	got.Operations[0].Name = "mutated"
	got.Fields[0].AllowedModes[0] = redact.Mode("mutated")
	for i := range got.Fields {
		if got.Fields[i].Reference != nil {
			got.Fields[i].Reference.Resource = "mutated"
		}
		if got.Fields[i].Collection != nil {
			got.Fields[i].Collection.Ordering = "mutated"
		}
	}

	again, err := resources.DescribeSemantics(spec, redact.ModeStandard)
	if err != nil {
		t.Fatalf("DescribeSemantics(zia/url-filtering-rules, standard) after mutation error = %v, want nil", err)
	}
	if again.Operations[0].Name == "mutated" || again.Fields[0].AllowedModes[0] == redact.Mode("mutated") {
		t.Fatalf("DescribeSemantics(zia/url-filtering-rules, standard) returned mutable shared state: %#v", again)
	}
	for _, field := range again.Fields {
		if field.Reference != nil && field.Reference.Resource == "mutated" {
			t.Fatalf("DescribeSemantics(zia/url-filtering-rules, standard) shared reference state: %#v", field)
		}
		if field.Collection != nil && field.Collection.Ordering == "mutated" {
			t.Fatalf("DescribeSemantics(zia/url-filtering-rules, standard) shared collection state: %#v", field)
		}
	}
}

func namesAllowedInMode(want map[string]resources.SemanticValueType, fields []resources.FieldSpec, mode redact.Mode) []string {
	var names []string
	for _, field := range fields {
		if _, selected := want[field.JSONField()]; selected && field.AllowedIn(mode) {
			names = append(names, field.JSONField())
		}
	}
	sort.Strings(names)
	return names
}

func semanticFieldNames(fields []resources.FieldSemantics) []string {
	names := make([]string, 0, len(fields))
	for _, field := range fields {
		names = append(names, field.Name)
	}
	sort.Strings(names)
	return names
}

func semanticField(fields []resources.FieldSemantics, name string) (resources.FieldSemantics, bool) {
	for _, field := range fields {
		if field.Name == name {
			return field, true
		}
	}
	return resources.FieldSemantics{}, false
}
