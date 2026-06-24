package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/cli"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

func TestIntrospectTree(t *testing.T) {
	a := cli.New(io.Discard, io.Discard, nil)
	doc := cli.IntrospectTree(a)

	// Contract version
	if doc.IntrospectVersion != "1" {
		t.Errorf("IntrospectVersion = %q, want %q", doc.IntrospectVersion, "1")
	}

	// CLI-wide read-only guarantee
	if !doc.ReadOnly {
		t.Error("ReadOnly = false, want true")
	}

	// CLIVersion is left empty by IntrospectTree; the command fills it.
	if doc.CLIVersion != "" {
		t.Errorf("CLIVersion = %q, want empty (set by command, not introspectTree)", doc.CLIVersion)
	}

	// Per-command `mutating` is the de-tautologized contract:
	//   - config init writes a LOCAL config file → Mutating must be true.
	//   - read-only commands (version, a product read like zia locations list)
	//     → Mutating must be false.
	// The CLI-wide read_only guarantee is tenant-scoped; `mutating` flags
	// local side effects, not tenant mutation.
	findByPath := func(path string) *cli.CommandDoc {
		for i := range doc.Commands {
			if doc.Commands[i].Path == path {
				return &doc.Commands[i]
			}
		}
		return nil
	}

	configInit := findByPath("config init")
	if configInit == nil {
		t.Fatal("command \"config init\" not found in doc.Commands")
	}
	if !configInit.Mutating {
		t.Errorf("config init: Mutating = false, want true (writes a local config file)")
	}

	for _, path := range []string{"version", "zia locations list"} {
		c := findByPath(path)
		if c == nil {
			t.Errorf("command %q not found in doc.Commands", path)
			continue
		}
		if c.Mutating {
			t.Errorf("command %q: Mutating = true, want false (read-only)", path)
		}
	}

	// GlobalFlags must match globalFlagDefs.
	if got := len(doc.GlobalFlags); got != len(cli.ExportedGlobalFlagDefs) {
		t.Errorf("len(GlobalFlags) = %d, want %d (len(globalFlagDefs))", got, len(cli.ExportedGlobalFlagDefs))
	}

	// A known command must be present: "zia locations list"
	var ziaLocList *cli.CommandDoc
	for i := range doc.Commands {
		if doc.Commands[i].Path == "zia locations list" {
			ziaLocList = &doc.Commands[i]
			break
		}
	}
	if ziaLocList == nil {
		paths := make([]string, 0, len(doc.Commands))
		for _, c := range doc.Commands {
			if strings.HasPrefix(c.Path, "zia") {
				paths = append(paths, c.Path)
			}
		}
		t.Fatalf("command %q not found; zia commands seen: %v", "zia locations list", paths)
	}

	// OutputFields must be non-empty for a resource read command.
	if len(ziaLocList.OutputFields) == 0 {
		t.Error("zia locations list: OutputFields is empty, want catalog field names")
	}

	// InheritedFlags must include a global like "format".
	found := false
	for _, f := range ziaLocList.InheritedFlags {
		if f == "format" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("zia locations list: InheritedFlags does not contain %q; got %v", "format", ziaLocList.InheritedFlags)
	}

	// Commands must be in alphabetical order by path at the top level.
	// Find "auth" and "version" (both are always present) and assert auth < version.
	authIdx, versionIdx := -1, -1
	for i, cmd := range doc.Commands {
		switch cmd.Path {
		case "auth":
			authIdx = i
		case "version":
			versionIdx = i
		}
	}
	if authIdx == -1 {
		t.Error("command \"auth\" not found in doc.Commands")
	}
	if versionIdx == -1 {
		t.Error("command \"version\" not found in doc.Commands")
	}
	if authIdx != -1 && versionIdx != -1 && authIdx > versionIdx {
		t.Errorf("commands not in alphabetical order: \"auth\" at index %d appears after \"version\" at index %d", authIdx, versionIdx)
	}

	// ExitCodes must be populated.
	if len(doc.ExitCodes) == 0 {
		t.Error("ExitCodes is empty, want at least one entry")
	}

	// Catalog must be populated with products and resources.
	if len(doc.Catalog.Products) == 0 {
		t.Error("Catalog.Products is empty")
	}
	if len(doc.Catalog.Resources) == 0 {
		t.Error("Catalog.Resources is empty")
	}
}

// TestIntrospectHelpArgsPolicyArbitrary asserts that the Cobra `help` command
// is documented as accepting an arbitrary number of positional arguments (e.g.
// `zscalerctl help config init`), not the range(N) fallback that the "[command]"
// Use suffix would otherwise produce.
func TestIntrospectHelpArgsPolicyArbitrary(t *testing.T) {
	t.Parallel()

	a := cli.New(io.Discard, io.Discard, nil)
	doc := cli.IntrospectTree(a)

	var helpCmd *cli.CommandDoc
	for i := range doc.Commands {
		if doc.Commands[i].Path == "help" {
			helpCmd = &doc.Commands[i]
			break
		}
	}
	if helpCmd == nil {
		t.Fatal("command \"help\" not found in introspect doc.Commands")
	}
	if helpCmd.Args.Policy != "arbitrary" {
		t.Errorf("help command args policy = %q, want %q", helpCmd.Args.Policy, "arbitrary")
	}
	if helpCmd.Args.N != 0 {
		t.Errorf("help command args N = %d, want 0 (omitted for arbitrary)", helpCmd.Args.N)
	}
}

// TestIntrospectCommand runs `introspect` via App.Run and verifies the JSON output.
func TestIntrospectCommand(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	a := cli.New(&out, io.Discard, nil)

	err := a.Run(context.Background(), []string{"introspect"})
	if err != nil {
		t.Fatalf("App.Run(introspect) error = %v, want nil", err)
	}

	// Output must be valid JSON.
	var doc map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
		t.Fatalf("introspect output is not valid JSON: %v\noutput: %s", err, out.String())
	}

	// Required top-level keys.
	for _, key := range []string{"introspect_version", "read_only", "commands", "global_flags", "catalog", "exit_codes"} {
		if _, ok := doc[key]; !ok {
			t.Errorf("introspect JSON missing top-level key %q", key)
		}
	}

	// cli_version must be present and non-empty (the command fills it).
	v, ok := doc["cli_version"]
	if !ok {
		t.Fatal("introspect JSON missing key \"cli_version\"")
	}
	vStr, isStr := v.(string)
	if !isStr {
		t.Fatalf("cli_version is not a string; got %T (%v)", v, v)
	}
	if strings.TrimSpace(vStr) == "" {
		t.Errorf("cli_version is empty; the command must set it from version.Current()")
	}
}

// TestIntrospectTableFormat exercises the human-readable tree path
// (--format table and --format pretty). It asserts no error and verifies
// stable structural landmarks emitted by introspectTreeText.
func TestIntrospectTableFormat(t *testing.T) {
	t.Parallel()

	for _, format := range []string{"table", "pretty"} {
		format := format
		t.Run(format, func(t *testing.T) {
			t.Parallel()

			var out bytes.Buffer
			a := cli.New(&out, io.Discard, nil)

			err := a.Run(context.Background(), []string{"--format", format, "introspect"})
			if err != nil {
				t.Fatalf("App.Run(--format %s introspect) error = %v, want nil", format, err)
			}

			got := out.String()

			// Header line — always the first line of introspectTreeText.
			if !strings.Contains(got, "zscalerctl CLI surface map") {
				t.Errorf("--format %s: output missing header \"zscalerctl CLI surface map\"; got:\n%s", format, got)
			}
			// Section labels that introspectTreeText always emits.
			for _, landmark := range []string{"Global flags (", "Commands (", "Catalog:", "Exit codes ("} {
				if !strings.Contains(got, landmark) {
					t.Errorf("--format %s: output missing landmark %q; got:\n%s", format, landmark, got)
				}
			}
		})
	}
}

// TestIntrospectRejectsNDJSON asserts that --format ndjson returns ErrUsage
// (introspect is a single document, not a record stream).
func TestIntrospectRejectsNDJSON(t *testing.T) {
	t.Parallel()

	a := cli.New(io.Discard, io.Discard, nil)

	err := a.Run(context.Background(), []string{"--format", "ndjson", "introspect"})
	if err == nil {
		t.Fatal("App.Run(--format ndjson introspect) error = nil, want ErrUsage")
	}
	if !errors.Is(err, cli.ErrUsage) {
		t.Errorf("App.Run(--format ndjson introspect) error = %v, want errors.Is(err, cli.ErrUsage)", err)
	}
}

// TestIntrospectConfigFree verifies that `introspect` succeeds without any
// environment variables or config file — it must not call LoadConfig.
func TestIntrospectConfigFree(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	// nil env map → no credentials, no config, no ZSCALERCTL_* variables.
	a := cli.New(&out, io.Discard, nil)

	err := a.Run(context.Background(), []string{"introspect"})
	if err != nil {
		t.Fatalf("App.Run(introspect) with empty env error = %v, want nil (command must be config-free)", err)
	}
	if out.Len() == 0 {
		t.Error("introspect produced no output")
	}
}

// neverCalledReader is a ResourceReader stub whose methods call t.Fatal if
// invoked. Injected into an App via NewWithOptions to prove introspect never
// touches the reader (no List/Get/Show calls means no credential or network
// access for any format path).
type neverCalledReader struct{ t *testing.T }

func (r neverCalledReader) List(_ context.Context, _ resources.Product, _ string) ([]resources.SourceRecord, error) {
	r.t.Fatal("introspect: ResourceReader.List must never be called")
	return nil, nil
}

func (r neverCalledReader) Get(_ context.Context, _ resources.Product, _ string, _ string) (resources.SourceRecord, error) {
	r.t.Fatal("introspect: ResourceReader.Get must never be called")
	return resources.SourceRecord{}, nil
}

func (r neverCalledReader) Show(_ context.Context, _ resources.Product, _ string) (resources.SourceRecord, error) {
	r.t.Fatal("introspect: ResourceReader.Show must never be called")
	return resources.SourceRecord{}, nil
}

// TestIntrospectNeverTouchesReader proves that all introspect format paths are
// config-free at the ResourceReader boundary. A reader stub whose methods call
// t.Fatal is injected into the App; if introspect ever calls List/Get/Show, the
// test fails immediately. This is stronger than TestIntrospectConfigFree (nil
// env) because it exercises all three output paths and asserts at the API
// boundary, not merely at the error-return level.
func TestIntrospectNeverTouchesReader(t *testing.T) {
	t.Parallel()

	formats := []string{"json", "table", "pretty"}
	for _, format := range formats {
		format := format
		t.Run(format, func(t *testing.T) {
			t.Parallel()

			stub := neverCalledReader{t: t}
			a := cli.NewWithOptions(io.Discard, io.Discard, nil, cli.Options{Reader: stub})

			args := []string{"--format", format, "introspect"}
			if err := a.Run(context.Background(), args); err != nil {
				t.Fatalf("App.Run(--format %s introspect) with reader stub error = %v, want nil", format, err)
			}
		})
	}
}
