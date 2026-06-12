package zscaler

// Wave-1 field-coverage decision record for zia/workload-groups.
//
// The only Wave-1 candidate, expressionJson, is EXCLUDED FOREVER rather than
// promoted: the SDK documents it as the workload-group expression "represented
// in a JSON format", i.e. a structured twin of the already-classified
// expression string (sensitiveIdentifierField, standard-only). Modeling the
// tree would either duplicate that standard-only exposure or widen it into
// share mode. These tests pin the exclusion and the existing mode boundaries
// so the decision cannot regress silently.

import (
	"testing"

	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"

	ziacommon "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/common"
	"github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/workloadgroups"
)

func wave1WorkloadGroupFixture() (workloadgroups.WorkloadGroup, []string) {
	const (
		adminCanary    = "wave1-wlg-admin-canary"
		tagTypeCanary  = "wave1-wlg-tagtype-canary"
		tagKeyCanary   = "wave1-wlg-tagkey-canary"
		tagValueCanary = "wave1-wlg-tagvalue-canary"
	)
	group := workloadgroups.WorkloadGroup{
		ID:               4271,
		Name:             "wave1 workload group",
		Description:      "wave1 workload group description",
		Expression:       "TAG.team.platform AND TAG.env.staging",
		LastModifiedTime: 1717900000,
		LastModifiedBy: &ziacommon.IDNameExtensions{
			ID:   9001,
			Name: adminCanary,
		},
		WorkloadTagExpression: workloadgroups.WorkloadTagExpression{
			ExpressionContainers: []workloadgroups.ExpressionContainer{
				{
					TagType:  tagTypeCanary,
					Operator: "AND",
					TagContainer: workloadgroups.TagContainer{
						Operator: "OR",
						Tags: []workloadgroups.Tags{
							{Key: tagKeyCanary, Value: tagValueCanary},
						},
					},
				},
			},
		},
	}
	return group, []string{adminCanary, tagTypeCanary, tagKeyCanary, tagValueCanary}
}

func TestWorkloadGroupsStandardProjectsClassifiedFieldsOnly(t *testing.T) {
	t.Parallel()

	group, canaries := wave1WorkloadGroupFixture()
	got := projectOneRecordInMode(
		t,
		resources.ProductZIA,
		resourceWorkloadGroups,
		redact.ModeStandard,
		[]resources.SourceRecord{workloadGroupSourceRecord(group)},
	)

	want := map[string]any{
		"id":               4271,
		"name":             "wave1 workload group",
		"description":      "wave1 workload group description",
		"expression":       "TAG.team.platform AND TAG.env.staging",
		"lastModifiedTime": 1717900000,
	}
	for field, value := range want {
		if got[field] != value {
			t.Errorf("standard projected workload-groups %s = %v, want %v", field, got[field], value)
		}
	}
	for _, field := range []string{"expressionJson", "lastModifiedBy"} {
		if _, ok := got[field]; ok {
			t.Errorf("standard projected workload-groups includes %s, want dropped", field)
		}
	}
	assertNoCanaries(t, "workload-groups standard", got, canaries...)
}

func TestWorkloadGroupsModeBoundaries(t *testing.T) {
	t.Parallel()

	group, canaries := wave1WorkloadGroupFixture()
	records := []resources.SourceRecord{workloadGroupSourceRecord(group)}

	tests := []struct {
		mode    redact.Mode
		present []string
		absent  []string
	}{
		{
			// name/lastModifiedTime keep standardShareModes; the
			// sensitive-identifier expression and free-text description drop.
			mode:    redact.ModeShare,
			present: []string{"id", "name", "lastModifiedTime"},
			absent:  []string{"expression", "description", "expressionJson", "lastModifiedBy"},
		},
		{
			// Only the operational id survives paranoid mode.
			mode:    redact.ModeParanoid,
			present: []string{"id"},
			absent:  []string{"name", "lastModifiedTime", "expression", "description", "expressionJson", "lastModifiedBy"},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(string(tc.mode), func(t *testing.T) {
			t.Parallel()

			got := projectOneRecordInMode(t, resources.ProductZIA, resourceWorkloadGroups, tc.mode, records)
			for _, field := range tc.present {
				if _, ok := got[field]; !ok {
					t.Errorf("%s projected workload-groups missing %s", tc.mode, field)
				}
			}
			for _, field := range tc.absent {
				if _, ok := got[field]; ok {
					t.Errorf("%s projected workload-groups includes %s, want dropped", tc.mode, field)
				}
			}
			assertNoCanaries(t, "workload-groups "+string(tc.mode), got, canaries...)
		})
	}
}

// TestWorkloadGroupLastModifiedBySecretPin pins lastModifiedBy as secretField:
// the admin identity must drop in every mode (see assertWave4SecretPin in
// reader_fields_admin_identity_test.go).
func TestWorkloadGroupLastModifiedBySecretPin(t *testing.T) {
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
