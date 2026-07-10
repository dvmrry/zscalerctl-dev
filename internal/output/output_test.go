package output_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/dvmrry/zscalerctl/internal/output"
	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/secret"
)

func TestParseFormatSupportsImplementedFormatsOnly(t *testing.T) {
	t.Parallel()

	for _, value := range []string{"auto", "table", "json", "ndjson", "pretty"} {
		value := value
		t.Run(value, func(t *testing.T) {
			t.Parallel()

			if _, err := output.ParseFormat(value); err != nil {
				t.Fatalf("ParseFormat(%q) error = %v, want nil", value, err)
			}
		})
	}
	for _, value := range []string{"yaml", "csv"} {
		value := value
		t.Run(value, func(t *testing.T) {
			t.Parallel()

			_, err := output.ParseFormat(value)
			if err == nil {
				t.Fatalf("ParseFormat(%q) error = nil, want unsupported format error", value)
			}
			if !strings.Contains(err.Error(), "supported: auto, table, json, ndjson, pretty") {
				t.Errorf("ParseFormat(%q) error = %q, want supported formats", value, err.Error())
			}
		})
	}
}

func TestRenderRecordsPrettyWrapsWideTableToWidth(t *testing.T) {
	t.Parallel()

	style := output.NewStyle(false, false) // color off -> byte-clean, easy width math
	style.Width = 50
	long := "192.0.2.10,192.0.2.11,192.0.2.12,192.0.2.13,192.0.2.14,192.0.2.15"
	got := output.RenderRecordsPretty(
		[]string{"id", "name", "ipAddresses"},
		[][]string{{"123", "HQ", long}},
		style,
	).String()

	if strings.Contains(got, "\x1b[") {
		t.Fatalf("pretty (color off) output = %q, want no ANSI escapes", got)
	}
	for _, line := range strings.Split(strings.TrimRight(got, "\n"), "\n") {
		if w := len([]rune(line)); w > style.Width {
			t.Errorf("line width = %d (> %d): %q", w, style.Width, line)
		}
	}
	// The long value must still be present, just wrapped across lines.
	if !strings.Contains(strings.ReplaceAll(got, "\n", ""), "192.0.2.15") {
		t.Errorf("pretty output = %q, want wrapped long value retained", got)
	}
}

func TestRenderRecordsPrettyDoesNotStretchNarrowTable(t *testing.T) {
	t.Parallel()

	style := output.NewStyle(false, false)
	style.Width = 120 // far wider than the content needs
	got := output.RenderRecordsPretty(
		[]string{"id", "name"},
		[][]string{{"1", "HQ"}},
		style,
	).String()

	for _, line := range strings.Split(strings.TrimRight(got, "\n"), "\n") {
		if w := len([]rune(line)); w > 40 {
			t.Errorf("narrow table line width = %d, want it left compact (<=40): %q", w, line)
		}
	}
}

func TestRenderKeyValuesPrettyRendersBorderedTable(t *testing.T) {
	t.Parallel()

	style := output.NewStyle(false, false)
	style.Width = 48
	got := output.RenderKeyValuesPretty([]output.KV{
		{Key: "Status", Value: "OK", Kind: "ok"},
		{Key: "Live API", Value: "requires credentials before contacting Zscaler"},
	}, style).String()

	if strings.Contains(got, "\x1b[") {
		t.Fatalf("key/value pretty (color off) output = %q, want no ANSI escapes", got)
	}
	for _, want := range []string{"┌", "field", "value", "Status", "OK", "Live API"} {
		if !strings.Contains(got, want) {
			t.Errorf("key/value pretty output = %q, want %q", got, want)
		}
	}
	for _, line := range strings.Split(strings.TrimRight(got, "\n"), "\n") {
		if width := lipgloss.Width(line); width > style.Width {
			t.Errorf("key/value pretty line width = %d (> %d): %q", width, style.Width, line)
		}
	}
}

func TestRenderRecordsPrettyKeepsWideCharactersWithinConfiguredWidth(t *testing.T) {
	t.Parallel()

	style := output.NewStyle(false, false)
	style.Width = 44
	got := output.RenderRecordsPretty(
		[]string{"name", "note"},
		[][]string{{
			"東京支社",
			"zero\u200dwidth e\u0301 中中文文中中文文 tail",
		}},
		style,
	).String()

	for _, line := range strings.Split(strings.TrimRight(got, "\n"), "\n") {
		if width := lipgloss.Width(line); width > style.Width {
			t.Errorf("pretty line width = %d (> %d): %q", width, style.Width, line)
		}
	}
	for _, want := range []string{"東京支社", "zero\u200dwidth", "e\u0301", "中中文文中中文文", "tail"} {
		if !strings.Contains(got, want) {
			t.Errorf("pretty output = %q, want retained %q", got, want)
		}
	}
}

func TestRenderKeyValuesPreservesWideAndCombiningText(t *testing.T) {
	t.Parallel()

	got := output.RenderKeyValues([]output.KV{
		{Key: "name", Value: "東京支社"},
		{Key: "note", Value: "zero\u200dwidth e\u0301 tail"},
	}, output.NewStyle(false, false)).String()

	if lines := strings.Count(strings.TrimRight(got, "\n"), "\n") + 1; lines != 2 {
		t.Fatalf("RenderKeyValues line count = %d, want 2: %q", lines, got)
	}
	for _, want := range []string{"name", "東京支社", "note", "zero\u200dwidth", "e\u0301", "tail"} {
		if !strings.Contains(got, want) {
			t.Errorf("RenderKeyValues output = %q, want retained %q", got, want)
		}
	}
}

func TestRendererWriteJSONUsesSecretSafeMarshalAndBackstopRedaction(t *testing.T) {
	t.Parallel()

	const raw = "client-secret-from-response"
	const authorizationCanary = "output-authorization-canary"
	var buf bytes.Buffer
	renderer := output.NewRenderer(redact.New(redact.ModeStandard))
	err := renderer.WriteJSON(&buf, safeJSONFixture{
		AuthHeader: "set the Authorization: Bearer " + authorizationCanary + " header",
		Secret:     secret.New(raw),
	})
	if err != nil {
		t.Fatalf("Renderer.WriteJSON() error = %v, want nil", err)
	}
	got := buf.String()
	if !json.Valid([]byte(got)) {
		t.Errorf("Renderer.WriteJSON() = invalid JSON %q, want valid JSON", got)
	}
	for _, forbidden := range []string{raw, authorizationCanary} {
		if strings.Contains(got, forbidden) {
			t.Errorf("Renderer.WriteJSON() = %q, want no %q", got, forbidden)
		}
	}
}

func TestRendererWriteJSONAppliesShareMode(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	renderer := output.NewRenderer(redact.New(redact.ModeShare))
	err := renderer.WriteJSON(&buf, safeIdentifierFixture{
		Email: "alice@example.com",
		IP:    "192.0.2.10",
	})
	if err != nil {
		t.Fatalf("Renderer.WriteJSON() error = %v, want nil", err)
	}
	got := buf.String()
	for _, forbidden := range []string{"alice@example.com", "192.0.2.10"} {
		if strings.Contains(got, forbidden) {
			t.Errorf("Renderer.WriteJSON() = %q, want no %q", got, forbidden)
		}
	}
}

func TestRendererWriteTextRedactsText(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	renderer := output.NewRenderer(redact.New(redact.ModeStandard))
	err := renderer.WriteText(&buf, output.NewSafeText("Authorization: Basic dXNlcjpzZWNyZXQ=\n"))
	if err != nil {
		t.Fatalf("Renderer.WriteText() error = %v, want nil", err)
	}
	if strings.Contains(buf.String(), "dXNlcjpzZWNyZXQ=") {
		t.Errorf("Renderer.WriteText() = %q, want no basic auth payload", buf.String())
	}
}

func TestRendererWriteNDJSONOneCompactRecordPerLineAndRedacts(t *testing.T) {
	t.Parallel()

	const firstAuthorizationCanary = "first-output-authorization-canary"
	const secondAuthorizationCanary = "second-output-authorization-canary"
	var buf bytes.Buffer
	redactingWriter := redact.NewWriter(&buf, redact.ModeStandard)
	renderer := output.NewRenderer(redact.New(redact.ModeStandard))
	err := renderer.WriteNDJSON(redactingWriter, []output.SafeJSON{
		safeJSONFixture{AuthHeader: "set the Authorization: Bearer " + firstAuthorizationCanary + " header", Secret: secret.New("client-secret-from-response")},
		safeJSONFixture{AuthHeader: "set the Authorization: Bearer " + secondAuthorizationCanary + " header", Secret: secret.New("another-client-secret")},
	})
	if err != nil {
		t.Fatalf("Renderer.WriteNDJSON() error = %v, want nil", err)
	}
	if err := redactingWriter.Close(); err != nil {
		t.Fatalf("redactingWriter.Close() error = %v, want nil", err)
	}
	got := buf.String()
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("WriteNDJSON() produced %d lines, want 2: %q", len(lines), got)
	}
	for _, ln := range lines {
		// Compact: no indentation, and the whole record fits on the one line.
		if strings.HasPrefix(ln, " ") {
			t.Errorf("WriteNDJSON line is not compact (indented): %q", ln)
		}
		var v map[string]any
		if err := json.Unmarshal([]byte(ln), &v); err != nil {
			t.Errorf("WriteNDJSON line is not valid JSON: %q: %v", ln, err)
		}
	}
	// secret.Secret marshals to <REDACTED:SECRET>, so the raw values never appear.
	for _, forbidden := range []string{"client-secret-from-response", "another-client-secret", firstAuthorizationCanary, secondAuthorizationCanary} {
		if strings.Contains(got, forbidden) {
			t.Errorf("WriteNDJSON() = %q, leaked %q", got, forbidden)
		}
	}
	if !strings.Contains(got, "<REDACTED:SECRET>") {
		t.Errorf("WriteNDJSON() = %q, want redaction marker present", got)
	}
}

func TestRendererThroughRedactingWriterPreservesLongJSONNumbers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		write  func(output.Renderer, io.Writer) error
		ndjson bool
	}{
		{
			name: "json",
			write: func(renderer output.Renderer, writer io.Writer) error {
				return renderer.WriteJSON(writer, longNumberJSONFixture{})
			},
		},
		{
			name: "ndjson",
			write: func(renderer output.Renderer, writer io.Writer) error {
				return renderer.WriteNDJSON(writer, []output.SafeJSON{longNumberJSONFixture{}, longNumberJSONFixture{}})
			},
			ndjson: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var destination bytes.Buffer
			outer := redact.NewWriter(&destination, redact.ModeStandard)
			renderer := output.NewRenderer(redact.New(redact.ModeStandard))
			if err := tt.write(renderer, outer); err != nil {
				t.Fatalf("render through redacting writer error = %v", err)
			}
			if err := outer.Close(); err != nil {
				t.Fatalf("redacting writer Close() error = %v", err)
			}

			if !tt.ndjson {
				assertLongNumberJSONFixtureOutput(t, destination.Bytes())
				return
			}

			lines := strings.Split(strings.TrimSuffix(destination.String(), "\n"), "\n")
			if len(lines) != 2 {
				t.Fatalf("rendered NDJSON line count = %d, want 2: %q", len(lines), destination.String())
			}
			for i, line := range lines {
				t.Run(fmt.Sprintf("record-%d", i+1), func(t *testing.T) {
					assertLongNumberJSONFixtureOutput(t, []byte(line))
				})
			}
		})
	}
}

func assertLongNumberJSONFixtureOutput(t *testing.T, body []byte) {
	t.Helper()

	const number = "1234567890123456.789012345678901e+2"
	if !json.Valid(body) {
		t.Fatalf("rendered JSON is invalid: %q", string(body))
	}
	var decoded struct {
		Credential        string      `json:"credential"`
		EscapedCredential string      `json:"escaped_credential"`
		Value             json.Number `json:"value"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("json.Unmarshal(rendered JSON) error = %v; body = %q", err, string(body))
	}
	if decoded.Value.String() != number {
		t.Errorf("rendered numeric value = %q, want %q", decoded.Value, number)
	}
	for field, value := range map[string]string{
		"credential":         decoded.Credential,
		"escaped_credential": decoded.EscapedCredential,
	} {
		if value != "<REDACTED:SECRET>" {
			t.Errorf("rendered %s = %q, want secret marker", field, value)
		}
	}
}

func TestRendererWriteNDJSONEmptyWritesNothing(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	renderer := output.NewRenderer(redact.New(redact.ModeStandard))
	if err := renderer.WriteNDJSON(&buf, nil); err != nil {
		t.Fatalf("Renderer.WriteNDJSON(nil) error = %v, want nil", err)
	}
	if buf.Len() != 0 {
		t.Errorf("WriteNDJSON(nil) = %q, want empty (zero-line NDJSON stream)", buf.String())
	}
}

type safeJSONFixture struct {
	AuthHeader string        `json:"auth_header"`
	Secret     secret.Secret `json:"secret"`
}

func (safeJSONFixture) OutputSafe() {}

type safeIdentifierFixture struct {
	Email string `json:"email"`
	IP    string `json:"ip"`
}

func (safeIdentifierFixture) OutputSafe() {}

type longNumberJSONFixture struct{}

func (longNumberJSONFixture) OutputSafe() {}

func (longNumberJSONFixture) MarshalJSON() ([]byte, error) {
	return []byte(`{"message":"Authorization: Bearer numeric-authorization-canary","credential":"A7b9C2d4E6f8G1h3J5k7L9m2N4p6Q8r0S2t4U6v","escaped_credential":"A7b9C2d\u0034E6f8G1h\u0033J5k7L9m\u0032N4p6Q8r\u0030S2t4U6v","value":1234567890123456.789012345678901e+2}`), nil
}
