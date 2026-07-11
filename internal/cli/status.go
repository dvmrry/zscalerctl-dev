package cli

import (
	"context"

	"github.com/dvmrry/zscalerctl/internal/config"
	"github.com/dvmrry/zscalerctl/internal/machine"
	machineruntime "github.com/dvmrry/zscalerctl/internal/runtime"
)

func inspectRuntimeStatus(
	ctx context.Context,
	cfg config.Config,
	opts globalOptions,
	operation machine.Operation,
) (machine.StatusResult, error) {
	inspector, err := machineruntime.NewStatusInspectorFromConfig(
		ctx,
		cfg,
		machineruntime.Options{Timeout: opts.timeout},
	)
	if err != nil {
		return machine.StatusResult{}, err
	}
	return inspector.Inspect(ctx, machine.StatusRequest{Operation: operation})
}
