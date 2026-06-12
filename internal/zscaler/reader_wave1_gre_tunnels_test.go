package zscaler

// Wave-1 field-coverage tests for zia/gre-tunnels: the promoted
// primaryDestVip/secondaryDestVip destination data-center VIP objects. Each
// test builds the SDK struct with a distinctive canary in every promoted
// sub-field, converts it with greTunnelSourceRecord, projects it through the
// resources catalog, and asserts the per-mode posture: full VIP detail in
// standard mode, infrastructure-only detail (no VIP address, no geo/location:
// latitude, longitude, city, region, and countryCode all drop) in share mode,
// nothing in paranoid mode, and never any admin-identity leakage from the
// excluded managedBy/lastModifiedBy references.
//
// This file owns its tests so reader_test.go and reader_sourcerecord_test.go
// stay untouched by the wave; it reuses package helpers (projectOneRecord,
// projectOneRecordInMode, assertNoCanaries, valueAtPath).

import (
	"strings"
	"testing"

	gretunnels "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/trafficforwarding/gretunnels"

	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

const (
	greWave1ManagedByCanary    = "gre-wave1-managedby-canary"
	greWave1LastModCanary      = "gre-wave1-lastmodifiedby-canary"
	greWave1PrimaryVIPCanary   = "203.0.113.41"
	greWave1SecondaryVIPCanary = "203.0.113.42"
)

func greWave1Tunnel() gretunnels.GreTunnels {
	withinCountry := true
	return gretunnels.GreTunnels{
		ID:                   654,
		SourceIP:             "192.0.2.10",
		InternalIpRange:      "10.10.10.0/29",
		LastModificationTime: 1712345678,
		WithinCountry:        &withinCountry,
		Comment:              "primary tunnel",
		IPUnnumbered:         true,
		SubCloud:             "us-east",
		ManagedBy: &gretunnels.ManagedBy{
			ID:   1001,
			Name: greWave1ManagedByCanary,
		},
		LastModifiedBy: &gretunnels.LastModifiedBy{
			ID:   1002,
			Name: greWave1LastModCanary,
		},
		PrimaryDestVip: &gretunnels.PrimaryDestVip{
			ID:                 901,
			VirtualIP:          greWave1PrimaryVIPCanary,
			PrivateServiceEdge: true,
			Datacenter:         "FRA4",
			Latitude:           50.1109221,
			Longitude:          8.6821267,
			City:               "Frankfurt",
			CountryCode:        "DE",
			Region:             "EMEA",
		},
		SecondaryDestVip: &gretunnels.SecondaryDestVip{
			ID:                 902,
			VirtualIP:          greWave1SecondaryVIPCanary,
			PrivateServiceEdge: false,
			Datacenter:         "AMS2",
			Latitude:           52.3675734,
			Longitude:          4.9041389,
			City:               "Amsterdam",
			CountryCode:        "NL",
			Region:             "EMEA",
		},
	}
}

// greWave1VIPObject returns the projected nested VIP object under field,
// failing the test if it is missing or not an object.
func greWave1VIPObject(t *testing.T, record map[string]any, field string) map[string]any {
	t.Helper()

	value, ok := record[field]
	if !ok {
		t.Fatalf("projected gre-tunnels missing %s, want map", field)
	}
	object, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("projected gre-tunnels %s = %T, want map[string]any", field, value)
	}
	return object
}

func TestGRETunnelSourceRecordProjectsDestVipsInStandardMode(t *testing.T) {
	t.Parallel()

	tunnel := greWave1Tunnel()
	got := projectOneRecord(t, resources.ProductZIA, resourceGRETunnels, []resources.SourceRecord{greTunnelSourceRecord(tunnel)})

	wantPrimary := map[string]any{
		"id":                 901,
		"virtualIp":          greWave1PrimaryVIPCanary,
		"privateServiceEdge": true,
		"datacenter":         "FRA4",
		"latitude":           50.1109221,
		"longitude":          8.6821267,
		"city":               "Frankfurt",
		"countryCode":        "DE",
		"region":             "EMEA",
	}
	wantSecondary := map[string]any{
		"id":                 902,
		"virtualIp":          greWave1SecondaryVIPCanary,
		"privateServiceEdge": false,
		"datacenter":         "AMS2",
		"latitude":           52.3675734,
		"longitude":          4.9041389,
		"city":               "Amsterdam",
		"countryCode":        "NL",
		"region":             "EMEA",
	}
	for field, want := range map[string]map[string]any{
		"primaryDestVip":   wantPrimary,
		"secondaryDestVip": wantSecondary,
	} {
		object := greWave1VIPObject(t, got, field)
		for key, wantValue := range want {
			gotValue, ok := object[key]
			if !ok {
				t.Errorf("projected gre-tunnels %s missing %s, want %v", field, key, wantValue)
				continue
			}
			if gotValue != wantValue {
				t.Errorf("projected gre-tunnels %s.%s = %v, want %v", field, key, gotValue, wantValue)
			}
		}
		if len(object) != len(want) {
			t.Errorf("projected gre-tunnels %s = %#v, want exactly keys of %#v", field, object, want)
		}
	}
}

func TestGRETunnelSourceRecordDestVipModeBehaviour(t *testing.T) {
	t.Parallel()

	tunnel := greWave1Tunnel()
	records := []resources.SourceRecord{greTunnelSourceRecord(tunnel)}

	// Share mode: the VIP objects remain (tenant-config, standard+share) with
	// their operational sub-fields and the datacenter label, while the
	// sensitive-identifier sub-fields (virtualIp, latitude, longitude, city,
	// region, countryCode) are dropped per the referee restriction matching
	// the catalog's existing standard-only region precedent.
	share := projectOneRecordInMode(t, resources.ProductZIA, resourceGRETunnels, redact.ModeShare, records)
	shareWant := []struct {
		field string
		want  map[string]any
	}{
		{"primaryDestVip", map[string]any{
			"id":                 901,
			"privateServiceEdge": true,
			"datacenter":         "FRA4",
		}},
		{"secondaryDestVip", map[string]any{
			"id":                 902,
			"privateServiceEdge": false,
			"datacenter":         "AMS2",
		}},
	}
	for _, tc := range shareWant {
		object := greWave1VIPObject(t, share, tc.field)
		for key, want := range tc.want {
			got, ok := object[key]
			if !ok {
				t.Errorf("share-mode gre-tunnels %s missing %s, want %v", tc.field, key, want)
				continue
			}
			if got != want {
				t.Errorf("share-mode gre-tunnels %s.%s = %v, want %v", tc.field, key, got, want)
			}
		}
		for _, dropped := range []string{"virtualIp", "latitude", "longitude", "city", "region", "countryCode"} {
			if value, ok := object[dropped]; ok {
				t.Errorf("share-mode gre-tunnels %s.%s = %v, want dropped", tc.field, dropped, value)
			}
		}
		if len(object) != len(tc.want) {
			t.Errorf("share-mode gre-tunnels %s = %#v, want exactly keys of %#v", tc.field, object, tc.want)
		}
	}
	assertNoCanaries(t, "gre-tunnels", share,
		greWave1PrimaryVIPCanary, greWave1SecondaryVIPCanary, "Frankfurt", "Amsterdam", "EMEA")

	// Paranoid mode: the parent VIP objects are standard+share only, so both
	// disappear entirely.
	paranoid := projectOneRecordInMode(t, resources.ProductZIA, resourceGRETunnels, redact.ModeParanoid, records)
	for _, field := range []string{"primaryDestVip", "secondaryDestVip"} {
		if value, ok := paranoid[field]; ok {
			t.Errorf("paranoid-mode gre-tunnels %s = %v, want dropped", field, value)
		}
	}
	assertNoCanaries(t, "gre-tunnels", paranoid,
		greWave1PrimaryVIPCanary, greWave1SecondaryVIPCanary, "FRA4", "AMS2", "Frankfurt", "Amsterdam", "EMEA")
}

func TestGRETunnelSourceRecordExcludedAdminIdentitiesStayDropped(t *testing.T) {
	t.Parallel()

	tunnel := greWave1Tunnel()
	records := []resources.SourceRecord{greTunnelSourceRecord(tunnel)}

	for _, mode := range []redact.Mode{redact.ModeStandard, redact.ModeShare, redact.ModeParanoid} {
		got := projectOneRecordInMode(t, resources.ProductZIA, resourceGRETunnels, mode, records)
		for _, field := range []string{"managedBy", "lastModifiedBy"} {
			if value, ok := got[field]; ok {
				t.Errorf("%s-mode gre-tunnels %s = %v, want excluded field absent", mode, field, value)
			}
		}
		assertNoCanaries(t, "gre-tunnels", got, greWave1ManagedByCanary, greWave1LastModCanary)
		if strings.Contains(toString(got["comment"]), greWave1ManagedByCanary) {
			t.Errorf("%s-mode gre-tunnels comment leaked managedBy canary", mode)
		}
	}
}
