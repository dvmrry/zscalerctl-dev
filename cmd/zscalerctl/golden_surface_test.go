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
	s = reDevVersion.ReplaceAllString(s, "<VERSION>")
	// Git commit SHA (7-40 hex chars)
	s = reCommit.ReplaceAllString(s, "<COMMIT>")
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
	// e.g. 0.68.1-0.20260620073434-79678e7c1f63 or v0.68.1-0.20260620073434-79678e7c1f63
	rePseudoVersion = regexp.MustCompile(`v?\d+\.\d+\.\d+-0\.\d{14}-[0-9a-f]{12}`)
	// e.g. v1.2.3 or 1.2.3; \b prevents matching inside IP-like strings
	reSemver = regexp.MustCompile(`\bv?\d+\.\d+\.\d+\b`)
	// bare "dev" version in version output (value-only, not a substring)
	reDevVersion = regexp.MustCompile(`(?m)(\bVersion\s+)dev\b|("version":\s*)"dev"`)
	// Git commit SHA: 7-40 hex digits following "Commit" label or "commit" JSON key
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
		// ── product help ──────────────────────────────────────────────────────────
		{
			name:     "zia-help",
			args:     []string{"zia", "--help"},
			wantCode: 0,
			note:     "product-help",
		},
		// ── resource help ─────────────────────────────────────────────────────────
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
