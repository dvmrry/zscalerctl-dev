package cli

// root.go — Cobra root command, redacting execute helper, and error-mapping
// utilities for the phased Cobra migration.
//
// # Architecture note
//
// This file does NOT wire the root command into App.Run. That is Task 1.4.
// Today, App.Run still delegates entirely to the legacy flag-based dispatch.
// newRootCmd + executeRoot exist so the root can be constructed and tested in
// isolation before the App.Run plumbing lands.
//
// # No-leak contract
//
// The real no-leak / exit-code boundary lives in cmd/zscalerctl/main.go:run().
// Every byte of Cobra output (help text, error messages) must flow through a
// redact.NewWriter so it is scanned before reaching the user. executeRoot wraps
// both writers and calls Close after Execute to flush the full-buffer redactor.

import (
	"context"

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

	// Disable the auto-generated "help" subcommand. We install a hidden no-op in
	// its place so Cobra's internal AddCommand("help") guard does not re-add one.
	// This avoids a conflict with the legacy "help" token handling in App.Run.
	// The --help FLAG is NOT affected by this; Cobra adds it separately and it
	// remains fully functional.
	root.SetHelpCommand(&cobra.Command{
		Use:    "no-op-help",
		Hidden: true,
		Run:    func(*cobra.Command, []string) {},
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
//	wrapping it into UsageError (exit 2) must happen at the App.Run call site
//	(Task 1.4) by inspecting the returned error string after executeRoot returns.
func (a *App) executeRoot(ctx context.Context, root *cobra.Command, args []string) error {
	outW := redact.NewWriter(a.out, redact.ModeStandard)
	errW := redact.NewWriter(a.err, redact.ModeStandard)
	// Use defers so both writers flush even on panic.
	defer func() { _ = outW.Close() }()
	defer func() { _ = errW.Close() }()

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
func (a *App) executeRootCompletion(ctx context.Context, root *cobra.Command, args []string) error {
	// stdout: raw writer — the generated script bytes must be emitted exactly.
	// stderr: still redacted — errors may echo user input.
	errW := redact.NewWriter(a.err, redact.ModeStandard)
	defer func() { _ = errW.Close() }()

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
