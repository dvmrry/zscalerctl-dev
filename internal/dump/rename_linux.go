//go:build linux

package dump

import (
	"errors"
	"io/fs"

	"golang.org/x/sys/unix"
)

func renameNoReplace(oldPath, newPath string) error {
	if err := unix.Renameat2(
		unix.AT_FDCWD,
		oldPath,
		unix.AT_FDCWD,
		newPath,
		unix.RENAME_NOREPLACE,
	); err != nil {
		if errors.Is(err, unix.EEXIST) {
			return fs.ErrExist
		}
		return err
	}
	return nil
}

func exchangeDirectories(firstPath, secondPath string) (bool, error) {
	err := unix.Renameat2(
		unix.AT_FDCWD,
		firstPath,
		unix.AT_FDCWD,
		secondPath,
		unix.RENAME_EXCHANGE,
	)
	if errors.Is(err, unix.ENOSYS) ||
		errors.Is(err, unix.EINVAL) ||
		errors.Is(err, unix.EOPNOTSUPP) {
		return false, nil
	}
	return true, err
}

func closeRootBeforeRename() bool { return false }
