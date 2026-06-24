package cli

// root_test.go - In-process tests for the Cobra root command.
//
// All tests use fake / dummy secrets. No real credentials appear anywhere in
// this file. The "password=FAKE-TEST-SECRET-DO-NOT-USE" string below is
// deliberately synthetic to exercise the redactor's secret_assignment rule.

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// testApp returns an App wired to in-memory buffers. stdoutTTY is false because
// bytes.Buffer is not a terminal — that is fine for all tests in this file.
func testApp(t *testing.T) (*App, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	var out, errBuf bytes.Buffer
	a := NewWithOptions(&out, &errBuf, nil, Options{})
	return a, &out, &errBuf
}

// TestExecuteRoot_HelpRedaction verifies that Cobra's help output routes through
// the redacting writer AND that Close flushes it to the real buffer.
//
// A dummy hidden subcommand carries a fake "password=FAKE-TEST-SECRET-DO-NOT-USE"
// token in its Long description. Running --help on that command should:
//
//  1. Produce non-empty output (the help text was written).
//  2. NOT contain the raw fake token (it was redacted before Close flushed it).
func TestExecuteRoot_HelpRedaction(t *testing.T) {
	a, out, _ := testApp(t)

	// A fake secret that matches the redactor's secret_assignment rule.
	// The "password=…" shape is detected by assignmentRules("secret_assignment").
	const fakeSecret = "FAKE-TEST-SECRET-DO-NOT-USE"
	const fakeAssignment = "password=" + fakeSecret

	root := newRootCmd(a)
	dummy := &cobra.Command{
		Use:    "dummycmd",
		Short:  "dummy command for testing",
		Long:   "Dummy command. Example credential: " + fakeAssignment,
		Hidden: true,
		RunE: func(_ *cobra.Command, _ []string) error {
			return nil
		},
	}
	root.AddCommand(dummy)

	ctx := context.Background()
	err := a.executeRoot(ctx, root, []string{"dummycmd", "--help"})
	if err != nil {
		t.Fatalf("executeRoot returned unexpected error: %v", err)
	}

	got := out.String()

	// (a) Help text was written and flushed.
	if got == "" {
		t.Fatal("stdout buffer is empty after --help: redact.NewWriter Close was not flushed, or SetOut was not called")
	}
	if !strings.Contains(got, "Dummy command") {
		t.Errorf("help output does not contain expected text; got:\n%s", got)
	}

	// (b) The raw fake token is absent — the redactor replaced it.
	if strings.Contains(got, fakeSecret) {
		t.Errorf("raw fake token %q found in help output: redactor did not run or did not flush\ngot:\n%s", fakeSecret, got)
	}
}

// TestExecuteRoot_FlagError_MapsToUsageError verifies SetFlagErrorFunc: passing
// an unknown flag to a subcommand must return an error that satisfies
// errors.Is(err, ErrUsage), which exitCodeForError in main.go maps to exit 2.
func TestExecuteRoot_FlagError_MapsToUsageError(t *testing.T) {
	a, _, _ := testApp(t)

	root := newRootCmd(a)
	dummy := &cobra.Command{
		Use:    "dummycmd",
		Hidden: true,
		RunE: func(_ *cobra.Command, _ []string) error {
			return nil
		},
	}
	root.AddCommand(dummy)

	ctx := context.Background()
	err := a.executeRoot(ctx, root, []string{"dummycmd", "--no-such-flag"})
	if err == nil {
		t.Fatal("expected an error for unknown flag, got nil")
	}
	if !errors.Is(err, ErrUsage) {
		t.Errorf("error %q should satisfy errors.Is(err, ErrUsage); SetFlagErrorFunc may not be firing", err)
	}
}

// TestExactArgs_MapsToUsageError verifies the exactArgs helper: a command whose
// Args validator is exactArgs(1) given 0 arguments must return an error that
// satisfies errors.Is(_, ErrUsage) so it exits 2, not 1.
func TestExactArgs_MapsToUsageError(t *testing.T) {
	a, _, _ := testApp(t)

	root := newRootCmd(a)
	dummy := &cobra.Command{
		Use:    "dummycmd <arg>",
		Hidden: true,
		Args:   exactArgs(1),
		RunE: func(_ *cobra.Command, _ []string) error {
			return nil
		},
	}
	root.AddCommand(dummy)

	ctx := context.Background()
	err := a.executeRoot(ctx, root, []string{"dummycmd"}) // 0 args, want 1
	if err == nil {
		t.Fatal("expected an error for wrong arg count, got nil")
	}
	if !errors.Is(err, ErrUsage) {
		t.Errorf("exactArgs(1) error %q should satisfy errors.Is(err, ErrUsage)", err)
	}
}

// TestRangeArgs_MapsToUsageError verifies the rangeArgs helper similarly.
func TestRangeArgs_MapsToUsageError(t *testing.T) {
	a, _, _ := testApp(t)

	root := newRootCmd(a)
	dummy := &cobra.Command{
		Use:    "dummycmd",
		Hidden: true,
		Args:   rangeArgs(1, 2),
		RunE: func(_ *cobra.Command, _ []string) error {
			return nil
		},
	}
	root.AddCommand(dummy)

	ctx := context.Background()
	err := a.executeRoot(ctx, root, []string{"dummycmd"}) // 0 args, want 1-2
	if err == nil {
		t.Fatal("expected an error for wrong arg count, got nil")
	}
	if !errors.Is(err, ErrUsage) {
		t.Errorf("rangeArgs(1,2) error %q should satisfy errors.Is(err, ErrUsage)", err)
	}
}

// TestPrefixMatchingOff_PackageVarSet asserts that cobra.EnablePrefixMatching is
// false after newRootCmd is called. This is the package-level variable guard: if
// a future dependency flips it to true, newRootCmd resets it.
func TestPrefixMatchingOff_PackageVarSet(t *testing.T) {
	a, _, _ := testApp(t)

	_ = newRootCmd(a)

	if cobra.EnablePrefixMatching {
		t.Error("cobra.EnablePrefixMatching should be false after newRootCmd; 'doc' must not alias 'doctor'")
	}
}

// TestPrefixMatchingOff_PrefixDoesNotResolve is the behavioural proxy confirming
// that prefix abbreviations do not resolve commands.
//
// The real root (newRootCmd) sets TraverseChildren=true, which routes lookup
// through cobra.Traverse rather than cobra.Find. Traverse returns the root when
// no child matches (not an error), so the decisive gate — EnablePrefixMatching —
// cannot be exercised via the real root's Execute path. This test therefore uses
// a minimal proxy root with TraverseChildren=false (the default) so that cobra
// routes through Find/findNext, where EnablePrefixMatching is the actual gate.
// We confirm the var is false (set by newRootCmd above) and that "dummy" does
// NOT silently resolve to "dummycmd". Execution is routed through executeRoot so
// that a future refactor of executeRoot is also exercised.
func TestPrefixMatchingOff_PrefixDoesNotResolve(t *testing.T) {
	a, _, errBuf := testApp(t)

	// Call newRootCmd to ensure EnablePrefixMatching is set to false (the guard).
	_ = newRootCmd(a)

	// Build a proxy root with TraverseChildren=false so Find is used.
	a2, _, _ := testApp(t)
	proxy := &cobra.Command{
		Use:           "root",
		SilenceErrors: true,
		SilenceUsage:  true,
		// TraverseChildren intentionally NOT set (false) so cobra uses Find.
	}
	proxy.AddCommand(&cobra.Command{
		Use:  "dummycmd",
		RunE: func(_ *cobra.Command, _ []string) error { return nil },
	})

	ctx := context.Background()
	err := a2.executeRoot(ctx, proxy, []string{"dummy"})
	if err == nil {
		t.Error("prefix match: 'dummy' should not resolve to 'dummycmd' when EnablePrefixMatching=false (TraverseChildren=false path)")
	}

	// errBuf from the first testApp is unused in this path; suppress the linter.
	_ = errBuf
}

// TestExecuteRoot_StderrRedaction verifies that executeRoot wraps a.err in a
// redact.NewWriter and that the stderr path is scanned before output reaches the
// real buffer. A dummy subcommand writes a fake high-entropy token to
// cmd.ErrOrStderr(); after executeRoot returns the captured err buffer must NOT
// contain the raw token (it was redacted) AND must be non-empty (Close flushed).
func TestExecuteRoot_StderrRedaction(t *testing.T) {
	a, _, errBuf := testApp(t)

	// A fake secret that matches the redactor's secret_assignment rule.
	const fakeSecret = "FAKE-TEST-SECRET-DO-NOT-USE"
	const fakeAssignment = "password=" + fakeSecret

	root := newRootCmd(a)
	dummy := &cobra.Command{
		Use:    "dummycmd",
		Hidden: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			// Write the fake assignment to the command's stderr writer. This goes
			// through the errW redact.NewWriter installed by executeRoot.
			_, _ = cmd.ErrOrStderr().Write([]byte(fakeAssignment + "\n"))
			return nil
		},
	}
	root.AddCommand(dummy)

	ctx := context.Background()
	err := a.executeRoot(ctx, root, []string{"dummycmd"})
	if err != nil {
		t.Fatalf("executeRoot returned unexpected error: %v", err)
	}

	got := errBuf.String()

	// (a) The err buffer must be non-empty — Close flushed the redactor.
	if got == "" {
		t.Fatal("err buffer is empty after writing to stderr: redact.NewWriter Close was not flushed, or SetErr was not called")
	}

	// (b) The raw fake token must be absent — the redactor replaced it.
	if strings.Contains(got, fakeSecret) {
		t.Errorf("raw fake token %q found in err buffer: stderr redactor did not run or did not flush\ngot:\n%s", fakeSecret, got)
	}
}

// TestHelpCommandMultiToken asserts that the Cobra `help` command accepts a
// multi-token path (e.g. `zscalerctl help config init`) and renders the
// target subcommand's help without loading credentials or config.
func TestHelpCommandMultiToken(t *testing.T) {
	a, out, _ := testApp(t)

	ctx := context.Background()
	err := a.Run(ctx, []string{"help", "config", "init"})
	if err != nil {
		t.Fatalf("App.Run(help config init) error = %v, want nil", err)
	}

	got := out.String()
	if got == "" {
		t.Fatal("help config init produced no output")
	}
	if !strings.Contains(got, "config init") {
		t.Errorf("help config init output does not contain %q; got:\n%s", "config init", got)
	}
}

// TestExecuteRoot_NoVersionFlag confirms that no --version flag is registered on
// the root (setting root.Version would add one, which is a surface change; the
// "version" subcommand is the correct path).
func TestExecuteRoot_NoVersionFlag(t *testing.T) {
	a, _, _ := testApp(t)
	root := newRootCmd(a)

	if f := root.Flags().Lookup("version"); f != nil {
		t.Error("root command must not register a --version flag; use the 'version' subcommand instead")
	}
	if root.Version != "" {
		t.Errorf("root.Version must be empty, got %q", root.Version)
	}
}
