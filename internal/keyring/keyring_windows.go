//go:build windows && (amd64 || arm64)

package keyring

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

type credentialW struct {
	Flags              uint32
	Type               uint32
	TargetName         *uint16
	Comment            *uint16
	LastWritten        windows.Filetime
	CredentialBlobSize uint32
	_                  [4]byte
	CredentialBlob     *byte
	Persist            uint32
	AttributeCount     uint32
	Attributes         uintptr
	TargetAlias        *uint16
	UserName           *uint16
}

const credTypeGeneric uint32 = 1

var (
	modAdvapi32   = windows.NewLazySystemDLL("advapi32.dll")
	procCredReadW = modAdvapi32.NewProc("CredReadW")
	procCredFree  = modAdvapi32.NewProc("CredFree")
)

type windowsGetter struct{}

func newBackend() Getter {
	return windowsGetter{}
}

func (windowsGetter) Get(ctx context.Context, service, key string) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}

	targetPtr, err := windows.UTF16PtrFromString(service + "/" + key)
	if err != nil {
		return "", fmt.Errorf("keyring: invalid target name for service %q", service)
	}

	var cred *credentialW
	r1, _, lastErr := procCredReadW.Call(
		uintptr(unsafe.Pointer(targetPtr)),
		uintptr(credTypeGeneric),
		0,
		uintptr(unsafe.Pointer(&cred)),
	)
	// LazyProc.Call is not the compiler-special-cased syscall.Syscall, so the
	// input buffer is otherwise dead after the uintptr conversion and could be
	// collected mid-syscall. Keep it alive until CredReadW returns.
	runtime.KeepAlive(targetPtr)
	if r1 == 0 {
		if errors.Is(lastErr, windows.ERROR_NOT_FOUND) {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("keyring: read failed for service %q (windows errno %d)", service, lastErr)
	}
	defer func() {
		procCredFree.Call(uintptr(unsafe.Pointer(cred))) //nolint:errcheck // CredFree has no return value.
	}()

	blobSize := cred.CredentialBlobSize
	if blobSize == 0 {
		return "", fmt.Errorf("keyring: credential for service %q has empty blob", service)
	}
	blob := make([]byte, blobSize)
	copy(blob, unsafe.Slice(cred.CredentialBlob, blobSize))
	value, err := decodeUTF16LE(blob)
	if err != nil {
		return "", fmt.Errorf("keyring: credential for service %q is corrupted", service)
	}
	if value == "" {
		return "", fmt.Errorf("keyring: credential for service %q is empty", service)
	}
	return value, nil
}
