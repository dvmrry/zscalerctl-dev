package redact_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/redact"
)

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

	input := `{
		"apiKey": "zia-cloud-key",
		"secretKey": "client-connector-secret",
		"sandboxApiToken": "sandbox-token-value",
		"bearerToken": "bearer-token-value",
		"provisioningKey": "1|api.private.example.net|abcdefghiJKLMNOP1234567890abcdefghijklmnopqrstuvwxyz",
		"privateKey": "-----BEGIN PRIVATE KEY-----\\nkey-material\\n-----END PRIVATE KEY-----",
		"certBlob": "-----BEGIN PRIVATE KEY-----\\ncert-blob-private-key-material\\n-----END PRIVATE KEY-----",
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

	input := strings.Join([]string{
		"-----BEGIN PRIVATE KEY-----",
		"abc123secretkeymaterial",
		"-----END PRIVATE KEY-----",
	}, "\n")
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

func TestParseModeRejectsOff(t *testing.T) {
	t.Parallel()

	if _, err := redact.ParseMode("off"); err == nil {
		t.Errorf("ParseMode(%q) error = nil, want error", "off")
	}
}
