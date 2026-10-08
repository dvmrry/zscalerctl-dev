package cli_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/cli"
	"github.com/dvmrry/zscalerctl/internal/machine"
	"github.com/dvmrry/zscalerctl/internal/zscaler"
)

type fakeAdminAuditLogsStatusReader struct {
	fakeResourceReader
	status zscaler.AdminAuditLogsStatus
	err    error
	calls  int
}

func (r *fakeAdminAuditLogsStatusReader) AdminAuditLogsStatus(context.Context) (zscaler.AdminAuditLogsStatus, error) {
	r.calls++
	if r.err != nil {
		return zscaler.AdminAuditLogsStatus{}, r.err
	}
	return r.status, nil
}

func TestAdminAuditLogsStatusRendersJSON(t *testing.T) {
	t.Parallel()

	reader := &fakeAdminAuditLogsStatusReader{
		status: zscaler.AdminAuditLogsStatus{
			Status:                "completed",
			ProgressItemsComplete: intPtr(12),
			CompletionTime:        "2023-11-14T22:13:20Z",
			ErrorCode:             "unknown",
		},
	}
	var out, errOut bytes.Buffer
	app := cli.NewWithOptions(&out, &errOut, nil, cli.Options{Reader: reader})

	if err := app.Run(context.Background(), []string{"--format", "json", "zia", "admin-audit-logs", "status"}); err != nil {
		t.Fatalf("App.Run(zia admin-audit-logs status json) error = %v, want nil", err)
	}
	want := "{\n  \"status\": \"completed\",\n  \"progress_items_complete\": 12,\n  \"completion_time\": \"2023-11-14T22:13:20Z\",\n  \"error_code\": \"unknown\"\n}\n"
	if got := out.String(); got != want {
		t.Errorf("status JSON = %q, want %q", got, want)
	}
	if reader.calls != 1 {
		t.Errorf("AdminAuditLogsStatus calls = %d, want 1", reader.calls)
	}
	if errOut.Len() != 0 {
		t.Errorf("stderr = %q, want empty", errOut.String())
	}
}

func TestAdminAuditLogsStatusOmitsUnavailableProgress(t *testing.T) {
	t.Parallel()

	for _, format := range []string{"json", "table", "pretty"} {
		t.Run(format, func(t *testing.T) {
			reader := &fakeAdminAuditLogsStatusReader{
				status: zscaler.AdminAuditLogsStatus{Status: "completed"},
			}
			var out bytes.Buffer
			app := cli.NewWithOptions(&out, &bytes.Buffer{}, nil, cli.Options{Reader: reader})
			if err := app.Run(context.Background(), []string{"--format", format, "zia", "admin-audit-logs", "status"}); err != nil {
				t.Fatalf("App.Run(zia admin-audit-logs status %s) error = %v, want nil", format, err)
			}
			if strings.Contains(out.String(), "progress_items_complete") {
				t.Errorf("%s output = %q, want unavailable progress omitted", format, out.String())
			}
			if format == "json" {
				want := "{\n  \"status\": \"completed\"\n}\n"
				if got := out.String(); got != want {
					t.Errorf("status JSON = %q, want %q", got, want)
				}
			}
		})
	}
}

func TestAdminAuditLogsStatusRendersTableAndPretty(t *testing.T) {
	t.Parallel()

	for _, format := range []string{"table", "pretty"} {
		t.Run(format, func(t *testing.T) {
			reader := &fakeAdminAuditLogsStatusReader{
				status: zscaler.AdminAuditLogsStatus{
					Status:                "in_progress",
					ProgressItemsComplete: intPtr(4),
				},
			}
			var out bytes.Buffer
			app := cli.NewWithOptions(&out, &bytes.Buffer{}, nil, cli.Options{Reader: reader})
			if err := app.Run(context.Background(), []string{"--format", format, "zia", "admin-audit-logs", "status"}); err != nil {
				t.Fatalf("App.Run(zia admin-audit-logs status %s) error = %v, want nil", format, err)
			}
			for _, want := range []string{"status", "in_progress", "progress_items_complete", "4"} {
				if !strings.Contains(out.String(), want) {
					t.Errorf("%s output = %q, want %q", format, out.String(), want)
				}
			}
		})
	}
}

func TestAdminAuditLogsStatusSanitizesAdapterValues(t *testing.T) {
	t.Parallel()

	reader := &fakeAdminAuditLogsStatusReader{
		status: zscaler.AdminAuditLogsStatus{
			Status:                "unreviewed-api-status",
			ProgressItemsComplete: intPtr(-3),
			CompletionTime:        "not a timestamp",
			ErrorCode:             "tenant-value-from-api",
		},
	}
	var out bytes.Buffer
	app := cli.NewWithOptions(&out, &bytes.Buffer{}, nil, cli.Options{Reader: reader})
	if err := app.Run(context.Background(), []string{"--format", "json", "zia", "admin-audit-logs", "status"}); err != nil {
		t.Fatalf("App.Run(zia admin-audit-logs status sanitized) error = %v, want nil", err)
	}
	if !strings.Contains(out.String(), "\"status\": \"unknown\"") {
		t.Errorf("status JSON = %q, want unknown status", out.String())
	}
	if strings.Contains(out.String(), "progress_items_complete") {
		t.Errorf("status JSON = %q, want negative progress count omitted", out.String())
	}
	for _, forbidden := range []string{"unreviewed-api-status", "tenant-value-from-api", "not a timestamp"} {
		if strings.Contains(out.String(), forbidden) {
			t.Errorf("status JSON = %q, want no %q", out.String(), forbidden)
		}
	}
}

func TestAdminAuditLogsStatusRejectsNDJSONBeforeRead(t *testing.T) {
	t.Parallel()

	reader := &fakeAdminAuditLogsStatusReader{}
	app := cli.NewWithOptions(&bytes.Buffer{}, &bytes.Buffer{}, nil, cli.Options{Reader: reader})
	err := app.Run(context.Background(), []string{"--format", "ndjson", "zia", "admin-audit-logs", "status"})
	if !errors.Is(err, cli.ErrUsage) {
		t.Fatalf("App.Run(zia admin-audit-logs status ndjson) error = %v, want ErrUsage", err)
	}
	if reader.calls != 0 {
		t.Errorf("AdminAuditLogsStatus calls = %d, want 0 for unsupported format", reader.calls)
	}
}

// cancelingAdminAuditLogsStatusReader cancels the command context during the
// read and then returns a valid status or error.
type cancelingAdminAuditLogsStatusReader struct {
	fakeAdminAuditLogsStatusReader
	cancel context.CancelFunc
}

func (r *cancelingAdminAuditLogsStatusReader) AdminAuditLogsStatus(ctx context.Context) (zscaler.AdminAuditLogsStatus, error) {
	r.cancel()
	return r.fakeAdminAuditLogsStatusReader.AdminAuditLogsStatus(ctx)
}

func TestAdminAuditLogsStatusCancellationDuringReadWins(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		err  error
	}{
		{name: "valid status", err: nil},
		{name: "not found", err: fmt.Errorf("%w: zia/admin-audit-logs", zscaler.ErrResourceNotFound)},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			reader := &cancelingAdminAuditLogsStatusReader{cancel: cancel}
			reader.status = zscaler.AdminAuditLogsStatus{Status: "completed"}
			reader.err = test.err
			var out bytes.Buffer
			app := cli.NewWithOptions(&out, &bytes.Buffer{}, nil, cli.Options{Reader: reader})
			err := app.Run(ctx, []string{"--format", "json", "zia", "admin-audit-logs", "status"})
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("App.Run(canceled during read) error = %v, want context.Canceled", err)
			}
			if errors.Is(err, zscaler.ErrResourceNotFound) {
				t.Errorf("App.Run(canceled during read) error = %v, want cancellation, not not-found", err)
			}
			if out.Len() != 0 {
				t.Errorf("stdout = %q, want empty after cancellation", out.String())
			}
		})
	}
}

func TestAdminAuditLogsStatusDeadlineUsesLiveReadExitCode(t *testing.T) {
	t.Parallel()

	const canary = "private-deadline-detail"
	reader := &fakeAdminAuditLogsStatusReader{
		err: fmt.Errorf("%w: %s", context.DeadlineExceeded, canary),
	}
	app := cli.NewWithOptions(&bytes.Buffer{}, &bytes.Buffer{}, nil, cli.Options{Reader: reader})
	err := app.Run(context.Background(), []string{"--format", "json", "zia", "admin-audit-logs", "status"})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("App.Run(status timeout) error = %v, want wrapped deadline", err)
	}
	var machineErr *machine.MachineError
	if !errors.As(err, &machineErr) {
		t.Fatalf("App.Run(status timeout) error %T is not a machine error", err)
	}
	if machineErr.Kind != machine.ErrorKindDeadlineExceeded {
		t.Errorf("status timeout kind = %q, want %q", machineErr.Kind, machine.ErrorKindDeadlineExceeded)
	}
	if code, ok := cli.ExitCodeForMachineErrorKind(machineErr.Kind); !ok || code != 5 {
		t.Errorf("status timeout exit mapping = %d, %t, want 5", code, ok)
	}
	if strings.Contains(err.Error(), canary) {
		t.Errorf("status timeout error = %q, want value-free message", err)
	}
}
