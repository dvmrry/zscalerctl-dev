// Command stdio-machine-adapter is an unsupported experiment that demonstrates
// consuming internal/machineio from an isolated adapter.
package main

import (
	"context"
	"io"
	"os"
	"strings"

	"github.com/dvmrry/zscalerctl/internal/machine"
	"github.com/dvmrry/zscalerctl/internal/machineio"
)

func main() {
	if err := run(context.Background(), os.Stdin, os.Stdout); err != nil {
		os.Exit(1)
	}
}

func run(ctx context.Context, stdin io.Reader, stdout io.Writer) error {
	return machineio.ExecuteJSON(ctx, stdin, stdout, staticExecutor{})
}

type staticExecutor struct{}

func (staticExecutor) Execute(_ context.Context, req machine.Request) (machine.Response, error) {
	resp := responseForRequest(req)
	product, resource, recordID := requestInput(req)

	if req.Operation == machine.OperationGet && recordID == "" {
		machineErr := &machine.MachineError{
			Kind:      machine.ErrorKindUsage,
			Message:   "missing required input: input.record_id",
			Missing:   []string{"input.record_id"},
			Operation: req.Operation,
			Product:   product,
			Resource:  resource,
		}
		resp.Error = machineErr
		return resp, machineErr
	}

	resp.Records = []map[string]any{
		{
			"id":        "experiment-record",
			"adapter":   "stdio-machine-adapter",
			"operation": string(req.Operation),
			"product":   product,
			"resource":  resource,
			"record_id": recordID,
		},
	}
	resp.Meta.Count = len(resp.Records)
	return resp, nil
}

func responseForRequest(req machine.Request) machine.Response {
	meta := machine.Meta{
		RequestID: req.RequestID,
		ReadOnly:  true,
	}
	meta.Product, meta.Resource, _ = requestInput(req)
	return machine.Response{
		RequestID:  req.RequestID,
		Capability: req.Capability,
		Operation:  req.Operation,
		Meta:       &meta,
	}
}

func requestInput(req machine.Request) (string, string, string) {
	if req.Input == nil {
		return "", "", ""
	}
	return strings.TrimSpace(req.Input.Product),
		strings.TrimSpace(req.Input.Resource),
		strings.TrimSpace(req.Input.RecordID)
}
