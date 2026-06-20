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

	// CLIVersion is left empty by IntrospectTree; the command fills it (Task 1.2).
	if doc.CLIVersion != "" {
		t.Errorf("CLIVersion = %q, want empty (set by command, not introspectTree)", doc.CLIVersion)
	}

	// Every command must be non-mutating (all ops are read-only today).
	for _, cmd := range doc.Commands {
		if cmd.Mutating {
			t.Errorf("command %q has Mutating=true but CLI is read-only", cmd.Path)
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
	vStr, _ := v.(string)
	if strings.TrimSpace(vStr) == "" {
		t.Errorf("cli_version is empty; the command must set it from version.Current()")
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
