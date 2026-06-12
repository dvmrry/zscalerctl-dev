package zscaler

// Wave-1 field-coverage promotion tests for zia/url-filtering-rules.
// Verifies that every newly promoted field projects in standard mode with
// the right key, that mode gating matches each field's AllowedModes, and
// that secret/never-promoted and excluded fields never leak.

import (
	"testing"

	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
	ziacommon "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/common"
	"github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/urlfilteringpolicies"
)

func TestURLFilteringRuleWave1PromotedFieldsAcrossModes(t *testing.T) {
	t.Parallel()

	const (
		adminCanary    = "wave1-url-admin-canary"
		cbiURLCanary   = "https://isolate.invalid/wave1-cbi-url-canary"
		excludedCanary = "WAVE1-TRUST-LEVEL-CANARY"
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
		LastModifiedBy:    &ziacommon.IDNameExtensions{ID: 9001, Name: adminCanary},
		DeviceTrustLevels: []string{excludedCanary},
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
	if _, ok := cbi["url"]; ok {
		t.Errorf("projected url-filtering-rules cbiProfile = %#v, want no url (secret sub-field)", cbi)
	}

	// Never-promoted admin identity and the excluded deviceTrustLevels field
	// must stay out of standard output, and none of their canaries may leak.
	if _, ok := standard["lastModifiedBy"]; ok {
		t.Errorf("projected url-filtering-rules = %#v, want no lastModifiedBy", standard)
	}
	if _, ok := standard["deviceTrustLevels"]; ok {
		t.Errorf("projected url-filtering-rules = %#v, want no deviceTrustLevels (unclassified)", standard)
	}
	assertNoCanaries(t, "url-filtering-rules", standard, adminCanary, cbiURLCanary, excludedCanary)

	// Mode gating: every promoted field is standard-only — including
	// browserEunTemplateId, a tenant-specific EUN template reference
	// (sensitiveIdentifierField) — and must disappear in share and paranoid.
	standardOnlyFields := []string{
		"departments",
		"groups",
		"users",
		"deviceGroups",
		"devices",
		"overrideUsers",
		"overrideGroups",
		"cbiProfile",
		"browserEunTemplateId",
	}
	for _, mode := range []redact.Mode{redact.ModeShare, redact.ModeParanoid} {
		got := projectOneRecordInMode(t, resources.ProductZIA, resourceURLRules, mode, records)
		for _, field := range standardOnlyFields {
			if _, ok := got[field]; ok {
				t.Errorf("projected url-filtering-rules (%v) = %#v, want no %s", mode, got, field)
			}
		}
		if _, ok := got["lastModifiedBy"]; ok {
			t.Errorf("projected url-filtering-rules (%v) = %#v, want no lastModifiedBy", mode, got)
		}
		assertNoCanaries(t, "url-filtering-rules", got, adminCanary, cbiURLCanary, excludedCanary)
	}
}
