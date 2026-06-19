//go:build windows

package keyring

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"testing"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	procCredWriteW  = modAdvapi32.NewProc("CredWriteW")
	procCredDeleteW = modAdvapi32.NewProc("CredDeleteW")
)

const credPersistSession uint32 = 1

func TestWindowsGetLive(t *testing.T) {
	if os.Getenv("ZSCALERCTL_KEYRING_LIVE") == "" {
		t.Skip("set ZSCALERCTL_KEYRING_LIVE=1 to run against Windows Credential Manager")
	}
	const svc, key, target, want = "zscalerctl-livetest", "k", "zscalerctl-livetest/k", "hunter2"
	if out, err := exec.Command("cmdkey", "/generic:"+target, "/user:"+target, "/pass:"+want).CombinedOutput(); err != nil {
		t.Fatalf("seed Credential Manager item failed: %v: %s", err, out)
	}
	t.Cleanup(func() {
		_ = exec.Command("cmdkey", "/delete:"+target).Run()
	})
	got, err := New().Get(context.Background(), svc, key)
	if err != nil || got != want {
		t.Fatalf("Credential Manager Get(live) = %q, %v; want %q, nil", got, err, want)
	}
	if _, err := New().Get(context.Background(), svc, "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Credential Manager Get(missing) error = %v, want ErrNotFound", err)
	}
}

func TestWindowsGetLiveUnicodeCredWrite(t *testing.T) {
	if os.Getenv("ZSCALERCTL_KEYRING_LIVE") == "" {
		t.Skip("set ZSCALERCTL_KEYRING_LIVE=1 to run against Windows Credential Manager")
	}
	const svc, key, want = "zscalerctl-livetest-unicode", "unicode", "unicode-value-🔑"
	if err := writeTestCredential(svc, key, want); err != nil {
		t.Fatalf("write test credential: %v", err)
	}
	t.Cleanup(func() {
		_ = deleteTestCredential(svc, key)
	})
	got, err := New().Get(context.Background(), svc, key)
	if err != nil || got != want {
		t.Fatalf("Credential Manager Get(unicode live) = %q, %v; want %q, nil", got, err, want)
	}
}

func writeTestCredential(service, key, value string) error {
	target := service + "/" + key
	targetPtr, err := windowsUTF16PtrFromString(target)
	if err != nil {
		return err
	}
	userPtr, err := windowsUTF16PtrFromString(target)
	if err != nil {
		return err
	}
	blob := encodeUTF16LE(value)
	if len(blob) == 0 {
		return errors.New("empty credential blob")
	}
	cred := credentialW{
		Type:               credTypeGeneric,
		TargetName:         targetPtr,
		CredentialBlobSize: uint32(len(blob)),
		CredentialBlob:     &blob[0],
		Persist:            credPersistSession,
		UserName:           userPtr,
	}
	r1, _, lastErr := procCredWriteW.Call(uintptr(unsafe.Pointer(&cred)), 0)
	runtime.KeepAlive(targetPtr)
	runtime.KeepAlive(userPtr)
	runtime.KeepAlive(blob)
	if r1 == 0 {
		return fmt.Errorf("CredWriteW failed: %d", lastErr)
	}
	return nil
}

func deleteTestCredential(service, key string) error {
	targetPtr, err := windowsUTF16PtrFromString(service + "/" + key)
	if err != nil {
		return err
	}
	r1, _, lastErr := procCredDeleteW.Call(uintptr(unsafe.Pointer(targetPtr)), uintptr(credTypeGeneric), 0)
	runtime.KeepAlive(targetPtr)
	if r1 == 0 && !errors.Is(lastErr, windows.ERROR_NOT_FOUND) {
		return fmt.Errorf("CredDeleteW failed: %d", lastErr)
	}
	return nil
}

func windowsUTF16PtrFromString(s string) (*uint16, error) {
	return windows.UTF16PtrFromString(s)
}

func encodeUTF16LE(s string) []byte {
	u := utf16.Encode([]rune(s))
	u = append(u, 0)
	out := make([]byte, len(u)*2)
	for i, c := range u {
		out[2*i] = byte(c)
		out[2*i+1] = byte(c >> 8)
	}
	return out
}
