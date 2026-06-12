package zscaler

// Parent/child catalog-consistency test for the zia locations family.
// locations and sublocations share one SDK struct
// (locationmanagement.Locations), so every field name promoted on the family
// must carry the identical classification and AllowedModes on both catalog
// entries (including nested reference sub-fields). A divergence would mean
// the same physical attribute is exposed differently on parent and child.
// The behavioral (projection) tests for these fields live in
// reader_fields_locations_test.go.

import (
	"testing"

	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

// TestLocationFamilyParentChildClassificationConsistency asserts that for
// every field name promoted in wave 4, the locations and sublocations catalog
// entries carry the identical classification and AllowedModes (including
// nested reference sub-fields).
func TestLocationFamilyParentChildClassificationConsistency(t *testing.T) {
	t.Parallel()

	wave4Promoted := []string{
		"geoOverride",
		"ipv6Enabled",
		"ipv6Dns64Prefix",
		"defaultExtranetTsPool",
		"defaultExtranetDns",
		"extranet",
		"extranetIpPool",
		"extranetDns",
		"subLocScopeEnabled",
		"subLocScope",
		"subLocScopeValues",
		"subLocAccIds",
		"otherSubLocation",
		"other6SubLocation",
		"ecLocation",
		"dynamiclocationGroups",
		"staticLocationGroups",
		"virtualZenClusters",
		"virtualZens",
	}

	locSpec, ok := resources.FindSpec(resources.ProductZIA, resourceLocations)
	if !ok {
		t.Fatalf("FindSpec(zia, locations) ok = false, want true")
	}
	subSpec, ok := resources.FindSpec(resources.ProductZIA, resourceSublocations)
	if !ok {
		t.Fatalf("FindSpec(zia, sublocations) ok = false, want true")
	}

	for _, name := range wave4Promoted {
		locField, ok := fieldSpecByName(locSpec.Fields, name)
		if !ok {
			t.Errorf("locations catalog missing promoted field %s", name)
			continue
		}
		subField, ok := fieldSpecByName(subSpec.Fields, name)
		if !ok {
			t.Errorf("sublocations catalog missing promoted field %s", name)
			continue
		}
		assertFieldSpecsEqual(t, name, locField, subField)
	}
}

// assertFieldSpecsEqual checks that two FieldSpecs (parent vs child) match on
// classification, allowed modes, and nested sub-field shape.
func assertFieldSpecsEqual(t *testing.T, name string, a, b resources.FieldSpec) {
	t.Helper()

	if a.Classification != b.Classification {
		t.Errorf("%s class mismatch: locations=%s sublocations=%s", name, a.Classification, b.Classification)
	}
	if !sameModes(a.AllowedModes, b.AllowedModes) {
		t.Errorf("%s modes mismatch: locations=%v sublocations=%v", name, a.AllowedModes, b.AllowedModes)
	}
	if len(a.Fields) != len(b.Fields) {
		t.Errorf("%s nested field count mismatch: locations=%d sublocations=%d", name, len(a.Fields), len(b.Fields))
		return
	}
	for _, child := range a.Fields {
		other, ok := fieldSpecByName(b.Fields, child.JSONField())
		if !ok {
			t.Errorf("%s.%s present on locations but not sublocations", name, child.JSONField())
			continue
		}
		assertFieldSpecsEqual(t, name+"."+child.JSONField(), child, other)
	}
}

func sameModes(a, b []redact.Mode) bool {
	if len(a) != len(b) {
		return false
	}
	seen := make(map[redact.Mode]int, len(a))
	for _, m := range a {
		seen[redact.EffectiveMode(m)]++
	}
	for _, m := range b {
		seen[redact.EffectiveMode(m)]--
	}
	for _, count := range seen {
		if count != 0 {
			return false
		}
	}
	return true
}
