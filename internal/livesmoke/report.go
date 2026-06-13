package livesmoke

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/dvmrry/zscalerctl/internal/redact"
)

type resultRow struct {
	resource, phase, status, records, note string
}

// reporter accumulates PASS/INFO/SKIP/FAIL markers and result rows. PASS/INFO/
// SKIP go to out (stdout); FAIL goes to errw (stderr) and counts a failure.
type reporter struct {
	out, errw       io.Writer
	failures        int
	failureMessages []string
	rows            []resultRow
}

func (r *reporter) pass(format string, a ...any) {
	fmt.Fprintf(r.out, "[PASS] %s\n", fmt.Sprintf(format, a...))
}
func (r *reporter) info(format string, a ...any) {
	fmt.Fprintf(r.out, "[INFO] %s\n", fmt.Sprintf(format, a...))
}
func (r *reporter) skip(format string, a ...any) {
	fmt.Fprintf(r.out, "[SKIP] %s\n", fmt.Sprintf(format, a...))
}

func (r *reporter) fail(format string, a ...any) {
	msg := fmt.Sprintf(format, a...)
	fmt.Fprintf(r.errw, "[FAIL] %s\n", msg)
	r.failureMessages = append(r.failureMessages, msg)
	r.failures++
}

func (r *reporter) record(resource, phase, status, records, note string) {
	note = strings.ReplaceAll(note, "\n", " ")
	note = strings.ReplaceAll(note, "|", " ")
	if records == "" {
		records = "-"
	}
	r.rows = append(r.rows, resultRow{resource, phase, status, records, note})
}

func (r *reporter) recordFromFailures(resource, phase string, startFailures int, records, passNote, failNote string) {
	if r.failures == startFailures {
		r.record(resource, phase, "PASS", records, passNote)
	} else {
		r.record(resource, phase, "FAIL", records, failNote)
	}
}

func (r *reporter) printTable(w io.Writer) {
	if len(r.rows) == 0 {
		return
	}
	fmt.Fprintf(w, "\nlive smoke results\n")
	fmt.Fprintf(w, "%-36s  %-10s  %-6s  %-7s  %s\n", "RESOURCE", "PHASE", "STATUS", "RECORDS", "NOTE")
	fmt.Fprintf(w, "%-36s  %-10s  %-6s  %-7s  %s\n", strings.Repeat("-", 36), strings.Repeat("-", 10), "------", "-------", "----")
	for _, row := range r.rows {
		fmt.Fprintf(w, "%-36s  %-10s  %-6s  %-7s  %s\n",
			fitCell(row.resource, 36), fitCell(row.phase, 10), row.status, fitCell(row.records, 7), fitCell(row.note, 72))
	}
	fmt.Fprintf(w, "\n")
}

func fitCell(value string, width int) string {
	value = strings.ReplaceAll(value, "\n", " ")
	if len(value) > width {
		if width <= 3 {
			return value[:width]
		}
		return value[:width-3] + "..."
	}
	return value
}

// writeFailureSummary writes a 0600 failure-summary.txt under outDir and returns
// its path. It lists the failure markers and a compacted snippet per captured
// stderr blob.
func (r *reporter) writeFailureSummary(outDir string, stderrs []namedStderr) (string, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "zscalerctl live-smoke failure summary\n")
	fmt.Fprintf(&b, "failures: %d\n", r.failures)
	fmt.Fprintf(&b, "artifacts: %s\n\n", outDir)
	fmt.Fprintf(&b, "failure markers:\n")
	for _, m := range r.failureMessages {
		fmt.Fprintf(&b, "- %s\n", m)
	}
	fmt.Fprintf(&b, "\nnon-empty stderr snippets:\n")
	any := false
	for _, s := range stderrs {
		if len(strings.TrimSpace(string(s.body))) == 0 {
			continue
		}
		any = true
		fmt.Fprintf(&b, "\n===== %s =====\n%s\n", s.name, compactAndRedactStderr(s.body))
	}
	if !any {
		fmt.Fprintf(&b, "<none>\n")
	}

	path := filepath.Join(outDir, "failure-summary.txt")
	if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
		return "", err
	}
	return path, nil
}

type namedStderr struct {
	name string
	body []byte
}

// compactStderr renders a captured stderr blob for the failure summary: a JSON
// error envelope becomes "error: kind - message"; otherwise the first few lines
// are whitespace-collapsed and truncated. Raw JSON is never echoed verbatim.
func compactStderr(body []byte) string {
	var env struct {
		Error struct {
			Kind    string `json:"kind"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &env); err == nil && env.Error.Kind != "" {
		if env.Error.Message != "" {
			return fmt.Sprintf("error: %s - %s", env.Error.Kind, env.Error.Message)
		}
		return fmt.Sprintf("error: %s", env.Error.Kind)
	}

	const maxLines, maxChars = 4, 220
	lines := strings.Split(strings.TrimRight(string(body), "\n"), "\n")
	var out []string
	for i, line := range lines {
		if i >= maxLines {
			out = append(out, "... <additional stderr omitted; see full artifact file>")
			break
		}
		line = strings.Join(strings.Fields(line), " ")
		if len(line) > maxChars {
			line = line[:maxChars] + "... <truncated>"
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

func compactAndRedactStderr(body []byte) string {
	out, _ := redact.New(redact.ModeStandard).ScanFreeText(compactStderr(body))
	return out
}
