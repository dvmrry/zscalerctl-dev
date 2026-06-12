package zscaler

// Wave-1 field-coverage tests for zia/firewall-filtering-rules. The mapper
// (firewallFilteringRuleSourceRecord) already emitted departments, groups,
// users, deviceGroups, devices, and zpaAppSegments; this wave promotes them in
// the catalog. These tests build the SDK struct with distinctive values in
// every promoted field, project through the catalog, and assert that (a) each
// promoted field appears in standard mode under the right key, (b) the
// standard-only promoted fields drop in share and paranoid per their
// AllowedModes while wider-mode fields survive, (c) never-promoted secret
// fields (lastModifiedBy, nested extensions/externalId) stay absent, and
// (d) no canary planted in an excluded field leaks into any projection.
//
// The file reuses shared helpers from this package (projectOneRecord,
// projectOneRecordInMode, mustProjectedList, assertNoCanaries) instead of
// editing the shared test files.

import (
	"testing"

	ziacommon "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/common"
	filteringrules "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/firewallpolicies/filteringrules"

	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

func wave1FirewallFilteringRuleFixture(adminCanary, extensionsCanary, externalIDCanary string) filteringrules.FirewallFilteringRules {
	return filteringrules.FirewallFilteringRules{
		ID:     7401,
		Name:   "Wave1 firewall rule",
		State:  "ENABLED",
		Order:  3,
		Rank:   5,
		Action: "BLOCK_DROP",
		LastModifiedBy: &ziacommon.IDNameExtensions{
			ID:   9001,
			Name: adminCanary,
		},
		Labels: []ziacommon.IDNameExtensions{
			{ID: 7410, Name: "Wave1 label"},
		},
		Departments: []ziacommon.IDNameExtensions{
			{ID: 7411, Name: "Engineering", Extensions: map[string]any{"leak": extensionsCanary}},
		},
		Groups: []ziacommon.IDNameExtensions{
			{ID: 7412, Name: "Contractors"},
		},
		Users: []ziacommon.IDNameExtensions{
			{ID: 7413, Name: "user@example.invalid"},
		},
		DeviceGroups: []ziacommon.IDNameExtensions{
			{ID: 7414, Name: "Managed laptops"},
		},
		Devices: []ziacommon.IDNameExtensions{
			{ID: 7415, Name: "build-host-01"},
		},
		ZPAAppSegments: []ziacommon.ZPAAppSegments{
			{ID: 7416, Name: "Internal wiki", ExternalID: externalIDCanary},
		},
	}
}

// mustFirstWave1Item returns the first element of a projected reference list
// as a map. Local to this file to avoid touching shared test files.
func mustFirstWave1Item(t *testing.T, record map[string]any, field string) map[string]any {
	t.Helper()

	items := mustProjectedList(t, record, field)
	object, ok := items[0].(map[string]any)
	if !ok {
		t.Fatalf("projected record %s[0] = %T, want map[string]any", field, items[0])
	}
	return object
}

func TestFirewallFilteringRulePromotedFieldsProjectInStandardMode(t *testing.T) {
	t.Parallel()

	const (
		adminCanary      = "ffr-last-modified-by-canary"
		extensionsCanary = "ffr-extensions-secret-canary"
		externalIDCanary = "ffr-external-id-secret-canary"
	)
	rule := wave1FirewallFilteringRuleFixture(adminCanary, extensionsCanary, externalIDCanary)

	got := projectOneRecord(t, resources.ProductZIA, resourceFirewallRules, []resources.SourceRecord{firewallFilteringRuleSourceRecord(rule)})

	wantItems := map[string]struct {
		id   int
		name string
	}{
		"departments":    {id: 7411, name: "Engineering"},
		"groups":         {id: 7412, name: "Contractors"},
		"users":          {id: 7413, name: "user@example.invalid"},
		"deviceGroups":   {id: 7414, name: "Managed laptops"},
		"devices":        {id: 7415, name: "build-host-01"},
		"zpaAppSegments": {id: 7416, name: "Internal wiki"},
	}
	for field, want := range wantItems {
		item := mustFirstWave1Item(t, got, field)
		if item["id"] != want.id {
			t.Errorf("projected firewall-filtering-rules %s[0].id = %v, want %d", field, item["id"], want.id)
		}
		if item["name"] != want.name {
			t.Errorf("projected firewall-filtering-rules %s[0].name = %v, want %s", field, item["name"], want.name)
		}
	}

	// Secret-classified nested sub-fields never survive projection.
	departments := mustFirstWave1Item(t, got, "departments")
	if _, ok := departments["extensions"]; ok {
		t.Errorf("projected firewall-filtering-rules departments[0] = %#v, want no extensions", departments)
	}
	zpaAppSegments := mustFirstWave1Item(t, got, "zpaAppSegments")
	if _, ok := zpaAppSegments["externalId"]; ok {
		t.Errorf("projected firewall-filtering-rules zpaAppSegments[0] = %#v, want no externalId", zpaAppSegments)
	}

	// Never-promoted admin identity stays absent.
	if _, ok := got["lastModifiedBy"]; ok {
		t.Errorf("projected firewall-filtering-rules = %#v, want no lastModifiedBy", got)
	}

	// No canary planted in an excluded or secret field leaks anywhere.
	assertNoCanaries(t, "firewall-filtering-rules standard", got, adminCanary, extensionsCanary, externalIDCanary)
}

func TestFirewallFilteringRulePromotedFieldsHonorModeBoundaries(t *testing.T) {
	t.Parallel()

	const (
		adminCanary      = "ffr-last-modified-by-canary"
		extensionsCanary = "ffr-extensions-secret-canary"
		externalIDCanary = "ffr-external-id-secret-canary"
	)
	rule := wave1FirewallFilteringRuleFixture(adminCanary, extensionsCanary, externalIDCanary)
	records := []resources.SourceRecord{firewallFilteringRuleSourceRecord(rule)}

	promoted := []string{"departments", "groups", "users", "deviceGroups", "devices", "zpaAppSegments"}

	standard := projectOneRecordInMode(t, resources.ProductZIA, resourceFirewallRules, redact.ModeStandard, records)
	for _, field := range promoted {
		if _, ok := standard[field]; !ok {
			t.Errorf("standard projected firewall-filtering-rules missing %s", field)
		}
	}

	// All promoted fields are standard-only: they must drop in share and
	// paranoid while wider-mode fields survive per their AllowedModes.
	share := projectOneRecordInMode(t, resources.ProductZIA, resourceFirewallRules, redact.ModeShare, records)
	for _, field := range promoted {
		if _, ok := share[field]; ok {
			t.Errorf("share projected firewall-filtering-rules includes %s, want dropped", field)
		}
	}
	if _, ok := share["labels"]; !ok {
		t.Errorf("share projected firewall-filtering-rules missing labels, want kept (standard+share)")
	}
	if _, ok := share["name"]; !ok {
		t.Errorf("share projected firewall-filtering-rules missing name, want kept (standard+share)")
	}
	assertNoCanaries(t, "firewall-filtering-rules share", share, adminCanary, extensionsCanary, externalIDCanary)

	paranoid := projectOneRecordInMode(t, resources.ProductZIA, resourceFirewallRules, redact.ModeParanoid, records)
	for _, field := range promoted {
		if _, ok := paranoid[field]; ok {
			t.Errorf("paranoid projected firewall-filtering-rules includes %s, want dropped", field)
		}
	}
	if _, ok := paranoid["labels"]; ok {
		t.Errorf("paranoid projected firewall-filtering-rules includes labels, want dropped (standard+share only)")
	}
	if _, ok := paranoid["state"]; !ok {
		t.Errorf("paranoid projected firewall-filtering-rules missing state, want kept (all modes)")
	}
	assertNoCanaries(t, "firewall-filtering-rules paranoid", paranoid, adminCanary, extensionsCanary, externalIDCanary)
}

// TestFirewallFilteringRuleLastModifiedBySecretPin pins lastModifiedBy as
// secretField: the admin identity must drop in every mode (see
// assertWave4SecretPin in reader_fields_admin_identity_test.go).
func TestFirewallFilteringRuleLastModifiedBySecretPin(t *testing.T) {
	t.Parallel()

	const canary = "wave4-firewall-rule-last-modified-by-canary"
	rule := filteringrules.FirewallFilteringRules{
		ID:    4406,
		Name:  "wave4 firewall rule",
		State: "ENABLED",
		LastModifiedBy: &ziacommon.IDNameExtensions{
			ID:   9109,
			Name: canary,
		},
	}
	records := []resources.SourceRecord{firewallFilteringRuleSourceRecord(rule)}

	assertWave4SecretPin(t, resourceFirewallRules, records,
		[]string{"lastModifiedBy"}, "id", canary)
}
