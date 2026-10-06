package runtime

import (
	"context"
	"errors"
	"fmt"

	"github.com/dvmrry/zscalerctl/internal/browser"
	"github.com/dvmrry/zscalerctl/internal/config"
	"github.com/dvmrry/zscalerctl/internal/machine"
	"github.com/dvmrry/zscalerctl/internal/zscaler"
)

const adminAuditLogsResourceName = "admin-audit-logs"

// AdminAuditLogsStatus is the trusted status view returned by the live
// Zscaler adapter.
type AdminAuditLogsStatus = zscaler.AdminAuditLogsStatus

type adminAuditLogsStatusReader interface {
	AdminAuditLogsStatus(context.Context) (zscaler.AdminAuditLogsStatus, error)
}

var _ adminAuditLogsStatusReader = (*zscaler.SDKReader)(nil)

// ReadAdminAuditLogsStatus loads effective configuration, constructs the
// configured reader, and reads the current report status.
func ReadAdminAuditLogsStatus(ctx context.Context, opts Options) (AdminAuditLogsStatus, error) {
	ctx = nonNilContext(ctx)
	if err := ctx.Err(); err != nil {
		return AdminAuditLogsStatus{}, adminAuditLogsStatusBoundaryError(err)
	}
	loadConfig := opts.loadConfig
	if loadConfig == nil {
		loadConfig = config.LoadConfig
	}
	cfg, err := loadConfig(append([]string(nil), opts.Env...), config.LoadOptions{
		Profile:    opts.Profile,
		ConfigPath: opts.ConfigPath,
	})
	if err != nil {
		return AdminAuditLogsStatus{}, adminAuditLogsStatusBoundaryError(err)
	}
	reader, err := NewReaderFromConfig(ctx, cfg, opts)
	if err != nil {
		return AdminAuditLogsStatus{}, adminAuditLogsStatusBoundaryError(err)
	}
	return ReadAdminAuditLogsStatusFromReader(ctx, reader)
}

// ReadAdminAuditLogsStatusFromReader reads status through an already trusted
// record reader, keeping CLI tests and in-process adapters independent of SDK
// construction.
func ReadAdminAuditLogsStatusFromReader(
	ctx context.Context,
	reader browser.RecordReader,
) (AdminAuditLogsStatus, error) {
	ctx = nonNilContext(ctx)
	if err := ctx.Err(); err != nil {
		return AdminAuditLogsStatus{}, adminAuditLogsStatusBoundaryError(err)
	}
	if reader == nil {
		return AdminAuditLogsStatus{}, browser.ErrMissingReader
	}
	statusReader, ok := reader.(adminAuditLogsStatusReader)
	if !ok {
		return AdminAuditLogsStatus{}, fmt.Errorf(
			"%w: zia/%s",
			zscaler.ErrUnsupportedResource,
			adminAuditLogsResourceName,
		)
	}
	status, err := statusReader.AdminAuditLogsStatus(ctx)
	// A context that ended during the read is reported as cancellation or a
	// deadline even when the reader returned a status or another error.
	if ctxErr := ctx.Err(); ctxErr != nil {
		return AdminAuditLogsStatus{}, adminAuditLogsStatusBoundaryError(ctxErr)
	}
	if err != nil {
		return AdminAuditLogsStatus{}, adminAuditLogsStatusBoundaryError(err)
	}
	return status, nil
}

func adminAuditLogsStatusBoundaryError(err error) error {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return machine.ErrorWithCause(&machine.MachineError{
			Kind:      machine.ErrorKindDeadlineExceeded,
			Message:   "request deadline exceeded",
			Operation: machine.Operation("status"),
			Product:   "zia",
			Resource:  adminAuditLogsResourceName,
		}, context.DeadlineExceeded)
	case errors.Is(err, context.Canceled):
		return machine.ErrorWithCause(&machine.MachineError{
			Kind:      machine.ErrorKindCanceled,
			Message:   "request canceled",
			Operation: machine.Operation("status"),
			Product:   "zia",
			Resource:  adminAuditLogsResourceName,
		}, context.Canceled)
	default:
		return err
	}
}
