package zscaler

// Wave-1 field-coverage tests for zia/locations. The location fixture carries a
// distinctive value in every promoted field, a canary in every secret field,
// and a canary in every intentionally-excluded field. The tests project the
// source record through the resources catalog and assert that promoted fields
// surface under the right keys in standard mode, that mode widening/narrowing
// matches each field's AllowedModes, that secret fields never surface, and
// that no excluded-field canary leaks into any projection.
//
// These tests live in their own file because reader_test.go and
// reader_sourcerecord_test.go are shared files owned by other changes; they
// reuse package helpers (projectOneRecord, projectOneRecordInMode,
// assertNoCanaries, assertFieldsAbsent) from the same package.

import (
	"reflect"
	"testing"

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
		// classification and mode behavior is asserted in
		// reader_wave4_locations_family_test.go. They are listed in the
		// standard-mode want map below so this exact-match test stays accurate.
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

func TestLocationSourceRecordProjectsWave1FieldsInStandardMode(t *testing.T) {
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
		// covered by reader_wave4_locations_family_test.go.)
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
