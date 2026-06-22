package cli_test

// cobra_product_test.go -- Phase 2a: product command Cobra migration tests.
//
// Tests verify that zia/zpa/ztw/zcc/zidentity (all knownProducts) are correctly
// wired through the Cobra path and produce behaviour identical to the legacy path.
//
// Test layers:
//  1. Data-path behaviour (fake reader): list/get/show produce correct projected
//     output, including --format json and --format ndjson (ndjson IS allowed).
//  2. Arity/error preservation: missing op -> UsageError; missing id -> UsageError;
//     bogus resource -> ResourceNotFoundError (exit 4 sentinel).
//  3. No-creds path: missing reader -> ErrMissingCredentials (exit 3 sentinel).
//  4. url-lookup: zia url-lookup reaches runURLLookup via the Cobra path.
//  5. isMigrated gate: product commands go through Cobra, not legacy path.
//  6. Phase 2c: resource-specific --help (SetHelpFunc) and catalog completion.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/cli"
	"github.com/dvmrry/zscalerctl/internal/resources"
	"github.com/dvmrry/zscalerctl/internal/zscaler"
)

// -- recordingResourceReader --------------------------------------------------

// recordCall records a single List, Get, or Show invocation.
type recordCall struct {
	op       string // "list", "get", or "show"
	product  resources.Product
	resource string
	id       string
}

// recordingResourceReader wraps fakeResourceReader and records each call so
// tests can assert that the Cobra product routing passed the correct
// (product, resource) pair. Follows the fakeURLLookupReader recording pattern.
type recordingResourceReader struct {
	fakeResourceReader
	calls []recordCall
}

func (r *recordingResourceReader) List(_ context.Context, product resources.Product, resource string) ([]resources.SourceRecord, error) {
	r.calls = append(r.calls, recordCall{op: "list", product: product, resource: resource})
	return r.fakeResourceReader.list, nil
}

func (r *recordingResourceReader) Get(_ context.Context, product resources.Product, resource string, id string) (resources.SourceRecord, error) {
	r.calls = append(r.calls, recordCall{op: "get", product: product, resource: resource, id: id})
	return r.fakeResourceReader.get, nil
}

func (r *recordingResourceReader) Show(_ context.Context, product resources.Product, resource string) (resources.SourceRecord, error) {
	r.calls = append(r.calls, recordCall{op: "show", product: product, resource: resource})
	return r.fakeResourceReader.show, nil
}

// -- helpers ------------------------------------------------------------------

// newProductApp returns an App wired to in-memory buffers with a given reader.
// Pass nil reader to get the no-credentials path.
func newProductApp(t *testing.T, reader cli.ResourceReader) (*cli.App, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	var out, errBuf bytes.Buffer
	var a *cli.App
	if reader != nil {
		a = cli.NewWithOptions(&out, &errBuf, nil, cli.Options{Reader: reader})
	} else {
		a = cli.New(&out, &errBuf, nil)
	}
	return a, &out, &errBuf
}

// -- Data-path: list ----------------------------------------------------------

// TestProductCmd_List_JSON verifies that "zia locations list --format json" via
// the Cobra path produces projected, redacted JSON output identical to the legacy
// path: secret fields dropped, unknown fields stripped, array wrapper.
// Also asserts M-5: the reader receives the correct (product, resource) routing.
func TestProductCmd_List_JSON(t *testing.T) {
	t.Parallel()

	const psk = "product-list-psk-canary"
	reader := &recordingResourceReader{
		fakeResourceReader: fakeResourceReader{
			list: []resources.SourceRecord{
				resources.NewSourceRecord(map[string]any{
					"id":           "1",
					"name":         "HQ",
					"ipAddresses":  []any{"192.0.2.1"},
					"preSharedKey": psk,
				}),
				resources.NewSourceRecord(map[string]any{
					"id":   "2",
					"name": "Branch",
				}),
			},
		},
	}
	a, out, errBuf := newProductApp(t, reader)
	err := a.Run(context.Background(), []string{"--format", "json", "zia", "locations", "list"})
	if err != nil {
		t.Fatalf("App.Run(zia locations list --format json) error = %v, want nil", err)
	}
	if strings.Contains(out.String(), psk) {
		t.Errorf("output leaked secret %q: %q", psk, out.String())
	}
	var decoded []map[string]any
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatalf("json.Unmarshal error = %v; output = %q", err, out.String())
	}
	if len(decoded) != 2 {
		t.Fatalf("decoded length = %d, want 2", len(decoded))
	}
	if _, ok := decoded[0]["preSharedKey"]; ok {
		t.Errorf("secret preSharedKey leaked into output: %#v", decoded[0])
	}
	if name, _ := decoded[0]["name"].(string); name != "HQ" {
		t.Errorf("decoded[0].name = %q, want HQ", name)
	}
	if errBuf.Len() != 0 {
		t.Errorf("stderr = %q, want empty", errBuf.String())
	}
	// M-5: assert routing correctness — reader must have received (zia, locations).
	if len(reader.calls) != 1 {
		t.Fatalf("reader.calls = %d, want 1", len(reader.calls))
	}
	if reader.calls[0].op != "list" || reader.calls[0].product != resources.ProductZIA || reader.calls[0].resource != "locations" {
		t.Errorf("reader call = %+v, want {list, zia, locations}", reader.calls[0])
	}
}

// TestProductCmd_List_NDJSON verifies that --format ndjson is allowed for list
// and emits one compact JSON line per record (ndjson IS a valid read format).
func TestProductCmd_List_NDJSON(t *testing.T) {
	t.Parallel()

	reader := fakeResourceReader{
		list: []resources.SourceRecord{
			resources.NewSourceRecord(map[string]any{"id": "1", "name": "Alpha"}),
			resources.NewSourceRecord(map[string]any{"id": "2", "name": "Beta"}),
		},
	}
	a, out, errBuf := newProductApp(t, reader)
	err := a.Run(context.Background(), []string{"--format", "ndjson", "zia", "locations", "list"})
	if err != nil {
		t.Fatalf("App.Run(zia locations list --format ndjson) error = %v, want nil", err)
	}
	lines := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("ndjson produced %d lines, want 2: %q", len(lines), out.String())
	}
	for i, line := range lines {
		if strings.HasPrefix(line, " ") {
			t.Errorf("ndjson line %d is indented (want compact): %q", i, line)
		}
		var rec map[string]any
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Fatalf("ndjson line %d is not valid JSON: %q: %v", i, line, err)
		}
	}
	if errBuf.Len() != 0 {
		t.Errorf("stderr = %q, want empty", errBuf.String())
	}
}

// -- Data-path: get -----------------------------------------------------------

// TestProductCmd_Get_JSON verifies that "zia locations get <id> --format json"
// via the Cobra path produces a single projected record (not an array).
// Also asserts M-5: the reader receives the correct (product, resource) routing.
func TestProductCmd_Get_JSON(t *testing.T) {
	t.Parallel()

	reader := &recordingResourceReader{
		fakeResourceReader: fakeResourceReader{
			get: resources.NewSourceRecord(map[string]any{
				"id":          "42",
				"name":        "GetResult",
				"ipAddresses": []any{"10.0.0.1"},
			}),
		},
	}
	a, out, errBuf := newProductApp(t, reader)
	err := a.Run(context.Background(), []string{"--format", "json", "zia", "locations", "get", "42"})
	if err != nil {
		t.Fatalf("App.Run(zia locations get 42) error = %v, want nil", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatalf("json.Unmarshal error = %v; output = %q", err, out.String())
	}
	if name, _ := decoded["name"].(string); name != "GetResult" {
		t.Errorf("decoded.name = %q, want GetResult", name)
	}
	if errBuf.Len() != 0 {
		t.Errorf("stderr = %q, want empty", errBuf.String())
	}
	// M-5: assert routing correctness — reader must have received (zia, locations).
	if len(reader.calls) != 1 {
		t.Fatalf("reader.calls = %d, want 1", len(reader.calls))
	}
	if reader.calls[0].op != "get" || reader.calls[0].product != resources.ProductZIA || reader.calls[0].resource != "locations" {
		t.Errorf("reader call = %+v, want {get, zia, locations}", reader.calls[0])
	}
}

// TestProductCmd_Get_DashPrefixedIDAfterTerminator verifies that the global
// parser preserves "--" when dispatching into Cobra, so a dash-prefixed ID is
// passed to runProduct instead of being parsed as an unknown flag.
func TestProductCmd_Get_DashPrefixedIDAfterTerminator(t *testing.T) {
	t.Parallel()

	reader := &recordingResourceReader{
		fakeResourceReader: fakeResourceReader{
			get: resources.NewSourceRecord(map[string]any{
				"id":   "--dash-id",
				"name": "DashID",
			}),
		},
	}
	a, out, errBuf := newProductApp(t, reader)
	err := a.Run(context.Background(), []string{"--format", "json", "zia", "locations", "get", "--", "--dash-id"})
	if err != nil {
		t.Fatalf("App.Run(zia locations get -- --dash-id) error = %v, want nil", err)
	}
	if out.Len() == 0 {
		t.Fatal("stdout is empty, want projected record")
	}
	if errBuf.Len() != 0 {
		t.Errorf("stderr = %q, want empty", errBuf.String())
	}
	if len(reader.calls) != 1 {
		t.Fatalf("reader.calls = %d, want 1", len(reader.calls))
	}
	if reader.calls[0].id != "--dash-id" {
		t.Errorf("reader id = %q, want --dash-id", reader.calls[0].id)
	}
}

// TestProductCmd_Get_NDJSON verifies that --format ndjson is allowed for get.
func TestProductCmd_Get_NDJSON(t *testing.T) {
	t.Parallel()

	reader := fakeResourceReader{
		get: resources.NewSourceRecord(map[string]any{"id": "1", "name": "NDGet"}),
	}
	a, out, _ := newProductApp(t, reader)
	err := a.Run(context.Background(), []string{"--format", "ndjson", "zia", "locations", "get", "1"})
	if err != nil {
		t.Fatalf("App.Run(zia locations get --format ndjson) error = %v, want nil", err)
	}
	// ndjson get emits a single record on one line.
	line := strings.TrimRight(out.String(), "\n")
	var rec map[string]any
	if err := json.Unmarshal([]byte(line), &rec); err != nil {
		t.Fatalf("ndjson get output is not valid JSON: %q: %v", line, err)
	}
	if name, _ := rec["name"].(string); name != "NDGet" {
		t.Errorf("rec.name = %q, want NDGet", name)
	}
}

// -- Data-path: show ----------------------------------------------------------

// TestProductCmd_Show_JSON verifies that "zia advanced-settings show" via the
// Cobra path produces a projected record. advanced-settings has only ShowOperation.
// Also asserts M-5: the reader receives the correct (product, resource) routing.
func TestProductCmd_Show_JSON(t *testing.T) {
	t.Parallel()

	reader := &recordingResourceReader{
		fakeResourceReader: fakeResourceReader{
			show: resources.NewSourceRecord(map[string]any{
				"advancedSettingField": "value",
			}),
		},
	}
	a, out, errBuf := newProductApp(t, reader)
	err := a.Run(context.Background(), []string{"--format", "json", "zia", "advanced-settings", "show"})
	if err != nil {
		t.Fatalf("App.Run(zia advanced-settings show) error = %v, want nil", err)
	}
	// advanced-settings is a show-only resource; output should be a JSON object.
	if out.Len() == 0 {
		t.Fatal("output is empty, want JSON object")
	}
	if errBuf.Len() != 0 {
		t.Errorf("stderr = %q, want empty", errBuf.String())
	}
	// M-5: assert routing correctness — reader must have received (zia, advanced-settings).
	if len(reader.calls) != 1 {
		t.Fatalf("reader.calls = %d, want 1", len(reader.calls))
	}
	if reader.calls[0].op != "show" || reader.calls[0].product != resources.ProductZIA || reader.calls[0].resource != "advanced-settings" {
		t.Errorf("reader call = %+v, want {show, zia, advanced-settings}", reader.calls[0])
	}
}

// TestProductCmd_Show_NDJSON verifies --format ndjson is allowed for show.
func TestProductCmd_Show_NDJSON(t *testing.T) {
	t.Parallel()

	reader := fakeResourceReader{
		show: resources.NewSourceRecord(map[string]any{"someField": "showValue"}),
	}
	a, out, _ := newProductApp(t, reader)
	err := a.Run(context.Background(), []string{"--format", "ndjson", "zia", "advanced-settings", "show"})
	if err != nil {
		t.Fatalf("App.Run(zia advanced-settings show --format ndjson) error = %v, want nil", err)
	}
	if out.Len() == 0 {
		t.Fatal("ndjson show output is empty")
	}
}

// -- Arity / error preservation -----------------------------------------------

// TestProductCmd_MissingOp_UsageError verifies that "zia locations" (no op)
// returns a UsageError (exit 2) containing resource-specific usage.
func TestProductCmd_MissingOp_UsageError(t *testing.T) {
	t.Parallel()

	a, _, _ := newProductApp(t, fakeResourceReader{})
	err := a.Run(context.Background(), []string{"zia", "locations"})
	if err == nil {
		t.Fatal("App.Run(zia locations) error = nil, want UsageError")
	}
	if !errors.Is(err, cli.ErrUsage) {
		t.Errorf("error = %v (%T), want ErrUsage", err, err)
	}
	// runProduct's resource-specific usage mentions the resource operations.
	if !strings.Contains(err.Error(), "locations") {
		t.Errorf("error = %q, want 'locations' in message", err.Error())
	}
}

// TestProductCmd_MissingID_UsageError verifies that "zia locations get" (missing
// id) returns a UsageError (exit 2) with the exact usage message.
func TestProductCmd_MissingID_UsageError(t *testing.T) {
	t.Parallel()

	a, _, _ := newProductApp(t, fakeResourceReader{})
	err := a.Run(context.Background(), []string{"zia", "locations", "get"})
	if err == nil {
		t.Fatal("App.Run(zia locations get) error = nil, want UsageError")
	}
	if !errors.Is(err, cli.ErrUsage) {
		t.Errorf("error = %v (%T), want ErrUsage", err, err)
	}
	if !strings.Contains(err.Error(), "usage: zscalerctl zia locations get <id>") {
		t.Errorf("error = %q, want exact get usage message", err.Error())
	}
}

// TestProductCmd_BogusResource_NotFound verifies that "zia bogusresource list"
// returns a ResourceNotFoundError (unwraps to ErrNotFound, exit 4).
func TestProductCmd_BogusResource_NotFound(t *testing.T) {
	t.Parallel()

	a, _, _ := newProductApp(t, fakeResourceReader{})
	err := a.Run(context.Background(), []string{"zia", "bogusresource", "list"})
	if err == nil {
		t.Fatal("App.Run(zia bogusresource list) error = nil, want ResourceNotFoundError")
	}
	if !errors.Is(err, cli.ErrNotFound) {
		t.Errorf("error = %v (%T), want ErrNotFound (exit 4)", err, err)
	}
}

// -- No-creds path ------------------------------------------------------------

// TestProductCmd_NoCreds verifies that "zia locations list" with no reader (no
// credentials configured) returns the missing-credentials error (exit 3), not a
// Cobra unknown-command error or a UsageError.
func TestProductCmd_NoCreds(t *testing.T) {
	t.Parallel()

	a, _, _ := newProductApp(t, nil) // nil reader -> no credentials in env
	err := a.Run(context.Background(), []string{"zia", "locations", "list"})
	if err == nil {
		t.Fatal("App.Run(zia locations list, no creds) error = nil, want credential error")
	}
	// Must NOT be a UsageError (exit 2) or a Cobra unknown-command error.
	if errors.Is(err, cli.ErrUsage) {
		t.Errorf("error = %v, want credential error (exit 3), not UsageError", err)
	}
	if strings.HasPrefix(err.Error(), "unknown command") {
		t.Errorf("error = %q, looks like Cobra unknown-command; Cobra path should have run", err.Error())
	}
	// Must carry the missing-credentials sentinel (ErrMissingCredentials).
	if !errors.Is(err, zscaler.ErrMissingCredentials) {
		t.Errorf("error = %v (%T), want ErrMissingCredentials", err, err)
	}
}

// -- url-lookup (Cobra subcommand -- Phase 2b) ---------------------------------

// TestProductCmd_URLLookup_ReachesRunURLLookup verifies that "zia url-lookup
// example.com" via the Cobra subcommand path calls the URLLookupReader capability.
// Phase 2b: url-lookup is now a real Cobra subcommand of zia (DisableFlagParsing).
func TestProductCmd_URLLookup_ReachesRunURLLookup(t *testing.T) {
	t.Parallel()

	reader := &fakeURLLookupReader{
		results: []zscaler.URLClassification{
			{
				URL:             "https://example.com",
				Classifications: []string{"TECHNOLOGY"},
			},
		},
	}
	var out, errBuf bytes.Buffer
	a := cli.NewWithOptions(&out, &errBuf, nil, cli.Options{Reader: reader})

	err := a.Run(context.Background(), []string{"--format", "json", "zia", "url-lookup", "https://example.com"})
	if err != nil {
		t.Fatalf("App.Run(zia url-lookup via Cobra subcommand) error = %v, want nil", err)
	}
	if len(reader.calls) != 1 {
		t.Fatalf("URLLookup calls = %d, want 1", len(reader.calls))
	}
	if len(reader.calls[0]) != 1 || reader.calls[0][0] != "https://example.com" {
		t.Errorf("URLLookup called with %v, want [https://example.com]", reader.calls)
	}
	if errBuf.Len() != 0 {
		t.Errorf("stderr = %q, want empty", errBuf.String())
	}
}

// TestProductCmd_URLLookup_NoArgs_UsageError verifies that "zia url-lookup"
// with no URL arguments returns a UsageError (exit 2) with the url-lookup usage
// message.
func TestProductCmd_URLLookup_NoArgs_UsageError(t *testing.T) {
	t.Parallel()

	a, _, _ := newProductApp(t, &fakeURLLookupReader{})
	err := a.Run(context.Background(), []string{"zia", "url-lookup"})
	if err == nil {
		t.Fatal("App.Run(zia url-lookup no args) error = nil, want UsageError")
	}
	if !errors.Is(err, cli.ErrUsage) {
		t.Errorf("error = %v (%T), want ErrUsage", err, err)
	}
	if !strings.Contains(err.Error(), "usage: zscalerctl zia url-lookup <url> [url...]") {
		t.Errorf("error = %q, want url-lookup usage message", err.Error())
	}
}

// TestProductCmd_URLLookup_BogusFlag_StrictMessage verifies that "zia url-lookup
// example.com --bogus" returns a UsageError with the url-lookup-specific strict
// flag message (NOT a generic Cobra "unknown flag" message). With
// DisableFlagParsing, the --bogus token reaches runURLLookup's "-" check, which
// emits "url-lookup takes no flags".
func TestProductCmd_URLLookup_BogusFlag_StrictMessage(t *testing.T) {
	t.Parallel()

	a, _, _ := newProductApp(t, &fakeURLLookupReader{})
	err := a.Run(context.Background(), []string{"zia", "url-lookup", "example.com", "--bogus"})
	if err == nil {
		t.Fatal("App.Run(zia url-lookup --bogus) error = nil, want UsageError")
	}
	if !errors.Is(err, cli.ErrUsage) {
		t.Errorf("error = %v (%T), want ErrUsage", err, err)
	}
	// Must be the url-lookup-specific message, not a generic Cobra "unknown flag".
	if !strings.Contains(err.Error(), "url-lookup takes no flags") {
		t.Errorf("error = %q, want 'url-lookup takes no flags' message (not a generic Cobra error)", err.Error())
	}
	if strings.HasPrefix(err.Error(), "unknown flag") {
		t.Errorf("error = %q, got generic Cobra unknown-flag instead of url-lookup strict message", err.Error())
	}
}

// TestProductCmd_URLLookup_Help_NotRejected verifies that "zia url-lookup --help"
// shows the subcommand help rather than triggering the "-" strict rejection.
// With DisableFlagParsing, RunE detects --help and calls cmd.Help() before
// forwarding to runURLLookup.
func TestProductCmd_URLLookup_Help_NotRejected(t *testing.T) {
	t.Parallel()

	a, out, errBuf := newProductApp(t, nil)
	err := a.Run(context.Background(), []string{"zia", "url-lookup", "--help"})
	if err != nil {
		t.Fatalf("App.Run(zia url-lookup --help) error = %v, want nil", err)
	}
	got := out.String()
	if !strings.Contains(got, "url-lookup") {
		t.Errorf("zia url-lookup --help stdout = %q, want 'url-lookup' in help text", got)
	}
	if errBuf.Len() != 0 {
		t.Errorf("zia url-lookup --help stderr = %q, want empty", errBuf.String())
	}
}

// TestProductCmd_URLLookup_NDJsonRejected verifies that --format ndjson is
// rejected for url-lookup with a UsageError (rejectUnsupportedFormat). ndjson is
// allowed for resource list/get/show but not for this diagnostic command.
func TestProductCmd_URLLookup_NDJsonRejected(t *testing.T) {
	t.Parallel()

	reader := &fakeURLLookupReader{
		results: []zscaler.URLClassification{{URL: "example.com"}},
	}
	a, _, _ := newProductApp(t, reader)
	err := a.Run(context.Background(), []string{"--format", "ndjson", "zia", "url-lookup", "example.com"})
	if err == nil {
		t.Fatal("App.Run(--format ndjson zia url-lookup) error = nil, want UsageError")
	}
	if !errors.Is(err, cli.ErrUsage) {
		t.Errorf("error = %v (%T), want ErrUsage (exit 2)", err, err)
	}
}

// TestProductCmd_URLLookup_JsonWorks verifies that --format json works for
// url-lookup and produces expected JSON output.
func TestProductCmd_URLLookup_JsonWorks(t *testing.T) {
	t.Parallel()

	reader := &fakeURLLookupReader{
		results: []zscaler.URLClassification{
			{URL: "example.com", Classifications: []string{"TECHNOLOGY"}},
		},
	}
	a, out, errBuf := newProductApp(t, reader)
	err := a.Run(context.Background(), []string{"--format", "json", "zia", "url-lookup", "example.com"})
	if err != nil {
		t.Fatalf("App.Run(--format json zia url-lookup) error = %v, want nil", err)
	}
	if !strings.Contains(out.String(), `"url"`) {
		t.Errorf("output = %q, want JSON with 'url' field", out.String())
	}
	if errBuf.Len() != 0 {
		t.Errorf("stderr = %q, want empty", errBuf.String())
	}
}

// TestProductCmd_ZIALocations_StillRoutesToProductRunE verifies that
// "zia locations list" still routes to the zia product RunE (not to any
// url-lookup subcommand). This is the regression guard ensuring that adding
// url-lookup as a subcommand did not break ordinary resource routing.
func TestProductCmd_ZIALocations_StillRoutesToProductRunE(t *testing.T) {
	t.Parallel()

	// With no credentials the product RunE fires (not a Cobra unknown-command
	// error), confirming that locations still goes through the product RunE path.
	a, _, _ := newProductApp(t, nil)
	err := a.Run(context.Background(), []string{"zia", "locations", "list"})
	if err == nil {
		t.Fatal("App.Run(zia locations list, no creds) error = nil, want credential error")
	}
	// Must NOT be a Cobra "unknown command" error -- that would mean routing broke.
	if strings.HasPrefix(err.Error(), "unknown command") {
		t.Errorf("zia locations list returned Cobra unknown-command %q; resource routing is broken", err.Error())
	}
	// Must NOT be a UsageError -- that would mean url-lookup incorrectly captured it.
	if errors.Is(err, cli.ErrUsage) {
		t.Errorf("zia locations list returned UsageError %q; should be credential error", err.Error())
	}
	// Must be the missing-credentials sentinel.
	if !errors.Is(err, zscaler.ErrMissingCredentials) {
		t.Errorf("error = %v (%T), want ErrMissingCredentials", err, err)
	}
}

// -- isMigrated gate / hybrid routing -----------------------------------------

// TestProductCmd_GoesViaCobra verifies that product commands are now routed
// through Cobra (not the legacy path). The positive signal is that a credential
// error is NOT a Cobra unknown-command error -- confirming the Cobra root
// registered the product command and attempted to execute it.
//
// Uses cli.KnownProductNames() (derived from the live catalog) so a future 6th
// product is automatically covered without touching this test.
func TestProductCmd_GoesViaCobra(t *testing.T) {
	t.Parallel()

	var out, errBuf bytes.Buffer
	a := cli.New(&out, &errBuf, nil)
	products := cli.KnownProductNames(a)

	for _, product := range products {
		product := product
		t.Run(product, func(t *testing.T) {
			t.Parallel()

			a, _, _ := newProductApp(t, nil)
			err := a.Run(context.Background(), []string{product, "locations", "list"})

			// With no credentials, the command SHOULD fail -- but NOT with a Cobra
			// "unknown command" error. That would mean the product was not registered.
			if err == nil {
				t.Fatalf("App.Run(%s locations list, no creds) error = nil, want credential error", product)
			}
			if strings.HasPrefix(err.Error(), "unknown command") {
				t.Errorf("App.Run(%s locations list) returned Cobra unknown-command %q; product not registered", product, err.Error())
			}
		})
	}
}

// TestProductCmd_Help_Cobra verifies that "zia --help" returns nil error and
// produces Cobra-formatted help containing "zscalerctl zia".
func TestProductCmd_Help_Cobra(t *testing.T) {
	t.Parallel()

	a, out, errBuf := newProductApp(t, nil)
	err := a.Run(context.Background(), []string{"zia", "--help"})
	if err != nil {
		t.Fatalf("App.Run(zia --help) error = %v, want nil", err)
	}
	got := out.String()
	if !strings.Contains(got, "zscalerctl zia") {
		t.Errorf("zia --help stdout = %q, want 'zscalerctl zia'", got)
	}
	if errBuf.Len() != 0 {
		t.Errorf("zia --help stderr = %q, want empty", errBuf.String())
	}
}

// TestProductCmd_AuthGoesViaCobra verifies that "auth status" is now handled by
// the Cobra path (Phase 4 migration). With no credentials the command succeeds
// (auth status does not require live creds) and the output contains the
// "credentials" field emitted by runAuth / newAuthStatus.
func TestProductCmd_AuthGoesViaCobra(t *testing.T) {
	t.Parallel()

	a, out, errBuf := newProductApp(t, nil)
	err := a.Run(context.Background(), []string{"--format", "json", "auth", "status"})
	if err != nil {
		t.Errorf("App.Run(auth status) error = %v, want nil", err)
	}
	// runAuth / newAuthStatus always emits the "credentials" field.
	if !strings.Contains(out.String(), `"credentials"`) {
		t.Errorf("auth status output = %q, want JSON with 'credentials' field", out.String())
	}
	if errBuf.Len() != 0 {
		t.Errorf("auth status stderr = %q, want empty", errBuf.String())
	}
}

// -- Phase 2c: resource-specific --help (SetHelpFunc) -------------------------

// TestProductCmd_ResourceHelp_WithHelp verifies that "zia locations --help"
// prints the resource-specific field/usage block (resourceUsage) rather than
// the generic Cobra product help. The output must contain the resource name and
// at least one field name from the locations spec.
func TestProductCmd_ResourceHelp_WithHelp(t *testing.T) {
	t.Parallel()

	a, out, errBuf := newProductApp(t, nil)
	err := a.Run(context.Background(), []string{"zia", "locations", "--help"})
	if err != nil {
		t.Fatalf("App.Run(zia locations --help) error = %v, want nil", err)
	}
	got := out.String()
	// resourceUsage always includes the resource name in the usage line.
	if !strings.Contains(got, "locations") {
		t.Errorf("zia locations --help stdout = %q, want 'locations' in resource usage", got)
	}
	// resourceUsage includes the fields block; 'name' is a known locations field.
	if !strings.Contains(got, "name") {
		t.Errorf("zia locations --help stdout = %q, want 'name' field in resource usage", got)
	}
	// Must NOT be the generic Cobra product help (which shows "read zia resources").
	if strings.Contains(got, "read zia resources") {
		t.Errorf("zia locations --help stdout = %q, got generic product help instead of resource help", got)
	}
	if errBuf.Len() != 0 {
		t.Errorf("zia locations --help stderr = %q, want empty", errBuf.String())
	}
}

// TestProductCmd_ResourceHelp_WithOpAndHelp verifies that "zia locations list --help"
// produces the same resource-specific help as "zia locations --help", restoring
// the legacy behaviour where an explicit op before --help still shows the fields.
func TestProductCmd_ResourceHelp_WithOpAndHelp(t *testing.T) {
	t.Parallel()

	a, out, errBuf := newProductApp(t, nil)
	err := a.Run(context.Background(), []string{"zia", "locations", "list", "--help"})
	if err != nil {
		t.Fatalf("App.Run(zia locations list --help) error = %v, want nil", err)
	}
	got := out.String()
	if !strings.Contains(got, "locations") {
		t.Errorf("zia locations list --help stdout = %q, want 'locations' in resource usage", got)
	}
	if !strings.Contains(got, "name") {
		t.Errorf("zia locations list --help stdout = %q, want 'name' field in resource usage", got)
	}
	// Must NOT be the generic Cobra product help.
	if strings.Contains(got, "read zia resources") {
		t.Errorf("zia locations list --help stdout = %q, got generic product help instead of resource help", got)
	}
	if errBuf.Len() != 0 {
		t.Errorf("zia locations list --help stderr = %q, want empty", errBuf.String())
	}
}

// TestProductCmd_HelpCommand_ResourceHelp verifies that the explicit
// "help <product> <resource>" command renders the same catalog-backed resource
// help as the --help flag path.
func TestProductCmd_HelpCommand_ResourceHelp(t *testing.T) {
	t.Parallel()

	a, out, errBuf := newProductApp(t, nil)
	err := a.Run(context.Background(), []string{"help", "zia", "locations"})
	if err != nil {
		t.Fatalf("App.Run(help zia locations) error = %v, want nil", err)
	}
	got := out.String()
	if !strings.Contains(got, "locations") {
		t.Errorf("help zia locations stdout = %q, want 'locations' in resource usage", got)
	}
	if !strings.Contains(got, "name") {
		t.Errorf("help zia locations stdout = %q, want 'name' field in resource usage", got)
	}
	if strings.Contains(got, "products: zia") {
		t.Errorf("help zia locations stdout = %q, got top-level usage instead of resource help", got)
	}
	if errBuf.Len() != 0 {
		t.Errorf("help zia locations stderr = %q, want empty", errBuf.String())
	}
}

// TestProductCmd_ProductHelp_NoResource verifies that "zia --help" (no resource
// specified) falls back to Cobra's default product help, which includes the
// product name, url-lookup subcommand, and the global flags.
func TestProductCmd_ProductHelp_NoResource(t *testing.T) {
	t.Parallel()

	a, out, errBuf := newProductApp(t, nil)
	err := a.Run(context.Background(), []string{"zia", "--help"})
	if err != nil {
		t.Fatalf("App.Run(zia --help) error = %v, want nil", err)
	}
	got := out.String()
	// The Cobra default product help shows "zscalerctl zia" in the Usage line.
	if !strings.Contains(got, "zscalerctl zia") {
		t.Errorf("zia --help stdout = %q, want 'zscalerctl zia'", got)
	}
	// The url-lookup subcommand must still be listed.
	if !strings.Contains(got, "url-lookup") {
		t.Errorf("zia --help stdout = %q, want 'url-lookup' listed in available commands", got)
	}
	if errBuf.Len() != 0 {
		t.Errorf("zia --help stderr = %q, want empty", errBuf.String())
	}
}

// -- Phase 2c: catalog ValidArgsFunction (tab completion) ---------------------

// TestProductCmd_ValidArgsFunction_FirstArg verifies that the ValidArgsFunction
// returns the product's catalog resource names as first-arg completions.
// Uses the exported ProductCmdCompletions helper (export_test.go) which calls the
// function directly, bypassing the hybrid App.Run dispatch layer.
func TestProductCmd_ValidArgsFunction_FirstArg(t *testing.T) {
	t.Parallel()

	completions, directive := cli.ProductCmdCompletions(t, "zia", nil)
	if directive != 4 { // cobra.ShellCompDirectiveNoFileComp = 4
		t.Errorf("directive = %d, want 4 (NoFileComp)", directive)
	}
	// locations is a well-known ZIA resource and must appear.
	found := false
	for _, c := range completions {
		if c == "locations" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("first-arg completions = %v, want 'locations' in list", completions)
	}
}

// TestProductCmd_ValidArgsFunction_SecondArg verifies that the ValidArgsFunction
// returns the supported read ops for a known resource as second-arg completions.
func TestProductCmd_ValidArgsFunction_SecondArg(t *testing.T) {
	t.Parallel()

	completions, directive := cli.ProductCmdCompletions(t, "zia", []string{"locations"})
	if directive != 4 { // cobra.ShellCompDirectiveNoFileComp = 4
		t.Errorf("directive = %d, want 4 (NoFileComp)", directive)
	}
	// locations supports list and get.
	hasList, hasGet := false, false
	for _, c := range completions {
		if c == "list" {
			hasList = true
		}
		if c == "get" {
			hasGet = true
		}
	}
	if !hasList {
		t.Errorf("second-arg completions for locations = %v, want 'list'", completions)
	}
	if !hasGet {
		t.Errorf("second-arg completions for locations = %v, want 'get'", completions)
	}
}

// TestProductCmd_ValidArgsFunction_NoNetwork verifies that the ValidArgsFunction
// does NOT require a reader or config: calling it with no env and nil reader
// returns catalog data without any credential error or panic.
func TestProductCmd_ValidArgsFunction_NoNetwork(t *testing.T) {
	t.Parallel()

	// nil reader + no env = no credentials whatsoever.
	// Must not panic, must not return a credential error.
	completions, directive := cli.ProductCmdCompletions(t, "zia", nil)
	if directive != 4 {
		t.Errorf("directive = %d, want 4 (NoFileComp); got completions = %v", directive, completions)
	}
	// Must return catalog names -- not an empty list -- confirming no-network access.
	if len(completions) == 0 {
		t.Errorf("completions = empty, want catalog resource names (no credentials required)")
	}
}

// TestProductCmd_ValidArgsFunction_UnknownResource verifies that an unknown
// resource as the first arg yields no completions for the second arg
// (not an error, not a panic).
func TestProductCmd_ValidArgsFunction_UnknownResource(t *testing.T) {
	t.Parallel()

	completions, directive := cli.ProductCmdCompletions(t, "zia", []string{"bogusresource"})
	// directive is still NoFileComp regardless.
	if directive != 4 {
		t.Errorf("directive = %d, want 4 (NoFileComp)", directive)
	}
	// No real ops should appear for an unknown resource.
	for _, c := range completions {
		if c == "list" || c == "get" || c == "show" {
			t.Errorf("completions for unknown resource = %v, unexpected op %q", completions, c)
		}
	}
}
