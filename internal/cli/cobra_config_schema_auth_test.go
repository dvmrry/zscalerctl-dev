package cli_test

// cobra_config_schema_auth_test.go - Tests for the Cobra config, schema, and
// auth command groups.
//
// Tests verify:
//  1. config init — creates file, path on stdout, hints on stderr, exit 0.
//  2. config init --force — overwrites an existing file.
//  3. config init (file exists, no --force) — UsageError (exit 2).
//  4. config init extra-arg — UsageError (exit 2).
//  5. --format ndjson config init — format-agnostic; exit 0 (NOT rejected).
//  6. config show — renders redacted config.
//  7. config show --format ndjson — UsageError (exit 2).
//  8. schema list — renders catalog.
//  9. schema list --format ndjson — UsageError (exit 2).
// 10. auth status — renders auth status.
// 11. auth status --format ndjson — UsageError (exit 2).
// 12. Bare config / schema / auth → UsageError (exit 2) listing subcommands.
// 13. config bogus / schema bogus / auth bogus → UsageError (exit 2) via Cobra.
// 14. config init --help / config show --help / schema list --help / auth status --help → Cobra help.
// 15. config --help / schema --help / auth --help → Cobra parent help.
// 16. config show redaction follows config.

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

// ── helpers ──────────────────────────────────────────────────────────────────

func testConfigApp(t *testing.T, env ...string) (*cli.App, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	var out, errBuf bytes.Buffer
	a := cli.New(&out, &errBuf, env)
	return a, &out, &errBuf
}

// ── config init ──────────────────────────────────────────────────────────────

// TestCobraConfigInitCreatesFile asserts that "config init" creates the file at
// the given --config path, prints the path to stdout, and emits hints to stderr.
func TestCobraConfigInitCreatesFile(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "config.yaml")
	a, out, errBuf := testConfigApp(t)

	if err := a.Run(context.Background(), []string{"--config", path, "config", "init"}); err != nil {
		t.Fatalf("App.Run(config init) error = %v, want nil", err)
	}

	// stdout must be the created path (trimmed).
	if strings.TrimSpace(out.String()) != path {
		t.Errorf("config init stdout = %q, want path %q", out.String(), path)
	}
	// stderr must have next-steps hints.
	if errBuf.Len() == 0 {
		t.Error("config init stderr = empty, want next-steps hint")
	}
	// File must exist.
	if _, err := os.Stat(path); err != nil {
		t.Errorf("config init did not create file: %v", err)
	}
}

// TestCobraConfigInitForceOverwrites asserts that --force replaces an existing file.
func TestCobraConfigInitForceOverwrites(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("default_profile: old\nprofiles:\n  old: {}\n"), 0o600); err != nil {
		t.Fatalf("seed WriteFile: %v", err)
	}

	a, _, _ := testConfigApp(t)
	if err := a.Run(context.Background(), []string{"--config", path, "config", "init", "--force"}); err != nil {
		t.Fatalf("App.Run(config init --force) error = %v, want nil", err)
	}

	cfg, err := config.LoadConfig(nil, config.LoadOptions{ConfigPath: path, Resolver: stubResolver{}})
	if err != nil {
		t.Fatalf("LoadConfig(overwritten) error = %v, want nil", err)
	}
	if cfg.Profile != "prod" {
		t.Errorf("profile after --force = %q, want prod (template default)", cfg.Profile)
	}
}

// TestCobraConfigInitRefusesOverwriteWithoutForce asserts UsageError (exit 2)
// when the target exists and --force is absent; the existing file is untouched.
func TestCobraConfigInitRefusesOverwriteWithoutForce(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "config.yaml")
	sentinel := "default_profile: keep-me\nprofiles:\n  keep-me:\n    vanity_domain: keep\n"
	if err := os.WriteFile(path, []byte(sentinel), 0o600); err != nil {
		t.Fatalf("seed WriteFile: %v", err)
	}

	a, _, _ := testConfigApp(t)
	err := a.Run(context.Background(), []string{"--config", path, "config", "init"})
	if !errors.Is(err, cli.ErrUsage) {
		t.Fatalf("config init (existing, no --force) error = %v, want ErrUsage", err)
	}
	if !strings.Contains(err.Error(), "--force") {
		t.Errorf("config init error = %q, want mention of --force", err.Error())
	}
	// File must be untouched.
	got, _ := os.ReadFile(path)
	if string(got) != sentinel {
		t.Errorf("config init (no --force) modified existing file: got %q", got)
	}
}

// TestCobraConfigInitRejectsExtraArgs asserts extra positional args → UsageError.
func TestCobraConfigInitRejectsExtraArgs(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "config.yaml")
	a, _, _ := testConfigApp(t)

	err := a.Run(context.Background(), []string{"--config", path, "config", "init", "extra"})
	if !errors.Is(err, cli.ErrUsage) {
		t.Fatalf("config init extra-arg error = %v, want ErrUsage", err)
	}
	if _, statErr := os.Stat(path); !errors.Is(statErr, os.ErrNotExist) {
		t.Error("config init (bad args) created a file; expected no file")
	}
}

// TestCobraConfigInitFormatAgnostic asserts that --format ndjson config init
// is format-agnostic: it exits 0 and creates the file (ndjson is NOT rejected).
// This is the key behavioral difference from config show / schema list / auth status
// which DO reject ndjson.
func TestCobraConfigInitFormatAgnostic(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "config.yaml")
	a, out, _ := testConfigApp(t)

	// --format ndjson must NOT cause an error — config init is format-agnostic.
	if err := a.Run(context.Background(), []string{"--format", "ndjson", "--config", path, "config", "init"}); err != nil {
		t.Fatalf("App.Run(--format ndjson config init) error = %v, want nil (format-agnostic)", err)
	}
	// stdout must still be the created path.
	if strings.TrimSpace(out.String()) != path {
		t.Errorf("config init (ndjson) stdout = %q, want path %q", out.String(), path)
	}
	// File must exist.
	if _, err := os.Stat(path); err != nil {
		t.Errorf("config init (ndjson) did not create file: %v", err)
	}
}

// ── config show ──────────────────────────────────────────────────────────────

// TestCobraConfigShowRenders asserts that "config show" renders the active
// config in redacted table form with the expected keys.
func TestCobraConfigShowRenders(t *testing.T) {
	t.Parallel()

	a, out, errBuf := testConfigApp(t)
	if err := a.Run(context.Background(), []string{"--format", "table", "config", "show"}); err != nil {
		t.Fatalf("App.Run(config show) error = %v, want nil", err)
	}
	got := out.String()
	for _, key := range []string{"Profile", "Config", "Auth Mode", "Redaction"} {
		if !strings.Contains(got, key) {
			t.Errorf("config show stdout = %q, want key %q", got, key)
		}
	}
	if errBuf.Len() != 0 {
		t.Errorf("config show stderr = %q, want empty", errBuf.String())
	}
}

// TestCobraConfigShowNDJSONRejected asserts that --format ndjson on config show
// returns UsageError (exit 2) — unlike config init, config show rejects ndjson.
func TestCobraConfigShowNDJSONRejected(t *testing.T) {
	t.Parallel()

	a, _, _ := testConfigApp(t)
	err := a.Run(context.Background(), []string{"--format", "ndjson", "config", "show"})
	if err == nil {
		t.Fatal("App.Run(--format ndjson config show) error = nil, want UsageError")
	}
	if !errors.Is(err, cli.ErrUsage) {
		t.Errorf("config show ndjson error = %v, want ErrUsage", err)
	}
}

// TestCobraConfigShowRedactionFollowsConfig asserts that "config show" uses
// a.renderer(cfg, opts) — the redaction mode comes from config, not hardcoded.
func TestCobraConfigShowRedactionFollowsConfig(t *testing.T) {
	t.Parallel()

	for _, mode := range []string{"share", "paranoid"} {
		mode := mode
		t.Run(mode, func(t *testing.T) {
			t.Parallel()

			var out, errBuf bytes.Buffer
			a := cli.New(&out, &errBuf, []string{config.EnvRedaction + "=" + mode})
			if err := a.Run(context.Background(), []string{"--format", "table", "config", "show"}); err != nil {
				t.Fatalf("App.Run(config show, redaction=%s) error = %v, want nil", mode, err)
			}
			if !strings.Contains(out.String(), mode) {
				t.Errorf("config show output = %q, want redaction mode %q", out.String(), mode)
			}
			if errBuf.Len() != 0 {
				t.Errorf("config show (redaction=%s) stderr = %q, want empty", mode, errBuf.String())
			}
		})
	}
}

// ── schema list ──────────────────────────────────────────────────────────────

// TestCobraSchemaListRenders asserts that "schema list" enumerates catalog
// resources in table form (product + resource + ops).
func TestCobraSchemaListRenders(t *testing.T) {
	t.Parallel()

	a, out, errBuf := testConfigApp(t)
	if err := a.Run(context.Background(), []string{"--format", "table", "schema", "list"}); err != nil {
		t.Fatalf("App.Run(schema list) error = %v, want nil", err)
	}
	got := out.String()
	// The catalog contains zia resources; the output should mention "zia".
	if !strings.Contains(got, "zia") {
		t.Errorf("schema list stdout = %q, want 'zia' product", got)
	}
	if errBuf.Len() != 0 {
		t.Errorf("schema list stderr = %q, want empty", errBuf.String())
	}
}

// TestCobraSchemaListNDJSONRejected asserts that --format ndjson on schema list
// returns UsageError (exit 2).
func TestCobraSchemaListNDJSONRejected(t *testing.T) {
	t.Parallel()

	a, _, _ := testConfigApp(t)
	err := a.Run(context.Background(), []string{"--format", "ndjson", "schema", "list"})
	if err == nil {
		t.Fatal("App.Run(--format ndjson schema list) error = nil, want UsageError")
	}
	if !errors.Is(err, cli.ErrUsage) {
		t.Errorf("schema list ndjson error = %v, want ErrUsage", err)
	}
}

// ── auth status ──────────────────────────────────────────────────────────────

// TestCobraAuthStatusRenders asserts that "auth status" renders auth-related rows.
func TestCobraAuthStatusRenders(t *testing.T) {
	t.Parallel()

	a, out, errBuf := testConfigApp(t)
	if err := a.Run(context.Background(), []string{"--format", "table", "auth", "status"}); err != nil {
		t.Fatalf("App.Run(auth status) error = %v, want nil", err)
	}
	got := out.String()
	for _, key := range []string{"Credentials", "Live API"} {
		if !strings.Contains(got, key) {
			t.Errorf("auth status stdout = %q, want key %q", got, key)
		}
	}
	if errBuf.Len() != 0 {
		t.Errorf("auth status stderr = %q, want empty", errBuf.String())
	}
}

// TestCobraAuthStatusNDJSONRejected asserts that --format ndjson on auth status
// returns UsageError (exit 2).
func TestCobraAuthStatusNDJSONRejected(t *testing.T) {
	t.Parallel()

	a, _, _ := testConfigApp(t)
	err := a.Run(context.Background(), []string{"--format", "ndjson", "auth", "status"})
	if err == nil {
		t.Fatal("App.Run(--format ndjson auth status) error = nil, want UsageError")
	}
	if !errors.Is(err, cli.ErrUsage) {
		t.Errorf("auth status ndjson error = %v, want ErrUsage", err)
	}
}

// ── Bare/unknown parent invocations (exit 2) ─────────────────────────────────

// TestCobareBareParentExitTwo asserts that bare "config", "schema", "auth" (no
// subcommand) return UsageError (exit 2) and list the valid subcommands.
func TestCobraBareParentExitTwo(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args []string
		want string // substring expected in error message
	}{
		{name: "bare-config", args: []string{"config"}, want: "usage: zscalerctl config <init|show>"},
		{name: "bare-schema", args: []string{"schema"}, want: "usage: zscalerctl schema list"},
		{name: "bare-auth", args: []string{"auth"}, want: "usage: zscalerctl auth status"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			a, _, _ := testConfigApp(t)
			err := a.Run(context.Background(), tc.args)
			if err == nil {
				t.Fatalf("App.Run(%v) error = nil, want UsageError", tc.args)
			}
			if !errors.Is(err, cli.ErrUsage) {
				t.Errorf("App.Run(%v) error = %v, want ErrUsage (exit 2)", tc.args, err)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("App.Run(%v) error = %q, want it to contain %q", tc.args, err.Error(), tc.want)
			}
		})
	}
}

// TestCobraUnknownSubcommandExitTwo asserts that "config bogus", "schema bogus",
// "auth bogus" all return UsageError (exit 2).
func TestCobraUnknownSubcommandExitTwo(t *testing.T) {
	t.Parallel()

	cases := [][]string{
		{"config", "bogus"},
		{"schema", "bogus"},
		{"auth", "bogus"},
	}

	for _, args := range cases {
		args := args
		t.Run(strings.Join(args, "-"), func(t *testing.T) {
			t.Parallel()

			a, _, _ := testConfigApp(t)
			err := a.Run(context.Background(), args)
			if err == nil {
				t.Fatalf("App.Run(%v) error = nil, want UsageError", args)
			}
			if !errors.Is(err, cli.ErrUsage) {
				t.Errorf("App.Run(%v) error = %v, want ErrUsage (exit 2)", args, err)
			}
		})
	}
}

// ── Help surfaces (--help → Cobra, exit 0) ───────────────────────────────────

// TestCobraSubcommandHelp asserts that --help on config init, config show,
// schema list, auth status returns exit 0 with Cobra-formatted help.
func TestCobraSubcommandHelp(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		args    []string
		wantStr string // expected substring in stdout
	}{
		{name: "config-init-help", args: []string{"config", "init", "--help"}, wantStr: "zscalerctl config init"},
		{name: "config-show-help", args: []string{"config", "show", "--help"}, wantStr: "zscalerctl config show"},
		{name: "schema-list-help", args: []string{"schema", "list", "--help"}, wantStr: "zscalerctl schema list"},
		{name: "auth-status-help", args: []string{"auth", "status", "--help"}, wantStr: "zscalerctl auth status"},
		{name: "config-help", args: []string{"config", "--help"}, wantStr: "zscalerctl config"},
		{name: "schema-help", args: []string{"schema", "--help"}, wantStr: "zscalerctl schema"},
		{name: "auth-help", args: []string{"auth", "--help"}, wantStr: "zscalerctl auth"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			a, out, errBuf := testConfigApp(t)
			err := a.Run(context.Background(), tc.args)
			if err != nil {
				t.Fatalf("App.Run(%v) error = %v, want nil (--help should exit 0)", tc.args, err)
			}
			got := out.String()
			if got == "" {
				t.Fatal("stdout is empty; redactor may not have flushed")
			}
			if !strings.Contains(got, tc.wantStr) {
				t.Errorf("App.Run(%v) stdout = %q, want %q", tc.args, got, tc.wantStr)
			}
			if errBuf.Len() != 0 {
				t.Errorf("App.Run(%v) stderr = %q, want empty", tc.args, errBuf.String())
			}
		})
	}
}
