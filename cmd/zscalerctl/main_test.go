package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/cli"
	"github.com/dvmrry/zscalerctl/internal/output"
	"github.com/dvmrry/zscalerctl/internal/resources"
	"github.com/dvmrry/zscalerctl/internal/zscaler"
)

func TestExitCodeForError(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		err  error
		want int
	}{
		{"usage", cli.ErrUsage, exitUsageError},
		{"partial_dump", cli.ErrPartialDump, exitPartialDump},
		{"wrapped_partial_dump", fmt.Errorf("dump zia: %w", cli.ErrPartialDump), exitPartialDump},
		{"not_found", cli.ErrNotFound, exitNotFound},
		{"missing_credentials", zscaler.ErrMissingCredentials, exitCredentialError},
		{"invalid_resource_id", zscaler.ErrInvalidResourceID, exitUsageError},
		{"live_access_failed", zscaler.ErrLiveAccessFailed, exitLiveAccessFailure},
		{"unknown", errors.New("boom"), exitInternalError},
	}
	for _, tc := range cases {
		if got := exitCodeForError(tc.err); got != tc.want {
			t.Errorf("exitCodeForError(%s) = %d, want %d", tc.name, got, tc.want)
		}
	}
}

func TestErrorEnvelopeJSONIncludesResourceContext(t *testing.T) {
	t.Parallel()

	err := cli.ResourceNotFoundError{Product: resources.ProductZIA, Resource: "locations"}
	var buf bytes.Buffer
	writeError(&buf, output.FormatJSON, err)

	var env errorEnvelope
	if e := json.Unmarshal(buf.Bytes(), &env); e != nil {
		t.Fatalf("json.Unmarshal(%q) error = %v, want nil", buf.String(), e)
	}
	if env.Error.Kind != "not_found" {
		t.Errorf("error.kind = %q, want not_found", env.Error.Kind)
	}
	if env.Error.Product != "zia" {
		t.Errorf("error.product = %q, want zia", env.Error.Product)
	}
	if env.Error.Resource != "locations" {
		t.Errorf("error.resource = %q, want locations", env.Error.Resource)
	}
}

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

func TestRunHelpFlagReturnsSuccess(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"--help"}, &stdout, &stderr, nil)
	if code != 0 {
		t.Fatalf("run(--help) exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "usage: zscalerctl") {
		t.Errorf("run(--help) stdout = %q, want usage text", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Errorf("run(--help) stderr = %q, want empty", stderr.String())
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

func TestRunJSONUsageErrorEnvelope(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"--format", "json", "--timeout", "0s", "doctor"}, &stdout, &stderr, nil)
	if code != exitUsageError {
		t.Fatalf("run(json usage error) exit code = %d, want %d", code, exitUsageError)
	}
	if stdout.Len() != 0 {
		t.Errorf("run(json usage error) stdout = %q, want empty", stdout.String())
	}
	got := decodeErrorEnvelope(t, stderr.Bytes())
	if got.Error.Kind != "usage" || !strings.Contains(got.Error.Message, "timeout must be positive") {
		t.Errorf("run(json usage error) envelope = %#v, want usage timeout error", got)
	}
	if strings.Contains(stderr.String(), "zscalerctl:") {
		t.Errorf("run(json usage error) stderr = %q, want JSON without text prefix", stderr.String())
	}
}

func TestRunJSONCredentialErrorEnvelope(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"--format", "json", "zia", "locations", "list"}, &stdout, &stderr, nil)
	if code != exitCredentialError {
		t.Fatalf("run(json missing credentials) exit code = %d, want %d", code, exitCredentialError)
	}
	if stdout.Len() != 0 {
		t.Errorf("run(json missing credentials) stdout = %q, want empty", stdout.String())
	}
	got := decodeErrorEnvelope(t, stderr.Bytes())
	if got.Error.Kind != "missing_credentials" {
		t.Errorf("run(json missing credentials) kind = %q, want missing_credentials", got.Error.Kind)
	}
	if !strings.Contains(got.Error.Message, "missing zscaler API credentials") {
		t.Errorf("run(json missing credentials) message = %q, want missing credentials text", got.Error.Message)
	}
}

func TestRunJSONNotFoundErrorEnvelope(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"--format", "json", "zia", "not-a-resource", "list"}, &stdout, &stderr, nil)
	if code != exitNotFound {
		t.Fatalf("run(json resource not found) exit code = %d, want %d", code, exitNotFound)
	}
	if stdout.Len() != 0 {
		t.Errorf("run(json resource not found) stdout = %q, want empty", stdout.String())
	}
	got := decodeErrorEnvelope(t, stderr.Bytes())
	if got.Error.Kind != "not_found" || got.Error.Product != "zia" || got.Error.Resource != "not-a-resource" {
		t.Errorf("run(json resource not found) envelope = %#v, want zia/not-a-resource not_found", got)
	}
}

func TestExitCodeForPartialDump(t *testing.T) {
	t.Parallel()

	err := cli.PartialDumpError{Dir: "/tmp/dump", Errors: 2}
	if got := exitCodeForError(err); got != exitPartialDump {
		t.Fatalf("exitCodeForError(PartialDumpError) = %d, want %d", got, exitPartialDump)
	}
}

func TestRunVersionReturnsSuccess(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"--format", "table", "version"}, &stdout, &stderr, nil)
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

func decodeErrorEnvelope(t *testing.T, body []byte) errorEnvelope {
	t.Helper()
	var got errorEnvelope
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("json.Unmarshal(error envelope %q) error = %v, want nil", body, err)
	}
	return got
}
