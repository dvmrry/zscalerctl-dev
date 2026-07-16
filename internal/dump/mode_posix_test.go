//go:build !windows

package dump

import (
	"os"
	"testing"
)

func assertMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("os.Stat(%q) error = %v, want nil", path, err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Errorf("os.Stat(%q).Mode().Perm() = %#o, want %#o", path, got, want)
	}
}
