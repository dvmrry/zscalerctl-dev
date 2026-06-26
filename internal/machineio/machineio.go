// Package machineio provides JSON request/response helpers for machine
// adapters.
//
// The package deliberately stops at transport-neutral JSON I/O. It does not
// load config, construct SDK clients, render human output, map process exit
// codes, or write CLI error envelopes.
package machineio

import (
	"context"
	"encoding/json"
	"io"

	"github.com/dvmrry/zscalerctl/internal/machine"
)

// Executor is the machine execution surface required by ExecuteJSON.
type Executor interface {
	Execute(context.Context, machine.Request) (machine.Response, error)
}

// DecodeRequest reads one JSON machine request from r.
func DecodeRequest(r io.Reader) (machine.Request, error) {
	var req machine.Request
	if err := json.NewDecoder(r).Decode(&req); err != nil {
		return machine.Request{}, err
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
	req, err := DecodeRequest(r)
	if err != nil {
		return err
	}
	resp, execErr := executor.Execute(ctx, req)
	if err := EncodeResponse(w, resp); err != nil {
		return err
	}
	return execErr
}
