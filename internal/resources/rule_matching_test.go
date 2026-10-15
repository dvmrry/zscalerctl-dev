package resources

import (
	"slices"
	"strings"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/redact"
)

// criteriaLikeFields are catalog field names that restrict which traffic a
// rule matches. Every one a documented rule resource has must be placed in a
// group or listed as unplaced, so a transcription gap cannot hide a criterion.
var criteriaLikeFields = []string{
	"users", "groups", "departments", "excludedUsers", "excludedGroups", "excludedDepartments",
	"locations", "locationGroups", "ecGroups", "timeWindows",
	"devices", "deviceGroups", "deviceTrustLevels", "workloadGroups", "userRiskScoreLevels", "platforms", "tenancyProfileIds",
	"sourceIpGroups", "srcIpGroups", "srcIpv6Groups", "srcIps", "sourceCountries",
	"destIpGroups", "destIpv6Groups", "destAddresses", "destCountries", "destIpCategories", "resCategories",
	"nwServices", "nwServiceGroups", "nwApplications", "nwApplicationGroups", "appServices", "appServiceGroups",
	"zpaAppSegments", "zpaApplicationSegments", "zpaApplicationSegmentGroups", "zpaIpGroup",
	"applications", "applicationGroups", "cloudApplications", "cloudAppInstances", "cloudAppRiskProfile",
	"urlCategories", "urlCategories2", "protocols", "requestMethods", "userAgentTypes", "httpHeaderProfiles",
	"fileTypes", "fileTypeCategories", "includedDomainProfiles", "excludedDomainProfiles", "proxyGateways",
}

func TestRuleMatchingPlacesEveryCriteriaField(t *testing.T) {
	t.Parallel()
	for key, definition := range ruleMatchingDefinitions {
		spec := mustRuleSpec(t, key)
		listed := definition.matchFields()
		for _, name := range criteriaLikeFields {
			if _, ok := findTopLevelField(spec.Fields, name); ok && !slices.Contains(listed, name) {
				t.Errorf("%s: catalog criteria field %q is neither grouped nor unplaced", key, name)
			}
		}
	}
}

// modifierFields change how other criteria match. Every one a documented
// rule resource has must be explained by one of its notes.
var modifierFields = []string{"excludeSrcCountries", "enforceTimeValidity", "excludedUsers", "excludedGroups", "excludedDepartments"}

func TestRuleMatchingNotesExplainEveryModifierField(t *testing.T) {
	t.Parallel()
	for key, definition := range ruleMatchingDefinitions {
		spec := mustRuleSpec(t, key)
		notes := strings.Join(definition.notes, "\n")
		for _, name := range modifierFields {
			if _, ok := findTopLevelField(spec.Fields, name); ok && !strings.Contains(notes, name) {
				t.Errorf("%s: catalog modifier field %q is not explained by a note", key, name)
			}
		}
	}
}

func TestRuleMatchingDescribedInEveryMode(t *testing.T) {
	t.Parallel()
	for key := range ruleMatchingDefinitions {
		spec := mustRuleSpec(t, key)
		for _, mode := range allModes() {
			doc, err := DescribeSemantics(spec, mode)
			if err != nil {
				t.Fatalf("DescribeSemantics(%s, %s) error = %v", key, mode, err)
			}
			if doc.Matching == nil || len(doc.Matching.Groups) == 0 {
				t.Fatalf("DescribeSemantics(%s, %s).Matching = %+v, want groups", key, mode, doc.Matching)
			}
			if !strings.HasPrefix(doc.Matching.Source, "https://help.zscaler.com/") || doc.Matching.Logic != RuleMatchingLogic {
				t.Errorf("%s: source %q / logic not the documented constants", key, doc.Matching.Source)
			}
		}
	}
}

// An agent must not read a field's absence as "no criterion" when the mode
// simply omits it: not_rendered names exactly the match fields the mode drops.
func TestRuleMatchingNotRenderedFollowsRedactionMode(t *testing.T) {
	t.Parallel()
	spec := mustRuleSpec(t, "zia/url-filtering-rules")
	for _, mode := range allModes() {
		doc, err := DescribeSemantics(spec, mode)
		if err != nil {
			t.Fatalf("DescribeSemantics(%s) error = %v", mode, err)
		}
		var want []string
		for _, name := range ruleMatchingDefinitions["zia/url-filtering-rules"].matchFields() {
			field, _ := findTopLevelField(spec.Fields, name)
			if !field.AllowedIn(mode) {
				want = append(want, name)
			}
		}
		if !slices.Equal(doc.Matching.NotRendered, want) {
			t.Errorf("%s: not_rendered = %v, want %v", mode, doc.Matching.NotRendered, want)
		}
		if got := slices.Contains(doc.Matching.NotRendered, "users"); got != (mode == redact.ModeParanoid) {
			t.Errorf("%s: users in not_rendered = %v, want %v", mode, got, mode == redact.ModeParanoid)
		}
	}
}

func TestRuleMatchingURLFilteringIdentityOperatorIsNotAssumed(t *testing.T) {
	t.Parallel()
	doc, err := DescribeSemantics(mustRuleSpec(t, "zia/url-filtering-rules"), redact.ModeStandard)
	if err != nil {
		t.Fatal(err)
	}
	who := doc.Matching.Groups[0]
	if who.Operator != MatchAdminSelected || !slices.Equal(who.Fields, []string{"users", "groups", "departments"}) {
		t.Errorf("identity group = %+v, want admin_selected over users, groups, departments", who)
	}
}

func TestRuleMatchingAbsentOutsideDocumentedRules(t *testing.T) {
	t.Parallel()
	for _, key := range []string{"zia/locations", "zia/casb-dlp-rules", "ztw/forwarding-rules"} {
		doc, err := DescribeSemantics(mustRuleSpec(t, key), redact.ModeStandard)
		if err != nil {
			t.Fatalf("DescribeSemantics(%s) error = %v", key, err)
		}
		if doc.Matching != nil {
			t.Errorf("%s: Matching = %+v, want nil", key, doc.Matching)
		}
	}
}

func TestRuleMatchingRejectsUnknownField(t *testing.T) {
	t.Parallel()
	spec := mustRuleSpec(t, "zia/url-filtering-rules")
	bad := ruleMatchingDefinitions["zia/url-filtering-rules"]
	bad.unplaced = append(slices.Clone(bad.unplaced), "noSuchField")
	if err := validateRuleMatching(spec, bad); err == nil {
		t.Fatal("validateRuleMatching accepted a field absent from the catalog")
	}
}

func mustRuleSpec(t *testing.T, key string) ResourceSpec {
	t.Helper()
	product, name, _ := strings.Cut(key, "/")
	spec, ok := FindSpec(Product(product), name)
	if !ok {
		t.Fatalf("FindSpec(%s) not found", key)
	}
	return spec
}
