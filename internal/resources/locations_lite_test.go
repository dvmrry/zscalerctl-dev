package resources

import (
	"slices"
	"testing"
)

// locations-lite returns a subset of a location's keys; each must render
// exactly as it does on zia/locations so the two views cannot disagree. The
// lite endpoint spells one key differently from /locations.
var liteToLocationsKey = map[string]string{"zappSslScanEnabled": "zappSSLScanEnabled"}

func TestLocationsLiteFieldsMirrorLocations(t *testing.T) {
	t.Parallel()
	lite, ok := FindSpec(ProductZIA, "locations-lite")
	if !ok {
		t.Fatal("zia/locations-lite missing")
	}
	full, _ := FindSpec(ProductZIA, "locations")
	for _, field := range lite.Fields {
		name := field.Name
		if alias, ok := liteToLocationsKey[name]; ok {
			name = alias
		}
		match, ok := findTopLevelField(full.Fields, name)
		if !ok {
			t.Errorf("lite field %q is not a zia/locations field", field.Name)
			continue
		}
		if field.Classification != match.Classification || !slices.Equal(field.AllowedModes, match.AllowedModes) {
			t.Errorf("lite field %q = %s %v, zia/locations has %s %v", field.Name, field.Classification, field.AllowedModes, match.Classification, match.AllowedModes)
		}
	}
}
