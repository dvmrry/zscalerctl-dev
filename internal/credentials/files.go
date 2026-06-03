package credentials

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/dvmrry/zscalerctl/internal/secret"
)

var ErrUnsafePermissions = errors.New("unsafe credential file permissions")
var ErrUnsupportedSecretFilePlatform = errors.New("credential files are not supported on this platform")

func ValidateOwnerOnlyFile(path string) error {
	return validateOwnerOnlyFile(path, runtime.GOOS)
}

func validateOwnerOnlyFile(path, goos string) error {
	if goos == "windows" {
		return fmt.Errorf("%w: %s; use inline environment variables until Windows ACL checks are supported", ErrUnsupportedSecretFilePlatform, path)
	}

	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat credential file: %w", err)
	}
	if info.IsDir() {
		return fmt.Errorf("%w: %s is a directory", ErrUnsafePermissions, path)
	}
	if info.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("%w: %s mode %03o", ErrUnsafePermissions, path, info.Mode().Perm())
	}
	return nil
}

func ReadOwnerOnlySecretFile(path string) (secret.Secret, error) {
	if err := ValidateOwnerOnlyFile(path); err != nil {
		return secret.Secret{}, err
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return secret.Secret{}, fmt.Errorf("read credential file: %w", err)
	}
	return secret.New(strings.TrimRight(string(body), "\r\n")), nil
}
