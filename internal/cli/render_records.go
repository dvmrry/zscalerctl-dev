package cli

import (
	"fmt"
	"strings"

	"github.com/dvmrry/zscalerctl/internal/config"
	"github.com/dvmrry/zscalerctl/internal/output"
	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
	machineruntime "github.com/dvmrry/zscalerctl/internal/runtime"
)

// effectiveFields returns the field order to render: the renderable fields for
// the mode, optionally narrowed to --fields. --fields can only select from the
// already-renderable set; an unknown field name (not in the catalog at all) is
// a usage error, while a known-but-not-rendered field (secret or mode-excluded)
// is silently skipped, so --fields can never widen the sanitized output.
func effectiveFields(spec resources.ResourceSpec, mode redact.Mode, requested []string) ([]string, error) {
	fields, err := resources.EffectiveFields(spec, mode, requested)
	if err != nil {
		return nil, UsageError{Message: err.Error()}
	}
	return fields, nil
}

func (a *App) writeProjectedRecord(
	cfg config.Config,
	opts globalOptions,
	spec resources.ResourceSpec,
	record resources.ProjectedRecord,
	operation string,
) error {
	fields, err := effectiveFields(spec, cfg.Defaults.Redaction, opts.fields)
	if err != nil {
		return err
	}
	switch opts.format {
	case output.FormatJSON:
		return a.renderer(cfg, opts).WriteJSON(a.out, record)
	case output.FormatNDJSON:
		return a.renderer(cfg, opts).WriteNDJSON(a.out, []output.SafeJSON{record})
	case output.FormatTable, output.FormatPretty:
		records := resources.NewProjectedRecords([]resources.ProjectedRecord{record})
		return a.renderer(cfg, opts).WriteScannedText(a.out, func(scan func(map[string]any) map[string]any) output.SafeText {
			switch {
			case opts.format == output.FormatTable && operation == "show":
				return renderRecordKeyValues(fields, record, scan, a.style(opts))
			case opts.format == output.FormatTable:
				return renderRecordsTable(fields, records, scan, a.style(opts))
			case operation == "show":
				return renderRecordPretty(fields, record, scan, a.style(opts))
			default:
				return renderRecordsPretty(fields, records, scan, a.style(opts))
			}
		})
	default:
		return fmt.Errorf("unhandled output format %q for resource %s", opts.format, operation)
	}
}

func (a *App) writeProjectedRecords(
	cfg config.Config,
	opts globalOptions,
	spec resources.ResourceSpec,
	records resources.ProjectedRecords,
) error {
	fields, err := effectiveFields(spec, cfg.Defaults.Redaction, opts.fields)
	if err != nil {
		return err
	}
	switch opts.format {
	case output.FormatJSON:
		if opts.limitSet {
			return a.renderer(cfg, opts).WriteJSON(a.out, newListPage(records, opts.offset, opts.limit))
		}
		return a.renderer(cfg, opts).WriteJSON(a.out, records)
	case output.FormatNDJSON:
		return a.renderer(cfg, opts).WriteNDJSON(a.out, safeJSONRecords(records))
	case output.FormatTable:
		return a.renderer(cfg, opts).WriteScannedText(a.out, func(scan func(map[string]any) map[string]any) output.SafeText {
			return renderRecordsTable(fields, records, scan, a.style(opts))
		})
	case output.FormatPretty:
		return a.renderer(cfg, opts).WriteScannedText(a.out, func(scan func(map[string]any) map[string]any) output.SafeText {
			return renderRecordsPretty(fields, records, scan, a.style(opts))
		})
	default:
		return fmt.Errorf("unhandled output format %q for resource list", opts.format)
	}
}

// safeJSONRecords adapts projected records to the output layer's SafeJSON slice
// for NDJSON rendering (one element per line). It preserves order; an empty set
// yields an empty slice, which WriteNDJSON renders as zero lines.
func safeJSONRecords(records resources.ProjectedRecords) []output.SafeJSON {
	recs := records.Records()
	out := make([]output.SafeJSON, len(recs))
	for i := range recs {
		out[i] = recs[i]
	}
	return out
}

func (a *App) renderer(cfg config.Config, _ globalOptions) output.Renderer {
	return output.NewRenderer(redact.New(cfg.Defaults.Redaction))
}

func renderRecordsTable(
	fields []string,
	records resources.ProjectedRecords,
	scan func(map[string]any) map[string]any,
	style output.Style,
) output.SafeText {
	var body strings.Builder
	for i, field := range fields {
		if i > 0 {
			body.WriteByte('\t')
		}
		body.WriteString(style.Key(field))
	}
	body.WriteByte('\n')
	for _, record := range records.Records() {
		for i, cell := range scannedCells(fields, record, scan) {
			if i > 0 {
				body.WriteByte('\t')
			}
			body.WriteString(style.Value(fields[i], cell))
		}
		body.WriteByte('\n')
	}
	return output.NewSafeText(body.String())
}

func renderRecordKeyValues(
	fields []string,
	record resources.ProjectedRecord,
	scan func(map[string]any) map[string]any,
	style output.Style,
) output.SafeText {
	return output.RenderKeyValues(scannedKeyValues(fields, record, scan), style)
}

func renderKeyValuesForFormat(rows []output.KV, format output.Format, style output.Style) output.SafeText {
	if format == output.FormatPretty {
		return output.RenderKeyValuesPretty(rows, style)
	}
	return output.RenderKeyValues(rows, style)
}

func renderRecordsPretty(
	fields []string,
	records resources.ProjectedRecords,
	scan func(map[string]any) map[string]any,
	style output.Style,
) output.SafeText {
	rows := make([][]string, 0, records.Len())
	for _, record := range records.Records() {
		rows = append(rows, scannedCells(fields, record, scan))
	}
	return output.RenderRecordsPretty(fields, rows, style)
}

func renderRecordPretty(
	fields []string,
	record resources.ProjectedRecord,
	scan func(map[string]any) map[string]any,
	style output.Style,
) output.SafeText {
	return output.RenderRecordPretty(scannedKeyValues(fields, record, scan), style)
}

// scannedCells renders one cell per field from the scanned record.
func scannedCells(
	fields []string,
	record resources.ProjectedRecord,
	scan func(map[string]any) map[string]any,
) []string {
	values := scan(record.Fields())
	cells := make([]string, len(fields))
	for i, field := range fields {
		cells[i] = formatTableValue(fieldValue(values, field))
	}
	return cells
}

// scannedKeyValues renders one row per field from the scanned record.
func scannedKeyValues(
	fields []string,
	record resources.ProjectedRecord,
	scan func(map[string]any) map[string]any,
) []output.KV {
	cells := scannedCells(fields, record, scan)
	rows := make([]output.KV, len(fields))
	for i, field := range fields {
		rows[i] = output.KV{Key: field, Kind: field, Value: cells[i]}
	}
	return rows
}

// The record renderers write through Renderer.WriteScannedText: they render
// every value from scan(record.Fields()), the record JSON output emits,
// scanned as JSON output scans it; the rest is field names (catalog paths,
// --fields validated against the catalog) and layout.

// fieldValue returns what a --fields name (a top-level field or a dotted path)
// reaches in a record, or nil when it reaches nothing.
func fieldValue(values map[string]any, field string) any {
	value, _ := resources.ProjectedPathValue(values, field)
	return value
}

// formatTableValue renders a value for the terminal-oriented table, pretty, and
// key-value sinks. Machine JSON uses a separate lossless path.
func formatTableValue(value any) string {
	return output.TerminalCell(resources.ProjectedValueText(value))
}

func doctorStatusRows(status machineruntime.DoctorStatus) []output.KV {
	return []output.KV{
		{Key: "Status", Value: status.Status, Kind: doctorStatusKind(status.Status)},
		{Key: "Mode", Value: status.Mode, Kind: "mode"},
		{Key: "Profile", Value: status.Profile},
		{Key: "Config", Value: status.Config},
		{Key: "Auth Mode", Value: status.AuthMode},
		{Key: "Redaction", Value: status.Redaction},
		{Key: "Timeout", Value: status.Timeout},
		{Key: "Cache", Value: status.Cache},
		{Key: "Proxy", Value: status.Proxy},
		{Key: "Credentials", Value: status.Credentials},
		{Key: "Live API", Value: status.LiveAPI},
	}
}

// doctorStatusKind colors the doctor status: green when live reads can run,
// a warning otherwise.
func doctorStatusKind(status string) string {
	if status == "OK" {
		return "ok"
	}
	return "warning"
}

func authStatusRows(status machineruntime.AuthStatus) []output.KV {
	return []output.KV{
		{Key: "Credentials", Value: status.Credentials},
		{Key: "Credential Exchange", Value: status.CredentialExchange},
		{Key: "Live API", Value: status.LiveAPI},
	}
}
