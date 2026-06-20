package cli

// root_test.go — in-process tests for the Cobra root command (Task 1.3).
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

	"github.com/dvmrry/zscalerctl/internal/redact"
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

	root := newRootCmd(a, globalOptions{})
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

	root := newRootCmd(a, globalOptions{})
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

	root := newRootCmd(a, globalOptions{})
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

	root := newRootCmd(a, globalOptions{})
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

// TestPrefixMatchingOff verifies that cobra.EnablePrefixMatching is false after
// newRootCmd is called, and that a prefix abbreviation does not resolve when
// the root uses Find (TraverseChildren=false) so the full cobra.Find path is
// exercised.
//
// Note: the root built by newRootCmd uses TraverseChildren=true, which routes
// command lookup through cobra.Traverse rather than cobra.Find. Traverse returns
// the root (not an error) when no child matches, so the behavioural check must
// use a separate minimal root with TraverseChildren=false where Find is used and
// EnablePrefixMatching is the decisive gating variable.
func TestPrefixMatchingOff(t *testing.T) {
	a, _, _ := testApp(t)

	_ = newRootCmd(a, globalOptions{})

	// Package-level var must be false after newRootCmd.
	if cobra.EnablePrefixMatching {
		t.Error("cobra.EnablePrefixMatching should be false after newRootCmd; 'doc' must not alias 'doctor'")
	}

	// Behavioural check using a minimal root with TraverseChildren=false so that
	// cobra uses Find (which gates on EnablePrefixMatching) not Traverse.
	var outBuf, errBuf bytes.Buffer
	a2 := NewWithOptions(&outBuf, &errBuf, nil, Options{})

	root2 := &cobra.Command{
		Use:          "root",
		SilenceErrors: true,
		SilenceUsage:  true,
		// TraverseChildren intentionally NOT set (false) so Find is used.
	}
	root2.AddCommand(&cobra.Command{
		Use:   "dummycmd",
		RunE:  func(_ *cobra.Command, _ []string) error { return nil },
	})

	outW := redact.NewWriter(a2.out, redact.ModeStandard)
	errW := redact.NewWriter(a2.err, redact.ModeStandard)
	defer func() { _ = outW.Close() }()
	defer func() { _ = errW.Close() }()
	root2.SetOut(outW)
	root2.SetErr(errW)
	root2.SetArgs([]string{"dummy"})

	ctx := context.Background()
	err := root2.ExecuteContext(ctx)
	if err == nil {
		t.Error("prefix match: 'dummy' should not resolve to 'dummycmd' when EnablePrefixMatching=false (TraverseChildren=false path)")
	}
}

// TestExecuteRoot_NoVersionFlag confirms that no --version flag is registered on
// the root (setting root.Version would add one, which is a surface change; the
// "version" subcommand is the correct path).
func TestExecuteRoot_NoVersionFlag(t *testing.T) {
	a, _, _ := testApp(t)
	root := newRootCmd(a, globalOptions{})

	if f := root.Flags().Lookup("version"); f != nil {
		t.Error("root command must not register a --version flag; use the 'version' subcommand instead")
	}
	if root.Version != "" {
		t.Errorf("root.Version must be empty, got %q", root.Version)
	}
}
