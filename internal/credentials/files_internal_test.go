package credentials

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestValidateOwnerOnlyFileRejectsWindowsSecretFiles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(path, []byte("fake-secret"), 0o600); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", path, err)
	}

	err := validateOwnerOnlyFile(path, "windows")
	if !errors.Is(err, ErrUnsupportedSecretFilePlatform) {
		t.Errorf("validateOwnerOnlyFile(%q, windows) error = %v, want ErrUnsupportedSecretFilePlatform", path, err)
	}
}
