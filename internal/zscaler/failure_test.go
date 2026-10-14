package zscaler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/resources"
	sdkerrorx "github.com/zscaler/zscaler-sdk-go/v3/zscaler/errorx"
)

// TestNormalizeLiveErrorClassifiesFailures pins the value-free failure
// vocabulary: every live-read failure gets a reason, the HTTP status when
// there was one, and a retry hint, and none of it carries the backend body.
func TestNormalizeLiveErrorClassifiesFailures(t *testing.T) {
	t.Parallel()

	sdkStatus := func(code int) error {
		return &sdkerrorx.ErrorResponse{
			Response: &http.Response{StatusCode: code, Status: http.StatusText(code)},
			Message:  "client_secret=raw-sdk-body",
		}
	}
	var syntaxErr error = &json.SyntaxError{Offset: 3}
	tests := []struct {
		name          string
		err           error
		wantReason    string
		wantStatus    int
		wantRetryable bool
	}{
		{"401", sdkStatus(http.StatusUnauthorized), FailureUnauthorized, 401, false},
		{"403", sdkStatus(http.StatusForbidden), FailureForbidden, 403, false},
		{"list 404", sdkStatus(http.StatusNotFound), FailureNotFound, 404, false},
		{"429", sdkStatus(http.StatusTooManyRequests), FailureRateLimited, 429, true},
		{"503", sdkStatus(http.StatusServiceUnavailable), FailureServerError, 503, true},
		{"409", sdkStatus(http.StatusConflict), FailureClientError, 409, false},
		{"pagination", fmt.Errorf("%w: repeated page content", errZiaPaginationValidation), FailurePaginationIncomplete, 0, false},
		{"not subscribed", errors.New("NOT_SUBSCRIBED: Cloud Browser Isolation subscription is required"), FailureNotSubscribed, 0, false},
		{"decode", fmt.Errorf("decode page: %w", syntaxErr), FailureDecodeFailed, 0, false},
		{"transport", &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("connection refused")}, FailureTransport, 0, true},
		{"408", sdkStatus(http.StatusRequestTimeout), FailureTimeout, 408, true},
		{"network timeout", &net.DNSError{Err: "i/o timeout", IsTimeout: true}, FailureTimeout, 0, true},
		{"wrapped not subscribed", fmt.Errorf("read: %w", errors.New("NOT_SUBSCRIBED: subscription is required")), FailureNotSubscribed, 0, false},
		{"other", errors.New("something else"), FailureUnknown, 0, false},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := normalizeLiveError(context.Background(), "list", resources.ProductZIA, "url-categories", tt.err)
			if !errors.Is(got, ErrLiveAccessFailed) {
				t.Fatalf("normalizeLiveError(%s) = %v, want ErrLiveAccessFailed", tt.name, got)
			}
			var detailer interface {
				FailureDetail() (string, int, bool)
			}
			if !errors.As(got, &detailer) {
				t.Fatalf("normalizeLiveError(%s) = %T, want a FailureDetail", tt.name, got)
			}
			reason, status, retryable := detailer.FailureDetail()
			if reason != tt.wantReason || status != tt.wantStatus || retryable != tt.wantRetryable {
				t.Errorf("FailureDetail() = (%q, %d, %v), want (%q, %d, %v)",
					reason, status, retryable, tt.wantReason, tt.wantStatus, tt.wantRetryable)
			}
			if strings.Contains(got.Error(), "raw-sdk-body") || strings.Contains(got.Error(), "something else") {
				t.Errorf("normalizeLiveError(%s) = %q leaks the backend message", tt.name, got.Error())
			}
		})
	}
}

// TestFailureReasonsMatchPublishedSchemas keeps the reason vocabulary, the
// error envelope schema and the dump error schema in step.
func TestFailureReasonsMatchPublishedSchemas(t *testing.T) {
	t.Parallel()

	want := append([]string(nil), failureReasons...)
	slices.Sort(want)
	for _, path := range []string{"../../docs/schema/error.schema.json", "../../docs/schema/dump-error.schema.json"} {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var schema struct {
			Properties map[string]json.RawMessage `json:"properties"`
		}
		if err := json.Unmarshal(body, &schema); err != nil {
			t.Fatal(err)
		}
		props := schema.Properties
		if errorObject, ok := props["error"]; ok {
			var nested struct {
				Properties map[string]json.RawMessage `json:"properties"`
			}
			if err := json.Unmarshal(errorObject, &nested); err != nil {
				t.Fatal(err)
			}
			props = nested.Properties
		}
		var reason struct {
			Enum []string `json:"enum"`
		}
		if err := json.Unmarshal(props["reason"], &reason); err != nil {
			t.Fatalf("%s reason: %v", path, err)
		}
		got := append([]string(nil), reason.Enum...)
		slices.Sort(got)
		if !slices.Equal(got, want) {
			t.Errorf("%s reason enum = %v, want %v", path, got, want)
		}
	}
}
