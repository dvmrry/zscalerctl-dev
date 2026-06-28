package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os/exec"
	"reflect"
	"strings"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/machine"
)

func TestRunValidRequestWritesNewlineDelimitedJSONResponse(t *testing.T) {
	req := machine.Request{
		RequestID:  "req-experiment-list",
		Capability: machine.CapabilityResourcesRead,
		Operation:  machine.OperationList,
		Input: &machine.Input{
			Product:  "zia",
			Resource: "locations",
		},
	}
	want := machine.Response{
		RequestID:  req.RequestID,
		Capability: req.Capability,
		Operation:  req.Operation,
		Records: []map[string]any{
			{
				"id":       "loc-1",
				"name":     "Branch",
				"readOnly": true,
			},
		},
		Meta: &machine.Meta{
			RequestID: req.RequestID,
			Product:   "zia",
			Resource:  "locations",
			ReadOnly:  true,
			Count:     1,
		},
	}
	rt := &recordingRuntime{response: want}
	var gotEnv []string
	var stdout bytes.Buffer

	err := run(context.Background(), strings.NewReader(mustJSON(t, req)), &stdout, runOptions{
		Env: []string{"ZSCALERCTL_CLIENT_ID=test-client"},
		NewRuntime: func(_ context.Context, env []string) (machineExecutor, error) {
			gotEnv = append([]string(nil), env...)
			return rt, nil
		},
	})
	if err != nil {
		t.Fatalf("run(valid request) error = %v, want nil", err)
	}
	if !reflect.DeepEqual(gotEnv, []string{"ZSCALERCTL_CLIENT_ID=test-client"}) {
		t.Fatalf("run(valid request) runtime env = %#v, want injected env", gotEnv)
	}
	if len(rt.calls) != 1 || !reflect.DeepEqual(rt.calls[0], req) {
		t.Fatalf("run(valid request) runtime calls = %#v, want one original request", rt.calls)
	}
	if !bytes.HasSuffix(stdout.Bytes(), []byte("\n")) {
		t.Fatalf("run(valid request) output = %q, want newline-delimited JSON", stdout.Bytes())
	}
	if got := decodeMachineResponse(t, stdout.Bytes()); !reflect.DeepEqual(got, want) {
		t.Fatalf("run(valid request) response = %#v, want %#v", got, want)
	}
}

func TestRunWritesMachineErrorResponseBeforeReturningError(t *testing.T) {
	req := machine.Request{
		RequestID:  "req-experiment-missing-id",
		Capability: machine.CapabilityResourcesRead,
		Operation:  machine.OperationGet,
		Input: &machine.Input{
			Product:  "zia",
			Resource: "locations",
		},
	}
	machineErr := &machine.MachineError{
		Kind:      machine.ErrorKindUsage,
		Message:   "missing required input: input.record_id",
		Missing:   []string{"input.record_id"},
		Operation: machine.OperationGet,
		Product:   "zia",
		Resource:  "locations",
	}
	rt := &recordingRuntime{
		response: machine.Response{
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
		},
		err: machineErr,
	}
	var stdout bytes.Buffer

	err := run(context.Background(), strings.NewReader(mustJSON(t, req)), &stdout, runOptions{
		NewRuntime: func(context.Context, []string) (machineExecutor, error) {
			return rt, nil
		},
	})
	var gotMachineErr *machine.MachineError
	if !errors.As(err, &gotMachineErr) {
		t.Fatalf("run(machine error request) error = %T %v, want *machine.MachineError", err, err)
	}
	if got, want := gotMachineErr.Kind, machine.ErrorKindUsage; got != want {
		t.Fatalf("run(machine error request) MachineError.Kind = %q, want %q", got, want)
	}
	if stdout.Len() == 0 {
		t.Fatal("run(machine error request) output is empty, want machine error response JSON")
	}

	got := decodeMachineResponse(t, stdout.Bytes())
	if got.Error == nil {
		t.Fatalf("run(machine error request) response error = nil, want MachineError; response %#v", got)
	}
	if got.Error.Kind != machine.ErrorKindUsage {
		t.Fatalf("run(machine error request) response error kind = %q, want %q",
			got.Error.Kind, machine.ErrorKindUsage)
	}
	if !reflect.DeepEqual(got.Error.Missing, []string{"input.record_id"}) {
		t.Fatalf("run(machine error request) missing fields = %#v, want %#v",
			got.Error.Missing, []string{"input.record_id"})
	}
}

func TestRunInvalidJSONFailsBeforeRuntimeConstruction(t *testing.T) {
	var stdout bytes.Buffer
	calledRuntime := false

	err := run(context.Background(), strings.NewReader(`{"operation":`), &stdout, runOptions{
		NewRuntime: func(context.Context, []string) (machineExecutor, error) {
			calledRuntime = true
			return &recordingRuntime{}, nil
		},
	})
	if err == nil {
		t.Fatal("run(invalid JSON) error = nil, want decode error")
	}
	if calledRuntime {
		t.Fatal("run(invalid JSON) constructed runtime, want strict decode failure first")
	}
	if stdout.Len() != 0 {
		t.Fatalf("run(invalid JSON) output = %q, want empty output", stdout.String())
	}
}

func TestRunRuntimeConstructionFailureWritesNoResponse(t *testing.T) {
	req := machine.Request{
		RequestID:  "req-runtime-failure",
		Capability: machine.CapabilityResourcesRead,
		Operation:  machine.OperationList,
		Input:      &machine.Input{Product: "zia", Resource: "locations"},
	}
	sentinel := errors.New("runtime construction failed")
	var stdout bytes.Buffer

	err := run(context.Background(), strings.NewReader(mustJSON(t, req)), &stdout, runOptions{
		NewRuntime: func(context.Context, []string) (machineExecutor, error) {
			return nil, sentinel
		},
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("run(runtime construction failure) error = %v, want sentinel", err)
	}
	if stdout.Len() != 0 {
		t.Fatalf("run(runtime construction failure) output = %q, want empty output", stdout.String())
	}
}

func TestExperimentHasNoForbiddenDependencies(t *testing.T) {
	assertNoForbiddenImports(t, directImports(t), []string{
		"github.com/dvmrry/zscalerctl/internal/cli",
		"github.com/dvmrry/zscalerctl/internal/output",
		"github.com/dvmrry/zscalerctl/internal/config",
		"github.com/dvmrry/zscalerctl/internal/credentials",
		"github.com/dvmrry/zscalerctl/internal/secret",
		"github.com/dvmrry/zscalerctl/internal/secretref",
		"github.com/dvmrry/zscalerctl/internal/zscaler",
	})

	assertNoForbiddenImports(t, dependencyImports(t), []string{
		"github.com/dvmrry/zscalerctl/internal/cli",
		"github.com/dvmrry/zscalerctl/internal/output",
		"github.com/charmbracelet/bubbletea",
		"github.com/charmbracelet/bubbles",
		"github.com/charmbracelet/fang",
		"github.com/charmbracelet/lipgloss/v2",
		"github.com/wailsapp/wails",
	})
}

type recordingRuntime struct {
	response machine.Response
	err      error
	calls    []machine.Request
}

func (r *recordingRuntime) Execute(_ context.Context, req machine.Request) (machine.Response, error) {
	r.calls = append(r.calls, req)
	if r.err != nil {
		return r.response, r.err
	}
	return r.response, nil
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

func directImports(t *testing.T) string {
	t.Helper()
	cmd := exec.Command("go", "list", "-f", "{{range .Imports}}{{.}}{{\"\\n\"}}{{end}}", ".")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go list direct imports error = %v\n%s", err, out)
	}
	return "\n" + string(out) + "\n"
}

func dependencyImports(t *testing.T) string {
	t.Helper()
	cmd := exec.Command("go", "list", "-deps", "-mod=mod", "./...")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go list -deps -mod=mod ./... error = %v\n%s", err, out)
	}
	return "\n" + string(out) + "\n"
}

func assertNoForbiddenImports(t *testing.T, imports string, forbidden []string) {
	t.Helper()
	for _, forbiddenImport := range forbidden {
		if strings.Contains(imports, "\n"+forbiddenImport+"\n") {
			t.Fatalf("go list imports include forbidden dependency %q\n%s",
				forbiddenImport, imports)
		}
	}
}
