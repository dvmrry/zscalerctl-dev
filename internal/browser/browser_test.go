package browser_test

import (
	"context"
	"encoding/json"
	"errors"
	"os/exec"
	"reflect"
	"strings"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/browser"
	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

func TestResourcesFiltersDeterministically(t *testing.T) {
	catalog := resources.ResourceCatalog{
		testSpec(resources.ProductZPA, "server-groups", resources.ReadOperations()),
		testSpec(resources.ProductZIA, "url-categories", resources.ReadOperations()),
		testSpec(resources.ProductZIA, "locations", resources.ReadOperations()),
	}
	service := browser.Service{Catalog: catalog}

	got := service.Resources(browser.Filter{
		Products: []resources.Product{resources.ProductZIA},
	})
	want := []browser.ResourceInfo{
		{
			Product:    "zia",
			Name:       "locations",
			Label:      "locations",
			Operations: []string{"list", "get"},
		},
		{
			Product:    "zia",
			Name:       "url-categories",
			Label:      "url-categories",
			Operations: []string{"list", "get"},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Service.Resources(zia filter) = %#v, want %#v", got, want)
	}

	got = service.Resources(browser.Filter{
		Resources: []string{"server-groups"},
	})
	want = []browser.ResourceInfo{
		{
			Product:    "zpa",
			Name:       "server-groups",
			Label:      "server-groups",
			Operations: []string{"list", "get"},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Service.Resources(server-groups filter) = %#v, want %#v", got, want)
	}
}

func TestLoadListBackedResourceProjectsRedactsAndSummarizes(t *testing.T) {
	const canary = "abc123abc123abc123abc123abc123abc123"
	reader := &fakeReader{
		list: map[resourceKey][]resources.SourceRecord{
			{product: resources.ProductZIA, name: "locations"}: {
				resources.NewSourceRecord(map[string]any{
					"id":          "123",
					"name":        "HQ",
					"status":      "active",
					"description": "api_key=" + canary,
					"apiKey":      canary,
				}),
			},
		},
	}
	service := browser.Service{
		Catalog: resources.ResourceCatalog{
			testSpec(resources.ProductZIA, "locations", resources.ReadOperations()),
		},
		Reader: reader,
		Mode:   redact.ModeStandard,
	}

	got, err := service.Load(context.Background(), "zia", "locations")
	if err != nil {
		t.Fatalf("Service.Load(zia, locations) error = %v, want nil", err)
	}
	if wantCalls := []string{"list:zia/locations"}; !reflect.DeepEqual(reader.calls, wantCalls) {
		t.Fatalf("Service.Load(zia, locations) calls = %#v, want %#v", reader.calls, wantCalls)
	}
	if len(got) != 1 {
		t.Fatalf("Service.Load(zia, locations) records length = %d, want 1", len(got))
	}
	want := browser.Record{
		ID:     "123",
		Name:   "HQ",
		Status: "active",
		Fields: []browser.Field{
			{Key: "id", Value: "123"},
			{Key: "name", Value: "HQ"},
			{Key: "status", Value: "active"},
		},
	}
	if got[0].ID != want.ID || got[0].Name != want.Name || got[0].Status != want.Status {
		t.Errorf("Service.Load(zia, locations) summary = id:%q name:%q status:%q, want id:%q name:%q status:%q",
			got[0].ID, got[0].Name, got[0].Status, want.ID, want.Name, want.Status)
	}
	assertField(t, got[0], "id", "123")
	assertField(t, got[0], "name", "HQ")
	assertField(t, got[0], "status", "active")
	assertHasRedactedField(t, got[0], "description", canary)

	body, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("json.Marshal(Service.Load(zia, locations)) error = %v, want nil", err)
	}
	if strings.Contains(string(body), canary) || strings.Contains(string(body), "apiKey") {
		t.Fatalf("Service.Load(zia, locations) JSON = %s, want no secret-like canary or dropped secret key", body)
	}
}

func TestLoadShowBackedResourceUsesShowReader(t *testing.T) {
	reader := &fakeReader{
		show: map[resourceKey]resources.SourceRecord{
			{product: resources.ProductZIA, name: "advanced-settings"}: resources.NewSourceRecord(map[string]any{
				"id":     "settings",
				"name":   "Advanced Settings",
				"status": "enabled",
			}),
		},
	}
	service := browser.Service{
		Catalog: resources.ResourceCatalog{
			testSpec(resources.ProductZIA, "advanced-settings", resources.ShowOperation()),
		},
		Reader: reader,
		Mode:   redact.ModeStandard,
	}

	got, err := service.Load(context.Background(), "zia", "advanced-settings")
	if err != nil {
		t.Fatalf("Service.Load(zia, advanced-settings) error = %v, want nil", err)
	}
	if wantCalls := []string{"show:zia/advanced-settings"}; !reflect.DeepEqual(reader.calls, wantCalls) {
		t.Fatalf("Service.Load(zia, advanced-settings) calls = %#v, want %#v", reader.calls, wantCalls)
	}
	if len(got) != 1 {
		t.Fatalf("Service.Load(zia, advanced-settings) records length = %d, want 1", len(got))
	}
	if got[0].Name != "Advanced Settings" || got[0].Status != "enabled" {
		t.Errorf("Service.Load(zia, advanced-settings) summary = name:%q status:%q, want name:%q status:%q",
			got[0].Name, got[0].Status, "Advanced Settings", "enabled")
	}
}

func TestLoadProjectedReturnsProjectedRecords(t *testing.T) {
	const canary = "abc123abc123abc123abc123abc123abc123"
	reader := &fakeReader{
		list: map[resourceKey][]resources.SourceRecord{
			{product: resources.ProductZIA, name: "locations"}: {
				resources.NewSourceRecord(map[string]any{
					"id":          "123",
					"name":        "HQ",
					"description": "api_key=" + canary,
					"apiKey":      canary,
				}),
			},
		},
	}
	service := browser.Service{
		Catalog: resources.ResourceCatalog{
			testSpec(resources.ProductZIA, "locations", resources.ReadOperations()),
		},
		Reader: reader,
		Mode:   redact.ModeStandard,
	}

	got, err := service.LoadProjected(context.Background(), "zia", "locations")
	if err != nil {
		t.Fatalf("Service.LoadProjected(zia, locations) error = %v, want nil", err)
	}
	if wantCalls := []string{"list:zia/locations"}; !reflect.DeepEqual(reader.calls, wantCalls) {
		t.Fatalf("Service.LoadProjected(zia, locations) calls = %#v, want %#v", reader.calls, wantCalls)
	}
	records := got.Records()
	if len(records) != 1 {
		t.Fatalf("Service.LoadProjected(zia, locations) records length = %d, want 1", len(records))
	}
	fields := records[0].Fields()
	if _, ok := fields["apiKey"]; ok {
		t.Fatalf("Service.LoadProjected(zia, locations) fields = %#v, want dropped apiKey", fields)
	}
	body, err := json.Marshal(fields)
	if err != nil {
		t.Fatalf("json.Marshal(Service.LoadProjected(zia, locations).Fields()) error = %v, want nil", err)
	}
	if strings.Contains(string(body), canary) {
		t.Fatalf("Service.LoadProjected(zia, locations) fields JSON = %s, want canary redacted", body)
	}
}

func TestLoadProjectedByIDReturnsProjectedRedactedRecord(t *testing.T) {
	const canary = "abc123abc123abc123abc123abc123abc123"
	spec := testSpec(resources.ProductZIA, "locations", resources.ReadOperations())
	spec.Fields = append(spec.Fields,
		resources.FieldSpec{
			Name:           "apiKey",
			Classification: resources.ClassSecret,
		},
		resources.FieldSpec{
			Name:           "password",
			Classification: resources.ClassSecret,
		},
	)
	reader := &fakeReader{
		get: map[resourceIDKey]resources.SourceRecord{
			{product: resources.ProductZIA, name: "locations", id: "123"}: resources.NewSourceRecord(map[string]any{
				"id":          "123",
				"name":        "HQ",
				"status":      "active",
				"description": "token=" + canary,
				"apiKey":      canary,
				"password":    "hunter2",
				"rawOnly":     canary,
			}),
		},
	}
	service := browser.Service{
		Catalog: resources.ResourceCatalog{spec},
		Reader:  reader,
		Mode:    redact.ModeStandard,
	}

	got, err := service.LoadProjectedByID(context.Background(), "zia", "locations", "123")
	if err != nil {
		t.Fatalf("Service.LoadProjectedByID(zia, locations, 123) error = %v, want nil", err)
	}
	if wantCalls := []string{"get:zia/locations/123"}; !reflect.DeepEqual(reader.calls, wantCalls) {
		t.Fatalf("Service.LoadProjectedByID(zia, locations, 123) calls = %#v, want %#v", reader.calls, wantCalls)
	}
	records := got.Records()
	if len(records) != 1 {
		t.Fatalf("Service.LoadProjectedByID(zia, locations, 123) records length = %d, want 1", len(records))
	}
	fields := records[0].Fields()
	for key, want := range map[string]any{
		"id":     "123",
		"name":   "HQ",
		"status": "active",
	} {
		if got := fields[key]; !reflect.DeepEqual(got, want) {
			t.Fatalf("Service.LoadProjectedByID(zia, locations, 123) field %q = %#v, want %#v", key, got, want)
		}
	}
	for _, key := range []string{"apiKey", "password", "rawOnly"} {
		if _, ok := fields[key]; ok {
			t.Fatalf("Service.LoadProjectedByID(zia, locations, 123) fields = %#v, want dropped %q", fields, key)
		}
	}
	body, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("json.Marshal(Service.LoadProjectedByID(zia, locations, 123)) error = %v, want nil", err)
	}
	if strings.Contains(string(body), canary) ||
		strings.Contains(string(body), "apiKey") ||
		strings.Contains(string(body), "password") ||
		strings.Contains(string(body), "rawOnly") {
		t.Fatalf("Service.LoadProjectedByID(zia, locations, 123) JSON = %s, want no raw or secret fields", body)
	}
}

func TestLoadProjectedByIDRejectsMissingIDBeforeReader(t *testing.T) {
	for _, id := range []string{"", " \t "} {
		t.Run("id="+id, func(t *testing.T) {
			reader := &fakeReader{}
			service := browser.Service{
				Catalog: resources.ResourceCatalog{
					testSpec(resources.ProductZIA, "locations", resources.ReadOperations()),
				},
				Reader: reader,
				Mode:   redact.ModeStandard,
			}

			_, err := service.LoadProjectedByID(context.Background(), "zia", "locations", id)
			if !errors.Is(err, browser.ErrMissingID) {
				t.Fatalf("Service.LoadProjectedByID(zia, locations, %q) error = %v, want errors.Is ErrMissingID", id, err)
			}
			if len(reader.calls) != 0 {
				t.Fatalf("Service.LoadProjectedByID(zia, locations, %q) calls = %#v, want none", id, reader.calls)
			}
		})
	}
}

func TestLoadProjectedByIDUnknownResourceReturnsCleanErrorBeforeReader(t *testing.T) {
	reader := &fakeReader{}
	service := browser.Service{
		Catalog: resources.ResourceCatalog{
			testSpec(resources.ProductZIA, "locations", resources.ReadOperations()),
		},
		Reader: reader,
		Mode:   redact.ModeStandard,
	}

	_, err := service.LoadProjectedByID(context.Background(), "zia", "missing", "123")
	if !errors.Is(err, browser.ErrUnknownResource) {
		t.Fatalf("Service.LoadProjectedByID(zia, missing, 123) error = %v, want errors.Is ErrUnknownResource", err)
	}
	if len(reader.calls) != 0 {
		t.Fatalf("Service.LoadProjectedByID(zia, missing, 123) calls = %#v, want none", reader.calls)
	}
}

func TestLoadProjectedByIDResourceWithoutGetSupportReturnsUsageBoundaryError(t *testing.T) {
	reader := &fakeReader{}
	service := browser.Service{
		Catalog: resources.ResourceCatalog{
			testSpec(resources.ProductZIA, "advanced-settings", resources.ShowOperation()),
		},
		Reader: reader,
		Mode:   redact.ModeStandard,
	}

	_, err := service.LoadProjectedByID(context.Background(), "zia", "advanced-settings", "123")
	if !errors.Is(err, browser.ErrUnsupportedLoad) {
		t.Fatalf("Service.LoadProjectedByID(zia, advanced-settings, 123) error = %v, want errors.Is ErrUnsupportedLoad", err)
	}
	if len(reader.calls) != 0 {
		t.Fatalf("Service.LoadProjectedByID(zia, advanced-settings, 123) calls = %#v, want none", reader.calls)
	}
}

func TestLoadProjectedByIDReturnsReaderGetError(t *testing.T) {
	backendErr := errors.New("backend failed")
	reader := &fakeReader{getErr: backendErr}
	service := browser.Service{
		Catalog: resources.ResourceCatalog{
			testSpec(resources.ProductZIA, "locations", resources.ReadOperations()),
		},
		Reader: reader,
		Mode:   redact.ModeStandard,
	}

	_, err := service.LoadProjectedByID(context.Background(), "zia", "locations", "123")
	if !errors.Is(err, backendErr) {
		t.Fatalf("Service.LoadProjectedByID(zia, locations, 123) error = %v, want errors.Is backendErr", err)
	}
	if wantCalls := []string{"get:zia/locations/123"}; !reflect.DeepEqual(reader.calls, wantCalls) {
		t.Fatalf("Service.LoadProjectedByID(zia, locations, 123) calls = %#v, want %#v", reader.calls, wantCalls)
	}
}

func TestLoadUnknownResourceReturnsCleanErrorBeforeReader(t *testing.T) {
	reader := &fakeReader{}
	service := browser.Service{
		Catalog: resources.ResourceCatalog{
			testSpec(resources.ProductZIA, "locations", resources.ReadOperations()),
		},
		Reader: reader,
		Mode:   redact.ModeStandard,
	}

	_, err := service.Load(context.Background(), "zia", "missing")
	if !errors.Is(err, browser.ErrUnknownResource) {
		t.Fatalf("Service.Load(zia, missing) error = %v, want errors.Is ErrUnknownResource", err)
	}
	if len(reader.calls) != 0 {
		t.Fatalf("Service.Load(zia, missing) calls = %#v, want none", reader.calls)
	}
}

func TestLoadEmptyResourceReturnsEmptyRecords(t *testing.T) {
	reader := &fakeReader{
		list: map[resourceKey][]resources.SourceRecord{
			{product: resources.ProductZIA, name: "locations"}: nil,
		},
	}
	service := browser.Service{
		Catalog: resources.ResourceCatalog{
			testSpec(resources.ProductZIA, "locations", resources.ReadOperations()),
		},
		Reader: reader,
		Mode:   redact.ModeStandard,
	}

	got, err := service.Load(context.Background(), "zia", "locations")
	if err != nil {
		t.Fatalf("Service.Load(zia, locations) error = %v, want nil", err)
	}
	if len(got) != 0 {
		t.Fatalf("Service.Load(zia, locations) records length = %d, want 0", len(got))
	}
}

func TestBrowserPackageDoesNotImportCLIOrUIRuntime(t *testing.T) {
	cmd := exec.Command("go", "list", "-deps", "-mod=vendor", "github.com/dvmrry/zscalerctl/internal/browser")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go list internal/browser error = %v\n%s", err, out)
	}
	for _, forbidden := range []string{
		"github.com/dvmrry/zscalerctl/internal/cli",
		"github.com/spf13/cobra",
		"bubbletea",
		"bubbles",
		"wails",
		"react",
		"vite",
		"lipgloss",
	} {
		if strings.Contains(string(out), forbidden) {
			t.Fatalf("internal/browser deps include %q, want no CLI or UI/runtime dependency\n%s", forbidden, out)
		}
	}
}

type resourceKey struct {
	product resources.Product
	name    string
}

type resourceIDKey struct {
	product resources.Product
	name    string
	id      string
}

type fakeReader struct {
	list   map[resourceKey][]resources.SourceRecord
	show   map[resourceKey]resources.SourceRecord
	get    map[resourceIDKey]resources.SourceRecord
	getErr error
	calls  []string
}

func (r *fakeReader) List(_ context.Context, product resources.Product, name string) ([]resources.SourceRecord, error) {
	r.calls = append(r.calls, "list:"+string(product)+"/"+name)
	if r.list == nil {
		return nil, nil
	}
	return r.list[resourceKey{product: product, name: name}], nil
}

func (r *fakeReader) Show(_ context.Context, product resources.Product, name string) (resources.SourceRecord, error) {
	r.calls = append(r.calls, "show:"+string(product)+"/"+name)
	if r.show == nil {
		return resources.SourceRecord{}, nil
	}
	return r.show[resourceKey{product: product, name: name}], nil
}

func (r *fakeReader) Get(_ context.Context, product resources.Product, name string, id string) (resources.SourceRecord, error) {
	r.calls = append(r.calls, "get:"+string(product)+"/"+name+"/"+id)
	if r.getErr != nil {
		return resources.SourceRecord{}, r.getErr
	}
	if r.get == nil {
		return resources.SourceRecord{}, nil
	}
	return r.get[resourceIDKey{product: product, name: name, id: id}], nil
}

func testSpec(product resources.Product, name string, operations []resources.Operation) resources.ResourceSpec {
	return resources.ResourceSpec{
		Product:    product,
		Name:       name,
		Operations: operations,
		Fields: []resources.FieldSpec{
			{
				Name:           "id",
				Classification: resources.ClassOperational,
				AllowedModes:   allModes(),
			},
			{
				Name:           "name",
				Classification: resources.ClassTenantConfig,
				AllowedModes:   []redact.Mode{redact.ModeStandard, redact.ModeShare},
			},
			{
				Name:           "status",
				Classification: resources.ClassOperational,
				AllowedModes:   allModes(),
			},
			{
				Name:                   "description",
				Classification:         resources.ClassFreeText,
				AllowedModes:           []redact.Mode{redact.ModeStandard},
				StandardFreeTextReason: "test free text",
			},
		},
	}
}

func allModes() []redact.Mode {
	return []redact.Mode{redact.ModeStandard, redact.ModeShare, redact.ModeParanoid}
}

func assertField(t *testing.T, record browser.Record, key string, want any) {
	t.Helper()
	for _, field := range record.Fields {
		if field.Key == key {
			if !reflect.DeepEqual(field.Value, want) {
				t.Fatalf("Service.Load field %q = %#v, want %#v", key, field.Value, want)
			}
			return
		}
	}
	t.Fatalf("Service.Load fields missing %q; got %#v", key, record.Fields)
}

func assertHasRedactedField(t *testing.T, record browser.Record, key, forbidden string) {
	t.Helper()
	for _, field := range record.Fields {
		if field.Key != key {
			continue
		}
		value, ok := field.Value.(string)
		if !ok {
			t.Fatalf("Service.Load field %q = %T, want string", key, field.Value)
		}
		if strings.Contains(value, forbidden) {
			t.Fatalf("Service.Load field %q = %q, want canary redacted", key, value)
		}
		return
	}
	t.Fatalf("Service.Load fields missing redacted field %q; got %#v", key, record.Fields)
}
