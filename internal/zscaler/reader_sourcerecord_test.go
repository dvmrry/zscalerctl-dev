package zscaler

// Source-record conversion tests for ZIA resources whose mapping functions had
// no direct coverage. Each test builds the SDK struct with a distinctive canary
// in every secret-classified field, converts it with the source-record function
// under test, projects it through the resources catalog in standard mode, and
// asserts that (a) allow-listed fields appear under the right keys, (b) secret
// fields are absent from the projection, and (c) no secret canary survives
// anywhere in the rendered record.
//
// These tests live in their own file (not reader_test.go) because an open PR
// owns reader_test.go; they reuse its helpers (projectOneRecord,
// assertNoCanaries, mustProjectedList) from the same package.

import (
	"testing"

	c2cincidentreceiver "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/c2c_incident_receiver"
	cloudappcontrol "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/cloudappcontrol"
	riskprofiles "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/cloudapplications/risk_profiles"
	cloudnss "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/cloudnss/cloudnss"
	nssservers "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/cloudnss/nss_servers"
	ziacommon "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/common"
	dlpengines "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/dlp/dlp_engines"
	dlpexactdatamatch "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/dlp/dlp_exact_data_match"
	dlpidmprofilelite "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/dlp/dlp_idm_profile_lite"
	dlpidmprofiles "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/dlp/dlp_idm_profiles"
	dlpdictionaries "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/dlp/dlpdictionaries"
	filetypecontrol "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/filetypecontrol"
	customfiletypes "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/filetypecontrol/custom_file_types"
	firewalldnscontrolpolicies "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/firewalldnscontrolpolicies"
	zpagateways "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/forwarding_control_policy/zpa_gateways"
	ipssignaturerules "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/ips_control_policies/ips_signature_rules"
	sandboxrules "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/sandbox/sandbox_rules"

	"github.com/dvmrry/zscalerctl/internal/resources"
)

// assertFieldsAbsent fails if any of the named keys survived projection. Use it
// for catalog secret-classified fields, which must be dropped in every mode.
func assertFieldsAbsent(t *testing.T, resource string, record map[string]any, fields ...string) {
	t.Helper()

	for _, field := range fields {
		if _, ok := record[field]; ok {
			t.Errorf("projected %s = %#v, want no %s", resource, record, field)
		}
	}
}

func mustProjectedMap(t *testing.T, record map[string]any, field string) map[string]any {
	t.Helper()

	value, ok := record[field]
	if !ok {
		t.Fatalf("projected record missing %s, want map", field)
	}
	object, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("projected record %s = %T, want map[string]any", field, value)
	}
	return object
}

func mustFirstProjectedItem(t *testing.T, record map[string]any, field string) map[string]any {
	t.Helper()

	items := mustProjectedList(t, record, field)
	object, ok := items[0].(map[string]any)
	if !ok {
		t.Fatalf("projected record %s[0] = %T, want map[string]any", field, items[0])
	}
	return object
}

func TestDLPEngineSourceRecordProjectsThroughAllowList(t *testing.T) {
	t.Parallel()

	const canary = "dlp-engine-psk-canary"
	engine := dlpengines.DLPEngines{
		ID:                   601,
		Name:                 "Custom engine",
		Description:          "temporary psk=" + canary,
		PredefinedEngineName: "EXTERNAL",
		EngineExpression:     "((D63.S > 1))",
		CustomDlpEngine:      true,
	}

	got := projectOneRecord(t, resources.ProductZIA, resourceDLPEngines, []resources.SourceRecord{dlpEngineSourceRecord(engine)})
	assertNoCanaries(t, "dlp-engines", got, canary)
	if got["id"] != 601 {
		t.Errorf("projected dlp-engines id = %v, want 601", got["id"])
	}
	if got["name"] != "Custom engine" {
		t.Errorf("projected dlp-engines name = %v, want Custom engine", got["name"])
	}
	if got["predefinedEngineName"] != "EXTERNAL" {
		t.Errorf("projected dlp-engines predefinedEngineName = %v, want EXTERNAL", got["predefinedEngineName"])
	}
	if got["engineExpression"] != "((D63.S > 1))" {
		t.Errorf("projected dlp-engines engineExpression = %v, want ((D63.S > 1))", got["engineExpression"])
	}
	if got["customDlpEngine"] != true {
		t.Errorf("projected dlp-engines customDlpEngine = %v, want true", got["customDlpEngine"])
	}
}

func TestDLPDictionarySourceRecordDropsDetectionContent(t *testing.T) {
	t.Parallel()

	const canary = "dlp-dictionary-psk-canary"
	dictionary := dlpdictionaries.DlpDictionary{
		ID:                    602,
		Name:                  "Custom dictionary",
		Description:           "temporary psk=" + canary,
		ConfidenceThreshold:   "CONFIDENCE_LEVEL_HIGH",
		CustomPhraseMatchType: "MATCH_ALL_CUSTOM_PHRASE_PATTERN_DICTIONARY",
		NameL10nTag:           true,
		Custom:                true,
		ThresholdType:         "VIOLATION_COUNT",
		DictionaryType:        "PATTERNS_AND_PHRASES",
		Proximity:             40,
		Phrases: []dlpdictionaries.Phrases{
			{Action: "PHRASE_COUNT_TYPE_ALL", Phrase: canary},
		},
		Patterns: []dlpdictionaries.Patterns{
			{Action: "PATTERN_COUNT_TYPE_ALL", Pattern: canary},
		},
		EDMMatchDetails: []dlpdictionaries.EDMMatchDetails{
			{DictionaryEdmMappingID: 21, SchemaID: 22},
		},
		IDMProfileMatchAccuracy: []dlpdictionaries.IDMProfileMatchAccuracy{
			{MatchAccuracy: canary},
		},
		HierarchicalIdentifiers:          []string{canary},
		PredefinedPhrases:                []string{canary},
		BinNumbers:                       []int{424242},
		DictTemplateId:                   23,
		ConfidenceLevelForPredefinedDict: "CONFIDENCE_LEVEL_LOW",
	}

	got := projectOneRecord(t, resources.ProductZIA, resourceDLPDictionaries, []resources.SourceRecord{dlpDictionarySourceRecord(dictionary)})
	assertNoCanaries(t, "dlp-dictionaries", got, canary)
	assertFieldsAbsent(t, "dlp-dictionaries", got,
		"phrases",
		"patterns",
		"exactDataMatchDetails",
		"idmProfileMatchAccuracyDetails",
		"binNumbers",
		"hierarchicalIdentifiers",
		"predefinedPhrases",
	)
	if got["name"] != "Custom dictionary" {
		t.Errorf("projected dlp-dictionaries name = %v, want Custom dictionary", got["name"])
	}
	if got["dictionaryType"] != "PATTERNS_AND_PHRASES" {
		t.Errorf("projected dlp-dictionaries dictionaryType = %v, want PATTERNS_AND_PHRASES", got["dictionaryType"])
	}
	if got["proximity"] != 40 {
		t.Errorf("projected dlp-dictionaries proximity = %v, want 40", got["proximity"])
	}
	if got["custom"] != true {
		t.Errorf("projected dlp-dictionaries custom = %v, want true", got["custom"])
	}
}

func TestDLPEDMSchemaSourceRecordDropsScheduleAndAdmins(t *testing.T) {
	t.Parallel()

	const canary = "dlp-edm-schema-psk-canary"
	schema := dlpexactdatamatch.DLPEDMSchema{
		SchemaID: 603,
		EDMClient: &ziacommon.IDNameExtensions{
			ID:         31,
			Name:       "EDM client",
			Extensions: map[string]any{"token": canary},
		},
		ProjectName:      "EDM project",
		Revision:         4,
		Filename:         "edm-schema",
		OriginalFileName: "edm-schema-original",
		FileUploadStatus: "EDM_FILE_UPLOAD_COMPLETED",
		SchemaStatus:     "EDM_PROD_OK",
		OrigColCount:     7,
		LastModifiedTime: 1700000100,
		ModifiedBy:       &ziacommon.IDNameExtensions{ID: 32, Name: canary},
		CreatedBy:        &ziacommon.IDNameExtensions{ID: 33, Name: canary},
		CellsUsed:        11,
		SchemaActive:     true,
		SchedulePresent:  true,
		TokenList: []dlpexactdatamatch.TokenList{
			{Name: canary, Type: "EDM_TOKEN"},
		},
		Schedule: dlpexactdatamatch.Schedule{ScheduleType: canary},
	}

	got := projectOneRecord(t, resources.ProductZIA, resourceDLPEDMSchemas, []resources.SourceRecord{dlpEDMSchemaSourceRecord(schema)})
	assertNoCanaries(t, "dlp-edm-schemas", got, canary)
	assertFieldsAbsent(t, "dlp-edm-schemas", got, "modifiedBy", "createdBy", "tokenList", "schedule")
	if got["schemaId"] != 603 {
		t.Errorf("projected dlp-edm-schemas schemaId = %v, want 603", got["schemaId"])
	}
	if got["projectName"] != "EDM project" {
		t.Errorf("projected dlp-edm-schemas projectName = %v, want EDM project", got["projectName"])
	}
	if got["schemaActive"] != true {
		t.Errorf("projected dlp-edm-schemas schemaActive = %v, want true", got["schemaActive"])
	}
	edmClient := mustProjectedMap(t, got, "edmClient")
	if edmClient["name"] != "EDM client" || edmClient["id"] != 31 {
		t.Errorf("projected dlp-edm-schemas edmClient = %#v, want id 31 name EDM client", edmClient)
	}
	if _, ok := edmClient["extensions"]; ok {
		t.Errorf("projected dlp-edm-schemas edmClient = %#v, want no extensions", edmClient)
	}
}

func TestDLPIDMProfileLiteSourceRecordCoversBothPointerArms(t *testing.T) {
	t.Parallel()

	const canary = "dlp-idm-profile-lite-psk-canary"
	profile := dlpidmprofilelite.DLPIDMProfileLite{
		ProfileID:        604,
		TemplateName:     "IDM template",
		ClientVM:         &ziacommon.IDNameExtensions{ID: 41, Name: "Index Tool VM"},
		NumDocuments:     12,
		LastModifiedTime: 1700000200,
		ModifiedBy:       &ziacommon.IDNameExtensions{ID: 42, Name: canary},
	}

	got := projectOneRecord(t, resources.ProductZIA, resourceDLPIDMLite, []resources.SourceRecord{dlpIDMProfileLiteSourceRecord(profile)})
	assertNoCanaries(t, "dlp-idm-profile-lite", got, canary)
	assertFieldsAbsent(t, "dlp-idm-profile-lite", got, "modifiedBy")
	if got["profileId"] != 604 {
		t.Errorf("projected dlp-idm-profile-lite profileId = %v, want 604", got["profileId"])
	}
	if got["templateName"] != "IDM template" {
		t.Errorf("projected dlp-idm-profile-lite templateName = %v, want IDM template", got["templateName"])
	}
	clientVM := mustProjectedMap(t, got, "clientVm")
	if clientVM["name"] != "Index Tool VM" {
		t.Errorf("projected dlp-idm-profile-lite clientVm = %#v, want name Index Tool VM", clientVM)
	}

	// Nil-pointer arm: optional references stay absent instead of rendering as
	// empty objects.
	minimal := dlpidmprofilelite.DLPIDMProfileLite{ProfileID: 605}
	gotMinimal := projectOneRecord(t, resources.ProductZIA, resourceDLPIDMLite, []resources.SourceRecord{dlpIDMProfileLiteSourceRecord(minimal)})
	assertFieldsAbsent(t, "dlp-idm-profile-lite", gotMinimal, "clientVm", "modifiedBy")
}

func TestDLPIDMProfileSourceRecordProjectsThroughAllowList(t *testing.T) {
	t.Parallel()

	const canary = "dlp-idm-profile-psk-canary"
	profile := dlpidmprofiles.DLPIDMProfile{
		ProfileID:          606,
		ProfileName:        "IDM profile",
		ProfileDesc:        "temporary psk=" + canary,
		ProfileType:        "LOCAL",
		Host:               "idm.internal.test",
		Port:               8443,
		ProfileDirPath:     "/data/idm",
		ScheduleType:       "MONTHLY",
		ScheduleDay:        3,
		ScheduleDayOfMonth: []string{"3"},
		ScheduleDayOfWeek:  []string{"MON"},
		ScheduleTime:       180,
		ScheduleDisabled:   true,
		UploadStatus:       "IDM_PROF_OK",
		UserName:           "svc-idm-index",
		Version:            2,
		IDMClient:          &ziacommon.IDNameExtensions{ID: 51, Name: "Index Tool"},
		VolumeOfDocuments:  9,
		NumDocuments:       13,
		LastModifiedTime:   1700000300,
		ModifiedBy:         &ziacommon.IDNameExtensions{ID: 52, Name: canary},
	}

	got := projectOneRecord(t, resources.ProductZIA, resourceDLPIDMProfiles, []resources.SourceRecord{dlpIDMProfileSourceRecord(profile)})
	assertNoCanaries(t, "dlp-idm-profiles", got, canary)
	assertFieldsAbsent(t, "dlp-idm-profiles", got, "modifiedBy")
	if got["profileName"] != "IDM profile" {
		t.Errorf("projected dlp-idm-profiles profileName = %v, want IDM profile", got["profileName"])
	}
	if got["host"] != "idm.internal.test" {
		t.Errorf("projected dlp-idm-profiles host = %v, want idm.internal.test", got["host"])
	}
	if got["port"] != 8443 {
		t.Errorf("projected dlp-idm-profiles port = %v, want 8443", got["port"])
	}
	if got["userName"] != "svc-idm-index" {
		t.Errorf("projected dlp-idm-profiles userName = %v, want svc-idm-index", got["userName"])
	}
	if got["uploadStatus"] != "IDM_PROF_OK" {
		t.Errorf("projected dlp-idm-profiles uploadStatus = %v, want IDM_PROF_OK", got["uploadStatus"])
	}
	idmClient := mustProjectedMap(t, got, "idmClient")
	if idmClient["name"] != "Index Tool" {
		t.Errorf("projected dlp-idm-profiles idmClient = %#v, want name Index Tool", idmClient)
	}
}

func TestRiskProfileSourceRecordDropsRiskAttributes(t *testing.T) {
	t.Parallel()

	const canary = "risk-profile-psk-canary"
	profile := riskprofiles.RiskProfiles{
		ID:                        607,
		ProfileName:               "Risk profile",
		ProfileType:               "CLOUD_APPLICATIONS",
		Status:                    "SANCTIONED",
		ExcludeCertificates:       4242,
		PoorItemsOfService:        canary,
		AdminAuditLogs:            canary,
		DataBreach:                canary,
		SourceIpRestrictions:      "YES",
		MfaSupport:                canary,
		SslPinned:                 canary,
		HttpSecurityHeaders:       canary,
		Evasive:                   canary,
		DnsCaaPolicy:              canary,
		WeakCipherSupport:         canary,
		PasswordStrength:          canary,
		SslCertValidity:           canary,
		Vulnerability:             canary,
		MalwareScanningForContent: canary,
		FileSharing:               canary,
		SslCertKeySize:            canary,
		VulnerableToHeartBleed:    canary,
		VulnerableToLogJam:        canary,
		VulnerableToPoodle:        canary,
		VulnerabilityDisclosure:   canary,
		SupportForWaf:             canary,
		RemoteScreenSharing:       canary,
		SenderPolicyFramework:     canary,
		DomainKeysIdentifiedMail:  canary,
		DomainBasedMessageAuth:    canary,
		LastModTime:               1700000400,
		CreateTime:                1700000500,
		Certifications:            []string{canary},
		DataEncryptionInTransit:   []string{canary},
		RiskIndex:                 []int{4242},
		ModifiedBy:                &ziacommon.IDNameExtensions{ID: 61, Name: "Risk admin"},
		CustomTags:                []ziacommon.IDNameExternalID{{ID: 62, Name: "tag-context", ExternalID: canary}},
	}

	got := projectOneRecord(t, resources.ProductZIA, resourceRiskProfiles, []resources.SourceRecord{riskProfileSourceRecord(profile)})
	assertNoCanaries(t, "risk-profiles", got, canary)
	assertFieldsAbsent(t, "risk-profiles", got,
		"adminAuditLogs", "certifications", "dataBreach", "dataEncryptionInTransit",
		"dnsCaaPolicy", "domainBasedMessageAuth", "domainKeysIdentifiedMail", "evasive",
		"excludeCertificates", "fileSharing", "httpSecurityHeaders", "malwareScanningForContent",
		"mfaSupport", "passwordStrength", "poorItemsOfService", "remoteScreenSharing",
		"riskIndex", "senderPolicyFramework", "sslCertKeySize", "sslCertValidity",
		"sslPinned", "supportForWaf", "vulnerability", "vulnerabilityDisclosure",
		"vulnerableToHeartBleed", "vulnerableToLogJam", "vulnerableToPoodle", "weakCipherSupport",
	)
	if got["profileName"] != "Risk profile" {
		t.Errorf("projected risk-profiles profileName = %v, want Risk profile", got["profileName"])
	}
	if got["status"] != "SANCTIONED" {
		t.Errorf("projected risk-profiles status = %v, want SANCTIONED", got["status"])
	}
	if got["sourceIpRestrictions"] != "YES" {
		t.Errorf("projected risk-profiles sourceIpRestrictions = %v, want YES", got["sourceIpRestrictions"])
	}
	modifiedBy := mustProjectedMap(t, got, "modifiedBy")
	if modifiedBy["name"] != "Risk admin" {
		t.Errorf("projected risk-profiles modifiedBy = %#v, want name Risk admin", modifiedBy)
	}
	customTag := mustFirstProjectedItem(t, got, "customTags")
	if customTag["name"] != "tag-context" {
		t.Errorf("projected risk-profiles customTags[0] = %#v, want name tag-context", customTag)
	}
	if _, ok := customTag["externalId"]; ok {
		t.Errorf("projected risk-profiles customTags[0] = %#v, want no externalId", customTag)
	}
}

func TestNSSServerSourceRecordProjectsThroughAllowList(t *testing.T) {
	t.Parallel()

	const canary = "nss-server-psk-canary"
	server := nssservers.NSSServers{
		ID:        608,
		Name:      "NSS server psk=" + canary,
		Status:    "ENABLED",
		State:     "HEALTHY",
		Type:      "NSS_FOR_WEB",
		IcapSvrId: 71,
	}

	got := projectOneRecord(t, resources.ProductZIA, resourceNSSServers, []resources.SourceRecord{nssServerSourceRecord(server)})
	assertNoCanaries(t, "nss-servers", got, canary)
	if got["id"] != 608 {
		t.Errorf("projected nss-servers id = %v, want 608", got["id"])
	}
	if got["status"] != "ENABLED" {
		t.Errorf("projected nss-servers status = %v, want ENABLED", got["status"])
	}
	if got["state"] != "HEALTHY" {
		t.Errorf("projected nss-servers state = %v, want HEALTHY", got["state"])
	}
	if got["type"] != "NSS_FOR_WEB" {
		t.Errorf("projected nss-servers type = %v, want NSS_FOR_WEB", got["type"])
	}
	if got["icapSvrId"] != 71 {
		t.Errorf("projected nss-servers icapSvrId = %v, want 71", got["icapSvrId"])
	}
}

func TestNSSFeedSourceRecordDropsCredentials(t *testing.T) {
	t.Parallel()

	const canary = "nss-feed-psk-canary"
	feed := cloudnss.NSSFeed{
		ID:                       609,
		Name:                     "SIEM feed",
		FeedStatus:               "ENABLED",
		NssLogType:               "WEBLOG",
		NssFeedType:              "JSON",
		FeedOutputFormat:         "%j{reason}",
		UserObfuscation:          "DISABLED",
		TimeZone:                 "UTC",
		EpsRateLimit:             100,
		JsonArrayToggle:          true,
		SiemType:                 "SPLUNK",
		MaxBatchSize:             512,
		ConnectionURL:            "https://siem.internal.test/services/collector",
		AuthenticationToken:      canary,
		ConnectionHeaders:        []string{"X-Auth: " + canary},
		Base64EncodedCertificate: canary,
		NssType:                  "NSS_FOR_WEB",
		ClientID:                 canary,
		ClientSecret:             canary,
		AuthenticationUrl:        "https://login.internal.test/oauth2/token",
		GrantType:                "CLIENT_CREDENTIALS",
		Scope:                    canary,
		CloudNSS:                 true,
		OauthAuthentication:      true,
		ServerIps:                []string{"203.0.113.10"},
		Domains:                  []string{"internal.test"},
		Users:                    []ziacommon.CommonNSS{{ID: 81, Name: canary}},
		Locations:                []ziacommon.CommonNSS{{ID: 82, Name: canary}},
	}

	got := projectOneRecord(t, resources.ProductZIA, resourceNSSFeeds, []resources.SourceRecord{nssFeedSourceRecord(feed)})
	assertNoCanaries(t, "nss-feeds", got, canary)
	assertFieldsAbsent(t, "nss-feeds", got,
		"authenticationToken", "clientId", "clientSecret", "scope",
		"base64EncodedCertificate", "connectionHeaders", "users", "locations",
	)
	if got["name"] != "SIEM feed" {
		t.Errorf("projected nss-feeds name = %v, want SIEM feed", got["name"])
	}
	if got["siemType"] != "SPLUNK" {
		t.Errorf("projected nss-feeds siemType = %v, want SPLUNK", got["siemType"])
	}
	if got["connectionURL"] != "https://siem.internal.test/services/collector" {
		t.Errorf("projected nss-feeds connectionURL = %v, want collector URL", got["connectionURL"])
	}
	if got["grantType"] != "CLIENT_CREDENTIALS" {
		t.Errorf("projected nss-feeds grantType = %v, want CLIENT_CREDENTIALS", got["grantType"])
	}
	if got["oauthAuthentication"] != true {
		t.Errorf("projected nss-feeds oauthAuthentication = %v, want true", got["oauthAuthentication"])
	}
}

func TestFileTypeRuleSourceRecordProjectsThroughAllowList(t *testing.T) {
	t.Parallel()

	const canary = "file-type-rule-psk-canary"
	rule := filetypecontrol.FileTypeRules{
		ID:                   610,
		Name:                 "File type rule",
		Description:          "temporary psk=" + canary,
		State:                "ENABLED",
		Order:                1,
		FilteringAction:      "BLOCK",
		TimeQuota:            15,
		SizeQuota:            100,
		AccessControl:        "READ_ONLY",
		Rank:                 7,
		CapturePCAP:          true,
		PasswordProtected:    true,
		Operation:            "DOWNLOAD",
		ActiveContent:        true,
		Unscannable:          false,
		BrowserEunTemplateID: 2,
		CloudApplications:    []string{"DROPBOX"},
		FileTypes:            []string{"FTCATEGORY_MS_WORD"},
		MinSize:              10,
		MaxSize:              100,
		Protocols:            []string{"ANY_RULE"},
		URLCategories:        []string{"FINANCE"},
		DeviceTrustLevels:    []string{"HIGH_TRUST"},
		LastModifiedTime:     1700000600,
		LastModifiedBy:       &ziacommon.IDNameExtensions{ID: 91, Name: canary},
		Locations:            []ziacommon.IDNameExtensions{{ID: 92, Name: "HQ"}},
		ZPAAppSegments:       []ziacommon.ZPAAppSegments{{ID: 93, Name: "Segment", ExternalID: canary}},
	}

	got := projectOneRecord(t, resources.ProductZIA, resourceFileTypeRules, []resources.SourceRecord{fileTypeRuleSourceRecord(rule)})
	assertNoCanaries(t, "file-type-rules", got, canary)
	assertFieldsAbsent(t, "file-type-rules", got, "passwordProtected", "lastModifiedBy")
	if got["name"] != "File type rule" {
		t.Errorf("projected file-type-rules name = %v, want File type rule", got["name"])
	}
	if got["filteringAction"] != "BLOCK" {
		t.Errorf("projected file-type-rules filteringAction = %v, want BLOCK", got["filteringAction"])
	}
	if got["operation"] != "DOWNLOAD" {
		t.Errorf("projected file-type-rules operation = %v, want DOWNLOAD", got["operation"])
	}
	location := mustFirstProjectedItem(t, got, "locations")
	if location["name"] != "HQ" {
		t.Errorf("projected file-type-rules locations[0] = %#v, want name HQ", location)
	}
	segment := mustFirstProjectedItem(t, got, "zpaAppSegments")
	if segment["name"] != "Segment" {
		t.Errorf("projected file-type-rules zpaAppSegments[0] = %#v, want name Segment", segment)
	}
	if _, ok := segment["externalId"]; ok {
		t.Errorf("projected file-type-rules zpaAppSegments[0] = %#v, want no externalId", segment)
	}
}

func TestSandboxRuleSourceRecordProjectsThroughAllowList(t *testing.T) {
	t.Parallel()

	const canary = "sandbox-rule-psk-canary"
	rule := sandboxrules.SandboxRules{
		ID:                 611,
		Name:               "Sandbox rule",
		Description:        "temporary psk=" + canary,
		State:              "ENABLED",
		Order:              2,
		BaRuleAction:       "BLOCK",
		FirstTimeEnable:    true,
		FirstTimeOperation: "ALLOW_SCAN",
		MLActionEnabled:    true,
		ByThreatScore:      40,
		AccessControl:      "READ_ONLY",
		Protocols:          []string{"ANY_RULE"},
		Rank:               7,
		BaPolicyCategories: []string{"ADWARE_BLOCK"},
		FileTypes:          []string{"FTCATEGORY_BZIP2"},
		URLCategories:      []string{"FINANCE"},
		LastModifiedTime:   1700000700,
		LastModifiedBy:     &ziacommon.IDNameExtensions{ID: 95, Name: canary},
		Labels:             []ziacommon.IDNameExtensions{{ID: 96, Name: "Label"}},
		ZPAAppSegments:     []ziacommon.ZPAAppSegments{{ID: 97, Name: "Segment", ExternalID: canary}},
		DefaultRule:        true,
	}

	got := projectOneRecord(t, resources.ProductZIA, resourceSandboxRules, []resources.SourceRecord{sandboxRuleSourceRecord(rule)})
	assertNoCanaries(t, "sandbox-rules", got, canary)
	assertFieldsAbsent(t, "sandbox-rules", got, "lastModifiedBy")
	if got["baRuleAction"] != "BLOCK" {
		t.Errorf("projected sandbox-rules baRuleAction = %v, want BLOCK", got["baRuleAction"])
	}
	if got["firstTimeOperation"] != "ALLOW_SCAN" {
		t.Errorf("projected sandbox-rules firstTimeOperation = %v, want ALLOW_SCAN", got["firstTimeOperation"])
	}
	if got["defaultRule"] != true {
		t.Errorf("projected sandbox-rules defaultRule = %v, want true", got["defaultRule"])
	}
	label := mustFirstProjectedItem(t, got, "labels")
	if label["name"] != "Label" {
		t.Errorf("projected sandbox-rules labels[0] = %#v, want name Label", label)
	}
	segment := mustFirstProjectedItem(t, got, "zpaAppSegments")
	if _, ok := segment["externalId"]; ok {
		t.Errorf("projected sandbox-rules zpaAppSegments[0] = %#v, want no externalId", segment)
	}
}

func TestFirewallDNSRuleSourceRecordProjectsThroughAllowList(t *testing.T) {
	t.Parallel()

	const canary = "firewall-dns-rule-psk-canary"
	rule := firewalldnscontrolpolicies.FirewallDNSRules{
		ID:                  612,
		Name:                "DNS rule",
		Order:               2,
		Rank:                7,
		AccessControl:       "READ_WRITE",
		Action:              "BLOCK",
		State:               "ENABLED",
		Description:         "temporary psk=" + canary,
		RedirectIP:          "203.0.113.20",
		BlockResponseCode:   "SERVFAIL",
		LastModifiedTime:    1700000800,
		LastModifiedBy:      &ziacommon.IDNameExtensions{ID: 101, Name: canary},
		SrcIps:              []string{"198.51.100.7"},
		DestAddresses:       []string{"203.0.113.30"},
		DestCountries:       []string{"COUNTRY_CA"},
		SourceCountries:     []string{"COUNTRY_US"},
		DNSRuleRequestTypes: []string{"A"},
		Protocols:           []string{"ANY_RULE"},
		CapturePCAP:         true,
		ApplicationGroups:   []ziacommon.IDNameExtensions{{ID: 102, Name: "App group"}},
		DNSGateway:          &ziacommon.IDName{ID: 103, Name: "DNS gateway", Parent: canary},
		Locations:           []ziacommon.IDNameExtensions{{ID: 104, Name: "HQ"}},
	}

	got := projectOneRecord(t, resources.ProductZIA, resourceFirewallDNSRules, []resources.SourceRecord{firewallDNSRuleSourceRecord(rule)})
	assertNoCanaries(t, "firewall-dns-rules", got, canary)
	assertFieldsAbsent(t, "firewall-dns-rules", got, "lastModifiedBy")
	// Nil-pointer arms of addIDNamePtr: unset references must stay absent.
	assertFieldsAbsent(t, "firewall-dns-rules", got, "zpaIpGroup", "ednsEcsObject")
	if got["action"] != "BLOCK" {
		t.Errorf("projected firewall-dns-rules action = %v, want BLOCK", got["action"])
	}
	if got["redirectIp"] != "203.0.113.20" {
		t.Errorf("projected firewall-dns-rules redirectIp = %v, want 203.0.113.20", got["redirectIp"])
	}
	if got["blockResponseCode"] != "SERVFAIL" {
		t.Errorf("projected firewall-dns-rules blockResponseCode = %v, want SERVFAIL", got["blockResponseCode"])
	}
	dnsGateway := mustProjectedMap(t, got, "dnsGateway")
	if dnsGateway["name"] != "DNS gateway" {
		t.Errorf("projected firewall-dns-rules dnsGateway = %#v, want name DNS gateway", dnsGateway)
	}
	if _, ok := dnsGateway["parent"]; ok {
		t.Errorf("projected firewall-dns-rules dnsGateway = %#v, want no parent", dnsGateway)
	}
}

func TestCustomFileTypeSourceRecordProjectsThroughAllowList(t *testing.T) {
	t.Parallel()

	const canary = "custom-file-type-psk-canary"
	fileType := customfiletypes.CustomFileTypes{
		ID:          613,
		Name:        "Custom file type",
		Description: "temporary psk=" + canary,
		Extension:   "zsc",
		FileTypeID:  105,
	}

	got := projectOneRecord(t, resources.ProductZIA, resourceCustomFileTypes, []resources.SourceRecord{customFileTypeSourceRecord(fileType)})
	assertNoCanaries(t, "custom-file-types", got, canary)
	if got["id"] != 613 {
		t.Errorf("projected custom-file-types id = %v, want 613", got["id"])
	}
	if got["name"] != "Custom file type" {
		t.Errorf("projected custom-file-types name = %v, want Custom file type", got["name"])
	}
	if got["extension"] != "zsc" {
		t.Errorf("projected custom-file-types extension = %v, want zsc", got["extension"])
	}
	if got["fileTypeId"] != 105 {
		t.Errorf("projected custom-file-types fileTypeId = %v, want 105", got["fileTypeId"])
	}
}

func TestZPAGatewaySourceRecordDropsExternalIdentifiers(t *testing.T) {
	t.Parallel()

	const canary = "zpa-gateway-psk-canary"
	gateway := zpagateways.ZPAGateways{
		ID:          614,
		Name:        "ZPA gateway",
		Description: "temporary psk=" + canary,
		ZPAServerGroup: zpagateways.ZPAServerGroup{
			ID:         106,
			Name:       "Server group",
			ExternalID: canary,
			Extensions: map[string]any{"token": canary},
		},
		ZPAAppSegments: []zpagateways.ZPAAppSegments{
			{ID: 107, Name: "Segment", ExternalID: canary, Extensions: map[string]any{"token": canary}},
		},
		ZPATenantId:      108,
		LastModifiedBy:   &ziacommon.IDNameExtensions{ID: 109, Name: canary},
		LastModifiedTime: 1700000900,
		Type:             "ZPA",
	}

	got := projectOneRecord(t, resources.ProductZIA, resourceZPAGateways, []resources.SourceRecord{zpaGatewaySourceRecord(gateway)})
	assertNoCanaries(t, "zpa-gateways", got, canary)
	assertFieldsAbsent(t, "zpa-gateways", got, "lastModifiedBy")
	if got["name"] != "ZPA gateway" {
		t.Errorf("projected zpa-gateways name = %v, want ZPA gateway", got["name"])
	}
	if got["type"] != "ZPA" {
		t.Errorf("projected zpa-gateways type = %v, want ZPA", got["type"])
	}
	if got["zpaTenantId"] != 108 {
		t.Errorf("projected zpa-gateways zpaTenantId = %v, want 108", got["zpaTenantId"])
	}
	serverGroup := mustProjectedMap(t, got, "zpaServerGroup")
	if serverGroup["name"] != "Server group" {
		t.Errorf("projected zpa-gateways zpaServerGroup = %#v, want name Server group", serverGroup)
	}
	for _, secret := range []string{"externalId", "extensions"} {
		if _, ok := serverGroup[secret]; ok {
			t.Errorf("projected zpa-gateways zpaServerGroup = %#v, want no %s", serverGroup, secret)
		}
	}
	segment := mustFirstProjectedItem(t, got, "zpaAppSegments")
	if segment["name"] != "Segment" {
		t.Errorf("projected zpa-gateways zpaAppSegments[0] = %#v, want name Segment", segment)
	}
	for _, secret := range []string{"externalId", "extensions"} {
		if _, ok := segment[secret]; ok {
			t.Errorf("projected zpa-gateways zpaAppSegments[0] = %#v, want no %s", segment, secret)
		}
	}

	// Empty-slice arm: a gateway with no app segments must not render the key.
	minimal := zpagateways.ZPAGateways{ID: 615, Name: "Minimal gateway", Type: "ZPA"}
	gotMinimal := projectOneRecord(t, resources.ProductZIA, resourceZPAGateways, []resources.SourceRecord{zpaGatewaySourceRecord(minimal)})
	assertFieldsAbsent(t, "zpa-gateways", gotMinimal, "zpaAppSegments", "lastModifiedBy")
}

func TestC2CIncidentReceiverSourceRecordDropsTenantAuthorization(t *testing.T) {
	t.Parallel()

	const canary = "c2c-incident-receiver-psk-canary"
	receiver := c2cincidentreceiver.C2CIncidentReceiver{
		ID:                       616,
		Name:                     "C2C receiver",
		Status:                   []string{"ENABLED"},
		ModifiedTime:             1700001000,
		LastTenantValidationTime: 1700001100,
		LastValidationMsg:        &c2cincidentreceiver.LastValidationMsg{ErrorMsg: canary, ErrorCode: canary},
		LastModifiedBy:           &ziacommon.IDNameExtensions{ID: 111, Name: canary},
		OnboardableEntity: &c2cincidentreceiver.OnboardableEntity{
			ID:                 112,
			Name:               "Onboardable entity",
			Type:               "INCIDENT_RECEIVER",
			EnterpriseTenantID: "tenant-ref-1",
			Application:        "ZSCALER_INCIDENT_RECEIVER",
			LastValidationMsg:  c2cincidentreceiver.LastValidationMsg{ErrorMsg: canary},
			TenantAuthorizationInfo: c2cincidentreceiver.TenantAuthorizationInfo{
				AccessToken: canary,
				BotToken:    canary,
			},
			ZscalerAppTenantID: &ziacommon.IDNameExtensions{ID: 113, Name: "App tenant"},
		},
	}

	got := projectOneRecord(t, resources.ProductZIA, resourceC2CIncidentRcvs, []resources.SourceRecord{c2cIncidentReceiverSourceRecord(receiver)})
	assertNoCanaries(t, "c2c-incident-receivers", got, canary)
	assertFieldsAbsent(t, "c2c-incident-receivers", got, "lastValidationMsg", "lastModifiedBy")
	if got["name"] != "C2C receiver" {
		t.Errorf("projected c2c-incident-receivers name = %v, want C2C receiver", got["name"])
	}
	if got["modifiedTime"] != 1700001000 {
		t.Errorf("projected c2c-incident-receivers modifiedTime = %v, want 1700001000", got["modifiedTime"])
	}
	entity := mustProjectedMap(t, got, "onboardableEntity")
	if entity["name"] != "Onboardable entity" {
		t.Errorf("projected c2c-incident-receivers onboardableEntity = %#v, want name Onboardable entity", entity)
	}
	if entity["enterpriseTenantId"] != "tenant-ref-1" {
		t.Errorf("projected c2c-incident-receivers onboardableEntity = %#v, want enterpriseTenantId tenant-ref-1", entity)
	}
	for _, secret := range []string{"lastValidationMsg", "tenantAuthorizationInfo"} {
		if _, ok := entity[secret]; ok {
			t.Errorf("projected c2c-incident-receivers onboardableEntity = %#v, want no %s", entity, secret)
		}
	}
	appTenant, ok := entity["zscalerAppTenantId"].(map[string]any)
	if !ok {
		t.Fatalf("projected c2c-incident-receivers zscalerAppTenantId = %T, want map[string]any", entity["zscalerAppTenantId"])
	}
	if appTenant["name"] != "App tenant" {
		t.Errorf("projected c2c-incident-receivers zscalerAppTenantId = %#v, want name App tenant", appTenant)
	}

	// Nil-entity arm: c2cOnboardableEntitySource returns nil, and the projected
	// record must not carry an empty onboardableEntity object.
	minimal := c2cincidentreceiver.C2CIncidentReceiver{ID: 617, Name: "Minimal receiver"}
	gotMinimal := projectOneRecord(t, resources.ProductZIA, resourceC2CIncidentRcvs, []resources.SourceRecord{c2cIncidentReceiverSourceRecord(minimal)})
	assertFieldsAbsent(t, "c2c-incident-receivers", gotMinimal, "onboardableEntity", "lastModifiedBy")
}

func TestIPSSignatureRuleSourceRecordOmitsRuleText(t *testing.T) {
	t.Parallel()

	const canary = "ips-signature-rule-psk-canary"
	rule := ipssignaturerules.IPSSignatureRules{
		ID:                         618,
		Name:                       "IPS signature rule",
		RuleText:                   canary,
		Description:                "temporary psk=" + canary,
		Enabled:                    true,
		Deleted:                    false,
		PromoteTime:                1700001200,
		RuleTextModTime:            1700001300,
		DynamicValidationSubmitted: true,
		DynamicValidationRejected:  false,
		DynamicValidationSucceeded: true,
		DisabledFromZSCM:           false,
		DynamicValRejectCode:       9,
	}

	got := projectOneRecord(t, resources.ProductZIA, resourceIPSSignatureRules, []resources.SourceRecord{ipsSignatureRuleSourceRecord(rule)})
	assertNoCanaries(t, "ips-signature-rules", got, canary)
	// ruleText is deliberately unmapped: the Suricata/Snort rule body never
	// reaches the source record, let alone the projection.
	assertFieldsAbsent(t, "ips-signature-rules", got, "ruleText")
	if got["name"] != "IPS signature rule" {
		t.Errorf("projected ips-signature-rules name = %v, want IPS signature rule", got["name"])
	}
	if got["enabled"] != true {
		t.Errorf("projected ips-signature-rules enabled = %v, want true", got["enabled"])
	}
	if got["dynamicValidationSucceeded"] != true {
		t.Errorf("projected ips-signature-rules dynamicValidationSucceeded = %v, want true", got["dynamicValidationSucceeded"])
	}
	if got["dynamicValRejectCode"] != 9 {
		t.Errorf("projected ips-signature-rules dynamicValRejectCode = %v, want 9", got["dynamicValRejectCode"])
	}
}

func TestCloudAppControlSourceRecordProjectsThroughAllowList(t *testing.T) {
	t.Parallel()

	const canary = "cloud-app-control-psk-canary"
	rule := cloudappcontrol.WebApplicationRules{
		ID:                   619,
		Name:                 "Cloud app rule",
		Description:          "temporary psk=" + canary,
		Actions:              []string{"ALLOW_STREAMING_VIEW_LISTEN"},
		State:                "ENABLED",
		Rank:                 7,
		Type:                 "STREAMING_MEDIA",
		Order:                3,
		TimeQuota:            15,
		SizeQuota:            10,
		CascadingEnabled:     true,
		AccessControl:        "READ_ONLY",
		Applications:         []string{"YOUTUBE"},
		NumberOfApplications: 1,
		EunEnabled:           true,
		EunTemplateID:        4,
		BrowserEunTemplateID: 5,
		ValidityStartTime:    1700001400,
		ValidityEndTime:      1700001500,
		ValidityTimeZoneID:   "UTC",
		UserAgentTypes:       []string{"CHROME"},
		LastModifiedTime:     1700001600,
		EnforceTimeValidity:  true,
		DeviceTrustLevels:    []string{"HIGH_TRUST"},
		UserRiskScoreLevels:  []string{"LOW"},
		Labels:               []ziacommon.IDNameExtensions{{ID: 121, Name: "Label"}},
		TimeWindows:          []ziacommon.IDNameExtensions{{ID: 122, Name: "Work hours"}},
		TenancyProfileIDs:    []ziacommon.IDNameExtensions{{ID: 123, Name: "Tenancy"}},
		// Unmapped by cloudAppControlSourceRecord: must never reach the record.
		CloudAppRiskProfile: &ziacommon.IDCustom{ID: 124, Name: canary},
	}

	got := projectOneRecord(t, resources.ProductZIA, resourceCloudAppControl, []resources.SourceRecord{cloudAppControlSourceRecord(rule)})
	assertNoCanaries(t, "cloud-app-control", got, canary)
	assertFieldsAbsent(t, "cloud-app-control", got, "cloudAppRiskProfile")
	if got["name"] != "Cloud app rule" {
		t.Errorf("projected cloud-app-control name = %v, want Cloud app rule", got["name"])
	}
	if got["type"] != "STREAMING_MEDIA" {
		t.Errorf("projected cloud-app-control type = %v, want STREAMING_MEDIA", got["type"])
	}
	actions, ok := got["actions"].([]string)
	if !ok || len(actions) != 1 || actions[0] != "ALLOW_STREAMING_VIEW_LISTEN" {
		t.Errorf("projected cloud-app-control actions = %#v, want [ALLOW_STREAMING_VIEW_LISTEN]", got["actions"])
	}
	label := mustFirstProjectedItem(t, got, "labels")
	if label["name"] != "Label" {
		t.Errorf("projected cloud-app-control labels[0] = %#v, want name Label", label)
	}
	tenancy := mustFirstProjectedItem(t, got, "tenancyProfileIds")
	if tenancy["name"] != "Tenancy" {
		t.Errorf("projected cloud-app-control tenancyProfileIds[0] = %#v, want name Tenancy", tenancy)
	}
}
