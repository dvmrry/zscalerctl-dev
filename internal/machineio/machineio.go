// Package machineio provides JSON request/response helpers for machine
// adapters.
//
// The package deliberately stops at transport-neutral JSON I/O. It does not
// load config, construct SDK clients, render human output, map process exit
// codes, or write CLI error envelopes.
package machineio

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/dvmrry/zscalerctl/internal/machine"
)

const (
	// DefaultDecodeMaxBytes is the default strict request size limit.
	DefaultDecodeMaxBytes int64 = 1 << 20
)

var (
	// ErrRequestTooLarge reports a strict decode request larger than MaxBytes.
	ErrRequestTooLarge = errors.New("machine request exceeds maximum size")

	// ErrTrailingJSON reports extra JSON values after the first request.
	ErrTrailingJSON = errors.New("machine request contains trailing JSON value")
)

// Executor is the machine execution surface required by ExecuteJSON.
type Executor interface {
	Execute(context.Context, machine.Request) (machine.Response, error)
}

// DecodeOptions controls strict machine request decoding.
type DecodeOptions struct {
	MaxBytes              int64
	DisallowUnknownFields bool
	RejectTrailingValues  bool
}

// DecodeRequest reads one JSON machine request from r.
func DecodeRequest(r io.Reader) (machine.Request, error) {
	return DecodeRequestWithOptions(r, DecodeOptions{})
}

// DecodeRequestStrict reads one bounded JSON machine request from r and rejects
// unknown fields or trailing JSON values.
func DecodeRequestStrict(r io.Reader) (machine.Request, error) {
	return DecodeRequestWithOptions(r, DecodeOptions{
		MaxBytes:              DefaultDecodeMaxBytes,
		DisallowUnknownFields: true,
		RejectTrailingValues:  true,
	})
}

// DecodeRequestWithOptions reads one JSON machine request from r using opts.
func DecodeRequestWithOptions(r io.Reader, opts DecodeOptions) (machine.Request, error) {
	reader := r
	if opts.MaxBytes > 0 {
		body, err := io.ReadAll(io.LimitReader(r, opts.MaxBytes+1))
		if err != nil {
			return machine.Request{}, err
		}
		if int64(len(body)) > opts.MaxBytes {
			return machine.Request{}, fmt.Errorf("%w: max %d bytes", ErrRequestTooLarge, opts.MaxBytes)
		}
		reader = bytes.NewReader(body)
	}

	decoder := json.NewDecoder(reader)
	if opts.DisallowUnknownFields {
		decoder.DisallowUnknownFields()
	}
	var req machine.Request
	if err := decoder.Decode(&req); err != nil {
		return machine.Request{}, err
	}
	if opts.RejectTrailingValues {
		var extra any
		if err := decoder.Decode(&extra); err == nil {
			return machine.Request{}, ErrTrailingJSON
		} else if !errors.Is(err, io.EOF) {
			return machine.Request{}, err
		}
	}
	return req, nil
}

// EncodeResponse writes one newline-delimited JSON machine response to w.
func EncodeResponse(w io.Writer, resp machine.Response) error {
	return json.NewEncoder(w).Encode(resp)
}

// ExecuteJSON decodes one request, executes it, and encodes the response.
//
// If the executor returns a machine error response, ExecuteJSON still writes
// that response before returning the executor error to the caller.
func ExecuteJSON(ctx context.Context, r io.Reader, w io.Writer, executor Executor) error {
	req, err := DecodeRequestStrict(r)
	if err != nil {
		return err
	}
	resp, execErr := executor.Execute(ctx, req)
	if err := EncodeResponse(w, resp); err != nil {
		return err
	}
	return execErr
}
