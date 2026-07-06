package cli

import (
	"github.com/dvmrry/zscalerctl/internal/machine"
	"github.com/dvmrry/zscalerctl/internal/output"
	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/spf13/cobra"
)

// newMachineCmd returns the Cobra "machine" parent command with machine-first
// contract/discovery subcommands. It is config-free: no subcommand in this
// group may load tenant config or credentials unless that behavior is made
// explicit in a future surface change.
func (a *App) newMachineCmd(opts globalOptions) *cobra.Command {
	parent := &cobra.Command{
		Use:   "machine",
		Short: "inspect machine-readable capability contracts",
		RunE: func(_ *cobra.Command, args []string) error {
			return UsageError{Message: "usage: zscalerctl machine manifest"}
		},
	}
	parent.AddCommand(a.newMachineManifestCmd(opts))
	return parent
}

// newMachineManifestCmd returns the "machine manifest" subcommand. The
// manifest is derived from the same resource catalog that drives resource
// execution; it does not execute resources, load config, construct SDK clients,
// or contact Zscaler.
func (a *App) newMachineManifestCmd(opts globalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "manifest",
		Short: "print the machine capability manifest",
		Long: "Print the catalog-derived machine capability manifest as JSON.\n\n" +
			"This command is config-free and never contacts Zscaler.",
		RunE: func(_ *cobra.Command, args []string) error {
			return a.runMachineManifest(opts, args)
		},
	}
}

func (a *App) runMachineManifest(opts globalOptions, args []string) error {
	if err := requireNoArgs("machine manifest", args); err != nil {
		return err
	}
	if opts.format != output.FormatJSON {
		return rejectUnsupportedFormat("machine manifest", opts.format)
	}
	manifest := machine.ManifestFromCatalog(a.resourceCatalog())
	return output.NewRenderer(redact.New(redact.ModeStandard)).WriteJSON(a.out, manifest)
}
