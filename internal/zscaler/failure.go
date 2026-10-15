package zscaler

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"

	"github.com/dvmrry/zscalerctl/internal/machine"
)

// The reader classifies into machine's failure-reason vocabulary.
const (
	FailureUnauthorized         = machine.FailureReasonUnauthorized
	FailureForbidden            = machine.FailureReasonForbidden
	FailureNotFound             = machine.FailureReasonNotFound
	FailureRateLimited          = machine.FailureReasonRateLimited
	FailureServerError          = machine.FailureReasonServerError
	FailureClientError          = machine.FailureReasonClientError
	FailureTransport            = machine.FailureReasonTransport
	FailureTimeout              = machine.FailureReasonTimeout
	FailurePaginationIncomplete = machine.FailureReasonPaginationIncomplete
	FailureDecodeFailed         = machine.FailureReasonDecodeFailed
	FailureNotSubscribed        = machine.FailureReasonNotSubscribed
	FailureUnknown              = machine.FailureReasonUnknown
)

// errNotSubscribed marks a read the tenant's licence does not include. Its text
// keeps the SDK wrapper's status-free "NOT_SUBSCRIBED: ..." operator message.
var errNotSubscribed = errors.New("NOT_SUBSCRIBED")

// classifyFailure maps a live-read error and its HTTP status (0 when the
// failure happened before or outside an HTTP response) to a failure reason
// and whether a retry could plausibly succeed.
func classifyFailure(err error, status int) (reason string, retryable bool) {
	switch {
	case status == http.StatusUnauthorized:
		return FailureUnauthorized, false
	case status == http.StatusForbidden:
		return FailureForbidden, false
	case status == http.StatusNotFound:
		return FailureNotFound, false
	case status == http.StatusTooManyRequests:
		return FailureRateLimited, true
	case status == http.StatusRequestTimeout:
		return FailureTimeout, true
	case status >= 500:
		return FailureServerError, true
	case status >= 400:
		return FailureClientError, false
	}
	var syntaxErr *json.SyntaxError
	var typeErr *json.UnmarshalTypeError
	var netErr net.Error
	switch {
	case errors.Is(err, errZiaPaginationValidation):
		// The page walker refused a result it could not prove complete
		// (repeated pages, duplicate records, changing page width).
		return FailurePaginationIncomplete, false
	case errors.Is(err, errNotSubscribed):
		return FailureNotSubscribed, false
	case errors.As(err, &syntaxErr), errors.As(err, &typeErr):
		return FailureDecodeFailed, false
	case errors.As(err, &netErr) && netErr.Timeout():
		return FailureTimeout, true
	case errors.As(err, &netErr):
		return FailureTransport, true
	}
	return FailureUnknown, false
}
