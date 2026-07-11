package cli

import (
	"context"
	"fmt"

	"github.com/dvmrry/zscalerctl/internal/config"
	"github.com/dvmrry/zscalerctl/internal/machine"
	"github.com/dvmrry/zscalerctl/internal/output"
	"github.com/dvmrry/zscalerctl/internal/redact"
	machineruntime "github.com/dvmrry/zscalerctl/internal/runtime"
	"github.com/dvmrry/zscalerctl/internal/version"
	"github.com/spf13/cobra"
)

// newVersionCmd returns the Cobra "version" subcommand. It delegates directly to
// runVersion so all format/arity/redaction behaviour goes through one code path.
// No restrictive Args validator is set here — runVersion's requireNoArgs
// produces the same UsageError message as before, preserving the surface.
func (a *App) newVersionCmd(opts globalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "print version, commit, build date, and runtime info",
		RunE: func(_ *cobra.Command, args []string) error {
			return a.runVersion(opts, args)
		},
	}
}

// newDoctorCmd returns the Cobra "doctor" subcommand. Doctor requires a loaded
// config, so RunE loads it lazily and then applies the parsed global options.
//
// No restrictive Args validator is set here — runDoctor's requireNoArgs produces
// the same UsageError message as before, preserving the surface.
func (a *App) newDoctorCmd(opts globalOptions) *cobra.Command {
	return &cobra.Command{
		Use:         "doctor",
		Short:       "check configuration, credentials, and connectivity",
		Annotations: map[string]string{effectsAnnotation: configReadEffects},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireNoArgs("doctor", args); err != nil {
				return err
			}
			if err := cmd.Context().Err(); err != nil {
				return machineruntime.StatusConfigError(machine.OperationDoctor, err)
			}
			cfg, err := config.LoadConfig(a.env, config.LoadOptions{
				Profile:    opts.profile,
				ConfigPath: opts.configPath,
			})
			if err != nil {
				return machineruntime.StatusConfigError(machine.OperationDoctor, err)
			}
			applyOptions(&cfg, opts)
			return a.runDoctor(cmd.Context(), cfg, opts, args)
		},
	}
}

func (a *App) runVersion(opts globalOptions, args []string) error {
	if err := requireNoArgs("version", args); err != nil {
		return err
	}
	info := version.Current()
	if opts.format == output.FormatJSON {
		return output.NewRenderer(redact.New(redact.ModeStandard)).WriteJSON(a.out, info)
	}
	if opts.format != output.FormatTable && opts.format != output.FormatPretty {
		return rejectUnsupportedFormat("version", opts.format)
	}
	rows := []output.KV{
		{Key: "Version", Value: info.Version},
		{Key: "Commit", Value: info.Commit},
		{Key: "Date", Value: info.Date},
		{Key: "Go", Value: info.Go},
		{Key: "Platform", Value: info.OS + "/" + info.Arch},
	}
	body := renderKeyValuesForFormat(rows, opts.format, a.style(opts))
	return output.NewRenderer(redact.New(redact.ModeStandard)).WriteText(a.out, body)
}

func (a *App) runDoctor(ctx context.Context, cfg config.Config, opts globalOptions, args []string) error {
	if err := requireNoArgs("doctor", args); err != nil {
		return err
	}
	result, err := inspectRuntimeStatus(ctx, cfg, opts, machine.OperationDoctor)
	if err != nil {
		return err
	}
	status, ok := result.Doctor()
	if !ok {
		return fmt.Errorf("doctor status operation returned %q result", result.Operation())
	}
	if opts.format == output.FormatJSON {
		return a.renderer(cfg, opts).WriteJSON(a.out, status)
	}
	if opts.format != output.FormatTable && opts.format != output.FormatPretty {
		return rejectUnsupportedFormat("doctor", opts.format)
	}
	body := renderKeyValuesForFormat(doctorStatusRows(status), opts.format, a.style(opts))
	return a.renderer(cfg, opts).WriteText(a.out, body)
}
