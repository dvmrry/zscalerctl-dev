package zscaler

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strings"

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
	case errors.Is(err, errPaginationValidation):
		// The page walker refused a result it could not prove complete
		// (repeated pages, duplicate records, changing page width).
		return FailurePaginationIncomplete, false
	case errors.Is(err, errNotSubscribed):
		return FailureNotSubscribed, false
	case errors.As(err, &syntaxErr), errors.As(err, &typeErr):
		return FailureDecodeFailed, false
	case sessionRetriesExhausted(err):
		return FailureForbidden, false
	case permanentTransportFailure(err):
		return FailureTransport, false
	case errors.As(err, &netErr) && netErr.Timeout():
		return FailureTimeout, true
	case errors.As(err, &netErr):
		return FailureTransport, true
	}
	return FailureUnknown, false
}

// permanentTransportFailure reports transport failures a retry cannot fix:
// an untrusted or mismatched TLS certificate (often a TLS-intercepting proxy),
// a host that does not resolve (wrong vanity domain or cloud), a redirect
// loop, and proxy authentication. *url.Error implements net.Error, so without
// this check every one of them would read as a retryable transport failure.
func permanentTransportFailure(err error) bool {
	var unknownAuthority x509.UnknownAuthorityError
	var hostname x509.HostnameError
	var invalid x509.CertificateInvalidError
	var verification *tls.CertificateVerificationError
	var dnsErr *net.DNSError
	switch {
	case errors.As(err, &unknownAuthority), errors.As(err, &hostname), errors.As(err, &invalid), errors.As(err, &verification):
		return true
	case errors.As(err, &dnsErr) && dnsErr.IsNotFound:
		return true
	}
	// net/http reports these two as untyped errors.
	text := err.Error()
	return strings.Contains(text, "stopped after") && strings.Contains(text, "redirects") ||
		strings.Contains(text, "Proxy Authentication Required")
}

// sessionRetriesExhausted reports the SDK's give-up after a 401 or 403 that
// stayed session-invalid ("Resource Access Blocked") through every token
// refresh. The SDK returns it as a plain error without the response, so no
// status survives; the refreshed token was accepted and the request still
// refused, which is an access verdict, as the 403 classified before. A
// persistent 401 gets the same forbidden label (the two cannot be told apart
// here); either way it is not retryable. TestSessionRetriesExhaustedMatches-
// TheRealSDKError pins the text to the vendored SDK.
func sessionRetriesExhausted(err error) bool {
	return strings.Contains(err.Error(), "max SESSION_NOT_VALID retries exceeded")
}
