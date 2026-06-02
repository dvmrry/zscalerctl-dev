package zscaler

import (
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/resources"

	advancedsettings "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/advanced_settings"
	advancedthreatsettings "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/advancedthreatsettings"
	"github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/alerts"
	authsettings "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/auth_settings"
	bandwidthclasses "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/bandwidth_control/bandwidth_classes"
	bandwidthcontrolrules "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/bandwidth_control/bandwidth_control_rules"
	cloudappinstances "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/cloud_app_instances"
	riskprofiles "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/cloudapplications/risk_profiles"
	cloudnss "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/cloudnss/cloudnss"
	nssservers "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/cloudnss/nss_servers"
	ziacommon "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/common"
	"github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/devicegroups"
	dlpicapservers "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/dlp/dlp_icap_servers"
	endusernotification "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/end_user_notification"
	filetypecontrol "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/filetypecontrol"
	customfiletypes "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/filetypecontrol/custom_file_types"
	firewalldnscontrolpolicies "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/firewalldnscontrolpolicies"
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
	zpagateways "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/forwarding_control_policy/zpa_gateways"
	"github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/location/locationgroups"
	"github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/location/locationmanagement"
	malwareprotection "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/malware_protection"
	mobilethreatsettings "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/mobile_threat_settings"
	natcontrol "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/nat_control_policies"
	organizationdetails "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/organization_details"
	rulelabels "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/rule_labels"
	sandboxrules "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/sandbox/sandbox_rules"
	sandboxsettings "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/sandbox/sandbox_settings"
	securitypolicysettings "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/security_policy_settings"
	"github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/sslinspection"
	tenancyrestriction "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/tenancy_restriction"
	timeintervals "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/time_intervals"
	gretunnels "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/trafficforwarding/gretunnels"
	staticips "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/trafficforwarding/staticips"
	"github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/urlcategories"
	"github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/urlfilteringpolicies"
	usergroups "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/usermanagement/groups"
	vzenclusters "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/vzen_clusters"
	vzennodes "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/vzen_nodes"
	"github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/workloadgroups"
	zpaappconnectorgroup "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/appconnectorgroup"
	zpaappservercontroller "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/appservercontroller"
	zpac2cipranges "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/c2c_ip_ranges"
	zpacloudconnectorgroup "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/cloud_connector_group"
	zpacbizpaprofile "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/cloudbrowserisolation/cbizpaprofile"
	zpamachinegroup "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/machinegroup"
	zpapostureprofile "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/postureprofile"
	zpaprivatecloudgroup "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/private_cloud_group"
	zpasegmentgroup "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/segmentgroup"
	zpaservergroup "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/servergroup"
	zpaserviceedgecontroller "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/serviceedgecontroller"
	zpaserviceedgegroup "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/serviceedgegroup"
	zpatrustednetwork "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/trustednetwork"
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

func TestReviewedSDKShapesCoverCatalogReadResources(t *testing.T) {
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
		if !specHasReadOperation(spec) {
			continue
		}
		key := resourceReviewKey(spec.Product, spec.Name)
		if _, ok := reviewed[key]; !ok {
			missing = append(missing, key)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Fatalf("reviewedSDKShapes() missing top-level SDK shape reviews for catalog read resources: %v", missing)
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
			name:         "authsettings.AuthenticationSettings",
			resource:     resources.ProductZIA,
			resourceName: resourceAuthSettings,
			typ:          reflect.TypeOf(authsettings.AuthenticationSettings{}),
			catalogFields: []string{
				"orgAuthType",
				"oneTimeAuth",
				"samlEnabled",
				"kerberosEnabled",
				"kerberosPwd",
				"authFrequency",
				"authCustomFrequency",
				"passwordStrength",
				"passwordExpiry",
				"lastSyncStartTime",
				"lastSyncEndTime",
				"mobileAdminSamlIdpEnabled",
				"autoProvision",
				"directorySyncMigrateToScimEnabled",
			},
			ignoredFields: map[string]string{},
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
			name: "ziacommon.IDCustom",
			typ:  reflect.TypeOf(ziacommon.IDCustom{}),
			ignoredFields: ignoredBecause(
				"used inside dropped or explicitly modeled custom ID references; parent catalog fields decide whether id/name can render",
				"id",
				"name",
			),
		},
		{
			name: "ziacommon.CommonNSS",
			typ:  reflect.TypeOf(ziacommon.CommonNSS{}),
			ignoredFields: ignoredBecause(
				"used inside dropped or explicitly modeled NSS feed references; parent catalog fields decide whether nested fields can render",
				"id",
				"pid",
				"name",
				"description",
				"deleted",
				"getlId",
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
			name:         "usergroups.Groups",
			resource:     resources.ProductZIA,
			resourceName: resourceGroups,
			typ:          reflect.TypeOf(usergroups.Groups{}),
			catalogFields: []string{
				"id",
				"name",
				"idpId",
				"comments",
				"isSystemDefined",
			},
		},
		{
			name:         "devicegroups.DeviceGroups",
			resource:     resources.ProductZIA,
			resourceName: resourceDeviceGroups,
			typ:          reflect.TypeOf(devicegroups.DeviceGroups{}),
			catalogFields: []string{
				"id",
				"name",
				"groupType",
				"description",
				"osType",
				"predefined",
				"deviceNames",
				"deviceCount",
			},
		},
		{
			name:         "workloadgroups.WorkloadGroup",
			resource:     resources.ProductZIA,
			resourceName: resourceWorkloadGroups,
			typ:          reflect.TypeOf(workloadgroups.WorkloadGroup{}),
			catalogFields: []string{
				"id",
				"name",
				"description",
				"expression",
				"lastModifiedTime",
			},
			ignoredFields: ignoredBecause(
				"structured expression details and admin references are mapped then dropped by projection",
				"expressionJson",
				"lastModifiedBy",
			),
		},
		{
			name:         "alerts.AlertSubscriptions",
			resource:     resources.ProductZIA,
			resourceName: resourceAlertSubs,
			typ:          reflect.TypeOf(alerts.AlertSubscriptions{}),
			catalogFields: []string{
				"id",
				"description",
				"email",
				"deleted",
				"pt0Severities",
				"secureSeverities",
				"manageSeverities",
				"complySeverities",
				"systemSeverities",
			},
		},
		{
			name:         "cloudappinstances.CloudApplicationInstances",
			resource:     resources.ProductZIA,
			resourceName: resourceCloudAppInsts,
			typ:          reflect.TypeOf(cloudappinstances.CloudApplicationInstances{}),
			catalogFields: []string{
				"instanceId",
				"instanceType",
				"instanceName",
				"modifiedAt",
				"modifiedBy",
				"instanceIdentifiers",
			},
		},
		{
			name: "cloudappinstances.InstanceIdentifiers",
			typ:  reflect.TypeOf(cloudappinstances.InstanceIdentifiers{}),
			ignoredFields: ignoredBecause(
				"covered by dropped instanceIdentifiers parent",
				"instanceId",
				"instanceIdentifier",
				"instanceIdentifierName",
				"identifierType",
				"modifiedAt",
				"modifiedBy",
			),
		},
		{
			name:         "tenancyrestriction.TenancyRestrictionProfile",
			resource:     resources.ProductZIA,
			resourceName: resourceTenancyProfiles,
			typ:          reflect.TypeOf(tenancyrestriction.TenancyRestrictionProfile{}),
			catalogFields: []string{
				"id",
				"name",
				"appType",
				"description",
				"itemTypePrimary",
				"itemTypeSecondary",
				"restrictPersonalO365Domains",
				"allowGoogleConsumers",
				"msLoginServicesTrV2",
				"allowGoogleVisitors",
				"allowGcpCloudStorageRead",
				"itemDataPrimary",
				"itemDataSecondary",
				"itemValue",
				"lastModifiedTime",
				"lastModifiedUserId",
			},
		},
		{
			name:         "vzenclusters.VZENClusters",
			resource:     resources.ProductZIA,
			resourceName: resourceVZENClusters,
			typ:          reflect.TypeOf(vzenclusters.VZENClusters{}),
			catalogFields: []string{
				"id",
				"name",
				"status",
				"ipAddress",
				"subnetMask",
				"defaultGateway",
				"type",
				"ipSecEnabled",
				"virtualZenNodes",
			},
		},
		{
			name:         "vzennodes.VZENNodes",
			resource:     resources.ProductZIA,
			resourceName: resourceVZENNodes,
			typ:          reflect.TypeOf(vzennodes.VZENNodes{}),
			catalogFields: []string{
				"id",
				"zgatewayId",
				"name",
				"status",
				"inProduction",
				"ipAddress",
				"subnetMask",
				"defaultGateway",
				"type",
				"ipSecEnabled",
				"onDemandSupportTunnelEnabled",
				"establishSupportTunnelEnabled",
				"loadBalancerIpAddress",
				"deploymentMode",
				"clusterName",
				"vzenSkuType",
			},
		},
		{
			name:         "dlpicapservers.DLPICAPServers",
			resource:     resources.ProductZIA,
			resourceName: resourceDLPICAPServers,
			typ:          reflect.TypeOf(dlpicapservers.DLPICAPServers{}),
			catalogFields: []string{
				"id",
				"name",
				"url",
				"status",
			},
		},
		{
			name:         "filetypecontrol.FileTypeRules",
			resource:     resources.ProductZIA,
			resourceName: resourceFileTypeRules,
			typ:          reflect.TypeOf(filetypecontrol.FileTypeRules{}),
			catalogFields: []string{
				"accessControl",
				"activeContent",
				"browserEunTemplateId",
				"capturePCAP",
				"cloudApplications",
				"departments",
				"description",
				"deviceGroups",
				"deviceTrustLevels",
				"devices",
				"fileTypes",
				"filteringAction",
				"groups",
				"id",
				"labels",
				"lastModifiedBy",
				"lastModifiedTime",
				"locationGroups",
				"locations",
				"maxSize",
				"minSize",
				"name",
				"operation",
				"order",
				"passwordProtected",
				"protocols",
				"rank",
				"sizeQuota",
				"state",
				"timeQuota",
				"timeWindows",
				"unscannable",
				"urlCategories",
				"users",
				"zpaAppSegments",
			},
		},
		{
			name:         "sandboxrules.SandboxRules",
			resource:     resources.ProductZIA,
			resourceName: resourceSandboxRules,
			typ:          reflect.TypeOf(sandboxrules.SandboxRules{}),
			catalogFields: []string{
				"accessControl",
				"baPolicyCategories",
				"baRuleAction",
				"byThreatScore",
				"defaultRule",
				"departments",
				"description",
				"deviceGroups",
				"devices",
				"fileTypes",
				"firstTimeEnable",
				"firstTimeOperation",
				"groups",
				"id",
				"labels",
				"lastModifiedBy",
				"lastModifiedTime",
				"locationGroups",
				"locations",
				"mlActionEnabled",
				"name",
				"order",
				"protocols",
				"rank",
				"state",
				"timeWindows",
				"urlCategories",
				"users",
				"zpaAppSegments",
			},
		},
		{
			name:         "firewalldnscontrolpolicies.FirewallDNSRules",
			resource:     resources.ProductZIA,
			resourceName: resourceFirewallDNSRules,
			typ:          reflect.TypeOf(firewalldnscontrolpolicies.FirewallDNSRules{}),
			catalogFields: []string{
				"accessControl",
				"action",
				"applicationGroups",
				"applications",
				"blockResponseCode",
				"capturePCAP",
				"defaultDnsRuleNameUsed",
				"defaultRule",
				"departments",
				"description",
				"destAddresses",
				"destCountries",
				"destIpCategories",
				"destIpGroups",
				"destIpv6Groups",
				"deviceGroups",
				"devices",
				"dnsGateway",
				"dnsRuleRequestTypes",
				"ednsEcsObject",
				"groups",
				"id",
				"isWebEunEnabled",
				"labels",
				"lastModifiedBy",
				"lastModifiedTime",
				"locationGroups",
				"locations",
				"name",
				"order",
				"predefined",
				"protocols",
				"rank",
				"redirectIp",
				"resCategories",
				"sourceCountries",
				"srcIpGroups",
				"srcIps",
				"srcIpv6Groups",
				"state",
				"timeWindows",
				"users",
				"zpaIpGroup",
			},
		},
		{
			name:         "customfiletypes.CustomFileTypes",
			resource:     resources.ProductZIA,
			resourceName: resourceCustomFileTypes,
			typ:          reflect.TypeOf(customfiletypes.CustomFileTypes{}),
			catalogFields: []string{
				"description",
				"extension",
				"fileTypeId",
				"id",
				"name",
			},
		},
		{
			name:         "riskprofiles.RiskProfiles",
			resource:     resources.ProductZIA,
			resourceName: resourceRiskProfiles,
			typ:          reflect.TypeOf(riskprofiles.RiskProfiles{}),
			catalogFields: []string{
				"adminAuditLogs",
				"certifications",
				"createTime",
				"customTags",
				"dataBreach",
				"dataEncryptionInTransit",
				"dnsCaaPolicy",
				"domainBasedMessageAuth",
				"domainKeysIdentifiedMail",
				"evasive",
				"excludeCertificates",
				"fileSharing",
				"httpSecurityHeaders",
				"id",
				"lastModTime",
				"malwareScanningForContent",
				"mfaSupport",
				"modifiedBy",
				"passwordStrength",
				"poorItemsOfService",
				"profileName",
				"profileType",
				"remoteScreenSharing",
				"riskIndex",
				"senderPolicyFramework",
				"sourceIpRestrictions",
				"sslCertKeySize",
				"sslCertValidity",
				"sslPinned",
				"status",
				"supportForWaf",
				"vulnerability",
				"vulnerabilityDisclosure",
				"vulnerableToHeartBleed",
				"vulnerableToLogJam",
				"vulnerableToPoodle",
				"weakCipherSupport",
			},
		},
		{
			name:         "nssservers.NSSServers",
			resource:     resources.ProductZIA,
			resourceName: resourceNSSServers,
			typ:          reflect.TypeOf(nssservers.NSSServers{}),
			catalogFields: []string{
				"icapSvrId",
				"id",
				"name",
				"state",
				"status",
				"type",
			},
		},
		{
			name:         "cloudnss.NSSFeed",
			resource:     resources.ProductZIA,
			resourceName: resourceNSSFeeds,
			typ:          reflect.TypeOf(cloudnss.NSSFeed{}),
			catalogFields: []string{
				"actionFilter",
				"activity",
				"advUserAgents",
				"advancedThreats",
				"alerts",
				"auditLogType",
				"authenticationToken",
				"authenticationUrl",
				"base64EncodedCertificate",
				"buckets",
				"casbAction",
				"casbApplications",
				"casbFileType",
				"casbFileTypeSuperCategories",
				"casbPolicyTypes",
				"casbSeverity",
				"casbTenant",
				"channelName",
				"clientDestinationIps",
				"clientDestinationPorts",
				"clientId",
				"clientIps",
				"clientSecret",
				"clientSourceIps",
				"clientSourcePorts",
				"cloudNss",
				"connectionHeaders",
				"connectionURL",
				"countries",
				"customEscapedCharacter",
				"departments",
				"direction",
				"dlpDictionaries",
				"dlpEngines",
				"dnsActions",
				"dnsRequestTypes",
				"dnsResponseTypes",
				"dnsResponses",
				"domains",
				"downloadTime",
				"durations",
				"emailDLPLogType",
				"emailDlpPolicyAction",
				"endPointDLPLogType",
				"epsRateLimit",
				"event",
				"externalCollaborators",
				"externalOwners",
				"feedOutputFormat",
				"feedStatus",
				"fileName",
				"fileSizes",
				"fileSource",
				"fileTypeCategories",
				"fileTypeSuperCategories",
				"firewallActions",
				"firewallLoggingMode",
				"fullUrls",
				"grantType",
				"hostNames",
				"id",
				"inBoundBytes",
				"internalCollaborators",
				"internalIps",
				"itsmObjectType",
				"jsonArrayToggle",
				"lastSuccessFullTest",
				"locationGroups",
				"locations",
				"malwareClasses",
				"malwareNames",
				"maxBatchSize",
				"messageSize",
				"name",
				"natActions",
				"nssFeedType",
				"nssLogType",
				"nssType",
				"nwApplications",
				"nwServices",
				"oauthAuthentication",
				"objectName",
				"objectType",
				"objectType1",
				"objectType2",
				"outBoundBytes",
				"pageRiskIndexes",
				"policyReasons",
				"projectName",
				"protocolTypes",
				"refererUrls",
				"repoName",
				"requestMethods",
				"requestSizes",
				"responseCodes",
				"responseSizes",
				"rules",
				"scanTime",
				"scope",
				"senderName",
				"serverDestinationIps",
				"serverIps",
				"serverSourceIps",
				"serverSourcePorts",
				"sessionCounts",
				"siemType",
				"testConnectivityCode",
				"threatNames",
				"timeZone",
				"trafficForwards",
				"transactionSizes",
				"tunnelDestIps",
				"tunnelIps",
				"tunnelSourceIps",
				"tunnelSourcePort",
				"tunnelTypes",
				"urlCategories",
				"urlClasses",
				"urlSuperCategories",
				"userAgents",
				"userObfuscation",
				"users",
				"vpnCredentials",
				"webApplicationClasses",
				"webApplications",
				"webTrafficForwards",
			},
		},
		{
			name:         "zpagateways.ZPAGateways",
			resource:     resources.ProductZIA,
			resourceName: resourceZPAGateways,
			typ:          reflect.TypeOf(zpagateways.ZPAGateways{}),
			catalogFields: []string{
				"description",
				"id",
				"lastModifiedBy",
				"lastModifiedTime",
				"name",
				"type",
				"zpaAppSegments",
				"zpaServerGroup",
				"zpaTenantId",
			},
		},
		{
			name: "zpagateways.ZPAServerGroup",
			typ:  reflect.TypeOf(zpagateways.ZPAServerGroup{}),
			ignoredFields: ignoredBecause(
				"nested ZPA server-group references are explicitly modeled in the zpa-gateways catalog; this entry catches SDK additions",
				"id",
				"name",
				"externalId",
				"extensions",
			),
		},
		{
			name: "zpagateways.ZPAAppSegments",
			typ:  reflect.TypeOf(zpagateways.ZPAAppSegments{}),
			ignoredFields: ignoredBecause(
				"nested ZPA application-segment references are explicitly modeled in the zpa-gateways catalog; this entry catches SDK additions",
				"id",
				"name",
				"externalId",
				"extensions",
			),
		},
		{
			name:          "advancedsettings.AdvancedSettings",
			resource:      resources.ProductZIA,
			resourceName:  resourceAdvancedSettings,
			typ:           reflect.TypeOf(advancedsettings.AdvancedSettings{}),
			catalogFields: catalogFieldsFor(resources.ProductZIA, resourceAdvancedSettings),
		},
		{
			name:          "advancedthreatsettings.AdvancedThreatSettings",
			resource:      resources.ProductZIA,
			resourceName:  resourceAdvancedThreatSettings,
			typ:           reflect.TypeOf(advancedthreatsettings.AdvancedThreatSettings{}),
			catalogFields: catalogFieldsFor(resources.ProductZIA, resourceAdvancedThreatSettings),
		},
		{
			name:          "mobilethreatsettings.MobileAdvanceThreatSettings",
			resource:      resources.ProductZIA,
			resourceName:  resourceMobileThreatSettings,
			typ:           reflect.TypeOf(mobilethreatsettings.MobileAdvanceThreatSettings{}),
			catalogFields: catalogFieldsFor(resources.ProductZIA, resourceMobileThreatSettings),
		},
		{
			name:          "sandboxsettings.BaAdvancedSettings",
			resource:      resources.ProductZIA,
			resourceName:  resourceSandboxSettings,
			typ:           reflect.TypeOf(sandboxsettings.BaAdvancedSettings{}),
			catalogFields: catalogFieldsFor(resources.ProductZIA, resourceSandboxSettings),
		},
		{
			name:          "endusernotification.UserNotificationSettings",
			resource:      resources.ProductZIA,
			resourceName:  resourceEndUserNotification,
			typ:           reflect.TypeOf(endusernotification.UserNotificationSettings{}),
			catalogFields: catalogFieldsFor(resources.ProductZIA, resourceEndUserNotification),
		},
		{
			name:          "organizationdetails.Organization",
			resource:      resources.ProductZIA,
			resourceName:  resourceOrgInformation,
			typ:           reflect.TypeOf(organizationdetails.Organization{}),
			catalogFields: catalogFieldsFor(resources.ProductZIA, resourceOrgInformation),
		},
		{
			name:          "malwareprotection.MalwarePolicy",
			resource:      resources.ProductZIA,
			resourceName:  resourceATPMalwarePolicy,
			typ:           reflect.TypeOf(malwareprotection.MalwarePolicy{}),
			catalogFields: catalogFieldsFor(resources.ProductZIA, resourceATPMalwarePolicy),
		},
		{
			name:          "malwareprotection.MalwareSettings",
			resource:      resources.ProductZIA,
			resourceName:  resourceATPMalwareSettings,
			typ:           reflect.TypeOf(malwareprotection.MalwareSettings{}),
			catalogFields: catalogFieldsFor(resources.ProductZIA, resourceATPMalwareSettings),
		},
		{
			name:          "malwareprotection.ATPMalwareInspection",
			resource:      resources.ProductZIA,
			resourceName:  resourceATPMalwareInspection,
			typ:           reflect.TypeOf(malwareprotection.ATPMalwareInspection{}),
			catalogFields: catalogFieldsFor(resources.ProductZIA, resourceATPMalwareInspection),
		},
		{
			name:          "malwareprotection.ATPMalwareProtocols",
			resource:      resources.ProductZIA,
			resourceName:  resourceATPMalwareProtocols,
			typ:           reflect.TypeOf(malwareprotection.ATPMalwareProtocols{}),
			catalogFields: catalogFieldsFor(resources.ProductZIA, resourceATPMalwareProtocols),
		},
		{
			name:          "advancedthreatsettings.MaliciousURLs",
			resource:      resources.ProductZIA,
			resourceName:  resourceMaliciousURLs,
			typ:           reflect.TypeOf(advancedthreatsettings.MaliciousURLs{}),
			catalogFields: catalogFieldsFor(resources.ProductZIA, resourceMaliciousURLs),
		},
		{
			name:          "advancedthreatsettings.SecurityExceptions",
			resource:      resources.ProductZIA,
			resourceName:  resourceSecurityExceptions,
			typ:           reflect.TypeOf(advancedthreatsettings.SecurityExceptions{}),
			catalogFields: catalogFieldsFor(resources.ProductZIA, resourceSecurityExceptions),
		},
		{
			name:          "securitypolicysettings.ListUrls/allow",
			resource:      resources.ProductZIA,
			resourceName:  resourceSecurityPolicyURLAllowlist,
			typ:           reflect.TypeOf(securitypolicysettings.ListUrls{}),
			catalogFields: catalogFieldsFor(resources.ProductZIA, resourceSecurityPolicyURLAllowlist),
		},
		{
			name:          "securitypolicysettings.ListUrls/deny",
			resource:      resources.ProductZIA,
			resourceName:  resourceSecurityPolicyURLDenylist,
			typ:           reflect.TypeOf(securitypolicysettings.ListUrls{}),
			catalogFields: catalogFieldsFor(resources.ProductZIA, resourceSecurityPolicyURLDenylist),
		},
		{
			name:         "zpaservergroup.ServerGroup",
			resource:     resources.ProductZPA,
			resourceName: resourceZPAServerGroups,
			typ:          reflect.TypeOf(zpaservergroup.ServerGroup{}),
			catalogFields: []string{
				"id",
				"enabled",
				"name",
				"description",
				"ipAnchored",
				"configSpace",
				"dynamicDiscovery",
				"extranetEnabled",
				"creationTime",
				"modifiedBy",
				"modifiedTime",
				"microtenantId",
				"microtenantName",
				"readOnly",
				"restrictionType",
				"zscalerManaged",
				"appConnectorGroups",
				"servers",
				"applications",
				"extranetDTO",
			},
		},
		{
			name:         "zpasegmentgroup.SegmentGroup",
			resource:     resources.ProductZPA,
			resourceName: resourceZPASegmentGroups,
			typ:          reflect.TypeOf(zpasegmentgroup.SegmentGroup{}),
			catalogFields: []string{
				"id",
				"name",
				"description",
				"enabled",
				"configSpace",
				"creationTime",
				"modifiedBy",
				"modifiedTime",
				"policyMigrated",
				"tcpKeepAliveEnabled",
				"microtenantId",
				"microtenantName",
				"addedApps",
				"deletedApps",
				"applications",
				"applicationNames",
			},
		},
		{
			name:         "zpaappconnectorgroup.AppConnectorGroup",
			resource:     resources.ProductZPA,
			resourceName: resourceZPAConnectorGrps,
			typ:          reflect.TypeOf(zpaappconnectorgroup.AppConnectorGroup{}),
			catalogFields: []string{
				"id",
				"name",
				"description",
				"enabled",
				"cityCountry",
				"countryCode",
				"creationTime",
				"dnsQueryType",
				"connectorGroupType",
				"geoLocationId",
				"latitude",
				"location",
				"longitude",
				"modifiedBy",
				"modifiedTime",
				"overrideVersionProfile",
				"praEnabled",
				"wafDisabled",
				"upgradeDay",
				"upgradeTimeInSecs",
				"versionProfileId",
				"versionProfileName",
				"versionProfileVisibilityScope",
				"tcpQuickAckApp",
				"tcpQuickAckAssistant",
				"useInDrMode",
				"tcpQuickAckReadAssistant",
				"lssAppConnectorGroup",
				"microtenantId",
				"microtenantName",
				"siteId",
				"siteName",
				"readOnly",
				"restrictionType",
				"zscalerManaged",
				"dcHostingInfo",
				"nameWithoutTrim",
				"serverGroups",
				"connectors",
				"npAssistantGroup",
				"enrollmentCertId",
			},
		},
		{
			name:         "zpaappservercontroller.ApplicationServer",
			resource:     resources.ProductZPA,
			resourceName: resourceZPAAppServers,
			typ:          reflect.TypeOf(zpaappservercontroller.ApplicationServer{}),
			catalogFields: []string{
				"address",
				"appServerGroupIds",
				"configSpace",
				"creationTime",
				"description",
				"enabled",
				"id",
				"modifiedBy",
				"modifiedTime",
				"name",
				"microtenantId",
				"microtenantName",
			},
		},
		{
			name:         "zpamachinegroup.MachineGroup",
			resource:     resources.ProductZPA,
			resourceName: resourceZPAMachineGroups,
			typ:          reflect.TypeOf(zpamachinegroup.MachineGroup{}),
			catalogFields: []string{
				"id",
				"name",
				"description",
				"enabled",
				"creationTime",
				"machines",
				"modifiedBy",
				"modifiedTime",
				"microtenantId",
				"microtenantName",
			},
		},
		{
			name:         "zpatrustednetwork.TrustedNetwork",
			resource:     resources.ProductZPA,
			resourceName: resourceZPATrustedNets,
			typ:          reflect.TypeOf(zpatrustednetwork.TrustedNetwork{}),
			catalogFields: []string{
				"creationTime",
				"domain",
				"id",
				"masterCustomerId",
				"modifiedBy",
				"modifiedTime",
				"name",
				"networkId",
				"zscalerCloud",
			},
		},
		{
			name:         "zpaserviceedgegroup.ServiceEdgeGroup",
			resource:     resources.ProductZPA,
			resourceName: resourceZPAServiceGrps,
			typ:          reflect.TypeOf(zpaserviceedgegroup.ServiceEdgeGroup{}),
			catalogFields: []string{
				"altCloud",
				"city",
				"cityCountry",
				"countryCode",
				"creationTime",
				"description",
				"enabled",
				"enrollmentCertId",
				"exclusiveForBusinessContinuity",
				"geoLocationId",
				"graceDistanceEnabled",
				"graceDistanceValue",
				"graceDistanceValueUnit",
				"id",
				"isPublic",
				"latitude",
				"location",
				"longitude",
				"microtenantId",
				"microtenantName",
				"modifiedBy",
				"modifiedTime",
				"name",
				"nameWithoutTrim",
				"objectType",
				"overrideVersionProfile",
				"readOnly",
				"restrictedEntity",
				"restrictionType",
				"scopeName",
				"serviceEdges",
				"siteId",
				"siteName",
				"trustedNetworks",
				"upgradeDay",
				"upgradeTimeInSecs",
				"useInDrMode",
				"versionProfileId",
				"versionProfileName",
				"versionProfileVisibilityScope",
				"zscalerManaged",
			},
		},
		{
			name:         "zpaserviceedgecontroller.ServiceEdgeController",
			resource:     resources.ProductZPA,
			resourceName: resourceZPAServiceEdges,
			typ:          reflect.TypeOf(zpaserviceedgecontroller.ServiceEdgeController{}),
			catalogFields: []string{
				"applicationStartTime",
				"controlChannelStatus",
				"creationTime",
				"ctrlBrokerName",
				"currentVersion",
				"description",
				"enabled",
				"enrollmentCert",
				"expectedUpgradeTime",
				"expectedVersion",
				"fingerprint",
				"id",
				"ipAcl",
				"issuedCertId",
				"lastBrokerConnectTime",
				"lastBrokerConnectTimeDuration",
				"lastBrokerDisconnectTime",
				"lastBrokerDisconnectTimeDuration",
				"lastUpgradeTime",
				"latitude",
				"listenIps",
				"location",
				"longitude",
				"microtenantId",
				"microtenantName",
				"modifiedBy",
				"modifiedTime",
				"name",
				"platform",
				"platformDetail",
				"previousVersion",
				"privateBrokerVersion",
				"privateIp",
				"provisioningKeyId",
				"provisioningKeyName",
				"publicIp",
				"publishIps",
				"publishIpv6",
				"runtimeOS",
				"sargeVersion",
				"serviceEdgeGroupId",
				"serviceEdgeGroupName",
				"upgradeAttempt",
				"upgradeStatus",
			},
		},
		{
			name:         "zpacloudconnectorgroup.CloudConnectorGroup",
			resource:     resources.ProductZPA,
			resourceName: resourceZPACloudConnGrps,
			typ:          reflect.TypeOf(zpacloudconnectorgroup.CloudConnectorGroup{}),
			catalogFields: []string{
				"cloudConnectors",
				"creationTime",
				"description",
				"enabled",
				"geoLocationId",
				"id",
				"modifiedBy",
				"modifiedTime",
				"name",
				"ziaCloud",
				"ziaOrgId",
				"znfGroupType",
			},
		},
		{
			name:         "zpapostureprofile.PostureProfile",
			resource:     resources.ProductZPA,
			resourceName: resourceZPAPostureProfs,
			typ:          reflect.TypeOf(zpapostureprofile.PostureProfile{}),
			catalogFields: []string{
				"applyToMachineTunnelEnabled",
				"creationTime",
				"crlCheckEnabled",
				"domain",
				"id",
				"masterCustomerId",
				"modifiedBy",
				"modifiedTime",
				"name",
				"nonExportablePrivateKeyEnabled",
				"platform",
				"postureType",
				"postureUdid",
				"rootCert",
				"zscalerCloud",
				"zscalerCustomerId",
			},
		},
		{
			name:         "zpacbizpaprofile.ZPAProfiles",
			resource:     resources.ProductZPA,
			resourceName: resourceZPACBIZPAProfs,
			typ:          reflect.TypeOf(zpacbizpaprofile.ZPAProfiles{}),
			catalogFields: []string{
				"cbiProfileId",
				"cbiTenantId",
				"cbiUrl",
				"creationTime",
				"description",
				"enabled",
				"id",
				"modifiedBy",
				"modifiedTime",
				"name",
			},
		},
		{
			name:         "zpac2cipranges.IPRanges",
			resource:     resources.ProductZPA,
			resourceName: resourceZPAC2CIPRanges,
			typ:          reflect.TypeOf(zpac2cipranges.IPRanges{}),
			catalogFields: []string{
				"availableIps",
				"countryCode",
				"creationTime",
				"customerId",
				"description",
				"enabled",
				"id",
				"ipRangeBegin",
				"ipRangeEnd",
				"isDeleted",
				"latitudeInDb",
				"location",
				"locationHint",
				"longitudeInDb",
				"modifiedBy",
				"modifiedTime",
				"name",
				"sccmFlag",
				"subnetCidr",
				"totalIps",
				"usedIps",
			},
		},
		{
			name:         "zpaprivatecloudgroup.PrivateCloudGroup",
			resource:     resources.ProductZPA,
			resourceName: resourceZPAPrivateClGrps,
			typ:          reflect.TypeOf(zpaprivatecloudgroup.PrivateCloudGroup{}),
			catalogFields: []string{
				"city",
				"cityCountry",
				"countryCode",
				"creationTime",
				"description",
				"enabled",
				"geoLocationId",
				"id",
				"isPublic",
				"latitude",
				"location",
				"longitude",
				"microtenantId",
				"microtenantName",
				"modifiedBy",
				"modifiedTime",
				"name",
				"overrideVersionProfile",
				"readOnly",
				"restrictionType",
				"siteId",
				"siteName",
				"upgradeDay",
				"upgradeTimeInSecs",
				"versionProfileId",
				"versionProfileName",
				"zscalerManaged",
			},
		},
		{
			name: "zpacloudconnectorgroup.CloudConnectors",
			typ:  reflect.TypeOf(zpacloudconnectorgroup.CloudConnectors{}),
			ignoredFields: ignoredBecause(
				"covered by dropped cloudConnectors parent",
				"creationTime",
				"description",
				"enabled",
				"fingerprint",
				"id",
				"ipAcl",
				"issuedCertId",
				"microtenantId",
				"microtenantName",
				"modifiedBy",
				"modifiedTime",
				"name",
				"readOnly",
				"restrictionType",
				"signingCert",
				"zscalerManaged",
			),
		},
		{
			name: "workloadgroups.WorkloadTagExpression",
			typ:  reflect.TypeOf(workloadgroups.WorkloadTagExpression{}),
			ignoredFields: ignoredBecause(
				"covered by dropped expressionJson parent",
				"expressionContainers",
			),
		},
		{
			name: "workloadgroups.ExpressionContainer",
			typ:  reflect.TypeOf(workloadgroups.ExpressionContainer{}),
			ignoredFields: ignoredBecause(
				"covered by dropped expressionJson parent",
				"tagType",
				"operator",
				"tagContainer",
			),
		},
		{
			name: "workloadgroups.TagContainer",
			typ:  reflect.TypeOf(workloadgroups.TagContainer{}),
			ignoredFields: ignoredBecause(
				"covered by dropped expressionJson parent",
				"tags",
				"operator",
			),
		},
		{
			name: "workloadgroups.Tags",
			typ:  reflect.TypeOf(workloadgroups.Tags{}),
			ignoredFields: ignoredBecause(
				"covered by dropped expressionJson parent",
				"key",
				"value",
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

func catalogFieldsFor(product resources.Product, name string) []string {
	spec, ok := resources.FindSpec(product, name)
	if !ok {
		panic("missing catalog spec " + resourceReviewKey(product, name))
	}
	fields := make([]string, 0, len(spec.Fields))
	for _, field := range spec.Fields {
		fields = append(fields, field.JSONField())
	}
	sort.Strings(fields)
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

func specHasReadOperation(spec resources.ResourceSpec) bool {
	for _, op := range spec.Operations {
		if op.Capability == resources.CapabilityRead {
			return true
		}
	}
	return false
}

func resourceReviewKey(product resources.Product, name string) string {
	return string(product) + "/" + name
}
