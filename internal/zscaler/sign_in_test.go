package zscaler

import (
	"context"
	"crypto/x509"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/rule_labels"

	"github.com/dvmrry/zscalerctl/internal/machine"
)

// TestSignInFailuresClassifyByOutcome drives the real SDK sign-in, OneAPI
// (eager, in NewOneAPIClient) and legacy ZIA (lazy, before the first read),
// against a sign-in endpoint that fails each way an operator meets: rejected
// credentials, a host that does not resolve (wrong vanity domain or cloud) and
// a TLS-intercepting proxy. Both SDK clients flatten these to text; each must
// still classify by its status or transport error, not as unknown, and the
// error must not echo the client ID or the response body.
func TestSignInFailuresClassifyByOutcome(t *testing.T) {
	t.Parallel()
	const body = `{"error":"invalid_client","error_description":"sign-in-body-canary"}`
	respond := func(status int) roundTripFunc {
		return func(request *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: status,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(body)),
				Request:    request,
			}, nil
		}
	}
	fail := func(err error) roundTripFunc {
		return func(*http.Request) (*http.Response, error) { return nil, err }
	}
	for _, test := range []struct {
		name          string
		transport     roundTripFunc
		wantReason    string
		wantRetryable bool
		wantStatus    int
	}{
		{"rejected client secret", respond(http.StatusUnauthorized), FailureCredentialsRejected, false, http.StatusUnauthorized},
		{"client without access", respond(http.StatusForbidden), FailureCredentialsRejected, false, http.StatusForbidden},
		{"invalid client", respond(http.StatusBadRequest), FailureCredentialsRejected, false, http.StatusBadRequest},
		{"rate limited", respond(http.StatusTooManyRequests), FailureRateLimited, true, http.StatusTooManyRequests},
		{"wrong sign-in host", respond(http.StatusNotFound), FailureNotFound, false, http.StatusNotFound},
		{"token endpoint outage", respond(http.StatusServiceUnavailable), FailureServerError, true, http.StatusServiceUnavailable},
		{"unknown vanity domain", fail(&net.DNSError{Err: "no such host", Name: "zscalerctl-vanity.zslogin.net", IsNotFound: true}), FailureTransport, false, 0},
		{"intercepting proxy", fail(x509.UnknownAuthorityError{}), FailureTransport, false, 0},
		{"connection reset", fail(&net.OpError{Op: "read", Net: "tcp", Err: errors.New("connection reset by peer")}), FailureTransport, true, 0},
	} {
		check := func(t *testing.T, err error) {
			t.Helper()
			var signIn *signInError
			if !errors.As(err, &signIn) {
				t.Fatalf("error = %v, want a signInError", err)
			}
			for _, leak := range []string{"zscalerctl-client-id", "zscalerctl-zia-admin", "sign-in-body-canary", "zscalerctl-vanity"} {
				if strings.Contains(err.Error(), leak) {
					t.Errorf("error %q echoes %q", err, leak)
				}
			}
			status := sdkStatusCode(err)
			reason, retryable := classifyFailure(err, status)
			if reason != test.wantReason || retryable != test.wantRetryable || status != test.wantStatus {
				t.Errorf("classify = (%q, retryable %v, status %d), want (%q, %v, %d)",
					reason, retryable, status, test.wantReason, test.wantRetryable, test.wantStatus)
			}
			// The reason crosses the machine boundary, and a sign-in 404
			// during a get is a live failure (exit 5), not a missing ID.
			live := normalizeLiveError(context.Background(), "get", "zia", "rule-labels", err)
			if !errors.Is(live, ErrLiveAccessFailed) {
				t.Errorf("normalizeLiveError(get) = %v, want ErrLiveAccessFailed", live)
			}
			if got := machine.FailureDetailOf(live); got.Reason != test.wantReason || got.Status != test.wantStatus || got.Retryable != test.wantRetryable {
				t.Errorf("machine.FailureDetailOf() = %+v, want reason %q status %d retryable %v", got, test.wantReason, test.wantStatus, test.wantRetryable)
			}
		}
		t.Run("oneapi "+test.name, func(t *testing.T) {
			t.Parallel()
			cfg := newSDKConfiguration(context.Background(), validReaderConfig())
			cfg.HTTPClient.Transport = test.transport
			service, err := newOneAPIService(cfg)
			if err == nil {
				service.Client.Close()
				t.Fatal("newOneAPIService() error = nil, want a sign-in failure")
			}
			check(t, err)
		})
		t.Run("legacy "+test.name, func(t *testing.T) {
			t.Parallel()
			service := newLegacyZIATestService(t, test.transport)
			_, err := rule_labels.GetAll(context.Background(), service)
			if err == nil {
				t.Fatal("rule_labels.GetAll() error = nil, want a sign-in failure")
			}
			check(t, err)
		})
	}
}

// A OneAPI sign-in that outlives Client.Timeout is a timeout. The sign-in
// transport wrapper makes net/http cancel through Request.Cancel, so a real
// http.Transport reports a plain "request canceled", which on its own
// classifies as unknown. net/http also sets the deadline on the request
// context, and the transport races the two; the inner round trip drops the
// context's cancellation so only Request.Cancel can end it, which pins the
// plain-cancel outcome deterministically.
func TestOneAPISignInTimeoutClassifiesAsTimeout(t *testing.T) {
	t.Parallel()
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-release:
		}
	}))
	t.Cleanup(func() { close(release); server.Close() })
	target, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	base := &http.Transport{}
	t.Cleanup(base.CloseIdleConnections)
	cfg := newSDKConfiguration(context.Background(), validReaderConfig())
	cfg.HTTPClient.Timeout = 100 * time.Millisecond
	cfg.HTTPClient.Transport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		request.URL.Scheme, request.URL.Host = target.Scheme, target.Host
		return base.RoundTrip(request.WithContext(context.WithoutCancel(request.Context())))
	})
	service, err := newOneAPIService(cfg)
	if err == nil {
		service.Client.Close()
		t.Fatal("newOneAPIService() error = nil, want a sign-in timeout")
	}
	if !errors.As(err, new(*signInError)) || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v, want a signInError that is context.DeadlineExceeded", err)
	}
	// What the operator sees: a deadline_exceeded error with reason timeout.
	live := normalizeLiveError(context.Background(), "list", "zia", "rule-labels", err)
	if !errors.Is(live, context.DeadlineExceeded) {
		t.Errorf("normalizeLiveError() = %v, want a deadline error", live)
	}
	if got := machine.FailureDetailOf(live); got.Reason != FailureTimeout || !got.Retryable {
		t.Errorf("machine.FailureDetailOf() = %+v, want reason timeout, retryable", got)
	}
}

// A redirect from the sign-in endpoint is a failure, never followed: net/http
// would re-send the POST body, which carries the credentials, to the new host.
func TestSignInDoesNotFollowRedirects(t *testing.T) {
	t.Parallel()
	transport := func(followed *int) roundTripFunc {
		return func(request *http.Request) (*http.Response, error) {
			if request.URL.Host == "redirect.example.invalid" {
				*followed++
			}
			return &http.Response{
				StatusCode: http.StatusTemporaryRedirect,
				Header:     http.Header{"Location": []string{"https://redirect.example.invalid/oauth2/v1/token"}},
				Body:       io.NopCloser(strings.NewReader("")),
				Request:    request,
			}, nil
		}
	}
	var oneAPIFollowed, legacyFollowed int
	cfg := newSDKConfiguration(context.Background(), validReaderConfig())
	cfg.HTTPClient.Transport = transport(&oneAPIFollowed)
	if service, err := newOneAPIService(cfg); err == nil {
		service.Client.Close()
		t.Error("newOneAPIService() error = nil, want a sign-in failure")
	} else if got := sdkStatusCode(err); got != http.StatusTemporaryRedirect {
		t.Errorf("sdkStatusCode(oneapi redirect) = %d, want 307", got)
	}
	if _, err := rule_labels.GetAll(context.Background(), newLegacyZIATestService(t, transport(&legacyFollowed))); err == nil {
		t.Error("legacy rule_labels.GetAll() error = nil, want a sign-in failure")
	}
	if oneAPIFollowed != 0 || legacyFollowed != 0 {
		t.Errorf("redirect followed: oneapi %d, legacy %d times, want 0", oneAPIFollowed, legacyFollowed)
	}
}
