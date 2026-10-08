//go:build unix

package dump

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// unsafeOpenTestDeadline bounds every substitution test. A regression that
// reintroduces a blocking open must fail this deadline instead of hanging the
// whole test binary.
const unsafeOpenTestDeadline = 5 * time.Second

// installLstatSwapHook installs a one-shot openRootEntryAfterLstatTestHook that
// replaces path with substitute() the first time the post-Lstat window for
// matchName is reached. matchName is the root-relative name passed to the hook;
// path is the filesystem location the substitution acts on. On the base
// implementation none of the covered sites call the hook, so the returned
// channel stays empty and the operation runs to completion instead of
// detecting the substitution.
func installLstatSwapHook(t *testing.T, matchName, path string, substitute func(string) error) <-chan error {
	t.Helper()
	hookErrs := make(chan error, 1)
	var once sync.Once
	openRootEntryAfterLstatTestHook = func(name string) {
		if name != matchName {
			return
		}
		once.Do(func() {
			hookErrs <- substitute(path)
		})
	}
	t.Cleanup(func() { openRootEntryAfterLstatTestHook = nil })
	return hookErrs
}

func renameAside(path string) error {
	return os.Rename(path, path+".validated")
}

func substituteWithFIFO(path string) error {
	if err := renameAside(path); err != nil {
		return err
	}
	return unix.Mkfifo(path, 0o600)
}

func substituteWithSymlink(path string) error {
	if err := renameAside(path); err != nil {
		return err
	}
	return os.Symlink("attacker-target", path)
}

// assertPromptUnsafePath waits for done and requires a prompt ErrUnsafePath.
// It also confirms the substitution hook actually fired, so a test cannot pass
// merely because the operation failed for an unrelated reason.
func assertPromptUnsafePath(t *testing.T, done <-chan error, hookErrs <-chan error) {
	t.Helper()
	select {
	case err := <-done:
		if err == nil || !errors.Is(err, ErrUnsafePath) {
			t.Fatalf("operation error = %v, want prompt ErrUnsafePath", err)
		}
	case <-time.After(unsafeOpenTestDeadline):
		t.Fatal("operation did not return promptly; a blocking open may have been reintroduced")
	}
	select {
	case err := <-hookErrs:
		if err != nil {
			t.Fatalf("substitution error = %v, want nil", err)
		}
	default:
		t.Fatal("operation returned before the substitution hook fired")
	}
}

func TestValidateCleanupEntryRejectsSubstitutionBeforeOpen(t *testing.T) {
	for _, tc := range []struct {
		name       string
		substitute func(string) error
	}{
		{name: "fifo", substitute: substituteWithFIFO},
		{name: "symlink", substitute: substituteWithSymlink},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "manifest.json")
			if err := os.WriteFile(path, []byte("{}"), 0o600); err != nil {
				t.Fatalf("os.WriteFile(%q) error = %v, want nil", path, err)
			}
			root, err := os.OpenRoot(dir)
			if err != nil {
				t.Fatalf("os.OpenRoot(%q) error = %v, want nil", dir, err)
			}
			defer root.Close()
			info, err := root.Lstat("manifest.json")
			if err != nil {
				t.Fatalf("root.Lstat(manifest.json) error = %v, want nil", err)
			}
			plan := artifactCleanupPlan{identities: map[string]os.FileInfo{"manifest.json": info}}

			hookErrs := installLstatSwapHook(t, "manifest.json", path, tc.substitute)
			done := make(chan error, 1)
			go func() {
				_, err := validateCleanupEntry(root, plan, "manifest.json", false)
				done <- err
			}()
			assertPromptUnsafePath(t, done, hookErrs)
		})
	}
}

func TestValidateAbsoluteCleanupEntryRejectsSubstitutionBeforeOpen(t *testing.T) {
	for _, tc := range []struct {
		name       string
		substitute func(string) error
	}{
		{name: "fifo", substitute: substituteWithFIFO},
		{name: "symlink", substitute: substituteWithSymlink},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "quarantine")
			if err := os.WriteFile(path, []byte("{}"), 0o600); err != nil {
				t.Fatalf("os.WriteFile(%q) error = %v, want nil", path, err)
			}
			info, err := os.Lstat(path)
			if err != nil {
				t.Fatalf("os.Lstat(%q) error = %v, want nil", path, err)
			}

			hookErrs := installLstatSwapHook(t, path, path, tc.substitute)
			done := make(chan error, 1)
			go func() {
				done <- validateAbsoluteCleanupEntry(path, info, false)
			}()
			assertPromptUnsafePath(t, done, hookErrs)
		})
	}
}

func TestClearDumpRootRejectsEntrySubstitutionBeforeRemoval(t *testing.T) {
	for _, tc := range []struct {
		name       string
		substitute func(string) error
	}{
		{name: "fifo", substitute: substituteWithFIFO},
		{name: "symlink", substitute: substituteWithSymlink},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			sub := filepath.Join(dir, "sub")
			if err := os.Mkdir(sub, 0o700); err != nil {
				t.Fatalf("os.Mkdir(%q) error = %v, want nil", sub, err)
			}
			root, err := os.OpenRoot(dir)
			if err != nil {
				t.Fatalf("os.OpenRoot(%q) error = %v, want nil", dir, err)
			}
			defer root.Close()

			hookErrs := installLstatSwapHook(t, "sub", sub, tc.substitute)
			done := make(chan error, 1)
			go func() {
				done <- clearDumpRootContext(t.Context(), root)
			}()
			assertPromptUnsafePath(t, done, hookErrs)
		})
	}
}

func TestClearDumpRootRootOpenReturnsPromptlyAfterPathSwap(t *testing.T) {
	// On Unix a swap cannot redirect Root.Open(".") because it resolves
	// through the already-open root descriptor. The nonblocking root open must
	// still be used and must return promptly rather than block on the hostile
	// replacement path.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "stale.txt"), []byte("x"), 0o600); err != nil {
		t.Fatalf("os.WriteFile(stale.txt) error = %v, want nil", err)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatalf("os.OpenRoot(%q) error = %v, want nil", dir, err)
	}
	defer root.Close()

	swap := installLstatSwapHook(t, ".", dir, func(string) error {
		if err := os.Rename(dir, dir+".validated"); err != nil {
			return err
		}
		return unix.Mkfifo(dir, 0o600)
	})
	done := make(chan error, 1)
	go func() {
		done <- clearDumpRootContext(t.Context(), root)
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("clearDumpRootContext(swapped root path) error = %v, want nil", err)
		}
	case <-time.After(unsafeOpenTestDeadline):
		t.Fatal("clearDumpRootContext() did not return promptly")
	}
	select {
	case err := <-swap:
		if err != nil {
			t.Fatalf("substitution error = %v, want nil", err)
		}
	default:
		t.Fatal("clearDumpRootContext() did not reach the root open hook")
	}
}
