package cli

import (
	"context"
	"strconv"
	"time"

	"github.com/dvmrry/zscalerctl/internal/output"
	"github.com/dvmrry/zscalerctl/internal/redact"
	machineruntime "github.com/dvmrry/zscalerctl/internal/runtime"
)

const adminAuditLogsCommandName = "admin-audit-logs"

type adminAuditLogsStatusOutput struct {
	Status                string `json:"status"`
	ProgressItemsComplete *int   `json:"progress_items_complete,omitempty"`
	CompletionTime        string `json:"completion_time,omitempty"`
	ErrorCode             string `json:"error_code,omitempty"`
}

func (adminAuditLogsStatusOutput) OutputSafe() {}

var adminAuditLogsStatusFieldOrder = []string{
	"status",
	"progress_items_complete",
	"completion_time",
	"error_code",
}

func (a *App) runAdminAuditLogsStatus(ctx context.Context, opts globalOptions) error {
	if opts.format != output.FormatJSON && opts.format != output.FormatTable && opts.format != output.FormatPretty {
		return rejectUnsupportedFormat("zia admin-audit-logs status", opts.format)
	}
	status, err := callWithSpinner(a, opts, "reading administrator audit-log status", func() (machineruntime.AdminAuditLogsStatus, error) {
		return a.readAdminAuditLogsStatus(ctx, opts)
	})
	if err != nil {
		return err
	}
	result := adminAuditLogsStatusOutputFrom(status)
	renderer := output.NewRenderer(redact.New(redact.ModeStandard))
	if opts.format == output.FormatJSON {
		return renderer.WriteJSON(a.out, result)
	}
	return renderer.WriteText(a.out, renderKeyValuesForFormat(
		adminAuditLogsStatusRows(result),
		opts.format,
		a.style(opts),
	))
}

func (a *App) readAdminAuditLogsStatus(
	ctx context.Context,
	opts globalOptions,
) (machineruntime.AdminAuditLogsStatus, error) {
	if a.reader != nil {
		return machineruntime.ReadAdminAuditLogsStatusFromReader(ctx, a.reader)
	}
	return machineruntime.ReadAdminAuditLogsStatus(ctx, machineruntime.Options{
		Env:        a.env,
		Profile:    opts.profile,
		ConfigPath: opts.configPath,
		Timeout:    opts.timeout,
		NoCache:    opts.noCache,
		Catalog:    a.resourceCatalog(),
		DiagLogger: a.sdkDiagLogger(opts),
	})
}

func adminAuditLogsStatusOutputFrom(status machineruntime.AdminAuditLogsStatus) adminAuditLogsStatusOutput {
	result := adminAuditLogsStatusOutput{
		ProgressItemsComplete: status.ProgressItemsComplete,
	}
	switch status.Status {
	case "in_progress", "completed", "failed", "unknown":
		result.Status = status.Status
	default:
		result.Status = "unknown"
	}
	if result.ProgressItemsComplete != nil && *result.ProgressItemsComplete < 0 {
		result.ProgressItemsComplete = nil
	}
	if completionTime, err := time.Parse(time.RFC3339Nano, status.CompletionTime); err == nil {
		result.CompletionTime = completionTime.UTC().Format(time.RFC3339Nano)
	}
	if status.ErrorCode == "unknown" {
		result.ErrorCode = status.ErrorCode
	}
	return result
}

func adminAuditLogsStatusRows(status adminAuditLogsStatusOutput) []output.KV {
	rows := []output.KV{
		{Key: "status", Value: status.Status, Kind: "status"},
	}
	if status.ProgressItemsComplete != nil {
		rows = append(rows, output.KV{
			Key:   "progress_items_complete",
			Value: strconv.Itoa(*status.ProgressItemsComplete),
			Kind:  "number",
		})
	}
	if status.CompletionTime != "" {
		rows = append(rows, output.KV{
			Key:   "completion_time",
			Value: status.CompletionTime,
			Kind:  "timestamp",
		})
	}
	if status.ErrorCode != "" {
		rows = append(rows, output.KV{
			Key:   "error_code",
			Value: status.ErrorCode,
			Kind:  "error_code",
		})
	}
	return rows
}
