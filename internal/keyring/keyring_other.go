//go:build !darwin && !linux && !windows

package keyring

import (
	"context"
	"fmt"
)

type unsupportedGetter struct{}

func newBackend() Getter {
	return unsupportedGetter{}
}

func (unsupportedGetter) Get(context.Context, string, string) (string, error) {
	return "", fmt.Errorf("keyring: not supported on this platform; use env:/file:/cmd:")
}
