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

// ReadOwnerOnlySecretFile opens the file, checks permissions on the open
// handle (eliminating the TOCTOU race between a separate stat and read), reads
// the contents, and returns the trimmed secret value.
//
// This is the only supported way to consume an owner-only credential file.
// Do not add a path-based "validate then read later" helper: checking
// permissions with a separate os.Stat reintroduces the stat-then-use race
// that checking the open handle exists to close.
func ReadOwnerOnlySecretFile(path string) (secret.Secret, error) {
	return readOwnerOnlySecretFile(path, runtime.GOOS)
}

// readOwnerOnlySecretFile takes goos as a parameter so the Windows
// fail-closed behavior can be exercised in tests on any platform.
func readOwnerOnlySecretFile(path, goos string) (secret.Secret, error) {
	if goos == "windows" {
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
	if info.IsDir() {
		return secret.Secret{}, fmt.Errorf("%w: %s is a directory", ErrUnsafePermissions, path)
	}
	if info.Mode().Perm()&0o077 != 0 {
		return secret.Secret{}, fmt.Errorf("%w: %s mode %03o", ErrUnsafePermissions, path, info.Mode().Perm())
	}

	body, err := io.ReadAll(f)
	if err != nil {
		return secret.Secret{}, fmt.Errorf("read credential file: %w", err)
	}
	return secret.New(strings.TrimRight(string(body), "\r\n")), nil
}
