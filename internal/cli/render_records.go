package cli

import (
	"fmt"
	"io"
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
	case output.FormatTable:
		if operation == "show" {
			return a.renderer(cfg, opts).WriteText(a.out, renderRecordKeyValues(fields, record, a.style(opts)))
		}
		return a.renderer(cfg, opts).WriteText(a.out, renderRecordsTable(fields, resources.NewProjectedRecords([]resources.ProjectedRecord{record}), a.style(opts)))
	case output.FormatPretty:
		if operation == "show" {
			return a.renderer(cfg, opts).WriteText(a.out, renderRecordPretty(fields, record, a.style(opts)))
		}
		return a.renderer(cfg, opts).WriteText(a.out, renderRecordsPretty(fields, resources.NewProjectedRecords([]resources.ProjectedRecord{record}), a.style(opts)))
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
	errW := redact.NewWriter(a.err, cfg.Defaults.Redaction)
	warnUnknownFilterKeys(errW, spec, opts.filters)
	if err := errW.Close(); err != nil {
		return err
	}
	switch opts.format {
	case output.FormatJSON:
		return a.renderer(cfg, opts).WriteJSON(a.out, records)
	case output.FormatNDJSON:
		return a.renderer(cfg, opts).WriteNDJSON(a.out, safeJSONRecords(records))
	case output.FormatTable:
		return a.renderer(cfg, opts).WriteText(a.out, renderRecordsTable(fields, records, a.style(opts)))
	case output.FormatPretty:
		return a.renderer(cfg, opts).WriteText(a.out, renderRecordsPretty(fields, records, a.style(opts)))
	default:
		return fmt.Errorf("unhandled output format %q for resource list", opts.format)
	}
}

func warnUnknownFilterKeys(w io.Writer, spec resources.ResourceSpec, filters []recordFilter) {
	if len(filters) == 0 {
		return
	}
	catalog := make(map[string]struct{}, len(spec.Fields))
	for _, field := range spec.Fields {
		catalog[field.JSONField()] = struct{}{}
	}
	warned := make(map[string]struct{}, len(filters))
	for _, filter := range filters {
		if _, ok := catalog[filter.key]; ok {
			continue
		}
		if _, ok := warned[filter.key]; ok {
			continue
		}
		warned[filter.key] = struct{}{}
		fmt.Fprintf(w, "warning: --filter key %q is not a field of %s/%s\n", filter.key, spec.Product, spec.Name)
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
		values := record.Fields()
		for i, field := range fields {
			if i > 0 {
				body.WriteByte('\t')
			}
			body.WriteString(style.Value(field, formatTableValue(values[field])))
		}
		body.WriteByte('\n')
	}
	return output.NewSafeText(body.String())
}

func renderRecordKeyValues(
	fields []string,
	record resources.ProjectedRecord,
	style output.Style,
) output.SafeText {
	values := record.Fields()
	rows := make([]output.KV, 0, len(fields))
	for _, field := range fields {
		rows = append(rows, output.KV{
			Key:   field,
			Kind:  field,
			Value: formatTableValue(values[field]),
		})
	}
	return output.RenderKeyValues(rows, style)
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
	style output.Style,
) output.SafeText {
	rows := make([][]string, 0, records.Len())
	for _, record := range records.Records() {
		values := record.Fields()
		row := make([]string, len(fields))
		for i, field := range fields {
			row[i] = formatTableValue(values[field])
		}
		rows = append(rows, row)
	}
	return output.RenderRecordsPretty(fields, rows, style)
}

func renderRecordPretty(
	fields []string,
	record resources.ProjectedRecord,
	style output.Style,
) output.SafeText {
	values := record.Fields()
	rows := make([]output.KV, 0, len(fields))
	for _, field := range fields {
		rows = append(rows, output.KV{
			Key:   field,
			Kind:  field,
			Value: formatTableValue(values[field]),
		})
	}
	return output.RenderRecordPretty(rows, style)
}

// formatTableValue renders a value for the terminal-oriented table, pretty, and
// key-value sinks. Machine JSON uses a separate lossless path.
func formatTableValue(value any) string {
	return output.TerminalCell(resources.ProjectedValueText(value))
}

func doctorStatusRows(status machineruntime.DoctorStatus) []output.KV {
	return []output.KV{
		{Key: "Status", Value: status.Status, Kind: "ok"},
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

func authStatusRows(status machineruntime.AuthStatus) []output.KV {
	return []output.KV{
		{Key: "Credentials", Value: status.Credentials},
		{Key: "Credential Exchange", Value: status.CredentialExchange},
		{Key: "Live API", Value: status.LiveAPI},
	}
}
