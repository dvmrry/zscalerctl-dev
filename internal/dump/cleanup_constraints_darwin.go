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
	security, err := readDarwinExtendedSecurity(file)
	if err != nil {
		return err
	}
	if len(security) != 0 {
		return errors.New("extended ACL is present")
	}
	return nil
}

func readDarwinExtendedSecurity(file *os.File) ([]byte, error) {
	if file == nil {
		return nil, errors.New("nil file handle")
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
		return nil, errno
	}
	total := int(binary.LittleEndian.Uint32(buffer[:4]))
	if total < 12 || total > len(buffer) {
		return nil, errors.New("malformed Darwin extended-security attributes")
	}
	length := int(binary.LittleEndian.Uint32(buffer[8:12]))
	if length == 0 {
		return nil, nil
	}
	offset := int(int32(binary.LittleEndian.Uint32(buffer[4:8])))
	start := 4 + offset
	if start < 12 || start > total || length > total-start {
		return nil, errors.New("malformed Darwin extended-security reference")
	}
	security := make([]byte, length)
	copy(security, buffer[start:start+length])
	return security, nil
}
