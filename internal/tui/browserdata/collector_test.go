package browserdata

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
	"github.com/dvmrry/zscalerctl/internal/tui/data"
)

func TestCollectorAllResourcesSuccess(t *testing.T) {
	reader := fakeReader{
		"zia/locations": {records: []resources.SourceRecord{
			resources.NewSourceRecord(map[string]any{"id": "1", "name": "A"}),
		}},
		"zpa/application-segments": {records: []resources.SourceRecord{
			resources.NewSourceRecord(map[string]any{"id": "2", "name": "B"}),
		}},
	}
	collector := &Collector{
		Catalog: resources.ResourceCatalog{
			{Product: resources.ProductZIA, Name: "locations", Operations: resources.ReadOperations(), Fields: standardFields()},
			{Product: resources.ProductZPA, Name: "application-segments", Operations: resources.ReadOperations(), Fields: standardFields()},
		},
		Reader: reader,
		Mode:   redact.ModeStandard,
	}
	data, err := collector.Collect(context.Background(), CollectOptions{})
	if err != nil {
		t.Fatalf("Collect error = %v", err)
	}
	if len(data.Products) != 2 {
		t.Fatalf("products = %d, want 2", len(data.Products))
	}
	if data.Products[0].Resources[0].Name != "locations" {
		t.Errorf("zia resource = %q, want locations", data.Products[0].Resources[0].Name)
	}
	if data.Products[1].Resources[0].Name != "application-segments" {
		t.Errorf("zpa resource = %q, want application-segments", data.Products[1].Resources[0].Name)
	}
}

func TestCollectorProductSubset(t *testing.T) {
	reader := fakeReader{
		"zia/locations": {records: []resources.SourceRecord{
			resources.NewSourceRecord(map[string]any{"id": "1", "name": "A"}),
		}},
		"zpa/application-segments": {records: []resources.SourceRecord{
			resources.NewSourceRecord(map[string]any{"id": "2", "name": "B"}),
		}},
	}
	collector := &Collector{
		Catalog: resources.ResourceCatalog{
			{Product: resources.ProductZIA, Name: "locations", Operations: resources.ReadOperations(), Fields: standardFields()},
			{Product: resources.ProductZPA, Name: "application-segments", Operations: resources.ReadOperations(), Fields: standardFields()},
		},
		Reader: reader,
		Mode:   redact.ModeStandard,
	}
	data, err := collector.Collect(context.Background(), CollectOptions{Products: []resources.Product{resources.ProductZIA}})
	if err != nil {
		t.Fatalf("Collect error = %v", err)
	}
	if len(data.Products) != 1 {
		t.Fatalf("products = %d, want 1", len(data.Products))
	}
	if data.Products[0].Name != "zia" {
		t.Errorf("product = %q, want zia", data.Products[0].Name)
	}
	if len(data.Products[0].Resources) != 1 {
		t.Fatalf("resources = %d, want 1", len(data.Products[0].Resources))
	}
}

func TestCollectorResourceSubset(t *testing.T) {
	reader := fakeReader{
		"zia/locations": {records: []resources.SourceRecord{
			resources.NewSourceRecord(map[string]any{"id": "1", "name": "A"}),
		}},
		"zia/url-filtering-rules": {records: []resources.SourceRecord{
			resources.NewSourceRecord(map[string]any{"id": "2", "name": "B"}),
		}},
	}
	collector := &Collector{
		Catalog: resources.ResourceCatalog{
			{Product: resources.ProductZIA, Name: "locations", Operations: resources.ReadOperations(), Fields: standardFields()},
			{Product: resources.ProductZIA, Name: "url-filtering-rules", Operations: resources.ReadOperations(), Fields: standardFields()},
		},
		Reader: reader,
		Mode:   redact.ModeStandard,
	}
	data, err := collector.Collect(context.Background(), CollectOptions{Resources: []string{"url-filtering-rules"}})
	if err != nil {
		t.Fatalf("Collect error = %v", err)
	}
	if len(data.Products) != 1 || len(data.Products[0].Resources) != 1 {
		t.Fatalf("unexpected shape: %+v", data.Products)
	}
	if data.Products[0].Resources[0].Name != "url-filtering-rules" {
		t.Errorf("resource = %q, want url-filtering-rules", data.Products[0].Resources[0].Name)
	}
}

func TestCollectorEmptyResource(t *testing.T) {
	reader := fakeReader{}
	collector := &Collector{
		Catalog: resources.ResourceCatalog{
			{Product: resources.ProductZIA, Name: "forwarding-rules", Operations: resources.ReadOperations(), Fields: standardFields()},
		},
		Reader: reader,
		Mode:   redact.ModeStandard,
	}
	data, err := collector.Collect(context.Background(), CollectOptions{})
	if err != nil {
		t.Fatalf("Collect error = %v", err)
	}
	res := data.Products[0].Resources[0]
	if !res.Empty {
		t.Errorf("Empty = false, want true")
	}
}

func TestCollectorResourceErrorContinueOnError(t *testing.T) {
	reader := fakeReader{
		"zia/locations": {records: []resources.SourceRecord{
			resources.NewSourceRecord(map[string]any{"id": "1", "name": "A"}),
		}},
		"zpa/app-connectors": {err: errors.New("connector list unavailable")},
	}
	collector := &Collector{
		Catalog: resources.ResourceCatalog{
			{Product: resources.ProductZIA, Name: "locations", Operations: resources.ReadOperations(), Fields: standardFields()},
			{Product: resources.ProductZPA, Name: "app-connectors", Operations: resources.ReadOperations(), Fields: standardFields()},
		},
		Reader: reader,
		Mode:   redact.ModeStandard,
	}
	data, err := collector.Collect(context.Background(), CollectOptions{ContinueOnError: true})
	if err != nil {
		t.Fatalf("Collect error = %v", err)
	}
	if data.Products[0].Resources[0].Records[0].Name != "A" {
		t.Errorf("zia record = %q, want A", data.Products[0].Resources[0].Records[0].Name)
	}
	if data.Products[1].Resources[0].Error != "connector list unavailable" {
		t.Errorf("zpa error = %q, want connector list unavailable", data.Products[1].Resources[0].Error)
	}
}

func TestCollectorResourceErrorFailFast(t *testing.T) {
	reader := fakeReader{
		"zia/locations": {records: []resources.SourceRecord{
			resources.NewSourceRecord(map[string]any{"id": "1", "name": "A"}),
		}},
		"zpa/app-connectors": {err: errors.New("connector list unavailable")},
	}
	collector := &Collector{
		Catalog: resources.ResourceCatalog{
			{Product: resources.ProductZIA, Name: "locations", Operations: resources.ReadOperations(), Fields: standardFields()},
			{Product: resources.ProductZPA, Name: "app-connectors", Operations: resources.ReadOperations(), Fields: standardFields()},
		},
		Reader: reader,
		Mode:   redact.ModeStandard,
	}
	_, err := collector.Collect(context.Background(), CollectOptions{})
	if err == nil {
		t.Fatalf("Collect error = nil, want connector list unavailable")
	}
	if !strings.Contains(err.Error(), "connector list unavailable") {
		t.Errorf("error = %q, want connector list unavailable", err.Error())
	}
}

func TestCollectorLoadResourceSuccess(t *testing.T) {
	fields := append(standardFields(),
		resources.FieldSpec{Name: "password", Classification: resources.ClassSecret},
	)
	reader := fakeReader{
		"zia/locations": {records: []resources.SourceRecord{
			resources.NewSourceRecord(map[string]any{"id": "1", "name": "A", "password": "hunter2"}),
		}},
	}
	collector := &Collector{
		Catalog: resources.ResourceCatalog{
			{Product: resources.ProductZIA, Name: "locations", Operations: resources.ReadOperations(), Fields: fields},
		},
		Reader: reader,
		Mode:   redact.ModeStandard,
	}

	node := collector.LoadResource(context.Background(), "zia", "locations")
	if got := node.EffectiveState(); got != data.ResourceStateLoaded {
		t.Fatalf("LoadResource state = %s, want %s", got, data.ResourceStateLoaded)
	}
	if len(node.Records) != 1 {
		t.Fatalf("LoadResource records = %d, want 1", len(node.Records))
	}
	if node.Records[0].Name != "A" {
		t.Errorf("LoadResource record name = %q, want A", node.Records[0].Name)
	}
	for _, f := range node.Records[0].Fields {
		if f.Key == "password" || strings.Contains(f.Value, "hunter2") {
			t.Errorf("secret leaked through LoadResource field %q=%q", f.Key, f.Value)
		}
	}
}

func TestCollectorLoadResourceErrorIsDisplayState(t *testing.T) {
	reader := fakeReader{
		"zia/locations": {err: errors.New("api failed client_secret=hunter2")},
	}
	collector := &Collector{
		Catalog: resources.ResourceCatalog{
			{Product: resources.ProductZIA, Name: "locations", Operations: resources.ReadOperations(), Fields: standardFields()},
		},
		Reader: reader,
		Mode:   redact.ModeStandard,
	}

	node := collector.LoadResource(context.Background(), "zia", "locations")
	if got := node.EffectiveState(); got != data.ResourceStateError {
		t.Fatalf("LoadResource state = %s, want %s", got, data.ResourceStateError)
	}
	if !strings.Contains(node.Error, "api failed") {
		t.Errorf("LoadResource error = %q, want api failed context", node.Error)
	}
	if strings.Contains(node.Error, "hunter2") {
		t.Errorf("LoadResource error = %q, want secret value redacted", node.Error)
	}
	if len(node.Records) != 0 {
		t.Errorf("LoadResource records on error = %d, want 0", len(node.Records))
	}
}

func TestCollectorContextCancellation(t *testing.T) {
	reader := slowReader{}
	collector := &Collector{
		Catalog: resources.ResourceCatalog{
			{Product: resources.ProductZIA, Name: "locations", Operations: resources.ReadOperations(), Fields: standardFields()},
		},
		Reader: reader,
		Mode:   redact.ModeStandard,
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := collector.Collect(ctx, CollectOptions{})
	if err == nil {
		t.Fatalf("Collect error = nil, want context error")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error = %q, want context.Canceled", err)
	}
}

func TestCollectorSecretRedaction(t *testing.T) {
	fields := append(standardFields(),
		resources.FieldSpec{Name: "password", Classification: resources.ClassSecret},
	)
	reader := fakeReader{
		"zia/locations": {records: []resources.SourceRecord{
			resources.NewSourceRecord(map[string]any{"id": "1", "name": "A", "password": "hunter2"}),
		}},
	}
	collector := &Collector{
		Catalog: resources.ResourceCatalog{
			{Product: resources.ProductZIA, Name: "locations", Operations: resources.ReadOperations(), Fields: fields},
		},
		Reader: reader,
		Mode:   redact.ModeStandard,
	}
	data, err := collector.Collect(context.Background(), CollectOptions{})
	if err != nil {
		t.Fatalf("Collect error = %v", err)
	}
	rec := data.Products[0].Resources[0].Records[0]
	for _, f := range rec.Fields {
		if f.Key == "password" {
			t.Errorf("secret field %q leaked into data.BrowserData", f.Key)
		}
		if strings.Contains(f.Value, "hunter2") {
			t.Errorf("secret value leaked in field %q=%q", f.Key, f.Value)
		}
	}
}

func TestCollectorDeterministicOrdering(t *testing.T) {
	reader := fakeReader{
		"zia/locations": {records: []resources.SourceRecord{
			resources.NewSourceRecord(map[string]any{"id": "1", "name": "A"}),
		}},
		"zpa/app-connectors": {records: []resources.SourceRecord{
			resources.NewSourceRecord(map[string]any{"id": "2", "name": "B"}),
		}},
		"zpa/application-segments": {records: []resources.SourceRecord{
			resources.NewSourceRecord(map[string]any{"id": "3", "name": "C"}),
		}},
	}
	collector := &Collector{
		Catalog: resources.ResourceCatalog{
			{Product: resources.ProductZIA, Name: "locations", Operations: resources.ReadOperations(), Fields: standardFields()},
			{Product: resources.ProductZPA, Name: "app-connectors", Operations: resources.ReadOperations(), Fields: standardFields()},
			{Product: resources.ProductZPA, Name: "application-segments", Operations: resources.ReadOperations(), Fields: standardFields()},
		},
		Reader: reader,
		Mode:   redact.ModeStandard,
	}
	data, err := collector.Collect(context.Background(), CollectOptions{})
	if err != nil {
		t.Fatalf("Collect error = %v", err)
	}
	if data.Products[0].Name != "zia" || data.Products[0].Resources[0].Name != "locations" {
		t.Errorf("first product/resource = %s/%s, want zia/locations", data.Products[0].Name, data.Products[0].Resources[0].Name)
	}
	if data.Products[1].Name != "zpa" || data.Products[1].Resources[0].Name != "app-connectors" || data.Products[1].Resources[1].Name != "application-segments" {
		t.Errorf("zpa resources = %v, want [app-connectors application-segments]", data.Products[1].Resources)
	}
}

func TestCollectorFixtureReader(t *testing.T) {
	collector := NewCollectorFixture()
	data, err := collector.Collect(context.Background(), CollectOptions{ContinueOnError: true})
	if err != nil {
		t.Fatalf("Collect error = %v", err)
	}
	if len(data.Products) != 3 {
		t.Errorf("products = %d, want 3", len(data.Products))
	}
	// Verify the error resource is preserved as a display-only error state.
	for _, p := range data.Products {
		for _, r := range p.Resources {
			if r.Name == "app-connectors" && r.Error != "connector list unavailable" {
				t.Errorf("app-connectors error = %q, want connector list unavailable", r.Error)
			}
		}
	}
	// Verify no secret material leaked through the fixture projection path.
	for _, p := range data.Products {
		for _, r := range p.Resources {
			for _, rec := range r.Records {
				for _, f := range rec.Fields {
					lower := strings.ToLower(f.Key)
					if strings.Contains(lower, "secret") || strings.Contains(lower, "password") || strings.Contains(lower, "psk") || strings.Contains(lower, "credential") {
						t.Errorf("secret-like key %q found in data.BrowserData", f.Key)
					}
					if strings.Contains(f.Value, "secret-key-material") || strings.Contains(f.Value, "secret") {
						t.Errorf("secret value leaked in field %q=%q", f.Key, f.Value)
					}
				}
			}
		}
	}
}

// fakeReader is a configurable RecordReader for tests.
type fakeReader map[string]readerEntry

type readerEntry struct {
	records []resources.SourceRecord
	err     error
}

func (r fakeReader) List(ctx context.Context, product resources.Product, resource string) ([]resources.SourceRecord, error) {
	key := string(product) + "/" + resource
	entry := r[key]
	if entry.err != nil {
		return nil, entry.err
	}
	return entry.records, nil
}

func (r fakeReader) Show(ctx context.Context, product resources.Product, resource string) (resources.SourceRecord, error) {
	records, err := r.List(ctx, product, resource)
	if err != nil {
		return resources.SourceRecord{}, err
	}
	if len(records) == 0 {
		return resources.SourceRecord{}, fmt.Errorf("singleton %s/%s not found", product, resource)
	}
	return records[0], nil
}

// slowReader blocks until its context is cancelled, so it can be used to test
// context cancellation in the collector.
type slowReader struct{}

func (slowReader) List(ctx context.Context, product resources.Product, resource string) ([]resources.SourceRecord, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func (slowReader) Show(ctx context.Context, product resources.Product, resource string) (resources.SourceRecord, error) {
	<-ctx.Done()
	return resources.SourceRecord{}, ctx.Err()
}
