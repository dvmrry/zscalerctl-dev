package zscaler

// Wave-4 field-coverage tests: the mechanical finish that drives the last
// deferred fields on seven ZIA resources to a decided state.
//
//   - Admin-identity references (rule-labels createdBy/lastModifiedBy,
//     workload-groups lastModifiedBy, static-ips managedBy/lastModifiedBy,
//     gre-tunnels managedBy/lastModifiedBy, url-filtering-rules
//     lastModifiedBy, location-groups lastModUser, firewall-filtering-rules
//     lastModifiedBy) are pinned as secretField: every mapper emits them into
//     the source record, and projection must drop them in ALL modes. Each
//     test plants a canary in the admin identity and asserts the field key is
//     absent and the canary never survives, with a control field present to
//     prove the projection ran.
//   - url-filtering-rules deviceTrustLevels mirrors the identical
//     ssl-inspection-rules field: tenantConfigField(standard+share).
//   - ssl-inspection-rules accessControl is a flat admin-RBA privilege enum,
//     classified operationalField(standard+share) per the
//     firewall-filtering-rules precedent.
//
// Helpers (projectOneRecord, projectOneRecordInMode, assertNoCanaries,
// assertFieldsAbsent) are reused from reader_test.go and
// reader_sourcerecord_test.go in this package.

import (
	"testing"

	ziacommon "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/common"
	filteringrules "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/firewallpolicies/filteringrules"
	"github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/location/locationgroups"
	rulelabels "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/rule_labels"
	"github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/sslinspection"
	gretunnels "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/trafficforwarding/gretunnels"
	staticips "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/trafficforwarding/staticips"
	"github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/urlfilteringpolicies"
	"github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/workloadgroups"

	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

var wave4AllModes = []redact.Mode{redact.ModeStandard, redact.ModeShare, redact.ModeParanoid}

// assertWave4SecretPin projects the records in every mode and asserts that
// each secret-pinned field is absent, that none of the canaries leak, and
// that the control field is present (so an empty projection cannot pass).
func assertWave4SecretPin(
	t *testing.T,
	resourceName string,
	records []resources.SourceRecord,
	secretFields []string,
	controlField string,
	canaries ...string,
) {
	t.Helper()

	for _, mode := range wave4AllModes {
		got := projectOneRecordInMode(t, resources.ProductZIA, resourceName, mode, records)
		if _, ok := got[controlField]; !ok {
			t.Errorf("projected %s (%v) = %#v, want control field %s present", resourceName, mode, got, controlField)
		}
		assertFieldsAbsent(t, resourceName+" ("+string(mode)+")", got, secretFields...)
		assertNoCanaries(t, resourceName+" ("+string(mode)+")", got, canaries...)
	}
}

func TestWave4LocationGroupLastModUserSecretPin(t *testing.T) {
	t.Parallel()

	const canary = "wave4-locgroup-lastmoduser-canary"
	group := locationgroups.LocationGroup{
		ID:        4401,
		Name:      "wave4 location group",
		GroupType: "STATIC",
		LastModUser: &locationgroups.LastModUser{
			ID:   9101,
			Name: canary,
		},
	}
	records := []resources.SourceRecord{locationGroupSourceRecord(group)}

	assertWave4SecretPin(t, resourceLocationGroups, records,
		[]string{"lastModUser"}, "id", canary)
}

func TestWave4RuleLabelAdminIdentitySecretPins(t *testing.T) {
	t.Parallel()

	const (
		createdByCanary      = "wave4-rule-label-created-by-canary"
		lastModifiedByCanary = "wave4-rule-label-last-modified-by-canary"
	)
	label := rulelabels.RuleLabels{
		ID:                  4402,
		Name:                "wave4 rule label",
		ReferencedRuleCount: 3,
		CreatedBy: &ziacommon.IDNameExtensions{
			ID:   9102,
			Name: createdByCanary,
		},
		LastModifiedBy: &ziacommon.IDNameExtensions{
			ID:   9103,
			Name: lastModifiedByCanary,
		},
	}
	records := []resources.SourceRecord{ruleLabelSourceRecord(label)}

	assertWave4SecretPin(t, resourceRuleLabels, records,
		[]string{"createdBy", "lastModifiedBy"}, "id",
		createdByCanary, lastModifiedByCanary)
}

func TestWave4StaticIPAdminIdentitySecretPins(t *testing.T) {
	t.Parallel()

	const (
		managedByCanary      = "wave4-static-ip-managed-by-canary"
		lastModifiedByCanary = "wave4-static-ip-last-modified-by-canary"
	)
	staticIP := staticips.StaticIP{
		ID:         4403,
		IpAddress:  "203.0.113.44",
		RoutableIP: true,
		ManagedBy: &staticips.ManagedBy{
			ID:   9104,
			Name: managedByCanary,
		},
		LastModifiedBy: &staticips.LastModifiedBy{
			ID:   9105,
			Name: lastModifiedByCanary,
		},
	}
	records := []resources.SourceRecord{staticIPSourceRecord(staticIP)}

	assertWave4SecretPin(t, resourceStaticIPs, records,
		[]string{"managedBy", "lastModifiedBy"}, "id",
		managedByCanary, lastModifiedByCanary)
}

func TestWave4GRETunnelAdminIdentitySecretPins(t *testing.T) {
	t.Parallel()

	const (
		managedByCanary      = "wave4-gre-tunnel-managed-by-canary"
		lastModifiedByCanary = "wave4-gre-tunnel-last-modified-by-canary"
	)
	tunnel := gretunnels.GreTunnels{
		ID:       4404,
		SourceIP: "198.51.100.44",
		ManagedBy: &gretunnels.ManagedBy{
			ID:   9106,
			Name: managedByCanary,
		},
		LastModifiedBy: &gretunnels.LastModifiedBy{
			ID:   9107,
			Name: lastModifiedByCanary,
		},
	}
	records := []resources.SourceRecord{greTunnelSourceRecord(tunnel)}

	assertWave4SecretPin(t, resourceGRETunnels, records,
		[]string{"managedBy", "lastModifiedBy"}, "id",
		managedByCanary, lastModifiedByCanary)
}

func TestWave4URLFilteringRuleLastModifiedBySecretPin(t *testing.T) {
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

	assertWave4SecretPin(t, resourceURLRules, records,
		[]string{"lastModifiedBy"}, "id", canary)
}

func TestWave4FirewallFilteringRuleLastModifiedBySecretPin(t *testing.T) {
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

func TestWave4WorkloadGroupLastModifiedBySecretPin(t *testing.T) {
	t.Parallel()

	const canary = "wave4-workload-group-last-modified-by-canary"
	group := workloadgroups.WorkloadGroup{
		ID:   4407,
		Name: "wave4 workload group",
		LastModifiedBy: &ziacommon.IDNameExtensions{
			ID:   9110,
			Name: canary,
		},
	}
	records := []resources.SourceRecord{workloadGroupSourceRecord(group)}

	assertWave4SecretPin(t, resourceWorkloadGroups, records,
		[]string{"lastModifiedBy"}, "id", canary)
}

func TestWave4URLFilteringRuleDeviceTrustLevelsModes(t *testing.T) {
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

func TestWave4SSLInspectionRuleAccessControlModes(t *testing.T) {
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
