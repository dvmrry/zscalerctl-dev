package cli

import (
	"github.com/dvmrry/zscalerctl/internal/output"
	"github.com/spf13/cobra"
)

// newDumpCmd returns the Cobra "dump" subcommand. Local flags (--out,
// --products, --resources, --continue-on-error, --force) are declared as Cobra
// local flags and translated into the typed engine request after parsing.
//
// --format ndjson is rejected before LoadConfig.
// --out validation (non-empty) is enforced inside runDumpWithOptions.
// MarkFlagRequired is NOT used for --out because it would bypass the canonical
// UsageError path.
//
// No cobra.Args validator is set: NArg() == 0 is checked in RunE so the exact
// UsageError message ("usage: zscalerctl dump ...") is preserved.
func (a *App) newDumpCmd(opts globalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dump",
		Short: "write a full or filtered resource dump to a directory",
		Annotations: map[string]string{
			effectsAnnotation: credentialedReadEffects + "," +
				effectKindLocalFilesystemRead + "@force," +
				effectKindLocalFilesystemWrite + "," +
				effectKindLocalFilesystemDelete + "@force",
			// App.Run rejects --output for normal dump execution. Dump owns its
			// output directory through --out instead.
			suppressGlobalFlagEffectsAnnotation: "output",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// Reject --format ndjson first, before any config work.
			if opts.format == output.FormatNDJSON {
				return rejectUnsupportedFormat("dump", opts.format)
			}
			// Reject extra positional args before config load.
			if cmd.Flags().NArg() != 0 {
				return UsageError{Message: dumpUsage(a.resourceCatalog())}
			}
			outDir, _ := cmd.Flags().GetString("out")
			productsFlag, _ := cmd.Flags().GetString("products")
			resourcesFlag, _ := cmd.Flags().GetString("resources")
			continueOnError, _ := cmd.Flags().GetBool("continue-on-error")
			force, _ := cmd.Flags().GetBool("force")
			return a.runDumpWithOptions(cmd.Context(), opts, dumpOptions{
				out:             outDir,
				products:        productsFlag,
				resources:       resourcesFlag,
				continueOnError: continueOnError,
				force:           force,
			})
		},
	}
	cmd.Flags().String("out", "", "dump output directory")
	cmd.Flags().String("products", "", "comma-separated products: zia,zpa")
	cmd.Flags().String("resources", "", "comma-separated resources: locations or zia/locations")
	cmd.Flags().Bool("continue-on-error", false, "write a partial dump when individual resources fail")
	cmd.Flags().Bool("force", false, "replace an existing zscalerctl dump directory")
	return cmd
}

// newDiffCmd returns the Cobra "diff" subcommand. Diff is config-FREE — it
// compares two local dump directories and never calls LoadConfig.
//
// Local flags (--products, --resources, --ignore-operational, --detail,
// --allow-partial, --fail-on-drift) are declared as Cobra local flags and
// read inside RunE after parsing.
//
// --format ndjson is rejected before any Compare work. The two positional dirs
// are read from cmd.Flags().Args() and
// exactly 2 are required (len != 2 → UsageError{diffUsage()}).
//
// MarkFlagRequired is NOT used because it would bypass the canonical UsageError
// path.
// cobra.ExactArgs is NOT used — plain error → wrong exit code.
func (a *App) newDiffCmd(opts globalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "diff <old-dump-dir> <new-dump-dir>",
		Short: "compare two dump directories and report configuration drift",
		Annotations: map[string]string{
			// Exactly 2 positionals required; Use suffix alone is not enough for
			// buildArgsDoc to infer this — the annotation makes it explicit.
			"introspect/args-policy": "exact:2",
			effectsAnnotation:        effectKindLocalFilesystemRead,
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// Reject --format ndjson first, before any work.
			if opts.format == output.FormatNDJSON {
				return rejectUnsupportedFormat("diff", opts.format)
			}
			// Cobra passes non-flag args here; require exactly 2 dir positionals.
			positionals := cmd.Flags().Args()
			if len(positionals) != 2 {
				return UsageError{Message: diffUsage(a.resourceCatalog())}
			}
			products, _ := cmd.Flags().GetString("products")
			resources, _ := cmd.Flags().GetString("resources")
			ignoreOperational, _ := cmd.Flags().GetBool("ignore-operational")
			detail, _ := cmd.Flags().GetBool("detail")
			allowPartial, _ := cmd.Flags().GetBool("allow-partial")
			failOnDrift, _ := cmd.Flags().GetBool("fail-on-drift")
			return a.runDiffWithOptions(cmd.Context(), opts, diffOptions{
				products:          products,
				resources:         resources,
				ignoreOperational: ignoreOperational,
				detail:            detail,
				allowPartial:      allowPartial,
				failOnDrift:       failOnDrift,
			}, positionals[0], positionals[1])
		},
	}
	cmd.Flags().String("products", "", "comma-separated products: zia,zpa")
	cmd.Flags().String("resources", "", "comma-separated resources: locations or zia/locations")
	cmd.Flags().Bool("ignore-operational", false, "ignore operational metadata on keyed and singleton resources")
	cmd.Flags().Bool("detail", false, "include record-level table details")
	cmd.Flags().Bool("allow-partial", false, "compare partial dumps instead of rejecting them")
	cmd.Flags().Bool("fail-on-drift", false, "exit 7 when drift is detected")
	return cmd
}
