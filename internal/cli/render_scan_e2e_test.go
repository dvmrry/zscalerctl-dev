package cli_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/cli"
	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

// pairCatalog has a list/get and a singleton resource whose tags are
// {"key": ..., "value": ...} pairs, the shape whose value only the final
// scan's sibling rule redacts (projection scans each string alone).
func pairCatalog() resources.ResourceCatalog {
	allModes := []redact.Mode{redact.ModeStandard, redact.ModeShare, redact.ModeParanoid}
	tags := resources.FieldSpec{
		Name: "tags", Classification: resources.ClassTenantConfig, AllowedModes: allModes,
		Fields: []resources.FieldSpec{
			{Name: "key", Classification: resources.ClassTenantConfig, AllowedModes: allModes, SensitiveNameReason: "test tag key"},
			{Name: "value", Classification: resources.ClassTenantConfig, AllowedModes: allModes, SensitiveNameReason: "test tag value"},
		},
	}
	id := resources.FieldSpec{Name: "id", Classification: resources.ClassOperational, AllowedModes: allModes}
	return resources.ResourceCatalog{
		{Product: resources.ProductZIA, Name: "pair-groups", Operations: resources.ReadOperations(), Fields: []resources.FieldSpec{id, tags}},
		{Product: resources.ProductZIA, Name: "pair-settings", Operations: resources.ShowOperation(), Fields: []resources.FieldSpec{tags}},
	}
}

// Every table and pretty record write through App.Run gives its records the
// final scan: a credential that only the scan's sibling rule catches stays
// hidden for list, get and show, with and without --fields. (--fields
// tags.value alone narrows the pair away before the scan, in JSON too.)
func TestRecordTextScansEveryRecordWrite(t *testing.T) {
	t.Parallel()
	const secret = "Zq8vK3" + "mP7xR2wT"
	record := resources.NewSourceRecord(map[string]any{
		"id": 7, "tags": []any{map[string]any{"key": "password", "value": secret}},
	})
	reader := fakeResourceReader{list: []resources.SourceRecord{record}, get: record, show: record}
	for _, format := range []string{"table", "pretty"} {
		for _, command := range [][]string{
			{"zia", "pair-groups", "list"},
			{"zia", "pair-groups", "get", "7"},
			{"zia", "pair-settings", "show"},
			{"--fields", "tags.key,tags.value", "zia", "pair-groups", "list"},
			{"--fields", "tags.key,tags.value", "zia", "pair-groups", "get", "7"},
			{"--fields", "tags", "zia", "pair-settings", "show"},
		} {
			var out, errOut bytes.Buffer
			app := cli.NewWithOptions(&out, &errOut, nil, cli.Options{Reader: reader, Catalog: pairCatalog()})
			args := append([]string{"--format", format}, command...)
			if err := app.Run(context.Background(), args); err != nil {
				t.Fatalf("App.Run(%v) error = %v", args, err)
			}
			if strings.Contains(out.String(), secret) || !strings.Contains(out.String(), "<REDACTED") {
				t.Errorf("App.Run(%v) output = %q, want the paired value replaced by a marker", args, out.String())
			}
		}
	}
}

// And the scan is the JSON one, not the whole-text one: a reviewed nested
// numeric setting shows in table and pretty as it does in JSON.
func TestRecordTextShowsWhatJSONShows(t *testing.T) {
	t.Parallel()
	reader := fakeResourceReader{list: []resources.SourceRecord{resources.NewSourceRecord(map[string]any{
		"id": 7, "policyExtension": map[string]any{"zpaAuthExpOnWinLogonSession": 480},
	})}}
	for _, format := range []string{"table", "pretty"} {
		for _, fields := range []string{"id,policyExtension", "policyExtension.zpaAuthExpOnWinLogonSession"} {
			var out, errOut bytes.Buffer
			app := cli.NewWithOptions(&out, &errOut, nil, cli.Options{Reader: reader})
			args := []string{"--format", format, "--fields", fields, "zcc", "application-profiles", "list"}
			if err := app.Run(context.Background(), args); err != nil {
				t.Fatalf("App.Run(%v) error = %v", args, err)
			}
			if !strings.Contains(out.String(), "480") || strings.Contains(out.String(), "<REDACTED") {
				t.Errorf("App.Run(%v) output = %q, want the setting's 480 shown", args, out.String())
			}
		}
	}
}
