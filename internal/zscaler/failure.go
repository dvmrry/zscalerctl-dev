package zscaler

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strings"
)

// Failure reasons are a closed, value-free vocabulary describing why a live
// read failed. They let a caller choose between retrying, fixing the API
// role, or giving up, without exposing the backend message (which can carry
// tenant values). Keep in sync with docs/schema/error.schema.json and
// docs/schema/dump-error.schema.json.
const (
	FailureUnauthorized         = "unauthorized"
	FailureForbidden            = "forbidden"
	FailureNotFound             = "not_found"
	FailureRateLimited          = "rate_limited"
	FailureServerError          = "server_error"
	FailureClientError          = "client_error"
	FailureTransport            = "transport"
	FailureTimeout              = "timeout"
	FailurePaginationIncomplete = "pagination_incomplete"
	FailureDecodeFailed         = "decode_failed"
	FailureNotSubscribed        = "not_subscribed"
	FailureUnknown              = "unknown"
)

// failureReasons is the whole vocabulary; a test keeps the published error
// and dump-error schema enums equal to it.
var failureReasons = []string{
	FailureUnauthorized, FailureForbidden, FailureNotFound, FailureRateLimited,
	FailureServerError, FailureClientError, FailureTransport, FailureTimeout,
	FailurePaginationIncomplete, FailureDecodeFailed, FailureNotSubscribed, FailureUnknown,
}

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
	case err != nil && strings.Contains(err.Error(), "NOT_SUBSCRIBED"):
		// The reader converts the subscription error to a status-free
		// "NOT_SUBSCRIBED: ..." error; match it wherever it is wrapped.
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
