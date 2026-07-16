//go:build windows

package dump

import (
	"errors"
	"io/fs"

	"golang.org/x/sys/windows"
)

func renameNoReplace(oldPath, newPath string) error {
	from, err := windows.UTF16PtrFromString(oldPath)
	if err != nil {
		return err
	}
	to, err := windows.UTF16PtrFromString(newPath)
	if err != nil {
		return err
	}
	if err := windows.MoveFile(from, to); err != nil {
		if errors.Is(err, windows.ERROR_ALREADY_EXISTS) ||
			errors.Is(err, windows.ERROR_FILE_EXISTS) {
			return fs.ErrExist
		}
		return err
	}
	return nil
}

func exchangeDirectories(string, string) (bool, error) {
	return false, nil
}

// Go's top-level os.OpenRoot uses syscall.Open, whose Windows share mode omits
// FILE_SHARE_DELETE. Close that validation handle before MoveFile; existing
// destinations still fail closed because Windows has no directory exchange.
func closeRootBeforeRename() bool { return true }
