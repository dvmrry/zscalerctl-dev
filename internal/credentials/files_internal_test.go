package credentials

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestReadOwnerOnlySecretFileRejectsWindowsSecretFiles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(path, []byte("fake-secret"), 0o600); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", path, err)
	}

	_, err := readOwnerOnlySecretFile(path, "windows")
	if !errors.Is(err, ErrUnsupportedSecretFilePlatform) {
		t.Errorf("readOwnerOnlySecretFile(%q, windows) error = %v, want ErrUnsupportedSecretFilePlatform", path, err)
	}
}
