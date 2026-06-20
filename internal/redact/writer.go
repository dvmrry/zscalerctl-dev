package redact

import (
	"bytes"
	"io"
)

// writer is a full-buffering io.WriteCloser that accumulates all written bytes
// and, on Close, runs the entire accumulated content through the redactor before
// writing to the wrapped writer.
//
// Full-buffering (rather than per-write or per-line) is deliberate: a secret
// split across two Write calls, or a multi-line PEM block, must still be caught
// as a unit.
type writer struct {
	dst      io.Writer
	redactor Redactor
	buf      bytes.Buffer
	closed   bool
}

// NewWriter returns an io.WriteCloser that accumulates all writes in memory.
// On Close, the entire accumulated buffer is run through ScanRenderedString
// once and the redacted result is written to dst.
//
// The caller must call Close to flush; if Close is never called the data is
// silently discarded. A second Close is a no-op.
func NewWriter(dst io.Writer, mode Mode) io.WriteCloser {
	return &writer{
		dst:      dst,
		redactor: New(mode),
	}
}

// Write appends p to the internal buffer. It never returns an error.
func (w *writer) Write(p []byte) (int, error) {
	if w.closed {
		return 0, io.ErrClosedPipe
	}
	return w.buf.Write(p)
}

// Close redacts the full accumulated buffer and writes it to the wrapped
// writer. It is idempotent after the first call.
func (w *writer) Close() error {
	if w.closed {
		return nil
	}
	w.closed = true

	redacted, _ := w.redactor.ScanRenderedString(w.buf.String()) // second value is a Report, not an error
	_, err := io.WriteString(w.dst, redacted)
	return err
}
