package cli_test

// spinner_integration_test.go — End-to-end gating + no-leak tests for the
// progress spinner (Task 2.4).
//
// These tests drive real App.Run calls with a fake ResourceReader to verify:
//
//  1. Gating — zero bytes on stderr when any spinner gate is off (non-TTY,
//     logging active, color=never/--no-color).
//  2. Gating — spinner DOES render (text visible) when all gates pass with
//     StderrTTY: true (now deterministic because Start/Update redraw immediately).
//  3. No-leak — the sentinel secret value returned by the fake reader NEVER
//     appears in stderr; only catalog identifiers and spinner literals do.
//  4. Stdout contains only data output (no braille/spinner text).
//
// Product routing: commands like "zia locations list" go through Cobra because
// "zia" is a migrated product command. We inject a fake reader into App so no
// network calls are made. The dump tests narrow to "--products zia" to avoid
// the constraint that parseProducts() iterates knownProducts() (the static
// catalog), not the injected catalog.

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/cli"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

// ── braille frame detection ──────────────────────────────────────────────────

// spinnerBrailleFrames mirrors the animation sequence in internal/output/spinner.go
// so we can assert presence/absence without importing the internal package.
const spinnerBrailleFrames = "⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏"

// containsBraille reports whether s contains any braille-spinner rune.
func containsBraille(s string) bool {
	for _, r := range spinnerBrailleFrames {
		if strings.ContainsRune(s, r) {
			return true
		}
	}
	return false
}

// containsSpinnerText reports whether s contains any literal the spinner emits:
// braille frames, "contacting Zscaler", or the dump "[N/" counter prefix.
func containsSpinnerText(s string) bool {
	return containsBraille(s) ||
		strings.Contains(s, "contacting Zscaler") ||
		strings.Contains(s, "[1/") // dump counter prefix
}

// ── fake reader for spinner tests ────────────────────────────────────────────

// spinnerFakeReader returns caller-supplied records for List and a single
// record for Show. Get panics — must not be called in these tests.
// This reader is injected into App so no real Zscaler API is contacted.
type spinnerFakeReader struct {
	records []resources.SourceRecord
}

func (r spinnerFakeReader) List(_ context.Context, _ resources.Product, _ string) ([]resources.SourceRecord, error) {
	return r.records, nil
}

func (r spinnerFakeReader) Get(_ context.Context, _ resources.Product, _ string, _ string) (resources.SourceRecord, error) {
	panic("spinnerFakeReader.Get must not be called in spinner integration tests")
}

func (r spinnerFakeReader) Show(_ context.Context, _ resources.Product, _ string) (resources.SourceRecord, error) {
	if len(r.records) > 0 {
		return r.records[0], nil
	}
	return resources.NewSourceRecord(map[string]any{"name": "default"}), nil
}

// newSpinnerListApp builds an App for list-command spinner tests.
// It uses the real catalog (so product routing works) but injects a fake reader.
func newSpinnerListApp(t *testing.T, stderrTTY bool, records []resources.SourceRecord) (*cli.App, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	var out, errBuf bytes.Buffer
	a := cli.NewWithOptions(&out, &errBuf, nil, cli.Options{
		StderrTTY: stderrTTY,
		Reader:    spinnerFakeReader{records: records},
		// Catalog: omitted — uses the real static catalog so "zia" is a known product.
	})
	return a, &out, &errBuf
}

// newSpinnerDumpApp builds an App for dump-command spinner tests.
// It uses the real catalog and injects a fake reader.
func newSpinnerDumpApp(t *testing.T, stderrTTY bool, records []resources.SourceRecord) (*cli.App, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	var out, errBuf bytes.Buffer
	a := cli.NewWithOptions(&out, &errBuf, nil, cli.Options{
		StderrTTY: stderrTTY,
		Reader:    spinnerFakeReader{records: records},
	})
	return a, &out, &errBuf
}

// ziaRecord returns a minimal zia/locations SourceRecord — just the "id" and
// "name" fields that the real zia/locations spec allows in standard mode.
func ziaRecord(id, name string) resources.SourceRecord {
	return resources.NewSourceRecord(map[string]any{
		"id":   id,
		"name": name,
	})
}

// ── §1: Gating — zero bytes when any gate is off ────────────────────────────

// TestSpinnerGating_NoTTY_ListCommand confirms that with StderrTTY: false,
// a "zia locations list" call writes zero spinner bytes to stderr (the piped/CI case).
func TestSpinnerGating_NoTTY_ListCommand(t *testing.T) {
	t.Parallel()

	a, out, errBuf := newSpinnerListApp(t, false, []resources.SourceRecord{ziaRecord("1", "NYC")})
	err := a.Run(context.Background(), []string{"zia", "locations", "list"})
	if err != nil {
		t.Fatalf("App.Run(zia locations list) error = %v, want nil", err)
	}
	if containsSpinnerText(errBuf.String()) {
		t.Errorf("non-TTY list: spinner text leaked to stderr: %q", errBuf.String())
	}
	// stdout must contain some data from the projected record.
	if out.Len() == 0 {
		t.Errorf("non-TTY list: stdout is empty; want projected record data")
	}
}

// TestSpinnerGating_NoTTY_DumpCommand confirms that with StderrTTY: false,
// a dump call writes zero spinner bytes to stderr.
func TestSpinnerGating_NoTTY_DumpCommand(t *testing.T) {
	t.Parallel()

	a, out, errBuf := newSpinnerDumpApp(t, false, []resources.SourceRecord{ziaRecord("1", "NYC")})
	outDir := filepath.Join(t.TempDir(), "spin-gate-no-tty")

	err := a.Run(context.Background(), []string{"dump", "--products", "zia", "--resources", "locations", "--out", outDir})
	if err != nil {
		t.Fatalf("App.Run(dump --products zia) error = %v, want nil", err)
	}
	if containsSpinnerText(errBuf.String()) {
		t.Errorf("non-TTY dump: spinner text leaked to stderr: %q", errBuf.String())
	}
	if out.Len() != 0 {
		t.Errorf("non-TTY dump: stdout = %q, want empty", out.String())
	}
}

// TestSpinnerGating_LogLevelInfo_ListCommand confirms that with StderrTTY: true
// but --log-level info, the spinner is suppressed. Stderr may contain log lines;
// we assert specifically that no braille rune and no spinner literal appear.
func TestSpinnerGating_LogLevelInfo_ListCommand(t *testing.T) {
	t.Parallel()

	a, _, errBuf := newSpinnerListApp(t, true, []resources.SourceRecord{ziaRecord("2", "LA")})
	err := a.Run(context.Background(), []string{
		"--log-level", "info",
		"zia", "locations", "list",
	})
	if err != nil {
		t.Fatalf("App.Run(zia locations list --log-level info) error = %v, want nil", err)
	}
	stderr := errBuf.String()
	// Stderr may contain log lines — assert only that spinner text is absent.
	if containsBraille(stderr) {
		t.Errorf("log-level=info list: braille frame leaked to stderr: %q", stderr)
	}
	if strings.Contains(stderr, "contacting Zscaler") {
		t.Errorf("log-level=info list: 'contacting Zscaler' leaked to stderr: %q", stderr)
	}
}

// TestSpinnerGating_LogLevelInfo_DumpCommand mirrors the list test for dump.
func TestSpinnerGating_LogLevelInfo_DumpCommand(t *testing.T) {
	t.Parallel()

	a, _, errBuf := newSpinnerDumpApp(t, true, []resources.SourceRecord{ziaRecord("3", "Chicago")})
	outDir := filepath.Join(t.TempDir(), "spin-gate-log-info")

	err := a.Run(context.Background(), []string{
		"--log-level", "info",
		"dump", "--products", "zia", "--resources", "locations", "--out", outDir,
	})
	if err != nil {
		t.Fatalf("App.Run(dump --log-level info) error = %v, want nil", err)
	}
	stderr := errBuf.String()
	if containsBraille(stderr) {
		t.Errorf("log-level=info dump: braille frame leaked to stderr: %q", stderr)
	}
	if strings.Contains(stderr, "[1/") {
		t.Errorf("log-level=info dump: dump counter '[1/' leaked to stderr: %q", stderr)
	}
}

// TestSpinnerGating_ColorNever_ListCommand confirms --color never suppresses
// the spinner even when StderrTTY is true.
func TestSpinnerGating_ColorNever_ListCommand(t *testing.T) {
	t.Parallel()

	a, _, errBuf := newSpinnerListApp(t, true, []resources.SourceRecord{ziaRecord("4", "Paris")})
	err := a.Run(context.Background(), []string{
		"--color", "never",
		"zia", "locations", "list",
	})
	if err != nil {
		t.Fatalf("App.Run(zia locations list --color never) error = %v, want nil", err)
	}
	if containsSpinnerText(errBuf.String()) {
		t.Errorf("--color never list: spinner text leaked to stderr: %q", errBuf.String())
	}
}

// TestSpinnerGating_NoColor_ListCommand confirms --no-color suppresses the
// spinner even when StderrTTY is true (equivalent to --color never).
func TestSpinnerGating_NoColor_ListCommand(t *testing.T) {
	t.Parallel()

	a, _, errBuf := newSpinnerListApp(t, true, []resources.SourceRecord{ziaRecord("5", "Tokyo")})
	err := a.Run(context.Background(), []string{
		"--no-color",
		"zia", "locations", "list",
	})
	if err != nil {
		t.Fatalf("App.Run(zia locations list --no-color) error = %v, want nil", err)
	}
	if containsSpinnerText(errBuf.String()) {
		t.Errorf("--no-color list: spinner text leaked to stderr: %q", errBuf.String())
	}
}

// TestSpinnerGating_ColorNever_DumpCommand confirms --color never suppresses
// the dump spinner.
func TestSpinnerGating_ColorNever_DumpCommand(t *testing.T) {
	t.Parallel()

	a, _, errBuf := newSpinnerDumpApp(t, true, []resources.SourceRecord{ziaRecord("6", "London")})
	outDir := filepath.Join(t.TempDir(), "spin-gate-color-never")

	err := a.Run(context.Background(), []string{
		"--color", "never",
		"dump", "--products", "zia", "--resources", "locations", "--out", outDir,
	})
	if err != nil {
		t.Fatalf("App.Run(dump --color never) error = %v, want nil", err)
	}
	if containsSpinnerText(errBuf.String()) {
		t.Errorf("--color never dump: spinner text leaked to stderr: %q", errBuf.String())
	}
}

// ── §2: Gating — spinner DOES render when all gates pass ────────────────────

// TestSpinnerGating_AllGatesPass_ListCommand confirms that with StderrTTY: true,
// log-level off (default), and color auto, a list command causes stderr to
// contain "contacting Zscaler" or a braille frame. Deterministic because
// Start() now redraws immediately before launching the goroutine.
func TestSpinnerGating_AllGatesPass_ListCommand(t *testing.T) {
	t.Parallel()

	a, _, errBuf := newSpinnerListApp(t, true, []resources.SourceRecord{ziaRecord("7", "Sydney")})
	err := a.Run(context.Background(), []string{"zia", "locations", "list"})
	if err != nil {
		t.Fatalf("App.Run(zia locations list) error = %v, want nil", err)
	}
	stderr := errBuf.String()
	if !containsBraille(stderr) && !strings.Contains(stderr, "contacting Zscaler") {
		t.Errorf("all-gates-pass list: expected spinner text in stderr, got %q", stderr)
	}
}

// TestSpinnerGating_AllGatesPass_DumpCommand confirms that with all gates on,
// a dump causes stderr to contain the "[1/" counter and "zia/locations".
// Deterministic because Update() now redraws immediately.
func TestSpinnerGating_AllGatesPass_DumpCommand(t *testing.T) {
	t.Parallel()

	a, _, errBuf := newSpinnerDumpApp(t, true, []resources.SourceRecord{ziaRecord("8", "Berlin")})
	outDir := filepath.Join(t.TempDir(), "spin-gate-all-pass")

	err := a.Run(context.Background(), []string{
		"dump", "--products", "zia", "--resources", "locations", "--out", outDir,
	})
	if err != nil {
		t.Fatalf("App.Run(dump) error = %v, want nil", err)
	}
	stderr := errBuf.String()
	if !strings.Contains(stderr, "[1/") {
		t.Errorf("all-gates-pass dump: expected '[1/' counter in stderr, got %q", stderr)
	}
	if !strings.Contains(stderr, "zia/locations") {
		t.Errorf("all-gates-pass dump: expected 'zia/locations' in stderr, got %q", stderr)
	}
}

// ── §3: No-leak — secret-shaped record values must never appear in stderr ────

// secretSentinel is a unique token that is secret-shaped (AWS-key prefix +
// suffix) but purely fictional. It must never appear in spinner output.
const secretSentinel = "SeNtInElSecret-AKIA1234567890TOKEN"

// TestSpinnerNoLeak_DumpSecretField drives a dump with a fake reader that
// returns records containing the sentinel in the "preSharedKey" field, which
// the zia/locations spec classifies as ClassSecret. The spinner must never
// emit that value to stderr — it only writes catalog identifiers.
func TestSpinnerNoLeak_DumpSecretField(t *testing.T) {
	t.Parallel()

	// preSharedKey is ClassSecret in the real zia/locations catalog spec.
	// ProjectRecordsAndVerify will redact/drop it; the spinner must also never
	// emit it since spinners only write catalog-derived text (not record data).
	secretRecord := resources.NewSourceRecord(map[string]any{
		"id":           "42",
		"name":         "public-location",
		"preSharedKey": secretSentinel,
	})

	var out, errBuf bytes.Buffer
	a := cli.NewWithOptions(&out, &errBuf, nil, cli.Options{
		StderrTTY: true,
		Reader:    spinnerFakeReader{records: []resources.SourceRecord{secretRecord}},
	})

	outDir := filepath.Join(t.TempDir(), "spin-no-leak-dump")
	err := a.Run(context.Background(), []string{
		"dump", "--products", "zia", "--resources", "locations", "--out", outDir,
	})
	if err != nil {
		t.Fatalf("App.Run(dump with secret record) error = %v, want nil", err)
	}

	stderr := errBuf.String()
	if strings.Contains(stderr, secretSentinel) {
		t.Errorf("SECURITY: secret sentinel leaked to stderr via spinner:\nstderr = %q", stderr)
	}
	// Confirm the spinner was active (so we're testing the live path, not no-op).
	// This must be an error: a regression that silently disables the spinner
	// would leave the no-leak assertion vacuously true.
	if !containsBraille(stderr) && !strings.Contains(stderr, "[1/") {
		t.Errorf("spinner appears inactive in dump no-leak test (StderrTTY=true, color=auto); stderr = %q", stderr)
	}
}

// TestSpinnerNoLeak_ListSecretField drives a "zia locations list" command with
// a record containing the sentinel in the "preSharedKey" (ClassSecret) field.
// Stderr must not contain the sentinel.
func TestSpinnerNoLeak_ListSecretField(t *testing.T) {
	t.Parallel()

	secretRecord := resources.NewSourceRecord(map[string]any{
		"id":           "99",
		"name":         "public-widget",
		"preSharedKey": secretSentinel,
	})

	var out, errBuf bytes.Buffer
	a := cli.NewWithOptions(&out, &errBuf, nil, cli.Options{
		StderrTTY: true,
		Reader:    spinnerFakeReader{records: []resources.SourceRecord{secretRecord}},
	})

	err := a.Run(context.Background(), []string{"zia", "locations", "list"})
	if err != nil {
		t.Fatalf("App.Run(zia locations list with secret record) error = %v, want nil", err)
	}

	stderr := errBuf.String()
	if strings.Contains(stderr, secretSentinel) {
		t.Errorf("SECURITY: secret sentinel leaked to stderr via spinner:\nstderr = %q", stderr)
	}
	// Confirm the sentinel is NOT on stdout either.
	if strings.Contains(out.String(), secretSentinel) {
		t.Errorf("SECURITY: secret sentinel appeared on stdout: %q", out.String())
	}
}

// TestSpinnerNoLeak_ShowSecretField exercises the show path (callWithSpinner
// with "contacting Zscaler"). zia/activation-status has a ShowOperation and
// a ClassSecret field, so we use a show-capable real catalog resource.
//
// Since the exact show-capable resources depend on the catalog, we use
// "zia/bandwidth-classes show" — but first check one exists. If the catalog
// changes, the fake reader handles any product/resource name gracefully.
//
// Strategy: inject a fake reader that returns a record with the sentinel in any
// field. The spinner only emits "contacting Zscaler" — never record field values.
func TestSpinnerNoLeak_ShowSecretField(t *testing.T) {
	t.Parallel()

	// Find a real show-capable resource in the catalog for routing.
	var showProduct resources.Product
	var showResource string
	for _, spec := range resources.Catalog() {
		if spec.SupportsReadOperation("show") {
			showProduct = spec.Product
			showResource = spec.Name
			break
		}
	}
	if showResource == "" {
		t.Skip("no show-capable resource in catalog; skipping show no-leak test")
	}

	// Build a record with the sentinel in any non-projected field — even an
	// unknown field that won't appear in the spec (so projection drops it),
	// but the raw record still exists in the reader's return value. The spinner
	// must not leak this raw value regardless.
	secretRecord := resources.NewSourceRecord(map[string]any{
		// A safe field that will project successfully under any spec.
		"name": "public-config",
		// An unknown field that will be dropped by projection — but if the
		// spinner incorrectly read from records, it would expose this.
		"__leak_test_sentinel__": secretSentinel,
	})

	var out, errBuf bytes.Buffer
	a := cli.NewWithOptions(&out, &errBuf, nil, cli.Options{
		StderrTTY: true,
		Reader:    spinnerFakeReader{records: []resources.SourceRecord{secretRecord}},
	})

	err := a.Run(context.Background(), []string{
		string(showProduct), showResource, "show",
	})
	// Accept any error (missing required fields etc.) — we only care about the
	// no-leak property, not the output correctness for this edge case.
	_ = err

	stderr := errBuf.String()
	if strings.Contains(stderr, secretSentinel) {
		t.Errorf("SECURITY: secret sentinel leaked to stderr via spinner (show path):\nstderr = %q", stderr)
	}
}

// ── §4: Stdout contains only data, no spinner text ───────────────────────────

// TestSpinnerStderrOnly_ListCommand verifies that with an active spinner,
// the spinner output goes ONLY to stderr — stdout must contain no braille
// runes and no "contacting Zscaler" text.
func TestSpinnerStderrOnly_ListCommand(t *testing.T) {
	t.Parallel()

	a, out, errBuf := newSpinnerListApp(t, true, []resources.SourceRecord{ziaRecord("10", "kappa")})
	err := a.Run(context.Background(), []string{"zia", "locations", "list"})
	if err != nil {
		t.Fatalf("App.Run(zia locations list) error = %v, want nil", err)
	}

	stdout := out.String()
	if containsBraille(stdout) {
		t.Errorf("spinner braille frame appeared on stdout: %q", stdout)
	}
	if strings.Contains(stdout, "contacting Zscaler") {
		t.Errorf("'contacting Zscaler' appeared on stdout: %q", stdout)
	}
	// Confirm spinner was active on stderr.
	// This must be an error: a regression that silently disables the spinner
	// would leave the stdout-clean assertion vacuously true.
	stderr := errBuf.String()
	if !containsBraille(stderr) && !strings.Contains(stderr, "contacting Zscaler") {
		t.Errorf("spinner appears inactive in list stderr-only test (StderrTTY=true, color=auto); stderr = %q", stderr)
	}
}

// TestSpinnerStderrOnly_DumpCommand verifies that the dump spinner counter
// appears only on stderr, never on stdout.
func TestSpinnerStderrOnly_DumpCommand(t *testing.T) {
	t.Parallel()

	a, out, errBuf := newSpinnerDumpApp(t, true, []resources.SourceRecord{ziaRecord("11", "lambda")})
	outDir := filepath.Join(t.TempDir(), "spin-stderr-only-dump")

	err := a.Run(context.Background(), []string{
		"dump", "--products", "zia", "--resources", "locations", "--out", outDir,
	})
	if err != nil {
		t.Fatalf("App.Run(dump) error = %v, want nil", err)
	}

	stdout := out.String()
	if containsBraille(stdout) {
		t.Errorf("spinner braille frame appeared on stdout: %q", stdout)
	}
	if strings.Contains(stdout, "[1/") {
		t.Errorf("dump counter '[1/' appeared on stdout: %q", stdout)
	}
	if out.Len() != 0 {
		t.Errorf("dump stdout = %q, want empty", stdout)
	}
	// Confirm spinner was active on stderr.
	// This must be an error: a regression that silently disables the spinner
	// would leave the stdout-clean assertion vacuously true.
	stderr := errBuf.String()
	if !containsBraille(stderr) && !strings.Contains(stderr, "[1/") {
		t.Errorf("spinner appears inactive in dump stderr-only test (StderrTTY=true, color=auto); stderr = %q", stderr)
	}
}

// ── sentinel uniqueness guard ─────────────────────────────────────────────────

// TestSpinnerSentinelIsUnique verifies the sentinel doesn't accidentally appear
// in any expected spinner output or catalog identifier, so false-positive
// no-leak failures are impossible.
func TestSpinnerSentinelIsUnique(t *testing.T) {
	t.Parallel()

	for _, safe := range []string{
		"contacting Zscaler",
		"[1/1] zia/locations",
		"dumping",
		spinnerBrailleFrames,
		"zia/locations",
	} {
		if strings.Contains(safe, secretSentinel) {
			t.Errorf("sentinel %q unexpectedly appears in safe string %q", secretSentinel, safe)
		}
	}
}

// TestSpinnerNoLeak_DumpFileContents confirms the dump output FILE contains
// the projected record data — proving data flows to files, not stderr.
func TestSpinnerNoLeak_DumpFileContents(t *testing.T) {
	t.Parallel()

	// Use a unique location name we can grep for in the dump file.
	const publicName = "unique-location-file-content-check"
	records := []resources.SourceRecord{
		resources.NewSourceRecord(map[string]any{
			"id":   "77",
			"name": publicName,
		}),
	}

	var out, errBuf bytes.Buffer
	a := cli.NewWithOptions(&out, &errBuf, nil, cli.Options{
		StderrTTY: true,
		Reader:    spinnerFakeReader{records: records},
	})

	outDir := filepath.Join(t.TempDir(), "spin-no-leak-file-check")
	err := a.Run(context.Background(), []string{
		"dump", "--products", "zia", "--resources", "locations", "--out", outDir,
	})
	if err != nil {
		t.Fatalf("App.Run(dump) error = %v, want nil", err)
	}

	// The dump file for zia/locations must exist and contain the public name.
	// dump.Write writes resource files to resources/<product>/<name>.json.
	dumpFile := filepath.Join(outDir, "resources", "zia", "locations.json")
	contents, err := os.ReadFile(dumpFile)
	if err != nil {
		t.Fatalf("dump file %s not created: %v", dumpFile, err)
	}
	if !strings.Contains(string(contents), publicName) {
		t.Errorf("dump file %s = %q, want '%s'", dumpFile, string(contents), publicName)
	}
	// Confirm no sentinel leak to stderr.
	if strings.Contains(errBuf.String(), secretSentinel) {
		t.Errorf("SECURITY: sentinel in stderr: %q", errBuf.String())
	}
	// Confirm data stayed out of stdout.
	if out.Len() != 0 {
		t.Errorf("dump stdout = %q, want empty", out.String())
	}
}
