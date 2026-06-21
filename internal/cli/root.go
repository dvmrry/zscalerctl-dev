package cli

// root.go — Cobra root command, redacting execute helper, and error-mapping
// utilities for the Cobra migration.
//
// # Architecture note
//
// execCobra in app.go is the wire point that routes dispatch to Cobra.
// newRootCmd builds the full command tree; executeRoot / executeRootCompletion
// wrap both writers through the redactor and drive ExecuteContext.
//
// # No-leak contract
//
// The real no-leak / exit-code boundary lives in cmd/zscalerctl/main.go:run().
// Every byte of Cobra output (help text, error messages) must flow through a
// redact.NewWriter so it is scanned before reaching the user. executeRoot wraps
// both writers and calls Close after Execute to flush the full-buffer redactor.

import (
	"context"
	"fmt"

	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/spf13/cobra"
)

// Prefix matching is disabled process-wide so an abbreviation like "doc" never
// silently aliases "doctor". cobra.EnablePrefixMatching is a package-level var;
// writing it inside newRootCmd (which runs on every dispatch) races under the
// parallel test suite, so it is set ONCE here at package load. Its default is
// already false — this is the race-free guard against a future dependency
// flipping it (B-1 from the adversarial review).
func init() {
	cobra.EnablePrefixMatching = false
}

// newRootCmd constructs the Cobra root command with the §8 settings required by
// the migration spec. It does not add any product subcommands; those are added
// by later phases.
func newRootCmd(a *App) *cobra.Command {
	root := &cobra.Command{
		// Use is intentionally empty — the root is never invoked directly.
		Use: "zscalerctl",

		// SilenceErrors: the App.Run → main.go boundary renders errors itself via
		// writeError; we don't want Cobra printing a second copy on stderr.
		SilenceErrors: true,

		// SilenceUsage: prevent Cobra dumping the usage block on every RunE error.
		// Usage is emitted selectively by the legacy App.writeUsageForHumans path.
		SilenceUsage: true,

		// TraverseChildren: allows the mirrored global persistent flags (--format,
		// --profile, etc.) to appear between a product token and a resource/verb
		// without Cobra failing. App.Run strips them via splitGlobalArgs before
		// passing args to Cobra, but TraverseChildren ensures Cobra's own flag
		// scanning does not reject interleaved globals during help rendering.
		TraverseChildren: true,

		// SuggestionsMinimumDistance: levenshtein distance for "did you mean X?"
		// suggestions. 2 is the cobra default (set here explicitly for clarity).
		SuggestionsMinimumDistance: 2,

		// RunE: latent guard against unknown top-level commands. Today execCobra is
		// only called when isMigrated(rest[0]) is true, so this RunE cannot fire for
		// any valid dispatch path — it is a forward-compatibility guard for the moment
		// Cobra owns the full root. TraverseChildren=true means that without this, an
		// unknown command would fall through to the root and print help (exit 0);
		// with this, it exits 2 via UsageError (M-9 from the adversarial review).
		//
		// INVARIANT: this must NOT change any current behaviour. Bare "zscalerctl"
		// (empty args) goes through the legacy empty-rest path in runParsed and never
		// reaches Cobra. Unknown commands exit 2 via the legacy isKnownCommand path.
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			return UsageError{Message: unknownCommandMessage(args[0])}
		},
	}

	// SetFlagErrorFunc maps pflag parse errors to UsageError so they exit 2 via
	// the main.go exitCodeForError path. Without this, a bad flag returns a plain
	// error that exitCodeForError maps to exit 1.
	//
	// Note: do NOT use cobra.MarkFlagRequired or flag-groups anywhere in the tree.
	// Their validation runs through a different internal Cobra path that bypasses
	// SetFlagErrorFunc, so required/exclusive flag checks must be done inside
	// RunE/PreRunE and return UsageError explicitly.
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return UsageError{Message: err.Error()}
	})

	// Register the mirrored global persistent flags so --help output and shell
	// completion see them. Cobra never parses these; App.Run strips globals before
	// dispatching via splitGlobalArgs.
	applyGlobalPersistentFlags(root)

	// Note: root.Version is intentionally NOT set. Setting it would add a --version
	// flag, which is a surface change; the "version" subcommand is the correct path.

	return root
}

// executeRoot wraps both App writers in redact.NewWriter, sets them on the
// command, runs ExecuteContext, and then flushes the redactors by calling Close.
//
// Writer lifecycle:
//
//	outW and errW accumulate all Cobra output (help text, error messages) in
//	memory. On Close the full buffer is scanned by the redactor before being
//	written to the real destination. Close MUST happen after Execute; if it were
//	deferred before Execute returns the buffer would be empty. We use explicit
//	defers below so both redactors are closed even if Execute panics.
//
// Unknown-command wrapping (NOT done here):
//
//	When Cobra cannot find a subcommand, ExecuteContext returns a plain string
//	error ("unknown command X for Y"). That error is NOT a UsageError here —
//	wrapping it into UsageError (exit 2) happens in execCobra (app.go) by
//	inspecting the returned error string after executeRoot returns.
func (a *App) executeRoot(ctx context.Context, root *cobra.Command, args []string) (err error) {
	outW := redact.NewWriter(a.out, redact.ModeStandard)
	errW := redact.NewWriter(a.err, redact.ModeStandard)
	// Deferred Close flushes both redactors even on panic. When ExecuteContext
	// succeeds (err == nil), a non-nil Close error surfaces so a failed flush
	// does not silently become exit 0. When ExecuteContext already returned an
	// error, Close errors are suppressed — the execute error takes precedence.
	defer func() {
		cerrOut := outW.Close()
		cerrErr := errW.Close()
		if err == nil {
			if cerrOut != nil {
				err = cerrOut
			} else if cerrErr != nil {
				err = cerrErr
			}
		}
	}()

	root.SetOut(outW)
	root.SetErr(errW)
	root.SetArgs(args)

	return root.ExecuteContext(ctx)
}

// executeRootCompletion is like executeRoot but bypasses the redactor on stdout.
// It must be used for completion paths ("completion", "__complete",
// "__completeNoDesc") where Cobra emits a generated shell script or completion
// candidates on stdout. The high-entropy redactor heuristic false-positives on
// shell variable assignments such as "local shellCompDirectiveFilterFileExt=8",
// corrupting the script and breaking file-extension filtering in the shell.
//
// Safety: completion output is derived entirely from the command tree and the
// static resource catalog — it never contains runtime credential values (proven
// by TestCompletionScriptsDoNotReadCredentialFilesOrUseReader). Bypassing the
// redactor on stdout cannot leak secrets; it only prevents script corruption.
// Stderr remains redacted because completion errors (if any) could in theory
// echo back user-supplied tokens.
func (a *App) executeRootCompletion(ctx context.Context, root *cobra.Command, args []string) (err error) {
	// stdout: raw writer — the generated script bytes must be emitted exactly.
	// stderr: still redacted — errors may echo user input.
	errW := redact.NewWriter(a.err, redact.ModeStandard)
	// Deferred Close flushes the stderr redactor even on panic. Surface the
	// Close error when ExecuteContext succeeded (same rationale as executeRoot).
	defer func() {
		cerrErr := errW.Close()
		if err == nil && cerrErr != nil {
			err = cerrErr
		}
	}()

	root.SetOut(a.out)
	root.SetErr(errW)
	root.SetArgs(args)

	return root.ExecuteContext(ctx)
}

// exactArgs returns a cobra.PositionalArgs validator that requires exactly n
// positional arguments. Failures are wrapped in UsageError so exitCodeForError
// maps them to exit 2, not exit 1.
//
// Use this instead of cobra.ExactArgs: the built-in helper returns a plain
// error, which exitCodeForError maps to exit 1.
func exactArgs(n int) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) == n {
			return nil
		}
		return UsageError{Message: cobra.ExactArgs(n)(cmd, args).Error()}
	}
}

// rangeArgs returns a cobra.PositionalArgs validator that requires between min
// and max positional arguments (inclusive). Failures are wrapped in UsageError
// so exitCodeForError maps them to exit 2.
//
// Use this instead of cobra.RangeArgs for the same reason as exactArgs.
func rangeArgs(min, max int) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) >= min && len(args) <= max {
			return nil
		}
		return UsageError{Message: cobra.RangeArgs(min, max)(cmd, args).Error()}
	}
}

// setExactArgs wires exactArgs(n) as cmd.Args validator and sets the
// "introspect/args-policy" annotation to "exact:N" in one call so they
// cannot drift apart.
func setExactArgs(cmd *cobra.Command, n int) {
	cmd.Args = exactArgs(n)
	if cmd.Annotations == nil {
		cmd.Annotations = map[string]string{}
	}
	cmd.Annotations["introspect/args-policy"] = fmt.Sprintf("exact:%d", n)
}
