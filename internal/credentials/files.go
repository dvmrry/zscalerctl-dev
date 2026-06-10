package credentials

import (
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"

	"github.com/dvmrry/zscalerctl/internal/secret"
)

var ErrUnsafePermissions = errors.New("unsafe credential file permissions")
var ErrUnsupportedSecretFilePlatform = errors.New("credential files are not supported on this platform")

// checkFileInfo applies owner-only permission checks to a FileInfo already in
// hand (either from os.Stat or from File.Stat on an open handle).
func checkFileInfo(path string, info os.FileInfo) error {
	if info.IsDir() {
		return fmt.Errorf("%w: %s is a directory", ErrUnsafePermissions, path)
	}
	if info.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("%w: %s mode %03o", ErrUnsafePermissions, path, info.Mode().Perm())
	}
	return nil
}

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
	return checkFileInfo(path, info)
}

// ReadOwnerOnlySecretFile opens the file, checks permissions on the open
// handle (eliminating the TOCTOU race between a separate stat and read), reads
// the contents, and returns the trimmed secret value.
func ReadOwnerOnlySecretFile(path string) (secret.Secret, error) {
	if runtime.GOOS == "windows" {
		return secret.Secret{}, fmt.Errorf("%w: %s; use inline environment variables until Windows ACL checks are supported", ErrUnsupportedSecretFilePlatform, path)
	}

	f, err := os.Open(path) // #nosec G304 -- permission check is applied to the open handle via f.Stat() below before any data is consumed
	if err != nil {
		return secret.Secret{}, fmt.Errorf("open credential file: %w", err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return secret.Secret{}, fmt.Errorf("stat credential file: %w", err)
	}
	if err := checkFileInfo(path, info); err != nil {
		return secret.Secret{}, err
	}

	body, err := io.ReadAll(f)
	if err != nil {
		return secret.Secret{}, fmt.Errorf("read credential file: %w", err)
	}
	return secret.New(strings.TrimRight(string(body), "\r\n")), nil
}
