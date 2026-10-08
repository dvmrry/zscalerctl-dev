//go:build !unix

package dump

import (
	"fmt"
	"os"
)

// Without Unix FIFOs in the filesystem namespace, os.OpenRoot cannot block on
// a substituted special file; check the type the caller expects.
func openRootDirectory(path string, followSymlinks bool) (*os.Root, error) {
	var (
		info os.FileInfo
		err  error
	)
	if followSymlinks {
		info, err = os.Stat(path)
	} else {
		info, err = os.Lstat(path)
	}
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", path)
	}
	root, err := os.OpenRoot(path)
	if err != nil {
		return nil, err
	}
	rootInfo, err := root.Stat(".")
	if err != nil || !os.SameFile(info, rootInfo) {
		_ = root.Close()
		if err == nil {
			err = fmt.Errorf("%s changed during open", path)
		}
		return nil, err
	}
	return root, nil
}
