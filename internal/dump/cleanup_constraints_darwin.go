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
		uintptr(unsafe.Pointer(&attributes)), // #nosec G103 -- Darwin syscall ABI requires pointers to the initialized attrlist.
		uintptr(unsafe.Pointer(&buffer[0])),  // #nosec G103 -- fixed-size live buffer is retained for the synchronous syscall.
		uintptr(len(buffer)),
		uintptr(unix.FSOPT_REPORT_FULLSIZE),
		0,
	)
	if errno != 0 {
		return nil, errno
	}
	reportedTotal := int64(binary.LittleEndian.Uint32(buffer[:4]))
	if reportedTotal < 12 || reportedTotal > int64(len(buffer)) {
		return nil, errors.New("malformed Darwin extended-security attributes")
	}
	reportedLength := int64(binary.LittleEndian.Uint32(buffer[8:12]))
	if reportedLength == 0 {
		return nil, nil
	}
	rawOffset := binary.LittleEndian.Uint32(buffer[4:8])
	offset := int64(rawOffset)
	if rawOffset&(1<<31) != 0 {
		offset -= 1 << 32
	}
	start64 := int64(4) + offset
	if start64 < 12 || start64 > reportedTotal || reportedLength > reportedTotal-start64 {
		return nil, errors.New("malformed Darwin extended-security reference")
	}
	start := int(start64)         // #nosec G115 -- bounds above constrain start64 to the 64 KiB in-memory buffer.
	length := int(reportedLength) // #nosec G115 -- bounds above constrain the length to the 64 KiB in-memory buffer.
	security := make([]byte, length)
	copy(security, buffer[start:start+length])
	return security, nil
}
