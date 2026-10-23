package diff

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/dump"
	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

func TestLoadCollectionReadsLocationsURLRulesSingletonAndGet(t *testing.T) {
	locations := collectionLocationsSpec()
	urlRules := collectionURLRulesSpec()
	singleton := collectionSingletonSpec()
	catalog := resources.ResourceCatalog{locations, urlRules, singleton}
	dir := writeTestDump(t, catalog, dumpFixture{
		entries: []dumpEntryFixture{
			{spec: locations, payload: `[{"id":"1","name":"HQ"},{"id":"2","name":"Branch"}]`},
			{spec: urlRules, payload: `[{"id":"r1","name":"Social","action":"block"}]`},
			{spec: singleton, payload: `{"enabled":true}`},
		},
	})

	collection, err := LoadCollection(context.Background(), dir, catalog)
	if err != nil {
		t.Fatalf("LoadCollection(%s) error = %v, want nil", dir, err)
	}
	if got := collection.Redaction(); got != redact.ModeStandard {
		t.Fatalf("Collection.Redaction() = %q, want standard", got)
	}
	if got := collection.Provenance(); got != (CollectionProvenance{
		ManifestSchema: "zscalerctl.dump.manifest.v2",
		Redaction:      "standard",
		Status:         "complete",
		ResourceCount:  3,
		RecordCount:    4,
	}) {
		t.Fatalf("Collection.Provenance() = %#v, want safe complete metadata", got)
	}

	list, err := collection.ListProjected(context.Background(), "zia", "locations")
	if err != nil {
		t.Fatalf("Collection.ListProjected(locations) error = %v, want nil", err)
	}
	gotRecords := list.Records()
	if len(gotRecords) != 2 {
		t.Fatalf("Collection.ListProjected(locations) records = %#v, want stable two-record order", gotRecords)
	}
	_, firstNameOK := gotRecords[0].Value("name")
	_, secondNameOK := gotRecords[1].Value("name")
	if !firstNameOK || !secondNameOK {
		t.Fatalf("Collection.ListProjected(locations) records = %#v, want stable two-record order", gotRecords)
	}
	show, err := collection.ShowProjected(context.Background(), "zia", "advanced-settings")
	if err != nil {
		t.Fatalf("Collection.ShowProjected(advanced-settings) error = %v, want nil", err)
	}
	if show.Len() != 1 {
		t.Fatalf("Collection.ShowProjected(advanced-settings) records = %d, want 1", show.Len())
	}
	got, err := collection.GetProjectedByID(context.Background(), "zia", "url-filtering-rules", "r1")
	if err != nil {
		t.Fatalf("Collection.GetProjectedByID(url-filtering-rules/r1) error = %v, want nil", err)
	}
	if got.Len() != 1 {
		t.Fatalf("Collection.GetProjectedByID(url-filtering-rules/r1) records = %d, want 1", got.Len())
	}
}

func TestLoadCollectionPreservesCredentialMetadataDescriptions(t *testing.T) {
	const uuid = "123e4567-e89b-12d3-a456-426614174000"
	spec, ok := resources.FindSpec(resources.ProductZIA, "locations")
	if !ok {
		t.Fatal("FindSpec(zia, locations) ok = false, want true")
	}
	catalog := resources.ResourceCatalog{spec}
	for _, tt := range []struct {
		description string
		wantErr     bool
	}{
		{"API token ID: " + uuid, false},
		{"Access token ID: " + uuid, false},
		{"API key ID: " + uuid, false},
		{"API key name is " + uuid, false},
		{"API token: <REDACTED:SECRET>", false},
		{"Access token: <REDACTED:SECRET>", false},
		{"API key: <REDACTED:SECRET>", false},
		{"API token: " + uuid, true},
		{"Access token: " + uuid, true},
		{"API key: " + uuid, true},
		{"API key is " + uuid, true},
	} {
		t.Run(tt.description, func(t *testing.T) {
			// Write the stored record directly so current projection cannot
			// mask a regression in admission of previously sanitized data.
			payload := `[{"id":"1","name":"HQ","description":"` + tt.description + `"}]`
			dir := writeTestDump(t, catalog, dumpFixture{
				entries: []dumpEntryFixture{{spec: spec, payload: payload}},
			})
			collection, err := LoadCollection(context.Background(), dir, catalog)
			if tt.wantErr {
				if !errors.Is(err, ErrInvalidDump) {
					t.Fatalf("LoadCollection() error = %v, want ErrInvalidDump", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("LoadCollection() error = %v, want nil", err)
			}
			projected, err := collection.ListProjected(context.Background(), "zia", "locations")
			if err != nil {
				t.Fatalf("Collection.ListProjected(locations) error = %v, want nil", err)
			}
			records := projected.Records()
			if len(records) != 1 {
				t.Fatalf("Collection.ListProjected(locations) records = %d, want 1", len(records))
			}
			if value, ok := records[0].Value("description"); !ok || value != tt.description {
				t.Errorf("admitted description = %#v (present %t), want %q", value, ok, tt.description)
			}
		})
	}
}

func TestLoadCollectionEnforcesWriterPayloadShape(t *testing.T) {
	listSpec := collectionLocationsSpec()
	showSpec := collectionSingletonSpec()
	tests := []struct {
		name    string
		spec    resources.ResourceSpec
		payload string
		wantErr bool
	}{
		{
			name:    "list rejects object payload",
			spec:    listSpec,
			payload: `{"id":"1","name":"HQ"}`,
			wantErr: true,
		},
		{
			name:    "show rejects array payload",
			spec:    showSpec,
			payload: `[{"enabled":true}]`,
			wantErr: true,
		},
		{
			name:    "show rejects empty array payload",
			spec:    showSpec,
			payload: `[]`,
			wantErr: true,
		},
		{
			name:    "list accepts array payload",
			spec:    listSpec,
			payload: `[{"id":"1","name":"HQ"}]`,
		},
		{
			name:    "show accepts object payload",
			spec:    showSpec,
			payload: `{"enabled":true}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := writeTestDump(t, resources.ResourceCatalog{tt.spec}, dumpFixture{
				entries: []dumpEntryFixture{{spec: tt.spec, payload: tt.payload}},
			})
			_, err := LoadCollection(context.Background(), dir, resources.ResourceCatalog{tt.spec})
			if tt.wantErr {
				if err == nil || !errors.Is(err, ErrInvalidDump) {
					t.Fatalf("LoadCollection(%s) error = %v, want ErrInvalidDump", tt.name, err)
				}
				if !strings.Contains(err.Error(), "payload shape") {
					t.Fatalf("LoadCollection(%s) error = %q, want payload-shape context", tt.name, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("LoadCollection(%s) error = %v, want nil", tt.name, err)
			}
		})
	}
}

func TestLoadCollectionKeepsLoadedRecordsImmutable(t *testing.T) {
	spec := collectionLocationsSpec()
	catalog := resources.ResourceCatalog{spec}
	dir := writeTestDump(t, catalog, dumpFixture{
		entries: []dumpEntryFixture{{spec: spec, payload: `[{"id":"1","name":"HQ"}]`}},
	})
	collection, err := LoadCollection(context.Background(), dir, catalog)
	if err != nil {
		t.Fatalf("LoadCollection(%s) error = %v, want nil", dir, err)
	}
	catalog[0].Fields[1].Name = "mutated-caller-catalog"
	if err := os.WriteFile(filepath.Join(dir, "resources", "zia", "locations.json"), []byte(`[{"id":"1","name":"changed-on-disk"}]`), 0o600); err != nil {
		t.Fatalf("os.WriteFile(resource) error = %v, want nil", err)
	}

	first, err := collection.ListProjected(context.Background(), "zia", "locations")
	if err != nil {
		t.Fatalf("Collection.ListProjected(first) error = %v, want nil", err)
	}
	fields := first.Records()[0].Fields()
	fields["name"] = "mutated-return"
	second, err := collection.ListProjected(context.Background(), "zia", "locations")
	if err != nil {
		t.Fatalf("Collection.ListProjected(second) error = %v, want nil", err)
	}
	if got, want := second.Records()[0].Value("name"); got != "HQ" {
		t.Fatalf("Collection.ListProjected(second).name = %v, want %v", got, want)
	}
}

func TestLoadCollectionRejectsPartialAndMalformedAdmission(t *testing.T) {
	spec := collectionLocationsSpec()
	catalog := resources.ResourceCatalog{spec}

	tests := []struct {
		name       string
		fixture    dumpFixture
		mutate     func(string)
		want       error
		wantInText string
	}{
		{
			name:    "partial resource",
			fixture: dumpFixture{status: "partial"},
			want:    ErrPartialDumpInput,
		},
		{
			name:    "shape tamper",
			fixture: dumpFixture{entries: []dumpEntryFixture{{spec: spec, payload: `[{"id":"1","name":"HQ"}]`}}},
			mutate: func(dir string) {
				rewriteManifest(t, dir, func(manifest *dump.Manifest) {
					manifest.Resources[0].Shape = "singleton"
				})
			},
			want:       ErrInvalidDump,
			wantInText: "manifest shape does not match",
		},
		{
			name:    "count tamper",
			fixture: dumpFixture{entries: []dumpEntryFixture{{spec: spec, payload: `[{"id":"1","name":"HQ"}]`}}},
			mutate: func(dir string) {
				rewriteManifest(t, dir, func(manifest *dump.Manifest) {
					manifest.Resources[0].Records = 2
				})
			},
			want:       ErrInvalidDump,
			wantInText: "record count does not match",
		},
		{
			name:    "unsafe path",
			fixture: dumpFixture{entries: []dumpEntryFixture{{spec: spec, payload: `[{"id":"1","name":"HQ"}]`}}},
			mutate: func(dir string) {
				rewriteManifest(t, dir, func(manifest *dump.Manifest) {
					manifest.Resources[0].Path = "../outside.json"
				})
			},
			want:       ErrInvalidDump,
			wantInText: "unsafe path",
		},
		{
			name:       "duplicate id",
			fixture:    dumpFixture{entries: []dumpEntryFixture{{spec: spec, payload: `[{"id":"1","name":"HQ"},{"id":"1","name":"Branch"}]`}}},
			want:       ErrInvalidDump,
			wantInText: "duplicate identity",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := writeTestDump(t, catalog, tt.fixture)
			if tt.mutate != nil {
				tt.mutate(dir)
			}
			_, err := LoadCollection(context.Background(), dir, catalog)
			if err == nil || !errors.Is(err, tt.want) {
				t.Fatalf("LoadCollection(%s) error = %v, want errors.Is(..., %v)", tt.name, err, tt.want)
			}
			if tt.wantInText != "" && !strings.Contains(err.Error(), tt.wantInText) {
				t.Fatalf("LoadCollection(%s) error = %q, want context %q", tt.name, err, tt.wantInText)
			}
		})
	}
}

func TestCollectionQueryErrorsAreMeaningfulAndValueFree(t *testing.T) {
	spec := collectionLocationsSpec()
	missingSpec := collectionURLRulesSpec()
	catalog := resources.ResourceCatalog{spec, missingSpec}
	dir := writeTestDump(t, catalog, dumpFixture{entries: []dumpEntryFixture{{spec: spec, payload: `[]`}}})
	collection, err := LoadCollection(context.Background(), dir, catalog)
	if err != nil {
		t.Fatalf("LoadCollection(%s) error = %v, want nil", dir, err)
	}
	tests := []struct {
		name string
		call func() error
		want error
	}{
		{name: "unknown", call: func() error {
			_, err := collection.ListProjected(context.Background(), "zia", "unknown-resource")
			return err
		}, want: resources.ErrUnknownResource},
		{name: "missing from dump", call: func() error {
			_, err := collection.ListProjected(context.Background(), "zia", "url-filtering-rules")
			return err
		}, want: ErrCollectionScopeMismatch},
		{name: "absent get", call: func() error {
			_, err := collection.GetProjectedByID(context.Background(), "zia", "locations", "missing-id")
			return err
		}, want: resources.ErrRecordNotFound},
		{name: "missing id", call: func() error {
			_, err := collection.GetProjectedByID(context.Background(), "zia", "locations", " ")
			return err
		}, want: resources.ErrMissingID},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.call()
			if !errors.Is(err, tt.want) {
				t.Fatalf("Collection query %s error = %v, want errors.Is(..., %v)", tt.name, err, tt.want)
			}
			if strings.Contains(err.Error(), "missing-id") || strings.Contains(err.Error(), "unknown-resource") {
				t.Fatalf("Collection query %s error = %q, want no raw request values", tt.name, err)
			}
		})
	}
}

func TestLoadCollectionAllowsModeSuppressedGetKeyForListReads(t *testing.T) {
	spec := resources.ResourceSpec{
		Product:    resources.ProductZIA,
		Name:       "locations",
		Operations: resources.ReadOperations(),
		Fields: []resources.FieldSpec{
			{Name: "id", Classification: resources.ClassSensitiveIdentifier, AllowedModes: []redact.Mode{redact.ModeStandard}},
			{Name: "name", Classification: resources.ClassTenantConfig, AllowedModes: testStandardShareModes()},
		},
	}
	catalog := resources.ResourceCatalog{spec}
	dir := writeTestDump(t, catalog, dumpFixture{
		redaction: redact.ModeShare,
		entries:   []dumpEntryFixture{{spec: spec, payload: `[{"name":"HQ"}]`}},
	})
	collection, err := LoadCollection(context.Background(), dir, catalog)
	if err != nil {
		t.Fatalf("LoadCollection(mode-suppressed get key) error = %v, want nil", err)
	}
	if _, err := collection.ListProjected(context.Background(), "zia", "locations"); err != nil {
		t.Fatalf("Collection.ListProjected(mode-suppressed get key) error = %v, want nil", err)
	}
	if _, err := collection.GetProjectedByID(context.Background(), "zia", "locations", "1"); !errors.Is(err, resources.ErrUnsupportedLoad) {
		t.Fatalf("Collection.GetProjectedByID(mode-suppressed get key) error = %v, want ErrUnsupportedLoad", err)
	}
}

func TestLoadCollectionHonorsCancellationAndAggregateBudget(t *testing.T) {
	spec := collectionLocationsSpec()
	catalog := resources.ResourceCatalog{spec}
	dir := writeTestDump(t, catalog, dumpFixture{entries: []dumpEntryFixture{{spec: spec, payload: `[{"id":"1","name":"HQ"}]`}}})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := LoadCollection(ctx, dir, catalog); !errors.Is(err, context.Canceled) {
		t.Fatalf("LoadCollection(canceled) error = %v, want context.Canceled", err)
	}

	catalogSpecs, err := validateCatalog(catalog)
	if err != nil {
		t.Fatalf("validateCatalog() error = %v, want nil", err)
	}
	selected := map[ResourceKey]bool{{Product: spec.Product, Name: spec.Name}: true}
	_, err = loadDumpWithBudget(context.Background(), dir, catalogSpecs, selected, 1)
	if !errors.Is(err, ErrCollectionTooLarge) || !errors.Is(err, ErrInvalidDump) {
		t.Fatalf("loadDumpWithBudget(1 byte) error = %v, want collection-too-large and invalid-dump classification", err)
	}
}

func TestLoadCollectionRejectsOverflowingNumberWithoutEcho(t *testing.T) {
	const numericCanary = "12345678901234567890123456789012345678901234567890e400"
	spec := collectionSingletonSpec()
	catalog := resources.ResourceCatalog{spec}
	dir := writeTestDump(t, catalog, dumpFixture{
		entries: []dumpEntryFixture{{spec: spec, payload: `{"enabled":0}`}},
	})
	body := `{"enabled":` + numericCanary + `}`
	path := filepath.Join(dir, "resources", string(spec.Product), spec.Name+".json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("os.WriteFile(%s) error = %v", path, err)
	}
	_, err := LoadCollection(context.Background(), dir, catalog)
	if err == nil || !errors.Is(err, ErrInvalidDump) {
		t.Fatalf("LoadCollection(overflowing number) error = %v, want ErrInvalidDump", err)
	}
	if strings.Contains(err.Error(), numericCanary) || strings.Contains(err.Error(), body) {
		t.Fatalf("LoadCollection(overflowing number) error = %q, want no artifact payload", err)
	}
}

func TestCompareRejectsOverflowingNumberWithoutEcho(t *testing.T) {
	const numericCanary = "12345678901234567890123456789012345678901234567890e400"
	spec := collectionLocationsSpec()
	catalog := resources.ResourceCatalog{spec}
	oldDir := writeTestDump(t, catalog, dumpFixture{
		entries: []dumpEntryFixture{{spec: spec, payload: `[{"id":"1","name":"HQ"}]`}},
	})
	newDir := writeTestDump(t, catalog, dumpFixture{
		entries: []dumpEntryFixture{{spec: spec, payload: `[{"id":"1","name":"HQ"}]`}},
	})
	body := `[{"id":"1","name":` + numericCanary + `}]`
	path := filepath.Join(newDir, "resources", string(spec.Product), spec.Name+".json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("os.WriteFile(%s) error = %v", path, err)
	}
	_, err := Compare(oldDir, newDir, Options{Catalog: catalog})
	if err == nil || !errors.Is(err, ErrInvalidDump) {
		t.Fatalf("Compare(overflowing number) error = %v, want ErrInvalidDump", err)
	}
	if strings.Contains(err.Error(), numericCanary) || strings.Contains(err.Error(), body) {
		t.Fatalf("Compare(overflowing number) error = %q, want no artifact payload", err)
	}
}

func collectionLocationsSpec() resources.ResourceSpec {
	return resources.ResourceSpec{
		Product:    resources.ProductZIA,
		Name:       "locations",
		Operations: resources.ReadOperations(),
		Fields: []resources.FieldSpec{
			{Name: "id", Classification: resources.ClassOperational, AllowedModes: testAllModes()},
			{Name: "name", Classification: resources.ClassTenantConfig, AllowedModes: testStandardShareModes()},
		},
	}
}

func collectionURLRulesSpec() resources.ResourceSpec {
	return resources.ResourceSpec{
		Product:    resources.ProductZIA,
		Name:       "url-filtering-rules",
		Operations: resources.ReadOperations(),
		Fields: []resources.FieldSpec{
			{Name: "id", Classification: resources.ClassOperational, AllowedModes: testAllModes()},
			{Name: "name", Classification: resources.ClassTenantConfig, AllowedModes: testStandardShareModes()},
			{Name: "action", Classification: resources.ClassTenantConfig, AllowedModes: testStandardShareModes()},
		},
	}
}

func collectionSingletonSpec() resources.ResourceSpec {
	return resources.ResourceSpec{
		Product:    resources.ProductZIA,
		Name:       "advanced-settings",
		Operations: resources.ShowOperation(),
		Fields: []resources.FieldSpec{
			{Name: "enabled", Classification: resources.ClassOperational, AllowedModes: testAllModes()},
		},
	}
}

func TestCollectionSpecCopyDoesNotShareNestedFields(t *testing.T) {
	spec := collectionLocationsSpec()
	catalog := resources.ResourceCatalog{spec}
	dir := writeTestDump(t, catalog, dumpFixture{entries: []dumpEntryFixture{{spec: spec, payload: `[{"id":"1","name":"HQ"}]`}}})
	collection, err := LoadCollection(context.Background(), dir, catalog)
	if err != nil {
		t.Fatalf("LoadCollection(%s) error = %v, want nil", dir, err)
	}
	catalog[0].Fields[0].AllowedModes[0] = redact.ModeParanoid
	got, err := collection.ListProjected(context.Background(), "zia", "locations")
	if err != nil {
		t.Fatalf("Collection.ListProjected(after catalog mutation) error = %v, want nil", err)
	}
	if !reflect.DeepEqual(got.Records()[0].Fields(), map[string]any{"id": "1", "name": "HQ"}) {
		t.Fatalf("Collection.ListProjected(after catalog mutation) fields = %#v, want original fields", got.Records()[0].Fields())
	}
}
