package zscaler

// Field-coverage tests for zia/locations. The location fixture carries a
// distinctive value in every promoted field, a canary in every secret field,
// and a canary in every intentionally-excluded field. The tests project the
// source record through the resources catalog and assert that promoted fields
// surface under the right keys in standard mode, that mode widening/narrowing
// matches each field's AllowedModes, that secret fields never surface, and
// that no excluded-field canary leaks into any projection.
//
// The location-family tests in the second half of this file cover the
// geo/extranet/IPv6 and scope/membership-reference fields on BOTH locations
// and sublocations: the two resources share one SDK struct
// (locationmanagement.Locations), so every field promoted there is classified
// identically on parent and child. reader_fields_consistency_test.go enforces
// that the two catalog entries stay identical.
//
// These tests reuse package helpers (projectOneRecord, projectOneRecordInMode,
// assertNoCanaries, assertFieldsAbsent, mustProjectedMap, mustProjectedList)
// from reader_test.go and reader_sourcerecord_test.go in this package.

import (
	"reflect"
	"testing"

	"github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/common"
	"github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/location/locationmanagement"

	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

const (
	wave1LocationPSKCanary      = "wave1-location-psk-canary"
	wave1LocationVPNFQDNCanary  = "vpn-fqdn-canary.example.com"
	wave1LocationExcludedCanary = "wave1-location-excluded-canary"
)

func wave1LocationFixture() locationmanagement.Locations {
	return locationmanagement.Locations{
		ID:          4501,
		Name:        "HQ Amsterdam",
		ParentID:    4400,
		Description: "wave1 location description",
		Country:     "NETHERLANDS",
		State:       "North Holland",
		TZ:          "EUROPE_AMSTERDAM",
		Language:    "ENGLISH",
		Profile:     "CORPORATE",
		UpBandwidth: 12000,
		DnBandwidth: 34000,
		IPAddresses: []string{"203.0.113.7"},
		Ports:       []int{8081, 9443},
		VPNCredentials: []locationmanagement.VPNCredentials{
			{
				ID:           91,
				Type:         "UFQDN",
				FQDN:         wave1LocationVPNFQDNCanary,
				PreSharedKey: wave1LocationPSKCanary,
				Comments:     "vpn comment " + wave1LocationExcludedCanary,
			},
		},
		AuthRequired:                        true,
		BasicAuthEnabled:                    true,
		DigestAuthEnabled:                   true,
		KerberosAuth:                        true,
		SSLScanEnabled:                      true,
		ZappSSLScanEnabled:                  true,
		XFFForwardEnabled:                   true,
		SurrogateIP:                         true,
		SurrogateIPEnforcedForKnownBrowsers: true,
		IdleTimeInMinutes:                   75,
		SurrogateRefreshTimeInMinutes:       105,
		OFWEnabled:                          true,
		IPSControl:                          true,
		AUPEnabled:                          true,
		CautionEnabled:                      true,
		AUPBlockInternetUntilAccepted:       true,
		AUPForceSSLInspection:               true,
		AUPTimeoutInDays:                    45,
		IOTDiscoveryEnabled:                 true,
		IOTEnforcePolicySet:                 true,
		CookiesAndProxy:                     true,
		// Wave-4 promoted fields carry distinctive, non-canary values; their
		// classification and mode behavior is asserted in the location-family
		// tests below. They are listed in the standard-mode want map below so
		// this exact-match test stays accurate.
		GeoOverride:        true,
		SubLocScopeEnabled: true,
		SubLocScope:        "WORKLOAD",
		OtherSubLocation:   true,
		// Intentionally excluded fields below carry canaries that must never
		// surface in any projection.
		ChildCount:               7777,
		MatchInChild:             true,
		DisplayTimeUnit:          "MINUTE-" + wave1LocationExcludedCanary,
		SurrogateRefreshTimeUnit: "HOUR-" + wave1LocationExcludedCanary,
		ExcludeFromDynamicGroups: true,
		ExcludeFromManualGroups:  true,
	}
}

func TestLocationSourceRecordProjectsPromotedFieldsInStandardMode(t *testing.T) {
	t.Parallel()

	location := wave1LocationFixture()
	got := projectOneRecord(t, resources.ProductZIA, resourceLocations, []resources.SourceRecord{locationSourceRecord(location)})

	want := map[string]any{
		"id":                                  4501,
		"name":                                "HQ Amsterdam",
		"parentId":                            4400,
		"description":                         "wave1 location description",
		"country":                             "NETHERLANDS",
		"state":                               "North Holland",
		"tz":                                  "EUROPE_AMSTERDAM",
		"language":                            "ENGLISH",
		"profile":                             "CORPORATE",
		"upBandwidth":                         12000,
		"dnBandwidth":                         34000,
		"ipAddresses":                         []string{"203.0.113.7"},
		"ports":                               []int{8081, 9443},
		"authRequired":                        true,
		"basicAuthEnabled":                    true,
		"digestAuthEnabled":                   true,
		"kerberosAuth":                        true,
		"sslScanEnabled":                      true,
		"zappSSLScanEnabled":                  true,
		"xffForwardEnabled":                   true,
		"surrogateIP":                         true,
		"surrogateIPEnforcedForKnownBrowsers": true,
		"idleTimeInMinutes":                   75,
		"surrogateRefreshTimeInMinutes":       105,
		"ofwEnabled":                          true,
		"ipsControl":                          true,
		"aupEnabled":                          true,
		"cautionEnabled":                      true,
		"aupBlockInternetUntilAccepted":       true,
		"aupForceSslInspection":               true,
		"aupTimeoutInDays":                    45,
		"iotDiscoveryEnabled":                 true,
		"iotEnforcePolicySet":                 true,
		"cookiesAndProxy":                     true,
		// Wave-4 promoted boolean/scope fields (always mapped; references and
		// scope-value lists are unset in this fixture so they stay absent).
		"geoOverride":           true,
		"ipv6Enabled":           false,
		"ipv6Dns64Prefix":       false,
		"defaultExtranetTsPool": false,
		"defaultExtranetDns":    false,
		"subLocScopeEnabled":    true,
		"subLocScope":           "WORKLOAD",
		"otherSubLocation":      true,
		"other6SubLocation":     false,
		"ecLocation":            false,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("projected locations standard mode = %#v, want %#v", got, want)
	}
}

func TestLocationSourceRecordHonorsAllowedModesAcrossShareAndParanoid(t *testing.T) {
	t.Parallel()

	location := wave1LocationFixture()
	records := []resources.SourceRecord{locationSourceRecord(location)}

	share := projectOneRecordInMode(t, resources.ProductZIA, resourceLocations, redact.ModeShare, records)
	paranoid := projectOneRecordInMode(t, resources.ProductZIA, resourceLocations, redact.ModeParanoid, records)

	// Standard+share fields stay visible in share mode and drop in paranoid.
	standardShare := map[string]any{
		"tz":                "EUROPE_AMSTERDAM",
		"sslScanEnabled":    true,
		"idleTimeInMinutes": 75,
		"parentId":          4400,
	}
	for field, want := range standardShare {
		if got, ok := share[field]; !ok || !reflect.DeepEqual(got, want) {
			t.Errorf("share-mode locations %s = %v (present=%v), want %v", field, got, ok, want)
		}
	}
	assertFieldsAbsent(t, "locations (paranoid)", paranoid,
		"tz",
		"sslScanEnabled",
		"idleTimeInMinutes",
		"parentId",
	)

	// id is allowed in all three modes and must survive paranoid projection.
	if got, ok := paranoid["id"]; !ok || got != 4501 {
		t.Errorf("paranoid-mode locations id = %v (present=%v), want 4501", got, ok)
	}

	// Standard-only fields must be dropped from share mode. country and state
	// are sensitive identifiers (geo footprint) per the referee verdicts, so
	// they must never reach share-mode exports.
	assertFieldsAbsent(t, "locations (share)", share,
		"country",
		"state",
		"ports",
		"ipAddresses",
		"description",
	)
	assertFieldsAbsent(t, "locations (paranoid)", paranoid,
		"country",
		"state",
		"ports",
		"ipAddresses",
		"description",
	)
}

func TestLocationSourceRecordDropsSecretAndExcludedFields(t *testing.T) {
	t.Parallel()

	location := wave1LocationFixture()
	records := []resources.SourceRecord{locationSourceRecord(location)}

	for _, mode := range []redact.Mode{redact.ModeStandard, redact.ModeShare, redact.ModeParanoid} {
		got := projectOneRecordInMode(t, resources.ProductZIA, resourceLocations, mode, records)

		// Secret-classified fields never surface in any mode.
		assertFieldsAbsent(t, "locations", got, "vpnCredentials", "preSharedKey")
		assertNoCanaries(t, "locations", got,
			wave1LocationPSKCanary,
			wave1LocationVPNFQDNCanary,
		)

		// Intentionally excluded fields stay out, and their canaries do not
		// leak through any other key. (Wave-4 promoted fields such as
		// geoOverride/subLocScope/otherSubLocation are now classified and are
		// covered by the location-family tests below.)
		assertFieldsAbsent(t, "locations", got,
			"displayTimeUnit",
			"surrogateRefreshTimeUnit",
			"childCount",
			"matchInChild",
			"excludeFromDynamicGroups",
			"excludeFromManualGroups",
		)
		assertNoCanaries(t, "locations", got, wave1LocationExcludedCanary)
	}
}

// ---------------------------------------------------------------------------
// Location-family tests: the geo/extranet/IPv6 and scope/membership-reference
// fields, exercised identically on locations and sublocations because the two
// resources share one SDK struct. The shared fixture carries a distinctive
// value in every mapped field, a canary in every nested secret sub-field
// (reference extensions), and a canary in every still-excluded field.
// ---------------------------------------------------------------------------

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

// TestLocationFamilyPromotedFieldsSurfaceInStandardMode asserts that every
// newly mapped field renders in standard mode on both resources, including
// nested reference id/name and the scope identifier lists.
func TestLocationFamilyPromotedFieldsSurfaceInStandardMode(t *testing.T) {
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

// TestLocationFamilyHonorsAllowedModes exercises mode visibility for well more
// than four promoted fields across share and paranoid on both resources.
func TestLocationFamilyHonorsAllowedModes(t *testing.T) {
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

// TestLocationFamilyNeverLeaksSecretsOrExcluded confirms that across all three
// modes the nested reference extensions and the still-excluded fields stay out
// of the projection, and no canary leaks.
func TestLocationFamilyNeverLeaksSecretsOrExcluded(t *testing.T) {
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
