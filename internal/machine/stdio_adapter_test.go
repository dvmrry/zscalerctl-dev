package machine_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/machine"
)

func TestStdioAdapterHarnessRunsOneJSONRequest(t *testing.T) {
	req := machine.Request{
		RequestID:  "req-stdio-get",
		Capability: machine.CapabilityResourcesRead,
		Operation:  machine.OperationGet,
		Input: &machine.Input{
			Product:  "zia",
			Resource: "locations",
			RecordID: "loc-1",
		},
	}
	resp := machine.Response{
		RequestID:  req.RequestID,
		Capability: req.Capability,
		Operation:  req.Operation,
		Records: []map[string]any{
			{"id": "loc-1", "name": "HQ", "status": "active"},
		},
		Meta: &machine.Meta{
			RequestID: req.RequestID,
			Product:   "zia",
			Resource:  "locations",
			ReadOnly:  true,
			Count:     1,
		},
	}
	executor := &stdioHarnessExecutor{resp: resp}
	var stdout bytes.Buffer

	if err := runMachineRequest(context.Background(), strings.NewReader(mustJSON(t, req)), &stdout, executor); err != nil {
		t.Fatalf("runMachineRequest(valid get request) error = %v, want nil", err)
	}

	wantCalls := []machine.Request{req}
	if !reflect.DeepEqual(executor.calls, wantCalls) {
		t.Fatalf("runMachineRequest(valid get request) executor calls = %#v, want %#v",
			executor.calls, wantCalls)
	}
	got := decodeMachineResponse(t, stdout.Bytes())
	if !reflect.DeepEqual(got, resp) {
		t.Fatalf("runMachineRequest(valid get request) response JSON = %#v, want %#v", got, resp)
	}
	if !bytes.HasSuffix(stdout.Bytes(), []byte("\n")) {
		t.Fatalf("runMachineRequest(valid get request) output = %q, want newline-delimited JSON", stdout.Bytes())
	}
}

func TestStdioAdapterHarnessWritesMachineErrorResponse(t *testing.T) {
	req := machine.Request{
		RequestID:  "req-stdio-missing-id",
		Capability: machine.CapabilityResourcesRead,
		Operation:  machine.OperationGet,
		Input:      &machine.Input{Product: "zia", Resource: "locations"},
	}
	machineErr := &machine.MachineError{
		Kind:      machine.ErrorKindUsage,
		Message:   "missing required input: input.record_id",
		Missing:   []string{"input.record_id"},
		Operation: machine.OperationGet,
		Product:   "zia",
		Resource:  "locations",
	}
	resp := machine.Response{
		RequestID:  req.RequestID,
		Capability: req.Capability,
		Operation:  req.Operation,
		Error:      machineErr,
		Meta: &machine.Meta{
			RequestID: req.RequestID,
			Product:   "zia",
			Resource:  "locations",
			ReadOnly:  true,
		},
	}
	executor := &stdioHarnessExecutor{resp: resp, err: machineErr}
	var stdout bytes.Buffer

	err := runMachineRequest(context.Background(), strings.NewReader(mustJSON(t, req)), &stdout, executor)
	if !errors.Is(err, machineErr) {
		t.Fatalf("runMachineRequest(machine error response) error = %v, want %v", err, machineErr)
	}
	got := decodeMachineResponse(t, stdout.Bytes())
	if !reflect.DeepEqual(got, resp) {
		t.Fatalf("runMachineRequest(machine error response) response JSON = %#v, want %#v", got, resp)
	}
}

func TestStdioAdapterHarnessRejectsInvalidJSONBeforeExecutor(t *testing.T) {
	executor := &stdioHarnessExecutor{}
	var stdout bytes.Buffer

	err := runMachineRequest(context.Background(), strings.NewReader(`{"operation":`), &stdout, executor)
	if err == nil {
		t.Fatal("runMachineRequest(invalid request JSON) error = nil, want decode error")
	}
	if len(executor.calls) != 0 {
		t.Fatalf("runMachineRequest(invalid request JSON) executor calls = %#v, want none", executor.calls)
	}
	if stdout.Len() != 0 {
		t.Fatalf("runMachineRequest(invalid request JSON) output = %q, want empty output", stdout.String())
	}
}

func TestStdioAdapterHarnessReturnsEncodeError(t *testing.T) {
	req := machine.Request{
		RequestID:  "req-stdio",
		Capability: machine.CapabilityResourcesRead,
		Operation:  machine.OperationList,
		Input:      &machine.Input{Product: "zia", Resource: "locations"},
	}
	writeErr := errors.New("write failed")
	executor := &stdioHarnessExecutor{
		resp: machine.Response{
			RequestID:  req.RequestID,
			Capability: req.Capability,
			Operation:  req.Operation,
			Meta:       &machine.Meta{RequestID: req.RequestID, ReadOnly: true},
		},
	}

	err := runMachineRequest(context.Background(), strings.NewReader(mustJSON(t, req)), failingStdioWriter{err: writeErr}, executor)
	if !errors.Is(err, writeErr) {
		t.Fatalf("runMachineRequest(failing writer) error = %v, want %v", err, writeErr)
	}
	wantCalls := []machine.Request{req}
	if !reflect.DeepEqual(executor.calls, wantCalls) {
		t.Fatalf("runMachineRequest(failing writer) executor calls = %#v, want %#v",
			executor.calls, wantCalls)
	}
}

type stdioMachineExecutor interface {
	Execute(context.Context, machine.Request) (machine.Response, error)
}

func runMachineRequest(
	ctx context.Context,
	r io.Reader,
	w io.Writer,
	executor stdioMachineExecutor,
) error {
	var req machine.Request
	if err := json.NewDecoder(r).Decode(&req); err != nil {
		return err
	}
	resp, execErr := executor.Execute(ctx, req)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		return err
	}
	return execErr
}

type stdioHarnessExecutor struct {
	resp  machine.Response
	err   error
	calls []machine.Request
}

func (e *stdioHarnessExecutor) Execute(_ context.Context, req machine.Request) (machine.Response, error) {
	e.calls = append(e.calls, req)
	return e.resp, e.err
}

type failingStdioWriter struct {
	err error
}

func (w failingStdioWriter) Write([]byte) (int, error) {
	return 0, w.err
}

func mustJSON(t *testing.T, value any) string {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal(%T) error = %v, want nil", value, err)
	}
	return string(body)
}

func decodeMachineResponse(t *testing.T, body []byte) machine.Response {
	t.Helper()
	var got machine.Response
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("json.Unmarshal(machine.Response) error = %v; body %s", err, body)
	}
	return got
}
