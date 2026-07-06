package cli

import (
	"fmt"
	"strings"

	"github.com/dvmrry/zscalerctl/internal/config"
	"github.com/dvmrry/zscalerctl/internal/resources"
	"github.com/spf13/cobra"
)

// newProductCmd returns a Cobra subcommand for the given product (e.g. "zia",
// "zpa", "ztw", "zcc"). All resource/op/id positional arguments are forwarded
// to runProduct, which enforces arity and produces the canonical usage messages.
//
// No restrictive cobra.Args validator is set: runProduct's own arity checks
// produce the canonical UsageError messages; a Cobra validator would fire first
// and change those messages.
//
// Config is loaded lazily inside RunE (same pattern as newDoctorCmd) so the
// no-credentials path (exit 3) is preserved for product commands: they load
// config and then attempt to build a reader, which fails when credentials are
// absent.
//
// Help (SetHelpFunc): when the first positional arg is a known catalog resource
// for this product, the help func prints the resource-specific field/usage block
// (resourceUsage) instead of Cobra's default product help. This preserves the
// contract where `zia locations --help` and `zia locations list --help`
// print the resource's supported ops and renderable field names.
//
// Completion (ValidArgsFunction): the first positional completion returns
// catalog resource names; the second returns the resource's supported read ops.
// SECURITY: the ValidArgsFunction reads ONLY the static catalog — it never loads
// config, resolves secrets, or dials the API.
func (a *App) newProductCmd(product resources.Product, opts globalOptions) *cobra.Command {
	catalog := a.resourceCatalog()

	cmd := &cobra.Command{
		Use:   string(product),
		Short: "read " + string(product) + " resources",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadConfig(a.env, config.LoadOptions{
				Profile:    opts.profile,
				ConfigPath: opts.configPath,
			})
			if err != nil {
				return err
			}
			applyOptions(&cfg, opts)
			return a.runProduct(cmd.Context(), cfg, opts, string(product), args)
		},
		ValidArgsFunction: func(_ *cobra.Command, args []string, _ string) ([]cobra.Completion, cobra.ShellCompDirective) {
			// SECURITY: reads only the static catalog — never loads config or dials API.
			switch len(args) {
			case 0:
				// First positional: offer the product's resource names.
				names := a.completionResourceNames(product)
				completions := make([]cobra.Completion, len(names))
				for i, n := range names {
					completions[i] = cobra.Completion(n)
				}
				return completions, cobra.ShellCompDirectiveNoFileComp
			case 1:
				// Second positional: offer the ops that this resource supports.
				spec, ok := catalog.FindSpec(product, args[0])
				if !ok {
					return nil, cobra.ShellCompDirectiveNoFileComp
				}
				ops := readOperationNames(spec)
				completions := make([]cobra.Completion, len(ops))
				for i, op := range ops {
					completions[i] = cobra.Completion(op)
				}
				return completions, cobra.ShellCompDirectiveNoFileComp
			default:
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
		},
	}

	// SetHelpFunc: intercept --help when the first positional is a known
	// catalog resource and print resource-specific help instead of Cobra's
	// default product help. Falls back to Cobra default for `zia --help`.
	//
	// Cobra's execute() parses flags before checking helpVal, so by the time
	// the help func fires, cmd.Flags().Args() is populated: it contains the
	// positional args (e.g. ["locations"] or ["locations", "list"]) stripped of
	// any flags. We use it as the reliable source for the resource name.
	cmd.SetHelpFunc(func(c *cobra.Command, args []string) {
		positionals := c.Flags().Args()
		if len(positionals) >= 1 {
			if spec, ok := catalog.FindSpec(product, positionals[0]); ok {
				fmt.Fprintln(c.OutOrStdout(), newHelpRenderer(a.style(opts)).renderResourceUsage(product, spec, 0))
				return
			}
		}
		if err := newHelpRenderer(a.style(opts)).writeHelp(c.OutOrStdout(), c); err != nil {
			c.PrintErrln(err)
		}
	})

	// url-lookup is a ZIA-only diagnostic verb (not a catalog resource). Wire it
	// as a Cobra subcommand so it owns its own help surface and uses
	// DisableFlagParsing to preserve its strict no-flags error message.
	if product == resources.ProductZIA {
		cmd.AddCommand(a.newURLLookupCmd(opts))
	}
	return cmd
}

// newURLLookupCmd returns the "url-lookup" subcommand of the "zia" product
// command. DisableFlagParsing is set so that all trailing tokens — including
// anything that looks like a flag — are forwarded raw to RunE and then to
// runURLLookup, which enforces its own strict rejection of args starting with
// "-". Without this, Cobra would intercept an unknown flag such as "--bogus"
// and emit a generic "unknown flag" error before RunE fires, losing the
// url-lookup-specific message.
//
// Help handling: with DisableFlagParsing, Cobra cannot intercept "-h"/"--help"
// automatically. RunE detects any help token in args and calls cmd.Help() so
// the user still gets the help text rather than the flag-rejection message.
func (a *App) newURLLookupCmd(opts globalOptions) *cobra.Command {
	return &cobra.Command{
		Use:                urlLookupCommandName + " <url> [url...]",
		Short:              "look up URL categories for one or more URLs",
		DisableFlagParsing: true,
		Annotations: map[string]string{
			// Use suffix "<url> [url...]" would be inferred as arbitrary by
			// buildArgsDoc; the annotation makes the real constraint explicit.
			"introspect/args-policy": "at_least:1",
			// url-lookup emits structured JSON; record the field order so
			// buildSingleCommandDoc can populate OutputFields.
			"introspect/output-fields": strings.Join(urlLookupFieldOrder, ","),
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// DisableFlagParsing means --help arrives as a raw arg; handle it
			// before runURLLookup's "-" check fires and rejects it.
			for _, arg := range args {
				if arg == "-h" || arg == "--help" {
					return cmd.Help()
				}
			}
			cfg, err := config.LoadConfig(a.env, config.LoadOptions{
				Profile:    opts.profile,
				ConfigPath: opts.configPath,
			})
			if err != nil {
				return err
			}
			applyOptions(&cfg, opts)
			return a.runURLLookup(cmd.Context(), cfg, opts, args)
		},
	}
}
