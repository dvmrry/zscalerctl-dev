package cli_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/cli"
	"github.com/dvmrry/zscalerctl/internal/resources"
	"github.com/spf13/cobra"
)

type ndjsonExpectation string

const (
	ndjsonRejectUnsupported ndjsonExpectation = "reject-unsupported"
	ndjsonRejectUsage       ndjsonExpectation = "reject-usage"
	ndjsonAccept            ndjsonExpectation = "accept"
)

type ndjsonCommandPolicy struct {
	expect ndjsonExpectation
	args   func(t *testing.T) []string
	reader func() cli.ResourceReader
}

// TestNDJSONSupportPolicyCoversCommandTree is the central NDJSON support gate.
// The real Cobra tree must be covered by an explicit policy entry unless the
// command is a catalog product parent; dynamic resource read operations are
// governed by the catalog and covered by the resource NDJSON tests.
func TestNDJSONSupportPolicyCoversCommandTree(t *testing.T) {
	policies := ndjsonCommandPolicies()
	root := cli.BuildCommandTree(cli.New(io.Discard, io.Discard, nil))
	root.InitDefaultCompletionCmd()
	products := productPathSet(resources.Catalog())

	seen := map[string]struct{}{}
	var missing []string
	cli.WalkCobraTree(root, func(cmd *cobra.Command, path string) {
		if cmd.Hidden || strings.HasPrefix(cmd.Name(), "__complete") {
			return
		}
		seen[path] = struct{}{}
		if _, ok := products[path]; ok {
			return
		}
		if _, ok := policies[path]; !ok {
			missing = append(missing, path)
		}
	})
	if len(missing) > 0 {
		sort.Strings(missing)
		t.Fatalf("Cobra commands missing NDJSON policy:\n  %s", strings.Join(missing, "\n  "))
	}

	var stale []string
	for path := range policies {
		if _, ok := seen[path]; !ok {
			stale = append(stale, path)
		}
	}
	if len(stale) > 0 {
		sort.Strings(stale)
		t.Fatalf("NDJSON policy entries for commands not in Cobra tree:\n  %s", strings.Join(stale, "\n  "))
	}
}

func TestNDJSONCommandPolicyBehavior(t *testing.T) {
	for path, policy := range ndjsonCommandPolicies() {
		path, policy := path, policy
		t.Run(strings.ReplaceAll(path, " ", "_"), func(t *testing.T) {
			err := runNDJSONPolicyCommand(t, policy)
			assertNDJSONExpectation(t, path, policy.expect, err)
		})
	}
}

func TestNDJSONProductParentsRemainUsageOnly(t *testing.T) {
	products := productPathSet(resources.Catalog())
	names := make([]string, 0, len(products))
	for name := range products {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		name := name
		t.Run(name, func(t *testing.T) {
			err := runNDJSONPolicyCommand(t, ndjsonCommandPolicy{
				expect: ndjsonRejectUsage,
				args: func(t *testing.T) []string {
					t.Helper()
					return []string{name}
				},
			})
			assertNDJSONExpectation(t, name, ndjsonRejectUsage, err)
		})
	}
}

func ndjsonCommandPolicies() map[string]ndjsonCommandPolicy {
	args := func(values ...string) func(*testing.T) []string {
		return func(t *testing.T) []string {
			t.Helper()
			return append([]string(nil), values...)
		}
	}

	return map[string]ndjsonCommandPolicy{
		"auth":                  {expect: ndjsonRejectUsage, args: args("auth")},
		"auth status":           {expect: ndjsonRejectUnsupported, args: args("auth", "status")},
		"completion":            {expect: ndjsonRejectUnsupported, args: args("completion")},
		"completion bash":       {expect: ndjsonRejectUnsupported, args: args("completion", "bash")},
		"completion fish":       {expect: ndjsonRejectUnsupported, args: args("completion", "fish")},
		"completion powershell": {expect: ndjsonRejectUnsupported, args: args("completion", "powershell")},
		"completion zsh":        {expect: ndjsonRejectUnsupported, args: args("completion", "zsh")},
		"config":                {expect: ndjsonRejectUsage, args: args("config")},
		"config init": {
			expect: ndjsonAccept,
			args: func(t *testing.T) []string {
				t.Helper()
				return []string{"--config", filepath.Join(t.TempDir(), "config.yaml"), "config", "init"}
			},
		},
		"config show":      {expect: ndjsonRejectUnsupported, args: args("config", "show")},
		"diff":             {expect: ndjsonRejectUnsupported, args: args("diff", "/tmp/zscalerctl-old-dump", "/tmp/zscalerctl-new-dump")},
		"doctor":           {expect: ndjsonRejectUnsupported, args: args("doctor")},
		"dump":             {expect: ndjsonRejectUnsupported, args: args("dump", "--out", "/tmp/zscalerctl-dump")},
		"help":             {expect: ndjsonAccept, args: args("help")},
		"introspect":       {expect: ndjsonRejectUnsupported, args: args("introspect")},
		"machine":          {expect: ndjsonRejectUsage, args: args("machine")},
		"machine manifest": {expect: ndjsonRejectUnsupported, args: args("machine", "manifest")},
		"schema":           {expect: ndjsonRejectUsage, args: args("schema")},
		"schema list":      {expect: ndjsonRejectUnsupported, args: args("schema", "list")},
		"version":          {expect: ndjsonRejectUnsupported, args: args("version")},
		"zia url-lookup": {
			expect: ndjsonRejectUnsupported,
			args:   args("zia", "url-lookup", "https://example.com"),
			reader: func() cli.ResourceReader {
				return &fakeURLLookupReader{}
			},
		},
	}
}

func runNDJSONPolicyCommand(t *testing.T, policy ndjsonCommandPolicy) error {
	t.Helper()

	var out, errOut bytes.Buffer
	options := cli.Options{}
	if policy.reader != nil {
		options.Reader = policy.reader()
	}
	app := cli.NewWithOptions(&out, &errOut, ndjsonPolicyEnv(t), options)
	args := append([]string{"--format", "ndjson"}, policy.args(t)...)
	return app.Run(context.Background(), args)
}

func assertNDJSONExpectation(t *testing.T, path string, expect ndjsonExpectation, err error) {
	t.Helper()

	switch expect {
	case ndjsonRejectUnsupported:
		if err == nil {
			t.Fatalf("%s accepted --format ndjson, want unsupported-format usage error", path)
		}
		if !errors.Is(err, cli.ErrUsage) {
			t.Fatalf("%s --format ndjson error = %v, want ErrUsage", path, err)
		}
		if !strings.Contains(err.Error(), "does not support ndjson") {
			t.Fatalf("%s --format ndjson error = %q, want unsupported-format message", path, err.Error())
		}
	case ndjsonRejectUsage:
		if err == nil {
			t.Fatalf("%s accepted --format ndjson, want usage error", path)
		}
		if !errors.Is(err, cli.ErrUsage) {
			t.Fatalf("%s --format ndjson error = %v, want ErrUsage", path, err)
		}
	case ndjsonAccept:
		if err != nil {
			t.Fatalf("%s --format ndjson error = %v, want nil by explicit policy exception", path, err)
		}
	default:
		t.Fatalf("%s has unknown NDJSON policy expectation %q", path, expect)
	}
}

func ndjsonPolicyEnv(t *testing.T) []string {
	t.Helper()
	home := t.TempDir()
	return []string{
		"HOME=" + home,
		"XDG_CONFIG_HOME=" + home,
	}
}

func productPathSet(catalog resources.ResourceCatalog) map[string]struct{} {
	products := map[string]struct{}{}
	for _, spec := range catalog {
		products[string(spec.Product)] = struct{}{}
	}
	return products
}

func TestNDJSONResourceReadOperationsAreCatalogRecordStreams(t *testing.T) {
	opsByName := map[string]int{}
	for _, spec := range resources.Catalog() {
		for _, op := range spec.Operations {
			if op.Capability == resources.CapabilityRead {
				opsByName[op.Name]++
			}
		}
	}
	for _, op := range []string{"list", "get", "show"} {
		if opsByName[op] == 0 {
			t.Fatalf("catalog has no %s read operations; NDJSON record-stream policy needs review", op)
		}
	}
	for op, count := range opsByName {
		switch op {
		case "list", "get", "show":
		default:
			t.Fatalf("catalog read operation %q has %d resources but no NDJSON policy classification", op, count)
		}
	}
}
