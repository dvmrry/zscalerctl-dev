package cli_test

// cobra_diff_test.go — Tests for the Cobra-migrated diff command (Phase 3b).
//
// Tests here verify:
//  1. diff <old> <new> with fixture dumps renders a report; exit 0 (no --fail-on-drift).
//  2. --fail-on-drift with drifted fixtures → DriftDetectedError (errors.Is ErrDriftDetected, exit 7).
//  3. --detail includes record-level detail strings in table output.
//  4. 1 positional → UsageError{diffUsage()}; 3 positionals → UsageError.
//  5. --format ndjson diff a b → exit-2 (rejectUnsupportedFormat).
//  6. Invalid dump dir → UsageError (ErrInvalidDump mapping preserved).
//  7. --allow-partial changes partial-dump rejection behaviour.
//  8. ModeStandard redaction preserved: diff ignores configured redaction mode.
//  9. Local flags survive splitGlobalArgs: diff --products zia --detail dir1 dir2
//     parses correctly (positionals = the 2 dirs).
// 10. diff --help → exit 0, Cobra-formatted help containing the 6 local flags + global flags.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/cli"
	"github.com/dvmrry/zscalerctl/internal/config"
	"github.com/dvmrry/zscalerctl/internal/dump"
	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

// newDiffApp returns an App wired to in-memory buffers for diff tests.
// catalog overrides the default catalog so tests use minimal fixtures only.
func newDiffApp(t *testing.T, catalog resources.ResourceCatalog, env []string) (*cli.App, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	var out, errBuf bytes.Buffer
	var a *cli.App
	if len(catalog) > 0 {
		a = cli.NewWithOptions(&out, &errBuf, env, cli.Options{Catalog: catalog})
	} else {
		a = cli.New(&out, &errBuf, env)
	}
	return a, &out, &errBuf
}

// diffTestSpec returns a minimal ResourceSpec for diff fixture tests.
func diffTestSpec() resources.ResourceSpec {
	return resources.ResourceSpec{
		Product:    resources.ProductZIA,
		Name:       "locations",
		Operations: resources.ReadOperations(),
		Fields: []resources.FieldSpec{
			{Name: "id", Classification: resources.ClassOperational},
			{Name: "name", Classification: resources.ClassTenantConfig},
		},
	}
}

// diffTestFixture is one set of records to write into a dump directory.
type diffTestFixture struct {
	spec    resources.ResourceSpec
	payload string // JSON array of records
}

// writeDiffDump creates a minimal valid dump directory for diff testing.
func writeDiffDump(t *testing.T, fixture diffTestFixture) string {
	t.Helper()
	dir := t.TempDir()
	relPath := filepath.ToSlash(
		filepath.Join("resources", string(fixture.spec.Product), fixture.spec.Name+".json"),
	)
	path := filepath.Join(dir, filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("os.MkdirAll(%s) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(fixture.payload), 0o600); err != nil {
		t.Fatalf("os.WriteFile(%s) error = %v", path, err)
	}
	manifest := dump.Manifest{
		Schema:      dump.ManifestSchemaID,
		CollectedAt: "2026-01-01T00:00:00Z",
		ToolVersion: "test",
		Redaction:   string(redact.ModeStandard),
		Warning:     "test fixture",
		Status:      "complete",
		Resources: []dump.ManifestResource{
			{
				Product: string(fixture.spec.Product),
				Name:    fixture.spec.Name,
				Shape:   string(fixture.spec.EffectiveShape()),
				Status:  "ok",
				Path:    relPath,
				Records: 1,
			},
		},
	}
	body, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatalf("json.MarshalIndent(manifest) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), body, 0o600); err != nil {
		t.Fatalf("os.WriteFile(manifest) error = %v", err)
	}
	return dir
}

// TestCobraDiff_NoDrift confirms that diffing two identical dumps exits 0 and
// emits a report (with schema zscalerctl.diff.v1) with no drift.
func TestCobraDiff_NoDrift(t *testing.T) {
	t.Parallel()

	spec := diffTestSpec()
	catalog := resources.ResourceCatalog{spec}
	samePayload := `[{"id":"1","name":"stable"}]`
	oldDir := writeDiffDump(t, diffTestFixture{spec: spec, payload: samePayload})
	newDir := writeDiffDump(t, diffTestFixture{spec: spec, payload: samePayload})
	app, out, errBuf := newDiffApp(t, catalog, nil)

	err := app.Run(context.Background(), []string{"--format", "json", "diff", oldDir, newDir})
	if err != nil {
		t.Fatalf("App.Run(diff, no drift) error = %v, want nil", err)
	}
	if errBuf.Len() != 0 {
		t.Errorf("App.Run(diff, no drift) stderr = %q, want empty", errBuf.String())
	}
	var report struct {
		Schema  string `json:"schema"`
		Summary struct {
			RecordsChanged int `json:"records_changed"`
		} `json:"summary"`
	}
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatalf("json.Unmarshal(diff report) error = %v; output=%q", err, out.String())
	}
	if report.Schema != "zscalerctl.diff.v1" {
		t.Errorf("diff schema = %q, want zscalerctl.diff.v1", report.Schema)
	}
	if report.Summary.RecordsChanged != 0 {
		t.Errorf("records_changed = %d, want 0 (no drift)", report.Summary.RecordsChanged)
	}
}

// TestCobraDiff_FailOnDrift confirms that --fail-on-drift with drifted dumps
// returns DriftDetectedError (errors.Is ErrDriftDetected).
func TestCobraDiff_FailOnDrift(t *testing.T) {
	t.Parallel()

	spec := diffTestSpec()
	catalog := resources.ResourceCatalog{spec}
	oldDir := writeDiffDump(t, diffTestFixture{spec: spec, payload: `[{"id":"1","name":"old"}]`})
	newDir := writeDiffDump(t, diffTestFixture{spec: spec, payload: `[{"id":"1","name":"new"}]`})
	app, _, errBuf := newDiffApp(t, catalog, nil)

	err := app.Run(context.Background(), []string{
		"--format", "json",
		"diff", oldDir, newDir, "--fail-on-drift",
	})
	if !errors.Is(err, cli.ErrDriftDetected) {
		t.Fatalf("App.Run(diff --fail-on-drift) error = %v, want ErrDriftDetected", err)
	}
	if errBuf.Len() != 0 {
		t.Errorf("App.Run(diff --fail-on-drift) stderr = %q, want empty", errBuf.String())
	}
}

// TestCobraDiff_Detail confirms that --detail includes record-level detail
// strings that are absent from the default (non-detail) table output.
func TestCobraDiff_Detail(t *testing.T) {
	t.Parallel()

	spec := diffTestSpec()
	catalog := resources.ResourceCatalog{spec}
	oldDir := writeDiffDump(t, diffTestFixture{spec: spec, payload: `[{"id":"1","name":"old"}]`})
	newDir := writeDiffDump(t, diffTestFixture{spec: spec, payload: `[{"id":"1","name":"new"}]`})

	// Without --detail: no record-level rows.
	appNoDetail, outNoDetail, _ := newDiffApp(t, catalog, nil)
	if err := appNoDetail.Run(context.Background(), []string{"--format", "table", "diff", oldDir, newDir}); err != nil {
		t.Fatalf("App.Run(diff, no detail) error = %v, want nil", err)
	}

	// With --detail: record-level rows must appear.
	appDetail, outDetail, _ := newDiffApp(t, catalog, nil)
	if err := appDetail.Run(context.Background(), []string{"--format", "table", "diff", oldDir, newDir, "--detail"}); err != nil {
		t.Fatalf("App.Run(diff --detail) error = %v, want nil", err)
	}

	// The detail output must be longer (has extra rows) than the non-detail output.
	if outDetail.Len() <= outNoDetail.Len() {
		t.Errorf("--detail output (%d bytes) is not longer than non-detail output (%d bytes); detail rows missing",
			outDetail.Len(), outNoDetail.Len())
	}
	// The detail output must reference the changed field "name".
	if !strings.Contains(outDetail.String(), "name") {
		t.Errorf("--detail output = %q, want 'name' in detail rows", outDetail.String())
	}
}

// TestCobraDiff_OnePositional confirms that exactly 1 positional arg returns a
// UsageError containing the diffUsage string.
func TestCobraDiff_OnePositional(t *testing.T) {
	t.Parallel()

	app, _, _ := newDiffApp(t, nil, nil)
	err := app.Run(context.Background(), []string{"diff", "/some/dir"})
	if err == nil {
		t.Fatal("App.Run(diff onlyonedir) error = nil, want UsageError")
	}
	if !errors.Is(err, cli.ErrUsage) {
		t.Errorf("App.Run(diff onlyonedir) error = %v, want ErrUsage (exit 2)", err)
	}
	if !strings.Contains(err.Error(), "usage: zscalerctl diff") {
		t.Errorf("App.Run(diff onlyonedir) error = %q, want diffUsage message", err.Error())
	}
}

// TestCobraDiff_ThreePositionals confirms that 3 positional args also returns
// a UsageError.
func TestCobraDiff_ThreePositionals(t *testing.T) {
	t.Parallel()

	app, _, _ := newDiffApp(t, nil, nil)
	err := app.Run(context.Background(), []string{"diff", "a", "b", "c"})
	if err == nil {
		t.Fatal("App.Run(diff a b c) error = nil, want UsageError")
	}
	if !errors.Is(err, cli.ErrUsage) {
		t.Errorf("App.Run(diff a b c) error = %v, want ErrUsage (exit 2)", err)
	}
}

// TestCobraDiff_FormatNDJSON_Rejected confirms that --format ndjson diff a b
// returns a UsageError (rejectUnsupportedFormat → exit 2) before any file work.
func TestCobraDiff_FormatNDJSON_Rejected(t *testing.T) {
	t.Parallel()

	app, _, _ := newDiffApp(t, nil, nil)
	err := app.Run(context.Background(), []string{"--format", "ndjson", "diff", "/fake/old", "/fake/new"})
	if err == nil {
		t.Fatal("App.Run(--format ndjson diff a b) error = nil, want UsageError")
	}
	if !errors.Is(err, cli.ErrUsage) {
		t.Errorf("App.Run(--format ndjson diff) error = %v, want ErrUsage (exit 2)", err)
	}
	if !strings.Contains(err.Error(), "ndjson") {
		t.Errorf("App.Run(--format ndjson diff) error = %q, want ndjson mentioned", err.Error())
	}
}

// TestCobraDiff_InvalidDumpDir confirms that an invalid/missing dump dir
// is mapped to UsageError (ErrInvalidDump → UsageError mapping preserved).
func TestCobraDiff_InvalidDumpDir(t *testing.T) {
	t.Parallel()

	app, _, _ := newDiffApp(t, nil, nil)
	err := app.Run(context.Background(), []string{
		"--format", "json", "diff",
		"/nonexistent/old-dump", "/nonexistent/new-dump",
	})
	if err == nil {
		t.Fatal("App.Run(diff nonexistent dirs) error = nil, want UsageError")
	}
	if !errors.Is(err, cli.ErrUsage) {
		t.Errorf("App.Run(diff nonexistent dirs) error = %v, want ErrUsage (ErrInvalidDump→UsageError mapping)", err)
	}
}

// TestCobraDiff_AllowPartial confirms that --allow-partial changes the partial-dump
// rejection: without it, a partial dump is rejected (UsageError); with it, Compare
// proceeds normally (no UsageError for partial input).
func TestCobraDiff_AllowPartial(t *testing.T) {
	t.Parallel()

	spec := diffTestSpec()
	catalog := resources.ResourceCatalog{spec}

	// Write a partial dump (status "partial" rather than "complete").
	writePartialDump := func(t *testing.T, payload string) string {
		t.Helper()
		dir := t.TempDir()
		relPath := filepath.ToSlash(
			filepath.Join("resources", string(spec.Product), spec.Name+".json"),
		)
		path := filepath.Join(dir, filepath.FromSlash(relPath))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatalf("os.MkdirAll error = %v", err)
		}
		if err := os.WriteFile(path, []byte(payload), 0o600); err != nil {
			t.Fatalf("os.WriteFile error = %v", err)
		}
		manifest := dump.Manifest{
			Schema:      dump.ManifestSchemaID,
			CollectedAt: "2026-01-01T00:00:00Z",
			ToolVersion: "test",
			Redaction:   string(redact.ModeStandard),
			Warning:     "partial test",
			Status:      "partial", // <-- partial, not complete
			Resources: []dump.ManifestResource{
				{
					Product: string(spec.Product),
					Name:    spec.Name,
					Shape:   string(spec.EffectiveShape()),
					Status:  "ok",
					Path:    relPath,
					Records: 1,
				},
			},
		}
		body, err := json.MarshalIndent(manifest, "", "  ")
		if err != nil {
			t.Fatalf("json.MarshalIndent error = %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "manifest.json"), body, 0o600); err != nil {
			t.Fatalf("os.WriteFile(manifest) error = %v", err)
		}
		return dir
	}

	oldDir := writePartialDump(t, `[{"id":"1","name":"old"}]`)
	newDir := writePartialDump(t, `[{"id":"1","name":"new"}]`)

	// Without --allow-partial: must return UsageError (ErrPartialDumpInput mapping).
	appNoPartial, _, _ := newDiffApp(t, catalog, nil)
	errNoPartial := appNoPartial.Run(context.Background(), []string{
		"--format", "json", "diff", oldDir, newDir,
	})
	if !errors.Is(errNoPartial, cli.ErrUsage) {
		t.Errorf("App.Run(diff partial, no --allow-partial) error = %v, want ErrUsage (ErrPartialDumpInput→UsageError)", errNoPartial)
	}

	// With --allow-partial: must NOT return ErrUsage; Compare proceeds.
	appPartial, _, _ := newDiffApp(t, catalog, nil)
	errPartial := appPartial.Run(context.Background(), []string{
		"--format", "json", "diff", oldDir, newDir, "--allow-partial",
	})
	if errors.Is(errPartial, cli.ErrUsage) {
		t.Errorf("App.Run(diff partial, --allow-partial) error = ErrUsage, want Compare to proceed")
	}
}

// TestCobraDiff_ModeStandardRedactionPreserved confirms that diff always uses
// ModeStandard redaction regardless of any configured (global --redaction) mode.
// We verify this by checking that both invocations produce the same report bytes.
func TestCobraDiff_ModeStandardRedactionPreserved(t *testing.T) {
	t.Parallel()

	spec := diffTestSpec()
	catalog := resources.ResourceCatalog{spec}
	oldDir := writeDiffDump(t, diffTestFixture{spec: spec, payload: `[{"id":"1","name":"old"}]`})
	newDir := writeDiffDump(t, diffTestFixture{spec: spec, payload: `[{"id":"1","name":"new"}]`})

	// Run once with no --redaction (defaults to ModeStandard).
	appDefault, outDefault, _ := newDiffApp(t, catalog, nil)
	if err := appDefault.Run(context.Background(), []string{"--format", "json", "diff", oldDir, newDir}); err != nil {
		t.Fatalf("App.Run(diff, default redaction) error = %v", err)
	}

	// Run again with --redaction paranoid (should produce the same output because
	// runDiffWithOptions always uses ModeStandard, not the configured mode).
	appParanoid, outParanoid, _ := newDiffApp(t, catalog, nil)
	if err := appParanoid.Run(context.Background(), []string{"--redaction", "paranoid", "--format", "json", "diff", oldDir, newDir}); err != nil {
		t.Fatalf("App.Run(diff, --redaction paranoid) error = %v", err)
	}

	if outDefault.String() != outParanoid.String() {
		t.Errorf("diff with --redaction paranoid produced different output than default:\ndefault:\n%s\nparanoid:\n%s",
			outDefault.String(), outParanoid.String())
	}
}

// TestCobraDiff_LocalFlagsSurviveSplitGlobalArgs confirms that local diff flags
// (--products, --detail) survive the splitGlobalArgs pass and that positional
// args (the two dirs) are correctly identified.
func TestCobraDiff_LocalFlagsSurviveSplitGlobalArgs(t *testing.T) {
	t.Parallel()

	spec := diffTestSpec()
	catalog := resources.ResourceCatalog{spec}
	// Use different payloads so --detail has drift to render.
	oldDir := writeDiffDump(t, diffTestFixture{spec: spec, payload: `[{"id":"1","name":"old"}]`})
	newDir := writeDiffDump(t, diffTestFixture{spec: spec, payload: `[{"id":"1","name":"new"}]`})
	app, out, _ := newDiffApp(t, catalog, nil)

	// Mix global flags (--format) with local diff flags (--products, --detail).
	// splitGlobalArgs must NOT strip --products (non-global despite the name)
	// or --detail (non-global bool), and must NOT swallow the positional dirs.
	err := app.Run(context.Background(), []string{
		"--format", "table",
		"diff",
		"--products", "zia",
		"--detail",
		oldDir, newDir,
	})
	if err != nil {
		t.Fatalf("App.Run(diff --products zia --detail dir1 dir2 through splitGlobalArgs) error = %v, want nil", err)
	}
	// --detail must have produced output.
	if out.Len() == 0 {
		t.Error("App.Run(diff --detail) stdout is empty; expected table output")
	}
}

// TestCobraDiff_HelpCobraFormatted confirms that diff --help exits 0 and renders
// Cobra-formatted help that includes all 6 local flags and does not leak credentials.
func TestCobraDiff_HelpCobraFormatted(t *testing.T) {
	t.Parallel()

	app, out, errBuf := newDiffApp(t, nil, []string{
		config.EnvClientSecretFile + "=/path/that/must/not-be-read",
	})
	err := app.Run(context.Background(), []string{"diff", "--help"})
	if err != nil {
		t.Fatalf("App.Run(diff --help) error = %v, want nil", err)
	}
	got := out.String()
	if got == "" {
		t.Fatal("App.Run(diff --help) stdout is empty; Cobra help should have been rendered")
	}
	// Cobra help must include "zscalerctl diff" in the Usage line.
	if !strings.Contains(got, "zscalerctl diff") {
		t.Errorf("App.Run(diff --help) stdout = %q, want 'zscalerctl diff'", got)
	}
	// All 6 local flags must appear.
	for _, flag := range []string{
		"--products", "--resources", "--ignore-operational",
		"--detail", "--allow-partial", "--fail-on-drift",
	} {
		if !strings.Contains(got, flag) {
			t.Errorf("App.Run(diff --help) stdout = %q, want flag %q", got, flag)
		}
	}
	// Global flags must appear too (inherited via persistent flags).
	if !strings.Contains(got, "Global Flags") {
		t.Errorf("App.Run(diff --help) stdout = %q, want 'Global Flags' section", got)
	}
	// Credential-like strings must not appear.
	if strings.Contains(got, "/path/that/must/not-be-read") {
		t.Errorf("App.Run(diff --help) stdout = %q, leaked secret path", got)
	}
	if errBuf.Len() != 0 {
		t.Errorf("App.Run(diff --help) stderr = %q, want empty", errBuf.String())
	}
}
