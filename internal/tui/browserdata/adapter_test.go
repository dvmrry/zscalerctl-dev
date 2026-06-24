package browserdata

import (
	"errors"
	"strings"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

func TestBuildDeterministicOrder(t *testing.T) {
	// Catalog must be grouped by product; the adapter preserves order but does
	// not merge non-consecutive product blocks.
	catalog := resources.ResourceCatalog{
		{Product: resources.ProductZIA, Name: "locations", Operations: resources.ReadOperations(), Fields: standardFields()},
		{Product: resources.ProductZPA, Name: "app-connectors", Operations: resources.ReadOperations(), Fields: standardFields()},
		{Product: resources.ProductZPA, Name: "application-segments", Operations: resources.ReadOperations(), Fields: standardFields()},
	}
	src := fakeSource{}
	data, err := Build(catalog, src)
	if err != nil {
		t.Fatalf("Build error = %v", err)
	}
	if len(data.Products) != 2 {
		t.Fatalf("products = %d, want 2", len(data.Products))
	}
	if data.Products[0].Name != "zia" {
		t.Errorf("first product = %q, want zia", data.Products[0].Name)
	}
	if data.Products[1].Name != "zpa" {
		t.Errorf("second product = %q, want zpa", data.Products[1].Name)
	}
	if len(data.Products[1].Resources) != 2 {
		t.Fatalf("zpa resources = %d, want 2", len(data.Products[1].Resources))
	}
	if data.Products[1].Resources[0].Name != "app-connectors" {
		t.Errorf("first zpa resource = %q, want app-connectors", data.Products[1].Resources[0].Name)
	}
	if data.Products[1].Resources[1].Name != "application-segments" {
		t.Errorf("second zpa resource = %q, want application-segments", data.Products[1].Resources[1].Name)
	}
}

func TestBuildNormalResource(t *testing.T) {
	catalog := simpleCatalog("zia", "locations", standardFields())
	src := fakeSource{
		"zia/locations": {records: []map[string]any{
			{"id": "1", "name": "A"},
			{"id": "2", "name": "B"},
		}},
	}
	data, err := Build(catalog, src)
	if err != nil {
		t.Fatalf("Build error = %v", err)
	}
	if len(data.Products) != 1 || len(data.Products[0].Resources) != 1 {
		t.Fatalf("unexpected shape: %+v", data.Products)
	}
	res := data.Products[0].Resources[0]
	if res.Name != "locations" {
		t.Errorf("resource name = %q, want locations", res.Name)
	}
	if len(res.Records) != 2 {
		t.Errorf("records = %d, want 2", len(res.Records))
	}
	if res.Records[0].ID != "1" || res.Records[0].Name != "A" {
		t.Errorf("first record = %+v, want id=1 name=A", res.Records[0])
	}
	if res.Records[1].Status != "active" {
		t.Errorf("second record status = %q, want active", res.Records[1].Status)
	}
}

func TestBuildEmptyResource(t *testing.T) {
	catalog := simpleCatalog("zia", "forwarding-rules", standardFields())
	src := fakeSource{}
	data, err := Build(catalog, src)
	if err != nil {
		t.Fatalf("Build error = %v", err)
	}
	res := data.Products[0].Resources[0]
	if !res.Empty {
		t.Errorf("Empty = false, want true")
	}
	if len(res.Records) != 0 {
		t.Errorf("records = %d, want 0", len(res.Records))
	}
}

func TestBuildResourceError(t *testing.T) {
	catalog := simpleCatalog("zpa", "app-connectors", standardFields())
	src := fakeSource{
		"zpa/app-connectors": {err: errors.New("connector list unavailable")},
	}
	data, err := Build(catalog, src)
	if err != nil {
		t.Fatalf("Build error = %v", err)
	}
	res := data.Products[0].Resources[0]
	if res.Error != "connector list unavailable" {
		t.Errorf("Error = %q, want connector list unavailable", res.Error)
	}
	if !res.Empty && len(res.Records) != 0 {
		t.Errorf("error resource should have no records")
	}
}

func TestBuildLongRecord(t *testing.T) {
	fields := append(standardFields(),
		resources.FieldSpec{Name: "a", Classification: resources.ClassTenantConfig, AllowedModes: standardShareModes()},
		resources.FieldSpec{Name: "b", Classification: resources.ClassTenantConfig, AllowedModes: standardShareModes()},
		resources.FieldSpec{Name: "c", Classification: resources.ClassTenantConfig, AllowedModes: standardShareModes()},
		resources.FieldSpec{Name: "d", Classification: resources.ClassTenantConfig, AllowedModes: standardShareModes()},
		resources.FieldSpec{Name: "e", Classification: resources.ClassTenantConfig, AllowedModes: standardShareModes()},
		resources.FieldSpec{Name: "f", Classification: resources.ClassTenantConfig, AllowedModes: standardShareModes()},
	)
	catalog := simpleCatalog("zia", "settings", fields)
	src := fakeSource{
		"zia/settings": {records: []map[string]any{
			{"id": "1", "name": "Global Policy", "a": "1", "b": "2", "c": "3", "d": "4", "e": "5", "f": "6"},
		}},
	}
	data, err := Build(catalog, src)
	if err != nil {
		t.Fatalf("Build error = %v", err)
	}
	rec := data.Products[0].Resources[0].Records[0]
	if len(rec.Fields) < 6 {
		t.Errorf("fields = %d, want >= 6", len(rec.Fields))
	}
	// id and name should be promoted, not repeated as generic KV fields.
	for _, f := range rec.Fields {
		if f.Key == "id" || f.Key == "name" {
			t.Errorf("field %q should not be repeated as generic KV", f.Key)
		}
	}
}

func TestBuildSecretFieldDropped(t *testing.T) {
	fields := append(standardFields(),
		resources.FieldSpec{Name: "password", Classification: resources.ClassSecret},
		resources.FieldSpec{Name: "token", Classification: resources.ClassSecret},
	)
	catalog := simpleCatalog("zia", "locations", fields)
	src := fakeSource{
		"zia/locations": {records: []map[string]any{
			{"id": "1", "name": "A", "password": "hunter2", "token": "sekret"},
		}},
	}
	data, err := Build(catalog, src)
	if err != nil {
		t.Fatalf("Build error = %v", err)
	}
	rec := data.Products[0].Resources[0].Records[0]
	for _, f := range rec.Fields {
		if f.Key == "password" || f.Key == "token" {
			t.Errorf("secret field %q leaked into BrowserData", f.Key)
		}
		if strings.Contains(f.Value, "hunter2") || strings.Contains(f.Value, "sekret") {
			t.Errorf("secret value leaked in field %q=%q", f.Key, f.Value)
		}
	}
}

func TestBuildFixtureSource(t *testing.T) {
	data, err := Build(DemoCatalog(), ProjectedFixtureSource{})
	if err != nil {
		t.Fatalf("Build error = %v", err)
	}
	if len(data.Products) != 3 {
		t.Errorf("products = %d, want 3", len(data.Products))
	}
	var zia, zpa, zcc bool
	var ziaResources, zpaResources, zccResources int
	for _, p := range data.Products {
		switch p.Name {
		case "zia":
			zia = true
			ziaResources = len(p.Resources)
		case "zpa":
			zpa = true
			zpaResources = len(p.Resources)
		case "zcc":
			zcc = true
			zccResources = len(p.Resources)
		}
	}
	if !zia || !zpa || !zcc {
		t.Errorf("missing expected products: zia=%v zpa=%v zcc=%v", zia, zpa, zcc)
	}
	if ziaResources != 3 {
		t.Errorf("zia resources = %d, want 3", ziaResources)
	}
	if zpaResources != 2 {
		t.Errorf("zpa resources = %d, want 2", zpaResources)
	}
	if zccResources != 1 {
		t.Errorf("zcc resources = %d, want 1", zccResources)
	}
	// Verify secrets were dropped by projection.
	for _, p := range data.Products {
		for _, r := range p.Resources {
			for _, rec := range r.Records {
				for _, f := range rec.Fields {
					lower := strings.ToLower(f.Key)
					if strings.Contains(lower, "secret") || strings.Contains(lower, "password") || strings.Contains(lower, "psk") || strings.Contains(lower, "credential") {
						t.Errorf("secret-like key %q found in BrowserData", f.Key)
					}
					if strings.Contains(f.Value, "secret-key-material") || strings.Contains(f.Value, "secret") {
						t.Errorf("secret value leaked in field %q=%q", f.Key, f.Value)
					}
				}
			}
		}
	}
}

func standardFields() []resources.FieldSpec {
	return []resources.FieldSpec{
		{Name: "id", Classification: resources.ClassOperational, AllowedModes: allModes()},
		{Name: "name", Classification: resources.ClassTenantConfig, AllowedModes: standardShareModes()},
	}
}

func allModes() []redact.Mode {
	return []redact.Mode{redact.ModeStandard, redact.ModeShare, redact.ModeParanoid}
}

func standardShareModes() []redact.Mode {
	return []redact.Mode{redact.ModeStandard, redact.ModeShare}
}

func simpleCatalog(product, resource string, fields []resources.FieldSpec) resources.ResourceCatalog {
	return resources.ResourceCatalog{
		{
			Product:    resources.Product(product),
			Name:       resource,
			Operations: resources.ReadOperations(),
			Fields:     fields,
		},
	}
}

// fakeSource is a configurable ProjectedRecordSource for tests. It projects the
// supplied raw maps using the requested spec and standard redaction, so tests
// exercise the same projection path as real callers.
type fakeSource map[string]fakeEntry

type fakeEntry struct {
	records []map[string]any
	err     error
}

func (s fakeSource) ProjectedRecords(spec resources.ResourceSpec) ([]resources.ProjectedRecord, error) {
	key := string(spec.Product) + "/" + spec.Name
	entry := s[key]
	if entry.err != nil {
		return nil, entry.err
	}
	if len(entry.records) == 0 {
		return nil, nil
	}
	sourceRecords := make([]resources.SourceRecord, 0, len(entry.records))
	for _, r := range entry.records {
		sourceRecords = append(sourceRecords, resources.NewSourceRecord(r))
	}
	projected, _, err := resources.ProjectRecords(spec, redact.ModeStandard, sourceRecords)
	return projected.Records(), err
}
