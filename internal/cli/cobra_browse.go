package cli

import (
	"context"
	"errors"
	"os"

	"github.com/spf13/cobra"

	"github.com/dvmrry/zscalerctl/internal/output"
	"github.com/dvmrry/zscalerctl/internal/tui/launcher"
)

// newBrowseCmd returns the experimental "browse" command. It is hidden from
// normal help and only launches the TUI when explicitly requested with --tui.
// The command is fixture-backed: it does not load config, resolve credentials,
// or contact Zscaler. It exists to prove the command→launcher→Bubble Tea path
// before wiring real reader data.
func (a *App) newBrowseCmd(opts globalOptions) *cobra.Command {
	var tuiFlag bool
	cmd := &cobra.Command{
		Use:    "browse",
		Short:  "experimental TUI browser",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !tuiFlag {
				return UsageError{Message: "browse currently requires --tui"}
			}
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			// a.stdoutTTY/a.stderrTTY were captured when the App was constructed from
			// the user's real terminal streams. They are not affected by the
			// --output-file wrapper or by muteProcessOutput. os.Stdin is never muted,
			// so it is still the user's terminal.
			err := launcher.LaunchBrowser(ctx, launcher.Config{
				Requested:  tuiFlag,
				StdinTTY:   output.IsTerminal(os.Stdin),
				StdoutTTY:  a.stdoutTTY,
				StderrTTY:  a.stderrTTY,
				Format:     opts.format,
				ColorMode:  opts.colorMode,
				OutputPath: opts.output,
				Env:        a.env,
				Input:      os.Stdin,
				Output:     a.out,
			})
			var launchErr launcher.LaunchError
			if errors.As(err, &launchErr) {
				return UsageError{Message: launchErr.Error()}
			}
			return err
		},
	}
	cmd.Flags().BoolVar(&tuiFlag, "tui", false, "launch the interactive TUI browser")
	return cmd
}
