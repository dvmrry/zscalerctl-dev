package redact_test

import (
	"bytes"
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
	io.WriteString(w, "token=AKIA")           // pattern split across writes
	io.WriteString(w, "abcdef1234567890XYZ\n")
	w.(io.Closer).Close() // flush
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
	w.(io.Closer).Close()
	if buf.String() != want {
		t.Errorf("non-secret line changed: got %q, want %q", buf.String(), want)
	}
}

// TestRedactingWriterRedactsMultiLinePEM verifies that a multi-line PEM block
// (which spans many writes) is fully redacted as a unit.
func TestRedactingWriterRedactsMultiLinePEM(t *testing.T) {
	t.Parallel()

	pem := "-----BEGIN PRIVATE KEY-----\nMIIEvgIBADANBgkqhkiG9w0BAQEFAASC\nBKYwggSiAgEAAoIBAQC7dummyfake==\n-----END PRIVATE KEY-----\n"

	var buf bytes.Buffer
	w := redact.NewWriter(&buf, redact.ModeStandard)
	// Write line by line to confirm full-buffer catches the block.
	for _, line := range strings.SplitAfter(pem, "\n") {
		io.WriteString(w, line)
	}
	w.(io.Closer).Close()
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
	w.(io.Closer).Close()
	if buf.String() != "no newline at end" {
		t.Errorf("close did not flush: got %q", buf.String())
	}
}
