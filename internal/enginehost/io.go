package enginehost

import (
	"context"
	"errors"
	"io"

	"github.com/dvmrry/zscalerctl/internal/enginewire"
)

type decodeResult struct {
	data []byte
	err  error
}

func runDecoder(
	ctx context.Context,
	input io.Reader,
	limits <-chan int,
	results chan<- decodeResult,
	done chan<- struct{},
) {
	defer close(done)
	reader := enginewire.NewFrameReader(input, enginewire.BootstrapFrameBytes)
	for {
		var limit int
		select {
		case <-ctx.Done():
			return
		case value, ok := <-limits:
			if !ok {
				return
			}
			limit = value
		}
		data, err := reader.ReadFrameLimit(limit)
		result := decodeResult{data: data, err: err}
		select {
		case results <- result:
		case <-ctx.Done():
			return
		}
		if err != nil {
			return
		}
	}
}

type outboundKind uint8

const (
	outboundHello outboundKind = iota + 1
	outboundReady
	outboundStarted
	outboundRejected
	outboundProvisional
	outboundSuccess
	outboundTerminal
	outboundFatal
)

type outboundFrame struct {
	kind      outboundKind
	bootstrap enginewire.BootstrapServerFrame
	v1        enginewire.ServerFrame
}

type writeAck struct {
	frame outboundFrame
	err   error
}

func runWriter(
	ctx context.Context,
	output io.Writer,
	writes <-chan outboundFrame,
	acks chan<- writeAck,
	done chan<- struct{},
) {
	defer close(done)
	for {
		var frame outboundFrame
		select {
		case <-ctx.Done():
			return
		case value, ok := <-writes:
			if !ok {
				return
			}
			frame = value
		}
		var err error
		if frame.bootstrap != nil {
			err = enginewire.WriteBootstrapServerFrame(output, frame.bootstrap)
		} else {
			err = enginewire.WriteServerFrame(output, frame.v1)
		}
		select {
		case acks <- writeAck{frame: frame, err: err}:
		case <-ctx.Done():
			return
		}
		if err != nil && !outboundCodecError(err) {
			return
		}
	}
}

func outboundCodecError(err error) bool {
	return errors.Is(err, enginewire.ErrInvalidFrame) ||
		errors.Is(err, enginewire.ErrFrameTooLarge) ||
		errors.Is(err, enginewire.ErrInvalidJSON) ||
		errors.Is(err, enginewire.ErrJSONDepth) ||
		errors.Is(err, enginewire.ErrInvalidUTF8) ||
		errors.Is(err, enginewire.ErrDuplicateKey)
}
