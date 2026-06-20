//go:build windows

package main

// main_windows_test.go — Windows smoke tests for the cmd/zscalerctl boundary.
//
// These tests run only on Windows (GOOS=windows). They call run() directly with
// the three most basic cases — version, --help, and an unknown command — and
// assert exit codes only. There is NO golden-file diffing because the POSIX path
// scrubbing in scrub() is not wired for Windows path separators.
//
// On POSIX platforms the golden surface tests (TestGoldenSurface) cover these
// cases with full output comparison; this file exists solely to ensure the
// Windows build path is exercised in CI and that the exit-code contract holds
// on Windows.

import (
	"bytes"
	"context"
	"testing"
)

func TestWindowsSmoke_Version(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"version"}, &stdout, &stderr, nil)
	if code != 0 {
		t.Errorf("run(version) on Windows exit code = %d, want 0\nstdout: %s\nstderr: %s",
			code, stdout.String(), stderr.String())
	}
}

func TestWindowsSmoke_Help(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"--help"}, &stdout, &stderr, nil)
	if code != 0 {
		t.Errorf("run(--help) on Windows exit code = %d, want 0\nstdout: %s\nstderr: %s",
			code, stdout.String(), stderr.String())
	}
}

func TestWindowsSmoke_UnknownCommand(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"bogus-windows-command"}, &stdout, &stderr, nil)
	if code != exitUsageError {
		t.Errorf("run(unknown-command) on Windows exit code = %d, want %d (exitUsageError)\nstdout: %s\nstderr: %s",
			code, exitUsageError, stdout.String(), stderr.String())
	}
}
