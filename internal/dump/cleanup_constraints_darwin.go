//go:build darwin

package dump

import (
	"encoding/binary"
	"errors"
	"os"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

func validateRemovalConstraints(file *os.File, info os.FileInfo) error {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat == nil {
		return errors.New("darwin file flags are unavailable")
	}
	const blockingFlags = unix.UF_IMMUTABLE | unix.UF_APPEND | unix.SF_IMMUTABLE | unix.SF_APPEND
	if stat.Flags&blockingFlags != 0 {
		return errors.New("immutable or append-only file flags are present")
	}
	attributes := unix.Attrlist{
		Bitmapcount: unix.ATTR_BIT_MAP_COUNT,
		Commonattr:  unix.ATTR_CMN_EXTENDED_SECURITY,
	}
	buffer := make([]byte, 64<<10)
	_, _, errno := syscall.Syscall6(
		syscall.SYS_FGETATTRLIST,
		file.Fd(),
		uintptr(unsafe.Pointer(&attributes)),
		uintptr(unsafe.Pointer(&buffer[0])),
		uintptr(len(buffer)),
		uintptr(unix.FSOPT_REPORT_FULLSIZE),
		0,
	)
	if errno != 0 {
		return errno
	}
	if total := binary.LittleEndian.Uint32(buffer[:4]); total < 12 {
		return errors.New("malformed Darwin extended-security attributes")
	}
	if length := binary.LittleEndian.Uint32(buffer[8:12]); length != 0 {
		return errors.New("extended ACL is present")
	}
	return nil
}
