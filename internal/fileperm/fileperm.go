package fileperm

import (
	"errors"
	"os"
)

var ErrInsecurePermissions = errors.New("unsafe file permissions")

func Validate(path string) error {
	return validate(path)
}

func ValidateOpenFile(file *os.File) error {
	return validateOpenFile(file)
}

func OpenOwnerOnly(path string) (*os.File, error) {
	return openOwnerOnly(path)
}

// WriteOwnerOnly creates path and writes data with owner-only permissions, the
// write-side complement of OpenOwnerOnly. It fails if path already exists
// (O_EXCL); callers that want to overwrite must remove the file first so the
// overwrite decision stays with the caller. On success the resulting file
// passes the same validator OpenOwnerOnly enforces.
func WriteOwnerOnly(path string, data []byte) error {
	return writeOwnerOnly(path, data)
}
