package enginewire

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
)

const frameReaderBufferBytes = 32 << 10

// FrameReader reads one bounded LF-terminated NDJSON frame at a time. The
// returned bytes exclude LF and the one optional CR immediately before it.
type FrameReader struct {
	reader       *bufio.Reader
	maximumBytes int
}

func NewFrameReader(reader io.Reader, maximumBytes int) *FrameReader {
	if maximumBytes <= 0 {
		maximumBytes = V1FrameBytes
	}
	frameReader := &FrameReader{maximumBytes: maximumBytes}
	if !isNilInterface(reader) {
		frameReader.reader = bufio.NewReaderSize(reader, frameReaderBufferBytes)
	}
	return frameReader
}

// ReadFrame returns one delimiter-free frame or a framing error.
func (r *FrameReader) ReadFrame() ([]byte, error) {
	if r == nil {
		return nil, fmt.Errorf("%w: nil frame reader", ErrInvalidFrame)
	}
	return r.ReadFrameLimit(r.maximumBytes)
}

// ReadFrameLimit returns one delimiter-free frame under a per-read byte
// limit. Callers may switch from the bootstrap limit to the negotiated limit
// without replacing the buffered reader or losing bytes already read ahead.
// It is not safe to call concurrently with another read.
func (r *FrameReader) ReadFrameLimit(maximumBytes int) ([]byte, error) {
	if r == nil || r.reader == nil {
		return nil, fmt.Errorf("%w: nil frame reader", ErrInvalidFrame)
	}
	if maximumBytes <= 0 || maximumBytes > AggregateItemBytes {
		return nil, fmt.Errorf("%w: frame limit must be a bounded positive integer", ErrInvalidFrame)
	}
	frame := make([]byte, 0, min(maximumBytes, frameReaderBufferBytes))
	for {
		fragment, err := r.reader.ReadSlice('\n')
		if len(frame)+len(fragment) > maximumBytes+2 {
			return nil, ErrFrameTooLarge
		}
		frame = append(frame, fragment...)
		switch {
		case err == nil:
			return finishFrame(frame, maximumBytes)
		case errors.Is(err, bufio.ErrBufferFull):
			continue
		case errors.Is(err, io.EOF) && len(frame) == 0:
			return nil, io.EOF
		case errors.Is(err, io.EOF):
			return nil, ErrUnterminatedFrame
		default:
			return nil, err
		}
	}
}

func finishFrame(frame []byte, maximumBytes int) ([]byte, error) {
	if len(frame) == 0 || frame[len(frame)-1] != '\n' {
		return nil, ErrUnterminatedFrame
	}
	frame = frame[:len(frame)-1]
	if len(frame) > 0 && frame[len(frame)-1] == '\r' {
		frame = frame[:len(frame)-1]
	}
	if len(frame) > maximumBytes {
		return nil, ErrFrameTooLarge
	}
	if bytes.IndexByte(frame, '\r') >= 0 {
		return nil, ErrBareCarriageReturn
	}
	return frame, nil
}
