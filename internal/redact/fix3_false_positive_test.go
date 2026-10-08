package redact_test

import (
	"strings"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/redact"
)

// TestNonCredentialTextIsPreserved pins administrative text that the generic
// label and entropy scanners must leave alone in standard mode:
// key phrases that name data or metadata, CamelCase setting names, word
// paths, AWS-like names, vendor-prefix documentation names, working
// directories and public identifiers.
func TestNonCredentialTextIsPreserved(t *testing.T) {
	t.Parallel()

	groups := []struct {
		name   string
		inputs []string
	}{
		{
			name: "non-credential-key-phrases",
			inputs: []string{
				"Partition key: 550e8400-e29b-41d4-a716-446655440000",
				"**Partition key**: `550e8400-e29b-41d4-a716-446655440000`",
				"Foreign key for the location is 123e4567-e89b-12d3-a456-426614174000",
				"The database row key is 550e8400-e29b-41d4-a716-446655440000.",
				"The CMDB record key is 123e4567-e89b-12d3-a456-426614174000.",
				"Composite key is 550e8400-e29b-41d4-a716-446655440000",
				"Idempotency key for the change record is 550e8400-e29b-41d4-a716-446655440000",
				"The public key object ID is 123e4567-e89b-12d3-a456-426614174000",
				"ForeignKey=123e4567-e89b-12d3-a456-426614174000",
				"PartitionKey=550e8400-e29b-41d4-a716-446655440000",
				"RowKey=" + "123e4567-e89b-12d3-a456-426614174000",
				"SortKey=" + "i-0d12e34f56a78b90c",
				"ObjectKey=" + "01ARZ3NDEKTSV4RRFFQ69G5FAV",
				"Cache key: " + "01ARZ3NDEKTSV4RRFFQ69G5FAV",
				"Tag key: subnet-0e12d34c56b78a90f",
				"Cloud asset key: " + "vol-0a1b2c3d4e5f67890",
				"HotKey=CtrlShiftF12",
				"Function key binding: CtrlShiftF11",
				"SAML signing key identifier is 550e8400-e29b-41d4-a716-446655440000",
				"Key name for the server is srvnycprodproxy01",
				"Key fingerprint is da39a3ee5e6b4b0d3255bfef95601890afd80709",
				"Signing key thumbprint: " + "da39a3ee5e6b4b0d3255bfef95601890afd80709",
				"Key checksum is d41d8cd98f00b204e9800998ecf8427e",
				"Key digest is 2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824",
				// "Key documentation MD5 is <hex>" is deliberately not preserved:
				// algorithm names alone are not metadata ("HMAC key (SHA256): <key>").
				"Key metadata revision is be8bd9a107b3393fae1145835d77f2094b1f6e4e",
				"Key rotation implementation commit: be8bd9a107b3393fae1145835d77f2094b1f6e4e",
				"Signing key certificate reference is 123e4567-e89b-12d3-a456-426614174000",
				"Signing key certificate digest is 2jmj7l5rSw0yVb/vlWAYkK/YBwk=",
				"Public key thumbprint (SHA-1): 2jmj7l5rSw0yVb/vlWAYkK/YBwk=",
				"Key pinning digest format example is 1B2M2Y8AsgTpgAmY7PhCfg==",
				"Public key length 2048; rollout: WiFiEnterprise2024",
				"Key rotation quarterly; asset: i-0d12e34f56a78b90c",
				"Password hash algorithm is PBKDF2HMACSHA256",
			},
		},
		{
			name: "camelcase-short-words-acronyms-and-spellings",
			inputs: []string{
				"key: EnableSignInForWindows",
				"key: RequireMFAAtSignIn",
				"key: RememberMFAOnTrustedDevices",
				"key: DoNotStoreLANManagerHashes",
				"key: LogOnAsServicePolicy",
				"key: AllowAccessToPrivateApps",
				"key: MicrosoftEntraId",
				"key: VoIPQualityOfService",
				"key: GoToMeetingApplication",
				"key: LogMeInRescueApplication",
				"AzurePointToSiteVPN2024",
				"AWSClientToSiteVPN2024",
				"DoHResolverBypass2024",
				"IoTDeviceCompliance2024",
				"MicrosoftTeamsAddOn2024",
				"SQLServerAlwaysOn2024",
				"WiFiAccessForContractors2024",
				"iOSDeviceCompliancePolicy2024",
				"mDNSResponderException2024",
				"mTLSClientAuthentication2024",
				"eBPFNetworkMonitoring2024",
				"uRPFValidationPolicy2024",
				"vNICNetworkConfiguration2024",
				"vCPUAllocationPolicy2024",
				"LocationId2024ConnectorId2025",
				"Allowed application VoIPQualityOfService2024.",
			},
		},
		{
			name: "word-paths",
			inputs: []string{
				"Tag key: Network/PointToSite",
				"Tag key: DirectoryUserId/DirectoryGroupId",
				"Tag key: Networks/mDNSResponder",
				"key: Projects/2024/NetworkSwitch2024",
				`{"key":"SQLServer/AlwaysOn","value":"enabled"}`,
				`Key schema example: "<LocationId/ConnectorId>"`,
				"projects/2024/networkplanning",
				"CORP/NYC/IDF01/SW02/PORT48",
				"NYC/BLDG02/FL03/SW01/GI0/1",
			},
		},
		{
			name: "aws-access-key-id-boundary",
			inputs: []string{
				"ASIAREGIONALHEADQUARTERS",
				"ASIA2024NETWORKCHANGE",
				"NORTHASIAREGIONALHEADQUARTERS",
				"SOUTHEASTASIANETWORKOPERATIONS",
				"/locations/ASIAREGIONALHEADQUARTERS",
			},
		},
		{
			name: "vendor-prefix-documentation",
			inputs: []string{
				"See docs/gl" + "pat-token-rotation-runbook.md",
				"See docs/xoxb-token-scope-reference.md",
				"File xoxr-token-refresh-procedure.md",
				"See docs/github_pat_rotation_and_revocation.md",
				"Allow https://kb.example.net/xoxb-token-format",
			},
		},
		{
			name: "working-directories",
			inputs: []string{
				"pwd: /opt/enterprise2024",
				"pwd: /var/tmp/CHG202410060123",
				"pwd=/opt/openssh9.9",
				"pwd=/srv/project2024",
				"pwd=/var/lib/postgresql16",
				"pwd output was /tmp/incident20241006",
				`{"pwd":"/srv/project2024"}`,
			},
		},
		{
			name: "public-identifiers",
			inputs: []string{
				"Audit event ID 01ARZ3NDEKTSV4RRFFQ69G5FAV",
				"Connector deployment event 01BX5ZZKBKACTAV9WEVGEMMVRZ completed.",
				"Change event KSUID 0ujsswThIGTUYm2K8FjOOfXtY1K",
				"MongoDB asset _id 65ab12cd34ef567890ab12cd",
				"MongoDB rule history ObjectId 64ab12cd34ef567890ab12cd",
				"Git abbreviated commit cc504f83a37c9f7e96e06e521",
				"Git abbreviated revision 36e9b261f7efe3557ef346b18adc",
				"CMDB lookup https://cmdb.example.net/asset?key=" + "i-0d12e34f56a78b90c",
				`{"key":"550e8400-e29b-41d4-a716-446655440000","value":"location-reference"}`,
				`{"key":"` + `i-0d12e34f56a78b90c","value":"asset"}`,
			},
		},
		{
			name: "embedded-json-literals",
			inputs: []string{
				`Key example: "Partition key is 550e8400-e29b-41d4-a716-446655440000\nOwner: netops"`,
				`Key mapping example: "Tag key: Network/PointToSite\nOwner: netops"`,
				`Key policy sample: "key: EnableSignInForWindows\nValue: enabled"`,
				`Runbook output: "{\"pwd\":\"/srv/project2024\"}"`,
			},
		},
		{
			name: "operational-names",
			inputs: []string{
				"crowdstrikefalcon2024",
				"CROWDSTRIKEFALCON2024",
				"zscalerbranchconnectorprod2024",
				"postgresqlreportingcluster2024",
				"Deployment group zscalerbranchconnector2024 approved.",
			},
		},
		{
			name: "digit-spelled-product-names",
			inputs: []string{
				"BlockO365GuestAccess2024",
				"M365AppsMonthlyChannel",
				"AwsS3BucketPolicyReview",
				"EC2InstanceConnectAccess",
			},
		},
		{
			name: "paste-labels-with-metadata",
			inputs: []string{
				"Key fingerprint → da39a3ee5e6b4b0d3255bfef95601890afd80709",
				"| Key checksum | d41d8cd98f00b204e9800998ecf8427e |",
				"Key identifier:\n550e8400-e29b-41d4-a716-446655440000",
				"HotKey → CtrlShiftF12",
			},
		},
	}

	for _, group := range groups {
		group := group
		t.Run(group.name, func(t *testing.T) {
			t.Parallel()
			for _, input := range group.inputs {
				for _, scanner := range allStringScanners {
					got, report := scanner.scan(redact.New(redact.ModeStandard), input)
					if got != input || !report.Empty() {
						t.Errorf("%s(%q) = %q (report %v), want unchanged", scanner.name, input, got, report.Counts)
					}
				}
			}
		})
	}
}

// TestCredentialFormsNextToPreservedClassesStillRedact pairs each preserved
// class with the credential form it must not swallow. Vendor-shaped tokens
// are built by concatenation so secret scanners do not flag this file.
func TestCredentialFormsNextToPreservedClassesStillRedact(t *testing.T) {
	t.Parallel()

	const uuid = "550e8400-e29b-41d4-a716-446655440000"
	randomKey := "A7b9" + "C2d4" + "E6f8" + "G1h3" + "J5k7" + "L9m2" + "N4p6"
	base64Key := "A7b9" + "/C2d4" + "/E6f8" + "/G1h3" + "/J5k7" + "/L9m2" + "/N4p6"
	awsKey := "AKIA" + "A1B2" + "C3D4" + "E5F6" + "G7H8"
	gitlabToken := "gl" + "pat-" + "A7b9" + "C2d4" + "E6f8" + "G1h3" + "J5k7"
	slackToken := "xo" + "xb-" + "1234567890" + "-" + "A7b9C2d4E6f8G1h3"
	githubToken := "github" + "_pat_" + "11A7B9C2D0" + "_" + "A7b9C2d4E6f8G1h3J5k7"

	cases := []struct {
		name   string
		input  string
		secret string
	}{
		{"api-key-uuid", "API key: " + uuid, uuid},
		{"access-key-uuid", "access key: " + uuid, uuid},
		{"account-key-uuid", "account key: " + uuid, uuid},
		{"subscription-key-uuid", "subscription key: " + uuid, uuid},
		{"bare-key-uuid", "The temporary key is " + uuid + ", please rotate it.", uuid},
		{"poc-key-uuid", "POC key: " + uuid, uuid},
		{"primary-access-key", "Primary access key: " + randomKey, randomKey},
		{"secondary-key", "Secondary key: " + randomKey, randomKey},
		{"api-key-name-phrase", "API key for the name is " + randomKey, randomKey},
		{"key-for-portal", "Key for the vendor portal is " + randomKey, randomKey},
		{"generic-key-material", "key: " + randomKey, randomKey},
		{"base64-key-material", "key: " + base64Key, base64Key},
		{"json-key-without-pair", `{"key":"` + uuid + `"}`, uuid},
		{"json-pair-key-material", `{"key":"` + randomKey + `","value":"vendor"}`, randomKey},
		{"aws-access-key-id", "aws_access_key_id=" + awsKey, awsKey},
		{"aws-access-key-id-punctuated", "Key ID " + awsKey + ", region us-east-1", awsKey},
		{"gitlab-token", "See " + gitlabToken, gitlabToken},
		{"slack-token", "token " + slackToken, slackToken},
		{"github-token", githubToken, githubToken},
		{"pwd-password", "pwd: " + randomKey, randomKey},
		{"pwd-single-segment", "pwd: /" + randomKey, randomKey},
		{"pwd-json-drive-single-segment", `{"pwd":"C:\\` + randomKey + `"}`, randomKey},
		{"pwd-deep-path-key-segment", "pwd: /home/svc/" + randomKey, randomKey},
		{"password-after-policy-word", "Password for the policy service is " + randomKey, randomKey},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			for _, scanner := range allStringScanners {
				got, report := scanner.scan(redact.New(redact.ModeStandard), tc.input)
				if strings.Contains(got, tc.secret) || !strings.Contains(got, "<REDACTED:") || report.Empty() {
					t.Errorf("%s(%q) = %q (report %v), want credential redacted", scanner.name, tc.input, got, report.Counts)
				}
			}
		})
	}
}

// TestFreeTextKeyMaterialNextToPublicIdentifierShapesStillRedacts covers
// unlabeled key material whose shape resembles a preserved public identifier
// but lacks its naming context or exact length.
func TestFreeTextKeyMaterialNextToPublicIdentifierShapesStillRedacts(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		input  string
		secret string
	}{
		{"a8f1c3d5e7b9024ac6d8f013", "a8f1c3d5e7b9024ac6d8f013"},
		{"trial digest 7a3c9e1f5b8d2a6c4e0f7b1d9a3c5e8b2f6d0a4", "7a3c9e1f5b8d2a6c4e0f7b1d9a3c5e8b2f6d0a4"},
		{"curl --digest --user 'svc_export:yD5xR2mN8vQ4kL1dT7pB9hF3cY6aJ0e'", "yD5xR2mN8vQ4kL1dT7pB9hF3cY6aJ0e"},
		{"note Pif+9deYbews6ay/ZTw4IJrx9ijcxBVs end", "Pif+9deYbews6ay/ZTw4IJrx9ijcxBVs"},
		{"accesskey=d41d8cd98f00b204e9800998ecf8427e", "d41d8cd98f00b204e9800998ecf8427e"},
		{"event 0ujsswThIGTUYm2K8FjOOfXtY1K", "0ujsswThIGTUYm2K8FjOOfXtY1K"},
	} {
		got, report := redact.New(redact.ModeStandard).ScanFreeText(tc.input)
		if strings.Contains(got, tc.secret) || report.Empty() {
			t.Errorf("ScanFreeText(%q) = %q (report %v), want key material redacted", tc.input, got, report.Counts)
		}
	}
}
