// Package keyring provides cgo-free, read-only access to OS keychains.
package keyring

import (
	"context"
	"errors"
	"time"
	"unicode/utf16"
)

// ErrNotFound is returned by Getter.Get when no credential exists for service/key.
var ErrNotFound = errors.New("keyring item not found")

// ErrUnavailable marks a backend that cannot service the request right now.
//
// Errors wrapping ErrUnavailable carry value-free, actionable text by contract,
// so callers may surface those messages to users.
var ErrUnavailable = errors.New("keyring backend unavailable")

const defaultKeyringTimeout = 10 * time.Second

// Getter reads credentials from the OS keychain.
type Getter interface {
	Get(ctx context.Context, service, key string) (string, error)
}

// New returns the production Getter for the current platform.
func New() Getter {
	return newBackend()
}

// runnerFunc executes argv without a shell and reports stdout, stderr, exit
// code, and exec-level errors separately.
type runnerFunc func(ctx context.Context, timeout time.Duration, argv []string) (stdout, stderr string, exitCode int, err error)

// decodeUTF16LE decodes a little-endian UTF-16 byte blob and truncates at the
// first NUL code unit.
func decodeUTF16LE(b []byte) (string, error) {
	if len(b)%2 != 0 {
		return "", errors.New("keyring: UTF-16LE blob has odd byte count")
	}
	u := make([]uint16, len(b)/2)
	for i := range u {
		u[i] = uint16(b[2*i]) | uint16(b[2*i+1])<<8
	}
	for i, c := range u {
		if c == 0 {
			u = u[:i]
			break
		}
	}
	return string(utf16.Decode(u)), nil
}
