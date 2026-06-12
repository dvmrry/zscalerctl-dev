package zscaler

// Wave-2 field-coverage tests for zia/sublocations, mirroring the wave-1
// locations tests. The sublocation fixture carries a distinctive value in
// every promoted field, a canary in every secret field, and a canary in every
// intentionally-excluded field. The tests project the source record through
// the resources catalog and assert that promoted fields surface under the
// right keys in standard mode, that mode widening/narrowing matches each
// field's AllowedModes, that secret fields never surface, and that no
// excluded-field canary leaks into any projection.
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
	wave2SublocationPSKCanary      = "wave2-sublocation-psk-canary"
	wave2SublocationVPNFQDNCanary  = "vpn-fqdn-canary.sub.example.com"
	wave2SublocationExcludedCanary = "wave2-sublocation-excluded-canary"
)

func wave2SublocationFixture() locationmanagement.Locations {
	return locationmanagement.Locations{
		ID:          4601,
		Name:        "Branch Floor 2",
		ParentID:    4501,
		Description: "wave2 sublocation description",
		Country:     "NETHERLANDS",
		State:       "North Holland",
		TZ:          "EUROPE_AMSTERDAM",
		Language:    "ENGLISH",
		Profile:     "CORPORATE",
		UpBandwidth: 13000,
		DnBandwidth: 35000,
		IPAddresses: []string{"10.20.30.0/24"},
		Ports:       []int{8082, 9444},
		VPNCredentials: []locationmanagement.VPNCredentials{
			{
				ID:           92,
				Type:         "UFQDN",
				FQDN:         wave2SublocationVPNFQDNCanary,
				PreSharedKey: wave2SublocationPSKCanary,
				Comments:     "vpn comment " + wave2SublocationExcludedCanary,
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
		IdleTimeInMinutes:                   85,
		SurrogateRefreshTimeInMinutes:       115,
		OFWEnabled:                          true,
		IPSControl:                          true,
		AUPEnabled:                          true,
		CautionEnabled:                      true,
		AUPBlockInternetUntilAccepted:       true,
		AUPForceSSLInspection:               true,
		AUPTimeoutInDays:                    55,
		IOTDiscoveryEnabled:                 true,
		IOTEnforcePolicySet:                 true,
		CookiesAndProxy:                     true,
		OtherSubLocation:                    true,
		Other6SubLocation:                   true,
		// Intentionally excluded fields below carry canaries that must never
		// surface in any projection.
		ChildCount:               7777,
		MatchInChild:             true,
		GeoOverride:              true,
		ECLocation:               true,
		IPv6Enabled:              true,
		DisplayTimeUnit:          "MINUTE-" + wave2SublocationExcludedCanary,
		SurrogateRefreshTimeUnit: "HOUR-" + wave2SublocationExcludedCanary,
		SubLocScopeEnabled:       true,
		SubLocScope:              "SCOPE-" + wave2SublocationExcludedCanary,
		SubLocScopeValues:        []string{"scope-value-" + wave2SublocationExcludedCanary},
		SubLocAccIDs:             []string{"acc-id-" + wave2SublocationExcludedCanary},
		ExcludeFromDynamicGroups: true,
		ExcludeFromManualGroups:  true,
	}
}

func TestSublocationSourceRecordProjectsWave2FieldsInStandardMode(t *testing.T) {
	t.Parallel()

	sublocation := wave2SublocationFixture()
	got := projectOneRecord(t, resources.ProductZIA, resourceSublocations, []resources.SourceRecord{sublocationSourceRecord(sublocation)})

	want := map[string]any{
		"id":                                  4601,
		"name":                                "Branch Floor 2",
		"parentId":                            4501,
		"description":                         "wave2 sublocation description",
		"country":                             "NETHERLANDS",
		"state":                               "North Holland",
		"tz":                                  "EUROPE_AMSTERDAM",
		"language":                            "ENGLISH",
		"profile":                             "CORPORATE",
		"upBandwidth":                         13000,
		"dnBandwidth":                         35000,
		"ipAddresses":                         []string{"10.20.30.0/24"},
		"ports":                               []int{8082, 9444},
		"authRequired":                        true,
		"basicAuthEnabled":                    true,
		"digestAuthEnabled":                   true,
		"kerberosAuth":                        true,
		"sslScanEnabled":                      true,
		"zappSSLScanEnabled":                  true,
		"xffForwardEnabled":                   true,
		"surrogateIP":                         true,
		"surrogateIPEnforcedForKnownBrowsers": true,
		"idleTimeInMinutes":                   85,
		"surrogateRefreshTimeInMinutes":       115,
		"ofwEnabled":                          true,
		"ipsControl":                          true,
		"aupEnabled":                          true,
		"cautionEnabled":                      true,
		"aupBlockInternetUntilAccepted":       true,
		"aupForceSslInspection":               true,
		"aupTimeoutInDays":                    55,
		"iotDiscoveryEnabled":                 true,
		"iotEnforcePolicySet":                 true,
		"cookiesAndProxy":                     true,
		"otherSubLocation":                    true,
		"other6SubLocation":                   true,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("projected sublocations standard mode = %#v, want %#v", got, want)
	}
}

func TestSublocationSourceRecordHonorsAllowedModesAcrossShareAndParanoid(t *testing.T) {
	t.Parallel()

	sublocation := wave2SublocationFixture()
	records := []resources.SourceRecord{sublocationSourceRecord(sublocation)}

	share := projectOneRecordInMode(t, resources.ProductZIA, resourceSublocations, redact.ModeShare, records)
	paranoid := projectOneRecordInMode(t, resources.ProductZIA, resourceSublocations, redact.ModeParanoid, records)

	// Standard+share fields stay visible in share mode and drop in paranoid.
	standardShare := map[string]any{
		"tz":                "EUROPE_AMSTERDAM",
		"sslScanEnabled":    true,
		"idleTimeInMinutes": 85,
		"otherSubLocation":  true,
		"other6SubLocation": true,
	}
	for field, want := range standardShare {
		if got, ok := share[field]; !ok || !reflect.DeepEqual(got, want) {
			t.Errorf("share-mode sublocations %s = %v (present=%v), want %v", field, got, ok, want)
		}
	}
	assertFieldsAbsent(t, "sublocations (paranoid)", paranoid,
		"tz",
		"sslScanEnabled",
		"idleTimeInMinutes",
		"otherSubLocation",
		"other6SubLocation",
	)

	// id and parentId are allowed in all three modes for sublocations (the
	// parent reference predates wave 2) and must survive paranoid projection.
	if got, ok := paranoid["id"]; !ok || got != 4601 {
		t.Errorf("paranoid-mode sublocations id = %v (present=%v), want 4601", got, ok)
	}
	if got, ok := paranoid["parentId"]; !ok || got != 4501 {
		t.Errorf("paranoid-mode sublocations parentId = %v (present=%v), want 4501", got, ok)
	}

	// Standard-only fields must be dropped from share mode. country and state
	// are sensitive identifiers (geo footprint) per the referee verdicts, so
	// they must never reach share-mode exports.
	assertFieldsAbsent(t, "sublocations (share)", share,
		"country",
		"state",
		"ports",
		"ipAddresses",
		"description",
	)
	assertFieldsAbsent(t, "sublocations (paranoid)", paranoid,
		"country",
		"state",
		"ports",
		"ipAddresses",
		"description",
	)
}

func TestSublocationSourceRecordDropsSecretAndExcludedFields(t *testing.T) {
	t.Parallel()

	sublocation := wave2SublocationFixture()
	records := []resources.SourceRecord{sublocationSourceRecord(sublocation)}

	for _, mode := range []redact.Mode{redact.ModeStandard, redact.ModeShare, redact.ModeParanoid} {
		got := projectOneRecordInMode(t, resources.ProductZIA, resourceSublocations, mode, records)

		// Secret-classified fields never surface in any mode.
		assertFieldsAbsent(t, "sublocations", got, "vpnCredentials", "preSharedKey")
		assertNoCanaries(t, "sublocations", got,
			wave2SublocationPSKCanary,
			wave2SublocationVPNFQDNCanary,
		)

		// Intentionally excluded fields stay out, and their canaries do not
		// leak through any other key.
		assertFieldsAbsent(t, "sublocations", got,
			"displayTimeUnit",
			"surrogateRefreshTimeUnit",
			"subLocScopeEnabled",
			"subLocScope",
			"subLocScopeValues",
			"subLocAccIds",
			"childCount",
			"matchInChild",
			"geoOverride",
			"ecLocation",
			"ipv6Enabled",
			"excludeFromDynamicGroups",
			"excludeFromManualGroups",
		)
		assertNoCanaries(t, "sublocations", got, wave2SublocationExcludedCanary)
	}
}
