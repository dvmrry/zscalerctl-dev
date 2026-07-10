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
		`{"message":"set the Authorization: Bearer fuzz-authorization-canary header","count":42}`,
		`{"message":"Authorization:","count":42}`,
		`{"message":"Authoriz\u0061tion: Bearer fuzz-escaped-auth-canary","count":42}`,
		`{"message":"Authorization: Bearer fuzz-multiline-canary\nsafe-status-line","count":42}`,
		`{"authorizationInfo":"public","sessionToken":"fuzz-suffix-canary"}`,
		`{"message":"Authorization: Bearer fuzz-number-canary","value":1234567890123456.789012345678901e+2}`,
		`{"message":"Authorization:","url":"https://user:fuzz-cross-token-canary@host.invalid","owner":"alice@example.com"}`,
		`{"message":"Authorization:","wrapper":{"\u0061uthorization":"fuzz-nested-canary"}}`,
		`["Authorization: Digest username=\"fuzz-user\", response=\"fuzz-response\"",42,true,null]`,
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

			gotRendered, _ := r.ScanRenderedString(input)
			if !json.Valid([]byte(gotRendered)) {
				t.Fatalf("Redactor.ScanRenderedString(%q, mode %s) = invalid JSON %q, want valid JSON", input, mode, gotRendered)
			}
			if gotTwice := r.String(got); gotTwice != got {
				t.Fatalf("Redactor.String(Redactor.String(%q), mode %s) = %q, want idempotent %q", input, mode, gotTwice, got)
			}
			if gotRenderedTwice, _ := r.ScanRenderedString(gotRendered); gotRenderedTwice != gotRendered {
				t.Fatalf("Redactor.ScanRenderedString(Redactor.ScanRenderedString(%q), mode %s) = %q, want idempotent %q", input, mode, gotRenderedTwice, gotRendered)
			}
		}
	})
}

func FuzzRedactorPreservesValidNDJSON(f *testing.F) {
	for _, seed := range []struct {
		first  string
		second string
	}{
		{first: "ordinary note", second: "branch office"},
		{first: "escaped quote \" and slash \\", second: "unicode 東京"},
		{first: "owner alice@example.com", second: "address 192.0.2.10"},
	} {
		f.Add(seed.first, seed.second)
	}

	const authorizationCanary = "fuzz-ndjson-authorization-canary"
	f.Fuzz(func(t *testing.T, first, second string) {
		if len(first)+len(second) > 8192 {
			return
		}
		line1, err := json.Marshal(map[string]string{
			"message": "Authorization: Bearer " + authorizationCanary,
			"note":    first,
		})
		if err != nil {
			t.Fatalf("json.Marshal(first record) error = %v", err)
		}
		line2, err := json.Marshal(map[string]string{
			"message": "Authorization:",
			"note":    second,
		})
		if err != nil {
			t.Fatalf("json.Marshal(second record) error = %v", err)
		}
		input := string(line1) + "\n" + string(line2) + "\n"

		for _, mode := range []redact.Mode{redact.ModeStandard, redact.ModeShare, redact.ModeParanoid} {
			got := redact.New(mode).String(input)
			if strings.Contains(got, authorizationCanary) {
				t.Fatalf("Redactor.String(NDJSON, mode %s) leaked authorization canary: %q", mode, got)
			}
			lines := strings.Split(strings.TrimSuffix(got, "\n"), "\n")
			if len(lines) != 2 {
				t.Fatalf("Redactor.String(NDJSON, mode %s) produced %d lines, want 2: %q", mode, len(lines), got)
			}
			for i, line := range lines {
				if !json.Valid([]byte(line)) {
					t.Fatalf("Redactor.String(NDJSON, mode %s) line %d = invalid JSON %q", mode, i+1, line)
				}
			}
			if gotTwice := redact.New(mode).String(got); gotTwice != got {
				t.Fatalf("Redactor.String(Redactor.String(NDJSON), mode %s) = %q, want idempotent %q", mode, gotTwice, got)
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
