package main

// golden_surface_test.go — Task 0.3 of the Cobra migration.
//
// Freezes the CURRENT (pre-Cobra) CLI surface by exec-ing the real binary through
// the cmd/zscalerctl boundary, capturing stdout+stderr+exit-code, and comparing
// against committed golden files.  It does NOT call internal functions — the whole
// point is to snapshot what the boundary itself emits.
//
// Usage:
//
//	go test ./cmd/zscalerctl/... -run TestGoldenSurface            # verify
//	go test ./cmd/zscalerctl/... -run TestGoldenSurface -update    # regenerate goldens
//
// Note: -update never changes wantCode in the table — those stay asserted in Go.

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/dvmrry/zscalerctl/internal/resources"
)

// updateGolden is set by -update to regenerate golden files instead of comparing.
var updateGolden = flag.Bool("update", false, "regenerate golden files")

// goldenBinary holds the path to the binary built in TestMain for this run.
var goldenBinary string

// TestMain builds the binary once for all golden tests.
func TestMain(m *testing.M) {
	// flag.Parse is called by testing infrastructure before TestMain in Go 1.13+,
	// but parse here explicitly in case that changes.
	flag.Parse()

	tmpDir, err := os.MkdirTemp("", "zscalerctl-golden-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "golden: create tmpdir: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tmpDir)

	binPath := filepath.Join(tmpDir, "zscalerctl")
	if runtime.GOOS == "windows" {
		binPath += ".exe"
	}
	cmd := exec.Command("go", "build", "-o", binPath, "./cmd/zscalerctl")
	cmd.Dir = filepath.Join("..", "..")
	out, buildErr := cmd.CombinedOutput()
	if buildErr != nil {
		fmt.Fprintf(os.Stderr, "golden: build failed: %v\n%s\n", buildErr, out)
		os.Exit(1)
	}
	goldenBinary = binPath
	os.Exit(m.Run())
}

// hermeticEnv builds a clean environment for each test case.
// It provides only PATH plus a fresh empty HOME/XDG_CONFIG_HOME so the CLI
// cannot find any real config file or credentials, and explicitly EXCLUDES every
// ZSCALER* and ZSCALERCTL* variable so no live API calls can be triggered.
//
// Net effect: commands that need credentials must fail with exit 3
// (missing_credentials); they will never reach a real API endpoint.
func hermeticEnv(homeDir string, extra []string) []string {
	env := []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + homeDir,
		"XDG_CONFIG_HOME=" + homeDir,
		// Provide a minimal locale so Go's text/tabwriter stays stable.
		"LANG=C",
	}
	env = append(env, extra...)
	return env
}

// scrub removes non-deterministic tokens from output before golden comparison.
// It replaces:
//   - Version strings (semver, pseudo-version, "dev")
//   - Git commit SHAs
//   - Build dates / timestamps
//   - Go runtime version
//   - OS/arch (varies across machines)
//   - Absolute temp paths and HOME paths
//   - Any "time=..." structured-log tokens
//   - The binary path itself
func scrub(s, homeDir, binPath string) string {
	// Replace absolute homeDir and binPath before regex passes (they contain
	// path separators that would confuse regex escaping).
	if homeDir != "" {
		s = strings.ReplaceAll(s, homeDir, "<TMPDIR>")
	}
	if binPath != "" {
		s = strings.ReplaceAll(s, binPath, "<BINARY>")
	}

	// Pseudo-version: v0.68.1-0.20260620073434-79678e7c1f63
	s = rePseudoVersion.ReplaceAllString(s, "<VERSION>")
	// Go runtime version: go1.22.3 — must run BEFORE reSemver so "go1.26.4" is
	// consumed as a unit (→ go<GOVERSION>) and reSemver does not strip the digits
	// first (which would yield the wrong "go<VERSION>" placeholder).
	s = reGoVersion.ReplaceAllString(s, "go<GOVERSION>")
	// Plain semver: v1.2.3 or 1.2.3 — backstop for standalone version strings.
	// Word boundaries prevent silent corruption of IP-like strings (e.g. 192.168.1.1).
	// Safe because reGoVersion already handled "go<version>" above.
	s = reSemver.ReplaceAllString(s, "<VERSION>")
	// "dev" version string (the fallback when built without ldflags)
	s = reDevVersion.ReplaceAllString(s, "${1}${2}${4}<VERSION>${3}")
	// Git commit SHA (7-40 hex chars)
	s = reCommit.ReplaceAllString(s, "${1}<COMMIT>")
	// Build date (ISO-8601 or RFC3339 timestamps)
	s = reDate.ReplaceAllString(s, "<DATE>")
	// OS/arch combinations that vary by machine:
	//   combined form (table "Platform" field): darwin/arm64 → <PLATFORM>
	s = reOSArch.ReplaceAllString(s, "<PLATFORM>")
	//   separate JSON fields from `version --format json`
	s = reJSONOS.ReplaceAllString(s, `${1}"<OS>"`)
	s = reJSONArch.ReplaceAllString(s, `${1}"<ARCH>"`)
	// time= structured log tokens
	s = reTimeToken.ReplaceAllString(s, "time=<TIME>")

	return s
}

var (
	// Go pseudo-version: covers both forms produced by go mod:
	//   no-base-tag:   v0.0.0-20260620152824-f3a2eda1c513  (timestamp directly after semver)
	//   with-base-tag: v0.68.1-0.20260620073434-79678e7c1f63 (pre=0, dot before timestamp)
	// The alternation (?:0\.\d{14}|\d{14}) distinguishes the two forms.
	// The trailing (?:\+[a-zA-Z0-9.]+)? optionally consumes the build-metadata
	// suffix Go's VCS stamping appends from a dirty working tree (e.g. "+dirty",
	// "+incompatible") so the whole token — not just the hash — scrubs to <VERSION>.
	rePseudoVersion = regexp.MustCompile(`v?\d+\.\d+\.\d+-(?:0\.\d{14}|\d{14})-[0-9a-f]{12}(?:\+[a-zA-Z0-9.]+)?`)
	// e.g. v1.2.3 or 1.2.3; \b prevents matching inside IP-like strings
	reSemver = regexp.MustCompile(`\bv?\d+\.\d+\.\d+\b`)
	// bare "dev" version in version output (value-only, not a substring)
	// arm1: (\bVersion\s+)dev\b captures the "Version   " label prefix (group 1).
	// arm2: ("(?:cli_)?version":\s*")dev(") captures the JSON key+open-quote
	//        (group 2) and closing quote (group 3) so both are preserved in the
	//        replacement. Handles both "version" (version --format json) and
	//        "cli_version" (introspect output).
	// arm3: (version:\s+)dev\b captures the introspect --format pretty tree
	//        label "version:   " (group 4, lowercase with colon) — the human
	//        tree renderer emits `  version:   <cli_version>\n`.
	reDevVersion = regexp.MustCompile(`(?m)(\bVersion\s+)dev\b|("(?:cli_)?version":\s*")dev(")|(version:\s+)dev\b`)
	// Git commit SHA: 7-40 hex digits following "Commit" label or "commit" JSON key.
	// Group 1 captures the label/key prefix (e.g. "Commit    " or `"commit": "`);
	// group 2 is consumed (the hex SHA). Replacement restores group 1.
	reCommit = regexp.MustCompile(`(?i)(commit["\s:]+)([0-9a-f]{7,40})\b`)
	// ISO-8601 / RFC3339 date or datetime
	reDate = regexp.MustCompile(`\d{4}-\d{2}-\d{2}(?:T\d{2}:\d{2}:\d{2}(?:Z|[+-]\d{2}:\d{2})?)?`)
	// Go runtime version: go1.22.3 or go1.22 — applied first in scrub() as "go<GOVERSION>"
	reGoVersion = regexp.MustCompile(`go\d+\.\d+(?:\.\d+)?`)
	// OS/arch combined form: darwin/arm64, linux/amd64, etc. (table Platform field)
	reOSArch = regexp.MustCompile(`(darwin|linux|windows|freebsd|openbsd|netbsd)/(amd64|arm64|386|arm|s390x|ppc64le)`)
	// JSON "os" field: "os": "darwin" (separate from arch in JSON version output)
	reJSONOS = regexp.MustCompile(`("os":\s*)"(darwin|linux|windows|freebsd|openbsd|netbsd)"`)
	// JSON "arch" field: "arch": "arm64"
	reJSONArch = regexp.MustCompile(`("arch":\s*)"(amd64|arm64|386|arm|s390x|ppc64le)"`)
	// time= token in structured logs
	reTimeToken = regexp.MustCompile(`time=\S+`)
)

// reIntrospectFieldArrays matches the per-field "fields" and "output_fields"
// arrays in introspect JSON output. [^\]]* matches newlines (it is any character
// except ']') so the pattern works for both single-line and multi-line arrays;
// the arrays contain only simple string elements with no nested brackets, so no
// look-ahead for nested structure is needed.
var reIntrospectFieldArrays = regexp.MustCompile(`"(fields|output_fields)": \[[^\]]*\]`)

// collapseIntrospectFieldArrays replaces the per-field "fields" and
// "output_fields" arrays in introspect output with a stable placeholder, so the
// surface golden captures command/flag/exit-code structure and catalog
// products/resources/ops without freezing the (separately gated) per-field
// catalog data. The catalog field content is asserted by TestIntrospectAndDocsAgree.
func collapseIntrospectFieldArrays(s string) string {
	return reIntrospectFieldArrays.ReplaceAllString(s, `"$1": ["<omitted>"]`)
}

// runCase executes one golden case against the pre-built binary.
// It returns scrubbed stdout, scrubbed stderr, and the actual exit code.
func runCase(t *testing.T, homeDir string, args []string, extraEnv []string) (stdout, stderr string, code int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, goldenBinary, args...)
	cmd.Env = hermeticEnv(homeDir, extraEnv)

	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	runErr := cmd.Run()
	code = 0
	if runErr != nil {
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			code = exitErr.ExitCode()
		} else if ctx.Err() != nil {
			t.Fatalf("case timed out after 15s: args=%v", args)
		} else {
			t.Fatalf("exec error (not exit): %v", runErr)
		}
	}

	stdout = scrub(outBuf.String(), homeDir, goldenBinary)
	stderr = scrub(errBuf.String(), homeDir, goldenBinary)
	return stdout, stderr, code
}

// goldenPath returns the path to the golden file for a given case name.
func goldenPath(name, stream string) string {
	return filepath.Join("testdata", "surface", name+"."+stream+".golden")
}

// assertGolden checks or updates a golden file for one stream (stdout/stderr).
func assertGolden(t *testing.T, name, stream, actual string) {
	t.Helper()
	path := goldenPath(name, stream)
	if *updateGolden {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, []byte(actual), 0o644); err != nil {
			t.Fatalf("write golden %s: %v", path, err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("missing golden %s (run with -update to create): %v", path, err)
	}
	if string(want) != actual {
		t.Errorf("golden mismatch for %s/%s:\nwant:\n%s\ngot:\n%s", name, stream, want, actual)
	}
}

// surfaceCase is one entry in the golden surface table.
type surfaceCase struct {
	// name is a slug used as the golden file basename and test sub-name.
	name string
	// args are the CLI arguments passed to the binary (no binary name).
	args []string
	// extraEnv is appended to the hermetic env for this case only.
	// Use sparingly — the hermetic env must stay the baseline.
	extraEnv []string
	// wantCode is the expected exit code. Asserted in Go; never overwritten by -update.
	wantCode int
	// note is a one-word description of why this case is in the table (for the reader).
	note string
}

// TestGoldenSurface freezes the pre-Cobra CLI surface by running each case
// through the real binary and comparing scrubbed stdout+stderr to golden files.
//
// Exit codes are asserted directly in Go; golden files capture the human/machine
// readable output shape. During the Cobra migration, each intentional change that
// causes a golden diff must be recorded in testdata/surface/surface_changes.md.
//
// NOTE: testdata/surface/surface_changes.md is a human-maintained convention —
// the test suite does NOT enforce that it is updated when goldens change.
// A maintainer who updates goldens without updating surface_changes.md will
// not see a test failure; the manifest is an audit trail, not a machine gate.
func TestGoldenSurface(t *testing.T) {
	if goldenBinary == "" {
		t.Fatal("goldenBinary not set — TestMain did not run")
	}

	// Each case gets its own hermetic HOME so no state leaks between them.
	baseHome := t.TempDir()

	cases := []surfaceCase{
		// ── Global help ──────────────────────────────────────────────────────────
		{
			name:     "help-flag",
			args:     []string{"--help"},
			wantCode: 0,
			note:     "global-help",
		},
		// ── No args ─────────────────────────────────────────────────────────────
		{
			name:     "no-args",
			args:     []string{},
			wantCode: 2,
			note:     "usage-error",
		},
		// ── version ──────────────────────────────────────────────────────────────
		{
			name:     "version",
			args:     []string{"--format", "table", "version"},
			wantCode: 0,
			note:     "success",
		},
		// ── --format json version ────────────────────────────────────────────────
		{
			name:     "version-json",
			args:     []string{"--format", "json", "version"},
			wantCode: 0,
			note:     "json-shape",
		},
		// ── --format ndjson version (§5c format-allowlist boundary) ─────────────
		{
			name:     "version-ndjson-rejected",
			args:     []string{"--format", "ndjson", "version"},
			wantCode: 2,
			note:     "format-allowlist",
		},
		// ── doctor (full output; hermetic env is stable) ─────────────────────────
		{
			name:     "doctor",
			args:     []string{"--format", "table", "doctor"},
			wantCode: 0,
			note:     "stable-in-hermetic-env",
		},
		// ── unknown command ───────────────────────────────────────────────────────
		{
			name:     "unknown-command",
			args:     []string{"frobnicate"},
			wantCode: 2,
			note:     "usage-error",
		},
		// ── unknown flag on a subcommand ──────────────────────────────────────────
		{
			name:     "version-unknown-flag",
			args:     []string{"version", "--nope"},
			wantCode: 2,
			note:     "usage-error",
		},
		// ── version --help (Cobra help surface; frozen after Task 1.5 migration) ──
		{
			name:     "version-help",
			args:     []string{"version", "--help"},
			wantCode: 0,
			note:     "cobra-help-surface",
		},
		// ── doctor --help (Cobra help surface; frozen after Task 1.5.2 migration) ─
		{
			name:     "doctor-help",
			args:     []string{"doctor", "--help"},
			wantCode: 0,
			note:     "cobra-help-surface",
		},
		// ── product help ──────────────────────────────────────────────────────────
		{
			name:     "zia-help",
			args:     []string{"zia", "--help"},
			wantCode: 0,
			note:     "product-help",
		},
		// ── url-lookup help (Phase 2b: DisableFlagParsing subcommand) ────────────
		{
			name:     "zia-url-lookup-help",
			args:     []string{"zia", "url-lookup", "--help"},
			wantCode: 0,
			note:     "cobra-help-surface",
		},
		// ── resource help (Phase 2c: SetHelpFunc restores legacy resource-specific help) ──
		{
			name:     "zia-locations-help",
			args:     []string{"zia", "locations", "--help"},
			wantCode: 0,
			note:     "resource-help",
		},
		{
			name:     "zia-locations-list-help",
			args:     []string{"zia", "locations", "list", "--help"},
			wantCode: 0,
			note:     "resource-help",
		},
		// ── resource list (hermetic → missing credentials) ────────────────────────
		{
			name:     "zia-locations-list-no-creds",
			args:     []string{"--format", "table", "zia", "locations", "list"},
			wantCode: 3,
			note:     "missing-creds",
		},
		// ── JSON error envelope for missing credentials ────────────────────────────
		{
			name:     "zia-locations-list-no-creds-json",
			args:     []string{"--format", "json", "zia", "locations", "list"},
			wantCode: 3,
			note:     "json-error-envelope",
		},
		// ── schema list ───────────────────────────────────────────────────────────
		{
			name:     "schema-list",
			args:     []string{"--format", "table", "schema", "list"},
			wantCode: 0,
			note:     "catalog-enumeration",
		},
		// ── completion bash (will change in Phase 5; documented) ─────────────────
		{
			name:     "completion-bash",
			args:     []string{"--format", "table", "completion", "bash"},
			wantCode: 0,
			note:     "shell-completion",
		},
		// ── completion --help (exercises isCompletionArgs redactor-bypass for the
		//    help path; static help text only — no credential data) ──────────────
		{
			name:     "completion-help",
			args:     []string{"completion", "--help"},
			wantCode: 0,
			note:     "cobra-help-surface",
		},
		// ── dump --help (Cobra help surface; frozen after Phase 3a migration) ─────
		{
			name:     "dump-help",
			args:     []string{"dump", "--help"},
			wantCode: 0,
			note:     "cobra-help-surface",
		},
		// ── diff --help (Cobra help surface; frozen after Phase 3b migration) ────
		{
			name:     "diff-help",
			args:     []string{"diff", "--help"},
			wantCode: 0,
			note:     "cobra-help-surface",
		},
		// ── Phase 4: config command group ────────────────────────────────────────
		// config init: path on stdout, exit 0; temp HOME is scrubbed to <TMPDIR>.
		{
			name:     "config-init",
			args:     []string{"config", "init"},
			wantCode: 0,
			note:     "config-init-creates-file",
		},
		// config --help: Cobra parent help listing init|show subcommands.
		{
			name:     "config-help",
			args:     []string{"config", "--help"},
			wantCode: 0,
			note:     "cobra-help-surface",
		},
		// config init --help: Cobra subcommand help.
		{
			name:     "config-init-help",
			args:     []string{"config", "init", "--help"},
			wantCode: 0,
			note:     "cobra-help-surface",
		},
		// config show --help: Cobra subcommand help.
		{
			name:     "config-show-help",
			args:     []string{"config", "show", "--help"},
			wantCode: 0,
			note:     "cobra-help-surface",
		},
		// bare config: UsageError exit 2.
		{
			name:     "config-bare",
			args:     []string{"config"},
			wantCode: 2,
			note:     "bare-parent-usage-error",
		},
		// ── Phase 4: schema command group ────────────────────────────────────────
		// schema --help: Cobra parent help.
		{
			name:     "schema-help",
			args:     []string{"schema", "--help"},
			wantCode: 0,
			note:     "cobra-help-surface",
		},
		// schema list --help: Cobra subcommand help.
		{
			name:     "schema-list-help",
			args:     []string{"schema", "list", "--help"},
			wantCode: 0,
			note:     "cobra-help-surface",
		},
		// bare schema: UsageError exit 2.
		{
			name:     "schema-bare",
			args:     []string{"schema"},
			wantCode: 2,
			note:     "bare-parent-usage-error",
		},
		// ── Phase 4: auth command group ──────────────────────────────────────────
		// auth --help: Cobra parent help.
		{
			name:     "auth-help",
			args:     []string{"auth", "--help"},
			wantCode: 0,
			note:     "cobra-help-surface",
		},
		// auth status --help: Cobra subcommand help.
		{
			name:     "auth-status-help",
			args:     []string{"auth", "status", "--help"},
			wantCode: 0,
			note:     "cobra-help-surface",
		},
		// bare auth: UsageError exit 2.
		{
			name:     "auth-bare",
			args:     []string{"auth"},
			wantCode: 2,
			note:     "bare-parent-usage-error",
		},
		// ── Task 1.4: introspect command ──────────────────────────────────────────
		// introspect (default JSON; stdout is not a TTY in the hermetic env so
		// FormatAuto resolves to JSON — the machine-first default).
		{
			name:     "introspect",
			args:     []string{"introspect"},
			wantCode: 0,
			note:     "json-surface-map",
		},
		// introspect --format pretty: human-readable tree renderer.
		{
			name:     "introspect-pretty",
			args:     []string{"--format", "pretty", "introspect"},
			wantCode: 0,
			note:     "human-tree",
		},
		// introspect --format ndjson: rejected (single document, not a stream).
		{
			name:     "introspect-ndjson-rejected",
			args:     []string{"--format", "ndjson", "introspect"},
			wantCode: 2,
			note:     "format-allowlist",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			// Give each case its own empty home so a case writing config cannot
			// affect any other case even if cases run in parallel.
			caseHome := filepath.Join(baseHome, tc.name)
			if err := os.MkdirAll(caseHome, 0o755); err != nil {
				t.Fatalf("mkdir case home: %v", err)
			}

			stdout, stderr, code := runCase(t, caseHome, tc.args, tc.extraEnv)

			// Collapse per-field arrays in introspect JSON output so the surface golden
			// captures CLI structure (commands/flags/exit-codes/catalog products+ops)
			// without freezing per-field catalog data that is separately gated by
			// TestIntrospectAndDocsAgree. Apply only to introspect cases; no other
			// goldens are affected.
			if strings.HasPrefix(tc.name, "introspect") {
				stdout = collapseIntrospectFieldArrays(stdout)
			}

			// Exit code is always asserted in Go — never overwritten by -update.
			if code != tc.wantCode {
				t.Errorf("exit code = %d, want %d (note: %s)\nstdout:\n%s\nstderr:\n%s",
					code, tc.wantCode, tc.note, stdout, stderr)
			}

			// Golden comparison for stdout and stderr.
			assertGolden(t, tc.name, "stdout", stdout)
			assertGolden(t, tc.name, "stderr", stderr)
		})
	}
}

// TestParseErrorsExitTwo is a consolidated, table-driven boundary test that
// asserts every parse-error CLASS maps to exit code 2 at the real binary boundary.
// It does NOT snapshot stdout/stderr (those are covered by TestGoldenSurface where
// relevant); it only asserts the exit code.
//
// Parse errors come from two sources:
//  1. Global flag parsing (parseGlobal / splitGlobalArgs): bad/missing values for
//     the 13 global flags → UsageError → exitCodeForError → exit 2.
//  2. Migrated commands (version, doctor) via Cobra: unknown flags (SetFlagErrorFunc),
//     ndjson format (rejectUnsupportedFormat), extra args (requireNoArgs) → exit 2.
//
// Cases already covered in TestGoldenSurface are noted with "also-golden" so a
// reader can see the overlap. They are repeated here because the two tests have
// different purposes: golden files freeze output shape; this test contracts the
// exit code only.
//
// If any case returns an exit code other than 2, the test fails loudly — that
// signals a real gap in the exit-2 contract.
func TestParseErrorsExitTwo(t *testing.T) {
	if goldenBinary == "" {
		t.Fatal("goldenBinary not set — TestMain did not run")
	}

	type exitCase struct {
		name       string   // sub-test name
		args       []string // CLI args passed to the binary
		source     string   // which parse layer this exercises
		alsoGolden bool     // true when TestGoldenSurface already covers this case
	}

	cases := []exitCase{
		// ── Unknown command ───────────────────────────────────────────────────────
		// Source: runParsed legacy switch → UsageError → exit 2.
		{
			name:       "unknown-command",
			args:       []string{"frobnicate"},
			source:     "legacy-dispatch",
			alsoGolden: true, // covered by TestGoldenSurface/unknown-command
		},

		// ── Unknown flag on migrated commands (Cobra SetFlagErrorFunc) ────────────
		// Source: Cobra SetFlagErrorFunc wraps the error in UsageError → exit 2.
		{
			name:       "version-unknown-flag",
			args:       []string{"version", "--nope"},
			source:     "cobra-SetFlagErrorFunc",
			alsoGolden: true, // covered by TestGoldenSurface/version-unknown-flag
		},
		{
			name:       "doctor-unknown-flag",
			args:       []string{"doctor", "--nope"},
			source:     "cobra-SetFlagErrorFunc",
			alsoGolden: false, // NOT in TestGoldenSurface
		},

		// ── Bad global flag values (parseGlobal) ──────────────────────────────────
		// Source: parseGlobal calls output.ParseFormat, redact.ParseMode,
		// output.ParseColorMode, newDiagLogger, and checks timeout > 0.
		{
			name:       "bad-timeout-value",
			args:       []string{"--timeout", "notaduration", "version"},
			source:     "parseGlobal-fs.Parse",
			alsoGolden: false,
		},
		{
			name:       "bad-format-value",
			args:       []string{"--format", "bogus", "version"},
			source:     "parseGlobal-output.ParseFormat",
			alsoGolden: false,
		},
		{
			name:       "bad-color-value",
			args:       []string{"--color", "bogus", "version"},
			source:     "parseGlobal-output.ParseColorMode",
			alsoGolden: false,
		},
		{
			name:       "bad-redaction-value",
			args:       []string{"--redaction", "bogus", "version"},
			source:     "parseGlobal-redact.ParseMode",
			alsoGolden: false,
		},
		{
			name:       "bad-log-level-value",
			args:       []string{"--log-level", "bogus", "version"},
			source:     "parseGlobal-newDiagLogger",
			alsoGolden: false,
		},

		// ── Missing global flag value (splitGlobalArgs trailing value-expecting flag) ─
		// Source: splitGlobalArgs detects i+1 >= len(args) → UsageError → exit 2.
		{
			name:       "trailing-timeout-no-value",
			args:       []string{"--timeout"},
			source:     "splitGlobalArgs-trailing-value",
			alsoGolden: false,
		},
		{
			name:       "trailing-timeout-no-value-with-subcmd",
			args:       []string{"version", "--timeout"},
			source:     "splitGlobalArgs-trailing-value",
			alsoGolden: false,
		},

		// ── Negative / zero timeout (parseGlobal explicit guard) ──────────────────
		// Source: parseGlobal checks *timeout <= 0 → UsageError → exit 2.
		{
			name:       "zero-timeout",
			args:       []string{"--timeout", "0", "version"},
			source:     "parseGlobal-timeout-positive",
			alsoGolden: false,
		},

		// ── ndjson on migrated commands (rejectUnsupportedFormat) ─────────────────
		// Source: runVersion / runDoctor check opts.format == output.FormatNDJSON →
		// rejectUnsupportedFormat → UsageError → exit 2.
		{
			name:       "ndjson-version",
			args:       []string{"--format", "ndjson", "version"},
			source:     "rejectUnsupportedFormat-version",
			alsoGolden: true, // covered by TestGoldenSurface/version-ndjson-rejected
		},
		{
			name:       "ndjson-doctor",
			args:       []string{"--format", "ndjson", "doctor"},
			source:     "rejectUnsupportedFormat-doctor",
			alsoGolden: false,
		},

		// ── Extra args on no-arg commands (requireNoArgs) ─────────────────────────
		// Source: requireNoArgs checks len(args) != 0 → UsageError → exit 2.
		{
			name:       "version-extra-arg",
			args:       []string{"version", "extra"},
			source:     "requireNoArgs-version",
			alsoGolden: false,
		},
		{
			name:       "doctor-extra-arg",
			args:       []string{"doctor", "extra"},
			source:     "requireNoArgs-doctor",
			alsoGolden: false,
		},

		// ── dump ndjson / no --out (Phase 3a — dump migrated to Cobra) ────────────
		// Source: newDumpCmd RunE rejects ndjson before config load → UsageError → exit 2.
		{
			name:       "ndjson-dump",
			args:       []string{"--format", "ndjson", "dump", "--out", "/tmp/zsc-ndjson-reject"},
			source:     "rejectUnsupportedFormat-dump",
			alsoGolden: false,
		},
		// Source: newDumpCmd RunE validates --out non-empty → UsageError → exit 2.
		{
			name:       "dump-no-out",
			args:       []string{"dump"},
			source:     "runDumpWithOptions-empty-out",
			alsoGolden: false,
		},

		// ── Phase 4: config/schema/auth ndjson rejection ──────────────────────────
		// config init is format-agnostic (exit 0), so it is NOT in this table.
		// config show rejects ndjson → exit 2.
		{
			name:       "ndjson-config-show",
			args:       []string{"--format", "ndjson", "config", "show"},
			source:     "rejectUnsupportedFormat-config-show",
			alsoGolden: false,
		},
		// schema list rejects ndjson → exit 2.
		{
			name:       "ndjson-schema-list",
			args:       []string{"--format", "ndjson", "schema", "list"},
			source:     "rejectUnsupportedFormat-schema-list",
			alsoGolden: false,
		},
		// auth status rejects ndjson → exit 2.
		{
			name:       "ndjson-auth-status",
			args:       []string{"--format", "ndjson", "auth", "status"},
			source:     "rejectUnsupportedFormat-auth-status",
			alsoGolden: false,
		},
		// bare config → exit 2; covered by TestGoldenSurface/config-bare.
		{
			name:       "bare-config",
			args:       []string{"config"},
			source:     "bare-parent-UsageError-config",
			alsoGolden: true,
		},
		// bare schema → exit 2; covered by TestGoldenSurface/schema-bare.
		{
			name:       "bare-schema",
			args:       []string{"schema"},
			source:     "bare-parent-UsageError-schema",
			alsoGolden: true,
		},
		// bare auth → exit 2; covered by TestGoldenSurface/auth-bare.
		{
			name:       "bare-auth",
			args:       []string{"auth"},
			source:     "bare-parent-UsageError-auth",
			alsoGolden: true,
		},
		// config bogus → exit 2 (Cobra unknown subcommand).
		{
			name:       "config-bogus",
			args:       []string{"config", "bogus"},
			source:     "cobra-unknown-subcommand-config",
			alsoGolden: false,
		},
		// schema bogus → exit 2.
		{
			name:       "schema-bogus",
			args:       []string{"schema", "bogus"},
			source:     "cobra-unknown-subcommand-schema",
			alsoGolden: false,
		},
		// auth bogus → exit 2.
		{
			name:       "auth-bogus",
			args:       []string{"auth", "bogus"},
			source:     "cobra-unknown-subcommand-auth",
			alsoGolden: false,
		},

		// ── L-4: diff arity errors ─────────────────────────────────────────────
		// diff with 0 positionals → exit 2 (requires exactly 2 dirs).
		{
			name:       "diff-no-args",
			args:       []string{"diff"},
			source:     "runDiff-arity-0-positionals",
			alsoGolden: false,
		},
		// diff with 1 positional → exit 2 (requires exactly 2 dirs).
		{
			name:       "diff-one-arg",
			args:       []string{"diff", "/tmp/some-dir"},
			source:     "runDiff-arity-1-positional",
			alsoGolden: false,
		},

		// ── L-5/N-3: --format ndjson completion bash → exit 2 ─────────────────
		// completion bash does not accept ndjson (shell script, not JSON).
		{
			name:       "ndjson-completion-bash",
			args:       []string{"--format", "ndjson", "completion", "bash"},
			source:     "rejectUnsupportedFormat-completion-bash",
			alsoGolden: false,
		},

		// ── N-1: unknown-resource coverage in product commands ────────────────
		// "zia bogus-resource bogus-op" reaches the product RunE which does not
		// recognise the resource and emits a UsageError (the resource list) →
		// exit 2. (An unknown *resource* resolved before the operation is checked,
		// producing a usage error rather than a not-found error.)
		{
			name:       "zia-bogus-resource-bogus-op",
			args:       []string{"zia", "bogus-resource", "bogus-op"},
			source:     "runProduct-unknown-resource-UsageError",
			alsoGolden: false,
		},
	}

	baseHome := t.TempDir()

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			caseHome := filepath.Join(baseHome, "parse-error-"+tc.name)
			if err := os.MkdirAll(caseHome, 0o755); err != nil {
				t.Fatalf("mkdir case home: %v", err)
			}

			_, stderr, code := runCase(t, caseHome, tc.args, nil)

			if code != 2 {
				t.Errorf(
					"FAIL: parse-error class %q (source: %s) returned exit %d, want 2\n"+
						"args: %v\nstderr: %s\n"+
						"This signals a real exit-2 contract gap — do NOT change wantCode here; fix the gap.",
					tc.name, tc.source, code, tc.args, stderr,
				)
			}
		})
	}
}

// TestCommandTreeInventory generates a deterministic text inventory of the full
// CLI surface — top-level verbs plus every catalog resource — and compares it to
// a committed golden file. This is the durable add/remove/rename gate: any change
// to the command tree will show as a diff here and must be recorded in
// testdata/surface/surface_changes.md.
func TestCommandTreeInventory(t *testing.T) {
	if goldenBinary == "" {
		t.Fatal("goldenBinary not set — TestMain did not run")
	}

	catalog := resources.Catalog()

	var b strings.Builder
	b.WriteString("# zscalerctl command tree inventory\n")
	b.WriteString("# Generated by TestCommandTreeInventory — do not edit by hand.\n")
	b.WriteString("# To update: go test ./cmd/zscalerctl/... -run TestCommandTreeInventory -update\n")
	b.WriteString("# Every change here must be recorded in testdata/surface/surface_changes.md.\n")
	b.WriteString("\n")

	// ── Top-level verbs ───────────────────────────────────────────────────────
	b.WriteString("## top-level verbs\n")
	topLevel := []string{
		"help",
		"version",
		"doctor",
		"auth status",
		"config show",
		"config init",
		"introspect",
		"schema list",
		"dump",
		"diff",
		"completion bash",
		"completion zsh",
		"completion fish",
		"completion powershell",
		"zia url-lookup",
	}
	for _, verb := range topLevel {
		fmt.Fprintf(&b, "  %s\n", verb)
	}
	b.WriteString("\n")

	// ── Resource catalog (from resources.Catalog()) ────────────────────────────
	b.WriteString("## catalog resources\n")
	b.WriteString("# product  resource  operations\n")

	// Group by product in catalog order, then sort resource names within each group
	// so the output is stable regardless of catalog slice order within a product.
	type productResources struct {
		product   string
		resources []resources.ResourceSpec
	}
	var groups []productResources
	productIndex := map[string]int{}
	for _, spec := range catalog {
		p := string(spec.Product)
		idx, ok := productIndex[p]
		if !ok {
			idx = len(groups)
			productIndex[p] = idx
			groups = append(groups, productResources{product: p})
		}
		groups[idx].resources = append(groups[idx].resources, spec)
	}
	// Sort resources within each product group by name for determinism.
	for i := range groups {
		sort.Slice(groups[i].resources, func(a, b int) bool {
			return groups[i].resources[a].Name < groups[i].resources[b].Name
		})
	}

	for _, g := range groups {
		for _, spec := range g.resources {
			ops := make([]string, 0, len(spec.Operations))
			for _, op := range spec.Operations {
				ops = append(ops, op.Name)
			}
			fmt.Fprintf(&b, "  %s  %s  %s\n", g.product, spec.Name, strings.Join(ops, "|"))
		}
	}

	actual := b.String()
	path := filepath.Join("testdata", "surface", "inventory.golden")

	if *updateGolden {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir testdata/surface: %v", err)
		}
		if err := os.WriteFile(path, []byte(actual), 0o644); err != nil {
			t.Fatalf("write inventory golden: %v", err)
		}
		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("missing inventory golden %s (run with -update to create): %v", path, err)
	}
	if string(want) != actual {
		t.Errorf("command tree inventory changed:\nwant:\n%s\ngot:\n%s", want, actual)
	}
}

// TestScrubPseudoVersion verifies that scrub() fully replaces both forms of
// Go pseudo-version to <VERSION>, regardless of whether the module has a base
// tag. This prevents CI failures on runners (Linux) where modules with no
// version tags produce the no-base-tag form v0.0.0-<timestamp>-<hash>.
func TestScrubPseudoVersion(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "no-base-tag form",
			input: "Version   v0.0.0-20260620152824-f3a2eda1c513",
			want:  "Version   <VERSION>",
		},
		{
			name:  "with-base-tag form",
			input: "Version   v0.68.1-0.20260620073434-79678e7c1f63",
			want:  "Version   <VERSION>",
		},
		{
			name:  "no-base-tag in json",
			input: `"version": "v0.0.0-20260620152824-f3a2eda1c513"`,
			want:  `"version": "<VERSION>"`,
		},
		{
			// cli_version is the JSON key used by introspect output; verify it is
			// scrubbed in both pseudo-version forms so introspect.stdout.golden is
			// byte-stable across machines with and without a base tag.
			name:  "cli_version no-base-tag in json",
			input: `"cli_version": "v0.0.0-20260620152824-f3a2eda1c513"`,
			want:  `"cli_version": "<VERSION>"`,
		},
		{
			name:  "cli_version with-base-tag in json",
			input: `"cli_version": "v0.68.1-0.20260620073434-79678e7c1f63"`,
			want:  `"cli_version": "<VERSION>"`,
		},
		{
			// Dirty working tree: Go's VCS stamping appends "+dirty" build metadata
			// after the hash. The whole token — suffix included — must scrub to
			// <VERSION>, else "<VERSION>+dirty" leaks into the golden comparison.
			name:  "with-base-tag dirty suffix",
			input: "Version   0.68.1-0.20260620073434-79678e7c1f63+dirty",
			want:  "Version   <VERSION>",
		},
		{
			name:  "cli_version dirty suffix in json",
			input: `"cli_version": "0.68.1-0.20260620073434-79678e7c1f63+dirty"`,
			want:  `"cli_version": "<VERSION>"`,
		},
		{
			name:  "plain semver unchanged by pseudo-version pass",
			input: "Version   v1.2.3",
			want:  "Version   <VERSION>",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := scrub(tc.input, "", "")
			if got != tc.want {
				t.Errorf("scrub(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

// TestScrubDevVersion verifies that the "dev" version scrubber preserves the
// surrounding label/key context (the bug was that it dropped the label, producing
// structurally wrong output).  Because no-ldflags builds yield version="dev" only
// when debug.ReadBuildInfo returns "(devel)" or fails, this trigger is invisible in
// normal 'go test' runs — the unit test provides the deterministic class-closer.
func TestScrubDevVersion(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input string
		want  string
	}{
		{
			// Table output: label + whitespace must be preserved.
			name:  "table form",
			input: "Version   dev",
			want:  "Version   <VERSION>",
		},
		{
			// JSON version output key must be preserved.
			name:  "json version key",
			input: `"version": "dev"`,
			want:  `"version": "<VERSION>"`,
		},
		{
			// introspect uses "cli_version" as the JSON key — must also be covered.
			name:  "json cli_version key",
			input: `"cli_version": "dev"`,
			want:  `"cli_version": "<VERSION>"`,
		},
		{
			// introspect --format pretty tree: lowercase "version:" label with
			// colon (distinct from the table "Version   " label). The human
			// tree renderer emits `  version:   <cli_version>\n`.
			name:  "pretty tree form",
			input: "  version:   dev",
			want:  "  version:   <VERSION>",
		},
		{
			// Multi-line: all three arms in one string, label whitespace preserved.
			name:  "multiline table, json, and pretty",
			input: "Version   dev\n\"version\": \"dev\"\n\"cli_version\": \"dev\"\n  version:   dev",
			want:  "Version   <VERSION>\n\"version\": \"<VERSION>\"\n\"cli_version\": \"<VERSION>\"\n  version:   <VERSION>",
		},
		{
			// Negative: "developer" must not be truncated.
			name:  "not a prefix of longer word",
			input: "Version   developer",
			want:  "Version   developer",
		},
		{
			// Negative: a real pseudo-version should not be double-touched here
			// (rePseudoVersion fires first and converts it; reDevVersion would not
			// match the already-replaced "<VERSION>").
			name:  "already-replaced placeholder untouched",
			input: `"version": "<VERSION>"`,
			want:  `"version": "<VERSION>"`,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := scrub(tc.input, "", "")
			if got != tc.want {
				t.Errorf("scrub(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

// TestScrubCommit verifies that the commit scrubber preserves the surrounding
// label/key context (the bug was that it dropped the label, producing
// structurally wrong output).  In practice, no-ldflags builds set commit="unknown"
// which is not hex and never fires the regex — the unit test is the deterministic
// class-closer.
func TestScrubCommit(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input string
		want  string
	}{
		{
			// Table output (7-char short SHA): label + whitespace preserved.
			name:  "table form short sha",
			input: "Commit    79678e7",
			want:  "Commit    <COMMIT>",
		},
		{
			// Table output (full 40-char SHA): label + whitespace preserved.
			name:  "table form full sha",
			input: "Commit    79678e7c1f6300000000000000000000000000",
			want:  "Commit    <COMMIT>",
		},
		{
			// JSON version output: key + open-quote preserved; closing quote preserved
			// because group 1 captures through the opening quote of the value.
			name:  "json commit key",
			input: `"commit": "79678e7c1f63"`,
			want:  `"commit": "<COMMIT>"`,
		},
		{
			// Mixed-case "Commit" in JSON (the (?i) flag).
			name:  "json commit key mixed case",
			input: `"Commit": "79678e7c1f63"`,
			want:  `"Commit": "<COMMIT>"`,
		},
		{
			// Negative: "unknown" is not hex and must not be replaced.
			name:  "unknown commit not replaced table",
			input: "Commit    unknown",
			want:  "Commit    unknown",
		},
		{
			// Negative: "unknown" in JSON must not be replaced.
			name:  "unknown commit not replaced json",
			input: `"commit": "unknown"`,
			want:  `"commit": "unknown"`,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := scrub(tc.input, "", "")
			if got != tc.want {
				t.Errorf("scrub(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

// TestScrubDate verifies that ISO-8601 / RFC3339 date and datetime strings are
// replaced with <DATE> and that no capture group is dropped (reDate has no groups).
func TestScrubDate(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input string
		want  string
	}{
		{name: "plain date", input: "2026-06-20", want: "<DATE>"},
		{name: "datetime UTC", input: "2026-06-20T14:30:00Z", want: "<DATE>"},
		{name: "datetime offset", input: "2026-06-20T14:30:00+05:30", want: "<DATE>"},
		{name: "datetime negative offset", input: "2026-06-20T14:30:00-08:00", want: "<DATE>"},
		{
			name:  "json date field preserved",
			input: `"date": "2026-06-20T14:30:00Z"`,
			want:  `"date": "<DATE>"`,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := scrub(tc.input, "", "")
			if got != tc.want {
				t.Errorf("scrub(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

// TestScrubGoVersion verifies that Go runtime version strings are replaced with
// "go<GOVERSION>" and that reSemver does not corrupt them (reGoVersion fires first).
func TestScrubGoVersion(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input string
		want  string
	}{
		{name: "three-part go version", input: "go1.22.3", want: "go<GOVERSION>"},
		{name: "two-part go version", input: "go1.22", want: "go<GOVERSION>"},
		{
			name:  "json go field",
			input: `"go": "go1.24.2"`,
			want:  `"go": "go<GOVERSION>"`,
		},
		{
			name:  "table Go field",
			input: "Go        go1.24.2",
			want:  "Go        go<GOVERSION>",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := scrub(tc.input, "", "")
			if got != tc.want {
				t.Errorf("scrub(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

// TestScrubOSArch verifies that OS/arch combinations (table Platform field) are
// replaced with <PLATFORM> (whole match replaced — intentional, no prefix to
// preserve) and that separate JSON os/arch fields use their own scrubbers.
func TestScrubOSArch(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input string
		want  string
	}{
		{name: "darwin arm64", input: "darwin/arm64", want: "<PLATFORM>"},
		{name: "linux amd64", input: "linux/amd64", want: "<PLATFORM>"},
		{name: "windows amd64", input: "windows/amd64", want: "<PLATFORM>"},
		{
			name:  "table Platform field",
			input: "Platform  darwin/arm64",
			want:  "Platform  <PLATFORM>",
		},
		{
			name:  "json os field",
			input: `"os": "darwin"`,
			want:  `"os": "<OS>"`,
		},
		{
			name:  "json arch field",
			input: `"arch": "arm64"`,
			want:  `"arch": "<ARCH>"`,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := scrub(tc.input, "", "")
			if got != tc.want {
				t.Errorf("scrub(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

// TestScrubTimeToken verifies that structured-log time= tokens are replaced with
// "time=<TIME>" (reTimeToken has no capture groups — no prefix to preserve).
func TestScrubTimeToken(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "iso timestamp token",
			input: "time=2026-06-20T14:30:00Z",
			want:  "time=<TIME>",
		},
		{
			name:  "quoted timestamp token",
			input: `time="2026-06-20T14:30:00Z"`,
			want:  "time=<TIME>",
		},
		{
			name:  "token in log line",
			input: `time=2026-06-20T14:30:00Z level=DEBUG msg="loading config"`,
			want:  `time=<TIME> level=DEBUG msg="loading config"`,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := scrub(tc.input, "", "")
			if got != tc.want {
				t.Errorf("scrub(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}
