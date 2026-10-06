//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
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

func TestWritersAcceptStickySharedParent(t *testing.T) {
	t.Parallel()

	const parent = "/tmp"
	info, err := os.Stat(parent)
	if err != nil {
		t.Skipf("stat shared parent %s: %v", parent, err)
	}
	if info.Mode()&os.ModeSticky == 0 || info.Mode().Perm()&0o002 == 0 {
		t.Skipf("%s is not a sticky world-writable directory", parent)
	}

	newPath := func() string {
		t.Helper()
		file, err := os.CreateTemp(parent, "zscalerctl-sticky-")
		if err != nil {
			t.Fatalf("os.CreateTemp(%q) error = %v, want nil", parent, err)
		}
		path := file.Name()
		t.Cleanup(func() {
			_ = os.Remove(path)
		})
		if err := file.Close(); err != nil {
			t.Fatalf("file.Close() error = %v, want nil", err)
		}
		if err := os.Remove(path); err != nil {
			t.Fatalf("os.Remove(%q) error = %v, want nil", path, err)
		}
		return path
	}

	outputPath := newPath()
	if err := writeOutputFile(outputPath, []byte("output")); err != nil {
		t.Fatalf("writeOutputFile under sticky shared parent error = %v, want nil", err)
	}
	if got, err := os.ReadFile(outputPath); err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v, want nil", outputPath, err)
	} else if string(got) != "output" {
		t.Errorf("output body = %q, want %q", got, "output")
	}

	configPath := newPath()
	var out, errOut bytes.Buffer
	app := New(&out, &errOut, nil)
	if err := app.Run(context.Background(), []string{"--config", configPath, "config", "init"}); err != nil {
		t.Fatalf("config init under sticky shared parent error = %v, want nil", err)
	}
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("os.Stat(%q) error = %v, want config file", configPath, err)
	}
}
