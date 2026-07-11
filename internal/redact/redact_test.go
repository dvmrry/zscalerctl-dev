package redact_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/redact"
)

// fakePrivateKeyBlock assembles a PEM-shaped redaction canary without placing
// a complete private-key detector signature in the repository source. Tests
// still exercise the exact runtime shape, while Gitleaks needs no exception
// that could also suppress real key material.
func fakePrivateKeyBlock(body string) string {
	const begin = "-----BEGIN PRIVATE" + " KEY-----"
	const end = "-----END PRIVATE" + " KEY-----"
	return begin + "\n" + body + "\n" + end
}

func TestRedactorRemovesCredentialPatterns(t *testing.T) {
	t.Parallel()

	input := strings.Join([]string{
		`Authorization: Bearer abcdefghijklmnopqrstuvwxyz`,
		`Authorization: Basic dXNlcjpzZWNyZXQ=`,
		`client_secret: "shh-this-is-secret"`,
		`token=tok_1234567890`,
		`psk=network-pre-shared-key`,
		`VPN PSK hunter2hunter2`,
		`provisioning_key=1|api.private.zscaler.com|abcdefghiJKLMNOP1234567890+/==`,
		`ZPA provision key 3|api.private.zscaler.com|xyzxyzxyzxyzxyzxyzxyzxyzxyzxyz`,
		`eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.signaturexyz`,
	}, "\n")

	got := redact.New(redact.ModeStandard).String(input)
	for _, forbidden := range []string{
		"abcdefghijklmnopqrstuvwxyz",
		"dXNlcjpzZWNyZXQ=",
		"shh-this-is-secret",
		"tok_1234567890",
		"network-pre-shared-key",
		"hunter2hunter2",
		"abcdefghiJKLMNOP1234567890",
		"xyzxyzxyzxyzxyzxyzxyzxyzxyzxyz",
		"eyJhbGci",
	} {
		if strings.Contains(got, forbidden) {
			t.Errorf("Redactor.String(%q) = %q, want no %q", input, got, forbidden)
		}
	}
	if !strings.Contains(got, "<REDACTED:SECRET>") {
		t.Errorf("Redactor.String(%q) = %q, want typed secret marker", input, got)
	}
}

// Resolve-failure errors mention the credential being resolved AND carry a
// diagnostic cause. The credential phrase must trail the cause (parenthesized)
// so the redactor does not read "<secret>: <cause>" as a key:value assignment
// and eat the diagnostic. Guards the app.go resourceReader wrappers.
func TestRedactorPreservesResolveErrorDiagnostics(t *testing.T) {
	t.Parallel()

	cases := []struct{ in, keep string }{
		{"missing credentials: unsafe file permissions: broad Windows owner S-1-1-0 (while resolving the client secret)", "unsafe file permissions"},
		{"missing credentials: keyring is not available in this build (while resolving the ZIA legacy password)", "keyring is not available"},
		{"missing credentials: env ref is not set: ZS_CLIENT (while resolving the ZIA legacy API key)", "env ref is not set"},
	}
	r := redact.New(redact.ModeStandard)
	for _, tc := range cases {
		got := r.String(tc.in)
		if strings.Contains(got, "<REDACTED") {
			t.Errorf("Redactor.String(%q) = %q, want no marker (diagnostic is not a secret)", tc.in, got)
		}
		if !strings.Contains(got, tc.keep) {
			t.Errorf("Redactor.String(%q) = %q, want diagnostic %q preserved", tc.in, got, tc.keep)
		}
	}
}

func TestRedactorRemovesNonBearerAuthSchemes(t *testing.T) {
	t.Parallel()

	cases := []struct{ name, in, leak string }{
		{"token", "Authorization: Token sk-supersecret-credential-value", "supersecret"},
		{"apikey", "Authorization: ApiKey abc123secretkeyvalue", "abc123secretkeyvalue"},
		{"ntlm", "Authorization: NTLM TlRMTVNTUAABBBBBccccc", "TlRMTVNTUAAB"},
		{"digest", `Authorization: Digest username="x", response=deadbeefdeadbeef`, "deadbeefdeadbeef"},
		{"aws", "Authorization: AWS4-HMAC-SHA256 Credential=AKIAIOSFODNN7EXAMPLE", "AKIAIOSFODNN7EXAMPLE"},
		{"bearer-still-works", "Authorization: Bearer abcdefghijklmnopqrstuvwxyz", "abcdefghijklmnopqrstuvwxyz"},
	}
	r := redact.New(redact.ModeStandard)
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := r.String(tc.in)
			if strings.Contains(got, tc.leak) {
				t.Errorf("Redactor.String(%q) = %q, want no %q", tc.in, got, tc.leak)
			}
		})
	}
}

func TestRedactorPreservesJSONSyntaxForAuthorizationHeaderText(t *testing.T) {
	t.Parallel()

	const authorizationCanary = "authorization-value-canary"
	input := `{"message":"set the Authorization: Bearer ` + authorizationCanary + ` header","count":42}`
	const want = `{"message":"set the Authorization: <REDACTED:SECRET>","count":42}`

	for _, mode := range []redact.Mode{redact.ModeStandard, redact.ModeShare, redact.ModeParanoid} {
		mode := mode
		t.Run(string(mode), func(t *testing.T) {
			t.Parallel()

			r := redact.New(mode)
			got, report := r.ScanString(input)
			if !json.Valid([]byte(got)) {
				t.Errorf("Redactor.ScanString(%q, %s) = invalid JSON %q, want valid JSON", input, mode, got)
			}
			if strings.Contains(got, authorizationCanary) {
				t.Errorf("Redactor.ScanString(%q, %s) = %q, want no %q", input, mode, got, authorizationCanary)
			}
			if !strings.Contains(got, "<REDACTED:SECRET>") {
				t.Errorf("Redactor.ScanString(%q, %s) = %q, want secret marker", input, mode, got)
			}
			if strings.Contains(got, `\u003cREDACTED:SECRET\u003e`) {
				t.Errorf("Redactor.ScanString(%q, %s) = %q, want unescaped secret marker", input, mode, got)
			}
			if got != want {
				t.Errorf("Redactor.ScanString(%q, %s) = %q, want %q", input, mode, got, want)
			}
			if gotCount := report.Counts["authorization_header"]; gotCount != 1 {
				t.Errorf("Redactor.ScanString(%q, %s) authorization_header count = %d, want 1", input, mode, gotCount)
			}

			if gotString := r.String(input); gotString != got {
				t.Errorf("Redactor.String(%q, %s) = %q, want %q", input, mode, gotString, got)
			}
			gotBytes := r.Bytes([]byte(input))
			if !json.Valid(gotBytes) {
				t.Errorf("Redactor.Bytes(%q, %s) = invalid JSON %q, want valid JSON", input, mode, string(gotBytes))
			}
			if string(gotBytes) != got {
				t.Errorf("Redactor.Bytes(%q, %s) = %q, want %q", input, mode, string(gotBytes), got)
			}
			if gotTwice := r.String(got); gotTwice != got {
				t.Errorf("Redactor.String(Redactor.String(%q), %s) = %q, want idempotent %q", input, mode, gotTwice, got)
			}
		})
	}
}

func TestRedactorPreservesJSONAuthorizationAssignmentAfterIncompleteHeader(t *testing.T) {
	t.Parallel()

	const assignmentCanary = "authorization-assignment-canary"
	input := `{"message":"Authorization:","authorization":"` + assignmentCanary + `","count":42}`
	const want = `{"message":"Authorization:","authorization":"<REDACTED:SECRET>","count":42}`

	for _, mode := range []redact.Mode{redact.ModeStandard, redact.ModeShare, redact.ModeParanoid} {
		mode := mode
		t.Run(string(mode), func(t *testing.T) {
			t.Parallel()

			got, report := redact.New(mode).ScanString(input)
			if !json.Valid([]byte(got)) {
				t.Errorf("Redactor.ScanString(%q, %s) = invalid JSON %q, want valid JSON", input, mode, got)
			}
			if strings.Contains(got, assignmentCanary) {
				t.Errorf("Redactor.ScanString(%q, %s) = %q, want no %q", input, mode, got, assignmentCanary)
			}
			if got != want {
				t.Errorf("Redactor.ScanString(%q, %s) = %q, want %q", input, mode, got, want)
			}
			if gotCount := report.Counts["secret_assignment"]; gotCount != 1 {
				t.Errorf("Redactor.ScanString(%q, %s) secret_assignment count = %d, want 1", input, mode, gotCount)
			}
		})
	}
}

func TestRedactorJSONRedactionStaysWithinStringTokenBoundaries(t *testing.T) {
	t.Parallel()

	const credentialCanary = "cross-token-password-canary"
	const owner = "alice@example.com"
	input := `{"message":"Authorization:","url":"https://user:` + credentialCanary + `@host.invalid","owner":"` + owner + `","count":42}`

	got := redact.New(redact.ModeStandard).String(input)
	if !json.Valid([]byte(got)) {
		t.Fatalf("Redactor.String(%q) = invalid JSON %q, want valid JSON", input, got)
	}
	if strings.Contains(got, credentialCanary) {
		t.Errorf("Redactor.String(%q) = %q, want no %q", input, got, credentialCanary)
	}

	var decoded struct {
		Message string `json:"message"`
		URL     string `json:"url"`
		Owner   string `json:"owner"`
		Count   int    `json:"count"`
	}
	if err := json.Unmarshal([]byte(got), &decoded); err != nil {
		t.Fatalf("json.Unmarshal(Redactor.String(%q)) error = %v", input, err)
	}
	if decoded.Message != "Authorization:" {
		t.Errorf("redacted message = %q, want incomplete header preserved", decoded.Message)
	}
	if decoded.URL != "https://<REDACTED:SECRET>@host.invalid" {
		t.Errorf("redacted URL = %q, want credential-only redaction", decoded.URL)
	}
	if decoded.Owner != owner {
		t.Errorf("redacted owner = %q, want unrelated token preserved as %q", decoded.Owner, owner)
	}
	if decoded.Count != 42 {
		t.Errorf("redacted count = %d, want 42", decoded.Count)
	}
}

func TestRedactorPreservesNDJSONAuthorizationRecords(t *testing.T) {
	t.Parallel()

	const authorizationCanary = "ndjson-auth-canary"
	const assignmentCanary = "ndjson-assignment-canary"
	input := `{"message":"Authoriz\u0061tion: Bearer ` + authorizationCanary + `","count":1}` + "\r\n" +
		`{"message":"Authoriz\u0061tion:","clientSecret":"` + assignmentCanary + `","count":2}` + "\r\n"

	got := redact.New(redact.ModeStandard).String(input)
	for _, forbidden := range []string{authorizationCanary, assignmentCanary} {
		if strings.Contains(got, forbidden) {
			t.Errorf("Redactor.String(NDJSON) = %q, want no %q", got, forbidden)
		}
	}
	if strings.Count(got, "\r\n") != 2 || strings.Contains(strings.ReplaceAll(got, "\r\n", ""), "\n") {
		t.Errorf("Redactor.String(NDJSON) line endings changed: %q", got)
	}

	lines := strings.Split(strings.TrimSuffix(got, "\r\n"), "\r\n")
	if len(lines) != 2 {
		t.Fatalf("Redactor.String(NDJSON) line count = %d, want 2: %q", len(lines), got)
	}
	for i, line := range lines {
		var decoded map[string]any
		if err := json.Unmarshal([]byte(line), &decoded); err != nil {
			t.Errorf("Redactor.String(NDJSON) line %d = invalid JSON %q: %v", i+1, line, err)
			continue
		}
		if decoded["count"] != float64(i+1) {
			t.Errorf("Redactor.String(NDJSON) line %d count = %#v, want %d", i+1, decoded["count"], i+1)
		}
	}
	if gotTwice := redact.New(redact.ModeStandard).String(got); gotTwice != got {
		t.Errorf("Redactor.String(Redactor.String(NDJSON)) = %q, want idempotent %q", gotTwice, got)
	}
}

func TestRedactorJSONClassifiesDecodedSensitiveKeys(t *testing.T) {
	t.Parallel()

	const nestedCanary = "nested-authorization-canary"
	const scalarCanary = "escaped-client-secret-canary"
	input := `{"message":"Authorization:","wrapper":{"\u0061uthorization":"` + nestedCanary + `"},"client\u0053ecret":"` + scalarCanary + `"}`

	got, report := redact.New(redact.ModeStandard).ScanString(input)
	if !json.Valid([]byte(got)) {
		t.Fatalf("Redactor.ScanString(%q) = invalid JSON %q, want valid JSON", input, got)
	}
	for _, forbidden := range []string{nestedCanary, scalarCanary} {
		if strings.Contains(got, forbidden) {
			t.Errorf("Redactor.ScanString(%q) = %q, want no %q", input, got, forbidden)
		}
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(got), &decoded); err != nil {
		t.Fatalf("json.Unmarshal(Redactor.ScanString(%q)) error = %v", input, err)
	}
	wrapper, ok := decoded["wrapper"].(map[string]any)
	if !ok {
		t.Fatalf("redacted wrapper = %#v, want object", decoded["wrapper"])
	}
	if wrapper["authorization"] != "<REDACTED:SECRET>" {
		t.Errorf("redacted nested authorization = %#v, want secret marker", wrapper["authorization"])
	}
	if decoded["clientSecret"] != "<REDACTED:SECRET>" {
		t.Errorf("redacted clientSecret = %#v, want secret marker", decoded["clientSecret"])
	}
	if gotCount := report.Counts["secret_assignment"]; gotCount != 2 {
		t.Errorf("Redactor.ScanString(%q) secret_assignment count = %d, want 2", input, gotCount)
	}
}

func TestRedactorJSONPreservesSensitiveSuffixAssignmentCoverage(t *testing.T) {
	t.Parallel()

	input := `{"authorizationInfo":"public","my_secret":"suffix-canary-one","sessionToken":"suffix-canary-two","appSecret":"suffix-canary-three","tenant_private_key":"suffix-canary-four","customProvisioningKey":"suffix-canary-five"}`
	want := map[string]string{
		"my_secret":             "<REDACTED:SECRET>",
		"sessionToken":          "<REDACTED:SECRET>",
		"appSecret":             "<REDACTED:SECRET>",
		"tenant_private_key":    "<REDACTED:PRIVATE_KEY>",
		"customProvisioningKey": "<REDACTED:PROVISIONING_KEY>",
	}

	for _, mode := range []redact.Mode{redact.ModeStandard, redact.ModeShare, redact.ModeParanoid} {
		mode := mode
		t.Run(string(mode), func(t *testing.T) {
			t.Parallel()

			got := redact.New(mode).String(input)
			if !json.Valid([]byte(got)) {
				t.Fatalf("Redactor.String(%q, %s) = invalid JSON %q", input, mode, got)
			}
			for _, canary := range []string{"suffix-canary-one", "suffix-canary-two", "suffix-canary-three", "suffix-canary-four", "suffix-canary-five"} {
				if strings.Contains(got, canary) {
					t.Errorf("Redactor.String(%q, %s) = %q, want no %q", input, mode, got, canary)
				}
			}

			var decoded map[string]string
			if err := json.Unmarshal([]byte(got), &decoded); err != nil {
				t.Fatalf("json.Unmarshal(Redactor.String(%q, %s)) error = %v", input, mode, err)
			}
			for key, marker := range want {
				if decoded[key] != marker {
					t.Errorf("redacted %s = %q, want %q", key, decoded[key], marker)
				}
			}
		})
	}
}

func TestRedactorDecodesEscapedJSONAuthorizationKeyword(t *testing.T) {
	t.Parallel()

	const canary = "escaped-auth-canary-value"
	input := `{"message":"Authoriz\u0061tion: Bearer ` + canary + `","count":1}`

	for _, mode := range []redact.Mode{redact.ModeStandard, redact.ModeShare, redact.ModeParanoid} {
		mode := mode
		t.Run(string(mode), func(t *testing.T) {
			t.Parallel()

			got := redact.New(mode).String(input)
			if !json.Valid([]byte(got)) {
				t.Fatalf("Redactor.String(%q, %s) = invalid JSON %q", input, mode, got)
			}
			if strings.Contains(got, canary) {
				t.Errorf("Redactor.String(%q, %s) = %q, want no %q", input, mode, got, canary)
			}
			var decoded struct {
				Message string `json:"message"`
			}
			if err := json.Unmarshal([]byte(got), &decoded); err != nil {
				t.Fatalf("json.Unmarshal(Redactor.String(%q, %s)) error = %v", input, mode, err)
			}
			if decoded.Message != "Authorization: <REDACTED:SECRET>" {
				t.Errorf("redacted message = %q, want decoded Authorization marker", decoded.Message)
			}
		})
	}
}

func TestRedactorJSONAuthorizationStopsAtDecodedNewline(t *testing.T) {
	t.Parallel()

	const canary = "multiline-authorization-canary"
	input := `{"message":"Authorization: Bearer ` + canary + `\nsafe-status-line","count":2}`

	got := redact.New(redact.ModeStandard).String(input)
	if !json.Valid([]byte(got)) {
		t.Fatalf("Redactor.String(%q) = invalid JSON %q", input, got)
	}
	if strings.Contains(got, canary) {
		t.Errorf("Redactor.String(%q) = %q, want no %q", input, got, canary)
	}
	var decoded struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal([]byte(got), &decoded); err != nil {
		t.Fatalf("json.Unmarshal(Redactor.String(%q)) error = %v", input, err)
	}
	const want = "Authorization: <REDACTED:SECRET>\nsafe-status-line"
	if decoded.Message != want {
		t.Errorf("redacted message = %q, want %q", decoded.Message, want)
	}
}

func TestScanRenderedStringPreservesJSONWithLongNumber(t *testing.T) {
	t.Parallel()

	const number = "1234567890123456.789012345678901e+2"
	const token = "A7b9C2d4E6f8G1h3J5k7L9m2N4p6Q8r0S2t4U6v"
	input := `{"message":"Authorization: Bearer numeric-authorization-canary","credential":"` + token + `","value":` + number + `}`

	got, report := redact.New(redact.ModeStandard).ScanRenderedString(input)
	if !json.Valid([]byte(got)) {
		t.Fatalf("Redactor.ScanRenderedString(%q) = invalid JSON %q", input, got)
	}
	if !strings.Contains(got, number) {
		t.Errorf("Redactor.ScanRenderedString(%q) = %q, want numeric JSON value preserved", input, got)
	}
	if strings.Contains(got, token) {
		t.Errorf("Redactor.ScanRenderedString(%q) = %q, want string token redacted", input, got)
	}
	if gotCount := report.Counts["high_entropy_rendered_token"]; gotCount != 1 {
		t.Errorf("Redactor.ScanRenderedString(%q) report count = %d, want 1", input, gotCount)
	}
}

func TestScanRenderedStringRedactsEscapedHighEntropyJSONToken(t *testing.T) {
	t.Parallel()

	const token = "A7b9C2d4E6f8G1h3J5k7L9m2N4p6Q8r0S2t4U6v"
	input := `{"credential":"A7b9C2d\u0034E6f8G1h\u0033J5k7L9m\u0032N4p6Q8r\u0030S2t4U6v"}`

	r := redact.New(redact.ModeStandard)
	got, report := r.ScanRenderedString(input)
	if !json.Valid([]byte(got)) {
		t.Fatalf("Redactor.ScanRenderedString(%q) = invalid JSON %q", input, got)
	}
	var decoded struct {
		Credential string `json:"credential"`
	}
	if err := json.Unmarshal([]byte(got), &decoded); err != nil {
		t.Fatalf("json.Unmarshal(Redactor.ScanRenderedString(%q)) error = %v", input, err)
	}
	if decoded.Credential == token {
		t.Errorf("Redactor.ScanRenderedString(%q) leaked decoded token %q", input, token)
	}
	if decoded.Credential != "<REDACTED:SECRET>" {
		t.Errorf("redacted credential = %q, want secret marker", decoded.Credential)
	}
	if gotCount := report.Counts["high_entropy_rendered_token"]; gotCount != 1 {
		t.Errorf("Redactor.ScanRenderedString(%q) report count = %d, want 1", input, gotCount)
	}
	if gotTwice, _ := r.ScanRenderedString(got); gotTwice != got {
		t.Errorf("Redactor.ScanRenderedString(Redactor.ScanRenderedString(%q)) = %q, want idempotent %q", input, gotTwice, got)
	}
}

func TestRedactorPreservesJSONSyntaxForEscapedDigestAuthorization(t *testing.T) {
	t.Parallel()

	const digestCanary = "digest-response-canary"
	const backslashCanary = "backslash-canary"
	input := `{"message":"Authorization: Digest username=\"redaction-user\", response=\"` + digestCanary + `\", opaque=\"path\\` + backslashCanary + `\"","count":7}`

	for _, mode := range []redact.Mode{redact.ModeStandard, redact.ModeShare, redact.ModeParanoid} {
		mode := mode
		t.Run(string(mode), func(t *testing.T) {
			t.Parallel()

			r := redact.New(mode)
			got := r.String(input)
			if !json.Valid([]byte(got)) {
				t.Errorf("Redactor.String(%q, %s) = invalid JSON %q, want valid JSON", input, mode, got)
			}
			for _, forbidden := range []string{"redaction-user", digestCanary, backslashCanary} {
				if strings.Contains(got, forbidden) {
					t.Errorf("Redactor.String(%q, %s) = %q, want no %q", input, mode, got, forbidden)
				}
			}
			gotBytes := r.Bytes([]byte(input))
			if !json.Valid(gotBytes) {
				t.Errorf("Redactor.Bytes(%q, %s) = invalid JSON %q, want valid JSON", input, mode, string(gotBytes))
			}
			for _, forbidden := range []string{"redaction-user", digestCanary, backslashCanary} {
				if strings.Contains(string(gotBytes), forbidden) {
					t.Errorf("Redactor.Bytes(%q, %s) = %q, want no %q", input, mode, string(gotBytes), forbidden)
				}
			}
		})
	}
}

func TestRedactorPlainTextDigestRedactsEntireHeader(t *testing.T) {
	t.Parallel()

	input := `Authorization: Digest username="redaction-user", realm="example.invalid", nonce="digest-nonce-canary", response="digest-response-canary"`
	const want = "Authorization: <REDACTED:SECRET>"
	if got := redact.New(redact.ModeStandard).String(input); got != want {
		t.Errorf("Redactor.String(%q) = %q, want %q", input, got, want)
	}
}

func TestRedactorRemovesCredentialURLWithAtInPassword(t *testing.T) {
	t.Parallel()

	cases := []struct{ name, in, leak string }{
		{"at-in-password", "see https://admin:P@ssw0rd@db.internal.example/path", "ssw0rd"},
		{"double-at-no-path", "url https://user:a@b@host.example", "a@b"},
		{"plain", "https://user:plainpass@host.example/x", "plainpass"},
	}
	r := redact.New(redact.ModeStandard)
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := r.String(tc.in)
			if strings.Contains(got, tc.leak) {
				t.Errorf("Redactor.String(%q) = %q, want no %q", tc.in, got, tc.leak)
			}
		})
	}
}

func TestRedactorRemovesZscalerCredentialFields(t *testing.T) {
	t.Parallel()

	input := strings.Join([]string{
		`apiKey=zia-cloud-key`,
		`secretKey=client-connector-secret`,
		`key_secret=zdx-key-secret`,
		`apiToken=sandbox-token-value`,
		`bearerToken=webhook-bearer-token`,
		`hecToken=splunk-hec-token-value`,
		`refreshToken=refresh-token-value`,
		`accessToken=access-token-value`,
		`jwtToken=jwt-token-value`,
		`preSharedKey=vpn-pre-shared-key`,
		`sharedSecret=webhook-shared-secret`,
		`password=pra-password`,
		`vncPassword=pra-vnc-password`,
		`sshPrivateKey=ssh-private-key-material`,
		`sshPassphrase=ssh-private-key-passphrase`,
		`passphrase=pra-passphrase`,
		`temporary password temporary-password-value`,
		`sessionId=session-id-value`,
		`cookie=gateway-cookie-value`,
		`otp=temporary-otp-value`,
		`https://user:password@example.com/private`,
	}, "\n")

	got := redact.New(redact.ModeStandard).String(input)
	for _, forbidden := range []string{
		"zia-cloud-key",
		"client-connector-secret",
		"zdx-key-secret",
		"sandbox-token-value",
		"webhook-bearer-token",
		"splunk-hec-token-value",
		"refresh-token-value",
		"access-token-value",
		"jwt-token-value",
		"vpn-pre-shared-key",
		"webhook-shared-secret",
		"pra-password",
		"pra-vnc-password",
		"ssh-private-key-material",
		"ssh-private-key-passphrase",
		"pra-passphrase",
		"temporary-password-value",
		"session-id-value",
		"gateway-cookie-value",
		"temporary-otp-value",
		"user:password",
	} {
		if strings.Contains(got, forbidden) {
			t.Errorf("Redactor.String(%q) = %q, want no %q", input, got, forbidden)
		}
	}
}

func TestRedactorPreservesJSONSyntaxForSecretAssignments(t *testing.T) {
	t.Parallel()

	privateKey := strings.ReplaceAll(fakePrivateKeyBlock("key-material"), "\n", `\\n`)
	certBlob := strings.ReplaceAll(fakePrivateKeyBlock("cert-blob-private-key-material"), "\n", `\\n`)
	input := `{
		"apiKey": "zia-cloud-key",
		"secretKey": "client-connector-secret",
		"sandboxApiToken": "sandbox-token-value",
		"bearerToken": "bearer-token-value",
		"provisioningKey": "1|api.private.example.net|abcdefghiJKLMNOP1234567890abcdefghijklmnopqrstuvwxyz",
		"privateKey": "` + privateKey + `",
		"certBlob": "` + certBlob + `",
		"zrsaencryptedprivatekey": "encrypted-private-key-material",
		"zrsaencryptedsessionkey": "encrypted-session-key-material",
		"description": "temporary shared secret abcdefghijklmnop in free text"
	}`

	got := redact.New(redact.ModeStandard).String(input)
	if !json.Valid([]byte(got)) {
		t.Fatalf("Redactor.String(%q) = invalid JSON %q, want valid JSON", input, got)
	}
	for _, forbidden := range []string{
		"zia-cloud-key",
		"client-connector-secret",
		"sandbox-token-value",
		"bearer-token-value",
		"abcdefghiJKLMNOP1234567890",
		"key-material",
		"cert-blob-private-key-material",
		"encrypted-private-key-material",
		"encrypted-session-key-material",
		"abcdefghijklmnop",
	} {
		if strings.Contains(got, forbidden) {
			t.Errorf("Redactor.String(%q) = %q, want no %q", input, got, forbidden)
		}
	}
	for _, marker := range []string{"<REDACTED:SECRET>", "<REDACTED:PRIVATE_KEY>", "<REDACTED:PROVISIONING_KEY>"} {
		if !strings.Contains(got, marker) {
			t.Errorf("Redactor.String(%q) = %q, want marker %q", input, got, marker)
		}
	}
}

func TestRedactorPreservesJSONSyntaxForEscapedSecretAssignments(t *testing.T) {
	t.Parallel()

	input := `{
		"clientSecret": "prefix-\"quoted\"-secret\\with\\slashes",
		"apiToken": "token-with-escaped-newline\\nmaterial",
		"password": "pass-with-unicode-escape-\\u003c"
	}`

	got := redact.New(redact.ModeStandard).String(input)
	if !json.Valid([]byte(got)) {
		t.Fatalf("Redactor.String(%q) = invalid JSON %q, want valid JSON", input, got)
	}
	for _, forbidden := range []string{"quoted", "slashes", "escaped-newline", "unicode-escape"} {
		if strings.Contains(got, forbidden) {
			t.Errorf("Redactor.String(%q) = %q, want no %q", input, got, forbidden)
		}
	}
}

func TestRedactorDoesNotBlankContextSensitiveGenericValues(t *testing.T) {
	t.Parallel()

	input := `{"value":"ordinary-config-value","keyValue":"public-key-material","certContent":"public-certificate-material"}`
	got := redact.New(redact.ModeStandard).String(input)
	if !json.Valid([]byte(got)) {
		t.Fatalf("Redactor.String(%q) = invalid JSON %q, want valid JSON", input, got)
	}
	for _, want := range []string{"ordinary-config-value", "public-key-material", "public-certificate-material"} {
		if !strings.Contains(got, want) {
			t.Errorf("Redactor.String(%q) = %q, want to preserve context-sensitive value %q", input, got, want)
		}
	}
}

func TestRedactorDoesNotBlankOperationalSecretWords(t *testing.T) {
	t.Parallel()

	input := "token endpoint is documented; password rotation policy is quarterly"
	got := redact.New(redact.ModeStandard).String(input)
	if got != input {
		t.Errorf("Redactor.String(%q) = %q, want unchanged operational text", input, got)
	}
}

func TestScanRenderedStringRemovesBareHighEntropyTokens(t *testing.T) {
	t.Parallel()

	for _, token := range []string{
		"A7b9C2d4E6f8G1h3J5k7L9m2N4p6Q8r0S2t4U6v",
		"eyJhbGciOiJIUzI1NiJ9_payload_signature_canary",
	} {
		token := token
		t.Run(token, func(t *testing.T) {
			t.Parallel()

			input := "temporary admin note " + token + " should not survive"
			got, report := redact.New(redact.ModeStandard).ScanRenderedString(input)
			if strings.Contains(got, token) {
				t.Errorf("Redactor.ScanRenderedString(%q) = %q, want no bare token", input, got)
			}
			if !strings.Contains(got, "<REDACTED:SECRET>") {
				t.Errorf("Redactor.ScanRenderedString(%q) = %q, want typed secret marker", input, got)
			}
			if report.Counts["high_entropy_rendered_token"] != 1 {
				t.Errorf("Redactor.ScanRenderedString(%q) report count = %d, want 1", input, report.Counts["high_entropy_rendered_token"])
			}
		})
	}
}

func TestScanRenderedStringPreservesStructuredPublicIdentifiers(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name     string
		input    string
		allModes bool
	}{
		{
			name:     "canonical UUID",
			input:    "550e8400-e29b-41d4-a716-446655440000",
			allModes: true,
		},
		{
			name:  "compact UUID",
			input: "550e8400e29b41d4a716446655440000",
		},
		{
			name:  "SHA1 fingerprint",
			input: "0123456789abcdef0123456789abcdef01234567",
		},
		{
			name:  "SHA256 fingerprint",
			input: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		},
	} {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, report := redact.New(redact.ModeStandard).ScanRenderedString(tt.input)
			if got != tt.input {
				t.Errorf("Redactor.ScanRenderedString(%q, standard) = %q, want unchanged structured identifier", tt.input, got)
			}
			if !report.Empty() {
				t.Errorf("Redactor.ScanRenderedString(%q, standard) report = %#v, want empty", tt.input, report)
			}

			for _, mode := range []redact.Mode{redact.ModeShare, redact.ModeParanoid} {
				got, report := redact.New(mode).ScanRenderedString(tt.input)
				if tt.allModes {
					if got != tt.input {
						t.Errorf("Redactor.ScanRenderedString(%q, %s) = %q, want unchanged structured identifier", tt.input, mode, got)
					}
					if !report.Empty() {
						t.Errorf("Redactor.ScanRenderedString(%q, %s) report = %#v, want empty", tt.input, mode, report)
					}
					continue
				}
				if strings.Contains(got, tt.input) {
					t.Errorf("Redactor.ScanRenderedString(%q, %s) = %q, want fingerprint-shaped value redacted outside standard", tt.input, mode, got)
				}
				if !strings.Contains(got, "<REDACTED:SECRET>") {
					t.Errorf("Redactor.ScanRenderedString(%q, %s) = %q, want secret marker", tt.input, mode, got)
				}
				if report.Counts["high_entropy_rendered_token"] != 1 {
					t.Errorf("Redactor.ScanRenderedString(%q, %s) report count = %d, want 1", tt.input, mode, report.Counts["high_entropy_rendered_token"])
				}
			}
		})
	}
}

func TestScanFreeTextRedactsBareHexFingerprintWithoutContext(t *testing.T) {
	t.Parallel()

	const token = "0123456789abcdef0123456789abcdef01234567"
	input := "temporary admin note " + token + " should not survive"
	got, report := redact.New(redact.ModeStandard).ScanFreeText(input)
	if strings.Contains(got, token) {
		t.Errorf("Redactor.ScanFreeText(%q) = %q, want no bare hex token", input, got)
	}
	if !strings.Contains(got, "<REDACTED:SECRET>") {
		t.Errorf("Redactor.ScanFreeText(%q) = %q, want typed secret marker", input, got)
	}
	if report.Counts["high_entropy_rendered_token"] != 1 {
		t.Errorf("Redactor.ScanFreeText(%q) report count = %d, want 1", input, report.Counts["high_entropy_rendered_token"])
	}
}

func TestScanStringDoesNotApplyRenderedStringEntropyHeuristic(t *testing.T) {
	t.Parallel()

	const token = "A7b9C2d4E6f8G1h3J5k7L9m2N4p6Q8r0S2t4U6v"
	got := redact.New(redact.ModeStandard).String("resource name " + token)
	if !strings.Contains(got, token) {
		t.Errorf("Redactor.String() = %q, want high-entropy heuristic limited to rendered field values", got)
	}
}

func TestScanFreeTextPreservesOrdinaryOperationalText(t *testing.T) {
	t.Parallel()

	for _, input := range []string{
		"Guest Wi-Fi rollout, ticket CHG-123456, contact network operations.",
		"Primary ISP circuit DIA-00001234, VLAN 120, rack A7.",
		"Change reference 550e8400-e29b-41d4-a716-446655440000 for audit lookup.",
		"Reviewed git commit 0123456789abcdef0123456789abcdef01234567 during rollout.",
		"See https://example.invalid/help/locations for the runbook.",
		"Quarterly password rotation policy reminder, no credential values here.",
	} {
		input := input
		t.Run(input, func(t *testing.T) {
			t.Parallel()

			got, report := redact.New(redact.ModeStandard).ScanFreeText(input)
			if got != input {
				t.Errorf("Redactor.ScanFreeText(%q) = %q, want unchanged text", input, got)
			}
			if !report.Empty() {
				t.Errorf("Redactor.ScanFreeText(%q) report = %#v, want empty", input, report)
			}
		})
	}
}

func TestRedactorRemovesZscalerShapedProvisioningKey(t *testing.T) {
	t.Parallel()

	input := "connector key 1|api.private.example.net|68F0AOEgpcG8McLmwdborq2m6v2A5oNEpSztJ=="
	got := redact.New(redact.ModeStandard).String(input)
	if strings.Contains(got, "68F0AOEgpcG8McLmwdborq2m6v2A5oNEpSztJ") {
		t.Errorf("Redactor.String(provisioning key) = %q, want key material redacted", got)
	}
	if !strings.Contains(got, "<REDACTED:PROVISIONING_KEY>") {
		t.Errorf("Redactor.String(provisioning key) = %q, want provisioning key marker", got)
	}
}

func TestRedactorRemovesPrivateKeyBlocks(t *testing.T) {
	t.Parallel()

	input := fakePrivateKeyBlock("abc123secretkeymaterial")
	got := redact.New(redact.ModeStandard).String(input)
	if strings.Contains(got, "abc123secretkeymaterial") {
		t.Errorf("Redactor.String(private key) = %q, want key material redacted", got)
	}
	if !strings.Contains(got, "<REDACTED:PRIVATE_KEY>") {
		t.Errorf("Redactor.String(private key) = %q, want private key marker", got)
	}
}

func TestShareModeRemovesSensitiveIdentifiers(t *testing.T) {
	t.Parallel()

	input := "owner alice@example.com uses 192.0.2.10"
	got := redact.New(redact.ModeShare).String(input)
	for _, forbidden := range []string{"alice@example.com", "192.0.2.10"} {
		if strings.Contains(got, forbidden) {
			t.Errorf("Redactor.String(%q) = %q, want no %q", input, got, forbidden)
		}
	}
	for _, marker := range []string{"<REDACTED:EMAIL>", "<REDACTED:IP>"} {
		if !strings.Contains(got, marker) {
			t.Errorf("Redactor.String(%q) = %q, want marker %q", input, got, marker)
		}
	}
}

func TestScanStringReportsFindings(t *testing.T) {
	t.Parallel()

	got, report := redact.New(redact.ModeShare).ScanString("owner alice@example.com psk=supersecret")
	if strings.Contains(got, "alice@example.com") || strings.Contains(got, "supersecret") {
		t.Errorf("Redactor.ScanString() = %q, want sensitive values removed", got)
	}
	if report.Counts["email"] != 1 {
		t.Errorf("Redactor.ScanString() email count = %d, want 1", report.Counts["email"])
	}
	if report.Counts["secret_assignment"] != 1 {
		t.Errorf("Redactor.ScanString() secret_assignment count = %d, want 1", report.Counts["secret_assignment"])
	}
}

func TestParseModeRejectsUnsupportedValueWithoutEchoingIt(t *testing.T) {
	t.Parallel()

	const canary = "plainredactioncanary"
	_, err := redact.ParseMode(canary)
	if err == nil {
		t.Fatalf("ParseMode(%q) error = nil, want error", canary)
	}
	if strings.Contains(err.Error(), canary) {
		t.Errorf("ParseMode(%q) error = %q, want no raw value echo", canary, err.Error())
	}
}
