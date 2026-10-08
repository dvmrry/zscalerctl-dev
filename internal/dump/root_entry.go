package dump

import (
	"fmt"
	"os"
)

// openRootEntryAfterLstatTestHook lets package tests substitute an entry
// between OpenRootEntry's or openPathEntry's validation and open. It is nil
// outside tests.
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
	if err := verifyOpenedEntry(name, info, file); err != nil {
		_ = file.Close()
		return nil, err
	}
	return file, nil
}

// openPathEntry opens an absolute filesystem path with the same nonblocking
// flags as OpenRootEntry and verifies through fstat on the opened handle that
// its type and identity match want. It exists for cleanup paths that are not
// addressed relative to a single already-open root, such as an absolute
// quarantine or staging path. Callers must close the returned file.
func openPathEntry(path string, want os.FileInfo) (*os.File, error) {
	if want == nil {
		return nil, fmt.Errorf("%s is missing an expected identity", path)
	}
	if hook := openRootEntryAfterLstatTestHook; hook != nil {
		hook(path)
	}
	file, err := os.OpenFile(path, os.O_RDONLY|rootEntryOpenFlags(want.IsDir()), 0)
	if err != nil {
		return nil, err
	}
	if err := verifyOpenedEntry(path, want, file); err != nil {
		_ = file.Close()
		return nil, err
	}
	return file, nil
}

func verifyOpenedEntry(name string, want os.FileInfo, file *os.File) error {
	openedInfo, err := file.Stat()
	if err != nil {
		return err
	}
	if want.IsDir() != openedInfo.IsDir() ||
		(!openedInfo.IsDir() && !openedInfo.Mode().IsRegular()) ||
		!os.SameFile(want, openedInfo) {
		return fmt.Errorf("%s changed during open", name)
	}
	return nil
}
