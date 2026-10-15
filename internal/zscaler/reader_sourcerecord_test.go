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
	"fmt"
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

	"github.com/dvmrry/zscalerctl/internal/redact"
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

// assertRefListIDsOnly asserts record[field] rendered as a non-empty list of
// {id} references: every element carries an id and nothing else (no name, no
// extensions). Reference lists ({id, name, extensions}) render their ids in
// share mode even where the referenced names are standard-only. When wantIDs
// is given, the element ids must print as those values, in order.
func assertRefListIDsOnly(t *testing.T, resource string, record map[string]any, field string, wantIDs ...any) {
	t.Helper()

	value, ok := record[field]
	if !ok {
		t.Errorf("projected %s missing %s, want id-only reference list", resource, field)
		return
	}
	items, ok := value.([]any)
	if !ok || len(items) == 0 {
		t.Errorf("projected %s %s = %T(%v), want non-empty reference list", resource, field, value, value)
		return
	}
	if len(wantIDs) > 0 && len(items) != len(wantIDs) {
		t.Errorf("projected %s %s = %#v, want %d references", resource, field, items, len(wantIDs))
	}
	for i, item := range items {
		ref, ok := item.(map[string]any)
		if !ok {
			t.Errorf("projected %s %s[%d] = %T, want reference map", resource, field, i, item)
			continue
		}
		id, hasID := ref["id"]
		if !hasID {
			t.Errorf("projected %s %s[%d] = %#v, want id", resource, field, i, ref)
		}
		if i < len(wantIDs) && fmt.Sprint(id) != fmt.Sprint(wantIDs[i]) {
			t.Errorf("projected %s %s[%d].id = %v, want %v", resource, field, i, id, wantIDs[i])
		}
		for key := range ref {
			if key != "id" {
				t.Errorf("projected %s %s[%d] = %#v, want only id (key %q must not render)", resource, field, i, ref, key)
			}
		}
	}
}

// assertRefIDsOnlyInShare checks reference fields whose names are
// standard-only: in share mode each renders as id-only (a single {id} map or a
// list of them) with the wanted id; in paranoid mode each is absent.
func assertRefIDsOnlyInShare(t *testing.T, resource string, mode redact.Mode, record map[string]any, refs map[string]int) {
	t.Helper()

	for field, wantID := range refs {
		if mode != redact.ModeShare {
			if _, ok := record[field]; ok {
				t.Errorf("%s projected %s includes %s, want dropped", mode, resource, field)
			}
			continue
		}
		if _, ok := record[field].(map[string]any); ok {
			assertAuditRefIDOnly(t, resource+" share", record, field, wantID)
			continue
		}
		assertRefListIDsOnly(t, resource+" share", record, field, wantID)
	}
}

// projectSourceRecordInModes projects records in share and paranoid mode,
// asserts that none of the canaries survive in either, and returns the
// projections keyed by mode. Use it for values that render in standard mode
// only: the catalog must keep them out of the wider-audience modes.
func projectSourceRecordInModes(
	t *testing.T,
	resource string,
	records []resources.SourceRecord,
	canaries ...string,
) map[redact.Mode]map[string]any {
	t.Helper()

	out := map[redact.Mode]map[string]any{}
	for _, mode := range []redact.Mode{redact.ModeShare, redact.ModeParanoid} {
		got := projectOneRecordInMode(t, resources.ProductZIA, resource, mode, records)
		assertNoCanaries(t, resource+" ("+string(mode)+")", got, canaries...)
		out[mode] = got
	}
	return out
}

// assertAdminRef asserts an admin audit reference rendered with the wanted
// id, and with the admin name only when wantName is non-empty.
func assertAdminRef(t *testing.T, resource string, record map[string]any, field string, wantID int, wantName string) {
	t.Helper()

	ref, ok := record[field].(map[string]any)
	if !ok {
		t.Errorf("projected %s %s = %T(%v), want admin reference map", resource, field, record[field], record[field])
		return
	}
	if ref["id"] != wantID {
		t.Errorf("projected %s %s = %#v, want id %d", resource, field, ref, wantID)
	}
	name, hasName := ref["name"]
	switch {
	case wantName == "" && hasName:
		t.Errorf("projected %s %s = %#v, want no name", resource, field, ref)
	case wantName != "" && name != wantName:
		t.Errorf("projected %s %s = %#v, want name %q", resource, field, ref, wantName)
	}
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

// TestDLPDictionarySourceRecordKeepsDetectionContentStandardOnly pins the
// dictionary detection content (phrases, patterns, predefined phrases, BIN
// numbers) as standard-only free text: it renders for the operator and never
// reaches share or paranoid output. Match structure renders in every mode.
func TestDLPDictionarySourceRecordKeepsDetectionContentStandardOnly(t *testing.T) {
	t.Parallel()

	const (
		canary        = "dlp-dictionary-psk-canary"
		contentCanary = "dlp-dictionary-content-canary"
	)
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
			{Action: "PHRASE_COUNT_TYPE_ALL", Phrase: contentCanary},
		},
		Patterns: []dlpdictionaries.Patterns{
			{Action: "PATTERN_COUNT_TYPE_ALL", Pattern: contentCanary},
		},
		EDMMatchDetails: []dlpdictionaries.EDMMatchDetails{
			{DictionaryEdmMappingID: 21, SchemaID: 22},
		},
		IDMProfileMatchAccuracy: []dlpdictionaries.IDMProfileMatchAccuracy{
			{MatchAccuracy: "HEAVY"},
		},
		HierarchicalIdentifiers:          []string{"CREDIT_CARD"},
		PredefinedPhrases:                []string{contentCanary},
		BinNumbers:                       []int{424242},
		DictTemplateId:                   23,
		ConfidenceLevelForPredefinedDict: "CONFIDENCE_LEVEL_LOW",
	}
	records := []resources.SourceRecord{dlpDictionarySourceRecord(dictionary)}

	got := projectOneRecord(t, resources.ProductZIA, resourceDLPDictionaries, records)
	assertNoCanaries(t, "dlp-dictionaries", got, canary)
	phrase := mustFirstProjectedItem(t, got, "phrases")
	if phrase["phrase"] != contentCanary || phrase["action"] != "PHRASE_COUNT_TYPE_ALL" {
		t.Errorf("projected dlp-dictionaries phrases[0] = %#v, want phrase and action", phrase)
	}
	pattern := mustFirstProjectedItem(t, got, "patterns")
	if pattern["pattern"] != contentCanary || pattern["action"] != "PATTERN_COUNT_TYPE_ALL" {
		t.Errorf("projected dlp-dictionaries patterns[0] = %#v, want pattern and action", pattern)
	}
	if fmt.Sprint(got["predefinedPhrases"]) != "["+contentCanary+"]" {
		t.Errorf("projected dlp-dictionaries predefinedPhrases = %#v, want [%s]", got["predefinedPhrases"], contentCanary)
	}
	if fmt.Sprint(got["binNumbers"]) != "[424242]" {
		t.Errorf("projected dlp-dictionaries binNumbers = %#v, want [424242]", got["binNumbers"])
	}
	if fmt.Sprint(got["hierarchicalIdentifiers"]) != "[CREDIT_CARD]" {
		t.Errorf("projected dlp-dictionaries hierarchicalIdentifiers = %#v, want [CREDIT_CARD]", got["hierarchicalIdentifiers"])
	}
	edm := mustFirstProjectedItem(t, got, "exactDataMatchDetails")
	if fmt.Sprint(edm["dictionaryEdmMappingId"]) != "21" || fmt.Sprint(edm["schemaId"]) != "22" {
		t.Errorf("projected dlp-dictionaries exactDataMatchDetails[0] = %#v, want mapping 21 schema 22", edm)
	}
	accuracy := mustFirstProjectedItem(t, got, "idmProfileMatchAccuracyDetails")
	if accuracy["matchAccuracy"] != "HEAVY" {
		t.Errorf("projected dlp-dictionaries idmProfileMatchAccuracyDetails[0] = %#v, want matchAccuracy HEAVY", accuracy)
	}

	// Detection content is standard-only; the match structure is not.
	for mode, wider := range projectSourceRecordInModes(t, resourceDLPDictionaries, records, canary, contentCanary) {
		assertFieldsAbsent(t, "dlp-dictionaries ("+string(mode)+")", wider, "binNumbers", "predefinedPhrases")
		phrase := mustFirstProjectedItem(t, wider, "phrases")
		if _, ok := phrase["phrase"]; ok || phrase["action"] != "PHRASE_COUNT_TYPE_ALL" {
			t.Errorf("projected dlp-dictionaries (%s) phrases[0] = %#v, want action only", mode, phrase)
		}
	}
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

// TestDLPEDMSchemaSourceRecordScheduleAndAdminsFollowModes pins the EDM
// schedule, column names and admin names as standard-only, admin ids in every
// mode, and the edmClient extensions as never rendered.
func TestDLPEDMSchemaSourceRecordScheduleAndAdminsFollowModes(t *testing.T) {
	t.Parallel()

	const (
		canary         = "dlp-edm-schema-psk-canary"
		standardCanary = "edm-standard-canary"
	)
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
		ModifiedBy:       &ziacommon.IDNameExtensions{ID: 32, Name: standardCanary},
		CreatedBy:        &ziacommon.IDNameExtensions{ID: 33, Name: standardCanary},
		CellsUsed:        11,
		SchemaActive:     true,
		SchedulePresent:  true,
		TokenList: []dlpexactdatamatch.TokenList{
			{Name: standardCanary, Type: "EDM_TOKEN"},
		},
		Schedule: dlpexactdatamatch.Schedule{ScheduleType: standardCanary},
	}
	records := []resources.SourceRecord{dlpEDMSchemaSourceRecord(schema)}

	got := projectOneRecord(t, resources.ProductZIA, resourceDLPEDMSchemas, records)
	assertNoCanaries(t, "dlp-edm-schemas", got, canary)
	assertAdminRef(t, "dlp-edm-schemas", got, "modifiedBy", 32, standardCanary)
	assertAdminRef(t, "dlp-edm-schemas", got, "createdBy", 33, standardCanary)
	token := mustFirstProjectedItem(t, got, "tokenList")
	if token["name"] != standardCanary || token["type"] != "EDM_TOKEN" {
		t.Errorf("projected dlp-edm-schemas tokenList[0] = %#v, want name and type", token)
	}
	if schedule := mustProjectedMap(t, got, "schedule"); schedule["scheduleType"] != standardCanary {
		t.Errorf("projected dlp-edm-schemas schedule = %#v, want scheduleType %s", schedule, standardCanary)
	}
	for mode, wider := range projectSourceRecordInModes(t, resourceDLPEDMSchemas, records, canary, standardCanary) {
		label := "dlp-edm-schemas (" + string(mode) + ")"
		assertFieldsAbsent(t, label, wider, "schedule")
		assertAdminRef(t, label, wider, "modifiedBy", 32, "")
		assertAdminRef(t, label, wider, "createdBy", 33, "")
		token := mustFirstProjectedItem(t, wider, "tokenList")
		if _, ok := token["name"]; ok || token["type"] != "EDM_TOKEN" {
			t.Errorf("projected %s tokenList[0] = %#v, want type only", label, token)
		}
	}
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

	records := []resources.SourceRecord{dlpIDMProfileLiteSourceRecord(profile)}
	got := projectOneRecord(t, resources.ProductZIA, resourceDLPIDMLite, records)
	// The admin name is standard-only; the admin id renders in every mode.
	assertAdminRef(t, "dlp-idm-profile-lite", got, "modifiedBy", 42, canary)
	for mode, wider := range projectSourceRecordInModes(t, resourceDLPIDMLite, records, canary) {
		assertAdminRef(t, "dlp-idm-profile-lite ("+string(mode)+")", wider, "modifiedBy", 42, "")
	}
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

	const (
		canary    = "dlp-idm-profile-psk-canary"
		adminName = "idm-profile-admin"
	)
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
		ModifiedBy:         &ziacommon.IDNameExtensions{ID: 52, Name: adminName},
	}

	records := []resources.SourceRecord{dlpIDMProfileSourceRecord(profile)}
	got := projectOneRecord(t, resources.ProductZIA, resourceDLPIDMProfiles, records)
	assertNoCanaries(t, "dlp-idm-profiles", got, canary)
	// The admin name is standard-only; the admin id renders in every mode.
	assertAdminRef(t, "dlp-idm-profiles", got, "modifiedBy", 52, adminName)
	for mode, wider := range projectSourceRecordInModes(t, resourceDLPIDMProfiles, records, canary, adminName) {
		assertAdminRef(t, "dlp-idm-profiles ("+string(mode)+")", wider, "modifiedBy", 52, "")
	}
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

// TestRiskProfileSourceRecordProjectsRiskAttributes pins the risk-profile
// criteria (the app attributes the profile matches on) as tenant
// configuration that renders in every mode, while reference extensions stay
// dropped.
func TestRiskProfileSourceRecordProjectsRiskAttributes(t *testing.T) {
	t.Parallel()

	const (
		canary    = "risk-profile-psk-canary"
		criterion = "risk-criterion-value"
	)
	profile := riskprofiles.RiskProfiles{
		ID:                        607,
		ProfileName:               "Risk profile",
		ProfileType:               "CLOUD_APPLICATIONS",
		Status:                    "SANCTIONED",
		ExcludeCertificates:       4242,
		PoorItemsOfService:        criterion,
		AdminAuditLogs:            criterion,
		DataBreach:                criterion,
		SourceIpRestrictions:      "YES",
		MfaSupport:                criterion,
		SslPinned:                 criterion,
		HttpSecurityHeaders:       criterion,
		Evasive:                   criterion,
		DnsCaaPolicy:              criterion,
		WeakCipherSupport:         criterion,
		PasswordStrength:          criterion,
		SslCertValidity:           criterion,
		Vulnerability:             criterion,
		MalwareScanningForContent: criterion,
		FileSharing:               criterion,
		SslCertKeySize:            criterion,
		VulnerableToHeartBleed:    criterion,
		VulnerableToLogJam:        criterion,
		VulnerableToPoodle:        criterion,
		VulnerabilityDisclosure:   criterion,
		SupportForWaf:             criterion,
		RemoteScreenSharing:       criterion,
		SenderPolicyFramework:     criterion,
		DomainKeysIdentifiedMail:  criterion,
		DomainBasedMessageAuth:    criterion,
		LastModTime:               1700000400,
		CreateTime:                1700000500,
		Certifications:            []string{criterion},
		DataEncryptionInTransit:   []string{criterion},
		RiskIndex:                 []int{4242},
		ModifiedBy: &ziacommon.IDNameExtensions{
			ID:         61,
			Name:       "Risk admin",
			Extensions: map[string]any{"token": canary},
		},
		CustomTags: []ziacommon.IDNameExternalID{{
			ID:         62,
			Name:       "tag-context",
			ExternalID: "tag-external-62",
			Extensions: map[string]any{"token": canary},
		}},
	}
	records := []resources.SourceRecord{riskProfileSourceRecord(profile)}

	stringCriteria := []string{
		"adminAuditLogs", "dataBreach", "dnsCaaPolicy", "domainBasedMessageAuth",
		"domainKeysIdentifiedMail", "evasive", "fileSharing", "httpSecurityHeaders",
		"malwareScanningForContent", "mfaSupport", "passwordStrength", "poorItemsOfService",
		"remoteScreenSharing", "senderPolicyFramework", "sslCertKeySize", "sslCertValidity",
		"sslPinned", "supportForWaf", "vulnerability", "vulnerabilityDisclosure",
		"vulnerableToHeartBleed", "vulnerableToLogJam", "vulnerableToPoodle", "weakCipherSupport",
	}
	listCriteria := map[string]string{
		"certifications":          "[" + criterion + "]",
		"dataEncryptionInTransit": "[" + criterion + "]",
		"riskIndex":               "[4242]",
	}
	for _, mode := range []redact.Mode{redact.ModeStandard, redact.ModeShare, redact.ModeParanoid} {
		got := projectOneRecordInMode(t, resources.ProductZIA, resourceRiskProfiles, mode, records)
		label := "risk-profiles (" + string(mode) + ")"
		assertNoCanaries(t, label, got, canary)
		for _, field := range stringCriteria {
			if got[field] != criterion {
				t.Errorf("projected %s %s = %v, want %s", label, field, got[field], criterion)
			}
		}
		for field, want := range listCriteria {
			if fmt.Sprint(got[field]) != want {
				t.Errorf("projected %s %s = %#v, want %s", label, field, got[field], want)
			}
		}
		if got["excludeCertificates"] != 4242 {
			t.Errorf("projected %s excludeCertificates = %v, want 4242", label, got["excludeCertificates"])
		}
	}

	got := projectOneRecord(t, resources.ProductZIA, resourceRiskProfiles, records)
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
	if _, ok := modifiedBy["extensions"]; ok {
		t.Errorf("projected risk-profiles modifiedBy = %#v, want no extensions", modifiedBy)
	}
	customTag := mustFirstProjectedItem(t, got, "customTags")
	if customTag["name"] != "tag-context" {
		t.Errorf("projected risk-profiles customTags[0] = %#v, want name tag-context", customTag)
	}
	if customTag["externalId"] != "tag-external-62" {
		t.Errorf("projected risk-profiles customTags[0] = %#v, want externalId tag-external-62", customTag)
	}
	if _, ok := customTag["extensions"]; ok {
		t.Errorf("projected risk-profiles customTags[0] = %#v, want no extensions", customTag)
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

	const (
		canary         = "nss-feed-psk-canary"
		standardCanary = "nss-feed-standard-canary"
	)
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
		Base64EncodedCertificate: standardCanary,
		NssType:                  "NSS_FOR_WEB",
		ClientID:                 standardCanary,
		ClientSecret:             canary,
		AuthenticationUrl:        "https://login.internal.test/oauth2/token",
		GrantType:                "CLIENT_CREDENTIALS",
		Scope:                    standardCanary,
		CloudNSS:                 true,
		OauthAuthentication:      true,
		ServerIps:                []string{"203.0.113.10"},
		Domains:                  []string{"internal.test"},
		Users:                    []ziacommon.CommonNSS{{ID: 81, Name: standardCanary}},
		Locations:                []ziacommon.CommonNSS{{ID: 82, Name: "HQ"}},
	}
	records := []resources.SourceRecord{nssFeedSourceRecord(feed)}

	// Credentials (token, client secret, connection headers) never render.
	got := projectOneRecord(t, resources.ProductZIA, resourceNSSFeeds, records)
	assertNoCanaries(t, "nss-feeds", got, canary)
	assertFieldsAbsent(t, "nss-feeds", got, "authenticationToken", "clientSecret", "connectionHeaders")
	// OAuth client id, scope and certificate are standard-only identifiers;
	// filter entries render with their ids in every mode.
	for _, field := range []string{"clientId", "scope", "base64EncodedCertificate"} {
		if got[field] != standardCanary {
			t.Errorf("projected nss-feeds %s = %v, want %s", field, got[field], standardCanary)
		}
	}
	if user := mustFirstProjectedItem(t, got, "users"); user["id"] != 81 || user["name"] != standardCanary {
		t.Errorf("projected nss-feeds users[0] = %#v, want id 81 name %s", user, standardCanary)
	}
	if location := mustFirstProjectedItem(t, got, "locations"); location["id"] != 82 || location["name"] != "HQ" {
		t.Errorf("projected nss-feeds locations[0] = %#v, want id 82 name HQ", location)
	}
	for mode, wider := range projectSourceRecordInModes(t, resourceNSSFeeds, records, canary, standardCanary) {
		label := "nss-feeds (" + string(mode) + ")"
		assertFieldsAbsent(t, label, wider,
			"authenticationToken", "clientSecret", "connectionHeaders",
			"clientId", "scope", "base64EncodedCertificate",
		)
		if user := mustFirstProjectedItem(t, wider, "users"); user["id"] != 81 {
			t.Errorf("projected %s users[0] = %#v, want id 81", label, user)
		}
	}
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

	const (
		canary    = "file-type-rule-psk-canary"
		adminName = "file-type-rule-admin"
	)
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
		LastModifiedBy:       &ziacommon.IDNameExtensions{ID: 91, Name: adminName},
		Locations:            []ziacommon.IDNameExtensions{{ID: 92, Name: "HQ", Extensions: map[string]any{"token": canary}}},
		ZPAAppSegments:       []ziacommon.ZPAAppSegments{{ID: 93, Name: "Segment", ExternalID: "segment-external-93"}},
	}
	records := []resources.SourceRecord{fileTypeRuleSourceRecord(rule)}

	got := projectOneRecord(t, resources.ProductZIA, resourceFileTypeRules, records)
	assertNoCanaries(t, "file-type-rules", got, canary)
	if got["passwordProtected"] != true {
		t.Errorf("projected file-type-rules passwordProtected = %v, want true", got["passwordProtected"])
	}
	assertAdminRef(t, "file-type-rules", got, "lastModifiedBy", 91, adminName)
	for mode, wider := range projectSourceRecordInModes(t, resourceFileTypeRules, records, canary, adminName) {
		assertAdminRef(t, "file-type-rules ("+string(mode)+")", wider, "lastModifiedBy", 91, "")
	}
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
	if segment["externalId"] != "segment-external-93" {
		t.Errorf("projected file-type-rules zpaAppSegments[0] = %#v, want externalId segment-external-93", segment)
	}
}

func TestSandboxRuleSourceRecordProjectsThroughAllowList(t *testing.T) {
	t.Parallel()

	const (
		canary    = "sandbox-rule-psk-canary"
		adminName = "sandbox-rule-admin"
	)
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
		LastModifiedBy:     &ziacommon.IDNameExtensions{ID: 95, Name: adminName},
		Labels:             []ziacommon.IDNameExtensions{{ID: 96, Name: "Label", Extensions: map[string]any{"token": canary}}},
		ZPAAppSegments:     []ziacommon.ZPAAppSegments{{ID: 97, Name: "Segment", ExternalID: "segment-external-97"}},
		DefaultRule:        true,
	}
	records := []resources.SourceRecord{sandboxRuleSourceRecord(rule)}

	got := projectOneRecord(t, resources.ProductZIA, resourceSandboxRules, records)
	assertNoCanaries(t, "sandbox-rules", got, canary)
	assertAdminRef(t, "sandbox-rules", got, "lastModifiedBy", 95, adminName)
	for mode, wider := range projectSourceRecordInModes(t, resourceSandboxRules, records, canary, adminName) {
		assertAdminRef(t, "sandbox-rules ("+string(mode)+")", wider, "lastModifiedBy", 95, "")
	}
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
	if segment["externalId"] != "segment-external-97" {
		t.Errorf("projected sandbox-rules zpaAppSegments[0] = %#v, want externalId segment-external-97", segment)
	}
}

func TestFirewallDNSRuleSourceRecordProjectsThroughAllowList(t *testing.T) {
	t.Parallel()

	const (
		canary    = "firewall-dns-rule-psk-canary"
		adminName = "dns-rule-admin"
	)
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
		LastModifiedBy:      &ziacommon.IDNameExtensions{ID: 101, Name: adminName},
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

	records := []resources.SourceRecord{firewallDNSRuleSourceRecord(rule)}
	got := projectOneRecord(t, resources.ProductZIA, resourceFirewallDNSRules, records)
	assertNoCanaries(t, "firewall-dns-rules", got, canary)
	assertAdminRef(t, "firewall-dns-rules", got, "lastModifiedBy", 101, adminName)
	for mode, wider := range projectSourceRecordInModes(t, resourceFirewallDNSRules, records, canary, adminName) {
		assertAdminRef(t, "firewall-dns-rules ("+string(mode)+")", wider, "lastModifiedBy", 101, "")
	}
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

// TestZPAGatewaySourceRecordDropsReferenceExtensions pins the ZPA server
// group and app segment references: their external ids (the ZPA object ids)
// render, their extensions never do.
func TestZPAGatewaySourceRecordDropsReferenceExtensions(t *testing.T) {
	t.Parallel()

	const (
		canary    = "zpa-gateway-psk-canary"
		adminName = "zpa-gateway-admin"
	)
	gateway := zpagateways.ZPAGateways{
		ID:          614,
		Name:        "ZPA gateway",
		Description: "temporary psk=" + canary,
		ZPAServerGroup: zpagateways.ZPAServerGroup{
			ID:         106,
			Name:       "Server group",
			ExternalID: "server-group-external-106",
			Extensions: map[string]any{"token": canary},
		},
		ZPAAppSegments: []zpagateways.ZPAAppSegments{
			{ID: 107, Name: "Segment", ExternalID: "segment-external-107", Extensions: map[string]any{"token": canary}},
		},
		ZPATenantId:      108,
		LastModifiedBy:   &ziacommon.IDNameExtensions{ID: 109, Name: adminName},
		LastModifiedTime: 1700000900,
		Type:             "ZPA",
	}
	records := []resources.SourceRecord{zpaGatewaySourceRecord(gateway)}

	got := projectOneRecord(t, resources.ProductZIA, resourceZPAGateways, records)
	assertNoCanaries(t, "zpa-gateways", got, canary)
	assertAdminRef(t, "zpa-gateways", got, "lastModifiedBy", 109, adminName)
	for mode, wider := range projectSourceRecordInModes(t, resourceZPAGateways, records, canary, adminName) {
		assertAdminRef(t, "zpa-gateways ("+string(mode)+")", wider, "lastModifiedBy", 109, "")
	}
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
	if serverGroup["externalId"] != "server-group-external-106" {
		t.Errorf("projected zpa-gateways zpaServerGroup = %#v, want externalId server-group-external-106", serverGroup)
	}
	if _, ok := serverGroup["extensions"]; ok {
		t.Errorf("projected zpa-gateways zpaServerGroup = %#v, want no extensions", serverGroup)
	}
	segment := mustFirstProjectedItem(t, got, "zpaAppSegments")
	if segment["name"] != "Segment" {
		t.Errorf("projected zpa-gateways zpaAppSegments[0] = %#v, want name Segment", segment)
	}
	if segment["externalId"] != "segment-external-107" {
		t.Errorf("projected zpa-gateways zpaAppSegments[0] = %#v, want externalId segment-external-107", segment)
	}
	if _, ok := segment["extensions"]; ok {
		t.Errorf("projected zpa-gateways zpaAppSegments[0] = %#v, want no extensions", segment)
	}

	// Empty-slice arm: a gateway with no app segments must not render the key.
	minimal := zpagateways.ZPAGateways{ID: 615, Name: "Minimal gateway", Type: "ZPA"}
	gotMinimal := projectOneRecord(t, resources.ProductZIA, resourceZPAGateways, []resources.SourceRecord{zpaGatewaySourceRecord(minimal)})
	assertFieldsAbsent(t, "zpa-gateways", gotMinimal, "zpaAppSegments", "lastModifiedBy")
}

func TestC2CIncidentReceiverSourceRecordDropsTenantAuthorization(t *testing.T) {
	t.Parallel()

	const (
		canary         = "c2c-incident-receiver-psk-canary"
		standardCanary = "c2c-standard-canary"
	)
	receiver := c2cincidentreceiver.C2CIncidentReceiver{
		ID:                       616,
		Name:                     "C2C receiver",
		Status:                   []string{"ENABLED"},
		ModifiedTime:             1700001000,
		LastTenantValidationTime: 1700001100,
		LastValidationMsg:        &c2cincidentreceiver.LastValidationMsg{ErrorMsg: standardCanary, ErrorCode: "VALIDATION_FAILED"},
		LastModifiedBy:           &ziacommon.IDNameExtensions{ID: 111, Name: standardCanary},
		OnboardableEntity: &c2cincidentreceiver.OnboardableEntity{
			ID:                 112,
			Name:               "Onboardable entity",
			Type:               "INCIDENT_RECEIVER",
			EnterpriseTenantID: "tenant-ref-1",
			Application:        "ZSCALER_INCIDENT_RECEIVER",
			LastValidationMsg:  c2cincidentreceiver.LastValidationMsg{ErrorMsg: standardCanary, ErrorCode: "OK"},
			TenantAuthorizationInfo: c2cincidentreceiver.TenantAuthorizationInfo{
				AccessToken: canary,
				BotToken:    canary,
			},
			ZscalerAppTenantID: &ziacommon.IDNameExtensions{ID: 113, Name: "App tenant"},
		},
	}

	records := []resources.SourceRecord{c2cIncidentReceiverSourceRecord(receiver)}
	got := projectOneRecord(t, resources.ProductZIA, resourceC2CIncidentRcvs, records)
	assertNoCanaries(t, "c2c-incident-receivers", got, canary)
	assertAdminRef(t, "c2c-incident-receivers", got, "lastModifiedBy", 111, standardCanary)
	validation := mustProjectedMap(t, got, "lastValidationMsg")
	if validation["errorMsg"] != standardCanary || validation["errorCode"] != "VALIDATION_FAILED" {
		t.Errorf("projected c2c-incident-receivers lastValidationMsg = %#v, want errorMsg and errorCode", validation)
	}
	// The tenant authorization (access and bot tokens) never renders; the
	// validation message and admin name are standard-only.
	for mode, wider := range projectSourceRecordInModes(t, resourceC2CIncidentRcvs, records, canary, standardCanary) {
		label := "c2c-incident-receivers (" + string(mode) + ")"
		assertAdminRef(t, label, wider, "lastModifiedBy", 111, "")
		if validation := mustProjectedMap(t, wider, "lastValidationMsg"); validation["errorCode"] != "VALIDATION_FAILED" {
			t.Errorf("projected %s lastValidationMsg = %#v, want errorCode only", label, validation)
		}
	}
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
	if _, ok := entity["tenantAuthorizationInfo"]; ok {
		t.Errorf("projected c2c-incident-receivers onboardableEntity = %#v, want no tenantAuthorizationInfo", entity)
	}
	if validation := mustProjectedMap(t, entity, "lastValidationMsg"); validation["errorMsg"] != standardCanary || validation["errorCode"] != "OK" {
		t.Errorf("projected c2c-incident-receivers onboardableEntity.lastValidationMsg = %#v, want errorMsg and errorCode", validation)
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
		CloudAppRiskProfile:  &ziacommon.IDCustom{ID: 124, Name: "Risk profile"},
	}

	got := projectOneRecord(t, resources.ProductZIA, resourceCloudAppControl, []resources.SourceRecord{cloudAppControlSourceRecord(rule)})
	assertNoCanaries(t, "cloud-app-control", got, canary)
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
