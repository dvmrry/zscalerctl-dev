package zscaler

import (
	"bytes"
	"errors"
	"log/slog"
	"strings"
	"testing"

	sdklogger "github.com/zscaler/zscaler-sdk-go/v3/logger"
)

// debugSlogLogger builds a debug-level text slog logger writing to buf, matching
// how the CLI wires --log-level debug to stderr.
func debugSlogLogger(buf *bytes.Buffer) *slog.Logger {
	return slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

func TestNewSDKLoggerNilReturnsNop(t *testing.T) {
	t.Parallel()

	got := newSDKLogger(nil)
	if got == nil {
		t.Fatal("newSDKLogger(nil) = nil, want non-nil nop logger")
	}
	// The nop logger must never panic and must produce no output.
	got.Printf("[INFO] got Retry-After from header:%s\n", "5")
	if _, ok := got.(sdkLogAdapter); ok {
		t.Fatalf("newSDKLogger(nil) returned %T, want the SDK nop logger", got)
	}
}

func TestSDKLogAdapterForwardsRetryAndAuthEvents(t *testing.T) {
	t.Parallel()

	// These mirror the SDK's real retry/backoff and session/token-renewal
	// format strings; each must be surfaced at debug.
	cases := []struct {
		name   string
		format string
		args   []interface{}
		want   string
	}{
		{"retry_after_header", "[INFO] got Retry-After from header:%s\n", []interface{}{"5"}, "Retry-After"},
		{"rate_limiter", "[DEBUG] Rate limiter triggered. Sleeping for %v", []interface{}{"2s"}, "Sleeping for 2s"},
		{"session_refresh", "[INFO] Session is invalid or expired. Refreshing session...", nil, "Refreshing session"},
		{"token_renewed", "[INFO] OAuth2 token successfully renewed", nil, "successfully renewed"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			adapter := newSDKLogger(debugSlogLogger(&buf))
			adapter.Printf(tc.format, tc.args...)
			out := buf.String()
			if !strings.Contains(out, tc.want) {
				t.Errorf("Printf(%q) logged %q, want it to contain %q", tc.format, out, tc.want)
			}
			if !strings.Contains(out, "source=zscaler-sdk") {
				t.Errorf("Printf(%q) logged %q, want it tagged source=zscaler-sdk", tc.format, out)
			}
			if !strings.Contains(out, "level=DEBUG") {
				t.Errorf("Printf(%q) logged %q, want it emitted at DEBUG level", tc.format, out)
			}
		})
	}
}

func TestSDKLogAdapterRedactsMalformedRetryAfterHeader(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		value       string
		wantVisible bool
	}{
		{"short_assignment", "key=short-key-canary", false},
		{"bare_hexadecimal", "b8e1093dc27f65a4e9021b7c8df6a350", false},
		{"delta_seconds", "5", true},
		{"http_date", "Sun, 06 Nov 1994 08:49:37 GMT", true},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			adapter := newSDKLogger(debugSlogLogger(&buf))
			adapter.Printf("[WARN] Could not parse Retry-After header: %s", tc.value)
			out := buf.String()
			if tc.wantVisible {
				if !strings.Contains(out, "header: "+tc.value) {
					t.Errorf("SDK log output = %q, want valid Retry-After value %q", out, tc.value)
				}
				return
			}
			if strings.Contains(out, tc.value) {
				t.Errorf("SDK log output = %q, want header canary redacted", out)
			}
			if !strings.Contains(out, "<REDACTED:SECRET>") {
				t.Errorf("SDK log output = %q, want secret redaction marker", out)
			}
		})
	}
}

func TestSDKLogAdapterRedactsMalformedRetryAfterInRateLimitSummary(t *testing.T) {
	t.Parallel()

	// The SDK logs raw rate-limit headers before it parses Retry-After, so the
	// summary must not carry a malformed value either.
	const format = "[DEBUG] Rate limit headers: Limit=%s, Remaining=%s, Reset=%s, Retry-After=%s, Status=%d"
	for _, canary := range []string{"key=short-key-canary", "b8e1093dc27f65a4e9021b7c8df6a350"} {
		var buf bytes.Buffer
		adapter := newSDKLogger(debugSlogLogger(&buf))
		adapter.Printf(format, "100", "1", "1", canary, 429)
		out := buf.String()
		if strings.Contains(out, canary) {
			t.Errorf("SDK log output = %q, want Retry-After canary %q redacted", out, canary)
		}
		if !strings.Contains(out, "Limit=100, Remaining=1, Reset=1") || !strings.Contains(out, "Status=429") {
			t.Errorf("SDK log output = %q, want numeric rate-limit headers kept", out)
		}
	}

	var buf bytes.Buffer
	adapter := newSDKLogger(debugSlogLogger(&buf))
	adapter.Printf(format, "100", "1", "1", "5", 429)
	if out := buf.String(); !strings.Contains(out, "Retry-After=5") {
		t.Errorf("SDK log output = %q, want valid Retry-After value kept", out)
	}
}

func TestSDKLogAdapterRedactsTextArgumentsInForwardedFormats(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	adapter := newSDKLogger(debugSlogLogger(&buf))
	adapter.Printf("[DEBUG] retrying after error: %v (waiting %s)", errors.New("token=short-key-canary"), "2s")
	out := buf.String()
	if strings.Contains(out, "short-key-canary") {
		t.Errorf("SDK log output = %q, want error text redacted", out)
	}
	if !strings.Contains(out, "waiting 2s") {
		t.Errorf("SDK log output = %q, want duration kept", out)
	}
}

// TestSDKLogAdapterDropsRequestResponseDumps is the security guard: the SDK logs
// full request/response dumps (Authorization headers, bodies) through the same
// Printf interface, and the adapter must never forward them.
func TestSDKLogAdapterDropsRequestResponseDumps(t *testing.T) {
	t.Parallel()

	const secret = "Bearer super-secret-token"
	dumps := []string{
		`[DEBUG] Request "%s %s" details:
---[ ZSCALER SDK REQUEST | ID:%s ]-------------------------------
%s
---------------------------------------------------------`,
		`[DEBUG] Response "%s %s" details:
---[ ZSCALER SDK RESPONSE | ID:%s | Duration:%s ]--------------------------------
%s
-------------------------------------------------------`,
	}
	for _, format := range dumps {
		var buf bytes.Buffer
		adapter := newSDKLogger(debugSlogLogger(&buf))
		adapter.Printf(format, "GET", "https://x/api?token="+secret, "id", "Authorization: "+secret)
		if buf.Len() != 0 {
			t.Fatalf("Printf(dump) logged %q, want empty (dumps must never be forwarded)", buf.String())
		}
		if strings.Contains(buf.String(), secret) {
			t.Fatalf("Printf(dump) leaked secret %q", secret)
		}
	}
}

// TestSDKLogAdapterDropsAuthFailureBodies is the second security guard: the
// SDK's auth-failure notices ("Failed to renew OAuth2 token: %v" /
// "Failed to refresh session: %v") interpolate an error that embeds the raw
// auth-endpoint response body, which can carry token material. The adapter must
// never forward them.
func TestSDKLogAdapterDropsAuthFailureBodies(t *testing.T) {
	t.Parallel()

	const secret = "eyJhbGciOiJIUzI1NiJ9.super-secret-token-body"
	cases := []struct {
		name   string
		format string
		arg    string
	}{
		{
			name:   "renew_oauth2",
			format: "[ERROR] Failed to renew OAuth2 token: %v",
			arg:    "got http status: 401, response body: {\"access_token\":\"" + secret + "\"}",
		},
		{
			name:   "refresh_session",
			format: "[ERROR] Failed to refresh session: %v",
			arg:    "HTTP 401 Unauthorized: {\"token\":\"" + secret + "\"}",
		},
		{
			name:   "auth_error",
			format: "auth error: %v",
			arg:    secret,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			adapter := newSDKLogger(debugSlogLogger(&buf))
			adapter.Printf(tc.format, tc.arg)
			if buf.Len() != 0 {
				t.Fatalf("Printf(%q) logged %q, want empty (auth-failure bodies must never be forwarded)", tc.format, buf.String())
			}
			if strings.Contains(buf.String(), secret) {
				t.Fatalf("Printf(%q) leaked secret %q", tc.format, secret)
			}
		})
	}
}

// TestSDKLogAdapterStillForwardsSafeSessionEvents guards against over-correcting
// the fix: dropping the auth-failure notices must not silence the safe
// renewal/session signals operators rely on at debug.
func TestSDKLogAdapterStillForwardsSafeSessionEvents(t *testing.T) {
	t.Parallel()

	safe := []string{
		"[INFO] OAuth2 token successfully renewed",
		"[INFO] Session is invalid or expired. Refreshing session...",
		"[INFO] Another goroutine is refreshing the session. Waiting...",
	}
	for _, format := range safe {
		var buf bytes.Buffer
		adapter := newSDKLogger(debugSlogLogger(&buf))
		adapter.Printf(format)
		if buf.Len() == 0 {
			t.Errorf("Printf(%q) logged nothing, want it forwarded as a safe session event", format)
		}
	}
}

func TestSDKLogAdapterDropsUnknownMessages(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	adapter := newSDKLogger(debugSlogLogger(&buf))
	// A message that is neither a known retry/auth event nor a dump is dropped
	// (fail-closed allow-list).
	adapter.Printf("[DEBUG] Retrieved URL Filter and Cloud App Settings: %+v", map[string]string{"k": "v"})
	if buf.Len() != 0 {
		t.Fatalf("Printf(unknown) logged %q, want empty", buf.String())
	}
}

// TestSDKLogAdapterSilentBelowDebug confirms that at info/off levels no
// SDK-origin lines appear, because the adapter logs at DEBUG.
func TestSDKLogAdapterSilentBelowDebug(t *testing.T) {
	t.Parallel()

	levels := map[string]slog.Level{
		"info":  slog.LevelInfo,
		"warn":  slog.LevelWarn,
		"error": slog.LevelError,
	}
	for name, lvl := range levels {
		name, lvl := name, lvl
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: lvl}))
			adapter := newSDKLogger(logger)
			adapter.Printf("[INFO] got Retry-After from header:%s\n", "5")
			if buf.Len() != 0 {
				t.Errorf("at level %s Printf logged %q, want empty", name, buf.String())
			}
		})
	}
}

// Compile-time assertion that the adapter satisfies the SDK Logger interface.
var _ sdklogger.Logger = sdkLogAdapter{}
