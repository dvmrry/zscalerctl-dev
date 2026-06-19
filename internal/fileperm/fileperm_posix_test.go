//go:build !windows

package fileperm

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestValidatePOSIXOwnerOnly(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("profiles: {}\n"), 0o600); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", path, err)
	}
	if err := Validate(path); err != nil {
		t.Fatalf("Validate(0600) error = %v, want nil", err)
	}
	if err := os.Chmod(path, 0o640); err != nil {
		t.Fatalf("os.Chmod(%q, 0640) error = %v, want nil", path, err)
	}
	if err := Validate(path); !errors.Is(err, ErrInsecurePermissions) {
		t.Fatalf("Validate(0640) error = %v, want ErrInsecurePermissions", err)
	}
	if err := os.Chmod(path, 0o604); err != nil {
		t.Fatalf("os.Chmod(%q, 0604) error = %v, want nil", path, err)
	}
	if err := Validate(path); !errors.Is(err, ErrInsecurePermissions) {
		t.Fatalf("Validate(0604) error = %v, want ErrInsecurePermissions", err)
	}
}

func TestWriteOwnerOnlyPOSIXCreates0600AndValidates(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "config.yaml")
	data := []byte("profiles: {}\n")
	if err := WriteOwnerOnly(path, data); err != nil {
		t.Fatalf("WriteOwnerOnly(%q) error = %v, want nil", path, err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("os.Stat(%q) error = %v, want nil", path, err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("WriteOwnerOnly mode = %03o, want 600", info.Mode().Perm())
	}
	// Self-verify: the file it wrote must pass the read-side validator.
	file, err := OpenOwnerOnly(path)
	if err != nil {
		t.Fatalf("OpenOwnerOnly(written) error = %v, want nil", err)
	}
	_ = file.Close()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v, want nil", path, err)
	}
	if string(got) != string(data) {
		t.Fatalf("WriteOwnerOnly wrote %q, want %q", got, data)
	}
}

func TestWriteOwnerOnlyPOSIXRefusesExisting(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := WriteOwnerOnly(path, []byte("first\n")); err != nil {
		t.Fatalf("WriteOwnerOnly(first) error = %v, want nil", err)
	}
	if err := WriteOwnerOnly(path, []byte("second\n")); !errors.Is(err, os.ErrExist) {
		t.Fatalf("WriteOwnerOnly(existing) error = %v, want os.ErrExist", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v, want nil", path, err)
	}
	if string(got) != "first\n" {
		t.Fatalf("WriteOwnerOnly overwrote existing file: got %q", got)
	}
}

func TestValidatePOSIXRejectsSymlink(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	target := filepath.Join(dir, "target.yaml")
	if err := os.WriteFile(target, []byte("profiles: {}\n"), 0o600); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", target, err)
	}
	link := filepath.Join(dir, "config.yaml")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("os.Symlink(%q, %q) error = %v, want nil", target, link, err)
	}
	if err := Validate(link); !errors.Is(err, ErrInsecurePermissions) {
		t.Fatalf("Validate(symlink) error = %v, want ErrInsecurePermissions", err)
	}
	if file, err := OpenOwnerOnly(link); !errors.Is(err, ErrInsecurePermissions) {
		if err == nil {
			_ = file.Close()
		}
		t.Fatalf("OpenOwnerOnly(symlink) error = %v, want ErrInsecurePermissions", err)
	}
}
