//go:build !windows

package diff

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"github.com/dvmrry/zscalerctl/internal/resources"
)

func TestCompareRejectsFIFOPromptly(t *testing.T) {
	spec := testKeyedSpec()
	catalog := resources.ResourceCatalog{spec}
	oldDir := writeFIFODump(t, catalog, spec)
	newDir := writeTestDump(t, catalog, dumpFixture{
		entries: []dumpEntryFixture{{spec: spec, payload: `[{"id":"1","name":"HQ"}]`}},
	})
	assertPromptAdmissionError(t, func() error {
		_, err := Compare(oldDir, newDir, Options{Catalog: catalog})
		return err
	})
}

func TestLoadCollectionRejectsFIFOPromptly(t *testing.T) {
	spec := testKeyedSpec()
	catalog := resources.ResourceCatalog{spec}
	dir := writeFIFODump(t, catalog, spec)
	assertPromptAdmissionError(t, func() error {
		_, err := LoadCollection(context.Background(), dir, catalog)
		return err
	})
}

func writeFIFODump(t *testing.T, catalog resources.ResourceCatalog, spec resources.ResourceSpec) string {
	t.Helper()

	dir := writeTestDump(t, catalog, dumpFixture{
		entries: []dumpEntryFixture{{spec: spec, payload: `[{"id":"1","name":"HQ"}]`}},
	})
	path := filepath.Join(dir, "resources", string(spec.Product), spec.Name+".json")
	if err := os.Remove(path); err != nil {
		t.Fatalf("os.Remove(%q) error = %v", path, err)
	}
	if err := unix.Mkfifo(path, 0o600); err != nil {
		t.Skipf("unix.Mkfifo(%q) unavailable: %v", path, err)
	}
	return dir
}

func assertPromptAdmissionError(t *testing.T, run func() error) {
	t.Helper()

	done := make(chan error, 1)
	go func() {
		done <- run()
	}()
	select {
	case err := <-done:
		if err == nil || !errors.Is(err, ErrInvalidDump) {
			t.Fatalf("admission error = %v, want prompt ErrInvalidDump", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("dump admission did not reject the FIFO promptly")
	}
}
