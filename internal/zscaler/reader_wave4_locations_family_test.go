package zscaler

// Wave-4 field-coverage tests for the zia locations family (locations and
// sublocations). Wave 4 drives the remaining deferred geo/extranet/IPv6 and
// scope/membership-reference fields to a classification on BOTH resources. The
// two resources share one SDK struct (locationmanagement.Locations), so every
// field name promoted here is classified identically on parent and child; the
// parent/child consistency test below enforces that.
//
// The shared fixture carries a distinctive value in every newly-mapped field,
// a canary in every nested secret sub-field (reference extensions), and a
// canary in every still-excluded field. The tests project the source record
// through the resources catalog and assert: promoted fields surface under the
// right keys in standard mode, mode widening/narrowing matches each field's
// AllowedModes (4+ fields), secret/never-rendered nested fields stay out, and
// no excluded- or secret-field canary leaks into any projection.
//
// This is a NEW file; it reuses package helpers (projectOneRecordInMode,
// assertFieldsAbsent, assertNoCanaries, mustProjectedMap, mustProjectedList,
// fieldSpecByName) from the shared test files in this package.

import (
	"reflect"
	"testing"

	"github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/common"
	"github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/location/locationmanagement"

	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

const (
	// Embedded in nested reference extensions (secret) and in still-excluded
	// fields; must never appear in any projection of either resource.
	wave4FamilySecretCanary = "wave4-family-extensions-canary"
)

// wave4FamilyFixture sets every wave-4 newly-mapped field with a distinctive
// value. Reference lists carry an id, a tenant-config name, and an extensions
// map whose only value is a secret canary. The scope value and account-id
// lists carry standard-only sensitive identifiers.
func wave4FamilyFixture() locationmanagement.Locations {
	ext := map[string]interface{}{"hidden": wave4FamilySecretCanary}
	return locationmanagement.Locations{
		ID:       7001,
		Name:     "Family Fixture",
		ParentID: 7000,
		// Group 1: geo / extranet / IPv6 posture booleans.
		GeoOverride:           true,
		IPv6Enabled:           true,
		IPv6Dns64Prefix:       true,
		DefaultExtranetTsPool: true,
		DefaultExtranetDns:    true,
		// Group 1: extranet reference objects (common.IDCustom = id + name).
		Extranet:       &common.IDCustom{ID: 811, Name: "extranet-corp"},
		ExtranetIpPool: &common.IDCustom{ID: 812, Name: "extranet-ts-pool"},
		ExtranetDns:    &common.IDCustom{ID: 813, Name: "extranet-dns"},
		// Group 2: scope and membership references.
		SubLocScopeEnabled: true,
		SubLocScope:        "WORKLOAD",
		SubLocScopeValues:  []string{"scope-value-7777"},
		SubLocAccIDs:       []string{"123456789012"},
		OtherSubLocation:   true,
		Other6SubLocation:  true,
		ECLocation:         true,
		DynamiclocationGroups: []common.IDNameExtensions{
			{ID: 901, Name: "dyn-group-a", Extensions: ext},
		},
		StaticLocationGroups: []common.IDNameExtensions{
			{ID: 902, Name: "static-group-a", Extensions: ext},
		},
		VirtualZenClusters: []common.IDNameExtensions{
			{ID: 903, Name: "vzen-cluster-a", Extensions: ext},
		},
		VirtualZens: []common.IDNameExtensions{
			{ID: 904, Name: "vzen-node-a", Extensions: ext},
		},
		// Still-excluded fields carry the secret canary; they must never render.
		ChildCount:               7777,
		MatchInChild:             true,
		DisplayTimeUnit:          "MINUTE-" + wave4FamilySecretCanary,
		SurrogateRefreshTimeUnit: "HOUR-" + wave4FamilySecretCanary,
		ExcludeFromDynamicGroups: true,
		ExcludeFromManualGroups:  true,
	}
}

// wave4FamilyResources returns the (resourceName, sourceRecord) pairs so each
// assertion runs identically against locations and sublocations.
func wave4FamilyResources(t *testing.T) []struct {
	name   string
	record resources.SourceRecord
} {
	t.Helper()
	fixture := wave4FamilyFixture()
	return []struct {
		name   string
		record resources.SourceRecord
	}{
		{resourceLocations, locationSourceRecord(fixture)},
		{resourceSublocations, sublocationSourceRecord(fixture)},
	}
}

// TestWave4FamilyPromotedFieldsSurfaceInStandardMode asserts that every newly
// mapped field renders in standard mode on both resources, including nested
// reference id/name and the scope identifier lists.
func TestWave4FamilyPromotedFieldsSurfaceInStandardMode(t *testing.T) {
	t.Parallel()

	for _, res := range wave4FamilyResources(t) {
		got := projectOneRecordInMode(t, resources.ProductZIA, res.name, redact.ModeStandard,
			[]resources.SourceRecord{res.record})

		// Posture booleans and the scope enable flag / scope type.
		scalarWant := map[string]any{
			"geoOverride":           true,
			"ipv6Enabled":           true,
			"ipv6Dns64Prefix":       true,
			"defaultExtranetTsPool": true,
			"defaultExtranetDns":    true,
			"subLocScopeEnabled":    true,
			"subLocScope":           "WORKLOAD",
			"otherSubLocation":      true,
			"other6SubLocation":     true,
			"ecLocation":            true,
		}
		for field, want := range scalarWant {
			if value, ok := got[field]; !ok || !reflect.DeepEqual(value, want) {
				t.Errorf("%s standard %s = %v (present=%v), want %v", res.name, field, value, ok, want)
			}
		}

		// Standard-only sensitive identifier lists render in standard mode.
		// These map as []string (like ipAddresses), not []any.
		if value, ok := got["subLocScopeValues"]; !ok || !reflect.DeepEqual(value, []string{"scope-value-7777"}) {
			t.Errorf("%s standard subLocScopeValues = %v (present=%v), want [scope-value-7777]", res.name, value, ok)
		}
		if value, ok := got["subLocAccIds"]; !ok || !reflect.DeepEqual(value, []string{"123456789012"}) {
			t.Errorf("%s standard subLocAccIds = %v (present=%v), want [123456789012]", res.name, value, ok)
		}

		// Extranet references project id (operational) and name (tenant-config).
		for field, wantName := range map[string]string{
			"extranet":       "extranet-corp",
			"extranetIpPool": "extranet-ts-pool",
			"extranetDns":    "extranet-dns",
		} {
			ref := mustProjectedMap(t, got, field)
			if ref["name"] != wantName {
				t.Errorf("%s standard %s.name = %v, want %q", res.name, field, ref["name"], wantName)
			}
			if _, ok := ref["id"]; !ok {
				t.Errorf("%s standard %s missing id", res.name, field)
			}
		}

		// Reference lists project id + name; extensions never surface.
		for field, wantName := range map[string]string{
			"dynamiclocationGroups": "dyn-group-a",
			"staticLocationGroups":  "static-group-a",
			"virtualZenClusters":    "vzen-cluster-a",
			"virtualZens":           "vzen-node-a",
		} {
			items := mustProjectedList(t, got, field)
			first, ok := items[0].(map[string]any)
			if !ok {
				t.Fatalf("%s standard %s[0] = %T, want map[string]any", res.name, field, items[0])
			}
			if first["name"] != wantName {
				t.Errorf("%s standard %s[0].name = %v, want %q", res.name, field, first["name"], wantName)
			}
			if _, ok := first["extensions"]; ok {
				t.Errorf("%s standard %s[0] leaked extensions = %#v", res.name, field, first)
			}
		}

		// Nested secret extensions never leak through any rendered string.
		assertNoCanaries(t, res.name, got, wave4FamilySecretCanary)
	}
}

// TestWave4FamilyHonorsAllowedModes exercises mode visibility for well more
// than four promoted fields across share and paranoid on both resources.
func TestWave4FamilyHonorsAllowedModes(t *testing.T) {
	t.Parallel()

	for _, res := range wave4FamilyResources(t) {
		records := []resources.SourceRecord{res.record}
		share := projectOneRecordInMode(t, resources.ProductZIA, res.name, redact.ModeShare, records)
		paranoid := projectOneRecordInMode(t, resources.ProductZIA, res.name, redact.ModeParanoid, records)

		// tenant-config + operational posture flags survive share, drop in
		// paranoid (they use standard+share modes).
		standardShare := []string{
			"geoOverride",
			"ipv6Enabled",
			"ipv6Dns64Prefix",
			"defaultExtranetTsPool",
			"defaultExtranetDns",
			"subLocScopeEnabled",
			"subLocScope",
			"otherSubLocation",
			"other6SubLocation",
			"ecLocation",
			"extranet",
			"extranetIpPool",
			"extranetDns",
		}
		for _, field := range standardShare {
			if _, ok := share[field]; !ok {
				t.Errorf("%s share-mode missing %s, want present", res.name, field)
			}
		}
		assertFieldsAbsent(t, res.name+" (paranoid)", paranoid, standardShare...)

		// Standard-only sensitive identifiers and the standard-only reference
		// lists drop in both share and paranoid.
		standardOnly := []string{
			"subLocScopeValues",
			"subLocAccIds",
			"dynamiclocationGroups",
			"staticLocationGroups",
			"virtualZenClusters",
			"virtualZens",
		}
		assertFieldsAbsent(t, res.name+" (share)", share, standardOnly...)
		assertFieldsAbsent(t, res.name+" (paranoid)", paranoid, standardOnly...)
	}
}

// TestWave4FamilyNeverLeaksSecretsOrExcluded confirms that across all three
// modes the nested reference extensions and the still-excluded fields stay out
// of the projection, and no canary leaks.
func TestWave4FamilyNeverLeaksSecretsOrExcluded(t *testing.T) {
	t.Parallel()

	for _, res := range wave4FamilyResources(t) {
		records := []resources.SourceRecord{res.record}
		for _, mode := range []redact.Mode{redact.ModeStandard, redact.ModeShare, redact.ModeParanoid} {
			got := projectOneRecordInMode(t, resources.ProductZIA, res.name, mode, records)

			assertFieldsAbsent(t, res.name, got,
				"displayTimeUnit",
				"surrogateRefreshTimeUnit",
				"childCount",
				"matchInChild",
				"excludeFromDynamicGroups",
				"excludeFromManualGroups",
			)
			assertNoCanaries(t, res.name, got, wave4FamilySecretCanary)
		}
	}
}

// TestWave4FamilyParentChildClassificationConsistency asserts that for every
// field name promoted in wave 4, the locations and sublocations catalog entries
// carry the identical classification and AllowedModes (including nested
// reference sub-fields). The two resources share one SDK struct; a divergence
// would mean the same physical attribute is exposed differently on parent and
// child.
func TestWave4FamilyParentChildClassificationConsistency(t *testing.T) {
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
