package redact_test

import (
	"strings"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/redact"
)

// TestOrdinaryAdminTextIsPreserved covers administrator text that main leaves
// unchanged and that earlier versions of the paste, label and short-token
// rules replaced: rule and host names, password policy and status text,
// identifiers named by an object noun, data-key names, and command
// documentation with placeholders.
func TestOrdinaryAdminTextIsPreserved(t *testing.T) {
	t.Parallel()

	uuid := "550e8400-" + "e29b-41d4-a716-446655440000"
	for _, input := range []string{
		"Rule name: EnableQoSForMicrosoftTeams2026",
		"Connector host nyc01zscalerconnectorprod02 is in the DMZ.",
		"Firewall group: VMwareNSXvFirewallProd2026",
		"Rule name: Enable8021XOnBranchPorts2026",
		"Policy name: AllowGoToMyPCRemoteAccess2026",
		"Database host usw2postgresqlclusterprod01 is scheduled for maintenance.",
		"Deployment group: 2026q3crowdstrikefalconprod",
		"Affected JavaScript bundle: app.652f8a1b" + "9c7d4e30a56b2f90.js",
		"Password is required!",
		"Password is case-sensitive!",
		"Password (status): Disabled!",
		"Password (last changed): 20261009",
		"Password (reset procedure): ResetGuide2026.pdf",
		"Passphrase (error message): Please Try Again!",
		"| Control | Review date |\n| Password | 20261009 |",
		"Token scope: " + uuid,
		"Token tid claim: " + uuid,
		"Key Vault tenant: " + uuid,
		"Password reset event: " + uuid,
		"Configuration key: EnableQoSForMicrosoftTeams",
		"Routing key: HQ01FW02WAN1",
		"Partition key: " + uuid,
		`sqlcmd -S sql.example.net -U !SQL_USER! -P "!SQL_PASS!" -Q "SELECT 1"`,
		"mysqldump -u auditor -p 'inventory2026' > inventory.sql",
		"mysql -u root -p inventory",
		"mysql -u svc -p$MYSQL_PWD inventory",
		"Routing key: HQ01FW02WAN1",
		"Setting key: EnableWoLForBranchPC",
		"Image key: " + "WindowsServer2022Datacenter21H2",
		"CMDB location key: " + "NYC01/IDF02/SW03/PORT48",
		"Rack inventory key: " + "nyc1-sw02-eth3-dmz4",
		"Routing key: " + uuid,
		"To check mysql schema changes, run git diff -p 'schema2026.sql'.",
		"sshpass -p should be avoided in unattended jobs.",
		`net use supports /user:DOMAIN\user for selecting an account.`,
		"The cmdkey syntax is /pass:PASSWORD; the word PASSWORD is a placeholder.",
		"curl output normalization uses sed -" + "u 's:active:ready:g' response.txt.",
		`curl -` + `u "!API_USER!:!API_PASS!" https://api.example.net/health`,
		"For password imports, ConvertTo-SecureString 'encrypted string' reads serialized input.",
		"PSCredential('user name', 'SecureString object') requires a SecureString.",
	} {
		for _, scanner := range allStringScanners {
			if got, report := scanner.scan(redact.New(redact.ModeStandard), input); got != input || !report.Empty() {
				t.Errorf("%s(%q) = %q (report %v), want unchanged", scanner.name, input, got, report.Counts)
			}
		}
	}
}

// TestCredentialsNextToPreservedAdminTextStillRedact pairs the preserved
// forms above with real credentials in the same positions.
func TestCredentialsNextToPreservedAdminTextStillRedact(t *testing.T) {
	t.Parallel()

	password := "Zx9!" + "kq2mLp"
	weak := "Winter" + "2026"
	material := "A7b9C2d4" + "E6f8G1h3" + "J5k7L9m2" + "N4p6Q8r1"
	for _, tc := range []struct{ input, secret string }{
		{"Password reset to: " + weak, weak},
		{"Password (prod): " + password, password},
		{"Password (admin): " + weak, weak},
		{"| Password | " + password + " |", password},
		{"Token scope: " + material, material},
		{"Function key: " + material, material},
		{"Policy key: " + material, material},
		{"Configuration key: " + material, material},
		{"Routing key: " + material, material},
		{"api key: " + "CorrectHorse" + "BatteryStaple2026", "CorrectHorse" + "BatteryStaple2026"},
		{"API key: " + "zscalerconnector" + "ProdKey9Xq2", "zscalerconnector" + "ProdKey9Xq2"},
		{"mysql -u root -p" + password + " inventory", password},
		{"mysqldump -u svc -p" + weak + " inventory > dump.sql", weak},
		{"sshpass -p " + weak + " ssh admin@host", weak},
		{"mysql -u svc -p'" + password + "' inventory", password},
		{`sqlcmd -S sql01 -U svc -P "` + password + `" -Q "SELECT 1"`, password},
		{"curl -" + "u 'svc:" + password + "' https://api.example.net/health", password},
		{"cmdkey /add:fs01 /user:svc /pass:" + password, password},
		{"net use Z: \\\\fs01\\share " + password + " /user:CORP\\svc", password},
		{"$pw = ConvertTo-SecureString '" + password + "' -AsPlainText -Force", password},
		{"New-Object System.Net.NetworkCredential('svc', '" + password + "')", password},
	} {
		for _, scanner := range allStringScanners {
			if got, _ := scanner.scan(redact.New(redact.ModeStandard), tc.input); strings.Contains(got, tc.secret) {
				t.Errorf("%s(%q) = %q, leaks the credential", scanner.name, tc.input, got)
			}
		}
	}
}
