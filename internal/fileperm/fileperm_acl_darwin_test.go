//go:build darwin

package fileperm

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestOwnerOnlyValidationRejectsMacOSExtendedACL(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(path, []byte("secret\n"), 0o600); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", path, err)
	}
	if err := exec.Command("chmod", "+a", "everyone allow read", path).Run(); err != nil {
		t.Fatalf("chmod(+a everyone allow read, %q) error = %v, want nil", path, err)
	}
	if err := Validate(path); !errors.Is(err, ErrInsecurePermissions) {
		t.Errorf("Validate(%q with extended ACL) error = %v, want ErrInsecurePermissions", path, err)
	}
	if file, err := OpenOwnerOnly(path); !errors.Is(err, ErrInsecurePermissions) {
		if file != nil {
			_ = file.Close()
		}
		t.Errorf("OpenOwnerOnly(%q with extended ACL) error = %v, want ErrInsecurePermissions", path, err)
	}
}
