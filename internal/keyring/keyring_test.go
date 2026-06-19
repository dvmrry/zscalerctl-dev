package keyring

import "testing"

func TestDecodeUTF16LE(t *testing.T) {
	got, err := decodeUTF16LE([]byte{0x68, 0x00, 0x69, 0x00})
	if err != nil || got != "hi" {
		t.Fatalf("decodeUTF16LE(hi) = %q, %v; want hi, nil", got, err)
	}
}

func TestDecodeUTF16LEOddBytes(t *testing.T) {
	if _, err := decodeUTF16LE([]byte{0x68}); err == nil {
		t.Fatal("decodeUTF16LE(odd bytes) error = nil, want error")
	}
}

func TestDecodeUTF16LEAllNul(t *testing.T) {
	got, err := decodeUTF16LE([]byte{0x00, 0x00, 0x00, 0x00})
	if err != nil || got != "" {
		t.Fatalf("decodeUTF16LE(all NUL) = %q, %v; want empty, nil", got, err)
	}
}

func TestDecodeUTF16LEEmbeddedNul(t *testing.T) {
	got, err := decodeUTF16LE([]byte{0x68, 0x00, 0x69, 0x00, 0x00, 0x00, 0x78, 0x00})
	if err != nil || got != "hi" {
		t.Fatalf("decodeUTF16LE(embedded NUL) = %q, %v; want hi, nil", got, err)
	}
}
