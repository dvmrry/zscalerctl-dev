//go:build unix && (darwin || linux)

package dump

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureStagingParentRejectsAncestrySubstitutionBeforeOpen(t *testing.T) {
	for _, entry := range []string{"parent", "ancestor"} {
		t.Run(entry, func(t *testing.T) {
			for _, tc := range []struct {
				name       string
				substitute func(string) error
			}{
				{name: "fifo", substitute: substituteWithFIFO},
				{name: "symlink", substitute: substituteWithSymlink},
			} {
				t.Run(tc.name, func(t *testing.T) {
					dir, err := filepath.EvalSymlinks(t.TempDir())
					if err != nil {
						t.Fatalf("filepath.EvalSymlinks(temp directory) error = %v, want nil", err)
					}
					ancestor := filepath.Join(dir, "ancestor")
					parent := filepath.Join(ancestor, "parent")
					if err := os.MkdirAll(parent, 0o700); err != nil {
						t.Fatalf("os.MkdirAll(%q) error = %v, want nil", parent, err)
					}
					path := parent
					if entry == "ancestor" {
						path = ancestor
					}
					target := filepath.Join(filepath.Dir(path), "attacker-target")
					if err := os.Mkdir(target, 0o700); err != nil {
						t.Fatalf("os.Mkdir(%q) error = %v, want nil", target, err)
					}

					hookErrs := installLstatSwapHook(t, path, path, tc.substitute)
					done := make(chan error, 1)
					go func() {
						_, err := ensureStagingParentContext(context.Background(), parent)
						done <- err
					}()
					assertPromptUnsafePath(t, done, hookErrs)
				})
			}
		})
	}
}
