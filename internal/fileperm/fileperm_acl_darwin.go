//go:build darwin

package fileperm

import (
	"encoding/binary"
	"fmt"
	"os"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

const macOSACLBufferSize = 64 << 10

func validateOwnerOnlyACLPath(path string) error {
	pathPointer, err := syscall.BytePtrFromString(path)
	if err != nil {
		return fmt.Errorf("%w: inspect extended ACL: %v", ErrInsecurePermissions, err)
	}
	return validateMacOSExtendedSecurity(func(attributes *unix.Attrlist, buffer []byte) syscall.Errno {
		_, _, errno := syscall.Syscall6(
			syscall.SYS_GETATTRLIST,
			uintptr(unsafe.Pointer(pathPointer)),
			uintptr(unsafe.Pointer(attributes)),
			uintptr(unsafe.Pointer(&buffer[0])),
			uintptr(len(buffer)),
			uintptr(unix.FSOPT_NOFOLLOW|unix.FSOPT_REPORT_FULLSIZE),
			0,
		)
		return errno
	})
}

func validateOwnerOnlyACLFile(file *os.File) error {
	if file == nil {
		return fmt.Errorf("%w: inspect extended ACL on nil file", ErrInsecurePermissions)
	}
	return validateMacOSExtendedSecurity(func(attributes *unix.Attrlist, buffer []byte) syscall.Errno {
		_, _, errno := syscall.Syscall6(
			syscall.SYS_FGETATTRLIST,
			file.Fd(),
			uintptr(unsafe.Pointer(attributes)),
			uintptr(unsafe.Pointer(&buffer[0])),
			uintptr(len(buffer)),
			uintptr(unix.FSOPT_REPORT_FULLSIZE),
			0,
		)
		return errno
	})
}

func validateMacOSExtendedSecurity(
	getAttributes func(*unix.Attrlist, []byte) syscall.Errno,
) error {
	attributes := unix.Attrlist{
		Bitmapcount: unix.ATTR_BIT_MAP_COUNT,
		Commonattr:  unix.ATTR_CMN_EXTENDED_SECURITY,
	}
	buffer := make([]byte, macOSACLBufferSize)
	if errno := getAttributes(&attributes, buffer); errno != 0 {
		return fmt.Errorf("%w: inspect extended ACL: %v", ErrInsecurePermissions, errno)
	}
	if total := binary.LittleEndian.Uint32(buffer[:4]); total < 12 {
		return fmt.Errorf("%w: inspect extended ACL: malformed attribute response", ErrInsecurePermissions)
	}
	if length := binary.LittleEndian.Uint32(buffer[8:12]); length != 0 {
		return fmt.Errorf("%w: extended ACL is present", ErrInsecurePermissions)
	}
	return nil
}
