package cli_test

// cobra_dump_test.go — Tests for the Cobra-migrated dump command (Phase 3a).
//
// Tests here verify:
//  1. dump --out <dir> with a fake reader writes files; stdout empty; exit 0.
//  2. Local flags survive splitGlobalArgs: dump --out /tmp/x --products zia --force
//     parses correctly (key risk: value-taking --out stays intact through the global-strip pass).
//  3. dump (no --out) → UsageError (dumpUsage); dump extra-positional --out x → NArg!=0 UsageError.
//  4. Partial dump → exit 6 (ErrPartialDump): fake reader errors on one resource with
//     --continue-on-error; without that flag the collect fails appropriately.
//  5. --format ndjson dump --out x → exit-2/UsageError (rejectUnsupportedFormat).
//  6. --output <file> dump --out x → still rejected at the App.Run boundary ("cannot be used with dump").
//  7. dump --help → exit 0, Cobra-formatted help containing local flags (--out/--products/etc.).

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
	"github.com/dvmrry/zscalerctl/internal/resources"
)

// newDumpApp returns an App wired to in-memory buffers with an optional fake reader.
func newDumpApp(t *testing.T, reader cli.ResourceReader, env []string) (*cli.App, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	var out, errBuf bytes.Buffer
	var a *cli.App
	if reader != nil {
		a = cli.NewWithOptions(&out, &errBuf, env, cli.Options{Reader: reader})
	} else {
		a = cli.New(&out, &errBuf, env)
	}
	return a, &out, &errBuf
}

// TestCobraDump_BasicWritesFiles confirms that dump --out <dir> with a fake reader
// writes dump files, emits a "dump written" status to stderr, and leaves stdout empty.
func TestCobraDump_BasicWritesFiles(t *testing.T) {
	t.Parallel()

	reader := dumpFakeReader{
		list: []resources.SourceRecord{resources.NewSourceRecord(map[string]any{
			"id":   "1",
			"name": "test",
		})},
	}
	outDir := filepath.Join(t.TempDir(), "dump-out")
	app, out, errBuf := newDumpApp(t, reader, nil)

	err := app.Run(context.Background(), []string{"dump", "--out", outDir})
	if err != nil {
		t.Fatalf("App.Run(dump --out) error = %v, want nil", err)
	}
	if out.Len() != 0 {
		t.Errorf("App.Run(dump --out) stdout = %q, want empty", out.String())
	}
	if !strings.Contains(errBuf.String(), "dump written: "+outDir) {
		t.Errorf("App.Run(dump --out) stderr = %q, want 'dump written: %s'", errBuf.String(), outDir)
	}
	// manifest.json must exist (dump.Write creates it).
	if _, err := os.Stat(filepath.Join(outDir, "manifest.json")); err != nil {
		t.Errorf("os.Stat(manifest.json) = %v, want nil", err)
	}
}

// TestCobraDump_LocalFlagsSurviveSplitGlobalArgs is the KEY RISK test: it verifies
// that local dump flags (especially value-taking --out and --products which look like
// global "products"/"resources" but are NOT global) are passed through splitGlobalArgs
// correctly and parsed by Cobra. If splitGlobalArgs stripped --out or --products
// as a global flag, the value would be lost and the command would fail with a usage error.
func TestCobraDump_LocalFlagsSurviveSplitGlobalArgs(t *testing.T) {
	t.Parallel()

	reader := dumpFakeReader{
		list: []resources.SourceRecord{resources.NewSourceRecord(map[string]any{
			"id":   "1",
			"name": "test",
		})},
	}
	outDir := filepath.Join(t.TempDir(), "split-global-dump")
	app, out, errBuf := newDumpApp(t, reader, nil)

	// This invocation mixes global flags (--format) with local dump flags (--out,
	// --products, --force). splitGlobalArgs must NOT strip --out (value-taking
	// non-global) or --products (non-global, despite sounding global-ish).
	err := app.Run(context.Background(), []string{
		"--format", "table",
		"dump",
		"--out", outDir,
		"--products", "zia",
		"--force",
	})
	if err != nil {
		t.Fatalf("App.Run(dump --out --products --force through splitGlobalArgs) error = %v, want nil", err)
	}
	if out.Len() != 0 {
		t.Errorf("App.Run(dump local flags through splitGlobalArgs) stdout = %q, want empty", out.String())
	}
	if !strings.Contains(errBuf.String(), "dump written: "+outDir) {
		t.Errorf("App.Run(dump local flags through splitGlobalArgs) stderr = %q, want 'dump written: %s'", errBuf.String(), outDir)
	}
}

// TestCobraDump_NoOutFlag confirms that dump without --out returns a UsageError
// with the dumpUsage string.
func TestCobraDump_NoOutFlag(t *testing.T) {
	t.Parallel()

	app, _, _ := newDumpApp(t, nil, nil)
	err := app.Run(context.Background(), []string{"dump"})
	if err == nil {
		t.Fatal("App.Run(dump, no --out) error = nil, want UsageError")
	}
	if !errors.Is(err, cli.ErrUsage) {
		t.Errorf("App.Run(dump, no --out) error = %v, want ErrUsage (exit 2)", err)
	}
	if !strings.Contains(err.Error(), "usage: zscalerctl dump") {
		t.Errorf("App.Run(dump, no --out) error = %q, want dumpUsage message", err.Error())
	}
}

// TestCobraDump_ExtraPositionalArg confirms that dump with a positional arg (NArg != 0)
// returns a UsageError with the dumpUsage string.
func TestCobraDump_ExtraPositionalArg(t *testing.T) {
	t.Parallel()

	outDir := filepath.Join(t.TempDir(), "dump-unused")
	app, _, _ := newDumpApp(t, nil, nil)
	err := app.Run(context.Background(), []string{"dump", "extra-positional", "--out", outDir})
	if err == nil {
		t.Fatal("App.Run(dump extra-positional --out x) error = nil, want UsageError")
	}
	if !errors.Is(err, cli.ErrUsage) {
		t.Errorf("App.Run(dump extra-positional --out x) error = %v, want ErrUsage (exit 2)", err)
	}
}

// TestCobraDump_PartialDumpExitsWithErrPartialDump confirms that when a fake reader
// errors on one resource with --continue-on-error, the command returns ErrPartialDump
// (errors.Is == cli.ErrPartialDump; exit 6).
func TestCobraDump_PartialDumpExitsWithErrPartialDump(t *testing.T) {
	t.Parallel()

	reader := dumpSelectiveErrorReader{
		list: []resources.SourceRecord{resources.NewSourceRecord(map[string]any{
			"id":   "1",
			"name": "test",
		})},
		failures: map[string]error{
			"zia/rule-labels": errors.New("simulated-failure"),
		},
	}
	outDir := filepath.Join(t.TempDir(), "partial-dump")
	app, out, errBuf := newDumpApp(t, reader, nil)

	err := app.Run(context.Background(), []string{
		"dump",
		"--products", "zia",
		"--resources", "locations,rule-labels",
		"--continue-on-error",
		"--out", outDir,
	})
	if !errors.Is(err, cli.ErrPartialDump) {
		t.Fatalf("App.Run(dump --continue-on-error) error = %v, want ErrPartialDump (exit 6)", err)
	}
	if !strings.Contains(errBuf.String(), "partial dump written: "+outDir) {
		t.Errorf("App.Run(dump --continue-on-error) stderr = %q, want partial dump written line", errBuf.String())
	}
	if out.Len() != 0 {
		t.Errorf("App.Run(dump --continue-on-error) stdout = %q, want empty", out.String())
	}
}

// TestCobraDump_WithoutContinueOnErrorFailsFast confirms that without --continue-on-error,
// a reader error causes the collect phase to fail immediately (not ErrPartialDump).
func TestCobraDump_WithoutContinueOnErrorFailsFast(t *testing.T) {
	t.Parallel()

	reader := dumpSelectiveErrorReader{
		list: []resources.SourceRecord{resources.NewSourceRecord(map[string]any{
			"id":   "1",
			"name": "test",
		})},
		failures: map[string]error{
			"zia/rule-labels": errors.New("simulated-failure"),
		},
	}
	outDir := filepath.Join(t.TempDir(), "fail-fast-dump")
	app, _, _ := newDumpApp(t, reader, nil)

	err := app.Run(context.Background(), []string{
		"dump",
		"--products", "zia",
		"--resources", "locations,rule-labels",
		"--out", outDir,
	})
	if err == nil {
		t.Fatal("App.Run(dump without --continue-on-error, reader fails) error = nil, want error")
	}
	// Must NOT be ErrPartialDump — that only fires when --continue-on-error is set
	// and the partial result is written. Without it, the collect fails immediately.
	if errors.Is(err, cli.ErrPartialDump) {
		t.Errorf("App.Run(dump without --continue-on-error) error = ErrPartialDump, want plain error from reader failure")
	}
}

// TestCobraDump_FormatNDJSON_Rejected confirms that --format ndjson dump --out x
// returns a UsageError (rejectUnsupportedFormat → exit 2), before any config load.
func TestCobraDump_FormatNDJSON_Rejected(t *testing.T) {
	t.Parallel()

	outDir := filepath.Join(t.TempDir(), "ndjson-dump-unused")
	app, _, _ := newDumpApp(t, nil, nil)
	err := app.Run(context.Background(), []string{"--format", "ndjson", "dump", "--out", outDir})
	if err == nil {
		t.Fatal("App.Run(--format ndjson dump --out x) error = nil, want UsageError")
	}
	if !errors.Is(err, cli.ErrUsage) {
		t.Errorf("App.Run(--format ndjson dump --out x) error = %v, want ErrUsage (exit 2)", err)
	}
	if !strings.Contains(err.Error(), "ndjson") {
		t.Errorf("App.Run(--format ndjson dump) error = %q, want ndjson mentioned", err.Error())
	}
}

// TestCobraDump_OutputGlobalFlagRejected is a regression guard confirming that
// --output <file> dump --out x is still rejected at the App.Run boundary
// ("--output cannot be used with dump"). This check runs before Cobra dispatch
// and must remain functional even after the migration.
func TestCobraDump_OutputGlobalFlagRejected(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	outFile := filepath.Join(tmpDir, "out.txt")
	dumpDir := filepath.Join(tmpDir, "dump-unused")
	app, out, errBuf := newDumpApp(t, nil, nil)

	err := app.Run(context.Background(), []string{"--output", outFile, "dump", "--out", dumpDir})
	if err == nil {
		t.Fatal("App.Run(--output <file> dump --out x) error = nil, want UsageError")
	}
	if !errors.Is(err, cli.ErrUsage) {
		t.Errorf("App.Run(--output dump) error = %v, want ErrUsage (exit 2)", err)
	}
	if !strings.Contains(err.Error(), "--output cannot be used with dump") {
		t.Errorf("App.Run(--output dump) error = %q, want '--output cannot be used with dump'", err.Error())
	}
	if out.Len() != 0 {
		t.Errorf("App.Run(--output dump) stdout = %q, want empty", out.String())
	}
	if errBuf.Len() != 0 {
		t.Errorf("App.Run(--output dump) stderr = %q, want empty", errBuf.String())
	}
	// The dump directory must not have been created.
	if _, statErr := os.Stat(dumpDir); !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("os.Stat(%q) = %v, want os.ErrNotExist", dumpDir, statErr)
	}
}

// TestCobraDump_HelpCobraFormatted confirms that dump --help exits 0 and renders
// Cobra-formatted help that includes the local flags (--out, --products, --resources,
// --continue-on-error, --force) and does not leak credentials.
func TestCobraDump_HelpCobraFormatted(t *testing.T) {
	t.Parallel()

	app, out, errBuf := newDumpApp(t, nil, []string{
		config.EnvClientSecretFile + "=/path/that/must/not-be-read",
	})
	err := app.Run(context.Background(), []string{"dump", "--help"})
	if err != nil {
		t.Fatalf("App.Run(dump --help) error = %v, want nil", err)
	}
	got := out.String()
	if got == "" {
		t.Fatal("App.Run(dump --help) stdout is empty; Cobra help should have been rendered")
	}
	// Cobra help must include "zscalerctl dump" in the Usage line.
	if !strings.Contains(got, "zscalerctl dump") {
		t.Errorf("App.Run(dump --help) stdout = %q, want 'zscalerctl dump'", got)
	}
	// All local flags must appear.
	for _, flag := range []string{"--out", "--products", "--resources", "--continue-on-error", "--force"} {
		if !strings.Contains(got, flag) {
			t.Errorf("App.Run(dump --help) stdout = %q, want flag %q", got, flag)
		}
	}
	// Credential-like strings must not appear.
	if strings.Contains(got, "/path/that/must/not-be-read") {
		t.Errorf("App.Run(dump --help) stdout = %q, leaked secret path", got)
	}
	if errBuf.Len() != 0 {
		t.Errorf("App.Run(dump --help) stderr = %q, want empty", errBuf.String())
	}
}

// TestCobraDump_ConfigError confirms that the migrated dump command surfaces a config
// load error (ErrInvalidConfig → exit 2) when --config points to a nonexistent path.
func TestCobraDump_ConfigError(t *testing.T) {
	t.Parallel()

	outDir := filepath.Join(t.TempDir(), "dump-config-err")
	app, _, _ := newDumpApp(t, nil, nil)
	err := app.Run(context.Background(), []string{
		"--config", "/nonexistent/path/zscalerctl.yaml",
		"dump", "--out", outDir,
	})
	if err == nil {
		t.Fatal("App.Run(dump --config /nonexistent) error = nil, want config error")
	}
	if !errors.Is(err, config.ErrInvalidConfig) {
		t.Errorf("App.Run(dump --config /nonexistent) error = %v, want ErrInvalidConfig", err)
	}
}

// TestCobraDump_DashPrefixedOutValue_Terminator confirms that
// "dump --out -- -weird-path" treats -weird-path as the value of --out, not as
// an unknown flag. The dump should proceed to the output directory named
// literally -weird-path.
func TestCobraDump_DashPrefixedOutValue_Terminator(t *testing.T) {
	t.Parallel()

	reader := dumpFakeReader{
		list: []resources.SourceRecord{resources.NewSourceRecord(map[string]any{
			"id":   "1",
			"name": "test",
		})},
	}
	outDir := "-weird-path"
	app, out, errBuf := newDumpApp(t, reader, nil)

	// Use an absolute path so the dump creates the literal -weird-path directory
	// inside a temp dir without changing the process working directory (this test
	// runs in parallel with other tests).
	outDir = filepath.Join(t.TempDir(), outDir)

	err := app.Run(context.Background(), []string{"dump", "--out", "--", outDir})
	if err != nil {
		t.Fatalf("App.Run(dump --out -- -weird-path) error = %v, want nil", err)
	}
	if out.Len() != 0 {
		t.Errorf("App.Run(dump --out -- -weird-path) stdout = %q, want empty", out.String())
	}
	if !strings.Contains(errBuf.String(), "dump written: "+outDir) {
		t.Errorf("App.Run(dump --out -- -weird-path) stderr = %q, want 'dump written: %s'", errBuf.String(), outDir)
	}
	if _, err := os.Stat(filepath.Join(outDir, "manifest.json")); err != nil {
		t.Errorf("os.Stat(%s/manifest.json) = %v, want nil", outDir, err)
	}
}

// TestCobraDump_BoolFlag_TerminatorStillProtectsPositionals confirms that when
// the token before "--" is a boolean local flag (e.g. --force), the terminator
// is still reinserted so the following dash-prefixed token is treated as a
// positional, not a flag. The command should fail with an extra-positional usage
// error, not an unknown flag error.
func TestCobraDump_BoolFlag_TerminatorStillProtectsPositionals(t *testing.T) {
	t.Parallel()

	app, _, _ := newDumpApp(t, nil, nil)
	err := app.Run(context.Background(), []string{"dump", "--force", "--", "-extra"})
	if err == nil {
		t.Fatal("App.Run(dump --force -- -extra) error = nil, want UsageError")
	}
	if !errors.Is(err, cli.ErrUsage) {
		t.Errorf("App.Run(dump --force -- -extra) error = %v, want ErrUsage (extra positional)", err)
	}
	if strings.Contains(err.Error(), "unknown flag") || strings.Contains(err.Error(), "unknown shorthand") {
		t.Errorf("App.Run(dump --force -- -extra) error = %q, want not a flag error", err.Error())
	}
}

// ── local fake readers for cobra_dump_test.go ────────────────────────────────

// dumpFakeReader implements cli.ResourceReader for dump tests.
// It always succeeds, returning the same list for every List call.
type dumpFakeReader struct {
	list []resources.SourceRecord
}

func (r dumpFakeReader) List(_ context.Context, _ resources.Product, _ string) ([]resources.SourceRecord, error) {
	return r.list, nil
}

func (r dumpFakeReader) Get(_ context.Context, _ resources.Product, _ string, _ string) (resources.SourceRecord, error) {
	return resources.SourceRecord{}, nil
}

func (r dumpFakeReader) Show(_ context.Context, _ resources.Product, _ string) (resources.SourceRecord, error) {
	return resources.SourceRecord{}, nil
}

// dumpSelectiveErrorReader returns an error for specific "product/resource" keys.
type dumpSelectiveErrorReader struct {
	list     []resources.SourceRecord
	failures map[string]error
}

func (r dumpSelectiveErrorReader) List(_ context.Context, product resources.Product, name string) ([]resources.SourceRecord, error) {
	if err := r.failures[string(product)+"/"+name]; err != nil {
		return nil, err
	}
	return r.list, nil
}

func (r dumpSelectiveErrorReader) Get(_ context.Context, _ resources.Product, _ string, _ string) (resources.SourceRecord, error) {
	return resources.SourceRecord{}, errors.New("get must not be called")
}

func (r dumpSelectiveErrorReader) Show(_ context.Context, _ resources.Product, _ string) (resources.SourceRecord, error) {
	return resources.SourceRecord{}, errors.New("show must not be called")
}
