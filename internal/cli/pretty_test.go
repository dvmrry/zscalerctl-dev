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
