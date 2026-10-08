package diff

import (
	"context"
	"errors"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/resources"
)

func TestLoadComparisonDumpsEnforcesAggregateBudget(t *testing.T) {
	spec := testKeyedSpec()
	catalog := resources.ResourceCatalog{spec}
	const payload = `[{"id":"1","name":"same"}]`
	oldDir := writeTestDump(t, catalog, dumpFixture{
		entries: []dumpEntryFixture{{spec: spec, payload: payload}},
	})
	newDir := writeTestDump(t, catalog, dumpFixture{
		entries: []dumpEntryFixture{{spec: spec, payload: payload}},
	})
	catalogSpecs, err := validateCatalog(catalog)
	if err != nil {
		t.Fatalf("validateCatalog() error = %v, want nil", err)
	}
	selected := map[ResourceKey]bool{{Product: spec.Product, Name: spec.Name}: true}
	resourceBudget := int64(len(payload))
	for _, dir := range []string{oldDir, newDir} {
		if _, err := loadDumpWithBudget(context.Background(), dir, catalogSpecs, selected, resourceBudget); err != nil {
			t.Fatalf("loadDumpWithBudget(%q, %d bytes) error = %v, want exact-limit success", dir, resourceBudget, err)
		}
	}
	if _, _, err := loadComparisonDumps(
		context.Background(),
		oldDir,
		newDir,
		catalogSpecs,
		selected,
		resourceBudget*2,
	); err != nil {
		t.Fatalf("loadComparisonDumps(exact aggregate budget) error = %v, want nil", err)
	}

	_, _, err = loadComparisonDumps(context.Background(), oldDir, newDir, catalogSpecs, selected, resourceBudget)
	if !errors.Is(err, ErrCollectionTooLarge) || !errors.Is(err, ErrInvalidDump) {
		t.Fatalf("loadComparisonDumps(zero remaining budget) error = %v, want aggregate size-limit and invalid-dump classification", err)
	}

	budget := resourceBudget*2 - 1
	_, _, err = loadComparisonDumps(context.Background(), oldDir, newDir, catalogSpecs, selected, budget)
	if !errors.Is(err, ErrCollectionTooLarge) || !errors.Is(err, ErrInvalidDump) {
		t.Fatalf("loadComparisonDumps(%d bytes) error = %v, want aggregate size-limit and invalid-dump classification", budget, err)
	}
}
