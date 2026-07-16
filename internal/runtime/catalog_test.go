package runtime

import (
	"context"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/machine"
	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

func TestDiscoverCatalogPreservesExplicitEmptyCatalog(t *testing.T) {
	t.Parallel()

	result, err := DiscoverCatalog(context.Background(), resources.ResourceCatalog{})
	if err != nil {
		t.Fatalf("DiscoverCatalog(explicit empty) error = %v, want nil", err)
	}
	if got := result.Catalog(); got == nil || len(got) != 0 {
		t.Fatalf("DiscoverCatalog(explicit empty).Catalog() = %#v, want initialized empty", got)
	}
}

func TestMachineDiscoverCatalogDoesNotInvokeReader(t *testing.T) {
	t.Parallel()

	reader := &runtimeFakeReader{}
	catalog := runtimeTestCatalog(t, resources.ProductZIA, "locations")
	rt := NewMachineFromReader(reader, catalog, redact.ModeStandard)
	result, err := rt.DiscoverCatalog(context.Background(), machine.CatalogRequest{})
	if err != nil {
		t.Fatalf("Machine.DiscoverCatalog() error = %v, want nil", err)
	}
	if len(reader.calls) != 0 {
		t.Fatalf("Machine.DiscoverCatalog() reader calls = %#v, want none", reader.calls)
	}
	got := result.Catalog()
	if len(got) != 1 || got[0].Product != resources.ProductZIA || got[0].Name != "locations" {
		t.Fatalf("Machine.DiscoverCatalog().Catalog() = %#v, want zia/locations", got)
	}
}
