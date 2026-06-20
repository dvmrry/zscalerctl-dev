package cli

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/dvmrry/zscalerctl/internal/config"
	"github.com/dvmrry/zscalerctl/internal/fileperm"
	"github.com/spf13/cobra"
)

// newConfigCmd returns the Cobra "config" parent command with two subcommands:
//   - init: config-FREE; writes a starter config; has a local --force flag
//   - show: config-LAZY; renders the active config in redacted form
//
// The parent RunE returns UsageError (exit 2) for bare "config" or unknown
// subcommands ("config bogus"), listing the real subcommands in the message.
// This preserves the legacy exit-2 contract for bare/invalid config invocations.
func (a *App) newConfigCmd(opts globalOptions) *cobra.Command {
	parent := &cobra.Command{
		Use:   "config",
		Short: "manage zscalerctl configuration",
		// RunE fires for bare "config" or any unrecognised subcommand.
		// Return UsageError so exitCodeForError → exit 2 (same as legacy path).
		RunE: func(cmd *cobra.Command, args []string) error {
			return UsageError{Message: "usage: zscalerctl config <init|show>"}
		},
	}

	parent.AddCommand(a.newConfigInitCmd(opts), a.newConfigShowCmd(opts))
	return parent
}

// newConfigInitCmd returns the "config init" subcommand.
//
// Design notes:
//   - config-FREE: intentionally runs before LoadConfig because the target
//     file is expected not to exist yet.
//   - --force is a LOCAL flag on this subcommand only (not inherited).
//   - Format-agnostic: it writes text regardless of --format; ndjson is NOT
//     rejected here — format-agnostic means any --format is accepted (exit 0).
//   - stdout: the created path only (machine-parseable; --output f captures it).
//   - stderr: human next-steps hints.
//   - Arity: NArg()==0 enforced in RunE → UsageError (exit 2) for extra args.
func (a *App) newConfigInitCmd(opts globalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "write a starter config file with owner-only permissions",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Reject extra positional args before any filesystem work.
			if cmd.Flags().NArg() != 0 {
				return UsageError{Message: "usage: zscalerctl config init [--force]"}
			}
			force, _ := cmd.Flags().GetBool("force")
			return a.runConfigInitWithForce(opts, force)
		},
	}
	cmd.Flags().Bool("force", false, "overwrite an existing config file")
	return cmd
}

// runConfigInitWithForce extracts the post-flag-parse logic from runConfigInit
// so it can be called from the Cobra RunE with an already-resolved force value.
// The legacy runConfigInit (flag.FlagSet path) is left intact for now.
func (a *App) runConfigInitWithForce(opts globalOptions, force bool) error {
	path, _ := config.ResolveConfigPath(a.env, config.LoadOptions{
		Profile:    opts.profile,
		ConfigPath: opts.configPath,
	})

	switch _, statErr := os.Lstat(path); {
	case statErr == nil:
		if !force {
			return UsageError{Message: fmt.Sprintf("config already exists at %s; pass --force to overwrite", path)}
		}
		// WriteOwnerOnly is O_EXCL, so we remove before re-creating.
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("remove existing config %s: %w", path, err)
		}
	case errors.Is(statErr, fs.ErrNotExist):
		// Expected: nothing to overwrite.
	default:
		return fmt.Errorf("stat config path %s: %w", path, statErr)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create config directory %s: %w", filepath.Dir(path), err)
	}
	if err := fileperm.WriteOwnerOnly(path, []byte(configInitTemplate)); err != nil {
		return fmt.Errorf("write config %s: %w", path, err)
	}

	// Machine-first: the path goes to stdout so it can be captured; the human
	// next-steps hint goes to stderr so it never pollutes that path.
	fmt.Fprintln(a.out, path)
	fmt.Fprintf(a.err, "Created owner-only config at %s\n", path)
	fmt.Fprintln(a.err, "Next: set a client secret — export ZSCALERCTL_CLIENT_SECRET, or uncomment a client_secret_ref in the file.")
	fmt.Fprintln(a.err, "Then run: zscalerctl doctor")
	return nil
}

// newConfigShowCmd returns the "config show" subcommand (config-LAZY).
//
// runConfig already enforces args==["show"] on the legacy path. Now that "show"
// is a structural Cobra subcommand, the positional constraint is guaranteed by
// routing, so we call runConfig with an empty args slice. The runner's arg check
// is adapted: passing args directly from the cobra RunE (which will be empty for
// "zscalerctl config show") preserves identical behaviour.
func (a *App) newConfigShowCmd(opts globalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "show the active configuration (redacted)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadConfig(a.env, config.LoadOptions{
				Profile:    opts.profile,
				ConfigPath: opts.configPath,
			})
			if err != nil {
				return err
			}
			applyOptions(&cfg, opts)
			// Pass args from the subcommand (normally empty). runConfig's
			// legacy args[0]=="show" guard is no longer structurally needed,
			// but we satisfy it by prepending "show" so the runner is unchanged.
			return a.runConfig(cmd.Context(), cfg, opts, append([]string{"show"}, args...))
		},
	}
}

// newSchemaCmd returns the Cobra "schema" parent command with one subcommand:
//   - list: config-LAZY; enumerates the resource catalog
//
// The parent RunE returns UsageError (exit 2) for bare "schema" or unknown
// subcommands, listing the real subcommand in the message.
func (a *App) newSchemaCmd(opts globalOptions) *cobra.Command {
	parent := &cobra.Command{
		Use:   "schema",
		Short: "inspect the resource catalog schema",
		RunE: func(cmd *cobra.Command, args []string) error {
			return UsageError{Message: "usage: zscalerctl schema list"}
		},
	}
	parent.AddCommand(a.newSchemaListCmd(opts))
	return parent
}

// newSchemaListCmd returns the "schema list" subcommand (config-LAZY).
//
// runSchema already enforces args==["list"]. We satisfy the legacy check by
// prepending "list" so the runner body is unchanged.
func (a *App) newSchemaListCmd(opts globalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "list all catalog resources and their supported operations",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadConfig(a.env, config.LoadOptions{
				Profile:    opts.profile,
				ConfigPath: opts.configPath,
			})
			if err != nil {
				return err
			}
			applyOptions(&cfg, opts)
			return a.runSchema(cmd.Context(), cfg, opts, append([]string{"list"}, args...))
		},
	}
}

// newAuthCmd returns the Cobra "auth" parent command with one subcommand:
//   - status: config-LAZY; shows credential and auth-mode status
//
// The parent RunE returns UsageError (exit 2) for bare "auth" or unknown
// subcommands, listing the real subcommand in the message.
func (a *App) newAuthCmd(opts globalOptions) *cobra.Command {
	parent := &cobra.Command{
		Use:   "auth",
		Short: "inspect authentication configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			return UsageError{Message: "usage: zscalerctl auth status"}
		},
	}
	parent.AddCommand(a.newAuthStatusCmd(opts))
	return parent
}

// newAuthStatusCmd returns the "auth status" subcommand (config-LAZY).
//
// runAuth already enforces args==["status"]. We satisfy the legacy check by
// prepending "status" so the runner body is unchanged.
func (a *App) newAuthStatusCmd(opts globalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "show authentication status for the active profile",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadConfig(a.env, config.LoadOptions{
				Profile:    opts.profile,
				ConfigPath: opts.configPath,
			})
			if err != nil {
				return err
			}
			applyOptions(&cfg, opts)
			return a.runAuth(cmd.Context(), cfg, opts, append([]string{"status"}, args...))
		},
	}
}
