package zscaler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	zsdk "github.com/zscaler/zscaler-sdk-go/v3/zscaler"

	"github.com/dvmrry/zscalerctl/internal/resources"
)

const (
	adminAuditLogsResourceName     = "admin-audit-logs"
	adminAuditLogsStatusEndpoint   = "/zia/api/v1/auditlogEntryReport"
	adminAuditLogsStatusUnknown    = "unknown"
	adminAuditLogsErrorCodeUnknown = "unknown"
	maxAuditLogsStatusUnixMilli    = int64(253402300799999)
)

// AdminAuditLogsStatus is the safe, adapter-owned view of the current shared
// administrator audit-log report status.
type AdminAuditLogsStatus struct {
	Status                string
	ProgressItemsComplete *int
	CompletionTime        string
	ErrorCode             string
}

type adminAuditLogsStatusResponse struct {
	Status                *string `json:"status"`
	ProgressItemsComplete *int    `json:"progressItemsComplete"`
	ProgressEndTime       int64   `json:"progressEndTime"`
	ErrorCode             string  `json:"errorCode"`
}

// AdminAuditLogsStatus performs one status operation (HTTP retries follow the
// shared SDK policy). The response is copied into a closed, safe view; an
// omitted or null progress count stays nil, and API errorMessage is never
// decoded or returned.
func (r *SDKReader) AdminAuditLogsStatus(ctx context.Context) (AdminAuditLogsStatus, error) {
	if r == nil {
		return AdminAuditLogsStatus{}, fmt.Errorf("%w: %s/%s", ErrUnsupportedResource, resources.ProductZIA, adminAuditLogsResourceName)
	}
	return readAdminAuditLogsStatus(ctx, sdkClient{services: perCallService{cfg: r.cfg}})
}

func readAdminAuditLogsStatus(ctx context.Context, client sdkClient) (AdminAuditLogsStatus, error) {
	call := ziaSDKShow(client, getZIAAdminAuditLogsStatus)
	response, err := call(ctx)
	if err != nil {
		if errors.Is(err, ErrMissingCredentials) {
			return AdminAuditLogsStatus{}, err
		}
		// A context that ended during the read wins over the HTTP status: the
		// SDK can still report a 404 whose body read was interrupted.
		if sdkStatusCode(err) == http.StatusNotFound && ctx.Err() == nil {
			return AdminAuditLogsStatus{}, adminAuditLogsStatusUnavailableError{}
		}
		return AdminAuditLogsStatus{}, normalizeLiveError(ctx, "status", resources.ProductZIA, adminAuditLogsResourceName, err)
	}
	if response == nil || response.Status == nil || strings.TrimSpace(*response.Status) == "" {
		return AdminAuditLogsStatus{}, normalizeLiveError(
			ctx,
			"status",
			resources.ProductZIA,
			adminAuditLogsResourceName,
			errors.New("status response is missing status"),
		)
	}
	return adminAuditLogsStatusFromResponse(*response), nil
}

func getZIAAdminAuditLogsStatus(
	ctx context.Context,
	service *zsdk.Service,
) (*adminAuditLogsStatusResponse, error) {
	var response adminAuditLogsStatusResponse
	if err := service.Client.Read(ctx, adminAuditLogsStatusEndpoint, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

func adminAuditLogsStatusFromResponse(response adminAuditLogsStatusResponse) AdminAuditLogsStatus {
	statusValue := ""
	if response.Status != nil {
		statusValue = *response.Status
	}
	status := AdminAuditLogsStatus{
		Status:                normalizeAdminAuditLogsStatus(statusValue),
		ProgressItemsComplete: response.ProgressItemsComplete,
		CompletionTime:        normalizeAdminAuditLogsCompletionTime(response.ProgressEndTime),
		ErrorCode:             normalizeAdminAuditLogsErrorCode(response.ErrorCode),
	}
	// A negative count is not a measurement; report progress as unavailable
	// rather than as a measured zero.
	if status.ProgressItemsComplete != nil && *status.ProgressItemsComplete < 0 {
		status.ProgressItemsComplete = nil
	}
	return status
}

func normalizeAdminAuditLogsStatus(value string) string {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "IN_PROGRESS":
		return "in_progress"
	case "COMPLETED":
		return "completed"
	case "FAILED":
		return "failed"
	default:
		return adminAuditLogsStatusUnknown
	}
}

func normalizeAdminAuditLogsCompletionTime(value int64) string {
	if value <= 0 || value > maxAuditLogsStatusUnixMilli {
		return ""
	}
	return time.UnixMilli(value).UTC().Format(time.RFC3339Nano)
}

func normalizeAdminAuditLogsErrorCode(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	return adminAuditLogsErrorCodeUnknown
}

type adminAuditLogsStatusUnavailableError struct{}

func (adminAuditLogsStatusUnavailableError) Error() string {
	return fmt.Sprintf("%v: zia/%s status unavailable", ErrResourceNotFound, adminAuditLogsResourceName)
}

func (adminAuditLogsStatusUnavailableError) Unwrap() error {
	return ErrResourceNotFound
}

func (adminAuditLogsStatusUnavailableError) ErrorContext() ErrorContext {
	return ErrorContext{
		Product:   string(resources.ProductZIA),
		Resource:  adminAuditLogsResourceName,
		Operation: "status",
	}
}
