package zscaler

import (
	"testing"

	ziacommon "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/common"
	forwardingrules "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/forwarding_control_policy/forwarding_rules"

	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

const (
	wave1FwdAdminCanary      = "admin-canary-excluded"
	wave1FwdExternalIDCanary = "external-id-canary-9001"
)

// wave1ForwardingRuleFixture carries a distinctive value in every field
// promoted for zia/forwarding-rules in wave 1, plus canaries in the
// excluded lastModifiedBy reference and the secret externalId sub-field.
func wave1ForwardingRuleFixture() forwardingrules.ForwardingRules {
	return forwardingrules.ForwardingRules{
		ID:            707,
		Name:          "Wave1 forwarding rule",
		Type:          "FORWARDING",
		ForwardMethod: "ZPA",
		State:         "ENABLED",
		Departments: []ziacommon.IDNameExtensions{
			{ID: 7101, Name: "dept-canary-finance"},
		},
		Groups: []ziacommon.IDNameExtensions{
			{ID: 7102, Name: "group-canary-engineering"},
		},
		Users: []ziacommon.IDNameExtensions{
			{ID: 7103, Name: "user-canary-jdoe"},
		},
		DeviceGroups: []ziacommon.IDNameExtensions{
			{ID: 7104, Name: "devicegroup-canary-laptops"},
		},
		LastModifiedBy: &ziacommon.IDNameExtensions{
			ID:   9100,
			Name: wave1FwdAdminCanary,
		},
		ZPAAppSegments: []ziacommon.ZPAAppSegments{
			{ID: 7105, Name: "zpa-segment-canary", ExternalID: wave1FwdExternalIDCanary},
		},
		ZPAApplicationSegments: []forwardingrules.ZPAApplicationSegments{
			{
				ID:          7106,
				Name:        "zpa-app-segment-canary",
				Description: "wave1 app segment description canary",
				ZPAID:       7201,
				Deleted:     true,
			},
		},
		ZPAApplicationSegmentGroups: []forwardingrules.ZPAApplicationSegmentGroups{
			{
				ID:                  7107,
				Name:                "zpa-app-segment-group-canary",
				ZPAID:               7202,
				Deleted:             true,
				ZPAAppSegmentsCount: 42,
			},
		},
	}
}

func wave1ForwardingRuleRecords() []resources.SourceRecord {
	return []resources.SourceRecord{forwardingRuleSourceRecord(wave1ForwardingRuleFixture())}
}

func wave1ProjectedEntry(t *testing.T, record map[string]any, field string) map[string]any {
	t.Helper()

	items := mustProjectedList(t, record, field)
	if len(items) != 1 {
		t.Fatalf("projected forwarding-rules %s length = %d, want 1", field, len(items))
	}
	entry, ok := items[0].(map[string]any)
	if !ok {
		t.Fatalf("projected forwarding-rules %s[0] = %T, want map[string]any", field, items[0])
	}
	return entry
}

func TestWave1ForwardingRulesPromotedFieldsRenderInStandardMode(t *testing.T) {
	t.Parallel()

	got := projectOneRecordInMode(
		t, resources.ProductZIA, resourceForwardingRules, redact.ModeStandard, wave1ForwardingRuleRecords(),
	)

	wantNames := map[string]string{
		"departments":  "dept-canary-finance",
		"groups":       "group-canary-engineering",
		"users":        "user-canary-jdoe",
		"deviceGroups": "devicegroup-canary-laptops",
	}
	for field, wantName := range wantNames {
		entry := wave1ProjectedEntry(t, got, field)
		if entry["name"] != wantName {
			t.Errorf("projected forwarding-rules %s[0].name = %v, want %s", field, entry["name"], wantName)
		}
		if _, ok := entry["id"]; !ok {
			t.Errorf("projected forwarding-rules %s[0] missing id", field)
		}
	}

	segment := wave1ProjectedEntry(t, got, "zpaAppSegments")
	if segment["name"] != "zpa-segment-canary" {
		t.Errorf("projected forwarding-rules zpaAppSegments[0].name = %v, want zpa-segment-canary", segment["name"])
	}
	if _, ok := segment["id"]; !ok {
		t.Errorf("projected forwarding-rules zpaAppSegments[0] missing id")
	}
	if _, ok := segment["externalId"]; ok {
		t.Errorf("projected forwarding-rules zpaAppSegments[0] = %#v, want no externalId (secret)", segment)
	}

	appSegment := wave1ProjectedEntry(t, got, "zpaApplicationSegments")
	if appSegment["name"] != "zpa-app-segment-canary" {
		t.Errorf(
			"projected forwarding-rules zpaApplicationSegments[0].name = %v, want zpa-app-segment-canary",
			appSegment["name"],
		)
	}
	if appSegment["description"] != "wave1 app segment description canary" {
		t.Errorf(
			"projected forwarding-rules zpaApplicationSegments[0].description = %v, want fixture description",
			appSegment["description"],
		)
	}
	for _, sub := range []string{"id", "zpaId", "deleted"} {
		if _, ok := appSegment[sub]; !ok {
			t.Errorf("projected forwarding-rules zpaApplicationSegments[0] missing %s", sub)
		}
	}

	appSegmentGroup := wave1ProjectedEntry(t, got, "zpaApplicationSegmentGroups")
	if appSegmentGroup["name"] != "zpa-app-segment-group-canary" {
		t.Errorf(
			"projected forwarding-rules zpaApplicationSegmentGroups[0].name = %v, want zpa-app-segment-group-canary",
			appSegmentGroup["name"],
		)
	}
	for _, sub := range []string{"id", "zpaId", "deleted", "zpaAppSegmentsCount"} {
		if _, ok := appSegmentGroup[sub]; !ok {
			t.Errorf("projected forwarding-rules zpaApplicationSegmentGroups[0] missing %s", sub)
		}
	}
}

func TestWave1ForwardingRulesPromotedFieldsFollowAllowedModes(t *testing.T) {
	t.Parallel()

	records := wave1ForwardingRuleRecords()
	standard := projectOneRecordInMode(t, resources.ProductZIA, resourceForwardingRules, redact.ModeStandard, records)
	share := projectOneRecordInMode(t, resources.ProductZIA, resourceForwardingRules, redact.ModeShare, records)
	paranoid := projectOneRecordInMode(t, resources.ProductZIA, resourceForwardingRules, redact.ModeParanoid, records)

	// Every promoted field is standard-only: present in standard, dropped in
	// share and paranoid, exactly per its AllowedModes.
	promoted := []string{
		"departments",
		"groups",
		"users",
		"deviceGroups",
		"zpaAppSegments",
		"zpaApplicationSegments",
		"zpaApplicationSegmentGroups",
	}
	for _, field := range promoted {
		if _, ok := standard[field]; !ok {
			t.Errorf("projected forwarding-rules standard mode missing %s", field)
		}
		if _, ok := share[field]; ok {
			t.Errorf("projected forwarding-rules share mode = %#v, want no %s", share, field)
		}
		if _, ok := paranoid[field]; ok {
			t.Errorf("projected forwarding-rules paranoid mode = %#v, want no %s", paranoid, field)
		}
	}

	// lastModifiedBy is classified secret and must never render, and no
	// canary value from an excluded or secret field may leak in any mode.
	for mode, record := range map[string]map[string]any{
		"standard": standard,
		"share":    share,
		"paranoid": paranoid,
	} {
		if _, ok := record["lastModifiedBy"]; ok {
			t.Errorf("projected forwarding-rules %s mode = %#v, want no lastModifiedBy", mode, record)
		}
		assertNoCanaries(t, "forwarding-rules "+mode, record, wave1FwdAdminCanary, wave1FwdExternalIDCanary)
	}
}
