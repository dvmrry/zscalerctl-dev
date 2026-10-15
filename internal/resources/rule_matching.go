package resources

import (
	"fmt"
	"slices"
	"strings"

	"github.com/dvmrry/zscalerctl/internal/redact"
)

// RuleMatchingLogic is the fixed reading of a RuleMatching document. It is
// Zscaler's documented policy execution: categories are ANDed, the criteria
// inside one category have their own operator, and "selecting no value
// ignores the criterion in the policy evaluation".
const RuleMatchingLogic = "A rule matches traffic only when every group matches; groups are ANDed. " +
	"Within a group the fields are joined by the group's operator. " +
	"A field with no value is ignored, so a group whose fields are all empty does not restrict the rule, " +
	"and a rule with no criteria set matches all traffic its policy evaluates. " +
	"A field that lists every value it can take is set but does not restrict the rule either. " +
	"Fields in unplaced_criteria are criteria the cited documentation does not place: when set they can restrict the rule, but how they combine with the groups is undocumented. " +
	"A field in not_rendered is omitted from output in this redaction mode, so its absence there does not mean it is empty. " +
	"A field named in notes modifies matching as the note says (an exclusion flag or a validity window); other catalog fields are not documented as match criteria."

// MatchOperator joins the fields of one criteria group.
type MatchOperator string

const (
	// MatchOr means the group matches when any non-empty field matches.
	MatchOr MatchOperator = "or"
	// MatchAdminSelected means the console lets the admin choose AND or OR
	// for the group and the API does not return the choice.
	MatchAdminSelected MatchOperator = "admin_selected"
)

// CriteriaGroup is one ANDed group of a rule's match criteria. Fields are
// top-level catalog field names of the resource.
type CriteriaGroup struct {
	Category string        `json:"category"`
	Operator MatchOperator `json:"operator"`
	Fields   []string      `json:"fields"`
}

// RuleMatching is the documented criteria logic of a policy rule resource.
// It is static metadata cited to Zscaler's help documentation and never
// contains tenant values.
type RuleMatching struct {
	Source string          `json:"source"`
	Logic  string          `json:"logic"`
	Groups []CriteriaGroup `json:"groups"`
	// Unplaced lists criteria fields the documentation does not place in
	// its expression.
	Unplaced []string `json:"unplaced_criteria,omitempty"`
	// NotRendered lists grouped or unplaced fields the catalog omits from
	// output in the requested redaction mode.
	NotRendered []string `json:"not_rendered,omitempty"`
	// NotRead lists documented criteria the API (as the SDK models it) does
	// not return, so a rule may be narrower than its rendered fields show.
	NotRead []string `json:"not_read,omitempty"`
	Notes   []string `json:"notes,omitempty"`
}

type ruleMatchingDefinition struct {
	source   string
	groups   []CriteriaGroup
	unplaced []string
	notRead  []string
	notes    []string
}

func orGroup(category string, fields ...string) CriteriaGroup {
	return CriteriaGroup{Category: category, Operator: MatchOr, Fields: fields}
}

const ziaHelp = "https://help.zscaler.com/zia/"

// ruleMatchingDefinitions transcribes each policy's "Policy Execution"
// section. A documented AND inside a category becomes separate groups, so
// every group is an OR set (or admin_selected) and every group is ANDed.
var ruleMatchingDefinitions = map[string]ruleMatchingDefinition{
	"zia/url-filtering-rules": {
		source: ziaHelp + "configuring-url-filtering-policy",
		groups: []CriteriaGroup{
			{Category: "who", Operator: MatchAdminSelected, Fields: []string{"users", "groups", "departments"}},
			orGroup("who", "userRiskScoreLevels"),
			orGroup("where", "locations", "locationGroups"),
			orGroup("when", "timeWindows"),
			orGroup("what", "requestMethods"),
			orGroup("what", "protocols"),
			orGroup("what", "urlCategories"),
			orGroup("what", "urlCategories2"),
			orGroup("what", "userAgentTypes"),
			orGroup("what", "httpHeaderProfiles"),
			orGroup("source", "sourceIpGroups"),
			orGroup("source", "sourceCountries"),
			orGroup("device", "devices", "deviceGroups"),
			orGroup("device", "workloadGroups"),
			orGroup("device", "deviceTrustLevels"),
		},
		notRead: []string{
			"Adaptive Access Profile",
			"additional Groups fields (console \"Add More Groups\"; ANDed with groups, so a member must belong to a group in every field)",
		},
		notes: []string{
			"requestMethods and protocols are usually filled with every value; they restrict the rule only when they list a subset.",
			"urlCategories2 is the console's second URL Categories field: a URL must be in a category of each.",
			"overrideUsers and overrideGroups name who may override a block; they are not match criteria.",
			"With enforceTimeValidity true the rule applies only between validityStartTime and validityEndTime.",
		},
	},
	"zia/firewall-filtering-rules": {
		source: ziaHelp + "configuring-firewall-filtering-policy",
		groups: []CriteriaGroup{
			orGroup("who", "users", "groups", "departments"),
			orGroup("what", "devices", "deviceGroups"),
			orGroup("risk", "workloadGroups", "deviceTrustLevels"),
			orGroup("where", "locations", "locationGroups"),
			orGroup("when", "timeWindows"),
			orGroup("services", "nwServiceGroups", "nwServices"),
			orGroup("applications", "nwApplicationGroups", "nwApplications", "appServiceGroups"),
			orGroup("applications", "zpaAppSegments"),
			orGroup("source", "srcIpGroups", "srcIps", "sourceCountries"),
			orGroup("destination", "destIpGroups", "destAddresses", "destCountries", "destIpCategories"),
		},
		unplaced: []string{"appServices"},
		notRead:  []string{"Endpoint Applications", "Endpoint Application Tags", "Endpoint Application Risk Level", "Source IPv6 Groups", "Destination IPv6 Groups"},
		notes: []string{
			"With excludeSrcCountries true, sourceCountries lists countries the rule does not apply to.",
		},
	},
	"zia/ssl-inspection-rules": {
		source: ziaHelp + "configuring-ssltls-inspection-policy",
		groups: []CriteriaGroup{
			orGroup("source", "sourceIpGroups"),
			orGroup("destination", "urlCategories", "cloudApplications", "destIpGroups", "proxyGateways"),
			orGroup("applications", "zpaAppSegments"),
			orGroup("where", "locationGroups", "locations"),
			orGroup("who", "users", "groups", "departments"),
			orGroup("device", "deviceGroups", "devices", "roadWarriorForKerberos"),
			orGroup("device", "deviceTrustLevels"),
			orGroup("device", "userAgentTypes"),
		},
		unplaced: []string{"timeWindows", "platforms", "workloadGroups"},
		notRead:  []string{"Endpoint Applications", "Endpoint Application Tags", "Endpoint Application Risk Level"},
	},
	"zia/cloud-app-control": {
		source: ziaHelp + "adding-rules-cloud-app-control-policy",
		groups: []CriteriaGroup{
			orGroup("applications", "applications", "cloudAppInstances", "cloudAppRiskProfile"),
			orGroup("who", "groups", "users", "departments"),
			orGroup("where", "locationGroups", "locations"),
			orGroup("when", "timeWindows"),
			orGroup("devices_and_agents", "devices", "deviceGroups", "deviceTrustLevels", "userAgentTypes", "userRiskScoreLevels"),
		},
		notes: []string{
			"With enforceTimeValidity true the rule applies only between validityStartTime and validityEndTime.",
		},
		unplaced: []string{"tenancyProfileIds"},
	},
	"zia/dlp-web-rules": {
		source: ziaHelp + "configuring-dlp-policy-rules-content-inspection",
		groups: []CriteriaGroup{
			orGroup("who", "users", "groups", "departments", "userRiskScoreLevels"),
			orGroup("where", "locations", "locationGroups", "sourceIpGroups"),
			orGroup("when", "timeWindows"),
			orGroup("destination", "urlCategories"),
			orGroup("applications", "cloudApplications"),
			orGroup("protocols", "protocols"),
			orGroup("what", "fileTypes"),
			orGroup("what", "minSize"),
			orGroup("what", "dlpEngines"),
			orGroup("what", "workloadGroups"),
		},
		unplaced: []string{"excludedUsers", "excludedGroups", "excludedDepartments", "fileTypeCategories", "includedDomainProfiles", "excludedDomainProfiles"},
		notRead:  []string{"ZPA Application Segment", "Content Matching"},
		notes: []string{
			"excludedUsers, excludedGroups and excludedDepartments are the console's Exclude choice: the rule applies to everyone except those listed.",
		},
	},
	"zia/firewall-dns-rules": {
		source: ziaHelp + "configuring-dns-control-policy",
		groups: []CriteriaGroup{
			orGroup("who", "users", "groups", "departments"),
			orGroup("where", "locations", "locationGroups"),
			orGroup("when", "timeWindows"),
			orGroup("source", "srcIpGroups", "srcIpv6Groups", "srcIps"),
			orGroup("destination", "destIpGroups", "destIpv6Groups", "destAddresses"),
			orGroup("dns_application", "applications", "applicationGroups"),
			orGroup("dns_application", "destCountries", "destIpCategories", "resCategories"),
			orGroup("dns_application", "dnsRuleRequestTypes"),
			orGroup("dns_application", "protocols"),
		},
		unplaced: []string{"devices", "deviceGroups", "sourceCountries", "zpaIpGroup"},
		notRead:  []string{"Endpoint Applications", "Endpoint Application Tags", "Endpoint Application Risk Level"},
	},
	"zia/file-type-rules": {
		source: ziaHelp + "configuring-file-type-control-policy",
		groups: []CriteriaGroup{
			orGroup("what", "fileTypes"),
			orGroup("what", "urlCategories"),
			orGroup("what", "cloudApplications"),
			orGroup("what", "zpaAppSegments"),
			orGroup("who", "users", "groups", "departments"),
			orGroup("where", "locationGroups", "locations"),
			orGroup("when", "timeWindows"),
			orGroup("device", "deviceGroups", "devices"),
			orGroup("protocols", "protocols"),
			orGroup("device", "deviceTrustLevels"),
			orGroup("what", "minSize"),
			orGroup("what", "maxSize"),
		},
	},
	"zia/forwarding-rules": {
		source: ziaHelp + "configuring-forwarding-control-policy",
		groups: []CriteriaGroup{
			orGroup("who", "users", "groups", "departments"),
			orGroup("where", "locations", "locationGroups"),
			orGroup("device", "deviceGroups"),
			orGroup("services", "nwServiceGroups", "nwServices"),
			orGroup("source", "srcIpGroups", "srcIps"),
			orGroup("destination", "destIpGroups", "destAddresses", "destCountries", "destIpCategories"),
		},
		unplaced: []string{"resCategories", "srcIpv6Groups", "destIpv6Groups", "ecGroups", "nwApplicationGroups", "appServiceGroups", "zpaAppSegments", "zpaApplicationSegments", "zpaApplicationSegmentGroups"},
	},
	"zia/bandwidth-control-rules": {
		source: ziaHelp + "adding-rules-bandwidth-control-policy",
		groups: []CriteriaGroup{
			orGroup("where", "locations"),
			orGroup("where", "locationGroups"),
			orGroup("when", "timeWindows"),
			orGroup("what", "protocols"),
			orGroup("what", "bandwidthClasses"),
		},
		unplaced: []string{"devices", "deviceGroups", "deviceTrustLevels"},
		notes: []string{
			"Unlike other policies, the documentation joins Locations AND Location Groups: a rule naming both matches only traffic that satisfies both.",
		},
	},
	"zia/ips-policies": {
		source: ziaHelp + "configuring-ips-control-policy",
		groups: []CriteriaGroup{
			orGroup("who", "users", "groups", "departments"),
			orGroup("where", "locations", "locationGroups"),
			orGroup("when", "timeWindows"),
			orGroup("services", "nwServiceGroups", "nwServices"),
			orGroup("threats", "threatCategories"),
			orGroup("source", "srcIpGroups", "srcIpv6Groups", "srcIps"),
			orGroup("destination", "destIpGroups", "destIpv6Groups", "destAddresses", "destCountries", "destIpCategories"),
		},
		unplaced: []string{"resCategories", "devices", "deviceGroups", "sourceCountries", "zpaAppSegments"},
	},
	"zia/sandbox-rules": {
		source: ziaHelp + "configuring-sandbox-policy",
		groups: []CriteriaGroup{
			orGroup("what", "fileTypes"),
			orGroup("what", "urlCategories", "baPolicyCategories"),
			orGroup("who", "users", "groups", "departments"),
			orGroup("where", "locationGroups", "locations"),
			orGroup("protocols", "protocols"),
			orGroup("applications", "zpaAppSegments"),
		},
		unplaced: []string{"timeWindows", "devices", "deviceGroups"},
	},
	"zia/nat-control-rules": {
		source: ziaHelp + "configuring-nat-control-policy",
		groups: []CriteriaGroup{
			orGroup("who", "departments", "groups", "users"),
			orGroup("where", "locations", "locationGroups"),
			orGroup("when", "timeWindows"),
			orGroup("services", "nwServices", "nwServiceGroups"),
			orGroup("source", "srcIps", "srcIpGroups", "srcIpv6Groups"),
			orGroup("destination", "destCountries", "destAddresses", "destIpCategories", "destIpGroups", "destIpv6Groups"),
		},
		unplaced: []string{"devices", "deviceGroups", "resCategories"},
	},
}

// describeRuleMatching returns the documented match logic for spec, with
// each group's fields that the catalog omits in mode. It returns nil for a
// resource without documented match logic.
func describeRuleMatching(spec ResourceSpec, mode redact.Mode) (*RuleMatching, error) {
	key := string(spec.Product) + "/" + spec.Name
	definition, ok := ruleMatchingDefinitions[key]
	if !ok {
		return nil, nil
	}
	if err := validateRuleMatching(spec, definition); err != nil {
		return nil, err
	}
	doc := &RuleMatching{
		Source:   definition.source,
		Logic:    RuleMatchingLogic,
		Groups:   make([]CriteriaGroup, 0, len(definition.groups)),
		Unplaced: slices.Clone(definition.unplaced),
		NotRead:  slices.Clone(definition.notRead),
		Notes:    slices.Clone(definition.notes),
	}
	for _, group := range definition.groups {
		doc.Groups = append(doc.Groups, CriteriaGroup{Category: group.Category, Operator: group.Operator, Fields: slices.Clone(group.Fields)})
	}
	for _, name := range definition.matchFields() {
		field, _ := findTopLevelField(spec.Fields, name)
		if !field.AllowedIn(mode) {
			doc.NotRendered = append(doc.NotRendered, name)
		}
	}
	return doc, nil
}

// matchFields returns every grouped field, then every unplaced field.
func (d ruleMatchingDefinition) matchFields() []string {
	var out []string
	for _, group := range d.groups {
		out = append(out, group.Fields...)
	}
	return append(out, d.unplaced...)
}

func validateRuleMatching(spec ResourceSpec, definition ruleMatchingDefinition) error {
	if !strings.HasPrefix(definition.source, "https://help.zscaler.com/") {
		return fmt.Errorf("%w: %s/%s match logic source %q is not Zscaler help documentation", ErrInvalidSemanticsSpec, spec.Product, spec.Name, definition.source)
	}
	for _, group := range definition.groups {
		if group.Category == "" || len(group.Fields) == 0 {
			return fmt.Errorf("%w: %s/%s has an empty match group", ErrInvalidSemanticsSpec, spec.Product, spec.Name)
		}
		if group.Operator != MatchOr && group.Operator != MatchAdminSelected {
			return fmt.Errorf("%w: %s/%s match group %q has operator %q", ErrInvalidSemanticsSpec, spec.Product, spec.Name, group.Category, group.Operator)
		}
	}
	seen := map[string]struct{}{}
	for _, name := range definition.matchFields() {
		if _, duplicate := seen[name]; duplicate {
			return fmt.Errorf("%w: %s/%s match field %q is listed twice", ErrInvalidSemanticsSpec, spec.Product, spec.Name, name)
		}
		seen[name] = struct{}{}
		if _, ok := findTopLevelField(spec.Fields, name); !ok {
			return fmt.Errorf("%w: %s/%s match field %q is absent from the catalog", ErrInvalidSemanticsSpec, spec.Product, spec.Name, name)
		}
	}
	return nil
}
