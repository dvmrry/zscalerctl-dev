package redact_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/redact"
)

// These cases come from the two leak classes the redaction design exists to
// stop: PoC/test keys pasted into admin free text and names, and key material
// that upstream treats as an ordinary field value. Each canary must not
// survive any scanner that can see it.

// escapedFixture writes JSON escapes as "^" in source and restores the
// backslash at runtime, so fixtures cannot be decoded by accident before the
// scanner sees them.
func escapedFixture(s string) string {
	return strings.ReplaceAll(s, "^", "\x5c")
}

type scannerCase struct {
	name string
	scan func(redact.Redactor, string) (string, redact.Report)
}

var allStringScanners = []scannerCase{
	{"ScanString", redact.Redactor.ScanString},
	{"ScanRenderedString", redact.Redactor.ScanRenderedString},
	{"ScanFreeText", redact.Redactor.ScanFreeText},
}

func TestScannersRedactGenericLabeledCredentials(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		input  string
		canary string
	}{
		{"POC key: " + "A7b9C2d4E6f8G1h3J5k7", "A7b9C2d4E6f8G1h3J5k7"},
		{"key=" + "A7b9C2d4E6f8G1h3J5k7", "A7b9C2d4E6f8G1h3J5k7"},
		{"poc_key=" + "A7b9C2d4E6f8G1h3J5k7 for the vendor", "A7b9C2d4E6f8G1h3J5k7"},
		{"Bearer A7b9C2d4E6f8J5k7", "A7b9C2d4E6f8J5k7"},
		{"POC key: 550e8400-e29b-41d4-a716-446655440000", "550e8400-e29b-41d4-a716-446655440000"},
		{"token A7b9C2d4E6f8G1h3J5k7L9m2N4p6", "A7b9C2d4E6f8G1h3J5k7L9m2N4p6"},
		{"x-api-key A7b9C2d4E6f8G1h3J5k7", "A7b9C2d4E6f8G1h3J5k7"},
		{"token: 550e8400-e29b-41d4-a716-446655440000", "550e8400-e29b-41d4-a716-446655440000"},
		{"password " + "Abc123def456", "Abc123def456"},
		{"pwd: Zx81Qw93Er72Ty", "Zx81Qw93Er72Ty"},
		{"secret 9f86d081884c7d659a2feaa0c55ad015", "9f86d081884c7d659a2feaa0c55ad015"},
		{`trial "key": ` + `"A7b9C2d4E6f8G1h3J5k7"`, "A7b9C2d4E6f8G1h3J5k7"},
		{`{"key":"` + `A7b9C2d4E6f8G1h3J5k7"}`, "A7b9C2d4E6f8G1h3J5k7"},
		{escapedFixture(`{"k^u0065y":"A7b9C2d4E6f8G1h3J5k7"}`), "A7b9C2d4E6f8G1h3J5k7"},
		{`{"note":"POC key: ` + `A7b9C2d4E6f8G1h3J5k7"}`, "A7b9C2d4E6f8G1h3J5k7"},
		// A rejected value that is itself a label must be reconsidered.
		{"POC key: Bearer A7b9C2d4E6f8J5k7", "A7b9C2d4E6f8J5k7"},
		{"secret token: key=" + "A7b9C2d4E6f8G1h3J5k7", "A7b9C2d4E6f8G1h3J5k7"},
		// Sentence punctuation and opening brackets around the value.
		{"POC key: 550e8400-e29b-41d4-a716-446655440000.", "550e8400-e29b-41d4-a716-446655440000"},
		{"POC key: " + "A7b9C2d4E6f8G1h3J5k7.", "A7b9C2d4E6f8G1h3J5k7"},
		{"POC key: (A7b9C2d4E6f8G1h3J5k7)", "A7b9C2d4E6f8G1h3J5k7"},
		{"POC key: " + "`A7b9C2d4E6f8G1h3J5k7`", "A7b9C2d4E6f8G1h3J5k7"},
		{"POC key: <A7b9C2d4E6f8G1h3J5k7>", "A7b9C2d4E6f8G1h3J5k7"},
		// Whitespace inside wrappers, Unicode spaces, and a label at the end of
		// a rejected value.
		{"POC key: ( A7b9C2d4E6f8J5k7 )", "A7b9C2d4E6f8J5k7"},
		{"POC key:\xc2\xa0A7b9C2d4E6f8J5k7", "A7b9C2d4E6f8J5k7"},
		{"POC key\xe2\x80\x8b: A7b9C2d4E6f8J5k7", "A7b9C2d4E6f8J5k7"},
		{"POC key:\xe3\x80\x80\"A7b9C2d4E6f8J5k7\"", "A7b9C2d4E6f8J5k7"},
		{"POC key: foo-token " + "A7b9C2d4E6f8J5k7", "A7b9C2d4E6f8J5k7"},
		// Nested valid labels inside an outer label's value, and a quoted
		// JSON-escaped value inside prose (canary is a raw fragment that only
		// survives if the literal is not redacted).
		{"POC key: key=" + "550e8400-e29b-41d4-a716-446655440000", "550e8400-e29b-41d4-a716-446655440000"},
		{"POC key: v2.token=A7b9C2d4E6f8G1h3J5k7", "A7b9C2d4E6f8G1h3J5k7"},
		{escapedFixture(`POC key: "A7b9C2d^u0034E6f8G1h^u0033J5k7L9m^u0032N4p6Q8r^u0030S2t4U6v"`), "J5k7L9m"},
		// Round 4: a label-shaped suffix inside the key, whitespace inside an
		// escaped quoted value, and a separator hidden by an escape.
		{"POC key: " + "A7b9G2d4E6f_keY=", "A7b9G2d4E6f"},
		{"POC key: " + "A7b9G2d4E6f8_key=Zx81", "A7b9G2d4E6f8"},
		{escapedFixture(`POC key: " A7b9C2d^u0034E6f8J5k7 "`), "E6f8J5k7"},
		{escapedFixture(`POC key: "key^u003d550e8400-e29b-41d4-a716-446655440000"`), "41d4-a716-446655440000"},
		// Round 5: typographic quotes from email/docs, and a wrapped UUID as
		// Go's JSON encoder writes it (< and > escaped).
		{"POC key: \xe2\x80\x9cA7b9C2d4E6f8J5k7\xe2\x80\x9d", "A7b9C2d4E6f8J5k7"},
		{"POC key: \xe2\x80\x98A7b9C2d4E6f8J5k7\xe2\x80\x99", "A7b9C2d4E6f8J5k7"},
		{"POC key: \xc2\xabA7b9C2d4E6f8J5k7\xc2\xbb", "A7b9C2d4E6f8J5k7"},
		{"POC key: <550e8400-e29b-41d4-a716-446655440000>", "550e8400-e29b-41d4-a716-446655440000"},
		{escapedFixture(`{"key":"^u003c550e8400-e29b-41d4-a716-446655440000^u003e"}`), "550e8400-e29b-41d4-a716-446655440000"},
		// Round 6: typographic quotes around the label too, and whitespace
		// inside a JSON-encoded wrapper.
		{"POC \xe2\x80\x9ckey\xe2\x80\x9d: \xe2\x80\x9cA7b9C2d4E6f8J5k7\xe2\x80\x9d", "A7b9C2d4E6f8J5k7"},
		{"POC \xe2\x80\x98key\xe2\x80\x99: \xe2\x80\x98A7b9C2d4E6f8J5k7\xe2\x80\x99", "A7b9C2d4E6f8J5k7"},
		{"POC key: < 550e8400-e29b-41d4-a716-446655440000 >", "550e8400-e29b-41d4-a716-446655440000"},
		{escapedFixture(`{"key":"^u003c 550e8400-e29b-41d4-a716-446655440000 ^u003e"}`), "550e8400-e29b-41d4-a716-446655440000"},
		// Round 7: Markdown backticks and German quotes around the label, and
		// a letters-only key.
		{"POC `key`: `A7b9C2d4E6f8J5k7`", "A7b9C2d4E6f8J5k7"},
		{"POC \xe2\x80\x9ekey\xe2\x80\x9c: \xe2\x80\x9eA7b9C2d4E6f8J5k7\xe2\x80\x9c", "A7b9C2d4E6f8J5k7"},
		{"POC key: QzWpRtYuHsDfGjKlXcVb", "QzWpRtYuHsDfGjKlXcVb"},
		// Round 8: padded base64 letters-only key, and Markdown emphasis
		// around the label.
		{"POC key: " + "QzWpRtYuHsDfGjKlXcVbZg==", "QzWpRtYuHsDfGjKlXcVbZg"},
		{`{"key":"` + `QzWpRtYuHsDfGjKlXcVbZg=="}`, "QzWpRtYuHsDfGjKlXcVbZg"},
		{"POC **key**: A7b9C2d4E6f8J5k7", "A7b9C2d4E6f8J5k7"},
		{"POC __key__: " + "A7b9C2d4E6f8J5k7", "A7b9C2d4E6f8J5k7"},
		{"POC *key*: **A7b9C2d4E6f8J5k7**", "A7b9C2d4E6f8J5k7"},
		// Round 9: Markdown underscores around a UUID.
		{"POC key: " + "_550e8400-e29b-41d4-a716-446655440000_", "550e8400-e29b-41d4-a716-446655440000"},
		{"POC key: " + "__550e8400-e29b-41d4-a716-446655440000__", "550e8400-e29b-41d4-a716-446655440000"},
		{`{"key":"` + `_550e8400-e29b-41d4-a716-446655440000_"}`, "550e8400-e29b-41d4-a716-446655440000"},
		// Corpus round: natural phrasing between label and value, CamelCase
		// and parameter-style labels, "credential", and vendor prefixes.
		{"The temporary key is 6b1e9a4d-2c7f-4d8a-9e3b-1f5c7a2d6b0e, please rotate it.", "6b1e9a4d-2c7f-4d8a-9e3b-1f5c7a2d6b0e"},
		{"Splunk HEC token for the test index: " + "6f1c9a3d-8b2e-4d7a-9c5f-1e3b6a8d2c4f", "6f1c9a3d-8b2e-4d7a-9c5f-1e3b6a8d2c4f"},
		{"API key for the vendor portal is `R8mQ2vN6xD1kF5hJ9wB3cT7pL4aE0z`", "R8mQ2vN6xD1kF5hJ9wB3cT7pL4aE0z"},
		{"The token was Q5tY8uI1oP4aS7dF0gH3jK; pasted into the site note", "Q5tY8uI1oP4aS7dF0gH3jK"},
		{"temp password is " + "W3cI8zA2fS7uT1n", "W3cI8zA2fS7uT1n"},
		{"AccountKey=" + "Q2hhbmdlVGhpc0Zha2VLZXk=", "Q2hhbmdlVGhpc0Zha2VLZXk"},
		{"blob?sp=rl&sig=K3vR8mQ2xN6pD1tF9hJ4bL7w", "K3vR8mQ2xN6pD1tF9hJ4bL7w"},
		{"credential: " + "'M1nB6vC0xZ5aE9sY4wF8j' # remove after validation", "M1nB6vC0xZ5aE9sY4wF8j"},
		{"vendor trial notes **secret:** `N6tF1hJ5wB9cA0zE4sY8m`", "N6tF1hJ5wB9cA0zE4sY8m"},
		{"gl" + "pat-X8wV3qM7nB2dF9hK4pR1cT6 copied into the description", "gl" + "pat-X8wV3qM7nB2dF9hK4pR1cT6"},
		{"PAT ghp_9Q2mV7xR4dK1pN6tB3hF8wL5cJ0aE2z", "ghp_9Q2mV7xR4dK1pN6tB3hF8wL5cJ0aE2z"},
		{"pasted xo" + "xb-180294756318-492716380541-aW3zS7xC by accident", "xo" + "xb-180294756318-492716380541-aW3zS7xC"},
		{"maps key AIzaSyD4mQ8vN2pR6tX1cB5hJ9kL3wF7gA0", "AIzaSyD4mQ8vN2pR6tX1cB5hJ9kL3wF7gA0"},
		{"stripe sk_" + "live_51Na4Qz8Yr2Mw6Xp9Dv3K", "sk_" + "live_51Na4Qz8Yr2Mw6Xp9Dv3K"},
		// Round 10: an assignment label with a backtick-wrapped value, as
		// main already redacted it.
		{"psk: `A7b9C2d4E6f8J5k7`", "A7b9C2d4E6f8J5k7"},
		{"client_secret: " + "`A7b9C2d4E6f8J5k7`", "A7b9C2d4E6f8J5k7"},
		{"password:** `A7b9C2d4E6f8J5k7`", "A7b9C2d4E6f8J5k7"},
		// Separator-split and standard base64 values after a label.
		{"key=" + "A7b9+C2d4/E6f8G1h3J5k7==", "E6f8G1h3J5k7"},
		{"token Zt4K-q9Lw_2Hx7-Vn3B_j8Mc", "Zt4K-q9Lw_2Hx7-Vn3B_j8Mc"},
	} {
		for _, scanner := range allStringScanners {
			tt, scanner := tt, scanner
			t.Run(scanner.name+"/"+tt.input, func(t *testing.T) {
				t.Parallel()

				for _, mode := range []redact.Mode{redact.ModeStandard, redact.ModeShare, redact.ModeParanoid} {
					got, report := scanner.scan(redact.New(mode), tt.input)
					if strings.Contains(got, tt.canary) {
						t.Errorf("%s(%q, %s) = %q, want canary redacted", scanner.name, tt.input, mode, got)
					}
					if !strings.Contains(got, "<REDACTED:") {
						t.Errorf("%s(%q, %s) = %q, want redaction marker", scanner.name, tt.input, mode, got)
					}
					if report.Empty() {
						t.Errorf("%s(%q, %s) report empty, want a redaction count", scanner.name, tt.input, mode)
					}
				}
			})
		}
	}
}

func TestScannersPreserveOrdinaryTextNearCredentialWords(t *testing.T) {
	t.Parallel()

	for _, input := range []string{
		"key: production",
		"key=region",
		"Primary key rotation is quarterly",
		"token bucket rate 100 per second",
		"secret santa event",
		"Bearer of bad news",
		"key: rule-2024-q3-block-gambling",
		"tag key Environment owner netops",
		"hotkey=CtrlShiftF12",
		"monkey 12345678abcdefgh",
		"token endpoint is documented; password rotation policy is quarterly",
		`{"key":"Environment","value":"prod"}`,
		`{"key":"kubernetes.io/cluster/prod-01","value":"owned"}`,
		`{"key":"12345678901234","value":"rack"}`,
		"tag key: Projects/2024/Q3-planning",
		`{"key":"Projects/2024/Q3-planning","value":"owner"}`,
		escapedFixture(`{"k^u0065y":"Projects/2024/Q3-planning","value":"owner"}`),
		"tag key: Projects/2024/production2024",
		`{"key":"Projects/2024/production2024","value":"owner"}`,
		"key: PrimarySite2024",
		"token production2024",
		"tag key: Projects/2024/NetworkSwitch2024",
		`{"key":"Projects/2024/NetworkSwitch2024","value":"owner"}`,
		"key: HTTPServer2024Primary",
		"key: CoreRouter01",
		"key: 2024ProductionRollout",
		"tag key: Projects/2024/IPv6Firewall",
		`{"key":"Projects/2024/IPv6Firewall","value":"owner"}`,
		"key: Switch01Port48",
		"key: CoreSwitch02Rack14",
		"key: Switch01Eth0",
		"key: " + "IPv6Firewall01",
		`{"key":"Switch01Eth0","value":"uplink"}`,
		"tag key: Site/NYC/Switch01Eth0",
		"tag key: Catalyst01Gi0/1",
		`{"key":"Catalyst01Gi0/1","value":"uplink"}`,
		"tag key: " + "CiscoNexus9K01",
		`{"key":"` + `CiscoNexus9K01","value":"uplink"}`,
		"key: ProductionDatabaseCluster",
		"key: internationalization",
		"tag key: monthlypatchwindows",
		`{"key":"monthlypatchwindows","value":"Sunday"}`,
		"**key**: production",
		"tag key: Projects/2024/iOS",
		`{"key":"Projects/2024/iOS","value":"owner"}`,
		"tag key: Teams/EMEA/macOS/2025",
		"Primary key rotation for vnet-hub-eastus2 is quarterly",
		"The key for site NYC is NYC-HQ-FLOOR12",
		"password policy requires 12 characters; password expiry is 90days",
		"credential rotation runbook v3",
		"Password reset ticket: INC0012345",
		"Key rotation ticket: " + "RITM0012345678",
		"Token revocation tracked in OPS-1234",
		"secret rotation change: CHG0034567",
		"PrimaryKey: CustomerID",
		"design: ProductionLayout2024",
		"key: \xe2\x80\x9cproduction\xe2\x80\x9d",
		"key: production.",
		"key: rule-2024-q3-block-gambling.",
	} {
		for _, scanner := range allStringScanners {
			input, scanner := input, scanner
			t.Run(scanner.name+"/"+input, func(t *testing.T) {
				t.Parallel()

				got, report := scanner.scan(redact.New(redact.ModeStandard), input)
				if got != input {
					t.Errorf("%s(%q) = %q, want unchanged text", scanner.name, input, got)
				}
				if !report.Empty() {
					t.Errorf("%s(%q) report = %#v, want empty", scanner.name, input, report)
				}
			})
		}
	}
}

// Rejected label values must not make the label rule quadratic: each value is
// examined once. The test compares scan time at n and 4n repetitions; linear
// scanning grows about 4x and quadratic about 16x. A ratio is used instead of
// a wall-clock bound so race-detector and machine overhead cancel out.
func TestGenericLabelScanIsLinearOnRepeatedLabels(t *testing.T) {
	if raceEnabled {
		t.Skip("skipping under -race: timing comparisons are distorted by race instrumentation")
	}
	t.Parallel()

	for _, tt := range []struct {
		name  string
		input func(n int) string
	}{
		{"key=", func(n int) string { return strings.Repeat("key=", n) + "production" }},
		{"a.key", func(n int) string { return strings.Repeat("a.key", n) + " production" }},
		{"key: Bearer", func(n int) string { return strings.Repeat("key: Bearer ", n) + "production" }},
		{"long value", func(n int) string { return "key=" + strings.Repeat("x", 4*n) }},
	} {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			r := redact.New(redact.ModeStandard)
			small, large := interleavedFastest(func(input string) {
				if got, _ := r.ScanString(input); got != input {
					t.Fatalf("ScanString(%d-byte %s input) changed text without a credential", len(input), tt.name)
				}
			}, tt.input(5000), tt.input(20000))
			if ratio := float64(large) / float64(small); ratio > 10 {
				t.Errorf("ScanString(%s) time grew %.1fx for 4x input (%s -> %s), want linear", tt.name, ratio, small, large)
			}
		})
	}
}

func TestScanFreeTextRedactsShortUnlabeledKeys(t *testing.T) {
	t.Parallel()

	for _, token := range []string{
		"A7b9C2d4E6f8G1h3J5k7L9m2",        // 24
		"A7b9C2d4E6f8G1h3J5k7L9m2N4p6",    // 28
		"Zt4Kq9Lw2Hx7Vn3Bj8Mc5Rp1Df6Gs0Y", // 31
	} {
		token := token
		t.Run(token, func(t *testing.T) {
			t.Parallel()

			input := "temporary vendor note " + token + " remove after trial"
			got, report := redact.New(redact.ModeStandard).ScanFreeText(input)
			if strings.Contains(got, token) {
				t.Errorf("ScanFreeText(%q) = %q, want short key redacted", input, got)
			}
			if report.Counts["high_entropy_short_free_text_token"] != 1 {
				t.Errorf("ScanFreeText(%q) report = %#v, want one short free-text token", input, report)
			}
		})
	}
}

func TestShortUnlabeledKeyRuleIsLimitedToFreeText(t *testing.T) {
	t.Parallel()

	const token = "A7b9C2d" + "4E6f8G1h3J5k7L9m2N4p6"
	input := "segment " + token
	for _, scanner := range []scannerCase{
		{"ScanString", redact.Redactor.ScanString},
		{"ScanRenderedString", redact.Redactor.ScanRenderedString},
	} {
		got, _ := scanner.scan(redact.New(redact.ModeStandard), input)
		if !strings.Contains(got, token) {
			t.Errorf("%s(%q) = %q, want short unlabeled token kept outside free text", scanner.name, input, got)
		}
	}
}

func TestScanFreeTextPreservesShortOperationalIdentifiers(t *testing.T) {
	t.Parallel()

	for _, input := range []string{
		"Guest Wi-Fi rollout, ticket CHG-123456, contact network operations.",
		"Primary ISP circuit DIA-00001234, VLAN 120, rack A7.",
		"Rollout of ZscalerClientConnector4 to EMEA in 2024.",
		"Order 4500012345678901234567 from procurement.",
		"Region uswest2production01primary standby pair.",
		"Pilot ZscalerClientConnector2024ab group.",
		"Ticket refs INC0012345SNOW7788ab and CHG.",
	} {
		input := input
		t.Run(input, func(t *testing.T) {
			t.Parallel()

			got, report := redact.New(redact.ModeStandard).ScanFreeText(input)
			if got != input {
				t.Errorf("ScanFreeText(%q) = %q, want unchanged text", input, got)
			}
			if !report.Empty() {
				t.Errorf("ScanFreeText(%q) report = %#v, want empty", input, report)
			}
		})
	}
}

// Unlabeled JSON-escaped key material embedded in prose must be judged on its
// decoded content by the rendered-string and free-text scanners, also when
// that prose is itself a string value of a JSON document.
func TestScannersRedactEscapedKeyEmbeddedInProse(t *testing.T) {
	t.Parallel()

	prose := escapedFixture(`vendor trial "A7b9C2d^u0034E6f8G1h^u0033J5k7L9m^u0032N4p6Q8r^u0030S2t4U6v" until Friday`)
	if strings.Count(prose, "\x5c") != 4 {
		t.Fatalf("fixture %q must contain four JSON escapes", prose)
	}
	wrapped, err := json.Marshal(map[string]string{"note": prose})
	if err != nil {
		t.Fatal(err)
	}
	for _, input := range []string{prose, string(wrapped)} {
		for _, scanner := range []scannerCase{
			{"ScanRenderedString", redact.Redactor.ScanRenderedString},
			{"ScanFreeText", redact.Redactor.ScanFreeText},
		} {
			got, report := scanner.scan(redact.New(redact.ModeStandard), input)
			if strings.Contains(got, "J5k7L9m") {
				t.Errorf("%s(%q) = %q, want escaped key redacted", scanner.name, input, got)
			}
			if report.Empty() {
				t.Errorf("%s(%q) report empty, want a redaction count", scanner.name, input)
			}
			if !strings.Contains(got, "vendor trial") || !strings.Contains(got, "until Friday") {
				t.Errorf("%s(%q) = %q, want surrounding prose kept", scanner.name, input, got)
			}
		}
	}
}

// Standard-mode display names use ScanString (see resources.scanStringValue),
// which must keep long operational names intact.
func TestScanStringPreservesOperationalNames(t *testing.T) {
	t.Parallel()

	for _, input := range []string{
		"location-company-cloud-zscaler-edge-prod-usw2-vnet-company-cloud-zscaler-edge-prod-usw2-resource-group-rg01",
		"api-gw-prod-usw2-01",
		"rule-2024-q3-block-gambling",
		"ZIA-LOC-NYC-HQ-FLOOR12",
		"550e8400-e29b-41d4-a716-446655440000",
		"550e8400e29b41d4a716446655440000",
		"0123456789abcdef0123456789abcdef01234567",
		"CN=proxy.example.com,O=Example",
		"Projects/2024/Q3-planning",
		"ZscalerClientConnectorRollout2024",
		"vnet-hub-eastus2-0a1b2c3d4e5f6a7b8c9d",
		"Projects/2024/IPv6Firewall",
		"Switch01Port48-CoreRouter02",
		"vSwitchProduction2024",
		"iPhoneFleetRollout2024",
		"risk-assessment-2024-q3-findings-summary",
		"desk-booking-floor12-east-wing-pilot",
	} {
		input := input
		t.Run(input, func(t *testing.T) {
			t.Parallel()

			got, report := redact.New(redact.ModeStandard).ScanString(input)
			if got != input {
				t.Errorf("ScanString(%q) = %q, want unchanged name", input, got)
			}
			if !report.Empty() {
				t.Errorf("ScanString(%q) report = %#v, want empty", input, report)
			}
		})
	}
}
