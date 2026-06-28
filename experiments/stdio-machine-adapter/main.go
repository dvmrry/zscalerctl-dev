// Command stdio-machine-adapter is an unsupported experiment that demonstrates
// consuming the trusted read-only machine runtime facade from an isolated
// adapter.
package main

import (
	"context"
	"io"
	"os"

	"github.com/dvmrry/zscalerctl/internal/machine"
	"github.com/dvmrry/zscalerctl/internal/machineio"
	machineruntime "github.com/dvmrry/zscalerctl/internal/runtime"
)

func main() {
	if err := run(context.Background(), os.Stdin, os.Stdout, runOptions{Env: os.Environ()}); err != nil {
		os.Exit(1)
	}
}

type machineExecutor interface {
	Execute(context.Context, machine.Request) (machine.Response, error)
}

type runtimeFactory func(context.Context, []string) (machineExecutor, error)

type runOptions struct {
	Env        []string
	NewRuntime runtimeFactory
}

func run(ctx context.Context, stdin io.Reader, stdout io.Writer, opts runOptions) error {
	req, err := machineio.DecodeRequestStrict(stdin)
	if err != nil {
		return err
	}
	newRuntime := opts.NewRuntime
	if newRuntime == nil {
		newRuntime = newRuntimeExecutor
	}
	executor, err := newRuntime(ctx, opts.Env)
	if err != nil {
		return err
	}
	resp, execErr := executor.Execute(ctx, req)
	if err := machineio.EncodeResponse(stdout, resp); err != nil {
		return err
	}
	return execErr
}

func newRuntimeExecutor(ctx context.Context, env []string) (machineExecutor, error) {
	rt, err := machineruntime.NewMachine(ctx, machineruntime.Options{
		Env: append([]string(nil), env...),
	})
	if err != nil {
		return nil, err
	}
	return rt, nil
}
