package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/cli"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

func TestIntrospectTree(t *testing.T) {
	a := cli.New(io.Discard, io.Discard, nil)
	doc := cli.IntrospectTree(a)

	// Contract version
	if doc.IntrospectVersion != "2" {
		t.Errorf("IntrospectVersion = %q, want %q", doc.IntrospectVersion, "2")
	}

	// CLI-wide read-only guarantee
	if !doc.ReadOnly {
		t.Error("ReadOnly = false, want true")
	}

	// CLIVersion is left empty by IntrospectTree; the command fills it.
	if doc.CLIVersion != "" {
		t.Errorf("CLIVersion = %q, want empty (set by command, not introspectTree)", doc.CLIVersion)
	}

	// The CLI-wide read_only guarantee is tenant-scoped. Per-command effects
	// describe local writes/deletes and network access, including side effects
	// that occur only when a flag is set. Mutating remains a conservative summary
	// for older consumers: any possible local or tenant mutation makes it true.
	findByPath := func(path string) *cli.CommandDoc {
		for i := range doc.Commands {
			if doc.Commands[i].Path == path {
				return &doc.Commands[i]
			}
		}
		return nil
	}

	effectCases := []struct {
		path string
		want []cli.EffectDoc
	}{
		{
			path: "config init",
			want: []cli.EffectDoc{
				{Kind: "local_filesystem_delete", When: "flag_set", Flag: "force"},
				{Kind: "local_filesystem_write", When: "always"},
				{Kind: "local_filesystem_write", When: "flag_set", Flag: "output"},
				{Kind: "process_execution", When: "configuration_dependent"},
			},
		},
		{
			path: "dump",
			want: []cli.EffectDoc{
				{Kind: "local_filesystem_delete", When: "flag_set", Flag: "force"},
				{Kind: "local_filesystem_read", When: "configuration_dependent"},
				{Kind: "local_filesystem_read", When: "flag_set", Flag: "force"},
				{Kind: "local_filesystem_write", When: "always"},
				{Kind: "network_access", When: "always"},
				{Kind: "process_execution", When: "configuration_dependent"},
			},
		},
		{
			path: "version",
			want: []cli.EffectDoc{
				{Kind: "local_filesystem_write", When: "flag_set", Flag: "output"},
			},
		},
		{
			path: "machine manifest",
			want: []cli.EffectDoc{
				{Kind: "local_filesystem_write", When: "flag_set", Flag: "output"},
			},
		},
		{
			path: "diff",
			want: []cli.EffectDoc{
				{Kind: "local_filesystem_read", When: "always"},
				{Kind: "local_filesystem_write", When: "flag_set", Flag: "output"},
			},
		},
		{
			path: "doctor",
			want: []cli.EffectDoc{
				{Kind: "local_filesystem_read", When: "configuration_dependent"},
				{Kind: "local_filesystem_write", When: "flag_set", Flag: "output"},
			},
		},
		{
			path: "zia locations list",
			want: []cli.EffectDoc{
				{Kind: "local_filesystem_read", When: "configuration_dependent"},
				{Kind: "local_filesystem_write", When: "flag_set", Flag: "output"},
				{Kind: "network_access", When: "always"},
				{Kind: "process_execution", When: "configuration_dependent"},
			},
		},
		{
			path: "zia url-lookup",
			want: []cli.EffectDoc{
				{Kind: "local_filesystem_read", When: "configuration_dependent"},
				{Kind: "local_filesystem_write", When: "flag_set", Flag: "output"},
				{Kind: "network_access", When: "always"},
				{Kind: "process_execution", When: "configuration_dependent"},
			},
		},
	}
	for _, tc := range effectCases {
		got := findByPath(tc.path)
		if got == nil {
			t.Errorf("command %q not found in doc.Commands", tc.path)
			continue
		}
		if !slices.Equal(got.Effects, tc.want) {
			t.Errorf("command %q effects = %#v, want %#v", tc.path, got.Effects, tc.want)
		}
		if !got.Mutating {
			t.Errorf("command %q mutating = false, want true for possible local filesystem effects", tc.path)
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

func TestIntrospectEffectsAreValid(t *testing.T) {
	t.Parallel()

	doc := cli.IntrospectTree(cli.New(io.Discard, io.Discard, nil))
	validKinds := map[string]bool{
		"tenant_mutation":         true,
		"local_filesystem_read":   true,
		"local_filesystem_write":  true,
		"local_filesystem_delete": true,
		"network_access":          true,
		"process_execution":       true,
	}

	for _, command := range doc.Commands {
		availableFlags := make(map[string]bool, len(command.Flags)+len(command.InheritedFlags))
		for _, flag := range command.Flags {
			availableFlags[flag.Name] = true
		}
		for _, flag := range command.InheritedFlags {
			availableFlags[flag] = true
		}

		seen := make(map[cli.EffectDoc]bool, len(command.Effects))
		mutating := false
		hasNetworkAccess := false
		hasConfiguredLocalRead := false
		hasConfiguredProcessExecution := false
		for _, effect := range command.Effects {
			if !validKinds[effect.Kind] {
				t.Errorf("command %q effect kind = %q, want a published effect kind", command.Path, effect.Kind)
			}
			if seen[effect] {
				t.Errorf("command %q effects contain duplicate %#v", command.Path, effect)
			}
			seen[effect] = true

			switch effect.When {
			case "always":
				if effect.Flag != "" {
					t.Errorf("command %q unconditional effect %#v has a flag, want none", command.Path, effect)
				}
			case "flag_set":
				if !availableFlags[effect.Flag] {
					t.Errorf("command %q effect %#v references unavailable flag; local=%v inherited=%v", command.Path, effect, command.Flags, command.InheritedFlags)
				}
			case "configuration_dependent":
				if effect.Flag != "" {
					t.Errorf("command %q configuration-dependent effect %#v has a flag, want none", command.Path, effect)
				}
			default:
				t.Errorf("command %q effect when = %q, want always, flag_set, or configuration_dependent", command.Path, effect.When)
			}

			switch effect.Kind {
			case "tenant_mutation":
				mutating = true
				if doc.ReadOnly {
					t.Errorf("command %q has tenant_mutation effect while document read_only = true", command.Path)
				}
			case "local_filesystem_write", "local_filesystem_delete":
				mutating = true
			case "network_access":
				hasNetworkAccess = true
			case "local_filesystem_read":
				hasConfiguredLocalRead = hasConfiguredLocalRead || effect.When == "configuration_dependent"
			case "process_execution":
				mutating = true
				hasConfiguredProcessExecution = effect.When == "configuration_dependent"
			}
		}

		if !slices.IsSortedFunc(command.Effects, func(a, b cli.EffectDoc) int {
			switch {
			case a.Kind < b.Kind:
				return -1
			case a.Kind > b.Kind:
				return 1
			case a.When < b.When:
				return -1
			case a.When > b.When:
				return 1
			case a.Flag < b.Flag:
				return -1
			case a.Flag > b.Flag:
				return 1
			default:
				return 0
			}
		}) {
			t.Errorf("command %q effects = %#v, want stable sorted order", command.Path, command.Effects)
		}
		if command.Mutating != mutating {
			t.Errorf("command %q mutating = %t, want derived value %t from effects %#v", command.Path, command.Mutating, mutating, command.Effects)
		}
		if len(command.OutputFields) > 0 && !hasNetworkAccess {
			t.Errorf("command %q exposes live output fields without a network_access effect", command.Path)
		}
		if len(command.OutputFields) > 0 && !hasConfiguredLocalRead {
			t.Errorf("command %q exposes live output fields without a configuration-dependent local_filesystem_read effect", command.Path)
		}
		if len(command.OutputFields) > 0 && !hasConfiguredProcessExecution {
			t.Errorf("command %q exposes live output fields without a configuration-dependent process_execution effect", command.Path)
		}
	}
}

func TestIntrospectCatalogEffectsAreComplete(t *testing.T) {
	t.Parallel()

	doc := cli.IntrospectTree(cli.New(io.Discard, io.Discard, nil))
	byPath := make(map[string]cli.CommandDoc, len(doc.Commands))
	for _, command := range doc.Commands {
		byPath[command.Path] = command
	}

	want := []cli.EffectDoc{
		{Kind: "local_filesystem_read", When: "configuration_dependent"},
		{Kind: "local_filesystem_write", When: "flag_set", Flag: "output"},
		{Kind: "network_access", When: "always"},
		{Kind: "process_execution", When: "configuration_dependent"},
	}
	checked := 0
	for _, resource := range doc.Catalog.Resources {
		for _, op := range resource.Ops {
			path := resource.Product + " " + resource.Name + " " + op
			command, ok := byPath[path]
			if !ok {
				t.Errorf("catalog command %q missing from introspection", path)
				continue
			}
			if !slices.Equal(command.Effects, want) {
				t.Errorf("catalog command %q effects = %#v, want %#v", path, command.Effects, want)
			}
			checked++
		}
	}
	if checked != 271 {
		t.Errorf("checked catalog operations = %d, want 271", checked)
	}
}

func TestIntrospectEffectCounts(t *testing.T) {
	t.Parallel()

	doc := cli.IntrospectTree(cli.New(io.Discard, io.Discard, nil))
	byKind := make(map[string]int)
	byWhen := make(map[string]int)
	commandsWithRead := 0
	for _, command := range doc.Commands {
		hasRead := false
		for _, effect := range command.Effects {
			byKind[effect.Kind]++
			byWhen[effect.When]++
			hasRead = hasRead || effect.Kind == "local_filesystem_read"
		}
		if hasRead {
			commandsWithRead++
		}
	}

	wantKinds := map[string]int{
		"local_filesystem_delete": 2,
		"local_filesystem_read":   279,
		"local_filesystem_write":  293,
		"network_access":          273,
		"process_execution":       274,
	}
	wantWhen := map[string]int{
		"always":                  276,
		"configuration_dependent": 551,
		"flag_set":                294,
	}
	if !maps.Equal(byKind, wantKinds) {
		t.Errorf("effect kind counts = %v, want %v", byKind, wantKinds)
	}
	if !maps.Equal(byWhen, wantWhen) {
		t.Errorf("effect condition counts = %v, want %v", byWhen, wantWhen)
	}
	if commandsWithRead != 278 {
		t.Errorf("commands with local reads = %d, want 278", commandsWithRead)
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
			for _, effectLandmark := range []string{
				"introspect-version: 2",
				"Effect conditions:",
				"local_filesystem_read",
				"local_filesystem_write (when --output is set)",
				"process_execution (configuration-dependent)",
			} {
				if !strings.Contains(got, effectLandmark) {
					t.Errorf("--format %s: output missing effect landmark %q", format, effectLandmark)
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
