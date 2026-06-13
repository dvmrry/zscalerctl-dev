package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/cli"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

const filterSecretCanary = "psk-canary-filter-value"

// filterFixtureReader returns three zia locations records mixing field types:
// numeric ids, strings, a string array, booleans, and a secret field that the
// allow-list projection must drop before --filter/--search ever run.
func filterFixtureReader() fakeResourceReader {
	return fakeResourceReader{
		list: []resources.SourceRecord{
			resources.NewSourceRecord(map[string]any{
				"id":           1,
				"name":         "HQ",
				"country":      "US",
				"ipAddresses":  []any{"192.0.2.10", "198.51.100.7"},
				"authRequired": true,
				"preSharedKey": filterSecretCanary,
			}),
			resources.NewSourceRecord(map[string]any{
				"id":           2,
				"name":         "Branch East",
				"country":      "US",
				"ipAddresses":  []any{"203.0.113.5"},
				"authRequired": false,
				"preSharedKey": filterSecretCanary,
			}),
			resources.NewSourceRecord(map[string]any{
				"id":           3,
				"name":         "Branch West",
				"country":      "DE",
				"ipAddresses":  []any{"192.0.2.99"},
				"authRequired": true,
			}),
		},
	}
}

// runListJSON runs args against the filter fixture and decodes the JSON list.
func runListJSON(t *testing.T, args []string) []map[string]any {
	t.Helper()

	var out, errOut bytes.Buffer
	app := cli.NewWithOptions(&out, &errOut, nil, cli.Options{Reader: filterFixtureReader()})

	if err := app.Run(context.Background(), args); err != nil {
		t.Fatalf("App.Run(%v) error = %v, want nil", args, err)
	}
	if errOut.Len() != 0 {
		t.Fatalf("App.Run(%v) stderr = %q, want empty", args, errOut.String())
	}
	var decoded []map[string]any
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatalf("json.Unmarshal(App.Run(%v) output) error = %v, want nil; output=%q", args, err, out.String())
	}
	return decoded
}

func recordNames(records []map[string]any) string {
	names := make([]string, len(records))
	for i, record := range records {
		names[i], _ = record["name"].(string)
	}
	return strings.Join(names, ",")
}

func TestFilterExactMatchNarrowsList(t *testing.T) {
	t.Parallel()

	args := []string{"--format", "json", "--filter", "name=HQ", "zia", "locations", "list"}
	decoded := runListJSON(t, args)
	if got := recordNames(decoded); got != "HQ" {
		t.Errorf("App.Run(%v) records = %q, want %q", args, got, "HQ")
	}

	// Exact match is case-sensitive and whole-value: a partial or
	// differently-cased value matches nothing.
	for _, filter := range []string{"name=hq", "name=H"} {
		args := []string{"--format", "json", "--filter", filter, "zia", "locations", "list"}
		if decoded := runListJSON(t, args); len(decoded) != 0 {
			t.Errorf("App.Run(%v) records = %d, want 0", args, len(decoded))
		}
	}

	// Non-string fields match on their rendered string form.
	args = []string{"--format", "json", "--filter", "id=2", "zia", "locations", "list"}
	if got := recordNames(runListJSON(t, args)); got != "Branch East" {
		t.Errorf("App.Run(%v) records = %q, want %q", args, got, "Branch East")
	}
}

func TestFilterSubstringMatchIsCaseInsensitive(t *testing.T) {
	t.Parallel()

	args := []string{"--format", "json", "--filter", "name~branch", "zia", "locations", "list"}
	decoded := runListJSON(t, args)
	if got := recordNames(decoded); got != "Branch East,Branch West" {
		t.Errorf("App.Run(%v) records = %q, want %q", args, got, "Branch East,Branch West")
	}
}

func TestRepeatedFiltersANDTogether(t *testing.T) {
	t.Parallel()

	args := []string{
		"--format", "json",
		"--filter", "country=US",
		"--filter", "name~branch",
		"zia", "locations", "list",
	}
	decoded := runListJSON(t, args)
	if got := recordNames(decoded); got != "Branch East" {
		t.Errorf("App.Run(%v) records = %q, want %q", args, got, "Branch East")
	}
}

func TestSearchMatchesAcrossFieldTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		search string
		want   string
	}{
		{name: "string field", search: "branch east", want: "Branch East"},
		{name: "array element", search: "203.0.113", want: "Branch East"},
		{name: "boolean rendered form", search: "true", want: "HQ,Branch West"},
		{name: "term across records", search: "192.0.2", want: "HQ,Branch West"},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			args := []string{"--format", "json", "--search", tt.search, "zia", "locations", "list"}
			decoded := runListJSON(t, args)
			if got := recordNames(decoded); got != tt.want {
				t.Errorf("App.Run(%v) records = %q, want %q", args, got, tt.want)
			}
		})
	}
}

func TestFilterNoMatchIsEmptySuccess(t *testing.T) {
	t.Parallel()

	// JSON renders an empty array, not null and not an error.
	var out, errOut bytes.Buffer
	app := cli.NewWithOptions(&out, &errOut, nil, cli.Options{Reader: filterFixtureReader()})
	args := []string{"--format", "json", "--filter", "name=Nowhere", "zia", "locations", "list"}
	if err := app.Run(context.Background(), args); err != nil {
		t.Fatalf("App.Run(%v) error = %v, want nil", args, err)
	}
	if got := strings.TrimSpace(out.String()); got != "[]" {
		t.Errorf("App.Run(%v) stdout = %q, want %q", args, got, "[]")
	}

	// Table renders the header row only.
	out.Reset()
	args = []string{"--format", "table", "--filter", "name=Nowhere", "zia", "locations", "list"}
	if err := app.Run(context.Background(), args); err != nil {
		t.Fatalf("App.Run(%v) error = %v, want nil", args, err)
	}
	lines := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
	if len(lines) != 1 || !strings.Contains(lines[0], "name") {
		t.Errorf("App.Run(%v) stdout = %q, want a single header line", args, out.String())
	}
	if errOut.Len() != 0 {
		t.Errorf("no-match runs stderr = %q, want empty", errOut.String())
	}
}

// TestFilterSearchCannotReachSecretOrDroppedFields proves the safety property:
// narrowing runs strictly post-projection, so a --filter naming a secret or
// unmodeled field, or a --search for a secret's value, matches nothing — the
// data was already dropped by the allow-list before narrowing ran, in every
// redaction mode.
func TestFilterSearchCannotReachSecretOrDroppedFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
	}{
		{
			name: "exact filter on secret field",
			args: []string{"--filter", "preSharedKey=" + filterSecretCanary},
		},
		{
			name: "substring filter on secret field",
			args: []string{"--filter", "preSharedKey~psk"},
		},
		{
			name: "filter on unmodeled field",
			args: []string{"--filter", "notAField~anything"},
		},
		{
			name: "search for secret value",
			args: []string{"--search", filterSecretCanary},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var out, errOut bytes.Buffer
			app := cli.NewWithOptions(&out, &errOut, nil, cli.Options{Reader: filterFixtureReader()})
			args := append([]string{"--format", "json"}, tt.args...)
			args = append(args, "zia", "locations", "list")
			if err := app.Run(context.Background(), args); err != nil {
				t.Fatalf("App.Run(%v) error = %v, want nil", args, err)
			}
			if got := strings.TrimSpace(out.String()); got != "[]" {
				t.Errorf("App.Run(%v) stdout = %q, want empty array", args, got)
			}
			if strings.Contains(out.String(), filterSecretCanary) || strings.Contains(errOut.String(), filterSecretCanary) {
				t.Errorf("App.Run(%v) output leaked secret canary; stdout=%q stderr=%q", args, out.String(), errOut.String())
			}
		})
	}
}

func TestFilterSearchAreUsageErrorsOutsideList(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
	}{
		{name: "filter on get", args: []string{"--filter", "name=HQ", "zia", "locations", "get", "123"}},
		{name: "search on get", args: []string{"--search", "HQ", "zia", "locations", "get", "123"}},
		{name: "filter on show", args: []string{"--filter", "apiSessionTimeout=30", "zia", "advanced-settings", "show"}},
		{name: "search on show", args: []string{"--search", "30", "zia", "advanced-settings", "show"}},
		{name: "filter on dump", args: []string{"--filter", "name=HQ", "dump", "--out", "ignored"}},
		{name: "filter on doctor", args: []string{"--filter", "name=HQ", "doctor"}},
		{name: "search on version", args: []string{"--search", "HQ", "version"}},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var out, errOut bytes.Buffer
			// failingResourceReader proves the guard fires before any reader call.
			app := cli.NewWithOptions(&out, &errOut, nil, cli.Options{Reader: failingResourceReader{}})

			err := app.Run(context.Background(), tt.args)
			if !errors.Is(err, cli.ErrUsage) {
				t.Fatalf("App.Run(%v) error = %v, want ErrUsage", tt.args, err)
			}
			if !strings.Contains(err.Error(), "list") {
				t.Errorf("App.Run(%v) error = %q, want message mentioning list", tt.args, err.Error())
			}
			if out.Len() != 0 {
				t.Errorf("App.Run(%v) stdout = %q, want empty", tt.args, out.String())
			}
		})
	}
}

func TestFieldsIsUsageErrorOutsideResourceReads(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
	}{
		{name: "fields on schema list", args: []string{"--fields", "name", "schema", "list"}},
		{name: "fields on doctor", args: []string{"--fields", "name", "doctor"}},
		{name: "fields on version", args: []string{"--fields", "name", "version"}},
		{name: "fields on dump", args: []string{"--fields", "name", "dump", "--out", "ignored"}},
		{name: "fields on resource without operation", args: []string{"--fields", "name", "zia", "locations"}},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var out, errOut bytes.Buffer
			// failingResourceReader proves the guard fires before any reader call.
			app := cli.NewWithOptions(&out, &errOut, nil, cli.Options{Reader: failingResourceReader{}})

			err := app.Run(context.Background(), tt.args)
			if !errors.Is(err, cli.ErrUsage) {
				t.Fatalf("App.Run(%v) error = %v, want ErrUsage", tt.args, err)
			}
			if !strings.Contains(err.Error(), "--fields") {
				t.Errorf("App.Run(%v) error = %q, want message mentioning --fields", tt.args, err.Error())
			}
			if out.Len() != 0 {
				t.Errorf("App.Run(%v) stdout = %q, want empty", tt.args, out.String())
			}
		})
	}
}

func TestFilterInvalidExpressionIsUsageError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		expr string
	}{
		{name: "missing operator", expr: "name"},
		{name: "missing key", expr: "=HQ"},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var out, errOut bytes.Buffer
			app := cli.NewWithOptions(&out, &errOut, nil, cli.Options{Reader: failingResourceReader{}})

			args := []string{"--filter", tt.expr, "zia", "locations", "list"}
			err := app.Run(context.Background(), args)
			if !errors.Is(err, cli.ErrUsage) {
				t.Fatalf("App.Run(%v) error = %v, want ErrUsage", args, err)
			}
			if !strings.Contains(err.Error(), "--filter") {
				t.Errorf("App.Run(%v) error = %q, want message naming --filter", args, err.Error())
			}
		})
	}
}

func TestFilterNarrowsPrettyAndTableOutput(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	app := cli.NewWithOptions(&out, &errOut, []string{"TERM=xterm-256color"}, cli.Options{
		StdoutTTY: true,
		Reader:    filterFixtureReader(),
	})
	args := []string{"--format", "pretty", "--filter", "name=HQ", "zia", "locations", "list"}
	if err := app.Run(context.Background(), args); err != nil {
		t.Fatalf("App.Run(%v) error = %v, want nil", args, err)
	}
	got := out.String()
	if !hasBoxDrawing(got) || !strings.Contains(got, "HQ") {
		t.Errorf("App.Run(%v) stdout = %q, want bordered pretty output with HQ", args, got)
	}
	if strings.Contains(got, "Branch East") || strings.Contains(got, "Branch West") {
		t.Errorf("App.Run(%v) stdout = %q, want filtered-out records absent", args, got)
	}

	out.Reset()
	args = []string{"--format", "table", "--search", "branch east", "zia", "locations", "list"}
	if err := app.Run(context.Background(), args); err != nil {
		t.Fatalf("App.Run(%v) error = %v, want nil", args, err)
	}
	got = out.String()
	if !strings.Contains(got, "Branch East") {
		t.Errorf("App.Run(%v) stdout = %q, want matching record", args, got)
	}
	if strings.Contains(got, "HQ") || strings.Contains(got, "Branch West") {
		t.Errorf("App.Run(%v) stdout = %q, want non-matching records absent", args, got)
	}
}

// TestFilterAppliesBeforeFieldsSelection pins the row/column ordering decision:
// --filter narrows rows against the full projected record, so it may reference
// a projected field that --fields does not select for display.
func TestFilterAppliesBeforeFieldsSelection(t *testing.T) {
	t.Parallel()

	args := []string{
		"--format", "json",
		"--fields", "name",
		"--filter", "country=DE",
		"zia", "locations", "list",
	}
	decoded := runListJSON(t, args)
	if got := recordNames(decoded); got != "Branch West" {
		t.Fatalf("App.Run(%v) records = %q, want %q", args, got, "Branch West")
	}
	if _, ok := decoded[0]["country"]; ok || len(decoded[0]) != 1 {
		t.Errorf("App.Run(%v) record = %#v, want only the selected name field", args, decoded[0])
	}
}
