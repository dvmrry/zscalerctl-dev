package redact_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/redact"
)

func FuzzRedactorPreservesValidJSON(f *testing.F) {
	for _, seed := range []string{
		`{"apiKey":"secret-value"}`,
		`{"token":1234567890}`,
		`{"clientSecret":true}`,
		`{"password":null}`,
		`{"authorization":"Bearer abcdefghijklmnopqrstuvwxyz"}`,
		`{"url":"https://user:password@example.invalid/private"}`,
		`{"nested":{"secretKey":"nested-secret"},"items":[{"apiToken":"item-token"}]}`,
		`{"ordinary":{"tokenEndpoint":"https://example.invalid/oauth/token"}}`,
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input string) {
		if len(input) > 8192 || !json.Valid([]byte(input)) {
			return
		}

		for _, mode := range []redact.Mode{redact.ModeStandard, redact.ModeShare, redact.ModeParanoid} {
			r := redact.New(mode)

			got := r.String(input)
			if !json.Valid([]byte(got)) {
				t.Fatalf("Redactor.String(%q, mode %s) = invalid JSON %q, want valid JSON", input, mode, got)
			}

			gotBytes := r.Bytes([]byte(input))
			if !json.Valid(gotBytes) {
				t.Fatalf("Redactor.Bytes(%q, mode %s) = invalid JSON %q, want valid JSON", input, mode, string(gotBytes))
			}
		}
	})
}

func FuzzScanRenderedStringRedactsBareHighEntropyCanary(f *testing.F) {
	for _, seed := range []struct {
		prefix string
		suffix string
	}{
		{prefix: "temporary admin note", suffix: "during rollout"},
		{prefix: "unicode snowman \u2603", suffix: "combining e\u0301"},
		{prefix: "url https://example.invalid/path", suffix: "ticket CHG-123456"},
	} {
		f.Add(seed.prefix, seed.suffix)
	}

	const canary = "A7b9C2d4E6f8G1h3J5k7L9m2N4p6Q8r0S2t4U6v"
	f.Fuzz(func(t *testing.T, prefix, suffix string) {
		if len(prefix)+len(suffix) > 8192 {
			return
		}
		if strings.Contains(prefix, canary) || strings.Contains(suffix, canary) {
			return
		}

		input := prefix + " " + canary + " " + suffix
		got, report := redact.New(redact.ModeStandard).ScanRenderedString(input)
		if strings.Contains(got, canary) {
			t.Fatalf("Redactor.ScanRenderedString(%q) = %q, want no canary", input, got)
		}
		if !strings.Contains(got, "<REDACTED:SECRET>") {
			t.Fatalf("Redactor.ScanRenderedString(%q) = %q, want secret marker", input, got)
		}
		if report.Counts["high_entropy_rendered_token"] == 0 {
			t.Fatalf("Redactor.ScanRenderedString(%q) report = %#v, want high entropy finding", input, report)
		}
	})
}
