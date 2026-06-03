package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"
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

func TestRunVersionReturnsSuccess(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"version"}, &stdout, &stderr, nil)
	if code != 0 {
		t.Fatalf("run(version) exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "Version") {
		t.Errorf("run(version) stdout = %q, want version text", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Errorf("run(version) stderr = %q, want empty", stderr.String())
	}
}

func TestRunRecoversPanicWithoutTracebackOrRawSecret(t *testing.T) {
	const raw = "panic client_secret=raw-panic-canary"

	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"help"}, panicWriter{value: raw}, &stderr, nil)
	if code != 1 {
		t.Fatalf("run(help with panicking stdout) exit code = %d, want 1", code)
	}
	if stdout.Len() != 0 {
		t.Errorf("run(help with panicking stdout) captured stdout = %q, want empty", stdout.String())
	}
	got := stderr.String()
	if !strings.Contains(got, "zscalerctl: internal error:") {
		t.Errorf("run panic stderr = %q, want internal error prefix", got)
	}
	if strings.Contains(got, raw) || strings.Contains(got, "raw-panic-canary") {
		t.Errorf("run panic stderr = %q, want no raw panic value", got)
	}
	if strings.Contains(got, "goroutine ") || strings.Contains(got, ".go:") {
		t.Errorf("run panic stderr = %q, want no traceback", got)
	}
	if !strings.Contains(got, "<REDACTED:SECRET>") {
		t.Errorf("run panic stderr = %q, want redaction marker", got)
	}
}

func TestMuteProcessOutputSuppressesGlobalStdoutAndLogger(t *testing.T) {
	processOutputMu.Lock()
	defer processOutputMu.Unlock()

	previousStdout := os.Stdout
	previousLogWriter := log.Writer()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v, want nil", err)
	}
	defer reader.Close()
	os.Stdout = writer
	var logBuffer bytes.Buffer
	log.SetOutput(&logBuffer)
	defer func() {
		os.Stdout = previousStdout
		log.SetOutput(previousLogWriter)
		_ = writer.Close()
	}()

	restore, err := muteProcessOutput()
	if err != nil {
		t.Fatalf("muteProcessOutput() error = %v, want nil", err)
	}
	fmt.Fprint(os.Stdout, "stdout-canary")
	log.Print("log-canary")
	restore()

	fmt.Fprint(os.Stdout, "visible")
	if err := writer.Close(); err != nil {
		t.Fatalf("stdout pipe Close() error = %v, want nil", err)
	}
	body, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("io.ReadAll(stdout pipe) error = %v, want nil", err)
	}
	if strings.Contains(string(body), "stdout-canary") {
		t.Errorf("captured stdout = %q, want no muted canary", body)
	}
	if !strings.Contains(string(body), "visible") {
		t.Errorf("captured stdout = %q, want post-restore output", body)
	}
	if strings.Contains(logBuffer.String(), "log-canary") {
		t.Errorf("captured log = %q, want no muted canary", logBuffer.String())
	}
}

type panicWriter struct {
	value string
}

func (w panicWriter) Write([]byte) (int, error) {
	panic(w.value)
}
