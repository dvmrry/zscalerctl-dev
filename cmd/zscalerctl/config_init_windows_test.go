//go:build windows

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/cli"
	"golang.org/x/sys/windows"
)

func TestWindowsConfigInitExistingWithExclusiveHandle(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	destination := filepath.Join(dir, "config.yaml")
	const sentinel = "existing config must remain unchanged\n"
	if err := os.WriteFile(destination, []byte(sentinel), 0o600); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", destination, err)
	}
	path, err := windows.UTF16PtrFromString(destination)
	if err != nil {
		t.Fatalf("UTF16PtrFromString(%q) error = %v, want nil", destination, err)
	}
	held, err := windows.CreateFile(
		path, windows.GENERIC_READ|windows.GENERIC_WRITE, 0, nil,
		windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0,
	)
	if err != nil {
		t.Fatalf("CreateFile(exclusive config) error = %v, want nil", err)
	}
	t.Cleanup(func() {
		if held != windows.InvalidHandle {
			if err := windows.CloseHandle(held); err != nil {
				t.Errorf("CloseHandle(exclusive config) error = %v, want nil", err)
			}
		}
	})

	info, err := os.Lstat(destination)
	if err != nil {
		t.Fatalf("os.Lstat(%q) error = %v, want attribute inspection to succeed", destination, err)
	}
	if !info.Mode().IsRegular() {
		t.Fatalf("os.Lstat(%q).Mode() = %v, want regular file", destination, info.Mode())
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatalf("os.OpenRoot(%q) error = %v, want nil", dir, err)
	}
	defer root.Close()
	if _, err := root.Lstat(filepath.Base(destination)); !errors.Is(err, windows.ERROR_SHARING_VIOLATION) {
		t.Fatalf("root.Lstat(destination) error = %v, want ERROR_SHARING_VIOLATION", err)
	}

	args := []string{"--format", "json", "--config", destination, "config", "init"}
	var out, errOut bytes.Buffer
	app := cli.New(&out, &errOut, nil)
	err = app.Run(context.Background(), args)
	if !errors.Is(err, cli.ErrUsage) {
		t.Fatalf("config init (exclusive existing config) error = %v, want ErrUsage", err)
	}
	wantMessage := "config already exists at " + destination + "; pass --force to overwrite"
	if err.Error() != wantMessage {
		t.Errorf("config init error = %q, want %q", err, wantMessage)
	}
	if out.Len() != 0 || errOut.Len() != 0 {
		t.Errorf("App.Run output = %q/%q, want empty stdout/stderr", out.String(), errOut.String())
	}

	var stdout, stderr bytes.Buffer
	code := run(context.Background(), args, &stdout, &stderr, nil)
	if code != 2 {
		t.Errorf("run(config init) exit code = %d, want 2", code)
	}
	if stdout.Len() != 0 {
		t.Errorf("run(config init) stdout = %q, want empty", stdout.String())
	}
	var envelope errorEnvelope
	if err := json.Unmarshal(stderr.Bytes(), &envelope); err != nil {
		t.Fatalf("json.Unmarshal(%q) error = %v, want nil", stderr.String(), err)
	}
	if envelope.Error.Kind != "usage" {
		t.Errorf("error.kind = %q, want usage", envelope.Error.Kind)
	}
	if !strings.HasPrefix(envelope.Error.Message, "config already exists at ") ||
		!strings.HasSuffix(envelope.Error.Message, "; pass --force to overwrite") {
		t.Errorf("error.message = %q, want existing-config guidance", envelope.Error.Message)
	}

	if err := windows.CloseHandle(held); err != nil {
		t.Fatalf("CloseHandle(exclusive config) error = %v, want nil", err)
	}
	held = windows.InvalidHandle
	if got, err := os.ReadFile(destination); err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v, want nil", destination, err)
	} else if string(got) != sentinel {
		t.Errorf("existing config body = %q, want %q", got, sentinel)
	}
}
