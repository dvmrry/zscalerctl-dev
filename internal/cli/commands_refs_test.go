package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/cli"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

// refsResourceReader serves a list per resource, fails the resources in
// failures, and returns an empty list for every other resource.
type refsResourceReader struct {
	lists    map[string][]map[string]any
	failures map[string]error
}

func (r refsResourceReader) List(_ context.Context, product resources.Product, name string) ([]resources.SourceRecord, error) {
	key := string(product) + "/" + name
	if err := r.failures[key]; err != nil {
		return nil, err
	}
	var out []resources.SourceRecord
	for _, fields := range r.lists[key] {
		out = append(out, resources.NewSourceRecord(fields))
	}
	return out, nil
}

func (r refsResourceReader) Get(context.Context, resources.Product, string, string) (resources.SourceRecord, error) {
	return resources.SourceRecord{}, errors.New("get must not be called")
}

func (r refsResourceReader) Show(context.Context, resources.Product, string) (resources.SourceRecord, error) {
	return resources.NewSourceRecord(map[string]any{}), nil
}

func refsFixtureReader() refsResourceReader {
	return refsResourceReader{
		lists: map[string][]map[string]any{
			"zia/url-filtering-rules": {
				{"id": 1, "name": "Rule one", "groups": []any{map[string]any{"id": 42}, map[string]any{"id": 99}}},
				{"id": 2, "name": "Rule two", "groups": []any{map[string]any{"id": 7}}},
			},
			"zia/groups": {{"id": 42, "name": "Finance"}, {"id": 7, "name": "Sales"}},
		},
		failures: map[string]error{"zia/ssl-inspection-rules": errors.New("client_secret=raw-error-value")},
	}
}

func TestRefsToJSONListsReferencingRecordsAndUncheckedSites(t *testing.T) {
	t.Parallel()
	var out, errOut bytes.Buffer
	app := cli.NewWithOptions(&out, &errOut, nil, cli.Options{Reader: refsFixtureReader()})
	if err := app.Run(context.Background(), []string{"--format", "json", "--redaction", "share", "refs", "to", "zia", "groups", "42"}); err != nil {
		t.Fatalf("refs to error = %v", err)
	}
	var doc resources.RefsTo
	if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
		t.Fatalf("decode %s: %v", out.String(), err)
	}
	if len(doc.ReferencedBy) != 1 || doc.ReferencedBy[0].Resource != "url-filtering-rules" || doc.ReferencedBy[0].Field != "groups" {
		t.Fatalf("referenced_by = %+v, want url-filtering-rules groups", doc.ReferencedBy)
	}
	var unchecked bool
	for _, skip := range doc.NotChecked {
		if skip.Resource == "ssl-inspection-rules" && skip.Reason == resources.RefSkipReadFailed {
			unchecked = true
		}
	}
	if !unchecked {
		t.Errorf("not_checked = %+v, want ssl-inspection-rules read_failed", doc.NotChecked)
	}
	if strings.Contains(out.String()+errOut.String(), "raw-error-value") {
		t.Errorf("raw reader error text leaked into output")
	}
}

func TestRefsCheckTextNamesIDsNotFoundInRead(t *testing.T) {
	t.Parallel()
	var out, errOut bytes.Buffer
	app := cli.NewWithOptions(&out, &errOut, nil, cli.Options{Reader: refsFixtureReader()})
	if err := app.Run(context.Background(), []string{"--format", "pretty", "--redaction", "share", "refs", "check", "zia", "url-filtering-rules"}); err != nil {
		t.Fatalf("refs check error = %v", err)
	}
	text := out.String()
	for _, want := range []string{"zia/url-filtering-rules: 2 records", "id=1 name=Rule one\tgroups=99\tnot in zia/groups\n"} {
		if !strings.Contains(text, want) {
			t.Errorf("output missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "groups=42") || strings.Contains(text, "groups=7") {
		t.Errorf("output reports ids that are in the groups list:\n%s", text)
	}
}

func TestRefsCheckFromDump(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "dump")
	var out, errOut bytes.Buffer
	app := cli.NewWithOptions(&out, &errOut, nil, cli.Options{Reader: refsFixtureReader()})
	if err := app.Run(context.Background(), []string{"--redaction", "share", "dump", "--products", "zia", "--resources", "url-filtering-rules,groups", "--out", dir}); err != nil {
		t.Fatalf("dump error = %v", err)
	}
	out.Reset()
	offline := cli.NewWithOptions(&out, &errOut, nil, cli.Options{Reader: neverCalledReader{t: t}})
	if err := offline.Run(context.Background(), []string{"--format", "json", "--from-dump", dir, "refs", "check", "zia", "url-filtering-rules"}); err != nil {
		t.Fatalf("refs check --from-dump error = %v", err)
	}
	var doc resources.RefsCheck
	if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
		t.Fatalf("decode %s: %v", out.String(), err)
	}
	var misses []string
	for _, miss := range doc.NotFoundInRead {
		if miss.Field == "groups" {
			misses = append(misses, resources.ProjectedValueText(miss.ID))
		}
	}
	if len(misses) != 1 || misses[0] != "99" {
		t.Errorf("groups not_found_in_read = %v, want [99]", misses)
	}
}

func TestRefsRejectsUnknownResourceAndBadArity(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{
		{"refs", "to", "zia", "no-such-resource", "1"},
		{"refs", "to", "zia", "groups"},
		{"refs", "check", "zia"},
	} {
		var out, errOut bytes.Buffer
		app := cli.NewWithOptions(&out, &errOut, nil, cli.Options{Reader: neverCalledReader{t: t}})
		if err := app.Run(context.Background(), args); err == nil {
			t.Errorf("Run(%v) error = nil, want an error", args)
		}
	}
}
