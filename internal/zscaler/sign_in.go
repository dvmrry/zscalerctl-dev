package zscaler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	zsdk "github.com/zscaler/zscaler-sdk-go/v3/zscaler"
)

// Sign-in requests: the OneAPI token endpoint (the SDK signs in eagerly, in
// NewOneAPIClient) and the legacy ZIA session endpoint (signed in lazily,
// before the first read). Both are POSTs; the legacy logout is a DELETE.
var signInPaths = []string{"/oauth2/v1/token", "/api/v1/authenticatedSession"}

// signInError is a failed sign-in: a non-2xx status (classifyFailure reads
// 400, 401 and 403 as credentials_rejected) or a request that got no response
// (a transport cause). Both SDK clients report a
// failed sign-in as text, dropping the status and the error chain, so
// classifyFailure would see neither and say unknown. signInTransport raises
// this typed error instead. Its text is static: the SDK's echoes the client ID
// and the response body, and a transport cause can name the host; the cause
// stays reachable through Unwrap for classification.
type signInError struct {
	status int   // sign-in HTTP status; 0 when there was no response
	cause  error // transport error when there was no response
}

func (e *signInError) Error() string {
	if e.cause != nil {
		return "sign-in failed (no response)"
	}
	return fmt.Sprintf("sign-in failed (HTTP %d)", e.status)
}

func (e *signInError) Unwrap() error { return e.cause }

// signInTransport fails a sign-in request that got an error status or no
// response with a signInError, and keeps the last one it raised. The legacy
// ZIA client returns a transport error unwrapped, so the signInError reaches
// classifyFailure as is. The OneAPI client formats it away (%v), so
// newOneAPIService restores the kept one. Every other request passes through.
type signInTransport struct {
	base http.RoundTripper
	mu   sync.Mutex
	last *signInError
}

func withSignInTransport(client *http.Client) *signInTransport {
	base := client.Transport
	if base == nil {
		base = http.DefaultTransport
	}
	transport := &signInTransport{base: base}
	client.Transport = transport
	return transport
}

func (t *signInTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	response, err := t.base.RoundTrip(request)
	if !isSignInRequest(request) {
		return response, err
	}
	var failure *signInError
	switch {
	case err != nil:
		failure = &signInError{cause: signInTimeout(request, err)}
	case response.StatusCode < 200 || response.StatusCode > 299:
		// Any non-2xx, so the client never follows a redirect from the
		// sign-in endpoint and re-sends the credentials to another host.
		if response.Body != nil {
			_ = response.Body.Close()
		}
		failure = &signInError{status: response.StatusCode}
	default:
		return response, nil
	}
	t.mu.Lock()
	t.last = failure
	t.mu.Unlock()
	return nil, failure
}

// signInTimeout marks a sign-in that failed at or after its deadline as a
// timeout. Any wrapper around the transport makes net/http enforce
// Client.Timeout by closing Request.Cancel, which the transport reports as a
// plain "request canceled" that classifies as unknown. net/http still sets
// the same deadline on the request's context, so a failure past it is the
// timeout. A different failure that lands within microseconds of the
// deadline is labelled a timeout too; checking ctx.Err() instead would miss
// timeouts that the cancel path reports first.
func signInTimeout(request *http.Request, err error) error {
	if deadline, ok := request.Context().Deadline(); ok && !time.Now().Before(deadline) && !errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("%w: %w", context.DeadlineExceeded, err)
	}
	return err
}

func (t *signInTransport) lastFailure() *signInError {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.last
}

func isSignInRequest(request *http.Request) bool {
	if request.Method != http.MethodPost {
		return false
	}
	for _, path := range signInPaths {
		if strings.HasSuffix(request.URL.Path, path) {
			return true
		}
	}
	return false
}

// newOneAPIService builds the OneAPI service, which signs in. A failed
// sign-in is returned as the signInError the transport raised, which the SDK's
// own error has flattened to text.
func newOneAPIService(cfg *zsdk.Configuration) (*zsdk.Service, error) {
	transport := withSignInTransport(cfg.HTTPClient)
	service, err := zsdk.NewOneAPIClient(cfg)
	if err != nil {
		if failure := transport.lastFailure(); failure != nil && !errors.As(err, new(*signInError)) {
			return nil, failure
		}
		return nil, err
	}
	return service, nil
}
