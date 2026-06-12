package zscaler

// Field-coverage tests for zia/rule-labels: createdBy and lastModifiedBy are
// admin-identity references pinned as secretField, so the mapper emits them
// into the source record and projection must drop them in ALL modes (see
// assertWave4SecretPin in reader_fields_admin_identity_test.go).

import (
	"testing"

	ziacommon "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/common"
	rulelabels "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/rule_labels"

	"github.com/dvmrry/zscalerctl/internal/resources"
)

func TestRuleLabelAdminIdentitySecretPins(t *testing.T) {
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
