//go:build !darwin && !linux && !windows

package dump

import "errors"

var errExclusiveRenameUnsupported = errors.New("exclusive rename is unsupported on this platform")

func renameNoReplace(string, string) error {
	return errExclusiveRenameUnsupported
}

func exchangeDirectories(string, string) (bool, error) {
	return false, nil
}

func closeRootBeforeRename() bool { return false }
