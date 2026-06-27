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
	var stdout bytes.Buffer

	if err := run(context.Background(), strings.NewReader(mustJSON(t, req)), &stdout); err != nil {
		t.Fatalf("run(valid request) error = %v, want nil", err)
	}
	if !bytes.HasSuffix(stdout.Bytes(), []byte("\n")) {
		t.Fatalf("run(valid request) output = %q, want newline-delimited JSON", stdout.Bytes())
	}

	got := decodeMachineResponse(t, stdout.Bytes())
	want := machine.Response{
		RequestID:  req.RequestID,
		Capability: req.Capability,
		Operation:  req.Operation,
		Records: []map[string]any{
			{
				"id":        "experiment-record",
				"adapter":   "stdio-machine-adapter",
				"operation": "list",
				"product":   "zia",
				"resource":  "locations",
				"record_id": "",
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
	if !reflect.DeepEqual(got, want) {
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
	var stdout bytes.Buffer

	err := run(context.Background(), strings.NewReader(mustJSON(t, req)), &stdout)
	var machineErr *machine.MachineError
	if !errors.As(err, &machineErr) {
		t.Fatalf("run(machine error request) error = %T %v, want *machine.MachineError", err, err)
	}
	if got, want := machineErr.Kind, machine.ErrorKindUsage; got != want {
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

func TestRunInvalidJSONFailsBeforeExecutor(t *testing.T) {
	var stdout bytes.Buffer

	err := run(context.Background(), strings.NewReader(`{"operation":`), &stdout)
	if err == nil {
		t.Fatal("run(invalid JSON) error = nil, want decode error")
	}
	if stdout.Len() != 0 {
		t.Fatalf("run(invalid JSON) output = %q, want empty output", stdout.String())
	}
}

func TestExperimentHasNoForbiddenDependencies(t *testing.T) {
	cmd := exec.Command("go", "list", "-deps", "-mod=mod", "./...")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go list -deps -mod=mod ./... error = %v\n%s", err, out)
	}

	forbidden := []string{
		"github.com/dvmrry/zscalerctl/internal/cli",
		"github.com/dvmrry/zscalerctl/internal/output",
		"github.com/dvmrry/zscalerctl/internal/config",
		"github.com/dvmrry/zscalerctl/internal/credentials",
		"github.com/dvmrry/zscalerctl/internal/secret",
		"github.com/dvmrry/zscalerctl/internal/secretref",
		"github.com/dvmrry/zscalerctl/internal/zscaler",
		"github.com/charmbracelet/bubbletea",
		"github.com/charmbracelet/bubbles",
		"github.com/charmbracelet/fang",
		"github.com/charmbracelet/lipgloss/v2",
		"github.com/wailsapp/wails",
	}
	deps := "\n" + string(out) + "\n"
	for _, forbiddenDep := range forbidden {
		if strings.Contains(deps, "\n"+forbiddenDep+"\n") {
			t.Fatalf("go list -deps -mod=mod ./... includes forbidden dependency %q\n%s",
				forbiddenDep, out)
		}
	}
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
