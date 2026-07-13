//go:build darwin

package dump

import (
	"errors"
	"io/fs"

	"golang.org/x/sys/unix"
)

func renameNoReplace(oldPath, newPath string) error {
	if err := unix.RenamexNp(oldPath, newPath, unix.RENAME_EXCL); err != nil {
		if errors.Is(err, unix.EEXIST) {
			return fs.ErrExist
		}
		return err
	}
	return nil
}

func exchangeDirectories(firstPath, secondPath string) (bool, error) {
	err := unix.RenamexNp(firstPath, secondPath, unix.RENAME_SWAP)
	if errors.Is(err, unix.ENOTSUP) || errors.Is(err, unix.EINVAL) {
		return false, nil
	}
	return true, err
}

func closeRootBeforeRename() bool { return false }
