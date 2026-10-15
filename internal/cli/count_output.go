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

// countedStringFlag is a string flag that records how many times it was set.
type countedStringFlag struct {
	value string
	sets  int
}

func (f *countedStringFlag) String() string { return f.value }

// Get makes it a flag.Getter of a string, like the stdlib string flag.
func (f *countedStringFlag) Get() any { return f.value }

func (f *countedStringFlag) Set(value string) error {
	f.value = value
	f.sets++
	return nil
}

// validateCountByFlag rejects a --count-by that names no path (an empty or
// comma-only value would otherwise silently list every record) and a repeated
// --count-by (a plain string flag keeps only the last occurrence).
func validateCountByFlag(f *countedStringFlag) error {
	switch {
	case f.sets == 0:
		return nil
	case f.sets > 1:
		return UsageError{Message: "--count-by was given more than once; pass one comma-separated list, such as --count-by groups.id,department.name"}
	case len(parseFieldsList(f.value)) == 0:
		return UsageError{Message: "--count-by needs at least one field path, such as --count-by groups.id"}
	}
	return nil
}

// countResult is the --count-by output: how many records were counted, how
// many distinct value combinations they hold, and the counts, largest first.
// Values come from projected, redacted records; nothing else is emitted.
type countResult struct {
	CountBy  []string `json:"count_by"`
	Records  int      `json:"records"`
	Distinct int      `json:"distinct"`
	// Truncated is true when --limit kept fewer rows than Distinct.
	Truncated bool       `json:"truncated"`
	Counts    []countRow `json:"counts"`
}

func (countResult) OutputSafe() {}

// countRow renders as {"values": {<path>: value, ...}, "count": N}, the
// values keyed by counted path in --count-by order (null when the record had
// none). Nesting the values keeps the row keys fixed, whatever the paths are
// called.
type countRow struct {
	paths []string
	row   resources.CountRow
}

func (countRow) OutputSafe() {}

func (r countRow) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer
	b.WriteString(`{"values":{`)
	for i, path := range r.paths {
		key, err := json.Marshal(path)
		if err != nil {
			return nil, err
		}
		value, err := json.Marshal(r.row.Values[i])
		if err != nil {
			return nil, err
		}
		if i > 0 {
			b.WriteByte(',')
		}
		b.Write(key)
		b.WriteByte(':')
		b.Write(value)
	}
	fmt.Fprintf(&b, `},"count":%d}`, r.row.Count)
	return b.Bytes(), nil
}

func (a *App) writeCounts(cfg config.Config, opts globalOptions, records resources.ProjectedRecords) error {
	rows := resources.CountProjectedRecords(records, opts.countBy)
	result := countResult{CountBy: opts.countBy, Records: records.Len(), Distinct: len(rows)}
	if opts.limitSet && opts.limit < len(rows) {
		rows = rows[:opts.limit]
		result.Truncated = true
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
	fmt.Fprintf(&body, "%d records, %d distinct", result.Records, result.Distinct)
	if result.Truncated {
		fmt.Fprintf(&body, ", showing the %d largest", len(result.Counts))
	}
	body.WriteByte('\n')
	return output.NewSafeText(body.String())
}
