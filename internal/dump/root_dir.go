package dump

import "os"

// openRootDirectoryTestHook lets package tests substitute the path between a
// caller's own checks and OpenRootDirectory's open. It is nil outside tests.
var openRootDirectoryTestHook func(path string)

// OpenRootDirectory opens the directory at path as an *os.Root without the
// blocking open that os.OpenRoot performs. os.OpenRoot opens its path with
// plain O_CLOEXEC, so a FIFO substituted for the directory after the caller's
// Lstat makes it wait for a writer forever. On Unix the directory is first
// opened O_NONBLOCK|O_DIRECTORY (and O_NOFOLLOW unless followSymlinks), which
// fails at once on a FIFO or any other non-directory; the Root is then opened
// through that verified handle and its identity checked against it. Callers
// must close the returned Root.
func OpenRootDirectory(path string, followSymlinks bool) (*os.Root, error) {
	if hook := openRootDirectoryTestHook; hook != nil {
		hook(path)
	}
	root, err := openRootDirectory(path, followSymlinks)
	if err != nil {
		return nil, err
	}
	registerRootPathForTestHooks(root, path)
	return root, nil
}
