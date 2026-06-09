package zscaler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	sdkcache "github.com/zscaler/zscaler-sdk-go/v3/cache"
	sdklogger "github.com/zscaler/zscaler-sdk-go/v3/logger"
	zsdk "github.com/zscaler/zscaler-sdk-go/v3/zscaler"
	sdkerrorx "github.com/zscaler/zscaler-sdk-go/v3/zscaler/errorx"
	sdkzia "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia"
	activation "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/activation"
	ziaadminusers "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/adminuserrolemgmt/admins"
	ziaadminroles "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/adminuserrolemgmt/roles"
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
	intermediatecacertificates "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/intermediatecacertificates"
	ipspolicies "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/ips_control_policies/ips_policies"
	"github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/location/locationgroups"
	"github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/location/locationmanagement"
	natcontrol "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/nat_control_policies"
	pacfiles "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/pacfiles"
	rulelabels "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/rule_labels"
	saassecurityapi "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/saas_security_api"
	casbdlprules "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/saas_security_api/casb_dlp_rules"
	casbmalwarerules "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/saas_security_api/casb_malware_rules"
	sandboxrules "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/sandbox/sandbox_rules"
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
	userdepartments "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/usermanagement/departments"
	usergroups "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/usermanagement/groups"
	ziausers "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/usermanagement/users"
	vzenclusters "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/vzen_clusters"
	vzennodes "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/vzen_nodes"
	"github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/workloadgroups"
	zidcommon "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zid/services/common"
	zidresourceservers "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zid/services/resource_servers"
	zpaapplicationsegment "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/applicationsegment"
	zpaapplicationsegmentbrowseraccess "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/applicationsegmentbrowseraccess"
	zpacommon "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/common"
	zpaservergroup "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/servergroup"
	ztwadminusers "github.com/zscaler/zscaler-sdk-go/v3/zscaler/ztw/services/adminuserrolemgmt/adminusers"
	ztwcommon "github.com/zscaler/zscaler-sdk-go/v3/zscaler/ztw/services/common"
	ztwnetworkservicegroups "github.com/zscaler/zscaler-sdk-go/v3/zscaler/ztw/services/policyresources/networkservicegroups"
	ztwnetworkservices "github.com/zscaler/zscaler-sdk-go/v3/zscaler/ztw/services/policyresources/networkservices"
	ztwworkloadgroups "github.com/zscaler/zscaler-sdk-go/v3/zscaler/ztw/services/workload_groups"

	"github.com/dvmrry/zscalerctl/internal/resources"
	"github.com/dvmrry/zscalerctl/internal/secret"
)

var (
	ErrMissingCredentials  = errors.New("missing zscaler API credentials")
	ErrUnsupportedResource = errors.New("unsupported zscaler resource")
	ErrInvalidResourceID   = errors.New("invalid zscaler resource id")
	ErrLiveAccessFailed    = errors.New("zscaler API request failed")
	ErrInvalidProxyConfig  = errors.New("invalid zscaler proxy config")
)

const defaultTimeout = 30 * time.Second

const (
	resourceLocations         = "locations"
	resourceLocationGroups    = "location-groups"
	resourceRuleLabels        = "rule-labels"
	resourceAuthSettings      = "auth-settings"
	resourceStaticIPs         = "static-ips"
	resourceGRETunnels        = "gre-tunnels"
	resourceSublocations      = "sublocations"
	resourceSSLRules          = "ssl-inspection-rules"
	resourceURLCategories     = "url-categories"
	resourceURLRules          = "url-filtering-rules"
	resourceFirewallRules     = "firewall-filtering-rules"
	resourceForwardingRules   = "forwarding-rules"
	resourceIPSourceGroups    = "ip-source-groups"
	resourceIPDestGroups      = "ip-destination-groups"
	resourceNetworkServices   = "network-services"
	resourceNetworkSvcGroups  = "network-service-groups"
	resourceNetworkApps       = "network-applications"
	resourceAppServices       = "application-services"
	resourceAppServiceGroups  = "application-service-groups"
	resourceNetworkAppGroups  = "network-application-groups"
	resourceTimeWindows       = "time-windows"
	resourceProxies           = "proxies"
	resourceProxyGateways     = "proxy-gateways"
	resourceDedicatedIPGWs    = "dedicated-ip-gateways"
	resourceTimeIntervals     = "time-intervals"
	resourceBandwidthClasses  = "bandwidth-classes"
	resourceBandwidthRules    = "bandwidth-control-rules"
	resourceIPSPolicies       = "ips-policies"
	resourceDNSGateways       = "dns-gateways"
	resourceNATRules          = "nat-control-rules"
	resourceCloudAppControl   = "cloud-app-control"
	resourceIPSSignatureRules = "ips-signature-rules"
	resourceGroups            = "groups"
	resourceDepartments       = "departments"
	resourceUsers             = "users"
	resourceDeviceGroups      = "device-groups"
	resourceDevices           = "devices"
	resourceWorkloadGroups    = "workload-groups"
	resourceAlertSubs         = "alert-subscriptions"
	resourceActivationStatus  = "activation-status"
	resourceEUSAStatus        = "eusa-status"
	resourceAuthExemptedURLs  = "auth-exempted-urls"
	resourceIntermediateCAs   = "intermediate-ca-certificates"
	resourceCloudAppInsts     = "cloud-app-instances"
	resourceTenancyProfiles   = "tenancy-restriction-profiles"
	resourceVZENClusters      = "vzen-clusters"
	resourceVZENNodes         = "vzen-nodes"
	resourceBrowserIsolation  = "browser-isolation-profiles"
	resourceDLPEngines        = "dlp-engines"
	resourceDLPDictionaries   = "dlp-dictionaries"
	resourceDLPEDMSchemas     = "dlp-edm-schemas"
	resourceDLPEDMLite        = "dlp-edm-schemas-lite"
	resourceDLPIDMLite        = "dlp-idm-profile-lite"
	resourceDLPIDMProfiles    = "dlp-idm-profiles"
	resourceDLPWebRules       = "dlp-web-rules"
	resourceDLPICAPServers    = "dlp-icap-servers"
	resourceDLPIncidentRcvs   = "dlp-incident-receiver-servers"
	resourceDLPNotifyTmpls    = "dlp-notification-templates"
	resourceC2CIncidentRcvs   = "c2c-incident-receivers"
	resourceRiskProfiles      = "risk-profiles"
	resourceNSSServers        = "nss-servers"
	resourceNSSFeeds          = "nss-feeds"
	resourceFileTypeRules     = "file-type-rules"
	resourceSandboxRules      = "sandbox-rules"
	resourceFirewallDNSRules  = "firewall-dns-rules"
	resourceCustomFileTypes   = "custom-file-types"
	resourceZPAGateways       = "zpa-gateways"
	resourceDCExclusions      = "dc-exclusions"
	resourceSubClouds         = "sub-clouds"
	resourceIPv6Config        = "ipv6-config"
	resourceIPv6DNS64Prefix   = "ipv6-dns64-prefixes"
	resourceIPv6NAT64Prefix   = "ipv6-nat64-prefixes"
	resourcePACFiles          = "pac-files"
	resourceCloudAppPolicy    = "cloud-application-policy"
	resourceCloudAppSSLPol    = "cloud-application-ssl-policy"
	resourceDomainProfiles    = "domain-profiles"
	resourceCASBTombstones    = "casb-tombstone-templates"
	resourceCASBEmailLabels   = "casb-email-labels"
	resourceCASBTenants       = "casb-tenants"
	resourceCASBDLPRules      = "casb-dlp-rules"
	resourceCASBMalwareRules  = "casb-malware-rules"
	resourceBrowserControl    = "browser-control-settings"
	resourceSupportedBrowsers = "supported-browser-versions"
	resourceFTPControl        = "ftp-control-policy"
	resourceRemoteAssistance  = "remote-assistance"
	resourcePublicCloudAccts  = "public-cloud-accounts"
	resourceForwardingGWs     = "forwarding-gateways"
	resourceECGroups          = "ec-groups"
	resourceIPGroups          = "ip-groups"
	resourceAdminUsers        = "admin-users"
	resourceAdminRoles        = "admin-roles"
	resourceLocationTmpls     = "location-templates"
	resourceAccountGroups     = "account-groups"
	resourcePublicCloudInfo   = "public-cloud-info"
	resourceZTWZPAAppSegs     = "zpa-application-segments"
	resourceTrafficDNSRules   = "traffic-dns-rules"
	resourceTrafficLogRules   = "traffic-log-rules"
	resourceZCCFailOpenPolicy = "fail-open-policy"
	resourceZCCFwdProfiles    = "forwarding-profiles"
	resourceZCCTrustedNets    = "trusted-networks"
	resourceZCCWebAppServices = "web-app-services"
	resourceZCCAppProfiles    = "application-profiles"
	resourceZCCCustomIPApps   = "custom-ip-apps"
	resourceZCCPredefIPApps   = "predefined-ip-apps"
	resourceZCCProcessApps    = "process-based-apps"
	resourceZCCDevices        = "devices"
	resourceZCCAdminRoles     = "admin-roles"
	resourceZCCCompanyInfo    = "company-info"
	resourceZTWActivationStat = "activation-status"
	resourceEmailProfiles     = "email-profiles"

	resourceAdvancedSettings           = "advanced-settings"
	resourceAdvancedThreatSettings     = "advanced-threat-settings"
	resourceMobileThreatSettings       = "mobile-threat-settings"
	resourceSandboxSettings            = "sandbox-settings"
	resourceEndUserNotification        = "end-user-notification-settings"
	resourceOrgInformation             = "org-information"
	resourceATPMalwarePolicy           = "atp-malware-policy"
	resourceATPMalwareSettings         = "atp-malware-settings"
	resourceATPMalwareInspection       = "atp-malware-inspection"
	resourceATPMalwareProtocols        = "atp-malware-protocols"
	resourceMaliciousURLs              = "malicious-urls"
	resourceSecurityExceptions         = "security-exceptions"
	resourceSecurityPolicyURLAllowlist = "url-allow-list"
	resourceSecurityPolicyURLDenylist  = "url-deny-list"
	resourceZPAServerGroups            = "server-groups"
	resourceZPAMicrotenants            = "microtenants"
	resourceZPAVersionProfiles         = "version-profiles"
	resourceZPAClientTypes             = "client-types"
	resourceZPAPlatforms               = "platforms"
	resourceZPASegmentGroups           = "segment-groups"
	resourceZPAAppSegments             = "application-segments"
	resourceZPAAppConnectors           = "app-connectors"
	resourceZPAConnectorGrps           = "app-connector-groups"
	resourceZPAAppServers              = "app-servers"
	resourceZPAMachineGroups           = "machine-groups"
	resourceZPATrustedNets             = "trusted-networks"
	resourceZPAServiceEdges            = "service-edges"
	resourceZPAServiceGrps             = "service-edge-groups"
	resourceZPACloudConns              = "cloud-connectors"
	resourceZPACloudConnGrps           = "cloud-connector-groups"
	resourceZPAPostureProfs            = "posture-profiles"
	resourceZPACBIZPAProfs             = "cbi-zpa-profiles"
	resourceZPAC2CIPRanges             = "c2c-ip-ranges"
	resourceZPAConfigOvrds             = "config-overrides"
	resourceZPAIsolationProfiles       = "isolation-profiles"
	resourceZPABranchConnectors        = "branch-connectors"
	resourceZPAUserPortals             = "user-portals"
	resourceZPAUserPortalAups          = "user-portal-aups"
	resourceZPAUserPortalLinks         = "user-portal-links"
	resourceZPABrowserAccess           = "browser-access"
	resourceZPAInspectionAppSegments   = "inspection-app-segments"
	resourceZPAPRAAppSegments          = "pra-app-segments"
	resourceZidentityGroups            = "groups"
	resourceZidentityUsers             = "users"
	resourceZidentityResourceServers   = "resource-servers"
	zidentityGroupsEndpoint            = "/admin/api/v1/groups"
	zidentityUsersEndpoint             = "/admin/api/v1/users"
	zidentityResourceServersEndpoint   = "/admin/api/v1/resource-servers"
)

type AuthMode string

const (
	AuthModeOneAPI    AuthMode = "oneapi"
	AuthModeZIALegacy AuthMode = "zia-legacy"
)

type ReaderConfig struct {
	ClientID         secret.Secret
	ClientSecret     secret.Secret
	VanityDomain     string
	Cloud            string
	ZPACustomerID    string
	ZPAMicrotenantID string
	AuthMode         AuthMode
	ZIALegacy        ZIALegacyConfig
	Proxy            ProxyConfig
	Timeout          time.Duration
	NoCache          bool
}

type ZIALegacyConfig struct {
	Username secret.Secret
	Password secret.Secret
	APIKey   secret.Secret
	Cloud    string
}

type ProxyConfig struct {
	URL             string
	FromEnvironment bool
}

type SDKReader struct {
	cfg      ReaderConfig
	handlers map[resourceKey]resourceHandler
}

type ResourceSession interface {
	List(context.Context, resources.Product, string) ([]resources.SourceRecord, error)
	Get(context.Context, resources.Product, string, string) (resources.SourceRecord, error)
	Show(context.Context, resources.Product, string) (resources.SourceRecord, error)
	Close()
}

type SDKSession struct {
	handlers  map[resourceKey]resourceHandler
	closeOnce sync.Once
	cleanup   func()
}

type resourceKey struct {
	product resources.Product
	name    string
}

type resourceHandler interface {
	List(context.Context) ([]resources.SourceRecord, error)
	Get(context.Context, string) (resources.SourceRecord, error)
	Show(context.Context) (resources.SourceRecord, error)
}

type zscalerServiceProvider interface {
	service(context.Context, resources.Product) (*zsdk.Service, func(), error)
}

var (
	_ resourceHandler = listGetHandler[struct{}]{}
	_ resourceHandler = listOnlyHandler[struct{}]{}
	_ resourceHandler = singletonHandler[struct{}]{}
	_ ResourceSession = (*SDKSession)(nil)
)

func NewReader(cfg ReaderConfig) (*SDKReader, error) {
	cfg.AuthMode = effectiveAuthMode(cfg.AuthMode)
	cfg.VanityDomain = strings.TrimSpace(cfg.VanityDomain)
	cfg.Cloud = strings.TrimSpace(cfg.Cloud)
	cfg.ZIALegacy.Cloud = strings.TrimSpace(cfg.ZIALegacy.Cloud)
	cfg.Timeout = effectiveTimeout(cfg.Timeout)
	if err := validateReaderConfig(cfg); err != nil {
		return nil, err
	}
	client := sdkClient{services: perCallService{cfg: cfg}}
	return &SDKReader{
		cfg:      cfg,
		handlers: newResourceHandlers(client),
	}, nil
}

func (r *SDKReader) Session(ctx context.Context, product resources.Product) (ResourceSession, error) {
	if r == nil {
		return nil, fmt.Errorf("%w: %s/session", ErrUnsupportedResource, product)
	}
	switch product {
	case resources.ProductZIA, resources.ProductZPA, resources.ProductZTW, resources.ProductZCC, resources.ProductZidentity:
	default:
		return nil, fmt.Errorf("%w: %s/session", ErrUnsupportedResource, product)
	}
	service, cleanup, err := perCallService{cfg: r.cfg}.service(ctx, product)
	if err != nil {
		if errors.Is(err, ErrMissingCredentials) {
			return nil, err
		}
		return nil, normalizeLiveError(ctx, "authenticate", product, "session", err)
	}
	client := sdkClient{services: fixedService{sdkService: service}}
	return &SDKSession{
		handlers: newResourceHandlers(client),
		cleanup:  cleanup,
	}, nil
}

func (s *SDKSession) Close() {
	if s == nil {
		return
	}
	s.closeOnce.Do(func() {
		if s.cleanup != nil {
			s.cleanup()
		}
	})
}

func (r *SDKReader) List(ctx context.Context, product resources.Product, name string) ([]resources.SourceRecord, error) {
	if r == nil {
		return listResource(ctx, nil, product, name)
	}
	return listResource(ctx, r.handlers, product, name)
}

func (r *SDKReader) Get(ctx context.Context, product resources.Product, name string, id string) (resources.SourceRecord, error) {
	if r == nil {
		return getResource(ctx, nil, product, name, id)
	}
	return getResource(ctx, r.handlers, product, name, id)
}

func (r *SDKReader) Show(ctx context.Context, product resources.Product, name string) (resources.SourceRecord, error) {
	if r == nil {
		return showResource(ctx, nil, product, name)
	}
	return showResource(ctx, r.handlers, product, name)
}

func (s *SDKSession) List(ctx context.Context, product resources.Product, name string) ([]resources.SourceRecord, error) {
	if s == nil {
		return listResource(ctx, nil, product, name)
	}
	return listResource(ctx, s.handlers, product, name)
}

func (s *SDKSession) Get(ctx context.Context, product resources.Product, name string, id string) (resources.SourceRecord, error) {
	if s == nil {
		return getResource(ctx, nil, product, name, id)
	}
	return getResource(ctx, s.handlers, product, name, id)
}

func (s *SDKSession) Show(ctx context.Context, product resources.Product, name string) (resources.SourceRecord, error) {
	if s == nil {
		return showResource(ctx, nil, product, name)
	}
	return showResource(ctx, s.handlers, product, name)
}

func listResource(
	ctx context.Context,
	handlers map[resourceKey]resourceHandler,
	product resources.Product,
	name string,
) ([]resources.SourceRecord, error) {
	handler, err := handlerFrom(handlers, product, name)
	if err != nil {
		return nil, err
	}
	records, err := handler.List(ctx)
	if err != nil {
		if errors.Is(err, ErrMissingCredentials) {
			return nil, err
		}
		return nil, normalizeLiveError(ctx, "list", product, name, err)
	}
	return records, nil
}

func getResource(
	ctx context.Context,
	handlers map[resourceKey]resourceHandler,
	product resources.Product,
	name string,
	id string,
) (resources.SourceRecord, error) {
	handler, err := handlerFrom(handlers, product, name)
	if err != nil {
		return resources.SourceRecord{}, err
	}
	record, err := handler.Get(ctx, id)
	if err != nil {
		if errors.Is(err, ErrInvalidResourceID) ||
			errors.Is(err, ErrUnsupportedResource) ||
			errors.Is(err, ErrMissingCredentials) {
			return resources.SourceRecord{}, err
		}
		return resources.SourceRecord{}, normalizeLiveError(ctx, "get", product, name, err)
	}
	return record, nil
}

func showResource(
	ctx context.Context,
	handlers map[resourceKey]resourceHandler,
	product resources.Product,
	name string,
) (resources.SourceRecord, error) {
	handler, err := handlerFrom(handlers, product, name)
	if err != nil {
		return resources.SourceRecord{}, err
	}
	record, err := handler.Show(ctx)
	if err != nil {
		// Missing credentials are caught upstream at session creation, but keep
		// the passthrough symmetric with listResource/getResource so a future
		// per-call surfacing classifies as a credential error, not live access.
		if errors.Is(err, ErrMissingCredentials) {
			return resources.SourceRecord{}, err
		}
		return resources.SourceRecord{}, normalizeLiveError(ctx, "show", product, name, err)
	}
	return record, nil
}

func handlerFrom(handlers map[resourceKey]resourceHandler, product resources.Product, name string) (resourceHandler, error) {
	if handlers == nil {
		return nil, fmt.Errorf("%w: %s/%s", ErrUnsupportedResource, product, name)
	}
	handler, ok := handlers[resourceKey{product: product, name: name}]
	if !ok {
		return nil, fmt.Errorf("%w: %s/%s", ErrUnsupportedResource, product, name)
	}
	return handler, nil
}

func newResourceHandlers(client sdkClient) map[resourceKey]resourceHandler {
	m := make(map[resourceKey]resourceHandler)
	addZIAHandlers(m, client)
	addZPAHandlers(m, client)
	addZTWHandlers(m, client)
	addZCCHandlers(m, client)
	addZidentityHandlers(m, client)
	return m
}

// addHandler registers h under key, panicking on a duplicate. Before the
// per-product split the handler registry was a single map literal, where a
// duplicate {product,name} key was a compile error; this restores that
// guarantee at construction time so a collision across product files fails
// loudly instead of silently overwriting.
func addHandler(m map[resourceKey]resourceHandler, key resourceKey, h resourceHandler) {
	if _, exists := m[key]; exists {
		panic(fmt.Sprintf("duplicate resource handler: %s/%s", key.product, key.name))
	}
	m[key] = h
}

func getNetworkApplicationsPage(ctx context.Context, service *zsdk.Service) ([]networkapplications.NetworkApplications, error) {
	const pageCeiling = 5000
	var applications []networkapplications.NetworkApplications
	// The SDK package's GetAll uses ReadAllPages, which can loop indefinitely
	// when this static catalog endpoint ignores page/pageSize and keeps
	// returning a full page. Read one large SDK page instead.
	err := ziacommon.ReadPage(ctx, service.Client, "/zia/api/v1/networkApplications", 1, &applications, pageCeiling)
	if err != nil {
		return nil, err
	}
	// This endpoint does not paginate, so a result that fills the single page is
	// indistinguishable from a truncated one. Fail closed rather than present a
	// possibly-incomplete list as complete — silent truncation is worse than a
	// visible error for an audit/diff tool.
	if len(applications) >= pageCeiling {
		return nil, fmt.Errorf("zia network-applications returned the full single-page ceiling of %d records; this endpoint does not paginate, so completeness cannot be guaranteed", pageCeiling)
	}
	return applications, nil
}

type listGetHandler[T any] struct {
	resourceName string
	list         func(context.Context) ([]T, error)
	get          func(context.Context, string) (*T, error)
	sourceRecord func(T) resources.SourceRecord
}

type listOnlyHandler[T any] struct {
	resourceName string
	list         func(context.Context) ([]T, error)
	sourceRecord func(T) resources.SourceRecord
}

func newListGetHandler[T any](
	resourceName string,
	list func(context.Context) ([]T, error),
	get func(context.Context, string) (*T, error),
	sourceRecord func(T) resources.SourceRecord,
) listGetHandler[T] {
	return listGetHandler[T]{
		resourceName: resourceName,
		list:         list,
		get:          get,
		sourceRecord: sourceRecord,
	}
}

func newListOnlyHandler[T any](
	resourceName string,
	list func(context.Context) ([]T, error),
	sourceRecord func(T) resources.SourceRecord,
) listOnlyHandler[T] {
	return listOnlyHandler[T]{
		resourceName: resourceName,
		list:         list,
		sourceRecord: sourceRecord,
	}
}

func (h listGetHandler[T]) List(ctx context.Context) ([]resources.SourceRecord, error) {
	items, err := h.list(ctx)
	if err != nil {
		return nil, err
	}
	records := make([]resources.SourceRecord, 0, len(items))
	for _, item := range items {
		records = append(records, h.sourceRecord(item))
	}
	return records, nil
}

func (h listGetHandler[T]) Get(ctx context.Context, id string) (resources.SourceRecord, error) {
	item, err := h.get(ctx, id)
	if err != nil {
		return resources.SourceRecord{}, err
	}
	if item == nil {
		return resources.SourceRecord{}, fmt.Errorf("empty sdk %s response", h.resourceName)
	}
	return h.sourceRecord(*item), nil
}

func (h listOnlyHandler[T]) List(ctx context.Context) ([]resources.SourceRecord, error) {
	items, err := h.list(ctx)
	if err != nil {
		return nil, err
	}
	records := make([]resources.SourceRecord, 0, len(items))
	for _, item := range items {
		records = append(records, h.sourceRecord(item))
	}
	return records, nil
}

func (h listOnlyHandler[T]) Get(context.Context, string) (resources.SourceRecord, error) {
	return resources.SourceRecord{}, fmt.Errorf("%w: %s get", ErrUnsupportedResource, h.resourceName)
}

func (h listGetHandler[T]) Show(context.Context) (resources.SourceRecord, error) {
	return resources.SourceRecord{}, fmt.Errorf("%w: %s/show", ErrUnsupportedResource, h.resourceName)
}

func (h listOnlyHandler[T]) Show(context.Context) (resources.SourceRecord, error) {
	return resources.SourceRecord{}, fmt.Errorf("%w: %s/show", ErrUnsupportedResource, h.resourceName)
}

type singletonHandler[T any] struct {
	resourceName string
	show         func(context.Context) (*T, error)
	sourceRecord func(T) resources.SourceRecord
}

func newSingletonHandler[T any](
	resourceName string,
	show func(context.Context) (*T, error),
	sourceRecord func(T) resources.SourceRecord,
) singletonHandler[T] {
	return singletonHandler[T]{
		resourceName: resourceName,
		show:         show,
		sourceRecord: sourceRecord,
	}
}

func (h singletonHandler[T]) List(ctx context.Context) ([]resources.SourceRecord, error) {
	record, err := h.Show(ctx)
	if err != nil {
		return nil, err
	}
	return []resources.SourceRecord{record}, nil
}

func (h singletonHandler[T]) Get(context.Context, string) (resources.SourceRecord, error) {
	return resources.SourceRecord{}, fmt.Errorf("%w: %s/get", ErrUnsupportedResource, h.resourceName)
}

func (h singletonHandler[T]) Show(ctx context.Context) (resources.SourceRecord, error) {
	item, err := h.show(ctx)
	if err != nil {
		return resources.SourceRecord{}, err
	}
	if item == nil {
		return resources.SourceRecord{}, fmt.Errorf("empty sdk %s response", h.resourceName)
	}
	return h.sourceRecord(*item), nil
}

func structSourceRecord[T any](value T) resources.SourceRecord {
	return sourceRecordFromStruct(value)
}

func parsePositiveIntID(id string) (int, error) {
	parsed, err := strconv.Atoi(id)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("%w: %q", ErrInvalidResourceID, id)
	}
	return parsed, nil
}

type sdkClient struct {
	services zscalerServiceProvider
}

func (c sdkClient) service(ctx context.Context) (*zsdk.Service, func(), error) {
	if c.services == nil {
		return nil, nil, errors.New("missing zscaler service provider")
	}
	return c.services.service(ctx, resources.ProductZIA)
}

func (c sdkClient) productService(ctx context.Context, product resources.Product) (*zsdk.Service, func(), error) {
	if c.services == nil {
		return nil, nil, errors.New("missing zscaler service provider")
	}
	return c.services.service(ctx, product)
}

// withService acquires a per-call SDK service, runs a single read, and cleans
// up. R is the SDK return type (`[]T` for lists, `*T` for show/singleton reads).
// It is the shared core for the list/show adapters below; per-call service
// acquisition and credential errors propagate unchanged for the reader layer to
// classify.
func withService[R any](
	getService func(context.Context) (*zsdk.Service, func(), error),
	call func(context.Context, *zsdk.Service) (R, error),
) func(context.Context) (R, error) {
	return func(ctx context.Context) (R, error) {
		service, cleanup, err := getService(ctx)
		if err != nil {
			var zero R
			return zero, err
		}
		defer cleanup()
		return call(ctx, service)
	}
}

func ziaSDKList[T any](
	client sdkClient,
	call func(context.Context, *zsdk.Service) ([]T, error),
) func(context.Context) ([]T, error) {
	return withService(client.service, call)
}

func ziaSDKShow[T any](
	client sdkClient,
	call func(context.Context, *zsdk.Service) (*T, error),
) func(context.Context) (*T, error) {
	return withService(client.service, call)
}

func sdkProductShow[T any](
	product resources.Product,
	client sdkClient,
	call func(context.Context, *zsdk.Service) (*T, error),
) func(context.Context) (*T, error) {
	return withService(func(ctx context.Context) (*zsdk.Service, func(), error) {
		return client.productService(ctx, product)
	}, call)
}

func ziaSDKGet[T any](
	client sdkClient,
	call func(context.Context, *zsdk.Service, int) (*T, error),
) func(context.Context, string) (*T, error) {
	return intIDGetter(func(ctx context.Context, id int) (*T, error) {
		service, cleanup, err := client.service(ctx)
		if err != nil {
			return nil, err
		}
		defer cleanup()
		return call(ctx, service, id)
	})
}

func ziaSDKStringGet[T any](
	client sdkClient,
	call func(context.Context, *zsdk.Service, string) (*T, error),
) func(context.Context, string) (*T, error) {
	return func(ctx context.Context, id string) (*T, error) {
		id = strings.TrimSpace(id)
		if id == "" {
			return nil, fmt.Errorf("%w: empty", ErrInvalidResourceID)
		}
		service, cleanup, err := client.service(ctx)
		if err != nil {
			return nil, err
		}
		defer cleanup()
		return call(ctx, service, id)
	}
}

func ziaSDKListGetByIntID[T any](
	client sdkClient,
	list func(context.Context, *zsdk.Service) ([]T, error),
	idOf func(T) int,
) func(context.Context, string) (*T, error) {
	return intIDGetter(func(ctx context.Context, id int) (*T, error) {
		service, cleanup, err := client.service(ctx)
		if err != nil {
			return nil, err
		}
		defer cleanup()
		items, err := list(ctx, service)
		if err != nil {
			return nil, err
		}
		for i := range items {
			if idOf(items[i]) == id {
				return &items[i], nil
			}
		}
		return nil, nil
	})
}

func sdkProductList[T any](
	product resources.Product,
	client sdkClient,
	call func(context.Context, *zsdk.Service) ([]T, error),
) func(context.Context) ([]T, error) {
	return withService(func(ctx context.Context) (*zsdk.Service, func(), error) {
		return client.productService(ctx, product)
	}, call)
}

func sdkProductGet[T any](
	product resources.Product,
	client sdkClient,
	call func(context.Context, *zsdk.Service, int) (*T, error),
) func(context.Context, string) (*T, error) {
	return intIDGetter(func(ctx context.Context, id int) (*T, error) {
		service, cleanup, err := client.productService(ctx, product)
		if err != nil {
			return nil, err
		}
		defer cleanup()
		return call(ctx, service, id)
	})
}

func sdkProductStringGet[T any](
	product resources.Product,
	client sdkClient,
	call func(context.Context, *zsdk.Service, string) (*T, error),
) func(context.Context, string) (*T, error) {
	return func(ctx context.Context, id string) (*T, error) {
		id = strings.TrimSpace(id)
		if id == "" {
			return nil, fmt.Errorf("%w: empty", ErrInvalidResourceID)
		}
		service, cleanup, err := client.productService(ctx, product)
		if err != nil {
			return nil, err
		}
		defer cleanup()
		return call(ctx, service, id)
	}
}

const (
	zidentityPageLimit = 1000
	zidentityMaxPages  = 1000
)

type zidentityPage[T any] struct {
	records      []T
	resultsTotal int
	pageOffset   int
	nextLink     string
}

func zidentityListAll[T any](ctx context.Context, service *zsdk.Service, endpoint string) ([]T, error) {
	return readAllZidentityPages(ctx, func(ctx context.Context, offset, limit int) (zidentityPage[T], error) {
		params := zidcommon.NewPaginationQueryParams(limit)
		params.WithOffset(offset)
		response, err := zidcommon.ReadPageWithPagination[T](ctx, service.Client, endpoint, &params)
		if err != nil {
			return zidentityPage[T]{}, err
		}
		return zidentityPage[T]{
			records:      response.Records,
			resultsTotal: response.ResultsTotal,
			pageOffset:   response.PageOffset,
			nextLink:     response.NextLink,
		}, nil
	})
}

func readAllZidentityPages[T any](
	ctx context.Context,
	readPage func(context.Context, int, int) (zidentityPage[T], error),
) ([]T, error) {
	var all []T
	for pageNumber, offset := 0, 0; ; pageNumber++ {
		if pageNumber >= zidentityMaxPages {
			return nil, fmt.Errorf("zidentity pagination exceeded %d pages at offset %d", zidentityMaxPages, offset)
		}
		page, err := readPage(ctx, offset, zidentityPageLimit)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch zidentity page at offset %d: %w", offset, err)
		}
		if pageNumber > 0 && page.pageOffset != offset {
			return nil, fmt.Errorf("zidentity pagination did not advance: requested offset %d, response pageOffset %d", offset, page.pageOffset)
		}
		all = append(all, page.records...)
		if len(page.records) == 0 {
			return all, nil
		}
		if page.resultsTotal > 0 && len(all) >= page.resultsTotal {
			return all, nil
		}
		if len(page.records) < zidentityPageLimit || page.nextLink == "" {
			return all, nil
		}
		offset += len(page.records)
	}
}

func zpaSDKList[T any](
	client sdkClient,
	call func(context.Context, *zsdk.Service) ([]T, *http.Response, error),
) func(context.Context) ([]T, error) {
	return func(ctx context.Context) ([]T, error) {
		service, cleanup, err := client.productService(ctx, resources.ProductZPA)
		if err != nil {
			return nil, err
		}
		defer cleanup()
		items, _, err := call(ctx, service)
		return items, err
	}
}

func zpaSDKShow[T any](
	client sdkClient,
	call func(context.Context, *zsdk.Service) (*T, *http.Response, error),
) func(context.Context) (*T, error) {
	return func(ctx context.Context) (*T, error) {
		service, cleanup, err := client.productService(ctx, resources.ProductZPA)
		if err != nil {
			return nil, err
		}
		defer cleanup()
		item, _, err := call(ctx, service)
		return item, err
	}
}

func zpaSDKStringGet[T any](
	client sdkClient,
	call func(context.Context, *zsdk.Service, string) (*T, *http.Response, error),
) func(context.Context, string) (*T, error) {
	return func(ctx context.Context, id string) (*T, error) {
		id = strings.TrimSpace(id)
		if id == "" {
			return nil, fmt.Errorf("%w: empty", ErrInvalidResourceID)
		}
		service, cleanup, err := client.productService(ctx, resources.ProductZPA)
		if err != nil {
			return nil, err
		}
		defer cleanup()
		item, _, err := call(ctx, service, id)
		return item, err
	}
}

func intIDGetter[T any](get func(context.Context, int) (*T, error)) func(context.Context, string) (*T, error) {
	return func(ctx context.Context, id string) (*T, error) {
		parsed, err := parsePositiveIntID(id)
		if err != nil {
			return nil, err
		}
		return get(ctx, parsed)
	}
}

func sourceRecordFromStruct(value any) resources.SourceRecord {
	return resources.NewSourceRecord(structMap(reflect.ValueOf(value)))
}

func structMap(value reflect.Value) map[string]any {
	value = dereferenceValue(value)
	if !value.IsValid() || value.Kind() != reflect.Struct {
		return nil
	}
	typ := value.Type()
	fields := make(map[string]any, value.NumField())
	for i := 0; i < value.NumField(); i++ {
		field := typ.Field(i)
		if field.PkgPath != "" {
			continue
		}
		name := sdkJSONFieldName(field)
		if name == "" || name == "-" {
			continue
		}
		fields[name] = sourceValue(value.Field(i))
	}
	return fields
}

func sourceValue(value reflect.Value) any {
	value = dereferenceValue(value)
	if !value.IsValid() {
		return nil
	}
	switch value.Kind() {
	case reflect.Struct:
		return structMap(value)
	case reflect.Slice, reflect.Array:
		out := make([]any, 0, value.Len())
		for i := 0; i < value.Len(); i++ {
			out = append(out, sourceValue(value.Index(i)))
		}
		return out
	case reflect.Map:
		out := make(map[string]any, value.Len())
		iter := value.MapRange()
		for iter.Next() {
			key := fmt.Sprint(sourceValue(iter.Key()))
			out[key] = sourceValue(iter.Value())
		}
		return out
	case reflect.Bool:
		return value.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return value.Int()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return value.Uint()
	case reflect.Float32, reflect.Float64:
		return value.Float()
	case reflect.String:
		return value.String()
	default:
		return nil
	}
}

func dereferenceValue(value reflect.Value) reflect.Value {
	for value.IsValid() && (value.Kind() == reflect.Pointer || value.Kind() == reflect.Interface) {
		if value.IsNil() {
			return reflect.Value{}
		}
		value = value.Elem()
	}
	return value
}

func sdkJSONFieldName(field reflect.StructField) string {
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

type perCallService struct {
	cfg ReaderConfig
}

func (s perCallService) service(ctx context.Context, product resources.Product) (*zsdk.Service, func(), error) {
	if s.cfg.AuthMode == AuthModeZIALegacy {
		if product != resources.ProductZIA {
			return nil, nil, fmt.Errorf("%w: %s requires OneAPI credentials", ErrMissingCredentials, product)
		}
		return s.legacyService(ctx)
	}
	if err := validateProductConfig(s.cfg, product); err != nil {
		return nil, nil, err
	}
	cfg := newSDKConfiguration(ctx, s.cfg)
	// Do not replace this with zsdk.NewConfiguration. That SDK constructor
	// reads ZSCALER_* environment variables and ~/.zscaler/zscaler.yaml before
	// setters run. This adapter must only use explicit ZSCALERCTL_* config.
	service, err := zsdk.NewOneAPIClient(cfg)
	if err != nil {
		return nil, nil, err
	}
	cleanup := func() {
		if service.Client != nil {
			service.Client.Close()
		}
	}
	return service, cleanup, nil
}

func (s perCallService) legacyService(ctx context.Context) (*zsdk.Service, func(), error) {
	ziaCfg, err := newLegacyZIAConfiguration(ctx, s.cfg)
	if err != nil {
		return nil, nil, err
	}
	legacyClient, err := newLegacyZIAClient(ziaCfg)
	if err != nil {
		return nil, nil, err
	}
	cfg := &zsdk.Configuration{
		Logger:          sdklogger.NewNopLogger(),
		DefaultHeader:   make(map[string]string),
		UserAgent:       "zscalerctl zscaler-sdk-go/v3",
		Context:         effectiveContext(ctx),
		CacheManager:    sdkcache.NewNopCache(),
		UseLegacyClient: true,
		LegacyClient: &zsdk.LegacyClient{
			ZiaClient: legacyClient,
		},
	}
	service, err := zsdk.NewOneAPIClient(cfg)
	if err != nil {
		legacyClient.Close()
		return nil, nil, err
	}
	cleanup := func() {
		if service.Client != nil {
			service.Client.Close()
		}
		// zia.Client.Close in zscaler-sdk-go v3.8.37 locks the client and then
		// calls Logout, whose request path locks the same mutex again. After a
		// successful legacy request this deadlocks the process during cleanup.
		// Do not call it here; this CLI process exits after the command.
	}
	return service, cleanup, nil
}

type fixedService struct {
	sdkService *zsdk.Service
}

func (s fixedService) service(ctx context.Context, _ resources.Product) (*zsdk.Service, func(), error) {
	if err := effectiveContext(ctx).Err(); err != nil {
		return nil, nil, err
	}
	if s.sdkService == nil {
		return nil, nil, errors.New("missing zscaler sdk service")
	}
	// The dump session has already authenticated the shared OneAPI service.
	return s.sdkService, func() {}, nil
}

func newLegacyZIAClient(cfg *sdkzia.Configuration) (*sdkzia.Client, error) {
	restore := suppressSDKLogEnv()
	defer restore()
	return sdkzia.NewClient(cfg)
}

// Source-record construction follows one rule: use the reflection helpers
// (jsonSourceRecord / structSourceRecord) when every SDK field maps 1:1 to an
// output key and the catalog does the dropping (the ZPA/ZCC convention, where
// secretField in the catalog removes fields). Use a hand-written
// xSourceRecord mapper when the shape must be reshaped, keys renamed, or fields
// omitted before they ever reach the projector (the common ZIA convention).
//
// jsonSourceRecord marshals via JSON tags into a flat map of output keys.
func jsonSourceRecord[T any](item T) resources.SourceRecord {
	fields := map[string]any{}
	body, err := json.Marshal(item)
	if err != nil {
		return resources.NewSourceRecord(fields)
	}
	if err := json.Unmarshal(body, &fields); err != nil {
		return resources.NewSourceRecord(map[string]any{})
	}
	return resources.NewSourceRecord(fields)
}

func newSDKConfiguration(ctx context.Context, cfg ReaderConfig) *zsdk.Configuration {
	timeout := effectiveTimeout(cfg.Timeout)
	httpClient := &http.Client{
		Timeout:   timeout,
		Transport: directTransport(cfg.Proxy),
	}
	sdkCfg := &zsdk.Configuration{
		Logger:        sdklogger.NewNopLogger(),
		HTTPClient:    httpClient,
		ZIAHTTPClient: httpClient,
		ZPAHTTPClient: httpClient,
		ZTWHTTPClient: httpClient,
		ZCCHTTPClient: httpClient,
		ZDXHTTPClient: httpClient,
		DefaultHeader: make(map[string]string),
		UserAgent:     "zscalerctl zscaler-sdk-go/v3",
		Context:       effectiveContext(ctx),
		CacheManager:  sdkcache.NewNopCache(),
	}
	sdkCfg.Zscaler.Client.ClientID = cfg.ClientID.Reveal()
	sdkCfg.Zscaler.Client.ClientSecret = cfg.ClientSecret.Reveal()
	sdkCfg.Zscaler.Client.VanityDomain = cfg.VanityDomain
	sdkCfg.Zscaler.Client.Cloud = cfg.Cloud
	sdkCfg.Zscaler.Client.CustomerID = strings.TrimSpace(cfg.ZPACustomerID)
	sdkCfg.Zscaler.Client.MicrotenantID = strings.TrimSpace(cfg.ZPAMicrotenantID)
	sdkCfg.Zscaler.Client.RequestTimeout = timeout
	sdkCfg.Zscaler.Client.RateLimit.MaxRetries = 2
	sdkCfg.Zscaler.Client.RateLimit.RetryWaitMin = time.Second
	sdkCfg.Zscaler.Client.RateLimit.RetryWaitMax = 3 * time.Second
	sdkCfg.Zscaler.Client.RateLimit.MaxSessionNotValidRetries = 1
	// SDK response caching remains disabled for every read path. NoCache is
	// retained in ReaderConfig so future cache support has to make a deliberate
	// compatibility decision instead of silently changing current behavior.
	sdkCfg.Zscaler.Client.Cache.Enabled = false
	sdkCfg.Zscaler.Client.AuthToken = &zsdk.AuthToken{}
	return sdkCfg
}

func newLegacyZIAConfiguration(ctx context.Context, cfg ReaderConfig) (*sdkzia.Configuration, error) {
	timeout := effectiveTimeout(cfg.Timeout)
	baseURL, err := legacyZIABaseURL(cfg.ZIALegacy.Cloud)
	if err != nil {
		return nil, err
	}
	httpClient := &http.Client{
		Timeout:   timeout,
		Transport: directTransport(cfg.Proxy),
	}
	ziaCfg := &sdkzia.Configuration{
		Logger:        sdklogger.NewNopLogger(),
		HTTPClient:    httpClient,
		BaseURL:       baseURL,
		DefaultHeader: make(map[string]string),
		UserAgent:     "zscalerctl zscaler-sdk-go/v3",
		Context:       effectiveContext(ctx),
		CacheManager:  sdkcache.NewNopCache(),
	}
	ziaCfg.ZIA.Client.ZIAUsername = cfg.ZIALegacy.Username.Reveal()
	ziaCfg.ZIA.Client.ZIAPassword = cfg.ZIALegacy.Password.Reveal()
	ziaCfg.ZIA.Client.ZIAApiKey = cfg.ZIALegacy.APIKey.Reveal()
	ziaCfg.ZIA.Client.ZIACloud = cfg.ZIALegacy.Cloud
	ziaCfg.ZIA.Client.RequestTimeout = timeout
	ziaCfg.ZIA.Client.RateLimit.MaxRetries = 2
	ziaCfg.ZIA.Client.RateLimit.RetryWaitMin = time.Second
	ziaCfg.ZIA.Client.RateLimit.RetryWaitMax = 3 * time.Second
	ziaCfg.ZIA.Client.Cache.Enabled = false
	return ziaCfg, nil
}

func legacyZIABaseURL(cloud string) (*url.URL, error) {
	cloud = strings.TrimSpace(cloud)
	if cloud == "" {
		return nil, fmt.Errorf("%w: ZSCALERCTL_ZIA_CLOUD is required", ErrMissingCredentials)
	}
	hostPrefix := "zsapi"
	if strings.EqualFold(cloud, "zspreview") {
		hostPrefix = "admin"
	}
	baseURL, err := url.Parse(fmt.Sprintf("https://%s.%s.net", hostPrefix, cloud))
	if err != nil {
		return nil, fmt.Errorf("parse ZIA legacy cloud: %w", err)
	}
	return baseURL, nil
}

func effectiveContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func directTransport(proxy ProxyConfig) http.RoundTripper {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = proxyFunc(proxy)
	return transport
}

func proxyFunc(proxy ProxyConfig) func(*http.Request) (*url.URL, error) {
	if proxy.FromEnvironment {
		return http.ProxyFromEnvironment
	}
	proxyURL := strings.TrimSpace(proxy.URL)
	if proxyURL == "" {
		return nil
	}
	parsed, _ := url.Parse(proxyURL)
	return http.ProxyURL(parsed)
}

func validateReaderConfig(cfg ReaderConfig) error {
	if err := validateProxyConfig(cfg.Proxy); err != nil {
		return err
	}
	switch effectiveAuthMode(cfg.AuthMode) {
	case AuthModeZIALegacy:
		switch {
		case !cfg.ZIALegacy.Username.IsSet():
			return fmt.Errorf("%w: ZSCALERCTL_ZIA_USERNAME is required", ErrMissingCredentials)
		case !cfg.ZIALegacy.Password.IsSet():
			return fmt.Errorf("%w: ZSCALERCTL_ZIA_PASSWORD is required", ErrMissingCredentials)
		case !cfg.ZIALegacy.APIKey.IsSet():
			return fmt.Errorf("%w: ZSCALERCTL_ZIA_API_KEY is required", ErrMissingCredentials)
		case strings.TrimSpace(cfg.ZIALegacy.Cloud) == "":
			return fmt.Errorf("%w: ZSCALERCTL_ZIA_CLOUD is required", ErrMissingCredentials)
		default:
			return nil
		}
	case AuthModeOneAPI:
		switch {
		case !cfg.ClientID.IsSet():
			return fmt.Errorf("%w: ZSCALERCTL_CLIENT_ID is required", ErrMissingCredentials)
		case !cfg.ClientSecret.IsSet():
			return fmt.Errorf("%w: ZSCALERCTL_CLIENT_SECRET is required", ErrMissingCredentials)
		case cfg.VanityDomain == "":
			return fmt.Errorf("%w: ZSCALERCTL_VANITY_DOMAIN is required", ErrMissingCredentials)
		default:
			return nil
		}
	default:
		return fmt.Errorf("%w: unsupported auth mode %q", ErrMissingCredentials, cfg.AuthMode)
	}
}

func validateProductConfig(cfg ReaderConfig, product resources.Product) error {
	if effectiveAuthMode(cfg.AuthMode) != AuthModeOneAPI {
		return nil
	}
	if product == resources.ProductZPA && strings.TrimSpace(cfg.ZPACustomerID) == "" {
		return fmt.Errorf("%w: ZSCALERCTL_ZPA_CUSTOMER_ID is required for ZPA resources", ErrMissingCredentials)
	}
	return nil
}

func validateProxyConfig(proxy ProxyConfig) error {
	if strings.TrimSpace(proxy.URL) != "" && proxy.FromEnvironment {
		return fmt.Errorf("%w: set only one of ZSCALERCTL_PROXY_URL or ZSCALERCTL_PROXY_FROM_ENV", ErrInvalidProxyConfig)
	}
	if strings.TrimSpace(proxy.URL) == "" {
		return nil
	}
	parsed, err := url.Parse(strings.TrimSpace(proxy.URL))
	if err != nil {
		return fmt.Errorf("%w: parse ZSCALERCTL_PROXY_URL: %w", ErrInvalidProxyConfig, err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("%w: ZSCALERCTL_PROXY_URL must include scheme and host", ErrInvalidProxyConfig)
	}
	switch parsed.Scheme {
	case "http", "https", "socks5":
		return nil
	default:
		return fmt.Errorf("%w: ZSCALERCTL_PROXY_URL unsupported scheme %q", ErrInvalidProxyConfig, parsed.Scheme)
	}
}

func effectiveAuthMode(mode AuthMode) AuthMode {
	if mode == "" {
		return AuthModeOneAPI
	}
	return mode
}

func effectiveTimeout(timeout time.Duration) time.Duration {
	if timeout <= 0 {
		return defaultTimeout
	}
	return timeout
}

func suppressSDKLogEnv() func() {
	keys := []string{"ZSCALER_SDK_LOG", "ZSCALER_SDK_VERBOSE"}
	previous := make(map[string]string, len(keys))
	present := make(map[string]bool, len(keys))
	for _, key := range keys {
		value, ok := os.LookupEnv(key)
		if ok {
			previous[key] = value
			present[key] = true
		}
		_ = os.Unsetenv(key)
	}
	return func() {
		for _, key := range keys {
			if present[key] {
				_ = os.Setenv(key, previous[key])
				continue
			}
			_ = os.Unsetenv(key)
		}
	}
}

func locationSourceRecord(location locationmanagement.Locations) resources.SourceRecord {
	fields := map[string]any{
		"id":          location.ID,
		"name":        location.Name,
		"description": location.Description,
	}
	if len(location.IPAddresses) > 0 {
		fields["ipAddresses"] = append([]string(nil), location.IPAddresses...)
	}
	if len(location.VPNCredentials) > 0 {
		fields["vpnCredentials"] = vpnCredentialsSource(location.VPNCredentials)
	}
	return resources.NewSourceRecord(fields)
}

func locationGroupSourceRecord(group locationgroups.LocationGroup) resources.SourceRecord {
	fields := map[string]any{
		"id":          group.ID,
		"name":        group.Name,
		"deleted":     group.Deleted,
		"groupType":   group.GroupType,
		"comments":    group.Comments,
		"lastModTime": group.LastModTime,
		"predefined":  group.Predefined,
	}
	if group.DynamicLocationGroupCriteria != nil {
		fields["dynamicLocationGroupCriteria"] = dynamicLocationGroupCriteriaSource(group.DynamicLocationGroupCriteria)
	}
	if len(group.Locations) > 0 {
		fields["locations"] = idNameExtensionsSliceSource(group.Locations)
	}
	if group.LastModUser != nil {
		fields["lastModUser"] = locationGroupLastModUserSource(group.LastModUser)
	}
	return resources.NewSourceRecord(fields)
}

func ruleLabelSourceRecord(label rulelabels.RuleLabels) resources.SourceRecord {
	fields := map[string]any{
		"id":                  label.ID,
		"name":                label.Name,
		"description":         label.Description,
		"lastModifiedTime":    label.LastModifiedTime,
		"referencedRuleCount": label.ReferencedRuleCount,
	}
	if label.CreatedBy != nil {
		fields["createdBy"] = idNameExtensionsSource(label.CreatedBy)
	}
	if label.LastModifiedBy != nil {
		fields["lastModifiedBy"] = idNameExtensionsSource(label.LastModifiedBy)
	}
	return resources.NewSourceRecord(fields)
}

func authSettingsSourceRecord(settings authsettings.AuthenticationSettings) resources.SourceRecord {
	return resources.NewSourceRecord(map[string]any{
		"orgAuthType":                       settings.OrgAuthType,
		"oneTimeAuth":                       settings.OneTimeAuth,
		"samlEnabled":                       settings.SamlEnabled,
		"kerberosEnabled":                   settings.KerberosEnabled,
		"kerberosPwd":                       settings.KerberosPwd,
		"authFrequency":                     settings.AuthFrequency,
		"authCustomFrequency":               settings.AuthCustomFrequency,
		"passwordStrength":                  settings.PasswordStrength,
		"passwordExpiry":                    settings.PasswordExpiry,
		"lastSyncStartTime":                 settings.LastSyncStartTime,
		"lastSyncEndTime":                   settings.LastSyncEndTime,
		"mobileAdminSamlIdpEnabled":         settings.MobileAdminSamlIdpEnabled,
		"autoProvision":                     settings.AutoProvision,
		"directorySyncMigrateToScimEnabled": settings.DirectorySyncMigrateToScimEnabled,
	})
}

func staticIPSourceRecord(staticIP staticips.StaticIP) resources.SourceRecord {
	fields := map[string]any{
		"id":                   staticIP.ID,
		"ipAddress":            staticIP.IpAddress,
		"geoOverride":          staticIP.GeoOverride,
		"latitude":             staticIP.Latitude,
		"longitude":            staticIP.Longitude,
		"routableIP":           staticIP.RoutableIP,
		"lastModificationTime": staticIP.LastModificationTime,
		"comment":              staticIP.Comment,
	}
	if staticIP.City != nil {
		fields["city"] = staticIPCitySource(staticIP.City)
	}
	if staticIP.ManagedBy != nil {
		fields["managedBy"] = staticIPManagedBySource(staticIP.ManagedBy)
	}
	if staticIP.LastModifiedBy != nil {
		fields["lastModifiedBy"] = staticIPLastModifiedBySource(staticIP.LastModifiedBy)
	}
	return resources.NewSourceRecord(fields)
}

func greTunnelSourceRecord(tunnel gretunnels.GreTunnels) resources.SourceRecord {
	fields := map[string]any{
		"id":                   tunnel.ID,
		"sourceIp":             tunnel.SourceIP,
		"internalIpRange":      tunnel.InternalIpRange,
		"lastModificationTime": tunnel.LastModificationTime,
		"withinCountry":        boolPointerValue(tunnel.WithinCountry),
		"comment":              tunnel.Comment,
		"ipUnnumbered":         tunnel.IPUnnumbered,
		"subcloud":             tunnel.SubCloud,
	}
	if tunnel.ManagedBy != nil {
		fields["managedBy"] = greManagedBySource(tunnel.ManagedBy)
	}
	if tunnel.LastModifiedBy != nil {
		fields["lastModifiedBy"] = greLastModifiedBySource(tunnel.LastModifiedBy)
	}
	if tunnel.PrimaryDestVip != nil {
		fields["primaryDestVip"] = primaryDestVIPSource(tunnel.PrimaryDestVip)
	}
	if tunnel.SecondaryDestVip != nil {
		fields["secondaryDestVip"] = secondaryDestVIPSource(tunnel.SecondaryDestVip)
	}
	return resources.NewSourceRecord(fields)
}

func sublocationSourceRecord(location locationmanagement.Locations) resources.SourceRecord {
	fields := map[string]any{
		"id":                       location.ID,
		"name":                     location.Name,
		"parentId":                 location.ParentID,
		"description":              location.Description,
		"country":                  location.Country,
		"state":                    location.State,
		"tz":                       location.TZ,
		"profile":                  location.Profile,
		"childCount":               location.ChildCount,
		"authRequired":             location.AuthRequired,
		"basicAuthEnabled":         location.BasicAuthEnabled,
		"digestAuthEnabled":        location.DigestAuthEnabled,
		"kerberosAuth":             location.KerberosAuth,
		"sslScanEnabled":           location.SSLScanEnabled,
		"zappSSLScanEnabled":       location.ZappSSLScanEnabled,
		"xffForwardEnabled":        location.XFFForwardEnabled,
		"surrogateIP":              location.SurrogateIP,
		"ofwEnabled":               location.OFWEnabled,
		"ipsControl":               location.IPSControl,
		"aupEnabled":               location.AUPEnabled,
		"cautionEnabled":           location.CautionEnabled,
		"otherSubLocation":         location.OtherSubLocation,
		"other6SubLocation":        location.Other6SubLocation,
		"subLocScopeEnabled":       location.SubLocScopeEnabled,
		"subLocScope":              location.SubLocScope,
		"excludeFromManualGroups":  location.ExcludeFromManualGroups,
		"excludeFromDynamicGroups": location.ExcludeFromDynamicGroups,
	}
	if len(location.IPAddresses) > 0 {
		fields["ipAddresses"] = append([]string(nil), location.IPAddresses...)
	}
	if len(location.Ports) > 0 {
		fields["ports"] = append([]int(nil), location.Ports...)
	}
	if len(location.SubLocScopeValues) > 0 {
		fields["subLocScopeValues"] = append([]string(nil), location.SubLocScopeValues...)
	}
	if len(location.SubLocAccIDs) > 0 {
		fields["subLocAccIds"] = append([]string(nil), location.SubLocAccIDs...)
	}
	if len(location.VPNCredentials) > 0 {
		fields["vpnCredentials"] = vpnCredentialsSource(location.VPNCredentials)
	}
	return resources.NewSourceRecord(fields)
}

func sslInspectionRuleSourceRecord(rule sslinspection.SSLInspectionRules) resources.SourceRecord {
	fields := map[string]any{
		"id":                     rule.ID,
		"name":                   rule.Name,
		"description":            rule.Description,
		"action":                 sslInspectionActionSource(rule.Action),
		"state":                  rule.State,
		"accessControl":          rule.AccessControl,
		"order":                  rule.Order,
		"rank":                   rule.Rank,
		"roadWarriorForKerberos": rule.RoadWarriorForKerberos,
		"lastModifiedTime":       rule.LastModifiedTime,
		"defaultRule":            rule.DefaultRule,
		"predefined":             rule.Predefined,
	}
	if len(rule.URLCategories) > 0 {
		fields["urlCategories"] = append([]string(nil), rule.URLCategories...)
	}
	if len(rule.Platforms) > 0 {
		fields["platforms"] = append([]string(nil), rule.Platforms...)
	}
	if len(rule.CloudApplications) > 0 {
		fields["cloudApplications"] = append([]string(nil), rule.CloudApplications...)
	}
	if len(rule.UserAgentTypes) > 0 {
		fields["userAgentTypes"] = append([]string(nil), rule.UserAgentTypes...)
	}
	if len(rule.DeviceTrustLevels) > 0 {
		fields["deviceTrustLevels"] = append([]string(nil), rule.DeviceTrustLevels...)
	}
	if len(rule.Locations) > 0 {
		fields["locations"] = idNameExtensionsSliceSource(rule.Locations)
	}
	if len(rule.LocationGroups) > 0 {
		fields["locationGroups"] = idNameExtensionsSliceSource(rule.LocationGroups)
	}
	if len(rule.Groups) > 0 {
		fields["groups"] = idNameExtensionsSliceSource(rule.Groups)
	}
	if len(rule.Departments) > 0 {
		fields["departments"] = idNameExtensionsSliceSource(rule.Departments)
	}
	if len(rule.Users) > 0 {
		fields["users"] = idNameExtensionsSliceSource(rule.Users)
	}
	if len(rule.DeviceGroups) > 0 {
		fields["deviceGroups"] = idNameExtensionsSliceSource(rule.DeviceGroups)
	}
	if len(rule.Devices) > 0 {
		fields["devices"] = idNameExtensionsSliceSource(rule.Devices)
	}
	if rule.LastModifiedBy != nil {
		fields["lastModifiedBy"] = idNameExtensionsSource(rule.LastModifiedBy)
	}
	if len(rule.DestIpGroups) > 0 {
		fields["destIpGroups"] = idNameExtensionsSliceSource(rule.DestIpGroups)
	}
	if len(rule.SourceIPGroups) > 0 {
		fields["sourceIpGroups"] = idNameExtensionsSliceSource(rule.SourceIPGroups)
	}
	if len(rule.ProxyGateways) > 0 {
		fields["proxyGateways"] = idNameExtensionsSliceSource(rule.ProxyGateways)
	}
	if len(rule.Labels) > 0 {
		fields["labels"] = idNameExtensionsSliceSource(rule.Labels)
	}
	if len(rule.TimeWindows) > 0 {
		fields["timeWindows"] = idNameExtensionsSliceSource(rule.TimeWindows)
	}
	if len(rule.ZPAAppSegments) > 0 {
		fields["zpaAppSegments"] = zpaAppSegmentsSource(rule.ZPAAppSegments)
	}
	if len(rule.WorkloadGroups) > 0 {
		fields["workloadGroups"] = idNameSliceSource(rule.WorkloadGroups)
	}
	return resources.NewSourceRecord(fields)
}

func urlCategorySourceRecord(category urlcategories.URLCategory) resources.SourceRecord {
	fields := map[string]any{
		"id":                                   category.ID,
		"configuredName":                       category.ConfiguredName,
		"description":                          category.Description,
		"type":                                 category.Type,
		"customCategory":                       category.CustomCategory,
		"editable":                             category.Editable,
		"customUrlsCount":                      category.CustomUrlsCount,
		"customIpRangesCount":                  category.CustomIpRangesCount,
		"urlsRetainingParentCategoryCount":     category.UrlsRetainingParentCategoryCount,
		"ipRangesRetainingParentCategoryCount": category.IPRangesRetainingParentCategoryCount,
		"categoryGroup":                        category.CategoryGroup,
		"superCategory":                        category.SuperCategory,
		"urlType":                              category.UrlType,
		"val":                                  category.Val,
	}
	if len(category.Keywords) > 0 {
		fields["keywords"] = append([]string(nil), category.Keywords...)
	}
	if len(category.KeywordsRetainingParentCategory) > 0 {
		fields["keywordsRetainingParentCategory"] = append([]string(nil), category.KeywordsRetainingParentCategory...)
	}
	if len(category.Urls) > 0 {
		fields["urls"] = append([]string(nil), category.Urls...)
	}
	if len(category.DBCategorizedUrls) > 0 {
		fields["dbCategorizedUrls"] = append([]string(nil), category.DBCategorizedUrls...)
	}
	if len(category.IPRanges) > 0 {
		fields["ipRanges"] = append([]string(nil), category.IPRanges...)
	}
	if len(category.IPRangesRetainingParentCategory) > 0 {
		fields["ipRangesRetainingParentCategory"] = append([]string(nil), category.IPRangesRetainingParentCategory...)
	}
	if len(category.RegexPatterns) > 0 {
		fields["regexPatterns"] = append([]string(nil), category.RegexPatterns...)
	}
	if len(category.RegexPatternsRetainingParentCategory) > 0 {
		fields["regexPatternsRetainingParentCategory"] = append([]string(nil), category.RegexPatternsRetainingParentCategory...)
	}
	if len(category.Scopes) > 0 {
		fields["scopes"] = urlCategoryScopesSource(category.Scopes)
	}
	if category.URLKeywordCounts != nil {
		fields["urlKeywordCounts"] = urlKeywordCountsSource(category.URLKeywordCounts)
	}
	return resources.NewSourceRecord(fields)
}

func urlFilteringRuleSourceRecord(rule urlfilteringpolicies.URLFilteringRule) resources.SourceRecord {
	fields := map[string]any{
		"id":                     rule.ID,
		"name":                   rule.Name,
		"order":                  rule.Order,
		"state":                  rule.State,
		"rank":                   rule.Rank,
		"description":            rule.Description,
		"action":                 rule.Action,
		"blockOverride":          rule.BlockOverride,
		"browserEunTemplateId":   rule.BrowserEunTemplateID,
		"timeQuota":              rule.TimeQuota,
		"sizeQuota":              rule.SizeQuota,
		"validityStartTime":      rule.ValidityStartTime,
		"validityEndTime":        rule.ValidityEndTime,
		"validityTimeZoneId":     rule.ValidityTimeZoneID,
		"lastModifiedTime":       rule.LastModifiedTime,
		"enforceTimeValidity":    rule.EnforceTimeValidity,
		"ciparule":               rule.Ciparule,
		"endUserNotificationUrl": rule.EndUserNotificationURL,
		"cbiProfileId":           rule.CBIProfileID,
	}
	addStringSlice(fields, "protocols", rule.Protocols)
	addStringSlice(fields, "urlCategories", rule.URLCategories)
	addStringSlice(fields, "urlCategories2", rule.URLCategories2)
	addStringSlice(fields, "userRiskScoreLevels", rule.UserRiskScoreLevels)
	addStringSlice(fields, "userAgentTypes", rule.UserAgentTypes)
	addStringSlice(fields, "requestMethods", rule.RequestMethods)
	addStringSlice(fields, "sourceCountries", rule.SourceCountries)
	addStringSlice(fields, "deviceTrustLevels", rule.DeviceTrustLevels)
	addIDNameExtensionsSlice(fields, "deviceGroups", rule.DeviceGroups)
	addIDNameExtensionsSlice(fields, "devices", rule.Devices)
	addIDNameExtensionsPtr(fields, "lastModifiedBy", rule.LastModifiedBy)
	addIDNameExtensionsSlice(fields, "overrideUsers", rule.OverrideUsers)
	addIDNameExtensionsSlice(fields, "overrideGroups", rule.OverrideGroups)
	addIDNameExtensionsSlice(fields, "locationGroups", rule.LocationGroups)
	addIDNameExtensionsSlice(fields, "labels", rule.Labels)
	addIDNameExtensionsSlice(fields, "locations", rule.Locations)
	addIDNameExtensionsSlice(fields, "groups", rule.Groups)
	addIDNameExtensionsSlice(fields, "departments", rule.Departments)
	addIDNameExtensionsSlice(fields, "users", rule.Users)
	addIDNameExtensionsSlice(fields, "sourceIpGroups", rule.SourceIPGroups)
	addIDNameExtensionsSlice(fields, "timeWindows", rule.TimeWindows)
	addIDNameSlice(fields, "workloadGroups", rule.WorkloadGroups)
	if rule.CBIProfile != nil {
		fields["cbiProfile"] = cbiProfileSource(rule.CBIProfile)
	}
	return resources.NewSourceRecord(fields)
}

func firewallFilteringRuleSourceRecord(rule filteringrules.FirewallFilteringRules) resources.SourceRecord {
	fields := map[string]any{
		"id":                  rule.ID,
		"name":                rule.Name,
		"order":               rule.Order,
		"rank":                rule.Rank,
		"accessControl":       rule.AccessControl,
		"enableFullLogging":   rule.EnableFullLogging,
		"action":              rule.Action,
		"state":               rule.State,
		"description":         rule.Description,
		"lastModifiedTime":    rule.LastModifiedTime,
		"excludeSrcCountries": rule.ExcludeSrcCountries,
		"defaultRule":         rule.DefaultRule,
		"predefined":          rule.Predefined,
	}
	addIDNameExtensionsPtr(fields, "lastModifiedBy", rule.LastModifiedBy)
	addStringSlice(fields, "srcIps", rule.SrcIps)
	addStringSlice(fields, "destAddresses", rule.DestAddresses)
	addStringSlice(fields, "destIpCategories", rule.DestIpCategories)
	addStringSlice(fields, "destCountries", rule.DestCountries)
	addStringSlice(fields, "sourceCountries", rule.SourceCountries)
	addStringSlice(fields, "nwApplications", rule.NwApplications)
	addStringSlice(fields, "deviceTrustLevels", rule.DeviceTrustLevels)
	addIDNameExtensionsSlice(fields, "locations", rule.Locations)
	addIDNameExtensionsSlice(fields, "locationGroups", rule.LocationsGroups)
	addIDNameExtensionsSlice(fields, "departments", rule.Departments)
	addIDNameExtensionsSlice(fields, "groups", rule.Groups)
	addIDNameExtensionsSlice(fields, "users", rule.Users)
	addIDNameExtensionsSlice(fields, "timeWindows", rule.TimeWindows)
	addIDNameExtensionsSlice(fields, "nwApplicationGroups", rule.NwApplicationGroups)
	addIDNameExtensionsSlice(fields, "appServices", rule.AppServices)
	addIDNameExtensionsSlice(fields, "appServiceGroups", rule.AppServiceGroups)
	addIDNameExtensionsSlice(fields, "labels", rule.Labels)
	addIDNameExtensionsSlice(fields, "destIpGroups", rule.DestIpGroups)
	addIDNameExtensionsSlice(fields, "nwServices", rule.NwServices)
	addIDNameExtensionsSlice(fields, "nwServiceGroups", rule.NwServiceGroups)
	addIDNameExtensionsSlice(fields, "srcIpGroups", rule.SrcIpGroups)
	addIDNameExtensionsSlice(fields, "deviceGroups", rule.DeviceGroups)
	addIDNameExtensionsSlice(fields, "devices", rule.Devices)
	addIDNameSlice(fields, "workloadGroups", rule.WorkloadGroups)
	if len(rule.ZPAAppSegments) > 0 {
		fields["zpaAppSegments"] = zpaAppSegmentsSource(rule.ZPAAppSegments)
	}
	return resources.NewSourceRecord(fields)
}

func forwardingRuleSourceRecord(rule forwardingrules.ForwardingRules) resources.SourceRecord {
	fields := map[string]any{
		"id":               rule.ID,
		"name":             rule.Name,
		"description":      rule.Description,
		"type":             rule.Type,
		"order":            rule.Order,
		"rank":             rule.Rank,
		"forwardMethod":    rule.ForwardMethod,
		"state":            rule.State,
		"lastModifiedTime": rule.LastModifiedTime,
		"zpaBrokerRule":    rule.ZPABrokerRule,
	}
	addIDNameExtensionsSlice(fields, "locations", rule.Locations)
	addIDNameExtensionsSlice(fields, "locationGroups", rule.LocationsGroups)
	addIDNameExtensionsSlice(fields, "ecGroups", rule.ECGroups)
	addIDNameExtensionsSlice(fields, "departments", rule.Departments)
	addIDNameExtensionsSlice(fields, "groups", rule.Groups)
	addIDNameExtensionsSlice(fields, "users", rule.Users)
	addIDNameExtensionsPtr(fields, "lastModifiedBy", rule.LastModifiedBy)
	addStringSlice(fields, "srcIps", rule.SrcIps)
	addIDNameExtensionsSlice(fields, "srcIpGroups", rule.SrcIpGroups)
	addIDNameExtensionsSlice(fields, "srcIpv6Groups", rule.SrcIpv6Groups)
	addStringSlice(fields, "destAddresses", rule.DestAddresses)
	addStringSlice(fields, "destIpCategories", rule.DestIpCategories)
	addStringSlice(fields, "resCategories", rule.ResCategories)
	addStringSlice(fields, "destCountries", rule.DestCountries)
	addIDNameExtensionsSlice(fields, "destIpGroups", rule.DestIpGroups)
	addIDNameExtensionsSlice(fields, "destIpv6Groups", rule.DestIpv6Groups)
	addIDNameExtensionsSlice(fields, "nwServices", rule.NwServices)
	addIDNameExtensionsSlice(fields, "nwServiceGroups", rule.NwServiceGroups)
	addIDNameExtensionsSlice(fields, "labels", rule.Labels)
	addIDNameExtensionsSlice(fields, "nwApplicationGroups", rule.NwApplicationGroups)
	addIDNameExtensionsSlice(fields, "appServiceGroups", rule.AppServiceGroups)
	addIDNamePtr(fields, "proxyGateway", rule.ProxyGateway)
	addIDNamePtr(fields, "dedicatedIPGateway", rule.DedicatedIPGateway)
	addIDNamePtr(fields, "zpaGateway", rule.ZPAGateway)
	if len(rule.ZPAAppSegments) > 0 {
		fields["zpaAppSegments"] = zpaAppSegmentsSource(rule.ZPAAppSegments)
	}
	if len(rule.ZPAApplicationSegments) > 0 {
		fields["zpaApplicationSegments"] = forwardingZPAApplicationSegmentsSource(rule.ZPAApplicationSegments)
	}
	if len(rule.ZPAApplicationSegmentGroups) > 0 {
		fields["zpaApplicationSegmentGroups"] = forwardingZPAApplicationSegmentGroupsSource(rule.ZPAApplicationSegmentGroups)
	}
	addIDNameExtensionsSlice(fields, "deviceGroups", rule.DeviceGroups)
	return resources.NewSourceRecord(fields)
}

func ipSourceGroupSourceRecord(group ipsourcegroups.IPSourceGroups) resources.SourceRecord {
	fields := map[string]any{
		"id":            group.ID,
		"name":          group.Name,
		"description":   group.Description,
		"isNonEditable": group.IsNonEditable,
	}
	addStringSlice(fields, "ipAddresses", group.IPAddresses)
	return resources.NewSourceRecord(fields)
}

func ipDestinationGroupSourceRecord(group ipdestinationgroups.IPDestinationGroups) resources.SourceRecord {
	fields := map[string]any{
		"id":            group.ID,
		"name":          group.Name,
		"description":   group.Description,
		"type":          group.Type,
		"isNonEditable": group.IsNonEditable,
	}
	addStringSlice(fields, "addresses", group.Addresses)
	addStringSlice(fields, "ipCategories", group.IPCategories)
	addStringSlice(fields, "countries", group.Countries)
	return resources.NewSourceRecord(fields)
}

func networkServiceSourceRecord(service networkservices.NetworkServices) resources.SourceRecord {
	fields := map[string]any{
		"id":            service.ID,
		"name":          service.Name,
		"tag":           service.Tag,
		"type":          service.Type,
		"description":   service.Description,
		"protocol":      service.Protocol,
		"isNameL10nTag": service.IsNameL10nTag,
	}
	addNetworkPorts(fields, "srcTcpPorts", service.SrcTCPPorts)
	addNetworkPorts(fields, "destTcpPorts", service.DestTCPPorts)
	addNetworkPorts(fields, "srcUdpPorts", service.SrcUDPPorts)
	addNetworkPorts(fields, "destUdpPorts", service.DestUDPPorts)
	return resources.NewSourceRecord(fields)
}

func networkServiceGroupSourceRecord(group networkservicegroups.NetworkServiceGroups) resources.SourceRecord {
	fields := map[string]any{
		"id":          group.ID,
		"name":        group.Name,
		"description": group.Description,
	}
	if len(group.Services) > 0 {
		fields["services"] = networkServiceRefsSource(group.Services)
	}
	return resources.NewSourceRecord(fields)
}

func networkApplicationSourceRecord(app networkapplications.NetworkApplications) resources.SourceRecord {
	return resources.NewSourceRecord(map[string]any{
		"id":             app.ID,
		"parentCategory": app.ParentCategory,
		"description":    app.Description,
		"deprecated":     app.Deprecated,
	})
}

func applicationServiceSourceRecord(service applicationservices.ApplicationServicesLite) resources.SourceRecord {
	return resources.NewSourceRecord(map[string]any{
		"id":          service.ID,
		"name":        service.Name,
		"nameL10nTag": service.NameL10nTag,
	})
}

func applicationServiceGroupSourceRecord(group appservicegroups.ApplicationServicesGroupLite) resources.SourceRecord {
	return resources.NewSourceRecord(map[string]any{
		"id":          group.ID,
		"name":        group.Name,
		"nameL10nTag": group.NameL10nTag,
	})
}

func networkApplicationGroupSourceRecord(group networkapplicationgroups.NetworkApplicationGroups) resources.SourceRecord {
	fields := map[string]any{
		"id":          group.ID,
		"name":        group.Name,
		"description": group.Description,
	}
	addStringSlice(fields, "networkApplications", group.NetworkApplications)
	return resources.NewSourceRecord(fields)
}

func timeWindowSourceRecord(window timewindow.TimeWindow) resources.SourceRecord {
	fields := map[string]any{
		"id":        window.ID,
		"name":      window.Name,
		"startTime": window.StartTime,
		"endTime":   window.EndTime,
	}
	addStringSlice(fields, "dayOfWeek", window.DayOfWeek)
	return resources.NewSourceRecord(fields)
}

func proxySourceRecord(proxy proxies.Proxies) resources.SourceRecord {
	fields := map[string]any{
		"id":                    proxy.ID,
		"name":                  proxy.Name,
		"type":                  proxy.Type,
		"address":               proxy.Address,
		"port":                  proxy.Port,
		"description":           proxy.Description,
		"insertXauHeader":       proxy.InsertXauHeader,
		"base64EncodeXauHeader": proxy.Base64EncodeXauHeader,
		"lastModifiedTime":      proxy.LastModifiedTime,
	}
	addIDNameExternalIDPtr(fields, "cert", proxy.Cert)
	addIDNameExternalIDPtr(fields, "lastModifiedBy", proxy.LastModifiedBy)
	return resources.NewSourceRecord(fields)
}

func proxyGatewaySourceRecord(gateway proxygateways.ProxyGateways) resources.SourceRecord {
	fields := map[string]any{
		"id":               gateway.ID,
		"name":             gateway.Name,
		"description":      gateway.Description,
		"failClosed":       gateway.FailClosed,
		"type":             gateway.Type,
		"lastModifiedTime": gateway.LastModifiedTime,
	}
	addIDNameExternalIDPtr(fields, "primaryProxy", gateway.PrimaryProxy)
	addIDNameExternalIDPtr(fields, "secondaryProxy", gateway.SecondaryProxy)
	addIDNameExtensionsPtr(fields, "lastModifiedBy", gateway.LastModifiedBy)
	return resources.NewSourceRecord(fields)
}

func dedicatedIPGatewaySourceRecord(gateway proxies.DedicatedIPGateways) resources.SourceRecord {
	fields := map[string]any{
		"id":               gateway.Id,
		"name":             gateway.Name,
		"description":      gateway.Description,
		"createTime":       gateway.CreateTime,
		"lastModifiedTime": gateway.LastModifiedTime,
		"default":          gateway.Default,
	}
	addIDNameExtensionsPtr(fields, "primaryDataCenter", gateway.PrimaryDataCenter)
	addIDNameExtensionsPtr(fields, "secondaryDataCenter", gateway.SecondaryDataCenter)
	addIDNameExtensionsPtr(fields, "lastModifiedBy", gateway.LastModifiedBy)
	return resources.NewSourceRecord(fields)
}

func timeIntervalSourceRecord(interval timeintervals.TimeInterval) resources.SourceRecord {
	fields := map[string]any{
		"id":        interval.ID,
		"name":      interval.Name,
		"startTime": interval.StartTime,
		"endTime":   interval.EndTime,
	}
	addStringSlice(fields, "daysOfWeek", interval.DaysOfWeek)
	return resources.NewSourceRecord(fields)
}

func bandwidthClassSourceRecord(class bandwidthclasses.BandwidthClasses) resources.SourceRecord {
	fields := map[string]any{
		"id":            class.ID,
		"isNameL10nTag": class.IsNameL10nTag,
		"name":          class.Name,
		"getfileSize":   class.GetfileSize,
		"fileSize":      class.FileSize,
		"type":          class.Type,
	}
	addStringSlice(fields, "webApplications", class.WebApplications)
	addStringSlice(fields, "urls", class.Urls)
	addStringSlice(fields, "applicationServiceGroups", class.ApplicationServiceGroups)
	addStringSlice(fields, "networkApplications", class.NetworkApplications)
	addStringSlice(fields, "networkServices", class.NetworkServices)
	addStringSlice(fields, "urlCategories", class.UrlCategories)
	addStringSlice(fields, "applications", class.Applications)
	return resources.NewSourceRecord(fields)
}

func bandwidthControlRuleSourceRecord(rule bandwidthcontrolrules.BandwidthControlRules) resources.SourceRecord {
	fields := map[string]any{
		"id":               rule.ID,
		"name":             rule.Name,
		"order":            rule.Order,
		"state":            rule.State,
		"description":      rule.Description,
		"maxBandwidth":     rule.MaxBandwidth,
		"minBandwidth":     rule.MinBandwidth,
		"rank":             rule.Rank,
		"lastModifiedTime": rule.LastModifiedTime,
		"accessControl":    rule.AccessControl,
		"defaultRule":      rule.DefaultRule,
	}
	addStringSlice(fields, "protocols", rule.Protocols)
	addStringSlice(fields, "deviceTrustLevels", rule.DeviceTrustLevels)
	addIDNameExtensionsPtr(fields, "lastModifiedBy", rule.LastModifiedBy)
	addIDNameExtensionsSlice(fields, "bandwidthClasses", rule.BandwidthClasses)
	addIDNameExtensionsSlice(fields, "locationGroups", rule.LocationGroups)
	addIDNameExtensionsSlice(fields, "labels", rule.Labels)
	addIDNameExtensionsSlice(fields, "devices", rule.Devices)
	addIDNameExtensionsSlice(fields, "deviceGroups", rule.DeviceGroups)
	addIDNameExtensionsSlice(fields, "locations", rule.Locations)
	addIDNameExtensionsSlice(fields, "timeWindows", rule.TimeWindows)
	return resources.NewSourceRecord(fields)
}

func ipsPolicySourceRecord(rule ipspolicies.FirewallIPSRules) resources.SourceRecord {
	fields := map[string]any{
		"id":                rule.ID,
		"name":              rule.Name,
		"order":             rule.Order,
		"rank":              rule.Rank,
		"accessControl":     rule.AccessControl,
		"enableFullLogging": rule.EnableFullLogging,
		"action":            rule.Action,
		"state":             rule.State,
		"description":       rule.Description,
		"lastModifiedTime":  rule.LastModifiedTime,
		"defaultRule":       rule.DefaultRule,
		"capturePCAP":       rule.CapturePCAP,
		"predefined":        rule.Predefined,
		"isEunEnabled":      rule.IsEUNEnabled,
		"eunTemplateId":     rule.EUNTemplateID,
	}
	addIDNameExtensionsPtr(fields, "lastModifiedBy", rule.LastModifiedBy)
	addStringSlice(fields, "srcIps", rule.SrcIps)
	addStringSlice(fields, "destAddresses", rule.DestAddresses)
	addStringSlice(fields, "destIpCategories", rule.DestIpCategories)
	addStringSlice(fields, "destCountries", rule.DestCountries)
	addStringSlice(fields, "sourceCountries", rule.SourceCountries)
	addStringSlice(fields, "resCategories", rule.ResCategories)
	addIDNameExtensionsSlice(fields, "locations", rule.Locations)
	addIDNameExtensionsSlice(fields, "locationGroups", rule.LocationsGroups)
	addIDNameExtensionsSlice(fields, "departments", rule.Departments)
	addIDNameExtensionsSlice(fields, "groups", rule.Groups)
	addIDNameExtensionsSlice(fields, "users", rule.Users)
	addIDNameExtensionsSlice(fields, "timeWindows", rule.TimeWindows)
	addIDNameExtensionsSlice(fields, "labels", rule.Labels)
	addIDNameExtensionsSlice(fields, "destIpGroups", rule.DestIpGroups)
	addIDNameExtensionsSlice(fields, "destIpv6Groups", rule.DestIpv6Groups)
	addIDNameExtensionsSlice(fields, "nwServices", rule.NwServices)
	addIDNameExtensionsSlice(fields, "nwServiceGroups", rule.NwServiceGroups)
	addIDNameExtensionsSlice(fields, "srcIpGroups", rule.SrcIpGroups)
	addIDNameExtensionsSlice(fields, "srcIpv6Groups", rule.SrcIpv6Groups)
	addIDNameExtensionsSlice(fields, "deviceGroups", rule.DeviceGroups)
	addIDNameExtensionsSlice(fields, "devices", rule.Devices)
	addIDNameExtensionsSlice(fields, "threatCategories", rule.ThreatCategories)
	if len(rule.ZPAAppSegments) > 0 {
		fields["zpaAppSegments"] = zpaAppSegmentsSource(rule.ZPAAppSegments)
	}
	return resources.NewSourceRecord(fields)
}

func eusaStatusSourceRecord(status activation.ZiaEusaStatus) resources.SourceRecord {
	fields := map[string]any{
		"id":             status.ID,
		"acceptedStatus": status.AcceptedStatus,
	}
	addIDNameExtensionsPtr(fields, "version", status.Version)
	return resources.NewSourceRecord(fields)
}

func intermediateCACertificateSourceRecord(cert intermediatecacertificates.IntermediateCACertificate) resources.SourceRecord {
	return resources.NewSourceRecord(map[string]any{
		"id":                         cert.ID,
		"name":                       cert.Name,
		"description":                cert.Description,
		"type":                       cert.Type,
		"region":                     cert.Region,
		"status":                     cert.Status,
		"defaultCertificate":         cert.DefaultCertificate,
		"certStartDate":              cert.CertStartDate,
		"certExpDate":                cert.CertExpDate,
		"currentState":               cert.CurrentState,
		"publicKey":                  cert.PublicKey,
		"keyGenerationTime":          cert.KeyGenerationTime,
		"hsmAttestationVerifiedTime": cert.HSMAttestationVerifiedTime,
		"csrFileName":                cert.CSRFileName,
		"csrGenerationTime":          cert.CSRGenerationTime,
	})
}

func dnsGatewaySourceRecord(gateway dnsgateways.DNSGateways) resources.SourceRecord {
	fields := map[string]any{
		"id":                gateway.ID,
		"name":              gateway.Name,
		"dnsGatewayType":    gateway.DnsGatewayType,
		"primaryIpOrFqdn":   gateway.PrimaryIpOrFqdn,
		"secondaryIpOrFqdn": gateway.SecondaryIpOrFqdn,
		"failureBehavior":   gateway.FailureBehavior,
		"lastModifiedTime":  gateway.LastModifiedTime,
		"autoCreated":       gateway.AutoCreated,
		"natZtrGateway":     gateway.NatZtrGateway,
	}
	addIntSlice(fields, "primaryPorts", gateway.PrimaryPorts)
	addIntSlice(fields, "secondaryPorts", gateway.SecondaryPorts)
	addStringSlice(fields, "protocols", gateway.Protocols)
	addStringSlice(fields, "dnsGatewayProtocols", gateway.DnsGatewayProtocols)
	addIDNameExtensionsPtr(fields, "lastModifiedBy", gateway.LastModifiedBy)
	return resources.NewSourceRecord(fields)
}

func natControlRuleSourceRecord(rule natcontrol.NatControlPolicies) resources.SourceRecord {
	fields := map[string]any{
		"accessControl":       rule.AccessControl,
		"id":                  rule.ID,
		"name":                rule.Name,
		"order":               rule.Order,
		"rank":                rule.Rank,
		"description":         rule.Description,
		"state":               rule.State,
		"redirectFqdn":        rule.RedirectFqdn,
		"redirectIp":          rule.RedirectIp,
		"redirectPort":        rule.RedirectPort,
		"lastModifiedTime":    rule.LastModifiedTime,
		"trustedResolverRule": rule.TrustedResolverRule,
		"enableFullLogging":   rule.EnableFullLogging,
		"predefined":          rule.Predefined,
		"defaultRule":         rule.DefaultRule,
	}
	addStringSlice(fields, "destAddresses", rule.DestAddresses)
	addStringSlice(fields, "srcIps", rule.SrcIps)
	addStringSlice(fields, "destCountries", rule.DestCountries)
	addStringSlice(fields, "destIpCategories", rule.DestIpCategories)
	addStringSlice(fields, "resCategories", rule.ResCategories)
	addIDNameExtensionsSlice(fields, "locations", rule.Locations)
	addIDNameExtensionsSlice(fields, "locationGroups", rule.LocationGroups)
	addIDNameExtensionsSlice(fields, "groups", rule.Groups)
	addIDNameExtensionsSlice(fields, "departments", rule.Departments)
	addIDNameExtensionsSlice(fields, "users", rule.Users)
	addIDNameExtensionsSlice(fields, "timeWindows", rule.TimeWindows)
	addIDNameExtensionsSlice(fields, "srcIpGroups", rule.SrcIpGroups)
	addIDNameExtensionsSlice(fields, "srcIpv6Groups", rule.SrcIpv6Groups)
	addIDNameExtensionsSlice(fields, "destIpGroups", rule.DestIpGroups)
	addIDNameExtensionsSlice(fields, "destIpv6Groups", rule.DestIpv6Groups)
	addIDNameExtensionsSlice(fields, "nwServices", rule.NwServices)
	addIDNameExtensionsSlice(fields, "nwServiceGroups", rule.NwServiceGroups)
	addIDNameExtensionsPtr(fields, "lastModifiedBy", rule.LastModifiedBy)
	addIDNameExtensionsSlice(fields, "devices", rule.Devices)
	addIDNameExtensionsSlice(fields, "deviceGroups", rule.DeviceGroups)
	addIDNameExtensionsSlice(fields, "labels", rule.Labels)
	return resources.NewSourceRecord(fields)
}

func groupSourceRecord(group usergroups.Groups) resources.SourceRecord {
	fields := map[string]any{
		"id":              group.ID,
		"name":            group.Name,
		"idpId":           group.IdpID,
		"comments":        group.Comments,
		"isSystemDefined": group.IsSystemDefined,
	}
	return resources.NewSourceRecord(fields)
}

func departmentSourceRecord(department userdepartments.Department) resources.SourceRecord {
	fields := map[string]any{
		"id":       department.ID,
		"name":     department.Name,
		"idpId":    department.IdpID,
		"comments": department.Comments,
		"deleted":  department.Deleted,
	}
	return resources.NewSourceRecord(fields)
}

func userSourceRecord(user ziausers.Users) resources.SourceRecord {
	fields := map[string]any{
		"id":            user.ID,
		"name":          user.Name,
		"email":         user.Email,
		"comments":      user.Comments,
		"tempAuthEmail": user.TempAuthEmail,
		"authMethods":   user.AuthMethods,
		"password":      user.Password,
		"adminUser":     user.AdminUser,
		"type":          user.Type,
		"deleted":       user.Deleted,
	}
	addUserGroups(fields, "groups", user.Groups)
	if user.Department != nil {
		fields["department"] = userDepartmentSource(*user.Department)
	}
	return resources.NewSourceRecord(fields)
}

func deviceGroupSourceRecord(group devicegroups.DeviceGroups) resources.SourceRecord {
	fields := map[string]any{
		"id":          group.ID,
		"name":        group.Name,
		"groupType":   group.GroupType,
		"description": group.Description,
		"osType":      group.OSType,
		"predefined":  group.Predefined,
		"deviceNames": group.DeviceNames,
		"deviceCount": group.DeviceCount,
	}
	return resources.NewSourceRecord(fields)
}

func deviceSourceRecord(device devicegroups.Devices) resources.SourceRecord {
	fields := map[string]any{
		"id":              device.ID,
		"name":            device.Name,
		"deviceGroupType": device.DeviceGroupType,
		"deviceModel":     device.DeviceModel,
		"osType":          device.OSType,
		"osVersion":       device.OSVersion,
		"description":     device.Description,
		"ownerUserId":     device.OwnerUserId,
		"ownerName":       device.OwnerName,
		"hostName":        device.HostName,
	}
	return resources.NewSourceRecord(fields)
}

func workloadGroupSourceRecord(group workloadgroups.WorkloadGroup) resources.SourceRecord {
	fields := map[string]any{
		"id":               group.ID,
		"name":             group.Name,
		"description":      group.Description,
		"expression":       group.Expression,
		"lastModifiedTime": group.LastModifiedTime,
	}
	addIDNameExtensionsPtr(fields, "lastModifiedBy", group.LastModifiedBy)
	if len(group.WorkloadTagExpression.ExpressionContainers) > 0 {
		fields["expressionJson"] = workloadTagExpressionSource(group.WorkloadTagExpression)
	}
	return resources.NewSourceRecord(fields)
}

func alertSubscriptionSourceRecord(subscription alerts.AlertSubscriptions) resources.SourceRecord {
	fields := map[string]any{
		"id":          subscription.ID,
		"description": subscription.Description,
		"email":       subscription.Email,
		"deleted":     subscription.Deleted,
	}
	addStringSlice(fields, "pt0Severities", subscription.Pt0Severities)
	addStringSlice(fields, "secureSeverities", subscription.SecureSeverities)
	addStringSlice(fields, "manageSeverities", subscription.ManageSeverities)
	addStringSlice(fields, "complySeverities", subscription.ComplySeverities)
	addStringSlice(fields, "systemSeverities", subscription.SystemSeverities)
	return resources.NewSourceRecord(fields)
}

func cloudAppInstanceSourceRecord(instance cloudappinstances.CloudApplicationInstances) resources.SourceRecord {
	fields := map[string]any{
		"instanceId":   instance.InstanceID,
		"instanceType": instance.InstanceType,
		"instanceName": instance.InstanceName,
		"modifiedAt":   instance.ModifiedAt,
	}
	addIDNameExtensionsPtr(fields, "modifiedBy", instance.ModifiedBy)
	addCloudInstanceIdentifiers(fields, "instanceIdentifiers", instance.InstanceIdentifiers)
	return resources.NewSourceRecord(fields)
}

func tenancyRestrictionProfileSourceRecord(profile tenancyrestriction.TenancyRestrictionProfile) resources.SourceRecord {
	fields := map[string]any{
		"id":                          profile.ID,
		"name":                        profile.Name,
		"appType":                     profile.AppType,
		"description":                 profile.Description,
		"itemTypePrimary":             profile.ItemTypePrimary,
		"itemTypeSecondary":           profile.ItemTypeSecondary,
		"restrictPersonalO365Domains": profile.RestrictPersonalO365Domains,
		"allowGoogleConsumers":        profile.AllowGoogleConsumers,
		"msLoginServicesTrV2":         profile.MsLoginServicesTrV2,
		"allowGoogleVisitors":         profile.AllowGoogleVisitors,
		"allowGcpCloudStorageRead":    profile.AllowGcpCloudStorageRead,
		"lastModifiedTime":            profile.LastModifiedTime,
		"lastModifiedUserId":          profile.LastModifiedUserID,
	}
	addStringSlice(fields, "itemDataPrimary", profile.ItemDataPrimary)
	addStringSlice(fields, "itemDataSecondary", profile.ItemDataSecondary)
	addStringSlice(fields, "itemValue", profile.ItemValue)
	return resources.NewSourceRecord(fields)
}

func vzenClusterSourceRecord(cluster vzenclusters.VZENClusters) resources.SourceRecord {
	fields := map[string]any{
		"id":             cluster.ID,
		"name":           cluster.Name,
		"status":         cluster.Status,
		"ipAddress":      cluster.IpAddress,
		"subnetMask":     cluster.SubnetMask,
		"defaultGateway": cluster.DefaultGateway,
		"type":           cluster.Type,
		"ipSecEnabled":   cluster.IpSecEnabled,
	}
	addIDNameExternalIDSlice(fields, "virtualZenNodes", cluster.VirtualZenNodes)
	return resources.NewSourceRecord(fields)
}

func vzenNodeSourceRecord(node vzennodes.VZENNodes) resources.SourceRecord {
	fields := map[string]any{
		"id":                            node.ID,
		"zgatewayId":                    node.ZGatewayID,
		"name":                          node.Name,
		"status":                        node.Status,
		"inProduction":                  node.InProduction,
		"ipAddress":                     node.IPAddress,
		"subnetMask":                    node.SubnetMask,
		"defaultGateway":                node.DefaultGateway,
		"type":                          node.Type,
		"ipSecEnabled":                  node.IPSecEnabled,
		"onDemandSupportTunnelEnabled":  node.OnDemandSupportTunnelEnabled,
		"establishSupportTunnelEnabled": node.EstablishSupportTunnelEnabled,
		"loadBalancerIpAddress":         node.LoadBalancerIPAddress,
		"deploymentMode":                node.DeploymentMode,
		"clusterName":                   node.ClusterName,
		"vzenSkuType":                   node.VzenSkuType,
	}
	return resources.NewSourceRecord(fields)
}

func dlpICAPServerSourceRecord(server dlpicapservers.DLPICAPServers) resources.SourceRecord {
	return resources.NewSourceRecord(map[string]any{
		"id":     server.ID,
		"name":   server.Name,
		"url":    server.URL,
		"status": server.Status,
	})
}

func dlpIncidentReceiverSourceRecord(receiver dlpincidentreceivers.IncidentReceiverServers) resources.SourceRecord {
	return resources.NewSourceRecord(map[string]any{
		"id":     receiver.ID,
		"name":   receiver.Name,
		"url":    receiver.URL,
		"status": receiver.Status,
		"flags":  receiver.Flags,
	})
}

func dlpNotificationTemplateSourceRecord(template dlpnotificationtemplates.DlpNotificationTemplates) resources.SourceRecord {
	return resources.NewSourceRecord(map[string]any{
		"id":               template.ID,
		"name":             template.Name,
		"subject":          template.Subject,
		"attachContent":    template.AttachContent,
		"plainTextMessage": template.PlainTextMessage,
		"htmlMessage":      template.HtmlMessage,
		"tlsEnabled":       template.TLSEnabled,
	})
}

func dlpEngineSourceRecord(engine dlpengines.DLPEngines) resources.SourceRecord {
	return resources.NewSourceRecord(map[string]any{
		"id":                   engine.ID,
		"name":                 engine.Name,
		"description":          engine.Description,
		"predefinedEngineName": engine.PredefinedEngineName,
		"engineExpression":     engine.EngineExpression,
		"customDlpEngine":      engine.CustomDlpEngine,
	})
}

func dlpDictionarySourceRecord(dictionary dlpdictionaries.DlpDictionary) resources.SourceRecord {
	return resources.NewSourceRecord(map[string]any{
		"id":                                  dictionary.ID,
		"name":                                dictionary.Name,
		"description":                         dictionary.Description,
		"confidenceThreshold":                 dictionary.ConfidenceThreshold,
		"customPhraseMatchType":               dictionary.CustomPhraseMatchType,
		"nameL10nTag":                         dictionary.NameL10nTag,
		"custom":                              dictionary.Custom,
		"thresholdType":                       dictionary.ThresholdType,
		"dictionaryType":                      dictionary.DictionaryType,
		"proximity":                           dictionary.Proximity,
		"phrases":                             dictionary.Phrases,
		"patterns":                            dictionary.Patterns,
		"exactDataMatchDetails":               dictionary.EDMMatchDetails,
		"idmProfileMatchAccuracyDetails":      dictionary.IDMProfileMatchAccuracy,
		"ignoreExactMatchIdmDict":             dictionary.IgnoreExactMatchIdmDict,
		"includeBinNumbers":                   dictionary.IncludeBinNumbers,
		"binNumbers":                          dictionary.BinNumbers,
		"dictTemplateId":                      dictionary.DictTemplateId,
		"predefinedClone":                     dictionary.PredefinedClone,
		"predefinedCountActionType":           dictionary.PredefinedCountActionType,
		"proximityLengthEnabled":              dictionary.ProximityLengthEnabled,
		"proximityEnabledForCustomDictionary": dictionary.ProximityEnabledForCustomDictionary,
		"dictionaryCloningEnabled":            dictionary.DictionaryCloningEnabled,
		"customPhraseSupported":               dictionary.CustomPhraseSupported,
		"hierarchicalDictionary":              dictionary.HierarchicalDictionary,
		"hierarchicalIdentifiers":             dictionary.HierarchicalIdentifiers,
		"predefinedPhrases":                   dictionary.PredefinedPhrases,
		"thresholdAllowed":                    dictionary.ThresholdAllowed,
		"confidenceLevelForPredefinedDict":    dictionary.ConfidenceLevelForPredefinedDict,
	})
}

func dlpEDMSchemaSourceRecord(schema dlpexactdatamatch.DLPEDMSchema) resources.SourceRecord {
	fields := map[string]any{
		"schemaId":         schema.SchemaID,
		"projectName":      schema.ProjectName,
		"revision":         schema.Revision,
		"filename":         schema.Filename,
		"originalFileName": schema.OriginalFileName,
		"fileUploadStatus": schema.FileUploadStatus,
		"schemaStatus":     schema.SchemaStatus,
		"origColCount":     schema.OrigColCount,
		"lastModifiedTime": schema.LastModifiedTime,
		"cellsUsed":        schema.CellsUsed,
		"schemaActive":     schema.SchemaActive,
		"schedulePresent":  schema.SchedulePresent,
		"tokenList":        schema.TokenList,
		"schedule":         schema.Schedule,
	}
	addIDNameExtensionsPtr(fields, "edmClient", schema.EDMClient)
	addIDNameExtensionsPtr(fields, "modifiedBy", schema.ModifiedBy)
	addIDNameExtensionsPtr(fields, "createdBy", schema.CreatedBy)
	return resources.NewSourceRecord(fields)
}

func browserIsolationProfileSourceRecord(profile browserisolation.CBIProfile) resources.SourceRecord {
	return resources.NewSourceRecord(map[string]any{
		"id":             profile.ID,
		"name":           profile.Name,
		"url":            profile.URL,
		"defaultProfile": profile.DefaultProfile,
	})
}

func dlpEDMLiteSourceRecord(schema dlpedmlite.DLPEDMLite) resources.SourceRecord {
	return resources.NewSourceRecord(map[string]any{
		"schema":    dlpEDMLiteSchemaSource(schema.Schema),
		"tokenList": schema.TokenList,
	})
}

func dlpEDMLiteSchemaSource(schema dlpedmlite.SchemaIDNameExtension) map[string]any {
	fields := map[string]any{
		"id":   schema.ID,
		"name": schema.Name,
	}
	if schema.ExternalID != "" {
		fields["externalId"] = schema.ExternalID
	}
	return fields
}

func dlpIDMProfileLiteSourceRecord(profile dlpidmprofilelite.DLPIDMProfileLite) resources.SourceRecord {
	fields := map[string]any{
		"profileId":        profile.ProfileID,
		"templateName":     profile.TemplateName,
		"numDocuments":     profile.NumDocuments,
		"lastModifiedTime": profile.LastModifiedTime,
	}
	addIDNameExtensionsPtr(fields, "clientVm", profile.ClientVM)
	addIDNameExtensionsPtr(fields, "modifiedBy", profile.ModifiedBy)
	return resources.NewSourceRecord(fields)
}

func dlpIDMProfileSourceRecord(profile dlpidmprofiles.DLPIDMProfile) resources.SourceRecord {
	fields := map[string]any{
		"profileId":          profile.ProfileID,
		"profileName":        profile.ProfileName,
		"profileDesc":        profile.ProfileDesc,
		"profileType":        profile.ProfileType,
		"host":               profile.Host,
		"port":               profile.Port,
		"profileDirPath":     profile.ProfileDirPath,
		"scheduleType":       profile.ScheduleType,
		"scheduleDay":        profile.ScheduleDay,
		"scheduleDayOfMonth": profile.ScheduleDayOfMonth,
		"scheduleDayOfWeek":  profile.ScheduleDayOfWeek,
		"scheduleTime":       profile.ScheduleTime,
		"scheduleDisabled":   profile.ScheduleDisabled,
		"uploadStatus":       profile.UploadStatus,
		"userName":           profile.UserName,
		"version":            profile.Version,
		"volumeOfDocuments":  profile.VolumeOfDocuments,
		"numDocuments":       profile.NumDocuments,
		"lastModifiedTime":   profile.LastModifiedTime,
	}
	addIDNameExtensionsPtr(fields, "idmClient", profile.IDMClient)
	addIDNameExtensionsPtr(fields, "modifiedBy", profile.ModifiedBy)
	return resources.NewSourceRecord(fields)
}

func dlpWebRuleSourceRecord(rule dlpwebrules.WebDLPRules) resources.SourceRecord {
	fields := map[string]any{
		"id":                        rule.ID,
		"order":                     rule.Order,
		"accessControl":             rule.AccessControl,
		"protocols":                 rule.Protocols,
		"rank":                      rule.Rank,
		"name":                      rule.Name,
		"description":               rule.Description,
		"fileTypes":                 rule.FileTypes,
		"cloudApplications":         rule.CloudApplications,
		"minSize":                   rule.MinSize,
		"action":                    rule.Action,
		"state":                     rule.State,
		"matchOnly":                 rule.MatchOnly,
		"lastModifiedTime":          rule.LastModifiedTime,
		"withoutContentInspection":  rule.WithoutContentInspection,
		"ocrEnabled":                rule.OcrEnabled,
		"dlpDownloadScanEnabled":    rule.DLPDownloadScanEnabled,
		"zccNotificationsEnabled":   rule.ZCCNotificationsEnabled,
		"zscalerIncidentReceiver":   rule.ZscalerIncidentReceiver,
		"eunTemplateId":             rule.EUNTemplateID,
		"externalAuditorEmail":      rule.ExternalAuditorEmail,
		"auditor":                   idCustomSource(rule.Auditor),
		"notificationTemplate":      idCustomSource(rule.NotificationTemplate),
		"icapServer":                idCustomSource(rule.IcapServer),
		"receiver":                  dlpWebRuleReceiverSource(rule.Receiver),
		"severity":                  rule.Severity,
		"parentRule":                rule.ParentRule,
		"subRules":                  rule.SubRules,
		"userRiskScoreLevels":       rule.UserRiskScoreLevels,
		"dlpContentLocationsScopes": rule.DlpContentLocationsScopes,
		"inspectHttpGetEnabled":     rule.InspectHttpGetEnabled,
	}
	addIDNameExtensionsPtr(fields, "lastModifiedBy", rule.LastModifiedBy)
	addIDNameExtensionsSlice(fields, "locations", rule.Locations)
	addIDNameExtensionsSlice(fields, "locationGroups", rule.LocationGroups)
	addIDNameExtensionsSlice(fields, "groups", rule.Groups)
	addIDNameExtensionsSlice(fields, "departments", rule.Departments)
	addIDNameExtensionsSlice(fields, "users", rule.Users)
	addIDNameExtensionsSlice(fields, "urlCategories", rule.URLCategories)
	addIDNameExtensionsSlice(fields, "dlpEngines", rule.DLPEngines)
	addIDNameExtensionsSlice(fields, "timeWindows", rule.TimeWindows)
	addIDNameExtensionsSlice(fields, "labels", rule.Labels)
	addIDNameExtensionsSlice(fields, "excludedGroups", rule.ExcludedGroups)
	addIDNameExtensionsSlice(fields, "excludedDepartments", rule.ExcludedDepartments)
	addIDNameExtensionsSlice(fields, "excludedUsers", rule.ExcludedUsers)
	addIDNameExtensionsSlice(fields, "includedDomainProfiles", rule.IncludedDomainProfiles)
	addIDNameExtensionsSlice(fields, "excludedDomainProfiles", rule.ExcludedDomainProfiles)
	addIDNameExtensionsSlice(fields, "sourceIpGroups", rule.SourceIpGroups)
	addIDNameSlice(fields, "workloadGroups", rule.WorkloadGroups)
	addIDNameSlice(fields, "fileTypeCategories", rule.FileTypeCategories)
	return resources.NewSourceRecord(fields)
}

func dlpWebRuleReceiverSource(receiver *dlpwebrules.Receiver) map[string]any {
	if receiver == nil {
		return nil
	}
	fields := map[string]any{
		"id":   receiver.ID,
		"name": receiver.Name,
		"type": receiver.Type,
	}
	if receiver.Tenant != nil {
		fields["tenant"] = idNameExtensionsSource(receiver.Tenant)
	}
	return fields
}

func riskProfileSourceRecord(profile riskprofiles.RiskProfiles) resources.SourceRecord {
	fields := map[string]any{
		"id":                        profile.ID,
		"profileName":               profile.ProfileName,
		"profileType":               profile.ProfileType,
		"status":                    profile.Status,
		"excludeCertificates":       profile.ExcludeCertificates,
		"poorItemsOfService":        profile.PoorItemsOfService,
		"adminAuditLogs":            profile.AdminAuditLogs,
		"dataBreach":                profile.DataBreach,
		"sourceIpRestrictions":      profile.SourceIpRestrictions,
		"mfaSupport":                profile.MfaSupport,
		"sslPinned":                 profile.SslPinned,
		"httpSecurityHeaders":       profile.HttpSecurityHeaders,
		"evasive":                   profile.Evasive,
		"dnsCaaPolicy":              profile.DnsCaaPolicy,
		"weakCipherSupport":         profile.WeakCipherSupport,
		"passwordStrength":          profile.PasswordStrength,
		"sslCertValidity":           profile.SslCertValidity,
		"vulnerability":             profile.Vulnerability,
		"malwareScanningForContent": profile.MalwareScanningForContent,
		"fileSharing":               profile.FileSharing,
		"sslCertKeySize":            profile.SslCertKeySize,
		"vulnerableToHeartBleed":    profile.VulnerableToHeartBleed,
		"vulnerableToLogJam":        profile.VulnerableToLogJam,
		"vulnerableToPoodle":        profile.VulnerableToPoodle,
		"vulnerabilityDisclosure":   profile.VulnerabilityDisclosure,
		"supportForWaf":             profile.SupportForWaf,
		"remoteScreenSharing":       profile.RemoteScreenSharing,
		"senderPolicyFramework":     profile.SenderPolicyFramework,
		"domainKeysIdentifiedMail":  profile.DomainKeysIdentifiedMail,
		"domainBasedMessageAuth":    profile.DomainBasedMessageAuth,
		"lastModTime":               profile.LastModTime,
		"createTime":                profile.CreateTime,
	}
	addStringSlice(fields, "certifications", profile.Certifications)
	addStringSlice(fields, "dataEncryptionInTransit", profile.DataEncryptionInTransit)
	addIntSlice(fields, "riskIndex", profile.RiskIndex)
	addIDNameExtensionsPtr(fields, "modifiedBy", profile.ModifiedBy)
	addIDNameExternalIDSlice(fields, "customTags", profile.CustomTags)
	return resources.NewSourceRecord(fields)
}

func nssServerSourceRecord(server nssservers.NSSServers) resources.SourceRecord {
	return resources.NewSourceRecord(map[string]any{
		"id":        server.ID,
		"name":      server.Name,
		"status":    server.Status,
		"state":     server.State,
		"type":      server.Type,
		"icapSvrId": server.IcapSvrId,
	})
}

func nssFeedSourceRecord(feed cloudnss.NSSFeed) resources.SourceRecord {
	fields := map[string]any{
		"id":                       feed.ID,
		"name":                     feed.Name,
		"feedStatus":               feed.FeedStatus,
		"nssLogType":               feed.NssLogType,
		"nssFeedType":              feed.NssFeedType,
		"feedOutputFormat":         feed.FeedOutputFormat,
		"userObfuscation":          feed.UserObfuscation,
		"timeZone":                 feed.TimeZone,
		"epsRateLimit":             feed.EpsRateLimit,
		"jsonArrayToggle":          feed.JsonArrayToggle,
		"siemType":                 feed.SiemType,
		"maxBatchSize":             feed.MaxBatchSize,
		"connectionURL":            feed.ConnectionURL,
		"authenticationToken":      feed.AuthenticationToken,
		"lastSuccessFullTest":      feed.LastSuccessFullTest,
		"testConnectivityCode":     feed.TestConnectivityCode,
		"base64EncodedCertificate": feed.Base64EncodedCertificate,
		"nssType":                  feed.NssType,
		"clientId":                 feed.ClientID,
		"clientSecret":             feed.ClientSecret,
		"authenticationUrl":        feed.AuthenticationUrl,
		"grantType":                feed.GrantType,
		"scope":                    feed.Scope,
		"cloudNss":                 feed.CloudNSS,
		"oauthAuthentication":      feed.OauthAuthentication,
		"firewallLoggingMode":      feed.FirewallLoggingMode,
		"actionFilter":             feed.ActionFilter,
		"emailDlpPolicyAction":     feed.EmailDlpPolicyAction,
		"direction":                feed.Direction,
		"event":                    feed.Event,
	}
	addStringSlice(fields, "customEscapedCharacter", feed.CustomEscapedCharacter)
	addStringSlice(fields, "connectionHeaders", feed.ConnectionHeaders)
	addStringSlice(fields, "serverIps", feed.ServerIps)
	addStringSlice(fields, "clientIps", feed.ClientIps)
	addStringSlice(fields, "domains", feed.Domains)
	addStringSlice(fields, "dnsRequestTypes", feed.DNSRequestTypes)
	addStringSlice(fields, "dnsResponseTypes", feed.DNSResponseTypes)
	addStringSlice(fields, "dnsResponses", feed.DNSResponses)
	addStringSlice(fields, "durations", feed.Durations)
	addStringSlice(fields, "dnsActions", feed.DNSActions)
	addStringSlice(fields, "clientSourceIps", feed.ClientSourceIps)
	addStringSlice(fields, "firewallActions", feed.FirewallActions)
	addStringSlice(fields, "countries", feed.Countries)
	addStringSlice(fields, "serverSourcePorts", feed.ServerSourcePorts)
	addStringSlice(fields, "clientSourcePorts", feed.ClientSourcePorts)
	addStringSlice(fields, "policyReasons", feed.PolicyReasons)
	addStringSlice(fields, "protocolTypes", feed.ProtocolTypes)
	addStringSlice(fields, "userAgents", feed.UserAgents)
	addStringSlice(fields, "requestMethods", feed.RequestMethods)
	addStringSlice(fields, "casbSeverity", feed.CasbSeverity)
	addStringSlice(fields, "casbPolicyTypes", feed.CasbPolicyTypes)
	addStringSlice(fields, "casbApplications", feed.CasbApplications)
	addStringSlice(fields, "casbAction", feed.CasbAction)
	addStringSlice(fields, "urlSuperCategories", feed.URLSuperCategories)
	addStringSlice(fields, "webApplications", feed.WebApplications)
	addStringSlice(fields, "webApplicationClasses", feed.WebApplicationClasses)
	addStringSlice(fields, "malwareNames", feed.MalwareNames)
	addStringSlice(fields, "urlClasses", feed.URLClasses)
	addStringSlice(fields, "malwareClasses", feed.MalwareClasses)
	addStringSlice(fields, "advancedThreats", feed.AdvancedThreats)
	addStringSlice(fields, "responseCodes", feed.ResponseCodes)
	addStringSlice(fields, "nwApplications", feed.NwApplications)
	addStringSlice(fields, "natActions", feed.NatActions)
	addStringSlice(fields, "trafficForwards", feed.TrafficForwards)
	addStringSlice(fields, "webTrafficForwards", feed.WebTrafficForwards)
	addStringSlice(fields, "tunnelTypes", feed.TunnelTypes)
	addStringSlice(fields, "alerts", feed.Alerts)
	addStringSlice(fields, "objectType", feed.ObjectType)
	addStringSlice(fields, "activity", feed.Activity)
	addStringSlice(fields, "objectType1", feed.ObjectType1)
	addStringSlice(fields, "objectType2", feed.ObjectType2)
	addStringSlice(fields, "endPointDLPLogType", feed.EndPointDLPLogType)
	addStringSlice(fields, "emailDLPLogType", feed.EmailDLPLogType)
	addStringSlice(fields, "fileTypeSuperCategories", feed.FileTypeSuperCategories)
	addStringSlice(fields, "fileTypeCategories", feed.FileTypeCategories)
	addStringSlice(fields, "casbFileType", feed.CasbFileType)
	addStringSlice(fields, "casbFileTypeSuperCategories", feed.CasbFileTypeSuperCategories)
	addStringSlice(fields, "messageSize", feed.MessageSize)
	addStringSlice(fields, "fileSizes", feed.FileSizes)
	addStringSlice(fields, "requestSizes", feed.RequestSizes)
	addStringSlice(fields, "responseSizes", feed.ResponseSizes)
	addStringSlice(fields, "transactionSizes", feed.TransactionSizes)
	addStringSlice(fields, "inBoundBytes", feed.InBoundBytes)
	addStringSlice(fields, "outBoundBytes", feed.OutBoundBytes)
	addStringSlice(fields, "downloadTime", feed.DownloadTime)
	addStringSlice(fields, "scanTime", feed.ScanTime)
	addStringSlice(fields, "serverSourceIps", feed.ServerSourceIps)
	addStringSlice(fields, "serverDestinationIps", feed.ServerDestinationIps)
	addStringSlice(fields, "tunnelIps", feed.TunnelIps)
	addStringSlice(fields, "internalIps", feed.InternalIps)
	addStringSlice(fields, "tunnelSourceIps", feed.TunnelSourceIps)
	addStringSlice(fields, "tunnelDestIps", feed.TunnelDestIps)
	addStringSlice(fields, "clientDestinationIps", feed.ClientDestinationIps)
	addStringSlice(fields, "auditLogType", feed.AuditLogType)
	addStringSlice(fields, "projectName", feed.ProjectName)
	addStringSlice(fields, "repoName", feed.RepoName)
	addStringSlice(fields, "objectName", feed.ObjectName)
	addStringSlice(fields, "channelName", feed.ChannelName)
	addStringSlice(fields, "fileSource", feed.FileSource)
	addStringSlice(fields, "fileName", feed.FileName)
	addStringSlice(fields, "sessionCounts", feed.SessionCounts)
	addStringSlice(fields, "advUserAgents", feed.AdvUserAgents)
	addStringSlice(fields, "refererUrls", feed.RefererUrls)
	addStringSlice(fields, "hostNames", feed.HostNames)
	addStringSlice(fields, "fullUrls", feed.FullUrls)
	addStringSlice(fields, "threatNames", feed.ThreatNames)
	addStringSlice(fields, "pageRiskIndexes", feed.PageRiskIndexes)
	addStringSlice(fields, "clientDestinationPorts", feed.ClientDestinationPorts)
	addStringSlice(fields, "tunnelSourcePort", feed.TunnelSourcePort)
	addCommonNSSSlice(fields, "casbTenant", feed.CasbTenant)
	addCommonNSSSlice(fields, "locations", feed.Locations)
	addCommonNSSSlice(fields, "locationGroups", feed.LocationGroups)
	addCommonNSSSlice(fields, "users", feed.Users)
	addCommonNSSSlice(fields, "departments", feed.Departments)
	addCommonNSSSlice(fields, "senderName", feed.SenderName)
	addCommonNSSSlice(fields, "buckets", feed.Buckets)
	addCommonNSSSlice(fields, "vpnCredentials", feed.VPNCredentials)
	addIDNameExtensionsSlice(fields, "externalOwners", feed.ExternalOwners)
	addIDNameExtensionsSlice(fields, "externalCollaborators", feed.ExternalCollaborators)
	addIDNameExtensionsSlice(fields, "internalCollaborators", feed.InternalCollaborators)
	addIDNameExtensionsSlice(fields, "itsmObjectType", feed.ItsmObjectType)
	addIDNameExtensionsSlice(fields, "urlCategories", feed.URLCategories)
	addIDNameExtensionsSlice(fields, "dlpEngines", feed.DLPEngines)
	addIDNameExtensionsSlice(fields, "dlpDictionaries", feed.DLPDictionaries)
	addIDNameExtensionsSlice(fields, "rules", feed.Rules)
	addIDNameExtensionsSlice(fields, "nwServices", feed.NwServices)
	return resources.NewSourceRecord(fields)
}

func fileTypeRuleSourceRecord(rule filetypecontrol.FileTypeRules) resources.SourceRecord {
	fields := map[string]any{
		"id":                   rule.ID,
		"name":                 rule.Name,
		"description":          rule.Description,
		"state":                rule.State,
		"order":                rule.Order,
		"filteringAction":      rule.FilteringAction,
		"timeQuota":            rule.TimeQuota,
		"sizeQuota":            rule.SizeQuota,
		"accessControl":        rule.AccessControl,
		"rank":                 rule.Rank,
		"capturePCAP":          rule.CapturePCAP,
		"passwordProtected":    rule.PasswordProtected,
		"operation":            rule.Operation,
		"activeContent":        rule.ActiveContent,
		"unscannable":          rule.Unscannable,
		"browserEunTemplateId": rule.BrowserEunTemplateID,
		"minSize":              rule.MinSize,
		"maxSize":              rule.MaxSize,
		"lastModifiedTime":     rule.LastModifiedTime,
	}
	addStringSlice(fields, "cloudApplications", rule.CloudApplications)
	addStringSlice(fields, "fileTypes", rule.FileTypes)
	addStringSlice(fields, "protocols", rule.Protocols)
	addStringSlice(fields, "urlCategories", rule.URLCategories)
	addStringSlice(fields, "deviceTrustLevels", rule.DeviceTrustLevels)
	addIDNameExtensionsPtr(fields, "lastModifiedBy", rule.LastModifiedBy)
	addIDNameExtensionsSlice(fields, "locations", rule.Locations)
	addIDNameExtensionsSlice(fields, "locationGroups", rule.LocationGroups)
	addIDNameExtensionsSlice(fields, "groups", rule.Groups)
	addIDNameExtensionsSlice(fields, "departments", rule.Departments)
	addIDNameExtensionsSlice(fields, "users", rule.Users)
	addIDNameExtensionsSlice(fields, "timeWindows", rule.TimeWindows)
	addIDNameExtensionsSlice(fields, "labels", rule.Labels)
	addIDNameExtensionsSlice(fields, "deviceGroups", rule.DeviceGroups)
	addIDNameExtensionsSlice(fields, "devices", rule.Devices)
	if len(rule.ZPAAppSegments) > 0 {
		fields["zpaAppSegments"] = zpaAppSegmentsSource(rule.ZPAAppSegments)
	}
	return resources.NewSourceRecord(fields)
}

func sandboxRuleSourceRecord(rule sandboxrules.SandboxRules) resources.SourceRecord {
	fields := map[string]any{
		"id":                 rule.ID,
		"name":               rule.Name,
		"description":        rule.Description,
		"state":              rule.State,
		"order":              rule.Order,
		"baRuleAction":       rule.BaRuleAction,
		"firstTimeEnable":    rule.FirstTimeEnable,
		"firstTimeOperation": rule.FirstTimeOperation,
		"mlActionEnabled":    rule.MLActionEnabled,
		"byThreatScore":      rule.ByThreatScore,
		"accessControl":      rule.AccessControl,
		"rank":               rule.Rank,
		"lastModifiedTime":   rule.LastModifiedTime,
		"defaultRule":        rule.DefaultRule,
	}
	addStringSlice(fields, "protocols", rule.Protocols)
	addStringSlice(fields, "baPolicyCategories", rule.BaPolicyCategories)
	addStringSlice(fields, "fileTypes", rule.FileTypes)
	addStringSlice(fields, "urlCategories", rule.URLCategories)
	addIDNameExtensionsPtr(fields, "lastModifiedBy", rule.LastModifiedBy)
	addIDNameExtensionsSlice(fields, "locations", rule.Locations)
	addIDNameExtensionsSlice(fields, "locationGroups", rule.LocationGroups)
	addIDNameExtensionsSlice(fields, "groups", rule.Groups)
	addIDNameExtensionsSlice(fields, "departments", rule.Departments)
	addIDNameExtensionsSlice(fields, "users", rule.Users)
	addIDNameExtensionsSlice(fields, "timeWindows", rule.TimeWindows)
	addIDNameExtensionsSlice(fields, "labels", rule.Labels)
	addIDNameExtensionsSlice(fields, "deviceGroups", rule.DeviceGroups)
	addIDNameExtensionsSlice(fields, "devices", rule.Devices)
	if len(rule.ZPAAppSegments) > 0 {
		fields["zpaAppSegments"] = zpaAppSegmentsSource(rule.ZPAAppSegments)
	}
	return resources.NewSourceRecord(fields)
}

func firewallDNSRuleSourceRecord(rule firewalldnscontrolpolicies.FirewallDNSRules) resources.SourceRecord {
	fields := map[string]any{
		"id":                     rule.ID,
		"name":                   rule.Name,
		"order":                  rule.Order,
		"rank":                   rule.Rank,
		"accessControl":          rule.AccessControl,
		"action":                 rule.Action,
		"state":                  rule.State,
		"description":            rule.Description,
		"redirectIp":             rule.RedirectIP,
		"blockResponseCode":      rule.BlockResponseCode,
		"lastModifiedTime":       rule.LastModifiedTime,
		"defaultRule":            rule.DefaultRule,
		"capturePCAP":            rule.CapturePCAP,
		"predefined":             rule.Predefined,
		"isWebEunEnabled":        rule.IsWebEUNEnabled,
		"defaultDnsRuleNameUsed": rule.DefaultDNSRuleNameUsed,
	}
	addIDNameExtensionsPtr(fields, "lastModifiedBy", rule.LastModifiedBy)
	addStringSlice(fields, "srcIps", rule.SrcIps)
	addStringSlice(fields, "destAddresses", rule.DestAddresses)
	addStringSlice(fields, "destIpCategories", rule.DestIpCategories)
	addStringSlice(fields, "destCountries", rule.DestCountries)
	addStringSlice(fields, "sourceCountries", rule.SourceCountries)
	addStringSlice(fields, "resCategories", rule.ResCategories)
	addStringSlice(fields, "applications", rule.Applications)
	addStringSlice(fields, "dnsRuleRequestTypes", rule.DNSRuleRequestTypes)
	addStringSlice(fields, "protocols", rule.Protocols)
	addIDNameExtensionsSlice(fields, "applicationGroups", rule.ApplicationGroups)
	addIDNamePtr(fields, "dnsGateway", rule.DNSGateway)
	addIDNamePtr(fields, "zpaIpGroup", rule.ZPAIPGroup)
	addIDNamePtr(fields, "ednsEcsObject", rule.EDNSEcsObject)
	addIDNameExtensionsSlice(fields, "locations", rule.Locations)
	addIDNameExtensionsSlice(fields, "locationGroups", rule.LocationsGroups)
	addIDNameExtensionsSlice(fields, "departments", rule.Departments)
	addIDNameExtensionsSlice(fields, "groups", rule.Groups)
	addIDNameExtensionsSlice(fields, "users", rule.Users)
	addIDNameExtensionsSlice(fields, "timeWindows", rule.TimeWindows)
	addIDNameExtensionsSlice(fields, "labels", rule.Labels)
	addIDNameExtensionsSlice(fields, "destIpGroups", rule.DestIpGroups)
	addIDNameExtensionsSlice(fields, "destIpv6Groups", rule.DestIpv6Groups)
	addIDNameExtensionsSlice(fields, "srcIpGroups", rule.SrcIpGroups)
	addIDNameExtensionsSlice(fields, "srcIpv6Groups", rule.SrcIpv6Groups)
	addIDNameExtensionsSlice(fields, "deviceGroups", rule.DeviceGroups)
	addIDNameExtensionsSlice(fields, "devices", rule.Devices)
	return resources.NewSourceRecord(fields)
}

func customFileTypeSourceRecord(fileType customfiletypes.CustomFileTypes) resources.SourceRecord {
	return resources.NewSourceRecord(map[string]any{
		"id":          fileType.ID,
		"name":        fileType.Name,
		"description": fileType.Description,
		"extension":   fileType.Extension,
		"fileTypeId":  fileType.FileTypeID,
	})
}

func zpaGatewaySourceRecord(gateway zpagateways.ZPAGateways) resources.SourceRecord {
	fields := map[string]any{
		"id":               gateway.ID,
		"name":             gateway.Name,
		"description":      gateway.Description,
		"zpaServerGroup":   zpaGatewayServerGroupSource(gateway.ZPAServerGroup),
		"zpaTenantId":      gateway.ZPATenantId,
		"lastModifiedTime": gateway.LastModifiedTime,
		"type":             gateway.Type,
	}
	if len(gateway.ZPAAppSegments) > 0 {
		fields["zpaAppSegments"] = zpaGatewayAppSegmentsSource(gateway.ZPAAppSegments)
	}
	addIDNameExtensionsPtr(fields, "lastModifiedBy", gateway.LastModifiedBy)
	return resources.NewSourceRecord(fields)
}

func dcExclusionSourceRecord(exclusion dcexclusions.DCExclusions) resources.SourceRecord {
	fields := map[string]any{
		"dcid":        exclusion.DcID,
		"expired":     exclusion.Expired,
		"startTime":   exclusion.StartTime,
		"endTime":     exclusion.EndTime,
		"description": exclusion.Description,
	}
	addIDNameExtensionsPtr(fields, "dcName", exclusion.DcName)
	return resources.NewSourceRecord(fields)
}

func subCloudSourceRecord(cloud subclouds.SubClouds) resources.SourceRecord {
	fields := map[string]any{
		"id":         cloud.ID,
		"name":       cloud.Name,
		"dcs":        subCloudDCsSource(cloud.Dcs),
		"exclusions": subCloudExclusionsSource(cloud.Exclusions),
	}
	return resources.NewSourceRecord(fields)
}

func subCloudDCsSource(values []subclouds.DCs) []map[string]any {
	out := make([]map[string]any, 0, len(values))
	for _, value := range values {
		out = append(out, map[string]any{
			"id":      value.ID,
			"name":    value.Name,
			"country": value.Country,
		})
	}
	return out
}

func subCloudExclusionsSource(values []subclouds.Exclusions) []map[string]any {
	out := make([]map[string]any, 0, len(values))
	for _, value := range values {
		item := map[string]any{
			"country":          value.Country,
			"expired":          value.Expired,
			"disabledByOps":    value.DisabledByOps,
			"createTime":       value.CreateTime,
			"startTime":        value.StartTime,
			"endTime":          value.EndTime,
			"lastModifiedTime": value.LastModifiedTime,
		}
		if value.Datacenter != nil {
			item["datacenter"] = idNameExtensionsSource(value.Datacenter)
		}
		if value.LastModifiedUser != nil {
			item["lastModifiedUser"] = idNameExtensionsSource(value.LastModifiedUser)
		}
		out = append(out, item)
	}
	return out
}

func ipv6ConfigSourceRecord(config ipv6config.IPv6Config) resources.SourceRecord {
	return resources.NewSourceRecord(map[string]any{
		"ipV6Enabled": config.IpV6Enabled,
		"natPrefixes": ipv6ConfigPrefixesSource(config.NatPrefixes),
		"dnsPrefix":   config.DnsPrefix,
	})
}

func ipv6ConfigPrefixSourceRecord(prefix ipv6config.IPv6ConfigPrefix) resources.SourceRecord {
	return resources.NewSourceRecord(ipv6ConfigPrefixSource(prefix))
}

func ipv6ConfigPrefixesSource(values []ipv6config.IPv6ConfigPrefix) []map[string]any {
	out := make([]map[string]any, 0, len(values))
	for _, value := range values {
		out = append(out, ipv6ConfigPrefixSource(value))
	}
	return out
}

func ipv6ConfigPrefixSource(prefix ipv6config.IPv6ConfigPrefix) map[string]any {
	return map[string]any{
		"id":          prefix.ID,
		"name":        prefix.Name,
		"description": prefix.Description,
		"prefixMask":  prefix.PrefixMask,
		"dnsPrefix":   prefix.DnsPrefix,
		"nonEditable": prefix.NonEditable,
	}
}

func pacFileSourceRecord(file pacfiles.PACFileConfig) resources.SourceRecord {
	fields := map[string]any{
		"id":                    file.ID,
		"name":                  file.Name,
		"description":           file.Description,
		"domain":                file.Domain,
		"pacUrl":                file.PACUrl,
		"pacContent":            file.PACContent,
		"editable":              file.Editable,
		"pacSubURL":             file.PACSubURL,
		"pacUrlObfuscated":      file.PACUrlObfuscated,
		"pacVerificationStatus": file.PACVerificationStatus,
		"pacVersionStatus":      file.PACVersionStatus,
		"pacVersion":            file.PACVersion,
		"pacCommitMessage":      file.PACCommitMessage,
		"totalHits":             file.TotalHits,
		"lastModificationTime":  file.LastModificationTime,
		"createTime":            file.CreateTime,
	}
	if file.LastModifiedBy.ID != 0 || file.LastModifiedBy.Name != "" || file.LastModifiedBy.ExternalID != "" {
		fields["lastModifiedBy"] = map[string]any{
			"id":         file.LastModifiedBy.ID,
			"name":       file.LastModifiedBy.Name,
			"externalId": file.LastModifiedBy.ExternalID,
			"extensions": file.LastModifiedBy.Extensions,
		}
	}
	return resources.NewSourceRecord(fields)
}

func cloudApplicationPolicySourceRecord(app cloudapplications.CloudApplications) resources.SourceRecord {
	return resources.NewSourceRecord(map[string]any{
		"app":        app.App,
		"appName":    app.AppName,
		"parent":     app.Parent,
		"parentName": app.ParentName,
	})
}

func domainProfileSourceRecord(profile saassecurityapi.DomainProfiles) resources.SourceRecord {
	fields := map[string]any{
		"profileId":             profile.ProfileID,
		"profileName":           profile.ProfileName,
		"includeCompanyDomains": profile.IncludeCompanyDomains,
		"includeSubdomains":     profile.IncludeSubdomains,
		"description":           profile.Description,
	}
	addStringSlice(fields, "customDomains", profile.CustomDomains)
	addStringSlice(fields, "predefinedEmailDomains", profile.PredefinedEmailDomains)
	return resources.NewSourceRecord(fields)
}

func casbTombstoneTemplateSourceRecord(template saassecurityapi.QuarantineTombstoneLite) resources.SourceRecord {
	return resources.NewSourceRecord(map[string]any{
		"id":          template.ID,
		"name":        template.Name,
		"description": template.Description,
	})
}

func casbEmailLabelSourceRecord(label saassecurityapi.CasbEmailLabel) resources.SourceRecord {
	return resources.NewSourceRecord(map[string]any{
		"id":           label.ID,
		"name":         label.Name,
		"labelDesc":    label.LabelDesc,
		"labelColor":   label.LabelColor,
		"labelDeleted": label.LabelDeleted,
	})
}

func casbTenantSourceRecord(tenant saassecurityapi.CasbTenants) resources.SourceRecord {
	return resources.NewSourceRecord(casbTenantSource(tenant))
}

func casbTenantSource(tenant saassecurityapi.CasbTenants) map[string]any {
	fields := map[string]any{
		"tenantId":                 tenant.TenantID,
		"modifiedTime":             tenant.ModifiedTime,
		"lastTenantValidationTime": tenant.LastTenantValidationTime,
		"tenantDeleted":            tenant.TenantDeleted,
		"tenantWebhookEnabled":     tenant.TenantWebhookEnabled,
		"reAuth":                   tenant.ReAuth,
		"enterpriseTenantId":       tenant.EnterpriseTenantID,
		"tenantName":               tenant.TenantName,
		"saasApplication":          tenant.SaaSApplication,
	}
	addStringSlice(fields, "featuresSupported", tenant.FeaturesSupported)
	addStringSlice(fields, "status", tenant.Status)
	if tenant.ZscalerAppTenantID != nil {
		fields["zscalerAppTenantId"] = idNameSource(tenant.ZscalerAppTenantID)
	}
	return fields
}

func casbDLPRuleSourceRecord(rule casbdlprules.CasbDLPRules) resources.SourceRecord {
	fields := map[string]any{
		"type":                          rule.Type,
		"id":                            rule.ID,
		"order":                         rule.Order,
		"rank":                          rule.Rank,
		"lastModifiedTime":              rule.LastModifiedTime,
		"name":                          rule.Name,
		"state":                         rule.State,
		"action":                        rule.Action,
		"severity":                      rule.Severity,
		"description":                   rule.Description,
		"bucketOwner":                   rule.BucketOwner,
		"externalAuditorEmail":          rule.ExternalAuditorEmail,
		"contentLocation":               rule.ContentLocation,
		"numberOfInternalCollaborators": rule.NumberOfInternalCollaborators,
		"numberOfExternalCollaborators": rule.NumberOfExternalCollaborators,
		"recipient":                     rule.Recipient,
		"quarantineLocation":            rule.QuarantineLocation,
		"accessControl":                 rule.AccessControl,
		"watermarkDeleteOldVersion":     rule.WatermarkDeleteOldVersion,
		"includeCriteriaDomainProfile":  rule.IncludeCriteriaDomainProfile,
		"includeEmailRecipientProfile":  rule.IncludeEmailRecipientProfile,
		"withoutContentInspection":      rule.WithoutContentInspection,
		"includeEntityGroups":           rule.IncludeEntityGroups,
	}
	addStringSlice(fields, "fileTypes", rule.FileTypes)
	addStringSlice(fields, "collaborationScope", rule.CollaborationScope)
	addStringSlice(fields, "domains", rule.Domains)
	addStringSlice(fields, "components", rule.Components)
	addStringSlice(fields, "deviceTrustLevels", rule.DeviceTrustLevels)
	addIDNameExtensionsSlice(fields, "objectTypes", rule.ObjectTypes)
	addIDNameExtensionsSlice(fields, "buckets", rule.Buckets)
	addIDNameExtensionsSlice(fields, "labels", rule.Labels)
	addIDNameExtensionsSlice(fields, "includedDomainProfiles", rule.IncludedDomainProfiles)
	addIDNameExtensionsSlice(fields, "excludedDomainProfiles", rule.ExcludedDomainProfiles)
	addIDNameExtensionsSlice(fields, "criteriaDomainProfiles", rule.CriteriaDomainProfiles)
	addIDNameExtensionsSlice(fields, "emailRecipientProfiles", rule.EmailRecipientProfiles)
	addIDNameExtensionsSlice(fields, "devices", rule.Devices)
	addIDNameExtensionsSlice(fields, "deviceGroups", rule.DeviceGroups)
	addIDNameExtensionsSlice(fields, "entityGroups", rule.EntityGroups)
	addIDNameExtensionsSlice(fields, "cloudAppTenants", rule.CloudAppTenants)
	addIDNameExtensionsSlice(fields, "users", rule.Users)
	addIDNameExtensionsSlice(fields, "groups", rule.Groups)
	addIDNameExtensionsSlice(fields, "departments", rule.Departments)
	addIDNameExtensionsSlice(fields, "dlpEngines", rule.DLPEngines)
	if rule.LastModifiedBy != nil {
		fields["lastModifiedBy"] = idNameExtensionsSource(rule.LastModifiedBy)
	}
	addIDCustomPtr(fields, "auditor", rule.Auditor)
	addIDCustomPtr(fields, "zscalerIncidentReceiver", rule.ZscalerIncidentReceiver)
	addIDCustomPtr(fields, "auditorNotification", rule.AuditorNotification)
	addIDCustomPtr(fields, "tag", rule.Tag)
	addIDCustomPtr(fields, "watermarkProfile", rule.WatermarkProfile)
	addIDCustomPtr(fields, "redactionProfile", rule.RedactionProfile)
	addIDCustomPtr(fields, "casbEmailLabel", rule.CasbEmailLabel)
	addIDCustomPtr(fields, "casbTombstoneTemplate", rule.CasbTombstoneTemplate)
	if rule.Receiver != nil {
		fields["receiver"] = casbReceiverSource(rule.Receiver)
	}
	return resources.NewSourceRecord(fields)
}

func casbMalwareRuleSourceRecord(rule casbmalwarerules.CasbMalwareRules) resources.SourceRecord {
	fields := map[string]any{
		"type":                 rule.Type,
		"id":                   rule.ID,
		"order":                rule.Order,
		"name":                 rule.Name,
		"state":                rule.State,
		"action":               rule.Action,
		"quarantineLocation":   rule.QuarantineLocation,
		"scanInboundEmailLink": rule.ScanInboundEmailLink,
		"lastModifiedTime":     rule.LastModifiedTime,
		"accessControl":        rule.AccessControl,
	}
	if rule.LastModifiedBy != nil {
		fields["lastModifiedBy"] = idNameExtensionsSource(rule.LastModifiedBy)
	}
	addIDCustomPtr(fields, "casbEmailLabel", rule.CasbEmailLabel)
	addIDCustomPtr(fields, "casbTombstoneTemplate", rule.CasbTombstoneTemplate)
	addIDNameExtensionsSlice(fields, "buckets", rule.Buckets)
	addIDNameExtensionsSlice(fields, "labels", rule.Labels)
	addIDNameExtensionsSlice(fields, "cloudAppTenantIds", rule.CloudAppTenantIDs)
	addIDNameExtensionsSlice(fields, "cloudAppTenants", rule.CloudAppTenants)
	if len(rule.CloudApplicationTenant) > 0 {
		out := make([]map[string]any, 0, len(rule.CloudApplicationTenant))
		for _, tenant := range rule.CloudApplicationTenant {
			out = append(out, casbTenantSource(tenant))
		}
		fields["cloudApplicationTenant"] = out
	}
	return resources.NewSourceRecord(fields)
}

func browserControlSettingsSourceRecord(settings browsercontrolsettings.BrowserControlSettings) resources.SourceRecord {
	fields := map[string]any{
		"pluginCheckFrequency":            settings.PluginCheckFrequency,
		"bypassPlugins":                   settings.BypassPlugins,
		"bypassApplications":              settings.BypassApplications,
		"blockedInternetExplorerVersions": settings.BlockedInternetExplorerVersions,
		"blockedChromeVersions":           settings.BlockedChromeVersions,
		"blockedFirefoxVersions":          settings.BlockedFirefoxVersions,
		"blockedSafariVersions":           settings.BlockedSafariVersions,
		"blockedOperaVersions":            settings.BlockedOperaVersions,
		"bypassAllBrowsers":               settings.BypassAllBrowsers,
		"allowAllBrowsers":                settings.AllowAllBrowsers,
		"enableWarnings":                  settings.EnableWarnings,
		"enableSmartBrowserIsolation":     settings.EnableSmartBrowserIsolation,
		"smartIsolationProfileId":         settings.SmartIsolationProfileID,
	}
	addIDNameExtensionsSlice(fields, "smartIsolationUsers", settings.SmartIsolationUsers)
	addIDNameExtensionsSlice(fields, "smartIsolationGroups", settings.SmartIsolationGroups)
	if settings.SmartIsolationProfile.ID != "" || settings.SmartIsolationProfile.Name != "" || settings.SmartIsolationProfile.URL != "" {
		fields["smartIsolationProfile"] = map[string]any{
			"id":             settings.SmartIsolationProfile.ID,
			"name":           settings.SmartIsolationProfile.Name,
			"url":            settings.SmartIsolationProfile.URL,
			"defaultProfile": settings.SmartIsolationProfile.DefaultProfile,
		}
	}
	return resources.NewSourceRecord(fields)
}

func ziaAdminUserSourceRecord(user ziaadminusers.AdminUsers) resources.SourceRecord {
	fields := map[string]any{
		"id":                          user.ID,
		"loginName":                   user.LoginName,
		"userName":                    user.UserName,
		"email":                       user.Email,
		"comments":                    user.Comments,
		"disabled":                    user.Disabled,
		"password":                    user.Password,
		"pwdLastModifiedTime":         user.PasswordLastModifiedTime,
		"isNonEditable":               user.IsNonEditable,
		"isPasswordLoginAllowed":      user.IsPasswordLoginAllowed,
		"isPasswordExpired":           user.IsPasswordExpired,
		"isAuditor":                   user.IsAuditor,
		"isSecurityReportCommEnabled": user.IsSecurityReportCommEnabled,
		"isServiceUpdateCommEnabled":  user.IsServiceUpdateCommEnabled,
		"isProductUpdateCommEnabled":  user.IsProductUpdateCommEnabled,
		"isExecMobileAppEnabled":      user.IsExecMobileAppEnabled,
		"adminScopeType":              user.AdminScopeType,
	}
	addIDNameExtensionsSlice(fields, "adminScopescopeGroupMemberEntities", user.AdminScopeGroupMemberEntities)
	addIDNameExtensionsSlice(fields, "adminScopeScopeEntities", user.AdminScopeEntities)
	if user.Role != nil {
		fields["role"] = ziaAdminUserRoleSource(user.Role)
	}
	if len(user.ExecMobileAppTokens) > 0 {
		fields["execMobileAppTokens"] = ziaExecMobileAppTokensSource(user.ExecMobileAppTokens)
	}
	return resources.NewSourceRecord(fields)
}

func ziaAdminRoleSourceRecord(role ziaadminroles.AdminRoles) resources.SourceRecord {
	fields := map[string]any{
		"id":                    role.ID,
		"rank":                  role.Rank,
		"name":                  role.Name,
		"policyAccess":          role.PolicyAccess,
		"alertingAccess":        role.AlertingAccess,
		"usernameAccess":        role.UsernameAccess,
		"deviceInfoAccess":      role.DeviceInfoAccess,
		"dashboardAccess":       role.DashboardAccess,
		"reportAccess":          role.ReportAccess,
		"analysisAccess":        role.AnalysisAccess,
		"adminAcctAccess":       role.AdminAcctAccess,
		"isAuditor":             role.IsAuditor,
		"featurePermissions":    role.FeaturePermissions,
		"extFeaturePermissions": role.ExtFeaturePermissions,
		"isNonEditable":         role.IsNonEditable,
		"logsLimit":             role.LogsLimit,
		"roleType":              role.RoleType,
		"reportTimeDuration":    role.ReportTimeDuration,
	}
	addStringSlice(fields, "permissions", role.Permissions)
	return resources.NewSourceRecord(fields)
}

func ziaAdminUserRoleSource(role *ziaadminusers.Role) map[string]any {
	fields := map[string]any{
		"id":            role.ID,
		"name":          role.Name,
		"isNameL10nTag": role.IsNameL10Tag,
	}
	if len(role.Extensions) > 0 {
		fields["extensions"] = role.Extensions
	}
	return fields
}

func ziaExecMobileAppTokensSource(values []ziaadminusers.ExecMobileAppTokens) []any {
	out := make([]any, 0, len(values))
	for _, value := range values {
		out = append(out, map[string]any{
			"cloud":       value.Cloud,
			"orgId":       value.OrgId,
			"name":        value.Name,
			"tokenId":     value.TokenId,
			"token":       value.Token,
			"tokenExpiry": value.TokenExpiry,
			"createTime":  value.CreateTime,
			"deviceId":    value.DeviceId,
			"deviceName":  value.DeviceName,
		})
	}
	return out
}

func addIDCustomPtr(fields map[string]any, name string, value *ziacommon.IDCustom) {
	if value == nil {
		return
	}
	fields[name] = map[string]any{
		"id":   value.ID,
		"name": value.Name,
	}
}

func casbReceiverSource(receiver *casbdlprules.Receiver) map[string]any {
	fields := map[string]any{
		"id":   receiver.ID,
		"name": receiver.Name,
		"type": receiver.Type,
	}
	if receiver.Tenant != nil {
		fields["tenant"] = idNameExtensionsSource(receiver.Tenant)
	}
	return fields
}

func c2cIncidentReceiverSourceRecord(receiver c2cincidentreceiver.C2CIncidentReceiver) resources.SourceRecord {
	fields := map[string]any{
		"id":                       receiver.ID,
		"name":                     receiver.Name,
		"status":                   receiver.Status,
		"modifiedTime":             receiver.ModifiedTime,
		"lastTenantValidationTime": receiver.LastTenantValidationTime,
		"lastValidationMsg":        receiver.LastValidationMsg,
		"onboardableEntity":        c2cOnboardableEntitySource(receiver.OnboardableEntity),
	}
	addIDNameExtensionsPtr(fields, "lastModifiedBy", receiver.LastModifiedBy)
	return resources.NewSourceRecord(fields)
}

func c2cOnboardableEntitySource(entity *c2cincidentreceiver.OnboardableEntity) map[string]any {
	if entity == nil {
		return nil
	}
	fields := map[string]any{
		"id":                      entity.ID,
		"name":                    entity.Name,
		"type":                    entity.Type,
		"enterpriseTenantId":      entity.EnterpriseTenantID,
		"application":             entity.Application,
		"lastValidationMsg":       entity.LastValidationMsg,
		"tenantAuthorizationInfo": entity.TenantAuthorizationInfo,
	}
	if entity.ZscalerAppTenantID != nil {
		fields["zscalerAppTenantId"] = idNameExtensionsSource(entity.ZscalerAppTenantID)
	}
	return fields
}

func emailProfileSourceRecord(profile emailprofiles.EmailProfiles) resources.SourceRecord {
	fields := map[string]any{
		"id":          profile.ID,
		"name":        profile.Name,
		"description": profile.Description,
	}
	addStringSlice(fields, "emails", profile.Emails)
	return resources.NewSourceRecord(fields)
}

func applicationSegmentSourceRecord(segment zpaapplicationsegment.ApplicationSegmentResource) resources.SourceRecord {
	fields := map[string]any{
		"adpEnabled":                segment.ADPEnabled,
		"apiProtectionEnabled":      segment.APIProtectionEnabled,
		"appRecommendationId":       segment.AppRecommendationId,
		"applications":              segment.Applications,
		"autoAppProtectEnabled":     segment.AutoAppProtectEnabled,
		"bypassOnReauth":            segment.BypassOnReauth,
		"bypassType":                segment.BypassType,
		"configSpace":               segment.ConfigSpace,
		"creationTime":              segment.CreationTime,
		"defaultIdleTimeout":        segment.DefaultIdleTimeout,
		"defaultMaxAge":             segment.DefaultMaxAge,
		"description":               segment.Description,
		"doubleEncrypt":             segment.DoubleEncrypt,
		"enabled":                   segment.Enabled,
		"extranetEnabled":           segment.ExtranetEnabled,
		"fqdnDnsCheck":              segment.FQDNDnsCheck,
		"healthCheckType":           segment.HealthCheckType,
		"healthReporting":           segment.HealthReporting,
		"icmpAccessType":            segment.IcmpAccessType,
		"id":                        segment.ID,
		"inspectTrafficWithZia":     segment.InspectTrafficWithZia,
		"ipAnchored":                segment.IpAnchored,
		"isCnameEnabled":            segment.IsCnameEnabled,
		"isIncompleteDRConfig":      segment.IsIncompleteDRConfig,
		"matchStyle":                segment.MatchStyle,
		"microtenantId":             segment.MicroTenantID,
		"microtenantName":           segment.MicroTenantName,
		"modifiedBy":                segment.ModifiedBy,
		"modifiedTime":              segment.ModifiedTime,
		"name":                      segment.Name,
		"passiveHealthEnabled":      segment.PassiveHealthEnabled,
		"policyStyle":               segment.PolicyStyle,
		"readOnly":                  segment.ReadOnly,
		"restrictionType":           segment.RestrictionType,
		"segmentGroupId":            segment.SegmentGroupID,
		"segmentGroupName":          segment.SegmentGroupName,
		"selectConnectorCloseToApp": segment.SelectConnectorCloseToApp,
		"tcpKeepAlive":              segment.TCPKeepAlive,
		"useInDrMode":               segment.UseInDrMode,
		"weightedLoadBalancing":     segment.WeightedLoadBalancing,
		"zscalerManaged":            segment.ZscalerManaged,
	}
	addStringSlice(fields, "domainNames", segment.DomainNames)
	addStringSlice(fields, "shareToMicrotenants", segment.ShareToMicrotenants)
	addStringSlice(fields, "tcpPortRanges", segment.TCPPortRanges)
	addStringSlice(fields, "udpPortRanges", segment.UDPPortRanges)
	addZPANetworkPorts(fields, "tcpPortRange", segment.TCPAppPortRange)
	addZPANetworkPorts(fields, "udpPortRange", segment.UDPAppPortRange)
	if len(segment.ServerGroups) > 0 {
		fields["serverGroups"] = zpaServerGroupReferenceSource(segment.ServerGroups)
	}
	if len(segment.ClientlessApps) > 0 {
		fields["clientlessApps"] = zpaClientlessAppsSource(segment.ClientlessApps)
	}
	if sharedMicrotenantDetailsHasValue(segment.SharedMicrotenantDetails) {
		fields["sharedMicrotenantDetails"] = zpaSharedMicrotenantDetailsSource(segment.SharedMicrotenantDetails)
	}
	if segment.ZPNERID != nil {
		fields["zpnErId"] = zpaZPNERIDSource(segment.ZPNERID)
	}
	if len(segment.Tags) > 0 {
		fields["tags"] = zpaApplicationSegmentTagsSource(segment.Tags)
	}
	return resources.NewSourceRecord(fields)
}

func addStringSlice(fields map[string]any, name string, values []string) {
	if len(values) > 0 {
		fields[name] = append([]string(nil), values...)
	}
}

func addIntSlice(fields map[string]any, name string, values []int) {
	if len(values) > 0 {
		fields[name] = append([]int(nil), values...)
	}
}

func addIDNameExternalIDPtr(fields map[string]any, name string, value *ziacommon.IDNameExternalID) {
	if value != nil {
		fields[name] = idNameExternalIDSource(value)
	}
}

func addIDNameExternalIDSlice(fields map[string]any, name string, values []ziacommon.IDNameExternalID) {
	if len(values) > 0 {
		items := make([]any, 0, len(values))
		for i := range values {
			items = append(items, idNameExternalIDSource(&values[i]))
		}
		fields[name] = items
	}
}

func addCloudInstanceIdentifiers(fields map[string]any, name string, values []cloudappinstances.InstanceIdentifiers) {
	if len(values) > 0 {
		items := make([]any, 0, len(values))
		for _, value := range values {
			items = append(items, cloudInstanceIdentifierSource(value))
		}
		fields[name] = items
	}
}

func addIDNameExtensionsPtr(fields map[string]any, name string, value *ziacommon.IDNameExtensions) {
	if value != nil {
		fields[name] = idNameExtensionsSource(value)
	}
}

func addZTWIDNameExtensionsPtr(fields map[string]any, name string, value *ztwcommon.IDNameExtensions) {
	if value != nil {
		fields[name] = ztwIDNameExtensionsSource(value)
	}
}

func addZTWIDNameExtensionsSlice(fields map[string]any, name string, values []ztwcommon.IDNameExtensions) {
	if len(values) > 0 {
		fields[name] = ztwIDNameExtensionsSliceSource(values)
	}
}

func addZTWCommonIDNamePtr(fields map[string]any, name string, value *ztwcommon.CommonIDName) {
	if value != nil {
		fields[name] = ztwCommonIDNameSource(value)
	}
}

func addZTWCommonIDNameExternalIDPtr(fields map[string]any, name string, value *ztwcommon.CommonIDNameExternalID) {
	if value != nil {
		fields[name] = ztwCommonIDNameExternalIDSource(value)
	}
}

func addZTWCommonIDNameExternalIDSlice(fields map[string]any, name string, values []ztwcommon.CommonIDNameExternalID) {
	if len(values) > 0 {
		fields[name] = ztwCommonIDNameExternalIDSliceSource(values)
	}
}

func addIDNameExtensionsSlice(fields map[string]any, name string, values []ziacommon.IDNameExtensions) {
	if len(values) > 0 {
		fields[name] = idNameExtensionsSliceSource(values)
	}
}

func addIDNamePtr(fields map[string]any, name string, value *ziacommon.IDName) {
	if value != nil {
		fields[name] = idNameSource(value)
	}
}

func addIDNameSlice(fields map[string]any, name string, values []ziacommon.IDName) {
	if len(values) > 0 {
		fields[name] = idNameSliceSource(values)
	}
}

func addCommonNSSSlice(fields map[string]any, name string, values []ziacommon.CommonNSS) {
	if len(values) > 0 {
		fields[name] = commonNSSSliceSource(values)
	}
}

func addNetworkPorts(fields map[string]any, name string, values []networkservices.NetworkPorts) {
	if len(values) > 0 {
		fields[name] = networkPortsSource(values)
	}
}

func addZPANetworkPorts(fields map[string]any, name string, values []zpacommon.NetworkPorts) {
	if len(values) > 0 {
		fields[name] = zpaNetworkPortsSource(values)
	}
}

func addZTWNetworkPorts(fields map[string]any, name string, values []ztwnetworkservices.NetworkPorts) {
	if len(values) > 0 {
		fields[name] = ztwNetworkPortsSource(values)
	}
}

func idNameExtensionsSource(value *ziacommon.IDNameExtensions) map[string]any {
	fields := map[string]any{
		"id":   value.ID,
		"name": value.Name,
	}
	return fields
}

func ztwIDNameExtensionsSource(value *ztwcommon.IDNameExtensions) map[string]any {
	return map[string]any{
		"id":   value.ID,
		"name": value.Name,
	}
}

func ztwIDNameExtensionsSliceSource(values []ztwcommon.IDNameExtensions) []any {
	out := make([]any, 0, len(values))
	for i := range values {
		out = append(out, ztwIDNameExtensionsSource(&values[i]))
	}
	return out
}

func ztwCommonIDNameSource(value *ztwcommon.CommonIDName) map[string]any {
	return map[string]any{
		"id":   value.ID,
		"name": value.Name,
	}
}

func idNameSource(value *ziacommon.IDName) map[string]any {
	fields := map[string]any{
		"id":   value.ID,
		"name": value.Name,
	}
	if value.Parent != "" {
		fields["parent"] = value.Parent
	}
	return fields
}

func idCustomSource(value *ziacommon.IDCustom) map[string]any {
	if value == nil {
		return nil
	}
	return map[string]any{
		"id":   value.ID,
		"name": value.Name,
	}
}

func idNameExternalIDSource(value *ziacommon.IDNameExternalID) map[string]any {
	fields := map[string]any{
		"id":   value.ID,
		"name": value.Name,
	}
	if value.ExternalID != "" {
		fields["externalId"] = value.ExternalID
	}
	return fields
}

func userGroupSource(value ziacommon.UserGroups) map[string]any {
	return map[string]any{
		"id":   value.ID,
		"name": value.Name,
	}
}

func addUserGroups(fields map[string]any, name string, values []ziacommon.UserGroups) {
	if len(values) == 0 {
		return
	}
	out := make([]any, 0, len(values))
	for _, value := range values {
		out = append(out, userGroupSource(value))
	}
	fields[name] = out
}

func userDepartmentSource(value ziacommon.UserDepartment) map[string]any {
	return map[string]any{
		"id":   value.ID,
		"name": value.Name,
	}
}

func cloudInstanceIdentifierSource(value cloudappinstances.InstanceIdentifiers) map[string]any {
	fields := map[string]any{
		"instanceId":             value.InstanceID,
		"instanceIdentifier":     value.InstanceIdentifier,
		"instanceIdentifierName": value.InstanceIdentifierName,
		"identifierType":         value.IdentifierType,
		"modifiedAt":             value.ModifiedAt,
	}
	addIDNameExtensionsPtr(fields, "modifiedBy", value.ModifiedBy)
	return fields
}

func idNameExtensionsSliceSource(values []ziacommon.IDNameExtensions) []any {
	out := make([]any, 0, len(values))
	for i := range values {
		out = append(out, idNameExtensionsSource(&values[i]))
	}
	return out
}

func idNameSliceSource(values []ziacommon.IDName) []any {
	out := make([]any, 0, len(values))
	for _, value := range values {
		out = append(out, idNameSource(&value))
	}
	return out
}

func commonNSSSliceSource(values []ziacommon.CommonNSS) []any {
	out := make([]any, 0, len(values))
	for _, value := range values {
		out = append(out, commonNSSSource(value))
	}
	return out
}

func commonNSSSource(value ziacommon.CommonNSS) map[string]any {
	return map[string]any{
		"id":          value.ID,
		"pid":         value.PID,
		"name":        value.Name,
		"description": value.Description,
		"deleted":     value.Deleted,
		"getlId":      value.GetlID,
	}
}

func networkPortsSource(values []networkservices.NetworkPorts) []any {
	out := make([]any, 0, len(values))
	for _, value := range values {
		out = append(out, map[string]any{
			"start": value.Start,
			"end":   value.End,
		})
	}
	return out
}

func zpaNetworkPortsSource(values []zpacommon.NetworkPorts) []any {
	out := make([]any, 0, len(values))
	for _, value := range values {
		out = append(out, map[string]any{
			"from": value.From,
			"to":   value.To,
		})
	}
	return out
}

func zpaServerGroupReferenceSource(values []zpaservergroup.ServerGroup) []any {
	out := make([]any, 0, len(values))
	for _, value := range values {
		out = append(out, map[string]any{
			"id":   value.ID,
			"name": value.Name,
		})
	}
	return out
}

func zpaClientlessAppsSource(values []zpaapplicationsegmentbrowseraccess.ClientlessApps) []any {
	out := make([]any, 0, len(values))
	for _, value := range values {
		out = append(out, map[string]any{
			"id":                  value.ID,
			"name":                value.Name,
			"description":         value.Description,
			"domain":              value.Domain,
			"localDomain":         value.LocalDomain,
			"certificateId":       value.CertificateID,
			"certificateName":     value.CertificateName,
			"applicationProtocol": value.ApplicationProtocol,
			"applicationPort":     value.ApplicationPort,
			"modifiedBy":          value.ModifiedBy,
			"microtenantId":       value.MicroTenantID,
			"microtenantName":     value.MicroTenantName,
		})
	}
	return out
}

func sharedMicrotenantDetailsHasValue(value zpaapplicationsegment.SharedMicrotenantDetails) bool {
	return value.SharedFromMicrotenant.ID != "" ||
		value.SharedFromMicrotenant.Name != "" ||
		len(value.SharedToMicrotenants) > 0
}

func zpaSharedMicrotenantDetailsSource(value zpaapplicationsegment.SharedMicrotenantDetails) map[string]any {
	fields := map[string]any{}
	if value.SharedFromMicrotenant.ID != "" || value.SharedFromMicrotenant.Name != "" {
		fields["sharedFromMicrotenant"] = map[string]any{
			"id":   value.SharedFromMicrotenant.ID,
			"name": value.SharedFromMicrotenant.Name,
		}
	}
	if len(value.SharedToMicrotenants) > 0 {
		items := make([]any, 0, len(value.SharedToMicrotenants))
		for _, tenant := range value.SharedToMicrotenants {
			items = append(items, map[string]any{
				"id":   tenant.ID,
				"name": tenant.Name,
			})
		}
		fields["sharedToMicrotenants"] = items
	}
	return fields
}

func zpaZPNERIDSource(value *zpacommon.ZPNERID) map[string]any {
	if value == nil {
		return nil
	}
	return map[string]any{
		"id":              value.ID,
		"creationTime":    value.CreationTime,
		"modifiedBy":      value.ModifiedBy,
		"modifiedTime":    value.ModifiedTime,
		"ziaCloud":        value.ZIACloud,
		"ziaErId":         value.ZIAErID,
		"ziaErName":       value.ZIAErName,
		"ziaModifiedTime": value.ZIAModifiedTime,
		"ziaOrgId":        value.ZIAOrgID,
	}
}

func zpaApplicationSegmentTagsSource(values []zpaapplicationsegment.Tag) []any {
	out := make([]any, 0, len(values))
	for _, value := range values {
		out = append(out, map[string]any{
			"namespace": tagCommonSummarySource(value.Namespace),
			"tagKey":    tagCommonSummarySource(value.TagKey),
			"tagValue":  tagCommonIDNameSource(value.TagValue),
			"origin":    value.Origin,
		})
	}
	return out
}

func tagCommonSummarySource(value zpacommon.CommonSummary) map[string]any {
	return map[string]any{
		"id":      value.ID,
		"name":    value.Name,
		"enabled": value.Enabled,
	}
}

func tagCommonIDNameSource(value zpacommon.CommonIDName) map[string]any {
	return map[string]any{
		"id":   value.ID,
		"name": value.Name,
	}
}

func workloadTagExpressionSource(value workloadgroups.WorkloadTagExpression) map[string]any {
	fields := map[string]any{}
	if len(value.ExpressionContainers) > 0 {
		items := make([]map[string]any, 0, len(value.ExpressionContainers))
		for _, container := range value.ExpressionContainers {
			items = append(items, expressionContainerSource(container))
		}
		fields["expressionContainers"] = items
	}
	return fields
}

func ztwWorkloadTagExpressionSource(value ztwworkloadgroups.WorkloadTagExpression) map[string]any {
	fields := map[string]any{}
	if len(value.ExpressionContainers) > 0 {
		items := make([]map[string]any, 0, len(value.ExpressionContainers))
		for _, container := range value.ExpressionContainers {
			items = append(items, ztwExpressionContainerSource(container))
		}
		fields["expressionContainers"] = items
	}
	return fields
}

func expressionContainerSource(value workloadgroups.ExpressionContainer) map[string]any {
	return map[string]any{
		"tagType":      value.TagType,
		"operator":     value.Operator,
		"tagContainer": tagContainerSource(value.TagContainer),
	}
}

func ztwExpressionContainerSource(value ztwworkloadgroups.ExpressionContainer) map[string]any {
	return map[string]any{
		"tagType":      value.TagType,
		"operator":     value.Operator,
		"tagContainer": ztwTagContainerSource(value.TagContainer),
	}
}

func tagContainerSource(value workloadgroups.TagContainer) map[string]any {
	fields := map[string]any{
		"operator": value.Operator,
	}
	if len(value.Tags) > 0 {
		items := make([]map[string]any, 0, len(value.Tags))
		for _, tag := range value.Tags {
			items = append(items, map[string]any{
				"key":   tag.Key,
				"value": tag.Value,
			})
		}
		fields["tags"] = items
	}
	return fields
}

func ztwTagContainerSource(value ztwworkloadgroups.TagContainer) map[string]any {
	fields := map[string]any{
		"operator": value.Operator,
	}
	if len(value.Tags) > 0 {
		items := make([]map[string]any, 0, len(value.Tags))
		for _, tag := range value.Tags {
			items = append(items, map[string]any{
				"key":   tag.Key,
				"value": tag.Value,
			})
		}
		fields["tags"] = items
	}
	return fields
}

func ztwCommonIDNameExternalIDSource(value *ztwcommon.CommonIDNameExternalID) map[string]any {
	fields := map[string]any{
		"id":   value.ID,
		"name": value.Name,
	}
	if value.ExternalID != "" {
		fields["externalId"] = value.ExternalID
	}
	return fields
}

func ztwCommonIDNameExternalIDSliceSource(values []ztwcommon.CommonIDNameExternalID) []any {
	out := make([]any, 0, len(values))
	for i := range values {
		out = append(out, ztwCommonIDNameExternalIDSource(&values[i]))
	}
	return out
}

func ztwRegionStatusSliceSource(values []ztwcommon.RegionStatus) []map[string]any {
	out := make([]map[string]any, 0, len(values))
	for _, value := range values {
		out = append(out, map[string]any{
			"id":        value.ID,
			"name":      value.Name,
			"cloudType": value.CloudType,
			"status":    value.Status,
		})
	}
	return out
}

func ztwSupportedRegionsSliceSource(values []ztwcommon.SupportedRegions) []map[string]any {
	out := make([]map[string]any, 0, len(values))
	for _, value := range values {
		out = append(out, map[string]any{
			"id":        value.ID,
			"name":      value.Name,
			"cloudType": value.CloudType,
		})
	}
	return out
}

func addZTWZPAApplicationSegments(
	fields map[string]any,
	name string,
	values []ztwcommon.ZPAApplicationSegments,
) {
	if len(values) == 0 {
		return
	}
	out := make([]map[string]any, 0, len(values))
	for _, value := range values {
		out = append(out, map[string]any{
			"id":          value.ID,
			"name":        value.Name,
			"description": value.Description,
			"zpaId":       value.ZPAID,
			"deleted":     value.Deleted,
		})
	}
	fields[name] = out
}

func addZTWZPAApplicationSegmentGroups(
	fields map[string]any,
	name string,
	values []ztwcommon.ZPAApplicationSegmentGroups,
) {
	if len(values) == 0 {
		return
	}
	out := make([]map[string]any, 0, len(values))
	for _, value := range values {
		out = append(out, map[string]any{
			"id":                  value.ID,
			"name":                value.Name,
			"zpaId":               value.ZPAID,
			"deleted":             value.Deleted,
			"zpaAppSegmentsCount": value.ZPAAppSegmentsCount,
		})
	}
	fields[name] = out
}

func ztwNetworkPortsSource(values []ztwnetworkservices.NetworkPorts) []any {
	out := make([]any, 0, len(values))
	for _, value := range values {
		out = append(out, map[string]any{
			"start": value.Start,
			"end":   value.End,
		})
	}
	return out
}

func ztwNetworkServiceRefsSource(values []ztwnetworkservicegroups.Services) []any {
	out := make([]any, 0, len(values))
	for _, value := range values {
		out = append(out, map[string]any{
			"id":   value.ID,
			"name": value.Name,
		})
	}
	return out
}

func networkServiceRefsSource(values []networkservicegroups.Services) []any {
	out := make([]any, 0, len(values))
	for _, value := range values {
		out = append(out, map[string]any{
			"id":   value.ID,
			"name": value.Name,
		})
	}
	return out
}

func ztwAdminUserRoleSource(value *ztwadminusers.Role) map[string]any {
	fields := map[string]any{
		"id":            value.ID,
		"name":          value.Name,
		"isNameL10nTag": value.IsNameL10Tag,
	}
	if len(value.Extensions) > 0 {
		fields["extensions"] = value.Extensions
	}
	return fields
}

func ztwExecMobileAppTokensSource(values []ztwadminusers.ExecMobileAppTokens) []any {
	out := make([]any, 0, len(values))
	for _, value := range values {
		out = append(out, map[string]any{
			"cloud":       value.Cloud,
			"orgId":       value.OrgId,
			"name":        value.Name,
			"tokenId":     value.TokenId,
			"token":       value.Token,
			"tokenExpiry": value.TokenExpiry,
			"createTime":  value.CreateTime,
			"deviceId":    value.DeviceId,
			"deviceName":  value.DeviceName,
		})
	}
	return out
}

func zidIDNameDisplayNameSource(value *zidcommon.IDNameDisplayName) map[string]any {
	return map[string]any{
		"id":          value.ID,
		"name":        value.Name,
		"displayName": value.DisplayName,
	}
}

func copyStringAnyMap(values map[string]interface{}) map[string]any {
	out := make(map[string]any, len(values))
	for key, value := range values {
		out[key] = value
	}
	return out
}

func zidentityServiceScopesSource(values []zidresourceservers.ServiceScopes) []any {
	out := make([]any, 0, len(values))
	for _, value := range values {
		out = append(out, map[string]any{
			"service": zidentityServiceRefSource(value.Service),
			"scopes":  zidentityScopesSource(value.Scopes),
		})
	}
	return out
}

func zidentityServiceRefSource(value zidresourceservers.Service) map[string]any {
	return map[string]any{
		"id":          value.ID,
		"name":        value.Name,
		"displayName": value.DisplayName,
	}
}

func zidentityScopesSource(values []zidresourceservers.Scopes) []any {
	out := make([]any, 0, len(values))
	for _, value := range values {
		out = append(out, map[string]any{
			"id":   value.ID,
			"name": value.Name,
		})
	}
	return out
}

func zpaAppSegmentsSource(values []ziacommon.ZPAAppSegments) []any {
	out := make([]any, 0, len(values))
	for _, value := range values {
		fields := map[string]any{
			"id":   value.ID,
			"name": value.Name,
		}
		if value.ExternalID != "" {
			fields["externalId"] = value.ExternalID
		}
		out = append(out, fields)
	}
	return out
}

func zpaGatewayServerGroupSource(value zpagateways.ZPAServerGroup) map[string]any {
	fields := map[string]any{
		"id":   value.ID,
		"name": value.Name,
	}
	if value.ExternalID != "" {
		fields["externalId"] = value.ExternalID
	}
	if len(value.Extensions) > 0 {
		fields["extensions"] = value.Extensions
	}
	return fields
}

func zpaGatewayAppSegmentsSource(values []zpagateways.ZPAAppSegments) []any {
	out := make([]any, 0, len(values))
	for _, value := range values {
		fields := map[string]any{
			"id":   value.ID,
			"name": value.Name,
		}
		if value.ExternalID != "" {
			fields["externalId"] = value.ExternalID
		}
		if len(value.Extensions) > 0 {
			fields["extensions"] = value.Extensions
		}
		out = append(out, fields)
	}
	return out
}

func cbiProfileSource(value *ziacommon.CBIProfile) map[string]any {
	return map[string]any{
		"id":         value.ID,
		"name":       value.Name,
		"url":        value.URL,
		"profileSeq": value.ProfileSeq,
	}
}

func forwardingZPAApplicationSegmentsSource(values []forwardingrules.ZPAApplicationSegments) []any {
	out := make([]any, 0, len(values))
	for _, value := range values {
		out = append(out, map[string]any{
			"id":          value.ID,
			"name":        value.Name,
			"description": value.Description,
			"zpaId":       value.ZPAID,
			"deleted":     value.Deleted,
		})
	}
	return out
}

func forwardingZPAApplicationSegmentGroupsSource(values []forwardingrules.ZPAApplicationSegmentGroups) []any {
	out := make([]any, 0, len(values))
	for _, value := range values {
		out = append(out, map[string]any{
			"id":                  value.ID,
			"name":                value.Name,
			"zpaId":               value.ZPAID,
			"deleted":             value.Deleted,
			"zpaAppSegmentsCount": value.ZPAAppSegmentsCount,
		})
	}
	return out
}

func sslInspectionActionSource(value sslinspection.Action) map[string]any {
	fields := map[string]any{
		"type":                       value.Type,
		"showEUN":                    value.ShowEUN,
		"showEUNATP":                 value.ShowEUNATP,
		"overrideDefaultCertificate": value.OverrideDefaultCertificate,
	}
	if value.SSLInterceptionCert != nil {
		fields["sslInterceptionCert"] = sslInterceptionCertSource(value.SSLInterceptionCert)
	}
	if value.DecryptSubActions != nil {
		fields["decryptSubActions"] = decryptSubActionsSource(value.DecryptSubActions)
	}
	if value.DoNotDecryptSubActions != nil {
		fields["doNotDecryptSubActions"] = doNotDecryptSubActionsSource(value.DoNotDecryptSubActions)
	}
	return fields
}

func sslInterceptionCertSource(value *sslinspection.SSLInterceptionCert) map[string]any {
	return map[string]any{
		"id":                 value.ID,
		"name":               value.Name,
		"defaultCertificate": value.DefaultCertificate,
	}
}

func decryptSubActionsSource(value *sslinspection.DecryptSubActions) map[string]any {
	return map[string]any{
		"serverCertificates":              value.ServerCertificates,
		"ocspCheck":                       value.OcspCheck,
		"blockSslTrafficWithNoSniEnabled": value.BlockSslTrafficWithNoSniEnabled,
		"minClientTLSVersion":             value.MinClientTLSVersion,
		"minServerTLSVersion":             value.MinServerTLSVersion,
		"blockUndecrypt":                  value.BlockUndecrypt,
		"http2Enabled":                    value.HTTP2Enabled,
	}
}

func doNotDecryptSubActionsSource(value *sslinspection.DoNotDecryptSubActions) map[string]any {
	return map[string]any{
		"bypassOtherPolicies":             value.BypassOtherPolicies,
		"serverCertificates":              value.ServerCertificates,
		"ocspCheck":                       value.OcspCheck,
		"blockSslTrafficWithNoSniEnabled": value.BlockSslTrafficWithNoSniEnabled,
		"minTLSVersion":                   value.MinTLSVersion,
	}
}

func urlCategoryScopesSource(values []urlcategories.Scopes) []any {
	out := make([]any, 0, len(values))
	for _, value := range values {
		fields := map[string]any{
			"Type": value.Type,
		}
		if len(value.ScopeEntities) > 0 {
			fields["ScopeEntities"] = idNameExtensionsSliceSource(value.ScopeEntities)
		}
		if len(value.ScopeGroupMemberEntities) > 0 {
			fields["scopeGroupMemberEntities"] = idNameExtensionsSliceSource(value.ScopeGroupMemberEntities)
		}
		out = append(out, fields)
	}
	return out
}

func urlKeywordCountsSource(value *urlcategories.URLKeywordCounts) map[string]any {
	return map[string]any{
		"totalUrlCount":            value.TotalURLCount,
		"retainParentUrlCount":     value.RetainParentURLCount,
		"totalKeywordCount":        value.TotalKeywordCount,
		"retainParentKeywordCount": value.RetainParentKeywordCount,
	}
}

func dynamicLocationGroupCriteriaSource(value *locationgroups.DynamicLocationGroupCriteria) map[string]any {
	fields := map[string]any{
		"enforceAuthentication":  value.EnforceAuthentication,
		"enforceAup":             value.EnforceAup,
		"enforceFirewallControl": value.EnforceFirewallControl,
		"enableXffForwarding":    value.EnableXffForwarding,
		"enableCaution":          value.EnableCaution,
		"enableBandwidthControl": value.EnableBandwidthControl,
	}
	if value.Name != nil {
		fields["name"] = locationGroupMatchSource(value.Name.MatchString, value.Name.MatchType)
	}
	if len(value.Countries) > 0 {
		fields["countries"] = append([]string(nil), value.Countries...)
	}
	if value.City != nil {
		fields["city"] = locationGroupMatchSource(value.City.MatchString, value.City.MatchType)
	}
	if len(value.ManagedBy) > 0 {
		fields["managedBy"] = locationGroupManagedBySliceSource(value.ManagedBy)
	}
	if len(value.Profiles) > 0 {
		fields["profiles"] = append([]string(nil), value.Profiles...)
	}
	return fields
}

func locationGroupMatchSource(matchString string, matchType string) map[string]any {
	return map[string]any{
		"matchString": matchString,
		"matchType":   matchType,
	}
}

func locationGroupManagedBySliceSource(values []locationgroups.ManagedBy) []any {
	out := make([]any, 0, len(values))
	for i := range values {
		fields := map[string]any{
			"id":   values[i].ID,
			"name": values[i].Name,
		}
		out = append(out, fields)
	}
	return out
}

func locationGroupLastModUserSource(value *locationgroups.LastModUser) map[string]any {
	fields := map[string]any{
		"id":   value.ID,
		"name": value.Name,
	}
	return fields
}

func staticIPCitySource(value *staticips.City) map[string]any {
	return map[string]any{
		"id":   value.ID,
		"name": value.Name,
	}
}

func staticIPManagedBySource(value *staticips.ManagedBy) map[string]any {
	fields := map[string]any{
		"id":   value.ID,
		"name": value.Name,
	}
	return fields
}

func staticIPLastModifiedBySource(value *staticips.LastModifiedBy) map[string]any {
	fields := map[string]any{
		"id":   value.ID,
		"name": value.Name,
	}
	return fields
}

func greManagedBySource(value *gretunnels.ManagedBy) map[string]any {
	fields := map[string]any{
		"id":   value.ID,
		"name": value.Name,
	}
	return fields
}

func greLastModifiedBySource(value *gretunnels.LastModifiedBy) map[string]any {
	fields := map[string]any{
		"id":   value.ID,
		"name": value.Name,
	}
	return fields
}

func primaryDestVIPSource(value *gretunnels.PrimaryDestVip) map[string]any {
	return map[string]any{
		"id":                 value.ID,
		"virtualIp":          value.VirtualIP,
		"privateServiceEdge": value.PrivateServiceEdge,
		"datacenter":         value.Datacenter,
		"latitude":           value.Latitude,
		"longitude":          value.Longitude,
		"city":               value.City,
		"countryCode":        value.CountryCode,
		"region":             value.Region,
	}
}

func secondaryDestVIPSource(value *gretunnels.SecondaryDestVip) map[string]any {
	return map[string]any{
		"id":                 value.ID,
		"virtualIp":          value.VirtualIP,
		"privateServiceEdge": value.PrivateServiceEdge,
		"datacenter":         value.Datacenter,
		"latitude":           value.Latitude,
		"longitude":          value.Longitude,
		"city":               value.City,
		"countryCode":        value.CountryCode,
		"region":             value.Region,
	}
}

func boolPointerValue(value *bool) any {
	if value == nil {
		return nil
	}
	return *value
}

func vpnCredentialsSource(credentials []locationmanagement.VPNCredentials) []any {
	out := make([]any, 0, len(credentials))
	for _, credential := range credentials {
		out = append(out, map[string]any{
			"id":           credential.ID,
			"type":         credential.Type,
			"fqdn":         credential.FQDN,
			"ipAddress":    credential.IPAddress,
			"preSharedKey": credential.PreSharedKey,
			"comments":     credential.Comments,
		})
	}
	return out
}

func normalizeLiveError(ctx context.Context, operation string, product resources.Product, resource string, err error) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("zscaler %s %s/%s cancelled: %w", operation, product, resource, err)
	}
	return liveAccessError{
		operation:  operation,
		product:    product,
		resource:   resource,
		statusCode: sdkStatusCode(err),
	}
}

type liveAccessError struct {
	operation  string
	product    resources.Product
	resource   string
	statusCode int
}

func (e liveAccessError) Error() string {
	if e.statusCode > 0 {
		return fmt.Sprintf("%s: %s %s/%s (status %d)", ErrLiveAccessFailed, e.operation, e.product, e.resource, e.statusCode)
	}
	return fmt.Sprintf("%s: %s %s/%s", ErrLiveAccessFailed, e.operation, e.product, e.resource)
}

func (e liveAccessError) Unwrap() error {
	return ErrLiveAccessFailed
}

func sdkStatusCode(err error) int {
	var sdkErr *sdkerrorx.ErrorResponse
	if !errors.As(err, &sdkErr) || sdkErr == nil {
		return 0
	}
	if sdkErr.Response != nil && sdkErr.Response.StatusCode > 0 {
		return sdkErr.Response.StatusCode
	}
	if sdkErr.Parsed != nil && sdkErr.Parsed.Status > 0 {
		return sdkErr.Parsed.Status
	}
	return 0
}
