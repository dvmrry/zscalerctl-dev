package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/cli"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

// locationsReader returns a single-record reader for zia locations, reused by
// the pretty/auto tests below.
func locationsReader() fakeResourceReader {
	return fakeResourceReader{
		list: []resources.SourceRecord{resources.NewSourceRecord(map[string]any{
			"id":   "123",
			"name": "HQ",
		})},
	}
}

// hasBoxDrawing reports whether s contains any of the lipgloss NormalBorder box
// characters, the cheap signal that the pretty (not table/json) renderer ran.
func hasBoxDrawing(s string) bool {
	for _, r := range []string{"─", "│", "┌", "┐", "└", "┘", "├", "┤", "┬", "┴"} {
		if strings.Contains(s, r) {
			return true
		}
	}
	return false
}

func TestAutoFormatRendersPrettyOnTTY(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	app := cli.NewWithOptions(&out, &errOut, []string{"TERM=xterm-256color"}, cli.Options{
		StdoutTTY: true,
		Reader:    locationsReader(),
	})

	// No --format: auto must resolve to pretty because stdout is a TTY.
	if err := app.Run(context.Background(), []string{"zia", "locations", "list"}); err != nil {
		t.Fatalf("App.Run(zia locations list) error = %v, want nil", err)
	}
	got := out.String()
	if !hasBoxDrawing(got) {
		t.Errorf("auto-on-TTY output = %q, want bordered pretty table", got)
	}
	if !strings.Contains(got, "HQ") {
		t.Errorf("auto-on-TTY output = %q, want record value HQ", got)
	}
	var sink any
	if json.Unmarshal(out.Bytes(), &sink) == nil {
		t.Errorf("auto-on-TTY output = %q, want non-JSON pretty output", got)
	}
}

func TestAutoFormatRendersJSONOffTTY(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	// StdoutTTY defaults to false for a bytes.Buffer destination.
	app := cli.NewWithOptions(&out, &errOut, nil, cli.Options{Reader: locationsReader()})

	if err := app.Run(context.Background(), []string{"zia", "locations", "list"}); err != nil {
		t.Fatalf("App.Run(zia locations list) error = %v, want nil", err)
	}
	var decoded []map[string]any
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatalf("auto-off-TTY output = %q, want JSON; err = %v", out.String(), err)
	}
	if len(decoded) != 1 || decoded[0]["name"] != "HQ" {
		t.Errorf("auto-off-TTY decoded = %#v, want one HQ record", decoded)
	}
}

func TestAutoFormatWithOutputFileIsJSON(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	// Even with a stdout TTY, --output writes to a file, so auto must pick JSON.
	app := cli.NewWithOptions(&out, &errOut, []string{"TERM=xterm-256color"}, cli.Options{
		StdoutTTY: true,
		Reader:    locationsReader(),
	})

	dir := t.TempDir()
	path := dir + "/locations.json"
	if err := app.Run(context.Background(), []string{"--output", path, "zia", "locations", "list"}); err != nil {
		t.Fatalf("App.Run(--output zia locations list) error = %v, want nil", err)
	}
	body := readFile(t, path)
	var decoded []map[string]any
	if err := json.Unmarshal([]byte(body), &decoded); err != nil {
		t.Fatalf("auto --output body = %q, want JSON; err = %v", body, err)
	}
	if hasBoxDrawing(body) || strings.Contains(body, "\x1b[") {
		t.Errorf("auto --output body = %q, want clean JSON (no border/escapes)", body)
	}
}

func TestPrettyListByteCleanUnderColorNever(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	app := cli.NewWithOptions(&out, &errOut, []string{"TERM=xterm-256color"}, cli.Options{
		StdoutTTY: true,
		Reader:    locationsReader(),
	})

	if err := app.Run(context.Background(), []string{"--format", "pretty", "--color", "never", "zia", "locations", "list"}); err != nil {
		t.Fatalf("App.Run(pretty --color never) error = %v, want nil", err)
	}
	got := out.String()
	if strings.Contains(got, "\x1b[") {
		t.Errorf("pretty --color never output = %q, want no ANSI escapes", got)
	}
	if !hasBoxDrawing(got) {
		t.Errorf("pretty --color never output = %q, want bordered table", got)
	}
}

func TestPrettyListColorEmitsANSIWhenRequested(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	app := cli.NewWithOptions(&out, &errOut, []string{"TERM=xterm-256color"}, cli.Options{
		StdoutTTY: true,
		Reader:    locationsReader(),
	})

	if err := app.Run(context.Background(), []string{"--format", "pretty", "--color", "always", "zia", "locations", "list"}); err != nil {
		t.Fatalf("App.Run(pretty --color always) error = %v, want nil", err)
	}
	if !strings.Contains(out.String(), "\x1b[") {
		t.Errorf("pretty --color always output = %q, want ANSI escapes", out.String())
	}
}

func TestPrettyFieldsNarrowsColumns(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	app := cli.NewWithOptions(&out, &errOut, nil, cli.Options{Reader: locationsReader()})

	if err := app.Run(context.Background(), []string{"--format", "pretty", "--color", "never", "--fields", "name", "zia", "locations", "list"}); err != nil {
		t.Fatalf("App.Run(pretty --fields name) error = %v, want nil", err)
	}
	got := out.String()
	if !strings.Contains(got, "name") || !strings.Contains(got, "HQ") {
		t.Errorf("pretty --fields name output = %q, want name column with HQ", got)
	}
	// --fields can only narrow: the id column header must be gone.
	if strings.Contains(got, "id") {
		t.Errorf("pretty --fields name output = %q, want id column dropped", got)
	}
}

func TestPrettyShowRendersKeyValues(t *testing.T) {
	t.Parallel()

	reader := fakeResourceReader{
		show: resources.NewSourceRecord(map[string]any{
			"apiSessionTimeout": 30,
		}),
	}
	var out, errOut bytes.Buffer
	app := cli.NewWithOptions(&out, &errOut, nil, cli.Options{Reader: reader})

	if err := app.Run(context.Background(), []string{"--format", "pretty", "--color", "never", "zia", "advanced-settings", "show"}); err != nil {
		t.Fatalf("App.Run(pretty show) error = %v, want nil", err)
	}
	got := out.String()
	if !strings.Contains(got, "apiSessionTimeout") || !strings.Contains(got, "30") {
		t.Errorf("pretty show output = %q, want apiSessionTimeout 30", got)
	}
	if strings.Contains(got, "\x1b[") {
		t.Errorf("pretty show --color never output = %q, want no ANSI escapes", got)
	}
}

func TestResourceUsageListsOperationsAndFields(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	app := cli.New(&out, &errOut, nil)

	// Known resource, missing operation -> resource-specific help.
	err := app.Run(context.Background(), []string{"zia", "locations"})
	if err == nil {
		t.Fatal("App.Run(zia locations) error = nil, want usage error")
	}
	msg := err.Error()
	for _, want := range []string{"zia locations list|get", "fields:", "ipAddresses"} {
		if !strings.Contains(msg, want) {
			t.Errorf("resource usage = %q, want %q", msg, want)
		}
	}
}

func TestUsageErrorsKeepStderrCleanForJSONConsumers(t *testing.T) {
	t.Parallel()

	// With --format json (or auto off a TTY), main renders the UsageError as a
	// JSON envelope on stderr — so the app must NOT also write the plain-text
	// usage block there, or the stream stops being parseable.
	cases := []struct {
		name string
		args []string
	}{
		{"json unknown command", []string{"--format", "json", "frobnicate"}},
		{"auto off-tty unknown command", []string{"frobnicate"}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var out, errOut bytes.Buffer
			app := cli.New(&out, &errOut, nil)
			if err := app.Run(context.Background(), tc.args); err == nil {
				t.Fatalf("App.Run(%v) error = nil, want usage error", tc.args)
			}
			if errOut.Len() != 0 {
				t.Errorf("App.Run(%v) stderr = %q, want empty (envelope is main's job)", tc.args, errOut.String())
			}
		})
	}
}

func TestUsageErrorsStillShowUsageBlockOnTTY(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	app := cli.NewWithOptions(&out, &errOut, []string{"TERM=xterm-256color"}, cli.Options{StdoutTTY: true})
	if err := app.Run(context.Background(), []string{"frobnicate"}); err == nil {
		t.Fatal("App.Run(unknown command, TTY) error = nil, want usage error")
	}
	if !strings.Contains(errOut.String(), "usage: zscalerctl") {
		t.Errorf("App.Run(unknown command, TTY) stderr = %q, want usage block", errOut.String())
	}
}

func TestUnknownCommandHintsAtSwallowedProduct(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	app := cli.New(&out, &errOut, nil)

	// --fields with no value consumes "zia"; the leftover "locations" is a
	// known resource, so the error should hint at the swallowed product.
	err := app.Run(context.Background(), []string{"--fields", "zia", "locations", "list"})
	if err == nil {
		t.Fatal("App.Run(--fields zia locations list) error = nil, want usage error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "is a resource") || !strings.Contains(msg, "--fields") {
		t.Errorf("unknown-command error = %q, want resource/flag hint", msg)
	}
}

func TestColorAlwaysSuppressedForOutputFile(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	app := cli.NewWithOptions(&out, &errOut, []string{"TERM=xterm-256color"}, cli.Options{
		StdoutTTY: true,
		Reader:    locationsReader(),
	})

	dir := t.TempDir()
	path := dir + "/locations.txt"
	// --color always must NOT put ANSI escapes into a file sink.
	if err := app.Run(context.Background(), []string{"--color", "always", "--format", "table", "--output", path, "zia", "locations", "list"}); err != nil {
		t.Fatalf("App.Run(--color always --output) error = %v, want nil", err)
	}
	body := readFile(t, path)
	if strings.Contains(body, "\x1b[") {
		t.Errorf("output file = %q, want no ANSI escapes", body)
	}
	if !strings.Contains(body, "HQ") {
		t.Errorf("output file = %q, want record content", body)
	}
}

func TestHelpRoutesToResourceAndProductScope(t *testing.T) {
	t.Parallel()

	// Product commands (zia, zpa, ...) are Cobra subcommands. This gives two help
	// surfaces:
	//
	//  - "zia --help" now shows Cobra-formatted help (short description + global
	//    flags) instead of the legacy resource-list. The product name appears in
	//    the Usage line ("zscalerctl zia"), but the resource list does not.
	//
	//  - "zia locations --help" also shows the Cobra zia-command help, because
	//    "locations" is a positional argument (not a Cobra subcommand); Cobra
	//    intercepts --help before RunE runs, so the resource-specific field list
	//    is NOT shown. This is an accepted model change documented in
	//    testdata/surface/surface_changes.md.
	//
	// The global-help path is unchanged.
	cases := []struct {
		name string
		args []string
		want []string
		deny []string
	}{
		// Resource help: Cobra intercepts --help; shows the product-level Cobra help
		// rather than resource-specific field list. Accepted model change.
		{"resource help", []string{"zia", "locations", "--help"}, []string{"zscalerctl zia"}, []string{}},
		// Product help: Cobra-formatted help with the product name in Usage.
		{"product help", []string{"zia", "--help"}, []string{"zscalerctl zia"}, []string{"fields:"}},
		// Global help: now rendered by Cobra rather than the legacy usage block.
		{"global help", []string{"--help"}, []string{"Usage:", "zscalerctl [command]"}, nil},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var out, errOut bytes.Buffer
			app := cli.New(&out, &errOut, nil)
			if err := app.Run(context.Background(), tc.args); err != nil {
				t.Fatalf("App.Run(%v) error = %v, want nil", tc.args, err)
			}
			got := out.String()
			for _, want := range tc.want {
				if !strings.Contains(got, want) {
					t.Errorf("App.Run(%v) stdout = %q, want %q", tc.args, got, want)
				}
			}
			for _, deny := range tc.deny {
				if strings.Contains(got, deny) {
					t.Errorf("App.Run(%v) stdout = %q, want no %q", tc.args, got, deny)
				}
			}
		})
	}
}

// TestResourceListCanaryAbsentAcrossFormatsAndModes asserts the no-leak
// guarantee holds end-to-end for every output format and redaction mode, not
// just JSON/standard: secret values, secret-shaped free text, and bare
// high-entropy tokens must never reach stdout in table or pretty, in share or
// paranoid, any combination.
func TestResourceListCanaryAbsentAcrossFormatsAndModes(t *testing.T) {
	t.Parallel()

	const (
		secretCanary   = "list-secret-canary-zzz"
		freeTextCanary = "free-text-canary-yyy"
		bareToken      = "A7b9C2d4E6f8G1h3J5k7L9m2N4p6Q8r0S2t4U6v"
	)
	reader := fakeResourceReader{
		list: []resources.SourceRecord{resources.NewSourceRecord(map[string]any{
			"id":           "1",
			"name":         "HQ",
			"ipAddresses":  []any{"192.0.2.10"},
			"description":  "note psk=" + freeTextCanary + " " + bareToken,
			"preSharedKey": secretCanary,
		})},
	}

	for _, format := range []string{"json", "table", "pretty"} {
		for _, mode := range []string{"standard", "share", "paranoid"} {
			format, mode := format, mode
			t.Run(format+"/"+mode, func(t *testing.T) {
				t.Parallel()
				var out, errOut bytes.Buffer
				app := cli.NewWithOptions(&out, &errOut, nil, cli.Options{Reader: reader})
				args := []string{"--format", format, "--redaction", mode, "--color", "never", "zia", "locations", "list"}
				if err := app.Run(context.Background(), args); err != nil {
					t.Fatalf("App.Run(%v) error = %v, want nil", args, err)
				}
				for _, canary := range []string{secretCanary, freeTextCanary, bareToken} {
					if strings.Contains(out.String(), canary) {
						t.Errorf("%s/%s stdout leaked canary %q: %q", format, mode, canary, out.String())
					}
					if strings.Contains(errOut.String(), canary) {
						t.Errorf("%s/%s stderr leaked canary %q: %q", format, mode, canary, errOut.String())
					}
				}
			})
		}
	}
}
