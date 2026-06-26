package credentials_test

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/credentials"
)

func TestReadOwnerOnlySecretFileRejectsGroupReadableFile(t *testing.T) {
	t.Parallel()
	skipOwnerOnlySecretFileTestOnWindows(t)

	path := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(path, []byte("fake-secret"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", path, err)
	}

	_, err := credentials.ReadOwnerOnlySecretFile(path)
	if !errors.Is(err, credentials.ErrUnsafePermissions) {
		t.Errorf("ReadOwnerOnlySecretFile(%q) error = %v, want ErrUnsafePermissions", path, err)
	}
}

func TestReadOwnerOnlySecretFile(t *testing.T) {
	t.Parallel()
	skipOwnerOnlySecretFileTestOnWindows(t)

	path := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(path, []byte("fake-secret\n"), 0o600); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", path, err)
	}

	got, err := credentials.ReadOwnerOnlySecretFile(path)
	if err != nil {
		t.Fatalf("ReadOwnerOnlySecretFile(%q) error = %v, want nil", path, err)
	}
	if got.Reveal() != "fake-secret" {
		t.Errorf("ReadOwnerOnlySecretFile(%q).Reveal() = %q, want %q", path, got.Reveal(), "fake-secret")
	}
}

func TestReadOwnerOnlySecretFileTrimsTrailingCRLF(t *testing.T) {
	t.Parallel()
	skipOwnerOnlySecretFileTestOnWindows(t)

	path := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(path, []byte("fake-secret\r\n\n"), 0o600); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", path, err)
	}

	got, err := credentials.ReadOwnerOnlySecretFile(path)
	if err != nil {
		t.Fatalf("ReadOwnerOnlySecretFile(%q) error = %v, want nil", path, err)
	}
	if got.Reveal() != "fake-secret" {
		t.Errorf("ReadOwnerOnlySecretFile(%q).Reveal() = %q, want %q", path, got.Reveal(), "fake-secret")
	}
}

func TestReadOwnerOnlySecretFileRejectsOversizedFile(t *testing.T) {
	t.Parallel()
	skipOwnerOnlySecretFileTestOnWindows(t)

	path := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(path, []byte(strings.Repeat("x", 64*1024+1)), 0o600); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", path, err)
	}

	_, err := credentials.ReadOwnerOnlySecretFile(path)
	if err == nil {
		t.Fatalf("ReadOwnerOnlySecretFile(%q) error = nil, want oversized credential file error", path)
	}
	if !strings.Contains(err.Error(), "credential file exceeds 65536 byte limit") {
		t.Fatalf("ReadOwnerOnlySecretFile(%q) error = %v, want oversized credential file error", path, err)
	}
}

func skipOwnerOnlySecretFileTestOnWindows(t *testing.T) {
	t.Helper()

	if runtime.GOOS == "windows" {
		t.Skip("POSIX mode-bit test; Windows DACL coverage lives in files_windows_test.go")
	}
}
