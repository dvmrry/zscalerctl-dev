//go:build unix

package dump

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"runtime"
	"strconv"

	"golang.org/x/sys/unix"
)

func openRootDirectory(path string, followSymlinks bool) (*os.Root, error) {
	flags := os.O_RDONLY | unix.O_NONBLOCK | unix.O_DIRECTORY
	if !followSymlinks {
		flags |= unix.O_NOFOLLOW
	}
	dir, err := os.OpenFile(path, flags, 0)
	if err != nil {
		return nil, err
	}
	defer dir.Close()
	dirInfo, err := dir.Stat()
	if err != nil {
		return nil, err
	}
	if !dirInfo.IsDir() {
		return nil, &os.PathError{Op: "open", Path: path, Err: unix.ENOTDIR}
	}
	conn, err := dir.SyscallConn()
	if err != nil {
		return nil, err
	}
	var (
		root    *os.Root
		openErr error
	)
	if err := conn.Control(func(fd uintptr) {
		// The descriptor link names the directory already opened above, not
		// whatever path now holds, so this open cannot reach a FIFO.
		root, openErr = os.OpenRoot(descriptorPath(fd))
	}); err != nil {
		return nil, err
	}
	if openErr != nil {
		if !errors.Is(openErr, fs.ErrNotExist) {
			return nil, &os.PathError{Op: "open", Path: path, Err: unwrapPathError(openErr)}
		}
		return nil, fmt.Errorf("open %s: descriptor directory %s is unavailable: %w", path, descriptorDir(), openErr)
	}
	rootInfo, err := root.Stat(".")
	if err != nil || !os.SameFile(dirInfo, rootInfo) {
		_ = root.Close()
		if err == nil {
			err = fmt.Errorf("%s changed during open", path)
		}
		return nil, err
	}
	return root, nil
}

// descriptorDir is the directory of per-descriptor links: /proc/self/fd on
// Linux, /dev/fd elsewhere.
func descriptorDir() string {
	if runtime.GOOS == "linux" {
		return "/proc/self/fd"
	}
	return "/dev/fd"
}

func descriptorPath(fd uintptr) string {
	return descriptorDir() + "/" + strconv.FormatUint(uint64(fd), 10)
}

func unwrapPathError(err error) error {
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		return pathErr.Err
	}
	return err
}
