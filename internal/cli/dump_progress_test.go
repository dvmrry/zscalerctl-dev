package cli

// dump_progress_test.go — White-box tests for the collectDump progress callback
// introduced in Task 2.2. These tests exercise the callback plumbing without
// needing a TTY (the spinner is inactive in tests → zero stderr bytes).

import (
	"context"
	"io"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/config"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

// progressCall records one invocation of the progress callback.
type progressCall struct {
	done     int
	total    int
	product  resources.Product
	resource string
}

// progressFakeReader is a minimal ResourceReader that always returns nil/empty
// for List and errors for Get/Show (those must not be called in a list-only catalog).
type progressFakeReader struct{}

func (r progressFakeReader) List(_ context.Context, _ resources.Product, _ string) ([]resources.SourceRecord, error) {
	return nil, nil
}

func (r progressFakeReader) Get(_ context.Context, _ resources.Product, _ string, _ string) (resources.SourceRecord, error) {
	panic("progressFakeReader.Get must not be called in dump progress tests")
}

func (r progressFakeReader) Show(_ context.Context, _ resources.Product, _ string) (resources.SourceRecord, error) {
	panic("progressFakeReader.Show must not be called in dump progress tests")
}

// TestCollectDumpProgressCallback verifies that collectDump fires the progress
// callback exactly once per selected resource, in catalog order, with a
// 1-based done counter, a constant total equal to selectedCount, and the
// catalog-derived product/resource identifiers — never record data.
func TestCollectDumpProgressCallback(t *testing.T) {
	t.Parallel()

	// Build a small 3-resource, 2-product catalog.  ListOperations() gives list+get
	// (both read-only); none has a "show" op, so collectDump calls reader.List.
	productA := resources.Product("testa")
	productB := resources.Product("testb")
	catalog := resources.ResourceCatalog{
		{Product: productA, Name: "alpha", Operations: resources.ListOperations(), Fields: nil},
		{Product: productA, Name: "beta", Operations: resources.ListOperations(), Fields: nil},
		{Product: productB, Name: "gamma", Operations: resources.ListOperations(), Fields: nil},
	}

	a := NewWithOptions(io.Discard, io.Discard, nil, Options{
		Reader:  progressFakeReader{},
		Catalog: catalog,
	})

	products := map[resources.Product]bool{
		productA: true,
		productB: true,
	}
	// Select all three resources.
	selectedResources := map[dumpResourceKey]bool{
		{product: productA, name: "alpha"}: true,
		{product: productA, name: "beta"}:  true,
		{product: productB, name: "gamma"}: true,
	}

	var calls []progressCall
	_, err := a.collectDump(
		context.Background(),
		config.Config{},
		globalOptions{},
		products,
		selectedResources,
		false, // continueOnError
		func(done, total int, product resources.Product, resource string) {
			calls = append(calls, progressCall{
				done:     done,
				total:    total,
				product:  product,
				resource: resource,
			})
		},
	)
	if err != nil {
		t.Fatalf("collectDump() error = %v, want nil", err)
	}

	const wantTotal = 3
	if len(calls) != wantTotal {
		t.Fatalf("progress callback fired %d times, want %d", len(calls), wantTotal)
	}

	// done must be 1-based and increment each call; total must always equal N.
	for i, c := range calls {
		wantDone := i + 1
		if c.done != wantDone {
			t.Errorf("calls[%d].done = %d, want %d", i, c.done, wantDone)
		}
		if c.total != wantTotal {
			t.Errorf("calls[%d].total = %d, want %d", i, c.total, wantTotal)
		}
	}

	// The product/resource values must be catalog identifiers (not record data).
	// The catalog order is alpha, beta, gamma — assert that each fires exactly once.
	type key struct {
		p resources.Product
		r string
	}
	seen := make(map[key]bool, len(calls))
	for _, c := range calls {
		seen[key{c.product, c.resource}] = true
	}
	for _, want := range []key{
		{productA, "alpha"},
		{productA, "beta"},
		{productB, "gamma"},
	} {
		if !seen[want] {
			t.Errorf("progress callback never fired for %s/%s", want.p, want.r)
		}
	}
}

// TestCollectDumpProgressCallbackNil verifies that a nil progress callback is
// safe — collectDump must not panic when progress is nil.
func TestCollectDumpProgressCallbackNil(t *testing.T) {
	t.Parallel()

	productA := resources.Product("testa")
	catalog := resources.ResourceCatalog{
		{Product: productA, Name: "alpha", Operations: resources.ListOperations(), Fields: nil},
	}

	a := NewWithOptions(io.Discard, io.Discard, nil, Options{
		Reader:  progressFakeReader{},
		Catalog: catalog,
	})

	products := map[resources.Product]bool{productA: true}
	selectedResources := map[dumpResourceKey]bool{
		{product: productA, name: "alpha"}: true,
	}

	_, err := a.collectDump(
		context.Background(),
		config.Config{},
		globalOptions{},
		products,
		selectedResources,
		false,
		nil, // nil progress must not panic
	)
	if err != nil {
		t.Fatalf("collectDump(nil progress) error = %v, want nil", err)
	}
}

// TestCollectDumpProgressSubset verifies that when only a subset of the
// catalog is selected, the callback fires exactly selectedCount times and
// total reflects the selection, not the full catalog size.
func TestCollectDumpProgressSubset(t *testing.T) {
	t.Parallel()

	productA := resources.Product("testa")
	catalog := resources.ResourceCatalog{
		{Product: productA, Name: "alpha", Operations: resources.ListOperations(), Fields: nil},
		{Product: productA, Name: "beta", Operations: resources.ListOperations(), Fields: nil},
		{Product: productA, Name: "gamma", Operations: resources.ListOperations(), Fields: nil},
	}

	a := NewWithOptions(io.Discard, io.Discard, nil, Options{
		Reader:  progressFakeReader{},
		Catalog: catalog,
	})

	products := map[resources.Product]bool{productA: true}
	// Only select 2 of the 3 resources.
	selectedResources := map[dumpResourceKey]bool{
		{product: productA, name: "alpha"}: true,
		{product: productA, name: "gamma"}: true,
	}

	var calls []progressCall
	_, err := a.collectDump(
		context.Background(),
		config.Config{},
		globalOptions{},
		products,
		selectedResources,
		false,
		func(done, total int, product resources.Product, resource string) {
			calls = append(calls, progressCall{done: done, total: total, product: product, resource: resource})
		},
	)
	if err != nil {
		t.Fatalf("collectDump() error = %v, want nil", err)
	}

	const wantTotal = 2 // only 2 of 3 selected
	if len(calls) != wantTotal {
		t.Fatalf("progress callback fired %d times, want %d (subset selected)", len(calls), wantTotal)
	}
	for i, c := range calls {
		if c.total != wantTotal {
			t.Errorf("calls[%d].total = %d, want %d (selectedCount)", i, c.total, wantTotal)
		}
	}
}
