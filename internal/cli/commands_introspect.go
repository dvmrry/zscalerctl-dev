package cli

import (
	"github.com/dvmrry/zscalerctl/internal/output"
	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/version"
	"github.com/spf13/cobra"
)

// newIntrospectCmd returns the Cobra "introspect" subcommand. It is config-free:
// it does NOT call LoadConfig, build a reader, or touch the network. The output
// is the static CLI surface map (commands, flags, catalog, exit codes) as JSON
// by default, or as a human-readable indented tree with --format table/pretty.
//
// FormatAuto resolves to JSON here (machine-first by default). Only explicit
// --format table/--format pretty produces the human tree renderer.
// --format ndjson is rejected: introspect is a single document, not a stream.
func (a *App) newIntrospectCmd(opts globalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "introspect",
		Short: "print a machine-readable map of all commands, flags, and resources (JSON)",
		RunE: func(_ *cobra.Command, args []string) error {
			return a.runIntrospect(opts, args)
		},
	}
}

func (a *App) runIntrospect(opts globalOptions, args []string) error {
	if err := requireNoArgs("introspect", args); err != nil {
		return err
	}
	doc := IntrospectTree(a)
	doc.CLIVersion = version.Current().Version
	// JSON is the happy-path (machine-first default); auto resolves to JSON for
	// non-TTY and to pretty for TTY via resolveFormat before RunE fires.
	if opts.format == output.FormatJSON {
		return output.NewRenderer(redact.New(redact.ModeStandard)).WriteJSON(a.out, doc)
	}
	if opts.format == output.FormatTable || opts.format == output.FormatPretty {
		treeText := introspectTreeText(doc)
		return output.NewRenderer(redact.New(redact.ModeStandard)).WriteText(a.out, output.NewSafeText(treeText))
	}
	// ndjson and any future unrecognised formats are rejected.
	return rejectUnsupportedFormat("introspect", opts.format)
}
