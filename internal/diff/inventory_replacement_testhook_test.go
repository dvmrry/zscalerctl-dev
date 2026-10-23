//go:build unix && zscalerctl_engine_testhooks

package diff

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/dump"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

func TestInventoryRejectsManifestReplacementAfterEnumeration(t *testing.T) {
	spec := testKeyedSpec()
	catalog := resources.ResourceCatalog{spec}
	for _, admission := range []struct {
		name string
		run  func(context.Context, string, string) error
		want error
	}{
		{
			name: "ValidateArtifactRootContext",
			run: func(ctx context.Context, oldDir, _ string) error {
				root, err := os.OpenRoot(oldDir)
				if err != nil {
					return err
				}
				defer root.Close()
				_, err = dump.ValidateArtifactRootContext(ctx, root)
				return err
			},
			want: dump.ErrInvalidArtifact,
		},
		{
			name: "CompareContext",
			run: func(ctx context.Context, oldDir, newDir string) error {
				_, err := CompareContext(ctx, oldDir, newDir, Options{Catalog: catalog}, nil)
				return err
			},
			want: ErrInvalidDump,
		},
		{
			name: "LoadCollection",
			run: func(ctx context.Context, oldDir, _ string) error {
				_, err := LoadCollection(ctx, oldDir, catalog)
				return err
			},
			want: ErrInvalidDump,
		},
	} {
		t.Run(admission.name, func(t *testing.T) {
			oldDir := writeTestDump(t, catalog, dumpFixture{
				entries: []dumpEntryFixture{{spec: spec, payload: `[{"id":"1","name":"HQ"}]`}},
			})
			newDir := writeTestDump(t, catalog, dumpFixture{
				entries: []dumpEntryFixture{{spec: spec, payload: `[{"id":"1","name":"HQ"}]`}},
			})
			manifestPath := filepath.Join(oldDir, "manifest.json")
			replacementPath := filepath.Join(t.TempDir(), "manifest.json")
			if err := os.WriteFile(replacementPath, []byte("not-json\n"), 0o600); err != nil {
				t.Fatalf("os.WriteFile(%q) error = %v", replacementPath, err)
			}
			originalInfo, err := os.Lstat(manifestPath)
			if err != nil {
				t.Fatalf("os.Lstat(%q) error = %v", manifestPath, err)
			}
			replacementInfo, err := os.Lstat(replacementPath)
			if err != nil {
				t.Fatalf("os.Lstat(%q) error = %v", replacementPath, err)
			}
			if !originalInfo.Mode().IsRegular() || !replacementInfo.Mode().IsRegular() ||
				os.SameFile(originalInfo, replacementInfo) {
				t.Fatal("manifest replacement must be a different regular file")
			}

			hook := inventoryTestHook{
				reached:     make(chan struct{}),
				release:     make(chan struct{}),
				fireOnce:    &sync.Once{},
				releaseOnce: &sync.Once{},
			}
			restore := dump.SetInventoryAfterReadDirTestHook(func(rootName, path string) {
				if filepath.Join(rootName, filepath.FromSlash(path)) != oldDir {
					return
				}
				hook.fireOnce.Do(func() {
					close(hook.reached)
					<-hook.release
				})
			})
			t.Cleanup(restore)
			t.Cleanup(hook.Release)

			done := make(chan error, 1)
			go func() {
				done <- admission.run(context.Background(), oldDir, newDir)
			}()
			waitForInventoryTestHook(t, hook, done)

			if err := os.Rename(replacementPath, manifestPath); err != nil {
				hook.Release()
				t.Fatalf("os.Rename(%q, %q) error = %v", replacementPath, manifestPath, err)
			}
			hook.Release()
			awaitInventoryAdmissionError(t, done, admission.want)
		})
	}
}
