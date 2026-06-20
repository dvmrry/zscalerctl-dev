package cli_test

// cobra_product_test.go — Phase 2a: product command Cobra migration tests.
//
// Tests verify that zia/zpa/ztw/zcc/zidentity (all knownProducts) are correctly
// wired through the Cobra path and produce behaviour identical to the legacy path.
//
// Test layers:
//  1. Data-path behaviour (fake reader): list/get/show produce correct projected
//     output, including --format json and --format ndjson (ndjson IS allowed).
//  2. Arity/error preservation: missing op → UsageError; missing id → UsageError;
//     bogus resource → ResourceNotFoundError (exit 4 sentinel).
//  3. No-creds path: missing reader → ErrMissingCredentials (exit 3 sentinel).
//  4. url-lookup: zia url-lookup reaches runURLLookup via the Cobra path.
//  5. isMigrated gate: product commands go through Cobra, not legacy path.

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

// ── helpers ──────────────────────────────────────────────────────────────────

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

// ── Data-path: list ──────────────────────────────────────────────────────────

// TestProductCmd_List_JSON verifies that "zia locations list --format json" via
// the Cobra path produces projected, redacted JSON output identical to the legacy
// path: secret fields dropped, unknown fields stripped, array wrapper.
func TestProductCmd_List_JSON(t *testing.T) {
	t.Parallel()

	const psk = "product-list-psk-canary"
	reader := fakeResourceReader{
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

// ── Data-path: get ───────────────────────────────────────────────────────────

// TestProductCmd_Get_JSON verifies that "zia locations get <id> --format json"
// via the Cobra path produces a single projected record (not an array).
func TestProductCmd_Get_JSON(t *testing.T) {
	t.Parallel()

	reader := fakeResourceReader{
		get: resources.NewSourceRecord(map[string]any{
			"id":          "42",
			"name":        "GetResult",
			"ipAddresses": []any{"10.0.0.1"},
		}),
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

// ── Data-path: show ──────────────────────────────────────────────────────────

// TestProductCmd_Show_JSON verifies that "zia advanced-settings show" via the
// Cobra path produces a projected record. advanced-settings has only ShowOperation.
func TestProductCmd_Show_JSON(t *testing.T) {
	t.Parallel()

	reader := fakeResourceReader{
		show: resources.NewSourceRecord(map[string]any{
			"advancedSettingField": "value",
		}),
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

// ── Arity / error preservation ────────────────────────────────────────────────

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

// ── No-creds path ────────────────────────────────────────────────────────────

// TestProductCmd_NoCreds verifies that "zia locations list" with no reader (no
// credentials configured) returns the missing-credentials error (exit 3), not a
// Cobra unknown-command error or a UsageError.
func TestProductCmd_NoCreds(t *testing.T) {
	t.Parallel()

	a, _, _ := newProductApp(t, nil) // nil reader → no credentials in env
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

// ── url-lookup (Cobra subcommand — Phase 2b) ──────────────────────────────────

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
	// Must NOT be a Cobra "unknown command" error — that would mean routing broke.
	if strings.HasPrefix(err.Error(), "unknown command") {
		t.Errorf("zia locations list returned Cobra unknown-command %q; resource routing is broken", err.Error())
	}
	// Must NOT be a UsageError — that would mean url-lookup incorrectly captured it.
	if errors.Is(err, cli.ErrUsage) {
		t.Errorf("zia locations list returned UsageError %q; should be credential error", err.Error())
	}
	// Must be the missing-credentials sentinel.
	if !errors.Is(err, zscaler.ErrMissingCredentials) {
		t.Errorf("error = %v (%T), want ErrMissingCredentials", err, err)
	}
}

// ── isMigrated gate / hybrid routing ─────────────────────────────────────────

// TestProductCmd_GoesViaCobra verifies that product commands are now routed
// through Cobra (not the legacy path). The positive signal is that a credential
// error is NOT a Cobra unknown-command error — confirming the Cobra root
// registered the product command and attempted to execute it.
func TestProductCmd_GoesViaCobra(t *testing.T) {
	t.Parallel()

	// Iterate all known products to ensure every product is migrated.
	products := []string{"zia", "zpa", "ztw", "zcc", "zidentity"}
	for _, product := range products {
		product := product
		t.Run(product, func(t *testing.T) {
			t.Parallel()

			a, _, _ := newProductApp(t, nil)
			err := a.Run(context.Background(), []string{product, "locations", "list"})

			// With no credentials, the command SHOULD fail — but NOT with a Cobra
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

// TestProductCmd_LegacyAuthStillWorks verifies that auth (un-migrated command)
// is still handled by the legacy path: it must NOT produce a Cobra unknown-command
// error when it reaches the un-migrated switch.
func TestProductCmd_LegacyAuthStillWorks(t *testing.T) {
	t.Parallel()

	a, _, _ := newProductApp(t, nil)
	err := a.Run(context.Background(), []string{"auth", "status"})
	// auth with no creds will error, but it must NOT be a Cobra unknown-command.
	if err != nil {
		if strings.HasPrefix(err.Error(), "unknown command") {
			t.Errorf("auth returned Cobra unknown-command %q; legacy path should handle it", err.Error())
		}
	}
}
