package zscaler

// Wave-1 field-coverage promotion tests for zia/url-filtering-rules.
// Verifies that every newly promoted field projects in standard mode with
// the right key, that mode gating matches each field's AllowedModes, and
// that standard-only and excluded fields never leak outside standard mode.

import (
	"testing"

	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
	ziacommon "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/common"
	"github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/urlfilteringpolicies"
)

func TestURLFilteringRulePromotedFieldsAcrossModes(t *testing.T) {
	t.Parallel()

	const (
		adminCanary  = "wave1-url-admin-canary"
		cbiURLCanary = "https://isolate.invalid/wave1-cbi-url-canary"
	)

	rule := urlfilteringpolicies.URLFilteringRule{
		ID:                   4242,
		Name:                 "wave1-url-rule",
		State:                "ENABLED",
		Action:               "ISOLATE",
		BrowserEunTemplateID: 7707,
		Departments:          []ziacommon.IDNameExtensions{{ID: 6101, Name: "wave1-department"}},
		Groups:               []ziacommon.IDNameExtensions{{ID: 6102, Name: "wave1-group"}},
		Users:                []ziacommon.IDNameExtensions{{ID: 6103, Name: "wave1-user"}},
		DeviceGroups:         []ziacommon.IDNameExtensions{{ID: 6104, Name: "wave1-device-group"}},
		Devices:              []ziacommon.IDNameExtensions{{ID: 6105, Name: "wave1-device"}},
		OverrideUsers:        []ziacommon.IDNameExtensions{{ID: 6106, Name: "wave1-override-user"}},
		OverrideGroups:       []ziacommon.IDNameExtensions{{ID: 6107, Name: "wave1-override-group"}},
		CBIProfile: &ziacommon.CBIProfile{
			ID:         "wave1-cbi-id",
			Name:       "wave1-cbi-profile",
			URL:        cbiURLCanary,
			ProfileSeq: 3,
		},
		LastModifiedBy: &ziacommon.IDNameExtensions{ID: 9001, Name: adminCanary},
		// Promoted in wave 4 (tenant config, standard+share); mode gating is
		// asserted in TestURLFilteringRuleDeviceTrustLevelsModes below.
		DeviceTrustLevels: []string{"HIGH_TRUST"},
	}
	records := []resources.SourceRecord{urlFilteringRuleSourceRecord(rule)}

	standard := projectOneRecord(t, resources.ProductZIA, resourceURLRules, records)

	refLists := []struct {
		field string
		id    int
		name  string
	}{
		{"departments", 6101, "wave1-department"},
		{"groups", 6102, "wave1-group"},
		{"users", 6103, "wave1-user"},
		{"deviceGroups", 6104, "wave1-device-group"},
		{"devices", 6105, "wave1-device"},
		{"overrideUsers", 6106, "wave1-override-user"},
		{"overrideGroups", 6107, "wave1-override-group"},
	}
	for _, want := range refLists {
		items := mustProjectedList(t, standard, want.field)
		if len(items) != 1 {
			t.Fatalf("projected url-filtering-rules %s = %#v, want one entry", want.field, items)
		}
		entry, ok := items[0].(map[string]any)
		if !ok {
			t.Fatalf("projected url-filtering-rules %s[0] = %#v, want map", want.field, items[0])
		}
		if entry["id"] != want.id || toString(entry["name"]) != want.name {
			t.Errorf("projected url-filtering-rules %s[0] = %#v, want id %d name %q", want.field, entry, want.id, want.name)
		}
	}

	if standard["browserEunTemplateId"] != 7707 {
		t.Errorf("projected url-filtering-rules browserEunTemplateId = %#v, want 7707", standard["browserEunTemplateId"])
	}

	cbi, ok := standard["cbiProfile"].(map[string]any)
	if !ok {
		t.Fatalf("projected url-filtering-rules cbiProfile = %#v, want map", standard["cbiProfile"])
	}
	if toString(cbi["id"]) != "wave1-cbi-id" || toString(cbi["name"]) != "wave1-cbi-profile" || cbi["profileSeq"] != 3 {
		t.Errorf("projected url-filtering-rules cbiProfile = %#v, want id wave1-cbi-id name wave1-cbi-profile profileSeq 3", cbi)
	}
	if _, ok := cbi["url"]; !ok {
		t.Errorf("projected url-filtering-rules cbiProfile = %#v, want url (standard-only sensitive identifier)", cbi)
	}

	// The admin audit reference renders its id and admin name in standard
	// mode; outside standard only the id may survive (asserted below).
	lastModifiedBy := assertAuditRefID(t, "url-filtering-rules", standard, "lastModifiedBy", 9001)
	if lastModifiedBy["name"] != adminCanary {
		t.Errorf("projected url-filtering-rules lastModifiedBy = %#v, want name %q", lastModifiedBy, adminCanary)
	}

	// Mode gating: cbiProfile and browserEunTemplateId (a tenant-specific EUN
	// template reference, sensitiveIdentifierField) are standard-only and must
	// disappear in share and paranoid. The reference lists keep their ids (never
	// their names) in share and disappear in paranoid.
	standardOnlyFields := []string{"cbiProfile", "browserEunTemplateId"}
	for _, mode := range []redact.Mode{redact.ModeShare, redact.ModeParanoid} {
		got := projectOneRecordInMode(t, resources.ProductZIA, resourceURLRules, mode, records)
		for _, field := range standardOnlyFields {
			if _, ok := got[field]; ok {
				t.Errorf("projected url-filtering-rules (%v) = %#v, want no %s", mode, got, field)
			}
		}
		for _, want := range refLists {
			if mode == redact.ModeShare {
				assertRefListIDsOnly(t, "url-filtering-rules (share)", got, want.field, want.id)
			} else if _, ok := got[want.field]; ok {
				t.Errorf("projected url-filtering-rules (%v) = %#v, want no %s", mode, got, want.field)
			}
			assertNoCanaries(t, "url-filtering-rules ("+string(mode)+")", got, want.name)
		}
		assertAuditRefIDOnly(t, "url-filtering-rules ("+string(mode)+")", got, "lastModifiedBy", 9001)
		assertNoCanaries(t, "url-filtering-rules", got, adminCanary, cbiURLCanary)
	}
}

// TestURLFilteringRuleLastModifiedByAuditRefPin pins lastModifiedBy as an
// admin audit reference: the id renders in every mode, the admin name only in
// standard (see assertAuditRefPin in reader_fields_admin_identity_test.go).
func TestURLFilteringRuleLastModifiedByAuditRefPin(t *testing.T) {
	t.Parallel()

	const canary = "wave4-url-rule-last-modified-by-canary"
	rule := urlfilteringpolicies.URLFilteringRule{
		ID:    4405,
		Name:  "wave4 url rule",
		State: "ENABLED",
		LastModifiedBy: &ziacommon.IDNameExtensions{
			ID:   9108,
			Name: canary,
		},
	}
	records := []resources.SourceRecord{urlFilteringRuleSourceRecord(rule)}

	assertAuditRefPin(t, resourceURLRules, records,
		[]auditRef{{field: "lastModifiedBy", id: 9108}}, "id", canary)
}

// TestURLFilteringRuleDeviceTrustLevelsModes asserts deviceTrustLevels is
// tenantConfigField(standard+share): it mirrors the identical
// ssl-inspection-rules field.
func TestURLFilteringRuleDeviceTrustLevelsModes(t *testing.T) {
	t.Parallel()

	rule := urlfilteringpolicies.URLFilteringRule{
		ID:                4408,
		Name:              "wave4 url rule trust levels",
		State:             "ENABLED",
		DeviceTrustLevels: []string{"HIGH_TRUST", "MEDIUM_TRUST"},
	}
	records := []resources.SourceRecord{urlFilteringRuleSourceRecord(rule)}

	standard := projectOneRecord(t, resources.ProductZIA, resourceURLRules, records)
	levels, ok := standard["deviceTrustLevels"].([]string)
	if !ok || len(levels) != 2 || levels[0] != "HIGH_TRUST" || levels[1] != "MEDIUM_TRUST" {
		t.Errorf("projected url-filtering-rules deviceTrustLevels = %#v, want [HIGH_TRUST MEDIUM_TRUST]", standard["deviceTrustLevels"])
	}

	// tenantConfigField(standard+share): present in share, dropped in paranoid.
	share := projectOneRecordInMode(t, resources.ProductZIA, resourceURLRules, redact.ModeShare, records)
	if _, ok := share["deviceTrustLevels"]; !ok {
		t.Errorf("projected url-filtering-rules share = %#v, want deviceTrustLevels present", share)
	}
	paranoid := projectOneRecordInMode(t, resources.ProductZIA, resourceURLRules, redact.ModeParanoid, records)
	assertFieldsAbsent(t, "url-filtering-rules (paranoid)", paranoid, "deviceTrustLevels")
}
