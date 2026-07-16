//go:build windows

package dump

import (
	"os"
	"testing"
)

// assertMode only verifies existence on Windows. Go synthesizes 0666/0777 mode
// bits there, and os.Chmod does not tighten a DACL. The native Windows tests
// separately prove the documented restricted-parent DACL inheritance path.
func assertMode(t *testing.T, path string, _ os.FileMode) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("os.Stat(%q) error = %v, want nil", path, err)
	}
}
