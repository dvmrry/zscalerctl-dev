package zscaler

import (
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/resources"

	bandwidthclasses "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/bandwidth_control/bandwidth_classes"
	bandwidthcontrolrules "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/bandwidth_control/bandwidth_control_rules"
	ziacommon "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/common"
	applicationservices "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/firewallpolicies/applicationservices"
	appservicegroups "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/firewallpolicies/appservicegroups"
	dnsgateways "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/firewallpolicies/dns_gateways"
	filteringrules "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/firewallpolicies/filteringrules"
	ipdestinationgroups "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/firewallpolicies/ipdestinationgroups"
	ipsourcegroups "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/firewallpolicies/ipsourcegroups"
	networkapplicationgroups "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/firewallpolicies/networkapplicationgroups"
	networkservices "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/firewallpolicies/networkservices"
	"github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/firewallpolicies/timewindow"
	forwardingrules "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/forwarding_control_policy/forwarding_rules"
	"github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/forwarding_control_policy/proxies"
	proxygateways "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/forwarding_control_policy/proxy_gateways"
	"github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/location/locationgroups"
	"github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/location/locationmanagement"
	natcontrol "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/nat_control_policies"
	rulelabels "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/rule_labels"
	"github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/sslinspection"
	timeintervals "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/time_intervals"
	gretunnels "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/trafficforwarding/gretunnels"
	staticips "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/trafficforwarding/staticips"
	"github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/urlcategories"
	"github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/urlfilteringpolicies"
)

func TestReviewedSDKShapesMatchCatalogOrIgnoredRegistry(t *testing.T) {
	t.Parallel()

	for _, shape := range reviewedSDKShapes() {
		shape := shape
		t.Run(shape.name, func(t *testing.T) {
			t.Parallel()
			shape.assertReviewed(t)
		})
	}
}

func TestReviewedSDKShapesCoverCatalogListResources(t *testing.T) {
	t.Parallel()

	reviewed := map[string]struct{}{}
	for _, shape := range reviewedSDKShapes() {
		if shape.resource == "" && shape.resourceName == "" {
			continue
		}
		reviewed[resourceReviewKey(shape.resource, shape.resourceName)] = struct{}{}
	}

	var missing []string
	for _, spec := range resources.Catalog() {
		if !specHasReadListOperation(spec) {
			continue
		}
		key := resourceReviewKey(spec.Product, spec.Name)
		if _, ok := reviewed[key]; !ok {
			missing = append(missing, key)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Fatalf("reviewedSDKShapes() missing top-level SDK shape reviews for catalog resources: %v", missing)
	}
}

type sdkShapeReview struct {
	name          string
	resource      resources.Product
	resourceName  string
	typ           reflect.Type
	catalogFields []string
	ignoredFields map[string]string
}

func (s sdkShapeReview) assertReviewed(t *testing.T) {
	t.Helper()

	catalog := map[string]struct{}{}
	if s.resource != "" || s.resourceName != "" {
		spec, ok := resources.FindSpec(s.resource, s.resourceName)
		if !ok {
			t.Fatalf("resources.FindSpec(%s, %s) ok = false, want true", s.resource, s.resourceName)
		}
		catalog = catalogFieldNames(spec)
	}

	classified := namesSet(s.catalogFields)
	for _, field := range s.catalogFields {
		if _, ok := catalog[field]; !ok {
			t.Errorf("%s catalog field %q missing from %s/%s", s.name, field, s.resource, s.resourceName)
		}
	}

	for field, reason := range s.ignoredFields {
		if strings.TrimSpace(reason) == "" {
			t.Errorf("%s ignored field %q has empty reason", s.name, field)
		}
		if _, ok := classified[field]; ok {
			t.Errorf("%s field %q is both catalog-classified and ignored", s.name, field)
		}
	}

	var missing []string
	exported := exportedJSONFields(s.typ)
	for _, field := range exported {
		if _, ok := classified[field]; ok {
			continue
		}
		if _, ok := s.ignoredFields[field]; ok {
			continue
		}
		missing = append(missing, field)
	}
	if len(missing) > 0 {
		t.Errorf("%s SDK fields missing catalog classification or ignore reason: %v", s.name, missing)
	}

	exportedSet := namesSet(exported)
	var stale []string
	for field := range s.ignoredFields {
		if _, ok := exportedSet[field]; !ok {
			stale = append(stale, field)
		}
	}
	sort.Strings(stale)
	if len(stale) > 0 {
		t.Errorf("%s ignored SDK fields no longer exist: %v", s.name, stale)
	}
}

func reviewedSDKShapes() []sdkShapeReview {
	// Register top-level SDK resource structs and every nested SDK helper struct
	// that reader mappings intentionally traverse. A struct-typed field ignored
	// at a parent level still needs its own review entry when its fields are
	// mapped into SourceRecord data, so SDK bumps cannot add nested fields
	// without an explicit classify-or-ignore decision.
	return []sdkShapeReview{
		{
			name:         "locationmanagement.Locations",
			resource:     resources.ProductZIA,
			resourceName: resourceLocations,
			typ:          reflect.TypeOf(locationmanagement.Locations{}),
			catalogFields: []string{
				"id",
				"name",
				"description",
				"ipAddresses",
				"vpnCredentials",
			},
			ignoredFields: mergeIgnoredFields(
				ignoredBecause(
					"narrow location identity surface; classify before exposing additional metadata",
					"parentId",
					"country",
					"state",
					"language",
					"tz",
					"profile",
				),
				ignoredBecause(
					"bandwidth, auth, and policy controls are not emitted in the read-only inventory surface",
					"upBandwidth",
					"dnBandwidth",
					"ports",
					"subLocScopeEnabled",
					"subLocScope",
					"subLocScopeValues",
					"subLocAccIds",
					"authRequired",
					"basicAuthEnabled",
					"digestAuthEnabled",
					"kerberosAuth",
					"iotDiscoveryEnabled",
					"iotEnforcePolicySet",
					"cookiesAndProxy",
					"sslScanEnabled",
					"zappSSLScanEnabled",
					"xffForwardEnabled",
					"surrogateIP",
					"idleTimeInMinutes",
					"displayTimeUnit",
					"surrogateIPEnforcedForKnownBrowsers",
					"surrogateRefreshTimeInMinutes",
					"surrogateRefreshTimeUnit",
					"ofwEnabled",
					"ipsControl",
					"aupEnabled",
					"cautionEnabled",
					"aupBlockInternetUntilAccepted",
					"aupForceSslInspection",
					"aupTimeoutInDays",
				),
				ignoredBecause(
					"hierarchy and membership references are intentionally omitted until separately modeled",
					"childCount",
					"matchInChild",
					"excludeFromDynamicGroups",
					"excludeFromManualGroups",
					"otherSubLocation",
					"other6SubLocation",
					"ecLocation",
					"dynamiclocationGroups",
					"staticLocationGroups",
					"virtualZenClusters",
					"virtualZens",
				),
				ignoredBecause(
					"extranet and IPv6 operational references are intentionally omitted until separately classified",
					"geoOverride",
					"ipv6Enabled",
					"defaultExtranetTsPool",
					"defaultExtranetDns",
					"extranet",
					"extranetIpPool",
					"extranetDns",
					"ipv6Dns64Prefix",
				),
			),
		},
		{
			name: "locationmanagement.VPNCredentials",
			typ:  reflect.TypeOf(locationmanagement.VPNCredentials{}),
			ignoredFields: ignoredBecause(
				"covered by secret vpnCredentials parent; nested credential payload is never emitted",
				"id",
				"type",
				"fqdn",
				"ipAddress",
				"preSharedKey",
				"comments",
				"location",
				"managedBy",
			),
		},
		{
			name:         "locationgroups.LocationGroup",
			resource:     resources.ProductZIA,
			resourceName: resourceLocationGroups,
			typ:          reflect.TypeOf(locationgroups.LocationGroup{}),
			catalogFields: []string{
				"id",
				"name",
				"deleted",
				"groupType",
				"comments",
				"lastModTime",
				"predefined",
			},
			ignoredFields: ignoredBecause(
				"nested criteria, member locations, and admin references are mapped then dropped by projection",
				"dynamicLocationGroupCriteria",
				"locations",
				"lastModUser",
			),
		},
		{
			name: "locationgroups.DynamicLocationGroupCriteria",
			typ:  reflect.TypeOf(locationgroups.DynamicLocationGroupCriteria{}),
			ignoredFields: ignoredBecause(
				"covered by dropped dynamicLocationGroupCriteria parent",
				"name",
				"countries",
				"city",
				"managedBy",
				"enforceAuthentication",
				"enforceAup",
				"enforceFirewallControl",
				"enableXffForwarding",
				"enableCaution",
				"enableBandwidthControl",
				"profiles",
			),
		},
		{
			name: "locationgroups.Name",
			typ:  reflect.TypeOf(locationgroups.Name{}),
			ignoredFields: ignoredBecause(
				"covered by dropped dynamicLocationGroupCriteria parent",
				"matchString",
				"matchType",
			),
		},
		{
			name: "locationgroups.City",
			typ:  reflect.TypeOf(locationgroups.City{}),
			ignoredFields: ignoredBecause(
				"covered by dropped dynamicLocationGroupCriteria parent",
				"matchString",
				"matchType",
			),
		},
		{
			name: "locationgroups.ManagedBy",
			typ:  reflect.TypeOf(locationgroups.ManagedBy{}),
			ignoredFields: ignoredBecause(
				"covered by dropped dynamicLocationGroupCriteria parent",
				"id",
				"name",
				"extensions",
			),
		},
		{
			name: "locationgroups.LastModUser",
			typ:  reflect.TypeOf(locationgroups.LastModUser{}),
			ignoredFields: ignoredBecause(
				"covered by dropped lastModUser parent",
				"id",
				"name",
				"extensions",
			),
		},
		{
			name:         "rulelabels.RuleLabels",
			resource:     resources.ProductZIA,
			resourceName: resourceRuleLabels,
			typ:          reflect.TypeOf(rulelabels.RuleLabels{}),
			catalogFields: []string{
				"id",
				"name",
				"description",
				"lastModifiedTime",
				"referencedRuleCount",
			},
			ignoredFields: ignoredBecause(
				"admin references are mapped then dropped by projection",
				"createdBy",
				"lastModifiedBy",
			),
		},
		{
			name: "ziacommon.IDNameExtensions",
			typ:  reflect.TypeOf(ziacommon.IDNameExtensions{}),
			ignoredFields: ignoredBecause(
				"used inside modeled and dropped reference parents; parent catalog fields decide whether id/name can render",
				"id",
				"name",
				"extensions",
			),
		},
		{
			name:         "staticips.StaticIP",
			resource:     resources.ProductZIA,
			resourceName: resourceStaticIPs,
			typ:          reflect.TypeOf(staticips.StaticIP{}),
			catalogFields: []string{
				"id",
				"ipAddress",
				"geoOverride",
				"latitude",
				"longitude",
				"routableIP",
				"lastModificationTime",
				"comment",
			},
			ignoredFields: ignoredBecause(
				"nested city/admin references are mapped then dropped by projection",
				"city",
				"managedBy",
				"lastModifiedBy",
			),
		},
		{
			name: "staticips.City",
			typ:  reflect.TypeOf(staticips.City{}),
			ignoredFields: ignoredBecause(
				"covered by dropped city parent",
				"id",
				"name",
			),
		},
		{
			name: "staticips.ManagedBy",
			typ:  reflect.TypeOf(staticips.ManagedBy{}),
			ignoredFields: ignoredBecause(
				"covered by dropped managedBy parent",
				"id",
				"name",
				"extensions",
			),
		},
		{
			name: "staticips.LastModifiedBy",
			typ:  reflect.TypeOf(staticips.LastModifiedBy{}),
			ignoredFields: ignoredBecause(
				"covered by dropped lastModifiedBy parent",
				"id",
				"name",
				"extensions",
			),
		},
		{
			name:         "gretunnels.GreTunnels",
			resource:     resources.ProductZIA,
			resourceName: resourceGRETunnels,
			typ:          reflect.TypeOf(gretunnels.GreTunnels{}),
			catalogFields: []string{
				"id",
				"sourceIp",
				"internalIpRange",
				"lastModificationTime",
				"withinCountry",
				"comment",
				"ipUnnumbered",
				"subcloud",
			},
			ignoredFields: ignoredBecause(
				"nested admin and destination VIP references are mapped then dropped by projection",
				"managedBy",
				"lastModifiedBy",
				"primaryDestVip",
				"secondaryDestVip",
			),
		},
		{
			name: "gretunnels.ManagedBy",
			typ:  reflect.TypeOf(gretunnels.ManagedBy{}),
			ignoredFields: ignoredBecause(
				"covered by dropped managedBy parent",
				"id",
				"name",
				"extensions",
			),
		},
		{
			name: "gretunnels.LastModifiedBy",
			typ:  reflect.TypeOf(gretunnels.LastModifiedBy{}),
			ignoredFields: ignoredBecause(
				"covered by dropped lastModifiedBy parent",
				"id",
				"name",
				"extensions",
			),
		},
		{
			name: "gretunnels.PrimaryDestVip",
			typ:  reflect.TypeOf(gretunnels.PrimaryDestVip{}),
			ignoredFields: ignoredBecause(
				"covered by dropped primaryDestVip parent",
				"id",
				"virtualIp",
				"privateServiceEdge",
				"datacenter",
				"latitude",
				"longitude",
				"city",
				"countryCode",
				"region",
			),
		},
		{
			name: "gretunnels.SecondaryDestVip",
			typ:  reflect.TypeOf(gretunnels.SecondaryDestVip{}),
			ignoredFields: ignoredBecause(
				"covered by dropped secondaryDestVip parent",
				"id",
				"virtualIp",
				"privateServiceEdge",
				"datacenter",
				"latitude",
				"longitude",
				"city",
				"countryCode",
				"region",
			),
		},
		{
			name:         "locationmanagement.Locations/sublocations",
			resource:     resources.ProductZIA,
			resourceName: resourceSublocations,
			typ:          reflect.TypeOf(locationmanagement.Locations{}),
			catalogFields: []string{
				"id",
				"parentId",
				"name",
				"ipAddresses",
				"ports",
				"description",
				"profile",
				"country",
				"state",
				"tz",
				"authRequired",
				"sslScanEnabled",
				"ofwEnabled",
				"ipsControl",
				"vpnCredentials",
			},
			ignoredFields: mergeIgnoredFields(
				ignoredBecause(
					"additional bandwidth, auth, and policy controls are mapped then dropped until separately reviewed for sublocation output",
					"upBandwidth",
					"dnBandwidth",
					"language",
					"basicAuthEnabled",
					"digestAuthEnabled",
					"kerberosAuth",
					"iotDiscoveryEnabled",
					"iotEnforcePolicySet",
					"cookiesAndProxy",
					"zappSSLScanEnabled",
					"xffForwardEnabled",
					"surrogateIP",
					"idleTimeInMinutes",
					"displayTimeUnit",
					"surrogateIPEnforcedForKnownBrowsers",
					"surrogateRefreshTimeInMinutes",
					"surrogateRefreshTimeUnit",
					"aupEnabled",
					"cautionEnabled",
					"aupBlockInternetUntilAccepted",
					"aupForceSslInspection",
					"aupTimeoutInDays",
				),
				ignoredBecause(
					"hierarchy, scope, and group references are mapped then dropped until separately modeled",
					"childCount",
					"matchInChild",
					"subLocScopeEnabled",
					"subLocScope",
					"subLocScopeValues",
					"subLocAccIds",
					"excludeFromDynamicGroups",
					"excludeFromManualGroups",
					"otherSubLocation",
					"other6SubLocation",
					"ecLocation",
					"dynamiclocationGroups",
					"staticLocationGroups",
					"virtualZenClusters",
					"virtualZens",
				),
				ignoredBecause(
					"geo, extranet, and IPv6 references are mapped then dropped until separately classified",
					"geoOverride",
					"ipv6Enabled",
					"defaultExtranetTsPool",
					"defaultExtranetDns",
					"extranet",
					"extranetIpPool",
					"extranetDns",
					"ipv6Dns64Prefix",
				),
			),
		},
		{
			name:         "sslinspection.SSLInspectionRules",
			resource:     resources.ProductZIA,
			resourceName: resourceSSLRules,
			typ:          reflect.TypeOf(sslinspection.SSLInspectionRules{}),
			catalogFields: []string{
				"id",
				"name",
				"description",
				"action",
				"state",
				"order",
				"rank",
				"urlCategories",
				"platforms",
				"cloudApplications",
				"lastModifiedTime",
				"defaultRule",
				"predefined",
			},
			ignoredFields: ignoredBecause(
				"rule criteria references and admin metadata are mapped then dropped until separately modeled",
				"accessControl",
				"locations",
				"locationGroups",
				"groups",
				"departments",
				"users",
				"roadWarriorForKerberos",
				"userAgentTypes",
				"deviceTrustLevels",
				"deviceGroups",
				"devices",
				"lastModifiedBy",
				"destIpGroups",
				"sourceIpGroups",
				"proxyGateways",
				"labels",
				"timeWindows",
				"zpaAppSegments",
				"workloadGroups",
			),
		},
		{
			name: "sslinspection.Action",
			typ:  reflect.TypeOf(sslinspection.Action{}),
			ignoredFields: ignoredBecause(
				"nested action fields are mapped so the parent action object can render its reviewed type while unmodeled sub-actions and certificate references drop",
				"type",
				"showEUN",
				"showEUNATP",
				"overrideDefaultCertificate",
				"sslInterceptionCert",
				"decryptSubActions",
				"doNotDecryptSubActions",
			),
		},
		{
			name: "sslinspection.SSLInterceptionCert",
			typ:  reflect.TypeOf(sslinspection.SSLInterceptionCert{}),
			ignoredFields: ignoredBecause(
				"covered by dropped action.sslInterceptionCert parent",
				"id",
				"name",
				"defaultCertificate",
			),
		},
		{
			name: "sslinspection.DecryptSubActions",
			typ:  reflect.TypeOf(sslinspection.DecryptSubActions{}),
			ignoredFields: ignoredBecause(
				"covered by dropped action.decryptSubActions parent",
				"serverCertificates",
				"ocspCheck",
				"blockSslTrafficWithNoSniEnabled",
				"minClientTLSVersion",
				"minServerTLSVersion",
				"blockUndecrypt",
				"http2Enabled",
			),
		},
		{
			name: "sslinspection.DoNotDecryptSubActions",
			typ:  reflect.TypeOf(sslinspection.DoNotDecryptSubActions{}),
			ignoredFields: ignoredBecause(
				"covered by dropped action.doNotDecryptSubActions parent",
				"bypassOtherPolicies",
				"serverCertificates",
				"ocspCheck",
				"blockSslTrafficWithNoSniEnabled",
				"minTLSVersion",
			),
		},
		{
			name:         "urlcategories.URLCategory",
			resource:     resources.ProductZIA,
			resourceName: resourceURLCategories,
			typ:          reflect.TypeOf(urlcategories.URLCategory{}),
			catalogFields: []string{
				"id",
				"configuredName",
				"description",
				"type",
				"customCategory",
				"editable",
				"customUrlsCount",
				"customIpRangesCount",
				"urlsRetainingParentCategoryCount",
				"ipRangesRetainingParentCategoryCount",
				"categoryGroup",
				"superCategory",
				"urlType",
				"urlKeywordCounts",
				"keywords",
				"keywordsRetainingParentCategory",
				"urls",
				"dbCategorizedUrls",
				"ipRanges",
				"ipRangesRetainingParentCategory",
				"regexPatterns",
				"regexPatternsRetainingParentCategory",
			},
			ignoredFields: ignoredBecause(
				"scope and opaque SDK helper fields are mapped then dropped until separately classified",
				"scopes",
				"val",
			),
		},
		{
			name: "urlcategories.URLKeywordCounts",
			typ:  reflect.TypeOf(urlcategories.URLKeywordCounts{}),
			ignoredFields: ignoredBecause(
				"nested count fields are explicitly modeled in the url-categories catalog; this entry catches SDK additions",
				"totalUrlCount",
				"retainParentUrlCount",
				"totalKeywordCount",
				"retainParentKeywordCount",
			),
		},
		{
			name: "urlcategories.Scopes",
			typ:  reflect.TypeOf(urlcategories.Scopes{}),
			ignoredFields: ignoredBecause(
				"covered by dropped scopes parent",
				"scopeGroupMemberEntities",
				"Type",
				"ScopeEntities",
			),
		},
		{
			name:         "urlfilteringpolicies.URLFilteringRule",
			resource:     resources.ProductZIA,
			resourceName: resourceURLRules,
			typ:          reflect.TypeOf(urlfilteringpolicies.URLFilteringRule{}),
			catalogFields: []string{
				"id",
				"name",
				"description",
				"state",
				"order",
				"rank",
				"action",
				"protocols",
				"requestMethods",
				"urlCategories",
				"urlCategories2",
				"userRiskScoreLevels",
				"userAgentTypes",
				"sourceCountries",
				"lastModifiedTime",
				"enforceTimeValidity",
				"validityStartTime",
				"validityEndTime",
				"validityTimeZoneId",
				"blockOverride",
				"timeQuota",
				"sizeQuota",
				"ciparule",
				"endUserNotificationUrl",
				"cbiProfileId",
				"labels",
				"timeWindows",
				"locations",
				"locationGroups",
				"sourceIpGroups",
				"workloadGroups",
			},
			ignoredFields: ignoredBecause(
				"admin/user/device references and isolation profile details are mapped then dropped until separately modeled",
				"browserEunTemplateId",
				"cbiProfile",
				"departments",
				"deviceGroups",
				"deviceTrustLevels",
				"devices",
				"groups",
				"lastModifiedBy",
				"overrideGroups",
				"overrideUsers",
				"users",
			),
		},
		{
			name:         "filteringrules.FirewallFilteringRules",
			resource:     resources.ProductZIA,
			resourceName: resourceFirewallRules,
			typ:          reflect.TypeOf(filteringrules.FirewallFilteringRules{}),
			catalogFields: []string{
				"id",
				"name",
				"description",
				"state",
				"order",
				"rank",
				"action",
				"accessControl",
				"enableFullLogging",
				"defaultRule",
				"predefined",
				"lastModifiedTime",
				"sourceCountries",
				"destCountries",
				"excludeSrcCountries",
				"nwApplications",
				"srcIps",
				"destAddresses",
				"destIpCategories",
				"deviceTrustLevels",
				"labels",
				"timeWindows",
				"locations",
				"locationGroups",
				"srcIpGroups",
				"destIpGroups",
				"nwServices",
				"nwServiceGroups",
				"nwApplicationGroups",
				"appServices",
				"appServiceGroups",
				"workloadGroups",
			},
			ignoredFields: ignoredBecause(
				"admin/user/device references and ZPA segment references are mapped then dropped until separately modeled",
				"departments",
				"deviceGroups",
				"devices",
				"groups",
				"lastModifiedBy",
				"users",
				"zpaAppSegments",
			),
		},
		{
			name:         "forwardingrules.ForwardingRules",
			resource:     resources.ProductZIA,
			resourceName: resourceForwardingRules,
			typ:          reflect.TypeOf(forwardingrules.ForwardingRules{}),
			catalogFields: []string{
				"id",
				"name",
				"description",
				"type",
				"state",
				"order",
				"rank",
				"forwardMethod",
				"lastModifiedTime",
				"zpaBrokerRule",
				"destCountries",
				"srcIps",
				"destAddresses",
				"destIpCategories",
				"resCategories",
				"labels",
				"locations",
				"locationGroups",
				"ecGroups",
				"srcIpGroups",
				"srcIpv6Groups",
				"destIpGroups",
				"destIpv6Groups",
				"nwServices",
				"nwServiceGroups",
				"nwApplicationGroups",
				"appServiceGroups",
				"proxyGateway",
				"dedicatedIPGateway",
				"zpaGateway",
			},
			ignoredFields: ignoredBecause(
				"admin/user/device references and ZPA segment details are mapped then dropped until separately modeled",
				"departments",
				"deviceGroups",
				"groups",
				"lastModifiedBy",
				"users",
				"zpaAppSegments",
				"zpaApplicationSegments",
				"zpaApplicationSegmentGroups",
			),
		},
		{
			name: "ziacommon.CBIProfile",
			typ:  reflect.TypeOf(ziacommon.CBIProfile{}),
			ignoredFields: ignoredBecause(
				"covered by dropped cbiProfile parent",
				"id",
				"name",
				"url",
				"profileSeq",
			),
		},
		{
			name: "forwardingrules.ZPAApplicationSegments",
			typ:  reflect.TypeOf(forwardingrules.ZPAApplicationSegments{}),
			ignoredFields: ignoredBecause(
				"covered by dropped zpaApplicationSegments parent",
				"id",
				"name",
				"description",
				"zpaId",
				"deleted",
			),
		},
		{
			name: "forwardingrules.ZPAApplicationSegmentGroups",
			typ:  reflect.TypeOf(forwardingrules.ZPAApplicationSegmentGroups{}),
			ignoredFields: ignoredBecause(
				"covered by dropped zpaApplicationSegmentGroups parent",
				"id",
				"name",
				"zpaId",
				"deleted",
				"zpaAppSegmentsCount",
			),
		},
		{
			name: "ziacommon.IDName",
			typ:  reflect.TypeOf(ziacommon.IDName{}),
			ignoredFields: ignoredBecause(
				"currently used only inside dropped or explicitly modeled nested references",
				"id",
				"name",
				"parent",
			),
		},
		{
			name: "ziacommon.ZPAAppSegments",
			typ:  reflect.TypeOf(ziacommon.ZPAAppSegments{}),
			ignoredFields: ignoredBecause(
				"covered by dropped zpaAppSegments parent",
				"id",
				"name",
				"externalId",
			),
		},
		{
			name:         "ipsourcegroups.IPSourceGroups",
			resource:     resources.ProductZIA,
			resourceName: resourceIPSourceGroups,
			typ:          reflect.TypeOf(ipsourcegroups.IPSourceGroups{}),
			catalogFields: []string{
				"id",
				"name",
				"description",
				"ipAddresses",
				"isNonEditable",
			},
		},
		{
			name:         "ipdestinationgroups.IPDestinationGroups",
			resource:     resources.ProductZIA,
			resourceName: resourceIPDestGroups,
			typ:          reflect.TypeOf(ipdestinationgroups.IPDestinationGroups{}),
			catalogFields: []string{
				"id",
				"name",
				"description",
				"type",
				"addresses",
				"ipCategories",
				"countries",
				"isNonEditable",
			},
		},
		{
			name:         "networkservices.NetworkServices",
			resource:     resources.ProductZIA,
			resourceName: resourceNetworkServices,
			typ:          reflect.TypeOf(networkservices.NetworkServices{}),
			catalogFields: []string{
				"id",
				"name",
				"description",
				"tag",
				"type",
				"protocol",
				"isNameL10nTag",
				"srcTcpPorts",
				"destTcpPorts",
				"srcUdpPorts",
				"destUdpPorts",
			},
		},
		{
			name: "networkservices.NetworkPorts",
			typ:  reflect.TypeOf(networkservices.NetworkPorts{}),
			ignoredFields: ignoredBecause(
				"nested port ranges are explicitly modeled in network service catalog fields; this entry catches SDK additions",
				"start",
				"end",
			),
		},
		{
			name:         "applicationservices.ApplicationServicesLite",
			resource:     resources.ProductZIA,
			resourceName: resourceAppServices,
			typ:          reflect.TypeOf(applicationservices.ApplicationServicesLite{}),
			catalogFields: []string{
				"id",
				"name",
				"nameL10nTag",
			},
		},
		{
			name:         "appservicegroups.ApplicationServicesGroupLite",
			resource:     resources.ProductZIA,
			resourceName: resourceAppServiceGroups,
			typ:          reflect.TypeOf(appservicegroups.ApplicationServicesGroupLite{}),
			catalogFields: []string{
				"id",
				"name",
				"nameL10nTag",
			},
		},
		{
			name:         "networkapplicationgroups.NetworkApplicationGroups",
			resource:     resources.ProductZIA,
			resourceName: resourceNetworkAppGroups,
			typ:          reflect.TypeOf(networkapplicationgroups.NetworkApplicationGroups{}),
			catalogFields: []string{
				"id",
				"name",
				"networkApplications",
				"description",
			},
		},
		{
			name:         "timewindow.TimeWindow",
			resource:     resources.ProductZIA,
			resourceName: resourceTimeWindows,
			typ:          reflect.TypeOf(timewindow.TimeWindow{}),
			catalogFields: []string{
				"id",
				"name",
				"startTime",
				"endTime",
				"dayOfWeek",
			},
		},
		{
			name:         "proxies.Proxies",
			resource:     resources.ProductZIA,
			resourceName: resourceProxies,
			typ:          reflect.TypeOf(proxies.Proxies{}),
			catalogFields: []string{
				"id",
				"name",
				"type",
				"address",
				"port",
				"description",
				"insertXauHeader",
				"base64EncodeXauHeader",
				"cert",
				"lastModifiedTime",
			},
			ignoredFields: ignoredBecause(
				"admin references are mapped then dropped by projection",
				"lastModifiedBy",
			),
		},
		{
			name:         "proxygateways.ProxyGateways",
			resource:     resources.ProductZIA,
			resourceName: resourceProxyGateways,
			typ:          reflect.TypeOf(proxygateways.ProxyGateways{}),
			catalogFields: []string{
				"id",
				"name",
				"description",
				"failClosed",
				"type",
				"primaryProxy",
				"secondaryProxy",
				"lastModifiedTime",
			},
			ignoredFields: ignoredBecause(
				"admin references are mapped then dropped by projection",
				"lastModifiedBy",
			),
		},
		{
			name:         "proxies.DedicatedIPGateways",
			resource:     resources.ProductZIA,
			resourceName: resourceDedicatedIPGWs,
			typ:          reflect.TypeOf(proxies.DedicatedIPGateways{}),
			catalogFields: []string{
				"id",
				"name",
				"description",
				"primaryDataCenter",
				"secondaryDataCenter",
				"createTime",
				"lastModifiedTime",
				"default",
			},
			ignoredFields: ignoredBecause(
				"admin references are mapped then dropped by projection",
				"lastModifiedBy",
			),
		},
		{
			name:         "timeintervals.TimeInterval",
			resource:     resources.ProductZIA,
			resourceName: resourceTimeIntervals,
			typ:          reflect.TypeOf(timeintervals.TimeInterval{}),
			catalogFields: []string{
				"id",
				"name",
				"startTime",
				"endTime",
				"daysOfWeek",
			},
		},
		{
			name:         "bandwidthclasses.BandwidthClasses",
			resource:     resources.ProductZIA,
			resourceName: resourceBandwidthClasses,
			typ:          reflect.TypeOf(bandwidthclasses.BandwidthClasses{}),
			catalogFields: []string{
				"id",
				"isNameL10nTag",
				"name",
				"getfileSize",
				"fileSize",
				"type",
				"webApplications",
				"urls",
				"applicationServiceGroups",
				"networkApplications",
				"networkServices",
				"urlCategories",
				"applications",
			},
		},
		{
			name:         "bandwidthcontrolrules.BandwidthControlRules",
			resource:     resources.ProductZIA,
			resourceName: resourceBandwidthRules,
			typ:          reflect.TypeOf(bandwidthcontrolrules.BandwidthControlRules{}),
			catalogFields: []string{
				"id",
				"name",
				"order",
				"state",
				"description",
				"maxBandwidth",
				"minBandwidth",
				"rank",
				"lastModifiedTime",
				"accessControl",
				"defaultRule",
				"protocols",
				"deviceTrustLevels",
				"bandwidthClasses",
				"locationGroups",
				"labels",
				"devices",
				"deviceGroups",
				"locations",
				"timeWindows",
			},
			ignoredFields: ignoredBecause(
				"admin references are mapped then dropped by projection",
				"lastModifiedBy",
			),
		},
		{
			name:         "dnsgateways.DNSGateways",
			resource:     resources.ProductZIA,
			resourceName: resourceDNSGateways,
			typ:          reflect.TypeOf(dnsgateways.DNSGateways{}),
			catalogFields: []string{
				"id",
				"name",
				"dnsGatewayType",
				"primaryIpOrFqdn",
				"primaryPorts",
				"secondaryIpOrFqdn",
				"secondaryPorts",
				"protocols",
				"failureBehavior",
				"lastModifiedTime",
				"autoCreated",
				"natZtrGateway",
				"dnsGatewayProtocols",
			},
			ignoredFields: ignoredBecause(
				"admin references are mapped then dropped by projection",
				"lastModifiedBy",
			),
		},
		{
			name:         "natcontrol.NatControlPolicies",
			resource:     resources.ProductZIA,
			resourceName: resourceNATRules,
			typ:          reflect.TypeOf(natcontrol.NatControlPolicies{}),
			catalogFields: []string{
				"accessControl",
				"id",
				"name",
				"order",
				"rank",
				"description",
				"state",
				"redirectFqdn",
				"redirectIp",
				"redirectPort",
				"lastModifiedTime",
				"trustedResolverRule",
				"enableFullLogging",
				"predefined",
				"defaultRule",
				"destAddresses",
				"srcIps",
				"destCountries",
				"destIpCategories",
				"resCategories",
				"locations",
				"locationGroups",
				"groups",
				"departments",
				"users",
				"timeWindows",
				"srcIpGroups",
				"srcIpv6Groups",
				"destIpGroups",
				"destIpv6Groups",
				"nwServices",
				"nwServiceGroups",
				"devices",
				"deviceGroups",
				"labels",
			},
			ignoredFields: ignoredBecause(
				"admin references are mapped then dropped by projection",
				"lastModifiedBy",
			),
		},
		{
			name: "ziacommon.IDNameExternalID",
			typ:  reflect.TypeOf(ziacommon.IDNameExternalID{}),
			ignoredFields: ignoredBecause(
				"currently used inside explicitly modeled nested references; this entry catches SDK additions",
				"id",
				"name",
				"externalId",
				"extensions",
			),
		},
	}
}

func catalogFieldNames(spec resources.ResourceSpec) map[string]struct{} {
	fields := map[string]struct{}{}
	for _, field := range spec.Fields {
		fields[field.JSONField()] = struct{}{}
	}
	return fields
}

func exportedJSONFields(typ reflect.Type) []string {
	if typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	var fields []string
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if field.PkgPath != "" {
			continue
		}
		name := jsonFieldName(field)
		if name == "" || name == "-" {
			continue
		}
		fields = append(fields, name)
	}
	sort.Strings(fields)
	return fields
}

func jsonFieldName(field reflect.StructField) string {
	tag := field.Tag.Get("json")
	if tag == "-" {
		return "-"
	}
	if index := strings.IndexByte(tag, ','); index >= 0 {
		tag = tag[:index]
	}
	if tag != "" {
		return tag
	}
	return field.Name
}

func ignoredBecause(reason string, fields ...string) map[string]string {
	ignored := map[string]string{}
	for _, field := range fields {
		ignored[field] = reason
	}
	return ignored
}

func mergeIgnoredFields(items ...map[string]string) map[string]string {
	merged := map[string]string{}
	for _, item := range items {
		for field, reason := range item {
			merged[field] = reason
		}
	}
	return merged
}

func namesSet(names []string) map[string]struct{} {
	out := make(map[string]struct{}, len(names))
	for _, name := range names {
		out[name] = struct{}{}
	}
	return out
}

func specHasReadListOperation(spec resources.ResourceSpec) bool {
	for _, op := range spec.Operations {
		if op.Name == "list" && op.Capability == resources.CapabilityRead {
			return true
		}
	}
	return false
}

func resourceReviewKey(product resources.Product, name string) string {
	return string(product) + "/" + name
}
