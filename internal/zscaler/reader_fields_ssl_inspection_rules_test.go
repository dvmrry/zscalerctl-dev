package zscaler

// Wave-1 field-coverage tests for zia/ssl-inspection-rules. The mapper
// (sslInspectionRuleSourceRecord) already emitted every reference list; this
// wave promotes them in the catalog. Each test builds the SDK struct with a
// distinctive value in every promoted field plus canaries in the secret
// fields (lastModifiedBy, zpaAppSegments externalId), projects through the
// catalog, and asserts the promoted fields render under the right keys per
// mode while the canaries never survive. accessControl was promoted to
// operational (standard+share) in wave 4; its mode gating is asserted in
// TestSSLInspectionRuleAccessControlModes below.
//
// Helpers (projectOneRecord, projectOneRecordInMode, assertNoCanaries,
// mustProjectedList, mustFirstProjectedItem, assertFieldsAbsent) are reused
// from reader_test.go and reader_sourcerecord_test.go in this package.

import (
	"testing"

	ziacommon "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/common"
	"github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/sslinspection"

	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

const (
	sslWave1LastModifiedByCanary = "ssl-wave1-last-modified-by-canary"
	sslWave1ExternalIDCanary     = "ssl-wave1-zpa-external-id-canary"
)

func sslInspectionRuleWave1Fixture() sslinspection.SSLInspectionRules {
	return sslinspection.SSLInspectionRules{
		ID:          7421,
		Name:        "Inspect finance traffic",
		Description: "Decrypts finance category traffic",
		Action:      sslinspection.Action{Type: "DECRYPT"},
		State:       "ENABLED",
		// Promoted in wave 4 as a flat RBA enum (standard+share).
		AccessControl:          "READ_ONLY",
		Order:                  3,
		Rank:                   7,
		RoadWarriorForKerberos: true,
		URLCategories:          []string{"FINANCE"},
		Platforms:              []string{"WINDOWS"},
		CloudApplications:      []string{"DROPBOX"},
		UserAgentTypes:         []string{"CHROME"},
		DeviceTrustLevels:      []string{"HIGH_TRUST"},
		Locations:              []ziacommon.IDNameExtensions{{ID: 101, Name: "HQ Amsterdam"}},
		LocationGroups:         []ziacommon.IDNameExtensions{{ID: 102, Name: "EMEA branches"}},
		Groups:                 []ziacommon.IDNameExtensions{{ID: 103, Name: "Finance users"}},
		Departments:            []ziacommon.IDNameExtensions{{ID: 104, Name: "Treasury"}},
		Users:                  []ziacommon.IDNameExtensions{{ID: 105, Name: "Exception approver"}},
		DeviceGroups:           []ziacommon.IDNameExtensions{{ID: 106, Name: "Managed laptops"}},
		Devices:                []ziacommon.IDNameExtensions{{ID: 107, Name: "laptop-0042"}},
		LastModifiedTime:       1717000000,
		// Secret-classified admin identity: must never reach the projection.
		LastModifiedBy: &ziacommon.IDNameExtensions{ID: 900, Name: sslWave1LastModifiedByCanary},
		DestIpGroups:   []ziacommon.IDNameExtensions{{ID: 108, Name: "Payment processors"}},
		SourceIPGroups: []ziacommon.IDNameExtensions{{ID: 109, Name: "Branch egress"}},
		ProxyGateways:  []ziacommon.IDNameExtensions{{ID: 110, Name: "Chain gateway"}},
		Labels:         []ziacommon.IDNameExtensions{{ID: 111, Name: "pci"}},
		TimeWindows:    []ziacommon.IDNameExtensions{{ID: 112, Name: "Work hours"}},
		ZPAAppSegments: []ziacommon.ZPAAppSegments{
			// externalId is secret-classified inside zpaAppSegments.
			{ID: 113, Name: "Internal ERP", ExternalID: sslWave1ExternalIDCanary},
		},
		WorkloadGroups: []ziacommon.IDName{{ID: 114, Name: "Prod workloads"}},
		DefaultRule:    false,
		Predefined:     false,
	}
}

// sslWave1ReferenceListNames maps every promoted id/name reference list to the
// distinctive name its first entry carries in the fixture.
var sslWave1ReferenceListNames = map[string]string{
	"locations":      "HQ Amsterdam",
	"locationGroups": "EMEA branches",
	"groups":         "Finance users",
	"departments":    "Treasury",
	"users":          "Exception approver",
	"deviceGroups":   "Managed laptops",
	"devices":        "laptop-0042",
	"destIpGroups":   "Payment processors",
	"sourceIpGroups": "Branch egress",
	"proxyGateways":  "Chain gateway",
	"labels":         "pci",
	"timeWindows":    "Work hours",
	"zpaAppSegments": "Internal ERP",
	"workloadGroups": "Prod workloads",
}

func TestSSLInspectionRulePromotedFieldsProjectInStandardMode(t *testing.T) {
	t.Parallel()

	rule := sslInspectionRuleWave1Fixture()
	got := projectOneRecord(t, resources.ProductZIA, resourceSSLRules, []resources.SourceRecord{sslInspectionRuleSourceRecord(rule)})

	assertNoCanaries(t, "ssl-inspection-rules", got,
		sslWave1LastModifiedByCanary,
		sslWave1ExternalIDCanary,
	)
	assertFieldsAbsent(t, "ssl-inspection-rules", got, "lastModifiedBy")

	for field, wantName := range sslWave1ReferenceListNames {
		item := mustFirstProjectedItem(t, got, field)
		if item["name"] != wantName {
			t.Errorf("projected ssl-inspection-rules %s[0].name = %v, want %s", field, item["name"], wantName)
		}
		if _, ok := item["id"]; !ok {
			t.Errorf("projected ssl-inspection-rules %s[0] = %#v, want id", field, item)
		}
	}

	// Secret sub-fields inside promoted reference lists must be dropped.
	zpaSegment := mustFirstProjectedItem(t, got, "zpaAppSegments")
	assertFieldsAbsent(t, "ssl-inspection-rules zpaAppSegments[0]", zpaSegment, "externalId", "extensions")

	if got["roadWarriorForKerberos"] != true {
		t.Errorf("projected ssl-inspection-rules roadWarriorForKerberos = %v, want true", got["roadWarriorForKerberos"])
	}
	userAgentTypes, ok := got["userAgentTypes"].([]string)
	if !ok || len(userAgentTypes) != 1 || userAgentTypes[0] != "CHROME" {
		t.Errorf("projected ssl-inspection-rules userAgentTypes = %#v, want [CHROME]", got["userAgentTypes"])
	}
	deviceTrustLevels, ok := got["deviceTrustLevels"].([]string)
	if !ok || len(deviceTrustLevels) != 1 || deviceTrustLevels[0] != "HIGH_TRUST" {
		t.Errorf("projected ssl-inspection-rules deviceTrustLevels = %#v, want [HIGH_TRUST]", got["deviceTrustLevels"])
	}
}

func TestSSLInspectionRulePromotedFieldsAcrossModes(t *testing.T) {
	t.Parallel()

	rule := sslInspectionRuleWave1Fixture()
	records := []resources.SourceRecord{sslInspectionRuleSourceRecord(rule)}

	standardOnlyFields := []string{
		"locations", "locationGroups", "groups", "departments", "users",
		"deviceGroups", "devices", "destIpGroups", "sourceIpGroups",
		"proxyGateways", "zpaAppSegments", "workloadGroups",
	}
	standardShareFields := []string{"userAgentTypes", "deviceTrustLevels", "labels", "timeWindows"}

	share := projectOneRecordInMode(t, resources.ProductZIA, resourceSSLRules, redact.ModeShare, records)
	assertNoCanaries(t, "ssl-inspection-rules share", share,
		sslWave1LastModifiedByCanary,
		sslWave1ExternalIDCanary,
	)
	assertFieldsAbsent(t, "ssl-inspection-rules share", share, "lastModifiedBy")
	assertFieldsAbsent(t, "ssl-inspection-rules share", share, standardOnlyFields...)
	for _, field := range standardShareFields {
		if _, ok := share[field]; !ok {
			t.Errorf("projected ssl-inspection-rules share record missing %s, want present per standard+share modes", field)
		}
	}
	if share["roadWarriorForKerberos"] != true {
		t.Errorf("projected ssl-inspection-rules share roadWarriorForKerberos = %v, want true", share["roadWarriorForKerberos"])
	}
	shareLabel := mustFirstProjectedItem(t, share, "labels")
	if shareLabel["name"] != "pci" {
		t.Errorf("projected ssl-inspection-rules share labels[0].name = %v, want pci", shareLabel["name"])
	}

	paranoid := projectOneRecordInMode(t, resources.ProductZIA, resourceSSLRules, redact.ModeParanoid, records)
	assertNoCanaries(t, "ssl-inspection-rules paranoid", paranoid,
		sslWave1LastModifiedByCanary,
		sslWave1ExternalIDCanary,
	)
	assertFieldsAbsent(t, "ssl-inspection-rules paranoid", paranoid, "accessControl", "lastModifiedBy")
	assertFieldsAbsent(t, "ssl-inspection-rules paranoid", paranoid, standardOnlyFields...)
	assertFieldsAbsent(t, "ssl-inspection-rules paranoid", paranoid, standardShareFields...)
	if paranoid["roadWarriorForKerberos"] != true {
		t.Errorf("projected ssl-inspection-rules paranoid roadWarriorForKerberos = %v, want true", paranoid["roadWarriorForKerberos"])
	}
}

// TestSSLInspectionRuleAccessControlModes asserts accessControl — a flat
// admin-RBA privilege enum — is operationalField(standard+share) per the
// firewall-filtering-rules precedent.
func TestSSLInspectionRuleAccessControlModes(t *testing.T) {
	t.Parallel()

	rule := sslinspection.SSLInspectionRules{
		ID:            4409,
		Name:          "wave4 ssl rule access control",
		State:         "ENABLED",
		AccessControl: "READ_WRITE",
	}
	records := []resources.SourceRecord{sslInspectionRuleSourceRecord(rule)}

	standard := projectOneRecord(t, resources.ProductZIA, resourceSSLRules, records)
	if standard["accessControl"] != "READ_WRITE" {
		t.Errorf("projected ssl-inspection-rules accessControl = %v, want READ_WRITE", standard["accessControl"])
	}

	// operationalField(standard+share): present in share, dropped in paranoid.
	share := projectOneRecordInMode(t, resources.ProductZIA, resourceSSLRules, redact.ModeShare, records)
	if share["accessControl"] != "READ_WRITE" {
		t.Errorf("projected ssl-inspection-rules share accessControl = %v, want READ_WRITE", share["accessControl"])
	}
	paranoid := projectOneRecordInMode(t, resources.ProductZIA, resourceSSLRules, redact.ModeParanoid, records)
	assertFieldsAbsent(t, "ssl-inspection-rules (paranoid)", paranoid, "accessControl")
}

func TestSSLInspectionRuleActionDetailsProjectAcrossModes(t *testing.T) {
	t.Parallel()

	const certName = "Fleet rollout cert 2026"
	rule := sslinspection.SSLInspectionRules{
		ID:    8842,
		Name:  "ssl action detail rule",
		State: "ENABLED",
		Action: sslinspection.Action{
			Type:                       "DECRYPT",
			ShowEUN:                    true,
			ShowEUNATP:                 true,
			OverrideDefaultCertificate: true,
			SSLInterceptionCert: &sslinspection.SSLInterceptionCert{
				ID:                 77,
				Name:               certName,
				DefaultCertificate: true,
			},
			DecryptSubActions: &sslinspection.DecryptSubActions{
				ServerCertificates:              "PASS_THRU",
				OcspCheck:                       true,
				BlockSslTrafficWithNoSniEnabled: true,
				MinClientTLSVersion:             "TLS_1_2",
				MinServerTLSVersion:             "TLS_1_3",
				BlockUndecrypt:                  true,
				HTTP2Enabled:                    true,
			},
			DoNotDecryptSubActions: &sslinspection.DoNotDecryptSubActions{
				BypassOtherPolicies:             true,
				ServerCertificates:              "BLOCK",
				OcspCheck:                       true,
				BlockSslTrafficWithNoSniEnabled: true,
				MinTLSVersion:                   "TLS_1_2",
			},
		},
	}
	records := []resources.SourceRecord{sslInspectionRuleSourceRecord(rule)}

	standard := projectOneRecord(t, resources.ProductZIA, resourceSSLRules, records)
	assertSSLInspectionActionDetails(t, "standard", standard, certName)

	share := projectOneRecordInMode(t, resources.ProductZIA, resourceSSLRules, redact.ModeShare, records)
	assertSSLInspectionActionDetails(t, "share", share, certName)

	paranoid := projectOneRecordInMode(t, resources.ProductZIA, resourceSSLRules, redact.ModeParanoid, records)
	action := sslInspectionRuleAction(t, "paranoid", paranoid)
	if action["type"] != "DECRYPT" {
		t.Errorf("projected ssl-inspection-rules paranoid action.type = %v, want DECRYPT", action["type"])
	}
	assertFieldsAbsent(t, "ssl-inspection-rules paranoid action", action,
		"showEUN",
		"showEUNATP",
		"overrideDefaultCertificate",
		"sslInterceptionCert",
		"decryptSubActions",
		"doNotDecryptSubActions",
	)
}

func assertSSLInspectionActionDetails(
	t *testing.T,
	mode string,
	record map[string]any,
	wantCertName string,
) {
	t.Helper()

	action := sslInspectionRuleAction(t, mode, record)
	if action["type"] != "DECRYPT" {
		t.Errorf("projected ssl-inspection-rules %s action.type = %v, want DECRYPT", mode, action["type"])
	}
	for _, field := range []string{"showEUN", "showEUNATP", "overrideDefaultCertificate"} {
		if action[field] != true {
			t.Errorf("projected ssl-inspection-rules %s action.%s = %v, want true", mode, field, action[field])
		}
	}

	cert, ok := action["sslInterceptionCert"].(map[string]any)
	if !ok {
		t.Fatalf("projected ssl-inspection-rules %s action.sslInterceptionCert = %T, want map[string]any", mode, action["sslInterceptionCert"])
	}
	if cert["name"] != wantCertName {
		t.Errorf("projected ssl-inspection-rules %s action.sslInterceptionCert.name = %v, want %s", mode, cert["name"], wantCertName)
	}
	if cert["id"] != 77 {
		t.Errorf("projected ssl-inspection-rules %s action.sslInterceptionCert.id = %v, want 77", mode, cert["id"])
	}
	if cert["defaultCertificate"] != true {
		t.Errorf("projected ssl-inspection-rules %s action.sslInterceptionCert.defaultCertificate = %v, want true", mode, cert["defaultCertificate"])
	}

	decryptSubActions, ok := action["decryptSubActions"].(map[string]any)
	if !ok {
		t.Fatalf("projected ssl-inspection-rules %s action.decryptSubActions = %T, want map[string]any", mode, action["decryptSubActions"])
	}
	wantDecrypt := map[string]any{
		"serverCertificates":              "PASS_THRU",
		"ocspCheck":                       true,
		"blockSslTrafficWithNoSniEnabled": true,
		"minClientTLSVersion":             "TLS_1_2",
		"minServerTLSVersion":             "TLS_1_3",
		"blockUndecrypt":                  true,
		"http2Enabled":                    true,
	}
	for field, want := range wantDecrypt {
		if decryptSubActions[field] != want {
			t.Errorf("projected ssl-inspection-rules %s action.decryptSubActions.%s = %v, want %v", mode, field, decryptSubActions[field], want)
		}
	}

	doNotDecryptSubActions, ok := action["doNotDecryptSubActions"].(map[string]any)
	if !ok {
		t.Fatalf("projected ssl-inspection-rules %s action.doNotDecryptSubActions = %T, want map[string]any", mode, action["doNotDecryptSubActions"])
	}
	wantDoNotDecrypt := map[string]any{
		"bypassOtherPolicies":             true,
		"serverCertificates":              "BLOCK",
		"ocspCheck":                       true,
		"blockSslTrafficWithNoSniEnabled": true,
		"minTLSVersion":                   "TLS_1_2",
	}
	for field, want := range wantDoNotDecrypt {
		if doNotDecryptSubActions[field] != want {
			t.Errorf("projected ssl-inspection-rules %s action.doNotDecryptSubActions.%s = %v, want %v", mode, field, doNotDecryptSubActions[field], want)
		}
	}
}

func sslInspectionRuleAction(t *testing.T, mode string, record map[string]any) map[string]any {
	t.Helper()

	action, ok := record["action"].(map[string]any)
	if !ok {
		t.Fatalf("projected ssl-inspection-rules %s action = %T, want map[string]any", mode, record["action"])
	}
	return action
}
