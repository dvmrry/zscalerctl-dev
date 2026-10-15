package zscaler

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	zsdk "github.com/zscaler/zscaler-sdk-go/v3/zscaler"
	zidusers "github.com/zscaler/zscaler-sdk-go/v3/zscaler/ziam/services/users"
)

// TestSessionRetriesExhaustedMatchesTheRealSDKError drives a real SDK client,
// configured as the reader configures it, against an API that answers every
// request with 403 "Resource Access Blocked". The SDK refreshes the token,
// gives up, and returns its own session-retry error; this pins the text that
// sessionRetriesExhausted matches to the vendored SDK, so an SDK bump that
// rewords it fails here instead of silently reporting unknown again.
func TestSessionRetriesExhaustedMatchesTheRealSDKError(t *testing.T) {
	t.Parallel()

	cfg := validReaderConfig()
	sdkCfg := newSDKConfiguration(context.Background(), cfg)
	var apiCalls int
	sdkCfg.HTTPClient.Transport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		status, body := http.StatusForbidden, `{"code":"Resource Access Blocked","message":"Resource Access Blocked"}`
		if request.URL.Path == "/oauth2/v1/token" {
			// Keep expiry within the SDK's one-minute renewal window so the
			// test does not start its background renewal goroutine.
			status, body = http.StatusOK, `{"access_token":"test-token","expires_in":60}`
		} else {
			apiCalls++
		}
		return &http.Response{
			StatusCode: status,
			Status:     http.StatusText(status),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(body)),
			Request:    request,
		}, nil
	})
	service, err := zsdk.NewOneAPIClient(sdkCfg)
	if err != nil {
		t.Fatalf("NewOneAPIClient() error = %v", err)
	}
	t.Cleanup(service.Client.Close)

	_, err = zidusers.GetUser(context.Background(), service, "user-1")
	if err == nil {
		t.Fatal("GetUser() error = nil, want the SDK's session-retry error")
	}
	if !sessionRetriesExhausted(err) {
		t.Fatalf("sessionRetriesExhausted(%q) = false; the SDK's give-up text changed", err)
	}
	if reason, retryable := classifyFailure(err, sdkStatusCode(err)); reason != FailureForbidden || retryable {
		t.Errorf("classifyFailure(real SDK session-retry error) = (%q, %v), want (forbidden, false)", reason, retryable)
	}
	if apiCalls < 2 {
		t.Errorf("API calls = %d, want the SDK to have retried after a token refresh", apiCalls)
	}
}
