package zscaler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	sdkcache "github.com/zscaler/zscaler-sdk-go/v3/cache"
	sdklogger "github.com/zscaler/zscaler-sdk-go/v3/logger"
	zsdk "github.com/zscaler/zscaler-sdk-go/v3/zscaler"
	sdkzia "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia"
	"github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/alerts"
	bandwidthclasses "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/bandwidth_control/bandwidth_classes"
	bandwidthcontrolrules "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/bandwidth_control/bandwidth_control_rules"
	cloudappinstances "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/cloud_app_instances"
	ziacommon "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/common"
	"github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/devicegroups"
	dlpicapservers "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/dlp/dlp_icap_servers"
	filetypecontrol "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/filetypecontrol"
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
	"github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/location/locationgroups"
	"github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/location/locationmanagement"
	natcontrol "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/nat_control_policies"
	rulelabels "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/rule_labels"
	sandboxrules "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/sandbox/sandbox_rules"
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
	resourceLocations        = "locations"
	resourceLocationGroups   = "location-groups"
	resourceRuleLabels       = "rule-labels"
	resourceStaticIPs        = "static-ips"
	resourceGRETunnels       = "gre-tunnels"
	resourceSublocations     = "sublocations"
	resourceSSLRules         = "ssl-inspection-rules"
	resourceURLCategories    = "url-categories"
	resourceURLRules         = "url-filtering-rules"
	resourceFirewallRules    = "firewall-filtering-rules"
	resourceForwardingRules  = "forwarding-rules"
	resourceIPSourceGroups   = "ip-source-groups"
	resourceIPDestGroups     = "ip-destination-groups"
	resourceNetworkServices  = "network-services"
	resourceAppServices      = "application-services"
	resourceAppServiceGroups = "application-service-groups"
	resourceNetworkAppGroups = "network-application-groups"
	resourceTimeWindows      = "time-windows"
	resourceProxies          = "proxies"
	resourceProxyGateways    = "proxy-gateways"
	resourceDedicatedIPGWs   = "dedicated-ip-gateways"
	resourceTimeIntervals    = "time-intervals"
	resourceBandwidthClasses = "bandwidth-classes"
	resourceBandwidthRules   = "bandwidth-control-rules"
	resourceDNSGateways      = "dns-gateways"
	resourceNATRules         = "nat-control-rules"
	resourceGroups           = "groups"
	resourceDeviceGroups     = "device-groups"
	resourceWorkloadGroups   = "workload-groups"
	resourceAlertSubs        = "alert-subscriptions"
	resourceCloudAppInsts    = "cloud-app-instances"
	resourceTenancyProfiles  = "tenancy-restriction-profiles"
	resourceVZENClusters     = "vzen-clusters"
	resourceVZENNodes        = "vzen-nodes"
	resourceDLPICAPServers   = "dlp-icap-servers"
	resourceFileTypeRules    = "file-type-rules"
	resourceSandboxRules     = "sandbox-rules"
	resourceFirewallDNSRules = "firewall-dns-rules"
)

type AuthMode string

const (
	AuthModeOneAPI    AuthMode = "oneapi"
	AuthModeZIALegacy AuthMode = "zia-legacy"
)

type ReaderConfig struct {
	ClientID     secret.Secret
	ClientSecret secret.Secret
	VanityDomain string
	Cloud        string
	AuthMode     AuthMode
	ZIALegacy    ZIALegacyConfig
	Proxy        ProxyConfig
	Timeout      time.Duration
	NoCache      bool
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
}

type ziaServiceProvider interface {
	service(context.Context) (*zsdk.Service, func(), error)
}

var (
	_ resourceHandler = listGetHandler[struct{}]{}
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
	ziaClient := sdkZIAClient{services: perCallZIAService{cfg: cfg}}
	return &SDKReader{
		cfg:      cfg,
		handlers: newResourceHandlers(ziaClient),
	}, nil
}

func (r *SDKReader) Session(ctx context.Context, product resources.Product) (ResourceSession, error) {
	if r == nil {
		return nil, fmt.Errorf("%w: %s/session", ErrUnsupportedResource, product)
	}
	if product != resources.ProductZIA {
		return nil, fmt.Errorf("%w: %s/session", ErrUnsupportedResource, product)
	}
	service, cleanup, err := perCallZIAService{cfg: r.cfg}.service(ctx)
	if err != nil {
		return nil, normalizeLiveError(ctx, "authenticate", product, "session")
	}
	ziaClient := sdkZIAClient{services: fixedZIAService{sdkService: service}}
	return &SDKSession{
		handlers: newResourceHandlers(ziaClient),
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
		return nil, normalizeLiveError(ctx, "list", product, name)
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
		if errors.Is(err, ErrInvalidResourceID) {
			return resources.SourceRecord{}, err
		}
		return resources.SourceRecord{}, normalizeLiveError(ctx, "get", product, name)
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

func newResourceHandlers(ziaClient sdkZIAClient) map[resourceKey]resourceHandler {
	return map[resourceKey]resourceHandler{
		{product: resources.ProductZIA, name: resourceLocations}: newListGetHandler(
			resourceLocations,
			ziaSDKList(ziaClient, func(ctx context.Context, service *zsdk.Service) ([]locationmanagement.Locations, error) {
				return locationmanagement.GetAll(ctx, service)
			}),
			ziaSDKGet(ziaClient, func(ctx context.Context, service *zsdk.Service, id int) (*locationmanagement.Locations, error) {
				return locationmanagement.GetLocation(ctx, service, id)
			}),
			locationSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceLocationGroups}: newListGetHandler(
			resourceLocationGroups,
			ziaSDKList(ziaClient, func(ctx context.Context, service *zsdk.Service) ([]locationgroups.LocationGroup, error) {
				fetchLocations := false
				return locationgroups.GetAll(ctx, service, &locationgroups.GetAllFilterOptions{
					FetchLocations: &fetchLocations,
				})
			}),
			ziaSDKGet(ziaClient, func(ctx context.Context, service *zsdk.Service, id int) (*locationgroups.LocationGroup, error) {
				return locationgroups.GetLocationGroup(ctx, service, id)
			}),
			locationGroupSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceRuleLabels}: newListGetHandler(
			resourceRuleLabels,
			ziaSDKList(ziaClient, func(ctx context.Context, service *zsdk.Service) ([]rulelabels.RuleLabels, error) {
				return rulelabels.GetAll(ctx, service)
			}),
			ziaSDKGet(ziaClient, func(ctx context.Context, service *zsdk.Service, id int) (*rulelabels.RuleLabels, error) {
				return rulelabels.Get(ctx, service, id)
			}),
			ruleLabelSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceStaticIPs}: newListGetHandler(
			resourceStaticIPs,
			ziaSDKList(ziaClient, func(ctx context.Context, service *zsdk.Service) ([]staticips.StaticIP, error) {
				return staticips.GetAll(ctx, service)
			}),
			ziaSDKGet(ziaClient, func(ctx context.Context, service *zsdk.Service, id int) (*staticips.StaticIP, error) {
				return staticips.Get(ctx, service, id)
			}),
			staticIPSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceGRETunnels}: newListGetHandler(
			resourceGRETunnels,
			ziaSDKList(ziaClient, func(ctx context.Context, service *zsdk.Service) ([]gretunnels.GreTunnels, error) {
				return gretunnels.GetAll(ctx, service)
			}),
			ziaSDKGet(ziaClient, func(ctx context.Context, service *zsdk.Service, id int) (*gretunnels.GreTunnels, error) {
				return gretunnels.GetGreTunnels(ctx, service, id)
			}),
			greTunnelSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceSublocations}: newListGetHandler(
			resourceSublocations,
			ziaSDKList(ziaClient, func(ctx context.Context, service *zsdk.Service) ([]locationmanagement.Locations, error) {
				return locationmanagement.GetAllSublocations(ctx, service)
			}),
			ziaSDKGet(ziaClient, func(ctx context.Context, service *zsdk.Service, id int) (*locationmanagement.Locations, error) {
				return locationmanagement.GetSubLocationBySubID(ctx, service, id)
			}),
			sublocationSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceSSLRules}: newListGetHandler(
			resourceSSLRules,
			ziaSDKList(ziaClient, func(ctx context.Context, service *zsdk.Service) ([]sslinspection.SSLInspectionRules, error) {
				return sslinspection.GetAll(ctx, service)
			}),
			ziaSDKGet(ziaClient, func(ctx context.Context, service *zsdk.Service, id int) (*sslinspection.SSLInspectionRules, error) {
				return sslinspection.Get(ctx, service, id)
			}),
			sslInspectionRuleSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceURLCategories}: newListGetHandler(
			resourceURLCategories,
			ziaSDKList(ziaClient, func(ctx context.Context, service *zsdk.Service) ([]urlcategories.URLCategory, error) {
				return urlcategories.GetAll(ctx, service, false, true, "")
			}),
			ziaSDKStringGet(ziaClient, func(ctx context.Context, service *zsdk.Service, id string) (*urlcategories.URLCategory, error) {
				return urlcategories.Get(ctx, service, id)
			}),
			urlCategorySourceRecord,
		),
		{product: resources.ProductZIA, name: resourceURLRules}: newListGetHandler(
			resourceURLRules,
			ziaSDKList(ziaClient, func(ctx context.Context, service *zsdk.Service) ([]urlfilteringpolicies.URLFilteringRule, error) {
				return urlfilteringpolicies.GetAll(ctx, service)
			}),
			ziaSDKGet(ziaClient, func(ctx context.Context, service *zsdk.Service, id int) (*urlfilteringpolicies.URLFilteringRule, error) {
				return urlfilteringpolicies.Get(ctx, service, id)
			}),
			urlFilteringRuleSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceFirewallRules}: newListGetHandler(
			resourceFirewallRules,
			ziaSDKList(ziaClient, func(ctx context.Context, service *zsdk.Service) ([]filteringrules.FirewallFilteringRules, error) {
				return filteringrules.GetAll(ctx, service, nil)
			}),
			ziaSDKGet(ziaClient, func(ctx context.Context, service *zsdk.Service, id int) (*filteringrules.FirewallFilteringRules, error) {
				return filteringrules.Get(ctx, service, id)
			}),
			firewallFilteringRuleSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceForwardingRules}: newListGetHandler(
			resourceForwardingRules,
			ziaSDKList(ziaClient, func(ctx context.Context, service *zsdk.Service) ([]forwardingrules.ForwardingRules, error) {
				return forwardingrules.GetAll(ctx, service)
			}),
			ziaSDKGet(ziaClient, func(ctx context.Context, service *zsdk.Service, id int) (*forwardingrules.ForwardingRules, error) {
				return forwardingrules.Get(ctx, service, id)
			}),
			forwardingRuleSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceIPSourceGroups}: newListGetHandler(
			resourceIPSourceGroups,
			ziaSDKList(ziaClient, func(ctx context.Context, service *zsdk.Service) ([]ipsourcegroups.IPSourceGroups, error) {
				return ipsourcegroups.GetAll(ctx, service)
			}),
			ziaSDKGet(ziaClient, func(ctx context.Context, service *zsdk.Service, id int) (*ipsourcegroups.IPSourceGroups, error) {
				return ipsourcegroups.Get(ctx, service, id)
			}),
			ipSourceGroupSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceIPDestGroups}: newListGetHandler(
			resourceIPDestGroups,
			ziaSDKList(ziaClient, func(ctx context.Context, service *zsdk.Service) ([]ipdestinationgroups.IPDestinationGroups, error) {
				return ipdestinationgroups.GetAll(ctx, service, "")
			}),
			ziaSDKGet(ziaClient, func(ctx context.Context, service *zsdk.Service, id int) (*ipdestinationgroups.IPDestinationGroups, error) {
				return ipdestinationgroups.Get(ctx, service, id)
			}),
			ipDestinationGroupSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceNetworkServices}: newListGetHandler(
			resourceNetworkServices,
			ziaSDKList(ziaClient, func(ctx context.Context, service *zsdk.Service) ([]networkservices.NetworkServices, error) {
				return networkservices.GetAllNetworkServices(ctx, service, nil, nil)
			}),
			ziaSDKGet(ziaClient, func(ctx context.Context, service *zsdk.Service, id int) (*networkservices.NetworkServices, error) {
				return networkservices.Get(ctx, service, id)
			}),
			networkServiceSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceAppServices}: newListGetHandler(
			resourceAppServices,
			ziaSDKList(ziaClient, func(ctx context.Context, service *zsdk.Service) ([]applicationservices.ApplicationServicesLite, error) {
				return applicationservices.GetAll(ctx, service)
			}),
			ziaSDKListGetByIntID(
				ziaClient,
				func(ctx context.Context, service *zsdk.Service) ([]applicationservices.ApplicationServicesLite, error) {
					return applicationservices.GetAll(ctx, service)
				},
				func(item applicationservices.ApplicationServicesLite) int { return item.ID },
			),
			applicationServiceSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceAppServiceGroups}: newListGetHandler(
			resourceAppServiceGroups,
			ziaSDKList(ziaClient, func(ctx context.Context, service *zsdk.Service) ([]appservicegroups.ApplicationServicesGroupLite, error) {
				return appservicegroups.GetAll(ctx, service)
			}),
			ziaSDKListGetByIntID(
				ziaClient,
				func(ctx context.Context, service *zsdk.Service) ([]appservicegroups.ApplicationServicesGroupLite, error) {
					return appservicegroups.GetAll(ctx, service)
				},
				func(item appservicegroups.ApplicationServicesGroupLite) int { return item.ID },
			),
			applicationServiceGroupSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceNetworkAppGroups}: newListGetHandler(
			resourceNetworkAppGroups,
			ziaSDKList(ziaClient, func(ctx context.Context, service *zsdk.Service) ([]networkapplicationgroups.NetworkApplicationGroups, error) {
				return networkapplicationgroups.GetAllNetworkApplicationGroups(ctx, service)
			}),
			ziaSDKGet(ziaClient, func(ctx context.Context, service *zsdk.Service, id int) (*networkapplicationgroups.NetworkApplicationGroups, error) {
				return networkapplicationgroups.GetNetworkApplicationGroups(ctx, service, id)
			}),
			networkApplicationGroupSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceTimeWindows}: newListGetHandler(
			resourceTimeWindows,
			ziaSDKList(ziaClient, func(ctx context.Context, service *zsdk.Service) ([]timewindow.TimeWindow, error) {
				return timewindow.GetAll(ctx, service)
			}),
			ziaSDKListGetByIntID(
				ziaClient,
				func(ctx context.Context, service *zsdk.Service) ([]timewindow.TimeWindow, error) {
					return timewindow.GetAll(ctx, service)
				},
				func(item timewindow.TimeWindow) int { return item.ID },
			),
			timeWindowSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceProxies}: newListGetHandler(
			resourceProxies,
			ziaSDKList(ziaClient, func(ctx context.Context, service *zsdk.Service) ([]proxies.Proxies, error) {
				return proxies.GetAll(ctx, service)
			}),
			ziaSDKGet(ziaClient, func(ctx context.Context, service *zsdk.Service, id int) (*proxies.Proxies, error) {
				return proxies.Get(ctx, service, id)
			}),
			proxySourceRecord,
		),
		{product: resources.ProductZIA, name: resourceProxyGateways}: newListGetHandler(
			resourceProxyGateways,
			ziaSDKList(ziaClient, func(ctx context.Context, service *zsdk.Service) ([]proxygateways.ProxyGateways, error) {
				return proxygateways.GetAll(ctx, service)
			}),
			ziaSDKListGetByIntID(
				ziaClient,
				func(ctx context.Context, service *zsdk.Service) ([]proxygateways.ProxyGateways, error) {
					return proxygateways.GetAll(ctx, service)
				},
				func(item proxygateways.ProxyGateways) int { return item.ID },
			),
			proxyGatewaySourceRecord,
		),
		{product: resources.ProductZIA, name: resourceDedicatedIPGWs}: newListGetHandler(
			resourceDedicatedIPGWs,
			ziaSDKList(ziaClient, func(ctx context.Context, service *zsdk.Service) ([]proxies.DedicatedIPGateways, error) {
				return proxies.GetDedicatedIPGWLite(ctx, service)
			}),
			ziaSDKListGetByIntID(
				ziaClient,
				func(ctx context.Context, service *zsdk.Service) ([]proxies.DedicatedIPGateways, error) {
					return proxies.GetDedicatedIPGWLite(ctx, service)
				},
				func(item proxies.DedicatedIPGateways) int { return item.Id },
			),
			dedicatedIPGatewaySourceRecord,
		),
		{product: resources.ProductZIA, name: resourceTimeIntervals}: newListGetHandler(
			resourceTimeIntervals,
			ziaSDKList(ziaClient, func(ctx context.Context, service *zsdk.Service) ([]timeintervals.TimeInterval, error) {
				return timeintervals.GetAll(ctx, service)
			}),
			ziaSDKGet(ziaClient, func(ctx context.Context, service *zsdk.Service, id int) (*timeintervals.TimeInterval, error) {
				return timeintervals.Get(ctx, service, id)
			}),
			timeIntervalSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceBandwidthClasses}: newListGetHandler(
			resourceBandwidthClasses,
			ziaSDKList(ziaClient, func(ctx context.Context, service *zsdk.Service) ([]bandwidthclasses.BandwidthClasses, error) {
				return bandwidthclasses.GetAll(ctx, service)
			}),
			ziaSDKGet(ziaClient, func(ctx context.Context, service *zsdk.Service, id int) (*bandwidthclasses.BandwidthClasses, error) {
				return bandwidthclasses.Get(ctx, service, id)
			}),
			bandwidthClassSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceBandwidthRules}: newListGetHandler(
			resourceBandwidthRules,
			ziaSDKList(ziaClient, func(ctx context.Context, service *zsdk.Service) ([]bandwidthcontrolrules.BandwidthControlRules, error) {
				return bandwidthcontrolrules.GetAll(ctx, service)
			}),
			ziaSDKGet(ziaClient, func(ctx context.Context, service *zsdk.Service, id int) (*bandwidthcontrolrules.BandwidthControlRules, error) {
				return bandwidthcontrolrules.Get(ctx, service, id)
			}),
			bandwidthControlRuleSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceDNSGateways}: newListGetHandler(
			resourceDNSGateways,
			ziaSDKList(ziaClient, func(ctx context.Context, service *zsdk.Service) ([]dnsgateways.DNSGateways, error) {
				return dnsgateways.GetAll(ctx, service)
			}),
			ziaSDKGet(ziaClient, func(ctx context.Context, service *zsdk.Service, id int) (*dnsgateways.DNSGateways, error) {
				return dnsgateways.Get(ctx, service, id)
			}),
			dnsGatewaySourceRecord,
		),
		{product: resources.ProductZIA, name: resourceNATRules}: newListGetHandler(
			resourceNATRules,
			ziaSDKList(ziaClient, func(ctx context.Context, service *zsdk.Service) ([]natcontrol.NatControlPolicies, error) {
				return natcontrol.GetAll(ctx, service)
			}),
			ziaSDKGet(ziaClient, func(ctx context.Context, service *zsdk.Service, id int) (*natcontrol.NatControlPolicies, error) {
				return natcontrol.Get(ctx, service, id)
			}),
			natControlRuleSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceGroups}: newListGetHandler(
			resourceGroups,
			ziaSDKList(ziaClient, func(ctx context.Context, service *zsdk.Service) ([]usergroups.Groups, error) {
				return usergroups.GetAllGroups(ctx, service, nil)
			}),
			ziaSDKGet(ziaClient, func(ctx context.Context, service *zsdk.Service, id int) (*usergroups.Groups, error) {
				return usergroups.GetGroups(ctx, service, id)
			}),
			groupSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceDeviceGroups}: newListGetHandler(
			resourceDeviceGroups,
			ziaSDKList(ziaClient, func(ctx context.Context, service *zsdk.Service) ([]devicegroups.DeviceGroups, error) {
				return devicegroups.GetAllDevicesGroups(ctx, service)
			}),
			ziaSDKListGetByIntID(
				ziaClient,
				func(ctx context.Context, service *zsdk.Service) ([]devicegroups.DeviceGroups, error) {
					return devicegroups.GetAllDevicesGroups(ctx, service)
				},
				func(item devicegroups.DeviceGroups) int { return item.ID },
			),
			deviceGroupSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceWorkloadGroups}: newListGetHandler(
			resourceWorkloadGroups,
			ziaSDKList(ziaClient, func(ctx context.Context, service *zsdk.Service) ([]workloadgroups.WorkloadGroup, error) {
				return workloadgroups.GetAll(ctx, service)
			}),
			ziaSDKGet(ziaClient, func(ctx context.Context, service *zsdk.Service, id int) (*workloadgroups.WorkloadGroup, error) {
				return workloadgroups.Get(ctx, service, id)
			}),
			workloadGroupSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceAlertSubs}: newListGetHandler(
			resourceAlertSubs,
			ziaSDKList(ziaClient, func(ctx context.Context, service *zsdk.Service) ([]alerts.AlertSubscriptions, error) {
				return alerts.GetAll(ctx, service)
			}),
			ziaSDKGet(ziaClient, func(ctx context.Context, service *zsdk.Service, id int) (*alerts.AlertSubscriptions, error) {
				return alerts.Get(ctx, service, id)
			}),
			alertSubscriptionSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceCloudAppInsts}: newListGetHandler(
			resourceCloudAppInsts,
			ziaSDKList(ziaClient, func(ctx context.Context, service *zsdk.Service) ([]cloudappinstances.CloudApplicationInstances, error) {
				return cloudappinstances.GetAll(ctx, service)
			}),
			ziaSDKGet(ziaClient, func(ctx context.Context, service *zsdk.Service, id int) (*cloudappinstances.CloudApplicationInstances, error) {
				return cloudappinstances.Get(ctx, service, id)
			}),
			cloudAppInstanceSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceTenancyProfiles}: newListGetHandler(
			resourceTenancyProfiles,
			ziaSDKList(ziaClient, func(ctx context.Context, service *zsdk.Service) ([]tenancyrestriction.TenancyRestrictionProfile, error) {
				return tenancyrestriction.GetAll(ctx, service)
			}),
			ziaSDKGet(ziaClient, func(ctx context.Context, service *zsdk.Service, id int) (*tenancyrestriction.TenancyRestrictionProfile, error) {
				return tenancyrestriction.Get(ctx, service, id)
			}),
			tenancyRestrictionProfileSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceVZENClusters}: newListGetHandler(
			resourceVZENClusters,
			ziaSDKList(ziaClient, func(ctx context.Context, service *zsdk.Service) ([]vzenclusters.VZENClusters, error) {
				return vzenclusters.GetAll(ctx, service)
			}),
			ziaSDKGet(ziaClient, func(ctx context.Context, service *zsdk.Service, id int) (*vzenclusters.VZENClusters, error) {
				return vzenclusters.Get(ctx, service, id)
			}),
			vzenClusterSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceVZENNodes}: newListGetHandler(
			resourceVZENNodes,
			ziaSDKList(ziaClient, func(ctx context.Context, service *zsdk.Service) ([]vzennodes.VZENNodes, error) {
				return vzennodes.GetAll(ctx, service)
			}),
			ziaSDKGet(ziaClient, func(ctx context.Context, service *zsdk.Service, id int) (*vzennodes.VZENNodes, error) {
				return vzennodes.Get(ctx, service, id)
			}),
			vzenNodeSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceDLPICAPServers}: newListGetHandler(
			resourceDLPICAPServers,
			ziaSDKList(ziaClient, func(ctx context.Context, service *zsdk.Service) ([]dlpicapservers.DLPICAPServers, error) {
				return dlpicapservers.GetAll(ctx, service)
			}),
			ziaSDKGet(ziaClient, func(ctx context.Context, service *zsdk.Service, id int) (*dlpicapservers.DLPICAPServers, error) {
				return dlpicapservers.Get(ctx, service, id)
			}),
			dlpICAPServerSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceFileTypeRules}: newListGetHandler(
			resourceFileTypeRules,
			ziaSDKList(ziaClient, func(ctx context.Context, service *zsdk.Service) ([]filetypecontrol.FileTypeRules, error) {
				return filetypecontrol.GetAll(ctx, service)
			}),
			ziaSDKGet(ziaClient, func(ctx context.Context, service *zsdk.Service, id int) (*filetypecontrol.FileTypeRules, error) {
				return filetypecontrol.Get(ctx, service, id)
			}),
			fileTypeRuleSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceSandboxRules}: newListGetHandler(
			resourceSandboxRules,
			ziaSDKList(ziaClient, func(ctx context.Context, service *zsdk.Service) ([]sandboxrules.SandboxRules, error) {
				return sandboxrules.GetAll(ctx, service)
			}),
			ziaSDKGet(ziaClient, func(ctx context.Context, service *zsdk.Service, id int) (*sandboxrules.SandboxRules, error) {
				return sandboxrules.Get(ctx, service, id)
			}),
			sandboxRuleSourceRecord,
		),
		{product: resources.ProductZIA, name: resourceFirewallDNSRules}: newListGetHandler(
			resourceFirewallDNSRules,
			ziaSDKList(ziaClient, func(ctx context.Context, service *zsdk.Service) ([]firewalldnscontrolpolicies.FirewallDNSRules, error) {
				return firewalldnscontrolpolicies.GetAll(ctx, service)
			}),
			ziaSDKGet(ziaClient, func(ctx context.Context, service *zsdk.Service, id int) (*firewalldnscontrolpolicies.FirewallDNSRules, error) {
				return firewalldnscontrolpolicies.Get(ctx, service, id)
			}),
			firewallDNSRuleSourceRecord,
		),
	}
}

type listGetHandler[T any] struct {
	resourceName string
	list         func(context.Context) ([]T, error)
	get          func(context.Context, string) (*T, error)
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

func parsePositiveIntID(id string) (int, error) {
	parsed, err := strconv.Atoi(id)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("%w: %q", ErrInvalidResourceID, id)
	}
	return parsed, nil
}

type sdkZIAClient struct {
	services ziaServiceProvider
}

func (c sdkZIAClient) service(ctx context.Context) (*zsdk.Service, func(), error) {
	if c.services == nil {
		return nil, nil, errors.New("missing zia service provider")
	}
	return c.services.service(ctx)
}

func ziaSDKList[T any](
	client sdkZIAClient,
	call func(context.Context, *zsdk.Service) ([]T, error),
) func(context.Context) ([]T, error) {
	return func(ctx context.Context) ([]T, error) {
		service, cleanup, err := client.service(ctx)
		if err != nil {
			return nil, err
		}
		defer cleanup()
		return call(ctx, service)
	}
}

func ziaSDKGet[T any](
	client sdkZIAClient,
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
	client sdkZIAClient,
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
	client sdkZIAClient,
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

func intIDGetter[T any](get func(context.Context, int) (*T, error)) func(context.Context, string) (*T, error) {
	return func(ctx context.Context, id string) (*T, error) {
		parsed, err := parsePositiveIntID(id)
		if err != nil {
			return nil, err
		}
		return get(ctx, parsed)
	}
}

type perCallZIAService struct {
	cfg ReaderConfig
}

func (s perCallZIAService) service(ctx context.Context) (*zsdk.Service, func(), error) {
	if s.cfg.AuthMode == AuthModeZIALegacy {
		return s.legacyService(ctx)
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

func (s perCallZIAService) legacyService(ctx context.Context) (*zsdk.Service, func(), error) {
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

type fixedZIAService struct {
	sdkService *zsdk.Service
}

func (s fixedZIAService) service(ctx context.Context) (*zsdk.Service, func(), error) {
	if err := effectiveContext(ctx).Err(); err != nil {
		return nil, nil, err
	}
	if s.sdkService == nil {
		return nil, nil, errors.New("missing zscaler sdk service")
	}
	return s.sdkService, func() {}, nil
}

func newLegacyZIAClient(cfg *sdkzia.Configuration) (*sdkzia.Client, error) {
	restore := suppressSDKLogEnv()
	defer restore()
	return sdkzia.NewClient(cfg)
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

func addNetworkPorts(fields map[string]any, name string, values []networkservices.NetworkPorts) {
	if len(values) > 0 {
		fields[name] = networkPortsSource(values)
	}
}

func idNameExtensionsSource(value *ziacommon.IDNameExtensions) map[string]any {
	fields := map[string]any{
		"id":   value.ID,
		"name": value.Name,
	}
	return fields
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

func expressionContainerSource(value workloadgroups.ExpressionContainer) map[string]any {
	return map[string]any{
		"tagType":      value.TagType,
		"operator":     value.Operator,
		"tagContainer": tagContainerSource(value.TagContainer),
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

func normalizeLiveError(ctx context.Context, operation string, product resources.Product, resource string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("zscaler %s %s/%s cancelled: %w", operation, product, resource, err)
	}
	return liveAccessError{
		operation: operation,
		product:   product,
		resource:  resource,
	}
}

type liveAccessError struct {
	operation string
	product   resources.Product
	resource  string
}

func (e liveAccessError) Error() string {
	return fmt.Sprintf("%s: %s %s/%s", ErrLiveAccessFailed, e.operation, e.product, e.resource)
}

func (e liveAccessError) Unwrap() error {
	return ErrLiveAccessFailed
}
