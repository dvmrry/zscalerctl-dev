//go:build windows && (amd64 || arm64)

package keyring

import (
	"testing"
	"unsafe"
)

// TestCredentialWLayout guards the unsafe CredReadW struct ABI. Both 64-bit
// Windows targets (amd64 and arm64) are LLP64 with 8-byte pointers and identical
// alignment, so the offset/size assertions hold on both. windows/386 (4-byte
// pointers) would need a separate layout and is intentionally excluded.
func TestCredentialWLayout(t *testing.T) {
	var c credentialW
	if off := unsafe.Offsetof(c.CredentialBlob); off != 40 {
		t.Fatalf("credentialW.CredentialBlob offset = %d, want 40", off)
	}
	if sz := unsafe.Sizeof(c); sz != 80 {
		t.Fatalf("unsafe.Sizeof(credentialW) = %d, want 80", sz)
	}
}
