package resources

import (
	"reflect"
	"strings"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/redact"
)

func TestFieldReferenceTablesMatchCatalog(t *testing.T) {
	t.Parallel()

	for _, err := range FieldReferenceErrors(Catalog()) {
		t.Error(err)
	}
}

// TestEveryReferenceFieldIsClassified keeps the reference tables complete:
// every reference-shaped field (an object or list with an id, or a scalar
// named ...Id/...Ids) is in fieldReferences or unmappedReferences, so a new
// catalog field cannot silently drop out of `schema references`. Run one
// product with -run 'TestEveryReferenceFieldIsClassified/zpa'.
func TestEveryReferenceFieldIsClassified(t *testing.T) {
	t.Parallel()

	byProduct := map[Product][]ResourceSpec{}
	for _, spec := range Catalog() {
		byProduct[spec.Product] = append(byProduct[spec.Product], spec)
	}
	for product, specs := range byProduct {
		t.Run(string(product), func(t *testing.T) {
			t.Parallel()
			for _, spec := range specs {
				key := specKey(spec)
				for _, path := range referenceShapedPaths(spec.Fields, "") {
					_, isMapped := fieldReferences[key][path]
					_, isUnmapped := unmappedReferences[key][path]
					if !isMapped && !isUnmapped {
						t.Errorf("%s.%s is reference-shaped but in neither fieldReferences nor unmappedReferences", key, path)
					}
				}
			}
		})
	}
}

// referenceShapedPaths lists the fields that look like references: an object
// or list with an id child, or a scalar named ...Id/...Ids. It descends into
// every object, including reference objects, so a reference nested inside
// another (cloudApplicationTenant.tenantId) is found too.
func referenceShapedPaths(fields []FieldSpec, prefix string) []string {
	var out []string
	for _, field := range fields {
		name := field.JSONField()
		path := prefix + name
		hasID := fieldAtPath(field.Fields, "id") != nil
		idNamed := name != "id" && (strings.HasSuffix(name, "Id") || strings.HasSuffix(name, "Ids"))
		if hasID || (idNamed && len(field.Fields) == 0) {
			out = append(out, path)
		}
		out = append(out, referenceShapedPaths(field.Fields, path+".")...)
	}
	return out
}

// TestDemotedReferencesStayUnmapped pins mappings removed in review because
// the id cannot belong to the target, so a later table edit cannot quietly
// bring them back.
func TestDemotedReferencesStayUnmapped(t *testing.T) {
	t.Parallel()

	for key, paths := range map[string][]string{
		// An integer id; browser-isolation-profiles is keyed by UUID.
		"zia/url-filtering-rules": {"cbiProfileId"},
		// No evidence these ids are casb-tenants tenantIds.
		"zia/casb-dlp-rules":     {"cloudAppTenants"},
		"zia/casb-malware-rules": {"cloudAppTenants", "cloudAppTenantIds"},
		// The SDK comment is copied from proxyGateway.
		"ztw/traffic-dns-rules": {"zpaIpGroup"},
	} {
		for _, path := range paths {
			if targets, ok := fieldReferences[key][path]; ok {
				t.Errorf("%s.%s maps to %v; it was demoted in review", key, path, targets)
			}
			if _, ok := unmappedReferences[key][path]; !ok {
				t.Errorf("%s.%s is missing from unmappedReferences", key, path)
			}
		}
	}
}

// TestCrossProductReferencesArePinned pins the ZIA-to-ZPA links: the SDK
// documents these externalId values as the ZPA-side ids ("an external
// identifier used for an entity that is managed outside of ZIA. Examples
// include zpaServerGroup and zpaAppSegments").
func TestCrossProductReferencesArePinned(t *testing.T) {
	t.Parallel()

	for key, want := range map[string]map[string][]ResourceRef{
		"zia/zpa-gateways": {
			"zpaServerGroup.externalId": zpaRef("server-groups"),
			"zpaAppSegments.externalId": zpaRef("application-segments"),
		},
		"zia/firewall-filtering-rules": {"zpaAppSegments.externalId": zpaRef("application-segments")},
		"zia/ssl-inspection-rules":     {"zpaAppSegments.externalId": zpaRef("application-segments")},
	} {
		for path, targets := range want {
			if got := fieldReferences[key][path]; !reflect.DeepEqual(got, targets) {
				t.Errorf("%s.%s targets %v, want %v", key, path, got, targets)
			}
		}
	}
}

func TestCatalogFieldsCarryReferences(t *testing.T) {
	t.Parallel()

	spec, ok := Catalog().FindSpec(ProductZIA, "url-filtering-rules")
	if !ok {
		t.Fatal("zia/url-filtering-rules missing from catalog")
	}
	// A rule's locations may name a location or a sublocation.
	locations := fieldAtPath(spec.Fields, "locations")
	if locations == nil || !reflect.DeepEqual(locations.References, ziaRef("locations", "sublocations")) {
		t.Fatalf("url-filtering-rules.locations References = %+v, want zia/locations and zia/sublocations", locations)
	}
	if timeWindows := fieldAtPath(spec.Fields, "timeWindows"); timeWindows == nil || timeWindows.References != nil {
		t.Errorf("url-filtering-rules.timeWindows References = %+v, want unmapped", timeWindows)
	}
}

func TestReferencesToListsMatchPathsAndModes(t *testing.T) {
	t.Parallel()

	location := ziaRef("locations")[0]
	doc := ReferencesTo(Catalog(), location)
	if doc.Target != location {
		t.Errorf("Target = %v, want zia/locations", doc.Target)
	}
	want := ReferenceSite{
		Product:      ProductZIA,
		Resource:     "url-filtering-rules",
		Field:        "locations",
		Match:        "locations.id",
		AllowedModes: []redact.Mode{redact.ModeStandard, redact.ModeShare},
		Targets:      ziaRef("locations", "sublocations"),
	}
	if !containsSite(doc.ReferredBy, want) {
		t.Errorf("ReferredBy = %+v, want it to contain %+v", doc.ReferredBy, want)
	}
	// The same field is listed under its other target.
	if !containsSite(ReferencesTo(Catalog(), ziaRef("sublocations")[0]).ReferredBy, want) {
		t.Errorf("ReferredBy(zia/sublocations) is missing %+v", want)
	}

	// A flat string list matches on the field itself.
	categories := ReferencesTo(Catalog(), ziaRef("url-categories")[0])
	wantCategories := ReferenceSite{
		Product:      ProductZIA,
		Resource:     "url-filtering-rules",
		Field:        "urlCategories",
		Match:        "urlCategories",
		AllowedModes: []redact.Mode{redact.ModeStandard, redact.ModeShare},
		Targets:      ziaRef("url-categories"),
	}
	if !containsSite(categories.ReferredBy, wantCategories) {
		t.Errorf("ReferredBy = %+v, want it to contain %+v", categories.ReferredBy, wantCategories)
	}

	// An admin audit reference: the id renders in every mode, so the
	// reverse index gives its filter path.
	admins := ReferencesTo(Catalog(), ziaRef("admin-users")[0])
	for _, site := range admins.ReferredBy {
		if site.Resource == "url-filtering-rules" && site.Field == "lastModifiedBy" {
			if site.Match != "lastModifiedBy.id" || len(site.AllowedModes) != 3 {
				t.Errorf("lastModifiedBy = %+v, want match lastModifiedBy.id in every mode", site)
			}
			return
		}
	}
	t.Errorf("ReferredBy(admin-users) = %+v, want url-filtering-rules.lastModifiedBy", admins.ReferredBy)
}

func TestReferencesToUnreferencedResourceIsEmptyList(t *testing.T) {
	t.Parallel()

	doc := ReferencesTo(Catalog(), ResourceRef{Product: ProductZIA, Resource: "activation-status"})
	if doc.ReferredBy == nil || len(doc.ReferredBy) != 0 {
		t.Errorf("ReferredBy = %#v, want empty non-nil list", doc.ReferredBy)
	}
}

func containsSite(sites []ReferenceSite, want ReferenceSite) bool {
	for _, site := range sites {
		if reflect.DeepEqual(site, want) {
			return true
		}
	}
	return false
}
