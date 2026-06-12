package zscaler

import (
	"fmt"
	"strings"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"

	staticips "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/trafficforwarding/staticips"
)

const (
	wave1StaticIPManagedByCanary      = "wave1-canary-managed-by-partner"
	wave1StaticIPLastModifiedByCanary = "wave1-canary-last-modified-by-admin"
)

func wave1StaticIPFixture() staticips.StaticIP {
	return staticips.StaticIP{
		ID:                   4101,
		IpAddress:            "203.0.113.77",
		GeoOverride:          true,
		Latitude:             47.6062095,
		Longitude:            -122.3320708,
		RoutableIP:           true,
		LastModificationTime: 1712345678,
		Comment:              "wave1 static ip fixture comment",
		City: &staticips.City{
			ID:   88231,
			Name: "wave1-city-reference",
		},
		ManagedBy: &staticips.ManagedBy{
			ID:   555,
			Name: wave1StaticIPManagedByCanary,
		},
		LastModifiedBy: &staticips.LastModifiedBy{
			ID:   556,
			Name: wave1StaticIPLastModifiedByCanary,
		},
	}
}

func wave1StaticIPRecords() []resources.SourceRecord {
	return []resources.SourceRecord{staticIPSourceRecord(wave1StaticIPFixture())}
}

func TestStaticIPWave1CityProjectedInStandardMode(t *testing.T) {
	got := projectOneRecord(t, resources.ProductZIA, resourceStaticIPs, wave1StaticIPRecords())

	city, ok := got["city"].(map[string]any)
	if !ok {
		t.Fatalf("standard mode city = %T(%v), want map[string]any", got["city"], got["city"])
	}
	if city["id"] != 88231 {
		t.Errorf("standard mode city.id = %v, want 88231", city["id"])
	}
	if city["name"] != "wave1-city-reference" {
		t.Errorf("standard mode city.name = %v, want wave1-city-reference", city["name"])
	}
}

func TestStaticIPWave1ModeMatrix(t *testing.T) {
	type fieldExpectation struct {
		field    string
		standard bool
		share    bool
		paranoid bool
	}
	expectations := []fieldExpectation{
		// Promoted this wave: idNameField("city", standardOnlyMode()).
		{field: "city", standard: true, share: false, paranoid: false},
		// Existing fields exercising the share/paranoid boundaries.
		{field: "id", standard: true, share: true, paranoid: true},
		{field: "routableIP", standard: true, share: true, paranoid: true},
		{field: "geoOverride", standard: true, share: true, paranoid: false},
		{field: "lastModificationTime", standard: true, share: true, paranoid: false},
		{field: "ipAddress", standard: true, share: false, paranoid: false},
		{field: "latitude", standard: true, share: false, paranoid: false},
		{field: "longitude", standard: true, share: false, paranoid: false},
		{field: "comment", standard: true, share: false, paranoid: false},
	}

	modes := []struct {
		mode redact.Mode
		want func(expectation fieldExpectation) bool
	}{
		{mode: redact.ModeStandard, want: func(e fieldExpectation) bool { return e.standard }},
		{mode: redact.ModeShare, want: func(e fieldExpectation) bool { return e.share }},
		{mode: redact.ModeParanoid, want: func(e fieldExpectation) bool { return e.paranoid }},
	}

	for _, m := range modes {
		got := projectOneRecordInMode(t, resources.ProductZIA, resourceStaticIPs, m.mode, wave1StaticIPRecords())
		for _, expectation := range expectations {
			_, present := got[expectation.field]
			if want := m.want(expectation); present != want {
				t.Errorf("mode %s field %q present = %t, want %t", m.mode, expectation.field, present, want)
			}
		}
	}
}

func TestStaticIPWave1ExcludedFieldsNeverLeak(t *testing.T) {
	for _, mode := range []redact.Mode{redact.ModeStandard, redact.ModeShare, redact.ModeParanoid} {
		got := projectOneRecordInMode(t, resources.ProductZIA, resourceStaticIPs, mode, wave1StaticIPRecords())

		if _, ok := got["managedBy"]; ok {
			t.Errorf("mode %s managedBy present, want absent (excluded admin identity)", mode)
		}
		if _, ok := got["lastModifiedBy"]; ok {
			t.Errorf("mode %s lastModifiedBy present, want absent (excluded admin identity)", mode)
		}

		rendered := fmt.Sprintf("%v", got)
		for _, canary := range []string{wave1StaticIPManagedByCanary, wave1StaticIPLastModifiedByCanary} {
			if strings.Contains(rendered, canary) {
				t.Errorf("mode %s projection leaked excluded-field canary %q: %s", mode, canary, rendered)
			}
		}
	}
}
