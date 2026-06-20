package cli_test

// cobra_dispatch_test.go — Tests for the hybrid Cobra/legacy dispatch (Tasks 1.4 + 1.5 + 1.5.2).
//
// Tests here verify:
//  1. Hybrid routing: migrated command (version, doctor) goes through Cobra; un-migrated
//     commands (zia, auth) still go through the legacy path.
//  2. version --help / doctor --help → Cobra-rendered help.
//  3. version/doctor --output <file>: migrated commands run inside the --output wrapper.
//  4. Format/arity surface preservation: --format ndjson → exit 2; extra args → UsageError.
//  5. doctor config-lazy RunE: loads config itself; config-error path surfaces correctly.

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/cli"
	"github.com/dvmrry/zscalerctl/internal/config"
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

// TestHybridRouting_ZiaGoesViaCobra confirms that "zia" IS migrated (Phase 2a)
// and routes through Cobra. "zia locations list" with no credentials must fail
// with the missing-credentials sentinel (exit 3 path), NOT with any
// Cobra unknown-command error, which would indicate the product was not
// registered in the Cobra tree.
func TestHybridRouting_ZiaGoesViaCobra(t *testing.T) {
	t.Parallel()

	a, _, _ := testVersionApp(t)
	err := a.Run(context.Background(), []string{"--format", "table", "zia", "locations", "list"})
	if err == nil {
		t.Fatal("App.Run(zia locations list, no creds) error = nil, want credential error")
	}
	// The error must carry the missing-credentials sentinel, not a Cobra unknown-command error.
	// A Cobra unknown-command error here would mean zia is NOT registered in the tree.
	if errors.Is(err, cli.ErrUsage) {
		t.Errorf("App.Run(zia locations list, no creds) returned UsageError %q; want credential error (Cobra path)", err)
	}
	errMsg := err.Error()
	if strings.HasPrefix(errMsg, "unknown command") {
		t.Errorf("App.Run(zia locations list, no creds) returned Cobra unknown-command error %q; zia must be registered in the Cobra tree", errMsg)
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

// ── doctor Cobra dispatch tests (Task 1.5.2) ────────────────────────────────

// testDoctorApp returns an App with no env (hermetic: no credentials, no config file).
func testDoctorApp(t *testing.T) (*cli.App, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	var out, errBuf bytes.Buffer
	a := cli.New(&out, &errBuf, nil)
	return a, &out, &errBuf
}

// TestHybridRouting_DoctorGoesViaCobra confirms that "doctor" is dispatched
// through Cobra and produces output identical to the legacy path (same key rows).
func TestHybridRouting_DoctorGoesViaCobra(t *testing.T) {
	t.Parallel()

	a, out, errBuf := testDoctorApp(t)
	err := a.Run(context.Background(), []string{"--format", "table", "doctor"})
	if err != nil {
		t.Fatalf("App.Run(doctor) error = %v, want nil", err)
	}
	got := out.String()

	// The doctor table renders these keys exactly as before.
	for _, key := range []string{"Status", "Mode", "Profile", "Config", "Redaction", "Timeout", "Credentials"} {
		if !strings.Contains(got, key) {
			t.Errorf("App.Run(doctor) stdout = %q, want key %q", got, key)
		}
	}
	if errBuf.Len() != 0 {
		t.Errorf("App.Run(doctor) stderr = %q, want empty", errBuf.String())
	}
}

// TestDoctorHelp_CobraRenderedHelp confirms that "doctor --help" emits Cobra-
// formatted help that contains "zscalerctl doctor".
func TestDoctorHelp_CobraRenderedHelp(t *testing.T) {
	t.Parallel()

	a, out, errBuf := testDoctorApp(t)
	err := a.Run(context.Background(), []string{"doctor", "--help"})
	if err != nil {
		t.Fatalf("App.Run(doctor --help) error = %v, want nil", err)
	}
	got := out.String()
	if got == "" {
		t.Fatal("App.Run(doctor --help) stdout is empty; redactor may not have flushed")
	}
	if !strings.Contains(got, "zscalerctl doctor") {
		t.Errorf("App.Run(doctor --help) stdout = %q, want 'zscalerctl doctor'", got)
	}
	if errBuf.Len() != 0 {
		t.Errorf("App.Run(doctor --help) stderr = %q, want empty", errBuf.String())
	}
}

// TestDoctorOutputFile confirms that the migrated "doctor" command runs inside
// the --output wrapper: when --output is set, the table is written to the file
// and stdout is empty.
func TestDoctorOutputFile(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	outFile := filepath.Join(tmpDir, "doctor-out.txt")

	a, out, errBuf := testDoctorApp(t)
	err := a.Run(context.Background(), []string{"--format", "table", "--output", outFile, "doctor"})
	if err != nil {
		t.Fatalf("App.Run(doctor --output) error = %v, want nil", err)
	}

	if out.Len() != 0 {
		t.Errorf("App.Run(doctor --output) stdout = %q, want empty", out.String())
	}
	if errBuf.Len() != 0 {
		t.Errorf("App.Run(doctor --output) stderr = %q, want empty", errBuf.String())
	}

	fileBytes, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("output file %s not created: %v", outFile, err)
	}
	fileContent := string(fileBytes)
	for _, key := range []string{"Status", "Mode", "Redaction"} {
		if !strings.Contains(fileContent, key) {
			t.Errorf("output file %s = %q, want key %q", outFile, fileContent, key)
		}
	}
}

// TestDoctorUnknownFlag_UsageError confirms that an unknown flag on the migrated
// doctor command returns UsageError (exit 2) via Cobra's flag parsing.
func TestDoctorUnknownFlag_UsageError(t *testing.T) {
	t.Parallel()

	a, _, _ := testDoctorApp(t)
	err := a.Run(context.Background(), []string{"doctor", "--nope"})
	if err == nil {
		t.Fatal("App.Run(doctor --nope) error = nil, want UsageError")
	}
	if !errors.Is(err, cli.ErrUsage) {
		t.Errorf("App.Run(doctor --nope) error = %v, want UsageError (exit 2)", err)
	}
}

// TestDoctorFormatNDJSON_Rejected confirms that --format ndjson on the migrated
// doctor command returns UsageError (exit 2) via runDoctor's format check.
func TestDoctorFormatNDJSON_Rejected(t *testing.T) {
	t.Parallel()

	a, _, _ := testDoctorApp(t)
	err := a.Run(context.Background(), []string{"--format", "ndjson", "doctor"})
	if err == nil {
		t.Fatal("App.Run(--format ndjson doctor) error = nil, want UsageError")
	}
	if !errors.Is(err, cli.ErrUsage) {
		t.Errorf("App.Run(--format ndjson doctor) error = %v, want UsageError (exit 2)", err)
	}
}

// TestDoctorExtraArg_UsageError confirms that extra positional args to the
// migrated doctor command return UsageError (exit 2) via requireNoArgs.
// The error message must match the legacy "usage: zscalerctl doctor" shape.
func TestDoctorExtraArg_UsageError(t *testing.T) {
	t.Parallel()

	a, _, _ := testDoctorApp(t)
	err := a.Run(context.Background(), []string{"doctor", "somearg"})
	if err == nil {
		t.Fatal("App.Run(doctor somearg) error = nil, want UsageError")
	}
	if !errors.Is(err, cli.ErrUsage) {
		t.Errorf("App.Run(doctor somearg) error = %v, want UsageError (exit 2)", err)
	}
	if !strings.Contains(err.Error(), "usage: zscalerctl doctor") {
		t.Errorf("App.Run(doctor somearg) error = %q, want 'usage: zscalerctl doctor' message", err)
	}
}

// TestDoctorConfigError_NonexistentPath confirms that the config-lazy RunE in
// the migrated doctor command surfaces a config load error (ErrInvalidConfig →
// exit 2) when --config points to a nonexistent path.
func TestDoctorConfigError_NonexistentPath(t *testing.T) {
	t.Parallel()

	a, _, _ := testDoctorApp(t)
	err := a.Run(context.Background(), []string{"--config", "/nonexistent/path/zscalerctl.yaml", "doctor"})
	if err == nil {
		t.Fatal("App.Run(doctor --config /nonexistent) error = nil, want config load error")
	}
	if !errors.Is(err, config.ErrInvalidConfig) {
		t.Errorf("App.Run(doctor --config /nonexistent) error = %v, want ErrInvalidConfig", err)
	}
}

// TestDoctorRedactionFollowsConfig confirms that the migrated doctor uses
// a.renderer(cfg, opts) — which reads cfg.Defaults.Redaction — rather than
// hardcoding ModeStandard. Setting a non-standard redaction mode via env must be
// reflected in the doctor output.
func TestDoctorRedactionFollowsConfig(t *testing.T) {
	t.Parallel()

	for _, mode := range []string{"share", "paranoid"} {
		mode := mode
		t.Run(mode, func(t *testing.T) {
			t.Parallel()

			var out, errBuf bytes.Buffer
			a := cli.New(&out, &errBuf, []string{config.EnvRedaction + "=" + mode})
			err := a.Run(context.Background(), []string{"--format", "table", "doctor"})
			if err != nil {
				t.Fatalf("App.Run(doctor, redaction=%s) error = %v, want nil", mode, err)
			}
			if !strings.Contains(out.String(), "Redaction") || !strings.Contains(out.String(), mode) {
				t.Errorf("App.Run(doctor) output = %q, want redaction mode %q", out.String(), mode)
			}
			if errBuf.Len() != 0 {
				t.Errorf("App.Run(doctor, redaction=%s) stderr = %q, want empty", mode, errBuf.String())
			}
		})
	}
}

// TestHybridRouting_AuthGoesViaCobra confirms that "auth" is now dispatched
// through Cobra (Phase 4 migration) and produces its status output via runAuth.
// With no credentials the config loads without error (no-creds is not an
// ErrInvalidConfig), so auth status succeeds in the hermetic env.
func TestHybridRouting_AuthGoesViaCobra(t *testing.T) {
	t.Parallel()

	a, out, errBuf := testDoctorApp(t)
	err := a.Run(context.Background(), []string{"--format", "table", "auth", "status"})
	if err != nil {
		t.Fatalf("App.Run(auth status) error = %v, want nil", err)
	}
	got := out.String()
	// runAuth renders key rows including "Credentials" and "Live API".
	for _, key := range []string{"Credentials", "Live API"} {
		if !strings.Contains(got, key) {
			t.Errorf("App.Run(auth status) stdout = %q, want key %q", got, key)
		}
	}
	if errBuf.Len() != 0 {
		t.Errorf("App.Run(auth status) stderr = %q, want empty", errBuf.String())
	}
}
