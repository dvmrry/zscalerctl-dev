//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package cli

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

func TestWriteOutputFileRejectsFIFO(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	destination := filepath.Join(dir, "destination")
	if err := unix.Mkfifo(destination, 0o600); err != nil {
		t.Fatalf("unix.Mkfifo(%q) error = %v, want nil", destination, err)
	}

	err := writeOutputFile(destination, []byte("must not replace FIFO"))
	if !errors.Is(err, ErrUsage) {
		t.Fatalf("writeOutputFile(FIFO) error = %v, want ErrUsage", err)
	}
	wantMessage := "write --output: " + destination + " is not a regular file"
	if err.Error() != wantMessage {
		t.Errorf("writeOutputFile(FIFO) error = %q, want %q", err, wantMessage)
	}
	if !strings.Contains(err.Error(), destination) {
		t.Errorf("writeOutputFile(FIFO) error = %q, want destination path", err)
	}
	assertNoOutputTempFiles(t, dir)
}
