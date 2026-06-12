package zscaler

import (
	"testing"

	ipssignaturerules "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/ips_control_policies/ips_signature_rules"

	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

// wave1IPSSignatureRule builds an SDK rule with a distinctive value in every
// wave-1 promoted field (category) and in the secret-classified ruleText, plus
// distinctive values in the already-classified fields used for mode-transition
// assertions.
func wave1IPSSignatureRule(ruleTextCanary string) ipssignaturerules.IPSSignatureRules {
	return ipssignaturerules.IPSSignatureRules{
		ID:          4242,
		Name:        "Wave1 signature rule",
		RuleText:    ruleTextCanary,
		Description: "Wave1 signature rule description",
		Category: &ipssignaturerules.IPSSignatureCategory{
			ID:            77,
			Name:          "WAVE1_THREAT_CATEGORY",
			IsNameL10nTag: true,
		},
		Enabled:                    true,
		Deleted:                    false,
		PromoteTime:                1700009900,
		RuleTextModTime:            1700009910,
		DynamicValidationSubmitted: true,
		DynamicValidationSucceeded: true,
		DynamicValRejectCode:       3,
	}
}

func TestIPSSignatureRuleCategoryProjectsInStandardMode(t *testing.T) {
	t.Parallel()

	const ruleTextCanary = "wave1-ruletext-canary-never-emit"
	rule := wave1IPSSignatureRule(ruleTextCanary)
	records := []resources.SourceRecord{ipsSignatureRuleSourceRecord(rule)}

	got := projectOneRecord(t, resources.ProductZIA, resourceIPSSignatureRules, records)

	// (a) The promoted category field appears in standard mode under its
	// catalog key, with every modeled sub-field present.
	category := mustProjectedMap(t, got, "category")
	if category["id"] != 77 {
		t.Errorf("projected ips-signature-rules category.id = %v, want 77", category["id"])
	}
	if category["name"] != "WAVE1_THREAT_CATEGORY" {
		t.Errorf("projected ips-signature-rules category.name = %v, want WAVE1_THREAT_CATEGORY", category["name"])
	}
	if category["isNameL10nTag"] != true {
		t.Errorf("projected ips-signature-rules category.isNameL10nTag = %v, want true", category["isNameL10nTag"])
	}

	// (c)+(d) ruleText is catalog-classified as a secret and deliberately
	// unmapped: the key and the canary value must both be absent even in
	// standard mode.
	assertFieldsAbsent(t, "ips-signature-rules", got, "ruleText")
	assertNoCanaries(t, "ips-signature-rules", got, ruleTextCanary)
}

func TestIPSSignatureRuleModeTransitions(t *testing.T) {
	t.Parallel()

	const ruleTextCanary = "wave1-ruletext-canary-never-emit"
	rule := wave1IPSSignatureRule(ruleTextCanary)
	records := []resources.SourceRecord{ipsSignatureRuleSourceRecord(rule)}

	// (b) Mode transitions exactly per AllowedModes. The wave-1 promotion adds
	// only category (standard-only) and ruleText (secret), so the share and
	// paranoid transitions of the standard+share fields name/promoteTime and
	// the all-modes fields id/enabled anchor the per-mode contract around the
	// promoted fields.
	share := projectOneRecordInMode(t, resources.ProductZIA, resourceIPSSignatureRules, redact.ModeShare, records)
	// category is standard-only: dropped in share. description (free text) is
	// standard-only too, and ruleText never renders anywhere.
	assertFieldsAbsent(t, "ips-signature-rules", share, "category", "ruleText", "description")
	if share["name"] != "Wave1 signature rule" {
		t.Errorf("share-mode ips-signature-rules name = %v, want Wave1 signature rule", share["name"])
	}
	if share["promoteTime"] != 1700009900 {
		t.Errorf("share-mode ips-signature-rules promoteTime = %v, want 1700009900", share["promoteTime"])
	}
	if share["enabled"] != true {
		t.Errorf("share-mode ips-signature-rules enabled = %v, want true", share["enabled"])
	}
	assertNoCanaries(t, "ips-signature-rules", share, ruleTextCanary)

	paranoid := projectOneRecordInMode(t, resources.ProductZIA, resourceIPSSignatureRules, redact.ModeParanoid, records)
	// Paranoid keeps only all-modes operational fields: the promoted category
	// stays dropped, and standard+share fields disappear as well.
	assertFieldsAbsent(t, "ips-signature-rules", paranoid,
		"category", "ruleText", "description", "name", "promoteTime", "ruleTextModTime")
	if paranoid["id"] != 4242 {
		t.Errorf("paranoid-mode ips-signature-rules id = %v, want 4242", paranoid["id"])
	}
	if paranoid["enabled"] != true {
		t.Errorf("paranoid-mode ips-signature-rules enabled = %v, want true", paranoid["enabled"])
	}
	if paranoid["dynamicValRejectCode"] != 3 {
		t.Errorf("paranoid-mode ips-signature-rules dynamicValRejectCode = %v, want 3", paranoid["dynamicValRejectCode"])
	}
	assertNoCanaries(t, "ips-signature-rules", paranoid, ruleTextCanary)
}

func TestIPSSignatureRuleNilCategoryStaysAbsent(t *testing.T) {
	t.Parallel()

	rule := wave1IPSSignatureRule("wave1-ruletext-canary-never-emit")
	rule.Category = nil
	got := projectOneRecord(t, resources.ProductZIA, resourceIPSSignatureRules,
		[]resources.SourceRecord{ipsSignatureRuleSourceRecord(rule)})

	// Nil-pointer arm: the mapper must not emit an empty category object.
	assertFieldsAbsent(t, "ips-signature-rules", got, "category")
}
