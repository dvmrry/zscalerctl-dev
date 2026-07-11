package cli

// dump_progress_test.go — White-box tests for collectDump's event plumbing.
// These exercise the same event stream that drives TTY progress without
// needing a TTY (the spinner is inactive in tests → zero stderr bytes).

import (
	"context"
	"io"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/config"
	"github.com/dvmrry/zscalerctl/internal/machine"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

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

// TestCollectDumpProgressEvents verifies the full lifecycle and one value-free
// progress event per selected resource in catalog order.
func TestCollectDumpProgressEvents(t *testing.T) {
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

	var events []machine.Event
	_, err := a.collectDump(
		context.Background(),
		config.Config{},
		globalOptions{},
		products,
		selectedResources,
		false, // continueOnError
		func(event machine.Event) error {
			events = append(events, event)
			return nil
		},
	)
	if err != nil {
		t.Fatalf("collectDump() error = %v, want nil", err)
	}

	wantKinds := []machine.EventKind{
		machine.EventStarted,
		machine.EventProgress,
		machine.EventProgress,
		machine.EventProgress,
		machine.EventCompleted,
	}
	if len(events) != len(wantKinds) {
		t.Fatalf("collectDump() events = %d, want %d", len(events), len(wantKinds))
	}
	for i, want := range wantKinds {
		if events[i].Kind != want {
			t.Fatalf("collectDump() event[%d].Kind = %q, want %q", i, events[i].Kind, want)
		}
	}

	const wantTotal = 3
	if events[0].Total != wantTotal {
		t.Errorf("started.Total = %d, want %d", events[0].Total, wantTotal)
	}

	// done must be 1-based and increment each call; total must always equal N.
	progressEvents := events[1:4]
	for i, event := range progressEvents {
		wantDone := i + 1
		if event.Done != wantDone {
			t.Errorf("progress[%d].Done = %d, want %d", i, event.Done, wantDone)
		}
		if event.Total != wantTotal {
			t.Errorf("progress[%d].Total = %d, want %d", i, event.Total, wantTotal)
		}
	}

	// The product/resource values must be catalog identifiers (not record data).
	// The catalog order is alpha, beta, gamma — assert that each fires exactly once.
	type key struct {
		p resources.Product
		r string
	}
	seen := make(map[key]bool, len(progressEvents))
	for _, event := range progressEvents {
		seen[key{resources.Product(event.Product), event.Resource}] = true
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
	completed := events[len(events)-1]
	if completed.Records != 0 || completed.Resources != wantTotal || completed.Warnings != 0 {
		t.Errorf("completed counts = records:%d resources:%d warnings:%d, want 0/%d/0",
			completed.Records, completed.Resources, completed.Warnings, wantTotal)
	}
}

// TestCollectDumpEventSinkNil verifies that a nil observer is safe. collectDump
// still supplies its internal logging sink to the collector.
func TestCollectDumpEventSinkNil(t *testing.T) {
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
		nil, // nil observer must not panic
	)
	if err != nil {
		t.Fatalf("collectDump(nil event sink) error = %v, want nil", err)
	}
}

// TestCollectDumpProgressEventSubset verifies that totals describe the selected
// subset, not the full catalog.
func TestCollectDumpProgressEventSubset(t *testing.T) {
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

	var progressEvents []machine.Event
	_, err := a.collectDump(
		context.Background(),
		config.Config{},
		globalOptions{},
		products,
		selectedResources,
		false,
		func(event machine.Event) error {
			if event.Kind == machine.EventProgress {
				progressEvents = append(progressEvents, event)
			}
			return nil
		},
	)
	if err != nil {
		t.Fatalf("collectDump() error = %v, want nil", err)
	}

	const wantTotal = 2 // only 2 of 3 selected
	if len(progressEvents) != wantTotal {
		t.Fatalf("progress events = %d, want %d (subset selected)", len(progressEvents), wantTotal)
	}
	for i, event := range progressEvents {
		if event.Total != wantTotal {
			t.Errorf("progress[%d].Total = %d, want %d (selectedCount)", i, event.Total, wantTotal)
		}
	}
}
