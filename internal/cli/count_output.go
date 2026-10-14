package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dvmrry/zscalerctl/internal/config"
	"github.com/dvmrry/zscalerctl/internal/output"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

// validateCountByInvocation rejects flags that would silently change what
// --count-by counts. --fields would drop the counted fields before counting,
// and --offset has no meaning for a ranked summary; --limit keeps the N largest
// counts.
func validateCountByInvocation(opts globalOptions) error {
	if len(opts.countBy) == 0 {
		return nil
	}
	if len(opts.fields) > 0 {
		return UsageError{Message: "--fields cannot be combined with --count-by; the output already holds only the counted fields"}
	}
	if opts.offsetSet {
		return UsageError{Message: "--offset cannot be combined with --count-by; use --limit N for the N largest counts"}
	}
	return nil
}

// countResult is the --count-by output: how many records were counted, how
// many distinct value combinations they hold, and the counts, largest first.
// Values come from projected, redacted records; nothing else is emitted.
type countResult struct {
	CountBy  []string   `json:"count_by"`
	Records  int        `json:"records"`
	Distinct int        `json:"distinct"`
	Counts   []countRow `json:"counts"`
}

func (countResult) OutputSafe() {}

// countRow renders as one flat object: each counted path with its value (null
// when the record had none), then the count.
type countRow struct {
	paths []string
	row   resources.CountRow
}

func (countRow) OutputSafe() {}

func (r countRow) countKey() string {
	for _, path := range r.paths {
		if path == "count" {
			return "record_count"
		}
	}
	return "count"
}

func (r countRow) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, path := range r.paths {
		key, err := json.Marshal(path)
		if err != nil {
			return nil, err
		}
		value, err := json.Marshal(r.row.Values[i])
		if err != nil {
			return nil, err
		}
		b.Write(key)
		b.WriteByte(':')
		b.Write(value)
		b.WriteByte(',')
	}
	fmt.Fprintf(&b, "%q:%d}", r.countKey(), r.row.Count)
	return b.Bytes(), nil
}

func (a *App) writeCounts(cfg config.Config, opts globalOptions, records resources.ProjectedRecords) error {
	rows := resources.CountProjectedRecords(records, opts.countBy)
	result := countResult{CountBy: opts.countBy, Records: records.Len(), Distinct: len(rows)}
	if opts.limitSet && opts.limit < len(rows) {
		rows = rows[:opts.limit]
	}
	result.Counts = make([]countRow, len(rows))
	for i, row := range rows {
		result.Counts[i] = countRow{paths: opts.countBy, row: row}
	}
	renderer := a.renderer(cfg, opts)
	switch opts.format {
	case output.FormatJSON:
		return renderer.WriteJSON(a.out, result)
	case output.FormatNDJSON:
		lines := make([]output.SafeJSON, len(result.Counts))
		for i := range result.Counts {
			lines[i] = result.Counts[i]
		}
		return renderer.WriteNDJSON(a.out, lines)
	case output.FormatTable, output.FormatPretty:
		return renderer.WriteText(a.out, renderCountTable(result, a.style(opts)))
	default:
		return fmt.Errorf("unhandled output format %q for --count-by", opts.format)
	}
}

func renderCountTable(result countResult, style output.Style) output.SafeText {
	var body strings.Builder
	for _, path := range result.CountBy {
		body.WriteString(style.Key(path))
		body.WriteByte('\t')
	}
	body.WriteString(style.Key("count"))
	body.WriteByte('\n')
	for _, row := range result.Counts {
		for i, path := range result.CountBy {
			value := row.row.Values[i]
			text := "(none)"
			if value != nil {
				text = formatTableValue(value)
			}
			body.WriteString(style.Value(path, text))
			body.WriteByte('\t')
		}
		fmt.Fprintf(&body, "%d\n", row.row.Count)
	}
	fmt.Fprintf(&body, "%d records, %d distinct\n", result.Records, result.Distinct)
	return output.NewSafeText(body.String())
}
