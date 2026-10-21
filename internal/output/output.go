package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/dvmrry/zscalerctl/internal/redact"
)

type Format string

const (
	// FormatAuto resolves at render time: a TTY destination gets pretty,
	// everything else (pipe, file) gets json. It is the default so interactive
	// use is readable while pipelines stay machine-parseable.
	FormatAuto  Format = "auto"
	FormatTable Format = "table"
	FormatJSON  Format = "json"
	// FormatNDJSON is newline-delimited JSON: one compact record per line, for
	// streaming/pipeline consumers (jq -c, SIEM ingest). It only applies to record
	// output (resource list/get/show); other commands reject it like any
	// unsupported format.
	FormatNDJSON Format = "ndjson"
	FormatPretty Format = "pretty"
)

func ParseFormat(value string) (Format, error) {
	switch Format(strings.ToLower(strings.TrimSpace(value))) {
	case "", FormatAuto:
		return FormatAuto, nil
	case FormatTable:
		return FormatTable, nil
	case FormatJSON:
		return FormatJSON, nil
	case FormatNDJSON:
		return FormatNDJSON, nil
	case FormatPretty:
		return FormatPretty, nil
	default:
		return "", fmt.Errorf("unsupported output format %q; supported: auto, table, json, ndjson, pretty", value)
	}
}

type Renderer struct {
	Redactor redact.Redactor
}

type SafeJSON interface {
	OutputSafe()
}

// SafeText marks text assembled from static strings or projected data. It is
// not a sanitizer: WriteText scans the whole text before output, and
// WriteScannedText requires its builder to render from scanned records.
type SafeText string

func NewSafeText(value string) SafeText {
	return SafeText(value)
}

func (t SafeText) String() string {
	return string(t)
}

func NewRenderer(redactor redact.Redactor) Renderer {
	return Renderer{Redactor: redactor}
}

func (r Renderer) WriteJSON(w io.Writer, value SafeJSON) error {
	body, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal json: %w", err)
	}
	body = append(body, '\n')
	if _, err := w.Write(r.Redactor.Bytes(body)); err != nil {
		return fmt.Errorf("write json: %w", err)
	}
	return nil
}

// WriteNDJSON writes records as newline-delimited JSON: one compact, redacted
// JSON object per line, for streaming/pipeline consumers. Each record is marshaled
// WITHOUT indentation and followed by '\n'; an empty set writes nothing (the empty
// NDJSON stream). Redaction is applied per line — the same fail-closed pass
// WriteJSON uses — so NDJSON can never reveal more than the indented JSON for the
// same projected data.
func (r Renderer) WriteNDJSON(w io.Writer, records []SafeJSON) error {
	for _, record := range records {
		body, err := json.Marshal(record)
		if err != nil {
			return fmt.Errorf("marshal ndjson: %w", err)
		}
		body = append(body, '\n')
		if _, err := w.Write(r.Redactor.Bytes(body)); err != nil {
			return fmt.Errorf("write ndjson: %w", err)
		}
	}
	return nil
}

func (r Renderer) WriteText(w io.Writer, value SafeText) error {
	if _, err := w.Write(r.Redactor.Bytes([]byte(value.String()))); err != nil {
		return fmt.Errorf("write text: %w", err)
	}
	return nil
}

// WriteScannedText writes record text without WriteText's whole-text scan.
// build gets scan (ScanRecord) and must render every value from records it
// has scanned; everything else it writes is layout and catalog vocabulary
// (field names, --fields paths validated against the catalog). The whole-text
// scan reads layout as prose: a cell rendering a nested object as
// map[zpaAuthExpOnWinLogonSession:480] matched the plain-text assignment
// rules, so table and pretty showed markers where JSON showed values.
// Scanning each record as JSON output scans it makes them agree.
func (r Renderer) WriteScannedText(w io.Writer, build func(scan func(map[string]any) map[string]any) SafeText) error {
	if _, err := io.WriteString(w, build(r.ScanRecord).String()); err != nil {
		return fmt.Errorf("write text: %w", err)
	}
	return nil
}

// ScanRecord applies WriteJSON's final scan to one projected record, as a
// whole: the record's members are scanned with their siblings, so the rules
// that read context (a {"key": "password", "value": ...} pair, a key that
// marks its sibling as a credential) see what they see in JSON output. The
// record is encoded, scanned as a document and decoded with UseNumber, so
// numbers render as JSON writes them (1000000, not 1e+06). A record that does
// not encode, or whose scan does not decode, has every member replaced by the
// secret marker.
func (r Renderer) ScanRecord(record map[string]any) map[string]any {
	body, err := json.Marshal(record)
	if err == nil {
		decoder := json.NewDecoder(bytes.NewReader(r.Redactor.Bytes(body)))
		decoder.UseNumber()
		var out map[string]any
		if err := decoder.Decode(&out); err == nil {
			return out
		}
	}
	redacted := make(map[string]any, len(record))
	for key := range record {
		redacted[key] = redactedValue
	}
	return redacted
}

const redactedValue = "<REDACTED:SECRET>"
