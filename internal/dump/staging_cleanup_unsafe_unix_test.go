//go:build unix && zscalerctl_engine_testhooks

package dump

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestPreflightProcessStagingCleanupRejectsDirectorySubstitution(t *testing.T) {
	for _, window := range []struct {
		name       string
		entry      string
		beforeOpen bool
	}{
		{name: "root-before-open", entry: ".", beforeOpen: true},
		{name: "child-before-open", entry: "sub", beforeOpen: true},
		{name: "root-before-enumeration", entry: "."},
		{name: "child-before-enumeration", entry: "sub"},
	} {
		t.Run(window.name, func(t *testing.T) {
			for _, tc := range []struct {
				name       string
				substitute func(string) error
			}{
				{name: "fifo", substitute: substituteWithFIFO},
				{name: "symlink", substitute: substituteWithSymlink},
			} {
				t.Run(tc.name, func(t *testing.T) {
					dir := filepath.Join(t.TempDir(), "staging")
					if err := os.MkdirAll(filepath.Join(dir, "sub"), 0o700); err != nil {
						t.Fatalf("os.MkdirAll(staging/sub) error = %v, want nil", err)
					}
					path := filepath.Join(dir, window.entry)
					target := filepath.Join(filepath.Dir(path), "attacker-target")
					if err := os.Mkdir(target, 0o700); err != nil {
						t.Fatalf("os.Mkdir(%q) error = %v, want nil", target, err)
					}
					original, err := os.Lstat(dir)
					if err != nil {
						t.Fatalf("os.Lstat(%q) error = %v, want nil", dir, err)
					}

					hookErrs := make(chan error, 1)
					var once sync.Once
					swap := func() {
						once.Do(func() {
							hookErrs <- tc.substitute(path)
						})
					}
					if window.beforeOpen {
						var opens int
						openRootEntryAfterLstatTestHook = func(name string) {
							if name != path {
								return
							}
							opens++
							// The first open validates removal constraints.
							// The second binds the enumeration handle.
							if opens == 2 {
								swap()
							}
						}
						t.Cleanup(func() { openRootEntryAfterLstatTestHook = nil })
					} else {
						restore := SetInventoryDirectoryTestHook(func(rootName, name string) {
							if filepath.Join(rootName, filepath.FromSlash(name)) == path {
								swap()
							}
						})
						t.Cleanup(restore)
					}

					done := make(chan error, 1)
					go func() {
						done <- preflightProcessStagingCleanup(dir, original)
					}()
					assertPromptUnsafePath(t, done, hookErrs)
				})
			}
		})
	}
}
