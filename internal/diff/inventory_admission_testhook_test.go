//go:build unix && zscalerctl_engine_testhooks

package diff

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"github.com/dvmrry/zscalerctl/internal/dump"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

type inventoryAdmissionTestCase struct {
	name string
	run  func(context.Context, string, string) error
}

func inventoryAdmissionTestCases(catalog resources.ResourceCatalog) []inventoryAdmissionTestCase {
	return []inventoryAdmissionTestCase{
		{
			name: "CompareContext",
			run: func(ctx context.Context, oldDir, newDir string) error {
				_, err := CompareContext(ctx, oldDir, newDir, Options{Catalog: catalog}, nil)
				return err
			},
		},
		{
			name: "LoadCollection",
			run: func(ctx context.Context, oldDir, _ string) error {
				_, err := LoadCollection(ctx, oldDir, catalog)
				return err
			},
		},
	}
}

func TestInventoryRejectsDirectoryToFIFOSubstitutionBeforeEnumeration(t *testing.T) {
	spec := testKeyedSpec()
	catalog := resources.ResourceCatalog{spec}
	for _, admission := range inventoryAdmissionTestCases(catalog) {
		t.Run(admission.name, func(t *testing.T) {
			oldDir := writeTestDump(t, catalog, dumpFixture{
				entries: []dumpEntryFixture{{spec: spec, payload: `[{"id":"1","name":"HQ"}]`}},
			})
			newDir := writeTestDump(t, catalog, dumpFixture{
				entries: []dumpEntryFixture{{spec: spec, payload: `[{"id":"1","name":"HQ"}]`}},
			})
			resourcePath := filepath.Join(oldDir, "resources")
			hook := installInventoryTestHook(t, resourcePath)
			done := make(chan error, 1)
			go func() {
				done <- admission.run(context.Background(), oldDir, newDir)
			}()
			waitForInventoryTestHook(t, hook, done)

			movedPath := filepath.Join(filepath.Dir(oldDir), "resources-original")
			if err := os.Rename(resourcePath, movedPath); err != nil {
				hook.Release()
				t.Fatalf("os.Rename(%q, %q) error = %v", resourcePath, movedPath, err)
			}
			if err := unix.Mkfifo(resourcePath, 0o600); err != nil {
				hook.Release()
				t.Fatalf("unix.Mkfifo(%q) error = %v", resourcePath, err)
			}
			hook.Release()
			awaitInventoryAdmissionError(t, done, ErrInvalidDump)
		})
	}
}

func TestInventoryCancellationReturnsPromptlyAfterDirectoryValidation(t *testing.T) {
	spec := testKeyedSpec()
	catalog := resources.ResourceCatalog{spec}
	for _, admission := range inventoryAdmissionTestCases(catalog) {
		t.Run(admission.name, func(t *testing.T) {
			oldDir := writeTestDump(t, catalog, dumpFixture{
				entries: []dumpEntryFixture{{spec: spec, payload: `[{"id":"1","name":"HQ"}]`}},
			})
			newDir := writeTestDump(t, catalog, dumpFixture{
				entries: []dumpEntryFixture{{spec: spec, payload: `[{"id":"1","name":"HQ"}]`}},
			})
			hook := installInventoryTestHook(t, filepath.Join(oldDir, "resources"))
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan error, 1)
			go func() {
				done <- admission.run(ctx, oldDir, newDir)
			}()
			waitForInventoryTestHook(t, hook, done)

			cancel()
			hook.Release()
			awaitInventoryAdmissionError(t, done, context.Canceled)
		})
	}
}

type inventoryTestHook struct {
	reached     chan struct{}
	release     chan struct{}
	fireOnce    *sync.Once
	releaseOnce *sync.Once
}

// Release lets the paused traversal continue. It is safe to call repeatedly.
func (h inventoryTestHook) Release() {
	h.releaseOnce.Do(func() { close(h.release) })
}

// installInventoryTestHook pauses inventory traversal once, after the
// directory at target has been opened and validated and before it is
// enumerated. Other directories pass through without pausing.
func installInventoryTestHook(t *testing.T, target string) inventoryTestHook {
	t.Helper()
	hook := inventoryTestHook{
		reached:     make(chan struct{}),
		release:     make(chan struct{}),
		fireOnce:    &sync.Once{},
		releaseOnce: &sync.Once{},
	}
	restore := dump.SetInventoryDirectoryTestHook(func(rootName, path string) {
		if filepath.Join(rootName, filepath.FromSlash(path)) != target {
			return
		}
		hook.fireOnce.Do(func() {
			close(hook.reached)
			<-hook.release
		})
	})
	t.Cleanup(restore)
	t.Cleanup(hook.Release)
	return hook
}

func waitForInventoryTestHook(t *testing.T, hook inventoryTestHook, done <-chan error) {
	t.Helper()
	select {
	case <-hook.reached:
	case err := <-done:
		t.Fatalf("admission returned before test hook: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("admission did not reach the inventory test hook")
	}
}

func awaitInventoryAdmissionError(t *testing.T, done <-chan error, want error) {
	t.Helper()
	select {
	case err := <-done:
		if err == nil || !errors.Is(err, want) {
			t.Fatalf("admission error = %v, want prompt %v", err, want)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("admission did not return promptly with %v", want)
	}
}
