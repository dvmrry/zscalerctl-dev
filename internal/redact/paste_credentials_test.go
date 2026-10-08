package redact_test

import (
	"encoding/base64"
	"encoding/json"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/dvmrry/zscalerctl/internal/redact"
)

// Paste rules look ahead from every label, cue and line; repeated labels on
// one long line must not make that lookahead quadratic. As in
// TestGenericLabelScanIsLinearOnRepeatedLabels, scan time at n and 4n
// repetitions is compared.
func TestPasteCredentialScanIsLinear(t *testing.T) {
	if raceEnabled {
		t.Skip("skipping under -race: timing comparisons are distorted by race instrumentation")
	}
	t.Parallel()

	for _, tt := range []struct {
		name  string
		input func(n int) string
	}{
		{"password (", func(n int) string { return strings.Repeat("password (", n) }},
		{"password '", func(n int) string { return strings.Repeat("password '", n) }},
		{"Key |", func(n int) string { return strings.Repeat("Key | ", n) }},
		{"is the key", func(n int) string { return strings.Repeat("x is the key ", n) }},
		{"Key: newline", func(n int) string { return strings.Repeat("Key:\n", n) }},
		{"YAML name", func(n int) string { return strings.Repeat("- name: API_KEY\n", n) }},
		{"code wrapper", func(n int) string { return "Key: <code>" + strings.Repeat("x", 4*n) }},
		{"sig=", func(n int) string { return strings.Repeat("&sig=%2F", n) }},
		{"JSON key members", func(n int) string {
			return "note: " + strings.Repeat(`{"key":"550e8400-e29b-41d4-a716-446655440000",`, n)
		}},
		{"spaced modifiers", func(n int) string { return strings.Repeat("Primary \t key ", n) }},
	} {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			r := redact.New(redact.ModeStandard)
			small, large := interleavedFastest(func(input string) { r.ScanString(input) }, tt.input(5000), tt.input(20000))
			if ratio := float64(large) / float64(small); ratio > 10 {
				t.Errorf("ScanString(%s) time grew %.1fx for 4x input (%s -> %s), want linear", tt.name, ratio, small, large)
			}
		})
	}
}

// interleavedFastest times scan on small and large inputs alternately and
// returns the fastest run of each (small at least 1ms). Alternating keeps
// machine load from skewing one size, so linearity checks stay stable when the
// whole suite runs in parallel; the caller's subtests run serially for the
// same reason.
func interleavedFastest(scan func(string), small, large string) (time.Duration, time.Duration) {
	bestSmall, bestLarge := time.Duration(-1), time.Duration(-1)
	for i := 0; i < 5; i++ {
		start := time.Now()
		scan(small)
		if elapsed := time.Since(start); bestSmall < 0 || elapsed < bestSmall {
			bestSmall = elapsed
		}
		start = time.Now()
		scan(large)
		if elapsed := time.Since(start); bestLarge < 0 || elapsed < bestLarge {
			bestLarge = elapsed
		}
	}
	if bestSmall < time.Millisecond {
		bestSmall = time.Millisecond
	}
	return bestSmall, bestLarge
}

// Synthetic credentials are assembled from fragments so secret scanners do
// not flag this file.
var (
	pastePassword = "m7!Qv2" + "@Lp9#Rx4" + "$Tn8%Wc3^Za6"
	pasteUUID     = strings.Join([]string{"e3a1f70c", "-9b26-", "4d85-", "a607-", "82d9c41f6b30"}, "")
	pasteShortKey = "q9Lm4Vx7" + "R2c8Np5Z"
)

func TestScannersRedactPasteCredentialFormats(t *testing.T) {
	t.Parallel()

	password, uuid, shortKey := pastePassword, pasteUUID, pasteShortKey
	runnerToken := "gl" + "rt-" + "r8Vq2Lm9Jx4Np7Az5Tkc"
	vaultToken := "hv" + "s." + "q8Vn3Lm7Rx2Ck9Wp5Jt4Nz6A"
	basic := base64.StdEncoding.EncodeToString([]byte("svc-review:" + "Maple" + "Cobalt!"))
	encodedSig := url.QueryEscape("A7b9C2d4E6f8G1h3J5k7L9m2" + "N4p6Q8r0S2t4U6v+/=")

	cases := []struct {
		class  string
		input  string
		secret string
	}{
		// Punctuation passwords after password-class labels.
		{"password", "Driver={ODBC Driver 18 for SQL Server};Server=sql.pilot.example;UID=svc;PWD=" + password + ";", password},
		{"password", "Server=sql.pilot.example;User Id=svc;Pwd='" + password + "';Encrypt=True", password},
		{"password", "[client]\nuser=svc\npasswd=" + password + "\nhost=db.pilot.example", password},
		{"password", "export MYSQL_PWD='" + password + "'", password},
		{"password", "poc:\n  passwd: '" + password + "'\n  user: svc", password},
		{"password", "[database]\npasswd = \"" + password + "\"", password},
		{"password", "Temporary password is " + password + "; rotate after the demo.", password},
		{"password", "The password was '" + password + "' during the vendor PoC.", password},
		{"password", "Please use password " + password + " for the test login.", password},
		{"password", "password for the trial account is " + password, password},
		{"password", "Trial password is Cobalt!Harbor@Willow.", "Cobalt!Harbor@Willow"},
		{"password", "The passphrase is Cobalt Willow Maple Harbor!", "Cobalt Willow Maple Harbor!"},
		{"password", "The temporary password is 70461938; delete the trial account afterwards.", "70461938"},
		{"password", "password 70461938", "70461938"},
		// Word-built passwords after snake_case "PASS" setting names.
		{"password", "export DB_PASS='CorrectHorseBatteryStaple2024Ultra'", "CorrectHorseBatteryStaple2024Ultra"},
		{"password", "SMTP_PASS: CorrectHorseBatteryStaple2024Ultra", "CorrectHorseBatteryStaple2024Ultra"},
		{"password", "db_pass=" + "CorrectHorseBatteryStaple2024Ultra", "CorrectHorseBatteryStaple2024Ultra"},
		{"password", `{"name":"SMTP_PASS","value":"CorrectHorseBatteryStaple2024Ultra"}`, "CorrectHorseBatteryStaple2024Ultra"},
		{"password", "password='" + "CorrectHorseBatteryStaple2024Ultra'", "CorrectHorseBatteryStaple2024Ultra"},
		{"password", "Machine account passphrase for the inventory job: " + shortKey + "Kd3W", shortKey + "Kd3W"},
		// CLI and PowerShell credential arguments.
		{"cli", "mysql --host=db.pilot.example --user=svc --password '" + password + "'", password},
		{"cli", "mysql -h db.pilot.example -u svc -p'" + password + "'", password},
		{"cli", "sqlcmd -S sql.pilot.example -U svc -P '" + password + "'", password},
		{"cli", "mysqldump -u svc -p'" + password + "' inventory", password},
		{"cli", `osql -S sql.pilot.example -U svc -P "` + password + `"`, password},
		// Controls next to preserved forms: a credential modifier with odd
		// spacing, and key/value JSON in prose whose key is not an identifier
		// or has no value member.
		{"separator", "API  key: " + uuid, uuid},
		{"json", `CMDB entry: {"Key":"` + shortKey + `","Value":{"asset":"branch"}}`, shortKey},
		{"json", `CMDB entry: {"Key":"` + uuid + `","Owner":"netops"}`, uuid},
		{"cli", "sshpass -p '" + password + "' ssh svc@pilot.example", password},
		{"cli", "sshpass -v -p '" + password + "' ssh -p 8022 svc@pilot.example", password},
		{"cli", "mysql -h db.pilot.example -P 3306 -u svc -p'" + password + "'", password},
		{"cli", "curl --" + "user 'svc-review:" + password + "' https://pilot.example/health", password},
		{"cli", "curl 'https://pilot.example/api?a=1&b=2' -" + "u 'svc-review:" + password + "'", password},
		{"cli", `cmdkey /add:pilot.example /user:svc /pass:"` + password + `"`, password},
		{"cli", `net use \\fileserver\share "` + password + `" /user:EXAMPLE\svc`, password},
		{"cli", "$password = ConvertTo-SecureString '" + password + "' -AsPlainText -Force", password},
		{"cli", "Set-ADAccountPassword -Identity svc -NewPassword (ConvertTo-SecureString '" + password + "' -AsPlainText -Force)", password},
		{"cli", "$cred = New-Object System.Management.Automation.PSCredential('svc','" + password + "')", password},
		{"cli", "New-Object System.Net.NetworkCredential('svc','" + password + "')", password},
		{"cli", "kubectl create secret generic pilot --from-literal=passwd='" + password + "'", password},
		{"cli", "$env:MYSQL_PWD = '" + password + "'", password},
		// JSON credential keys and name/value pairs.
		{"json", `{"poc_key":"` + uuid + `"}`, uuid},
		{"json", `{"serviceKey":"` + uuid + `"}`, uuid},
		{"json", `{"credentials":"` + uuid + `"}`, uuid},
		{"json", `{"consumerKey":"` + shortKey + `"}`, shortKey},
		{"json", `{"keyValue":"` + uuid + `"}`, uuid},
		{"json", `{"SecretID":"` + uuid + `","Description":"consul trial"}`, uuid},
		{"json", `{"token":["` + uuid + `"]}`, uuid},
		{"json", `{"name":"API_KEY","value":"` + uuid + `"}`, uuid},
		{"json", `{"key":"password","value":"` + password + `"}`, password},
		{"json", `{"env":[{"name":"MYSQL_PWD","value":"` + password + `"}]}`, password},
		{"json", `{"fields":[{"label":"password","value":"` + shortKey + `"}]}`, shortKey},
		{"json", "env:\n  - name: MYSQL_PWD\n    value: '" + password + "'", password},
		{"json", "- name: API_KEY\n  value: " + uuid, uuid},
		// Separators: tables, arrows, dashes, full-width colon, HTML wrappers.
		{"separator", "| API key | " + uuid + " |", uuid},
		{"separator", "| Key | " + shortKey + " |", shortKey},
		{"separator", "| Password | " + password + " |", password},
		{"separator", "**Key** | `" + uuid + "`", uuid},
		{"separator", "Key → " + uuid, uuid},
		{"separator", "Token — " + uuid, uuid},
		{"separator", "API key： " + uuid, uuid},
		{"separator", "password： " + password, password},
		{"separator", "Key: <code>" + uuid + "</code>", uuid},
		{"separator", "Password: <kbd>" + password + "</kbd>", password},
		// Values on the next line.
		{"next line", "Key:\n> " + uuid, uuid},
		{"next line", "Trial key:\n```text\n" + uuid + "\n```", uuid},
		{"next line", "Token:\n```sh\n" + uuid + "\n```", uuid},
		{"next line", "key: |-\n  " + uuid, uuid},
		{"next line", "token: >-\n  " + uuid, uuid},
		{"next line", "POC key:\n# copied from the vendor portal\n" + uuid, uuid},
		{"next line", "API key (test tenant)\n" + uuid, uuid},
		{"next line", "The PoC credential shown on screen:\n" + uuid, uuid},
		// Values before their label.
		{"before label", "Use " + uuid + " as the key for the test tenant.", uuid},
		{"before label", "The portal gave us " + uuid + " (API key).", uuid},
		{"before label", uuid + " is the current token.", uuid},
		{"before label", "`" + uuid + "` — key copied from the partner portal.", uuid},
		{"before label", password + " is the password for the trial account.", password},
		// Percent-encoded query credentials.
		{"encoded query", "https://pilot.blob.core.windows.net/c/notes.txt?sv=2023-11-03&sp=r&sig=" + encodedSig, encodedSig},
		{"encoded query", "az storage blob download --sas-token '?sv=2023-11-03&sp=r&sig=" + encodedSig + "'", encodedSig},
		{"encoded query", "https://pilot.example/download?key=" + encodedSig, encodedSig},
		{"encoded query", "https://pilot.example/poc?credential=" + encodedSig, encodedSig},
		{"encoded query", "The signed link ends in sig=" + encodedSig, encodedSig},
		// Basic credentials.
		{"basic", "Basic " + basic, basic},
		{"basic", "BASIC_AUTH=" + basic, basic},
		{"basic", `C:\Admin> set SNOW_BASIC_AUTH=svc_audit:` + password, password},
		{"basic", "data:\n  auth: " + basic, basic},
		{"basic", "curl -H 'X-Auth: Basic " + basic + "' https://pilot.example/health", basic},
		{"basic", `{"basicAuth":"Basic ` + basic + `"}`, basic},
		// Vendor formats and localized labels.
		{"vendor", "Runner authentication: " + runnerToken, runnerToken},
		{"vendor", runnerToken, runnerToken},
		{"vendor", "Vault " + vaultToken, vaultToken},
		{"vendor", "SecretID: " + uuid, uuid},
		{"vendor", "Vercel auth: " + shortKey + "Lm7Rx2Ck", shortKey + "Lm7Rx2Ck"},
		{"localized", "API-Schlüssel: " + uuid, uuid},
		{"localized", "Zugangsschlüssel = " + uuid, uuid},
		{"localized", "Passwort für den Testzugang: " + password, password},
		{"localized", "Kennwort: " + password, password},
		{"localized", "API-Token ist " + uuid, uuid},
		{"localized", "Clé API : " + uuid, uuid},
		{"localized", "Jeton d'accès : " + uuid, uuid},
		{"localized", "Mot de passe : " + password, password},
		{"localized", "Clave de API: " + uuid, uuid},
		{"localized", "Contraseña temporal: " + password, password},
		{"localized", "Credencial del portal: " + uuid, uuid},
		{"localized", "API key es " + uuid, uuid},
		{"localized", "APIキー：" + uuid, uuid},
		{"localized", "トークン：" + uuid, uuid},
		{"localized", "パスワード：" + password, password},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.class+"/"+tc.input, func(t *testing.T) {
			t.Parallel()
			for _, scanner := range allStringScanners {
				got, report := scanner.scan(redact.New(redact.ModeStandard), tc.input)
				if strings.Contains(got, tc.secret) {
					t.Errorf("%s(%q) = %q, leaked %q", scanner.name, tc.input, got, tc.secret)
				}
				if report.Empty() || !strings.Contains(got, "<REDACTED:SECRET>") {
					t.Errorf("%s(%q) = %q, report %#v, want a secret marker and finding", scanner.name, tc.input, got, report)
				}
				if strings.HasPrefix(tc.input, "{") && !json.Valid([]byte(got)) {
					t.Errorf("%s(%q) returned invalid JSON %q", scanner.name, tc.input, got)
				}
			}
		})
	}
}

// Each paste rule is paired with ordinary text that carries the same label or
// syntax but no credential.
func TestScannersPreserveOrdinaryPasteCredentialText(t *testing.T) {
	t.Parallel()

	uuid := strings.Join([]string{"550e8400", "-e29b-", "41d4-", "a716-", "446655440000"}, "")
	cases := []string{
		"--password-file /etc/app/db-password",
		"mysql --password-file /etc/x --user svc",
		// A literal that is another parameter's or command's argument.
		"Get-Secret 'connector-password' -AsPlainText",
		`Get-Secret "connector-password" -AsPlainText`,
		`Get-Secret -Name "connector-password" -AsPlainText`,
		// The -u of a command after a pipe or separator is not curl's.
		"curl -sN https://host/events | sed -" + "u 's/^data: //g'",
		"curl -sN https://host/events; sed -" + "u 's/^data: //g' events.log",
		"curl -sN https://host/events && sed -" + "u 's/^data: //g' events.log",
		"$secure = ConvertTo-SecureString -String (Get-Secret -Name 'connector-password' -AsPlainText) -AsPlainText -Force",
		// sshpass's own -P prompt and the wrapped command's -p port.
		"sshpass -f /run/ssh.pass ssh -p 8022 admin@host",
		"sshpass -P 'Password:' -f /run/ssh.pass ssh admin@host",
		"sshpass -e ssh -p 2222 admin@host",
		"sshpass -f ~/.ssh/pass scp -P 2222 backup.tar admin@host:/srv/",
		// mysql's -P is the port; sqlcmd's -p prints statistics.
		"mysql -h db.example.net -P '3306' -u svc -p",
		`sqlcmd -S sql.example.net -E -p "1"`,
		"Password policy is NIST80063B",
		"Password hash format is bcrypt12",
		"Password reset guide: ResetGuide2024.pdf",
		"passwd = ${DB_PASSWORD}",
		"pwd: ********",
		"Passwd: stored in CyberArk under the owner's safe.",
		"The password expires in 90 days.",
		"pwd output is /srv/netops2024",
		"Set-ADAccountPassword -Identity svc -NewPassword $secure",
		"docker run -p '8080:80' nginx",
		`net use Z: \\fileserver\share /user:EXAMPLE\svc`,
		"curl -u $USER:$TOKEN https://pilot.example/health",
		"$cred = New-Object PSCredential('svc', $secure)",
		"Basic authentication is disabled for this app.",
		"| Key | Value |",
		"| Key length | 2048 |",
		"| Setting | Value |\n| --- | --- |\n| Password expiry | 90 days |\n| Token lifetime | 60 minutes |",
		"| Partition key | " + uuid + " |",
		"Partition key → " + uuid,
		"sig verification disabled; incident: INC202410060456",
		"https://pilot.example/search?key=%2Fdocs%2Fguide&page=2",
		"Key:\n- rotate quarterly\n- owner: identity team",
		"Credential:\nRequired",
		"API key (test tenant)\nNot issued yet",
		uuid + " is the partition key for the location record.",
		uuid + " (key ID)",
		`{"name":"LOG_LEVEL","value":"debug"}`,
		`{"Key":"Environment","Value":"production"}`,
		`{"credentials":"none"}`,
		`{"label":"Password policy","value":"strict"}`,
		// Working directories on Windows drives and UNC shares, raw and
		// JSON-escaped.
		`pwd: C:\Users\Admin\Projects2024`,
		`pwd: C:/Users/Admin/Projects2024`,
		`pwd=\\fileserver\share\Projects2024`,
		`{"pwd":"C:\\Users\\Admin\\Projects2024"}`,
		`Runbook output: "{\"pwd\":\"C:\\\\Users\\\\Admin\\\\Projects2024\"}"`,
		// Path segments containing spaces.
		`pwd: C:\Python312`,
		`{"pwd":"C:\\Python312"}`,
		`{"pwd":"D:\\Data2024"}`,
		`{"cwd":"/data2024"}`,
		`{"pwd":"\\\\?\\C:\\Temp\\run2024"}`,
		`{"pwd":"C:\\Windows\\Temp\\MSI8f3a2.LOG"}`,
		`Runbook output: "{\"pwd\":\"C:\\\\Python312\"}"`,
		`{"pwd":"C:\\Program Files\\PowerShell\\7"}`,
		`{"pwd":"C:\\Program Files (x86)\\Microsoft\\Edge"}`,
		`{"cwd":"/home/svc/Network Projects/branch2024"}`,
		`Runbook output: "{\"pwd\":\"C:\\\\Program Files\\\\PowerShell\\\\7\"}"`,
		"Mountain pass: Route66Highway",
		// Data-key modifiers with extra whitespace or Markdown around them.
		"Partition  key: 550e8400-e29b-41d4-a716-446655440000",
		"Partition\tkey: 550e8400-e29b-41d4-a716-446655440000",
		"Partition **key**: 550e8400-e29b-41d4-a716-446655440000",
		"**Partition** key: 550e8400-e29b-41d4-a716-446655440000",
		"Partition `key`: 550e8400-e29b-41d4-a716-446655440000",
		// "-p" outside password-taking commands is a different flag.
		"git log -p 'feature/network2024'",
		// Key/value JSON pairs inside prose, either member order.
		`CMDB entry: {"Key":"550e8400-e29b-41d4-a716-446655440000","Value":{"asset":"branch"}}`,
		`CMDB entry: {"value":"asset","key":"550e8400-e29b-41d4-a716-446655440000"}`,
		"first_pass: complete",
		// A public identifier under "key" in a key/value pair, any casing.
		`{"Key":"550e8400-e29b-41d4-a716-446655440000","Value":"asset"}`,
		`{"KEY":"550e8400-e29b-41d4-a716-446655440000","VALUE":"asset"}`,
		`{"Key":"550e8400-e29b-41d4-a716-446655440000","Value":{"asset":"branch"}}`,
		`{"Value":["asset","branch"],"key":"550e8400-e29b-41d4-a716-446655440000"}`,
		// A MongoDB ObjectId named by its member key, and the prose control.
		`{"_id":"65ab12cd34ef567890ab12cd"}`,
		"MongoDB asset _id 65ab12cd34ef567890ab12cd",
		`{"key":"550e8400-e29b-41d4-a716-446655440000","Value":"asset"}`,
		// The same pair pasted as JSON into a description string.
		`{"description":"{\"key\":\"550e8400-e29b-41d4-a716-446655440000\",\"value\":\"asset\"}"}`,
		`{"description":"{\"Key\":\"550e8400-e29b-41d4-a716-446655440000\",\"Value\":\"asset\"}"}`,
		`{"ulid":"01ARZ3NDEKTSV4RRFFQ69G5FAV","primaryKey":"01ARZ3NDEKTSV4RRFFQ69G5FAV"}`,
		"BASIC_AUTH=disabled",
		"passwd source file is Users2024.csv",
		"**Password policy**: NIST80063B",
		"- name: DB_PASSWORD\n  valueFrom:\n    secretKeyRef:\n      name: pilot",
		"glrt-short",
	}

	for _, input := range cases {
		input := input
		t.Run(input, func(t *testing.T) {
			t.Parallel()
			for _, scanner := range allStringScanners {
				got, report := scanner.scan(redact.New(redact.ModeStandard), input)
				if got != input {
					t.Errorf("%s(%q) = %q, want unchanged text", scanner.name, input, got)
				}
				if !report.Empty() {
					t.Errorf("%s(%q) report = %#v, want empty", scanner.name, input, report)
				}
			}
		})
	}
}

// Word-built passwords of 32 or more characters keep main's long-entropy
// decision in every phrasing and wrapper; only recognized public identifiers
// are exempt on that path, never words, paths or label phrasing.
func TestWordBuiltLongPasswordsKeepEntropyDecision(t *testing.T) {
	t.Parallel()

	const value = "CorrectHorseBatteryStaple2024Ultra"
	r := redact.New(redact.ModeStandard)
	for _, tc := range []struct{ input, secret string }{
		{"export APP_SETTING='" + value + "'", value},
		{"app_setting=" + value, value},
		{"app setting: " + value, value},
		{"The password for the trial account is " + value, value},
		{"THE PASSWORD FOR THE TRIAL ACCOUNT IS " + value, value},
		{"The API key for the pilot tenant was '" + value + "'", value},
		{"Token for the export job is " + value, value},
		{"The password for the trial account is **" + value + "**", value},
		{"The password for the trial account is CorrectHorseBatteryStaple/Ultra2024", "CorrectHorseBatteryStaple/Ultra2024"},
		{"Rollout ring " + value + " approved.", value},
		// Deliberate: a word-built name of 32 or more characters in free text
		// keeps main's entropy decision and is redacted, because it cannot be
		// told apart from a word-built password of the same shape.
		{"UserMustChangePasswordAtNextLogon2024", "UserMustChangePasswordAtNextLogon2024"},
		{"Policy BlockO365GuestAccessForContractors applies.", "BlockO365GuestAccessForContractors"},
	} {
		for _, scan := range []func(string) (string, redact.Report){r.ScanFreeText, r.ScanRenderedString} {
			got, report := scan(tc.input)
			if strings.Contains(got, tc.secret) || report.Empty() || !strings.Contains(got, "<REDACTED:SECRET>") {
				t.Errorf("scan(%q) = %q (report %v), want the password redacted", tc.input, got, report.Counts)
			}
		}
	}
	for _, input := range []string{
		"Policy VoIPQualityOfService2024 applies to the branch.",
	} {
		for _, scan := range []func(string) (string, redact.Report){r.ScanFreeText, r.ScanRenderedString, r.ScanString} {
			if got, _ := scan(input); got != input {
				t.Errorf("scan(%q) = %q, want unchanged", input, got)
			}
		}
	}
}

// JSON pasted into a description is preserved by projection (ScanFreeText of
// the description) and must survive the final ScanString over rendered JSON
// and NDJSON unchanged, so emitted values agree with projection.
func TestNestedJSONDescriptionRendersAsProjected(t *testing.T) {
	t.Parallel()

	r := redact.New(redact.ModeStandard)
	for _, inner := range []string{
		`{"key":"550e8400-e29b-41d4-a716-446655440000","value":"asset"}`,
		`{"Key":"550e8400-e29b-41d4-a716-446655440000","Value":"asset"}`,
		`[{"key":"` + `i-0d12e34f56a78b90c","value":"asset"}]`,
	} {
		projected, report := r.ScanFreeText(inner)
		if projected != inner || !report.Empty() {
			t.Fatalf("ScanFreeText(%q) = %q (report %v), want unchanged", inner, projected, report.Counts)
		}
		record, err := json.Marshal(map[string]string{"description": projected})
		if err != nil {
			t.Fatal(err)
		}
		for _, rendered := range []string{string(record), string(record) + "\n" + string(record) + "\n"} {
			got, _ := r.ScanString(rendered)
			for _, line := range strings.Split(strings.TrimSuffix(got, "\n"), "\n") {
				var decoded struct {
					Description string `json:"description"`
				}
				if err := json.Unmarshal([]byte(line), &decoded); err != nil || decoded.Description != inner {
					t.Errorf("ScanString(%q) description = %q (err %v), want %q", rendered, decoded.Description, err, inner)
				}
			}
		}
	}

	// A credential in nested JSON is still redacted.
	secret := "q9Lm4Vx" + "7R2c8" + "Np5Zk3Wt7Bd1Hs6Y"
	nested, _ := json.Marshal(map[string]string{"description": `{"name":"API_KEY","value":"` + secret + `"}`})
	if got, _ := r.ScanString(string(nested)); strings.Contains(got, secret) {
		t.Errorf("ScanString(%s) = %q, leaked nested credential", nested, got)
	}
}

// A JSON primaryKey holding a UUID is usually a record identifier; it is
// redacted only alongside another credential key, while a non-identifier
// value (an Azure-style access key) is redacted on its own.
func TestJSONPrimaryKeyNeedsCredentialCueForIdentifiers(t *testing.T) {
	t.Parallel()

	uuid := strings.Join([]string{"550e8400", "-e29b-", "41d4-", "a716-", "446655440000"}, "")
	accessKey := "q9Lm4Vx" + "7R2c8" + "Np5Zk3Wt7Bd1Hs6Y"
	for _, tc := range []struct {
		name   string
		input  string
		secret string
		redact bool
	}{
		{"identifier alone", `{"primaryKey":"` + uuid + `","region":"eu"}`, uuid, false},
		{"ULID alone", `{"primaryKey":"01ARZ3NDEKTSV4RRFFQ69G5FAV","region":"eu"}`, "01ARZ3NDEKTSV4RRFFQ69G5FAV", false},
		{"secondary ULID alone", `{"SecondaryKey":"` + `01ARZ3NDEKTSV4RRFFQ69G5FAV","region":"eu"}`, "01ARZ3NDEKTSV4RRFFQ69G5FAV", false},
		{"cloud resource ID alone", `{"primary_key":"i-0d12e34f56a78b90c"}`, "i-0d12e34f56a78b90c", false},
		{"identifier with credential cue", `{"primaryKey":"` + uuid + `","credential":"trial"}`, uuid, true},
		{"access key alone", `{"primaryKey":"` + accessKey + `"}`, accessKey, true},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, _ := redact.New(redact.ModeStandard).ScanString(tc.input)
			if !json.Valid([]byte(got)) {
				t.Fatalf("ScanString(%q) returned invalid JSON %q", tc.input, got)
			}
			if leaked := strings.Contains(got, tc.secret); leaked == tc.redact {
				t.Errorf("ScanString(%q) = %q, want redacted=%v", tc.input, got, tc.redact)
			}
		})
	}
}

// TestAssignedGetSecretKeepsVaultName covers "$password = Get-Secret 'name'
// -AsPlainText": the password assignment rule replaces the command name, as
// on main, but the vault name is not a plaintext password. Get-Secret takes
// no -Force; ConvertTo-SecureString's literal before "-AsPlainText -Force"
// is still redacted.
func TestAssignedGetSecretKeepsVaultName(t *testing.T) {
	t.Parallel()

	for _, input := range []string{
		"$password = Get-Secret 'connector-password' -AsPlainText",
		`$password = Get-Secret "connector-password" -AsPlainText`,
		"$password = Get-Secret -Name 'connector-password' -AsPlainText",
		"$password = <REDACTED:SECRET> 'connector-password' -AsPlainText",
	} {
		for _, scanner := range allStringScanners {
			if got, _ := scanner.scan(redact.New(redact.ModeStandard), input); !strings.Contains(got, "connector-password") {
				t.Errorf("%s(%q) = %q, want the vault name kept", scanner.name, input, got)
			}
		}
	}
	const literal = "Wint" + "er!2026x"
	for _, input := range []string{
		"$password = ConvertTo-SecureString '" + literal + "' -AsPlainText -Force",
		"$password = ConvertTo-SecureString \"" + literal + "\" -AsPlainText -Force)",
	} {
		for _, scanner := range allStringScanners {
			if got, _ := scanner.scan(redact.New(redact.ModeStandard), input); strings.Contains(got, literal) {
				t.Errorf("%s(%q) = %q, want the literal redacted", scanner.name, input, got)
			}
		}
	}
}
