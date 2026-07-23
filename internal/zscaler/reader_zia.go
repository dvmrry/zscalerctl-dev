package zscaler

import (
	"context"
	"fmt"
	"net/url"
	"sort"

	zsdk "github.com/zscaler/zscaler-sdk-go/v3/zscaler"
	activation "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/activation"
	ziaadminusers "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/adminuserrolemgmt/admins"
	ziaadminroles "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/adminuserrolemgmt/roles"
	advancedsettings "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/advanced_settings"
	advancedthreatsettings "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/advancedthreatsettings"
	"github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/alerts"
	authsettings "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/auth_settings"
	bandwidthclasses "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/bandwidth_control/bandwidth_classes"
	bandwidthcontrolrules "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/bandwidth_control/bandwidth_control_rules"
	browsercontrolsettings "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/browser_control_settings"
	browserisolation "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/browser_isolation"
	c2cincidentreceiver "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/c2c_incident_receiver"
	cloudappinstances "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/cloud_app_instances"
	cloudappcontrol "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/cloudappcontrol"
	cloudapplications "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/cloudapplications/cloudapplications"
	riskprofiles "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/cloudapplications/risk_profiles"
	cloudnss "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/cloudnss/cloudnss"
	nssservers "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/cloudnss/nss_servers"
	ziacommon "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/common"
	"github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/devicegroups"
	dlpengines "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/dlp/dlp_engines"
	dlpexactdatamatch "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/dlp/dlp_exact_data_match"
	dlpedmlite "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/dlp/dlp_exact_data_match_lite"
	dlpicapservers "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/dlp/dlp_icap_servers"
	dlpidmprofilelite "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/dlp/dlp_idm_profile_lite"
	dlpidmprofiles "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/dlp/dlp_idm_profiles"
	dlpincidentreceivers "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/dlp/dlp_incident_receiver_servers"
	dlpnotificationtemplates "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/dlp/dlp_notification_templates"
	dlpwebrules "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/dlp/dlp_web_rules"
	dlpdictionaries "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/dlp/dlpdictionaries"
	emailprofiles "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/email_profiles"
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
	networkapplications "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/firewallpolicies/networkapplications"
	networkservicegroups "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/firewallpolicies/networkservicegroups"
	networkservices "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/firewallpolicies/networkservices"
	"github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/firewallpolicies/timewindow"
	forwardingrules "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/forwarding_control_policy/forwarding_rules"
	"github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/forwarding_control_policy/proxies"
	proxygateways "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/forwarding_control_policy/proxy_gateways"
	zpagateways "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/forwarding_control_policy/zpa_gateways"
	ftpcontrolpolicy "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/ftp_control_policy"
	intermediatecacertificates "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/intermediatecacertificates"
	ipspolicies "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/ips_control_policies/ips_policies"
	ipssignaturerules "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/ips_control_policies/ips_signature_rules"
	"github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/location/locationgroups"
	"github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/location/locationmanagement"
	malwareprotection "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/malware_protection"
	mobilethreatsettings "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/mobile_threat_settings"
	natcontrol "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/nat_control_policies"
	organizationdetails "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/organization_details"
	pacfiles "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/pacfiles"
	remoteassistance "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/remote_assistance"
	rulelabels "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/rule_labels"
	saassecurityapi "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/saas_security_api"
	casbdlprules "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/saas_security_api/casb_dlp_rules"
	casbmalwarerules "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/saas_security_api/casb_malware_rules"
	sandboxrules "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/sandbox/sandbox_rules"
	sandboxsettings "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/sandbox/sandbox_settings"
	securebrowsing "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/secure_browsing"
	securitypolicysettings "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/security_policy_settings"
	"github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/sslinspection"
	tenancyrestriction "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/tenancy_restriction"
	timeintervals "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/time_intervals"
	dcexclusions "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/trafficforwarding/dc_exclusions"
	gretunnels "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/trafficforwarding/gretunnels"
	ipv6config "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/trafficforwarding/ipv6_config"
	staticips "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/trafficforwarding/staticips"
	subclouds "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/trafficforwarding/sub_clouds"
	"github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/urlcategories"
	"github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/urlfilteringpolicies"
	userauthsettings "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/user_authentication_settings"
	userdepartments "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/usermanagement/departments"
	usergroups "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/usermanagement/groups"
	ziausers "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/usermanagement/users"
	vzenclusters "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/vzen_clusters"
	vzennodes "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/vzen_nodes"
	"github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/workloadgroups"

	"github.com/dvmrry/zscalerctl/internal/resources"
)

// ziaMaxPages is a fail-closed ceiling on the number of pages the bounded ZIA
// paginators will fetch. Like the zcc/zidentity guards, termination otherwise
// relies entirely on the server returning a short final page; an endpoint that
// keeps returning a persistently-full page would loop until --timeout fires on
// every request. At the smallest 100-record page size used by these wrappers,
// the ceiling still admits 100,000 records, but converts a pathological
// infinite loop into a visible, descriptive error.
const ziaMaxPages = 1000

// ziaPaginate walks every page of a ZIA list endpoint, mirroring the SDK's
// ReadAllPages contract (advance until a page returns fewer than pageSize
// records) while enforcing the ziaMaxPages ceiling the vendored ReadAllPages
// lacks. fetchPage is injectable so the ceiling is unit-testable without a live
// tenant.
func ziaPaginate[T any](ctx context.Context, pageSize int, fetchPage func(ctx context.Context, page, pageSize int) ([]T, error)) ([]T, error) {
	var all []T
	for page := 1; ; page++ {
		if page > ziaMaxPages {
			return nil, fmt.Errorf("zia pagination exceeded the ceiling of %d pages (%d records); the endpoint kept returning full pages, so completeness cannot be guaranteed", ziaMaxPages, len(all))
		}
		items, err := fetchPage(ctx, page, pageSize)
		if err != nil {
			return nil, err
		}
		all = append(all, items...)
		if len(items) < pageSize {
			break
		}
	}
	return all, nil
}

func getZIAAllPages[T any](
	ctx context.Context,
	service *zsdk.Service,
	endpoint string,
) ([]T, error) {
	return getZIAAllPagesWithSize[T](ctx, service, endpoint, ziacommon.GetPageSize())
}

func getZIAAllPagesWithSize[T any](
	ctx context.Context,
	service *zsdk.Service,
	endpoint string,
	pageSize int,
) ([]T, error) {
	items, err := ziaPaginate(ctx, pageSize, func(ctx context.Context, page, size int) ([]T, error) {
		var pageItems []T
		err := ziacommon.ReadPage(ctx, service.Client, endpoint, page, &pageItems, size)
		return pageItems, err
	})
	if err != nil {
		return nil, err
	}
	items, err = zsdk.ApplyJMESPathFromContext(ctx, items)
	if err != nil {
		return nil, fmt.Errorf("apply zia list filter: %w", err)
	}
	return items, nil
}

func ziaSortedListEndpoint(service *zsdk.Service, endpoint string) string {
	query := url.Values{}
	if service.SortBy != "" {
		query.Set("sortBy", string(service.SortBy))
	}
	if service.SortOrder != "" {
		query.Set("sortOrder", string(service.SortOrder))
	}
	if len(query) == 0 {
		return endpoint
	}
	return endpoint + "?" + query.Encode()
}

func getZIASublocationsAllPages(ctx context.Context, service *zsdk.Service) ([]locationmanagement.Locations, error) {
	parents, err := getZIALocationsAllPages(ctx, service)
	if err != nil {
		return nil, err
	}

	var sublocations []locationmanagement.Locations
	for _, parent := range parents {
		endpoint := fmt.Sprintf("/zia/api/v1/locations/%d/sublocations", parent.ID)
		items, err := getZIAAllPages[locationmanagement.Locations](ctx, service, endpoint)
		if err != nil {
			return nil, err
		}
		sublocations = append(sublocations, items...)
	}
	return sublocations, nil
}

func getZIASublocationByID(
	ctx context.Context,
	service *zsdk.Service,
	id int,
) (*locationmanagement.Locations, error) {
	sublocations, err := getZIASublocationsAllPages(ctx, service)
	if err != nil {
		return nil, err
	}
	for index := range sublocations {
		if sublocations[index].ID == id {
			return &sublocations[index], nil
		}
	}
	return nil, fmt.Errorf("sublocation not found: %d", id)
}

func getZIADeviceByID(
	ctx context.Context,
	service *zsdk.Service,
	id int,
) (*devicegroups.Devices, error) {
	devices, err := getZIAAllPages[devicegroups.Devices](ctx, service, "/zia/api/v1/deviceGroups/devices")
	if err != nil {
		return nil, err
	}
	for index := range devices {
		if devices[index].ID == id {
			return &devices[index], nil
		}
	}
	return nil, fmt.Errorf("no device found with ID: %d", id)
}

// getZIAUsersAllPages reads /zia/api/v1/users with a bounded page walk. It
// replaces ziausers.GetAllUsers, whose vendored ReadAllPages loop has no page
// ceiling; the endpoint, service-level sort options, 10000-record page size,
// short-page termination, and post-aggregation JMESPath filtering are
// preserved.
func getZIAUsersAllPages(ctx context.Context, service *zsdk.Service) ([]ziausers.Users, error) {
	const pageSize = 10000
	return getZIAAllPagesWithSize[ziausers.Users](
		ctx,
		service,
		ziaSortedListEndpoint(service, "/zia/api/v1/users"),
		pageSize,
	)
}

// getZIALocationsAllPages reads /zia/api/v1/locations with a bounded page walk,
// replacing locationmanagement.GetAll's unbounded loop. Endpoint, 1000-record
// page size, and short-page termination match the SDK.
func getZIALocationsAllPages(ctx context.Context, service *zsdk.Service) ([]locationmanagement.Locations, error) {
	const pageSize = 1000
	return ziaPaginate(ctx, pageSize, func(ctx context.Context, page, size int) ([]locationmanagement.Locations, error) {
		var items []locationmanagement.Locations
		err := ziacommon.ReadPage(ctx, service.Client, "/zia/api/v1/locations", page, &items, size)
		return items, err
	})
}

// getZIALocationGroupsAllPages reads /zia/api/v1/locations/groups with the
// fetchLocations option enabled so list and dump records include associated
// parent locations and sublocations. It preserves the SDK's documented
// 1000-record page size while replacing its unbounded ReadAllPages loop with
// ziaPaginate's fail-closed page ceiling.
func getZIALocationGroupsAllPages(ctx context.Context, service *zsdk.Service) ([]locationgroups.LocationGroup, error) {
	const pageSize = 1000
	return getZIAAllPagesWithSize[locationgroups.LocationGroup](
		ctx,
		service,
		"/zia/api/v1/locations/groups?fetchLocations=true",
		pageSize,
	)
}

// getZIAURLFilteringRulesAllPages reads /zia/api/v1/urlFilteringRules with an
// explicit bounded page walk. The vendored SDK's GetAll performs one bare Read,
// so tenants with more than the API's 100-record default page are silently
// truncated. Keeping the explicit page size at 100 matches Zscaler's published
// pagination example and the observed default boundary. It also avoids relying
// on a larger, endpoint-specific maximum that the API reference does not state.
func getZIAURLFilteringRulesAllPages(ctx context.Context, service *zsdk.Service) ([]urlfilteringpolicies.URLFilteringRule, error) {
	const pageSize = 100
	return ziaPaginate(ctx, pageSize, func(ctx context.Context, page, size int) ([]urlfilteringpolicies.URLFilteringRule, error) {
		var items []urlfilteringpolicies.URLFilteringRule
		err := ziacommon.ReadPage(ctx, service.Client, "/zia/api/v1/urlFilteringRules", page, &items, size)
		return items, err
	})
}

// getZIAURLCategoriesAll reads /zia/api/v1/urlCategories. This endpoint does not
// paginate (the SDK's GetAll issues a single Read), so it follows the
// networkApplications pattern instead of ziaPaginate: read one large bounded
// page and fail closed if it fills the ceiling, since a full single page is
// indistinguishable from a truncated one. includeOnlyUrlKeywordCounts=true
// preserves the prior GetAll(customOnly=false, includeOnlyUrlKeywordCounts=true)
// payload shape. type=ALL is required to include TLD_CATEGORY records; omitting
// the type returns only predefined and custom URL_CATEGORY records.
func getZIAURLCategoriesAll(ctx context.Context, service *zsdk.Service) ([]urlcategories.URLCategory, error) {
	const pageCeiling = 5000
	var categories []urlcategories.URLCategory
	err := ziacommon.ReadPage(ctx, service.Client, "/zia/api/v1/urlCategories?includeOnlyUrlKeywordCounts=true&type=ALL", 1, &categories, pageCeiling)
	if err != nil {
		return nil, err
	}
	if len(categories) >= pageCeiling {
		return nil, fmt.Errorf("zia url-categories returned the full single-page ceiling of %d records; this endpoint does not paginate, so completeness cannot be guaranteed", pageCeiling)
	}
	return categories, nil
}

func addZIAHandlers(m map[resourceKey]resourceHandler, client sdkClient) {
	entries := map[resourceKey]resourceHandler{
		{product: resources.ProductZIA, name: resourceLocations}: newListGetHandler(
			resourceLocations,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]locationmanagement.Locations, error) {
				return getZIALocationsAllPages(ctx, service)
			}),
			ziaSDKGet(client, func(ctx context.Context, service *zsdk.Service, id int) (*locationmanagement.Locations, error) {
				return locationmanagement.GetLocation(ctx, service, id)
			}),
			locationSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceLocationGroups}: newListGetHandler(
			resourceLocationGroups,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]locationgroups.LocationGroup, error) {
				return getZIALocationGroupsAllPages(ctx, service)
			}),
			ziaSDKGet(client, func(ctx context.Context, service *zsdk.Service, id int) (*locationgroups.LocationGroup, error) {
				return locationgroups.GetLocationGroup(ctx, service, id)
			}),
			locationGroupSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceRuleLabels}: newListGetHandler(
			resourceRuleLabels,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]rulelabels.RuleLabels, error) {
				return getZIAAllPages[rulelabels.RuleLabels](ctx, service, "/zia/api/v1/ruleLabels")
			}),
			ziaSDKGet(client, func(ctx context.Context, service *zsdk.Service, id int) (*rulelabels.RuleLabels, error) {
				return rulelabels.Get(ctx, service, id)
			}),
			ruleLabelSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceAuthSettings}: newSingletonHandler(
			resourceAuthSettings,
			ziaSDKShow(client, func(ctx context.Context, service *zsdk.Service) (*authsettings.AuthenticationSettings, error) {
				return authsettings.Get(ctx, service)
			}),
			authSettingsSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceStaticIPs}: newListGetHandler(
			resourceStaticIPs,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]staticips.StaticIP, error) {
				return getZIAAllPages[staticips.StaticIP](ctx, service, "/zia/api/v1/staticIP")
			}),
			ziaSDKGet(client, func(ctx context.Context, service *zsdk.Service, id int) (*staticips.StaticIP, error) {
				return staticips.Get(ctx, service, id)
			}),
			staticIPSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceGRETunnels}: newListGetHandler(
			resourceGRETunnels,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]gretunnels.GreTunnels, error) {
				return getZIAAllPages[gretunnels.GreTunnels](ctx, service, "/zia/api/v1/greTunnels")
			}),
			ziaSDKGet(client, func(ctx context.Context, service *zsdk.Service, id int) (*gretunnels.GreTunnels, error) {
				return gretunnels.GetGreTunnels(ctx, service, id)
			}),
			greTunnelSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceSublocations}: newListGetHandler(
			resourceSublocations,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]locationmanagement.Locations, error) {
				return getZIASublocationsAllPages(ctx, service)
			}),
			ziaSDKGet(client, func(ctx context.Context, service *zsdk.Service, id int) (*locationmanagement.Locations, error) {
				return getZIASublocationByID(ctx, service, id)
			}),
			sublocationSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceSSLRules}: newListGetHandler(
			resourceSSLRules,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]sslinspection.SSLInspectionRules, error) {
				return sslinspection.GetAll(ctx, service)
			}),
			ziaSDKGet(client, func(ctx context.Context, service *zsdk.Service, id int) (*sslinspection.SSLInspectionRules, error) {
				return sslinspection.Get(ctx, service, id)
			}),
			sslInspectionRuleSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceURLCategories}: newListGetHandler(
			resourceURLCategories,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]urlcategories.URLCategory, error) {
				return getZIAURLCategoriesAll(ctx, service)
			}),
			ziaSDKStringGet(client, func(ctx context.Context, service *zsdk.Service, id string) (*urlcategories.URLCategory, error) {
				return urlcategories.Get(ctx, service, id)
			}),
			urlCategorySourceRecord,
		),
		{product: resources.ProductZIA, name: resourceURLRules}: newListGetHandler(
			resourceURLRules,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]urlfilteringpolicies.URLFilteringRule, error) {
				return getZIAURLFilteringRulesAllPages(ctx, service)
			}),
			ziaSDKGet(client, func(ctx context.Context, service *zsdk.Service, id int) (*urlfilteringpolicies.URLFilteringRule, error) {
				return urlfilteringpolicies.Get(ctx, service, id)
			}),
			urlFilteringRuleSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceFirewallRules}: newListGetHandler(
			resourceFirewallRules,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]filteringrules.FirewallFilteringRules, error) {
				return getZIAAllPagesWithSize[filteringrules.FirewallFilteringRules](
					ctx,
					service,
					"/zia/api/v1/firewallFilteringRules",
					5000,
				)
			}),
			ziaSDKGet(client, func(ctx context.Context, service *zsdk.Service, id int) (*filteringrules.FirewallFilteringRules, error) {
				return filteringrules.Get(ctx, service, id)
			}),
			firewallFilteringRuleSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceForwardingRules}: newListGetHandler(
			resourceForwardingRules,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]forwardingrules.ForwardingRules, error) {
				return getZIAAllPages[forwardingrules.ForwardingRules](ctx, service, "/zia/api/v1/forwardingRules")
			}),
			ziaSDKGet(client, func(ctx context.Context, service *zsdk.Service, id int) (*forwardingrules.ForwardingRules, error) {
				return forwardingrules.Get(ctx, service, id)
			}),
			forwardingRuleSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceIPSourceGroups}: newListGetHandler(
			resourceIPSourceGroups,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]ipsourcegroups.IPSourceGroups, error) {
				return ipsourcegroups.GetAll(ctx, service)
			}),
			ziaSDKGet(client, func(ctx context.Context, service *zsdk.Service, id int) (*ipsourcegroups.IPSourceGroups, error) {
				return ipsourcegroups.Get(ctx, service, id)
			}),
			ipSourceGroupSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceIPDestGroups}: newListGetHandler(
			resourceIPDestGroups,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]ipdestinationgroups.IPDestinationGroups, error) {
				return ipdestinationgroups.GetAll(ctx, service, "")
			}),
			ziaSDKGet(client, func(ctx context.Context, service *zsdk.Service, id int) (*ipdestinationgroups.IPDestinationGroups, error) {
				return ipdestinationgroups.Get(ctx, service, id)
			}),
			ipDestinationGroupSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceNetworkServices}: newListGetHandler(
			resourceNetworkServices,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]networkservices.NetworkServices, error) {
				return getZIAAllPages[networkservices.NetworkServices](ctx, service, "/zia/api/v1/networkServices")
			}),
			ziaSDKGet(client, func(ctx context.Context, service *zsdk.Service, id int) (*networkservices.NetworkServices, error) {
				return networkservices.Get(ctx, service, id)
			}),
			networkServiceSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceNetworkSvcGroups}: newListGetHandler(
			resourceNetworkSvcGroups,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]networkservicegroups.NetworkServiceGroups, error) {
				return getZIAAllPages[networkservicegroups.NetworkServiceGroups](ctx, service, "/zia/api/v1/networkServiceGroups")
			}),
			ziaSDKGet(client, func(ctx context.Context, service *zsdk.Service, id int) (*networkservicegroups.NetworkServiceGroups, error) {
				return networkservicegroups.GetNetworkServiceGroups(ctx, service, id)
			}),
			networkServiceGroupSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceNetworkApps}: newListGetHandler(
			resourceNetworkApps,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]networkapplications.NetworkApplications, error) {
				return getNetworkApplicationsPage(ctx, service)
			}),
			ziaSDKStringGet(client, func(ctx context.Context, service *zsdk.Service, id string) (*networkapplications.NetworkApplications, error) {
				return networkapplications.GetNetworkApplication(ctx, service, id, "")
			}),
			networkApplicationSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceAppServices}: newListGetHandler(
			resourceAppServices,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]applicationservices.ApplicationServicesLite, error) {
				return getZIAAllPages[applicationservices.ApplicationServicesLite](ctx, service, "/zia/api/v1/appServices/lite")
			}),
			ziaSDKListGetByIntID(
				client,
				func(ctx context.Context, service *zsdk.Service) ([]applicationservices.ApplicationServicesLite, error) {
					return getZIAAllPages[applicationservices.ApplicationServicesLite](ctx, service, "/zia/api/v1/appServices/lite")
				},
				func(item applicationservices.ApplicationServicesLite) int { return item.ID },
			),
			applicationServiceSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceAppServiceGroups}: newListGetHandler(
			resourceAppServiceGroups,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]appservicegroups.ApplicationServicesGroupLite, error) {
				return getZIAAllPages[appservicegroups.ApplicationServicesGroupLite](ctx, service, "/zia/api/v1/appServiceGroups/lite")
			}),
			ziaSDKListGetByIntID(
				client,
				func(ctx context.Context, service *zsdk.Service) ([]appservicegroups.ApplicationServicesGroupLite, error) {
					return getZIAAllPages[appservicegroups.ApplicationServicesGroupLite](ctx, service, "/zia/api/v1/appServiceGroups/lite")
				},
				func(item appservicegroups.ApplicationServicesGroupLite) int { return item.ID },
			),
			applicationServiceGroupSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceNetworkAppGroups}: newListGetHandler(
			resourceNetworkAppGroups,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]networkapplicationgroups.NetworkApplicationGroups, error) {
				return getZIAAllPages[networkapplicationgroups.NetworkApplicationGroups](ctx, service, "/zia/api/v1/networkApplicationGroups")
			}),
			ziaSDKGet(client, func(ctx context.Context, service *zsdk.Service, id int) (*networkapplicationgroups.NetworkApplicationGroups, error) {
				return networkapplicationgroups.GetNetworkApplicationGroups(ctx, service, id)
			}),
			networkApplicationGroupSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceTimeWindows}: newListGetHandler(
			resourceTimeWindows,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]timewindow.TimeWindow, error) {
				return timewindow.GetAll(ctx, service)
			}),
			ziaSDKListGetByIntID(
				client,
				func(ctx context.Context, service *zsdk.Service) ([]timewindow.TimeWindow, error) {
					return timewindow.GetAll(ctx, service)
				},
				func(item timewindow.TimeWindow) int { return item.ID },
			),
			timeWindowSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceProxies}: newListGetHandler(
			resourceProxies,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]proxies.Proxies, error) {
				return getZIAAllPages[proxies.Proxies](ctx, service, "/zia/api/v1/proxies")
			}),
			ziaSDKGet(client, func(ctx context.Context, service *zsdk.Service, id int) (*proxies.Proxies, error) {
				return proxies.Get(ctx, service, id)
			}),
			proxySourceRecord,
		),
		{product: resources.ProductZIA, name: resourceProxyGateways}: newListGetHandler(
			resourceProxyGateways,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]proxygateways.ProxyGateways, error) {
				return getZIAAllPages[proxygateways.ProxyGateways](ctx, service, "/zia/api/v1/proxyGateways")
			}),
			ziaSDKListGetByIntID(
				client,
				func(ctx context.Context, service *zsdk.Service) ([]proxygateways.ProxyGateways, error) {
					return getZIAAllPages[proxygateways.ProxyGateways](ctx, service, "/zia/api/v1/proxyGateways")
				},
				func(item proxygateways.ProxyGateways) int { return item.ID },
			),
			proxyGatewaySourceRecord,
		),
		{product: resources.ProductZIA, name: resourceDedicatedIPGWs}: newListGetHandler(
			resourceDedicatedIPGWs,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]proxies.DedicatedIPGateways, error) {
				return getZIAAllPages[proxies.DedicatedIPGateways](ctx, service, "/zia/api/v1/dedicatedIPGateways/lite")
			}),
			ziaSDKListGetByIntID(
				client,
				func(ctx context.Context, service *zsdk.Service) ([]proxies.DedicatedIPGateways, error) {
					return getZIAAllPages[proxies.DedicatedIPGateways](ctx, service, "/zia/api/v1/dedicatedIPGateways/lite")
				},
				func(item proxies.DedicatedIPGateways) int { return item.Id },
			),
			dedicatedIPGatewaySourceRecord,
		),
		{product: resources.ProductZIA, name: resourceTimeIntervals}: newListGetHandler(
			resourceTimeIntervals,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]timeintervals.TimeInterval, error) {
				return getZIAAllPages[timeintervals.TimeInterval](ctx, service, "/zia/api/v1/timeIntervals")
			}),
			ziaSDKGet(client, func(ctx context.Context, service *zsdk.Service, id int) (*timeintervals.TimeInterval, error) {
				return timeintervals.Get(ctx, service, id)
			}),
			timeIntervalSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceBandwidthClasses}: newListGetHandler(
			resourceBandwidthClasses,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]bandwidthclasses.BandwidthClasses, error) {
				return getZIAAllPages[bandwidthclasses.BandwidthClasses](ctx, service, "/zia/api/v1/bandwidthClasses")
			}),
			ziaSDKGet(client, func(ctx context.Context, service *zsdk.Service, id int) (*bandwidthclasses.BandwidthClasses, error) {
				return bandwidthclasses.Get(ctx, service, id)
			}),
			bandwidthClassSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceBandwidthRules}: newListGetHandler(
			resourceBandwidthRules,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]bandwidthcontrolrules.BandwidthControlRules, error) {
				return getZIAAllPages[bandwidthcontrolrules.BandwidthControlRules](ctx, service, "/zia/api/v1/bandwidthControlRules")
			}),
			ziaSDKGet(client, func(ctx context.Context, service *zsdk.Service, id int) (*bandwidthcontrolrules.BandwidthControlRules, error) {
				return bandwidthcontrolrules.Get(ctx, service, id)
			}),
			bandwidthControlRuleSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceIPSSignatureRules}: newListGetHandler(
			resourceIPSSignatureRules,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]ipssignaturerules.IPSSignatureRules, error) {
				return getZIAAllPages[ipssignaturerules.IPSSignatureRules](ctx, service, "/zia/api/v1/ipsSignatureRules")
			}),
			ziaSDKGet(client, func(ctx context.Context, service *zsdk.Service, id int) (*ipssignaturerules.IPSSignatureRules, error) {
				return ipssignaturerules.Get(ctx, service, id)
			}),
			ipsSignatureRuleSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceIPSPolicies}: newListGetHandler(
			resourceIPSPolicies,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]ipspolicies.FirewallIPSRules, error) {
				return getZIAAllPages[ipspolicies.FirewallIPSRules](ctx, service, "/zia/api/v1/firewallIpsRules")
			}),
			ziaSDKGet(client, func(ctx context.Context, service *zsdk.Service, id int) (*ipspolicies.FirewallIPSRules, error) {
				return ipspolicies.Get(ctx, service, id)
			}),
			ipsPolicySourceRecord,
		),
		{product: resources.ProductZIA, name: resourceDNSGateways}: newListGetHandler(
			resourceDNSGateways,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]dnsgateways.DNSGateways, error) {
				return getZIAAllPages[dnsgateways.DNSGateways](ctx, service, "/zia/api/v1/dnsGateways")
			}),
			ziaSDKGet(client, func(ctx context.Context, service *zsdk.Service, id int) (*dnsgateways.DNSGateways, error) {
				return dnsgateways.Get(ctx, service, id)
			}),
			dnsGatewaySourceRecord,
		),
		{product: resources.ProductZIA, name: resourceNATRules}: newListGetHandler(
			resourceNATRules,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]natcontrol.NatControlPolicies, error) {
				return getZIAAllPages[natcontrol.NatControlPolicies](ctx, service, "/zia/api/v1/dnatRules")
			}),
			ziaSDKGet(client, func(ctx context.Context, service *zsdk.Service, id int) (*natcontrol.NatControlPolicies, error) {
				return natcontrol.Get(ctx, service, id)
			}),
			natControlRuleSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceGroups}: newListGetHandler(
			resourceGroups,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]usergroups.Groups, error) {
				return getZIAAllPages[usergroups.Groups](
					ctx,
					service,
					ziaSortedListEndpoint(service, "/zia/api/v1/groups"),
				)
			}),
			ziaSDKGet(client, func(ctx context.Context, service *zsdk.Service, id int) (*usergroups.Groups, error) {
				return usergroups.GetGroups(ctx, service, id)
			}),
			groupSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceDepartments}: newListGetHandler(
			resourceDepartments,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]userdepartments.Department, error) {
				return getZIAAllPages[userdepartments.Department](
					ctx,
					service,
					ziaSortedListEndpoint(service, "/zia/api/v1/departments"),
				)
			}),
			ziaSDKGet(client, func(ctx context.Context, service *zsdk.Service, id int) (*userdepartments.Department, error) {
				return userdepartments.GetDepartments(ctx, service, id)
			}),
			departmentSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceUsers}: newListGetHandler(
			resourceUsers,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]ziausers.Users, error) {
				return getZIAUsersAllPages(ctx, service)
			}),
			ziaSDKGet(client, func(ctx context.Context, service *zsdk.Service, id int) (*ziausers.Users, error) {
				return ziausers.Get(ctx, service, id)
			}),
			userSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceDeviceGroups}: newListGetHandler(
			resourceDeviceGroups,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]devicegroups.DeviceGroups, error) {
				return getZIAAllPages[devicegroups.DeviceGroups](ctx, service, "/zia/api/v1/deviceGroups")
			}),
			ziaSDKListGetByIntID(
				client,
				func(ctx context.Context, service *zsdk.Service) ([]devicegroups.DeviceGroups, error) {
					return getZIAAllPages[devicegroups.DeviceGroups](ctx, service, "/zia/api/v1/deviceGroups")
				},
				func(item devicegroups.DeviceGroups) int { return item.ID },
			),
			deviceGroupSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceDevices}: newListGetHandler(
			resourceDevices,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]devicegroups.Devices, error) {
				return getZIAAllPages[devicegroups.Devices](ctx, service, "/zia/api/v1/deviceGroups/devices")
			}),
			ziaSDKGet(client, func(ctx context.Context, service *zsdk.Service, id int) (*devicegroups.Devices, error) {
				return getZIADeviceByID(ctx, service, id)
			}),
			deviceSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceWorkloadGroups}: newListGetHandler(
			resourceWorkloadGroups,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]workloadgroups.WorkloadGroup, error) {
				return getZIAAllPages[workloadgroups.WorkloadGroup](ctx, service, "/zia/api/v1/workloadGroups")
			}),
			ziaSDKGet(client, func(ctx context.Context, service *zsdk.Service, id int) (*workloadgroups.WorkloadGroup, error) {
				return workloadgroups.Get(ctx, service, id)
			}),
			workloadGroupSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceAlertSubs}: newListGetHandler(
			resourceAlertSubs,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]alerts.AlertSubscriptions, error) {
				return getZIAAllPages[alerts.AlertSubscriptions](ctx, service, "/zia/api/v1/alertSubscriptions")
			}),
			ziaSDKGet(client, func(ctx context.Context, service *zsdk.Service, id int) (*alerts.AlertSubscriptions, error) {
				return alerts.Get(ctx, service, id)
			}),
			alertSubscriptionSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceActivationStatus}: newSingletonHandler(
			resourceActivationStatus,
			ziaSDKShow(client, activation.GetActivationStatus),
			structSourceRecord[activation.Activation],
		),
		{product: resources.ProductZIA, name: resourceEUSAStatus}: newSingletonHandler(
			resourceEUSAStatus,
			ziaSDKShow(client, activation.GetEusaStatus),
			eusaStatusSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceAuthExemptedURLs}: newSingletonHandler(
			resourceAuthExemptedURLs,
			ziaSDKShow(client, userauthsettings.Get),
			structSourceRecord[userauthsettings.ExemptedUrls],
		),
		{product: resources.ProductZIA, name: resourceIntermediateCAs}: newListGetHandler(
			resourceIntermediateCAs,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]intermediatecacertificates.IntermediateCACertificate, error) {
				return getZIAAllPages[intermediatecacertificates.IntermediateCACertificate](ctx, service, "/zia/api/v1/intermediateCaCertificate")
			}),
			ziaSDKGet(client, func(ctx context.Context, service *zsdk.Service, id int) (*intermediatecacertificates.IntermediateCACertificate, error) {
				return intermediatecacertificates.GetCertificate(ctx, service, id)
			}),
			intermediateCACertificateSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceCloudAppInsts}: newListGetHandler(
			resourceCloudAppInsts,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]cloudappinstances.CloudApplicationInstances, error) {
				return getZIAAllPages[cloudappinstances.CloudApplicationInstances](ctx, service, "/zia/api/v1/cloudApplicationInstances")
			}),
			ziaSDKGet(client, func(ctx context.Context, service *zsdk.Service, id int) (*cloudappinstances.CloudApplicationInstances, error) {
				return cloudappinstances.Get(ctx, service, id)
			}),
			cloudAppInstanceSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceTenancyProfiles}: newListGetHandler(
			resourceTenancyProfiles,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]tenancyrestriction.TenancyRestrictionProfile, error) {
				return getZIAAllPages[tenancyrestriction.TenancyRestrictionProfile](ctx, service, "/zia/api/v1/tenancyRestrictionProfile")
			}),
			ziaSDKGet(client, func(ctx context.Context, service *zsdk.Service, id int) (*tenancyrestriction.TenancyRestrictionProfile, error) {
				return tenancyrestriction.Get(ctx, service, id)
			}),
			tenancyRestrictionProfileSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceVZENClusters}: newListGetHandler(
			resourceVZENClusters,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]vzenclusters.VZENClusters, error) {
				return getZIAAllPages[vzenclusters.VZENClusters](ctx, service, "/zia/api/v1/virtualZenClusters")
			}),
			ziaSDKGet(client, func(ctx context.Context, service *zsdk.Service, id int) (*vzenclusters.VZENClusters, error) {
				return vzenclusters.Get(ctx, service, id)
			}),
			vzenClusterSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceVZENNodes}: newListGetHandler(
			resourceVZENNodes,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]vzennodes.VZENNodes, error) {
				return getZIAAllPages[vzennodes.VZENNodes](ctx, service, "/zia/api/v1/virtualZenNodes")
			}),
			ziaSDKGet(client, func(ctx context.Context, service *zsdk.Service, id int) (*vzennodes.VZENNodes, error) {
				return vzennodes.Get(ctx, service, id)
			}),
			vzenNodeSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceBrowserIsolation}: newListOnlyHandler(
			resourceBrowserIsolation,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]browserisolation.CBIProfile, error) {
				return getZIAAllPages[browserisolation.CBIProfile](ctx, service, "/zia/api/v1/browserIsolation/profiles")
			}),
			browserIsolationProfileSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceDLPEngines}: newListGetHandler(
			resourceDLPEngines,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]dlpengines.DLPEngines, error) {
				return getZIAAllPages[dlpengines.DLPEngines](ctx, service, "/zia/api/v1/dlpEngines")
			}),
			ziaSDKGet(client, func(ctx context.Context, service *zsdk.Service, id int) (*dlpengines.DLPEngines, error) {
				return dlpengines.Get(ctx, service, id)
			}),
			dlpEngineSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceDLPDictionaries}: newListGetHandler(
			resourceDLPDictionaries,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]dlpdictionaries.DlpDictionary, error) {
				return getZIAAllPages[dlpdictionaries.DlpDictionary](ctx, service, "/zia/api/v1/dlpDictionaries")
			}),
			ziaSDKGet(client, func(ctx context.Context, service *zsdk.Service, id int) (*dlpdictionaries.DlpDictionary, error) {
				return dlpdictionaries.Get(ctx, service, id)
			}),
			dlpDictionarySourceRecord,
		),
		{product: resources.ProductZIA, name: resourceDLPEDMSchemas}: newListGetHandler(
			resourceDLPEDMSchemas,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]dlpexactdatamatch.DLPEDMSchema, error) {
				return getZIAAllPages[dlpexactdatamatch.DLPEDMSchema](ctx, service, "/zia/api/v1/dlpExactDataMatchSchemas")
			}),
			ziaSDKGet(client, func(ctx context.Context, service *zsdk.Service, id int) (*dlpexactdatamatch.DLPEDMSchema, error) {
				return dlpexactdatamatch.GetDLPEDMSchemaID(ctx, service, id)
			}),
			dlpEDMSchemaSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceDLPEDMLite}: newListOnlyHandler(
			resourceDLPEDMLite,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]dlpedmlite.DLPEDMLite, error) {
				return getZIAAllPages[dlpedmlite.DLPEDMLite](ctx, service, "/zia/api/v1/dlpExactDataMatchSchemas/lite")
			}),
			dlpEDMLiteSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceDLPIDMLite}: newListGetHandler(
			resourceDLPIDMLite,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]dlpidmprofilelite.DLPIDMProfileLite, error) {
				return getZIAAllPages[dlpidmprofilelite.DLPIDMProfileLite](ctx, service, "/zia/api/v1/idmprofile/lite")
			}),
			ziaSDKGet(client, func(ctx context.Context, service *zsdk.Service, id int) (*dlpidmprofilelite.DLPIDMProfileLite, error) {
				return dlpidmprofilelite.GetDLPProfileLiteID(ctx, service, id, false)
			}),
			dlpIDMProfileLiteSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceDLPIDMProfiles}: newListGetHandler(
			resourceDLPIDMProfiles,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]dlpidmprofiles.DLPIDMProfile, error) {
				return getZIAAllPages[dlpidmprofiles.DLPIDMProfile](ctx, service, "/zia/api/v1/idmprofile")
			}),
			ziaSDKGet(client, func(ctx context.Context, service *zsdk.Service, id int) (*dlpidmprofiles.DLPIDMProfile, error) {
				return dlpidmprofiles.Get(ctx, service, id)
			}),
			dlpIDMProfileSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceDLPWebRules}: newListGetHandler(
			resourceDLPWebRules,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]dlpwebrules.WebDLPRules, error) {
				return getZIAAllPages[dlpwebrules.WebDLPRules](ctx, service, "/zia/api/v1/webDlpRules")
			}),
			ziaSDKGet(client, func(ctx context.Context, service *zsdk.Service, id int) (*dlpwebrules.WebDLPRules, error) {
				return dlpwebrules.Get(ctx, service, id)
			}),
			dlpWebRuleSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceDLPICAPServers}: newListGetHandler(
			resourceDLPICAPServers,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]dlpicapservers.DLPICAPServers, error) {
				return getZIAAllPages[dlpicapservers.DLPICAPServers](ctx, service, "/zia/api/v1/icapServers")
			}),
			ziaSDKGet(client, func(ctx context.Context, service *zsdk.Service, id int) (*dlpicapservers.DLPICAPServers, error) {
				return dlpicapservers.Get(ctx, service, id)
			}),
			dlpICAPServerSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceDLPIncidentRcvs}: newListGetHandler(
			resourceDLPIncidentRcvs,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]dlpincidentreceivers.IncidentReceiverServers, error) {
				return getZIAAllPages[dlpincidentreceivers.IncidentReceiverServers](ctx, service, "/zia/api/v1/incidentReceiverServers")
			}),
			ziaSDKGet(client, func(ctx context.Context, service *zsdk.Service, id int) (*dlpincidentreceivers.IncidentReceiverServers, error) {
				return dlpincidentreceivers.Get(ctx, service, id)
			}),
			dlpIncidentReceiverSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceDLPNotifyTmpls}: newListGetHandler(
			resourceDLPNotifyTmpls,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]dlpnotificationtemplates.DlpNotificationTemplates, error) {
				return getZIAAllPages[dlpnotificationtemplates.DlpNotificationTemplates](ctx, service, "/zia/api/v1/dlpNotificationTemplates")
			}),
			ziaSDKGet(client, func(ctx context.Context, service *zsdk.Service, id int) (*dlpnotificationtemplates.DlpNotificationTemplates, error) {
				return dlpnotificationtemplates.Get(ctx, service, id)
			}),
			dlpNotificationTemplateSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceC2CIncidentRcvs}: newListGetHandler(
			resourceC2CIncidentRcvs,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]c2cincidentreceiver.C2CIncidentReceiver, error) {
				return getZIAAllPages[c2cincidentreceiver.C2CIncidentReceiver](ctx, service, "/zia/api/v1/cloudToCloudIR")
			}),
			ziaSDKGet(client, func(ctx context.Context, service *zsdk.Service, id int) (*c2cincidentreceiver.C2CIncidentReceiver, error) {
				return c2cincidentreceiver.Get(ctx, service, id)
			}),
			c2cIncidentReceiverSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceRiskProfiles}: newListGetHandler(
			resourceRiskProfiles,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]riskprofiles.RiskProfiles, error) {
				return getZIAAllPages[riskprofiles.RiskProfiles](ctx, service, "/zia/api/v1/riskProfiles")
			}),
			ziaSDKGet(client, func(ctx context.Context, service *zsdk.Service, id int) (*riskprofiles.RiskProfiles, error) {
				return riskprofiles.Get(ctx, service, id)
			}),
			riskProfileSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceNSSServers}: newListGetHandler(
			resourceNSSServers,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]nssservers.NSSServers, error) {
				return nssservers.GetAll(ctx, service, nil)
			}),
			ziaSDKGet(client, func(ctx context.Context, service *zsdk.Service, id int) (*nssservers.NSSServers, error) {
				return nssservers.Get(ctx, service, id)
			}),
			nssServerSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceNSSFeeds}: newListGetHandler(
			resourceNSSFeeds,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]cloudnss.NSSFeed, error) {
				return getZIAAllPages[cloudnss.NSSFeed](ctx, service, "/zia/api/v1/nssFeeds")
			}),
			ziaSDKGet(client, func(ctx context.Context, service *zsdk.Service, id int) (*cloudnss.NSSFeed, error) {
				return cloudnss.Get(ctx, service, id)
			}),
			nssFeedSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceFileTypeRules}: newListGetHandler(
			resourceFileTypeRules,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]filetypecontrol.FileTypeRules, error) {
				return getZIAAllPages[filetypecontrol.FileTypeRules](ctx, service, "/zia/api/v1/fileTypeRules")
			}),
			ziaSDKGet(client, func(ctx context.Context, service *zsdk.Service, id int) (*filetypecontrol.FileTypeRules, error) {
				return filetypecontrol.Get(ctx, service, id)
			}),
			fileTypeRuleSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceSandboxRules}: newListGetHandler(
			resourceSandboxRules,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]sandboxrules.SandboxRules, error) {
				return sandboxrules.GetAll(ctx, service)
			}),
			ziaSDKGet(client, func(ctx context.Context, service *zsdk.Service, id int) (*sandboxrules.SandboxRules, error) {
				return sandboxrules.Get(ctx, service, id)
			}),
			sandboxRuleSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceFirewallDNSRules}: newListGetHandler(
			resourceFirewallDNSRules,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]firewalldnscontrolpolicies.FirewallDNSRules, error) {
				return getZIAAllPages[firewalldnscontrolpolicies.FirewallDNSRules](ctx, service, "/zia/api/v1/firewallDnsRules")
			}),
			ziaSDKGet(client, func(ctx context.Context, service *zsdk.Service, id int) (*firewalldnscontrolpolicies.FirewallDNSRules, error) {
				return firewalldnscontrolpolicies.Get(ctx, service, id)
			}),
			firewallDNSRuleSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceCustomFileTypes}: newListGetHandler(
			resourceCustomFileTypes,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]customfiletypes.CustomFileTypes, error) {
				return getZIAAllPages[customfiletypes.CustomFileTypes](ctx, service, "/zia/api/v1/customFileTypes")
			}),
			ziaSDKGet(client, func(ctx context.Context, service *zsdk.Service, id int) (*customfiletypes.CustomFileTypes, error) {
				return customfiletypes.Get(ctx, service, id)
			}),
			customFileTypeSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceZPAGateways}: newListGetHandler(
			resourceZPAGateways,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]zpagateways.ZPAGateways, error) {
				return getZIAAllPages[zpagateways.ZPAGateways](ctx, service, "/zia/api/v1/zpaGateways")
			}),
			ziaSDKGet(client, func(ctx context.Context, service *zsdk.Service, id int) (*zpagateways.ZPAGateways, error) {
				return zpagateways.Get(ctx, service, id)
			}),
			zpaGatewaySourceRecord,
		),
		{product: resources.ProductZIA, name: resourceDCExclusions}: newListOnlyHandler(
			resourceDCExclusions,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]dcexclusions.DCExclusions, error) {
				return dcexclusions.GetAll(ctx, service)
			}),
			dcExclusionSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceSubClouds}: newListOnlyHandler(
			resourceSubClouds,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]subclouds.SubClouds, error) {
				return getZIAAllPages[subclouds.SubClouds](ctx, service, "/zia/api/v1/subclouds")
			}),
			subCloudSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceIPv6Config}: newSingletonHandler(
			resourceIPv6Config,
			ziaSDKShow(client, ipv6config.GetIPv6Config),
			ipv6ConfigSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceIPv6DNS64Prefix}: newListOnlyHandler(
			resourceIPv6DNS64Prefix,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]ipv6config.IPv6ConfigPrefix, error) {
				return getZIAAllPages[ipv6config.IPv6ConfigPrefix](ctx, service, "/zia/api/v1/ipv6config/dns64prefix")
			}),
			ipv6ConfigPrefixSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceIPv6NAT64Prefix}: newListOnlyHandler(
			resourceIPv6NAT64Prefix,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]ipv6config.IPv6ConfigPrefix, error) {
				return getZIAAllPages[ipv6config.IPv6ConfigPrefix](ctx, service, "/zia/api/v1/ipv6config/nat64prefix")
			}),
			ipv6ConfigPrefixSourceRecord,
		),
		{product: resources.ProductZIA, name: resourcePACFiles}: newListOnlyHandler(
			resourcePACFiles,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]pacfiles.PACFileConfig, error) {
				return getZIAAllPages[pacfiles.PACFileConfig](ctx, service, "/zia/api/v1/pacFiles")
			}),
			pacFileSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceCloudAppControl}: newListOnlyHandler(
			resourceCloudAppControl,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]cloudappcontrol.WebApplicationRules, error) {
				// Cloud App Control has no flat list endpoint; enumerate the rule
				// types and concatenate each type's rules (sorted for determinism).
				mapping, err := cloudappcontrol.GetRuleTypeMapping(ctx, service)
				if err != nil {
					return nil, err
				}
				// The mapping's direction (rule-type code vs display name) is not
				// guaranteed, so probe both keys and values; invalid entries are
				// skipped by the tolerant GetByRuleType loop below.
				seen := map[string]struct{}{}
				ruleTypes := make([]string, 0, len(mapping)*2)
				for k, v := range mapping {
					for _, candidate := range []string{k, v} {
						if candidate == "" {
							continue
						}
						if _, ok := seen[candidate]; ok {
							continue
						}
						seen[candidate] = struct{}{}
						ruleTypes = append(ruleTypes, candidate)
					}
				}
				if len(ruleTypes) == 0 {
					ruleTypes = cloudAppControlRuleTypes
				}
				sort.Strings(ruleTypes)
				var all []cloudappcontrol.WebApplicationRules
				for _, ruleType := range ruleTypes {
					rules, err := cloudappcontrol.GetByRuleType(ctx, service, ruleType)
					if err != nil {
						// A rule type can be unavailable or not entitled in a given
						// tenant; skip it rather than failing the whole list.
						continue
					}
					all = append(all, rules...)
				}
				return all, nil
			}),
			cloudAppControlSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceCloudAppPolicy}: newListOnlyHandler(
			resourceCloudAppPolicy,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]cloudapplications.CloudApplications, error) {
				return getZIAAllPages[cloudapplications.CloudApplications](ctx, service, "/zia/api/v1/cloudApplications/policy")
			}),
			cloudApplicationPolicySourceRecord,
		),
		{product: resources.ProductZIA, name: resourceCloudAppSSLPol}: newListOnlyHandler(
			resourceCloudAppSSLPol,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]cloudapplications.CloudApplications, error) {
				return getZIAAllPages[cloudapplications.CloudApplications](ctx, service, "/zia/api/v1/cloudApplications/sslPolicy")
			}),
			cloudApplicationPolicySourceRecord,
		),
		{product: resources.ProductZIA, name: resourceDomainProfiles}: newListOnlyHandler(
			resourceDomainProfiles,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]saassecurityapi.DomainProfiles, error) {
				return getZIAAllPages[saassecurityapi.DomainProfiles](ctx, service, "/zia/api/v1/domainProfiles")
			}),
			domainProfileSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceCASBTombstones}: newListOnlyHandler(
			resourceCASBTombstones,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]saassecurityapi.QuarantineTombstoneLite, error) {
				return getZIAAllPages[saassecurityapi.QuarantineTombstoneLite](ctx, service, "/zia/api/v1/quarantineTombstoneTemplate/lite")
			}),
			casbTombstoneTemplateSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceCASBEmailLabels}: newListOnlyHandler(
			resourceCASBEmailLabels,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]saassecurityapi.CasbEmailLabel, error) {
				return getZIAAllPages[saassecurityapi.CasbEmailLabel](ctx, service, "/zia/api/v1/casbEmailLabel/lite")
			}),
			casbEmailLabelSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceCASBTenants}: newListOnlyHandler(
			resourceCASBTenants,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]saassecurityapi.CasbTenants, error) {
				return getZIAAllPages[saassecurityapi.CasbTenants](ctx, service, "/zia/api/v1/casbTenant/lite")
			}),
			casbTenantSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceCASBDLPRules}: newListOnlyHandler(
			resourceCASBDLPRules,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]casbdlprules.CasbDLPRules, error) {
				return getZIAAllPages[casbdlprules.CasbDLPRules](ctx, service, "/zia/api/v1/casbDlpRules/all")
			}),
			casbDLPRuleSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceCASBMalwareRules}: newListOnlyHandler(
			resourceCASBMalwareRules,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]casbmalwarerules.CasbMalwareRules, error) {
				return getZIAAllPages[casbmalwarerules.CasbMalwareRules](ctx, service, "/zia/api/v1/casbMalwareRules/all")
			}),
			casbMalwareRuleSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceBrowserControl}: newSingletonHandler(
			resourceBrowserControl,
			ziaSDKShow(client, browsercontrolsettings.GetBrowserControlSettings),
			browserControlSettingsSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceSupportedBrowsers}: newListOnlyHandler(
			resourceSupportedBrowsers,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]securebrowsing.SupportedBrowserVersion, error) {
				return securebrowsing.GetSupportedBrowserVersions(ctx, service)
			}),
			structSourceRecord[securebrowsing.SupportedBrowserVersion],
		),
		{product: resources.ProductZIA, name: resourceFTPControl}: newSingletonHandler(
			resourceFTPControl,
			ziaSDKShow(client, ftpcontrolpolicy.GetFTPControlPolicy),
			structSourceRecord[ftpcontrolpolicy.FTPControlPolicy],
		),
		{product: resources.ProductZIA, name: resourceRemoteAssistance}: newSingletonHandler(
			resourceRemoteAssistance,
			ziaSDKShow(client, remoteassistance.GetRemoteAssistance),
			structSourceRecord[remoteassistance.RemoteAssistance],
		),
		{product: resources.ProductZIA, name: resourceAdminUsers}: newListGetHandler(
			resourceAdminUsers,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]ziaadminusers.AdminUsers, error) {
				return getZIAAllPages[ziaadminusers.AdminUsers](
					ctx,
					service,
					"/zia/api/v1/adminUsers?includeAuditorUsers=true&includeAdminUsers=true",
				)
			}),
			ziaSDKGet(client, func(ctx context.Context, service *zsdk.Service, id int) (*ziaadminusers.AdminUsers, error) {
				return ziaadminusers.GetAdminUsers(ctx, service, id)
			}),
			ziaAdminUserSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceAdminRoles}: newListGetHandler(
			resourceAdminRoles,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]ziaadminroles.AdminRoles, error) {
				return ziaadminroles.GetAllAdminRoles(ctx, service)
			}),
			ziaSDKGet(client, func(ctx context.Context, service *zsdk.Service, id int) (*ziaadminroles.AdminRoles, error) {
				return ziaadminroles.Get(ctx, service, id)
			}),
			ziaAdminRoleSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceEmailProfiles}: newListGetHandler(
			resourceEmailProfiles,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]emailprofiles.EmailProfiles, error) {
				return getZIAAllPages[emailprofiles.EmailProfiles](ctx, service, "/zia/api/v1/emailRecipientProfile")
			}),
			ziaSDKGet(client, func(ctx context.Context, service *zsdk.Service, id int) (*emailprofiles.EmailProfiles, error) {
				return emailprofiles.Get(ctx, service, id)
			}),
			emailProfileSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceAdvancedSettings}: newSingletonHandler(
			resourceAdvancedSettings,
			ziaSDKShow(client, advancedsettings.GetAdvancedSettings),
			structSourceRecord[advancedsettings.AdvancedSettings],
		),
		{product: resources.ProductZIA, name: resourceAdvancedThreatSettings}: newSingletonHandler(
			resourceAdvancedThreatSettings,
			ziaSDKShow(client, advancedthreatsettings.GetAdvancedThreatSettings),
			structSourceRecord[advancedthreatsettings.AdvancedThreatSettings],
		),
		{product: resources.ProductZIA, name: resourceMobileThreatSettings}: newSingletonHandler(
			resourceMobileThreatSettings,
			ziaSDKShow(client, mobilethreatsettings.GetMobileThreatSettings),
			structSourceRecord[mobilethreatsettings.MobileAdvanceThreatSettings],
		),
		{product: resources.ProductZIA, name: resourceSandboxSettings}: newSingletonHandler(
			resourceSandboxSettings,
			ziaSDKShow(client, sandboxsettings.Get),
			structSourceRecord[sandboxsettings.BaAdvancedSettings],
		),
		{product: resources.ProductZIA, name: resourceEndUserNotification}: newSingletonHandler(
			resourceEndUserNotification,
			ziaSDKShow(client, endusernotification.GetUserNotificationSettings),
			structSourceRecord[endusernotification.UserNotificationSettings],
		),
		{product: resources.ProductZIA, name: resourceOrgInformation}: newSingletonHandler(
			resourceOrgInformation,
			ziaSDKShow(client, organizationdetails.GetOrgInformation),
			structSourceRecord[organizationdetails.Organization],
		),
		{product: resources.ProductZIA, name: resourceATPMalwarePolicy}: newSingletonHandler(
			resourceATPMalwarePolicy,
			ziaSDKShow(client, malwareprotection.GetATPMalwarePolicy),
			structSourceRecord[malwareprotection.MalwarePolicy],
		),
		{product: resources.ProductZIA, name: resourceATPMalwareSettings}: newSingletonHandler(
			resourceATPMalwareSettings,
			ziaSDKShow(client, malwareprotection.GetATPMalwareSettings),
			structSourceRecord[malwareprotection.MalwareSettings],
		),
		{product: resources.ProductZIA, name: resourceATPMalwareInspection}: newSingletonHandler(
			resourceATPMalwareInspection,
			ziaSDKShow(client, malwareprotection.GetATPMalwareInspection),
			structSourceRecord[malwareprotection.ATPMalwareInspection],
		),
		{product: resources.ProductZIA, name: resourceATPMalwareProtocols}: newSingletonHandler(
			resourceATPMalwareProtocols,
			ziaSDKShow(client, malwareprotection.GetATPMalwareProtocols),
			structSourceRecord[malwareprotection.ATPMalwareProtocols],
		),
		{product: resources.ProductZIA, name: resourceMaliciousURLs}: newSingletonHandler(
			resourceMaliciousURLs,
			ziaSDKShow(client, advancedthreatsettings.GetMaliciousURLs),
			structSourceRecord[advancedthreatsettings.MaliciousURLs],
		),
		{product: resources.ProductZIA, name: resourceSecurityExceptions}: newSingletonHandler(
			resourceSecurityExceptions,
			ziaSDKShow(client, advancedthreatsettings.GetSecurityExceptions),
			structSourceRecord[advancedthreatsettings.SecurityExceptions],
		),
		{product: resources.ProductZIA, name: resourceSecurityPolicyURLAllowlist}: newSingletonHandler(
			resourceSecurityPolicyURLAllowlist,
			ziaSDKShow(client, securitypolicysettings.GetWhiteListUrls),
			structSourceRecord[securitypolicysettings.ListUrls],
		),
		{product: resources.ProductZIA, name: resourceSecurityPolicyURLDenylist}: newSingletonHandler(
			resourceSecurityPolicyURLDenylist,
			ziaSDKShow(client, securitypolicysettings.GetBlackListUrls),
			structSourceRecord[securitypolicysettings.ListUrls],
		),
	}
	for k, v := range entries {
		addHandler(m, k, v)
	}
}

func ipsSignatureRuleSourceRecord(rule ipssignaturerules.IPSSignatureRules) resources.SourceRecord {
	fields := map[string]any{
		"id":                         rule.ID,
		"name":                       rule.Name,
		"description":                rule.Description,
		"enabled":                    rule.Enabled,
		"deleted":                    rule.Deleted,
		"promoteTime":                rule.PromoteTime,
		"ruleTextModTime":            rule.RuleTextModTime,
		"dynamicValidationSubmitted": rule.DynamicValidationSubmitted,
		"dynamicValidationRejected":  rule.DynamicValidationRejected,
		"dynamicValidationSucceeded": rule.DynamicValidationSucceeded,
		"disabledFromZSCM":           rule.DisabledFromZSCM,
		"dynamicValRejectCode":       rule.DynamicValRejectCode,
	}
	// ruleText is deliberately never mapped: the Suricata/Snort signature body
	// is catalog-classified as a secret and must not reach the source record.
	if rule.Category != nil {
		fields["category"] = map[string]any{
			"id":            rule.Category.ID,
			"name":          rule.Category.Name,
			"isNameL10nTag": rule.Category.IsNameL10nTag,
		}
	}
	return resources.NewSourceRecord(fields)
}

// cloudAppControlRuleTypes is a fallback list of Cloud App Control rule-type
// codes used only when the live ruleTypeMapping endpoint returns nothing.
var cloudAppControlRuleTypes = []string{
	"AI_ML",
	"BUSINESS_PRODUCTIVITY",
	"CONSUMER",
	"CUSTOM_CAPP",
	"DNS_OVER_HTTPS",
	"ENTERPRISE_COLLABORATION",
	"FILE_SHARE",
	"FINANCE",
	"HEALTH_CARE",
	"HOSTING_PROVIDER",
	"HUMAN_RESOURCES",
	"INSTANT_MESSAGING",
	"IT_SERVICES",
	"LEGAL",
	"SALES_AND_MARKETING",
	"SOCIAL_NETWORKING",
	"STREAMING_MEDIA",
	"SYSTEM_AND_DEVELOPMENT",
	"WEBMAIL",
	"WEB_CONFERENCING",
}

func cloudAppControlSourceRecord(rule cloudappcontrol.WebApplicationRules) resources.SourceRecord {
	fields := map[string]any{
		"id":                   rule.ID,
		"name":                 rule.Name,
		"description":          rule.Description,
		"state":                rule.State,
		"rank":                 rule.Rank,
		"type":                 rule.Type,
		"order":                rule.Order,
		"timeQuota":            rule.TimeQuota,
		"sizeQuota":            rule.SizeQuota,
		"cascadingEnabled":     rule.CascadingEnabled,
		"accessControl":        rule.AccessControl,
		"numberOfApplications": rule.NumberOfApplications,
		"eunEnabled":           rule.EunEnabled,
		"eunTemplateId":        rule.EunTemplateID,
		"browserEunTemplateId": rule.BrowserEunTemplateID,
		"predefined":           rule.Predefined,
		"validityStartTime":    rule.ValidityStartTime,
		"validityEndTime":      rule.ValidityEndTime,
		"validityTimeZoneId":   rule.ValidityTimeZoneID,
		"lastModifiedTime":     rule.LastModifiedTime,
		"enforceTimeValidity":  rule.EnforceTimeValidity,
	}
	addStringSlice(fields, "actions", rule.Actions)
	addStringSlice(fields, "applications", rule.Applications)
	addStringSlice(fields, "userAgentTypes", rule.UserAgentTypes)
	addStringSlice(fields, "deviceTrustLevels", rule.DeviceTrustLevels)
	addStringSlice(fields, "userRiskScoreLevels", rule.UserRiskScoreLevels)
	addIDNameExtensionsSlice(fields, "labels", rule.Labels)
	addIDNameExtensionsSlice(fields, "timeWindows", rule.TimeWindows)
	addIDNameExtensionsSlice(fields, "locations", rule.Locations)
	addIDNameExtensionsSlice(fields, "locationGroups", rule.LocationGroups)
	addIDNameExtensionsSlice(fields, "tenancyProfileIds", rule.TenancyProfileIDs)
	addIDNameExtensionsSlice(fields, "departments", rule.Departments)
	addIDNameExtensionsSlice(fields, "groups", rule.Groups)
	addIDNameExtensionsSlice(fields, "users", rule.Users)
	addIDNameExtensionsSlice(fields, "deviceGroups", rule.DeviceGroups)
	addIDNameExtensionsSlice(fields, "devices", rule.Devices)
	addIDCustomPtr(fields, "cloudAppRiskProfile", rule.CloudAppRiskProfile)
	if len(rule.CloudAppInstances) > 0 {
		fields["cloudAppInstances"] = cloudAppControlInstancesSource(rule.CloudAppInstances)
	}
	if rule.CBIProfile != (cloudappcontrol.CBIProfile{}) {
		fields["cbiProfile"] = cloudAppControlCBIProfileSource(rule.CBIProfile)
	}
	return resources.NewSourceRecord(fields)
}

// cloudAppControlCBIProfileSource maps the cloudappcontrol-local CBIProfile
// struct (a superset of ziacommon.CBIProfile with defaultProfile/sandboxMode).
// The url value still reaches the source record but the catalog classifies it
// as a secret, so projection drops it in every mode.
func cloudAppControlCBIProfileSource(value cloudappcontrol.CBIProfile) map[string]any {
	return map[string]any{
		"id":             value.ID,
		"name":           value.Name,
		"url":            value.URL,
		"profileSeq":     value.ProfileSeq,
		"defaultProfile": value.DefaultProfile,
		"sandboxMode":    value.SandboxMode,
	}
}

func cloudAppControlInstancesSource(values []cloudappcontrol.CloudAppInstances) []any {
	out := make([]any, 0, len(values))
	for _, value := range values {
		out = append(out, map[string]any{
			"id":   value.ID,
			"name": value.Name,
			"type": value.Type,
		})
	}
	return out
}
