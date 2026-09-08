package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dvmrry/zscalerctl/internal/config"
	"github.com/dvmrry/zscalerctl/internal/diff"
	"github.com/dvmrry/zscalerctl/internal/dump"
	"github.com/dvmrry/zscalerctl/internal/machine"
	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

func TestFromDumpUsesSavedCollectionWithoutLiveRuntime(t *testing.T) {
	dir, catalog := writeCLISavedDump(t, redact.ModeStandard, false, true)
	app, out, errOut := newSavedDumpTestApp(t, catalog)

	args := []string{"--format", "json", "--from-dump", dir, "zia", "locations", "list"}
	if err := app.Run(context.Background(), args); err != nil {
		t.Fatalf("App.Run(%v) error = %v, want nil", args, err)
	}
	var list []map[string]any
	if err := json.Unmarshal(out.Bytes(), &list); err != nil {
		t.Fatalf("json.Unmarshal(list) error = %v; output = %q", err, out.String())
	}
	if len(list) != 2 || list[0]["name"] != "HQ" || list[1]["name"] != "Branch" {
		t.Fatalf("list output = %#v, want saved records in source order", list)
	}
	if errOut.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", errOut.String())
	}

	out.Reset()
	args = []string{"--format", "json", "--from-dump", dir, "zia", "locations", "get", "2"}
	if err := app.Run(context.Background(), args); err != nil {
		t.Fatalf("App.Run(%v) error = %v, want nil", args, err)
	}
	var got map[string]any
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("json.Unmarshal(get) error = %v; output = %q", err, out.String())
	}
	if got["id"] != "2" || got["name"] != "Branch" {
		t.Fatalf("get output = %#v, want saved id=2 record", got)
	}

	out.Reset()
	args = []string{"--format", "json", "--from-dump", dir, "zia", "advanced-settings", "show"}
	if err := app.Run(context.Background(), args); err != nil {
		t.Fatalf("App.Run(%v) error = %v, want nil", args, err)
	}
	got = nil
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("json.Unmarshal(show) error = %v; output = %q", err, out.String())
	}
	if got["name"] != "Tenant settings" {
		t.Fatalf("show output = %#v, want saved singleton record", got)
	}

	out.Reset()
	args = []string{"--format", "ndjson", "--from-dump", dir, "zia", "locations", "list"}
	if err := app.Run(context.Background(), args); err != nil {
		t.Fatalf("App.Run(%v) error = %v, want nil", args, err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("NDJSON output lines = %d, want 2; output = %q", len(lines), out.String())
	}
	for i, line := range lines {
		var record map[string]any
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatalf("json.Unmarshal(NDJSON line %d) error = %v; line = %q", i, err, line)
		}
	}

	out.Reset()
	args = []string{"--format", "pretty", "--from-dump", dir, "zia", "locations", "list"}
	if err := app.Run(context.Background(), args); err != nil {
		t.Fatalf("App.Run(%v) error = %v, want nil", args, err)
	}
	if !strings.Contains(out.String(), "HQ") || !strings.Contains(out.String(), "Branch") {
		t.Fatalf("pretty output = %q, want saved record values", out.String())
	}

	outputPath := filepath.Join(t.TempDir(), "locations.json")
	out.Reset()
	args = []string{"--format", "json", "--from-dump", dir, "--output", outputPath, "zia", "locations", "list"}
	if err := app.Run(context.Background(), args); err != nil {
		t.Fatalf("App.Run(%v) error = %v, want nil", args, err)
	}
	body, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("ReadFile(--output) error = %v", err)
	}
	if len(body) == 0 || !strings.Contains(string(body), "HQ") {
		t.Fatalf("--output body = %q, want saved list JSON", body)
	}
}

func TestFromDumpRejectsMalformedPayloadWithoutEcho(t *testing.T) {
	const numericCanary = "12345678901234567890123456789012345678901234567890e400"
	for _, tc := range []struct {
		name string
		body string
	}{
		{"overflowing number", `{"id":"settings","name":` + numericCanary + `}`},
		{"show resource array", `[{"id":"settings","name":"Tenant settings"}]`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir, catalog := writeCLISavedDump(t, redact.ModeStandard, false, true)
			path := filepath.Join(dir, "resources", "zia", "advanced-settings.json")
			if err := os.WriteFile(path, []byte(tc.body), 0o600); err != nil {
				t.Fatal(err)
			}
			app, out, _ := newSavedDumpTestApp(t, catalog)
			err := app.Run(context.Background(), []string{"--format", "json", "--from-dump", dir, "zia", "advanced-settings", "show"})
			if !errors.Is(err, ErrUsage) || !errors.Is(err, diff.ErrInvalidDump) {
				t.Fatalf("error = %v, want usage and invalid dump", err)
			}
			if strings.Contains(err.Error(), numericCanary) || strings.Contains(err.Error(), tc.body) {
				t.Fatalf("error echoes malformed artifact payload: %v", err)
			}
			if out.Len() != 0 {
				t.Fatalf("invalid artifact produced stdout: %s", out)
			}
		})
	}
}

func TestFromDumpUsesManifestModeAndRejectsMismatch(t *testing.T) {
	dir, catalog := writeCLISavedDump(t, redact.ModeShare, true, false)
	app, out, errOut := newSavedDumpTestApp(t, catalog)

	args := []string{"--format", "json", "--from-dump", dir, "zia", "locations", "list"}
	if err := app.Run(context.Background(), args); err != nil {
		t.Fatalf("App.Run(default mode, %v) error = %v, want nil", args, err)
	}
	var list []map[string]any
	if err := json.Unmarshal(out.Bytes(), &list); err != nil {
		t.Fatalf("json.Unmarshal(default mode) error = %v; output = %q", err, out.String())
	}
	if len(list) != 2 || list[0]["share_only"] != "share-value" {
		t.Fatalf("default mode output = %#v, want manifest-share field", list)
	}
	if errOut.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", errOut.String())
	}

	out.Reset()
	args = []string{"--format", "json", "--redaction", "standard", "--from-dump", dir, "zia", "locations", "list"}
	err := app.Run(context.Background(), args)
	if err == nil {
		t.Fatalf("App.Run(%v) error = nil, want redaction mismatch", args)
	}
	if !errors.Is(err, diff.ErrRedactionMismatch) {
		t.Fatalf("App.Run(%v) error = %v, want diff.ErrRedactionMismatch", args, err)
	}
	if !errors.Is(err, ErrUsage) {
		t.Fatalf("App.Run(%v) error = %v, want ErrUsage classification", args, err)
	}
	if out.Len() != 0 {
		t.Fatalf("mismatch stdout = %q, want empty", out.String())
	}
}

func TestFromDumpNarrowingAndPagingUseSavedRecords(t *testing.T) {
	dir, catalog := writeCLISavedDump(t, redact.ModeStandard, false, false)
	app, out, _ := newSavedDumpTestApp(t, catalog)

	args := []string{
		"--format", "json",
		"--from-dump", dir,
		"--fields", "name",
		"--filter", "country=US",
		"--limit", "1",
		"--offset", "0",
		"zia", "locations", "list",
	}
	var page struct {
		Records    []map[string]any `json:"records"`
		Pagination struct {
			MatchedCount       int  `json:"matched_count"`
			ReturnedCount      int  `json:"returned_count"`
			CollectionComplete bool `json:"collection_complete"`
			HasMore            bool `json:"has_more"`
		} `json:"pagination"`
	}
	if err := app.Run(context.Background(), args); err != nil {
		t.Fatalf("App.Run(%v) error = %v, want nil", args, err)
	}
	if err := json.Unmarshal(out.Bytes(), &page); err != nil {
		t.Fatalf("json.Unmarshal(page) error = %v; output = %q", err, out.String())
	}
	if len(page.Records) != 1 || page.Records[0]["name"] != "HQ" {
		t.Fatalf("page records = %#v, want one projected HQ record", page.Records)
	}
	if page.Pagination.MatchedCount != 1 || page.Pagination.ReturnedCount != 1 ||
		!page.Pagination.CollectionComplete || page.Pagination.HasMore {
		t.Fatalf("page pagination = %#v, want one complete matched record", page.Pagination)
	}
}

func TestFromDumpInvalidInvocationPrecedesFilesystemReads(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "does-not-exist")
	catalog := cliSavedDumpCatalog()
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "non-resource command",
			args: []string{"--from-dump", dir, "version"},
			want: "resource read operations",
		},
		{
			name: "missing get id",
			args: []string{"--from-dump", dir, "zia", "locations", "get"},
			want: "usage: zscalerctl zia locations get <id>",
		},
		{
			name: "empty get id",
			args: []string{"--from-dump", dir, "zia", "locations", "get", ""},
			want: "usage: zscalerctl zia locations get <id>",
		},
		{
			name: "whitespace get id",
			args: []string{"--from-dump", dir, "zia", "locations", "get", "   "},
			want: "usage: zscalerctl zia locations get <id>",
		},
		{
			name: "extra list arg",
			args: []string{"--from-dump", dir, "zia", "locations", "list", "extra"},
			want: "usage: zscalerctl zia locations list",
		},
		{
			name: "page on get",
			args: []string{"--from-dump", dir, "--limit", "1", "zia", "locations", "get", "1"},
			want: "--limit applies to list operations only",
		},
		{
			name: "filter on get",
			args: []string{"--from-dump", dir, "--filter", "name=HQ", "zia", "locations", "get", "1"},
			want: "--filter applies to list operations only",
		},
		{
			name: "profile conflict",
			args: []string{"--from-dump", dir, "--profile", "poison", "zia", "locations", "list"},
			want: "--from-dump cannot be used with --profile",
		},
		{
			name: "config conflict",
			args: []string{"--from-dump", dir, "--config", filepath.Join(t.TempDir(), "poison.yml"), "zia", "locations", "list"},
			want: "--from-dump cannot be used with --config",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app, _, _ := newSavedDumpTestApp(t, catalog)
			err := app.Run(context.Background(), tt.args)
			if err == nil {
				t.Fatalf("App.Run(%v) error = nil, want usage error", tt.args)
			}
			if !errors.Is(err, ErrUsage) {
				t.Fatalf("App.Run(%v) error = %v, want ErrUsage", tt.args, err)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("App.Run(%v) error = %q, want substring %q", tt.args, err, tt.want)
			}
			if strings.Contains(err.Error(), "does-not-exist") || strings.Contains(err.Error(), "invalid dump") {
				t.Fatalf("App.Run(%v) error = %q, want no filesystem/artifact read", tt.args, err)
			}
		})
	}
}

func TestFromDumpContextErrorsUseMachineBoundary(t *testing.T) {
	dir, catalog := writeCLISavedDump(t, redact.ModeStandard, false, false)
	tests := []struct {
		name     string
		ctx      func() (context.Context, context.CancelFunc)
		kind     string
		sentinel error
	}{
		{
			name: "canceled",
			ctx: func() (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx, func() {}
			},
			kind:     machine.ErrorKindCanceled,
			sentinel: context.Canceled,
		},
		{
			name: "deadline",
			ctx: func() (context.Context, context.CancelFunc) {
				return context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
			},
			kind:     machine.ErrorKindDeadlineExceeded,
			sentinel: context.DeadlineExceeded,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := tc.ctx()
			defer cancel()
			app, out, errOut := newSavedDumpTestApp(t, catalog)
			args := []string{"--format", "json", "--from-dump", dir, "zia", "locations", "list"}
			err := app.Run(ctx, args)
			var machineErr *machine.MachineError
			if !errors.As(err, &machineErr) {
				t.Fatalf("App.Run(%v) error = %T %v, want machine context error", args, err, err)
			}
			if machineErr.Kind != tc.kind || machineErr.Operation != machine.OperationList ||
				machineErr.Product != "zia" || machineErr.Resource != "locations" {
				t.Fatalf("App.Run(%v) machine error = %#v, want kind=%q operation=list product=zia resource=locations", args, machineErr, tc.kind)
			}
			if !errors.Is(err, tc.sentinel) {
				t.Fatalf("App.Run(%v) error = %v, want errors.Is(..., %v)", args, err, tc.sentinel)
			}
			if out.Len() != 0 || errOut.Len() != 0 {
				t.Fatalf("App.Run(%v) output stdout=%q stderr=%q, want both empty", args, out.String(), errOut.String())
			}
		})
	}
}

func TestFromDumpMissingResourceIsAnError(t *testing.T) {
	dir, catalog := writeCLISavedDump(t, redact.ModeStandard, false, false)
	app, out, _ := newSavedDumpTestApp(t, catalog)
	args := []string{"--format", "json", "--from-dump", dir, "zia", "advanced-settings", "show"}
	err := app.Run(context.Background(), args)
	if err == nil {
		t.Fatalf("App.Run(%v) error = nil, want missing resource error", args)
	}
	var machineErr *machine.MachineError
	if !errors.As(err, &machineErr) {
		t.Fatalf("App.Run(%v) error = %T %v, want machine not-found error", args, err, err)
	}
	if machineErr.Kind != machine.ErrorKindNotFound || machineErr.Message != "resource is not present in saved collection" {
		t.Fatalf("App.Run(%v) machine error = %#v, want saved-resource not-found classification", args, machineErr)
	}
	if out.Len() != 0 {
		t.Fatalf("missing resource stdout = %q, want empty", out.String())
	}
}

func newSavedDumpTestApp(t *testing.T, catalog resources.ResourceCatalog) (*App, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	var out, errOut bytes.Buffer
	app := NewWithOptions(&out, &errOut, []string{"ZSCALERCTL_CLIENT_ID=poison"}, Options{
		Catalog: catalog,
		Reader:  panicSavedDumpReader{},
	})
	app.machineRuntimeFactory = func(context.Context, config.Config, globalOptions) (machineRuntime, error) {
		t.Fatalf("saved-dump invocation constructed the live machine runtime")
		return nil, errors.New("unreachable")
	}
	return app, &out, &errOut
}

func writeCLISavedDump(t *testing.T, mode redact.Mode, includeShareOnly, includeSettings bool) (string, resources.ResourceCatalog) {
	t.Helper()
	catalog := cliSavedDumpCatalog()
	first := map[string]any{"id": "1", "name": "HQ", "country": "US"}
	second := map[string]any{"id": "2", "name": "Branch", "country": "DE"}
	if includeShareOnly {
		first["share_only"] = "share-value"
		second["share_only"] = "branch-share-value"
	}
	locations := resources.NewProjectedRecordsFromProjectedFields([]map[string]any{first, second})
	dir := filepath.Join(t.TempDir(), "saved-dump")
	entries := []dump.ResourceDump{{Spec: catalog[0], Records: locations}}
	if includeSettings {
		settings := resources.NewProjectedRecordsFromProjectedFields([]map[string]any{{
			"id":   "settings",
			"name": "Tenant settings",
		}}).Records()[0]
		entries = append(entries, dump.ResourceDump{Spec: catalog[1], Record: &settings})
	}
	if err := dump.Write(dir, mode, dump.Result{Entries: entries}); err != nil {
		t.Fatalf("dump.Write(%s) error = %v", dir, err)
	}
	return dir, catalog
}

func cliSavedDumpCatalog() resources.ResourceCatalog {
	allModes := []redact.Mode{redact.ModeStandard, redact.ModeShare, redact.ModeParanoid}
	return resources.ResourceCatalog{
		{
			Product:    resources.ProductZIA,
			Name:       "locations",
			Operations: resources.ReadOperations(),
			Fields: []resources.FieldSpec{
				{Name: "id", Classification: resources.ClassOperational, AllowedModes: allModes},
				{Name: "name", Classification: resources.ClassTenantConfig, AllowedModes: allModes},
				{Name: "country", Classification: resources.ClassTenantConfig, AllowedModes: allModes},
				{Name: "share_only", Classification: resources.ClassTenantConfig, AllowedModes: []redact.Mode{redact.ModeShare}},
			},
		},
		{
			Product:    resources.ProductZIA,
			Name:       "advanced-settings",
			Operations: resources.ShowOperation(),
			Fields: []resources.FieldSpec{
				{Name: "id", Classification: resources.ClassOperational, AllowedModes: allModes},
				{Name: "name", Classification: resources.ClassTenantConfig, AllowedModes: allModes},
			},
		},
	}
}

type panicSavedDumpReader struct{}

func (panicSavedDumpReader) List(context.Context, resources.Product, string) ([]resources.SourceRecord, error) {
	panic("saved-dump test called live reader")
}

func (panicSavedDumpReader) Get(context.Context, resources.Product, string, string) (resources.SourceRecord, error) {
	panic("saved-dump test called live reader")
}

func (panicSavedDumpReader) Show(context.Context, resources.Product, string) (resources.SourceRecord, error) {
	panic("saved-dump test called live reader")
}

var _ ResourceReader = panicSavedDumpReader{}
