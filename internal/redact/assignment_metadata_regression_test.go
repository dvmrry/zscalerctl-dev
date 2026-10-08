package redact_test

import (
	"testing"

	"github.com/dvmrry/zscalerctl/internal/redact"
)

func TestPublicAssignmentsRequireIdentifierNames(t *testing.T) {
	t.Parallel()

	const compact = "b7e19c024af683d095bc72e461da8f30"
	const uuid = "123e4567-e89b-12d3-a456-426614174000"
	names := []struct {
		name   string
		public bool
	}{
		{"community", false},
		{"pin", false},
		{"opaque", false},
		{"liquid", false},
		{"invalid", false},
		{"id", true},
		{"identifier", true},
		{"uuid", true},
		{"guid", true},
		{"ref", true},
		{"reference", true},
		{"LocationId", true},
		{"location_id", true},
		{"location-uuid", true},
		{"ObjectKey", true},
		{"RowKey", true},
		{"PartitionKey", true},
		{"SortKey", true},
		{"ForeignKey", true},
		{"PrimaryKey", false}, // Azure access keys are named "Primary key"
		{"digest", true},
		{"commit", true},
	}
	scanners := []scannerCase{
		{"ScanRenderedString", redact.Redactor.ScanRenderedString},
		{"ScanFreeText", redact.Redactor.ScanFreeText},
	}

	for _, name := range names {
		for _, value := range []string{compact, uuid, compact + "12345678"} {
			input := name.name + "=" + value
			want := "<REDACTED:SECRET>"
			if name.public {
				want = input
			}
			for _, scanner := range scanners {
				got, report := scanner.scan(redact.New(redact.ModeStandard), input)
				// A credential label may redact only the value.
				if !name.public && got == name.name+"=<REDACTED:SECRET>" {
					got = want
				}
				if got != want {
					t.Errorf("%s(%q) = %q, want %q", scanner.name, input, got, want)
				}
				if report.Empty() != name.public {
					t.Errorf("%s(%q) report = %v, want empty = %t", scanner.name, input, report.Counts, name.public)
				}
			}
		}
	}

	const input = "community=b7e19c024af683d095bc72e461da8f30"
	if got, report := redact.New(redact.ModeStandard).ScanFreeText(input); got != "<REDACTED:SECRET>" || report.Empty() {
		t.Errorf("ScanFreeText(%q) = %q (report %v), want complete redaction", input, got, report.Counts)
	}

	const path = "mapping=Projects/2024/NetworkSwitch2024"
	if got, report := redact.New(redact.ModeStandard).ScanString(path); got != path || !report.Empty() {
		t.Errorf("ScanString(%q) = %q (report %v), want unchanged", path, got, report.Counts)
	}
}

func TestCredentialMetadataAndValuesStayDistinct(t *testing.T) {
	t.Parallel()

	const uuid = "123e4567-e89b-12d3-a456-426614174000"
	const hmacMaterial = "b7e19c02" + "4af683d0" + "95bc72e4" + "61da8f30" + "b7e19c02" + "4af683d0" + "95bc72e4" + "61da8f30"
	const checksumHex = "d41d8cd98f00b204e9800998ecf8427e" + "d41d8cd98f00b204e9800998ecf8427e"
	const jobULID = "01ARZ3NDEK" + "TSV4RRFFQ69G5FAV"
	cases := []struct {
		input string
		want  string
	}{
		{"Token ID: " + uuid, "Token ID: " + uuid},
		{"Token identifier is " + uuid, "Token identifier is " + uuid},
		{`{"tokens":[{"scopes":["` + uuid + `"]}]}`, `{"tokens":[{"scopes":["` + uuid + `"]}]}`},
		{`{"credentials":[{"objectId":"` + uuid + `"}]}`, `{"credentials":[{"objectId":"` + uuid + `"}]}`},
		{`{"tokens":["` + uuid + `"]}`, `{"tokens":["<REDACTED:SECRET>"]}`},
		{"McAfeeEndpointSecurity2026", "McAfeeEndpointSecurity2026"},
		{"McAfeeWebGateway2026", "McAfeeWebGateway2026"},
		{"iSCSIStorageNetwork2026", "iSCSIStorageNetwork2026"},
		{"AzureApplicationGatewayV2", "AzureApplicationGatewayV2"},
		{"ApplicationGatewayV2", "ApplicationGatewayV2"},
		{"MicrosoftEntraConnectV2", "MicrosoftEntraConnectV2"},
		{"ServiceNowChangeWindowR2", "ServiceNowChangeWindowR2"},
		{"WindowsR2LegacyServersDev", "WindowsR2LegacyServersDev"},
		{"NetAppFSxStorageProd2026", "NetAppFSxStorageProd2026"},
		{"RaspberryPiComputeModule4", "RaspberryPiComputeModule4"},
		{"API subscription Primary" + "Key=" + hmacMaterial[:32], "API subscription Primary" + "Key=<REDACTED:SECRET>"},
		{`MongoDB asset ObjectId("652f8a1b9c7d4e30a56b2f90")`, `MongoDB asset ObjectId("652f8a1b9c7d4e30a56b2f90")`},
		{`ObjectId('652f8a1b9c7d4e30a56b2f90')`, `ObjectId('652f8a1b9c7d4e30a56b2f90')`},
		{"HMAC-SHA256 key → " + hmacMaterial, "HMAC-SHA256 key → <REDACTED:SECRET>"},
		{"HMAC key (SHA256): " + hmacMaterial, "HMAC key (SHA256): <REDACTED:SECRET>"},
		{"HMAC key (MD5): " + hmacMaterial, "HMAC key (MD5): <REDACTED:SECRET>"},
		{"HMAC-SHA256 Secret" + "Key → " + hmacMaterial, "HMAC-SHA256 Secret" + "Key → <REDACTED:SECRET>"},
		{"SHA256 signing_" + "key: " + hmacMaterial, "SHA256 signing_" + "key: <REDACTED:SECRET>"},
		{"HMAC-SHA256 se" + "cret: " + hmacMaterial, "HMAC-SHA256 se" + "cret: <REDACTED:SECRET>"},
		{"Key checksum → " + checksumHex, "Key checksum → " + checksumHex},
		{"Key fingerprint is da39a3ee5e6b4b0d3255bfef95601890afd80709", "Key fingerprint is da39a3ee5e6b4b0d3255bfef95601890afd80709"},
		{"API key owner: " + uuid, "API key owner: " + uuid},
		{"API token IDs: " + uuid, "API token IDs: " + uuid},
		{"API key owner:\n" + uuid, "API key owner:\n" + uuid},
		{"API token IDs:\n" + uuid, "API token IDs:\n" + uuid},
		{"Splunk HEC token for the test index: " + uuid, "Splunk HEC token for the test index: <REDACTED:SECRET>"},
		{"Token: " + uuid, "Token: <REDACTED:SECRET>"},
		{"Token is " + uuid, "Token is <REDACTED:SECRET>"},
		{"Token for the vendor portal is " + uuid, "Token for the vendor portal is <REDACTED:SECRET>"},
		{"API key name is " + uuid, "API key name is " + uuid},
		{"Account token identifier is " + uuid, "Account token identifier is " + uuid},
		{"Token for the name is " + uuid, "Token for the name is " + uuid},
		{"API token for the name is " + uuid, "API token for the name is <REDACTED:SECRET>"},
		{"Access token for the name is " + uuid, "Access token for the name is <REDACTED:SECRET>"},
		{"API key for the name is " + uuid, "API key for the name is <REDACTED:SECRET>"},
		// An identifier noun directly before the value names it, even with
		// other words between it and the credential label.
		{"The API key object ID is " + uuid + "; search the audit inventory.", "The API key object ID is " + uuid + "; search the audit inventory."},
		{`Password reset correlation ID = "` + uuid + `"`, `Password reset correlation ID = "` + uuid + `"`},
		{"Token signing certificate serial is 046A912C83107E55", "Token signing certificate serial is 046A912C83107E55"},
		{"Secret rotation job ULID: " + jobULID, "Secret rotation job ULID: " + jobULID},
		{"API key for the ID service is " + uuid, "API key for the ID service is <REDACTED:SECRET>"},
		{"Password for the ID portal: " + hmacMaterial[:24], "Password for the ID portal: <REDACTED:SECRET>"},
	}
	for _, label := range []string{
		"Key", "Token", "Secret", "Credential", "Credentials",
		"Password", "Passwd", "PWD", "Bearer", "Sig",
	} {
		metadata := label + " identifier is " + uuid
		cases = append(cases,
			struct {
				input string
				want  string
			}{metadata, metadata},
			struct {
				input string
				want  string
			}{label + " is " + uuid, label + " is <REDACTED:SECRET>"},
		)
	}
	for _, modifier := range []string{"", "API ", "Access ", "Secret "} {
		for _, word := range []string{"key", "token"} {
			label := modifier + word
			for _, noun := range []string{"ID", "name", "alias"} {
				for _, cue := range []string{": ", " is ", ":\n"} {
					metadata := label + " " + noun + cue + uuid
					cases = append(cases, struct {
						input string
						want  string
					}{metadata, metadata})
				}
			}
			for _, cue := range []string{": ", " is "} {
				cases = append(cases, struct {
					input string
					want  string
				}{label + cue + uuid, label + cue + "<REDACTED:SECRET>"})
			}
		}
	}

	for _, tc := range cases {
		for _, scanner := range allStringScanners {
			got, report := scanner.scan(redact.New(redact.ModeStandard), tc.input)
			if got != tc.want {
				t.Errorf("%s(%q) = %q, want %q", scanner.name, tc.input, got, tc.want)
			}
			if report.Empty() != (tc.want == tc.input) {
				t.Errorf("%s(%q) report = %v, want empty = %t", scanner.name, tc.input, report.Counts, tc.want == tc.input)
			}
		}
	}

	// An Azure resource ID after a credential label is not key material.
	// Free-text scanners keep main's entropy decision for its segments.
	azure := "Secret rotation target: /subscriptions/" + uuid + "/resourceGroups/rg-identity-prod"
	if got, report := redact.New(redact.ModeStandard).ScanString(azure); got != azure || !report.Empty() {
		t.Errorf("ScanString(%q) = %q (report %v), want unchanged", azure, got, report.Counts)
	}
}
