package redact_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/redact"
)

// TestPrivateKeyArmorRedactsCompletely covers PGP private key blocks and
// private keys pasted without their END line: no body line, however short,
// may survive, whatever headers or line breaks precede it, and JSON output
// stays valid.
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
	rsaEnd := "-----END RSA PRIVATE" + " KEY-----"
	for _, tc := range []struct{ name, input string }{
		{"pem without end", "Lab key, do not use:\n" + rsa + "\n" + strings.Join(body, "\n")},
		{"encrypted pem without end", rsa + "\nProc-Type: 4,ENCRYPTED\nDEK-Info: AES-128-CBC,0A1B2C3D4E5F60718293A4B5C6D7E8F9\n\n" + strings.Join(body, "\n")},
		{"pgp block", pgp + "\nVersion: GnuPG v2\n\n" + strings.Join(body, "\n") + "\n" + pgpEnd},
		{"pgp without end", pgp + "\n\n" + strings.Join(body, "\n")},
		{"single line without end", rsa + " " + strings.Join(body, " ")},
		// Armour headers come before the body and may hold any printable
		// character, so a header must not end the match early.
		{"pgp comment header without end", pgp + "\nVersion: GnuPG v2\nComment: Alice Example <alice@example.com>\nComment: https://gpgtools.org (GPG Suite)\n\n" + strings.Join(body, "\n")},
		{"pgp comment header crlf without end", pgp + "\r\nComment: Alice <alice@example.com>\r\n\r\n" + strings.Join(body, "\r\n")},
		{"pgp comment header single line without end", pgp + " Comment: Alice <alice@example.com> " + strings.Join(body, " ")},
		{"pgp comment header", pgp + "\nComment: Alice <alice@example.com>\n\n" + strings.Join(body, "\n") + "\n" + pgpEnd},
		{"indented comment header without end", rsa + "\n  Comment: Alice <alice@example.com>\n\n" + strings.Join(body, "\n")},
		{"literal backslash-n in header without end", pgp + "\nComment: Use \\n for newline <test>\n\n" + strings.Join(body, "\n")},
		{"backslash at end of header without end", pgp + "\nComment: Alice\\\nVersion: GnuPG v2\n\n" + strings.Join(body, "\n")},
		{"truncated inside headers", pgp + "\nComment: Alice <alice@example.com>\nVersion: " + body[2]},
		{"literal escaped blank line in header without end", pgp + "\nComment: Use \\n\\n for newline <test>\n\n" + strings.Join(body, "\n")},
		{"literal escaped blank line with space in header without end", pgp + "\nComment: Use \\n \\n for newline <test>\n\n" + strings.Join(body, "\n")},
		{"blank line before header without end", rsa + "\n\nComment: Alice <alice@example.com>\n\n" + strings.Join(body, "\n")},
		{"escaped then raw break before header without end", rsa + "\\n\nComment: Alice <alice@example.com>\n\n" + strings.Join(body, "\n")},
		{"raw then escaped break before header without end", rsa + "\n\\nComment: Alice <alice@example.com>\n\n" + strings.Join(body, "\n")},
		{"all escaped with literal blank line in header without end", rsa + `\nComment: Use \n\n for newline <test>\n\n` + strings.Join(body, `\n`)},
		{"cr-only line breaks without end", rsa + "\rComment: Alice <alice@example.com>\r\r" + strings.Join(body, "\r")},
		{"header adjacent to begin without end", rsa + "Comment: Alice <alice@example.com>\n\n" + strings.Join(body, "\n")},
		{"unusual header name without end", rsa + "\nComment_2: Alice <alice@example.com>\n\n" + strings.Join(body, "\n")},
		{"prose between header and body without end", pgp + "\nComment: x\n\nRotated; see INC0012345.\n" + strings.Join(body, "\n")},
		// An END-like line before the body must not end the key early.
		{"pgp header holding end without real end", pgp + "\nComment: " + pgpEnd + "\n" + strings.Join(body, "\n")},
		{"pgp header holding end before real end", pgp + "\nComment: " + pgpEnd + "\n" + strings.Join(body, "\n") + "\n" + pgpEnd},
		{"rsa header holding end without real end", rsa + "\nComment: " + rsaEnd + "\n" + strings.Join(body, "\n")},
		{"prose holding end before body", rsa + "\nRotated; see INC0012345.\n" + rsaEnd + "\n" + strings.Join(body, "\n")},
		{"complete block then truncated block", rsa + "\nq7Rx\n" + rsaEnd + "\nnext:\n" + rsa + "\n" + strings.Join(body, "\n")},
		// BEGIN lines in any case, with loose dashes or spacing.
		{"lowercase begin", "-----begin rsa private" + " key-----\n" + strings.Join(body, "\n")},
		{"begin without trailing dashes", "-----BEGIN RSA PRIVATE" + " KEY\n" + strings.Join(body, "\n")},
		{"begin with four dashes", "----BEGIN RSA PRIVATE" + " KEY-----\n" + strings.Join(body, "\n")},
		{"begin with doubled spaces", "-----BEGIN  RSA PRIVATE" + "  KEY-----\n" + strings.Join(body, "\n")},
		// Labels with digits and punctuation.
		{"rsa1 label", "-----BEGIN RSA1 PRIVATE" + " KEY-----\n" + strings.Join(body, "\n")},
		{"ssh2 label", "---- BEGIN SSH2 ENCRYPTED PRIVATE" + " KEY ----\n" + strings.Join(body, "\n")},
		{"pkcs8 label", "-----BEGIN PKCS8 PRIVATE" + " KEY-----\n" + strings.Join(body, "\n")},
		{"x9.42 label", "-----BEGIN X9.42 DH PRIVATE" + " KEY-----\n" + strings.Join(body, "\n")},
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

	// A key is redacted to the end of the value that holds it, with or
	// without an END line: text before BEGIN survives, prose after the key
	// does not, and other values in the same document are untouched.
	for _, input := range []string{
		"Lab key:\n" + rsa + "\n" + body[0] + "\n" + rsaEnd + "\nRotated by netops.",
		"Lab key:\n" + rsa + "\n" + body[0] + "\nRotated by netops.",
	} {
		for _, scanner := range allStringScanners {
			if got, _ := scanner.scan(redact.New(redact.ModeStandard), input); got != "Lab key:\n<REDACTED:PRIVATE_KEY>" {
				t.Errorf("%s(%q) = %q, want text before BEGIN and one marker", scanner.name, input, got)
			}
		}
	}
	doc, _ := json.Marshal(map[string]string{"description": rsa + "\n" + body[0], "name": "Branch East"})
	var decoded map[string]string
	if err := json.Unmarshal(redact.New(redact.ModeStandard).Bytes(doc), &decoded); err != nil {
		t.Fatalf("Bytes(two-field document) is not valid JSON: %v", err)
	}
	if decoded["description"] != "<REDACTED:PRIVATE_KEY>" || decoded["name"] != "Branch East" {
		t.Errorf("Bytes(two-field document) = %#v, want only description redacted", decoded)
	}
	public := "-----BEGIN PUBLIC" + " KEY-----\n" + body[0] + "\n-----END PUBLIC" + " KEY-----"
	if got, _ := redact.New(redact.ModeStandard).ScanString(public); strings.Contains(got, "PRIVATE_KEY") {
		t.Errorf("ScanString(public key) = %q, want no private key marker", got)
	}
}
