//go:build !windows

package fileperm

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

func validate(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("stat file: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%w: %s is a symlink", ErrInsecurePermissions, path)
	}
	if info.IsDir() {
		return fmt.Errorf("%w: %s is a directory", ErrInsecurePermissions, path)
	}
	if info.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("%w: %s mode %03o", ErrInsecurePermissions, path, info.Mode().Perm())
	}
	return nil
}

func validateOpenFile(file *os.File) error {
	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("stat file: %w", err)
	}
	if info.IsDir() {
		return fmt.Errorf("%w: %s is a directory", ErrInsecurePermissions, file.Name())
	}
	if info.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("%w: %s mode %03o", ErrInsecurePermissions, file.Name(), info.Mode().Perm())
	}
	return nil
}

func writeOwnerOnly(path string, data []byte) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600) // #nosec G304 -- caller-supplied config path; created O_EXCL with 0600 and re-validated below.
	if err != nil {
		return err
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return fmt.Errorf("write file: %w", err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return fmt.Errorf("close file: %w", err)
	}
	// Re-assert 0600 explicitly in case a permissive umask widened the create
	// mode, then self-verify the file passes the read-side validator.
	if err := os.Chmod(path, 0o600); err != nil {
		_ = os.Remove(path)
		return fmt.Errorf("set owner-only permissions: %w", err)
	}
	verify, err := openOwnerOnly(path)
	if err != nil {
		_ = os.Remove(path)
		return fmt.Errorf("verify owner-only permissions: %w", err)
	}
	_ = verify.Close()
	return nil
}

func openOwnerOnly(path string) (*os.File, error) {
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		if errors.Is(err, unix.ELOOP) {
			return nil, fmt.Errorf("%w: %s is a symlink", ErrInsecurePermissions, path)
		}
		return nil, err
	}
	file := os.NewFile(uintptr(fd), path)
	if err := validateOpenFile(file); err != nil {
		_ = file.Close()
		return nil, err
	}
	return file, nil
}
