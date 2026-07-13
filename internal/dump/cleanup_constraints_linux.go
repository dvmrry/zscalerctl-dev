//go:build linux

package dump

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

const (
	linuxFSImmutable = 0x00000010
	linuxFSAppend    = 0x00000020
)

func validateRemovalConstraints(file *os.File, _ os.FileInfo) error {
	flags, err := unix.IoctlGetInt(int(file.Fd()), unix.FS_IOC_GETFLAGS)
	if err != nil {
		return err
	}
	if flags&(linuxFSImmutable|linuxFSAppend) != 0 {
		return errors.New("immutable or append-only inode flags are present")
	}
	return nil
}
