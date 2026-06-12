package zscaler

// Wave-2 field-coverage promotion tests for zia/cloud-app-control.
// Verifies that every newly promoted field (user/group/device reference
// lists plus the nested cbiProfile, cloudAppRiskProfile, and
// cloudAppInstances structures) projects in standard mode with the right
// key and sub-fields, that mode gating matches each field's AllowedModes,
// and that secret sub-fields (cbiProfile.url, reference extensions) never
// leak in any mode.

import (
	"testing"

	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
	"github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/cloudappcontrol"
	ziacommon "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/common"
)

func TestCloudAppControlPromotedFieldsAcrossModes(t *testing.T) {
	t.Parallel()

	const (
		cbiURLCanary    = "https://isolate.invalid/wave2-cac-cbi-url-canary"
		extensionCanary = "wave2-cac-extension-canary"
	)

	rule := cloudappcontrol.WebApplicationRules{
		ID:    8181,
		Name:  "wave2-cac-rule",
		State: "ENABLED",
		Type:  "STREAMING_MEDIA",
		Groups: []ziacommon.IDNameExtensions{{
			ID:         7101,
			Name:       "wave2-cac-group",
			Extensions: map[string]any{"note": extensionCanary},
		}},
		Users:        []ziacommon.IDNameExtensions{{ID: 7102, Name: "wave2-cac-user"}},
		Departments:  []ziacommon.IDNameExtensions{{ID: 7103, Name: "wave2-cac-department"}},
		Devices:      []ziacommon.IDNameExtensions{{ID: 7104, Name: "wave2-cac-device"}},
		DeviceGroups: []ziacommon.IDNameExtensions{{ID: 7105, Name: "wave2-cac-device-group"}},
		CloudAppInstances: []cloudappcontrol.CloudAppInstances{{
			ID:   7106,
			Name: "wave2-cac-instance",
			Type: "SHAREPOINTONLINE",
		}},
		CloudAppRiskProfile: &ziacommon.IDCustom{ID: 7107, Name: "wave2-cac-risk-profile"},
		CBIProfile: cloudappcontrol.CBIProfile{
			ID:             "wave2-cac-cbi-id",
			Name:           "wave2-cac-cbi-profile",
			URL:            cbiURLCanary,
			ProfileSeq:     5,
			DefaultProfile: true,
			SandboxMode:    true,
		},
	}
	records := []resources.SourceRecord{cloudAppControlSourceRecord(rule)}

	standard := projectOneRecord(t, resources.ProductZIA, resourceCloudAppControl, records)

	refLists := []struct {
		field string
		id    int
		name  string
	}{
		{"groups", 7101, "wave2-cac-group"},
		{"users", 7102, "wave2-cac-user"},
		{"departments", 7103, "wave2-cac-department"},
		{"devices", 7104, "wave2-cac-device"},
		{"deviceGroups", 7105, "wave2-cac-device-group"},
	}
	for _, want := range refLists {
		items := mustProjectedList(t, standard, want.field)
		if len(items) != 1 {
			t.Fatalf("projected cloud-app-control %s = %#v, want one entry", want.field, items)
		}
		entry, ok := items[0].(map[string]any)
		if !ok {
			t.Fatalf("projected cloud-app-control %s[0] = %#v, want map", want.field, items[0])
		}
		if entry["id"] != want.id || toString(entry["name"]) != want.name {
			t.Errorf("projected cloud-app-control %s[0] = %#v, want id %d name %q", want.field, entry, want.id, want.name)
		}
	}

	instances := mustProjectedList(t, standard, "cloudAppInstances")
	if len(instances) != 1 {
		t.Fatalf("projected cloud-app-control cloudAppInstances = %#v, want one entry", instances)
	}
	instance, ok := instances[0].(map[string]any)
	if !ok {
		t.Fatalf("projected cloud-app-control cloudAppInstances[0] = %#v, want map", instances[0])
	}
	if instance["id"] != 7106 || toString(instance["name"]) != "wave2-cac-instance" || toString(instance["type"]) != "SHAREPOINTONLINE" {
		t.Errorf("projected cloud-app-control cloudAppInstances[0] = %#v, want id 7106 name wave2-cac-instance type SHAREPOINTONLINE", instance)
	}

	risk, ok := standard["cloudAppRiskProfile"].(map[string]any)
	if !ok {
		t.Fatalf("projected cloud-app-control cloudAppRiskProfile = %#v, want map", standard["cloudAppRiskProfile"])
	}
	if risk["id"] != 7107 || toString(risk["name"]) != "wave2-cac-risk-profile" {
		t.Errorf("projected cloud-app-control cloudAppRiskProfile = %#v, want id 7107 name wave2-cac-risk-profile", risk)
	}

	cbi, ok := standard["cbiProfile"].(map[string]any)
	if !ok {
		t.Fatalf("projected cloud-app-control cbiProfile = %#v, want map", standard["cbiProfile"])
	}
	if toString(cbi["id"]) != "wave2-cac-cbi-id" || toString(cbi["name"]) != "wave2-cac-cbi-profile" || cbi["profileSeq"] != 5 {
		t.Errorf("projected cloud-app-control cbiProfile = %#v, want id wave2-cac-cbi-id name wave2-cac-cbi-profile profileSeq 5", cbi)
	}
	if cbi["defaultProfile"] != true || cbi["sandboxMode"] != true {
		t.Errorf("projected cloud-app-control cbiProfile = %#v, want defaultProfile true sandboxMode true", cbi)
	}
	if _, ok := cbi["url"]; ok {
		t.Errorf("projected cloud-app-control cbiProfile = %#v, want no url (secret sub-field)", cbi)
	}

	// Secret sub-fields must stay out of standard output too: the CBI
	// profile URL and the reference-list extensions map never project.
	assertNoCanaries(t, "cloud-app-control", standard, cbiURLCanary, extensionCanary)

	// Mode gating: every promoted parent field is standard-only and must
	// disappear in share and paranoid, taking all nested canaries with it.
	standardOnlyFields := []string{
		"groups",
		"users",
		"departments",
		"devices",
		"deviceGroups",
		"cloudAppInstances",
		"cloudAppRiskProfile",
		"cbiProfile",
	}
	for _, mode := range []redact.Mode{redact.ModeShare, redact.ModeParanoid} {
		got := projectOneRecordInMode(t, resources.ProductZIA, resourceCloudAppControl, mode, records)
		for _, field := range standardOnlyFields {
			if _, ok := got[field]; ok {
				t.Errorf("projected cloud-app-control (%v) = %#v, want no %s", mode, got, field)
			}
		}
		assertNoCanaries(t, "cloud-app-control", got,
			cbiURLCanary,
			extensionCanary,
			"wave2-cac-group",
			"wave2-cac-user",
			"wave2-cac-department",
			"wave2-cac-device",
			"wave2-cac-instance",
			"wave2-cac-risk-profile",
			"wave2-cac-cbi-id",
			"wave2-cac-cbi-profile",
		)
	}

	// Positive mode contrast: the rule name (tenantConfig, standard+share)
	// survives share mode but drops in paranoid.
	share := projectOneRecordInMode(t, resources.ProductZIA, resourceCloudAppControl, redact.ModeShare, records)
	if toString(share["name"]) != "wave2-cac-rule" {
		t.Errorf("projected cloud-app-control (share) name = %#v, want wave2-cac-rule", share["name"])
	}
	paranoid := projectOneRecordInMode(t, resources.ProductZIA, resourceCloudAppControl, redact.ModeParanoid, records)
	if _, ok := paranoid["name"]; ok {
		t.Errorf("projected cloud-app-control (paranoid) = %#v, want no name", paranoid)
	}
}
