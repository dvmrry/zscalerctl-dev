//go:build unix

package dump

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestOpenRootEntryRejectsSubstitutionBetweenLstatAndOpen(t *testing.T) {
	t.Run("symlink", func(t *testing.T) {
		assertRootEntrySubstitutionRejected(t, func(path string) error {
			return os.Symlink("replacement.json", path)
		})
	})
	t.Run("fifo", func(t *testing.T) {
		assertRootEntrySubstitutionRejected(t, func(path string) error {
			return unix.Mkfifo(path, 0o600)
		})
	})
}

func assertRootEntrySubstitutionRejected(t *testing.T, substitute func(string) error) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.json")
	savedPath := filepath.Join(dir, "manifest-original.json")
	if err := os.WriteFile(path, []byte(`{}`), 0o600); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v", path, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "replacement.json"), []byte(`{"replacement":true}`), 0o600); err != nil {
		t.Fatalf("os.WriteFile(replacement) error = %v", err)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatalf("os.OpenRoot(%q) error = %v", dir, err)
	}
	defer root.Close()

	hookErrs := make(chan error, 1)
	openRootEntryAfterLstatTestHook = func(name string) {
		if name != "manifest.json" {
			return
		}
		if err := os.Rename(path, savedPath); err != nil {
			hookErrs <- err
			return
		}
		hookErrs <- substitute(path)
	}
	t.Cleanup(func() { openRootEntryAfterLstatTestHook = nil })

	done := make(chan error, 1)
	go func() {
		file, err := OpenRootEntry(root, "manifest.json")
		if file != nil {
			if closeErr := file.Close(); err == nil {
				err = closeErr
			}
		}
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("OpenRootEntry() succeeded after entry substitution")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("OpenRootEntry() did not reject the substitution promptly")
	}
	select {
	case err := <-hookErrs:
		if err != nil {
			t.Fatalf("substitute(%q) error = %v", path, err)
		}
	default:
		t.Fatal("OpenRootEntry() returned without reaching the test hook")
	}
}
