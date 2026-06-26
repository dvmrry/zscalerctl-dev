package cli

import (
	"encoding/json"
	"strings"
	"testing"

	dumpdiff "github.com/dvmrry/zscalerctl/internal/diff"
	"github.com/dvmrry/zscalerctl/internal/output"
)

func TestWriteDiffDetailRowsEscapesTerminalControls(t *testing.T) {
	t.Parallel()

	var body strings.Builder
	writeDiffDetailRows(&body, "zia/locations\x1b[2J", dumpdiff.ResourceDiff{
		Added: []dumpdiff.RecordRef{
			{Key: "added\x1b[2J"},
		},
		Removed: []dumpdiff.RecordRef{
			{Key: "removed\x1b]0;owned\x07"},
		},
		Changed: []dumpdiff.RecordChange{
			{
				Key: "changed\nhidden\tfakecol\u202e",
				Changes: []dumpdiff.FieldChange{
					{Field: "ansi\x1b[2J"},
					{Field: "osc\x1b]0;owned\x07"},
					{Field: "line\nhidden"},
					{Field: "tab\tfakecol"},
					{Field: "bidi\u202e"},
				},
			},
		},
	})
	got := body.String()

	for _, want := range []string{
		`zia/locations\x1b[2J`,
		`added\x1b[2J`,
		`removed\x1b]0;owned\x07`,
		`changed\nhidden\tfakecol\u202e`,
		`ansi\x1b[2J`,
		`osc\x1b]0;owned\x07`,
		`line\nhidden`,
		`tab\tfakecol`,
		`bidi\u202e`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("writeDiffDetailRows() = %q, want visible escape %q", got, want)
		}
	}
	if strings.ContainsRune(got, '\x1b') {
		t.Fatalf("writeDiffDetailRows() = %q, want no raw ESC bytes", got)
	}
	if strings.ContainsRune(got, '\u202e') {
		t.Fatalf("writeDiffDetailRows() = %q, want no raw bidi override", got)
	}
	for _, r := range got {
		if r < 0x20 && r != '\t' && r != '\n' {
			t.Fatalf("writeDiffDetailRows() = %q, want no raw control rune %#U", got, r)
		}
	}

	lines := strings.Split(strings.TrimSuffix(got, "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("writeDiffDetailRows() rows = %d, want 3; output=%q", len(lines), got)
	}
	for _, line := range lines {
		if tabs := strings.Count(line, "\t"); tabs != 4 {
			t.Fatalf("writeDiffDetailRows() row %q has %d tab separators, want 4", line, tabs)
		}
	}
}

func TestTerminalCellPreservesPrintableText(t *testing.T) {
	t.Parallel()

	const value = "zia/locations branch-01_name"
	if got := terminalCell(value); got != value {
		t.Fatalf("terminalCell(%q) = %q, want unchanged", value, got)
	}
}

func TestDiffReportJSONKeepsRawControlValues(t *testing.T) {
	t.Parallel()

	report := diffControlReport()
	_ = renderDiffTable(report, true, output.Style{})

	body, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("json.Marshal(diffControlReport()) error = %v, want nil", err)
	}
	if strings.Contains(string(body), `\x1b`) {
		t.Fatalf("json.Marshal(diffControlReport()) = %s, want no human-cell escaping", body)
	}

	var decoded dumpdiff.Report
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("json.Unmarshal(diffControlReport()) error = %v, want nil", err)
	}
	change := decoded.Resources[0].Changed[0]
	if want := "changed\nhidden\tfakecol\u202e"; change.Key != want {
		t.Fatalf("json round-trip changed key = %q, want %q", change.Key, want)
	}
	if want := "ansi\x1b[2J"; change.Changes[0].Field != want {
		t.Fatalf("json round-trip field[0] = %q, want %q", change.Changes[0].Field, want)
	}
	if want := "osc\x1b]0;owned\x07"; change.Changes[1].Field != want {
		t.Fatalf("json round-trip field[1] = %q, want %q", change.Changes[1].Field, want)
	}
}

func diffControlReport() dumpdiff.Report {
	return dumpdiff.Report{
		Schema: dumpdiff.SchemaID,
		Resources: []dumpdiff.ResourceDiff{
			{
				Product:  "zia",
				Resource: "locations",
				Added: []dumpdiff.RecordRef{
					{Key: "added\x1b[2J"},
				},
				Removed: []dumpdiff.RecordRef{
					{Key: "removed\x1b]0;owned\x07"},
				},
				Changed: []dumpdiff.RecordChange{
					{
						Key: "changed\nhidden\tfakecol\u202e",
						Changes: []dumpdiff.FieldChange{
							{Field: "ansi\x1b[2J"},
							{Field: "osc\x1b]0;owned\x07"},
							{Field: "line\nhidden"},
							{Field: "tab\tfakecol"},
							{Field: "bidi\u202e"},
						},
					},
				},
			},
		},
	}
}
