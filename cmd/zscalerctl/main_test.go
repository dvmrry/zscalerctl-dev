package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestRunHelpReturnsSuccess(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"help"}, &stdout, &stderr, nil)
	if code != 0 {
		t.Fatalf("run(help) exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "zscalerctl") {
		t.Errorf("run(help) stdout = %q, want usage text", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Errorf("run(help) stderr = %q, want empty", stderr.String())
	}
}

func TestRunUsageErrorReturnsTwo(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"--timeout", "0s", "doctor"}, &stdout, &stderr, nil)
	if code != 2 {
		t.Fatalf("run(usage error) exit code = %d, want 2", code)
	}
	if stdout.Len() != 0 {
		t.Errorf("run(usage error) stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "timeout must be positive") {
		t.Errorf("run(usage error) stderr = %q, want usage error", stderr.String())
	}
}
