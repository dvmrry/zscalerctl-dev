package machineio_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/machine"
	"github.com/dvmrry/zscalerctl/internal/machineio"
)

func TestExecuteJSONRunsOneMachineRequest(t *testing.T) {
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
	executor := &machineIOHarnessExecutor{resp: resp}
	var stdout bytes.Buffer

	if err := machineio.ExecuteJSON(
		context.Background(),
		strings.NewReader(mustJSON(t, req)),
		&stdout,
		executor,
	); err != nil {
		t.Fatalf("machineio.ExecuteJSON(valid get request) error = %v, want nil", err)
	}

	wantCalls := []machine.Request{req}
	if !reflect.DeepEqual(executor.calls, wantCalls) {
		t.Fatalf("machineio.ExecuteJSON(valid get request) executor calls = %#v, want %#v",
			executor.calls, wantCalls)
	}
	got := decodeMachineResponse(t, stdout.Bytes())
	if !reflect.DeepEqual(got, resp) {
		t.Fatalf("machineio.ExecuteJSON(valid get request) response JSON = %#v, want %#v", got, resp)
	}
	if !bytes.HasSuffix(stdout.Bytes(), []byte("\n")) {
		t.Fatalf("machineio.ExecuteJSON(valid get request) output = %q, want newline-delimited JSON", stdout.Bytes())
	}
}

func TestExecuteJSONWritesMachineErrorResponse(t *testing.T) {
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
	executor := &machineIOHarnessExecutor{resp: resp, err: machineErr}
	var stdout bytes.Buffer

	err := machineio.ExecuteJSON(context.Background(), strings.NewReader(mustJSON(t, req)), &stdout, executor)
	if !errors.Is(err, machineErr) {
		t.Fatalf("machineio.ExecuteJSON(machine error response) error = %v, want %v", err, machineErr)
	}
	got := decodeMachineResponse(t, stdout.Bytes())
	if !reflect.DeepEqual(got, resp) {
		t.Fatalf("machineio.ExecuteJSON(machine error response) response JSON = %#v, want %#v", got, resp)
	}
}

func TestExecuteJSONRejectsInvalidJSONBeforeExecutor(t *testing.T) {
	executor := &machineIOHarnessExecutor{}
	var stdout bytes.Buffer

	err := machineio.ExecuteJSON(context.Background(), strings.NewReader(`{"operation":`), &stdout, executor)
	if err == nil {
		t.Fatal("machineio.ExecuteJSON(invalid request JSON) error = nil, want decode error")
	}
	if len(executor.calls) != 0 {
		t.Fatalf("machineio.ExecuteJSON(invalid request JSON) executor calls = %#v, want none", executor.calls)
	}
	if stdout.Len() != 0 {
		t.Fatalf("machineio.ExecuteJSON(invalid request JSON) output = %q, want empty output", stdout.String())
	}
}

func TestExecuteJSONRejectsStrictDecodeFailuresBeforeExecutor(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantErr error
	}{
		{
			name: "trailing JSON value",
			body: `{"operation":"list"} {"operation":"get"}`,
		},
		{
			name: "unknown field",
			body: `{"operation":"list","surprise":true}`,
		},
		{
			name:    "oversized request",
			body:    `{"request_id":"` + strings.Repeat("x", int(machineio.DefaultDecodeMaxBytes)) + `","operation":"list"}`,
			wantErr: machineio.ErrRequestTooLarge,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := &machineIOHarnessExecutor{}
			var stdout bytes.Buffer

			err := machineio.ExecuteJSON(context.Background(), strings.NewReader(tt.body), &stdout, executor)
			if err == nil {
				t.Fatalf("machineio.ExecuteJSON(%s) error = nil, want decode error", tt.name)
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Fatalf("machineio.ExecuteJSON(%s) error = %v, want %v", tt.name, err, tt.wantErr)
			}
			if len(executor.calls) != 0 {
				t.Fatalf("machineio.ExecuteJSON(%s) executor calls = %#v, want none", tt.name, executor.calls)
			}
			if stdout.Len() != 0 {
				t.Fatalf("machineio.ExecuteJSON(%s) output = %q, want empty output", tt.name, stdout.String())
			}
		})
	}
}

func TestExecuteJSONReturnsEncodeError(t *testing.T) {
	req := machine.Request{
		RequestID:  "req-stdio",
		Capability: machine.CapabilityResourcesRead,
		Operation:  machine.OperationList,
		Input:      &machine.Input{Product: "zia", Resource: "locations"},
	}
	writeErr := errors.New("write failed")
	executor := &machineIOHarnessExecutor{
		resp: machine.Response{
			RequestID:  req.RequestID,
			Capability: req.Capability,
			Operation:  req.Operation,
			Meta:       &machine.Meta{RequestID: req.RequestID, ReadOnly: true},
		},
	}

	err := machineio.ExecuteJSON(
		context.Background(),
		strings.NewReader(mustJSON(t, req)),
		failingMachineIOWriter{err: writeErr},
		executor,
	)
	if !errors.Is(err, writeErr) {
		t.Fatalf("machineio.ExecuteJSON(failing writer) error = %v, want %v", err, writeErr)
	}
	wantCalls := []machine.Request{req}
	if !reflect.DeepEqual(executor.calls, wantCalls) {
		t.Fatalf("machineio.ExecuteJSON(failing writer) executor calls = %#v, want %#v",
			executor.calls, wantCalls)
	}
}

func TestDecodeRequest(t *testing.T) {
	want := machine.Request{
		RequestID:  "req-decode",
		Capability: machine.CapabilityResourcesRead,
		Operation:  machine.OperationShow,
		Input:      &machine.Input{Product: "zia", Resource: "advanced-settings"},
	}

	got, err := machineio.DecodeRequest(strings.NewReader(mustJSON(t, want)))
	if err != nil {
		t.Fatalf("machineio.DecodeRequest(valid request) error = %v, want nil", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("machineio.DecodeRequest(valid request) = %#v, want %#v", got, want)
	}
}

func TestDecodeRequestStrict(t *testing.T) {
	want := machine.Request{
		RequestID:  "req-strict",
		Capability: machine.CapabilityResourcesRead,
		Operation:  machine.OperationList,
		Input:      &machine.Input{Product: "zia", Resource: "locations"},
	}

	got, err := machineio.DecodeRequestStrict(strings.NewReader(mustJSON(t, want)))
	if err != nil {
		t.Fatalf("machineio.DecodeRequestStrict(valid request) error = %v, want nil", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("machineio.DecodeRequestStrict(valid request) = %#v, want %#v", got, want)
	}
}

func TestDecodeRequestStrictRejectsTrailingJSON(t *testing.T) {
	_, err := machineio.DecodeRequestStrict(strings.NewReader(`{"operation":"list"} {"operation":"get"}`))
	if !errors.Is(err, machineio.ErrTrailingJSON) {
		t.Fatalf("machineio.DecodeRequestStrict(trailing JSON) error = %v, want ErrTrailingJSON", err)
	}
}

func TestDecodeRequestStrictRejectsUnknownFields(t *testing.T) {
	_, err := machineio.DecodeRequestStrict(strings.NewReader(`{"operation":"list","surprise":true}`))
	if err == nil {
		t.Fatal("machineio.DecodeRequestStrict(unknown field) error = nil, want decode error")
	}
}

func TestDecodeRequestWithOptionsRejectsOversizedInput(t *testing.T) {
	req := machine.Request{
		RequestID: "req-too-large",
		Operation: machine.OperationList,
	}

	_, err := machineio.DecodeRequestWithOptions(strings.NewReader(mustJSON(t, req)), machineio.DecodeOptions{
		MaxBytes: 8,
	})
	if !errors.Is(err, machineio.ErrRequestTooLarge) {
		t.Fatalf("machineio.DecodeRequestWithOptions(oversized request) error = %v, want ErrRequestTooLarge", err)
	}
}

func TestEncodeResponse(t *testing.T) {
	want := machine.Response{
		RequestID:  "req-encode",
		Capability: machine.CapabilityResourcesRead,
		Operation:  machine.OperationList,
		Records:    []map[string]any{{"id": "loc-1", "name": "HQ"}},
		Meta:       &machine.Meta{RequestID: "req-encode", ReadOnly: true, Count: 1},
	}
	var stdout bytes.Buffer

	if err := machineio.EncodeResponse(&stdout, want); err != nil {
		t.Fatalf("machineio.EncodeResponse(response) error = %v, want nil", err)
	}
	got := decodeMachineResponse(t, stdout.Bytes())
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("machineio.EncodeResponse(response) JSON = %#v, want %#v", got, want)
	}
}

type machineIOHarnessExecutor struct {
	resp  machine.Response
	err   error
	calls []machine.Request
}

func (e *machineIOHarnessExecutor) Execute(_ context.Context, req machine.Request) (machine.Response, error) {
	e.calls = append(e.calls, req)
	return e.resp, e.err
}

type failingMachineIOWriter struct {
	err error
}

func (w failingMachineIOWriter) Write([]byte) (int, error) {
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
