package redact_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/redact"
)

// TestPrivateKeyArmorRedactsCompletely covers PGP private key blocks and
// private keys pasted without their END line: no body line, however short,
// may survive, and JSON output stays valid.
func TestPrivateKeyArmorRedactsCompletely(t *testing.T) {
	t.Parallel()

	body := []string{
		strings.Repeat("MIIEvQIBADANBgkqhkiG9w0BAQEF", 2) + "AASC",
		strings.Repeat("Zx81Qw93Er72Ty64Ui05Op", 2) + "Lk3Jh2Gf",
		"q7Rt",
	}
	rsa := "-----BEGIN RSA PRIVATE" + " KEY-----"
	pgp := "-----BEGIN PGP PRIVATE" + " KEY BLOCK-----"
	pgpEnd := "-----END PGP PRIVATE" + " KEY BLOCK-----"
	for _, tc := range []struct{ name, input string }{
		{"pem without end", "Lab key, do not use:\n" + rsa + "\n" + strings.Join(body, "\n")},
		{"encrypted pem without end", rsa + "\nProc-Type: 4,ENCRYPTED\nDEK-Info: AES-128-CBC,0A1B2C3D4E5F60718293A4B5C6D7E8F9\n\n" + strings.Join(body, "\n")},
		{"pgp block", pgp + "\nVersion: GnuPG v2\n\n" + strings.Join(body, "\n") + "\n" + pgpEnd},
		{"pgp without end", pgp + "\n\n" + strings.Join(body, "\n")},
		{"single line without end", rsa + " " + strings.Join(body, " ")},
	} {
		for _, scanner := range allStringScanners {
			got, report := scanner.scan(redact.New(redact.ModeStandard), tc.input)
			for _, line := range body {
				if strings.Contains(got, line) {
					t.Errorf("%s: %s left body line %q in %q", tc.name, scanner.name, line, got)
				}
			}
			if report.Empty() || !strings.Contains(got, "<REDACTED:PRIVATE_KEY>") {
				t.Errorf("%s: %s = %q, want a private key marker", tc.name, scanner.name, got)
			}
		}
		// The same paste inside a JSON string, with escaped line breaks.
		doc, _ := json.Marshal(map[string]string{"description": tc.input})
		out := redact.New(redact.ModeStandard).Bytes(doc)
		var decoded map[string]string
		if err := json.Unmarshal(out, &decoded); err != nil {
			t.Errorf("%s: Bytes output is not valid JSON: %v (%s)", tc.name, err, out)
		}
		for _, line := range body {
			if strings.Contains(string(out), line) {
				t.Errorf("%s: Bytes left body line %q", tc.name, line)
			}
		}
	}

	// Prose after a truncated key survives from its first punctuation, and
	// public blocks are untouched.
	got, _ := redact.New(redact.ModeStandard).ScanFreeText(rsa + "\n" + body[0] + "\nRotated by netops; see INC0012345.")
	if !strings.Contains(got, "; see INC0012345.") {
		t.Errorf("ScanFreeText(truncated key + prose) = %q, want the prose kept", got)
	}
	public := "-----BEGIN PUBLIC" + " KEY-----\n" + body[0] + "\n-----END PUBLIC" + " KEY-----"
	if got, _ := redact.New(redact.ModeStandard).ScanString(public); strings.Contains(got, "PRIVATE_KEY") {
		t.Errorf("ScanString(public key) = %q, want no private key marker", got)
	}
}
