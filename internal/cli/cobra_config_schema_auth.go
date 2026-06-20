package cli

import (
	"errors"
	"fmt"
	"io"
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
		// config init writes a LOCAL config file (os.MkdirAll +
		// fileperm.WriteOwnerOnly); it never mutates the Zscaler tenant. The
		// introspect/mutating annotation marks the local side effect so the
		// surface map reports it accurately — the CLI-wide read_only guarantee
		// is tenant-scoped, not "no side effects at all".
		Annotations: map[string]string{"introspect/mutating": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			// Reject extra positional args before any filesystem work.
			if cmd.Flags().NArg() != 0 {
				return UsageError{Message: "usage: zscalerctl config init [--force]"}
			}
			force, _ := cmd.Flags().GetBool("force")
			return a.runConfigInitWithForce(opts, force, a.out, a.err)
		},
	}
	cmd.Flags().Bool("force", false, "overwrite an existing config file")
	return cmd
}

// runConfigInitWithForce extracts the post-flag-parse logic from runConfigInit
// so it can be called from the Cobra RunE with an already-resolved force value.
//
// Both out and errW are the raw App writers (a.out / a.err). config-init is
// intentionally exempt from the Cobra-installed redacting writer for its path
// output: filesystem paths appear on both stdout (the machine-parseable path)
// and in the stderr hint ("Created owner-only config at …"), and the
// high-entropy redactor false-positives on entropy-heavy OS temp directories,
// producing "<REDACTED:SECRET>" instead of the real path. The config path is
// never a credential, so bypassing the redactor here is safe. If this function
// ever outputs anything credential-like it must be added to the redact package's
// test corpus instead.
func (a *App) runConfigInitWithForce(opts globalOptions, force bool, out, errW io.Writer) error {
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

	// Machine-first: the path goes to stdout (out = a.out, raw writer) so it
	// can be captured by scripts without redactor interference. The human
	// next-steps hints go to errW (a.err, the raw App writer) — config-init
	// intentionally bypasses the Cobra-installed redacting writer for both
	// streams (see the runConfigInitWithForce doc block above for rationale).
	fmt.Fprintln(out, path)
	fmt.Fprintf(errW, "Created owner-only config at %s\n", path)
	fmt.Fprintln(errW, "Next: set a client secret — export ZSCALERCTL_CLIENT_SECRET, or uncomment a client_secret_ref in the file.")
	fmt.Fprintln(errW, "Then run: zscalerctl doctor")
	return nil
}

// newConfigShowCmd returns the "config show" subcommand (config-LAZY).
//
// Cobra routing guarantees the "show" verb was matched; we pass args directly
// (normally empty for "zscalerctl config show") so runConfig receives only the
// post-verb positional args and can enforce len(args)==0 cleanly.
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
			return a.runConfig(cmd.Context(), cfg, opts, args)
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
// Cobra routing guarantees the "list" verb was matched; we pass args directly
// (normally empty) so runSchema receives only the post-verb positional args.
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
			return a.runSchema(cmd.Context(), cfg, opts, args)
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
// Cobra routing guarantees the "status" verb was matched; we pass args directly
// (normally empty) so runAuth receives only the post-verb positional args.
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
			return a.runAuth(cmd.Context(), cfg, opts, args)
		},
	}
}
