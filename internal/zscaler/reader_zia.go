package zscaler

import (
	"context"

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
	cloudapplications "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/cloudapplications/cloudapplications"
	riskprofiles "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/cloudapplications/risk_profiles"
	cloudnss "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/cloudnss/cloudnss"
	nssservers "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/cloudnss/nss_servers"
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

func addZIAHandlers(m map[resourceKey]resourceHandler, client sdkClient) {
	entries := map[resourceKey]resourceHandler{
		{product: resources.ProductZIA, name: resourceLocations}: newListGetHandler(
			resourceLocations,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]locationmanagement.Locations, error) {
				return locationmanagement.GetAll(ctx, service)
			}),
			ziaSDKGet(client, func(ctx context.Context, service *zsdk.Service, id int) (*locationmanagement.Locations, error) {
				return locationmanagement.GetLocation(ctx, service, id)
			}),
			locationSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceLocationGroups}: newListGetHandler(
			resourceLocationGroups,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]locationgroups.LocationGroup, error) {
				fetchLocations := false
				return locationgroups.GetAll(ctx, service, &locationgroups.GetAllFilterOptions{
					FetchLocations: &fetchLocations,
				})
			}),
			ziaSDKGet(client, func(ctx context.Context, service *zsdk.Service, id int) (*locationgroups.LocationGroup, error) {
				return locationgroups.GetLocationGroup(ctx, service, id)
			}),
			locationGroupSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceRuleLabels}: newListGetHandler(
			resourceRuleLabels,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]rulelabels.RuleLabels, error) {
				return rulelabels.GetAll(ctx, service)
			}),
			ziaSDKGet(client, func(ctx context.Context, service *zsdk.Service, id int) (*rulelabels.RuleLabels, error) {
				return rulelabels.Get(ctx, service, id)
			}),
			ruleLabelSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceAuthSettings}: newSingletonHandler(
			resourceAuthSettings,
			ziaSDKSingleton(client, func(ctx context.Context, service *zsdk.Service) (*authsettings.AuthenticationSettings, error) {
				return authsettings.Get(ctx, service)
			}),
			authSettingsSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceStaticIPs}: newListGetHandler(
			resourceStaticIPs,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]staticips.StaticIP, error) {
				return staticips.GetAll(ctx, service)
			}),
			ziaSDKGet(client, func(ctx context.Context, service *zsdk.Service, id int) (*staticips.StaticIP, error) {
				return staticips.Get(ctx, service, id)
			}),
			staticIPSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceGRETunnels}: newListGetHandler(
			resourceGRETunnels,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]gretunnels.GreTunnels, error) {
				return gretunnels.GetAll(ctx, service)
			}),
			ziaSDKGet(client, func(ctx context.Context, service *zsdk.Service, id int) (*gretunnels.GreTunnels, error) {
				return gretunnels.GetGreTunnels(ctx, service, id)
			}),
			greTunnelSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceSublocations}: newListGetHandler(
			resourceSublocations,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]locationmanagement.Locations, error) {
				return locationmanagement.GetAllSublocations(ctx, service)
			}),
			ziaSDKGet(client, func(ctx context.Context, service *zsdk.Service, id int) (*locationmanagement.Locations, error) {
				return locationmanagement.GetSubLocationBySubID(ctx, service, id)
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
				return urlcategories.GetAll(ctx, service, false, true, "")
			}),
			ziaSDKStringGet(client, func(ctx context.Context, service *zsdk.Service, id string) (*urlcategories.URLCategory, error) {
				return urlcategories.Get(ctx, service, id)
			}),
			urlCategorySourceRecord,
		),
		{product: resources.ProductZIA, name: resourceURLRules}: newListGetHandler(
			resourceURLRules,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]urlfilteringpolicies.URLFilteringRule, error) {
				return urlfilteringpolicies.GetAll(ctx, service)
			}),
			ziaSDKGet(client, func(ctx context.Context, service *zsdk.Service, id int) (*urlfilteringpolicies.URLFilteringRule, error) {
				return urlfilteringpolicies.Get(ctx, service, id)
			}),
			urlFilteringRuleSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceFirewallRules}: newListGetHandler(
			resourceFirewallRules,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]filteringrules.FirewallFilteringRules, error) {
				return filteringrules.GetAll(ctx, service, nil)
			}),
			ziaSDKGet(client, func(ctx context.Context, service *zsdk.Service, id int) (*filteringrules.FirewallFilteringRules, error) {
				return filteringrules.Get(ctx, service, id)
			}),
			firewallFilteringRuleSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceForwardingRules}: newListGetHandler(
			resourceForwardingRules,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]forwardingrules.ForwardingRules, error) {
				return forwardingrules.GetAll(ctx, service)
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
				return networkservices.GetAllNetworkServices(ctx, service, nil, nil)
			}),
			ziaSDKGet(client, func(ctx context.Context, service *zsdk.Service, id int) (*networkservices.NetworkServices, error) {
				return networkservices.Get(ctx, service, id)
			}),
			networkServiceSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceNetworkSvcGroups}: newListGetHandler(
			resourceNetworkSvcGroups,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]networkservicegroups.NetworkServiceGroups, error) {
				return networkservicegroups.GetAllNetworkServiceGroups(ctx, service)
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
				return applicationservices.GetAll(ctx, service)
			}),
			ziaSDKListGetByIntID(
				client,
				func(ctx context.Context, service *zsdk.Service) ([]applicationservices.ApplicationServicesLite, error) {
					return applicationservices.GetAll(ctx, service)
				},
				func(item applicationservices.ApplicationServicesLite) int { return item.ID },
			),
			applicationServiceSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceAppServiceGroups}: newListGetHandler(
			resourceAppServiceGroups,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]appservicegroups.ApplicationServicesGroupLite, error) {
				return appservicegroups.GetAll(ctx, service)
			}),
			ziaSDKListGetByIntID(
				client,
				func(ctx context.Context, service *zsdk.Service) ([]appservicegroups.ApplicationServicesGroupLite, error) {
					return appservicegroups.GetAll(ctx, service)
				},
				func(item appservicegroups.ApplicationServicesGroupLite) int { return item.ID },
			),
			applicationServiceGroupSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceNetworkAppGroups}: newListGetHandler(
			resourceNetworkAppGroups,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]networkapplicationgroups.NetworkApplicationGroups, error) {
				return networkapplicationgroups.GetAllNetworkApplicationGroups(ctx, service)
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
				return proxies.GetAll(ctx, service)
			}),
			ziaSDKGet(client, func(ctx context.Context, service *zsdk.Service, id int) (*proxies.Proxies, error) {
				return proxies.Get(ctx, service, id)
			}),
			proxySourceRecord,
		),
		{product: resources.ProductZIA, name: resourceProxyGateways}: newListGetHandler(
			resourceProxyGateways,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]proxygateways.ProxyGateways, error) {
				return proxygateways.GetAll(ctx, service)
			}),
			ziaSDKListGetByIntID(
				client,
				func(ctx context.Context, service *zsdk.Service) ([]proxygateways.ProxyGateways, error) {
					return proxygateways.GetAll(ctx, service)
				},
				func(item proxygateways.ProxyGateways) int { return item.ID },
			),
			proxyGatewaySourceRecord,
		),
		{product: resources.ProductZIA, name: resourceDedicatedIPGWs}: newListGetHandler(
			resourceDedicatedIPGWs,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]proxies.DedicatedIPGateways, error) {
				return proxies.GetDedicatedIPGWLite(ctx, service)
			}),
			ziaSDKListGetByIntID(
				client,
				func(ctx context.Context, service *zsdk.Service) ([]proxies.DedicatedIPGateways, error) {
					return proxies.GetDedicatedIPGWLite(ctx, service)
				},
				func(item proxies.DedicatedIPGateways) int { return item.Id },
			),
			dedicatedIPGatewaySourceRecord,
		),
		{product: resources.ProductZIA, name: resourceTimeIntervals}: newListGetHandler(
			resourceTimeIntervals,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]timeintervals.TimeInterval, error) {
				return timeintervals.GetAll(ctx, service)
			}),
			ziaSDKGet(client, func(ctx context.Context, service *zsdk.Service, id int) (*timeintervals.TimeInterval, error) {
				return timeintervals.Get(ctx, service, id)
			}),
			timeIntervalSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceBandwidthClasses}: newListGetHandler(
			resourceBandwidthClasses,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]bandwidthclasses.BandwidthClasses, error) {
				return bandwidthclasses.GetAll(ctx, service)
			}),
			ziaSDKGet(client, func(ctx context.Context, service *zsdk.Service, id int) (*bandwidthclasses.BandwidthClasses, error) {
				return bandwidthclasses.Get(ctx, service, id)
			}),
			bandwidthClassSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceBandwidthRules}: newListGetHandler(
			resourceBandwidthRules,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]bandwidthcontrolrules.BandwidthControlRules, error) {
				return bandwidthcontrolrules.GetAll(ctx, service)
			}),
			ziaSDKGet(client, func(ctx context.Context, service *zsdk.Service, id int) (*bandwidthcontrolrules.BandwidthControlRules, error) {
				return bandwidthcontrolrules.Get(ctx, service, id)
			}),
			bandwidthControlRuleSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceIPSPolicies}: newListGetHandler(
			resourceIPSPolicies,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]ipspolicies.FirewallIPSRules, error) {
				return ipspolicies.GetAll(ctx, service)
			}),
			ziaSDKGet(client, func(ctx context.Context, service *zsdk.Service, id int) (*ipspolicies.FirewallIPSRules, error) {
				return ipspolicies.Get(ctx, service, id)
			}),
			ipsPolicySourceRecord,
		),
		{product: resources.ProductZIA, name: resourceDNSGateways}: newListGetHandler(
			resourceDNSGateways,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]dnsgateways.DNSGateways, error) {
				return dnsgateways.GetAll(ctx, service)
			}),
			ziaSDKGet(client, func(ctx context.Context, service *zsdk.Service, id int) (*dnsgateways.DNSGateways, error) {
				return dnsgateways.Get(ctx, service, id)
			}),
			dnsGatewaySourceRecord,
		),
		{product: resources.ProductZIA, name: resourceNATRules}: newListGetHandler(
			resourceNATRules,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]natcontrol.NatControlPolicies, error) {
				return natcontrol.GetAll(ctx, service)
			}),
			ziaSDKGet(client, func(ctx context.Context, service *zsdk.Service, id int) (*natcontrol.NatControlPolicies, error) {
				return natcontrol.Get(ctx, service, id)
			}),
			natControlRuleSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceGroups}: newListGetHandler(
			resourceGroups,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]usergroups.Groups, error) {
				return usergroups.GetAllGroups(ctx, service, nil)
			}),
			ziaSDKGet(client, func(ctx context.Context, service *zsdk.Service, id int) (*usergroups.Groups, error) {
				return usergroups.GetGroups(ctx, service, id)
			}),
			groupSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceDepartments}: newListGetHandler(
			resourceDepartments,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]userdepartments.Department, error) {
				return userdepartments.GetAll(ctx, service, nil)
			}),
			ziaSDKGet(client, func(ctx context.Context, service *zsdk.Service, id int) (*userdepartments.Department, error) {
				return userdepartments.GetDepartments(ctx, service, id)
			}),
			departmentSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceUsers}: newListGetHandler(
			resourceUsers,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]ziausers.Users, error) {
				return ziausers.GetAllUsers(ctx, service, nil)
			}),
			ziaSDKGet(client, func(ctx context.Context, service *zsdk.Service, id int) (*ziausers.Users, error) {
				return ziausers.Get(ctx, service, id)
			}),
			userSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceDeviceGroups}: newListGetHandler(
			resourceDeviceGroups,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]devicegroups.DeviceGroups, error) {
				return devicegroups.GetAllDevicesGroups(ctx, service)
			}),
			ziaSDKListGetByIntID(
				client,
				func(ctx context.Context, service *zsdk.Service) ([]devicegroups.DeviceGroups, error) {
					return devicegroups.GetAllDevicesGroups(ctx, service)
				},
				func(item devicegroups.DeviceGroups) int { return item.ID },
			),
			deviceGroupSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceDevices}: newListGetHandler(
			resourceDevices,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]devicegroups.Devices, error) {
				return devicegroups.GetAllDevices(ctx, service)
			}),
			ziaSDKGet(client, func(ctx context.Context, service *zsdk.Service, id int) (*devicegroups.Devices, error) {
				return devicegroups.GetDevicesByID(ctx, service, id)
			}),
			deviceSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceWorkloadGroups}: newListGetHandler(
			resourceWorkloadGroups,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]workloadgroups.WorkloadGroup, error) {
				return workloadgroups.GetAll(ctx, service)
			}),
			ziaSDKGet(client, func(ctx context.Context, service *zsdk.Service, id int) (*workloadgroups.WorkloadGroup, error) {
				return workloadgroups.Get(ctx, service, id)
			}),
			workloadGroupSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceAlertSubs}: newListGetHandler(
			resourceAlertSubs,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]alerts.AlertSubscriptions, error) {
				return alerts.GetAll(ctx, service)
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
				return intermediatecacertificates.GetAll(ctx, service)
			}),
			ziaSDKGet(client, func(ctx context.Context, service *zsdk.Service, id int) (*intermediatecacertificates.IntermediateCACertificate, error) {
				return intermediatecacertificates.GetCertificate(ctx, service, id)
			}),
			intermediateCACertificateSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceCloudAppInsts}: newListGetHandler(
			resourceCloudAppInsts,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]cloudappinstances.CloudApplicationInstances, error) {
				return cloudappinstances.GetAll(ctx, service)
			}),
			ziaSDKGet(client, func(ctx context.Context, service *zsdk.Service, id int) (*cloudappinstances.CloudApplicationInstances, error) {
				return cloudappinstances.Get(ctx, service, id)
			}),
			cloudAppInstanceSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceTenancyProfiles}: newListGetHandler(
			resourceTenancyProfiles,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]tenancyrestriction.TenancyRestrictionProfile, error) {
				return tenancyrestriction.GetAll(ctx, service)
			}),
			ziaSDKGet(client, func(ctx context.Context, service *zsdk.Service, id int) (*tenancyrestriction.TenancyRestrictionProfile, error) {
				return tenancyrestriction.Get(ctx, service, id)
			}),
			tenancyRestrictionProfileSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceVZENClusters}: newListGetHandler(
			resourceVZENClusters,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]vzenclusters.VZENClusters, error) {
				return vzenclusters.GetAll(ctx, service)
			}),
			ziaSDKGet(client, func(ctx context.Context, service *zsdk.Service, id int) (*vzenclusters.VZENClusters, error) {
				return vzenclusters.Get(ctx, service, id)
			}),
			vzenClusterSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceVZENNodes}: newListGetHandler(
			resourceVZENNodes,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]vzennodes.VZENNodes, error) {
				return vzennodes.GetAll(ctx, service)
			}),
			ziaSDKGet(client, func(ctx context.Context, service *zsdk.Service, id int) (*vzennodes.VZENNodes, error) {
				return vzennodes.Get(ctx, service, id)
			}),
			vzenNodeSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceBrowserIsolation}: newListOnlyHandler(
			resourceBrowserIsolation,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]browserisolation.CBIProfile, error) {
				return browserisolation.GetAll(ctx, service)
			}),
			browserIsolationProfileSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceDLPEngines}: newListGetHandler(
			resourceDLPEngines,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]dlpengines.DLPEngines, error) {
				return dlpengines.GetAll(ctx, service)
			}),
			ziaSDKGet(client, func(ctx context.Context, service *zsdk.Service, id int) (*dlpengines.DLPEngines, error) {
				return dlpengines.Get(ctx, service, id)
			}),
			dlpEngineSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceDLPDictionaries}: newListGetHandler(
			resourceDLPDictionaries,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]dlpdictionaries.DlpDictionary, error) {
				return dlpdictionaries.GetAll(ctx, service)
			}),
			ziaSDKGet(client, func(ctx context.Context, service *zsdk.Service, id int) (*dlpdictionaries.DlpDictionary, error) {
				return dlpdictionaries.Get(ctx, service, id)
			}),
			dlpDictionarySourceRecord,
		),
		{product: resources.ProductZIA, name: resourceDLPEDMSchemas}: newListGetHandler(
			resourceDLPEDMSchemas,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]dlpexactdatamatch.DLPEDMSchema, error) {
				return dlpexactdatamatch.GetAll(ctx, service)
			}),
			ziaSDKGet(client, func(ctx context.Context, service *zsdk.Service, id int) (*dlpexactdatamatch.DLPEDMSchema, error) {
				return dlpexactdatamatch.GetDLPEDMSchemaID(ctx, service, id)
			}),
			dlpEDMSchemaSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceDLPEDMLite}: newListOnlyHandler(
			resourceDLPEDMLite,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]dlpedmlite.DLPEDMLite, error) {
				return dlpedmlite.GetAllEDMSchema(ctx, service, false, false)
			}),
			dlpEDMLiteSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceDLPIDMLite}: newListGetHandler(
			resourceDLPIDMLite,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]dlpidmprofilelite.DLPIDMProfileLite, error) {
				return dlpidmprofilelite.GetAll(ctx, service, false)
			}),
			ziaSDKGet(client, func(ctx context.Context, service *zsdk.Service, id int) (*dlpidmprofilelite.DLPIDMProfileLite, error) {
				return dlpidmprofilelite.GetDLPProfileLiteID(ctx, service, id, false)
			}),
			dlpIDMProfileLiteSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceDLPIDMProfiles}: newListGetHandler(
			resourceDLPIDMProfiles,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]dlpidmprofiles.DLPIDMProfile, error) {
				return dlpidmprofiles.GetAll(ctx, service)
			}),
			ziaSDKGet(client, func(ctx context.Context, service *zsdk.Service, id int) (*dlpidmprofiles.DLPIDMProfile, error) {
				return dlpidmprofiles.Get(ctx, service, id)
			}),
			dlpIDMProfileSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceDLPWebRules}: newListGetHandler(
			resourceDLPWebRules,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]dlpwebrules.WebDLPRules, error) {
				return dlpwebrules.GetAll(ctx, service)
			}),
			ziaSDKGet(client, func(ctx context.Context, service *zsdk.Service, id int) (*dlpwebrules.WebDLPRules, error) {
				return dlpwebrules.Get(ctx, service, id)
			}),
			dlpWebRuleSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceDLPICAPServers}: newListGetHandler(
			resourceDLPICAPServers,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]dlpicapservers.DLPICAPServers, error) {
				return dlpicapservers.GetAll(ctx, service)
			}),
			ziaSDKGet(client, func(ctx context.Context, service *zsdk.Service, id int) (*dlpicapservers.DLPICAPServers, error) {
				return dlpicapservers.Get(ctx, service, id)
			}),
			dlpICAPServerSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceDLPIncidentRcvs}: newListGetHandler(
			resourceDLPIncidentRcvs,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]dlpincidentreceivers.IncidentReceiverServers, error) {
				return dlpincidentreceivers.GetAll(ctx, service)
			}),
			ziaSDKGet(client, func(ctx context.Context, service *zsdk.Service, id int) (*dlpincidentreceivers.IncidentReceiverServers, error) {
				return dlpincidentreceivers.Get(ctx, service, id)
			}),
			dlpIncidentReceiverSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceDLPNotifyTmpls}: newListGetHandler(
			resourceDLPNotifyTmpls,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]dlpnotificationtemplates.DlpNotificationTemplates, error) {
				return dlpnotificationtemplates.GetAll(ctx, service)
			}),
			ziaSDKGet(client, func(ctx context.Context, service *zsdk.Service, id int) (*dlpnotificationtemplates.DlpNotificationTemplates, error) {
				return dlpnotificationtemplates.Get(ctx, service, id)
			}),
			dlpNotificationTemplateSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceC2CIncidentRcvs}: newListGetHandler(
			resourceC2CIncidentRcvs,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]c2cincidentreceiver.C2CIncidentReceiver, error) {
				return c2cincidentreceiver.GetAll(ctx, service)
			}),
			ziaSDKGet(client, func(ctx context.Context, service *zsdk.Service, id int) (*c2cincidentreceiver.C2CIncidentReceiver, error) {
				return c2cincidentreceiver.Get(ctx, service, id)
			}),
			c2cIncidentReceiverSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceRiskProfiles}: newListGetHandler(
			resourceRiskProfiles,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]riskprofiles.RiskProfiles, error) {
				return riskprofiles.GetAll(ctx, service)
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
				return cloudnss.GetAll(ctx, service)
			}),
			ziaSDKGet(client, func(ctx context.Context, service *zsdk.Service, id int) (*cloudnss.NSSFeed, error) {
				return cloudnss.Get(ctx, service, id)
			}),
			nssFeedSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceFileTypeRules}: newListGetHandler(
			resourceFileTypeRules,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]filetypecontrol.FileTypeRules, error) {
				return filetypecontrol.GetAll(ctx, service)
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
				return firewalldnscontrolpolicies.GetAll(ctx, service)
			}),
			ziaSDKGet(client, func(ctx context.Context, service *zsdk.Service, id int) (*firewalldnscontrolpolicies.FirewallDNSRules, error) {
				return firewalldnscontrolpolicies.Get(ctx, service, id)
			}),
			firewallDNSRuleSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceCustomFileTypes}: newListGetHandler(
			resourceCustomFileTypes,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]customfiletypes.CustomFileTypes, error) {
				return customfiletypes.GetCustomFileTypes(ctx, service)
			}),
			ziaSDKGet(client, func(ctx context.Context, service *zsdk.Service, id int) (*customfiletypes.CustomFileTypes, error) {
				return customfiletypes.Get(ctx, service, id)
			}),
			customFileTypeSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceZPAGateways}: newListGetHandler(
			resourceZPAGateways,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]zpagateways.ZPAGateways, error) {
				return zpagateways.GetAll(ctx, service)
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
				return subclouds.GetAll(ctx, service)
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
				return ipv6config.GetDns64Prefix(ctx, service)
			}),
			ipv6ConfigPrefixSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceIPv6NAT64Prefix}: newListOnlyHandler(
			resourceIPv6NAT64Prefix,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]ipv6config.IPv6ConfigPrefix, error) {
				return ipv6config.GetNat64Prefix(ctx, service)
			}),
			ipv6ConfigPrefixSourceRecord,
		),
		{product: resources.ProductZIA, name: resourcePACFiles}: newListOnlyHandler(
			resourcePACFiles,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]pacfiles.PACFileConfig, error) {
				return pacfiles.GetPacFiles(ctx, service, "")
			}),
			pacFileSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceCloudAppPolicy}: newListOnlyHandler(
			resourceCloudAppPolicy,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]cloudapplications.CloudApplications, error) {
				return cloudapplications.GetCloudApplicationPolicy(ctx, service, map[string]any{})
			}),
			cloudApplicationPolicySourceRecord,
		),
		{product: resources.ProductZIA, name: resourceCloudAppSSLPol}: newListOnlyHandler(
			resourceCloudAppSSLPol,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]cloudapplications.CloudApplications, error) {
				return cloudapplications.GetCloudApplicationSSLPolicy(ctx, service, map[string]any{})
			}),
			cloudApplicationPolicySourceRecord,
		),
		{product: resources.ProductZIA, name: resourceDomainProfiles}: newListOnlyHandler(
			resourceDomainProfiles,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]saassecurityapi.DomainProfiles, error) {
				return saassecurityapi.GetDomainProfiles(ctx, service)
			}),
			domainProfileSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceCASBTombstones}: newListOnlyHandler(
			resourceCASBTombstones,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]saassecurityapi.QuarantineTombstoneLite, error) {
				return saassecurityapi.GetQuarantineTombstoneLite(ctx, service)
			}),
			casbTombstoneTemplateSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceCASBEmailLabels}: newListOnlyHandler(
			resourceCASBEmailLabels,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]saassecurityapi.CasbEmailLabel, error) {
				return saassecurityapi.GetCasbEmailLabelLite(ctx, service)
			}),
			casbEmailLabelSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceCASBTenants}: newListOnlyHandler(
			resourceCASBTenants,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]saassecurityapi.CasbTenants, error) {
				return saassecurityapi.GetCasbTenantLite(ctx, service, map[string]any{})
			}),
			casbTenantSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceCASBDLPRules}: newListOnlyHandler(
			resourceCASBDLPRules,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]casbdlprules.CasbDLPRules, error) {
				return casbdlprules.GetAll(ctx, service)
			}),
			casbDLPRuleSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceCASBMalwareRules}: newListOnlyHandler(
			resourceCASBMalwareRules,
			ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]casbmalwarerules.CasbMalwareRules, error) {
				return casbmalwarerules.GetAll(ctx, service)
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
				return ziaadminusers.GetAllAdminUsers(ctx, service)
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
				return emailprofiles.GetAll(ctx, service, nil)
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
		m[k] = v
	}
}
