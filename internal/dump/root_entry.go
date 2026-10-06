package dump

import (
	"fmt"
	"os"
)

// openRootEntryAfterLstatTestHook lets package tests substitute an entry
// between OpenRootEntry's Lstat and open. It is nil outside tests.
var openRootEntryAfterLstatTestHook func(name string)

// OpenRootEntry opens a regular file or directory below root and verifies,
// through fstat on the opened handle, that its type and identity match the
// preceding Lstat. Callers must close the returned file.
//
// On Unix the open is O_NONBLOCK, so a FIFO or device substituted after Lstat
// cannot stall it, and directories add O_DIRECTORY. O_NOFOLLOW is not a
// symlink guarantee: Go 1.26 Root.OpenFile always sets it, but when openat
// fails with ELOOP or ENOTDIR it reads the link and re-resolves it, following
// any symlink whose target stays inside root (with the same flags). Only the
// post-open type and identity check rejects a symlink or special file swapped
// in between Lstat and open. Other platforms open without extra flags and rely
// on the same post-open check.
func OpenRootEntry(root *os.Root, name string) (*os.File, error) {
	info, err := root.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() && !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a regular file", name)
	}
	if hook := openRootEntryAfterLstatTestHook; hook != nil {
		hook(name)
	}
	file, err := root.OpenFile(name, os.O_RDONLY|rootEntryOpenFlags(info.IsDir()), 0)
	if err != nil {
		return nil, err
	}
	openedInfo, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	if info.IsDir() != openedInfo.IsDir() ||
		(!openedInfo.IsDir() && !openedInfo.Mode().IsRegular()) ||
		!os.SameFile(info, openedInfo) {
		_ = file.Close()
		return nil, fmt.Errorf("%s changed during open", name)
	}
	return file, nil
}
