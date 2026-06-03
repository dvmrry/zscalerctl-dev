package dump

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureDirRejectsSymlink(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatalf("os.Mkdir(%q) error = %v, want nil", target, err)
	}
	link := filepath.Join(dir, "out")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("os.Symlink(%q, %q) error = %v; symlinks unavailable", target, link, err)
	}

	err := ensureDir(link)
	if !errors.Is(err, ErrUnsafePath) {
		t.Fatalf("ensureDir(symlink) error = %v, want ErrUnsafePath", err)
	}
}

func TestWriteFileExclusiveRefusesExistingPath(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "manifest.json")
	if err := os.WriteFile(path, []byte("existing"), filePerm); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", path, err)
	}

	err := writeFileExclusive(path, []byte("new"))
	if !errors.Is(err, ErrUnsafeOverwrite) {
		t.Fatalf("writeFileExclusive(existing) error = %v, want ErrUnsafeOverwrite", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v, want nil", path, err)
	}
	if string(got) != "existing" {
		t.Errorf("existing file content = %q, want unchanged", got)
	}
}
