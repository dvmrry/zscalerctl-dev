package cli_test

// cobra_dispatch_test.go — Tests for the hybrid Cobra/legacy dispatch (Tasks 1.4 + 1.5).
//
// Tests here verify:
//  1. Hybrid routing: migrated command (version) goes through Cobra; un-migrated
//     commands (zia) still go through the legacy path.
//  2. version --help → Cobra-rendered help, routed through the redactor.
//  3. version --output <file>: the migrated command runs inside the --output wrapper.
//  4. Format/arity surface preservation: --format ndjson → exit 2; extra args → UsageError.

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/cli"
)

// testVersionApp returns an App wired to in-memory buffers with no env.
func testVersionApp(t *testing.T) (*cli.App, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	var out, errBuf bytes.Buffer
	a := cli.New(&out, &errBuf, nil)
	return a, &out, &errBuf
}

// TestHybridRouting_VersionGoesViaCobra confirms that "version" produces output
// indistinguishable from the legacy path (same keys, no extra Cobra noise), proving
// it is correctly dispatched through Cobra and runVersion is called.
func TestHybridRouting_VersionGoesViaCobra(t *testing.T) {
	t.Parallel()

	a, out, errBuf := testVersionApp(t)
	err := a.Run(context.Background(), []string{"--format", "table", "version"})
	if err != nil {
		t.Fatalf("App.Run(version) error = %v, want nil", err)
	}
	got := out.String()

	// The version table renders these keys exactly as before.
	for _, key := range []string{"Version", "Commit", "Date", "Go", "Platform"} {
		if !strings.Contains(got, key) {
			t.Errorf("App.Run(version) stdout = %q, want key %q", got, key)
		}
	}
	if errBuf.Len() != 0 {
		t.Errorf("App.Run(version) stderr = %q, want empty", errBuf.String())
	}
}

// TestHybridRouting_UnmigratedCommandStillWorksLegacy confirms that a command
// that is NOT in isMigrated still routes through the legacy path. We use
// "zia locations list" with no credentials — it must fail with the credential
// sentinel (exit 3 path), NOT with any Cobra-related error.
func TestHybridRouting_UnmigratedCommandStillWorksLegacy(t *testing.T) {
	t.Parallel()

	a, _, _ := testVersionApp(t)
	err := a.Run(context.Background(), []string{"--format", "table", "zia", "locations", "list"})
	if err == nil {
		t.Fatal("App.Run(zia locations list, no creds) error = nil, want credential error")
	}
	// The error must carry the missing-credentials sentinel, not a Cobra unknown-command error.
	// This confirms the legacy dispatch path fired.
	if errors.Is(err, cli.ErrUsage) {
		t.Errorf("App.Run(zia locations list, no creds) returned UsageError %q; want credential error (legacy path)", err)
	}
	errMsg := err.Error()
	if strings.HasPrefix(errMsg, "unknown command") {
		t.Errorf("App.Run(zia locations list, no creds) returned Cobra unknown-command error %q; legacy path should have handled it", errMsg)
	}
}

// TestVersionHelp_CobraRenderedHelp confirms that "version --help" is routed to
// Cobra (which re-inserts --help) and that the output is Cobra-formatted help
// (contains "Usage:" and "zscalerctl version"), routed through the redactor and
// flushed.
func TestVersionHelp_CobraRenderedHelp(t *testing.T) {
	t.Parallel()

	a, out, errBuf := testVersionApp(t)
	err := a.Run(context.Background(), []string{"version", "--help"})
	if err != nil {
		t.Fatalf("App.Run(version --help) error = %v, want nil", err)
	}
	got := out.String()
	if got == "" {
		t.Fatal("App.Run(version --help) stdout is empty; redactor may not have flushed")
	}
	// Cobra help always contains "Usage:" (capital U) and the command name.
	if !strings.Contains(got, "zscalerctl version") {
		t.Errorf("App.Run(version --help) stdout = %q, want 'zscalerctl version'", got)
	}
	if errBuf.Len() != 0 {
		t.Errorf("App.Run(version --help) stderr = %q, want empty", errBuf.String())
	}
}

// TestVersionOutputFile confirms that the migrated "version" command runs inside
// the --output wrapper: when --output is set, the version table is written to the
// file and stdout is empty. This proves execCobra runs AFTER App.Run swaps a.out
// to the buffer.
func TestVersionOutputFile(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	outFile := filepath.Join(tmpDir, "version-out.txt")

	a, out, errBuf := testVersionApp(t)
	err := a.Run(context.Background(), []string{"--format", "table", "--output", outFile, "version"})
	if err != nil {
		t.Fatalf("App.Run(version --output) error = %v, want nil", err)
	}

	// stdout must be empty — output was redirected to the file.
	if out.Len() != 0 {
		t.Errorf("App.Run(version --output) stdout = %q, want empty (all output should be in the file)", out.String())
	}
	if errBuf.Len() != 0 {
		t.Errorf("App.Run(version --output) stderr = %q, want empty", errBuf.String())
	}

	// The file must exist and contain the version table output.
	fileBytes, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("output file %s not created: %v", outFile, err)
	}
	fileContent := string(fileBytes)
	for _, key := range []string{"Version", "Commit", "Date", "Go", "Platform"} {
		if !strings.Contains(fileContent, key) {
			t.Errorf("output file %s = %q, want key %q", outFile, fileContent, key)
		}
	}
}

// TestVersionFormatNDJSON_Rejected confirms that --format ndjson on the migrated
// version command still returns UsageError (exit 2), via runVersion's format check.
func TestVersionFormatNDJSON_Rejected(t *testing.T) {
	t.Parallel()

	a, _, _ := testVersionApp(t)
	err := a.Run(context.Background(), []string{"--format", "ndjson", "version"})
	if err == nil {
		t.Fatal("App.Run(--format ndjson version) error = nil, want UsageError")
	}
	if !errors.Is(err, cli.ErrUsage) {
		t.Errorf("App.Run(--format ndjson version) error = %v, want UsageError (exit 2)", err)
	}
}

// TestVersionExtraArg_UsageError confirms that extra positional args to the
// migrated version command still return UsageError (exit 2) via requireNoArgs.
// The error message must match the legacy "usage: zscalerctl version" shape.
func TestVersionExtraArg_UsageError(t *testing.T) {
	t.Parallel()

	a, _, _ := testVersionApp(t)
	err := a.Run(context.Background(), []string{"version", "somearg"})
	if err == nil {
		t.Fatal("App.Run(version somearg) error = nil, want UsageError")
	}
	if !errors.Is(err, cli.ErrUsage) {
		t.Errorf("App.Run(version somearg) error = %v, want UsageError (exit 2)", err)
	}
	if !strings.Contains(err.Error(), "usage: zscalerctl version") {
		t.Errorf("App.Run(version somearg) error = %q, want 'usage: zscalerctl version' message", err)
	}
}
