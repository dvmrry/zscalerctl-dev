package redact_test

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/redact"
)

// TestRedactingWriterRedactsAcrossWrites verifies that a secret split across
// two Write calls is still caught — the writer must full-buffer, not redact
// per-write.
func TestRedactingWriterRedactsAcrossWrites(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	w := redact.NewWriter(&buf, redact.ModeStandard)
	_, err := io.WriteString(w, "token=AKIA") // pattern split across writes
	if err != nil {
		t.Fatal(err)
	}
	_, err = io.WriteString(w, "abcdef1234567890XYZ\n")
	if err != nil {
		t.Fatal(err)
	}
	w.Close() // flush
	if strings.Contains(buf.String(), "AKIAabcdef1234567890XYZ") {
		t.Fatalf("secret leaked across write boundary: %q", buf.String())
	}
}

// TestRedactingWriterPassesThroughNonSecret verifies that output containing no
// secrets is written unchanged.
func TestRedactingWriterPassesThroughNonSecret(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	w := redact.NewWriter(&buf, redact.ModeStandard)
	want := "Usage: zscalerctl [flags]\n"
	io.WriteString(w, want)
	w.Close()
	if buf.String() != want {
		t.Errorf("non-secret line changed: got %q, want %q", buf.String(), want)
	}
}

// TestRedactingWriterRedactsMultiLinePEM verifies that a multi-line PEM block
// (which spans many writes) is fully redacted as a unit.
func TestRedactingWriterRedactsMultiLinePEM(t *testing.T) {
	t.Parallel()

	// The body carries the repo's "key-material" canary marker so the gitleaks
	// test-fixture allowlist (.gitleaks.toml) recognizes it as a known fake, not
	// a real leak. The redactor keys on the BEGIN/END markers, so the body
	// content does not affect what this test verifies.
	pem := "-----BEGIN PRIVATE KEY-----\nMIIEvgIBADANBgkqhkiG9w0BAQEFAASC\nfake-key-material-canary-not-a-real-key\n-----END PRIVATE KEY-----\n"

	var buf bytes.Buffer
	w := redact.NewWriter(&buf, redact.ModeStandard)
	// Write line by line to confirm full-buffer catches the block.
	for _, line := range strings.SplitAfter(pem, "\n") {
		_, err := io.WriteString(w, line)
		if err != nil {
			t.Fatal(err)
		}
	}
	w.Close()
	out := buf.String()
	if strings.Contains(out, "BEGIN PRIVATE KEY") {
		t.Errorf("PEM block not redacted: %q", out)
	}
	if !strings.Contains(out, "<REDACTED:PRIVATE_KEY>") {
		t.Errorf("expected REDACTED:PRIVATE_KEY marker, got %q", out)
	}
}

// TestRedactingWriterFlushesOnCloseWithoutNewline verifies that a final partial
// write (no trailing newline) is still flushed when Close is called.
func TestRedactingWriterFlushesOnCloseWithoutNewline(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	w := redact.NewWriter(&buf, redact.ModeStandard)
	io.WriteString(w, "no newline at end")
	w.Close()
	if buf.String() != "no newline at end" {
		t.Errorf("close did not flush: got %q", buf.String())
	}
}

// errWriter is a stub io.Writer whose Write always returns an error.
type errWriter struct{}

func (errWriter) Write([]byte) (int, error) { return 0, errors.New("boom") }

// TestRedactingWriterUnderlyingWriterErrorPropagatesOnClose verifies that an
// error from the wrapped writer surfaces through Close, so a broken downstream
// writer (e.g. Cobra's) is not silently swallowed.
func TestRedactingWriterUnderlyingWriterErrorPropagatesOnClose(t *testing.T) {
	t.Parallel()

	w := redact.NewWriter(errWriter{}, redact.ModeStandard)
	io.WriteString(w, "not a secret")
	if err := w.Close(); err == nil {
		t.Fatal("expected Close to return an error from the underlying writer, got nil")
	}
}

// TestRedactingWriterEmptyInputClose verifies that Close on a writer that has
// received no writes neither panics nor returns an error, and leaves the
// destination buffer empty.
func TestRedactingWriterEmptyInputClose(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	w := redact.NewWriter(&buf, redact.ModeStandard)
	if err := w.Close(); err != nil {
		t.Fatalf("Close on empty writer returned error: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("expected empty buffer after no writes, got %q", buf.String())
	}
}

// TestRedactingWriterWriteAfterClose verifies that a Write call after Close
// returns io.ErrClosedPipe.
func TestRedactingWriterWriteAfterClose(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	w := redact.NewWriter(&buf, redact.ModeStandard)
	w.Close()
	_, err := w.Write([]byte("late write"))
	if !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("expected io.ErrClosedPipe after Close, got %v", err)
	}
}
