package zscaler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	sdkerrorx "github.com/zscaler/zscaler-sdk-go/v3/zscaler/errorx"
	ziaadminusers "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/adminuserrolemgmt/admins"
	ziaadminroles "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/adminuserrolemgmt/roles"
	"github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/alerts"
	authsettings "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/auth_settings"
	bandwidthclasses "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/bandwidth_control/bandwidth_classes"
	bandwidthcontrolrules "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/bandwidth_control/bandwidth_control_rules"
	browsercontrolsettings "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/browser_control_settings"
	browserisolation "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/browser_isolation"
	cloudappinstances "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/cloud_app_instances"
	cloudapplications "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/cloudapplications/cloudapplications"
	ziacommon "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/common"
	"github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/devicegroups"
	dlpedmlite "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/dlp/dlp_exact_data_match_lite"
	dlpicapservers "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/dlp/dlp_icap_servers"
	dlpincidentreceivers "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/dlp/dlp_incident_receiver_servers"
	dlpnotificationtemplates "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/dlp/dlp_notification_templates"
	dlpwebrules "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/dlp/dlp_web_rules"
	emailprofiles "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/email_profiles"
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
	ftpcontrolpolicy "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/ftp_control_policy"
	"github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/location/locationgroups"
	"github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/location/locationmanagement"
	natcontrol "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/nat_control_policies"
	pacfiles "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/pacfiles"
	remoteassistance "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/remote_assistance"
	rulelabels "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/rule_labels"
	saassecurityapi "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/saas_security_api"
	casbdlprules "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/saas_security_api/casb_dlp_rules"
	casbmalwarerules "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/saas_security_api/casb_malware_rules"
	securebrowsing "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/secure_browsing"
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
	zidgroups "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zid/services/groups"
	zidresourceservers "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zid/services/resource_servers"
	zidusers "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zid/services/users"
	zpaappconnectorcontroller "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/appconnectorcontroller"
	zpaappconnectorgroup "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/appconnectorgroup"
	zpaapplicationsegment "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/applicationsegment"
	zpaapplicationsegmentbrowseraccess "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/applicationsegmentbrowseraccess"
	zpaappservercontroller "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/appservercontroller"
	zpac2cipranges "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/c2c_ip_ranges"
	zpacloudconnector "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/cloud_connector"
	zpacloudconnectorgroup "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/cloud_connector_group"
	zpacbizpaprofile "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/cloudbrowserisolation/cbizpaprofile"
	zpacommon "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/common"
	zpaconfigoverride "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/config_override"
	zpapostureprofile "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/postureprofile"
	zpaservergroup "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/servergroup"
	zpaserviceedgecontroller "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/serviceedgecontroller"
	zpaserviceedgegroup "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/serviceedgegroup"
	ztwadminroles "github.com/zscaler/zscaler-sdk-go/v3/zscaler/ztw/services/adminuserrolemgmt/adminroles"
	ztwadminusers "github.com/zscaler/zscaler-sdk-go/v3/zscaler/ztw/services/adminuserrolemgmt/adminusers"
	ztwcommon "github.com/zscaler/zscaler-sdk-go/v3/zscaler/ztw/services/common"
	ztwdnsgateway "github.com/zscaler/zscaler-sdk-go/v3/zscaler/ztw/services/dns_gateway"
	ztwecgroup "github.com/zscaler/zscaler-sdk-go/v3/zscaler/ztw/services/ecgroup"
	ztwziaforwardinggateway "github.com/zscaler/zscaler-sdk-go/v3/zscaler/ztw/services/forwarding_gateways/zia_forwarding_gateway"
	ztwlocation "github.com/zscaler/zscaler-sdk-go/v3/zscaler/ztw/services/locationmanagement/location"
	ztwlocationtemplate "github.com/zscaler/zscaler-sdk-go/v3/zscaler/ztw/services/locationmanagement/locationtemplate"
	ztwaccountgroups "github.com/zscaler/zscaler-sdk-go/v3/zscaler/ztw/services/partner_integrations/account_groups"
	ztwpubliccloudinfo "github.com/zscaler/zscaler-sdk-go/v3/zscaler/ztw/services/partner_integrations/public_cloud_info"
	ztwforwardingrules "github.com/zscaler/zscaler-sdk-go/v3/zscaler/ztw/services/policy_management/forwarding_rules"
	ztwtrafficdnsrules "github.com/zscaler/zscaler-sdk-go/v3/zscaler/ztw/services/policy_management/traffic_dns_rules"
	ztwtrafficlogrules "github.com/zscaler/zscaler-sdk-go/v3/zscaler/ztw/services/policy_management/traffic_log_rules"
	ztwipdestinationgroups "github.com/zscaler/zscaler-sdk-go/v3/zscaler/ztw/services/policyresources/ipdestinationgroups"
	ztwipgroups "github.com/zscaler/zscaler-sdk-go/v3/zscaler/ztw/services/policyresources/ipgroups"
	ztwipsourcegroups "github.com/zscaler/zscaler-sdk-go/v3/zscaler/ztw/services/policyresources/ipsourcegroups"
	ztwnetworkservicegroups "github.com/zscaler/zscaler-sdk-go/v3/zscaler/ztw/services/policyresources/networkservicegroups"
	ztwnetworkservices "github.com/zscaler/zscaler-sdk-go/v3/zscaler/ztw/services/policyresources/networkservices"
	ztwzparesources "github.com/zscaler/zscaler-sdk-go/v3/zscaler/ztw/services/policyresources/zparesources"
	ztwpubliccloudaccount "github.com/zscaler/zscaler-sdk-go/v3/zscaler/ztw/services/provisioning/public_cloud_account"
	ztwworkloadgroups "github.com/zscaler/zscaler-sdk-go/v3/zscaler/ztw/services/workload_groups"

	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
	"github.com/dvmrry/zscalerctl/internal/secret"
)

type singletonTestRecord struct {
	ID   int
	Name string
}

func TestSingletonHandlerShowMapsResponse(t *testing.T) {
	t.Parallel()

	handler := newSingletonHandler(
		"test-settings",
		func(context.Context) (*singletonTestRecord, error) {
			return &singletonTestRecord{ID: 42, Name: "mapped"}, nil
		},
		func(record singletonTestRecord) resources.SourceRecord {
			return resources.NewSourceRecord(map[string]any{
				"id":   record.ID,
				"name": record.Name,
			})
		},
	)

	record, err := handler.Show(context.Background())
	if err != nil {
		t.Fatalf("singletonHandler.Show() error = %v, want nil", err)
	}
	spec := resources.ResourceSpec{
		Product:    resources.ProductZIA,
		Name:       "test-settings",
		Operations: resources.ShowOperation(),
		Fields: []resources.FieldSpec{
			{
				Name:           "id",
				Classification: resources.ClassOperational,
				AllowedModes:   []redact.Mode{redact.ModeStandard},
			},
			{
				Name:           "name",
				Classification: resources.ClassTenantConfig,
				AllowedModes:   []redact.Mode{redact.ModeStandard},
			},
		},
	}
	projected, _, err := resources.ProjectRecordAndVerify(spec, redact.ModeStandard, record)
	if err != nil {
		t.Fatalf("ProjectRecordAndVerify() error = %v, want nil", err)
	}
	body, err := json.Marshal(projected)
	if err != nil {
		t.Fatalf("json.Marshal(projected) error = %v, want nil", err)
	}
	if got, want := string(body), `{"id":42,"name":"mapped"}`; got != want {
		t.Errorf("singletonHandler.Show() projected JSON = %s, want %s", got, want)
	}
}

func TestSingletonHandlerRejectsGet(t *testing.T) {
	t.Parallel()

	handler := newSingletonHandler(
		"test-settings",
		func(context.Context) (*singletonTestRecord, error) {
			t.Fatal("singletonHandler.Get() called show")
			return nil, nil
		},
		func(singletonTestRecord) resources.SourceRecord {
			t.Fatal("singletonHandler.Get() called mapper")
			return resources.SourceRecord{}
		},
	)

	if _, err := handler.Get(context.Background(), "1"); !errors.Is(err, ErrUnsupportedResource) {
		t.Fatalf("singletonHandler.Get() error = %v, want ErrUnsupportedResource", err)
	}
}

func TestSingletonHandlerShowRejectsNilResponse(t *testing.T) {
	t.Parallel()

	handler := newSingletonHandler(
		"test-settings",
		func(context.Context) (*singletonTestRecord, error) {
			return nil, nil
		},
		func(singletonTestRecord) resources.SourceRecord {
			t.Fatal("singletonHandler.Show(nil) called mapper")
			return resources.SourceRecord{}
		},
	)

	if _, err := handler.Show(context.Background()); err == nil {
		t.Fatal("singletonHandler.Show(nil) error = nil, want error")
	} else if !strings.Contains(err.Error(), "empty sdk test-settings response") {
		t.Fatalf("singletonHandler.Show(nil) error = %v, want empty sdk response error", err)
	}
}

func TestNewReaderRequiresExplicitZscalerctlCredentials(t *testing.T) {
	t.Parallel()

	if _, err := NewReader(ReaderConfig{}); !errors.Is(err, ErrMissingCredentials) {
		t.Fatalf("NewReader(empty) error = %v, want ErrMissingCredentials", err)
	}
}

func TestPerCallServiceRejectsZTWWithLegacyZIACredentials(t *testing.T) {
	t.Parallel()

	service := perCallService{cfg: validLegacyReaderConfig()}

	_, _, err := service.service(context.Background(), resources.ProductZTW)
	if !errors.Is(err, ErrMissingCredentials) {
		t.Fatalf("perCallService.service(ctx, ztw) error = %v, want ErrMissingCredentials", err)
	}
}

func TestNewReaderIgnoresSDKEnvironmentNames(t *testing.T) {
	t.Setenv("ZSCALER_CLIENT_ID", "sdk-client-id")
	t.Setenv("ZSCALER_CLIENT_SECRET", "sdk-client-secret")
	t.Setenv("ZSCALER_VANITY_DOMAIN", "sdk-vanity")

	if _, err := NewReader(ReaderConfig{}); !errors.Is(err, ErrMissingCredentials) {
		t.Fatalf("NewReader(empty with SDK env) error = %v, want ErrMissingCredentials", err)
	}
}

func TestNewReaderAcceptsZIALegacyCredentials(t *testing.T) {
	t.Parallel()

	if _, err := NewReader(validLegacyReaderConfig()); err != nil {
		t.Fatalf("NewReader(valid ZIA legacy) error = %v, want nil", err)
	}
}

func TestNewReaderRequiresZIALegacyCredentials(t *testing.T) {
	t.Parallel()

	cfg := validLegacyReaderConfig()
	cfg.ZIALegacy.APIKey = secret.Secret{}
	if _, err := NewReader(cfg); !errors.Is(err, ErrMissingCredentials) {
		t.Fatalf("NewReader(missing ZIA legacy API key) error = %v, want ErrMissingCredentials", err)
	}
}

func TestZPALegacyAuthFailsClosedBeforeServiceConstruction(t *testing.T) {
	t.Parallel()

	service := perCallService{cfg: validLegacyReaderConfig()}
	_, _, err := service.service(context.Background(), resources.ProductZPA)
	if !errors.Is(err, ErrMissingCredentials) {
		t.Fatalf("perCallService.service(zpa with legacy auth) error = %v, want ErrMissingCredentials", err)
	}
}

func TestNewSDKConfigurationDoesNotUseSDKDiscoveryOrLogging(t *testing.T) {
	t.Setenv("ZSCALER_CLIENT_ID", "sdk-client-id")
	t.Setenv("ZSCALER_CLIENT_SECRET", "sdk-client-secret")
	t.Setenv("ZSCALER_VANITY_DOMAIN", "sdk-vanity")
	t.Setenv("ZSCALER_CLOUD", "sdk-cloud")
	t.Setenv("ZPA_CUSTOMER_ID", "sdk-zpa-customer-id")
	t.Setenv("ZPA_MICROTENANT_ID", "sdk-zpa-microtenant-id")
	t.Setenv("ZSCALER_CLIENT_PROXY_HOST", "sdk-proxy.example.invalid")
	t.Setenv("ZSCALER_CLIENT_CACHE_ENABLED", "true")
	t.Setenv("ZSCALER_SDK_LOG", "true")
	t.Setenv("ZSCALER_SDK_VERBOSE", "true")
	t.Setenv("HTTPS_PROXY", "http://standard-proxy.example.invalid:8080")

	cfg := newSDKConfiguration(context.Background(), validReaderConfig())
	if got := cfg.Zscaler.Client.ClientID; got != "zscalerctl-client-id" {
		t.Errorf("newSDKConfiguration().ClientID = %q, want zscalerctl-client-id", got)
	}
	if got := cfg.Zscaler.Client.ClientSecret; got != "zscalerctl-client-secret" {
		t.Errorf("newSDKConfiguration().ClientSecret = %q, want zscalerctl-client-secret", got)
	}
	if got := cfg.Zscaler.Client.VanityDomain; got != "zscalerctl-vanity" {
		t.Errorf("newSDKConfiguration().VanityDomain = %q, want zscalerctl-vanity", got)
	}
	if got := cfg.Zscaler.Client.Cloud; got != "" {
		t.Errorf("newSDKConfiguration().Cloud = %q, want empty", got)
	}
	if got := cfg.Zscaler.Client.CustomerID; got != "zscalerctl-zpa-customer-id" {
		t.Errorf("newSDKConfiguration().CustomerID = %q, want zscalerctl-zpa-customer-id", got)
	}
	if got := cfg.Zscaler.Client.MicrotenantID; got != "zscalerctl-zpa-microtenant-id" {
		t.Errorf("newSDKConfiguration().MicrotenantID = %q, want zscalerctl-zpa-microtenant-id", got)
	}
	if got := cfg.Zscaler.Client.Proxy.Host; got != "" {
		t.Errorf("newSDKConfiguration().Proxy.Host = %q, want empty", got)
	}
	if cfg.Zscaler.Client.Cache.Enabled {
		t.Errorf("newSDKConfiguration().Cache.Enabled = true, want false")
	}
	if cfg.Debug {
		t.Errorf("newSDKConfiguration().Debug = true, want false")
	}
	transport, ok := cfg.HTTPClient.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("newSDKConfiguration().HTTPClient.Transport = %T, want *http.Transport", cfg.HTTPClient.Transport)
	}
	if transport.Proxy != nil {
		t.Errorf("newSDKConfiguration().HTTPClient.Transport.Proxy is non-nil, want nil")
	}
}

func TestNewSDKConfigurationCanUseExplicitEnvironmentProxy(t *testing.T) {
	t.Setenv("HTTPS_PROXY", "http://proxy.example.invalid:8080")
	t.Setenv("HTTP_PROXY", "")
	t.Setenv("NO_PROXY", "")

	cfg := validReaderConfig()
	cfg.Proxy.FromEnvironment = true
	sdkCfg := newSDKConfiguration(context.Background(), cfg)
	transport, ok := sdkCfg.HTTPClient.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("newSDKConfiguration().HTTPClient.Transport = %T, want *http.Transport", sdkCfg.HTTPClient.Transport)
	}
	if transport.Proxy == nil {
		t.Fatal("newSDKConfiguration().HTTPClient.Transport.Proxy = nil, want environment proxy")
	}
	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://api.example.invalid", nil)
	if err != nil {
		t.Fatalf("http.NewRequest() error = %v, want nil", err)
	}
	proxyURL, err := transport.Proxy(request)
	if err != nil {
		t.Fatalf("transport.Proxy() error = %v, want nil", err)
	}
	if got := proxyURL.String(); got != "http://proxy.example.invalid:8080" {
		t.Errorf("transport.Proxy() = %q, want http://proxy.example.invalid:8080", got)
	}
}

func TestNewSDKConfigurationCanUseExplicitProxyURL(t *testing.T) {
	cfg := validReaderConfig()
	cfg.Proxy.URL = "http://proxy.example.invalid:8080"
	sdkCfg := newSDKConfiguration(context.Background(), cfg)
	transport, ok := sdkCfg.HTTPClient.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("newSDKConfiguration().HTTPClient.Transport = %T, want *http.Transport", sdkCfg.HTTPClient.Transport)
	}
	if transport.Proxy == nil {
		t.Fatal("newSDKConfiguration().HTTPClient.Transport.Proxy = nil, want explicit proxy")
	}
	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://api.example.invalid", nil)
	if err != nil {
		t.Fatalf("http.NewRequest() error = %v, want nil", err)
	}
	proxyURL, err := transport.Proxy(request)
	if err != nil {
		t.Fatalf("transport.Proxy() error = %v, want nil", err)
	}
	if got := proxyURL.String(); got != "http://proxy.example.invalid:8080" {
		t.Errorf("transport.Proxy() = %q, want http://proxy.example.invalid:8080", got)
	}
}

func TestNewReaderRejectsInvalidProxyConfig(t *testing.T) {
	t.Parallel()

	cfg := validReaderConfig()
	cfg.Proxy.URL = "proxy.example.invalid:8080"
	if _, err := NewReader(cfg); !errors.Is(err, ErrInvalidProxyConfig) {
		t.Fatalf("NewReader(invalid proxy URL) error = %v, want ErrInvalidProxyConfig", err)
	}

	cfg = validReaderConfig()
	cfg.Proxy.URL = "http://proxy.example.invalid:8080"
	cfg.Proxy.FromEnvironment = true
	if _, err := NewReader(cfg); !errors.Is(err, ErrInvalidProxyConfig) {
		t.Fatalf("NewReader(conflicting proxy config) error = %v, want ErrInvalidProxyConfig", err)
	}
}

func TestZPAReaderRequiresCustomerIDBeforeNetwork(t *testing.T) {
	t.Parallel()

	cfg := validReaderConfig()
	cfg.ZPACustomerID = ""
	reader, err := NewReader(cfg)
	if err != nil {
		t.Fatalf("NewReader(valid OneAPI without ZPA customer id) error = %v, want nil", err)
	}
	_, err = reader.List(context.Background(), resources.ProductZPA, resourceZPAServerGroups)
	if !errors.Is(err, ErrMissingCredentials) {
		t.Fatalf("SDKReader.List(zpa, server-groups without customer id) error = %v, want ErrMissingCredentials", err)
	}
	if !strings.Contains(err.Error(), "ZSCALERCTL_ZPA_CUSTOMER_ID") {
		t.Errorf("SDKReader.List(zpa, server-groups without customer id) error = %v, want ZSCALERCTL_ZPA_CUSTOMER_ID guidance", err)
	}
}

func TestNewLegacyZIAConfigurationDoesNotUseSDKDiscoveryOrProxy(t *testing.T) {
	t.Setenv("ZIA_USERNAME", "sdk-legacy-admin@example.invalid")
	t.Setenv("ZIA_PASSWORD", "sdk-legacy-password")
	t.Setenv("ZIA_API_KEY", "sdk-legacy-api-key")
	t.Setenv("ZIA_CLOUD", "sdk-legacy-cloud")
	t.Setenv("ZIA_CLIENT_PROXY_HOST", "sdk-proxy.example.invalid")
	t.Setenv("ZSCALER_SDK_LOG", "true")
	t.Setenv("ZSCALER_SDK_VERBOSE", "true")
	t.Setenv("HTTPS_PROXY", "http://standard-proxy.example.invalid:8080")

	cfg, err := newLegacyZIAConfiguration(context.Background(), validLegacyReaderConfig())
	if err != nil {
		t.Fatalf("newLegacyZIAConfiguration() error = %v, want nil", err)
	}
	if got := cfg.ZIA.Client.ZIAUsername; got != "zscalerctl-zia-admin@example.invalid" {
		t.Errorf("newLegacyZIAConfiguration().ZIAUsername = %q, want zscalerctl-zia-admin@example.invalid", got)
	}
	if got := cfg.ZIA.Client.ZIAPassword; got != "zscalerctl-zia-password" {
		t.Errorf("newLegacyZIAConfiguration().ZIAPassword = %q, want zscalerctl-zia-password", got)
	}
	if got := cfg.ZIA.Client.ZIAApiKey; got != "zscalerctl-zia-api-key" {
		t.Errorf("newLegacyZIAConfiguration().ZIAApiKey = %q, want zscalerctl-zia-api-key", got)
	}
	if got := cfg.ZIA.Client.ZIACloud; got != "zscalerthree" {
		t.Errorf("newLegacyZIAConfiguration().ZIACloud = %q, want zscalerthree", got)
	}
	if cfg.ZIA.Client.Proxy.Host != "" {
		t.Errorf("newLegacyZIAConfiguration().Proxy.Host = %q, want empty", cfg.ZIA.Client.Proxy.Host)
	}
	if cfg.ZIA.Client.Cache.Enabled {
		t.Errorf("newLegacyZIAConfiguration().Cache.Enabled = true, want false")
	}
	if cfg.BaseURL.String() != "https://zsapi.zscalerthree.net" {
		t.Errorf("newLegacyZIAConfiguration().BaseURL = %q, want https://zsapi.zscalerthree.net", cfg.BaseURL)
	}
	transport, ok := cfg.HTTPClient.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("newLegacyZIAConfiguration().HTTPClient.Transport = %T, want *http.Transport", cfg.HTTPClient.Transport)
	}
	if transport.Proxy != nil {
		t.Errorf("newLegacyZIAConfiguration().HTTPClient.Transport.Proxy is non-nil, want nil")
	}
}

func TestNewLegacyZIAConfigurationCanUseExplicitEnvironmentProxy(t *testing.T) {
	t.Setenv("HTTPS_PROXY", "http://proxy.example.invalid:8080")
	t.Setenv("HTTP_PROXY", "")
	t.Setenv("NO_PROXY", "")

	cfg := validLegacyReaderConfig()
	cfg.Proxy.FromEnvironment = true
	ziaCfg, err := newLegacyZIAConfiguration(context.Background(), cfg)
	if err != nil {
		t.Fatalf("newLegacyZIAConfiguration(proxy from env) error = %v, want nil", err)
	}
	transport, ok := ziaCfg.HTTPClient.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("newLegacyZIAConfiguration().HTTPClient.Transport = %T, want *http.Transport", ziaCfg.HTTPClient.Transport)
	}
	if transport.Proxy == nil {
		t.Fatal("newLegacyZIAConfiguration().HTTPClient.Transport.Proxy = nil, want environment proxy")
	}
}

func TestNewLegacyZIAClientSuppressesSDKLogEnvironment(t *testing.T) {
	t.Setenv("ZSCALER_SDK_LOG", "true")
	t.Setenv("ZSCALER_SDK_VERBOSE", "true")

	cfg, err := newLegacyZIAConfiguration(context.Background(), validLegacyReaderConfig())
	if err != nil {
		t.Fatalf("newLegacyZIAConfiguration() error = %v, want nil", err)
	}
	client, err := newLegacyZIAClient(cfg)
	if err != nil {
		t.Fatalf("newLegacyZIAClient() error = %v, want nil", err)
	}
	client.Close()
	if got := os.Getenv("ZSCALER_SDK_LOG"); got != "true" {
		t.Errorf("newLegacyZIAClient() restored ZSCALER_SDK_LOG = %q, want true", got)
	}
	if got := os.Getenv("ZSCALER_SDK_VERBOSE"); got != "true" {
		t.Errorf("newLegacyZIAClient() restored ZSCALER_SDK_VERBOSE = %q, want true", got)
	}
}

func TestReaderListLocationsProjectsSDKShapeThroughAllowList(t *testing.T) {
	t.Parallel()

	const (
		psk               = "plain-raw-sdk-psk-canary"
		bareFreeTextToken = "A7b9C2d4E6f8G1h3J5k7L9m2N4p6Q8r0S2t4U6v"
	)
	reader := &SDKReader{
		cfg: validReaderConfig(),
		handlers: map[resourceKey]resourceHandler{
			{product: resources.ProductZIA, name: resourceLocations}: fakeLocationsResourceHandler(fakeZIALocationClient{
				locations: []locationmanagement.Locations{
					{
						ID:          123,
						Name:        "HQ",
						IPAddresses: []string{"192.0.2.10"},
						Description: "temporary psk=" + psk + " " + bareFreeTextToken,
						VPNCredentials: []locationmanagement.VPNCredentials{
							{
								ID:           456,
								Type:         "UFQDN",
								FQDN:         "hq@example.invalid",
								PreSharedKey: psk,
							},
						},
					},
				},
			}),
		},
	}

	records, err := reader.List(context.Background(), resources.ProductZIA, "locations")
	if err != nil {
		t.Fatalf("SDKReader.List(zia, locations) error = %v, want nil", err)
	}
	spec, ok := resources.FindSpec(resources.ProductZIA, "locations")
	if !ok {
		t.Fatal("FindSpec(zia, locations) ok = false, want true")
	}
	projected, _, err := resources.ProjectRecords(spec, redact.ModeStandard, records)
	if err != nil {
		t.Fatalf("ProjectRecords(zia locations) error = %v, want nil", err)
	}
	got := projected.Records()[0].Fields()
	if strings.Contains(toString(got["description"]), psk) {
		t.Errorf("projected description = %v, want no %q", got["description"], psk)
	}
	if strings.Contains(toString(got["description"]), bareFreeTextToken) {
		t.Errorf("projected description = %v, want no bare token", got["description"])
	}
	if _, ok := got["vpnCredentials"]; ok {
		t.Errorf("projected record = %#v, want no vpnCredentials", got)
	}
	if err := resources.AssertRenderedSubset(spec, redact.ModeStandard, got); err != nil {
		t.Errorf("AssertRenderedSubset(projected SDK shape) error = %v, want nil", err)
	}
}

func TestReaderListLocationGroupsProjectsSDKShapeThroughAllowList(t *testing.T) {
	t.Parallel()

	const (
		canary            = "location-group-psk-canary"
		adminCanary       = "location-group-admin-canary"
		locationCanary    = "location-group-member-canary"
		criteriaCanary    = "location-group-criteria-canary"
		bareFreeTextToken = "A7b9C2d4E6f8G1h3J5k7L9m2N4p6Q8r0S2t4U6v"
	)
	reader := &SDKReader{
		cfg: validReaderConfig(),
		handlers: map[resourceKey]resourceHandler{
			{product: resources.ProductZIA, name: resourceLocationGroups}: fakeLocationGroupsResourceHandler(fakeZIALocationGroupsClient{
				groups: []locationgroups.LocationGroup{
					{
						ID:        987,
						Name:      "Branch group psk=" + canary,
						Deleted:   false,
						GroupType: "DYNAMIC_GROUP",
						DynamicLocationGroupCriteria: &locationgroups.DynamicLocationGroupCriteria{
							Name: &locationgroups.Name{
								MatchString: criteriaCanary,
								MatchType:   "contains",
							},
							Countries: []string{"US"},
							City: &locationgroups.City{
								MatchString: criteriaCanary,
								MatchType:   "contains",
							},
							ManagedBy: []locationgroups.ManagedBy{
								{
									ID:   1003,
									Name: adminCanary,
								},
							},
							EnforceAuthentication: true,
							Profiles:              []string{"Corp"},
						},
						Comments: "temporary psk=" + canary + " " + bareFreeTextToken,
						Locations: []ziacommon.IDNameExtensions{
							{
								ID:   123,
								Name: locationCanary,
							},
						},
						LastModUser: &locationgroups.LastModUser{
							ID:   1001,
							Name: adminCanary,
						},
						LastModTime: 1712345678,
						Predefined:  false,
					},
				},
			}),
		},
	}

	records, err := reader.List(context.Background(), resources.ProductZIA, "location-groups")
	if err != nil {
		t.Fatalf("SDKReader.List(zia, location-groups) error = %v, want nil", err)
	}
	spec, ok := resources.FindSpec(resources.ProductZIA, "location-groups")
	if !ok {
		t.Fatal("FindSpec(zia, location-groups) ok = false, want true")
	}
	projected, _, err := resources.ProjectRecords(spec, redact.ModeStandard, records)
	if err != nil {
		t.Fatalf("ProjectRecords(zia location-groups) error = %v, want nil", err)
	}
	got := projected.Records()[0].Fields()
	for _, field := range []string{"name", "comments"} {
		value := toString(got[field])
		if strings.Contains(value, canary) {
			t.Errorf("projected location-groups %s = %v, want no %q", field, got[field], canary)
		}
		if field == "comments" && strings.Contains(value, bareFreeTextToken) {
			t.Errorf("projected location-groups %s = %v, want no bare token", field, got[field])
		}
		if !strings.Contains(value, "<REDACTED:SECRET>") {
			t.Errorf("projected location-groups %s = %v, want typed redaction marker", field, got[field])
		}
	}
	for _, field := range []string{"dynamicLocationGroupCriteria", "locations", "lastModUser"} {
		if _, ok := got[field]; ok {
			t.Errorf("projected location-groups = %#v, want no %s", got, field)
		}
	}
	for _, forbidden := range []string{adminCanary, locationCanary, criteriaCanary} {
		if strings.Contains(fmt.Sprint(got), forbidden) {
			t.Errorf("projected location-groups = %#v, want no %q", got, forbidden)
		}
	}
	if err := resources.AssertRenderedSubset(spec, redact.ModeStandard, got); err != nil {
		t.Errorf("AssertRenderedSubset(projected location-groups SDK shape) error = %v, want nil", err)
	}
}

func TestReaderListRuleLabelsProjectsSDKShapeThroughAllowList(t *testing.T) {
	t.Parallel()

	const (
		canary            = "rule-label-psk-canary"
		adminCanary       = "rule-label-admin-canary"
		bareFreeTextToken = "A7b9C2d4E6f8G1h3J5k7L9m2N4p6Q8r0S2t4U6v"
	)
	reader := &SDKReader{
		cfg: validReaderConfig(),
		handlers: map[resourceKey]resourceHandler{
			{product: resources.ProductZIA, name: resourceRuleLabels}: fakeRuleLabelsResourceHandler(fakeZIARuleLabelsClient{
				labels: []rulelabels.RuleLabels{
					{
						ID:                  789,
						Name:                "Outbound psk=" + canary,
						Description:         "temporary psk=" + canary + " " + bareFreeTextToken,
						LastModifiedTime:    1712345678,
						ReferencedRuleCount: 3,
						CreatedBy: &ziacommon.IDNameExtensions{
							ID:   1001,
							Name: adminCanary,
						},
						LastModifiedBy: &ziacommon.IDNameExtensions{
							ID:   1002,
							Name: adminCanary,
						},
					},
				},
			}),
		},
	}

	records, err := reader.List(context.Background(), resources.ProductZIA, "rule-labels")
	if err != nil {
		t.Fatalf("SDKReader.List(zia, rule-labels) error = %v, want nil", err)
	}
	spec, ok := resources.FindSpec(resources.ProductZIA, "rule-labels")
	if !ok {
		t.Fatal("FindSpec(zia, rule-labels) ok = false, want true")
	}
	projected, _, err := resources.ProjectRecords(spec, redact.ModeStandard, records)
	if err != nil {
		t.Fatalf("ProjectRecords(zia rule-labels) error = %v, want nil", err)
	}
	got := projected.Records()[0].Fields()
	for _, field := range []string{"name", "description"} {
		value := toString(got[field])
		if strings.Contains(value, canary) {
			t.Errorf("projected rule-labels %s = %v, want no %q", field, got[field], canary)
		}
		if field == "description" && strings.Contains(value, bareFreeTextToken) {
			t.Errorf("projected rule-labels %s = %v, want no bare token", field, got[field])
		}
		if !strings.Contains(value, "<REDACTED:SECRET>") {
			t.Errorf("projected rule-labels %s = %v, want typed redaction marker", field, got[field])
		}
	}
	for _, field := range []string{"createdBy", "lastModifiedBy"} {
		if _, ok := got[field]; ok {
			t.Errorf("projected rule-labels = %#v, want no %s", got, field)
		}
	}
	if strings.Contains(fmt.Sprint(got), adminCanary) {
		t.Errorf("projected rule-labels = %#v, want no %q", got, adminCanary)
	}
	if err := resources.AssertRenderedSubset(spec, redact.ModeStandard, got); err != nil {
		t.Errorf("AssertRenderedSubset(projected rule-labels SDK shape) error = %v, want nil", err)
	}
}

func TestSourceRecordReferenceHelpersStripExtensions(t *testing.T) {
	t.Parallel()

	const canary = "client_secret=extension-canary"
	extensions := map[string]any{"raw": canary}
	managedBy := locationGroupManagedBySliceSource([]locationgroups.ManagedBy{{
		ID:         1,
		Name:       "admin",
		Extensions: extensions,
	}})
	locationGroupManagedBy, ok := managedBy[0].(map[string]any)
	if !ok {
		t.Fatalf("locationGroupManagedBySliceSource()[0] = %T, want map[string]any", managedBy[0])
	}
	tests := []struct {
		name string
		got  map[string]any
	}{
		{
			name: "IDNameExtensions",
			got: idNameExtensionsSource(&ziacommon.IDNameExtensions{
				ID:         1,
				Name:       "reference",
				Extensions: extensions,
			}),
		},
		{
			name: "IDNameExternalID",
			got: idNameExternalIDSource(&ziacommon.IDNameExternalID{
				ID:         1,
				Name:       "reference",
				ExternalID: "external",
				Extensions: extensions,
			}),
		},
		{
			name: "LocationGroupManagedBy",
			got:  locationGroupManagedBy,
		},
		{
			name: "LocationGroupLastModUser",
			got: locationGroupLastModUserSource(&locationgroups.LastModUser{
				ID:         1,
				Name:       "admin",
				Extensions: extensions,
			}),
		},
		{
			name: "StaticIPManagedBy",
			got: staticIPManagedBySource(&staticips.ManagedBy{
				ID:         1,
				Name:       "admin",
				Extensions: extensions,
			}),
		},
		{
			name: "StaticIPLastModifiedBy",
			got: staticIPLastModifiedBySource(&staticips.LastModifiedBy{
				ID:         1,
				Name:       "admin",
				Extensions: extensions,
			}),
		},
		{
			name: "GREManagedBy",
			got: greManagedBySource(&gretunnels.ManagedBy{
				ID:         1,
				Name:       "admin",
				Extensions: extensions,
			}),
		},
		{
			name: "GRELastModifiedBy",
			got: greLastModifiedBySource(&gretunnels.LastModifiedBy{
				ID:         1,
				Name:       "admin",
				Extensions: extensions,
			}),
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if _, ok := tt.got["extensions"]; ok {
				t.Errorf("%s source = %#v, want no extensions key", tt.name, tt.got)
			}
			if strings.Contains(fmt.Sprint(tt.got), canary) {
				t.Errorf("%s source = %#v, want no extension canary", tt.name, tt.got)
			}
		})
	}
}

func TestReaderListAuthSettingsProjectsSingletonThroughAllowList(t *testing.T) {
	t.Parallel()

	const (
		kerberosCanary = "kerberos-password-canary"
		strengthCanary = "password-strength-canary"
		expiryCanary   = "password-expiry-canary"
	)
	reader := &SDKReader{
		cfg: validReaderConfig(),
		handlers: map[resourceKey]resourceHandler{
			{product: resources.ProductZIA, name: resourceAuthSettings}: newSingletonHandler(
				resourceAuthSettings,
				func(context.Context) (*authsettings.AuthenticationSettings, error) {
					return &authsettings.AuthenticationSettings{
						OrgAuthType:                       "SAML",
						OneTimeAuth:                       "DISABLED",
						SamlEnabled:                       true,
						KerberosEnabled:                   true,
						KerberosPwd:                       kerberosCanary,
						AuthFrequency:                     "CUSTOM",
						AuthCustomFrequency:               30,
						PasswordStrength:                  strengthCanary,
						PasswordExpiry:                    expiryCanary,
						LastSyncStartTime:                 1712345000,
						LastSyncEndTime:                   1712345600,
						MobileAdminSamlIdpEnabled:         false,
						AutoProvision:                     true,
						DirectorySyncMigrateToScimEnabled: true,
					}, nil
				},
				authSettingsSourceRecord,
			),
		},
	}

	records, err := reader.List(context.Background(), resources.ProductZIA, resourceAuthSettings)
	if err != nil {
		t.Fatalf("SDKReader.List(zia, auth-settings) error = %v, want nil", err)
	}
	got := projectOneRecord(t, resources.ProductZIA, resourceAuthSettings, records)
	for _, field := range []string{"kerberosPwd", "passwordStrength", "passwordExpiry"} {
		if _, ok := got[field]; ok {
			t.Errorf("projected auth-settings = %#v, want no %s", got, field)
		}
	}
	assertNoCanaries(t, "auth-settings", got, kerberosCanary, strengthCanary, expiryCanary)
	if got["orgAuthType"] != "SAML" {
		t.Errorf("projected auth-settings orgAuthType = %v, want SAML", got["orgAuthType"])
	}
	if got["authFrequency"] != "CUSTOM" {
		t.Errorf("projected auth-settings authFrequency = %v, want CUSTOM", got["authFrequency"])
	}
	if got["authCustomFrequency"] != 30 {
		t.Errorf("projected auth-settings authCustomFrequency = %v, want 30", got["authCustomFrequency"])
	}
	if got["autoProvision"] != true {
		t.Errorf("projected auth-settings autoProvision = %v, want true", got["autoProvision"])
	}
}

func TestSourceRecordFromStructDropsUnsupportedKinds(t *testing.T) {
	t.Parallel()

	type unsupportedKinds struct {
		Name     string    `json:"name"`
		Callback func()    `json:"callback"`
		Complex  complex64 `json:"complex"`
	}

	record := sourceRecordFromStruct(unsupportedKinds{
		Name:     "settings",
		Callback: func() {},
		Complex:  1 + 2i,
	})
	projected, _, err := resources.ProjectRecord(resources.ResourceSpec{
		Product:    resources.ProductZIA,
		Name:       "unsupported-kinds",
		Operations: resources.ShowOperation(),
		Fields: []resources.FieldSpec{
			{Name: "name", Classification: resources.ClassTenantConfig, AllowedModes: []redact.Mode{redact.ModeStandard}},
			{Name: "callback", Classification: resources.ClassOperational, AllowedModes: []redact.Mode{redact.ModeStandard}},
			{Name: "complex", Classification: resources.ClassOperational, AllowedModes: []redact.Mode{redact.ModeStandard}},
		},
	}, redact.ModeStandard, record)
	if err != nil {
		t.Fatalf("ProjectRecord(unsupported kind record) error = %v, want nil", err)
	}
	if got, _ := projected.Value("name"); got != "settings" {
		t.Fatalf("projected name = %#v, want settings", got)
	}
	if got, _ := projected.Value("callback"); got != nil {
		t.Errorf("projected callback = %#v, want nil", got)
	}
	if got, _ := projected.Value("complex"); got != nil {
		t.Errorf("projected complex = %#v, want nil", got)
	}
}

func TestReaderListStaticIPsProjectsSDKShapeThroughAllowList(t *testing.T) {
	t.Parallel()

	const (
		canary            = "static-ip-psk-canary"
		adminCanary       = "static-ip-admin-canary"
		bareFreeTextToken = "A7b9C2d4E6f8G1h3J5k7L9m2N4p6Q8r0S2t4U6v"
	)
	reader := &SDKReader{
		cfg: validReaderConfig(),
		handlers: map[resourceKey]resourceHandler{
			{product: resources.ProductZIA, name: resourceStaticIPs}: fakeStaticIPsResourceHandler(fakeZIAStaticIPsClient{
				staticIPs: []staticips.StaticIP{
					{
						ID:                   321,
						IpAddress:            "192.0.2.44",
						GeoOverride:          true,
						Latitude:             40.7128,
						Longitude:            -74.0060,
						RoutableIP:           true,
						LastModificationTime: 1712345678,
						Comment:              "temporary psk=" + canary + " " + bareFreeTextToken,
						City: &staticips.City{
							ID:   44,
							Name: "Metropolis",
						},
						ManagedBy: &staticips.ManagedBy{
							ID:   1001,
							Name: adminCanary,
						},
						LastModifiedBy: &staticips.LastModifiedBy{
							ID:   1002,
							Name: adminCanary,
						},
					},
				},
			}),
		},
	}

	records, err := reader.List(context.Background(), resources.ProductZIA, "static-ips")
	if err != nil {
		t.Fatalf("SDKReader.List(zia, static-ips) error = %v, want nil", err)
	}
	spec, ok := resources.FindSpec(resources.ProductZIA, "static-ips")
	if !ok {
		t.Fatal("FindSpec(zia, static-ips) ok = false, want true")
	}
	projected, _, err := resources.ProjectRecords(spec, redact.ModeStandard, records)
	if err != nil {
		t.Fatalf("ProjectRecords(zia static-ips) error = %v, want nil", err)
	}
	got := projected.Records()[0].Fields()
	comment := toString(got["comment"])
	if strings.Contains(comment, canary) {
		t.Errorf("projected static-ips comment = %v, want no %q", got["comment"], canary)
	}
	if strings.Contains(comment, bareFreeTextToken) {
		t.Errorf("projected static-ips comment = %v, want no bare token", got["comment"])
	}
	if !strings.Contains(comment, "<REDACTED:SECRET>") {
		t.Errorf("projected static-ips comment = %v, want typed redaction marker", got["comment"])
	}
	for _, field := range []string{"city", "managedBy", "lastModifiedBy"} {
		if _, ok := got[field]; ok {
			t.Errorf("projected static-ips = %#v, want no %s", got, field)
		}
	}
	if strings.Contains(fmt.Sprint(got), adminCanary) {
		t.Errorf("projected static-ips = %#v, want no %q", got, adminCanary)
	}
	if err := resources.AssertRenderedSubset(spec, redact.ModeStandard, got); err != nil {
		t.Errorf("AssertRenderedSubset(projected static-ips SDK shape) error = %v, want nil", err)
	}
}

func TestReaderListGRETunnelsProjectsSDKShapeThroughAllowList(t *testing.T) {
	t.Parallel()

	const (
		canary            = "gre-tunnel-psk-canary"
		adminCanary       = "gre-tunnel-admin-canary"
		bareFreeTextToken = "A7b9C2d4E6f8G1h3J5k7L9m2N4p6Q8r0S2t4U6v"
		virtualIPCanary   = "198.51.100.77"
	)
	withinCountry := true
	reader := &SDKReader{
		cfg: validReaderConfig(),
		handlers: map[resourceKey]resourceHandler{
			{product: resources.ProductZIA, name: resourceGRETunnels}: fakeGRETunnelsResourceHandler(fakeZIAGRETunnelsClient{
				tunnels: []gretunnels.GreTunnels{
					{
						ID:                   654,
						SourceIP:             "192.0.2.10",
						InternalIpRange:      "10.10.10.0/29",
						LastModificationTime: 1712345678,
						WithinCountry:        &withinCountry,
						Comment:              "temporary psk=" + canary + " " + bareFreeTextToken,
						IPUnnumbered:         true,
						SubCloud:             "us-east",
						ManagedBy: &gretunnels.ManagedBy{
							ID:   1001,
							Name: adminCanary,
						},
						LastModifiedBy: &gretunnels.LastModifiedBy{
							ID:   1002,
							Name: adminCanary,
						},
						PrimaryDestVip: &gretunnels.PrimaryDestVip{
							ID:        901,
							VirtualIP: virtualIPCanary,
						},
					},
				},
			}),
		},
	}

	records, err := reader.List(context.Background(), resources.ProductZIA, "gre-tunnels")
	if err != nil {
		t.Fatalf("SDKReader.List(zia, gre-tunnels) error = %v, want nil", err)
	}
	spec, ok := resources.FindSpec(resources.ProductZIA, "gre-tunnels")
	if !ok {
		t.Fatal("FindSpec(zia, gre-tunnels) ok = false, want true")
	}
	projected, _, err := resources.ProjectRecords(spec, redact.ModeStandard, records)
	if err != nil {
		t.Fatalf("ProjectRecords(zia gre-tunnels) error = %v, want nil", err)
	}
	got := projected.Records()[0].Fields()
	comment := toString(got["comment"])
	if strings.Contains(comment, canary) {
		t.Errorf("projected gre-tunnels comment = %v, want no %q", got["comment"], canary)
	}
	if strings.Contains(comment, bareFreeTextToken) {
		t.Errorf("projected gre-tunnels comment = %v, want no bare token", got["comment"])
	}
	if !strings.Contains(comment, "<REDACTED:SECRET>") {
		t.Errorf("projected gre-tunnels comment = %v, want typed redaction marker", got["comment"])
	}
	for _, field := range []string{"managedBy", "lastModifiedBy", "primaryDestVip", "secondaryDestVip"} {
		if _, ok := got[field]; ok {
			t.Errorf("projected gre-tunnels = %#v, want no %s", got, field)
		}
	}
	for _, forbidden := range []string{adminCanary, virtualIPCanary} {
		if strings.Contains(fmt.Sprint(got), forbidden) {
			t.Errorf("projected gre-tunnels = %#v, want no %q", got, forbidden)
		}
	}
	if err := resources.AssertRenderedSubset(spec, redact.ModeStandard, got); err != nil {
		t.Errorf("AssertRenderedSubset(projected gre-tunnels SDK shape) error = %v, want nil", err)
	}
}

func TestReaderListSublocationsProjectsSDKShapeThroughAllowList(t *testing.T) {
	t.Parallel()

	const (
		canary            = "sublocation-psk-canary"
		bareFreeTextToken = "A7b9C2d4E6f8G1h3J5k7L9m2N4p6Q8r0S2t4U6v"
		scopeCanary       = "subLocAcc-canary"
	)
	reader := &SDKReader{
		cfg: validReaderConfig(),
		handlers: map[resourceKey]resourceHandler{
			{product: resources.ProductZIA, name: resourceSublocations}: newListGetHandler(
				resourceSublocations,
				func(context.Context) ([]locationmanagement.Locations, error) {
					return []locationmanagement.Locations{
						{
							ID:                 222,
							ParentID:           111,
							Name:               "Floor 1 psk=" + canary,
							Description:        "temporary psk=" + canary + " " + bareFreeTextToken,
							IPAddresses:        []string{"10.10.10.0/24"},
							Ports:              []int{80, 443},
							Profile:            "Workload",
							Country:            "US",
							State:              "NY",
							TZ:                 "America/New_York",
							AuthRequired:       true,
							SSLScanEnabled:     true,
							OFWEnabled:         true,
							IPSControl:         true,
							SubLocScopeValues:  []string{scopeCanary},
							SubLocAccIDs:       []string{scopeCanary},
							SubLocScopeEnabled: true,
							VPNCredentials: []locationmanagement.VPNCredentials{
								{
									ID:           456,
									Type:         "UFQDN",
									PreSharedKey: canary,
								},
							},
						},
					}, nil
				},
				intIDGetter(func(context.Context, int) (*locationmanagement.Locations, error) { return nil, nil }),
				sublocationSourceRecord,
			),
		},
	}

	records, err := reader.List(context.Background(), resources.ProductZIA, "sublocations")
	if err != nil {
		t.Fatalf("SDKReader.List(zia, sublocations) error = %v, want nil", err)
	}
	spec, ok := resources.FindSpec(resources.ProductZIA, "sublocations")
	if !ok {
		t.Fatal("FindSpec(zia, sublocations) ok = false, want true")
	}
	projected, _, err := resources.ProjectRecords(spec, redact.ModeStandard, records)
	if err != nil {
		t.Fatalf("ProjectRecords(zia sublocations) error = %v, want nil", err)
	}
	got := projected.Records()[0].Fields()
	for _, field := range []string{"name", "description"} {
		value := toString(got[field])
		if strings.Contains(value, canary) {
			t.Errorf("projected sublocations %s = %v, want no %q", field, got[field], canary)
		}
		if field == "description" && strings.Contains(value, bareFreeTextToken) {
			t.Errorf("projected sublocations %s = %v, want no bare token", field, got[field])
		}
	}
	for _, field := range []string{"vpnCredentials", "subLocScopeValues", "subLocAccIds", "subLocScopeEnabled"} {
		if _, ok := got[field]; ok {
			t.Errorf("projected sublocations = %#v, want no %s", got, field)
		}
	}
	for _, forbidden := range []string{canary, scopeCanary} {
		if strings.Contains(fmt.Sprint(got), forbidden) {
			t.Errorf("projected sublocations = %#v, want no %q", got, forbidden)
		}
	}
	if got["parentId"] != 111 {
		t.Errorf("projected sublocations parentId = %v, want 111", got["parentId"])
	}
	if err := resources.AssertRenderedSubset(spec, redact.ModeStandard, got); err != nil {
		t.Errorf("AssertRenderedSubset(projected sublocations SDK shape) error = %v, want nil", err)
	}
}

func TestReaderListSSLInspectionRulesProjectsSDKShapeThroughAllowList(t *testing.T) {
	t.Parallel()

	const (
		canary            = "ssl-rule-psk-canary"
		bareFreeTextToken = "A7b9C2d4E6f8G1h3J5k7L9m2N4p6Q8r0S2t4U6v"
		adminCanary       = "ssl-rule-admin-canary"
		certCanary        = "ssl-rule-cert-canary"
	)
	reader := &SDKReader{
		cfg: validReaderConfig(),
		handlers: map[resourceKey]resourceHandler{
			{product: resources.ProductZIA, name: resourceSSLRules}: newListGetHandler(
				resourceSSLRules,
				func(context.Context) ([]sslinspection.SSLInspectionRules, error) {
					return []sslinspection.SSLInspectionRules{
						{
							ID:          333,
							Name:        "Decrypt psk=" + canary,
							Description: "temporary psk=" + canary + " " + bareFreeTextToken,
							Action: sslinspection.Action{
								Type: "DECRYPT",
								SSLInterceptionCert: &sslinspection.SSLInterceptionCert{
									ID:   44,
									Name: certCanary,
								},
							},
							State:             "ENABLED",
							Order:             10,
							Rank:              7,
							URLCategories:     []string{"ANY"},
							Platforms:         []string{"WINDOWS"},
							CloudApplications: []string{"OFFICE365"},
							LastModifiedBy: &ziacommon.IDNameExtensions{
								ID:   1001,
								Name: adminCanary,
							},
							Users: []ziacommon.IDNameExtensions{
								{
									ID:   1002,
									Name: adminCanary,
								},
							},
							LastModifiedTime: 1712345678,
							DefaultRule:      false,
							Predefined:       false,
						},
					}, nil
				},
				intIDGetter(func(context.Context, int) (*sslinspection.SSLInspectionRules, error) { return nil, nil }),
				sslInspectionRuleSourceRecord,
			),
		},
	}

	records, err := reader.List(context.Background(), resources.ProductZIA, "ssl-inspection-rules")
	if err != nil {
		t.Fatalf("SDKReader.List(zia, ssl-inspection-rules) error = %v, want nil", err)
	}
	spec, ok := resources.FindSpec(resources.ProductZIA, "ssl-inspection-rules")
	if !ok {
		t.Fatal("FindSpec(zia, ssl-inspection-rules) ok = false, want true")
	}
	projected, _, err := resources.ProjectRecords(spec, redact.ModeStandard, records)
	if err != nil {
		t.Fatalf("ProjectRecords(zia ssl-inspection-rules) error = %v, want nil", err)
	}
	got := projected.Records()[0].Fields()
	for _, field := range []string{"name", "description"} {
		value := toString(got[field])
		if strings.Contains(value, canary) {
			t.Errorf("projected ssl-inspection-rules %s = %v, want no %q", field, got[field], canary)
		}
		if field == "description" && strings.Contains(value, bareFreeTextToken) {
			t.Errorf("projected ssl-inspection-rules %s = %v, want no bare token", field, got[field])
		}
	}
	action, ok := got["action"].(map[string]any)
	if !ok {
		t.Fatalf("projected ssl-inspection-rules action = %T, want map[string]any", got["action"])
	}
	if action["type"] != "DECRYPT" {
		t.Errorf("projected ssl-inspection-rules action.type = %v, want DECRYPT", action["type"])
	}
	for _, field := range []string{"sslInterceptionCert", "users", "lastModifiedBy"} {
		if _, ok := got[field]; ok {
			t.Errorf("projected ssl-inspection-rules = %#v, want no %s", got, field)
		}
	}
	if _, ok := action["sslInterceptionCert"]; ok {
		t.Errorf("projected ssl-inspection-rules action = %#v, want no sslInterceptionCert", action)
	}
	for _, forbidden := range []string{canary, adminCanary, certCanary} {
		if strings.Contains(fmt.Sprint(got), forbidden) {
			t.Errorf("projected ssl-inspection-rules = %#v, want no %q", got, forbidden)
		}
	}
	if err := resources.AssertRenderedSubset(spec, redact.ModeStandard, got); err != nil {
		t.Errorf("AssertRenderedSubset(projected ssl-inspection-rules SDK shape) error = %v, want nil", err)
	}
}

func TestReaderListURLCategoriesProjectsSDKShapeThroughAllowList(t *testing.T) {
	t.Parallel()

	const (
		canary            = "url-category-psk-canary"
		bareFreeTextToken = "A7b9C2d4E6f8G1h3J5k7L9m2N4p6Q8r0S2t4U6v"
		urlCanary         = "https://operator:url-category-secret@example.invalid"
		scopeCanary       = "url-category-scope-canary"
	)
	reader := &SDKReader{
		cfg: validReaderConfig(),
		handlers: map[resourceKey]resourceHandler{
			{product: resources.ProductZIA, name: resourceURLCategories}: newListGetHandler(
				resourceURLCategories,
				func(context.Context) ([]urlcategories.URLCategory, error) {
					return []urlcategories.URLCategory{
						{
							ID:                                   "CUSTOM_01",
							ConfiguredName:                       "Category psk=" + canary,
							Description:                          "temporary psk=" + canary + " " + bareFreeTextToken,
							Type:                                 "URL_CATEGORY",
							CustomCategory:                       true,
							Editable:                             true,
							CustomUrlsCount:                      1,
							CustomIpRangesCount:                  2,
							UrlsRetainingParentCategoryCount:     3,
							IPRangesRetainingParentCategoryCount: 4,
							CategoryGroup:                        "User Defined",
							SuperCategory:                        "CUSTOM",
							UrlType:                              "EXACT",
							Urls:                                 []string{"example.invalid/path", urlCanary},
							DBCategorizedUrls:                    []string{"retained.example.invalid"},
							Keywords:                             []string{"finance", "psk=" + canary},
							KeywordsRetainingParentCategory:      []string{"retained-keyword"},
							IPRanges:                             []string{"203.0.113.0/24"},
							IPRangesRetainingParentCategory:      []string{"198.51.100.0/24"},
							RegexPatterns:                        []string{"^https://example\\.invalid/.*", "token=" + canary},
							RegexPatternsRetainingParentCategory: []string{"^https://retained\\.example\\.invalid/.*"},
							Scopes: []urlcategories.Scopes{
								{
									Type: "LOCATION",
									ScopeEntities: []ziacommon.IDNameExtensions{
										{
											ID:   1001,
											Name: scopeCanary,
										},
									},
								},
							},
							URLKeywordCounts: &urlcategories.URLKeywordCounts{
								TotalURLCount:            10,
								RetainParentURLCount:     1,
								TotalKeywordCount:        5,
								RetainParentKeywordCount: 2,
							},
						},
					}, nil
				},
				func(context.Context, string) (*urlcategories.URLCategory, error) { return nil, nil },
				urlCategorySourceRecord,
			),
		},
	}

	records, err := reader.List(context.Background(), resources.ProductZIA, "url-categories")
	if err != nil {
		t.Fatalf("SDKReader.List(zia, url-categories) error = %v, want nil", err)
	}
	spec, ok := resources.FindSpec(resources.ProductZIA, "url-categories")
	if !ok {
		t.Fatal("FindSpec(zia, url-categories) ok = false, want true")
	}
	projected, _, err := resources.ProjectRecords(spec, redact.ModeStandard, records)
	if err != nil {
		t.Fatalf("ProjectRecords(zia url-categories) error = %v, want nil", err)
	}
	got := projected.Records()[0].Fields()
	for _, field := range []string{"configuredName", "description"} {
		value := toString(got[field])
		if strings.Contains(value, canary) {
			t.Errorf("projected url-categories %s = %v, want no %q", field, got[field], canary)
		}
		if field == "description" && strings.Contains(value, bareFreeTextToken) {
			t.Errorf("projected url-categories %s = %v, want no bare token", field, got[field])
		}
	}
	for _, field := range []string{"urls", "keywords", "regexPatterns"} {
		values, ok := got[field].([]string)
		if !ok || len(values) == 0 {
			t.Fatalf("projected url-categories %s = %T %#v, want non-empty []string", field, got[field], got[field])
		}
	}
	for _, field := range []string{"dbCategorizedUrls", "keywordsRetainingParentCategory", "ipRanges", "ipRangesRetainingParentCategory", "regexPatternsRetainingParentCategory"} {
		values, ok := got[field].([]string)
		if !ok || len(values) == 0 {
			t.Fatalf("projected url-categories %s = %T %#v, want non-empty []string", field, got[field], got[field])
		}
	}
	if _, ok := got["scopes"]; ok {
		t.Errorf("projected url-categories = %#v, want no scopes", got)
	}
	for _, field := range []string{"urls", "keywords", "regexPatterns"} {
		if _, ok := got[field]; ok {
			if strings.Contains(fmt.Sprint(got[field]), canary) || strings.Contains(fmt.Sprint(got[field]), "url-category-secret") {
				t.Errorf("projected url-categories %s = %#v, want secret-shaped values redacted", field, got[field])
			}
		}
	}
	for _, forbidden := range []string{canary, urlCanary, scopeCanary} {
		if strings.Contains(fmt.Sprint(got), forbidden) {
			t.Errorf("projected url-categories = %#v, want no %q", got, forbidden)
		}
	}
	counts, ok := got["urlKeywordCounts"].(map[string]any)
	if !ok {
		t.Fatalf("projected url-categories urlKeywordCounts = %T, want map[string]any", got["urlKeywordCounts"])
	}
	if counts["totalUrlCount"] != 10 {
		t.Errorf("projected url-categories urlKeywordCounts.totalUrlCount = %v, want 10", counts["totalUrlCount"])
	}
	if err := resources.AssertRenderedSubset(spec, redact.ModeStandard, got); err != nil {
		t.Errorf("AssertRenderedSubset(projected url-categories SDK shape) error = %v, want nil", err)
	}
}

func TestReaderListURLFilteringRulesProjectsSDKShapeThroughAllowList(t *testing.T) {
	t.Parallel()

	const (
		canary            = "url-rule-psk-canary"
		bareFreeTextToken = "A7b9C2d4E6f8G1h3J5k7L9m2N4p6Q8r0S2t4U6v"
		adminCanary       = "url-rule-admin-canary"
		userCanary        = "url-rule-user-canary"
		profileCanary     = "https://operator:url-rule-secret@example.invalid"
	)
	reader := &SDKReader{
		cfg: validReaderConfig(),
		handlers: map[resourceKey]resourceHandler{
			{product: resources.ProductZIA, name: resourceURLRules}: newListGetHandler(
				resourceURLRules,
				func(context.Context) ([]urlfilteringpolicies.URLFilteringRule, error) {
					return []urlfilteringpolicies.URLFilteringRule{
						{
							ID:                     101,
							Name:                   "URL rule psk=" + canary,
							Description:            "temporary psk=" + canary + " " + bareFreeTextToken,
							Order:                  1,
							Rank:                   7,
							State:                  "ENABLED",
							Action:                 "ALLOW",
							Protocols:              []string{"HTTPS_RULE"},
							URLCategories:          []string{"CUSTOM_01"},
							URLCategories2:         []string{"CUSTOM_02"},
							UserRiskScoreLevels:    []string{"LOW"},
							UserAgentTypes:         []string{"OPERA"},
							RequestMethods:         []string{"GET"},
							SourceCountries:        []string{"US"},
							EndUserNotificationURL: "https://notify.example.invalid",
							BlockOverride:          true,
							TimeQuota:              30,
							SizeQuota:              1024,
							LastModifiedTime:       1700000000,
							EnforceTimeValidity:    true,
							ValidityStartTime:      1700000001,
							ValidityEndTime:        1700000100,
							ValidityTimeZoneID:     "America/New_York",
							Ciparule:               false,
							CBIProfileID:           55,
							CBIProfile: &ziacommon.CBIProfile{
								ID:   "cbi-1",
								Name: "CBI profile",
								URL:  profileCanary,
							},
							LastModifiedBy: &ziacommon.IDNameExtensions{
								ID:   9001,
								Name: adminCanary,
							},
							Users: []ziacommon.IDNameExtensions{
								{ID: 2001, Name: userCanary},
							},
							Locations: []ziacommon.IDNameExtensions{
								{ID: 3001, Name: "Branch"},
							},
							Labels: []ziacommon.IDNameExtensions{
								{ID: 4001, Name: "Security"},
							},
							SourceIPGroups: []ziacommon.IDNameExtensions{
								{ID: 5001, Name: "source-networks"},
							},
						},
					}, nil
				},
				func(context.Context, string) (*urlfilteringpolicies.URLFilteringRule, error) { return nil, nil },
				urlFilteringRuleSourceRecord,
			),
		},
	}

	records, err := reader.List(context.Background(), resources.ProductZIA, "url-filtering-rules")
	if err != nil {
		t.Fatalf("SDKReader.List(zia, url-filtering-rules) error = %v, want nil", err)
	}
	got := projectOneRecord(t, resources.ProductZIA, "url-filtering-rules", records)
	assertNoCanaries(t, "url-filtering-rules", got, canary, bareFreeTextToken, adminCanary, userCanary, profileCanary)
	for _, field := range []string{"cbiProfile", "lastModifiedBy", "users"} {
		if _, ok := got[field]; ok {
			t.Errorf("projected url-filtering-rules = %#v, want no %s", got, field)
		}
	}
	if got["action"] != "ALLOW" {
		t.Errorf("projected url-filtering-rules action = %v, want ALLOW", got["action"])
	}
}

func TestReaderListFirewallFilteringRulesProjectsSDKShapeThroughAllowList(t *testing.T) {
	t.Parallel()

	const (
		canary            = "firewall-rule-psk-canary"
		bareFreeTextToken = "A7b9C2d4E6f8G1h3J5k7L9m2N4p6Q8r0S2t4U6v"
		adminCanary       = "firewall-rule-admin-canary"
		userCanary        = "firewall-rule-user-canary"
		zpaCanary         = "firewall-rule-zpa-canary"
	)
	reader := &SDKReader{
		cfg: validReaderConfig(),
		handlers: map[resourceKey]resourceHandler{
			{product: resources.ProductZIA, name: resourceFirewallRules}: newListGetHandler(
				resourceFirewallRules,
				func(context.Context) ([]filteringrules.FirewallFilteringRules, error) {
					return []filteringrules.FirewallFilteringRules{
						{
							ID:                202,
							Name:              "Firewall rule psk=" + canary,
							Description:       "temporary psk=" + canary + " " + bareFreeTextToken,
							Order:             2,
							Rank:              8,
							State:             "ENABLED",
							Action:            "ALLOW",
							AccessControl:     "READ_WRITE",
							EnableFullLogging: true,
							DefaultRule:       false,
							Predefined:        false,
							LastModifiedTime:  1700000000,
							SrcIps:            []string{"192.0.2.10"},
							DestAddresses:     []string{"203.0.113.10"},
							DestIpCategories:  []string{"COUNTRY_US"},
							DestCountries:     []string{"US"},
							SourceCountries:   []string{"US"},
							NwApplications:    []string{"HTTP"},
							LastModifiedBy: &ziacommon.IDNameExtensions{
								ID:   9002,
								Name: adminCanary,
							},
							Users: []ziacommon.IDNameExtensions{
								{ID: 2002, Name: userCanary},
							},
							Locations: []ziacommon.IDNameExtensions{
								{ID: 3002, Name: "Branch"},
							},
							Labels: []ziacommon.IDNameExtensions{
								{ID: 4002, Name: "Security"},
							},
							ZPAAppSegments: []ziacommon.ZPAAppSegments{
								{ID: 5002, Name: zpaCanary},
							},
						},
					}, nil
				},
				func(context.Context, string) (*filteringrules.FirewallFilteringRules, error) { return nil, nil },
				firewallFilteringRuleSourceRecord,
			),
		},
	}

	records, err := reader.List(context.Background(), resources.ProductZIA, "firewall-filtering-rules")
	if err != nil {
		t.Fatalf("SDKReader.List(zia, firewall-filtering-rules) error = %v, want nil", err)
	}
	got := projectOneRecord(t, resources.ProductZIA, "firewall-filtering-rules", records)
	assertNoCanaries(t, "firewall-filtering-rules", got, canary, bareFreeTextToken, adminCanary, userCanary, zpaCanary)
	for _, field := range []string{"lastModifiedBy", "users", "zpaAppSegments"} {
		if _, ok := got[field]; ok {
			t.Errorf("projected firewall-filtering-rules = %#v, want no %s", got, field)
		}
	}
	if got["action"] != "ALLOW" {
		t.Errorf("projected firewall-filtering-rules action = %v, want ALLOW", got["action"])
	}
}

func TestReaderListForwardingRulesProjectsSDKShapeThroughAllowList(t *testing.T) {
	t.Parallel()

	const (
		canary            = "forwarding-rule-psk-canary"
		bareFreeTextToken = "A7b9C2d4E6f8G1h3J5k7L9m2N4p6Q8r0S2t4U6v"
		adminCanary       = "forwarding-rule-admin-canary"
		userCanary        = "forwarding-rule-user-canary"
		zpaCanary         = "forwarding-rule-zpa-canary"
	)
	reader := &SDKReader{
		cfg: validReaderConfig(),
		handlers: map[resourceKey]resourceHandler{
			{product: resources.ProductZIA, name: resourceForwardingRules}: newListGetHandler(
				resourceForwardingRules,
				func(context.Context) ([]forwardingrules.ForwardingRules, error) {
					return []forwardingrules.ForwardingRules{
						{
							ID:               303,
							Name:             "Forwarding rule psk=" + canary,
							Description:      "temporary psk=" + canary + " " + bareFreeTextToken,
							Type:             "FORWARDING",
							Order:            3,
							Rank:             9,
							ForwardMethod:    "DIRECT",
							State:            "ENABLED",
							LastModifiedTime: 1700000000,
							SrcIps:           []string{"198.51.100.10"},
							DestAddresses:    []string{"example.invalid"},
							DestCountries:    []string{"US"},
							DestIpCategories: []string{"COUNTRY_US"},
							ResCategories:    []string{"COUNTRY_CA"},
							LastModifiedBy: &ziacommon.IDNameExtensions{
								ID:   9003,
								Name: adminCanary,
							},
							Users: []ziacommon.IDNameExtensions{
								{ID: 2003, Name: userCanary},
							},
							Locations: []ziacommon.IDNameExtensions{
								{ID: 3003, Name: "Branch"},
							},
							Labels: []ziacommon.IDNameExtensions{
								{ID: 4003, Name: "Security"},
							},
							ZPAApplicationSegments: []forwardingrules.ZPAApplicationSegments{
								{ID: 5003, Name: zpaCanary, Description: "psk=" + canary},
							},
							ZPAApplicationSegmentGroups: []forwardingrules.ZPAApplicationSegmentGroups{
								{ID: 6003, Name: zpaCanary},
							},
						},
					}, nil
				},
				func(context.Context, string) (*forwardingrules.ForwardingRules, error) { return nil, nil },
				forwardingRuleSourceRecord,
			),
		},
	}

	records, err := reader.List(context.Background(), resources.ProductZIA, "forwarding-rules")
	if err != nil {
		t.Fatalf("SDKReader.List(zia, forwarding-rules) error = %v, want nil", err)
	}
	got := projectOneRecord(t, resources.ProductZIA, "forwarding-rules", records)
	assertNoCanaries(t, "forwarding-rules", got, canary, bareFreeTextToken, adminCanary, userCanary, zpaCanary)
	for _, field := range []string{"lastModifiedBy", "users", "zpaApplicationSegments", "zpaApplicationSegmentGroups"} {
		if _, ok := got[field]; ok {
			t.Errorf("projected forwarding-rules = %#v, want no %s", got, field)
		}
	}
	if got["forwardMethod"] != "DIRECT" {
		t.Errorf("projected forwarding-rules forwardMethod = %v, want DIRECT", got["forwardMethod"])
	}
}

func TestReaderListIPSourceGroupsProjectsSDKShapeThroughAllowList(t *testing.T) {
	t.Parallel()

	const (
		canary            = "ip-source-group-psk-canary"
		bareFreeTextToken = "A7b9C2d4E6f8G1h3J5k7L9m2N4p6Q8r0S2t4U6v"
	)
	reader := &SDKReader{
		cfg: validReaderConfig(),
		handlers: map[resourceKey]resourceHandler{
			{product: resources.ProductZIA, name: resourceIPSourceGroups}: newListGetHandler(
				resourceIPSourceGroups,
				func(context.Context) ([]ipsourcegroups.IPSourceGroups, error) {
					return []ipsourcegroups.IPSourceGroups{
						{
							ID:            401,
							Name:          "Source group psk=" + canary,
							Description:   "temporary psk=" + canary + " " + bareFreeTextToken,
							IPAddresses:   []string{"192.0.2.10", "198.51.100.0/24"},
							IsNonEditable: true,
						},
					}, nil
				},
				func(context.Context, string) (*ipsourcegroups.IPSourceGroups, error) { return nil, nil },
				ipSourceGroupSourceRecord,
			),
		},
	}

	records, err := reader.List(context.Background(), resources.ProductZIA, resourceIPSourceGroups)
	if err != nil {
		t.Fatalf("SDKReader.List(zia, ip-source-groups) error = %v, want nil", err)
	}
	got := projectOneRecord(t, resources.ProductZIA, resourceIPSourceGroups, records)
	assertNoCanaries(t, "ip-source-groups", got, canary, bareFreeTextToken)
	if got["isNonEditable"] != true {
		t.Errorf("projected ip-source-groups isNonEditable = %v, want true", got["isNonEditable"])
	}
}

func TestReaderListIPDestinationGroupsProjectsSDKShapeThroughAllowList(t *testing.T) {
	t.Parallel()

	const (
		canary            = "ip-destination-group-psk-canary"
		bareFreeTextToken = "A7b9C2d4E6f8G1h3J5k7L9m2N4p6Q8r0S2t4U6v"
	)
	reader := &SDKReader{
		cfg: validReaderConfig(),
		handlers: map[resourceKey]resourceHandler{
			{product: resources.ProductZIA, name: resourceIPDestGroups}: newListGetHandler(
				resourceIPDestGroups,
				func(context.Context) ([]ipdestinationgroups.IPDestinationGroups, error) {
					return []ipdestinationgroups.IPDestinationGroups{
						{
							ID:            402,
							Name:          "Destination group psk=" + canary,
							Description:   "temporary psk=" + canary + " " + bareFreeTextToken,
							Type:          "DSTN_IP",
							Addresses:     []string{"203.0.113.10", "example.invalid"},
							IPCategories:  []string{"CUSTOM_01"},
							Countries:     []string{"US"},
							IsNonEditable: false,
						},
					}, nil
				},
				func(context.Context, string) (*ipdestinationgroups.IPDestinationGroups, error) { return nil, nil },
				ipDestinationGroupSourceRecord,
			),
		},
	}

	records, err := reader.List(context.Background(), resources.ProductZIA, resourceIPDestGroups)
	if err != nil {
		t.Fatalf("SDKReader.List(zia, ip-destination-groups) error = %v, want nil", err)
	}
	got := projectOneRecord(t, resources.ProductZIA, resourceIPDestGroups, records)
	assertNoCanaries(t, "ip-destination-groups", got, canary, bareFreeTextToken)
	if got["type"] != "DSTN_IP" {
		t.Errorf("projected ip-destination-groups type = %v, want DSTN_IP", got["type"])
	}
}

func TestReaderListNetworkServicesProjectsSDKShapeThroughAllowList(t *testing.T) {
	t.Parallel()

	const (
		canary            = "network-service-psk-canary"
		bareFreeTextToken = "A7b9C2d4E6f8G1h3J5k7L9m2N4p6Q8r0S2t4U6v"
	)
	reader := &SDKReader{
		cfg: validReaderConfig(),
		handlers: map[resourceKey]resourceHandler{
			{product: resources.ProductZIA, name: resourceNetworkServices}: newListGetHandler(
				resourceNetworkServices,
				func(context.Context) ([]networkservices.NetworkServices, error) {
					return []networkservices.NetworkServices{
						{
							ID:           403,
							Name:         "Network service psk=" + canary,
							Tag:          "custom-service",
							Description:  "temporary psk=" + canary + " " + bareFreeTextToken,
							Type:         "CUSTOM",
							Protocol:     "TCP",
							SrcTCPPorts:  []networkservices.NetworkPorts{{Start: 1024, End: 65535}},
							DestTCPPorts: []networkservices.NetworkPorts{{Start: 443, End: 443}},
						},
					}, nil
				},
				func(context.Context, string) (*networkservices.NetworkServices, error) { return nil, nil },
				networkServiceSourceRecord,
			),
		},
	}

	records, err := reader.List(context.Background(), resources.ProductZIA, resourceNetworkServices)
	if err != nil {
		t.Fatalf("SDKReader.List(zia, network-services) error = %v, want nil", err)
	}
	got := projectOneRecord(t, resources.ProductZIA, resourceNetworkServices, records)
	assertNoCanaries(t, "network-services", got, canary, bareFreeTextToken)
	ports, ok := got["destTcpPorts"].([]any)
	if !ok || len(ports) != 1 {
		t.Fatalf("projected network-services destTcpPorts = %T %#v, want one port range", got["destTcpPorts"], got["destTcpPorts"])
	}
}

func TestReaderListApplicationServicesProjectsSDKShapeThroughAllowList(t *testing.T) {
	t.Parallel()

	const canary = "application-service-psk-canary"
	reader := &SDKReader{
		cfg: validReaderConfig(),
		handlers: map[resourceKey]resourceHandler{
			{product: resources.ProductZIA, name: resourceAppServices}: newListGetHandler(
				resourceAppServices,
				func(context.Context) ([]applicationservices.ApplicationServicesLite, error) {
					return []applicationservices.ApplicationServicesLite{
						{
							ID:          501,
							Name:        "Application service psk=" + canary,
							NameL10nTag: true,
						},
					}, nil
				},
				func(context.Context, string) (*applicationservices.ApplicationServicesLite, error) { return nil, nil },
				applicationServiceSourceRecord,
			),
		},
	}

	records, err := reader.List(context.Background(), resources.ProductZIA, resourceAppServices)
	if err != nil {
		t.Fatalf("SDKReader.List(zia, application-services) error = %v, want nil", err)
	}
	got := projectOneRecord(t, resources.ProductZIA, resourceAppServices, records)
	assertNoCanaries(t, "application-services", got, canary)
	if got["nameL10nTag"] != true {
		t.Errorf("projected application-services nameL10nTag = %v, want true", got["nameL10nTag"])
	}
}

func TestReaderListApplicationServiceGroupsProjectsSDKShapeThroughAllowList(t *testing.T) {
	t.Parallel()

	const canary = "application-service-group-psk-canary"
	reader := &SDKReader{
		cfg: validReaderConfig(),
		handlers: map[resourceKey]resourceHandler{
			{product: resources.ProductZIA, name: resourceAppServiceGroups}: newListGetHandler(
				resourceAppServiceGroups,
				func(context.Context) ([]appservicegroups.ApplicationServicesGroupLite, error) {
					return []appservicegroups.ApplicationServicesGroupLite{
						{
							ID:          502,
							Name:        "Application service group psk=" + canary,
							NameL10nTag: true,
						},
					}, nil
				},
				func(context.Context, string) (*appservicegroups.ApplicationServicesGroupLite, error) { return nil, nil },
				applicationServiceGroupSourceRecord,
			),
		},
	}

	records, err := reader.List(context.Background(), resources.ProductZIA, resourceAppServiceGroups)
	if err != nil {
		t.Fatalf("SDKReader.List(zia, application-service-groups) error = %v, want nil", err)
	}
	got := projectOneRecord(t, resources.ProductZIA, resourceAppServiceGroups, records)
	assertNoCanaries(t, "application-service-groups", got, canary)
	if got["nameL10nTag"] != true {
		t.Errorf("projected application-service-groups nameL10nTag = %v, want true", got["nameL10nTag"])
	}
}

func TestReaderListNetworkApplicationGroupsProjectsSDKShapeThroughAllowList(t *testing.T) {
	t.Parallel()

	const (
		canary            = "network-application-group-psk-canary"
		bareFreeTextToken = "A7b9C2d4E6f8G1h3J5k7L9m2N4p6Q8r0S2t4U6v"
	)
	reader := &SDKReader{
		cfg: validReaderConfig(),
		handlers: map[resourceKey]resourceHandler{
			{product: resources.ProductZIA, name: resourceNetworkAppGroups}: newListGetHandler(
				resourceNetworkAppGroups,
				func(context.Context) ([]networkapplicationgroups.NetworkApplicationGroups, error) {
					return []networkapplicationgroups.NetworkApplicationGroups{
						{
							ID:                  503,
							Name:                "Network application group psk=" + canary,
							NetworkApplications: []string{"HTTP", "psk=" + canary},
							Description:         "temporary psk=" + canary + " " + bareFreeTextToken,
						},
					}, nil
				},
				func(context.Context, string) (*networkapplicationgroups.NetworkApplicationGroups, error) {
					return nil, nil
				},
				networkApplicationGroupSourceRecord,
			),
		},
	}

	records, err := reader.List(context.Background(), resources.ProductZIA, resourceNetworkAppGroups)
	if err != nil {
		t.Fatalf("SDKReader.List(zia, network-application-groups) error = %v, want nil", err)
	}
	got := projectOneRecord(t, resources.ProductZIA, resourceNetworkAppGroups, records)
	assertNoCanaries(t, "network-application-groups", got, canary, bareFreeTextToken)
	apps, ok := got["networkApplications"].([]string)
	if !ok || len(apps) != 2 {
		t.Fatalf("projected network-application-groups networkApplications = %T %#v, want two applications", got["networkApplications"], got["networkApplications"])
	}
}

func TestZIADeferredOrdinaryBatchProjectionBoundaries(t *testing.T) {
	t.Parallel()

	const (
		canary            = "zia-deferred-batch-psk-canary"
		bareFreeTextToken = "A7b9C2d4E6f8G1h3J5k7L9m2N4p6Q8r0S2t4U6v"
	)
	cases := []struct {
		name         string
		record       resources.SourceRecord
		wantStandard map[string]any
		absentFields []string
	}{
		{
			name: resourceNetworkSvcGroups,
			record: networkServiceGroupSourceRecord(networkservicegroups.NetworkServiceGroups{
				ID:          510,
				Name:        "network service group",
				Description: "temporary psk=" + canary + " " + bareFreeTextToken,
				Services: []networkservicegroups.Services{
					{
						ID:          10,
						Name:        "service",
						Tag:         "tag psk=" + canary,
						Description: "service psk=" + canary,
					},
				},
			}),
			wantStandard: map[string]any{"services": "present"},
		},
		{
			name: resourceNetworkApps,
			record: networkApplicationSourceRecord(networkapplications.NetworkApplications{
				ID:             "APP_CANARY",
				ParentCategory: "parent",
				Description:    "temporary psk=" + canary + " " + bareFreeTextToken,
				Deprecated:     true,
			}),
			wantStandard: map[string]any{"deprecated": true},
		},
		{
			name: resourceEmailProfiles,
			record: emailProfileSourceRecord(emailprofiles.EmailProfiles{
				ID:          511,
				Name:        "email profile",
				Description: "temporary psk=" + canary + " " + bareFreeTextToken,
				Emails:      []string{"recipient@example.invalid"},
			}),
			wantStandard: map[string]any{"emails": "present"},
		},
		{
			name: resourceDLPIncidentRcvs,
			record: dlpIncidentReceiverSourceRecord(dlpincidentreceivers.IncidentReceiverServers{
				ID:     512,
				Name:   "incident receiver",
				URL:    "https://receiver.example.invalid/ingest",
				Status: "ENABLED",
				Flags:  7,
			}),
			wantStandard: map[string]any{"url": "present"},
		},
		{
			name: resourceDLPNotifyTmpls,
			record: dlpNotificationTemplateSourceRecord(dlpnotificationtemplates.DlpNotificationTemplates{
				ID:               513,
				Name:             "notification template",
				Subject:          "subject psk=" + canary + " " + bareFreeTextToken,
				AttachContent:    true,
				PlainTextMessage: "plain body psk=" + canary,
				HtmlMessage:      "<p>html body psk=" + canary + "</p>",
				TLSEnabled:       true,
			}),
			wantStandard: map[string]any{"attachContent": true},
			absentFields: []string{
				"plainTextMessage",
				"htmlMessage",
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := projectOneRecord(t, resources.ProductZIA, tc.name, []resources.SourceRecord{tc.record})
			assertNoCanaries(t, tc.name, got, canary, bareFreeTextToken)
			for field, want := range tc.wantStandard {
				value, ok := got[field]
				if !ok {
					t.Fatalf("projected %s missing standard field %q", tc.name, field)
				}
				if want != "present" && value != want {
					t.Fatalf("projected %s field %q = %v, want %v", tc.name, field, value, want)
				}
			}
			for _, field := range tc.absentFields {
				if _, ok := got[field]; ok {
					t.Fatalf("projected %s unexpectedly included %q", tc.name, field)
				}
			}

			share := projectOneRecordInMode(t, resources.ProductZIA, tc.name, redact.ModeShare, []resources.SourceRecord{tc.record})
			assertNoCanaries(t, tc.name+" share", share, canary, bareFreeTextToken)
			if _, ok := share["emails"]; ok {
				t.Fatalf("share projection for %s included emails", tc.name)
			}
			if _, ok := share["url"]; ok {
				t.Fatalf("share projection for %s included url", tc.name)
			}
			if _, ok := share["services"]; ok {
				t.Fatalf("share projection for %s included services", tc.name)
			}
			if _, ok := share["attachContent"]; ok {
				t.Fatalf("share projection for %s included attachContent", tc.name)
			}
		})
	}
}

func TestZIAIdentityDeviceProjectionBoundaries(t *testing.T) {
	t.Parallel()

	const (
		freeTextCanary = "zia-identity-device-psk-canary"
		longToken      = "A7b9C2d4E6f8G1h3J5k7L9m2N4p6Q8r0S2t4U6v"
	)

	departmentSpec := mustFindSpec(t, resources.ProductZIA, resourceDepartments)
	departmentRecords := []resources.SourceRecord{departmentSourceRecord(userdepartments.Department{
		ID:       20,
		Name:     "Engineering",
		IdpID:    7,
		Comments: "dept psk=" + freeTextCanary + " " + longToken,
		Deleted:  false,
	})}
	departments, departmentReports, err := resources.ProjectRecordsAndVerify(departmentSpec, redact.ModeStandard, departmentRecords)
	if err != nil {
		t.Fatalf("ProjectRecordsAndVerify(zia/departments standard) error = %v, want nil", err)
	}
	departmentGot := departments.Records()[0].Fields()
	if departmentGot["name"] != "Engineering" || departmentGot["idpId"] != 7 || departmentGot["deleted"] != false {
		t.Errorf("ProjectRecordsAndVerify(zia/departments standard) = %#v, want department fields visible", departmentGot)
	}
	if strings.Contains(fmt.Sprint(departmentGot), freeTextCanary) {
		t.Errorf("ProjectRecordsAndVerify(zia/departments standard) = %#v, want comment canary redacted", departmentGot)
	}
	assertReportContains(t, departmentReports[0].RedactedFields, "comments")

	userSpec := mustFindSpec(t, resources.ProductZIA, resourceUsers)
	userRecords := []resources.SourceRecord{userSourceRecord(ziausers.Users{
		ID:            21,
		Name:          "Jane Doe",
		Email:         "jane.doe@example.internal",
		Groups:        []ziacommon.UserGroups{{ID: 30, Name: "Engineering Admins", Comments: "group psk=" + freeTextCanary}},
		Department:    &ziacommon.UserDepartment{ID: 20, Name: "Engineering", Comments: "dept psk=" + freeTextCanary},
		Comments:      "user psk=" + freeTextCanary + " " + longToken,
		TempAuthEmail: "jane.temp@example.internal",
		AuthMethods:   []string{"PASSWORD", "SAML"},
		Password:      "password psk=" + freeTextCanary,
		AdminUser:     true,
		Type:          "EMPLOYEE",
		Deleted:       false,
	})}
	users, userReports, err := resources.ProjectRecordsAndVerify(userSpec, redact.ModeStandard, userRecords)
	if err != nil {
		t.Fatalf("ProjectRecordsAndVerify(zia/users standard) error = %v, want nil", err)
	}
	userGot := users.Records()[0].Fields()
	if userGot["name"] != "Jane Doe" || userGot["email"] != "jane.doe@example.internal" || userGot["tempAuthEmail"] != "jane.temp@example.internal" {
		t.Errorf("ProjectRecordsAndVerify(zia/users standard) = %#v, want employee identity visible", userGot)
	}
	if userGot["adminUser"] != true || userGot["type"] != "EMPLOYEE" || userGot["deleted"] != false {
		t.Errorf("ProjectRecordsAndVerify(zia/users standard) = %#v, want status fields visible", userGot)
	}
	userGroups := mustProjectedList(t, userGot, "groups")
	group := userGroups[0].(map[string]any)
	if group["id"] != 30 || group["name"] != "Engineering Admins" {
		t.Errorf("ProjectRecordsAndVerify(zia/users standard) groups = %#v, want id/name reference", group)
	}
	if _, ok := group["comments"]; ok {
		t.Errorf("ProjectRecordsAndVerify(zia/users standard) group reference includes comments, want id/name only")
	}
	userDepartment, ok := userGot["department"].(map[string]any)
	if !ok {
		t.Fatalf("ProjectRecordsAndVerify(zia/users standard) department = %T, want map[string]any", userGot["department"])
	}
	if userDepartment["id"] != 20 || userDepartment["name"] != "Engineering" {
		t.Errorf("ProjectRecordsAndVerify(zia/users standard) department = %#v, want id/name reference", userDepartment)
	}
	if _, ok := userGot["password"]; ok {
		t.Errorf("ProjectRecordsAndVerify(zia/users standard) includes password, want dropped")
	}
	if strings.Contains(fmt.Sprint(userGot), freeTextCanary) {
		t.Errorf("ProjectRecordsAndVerify(zia/users standard) = %#v, want secret/free-text canary absent", userGot)
	}
	assertReportContains(t, userReports[0].DroppedFields, "password")
	assertReportContains(t, userReports[0].RedactedFields, "comments")

	userShare, userShareReports, err := resources.ProjectRecordsAndVerify(userSpec, redact.ModeShare, userRecords)
	if err != nil {
		t.Fatalf("ProjectRecordsAndVerify(zia/users share) error = %v, want nil", err)
	}
	userShareGot := userShare.Records()[0].Fields()
	for _, field := range []string{"name", "email", "groups", "department", "comments", "tempAuthEmail", "authMethods", "password"} {
		if _, ok := userShareGot[field]; ok {
			t.Errorf("ProjectRecordsAndVerify(zia/users share) includes %s, want dropped", field)
		}
		assertReportContains(t, userShareReports[0].DroppedFields, field)
	}
	if userShareGot["adminUser"] != true || userShareGot["type"] != "EMPLOYEE" || userShareGot["deleted"] != false {
		t.Errorf("ProjectRecordsAndVerify(zia/users share) = %#v, want share-safe status fields", userShareGot)
	}

	deviceSpec := mustFindSpec(t, resources.ProductZIA, resourceDevices)
	deviceRecords := []resources.SourceRecord{deviceSourceRecord(devicegroups.Devices{
		ID:              22,
		Name:            "macbook-123",
		DeviceGroupType: "USER_DEFINED",
		DeviceModel:     "MacBookPro18,3",
		OSType:          "macOS",
		OSVersion:       "14.5",
		Description:     "device psk=" + freeTextCanary + " " + longToken,
		OwnerUserId:     21,
		OwnerName:       "Jane Doe",
		HostName:        "macbook-123.example.internal",
	})}
	devices, deviceReports, err := resources.ProjectRecordsAndVerify(deviceSpec, redact.ModeStandard, deviceRecords)
	if err != nil {
		t.Fatalf("ProjectRecordsAndVerify(zia/devices standard) error = %v, want nil", err)
	}
	deviceGot := devices.Records()[0].Fields()
	if deviceGot["name"] != "macbook-123" || deviceGot["ownerName"] != "Jane Doe" || deviceGot["hostName"] != "macbook-123.example.internal" {
		t.Errorf("ProjectRecordsAndVerify(zia/devices standard) = %#v, want device identity visible", deviceGot)
	}
	if deviceGot["deviceGroupType"] != "USER_DEFINED" || deviceGot["osType"] != "macOS" || deviceGot["osVersion"] != "14.5" {
		t.Errorf("ProjectRecordsAndVerify(zia/devices standard) = %#v, want device status/config visible", deviceGot)
	}
	if strings.Contains(fmt.Sprint(deviceGot), freeTextCanary) {
		t.Errorf("ProjectRecordsAndVerify(zia/devices standard) = %#v, want description canary redacted", deviceGot)
	}
	assertReportContains(t, deviceReports[0].RedactedFields, "description")

	deviceShare, deviceShareReports, err := resources.ProjectRecordsAndVerify(deviceSpec, redact.ModeShare, deviceRecords)
	if err != nil {
		t.Fatalf("ProjectRecordsAndVerify(zia/devices share) error = %v, want nil", err)
	}
	deviceShareGot := deviceShare.Records()[0].Fields()
	for _, field := range []string{"name", "deviceModel", "description", "ownerUserId", "ownerName", "hostName"} {
		if _, ok := deviceShareGot[field]; ok {
			t.Errorf("ProjectRecordsAndVerify(zia/devices share) includes %s, want dropped", field)
		}
		assertReportContains(t, deviceShareReports[0].DroppedFields, field)
	}
	if deviceShareGot["deviceGroupType"] != "USER_DEFINED" || deviceShareGot["osType"] != "macOS" || deviceShareGot["osVersion"] != "14.5" {
		t.Errorf("ProjectRecordsAndVerify(zia/devices share) = %#v, want share-safe device status/config", deviceShareGot)
	}
}

func TestNetworkApplicationsListAvoidsUnboundedSDKPagination(t *testing.T) {
	t.Parallel()

	body, err := os.ReadFile("reader.go")
	if err != nil {
		t.Fatalf("ReadFile(reader.go) error = %v, want nil", err)
	}
	source := string(body)

	if strings.Contains(source, "return networkapplications.GetAll(ctx, service") {
		t.Fatalf("network-applications list uses SDK GetAll; want bounded single-page read")
	}
	want := `ziacommon.ReadPage(ctx, service.Client, "/zia/api/v1/networkApplications", 1, &applications, 5000)`
	if !strings.Contains(source, want) {
		t.Fatalf("network-applications list missing bounded SDK page read %q", want)
	}
}

func TestReaderListTimeWindowsProjectsSDKShapeThroughAllowList(t *testing.T) {
	t.Parallel()

	const canary = "time-window-psk-canary"
	reader := &SDKReader{
		cfg: validReaderConfig(),
		handlers: map[resourceKey]resourceHandler{
			{product: resources.ProductZIA, name: resourceTimeWindows}: newListGetHandler(
				resourceTimeWindows,
				func(context.Context) ([]timewindow.TimeWindow, error) {
					return []timewindow.TimeWindow{
						{
							ID:        601,
							Name:      "Time window psk=" + canary,
							StartTime: 800,
							EndTime:   1700,
							DayOfWeek: []string{"MON", "psk=" + canary},
						},
					}, nil
				},
				func(context.Context, string) (*timewindow.TimeWindow, error) { return nil, nil },
				timeWindowSourceRecord,
			),
		},
	}

	records, err := reader.List(context.Background(), resources.ProductZIA, resourceTimeWindows)
	if err != nil {
		t.Fatalf("SDKReader.List(zia, time-windows) error = %v, want nil", err)
	}
	got := projectOneRecord(t, resources.ProductZIA, resourceTimeWindows, records)
	assertNoCanaries(t, "time-windows", got, canary)
	if got["startTime"] != int32(800) {
		t.Errorf("projected time-windows startTime = %v, want 800", got["startTime"])
	}
}

func TestReaderListProxiesProjectsSDKShapeThroughAllowList(t *testing.T) {
	t.Parallel()

	const (
		canary            = "proxy-psk-canary"
		bareFreeTextToken = "A7b9C2d4E6f8G1h3J5k7L9m2N4p6Q8r0S2t4U6v"
	)
	reader := &SDKReader{
		cfg: validReaderConfig(),
		handlers: map[resourceKey]resourceHandler{
			{product: resources.ProductZIA, name: resourceProxies}: newListGetHandler(
				resourceProxies,
				func(context.Context) ([]proxies.Proxies, error) {
					return []proxies.Proxies{
						{
							ID:                    602,
							Name:                  "Proxy psk=" + canary,
							Type:                  "PROXYCHAIN",
							Address:               "proxy.example.invalid",
							Port:                  8080,
							Description:           "temporary psk=" + canary + " " + bareFreeTextToken,
							InsertXauHeader:       true,
							Base64EncodeXauHeader: true,
							Cert: &ziacommon.IDNameExternalID{
								ID:         11,
								Name:       "Cert psk=" + canary,
								ExternalID: canary,
							},
							LastModifiedBy: &ziacommon.IDNameExternalID{
								ID:         12,
								Name:       "Admin psk=" + canary,
								ExternalID: canary,
							},
							LastModifiedTime: 12345,
						},
					}, nil
				},
				func(context.Context, string) (*proxies.Proxies, error) { return nil, nil },
				proxySourceRecord,
			),
		},
	}

	records, err := reader.List(context.Background(), resources.ProductZIA, resourceProxies)
	if err != nil {
		t.Fatalf("SDKReader.List(zia, proxies) error = %v, want nil", err)
	}
	got := projectOneRecord(t, resources.ProductZIA, resourceProxies, records)
	assertNoCanaries(t, "proxies", got, canary, bareFreeTextToken)
	if got["port"] != 8080 {
		t.Errorf("projected proxies port = %v, want 8080", got["port"])
	}
}

func TestReaderListProxyGatewaysProjectsSDKShapeThroughAllowList(t *testing.T) {
	t.Parallel()

	const (
		canary            = "proxy-gateway-psk-canary"
		bareFreeTextToken = "A7b9C2d4E6f8G1h3J5k7L9m2N4p6Q8r0S2t4U6v"
	)
	reader := &SDKReader{
		cfg: validReaderConfig(),
		handlers: map[resourceKey]resourceHandler{
			{product: resources.ProductZIA, name: resourceProxyGateways}: newListGetHandler(
				resourceProxyGateways,
				func(context.Context) ([]proxygateways.ProxyGateways, error) {
					return []proxygateways.ProxyGateways{
						{
							ID:          603,
							Name:        "Proxy gateway psk=" + canary,
							Description: "temporary psk=" + canary + " " + bareFreeTextToken,
							FailClosed:  true,
							Type:        "PROXYCHAIN",
							PrimaryProxy: &ziacommon.IDNameExternalID{
								ID:         21,
								Name:       "Primary psk=" + canary,
								ExternalID: canary,
							},
							SecondaryProxy: &ziacommon.IDNameExternalID{
								ID:         22,
								Name:       "Secondary psk=" + canary,
								ExternalID: canary,
							},
							LastModifiedBy: &ziacommon.IDNameExtensions{
								ID:   23,
								Name: "Admin psk=" + canary,
							},
							LastModifiedTime: 12346,
						},
					}, nil
				},
				func(context.Context, string) (*proxygateways.ProxyGateways, error) { return nil, nil },
				proxyGatewaySourceRecord,
			),
		},
	}

	records, err := reader.List(context.Background(), resources.ProductZIA, resourceProxyGateways)
	if err != nil {
		t.Fatalf("SDKReader.List(zia, proxy-gateways) error = %v, want nil", err)
	}
	got := projectOneRecord(t, resources.ProductZIA, resourceProxyGateways, records)
	assertNoCanaries(t, "proxy-gateways", got, canary, bareFreeTextToken)
	if got["failClosed"] != true {
		t.Errorf("projected proxy-gateways failClosed = %v, want true", got["failClosed"])
	}
}

func TestReaderListDedicatedIPGatewaysProjectsSDKShapeThroughAllowList(t *testing.T) {
	t.Parallel()

	const (
		canary            = "dedicated-ip-gateway-psk-canary"
		bareFreeTextToken = "A7b9C2d4E6f8G1h3J5k7L9m2N4p6Q8r0S2t4U6v"
	)
	reader := &SDKReader{
		cfg: validReaderConfig(),
		handlers: map[resourceKey]resourceHandler{
			{product: resources.ProductZIA, name: resourceDedicatedIPGWs}: newListGetHandler(
				resourceDedicatedIPGWs,
				func(context.Context) ([]proxies.DedicatedIPGateways, error) {
					return []proxies.DedicatedIPGateways{
						{
							Id:          604,
							Name:        "Dedicated IP gateway psk=" + canary,
							Description: "temporary psk=" + canary + " " + bareFreeTextToken,
							PrimaryDataCenter: &ziacommon.IDNameExtensions{
								ID:   31,
								Name: "Primary DC psk=" + canary,
							},
							SecondaryDataCenter: &ziacommon.IDNameExtensions{
								ID:   32,
								Name: "Secondary DC psk=" + canary,
							},
							LastModifiedBy: &ziacommon.IDNameExtensions{
								ID:   33,
								Name: "Admin psk=" + canary,
							},
							CreateTime:       12340,
							LastModifiedTime: 12347,
							Default:          true,
						},
					}, nil
				},
				func(context.Context, string) (*proxies.DedicatedIPGateways, error) { return nil, nil },
				dedicatedIPGatewaySourceRecord,
			),
		},
	}

	records, err := reader.List(context.Background(), resources.ProductZIA, resourceDedicatedIPGWs)
	if err != nil {
		t.Fatalf("SDKReader.List(zia, dedicated-ip-gateways) error = %v, want nil", err)
	}
	got := projectOneRecord(t, resources.ProductZIA, resourceDedicatedIPGWs, records)
	assertNoCanaries(t, "dedicated-ip-gateways", got, canary, bareFreeTextToken)
	if got["default"] != true {
		t.Errorf("projected dedicated-ip-gateways default = %v, want true", got["default"])
	}
}

func TestReaderListTimeIntervalsProjectsSDKShapeThroughAllowList(t *testing.T) {
	t.Parallel()

	const canary = "time-interval-psk-canary"
	reader := &SDKReader{
		cfg: validReaderConfig(),
		handlers: map[resourceKey]resourceHandler{
			{product: resources.ProductZIA, name: resourceTimeIntervals}: newListGetHandler(
				resourceTimeIntervals,
				func(context.Context) ([]timeintervals.TimeInterval, error) {
					return []timeintervals.TimeInterval{
						{
							ID:         701,
							Name:       "Business hours psk=" + canary,
							StartTime:  800,
							EndTime:    1700,
							DaysOfWeek: []string{"MON", "TUE", "psk=" + canary},
						},
					}, nil
				},
				intIDGetter(func(context.Context, int) (*timeintervals.TimeInterval, error) { return nil, nil }),
				timeIntervalSourceRecord,
			),
		},
	}

	records, err := reader.List(context.Background(), resources.ProductZIA, resourceTimeIntervals)
	if err != nil {
		t.Fatalf("SDKReader.List(zia, time-intervals) error = %v, want nil", err)
	}
	got := projectOneRecord(t, resources.ProductZIA, resourceTimeIntervals, records)
	assertNoCanaries(t, "time-intervals", got, canary)
	if got["startTime"] != 800 {
		t.Errorf("projected time-intervals startTime = %v, want 800", got["startTime"])
	}
}

func TestReaderListBandwidthClassesProjectsSDKShapeThroughAllowList(t *testing.T) {
	t.Parallel()

	const canary = "bandwidth-class-psk-canary"
	reader := &SDKReader{
		cfg: validReaderConfig(),
		handlers: map[resourceKey]resourceHandler{
			{product: resources.ProductZIA, name: resourceBandwidthClasses}: newListGetHandler(
				resourceBandwidthClasses,
				func(context.Context) ([]bandwidthclasses.BandwidthClasses, error) {
					return []bandwidthclasses.BandwidthClasses{
						{
							ID:                       702,
							IsNameL10nTag:            true,
							Name:                     "Streaming psk=" + canary,
							GetfileSize:              "10MB",
							FileSize:                 "20MB",
							Type:                     "WEB_APPLICATION",
							WebApplications:          []string{"YOUTUBE"},
							Urls:                     []string{"https://operator:" + canary + "@example.invalid"},
							ApplicationServiceGroups: []string{"app-service-group"},
							NetworkApplications:      []string{"HTTP"},
							NetworkServices:          []string{"service"},
							UrlCategories:            []string{"CUSTOM_01"},
							Applications:             []string{"app"},
						},
					}, nil
				},
				intIDGetter(func(context.Context, int) (*bandwidthclasses.BandwidthClasses, error) { return nil, nil }),
				bandwidthClassSourceRecord,
			),
		},
	}

	records, err := reader.List(context.Background(), resources.ProductZIA, resourceBandwidthClasses)
	if err != nil {
		t.Fatalf("SDKReader.List(zia, bandwidth-classes) error = %v, want nil", err)
	}
	got := projectOneRecord(t, resources.ProductZIA, resourceBandwidthClasses, records)
	assertNoCanaries(t, "bandwidth-classes", got, canary)
	if got["type"] != "WEB_APPLICATION" {
		t.Errorf("projected bandwidth-classes type = %v, want WEB_APPLICATION", got["type"])
	}
}

func TestReaderListBandwidthControlRulesProjectsSDKShapeThroughAllowList(t *testing.T) {
	t.Parallel()

	const (
		canary            = "bandwidth-rule-psk-canary"
		bareFreeTextToken = "A7b9C2d4E6f8G1h3J5k7L9m2N4p6Q8r0S2t4U6v"
		adminCanary       = "bandwidth-rule-admin-canary"
	)
	reader := &SDKReader{
		cfg: validReaderConfig(),
		handlers: map[resourceKey]resourceHandler{
			{product: resources.ProductZIA, name: resourceBandwidthRules}: newListGetHandler(
				resourceBandwidthRules,
				func(context.Context) ([]bandwidthcontrolrules.BandwidthControlRules, error) {
					return []bandwidthcontrolrules.BandwidthControlRules{
						{
							ID:                703,
							Name:              "Bandwidth rule psk=" + canary,
							Description:       "temporary psk=" + canary + " " + bareFreeTextToken,
							Order:             1,
							State:             "ENABLED",
							MaxBandwidth:      100,
							MinBandwidth:      10,
							Rank:              7,
							LastModifiedTime:  1700000000,
							AccessControl:     "READ_WRITE",
							DefaultRule:       false,
							Protocols:         []string{"ANY_RULE"},
							DeviceTrustLevels: []string{"ANY"},
							LastModifiedBy: &ziacommon.IDNameExtensions{
								ID:   9001,
								Name: adminCanary,
							},
							BandwidthClasses: []ziacommon.IDNameExtensions{
								{ID: 31, Name: "Streaming psk=" + canary},
							},
							LocationGroups: []ziacommon.IDNameExtensions{
								{ID: 32, Name: "Location group"},
							},
							Labels: []ziacommon.IDNameExtensions{
								{ID: 33, Name: "Security"},
							},
							Devices: []ziacommon.IDNameExtensions{
								{ID: 34, Name: "Device"},
							},
							DeviceGroups: []ziacommon.IDNameExtensions{
								{ID: 35, Name: "Device group"},
							},
							Locations: []ziacommon.IDNameExtensions{
								{ID: 36, Name: "Location"},
							},
							TimeWindows: []ziacommon.IDNameExtensions{
								{ID: 37, Name: "Business hours"},
							},
						},
					}, nil
				},
				intIDGetter(func(context.Context, int) (*bandwidthcontrolrules.BandwidthControlRules, error) { return nil, nil }),
				bandwidthControlRuleSourceRecord,
			),
		},
	}

	records, err := reader.List(context.Background(), resources.ProductZIA, resourceBandwidthRules)
	if err != nil {
		t.Fatalf("SDKReader.List(zia, bandwidth-control-rules) error = %v, want nil", err)
	}
	got := projectOneRecord(t, resources.ProductZIA, resourceBandwidthRules, records)
	assertNoCanaries(t, "bandwidth-control-rules", got, canary, bareFreeTextToken, adminCanary)
	if _, ok := got["lastModifiedBy"]; ok {
		t.Errorf("projected bandwidth-control-rules = %#v, want no lastModifiedBy", got)
	}
	if got["maxBandwidth"] != 100 {
		t.Errorf("projected bandwidth-control-rules maxBandwidth = %v, want 100", got["maxBandwidth"])
	}
}

func TestReaderListDNSGatewaysProjectsSDKShapeThroughAllowList(t *testing.T) {
	t.Parallel()

	const (
		canary      = "dns-gateway-psk-canary"
		adminCanary = "dns-gateway-admin-canary"
	)
	reader := &SDKReader{
		cfg: validReaderConfig(),
		handlers: map[resourceKey]resourceHandler{
			{product: resources.ProductZIA, name: resourceDNSGateways}: newListGetHandler(
				resourceDNSGateways,
				func(context.Context) ([]dnsgateways.DNSGateways, error) {
					return []dnsgateways.DNSGateways{
						{
							ID:                  704,
							Name:                "DNS gateway psk=" + canary,
							DnsGatewayType:      "IP",
							PrimaryIpOrFqdn:     "https://operator:" + canary + "@example.invalid",
							PrimaryPorts:        []int{53},
							SecondaryIpOrFqdn:   "secondary.example.invalid",
							SecondaryPorts:      []int{853},
							Protocols:           []string{"UDP", "TCP"},
							FailureBehavior:     "ALLOW",
							LastModifiedTime:    1700000000,
							AutoCreated:         false,
							NatZtrGateway:       true,
							DnsGatewayProtocols: []string{"DNS", "DNS_OVER_TLS"},
							LastModifiedBy: &ziacommon.IDNameExtensions{
								ID:   9002,
								Name: adminCanary,
							},
						},
					}, nil
				},
				intIDGetter(func(context.Context, int) (*dnsgateways.DNSGateways, error) { return nil, nil }),
				dnsGatewaySourceRecord,
			),
		},
	}

	records, err := reader.List(context.Background(), resources.ProductZIA, resourceDNSGateways)
	if err != nil {
		t.Fatalf("SDKReader.List(zia, dns-gateways) error = %v, want nil", err)
	}
	got := projectOneRecord(t, resources.ProductZIA, resourceDNSGateways, records)
	assertNoCanaries(t, "dns-gateways", got, canary, adminCanary)
	if _, ok := got["lastModifiedBy"]; ok {
		t.Errorf("projected dns-gateways = %#v, want no lastModifiedBy", got)
	}
	if got["natZtrGateway"] != true {
		t.Errorf("projected dns-gateways natZtrGateway = %v, want true", got["natZtrGateway"])
	}
}

func TestReaderListNATControlRulesProjectsSDKShapeThroughAllowList(t *testing.T) {
	t.Parallel()

	const (
		canary            = "nat-rule-psk-canary"
		bareFreeTextToken = "A7b9C2d4E6f8G1h3J5k7L9m2N4p6Q8r0S2t4U6v"
		adminCanary       = "nat-rule-admin-canary"
	)
	reader := &SDKReader{
		cfg: validReaderConfig(),
		handlers: map[resourceKey]resourceHandler{
			{product: resources.ProductZIA, name: resourceNATRules}: newListGetHandler(
				resourceNATRules,
				func(context.Context) ([]natcontrol.NatControlPolicies, error) {
					return []natcontrol.NatControlPolicies{
						{
							AccessControl:       "READ_WRITE",
							ID:                  705,
							Name:                "NAT rule psk=" + canary,
							Order:               2,
							Rank:                8,
							Description:         "temporary psk=" + canary + " " + bareFreeTextToken,
							State:               "ENABLED",
							RedirectFqdn:        "https://operator:" + canary + "@example.invalid",
							RedirectIp:          "203.0.113.10",
							RedirectPort:        443,
							LastModifiedTime:    1700000000,
							TrustedResolverRule: true,
							EnableFullLogging:   true,
							Predefined:          false,
							DefaultRule:         false,
							DestAddresses:       []string{"example.invalid"},
							SrcIps:              []string{"192.0.2.10"},
							DestCountries:       []string{"US"},
							DestIpCategories:    []string{"COUNTRY_US"},
							ResCategories:       []string{"COUNTRY_CA"},
							Locations:           []ziacommon.IDNameExtensions{{ID: 41, Name: "Location"}},
							LocationGroups:      []ziacommon.IDNameExtensions{{ID: 42, Name: "Location group"}},
							Groups:              []ziacommon.IDNameExtensions{{ID: 43, Name: "Group"}},
							Departments:         []ziacommon.IDNameExtensions{{ID: 44, Name: "Department"}},
							Users:               []ziacommon.IDNameExtensions{{ID: 45, Name: "User"}},
							TimeWindows:         []ziacommon.IDNameExtensions{{ID: 46, Name: "Time window"}},
							SrcIpGroups:         []ziacommon.IDNameExtensions{{ID: 47, Name: "Source group"}},
							DestIpGroups:        []ziacommon.IDNameExtensions{{ID: 48, Name: "Destination group"}},
							NwServices:          []ziacommon.IDNameExtensions{{ID: 49, Name: "Service"}},
							NwServiceGroups:     []ziacommon.IDNameExtensions{{ID: 50, Name: "Service group"}},
							LastModifiedBy: &ziacommon.IDNameExtensions{
								ID:   9003,
								Name: adminCanary,
							},
							Devices:      []ziacommon.IDNameExtensions{{ID: 51, Name: "Device"}},
							DeviceGroups: []ziacommon.IDNameExtensions{{ID: 52, Name: "Device group"}},
							Labels:       []ziacommon.IDNameExtensions{{ID: 53, Name: "Security"}},
						},
					}, nil
				},
				intIDGetter(func(context.Context, int) (*natcontrol.NatControlPolicies, error) { return nil, nil }),
				natControlRuleSourceRecord,
			),
		},
	}

	records, err := reader.List(context.Background(), resources.ProductZIA, resourceNATRules)
	if err != nil {
		t.Fatalf("SDKReader.List(zia, nat-control-rules) error = %v, want nil", err)
	}
	got := projectOneRecord(t, resources.ProductZIA, resourceNATRules, records)
	assertNoCanaries(t, "nat-control-rules", got, canary, bareFreeTextToken, adminCanary)
	if _, ok := got["lastModifiedBy"]; ok {
		t.Errorf("projected nat-control-rules = %#v, want no lastModifiedBy", got)
	}
	if got["redirectPort"] != 443 {
		t.Errorf("projected nat-control-rules redirectPort = %v, want 443", got["redirectPort"])
	}
}

func TestReaderListGroupsProjectsSDKShapeThroughAllowList(t *testing.T) {
	t.Parallel()

	const (
		canary            = "group-psk-canary"
		bareFreeTextToken = "A7b9C2d4E6f8G1h3J5k7L9m2N4p6Q8r0S2t4U6v"
	)
	reader := &SDKReader{
		cfg: validReaderConfig(),
		handlers: map[resourceKey]resourceHandler{
			{product: resources.ProductZIA, name: resourceGroups}: newListGetHandler(
				resourceGroups,
				func(context.Context) ([]usergroups.Groups, error) {
					return []usergroups.Groups{
						{
							ID:              801,
							Name:            "Finance psk=" + canary,
							IdpID:           17,
							Comments:        "temporary psk=" + canary + " " + bareFreeTextToken,
							IsSystemDefined: true,
						},
					}, nil
				},
				intIDGetter(func(context.Context, int) (*usergroups.Groups, error) { return nil, nil }),
				groupSourceRecord,
			),
		},
	}

	records, err := reader.List(context.Background(), resources.ProductZIA, resourceGroups)
	if err != nil {
		t.Fatalf("SDKReader.List(zia, groups) error = %v, want nil", err)
	}
	got := projectOneRecord(t, resources.ProductZIA, resourceGroups, records)
	assertNoCanaries(t, "groups", got, canary, bareFreeTextToken)
	if got["isSystemDefined"] != true {
		t.Errorf("projected groups isSystemDefined = %v, want true", got["isSystemDefined"])
	}
}

func TestReaderListDeviceGroupsProjectsSDKShapeThroughAllowList(t *testing.T) {
	t.Parallel()

	const (
		canary            = "device-group-psk-canary"
		bareFreeTextToken = "A7b9C2d4E6f8G1h3J5k7L9m2N4p6Q8r0S2t4U6v"
	)
	reader := &SDKReader{
		cfg: validReaderConfig(),
		handlers: map[resourceKey]resourceHandler{
			{product: resources.ProductZIA, name: resourceDeviceGroups}: newListGetHandler(
				resourceDeviceGroups,
				func(context.Context) ([]devicegroups.DeviceGroups, error) {
					return []devicegroups.DeviceGroups{
						{
							ID:          804,
							Name:        "Managed laptops psk=" + canary,
							GroupType:   "STATIC",
							Description: "temporary psk=" + canary + " " + bareFreeTextToken,
							OSType:      "WINDOWS",
							Predefined:  false,
							DeviceNames: "device psk=" + canary,
							DeviceCount: 12,
						},
					}, nil
				},
				intIDGetter(func(context.Context, int) (*devicegroups.DeviceGroups, error) { return nil, nil }),
				deviceGroupSourceRecord,
			),
		},
	}

	records, err := reader.List(context.Background(), resources.ProductZIA, resourceDeviceGroups)
	if err != nil {
		t.Fatalf("SDKReader.List(zia, device-groups) error = %v, want nil", err)
	}
	got := projectOneRecord(t, resources.ProductZIA, resourceDeviceGroups, records)
	assertNoCanaries(t, "device-groups", got, canary, bareFreeTextToken)
	if got["deviceCount"] != 12 {
		t.Errorf("projected device-groups deviceCount = %v, want 12", got["deviceCount"])
	}
}

func TestReaderListWorkloadGroupsProjectsSDKShapeThroughAllowList(t *testing.T) {
	t.Parallel()

	const (
		canary            = "workload-group-psk-canary"
		bareFreeTextToken = "A7b9C2d4E6f8G1h3J5k7L9m2N4p6Q8r0S2t4U6v"
		adminCanary       = "workload-group-admin-canary"
		tagCanary         = "workload-group-tag-canary"
	)
	reader := &SDKReader{
		cfg: validReaderConfig(),
		handlers: map[resourceKey]resourceHandler{
			{product: resources.ProductZIA, name: resourceWorkloadGroups}: newListGetHandler(
				resourceWorkloadGroups,
				func(context.Context) ([]workloadgroups.WorkloadGroup, error) {
					return []workloadgroups.WorkloadGroup{
						{
							ID:               806,
							Name:             "Workload group psk=" + canary,
							Description:      "temporary psk=" + canary + " " + bareFreeTextToken,
							Expression:       "tag.value psk=" + canary,
							LastModifiedTime: 1700000000,
							LastModifiedBy: &ziacommon.IDNameExtensions{
								ID:   9004,
								Name: adminCanary,
							},
							WorkloadTagExpression: workloadgroups.WorkloadTagExpression{
								ExpressionContainers: []workloadgroups.ExpressionContainer{
									{
										TagType:  "TAG",
										Operator: "AND",
										TagContainer: workloadgroups.TagContainer{
											Operator: "OR",
											Tags: []workloadgroups.Tags{
												{
													Key:   "env",
													Value: tagCanary,
												},
											},
										},
									},
								},
							},
						},
					}, nil
				},
				intIDGetter(func(context.Context, int) (*workloadgroups.WorkloadGroup, error) { return nil, nil }),
				workloadGroupSourceRecord,
			),
		},
	}

	records, err := reader.List(context.Background(), resources.ProductZIA, resourceWorkloadGroups)
	if err != nil {
		t.Fatalf("SDKReader.List(zia, workload-groups) error = %v, want nil", err)
	}
	got := projectOneRecord(t, resources.ProductZIA, resourceWorkloadGroups, records)
	assertNoCanaries(t, "workload-groups", got, canary, bareFreeTextToken, adminCanary, tagCanary)
	for _, field := range []string{"lastModifiedBy", "expressionJson"} {
		if _, ok := got[field]; ok {
			t.Errorf("projected workload-groups = %#v, want no %s", got, field)
		}
	}
	if got["lastModifiedTime"] != 1700000000 {
		t.Errorf("projected workload-groups lastModifiedTime = %v, want 1700000000", got["lastModifiedTime"])
	}
}

func TestReaderListZTWWorkloadGroupsProjectsSDKShapeThroughAllowList(t *testing.T) {
	t.Parallel()

	const (
		canary      = "ztw-workload-group-psk-canary"
		adminCanary = "ztw-workload-group-admin-canary"
		tagCanary   = "ztw-workload-group-tag-canary"
	)
	reader := &SDKReader{
		cfg: validReaderConfig(),
		handlers: map[resourceKey]resourceHandler{
			{product: resources.ProductZTW, name: resourceWorkloadGroups}: newListGetHandler(
				resourceWorkloadGroups,
				func(context.Context) ([]ztwworkloadgroups.WorkloadGroup, error) {
					return []ztwworkloadgroups.WorkloadGroup{
						{
							ID:               1901,
							Name:             "Cloud workload group",
							Description:      "temporary psk=" + canary,
							Expression:       "tags.value psk=" + canary,
							LastModifiedTime: 1700000100,
							LastModifiedBy: &ztwcommon.IDNameExtensions{
								ID:   9004,
								Name: adminCanary,
							},
							WorkloadTagExpression: ztwworkloadgroups.WorkloadTagExpression{
								ExpressionContainers: []ztwworkloadgroups.ExpressionContainer{
									{
										TagType:  "TAG",
										Operator: "AND",
										TagContainer: ztwworkloadgroups.TagContainer{
											Operator: "OR",
											Tags: []ztwworkloadgroups.Tags{
												{
													Key:   "owner",
													Value: tagCanary,
												},
											},
										},
									},
								},
							},
						},
					}, nil
				},
				intIDGetter(func(context.Context, int) (*ztwworkloadgroups.WorkloadGroup, error) { return nil, nil }),
				ztwWorkloadGroupSourceRecord,
			),
		},
	}

	records, err := reader.List(context.Background(), resources.ProductZTW, resourceWorkloadGroups)
	if err != nil {
		t.Fatalf("SDKReader.List(ztw, workload-groups) error = %v, want nil", err)
	}
	got := projectOneRecord(t, resources.ProductZTW, resourceWorkloadGroups, records)
	assertNoCanaries(t, "ztw workload-groups", got, canary, adminCanary, tagCanary)
	for _, field := range []string{"expression", "lastModifiedBy", "expressionJson"} {
		if _, ok := got[field]; ok {
			t.Errorf("projected ztw workload-groups = %#v, want no %s", got, field)
		}
	}
	if got["lastModifiedTime"] != 1700000100 {
		t.Errorf("projected ztw workload-groups lastModifiedTime = %v, want 1700000100", got["lastModifiedTime"])
	}
	description := toString(got["description"])
	if !strings.Contains(description, "<REDACTED:SECRET>") {
		t.Errorf("projected ztw workload-groups description = %v, want redacted secret marker", got["description"])
	}
}

func TestReaderZTWReferenceBatchProjectsSDKShapesThroughAllowList(t *testing.T) {
	t.Parallel()

	const canary = "ztw-reference-batch-psk-canary"
	cases := []struct {
		name         string
		record       resources.SourceRecord
		absentFields []string
	}{
		{
			name: resourcePublicCloudAccts,
			record: ztwPublicCloudAccountSourceRecord(ztwpubliccloudaccount.PublicCloudAccountDetails{
				ID:         1902,
				AccountID:  "account psk=" + canary,
				PlatformID: "AWS",
			}),
		},
		{
			name: resourceDNSGateways,
			record: ztwDNSGatewaySourceRecord(ztwdnsgateway.DNSGateway{
				ID:                           1903,
				Name:                         "DNS gateway",
				DNSGatewayType:               "PRIMARY",
				ECDnsGatewayOptionsPrimary:   "primary psk=" + canary,
				ECDnsGatewayOptionsSecondary: "secondary psk=" + canary,
				FailureBehavior:              "FAIL_CLOSED",
				PrimaryIP:                    "primary psk=" + canary,
				SecondaryIP:                  "secondary psk=" + canary,
				LastModifiedTime:             1700000200,
				LastModifiedBy: &ztwcommon.CommonIDNameExternalID{
					ID:   9005,
					Name: "admin psk=" + canary,
				},
			}),
			absentFields: []string{"ecDnsGatewayOptionsPrimary", "ecDnsGatewayOptionsSecondary", "lastModifiedBy"},
		},
		{
			name: resourceForwardingGWs,
			record: ztwForwardingGatewaySourceRecord(ztwziaforwardinggateway.ECGateway{
				ID:                           1904,
				Name:                         "Forwarding gateway",
				Description:                  "temporary psk=" + canary,
				FailClosed:                   true,
				ManualPrimary:                "manual psk=" + canary,
				ManualSecondary:              "secondary psk=" + canary,
				SubCloudPrimary:              &ztwcommon.CommonIDNameExternalID{ID: 10, Name: "subcloud", ExternalID: canary},
				SubCloudSecondary:            &ztwcommon.CommonIDNameExternalID{ID: 11, Name: "subcloud secondary", ExternalID: canary},
				PrimaryType:                  "MANUAL_OVERRIDE",
				SecondaryType:                "AUTO",
				Type:                         "ZIA",
				FailureBehavior:              "FAIL_CLOSED",
				DNSGatewayType:               "PRIMARY",
				PrimaryIP:                    "primary psk=" + canary,
				SecondaryIP:                  "secondary psk=" + canary,
				ECDNSGatewayOptionsPrimary:   "primary option psk=" + canary,
				ECDNSGatewayOptionsSecondary: "secondary option psk=" + canary,
				LastModifiedBy:               &ztwcommon.IDNameExtensions{ID: 9006, Name: "admin psk=" + canary},
				LastModifiedTime:             1700000300,
			}),
			absentFields: []string{"ecDnsGatewayOptionsPrimary", "ecDnsGatewayOptionsSecondary", "lastModifiedBy"},
		},
		{
			name: resourceECGroups,
			record: ztwECGroupSourceRecord(ztwecgroup.EcGroup{
				ID:                    1905,
				Name:                  "EC group",
				Description:           "temporary psk=" + canary,
				DeployType:            "DEDICATED",
				Status:                []string{"ACTIVE"},
				Platform:              "AWS",
				AWSAvailabilityZone:   "az psk=" + canary,
				AzureAvailabilityZone: "azure psk=" + canary,
				MaxEcCount:            4,
				TunnelMode:            "GRE",
				Location:              &ztwcommon.CommonIDNameExternalID{ID: 20, Name: "location", ExternalID: canary},
				ProvTemplate:          &ztwcommon.CommonIDNameExternalID{ID: 21, Name: "template", ExternalID: canary},
				ECVMs:                 []ztwcommon.ECVMs{{ID: 30, Name: "ecvm psk=" + canary}},
			}),
			absentFields: []string{"ecVMs"},
		},
		{
			name: resourceIPSourceGroups,
			record: ztwIPSourceGroupSourceRecord(ztwipsourcegroups.IPSourceGroups{
				ID:             1906,
				Name:           "IP source group",
				Description:    "temporary psk=" + canary,
				IPAddresses:    []string{"source psk=" + canary},
				CreatorContext: "EC",
				IsNonEditable:  false,
			}),
		},
		{
			name: resourceIPDestGroups,
			record: ztwIPDestinationGroupSourceRecord(ztwipdestinationgroups.IPDestinationGroups{
				ID:            1907,
				Name:          "IP destination group",
				Description:   "temporary psk=" + canary,
				Type:          "DSTN_IP",
				Addresses:     []string{"dest psk=" + canary},
				IPCategories:  []string{"category psk=" + canary},
				Countries:     []string{"US"},
				IsNonEditable: false,
			}),
		},
		{
			name: resourceIPGroups,
			record: ztwIPGroupSourceRecord(ztwipgroups.IPGroups{
				ID:             1908,
				Name:           "IP group",
				Description:    "temporary psk=" + canary,
				IPAddresses:    []string{"ip psk=" + canary},
				CreatorContext: "EC",
				IsNonEditable:  false,
				ExtranetIPPool: true,
				IsPredefined:   false,
			}),
		},
		{
			name: resourceNetworkServices,
			record: ztwNetworkServiceSourceRecord(ztwnetworkservices.NetworkServices{
				ID:             1909,
				Name:           "Network service",
				Description:    "temporary psk=" + canary,
				Tag:            "tag psk=" + canary,
				SrcTCPPorts:    []ztwnetworkservices.NetworkPorts{{Start: 1000, End: 1001}},
				DestTCPPorts:   []ztwnetworkservices.NetworkPorts{{Start: 443, End: 443}},
				SrcUDPPorts:    []ztwnetworkservices.NetworkPorts{{Start: 2000, End: 2001}},
				DestUDPPorts:   []ztwnetworkservices.NetworkPorts{{Start: 53, End: 53}},
				Type:           "CUSTOM",
				IsNameL10nTag:  false,
				CreatorContext: "EC",
			}),
		},
		{
			name: resourceNetworkSvcGroups,
			record: ztwNetworkServiceGroupSourceRecord(ztwnetworkservicegroups.NetworkServiceGroups{
				ID:          1910,
				Name:        "Network service group",
				Description: "temporary psk=" + canary,
				Services: []ztwnetworkservicegroups.Services{
					{
						ID:          100,
						Name:        "Referenced service",
						Tag:         "service tag psk=" + canary,
						Description: "service description psk=" + canary,
					},
				},
				CreatorContext: "EC",
			}),
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := projectOneRecord(t, resources.ProductZTW, tc.name, []resources.SourceRecord{tc.record})
			assertNoCanaries(t, "ztw "+tc.name, got, canary)
			for _, field := range tc.absentFields {
				if _, ok := got[field]; ok {
					t.Errorf("projected ztw %s = %#v, want no %s", tc.name, got, field)
				}
			}
		})
	}
}

func TestReaderZTWAdminGovernanceProjectsSDKShapesThroughAllowList(t *testing.T) {
	t.Parallel()

	const canary = "ztw-admin-governance-psk-canary"
	adminUserRecord := ztwAdminUserSourceRecord(ztwadminusers.AdminUsers{
		ID:                          1911,
		LoginName:                   "admin@example.internal",
		UserName:                    "cloud-admin",
		Email:                       "admin@example.internal",
		Comments:                    "temporary psk=" + canary,
		Disabled:                    false,
		Password:                    "password psk=" + canary,
		PasswordLastModifiedTime:    1700000400,
		IsNonEditable:               false,
		IsPasswordLoginAllowed:      true,
		IsPasswordExpired:           false,
		IsAuditor:                   true,
		IsSecurityReportCommEnabled: true,
		IsServiceUpdateCommEnabled:  false,
		IsProductUpdateCommEnabled:  true,
		IsExecMobileAppEnabled:      true,
		AdminScopeGroupMemberEntities: []ztwcommon.IDNameExtensions{{
			ID:   10,
			Name: "scope member psk=" + canary,
		}},
		AdminScopeEntities: []ztwcommon.IDNameExtensions{{
			ID:   11,
			Name: "scope entity psk=" + canary,
		}},
		AdminScopeType: "LOCATION_GROUP",
		Role: &ztwadminusers.Role{
			ID:           20,
			Name:         "Super Admin",
			IsNameL10Tag: false,
			Extensions:   map[string]any{"secret": "extension psk=" + canary},
		},
		ExecMobileAppTokens: []ztwadminusers.ExecMobileAppTokens{{
			Cloud:      "zscloud",
			OrgId:      101,
			Name:       "phone",
			TokenId:    "token-id psk=" + canary,
			Token:      "token psk=" + canary,
			DeviceId:   "device psk=" + canary,
			DeviceName: "device-name psk=" + canary,
		}},
	})
	adminRoleRecord := ztwAdminRoleSourceRecord(ztwadminroles.AdminRoles{
		ID:                 1912,
		Rank:               2,
		Name:               "Cloud Security Admin",
		PolicyAccess:       "READ_WRITE",
		AlertingAccess:     "READ_ONLY",
		DashboardAccess:    "READ_ONLY",
		ReportAccess:       "READ_ONLY",
		AnalysisAccess:     "READ_ONLY",
		UsernameAccess:     "FULL",
		AdminAcctAccess:    "READ_ONLY",
		DeviceInfoAccess:   "READ_ONLY",
		IsAuditor:          false,
		Permissions:        []string{"POLICY_READ", "ADMIN_USERS psk=" + canary},
		IsNonEditable:      false,
		LogsLimit:          "LAST_30_DAYS",
		RoleType:           "ADMIN",
		FeaturePermissions: map[string]any{"feature": "permission psk=" + canary},
	})

	userStandard, userReports, err := resources.ProjectRecordsAndVerify(
		mustFindSpec(t, resources.ProductZTW, resourceAdminUsers),
		redact.ModeStandard,
		[]resources.SourceRecord{adminUserRecord},
	)
	if err != nil {
		t.Fatalf("ProjectRecordsAndVerify(ztw/admin-users standard) error = %v, want nil", err)
	}
	userGot := userStandard.Records()[0].Fields()
	if userGot["loginName"] != "admin@example.internal" || userGot["userName"] != "cloud-admin" || userGot["email"] != "admin@example.internal" {
		t.Errorf("ProjectRecordsAndVerify(ztw/admin-users standard) identity fields = %#v, want visible", userGot)
	}
	if userGot["disabled"] != false || userGot["isAuditor"] != true || userGot["adminScopeType"] != "LOCATION_GROUP" {
		t.Errorf("ProjectRecordsAndVerify(ztw/admin-users standard) governance fields = %#v, want visible", userGot)
	}
	if role, ok := userGot["role"].(map[string]any); !ok || role["id"] != 20 || role["name"] != "Super Admin" {
		t.Errorf("ProjectRecordsAndVerify(ztw/admin-users standard) role = %#v, want id/name reference", userGot["role"])
	}
	for _, field := range []string{"password", "execMobileAppTokens", "role.extensions"} {
		assertReportContains(t, userReports[0].DroppedFields, field)
	}
	for _, field := range []string{"isPasswordLoginAllowed", "isPasswordExpired"} {
		if _, ok := userGot[field]; ok {
			t.Errorf("ProjectRecordsAndVerify(ztw/admin-users standard) includes %s, want dropped", field)
		}
		assertReportContains(t, userReports[0].DroppedFields, field)
	}
	assertNoCanaries(t, "ztw/admin-users standard", userGot, canary)

	userShare, userShareReports, err := resources.ProjectRecordsAndVerify(
		mustFindSpec(t, resources.ProductZTW, resourceAdminUsers),
		redact.ModeShare,
		[]resources.SourceRecord{adminUserRecord},
	)
	if err != nil {
		t.Fatalf("ProjectRecordsAndVerify(ztw/admin-users share) error = %v, want nil", err)
	}
	userShareGot := userShare.Records()[0].Fields()
	for _, field := range []string{
		"loginName",
		"userName",
		"email",
		"comments",
		"pwdLastModifiedTime",
		"adminScopescopeGroupMemberEntities",
		"adminScopeScopeEntities",
		"role",
	} {
		if _, ok := userShareGot[field]; ok {
			t.Errorf("ProjectRecordsAndVerify(ztw/admin-users share) includes %s, want dropped", field)
		}
		assertReportContains(t, userShareReports[0].DroppedFields, field)
	}
	assertNoCanaries(t, "ztw/admin-users share", userShareGot, canary, "admin@example.internal", "cloud-admin")

	roleStandard, roleReports, err := resources.ProjectRecordsAndVerify(
		mustFindSpec(t, resources.ProductZTW, resourceAdminRoles),
		redact.ModeStandard,
		[]resources.SourceRecord{adminRoleRecord},
	)
	if err != nil {
		t.Fatalf("ProjectRecordsAndVerify(ztw/admin-roles standard) error = %v, want nil", err)
	}
	roleGot := roleStandard.Records()[0].Fields()
	if roleGot["name"] != "Cloud Security Admin" || roleGot["policyAccess"] != "READ_WRITE" || roleGot["rank"] != 2 {
		t.Errorf("ProjectRecordsAndVerify(ztw/admin-roles standard) = %#v, want role governance fields visible", roleGot)
	}
	assertReportContains(t, roleReports[0].DroppedFields, "featurePermissions")
	assertNoCanaries(t, "ztw/admin-roles standard", roleGot, canary)

	roleShare, roleShareReports, err := resources.ProjectRecordsAndVerify(
		mustFindSpec(t, resources.ProductZTW, resourceAdminRoles),
		redact.ModeShare,
		[]resources.SourceRecord{adminRoleRecord},
	)
	if err != nil {
		t.Fatalf("ProjectRecordsAndVerify(ztw/admin-roles share) error = %v, want nil", err)
	}
	roleShareGot := roleShare.Records()[0].Fields()
	for _, field := range []string{
		"rank",
		"policyAccess",
		"alertingAccess",
		"dashboardAccess",
		"reportAccess",
		"analysisAccess",
		"usernameAccess",
		"adminAcctAccess",
		"deviceInfoAccess",
		"permissions",
		"logsLimit",
		"featurePermissions",
	} {
		if _, ok := roleShareGot[field]; ok {
			t.Errorf("ProjectRecordsAndVerify(ztw/admin-roles share) includes %s, want dropped", field)
		}
		assertReportContains(t, roleShareReports[0].DroppedFields, field)
	}
	if roleShareGot["name"] != "Cloud Security Admin" || roleShareGot["roleType"] != "ADMIN" {
		t.Errorf("ProjectRecordsAndVerify(ztw/admin-roles share) = %#v, want share-safe role identity", roleShareGot)
	}
	assertNoCanaries(t, "ztw/admin-roles share", roleShareGot, canary)
}

func TestReaderZTWCloseoutBatchProjectsSDKShapesThroughAllowList(t *testing.T) {
	t.Parallel()

	const canary = "ztw-closeout-batch-psk-canary"
	cases := []struct {
		name         string
		record       resources.SourceRecord
		absentFields []string
	}{
		{
			name: resourceLocations,
			record: ztwLocationSourceRecord(ztwlocation.Locations{
				ID:          2001,
				Name:        "Cloud Connector location",
				ParentID:    99,
				Country:     "US",
				State:       "CA",
				IPAddresses: []string{"198.51.100.10 psk=" + canary},
				Ports:       []int{80, 443},
				Description: "temporary psk=" + canary,
				VPNCredentials: []ztwlocation.VPNCredentials{{
					ID:           1,
					IPAddress:    "198.51.100.20",
					PreSharedKey: "psk=" + canary,
				}},
				PublicCloudAccountID: &ztwcommon.CommonIDName{ID: 2, Name: "account psk=" + canary},
				VPCInfo: ztwlocation.VPCInfo{
					CloudProvider: "AWS",
					CloudMeta:     ztwlocation.CloudMeta{ID: 3, Name: "vpc psk=" + canary},
				},
			}),
			absentFields: []string{"vpnCredentials", "vpcInfo"},
		},
		{
			name: resourceLocationTmpls,
			record: ztwLocationTemplateSourceRecord(ztwlocationtemplate.LocationTemplate{
				ID:          2002,
				Name:        "Template",
				Description: "temporary psk=" + canary,
				LocationTemplateDetails: &ztwlocationtemplate.LocationTemplateDetails{
					TemplatePrefix: "prefix psk=" + canary,
					AuthRequired:   true,
					LastModUid:     &ztwcommon.CommonIDNameExternalID{ID: 4, Name: "admin psk=" + canary, ExternalID: canary},
				},
				Editable:    true,
				LastModTime: 1700000500,
				LastModUid:  &ztwcommon.CommonIDNameExternalID{ID: 5, Name: "admin psk=" + canary, ExternalID: canary},
			}),
			absentFields: []string{"lastModUid"},
		},
		{
			name: resourceAccountGroups,
			record: ztwAccountGroupSourceRecord(ztwaccountgroups.AccountGroups{
				ID:          2003,
				Name:        "Account group",
				Description: "temporary psk=" + canary,
				CloudType:   "AWS",
				PublicCloudAccounts: []ztwcommon.IDNameExtensions{{
					ID:   6,
					Name: "account psk=" + canary,
				}},
				CloudConnectorGroups: []ztwcommon.IDNameExtensions{{
					ID:   7,
					Name: "connector group psk=" + canary,
				}},
			}),
		},
		{
			name: resourcePublicCloudInfo,
			record: ztwPublicCloudInfoSourceRecord(ztwpubliccloudinfo.PublicCloudInfo{
				ID:           2004,
				Name:         "cloud account psk=" + canary,
				CloudType:    "AWS",
				ExternalID:   "external psk=" + canary,
				LastModTime:  1700000600,
				LastSyncTime: 1700000700,
				AccountGroups: []ztwcommon.IDNameExtensions{{
					ID:   8,
					Name: "account group psk=" + canary,
				}},
				LastModUser:      &ztwcommon.CommonIDNameExternalID{ID: 9, Name: "admin psk=" + canary, ExternalID: canary},
				RegionStatus:     []ztwcommon.RegionStatus{{ID: 10, Name: "us-west-2", CloudType: "AWS", Status: true}},
				SupportedRegions: []ztwcommon.SupportedRegions{{ID: 11, Name: "us-east-1", CloudType: "AWS"}},
				AccountDetails: &ztwpubliccloudinfo.AccountDetails{
					AwsAccountID:       "123456789012",
					AwsRoleName:        "role psk=" + canary,
					CloudWatchGroupArn: "arn:aws:logs:::psk=" + canary,
					ExternalID:         "external psk=" + canary,
					TrustedAccountID:   "trusted psk=" + canary,
				},
			}),
			absentFields: []string{"externalId", "lastModUser", "accountDetails"},
		},
		{
			name: resourceZTWZPAAppSegs,
			record: ztwZPAApplicationSegmentSourceRecord(ztwzparesources.ZPAApplicationSegment{
				ID:          2005,
				Name:        "Application segment",
				Description: "temporary psk=" + canary,
				ZpaID:       12,
				Deleted:     false,
			}),
		},
		{
			name: resourceForwardingRules,
			record: ztwForwardingRuleSourceRecord(ztwforwardingrules.ForwardingRules{
				ID:               2006,
				Name:             "Forwarding rule",
				Description:      "temporary psk=" + canary,
				Type:             "EC_RDR",
				Order:            1,
				Rank:             7,
				ForwardMethod:    "ECZPA",
				State:            "ENABLED",
				LastModifiedTime: 1700000800,
				SrcIps:           []string{"192.0.2.10 psk=" + canary},
				DestAddresses:    []string{"example.internal psk=" + canary},
				DestIpCategories: []string{"category psk=" + canary},
				ResCategories:    []string{"resource psk=" + canary},
				Locations:        []ztwcommon.IDNameExtensions{{ID: 13, Name: "location psk=" + canary}},
				LastModifiedBy:   &ztwcommon.IDNameExtensions{ID: 14, Name: "admin psk=" + canary},
				ProxyGateway:     &ztwcommon.CommonIDName{ID: 15, Name: "proxy psk=" + canary},
				ZPAApplicationSegments: []ztwcommon.ZPAApplicationSegments{{
					ID:          16,
					Name:        "segment psk=" + canary,
					Description: "segment description psk=" + canary,
					ZPAID:       17,
				}},
			}),
			absentFields: []string{"lastModifiedBy"},
		},
		{
			name: resourceTrafficDNSRules,
			record: ztwTrafficDNSRuleSourceRecord(ztwtrafficdnsrules.ECDNSRules{
				ID:               2007,
				Name:             "DNS rule",
				Description:      "temporary psk=" + canary,
				Type:             "EC_DNS",
				Action:           "ALLOW",
				Order:            2,
				Rank:             8,
				State:            "ENABLED",
				SrcIps:           []string{"192.0.2.11 psk=" + canary},
				DestAddresses:    []string{"dns.example.internal psk=" + canary},
				Locations:        []ztwcommon.IDNameExtensions{{ID: 18, Name: "location psk=" + canary}},
				DNSGateway:       &ztwcommon.CommonIDName{ID: 19, Name: "dns gateway psk=" + canary},
				LastModifiedTime: 1700000900,
				LastModifiedBy:   &ztwcommon.IDNameExtensions{ID: 20, Name: "admin psk=" + canary},
			}),
			absentFields: []string{"lastModifiedBy"},
		},
		{
			name: resourceTrafficLogRules,
			record: ztwTrafficLogRuleSourceRecord(ztwtrafficlogrules.ECTrafficLogRules{
				ID:               2008,
				Name:             "Traffic log rule",
				Description:      "temporary psk=" + canary,
				Order:            3,
				Rank:             9,
				State:            "ENABLED",
				Type:             "EC_SELF",
				ForwardMethod:    "ECSELF",
				Locations:        []ztwcommon.IDNameExtensions{{ID: 21, Name: "location psk=" + canary}},
				ProxyGateway:     &ztwcommon.CommonIDName{ID: 22, Name: "proxy psk=" + canary},
				LastModifiedTime: 1700001000,
				LastModifiedBy:   &ztwcommon.IDNameExtensions{ID: 23, Name: "admin psk=" + canary},
			}),
			absentFields: []string{"lastModifiedBy"},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := projectOneRecord(t, resources.ProductZTW, tc.name, []resources.SourceRecord{tc.record})
			assertNoCanaries(t, "ztw "+tc.name, got, canary)
			for _, field := range tc.absentFields {
				if _, ok := got[field]; ok {
					t.Errorf("projected ztw %s = %#v, want no %s", tc.name, got, field)
				}
			}
		})
	}
}

func TestReaderZTWCloseoutBatchShareModeDropsSensitiveCriteria(t *testing.T) {
	t.Parallel()

	const canary = "ztw-closeout-share-psk-canary"
	cases := []struct {
		name         string
		record       resources.SourceRecord
		absentFields []string
	}{
		{
			name: resourceLocations,
			record: ztwLocationSourceRecord(ztwlocation.Locations{
				ID:          2101,
				Name:        "Cloud Connector location",
				IPAddresses: []string{"198.51.100.10 psk=" + canary},
				Description: "temporary psk=" + canary,
			}),
			absentFields: []string{"ipAddresses", "description", "authRequired"},
		},
		{
			name: resourcePublicCloudInfo,
			record: ztwPublicCloudInfoSourceRecord(ztwpubliccloudinfo.PublicCloudInfo{
				ID:        2102,
				Name:      "cloud account psk=" + canary,
				CloudType: "AWS",
			}),
			absentFields: []string{"name"},
		},
		{
			name: resourceForwardingRules,
			record: ztwForwardingRuleSourceRecord(ztwforwardingrules.ForwardingRules{
				ID:            2103,
				Name:          "Forwarding rule",
				SrcIps:        []string{"192.0.2.10 psk=" + canary},
				DestAddresses: []string{"example.internal psk=" + canary},
				Locations:     []ztwcommon.IDNameExtensions{{ID: 13, Name: "location psk=" + canary}},
			}),
			absentFields: []string{"srcIps", "destAddresses", "locations"},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			spec := mustFindSpec(t, resources.ProductZTW, tc.name)
			projected, _, err := resources.ProjectRecordsAndVerify(spec, redact.ModeShare, []resources.SourceRecord{tc.record})
			if err != nil {
				t.Fatalf("ProjectRecordsAndVerify(ztw/%s share) error = %v, want nil", tc.name, err)
			}
			got := projected.Records()[0].Fields()
			assertNoCanaries(t, "ztw "+tc.name+" share", got, canary)
			for _, field := range tc.absentFields {
				if _, ok := got[field]; ok {
					t.Errorf("ProjectRecordsAndVerify(ztw/%s share) includes %s, want dropped", tc.name, field)
				}
			}
		})
	}
}

func TestReaderListAlertSubscriptionsProjectsSDKShapeThroughAllowList(t *testing.T) {
	t.Parallel()

	const (
		canary            = "alert-subscription-psk-canary"
		bareFreeTextToken = "A7b9C2d4E6f8G1h3J5k7L9m2N4p6Q8r0S2t4U6v"
	)
	reader := &SDKReader{
		cfg: validReaderConfig(),
		handlers: map[resourceKey]resourceHandler{
			{product: resources.ProductZIA, name: resourceAlertSubs}: newListGetHandler(
				resourceAlertSubs,
				func(context.Context) ([]alerts.AlertSubscriptions, error) {
					return []alerts.AlertSubscriptions{
						{
							ID:               901,
							Description:      "temporary psk=" + canary + " " + bareFreeTextToken,
							Email:            "ops@example.invalid",
							Deleted:          false,
							Pt0Severities:    []string{"CRITICAL"},
							SecureSeverities: []string{"MAJOR"},
							ManageSeverities: []string{"MINOR"},
							ComplySeverities: []string{"INFO"},
							SystemSeverities: []string{"SYSTEM"},
						},
					}, nil
				},
				intIDGetter(func(context.Context, int) (*alerts.AlertSubscriptions, error) { return nil, nil }),
				alertSubscriptionSourceRecord,
			),
		},
	}

	records, err := reader.List(context.Background(), resources.ProductZIA, resourceAlertSubs)
	if err != nil {
		t.Fatalf("SDKReader.List(zia, alert-subscriptions) error = %v, want nil", err)
	}
	got := projectOneRecord(t, resources.ProductZIA, resourceAlertSubs, records)
	assertNoCanaries(t, "alert-subscriptions", got, canary, bareFreeTextToken)
	if got["deleted"] != false {
		t.Errorf("projected alert-subscriptions deleted = %v, want false", got["deleted"])
	}
}

func TestReaderListCloudAppInstancesProjectsSDKShapeThroughAllowList(t *testing.T) {
	t.Parallel()

	const (
		adminCanary      = "cloud-app-instance-admin-canary"
		identifierCanary = "cloud-app-instance-identifier-canary"
	)
	reader := &SDKReader{
		cfg: validReaderConfig(),
		handlers: map[resourceKey]resourceHandler{
			{product: resources.ProductZIA, name: resourceCloudAppInsts}: newListGetHandler(
				resourceCloudAppInsts,
				func(context.Context) ([]cloudappinstances.CloudApplicationInstances, error) {
					return []cloudappinstances.CloudApplicationInstances{
						{
							InstanceID:   902,
							InstanceType: "SAAS",
							InstanceName: "Finance cloud app",
							ModifiedAt:   1700000001,
							ModifiedBy: &ziacommon.IDNameExtensions{
								ID:   77,
								Name: adminCanary,
							},
							InstanceIdentifiers: []cloudappinstances.InstanceIdentifiers{
								{
									InstanceID:             902,
									InstanceIdentifier:     identifierCanary,
									InstanceIdentifierName: "Tenant ref",
									IdentifierType:         "TENANT",
									ModifiedAt:             1700000002,
									ModifiedBy: &ziacommon.IDNameExtensions{
										ID:   78,
										Name: adminCanary,
									},
								},
							},
						},
					}, nil
				},
				intIDGetter(func(context.Context, int) (*cloudappinstances.CloudApplicationInstances, error) { return nil, nil }),
				cloudAppInstanceSourceRecord,
			),
		},
	}

	records, err := reader.List(context.Background(), resources.ProductZIA, resourceCloudAppInsts)
	if err != nil {
		t.Fatalf("SDKReader.List(zia, cloud-app-instances) error = %v, want nil", err)
	}
	got := projectOneRecord(t, resources.ProductZIA, resourceCloudAppInsts, records)
	assertNoCanaries(t, "cloud-app-instances", got, adminCanary, identifierCanary)
	for _, field := range []string{"modifiedBy", "instanceIdentifiers"} {
		if _, ok := got[field]; ok {
			t.Errorf("projected cloud-app-instances = %#v, want no %s", got, field)
		}
	}
	if got["instanceId"] != 902 {
		t.Errorf("projected cloud-app-instances instanceId = %v, want 902", got["instanceId"])
	}
}

func TestReaderListTenancyRestrictionProfilesProjectsSDKShapeThroughAllowList(t *testing.T) {
	t.Parallel()

	const (
		canary            = "tenancy-restriction-psk-canary"
		bareFreeTextToken = "A7b9C2d4E6f8G1h3J5k7L9m2N4p6Q8r0S2t4U6v"
	)
	reader := &SDKReader{
		cfg: validReaderConfig(),
		handlers: map[resourceKey]resourceHandler{
			{product: resources.ProductZIA, name: resourceTenancyProfiles}: newListGetHandler(
				resourceTenancyProfiles,
				func(context.Context) ([]tenancyrestriction.TenancyRestrictionProfile, error) {
					return []tenancyrestriction.TenancyRestrictionProfile{
						{
							ID:                          904,
							Name:                        "Tenant restriction psk=" + canary,
							AppType:                     "O365",
							Description:                 "temporary psk=" + canary + " " + bareFreeTextToken,
							ItemTypePrimary:             "DOMAIN",
							ItemTypeSecondary:           "TENANT",
							RestrictPersonalO365Domains: true,
							AllowGoogleConsumers:        false,
							MsLoginServicesTrV2:         true,
							AllowGoogleVisitors:         false,
							AllowGcpCloudStorageRead:    true,
							ItemDataPrimary:             []string{"primary psk=" + canary},
							ItemDataSecondary:           []string{"secondary psk=" + canary},
							ItemValue:                   []string{"value psk=" + canary},
							LastModifiedTime:            1700000003,
							LastModifiedUserID:          8101,
						},
					}, nil
				},
				intIDGetter(func(context.Context, int) (*tenancyrestriction.TenancyRestrictionProfile, error) { return nil, nil }),
				tenancyRestrictionProfileSourceRecord,
			),
		},
	}

	records, err := reader.List(context.Background(), resources.ProductZIA, resourceTenancyProfiles)
	if err != nil {
		t.Fatalf("SDKReader.List(zia, tenancy-restriction-profiles) error = %v, want nil", err)
	}
	got := projectOneRecord(t, resources.ProductZIA, resourceTenancyProfiles, records)
	assertNoCanaries(t, "tenancy-restriction-profiles", got, canary, bareFreeTextToken)
	if got["restrictPersonalO365Domains"] != true {
		t.Errorf("projected tenancy-restriction-profiles restrictPersonalO365Domains = %v, want true", got["restrictPersonalO365Domains"])
	}
}

func TestReaderListVZENClustersProjectsSDKShapeThroughAllowList(t *testing.T) {
	t.Parallel()

	const nodeCanary = "vzen-cluster-node-canary"
	reader := &SDKReader{
		cfg: validReaderConfig(),
		handlers: map[resourceKey]resourceHandler{
			{product: resources.ProductZIA, name: resourceVZENClusters}: newListGetHandler(
				resourceVZENClusters,
				func(context.Context) ([]vzenclusters.VZENClusters, error) {
					return []vzenclusters.VZENClusters{
						{
							ID:             905,
							Name:           "VZEN cluster",
							Status:         "ENABLED",
							IpAddress:      "198.51.100.10",
							SubnetMask:     "255.255.255.0",
							DefaultGateway: "198.51.100.1",
							Type:           "CLUSTER",
							IpSecEnabled:   true,
							VirtualZenNodes: []ziacommon.IDNameExternalID{
								{
									ID:         1,
									Name:       "Node psk=" + nodeCanary,
									ExternalID: nodeCanary,
								},
							},
						},
					}, nil
				},
				intIDGetter(func(context.Context, int) (*vzenclusters.VZENClusters, error) { return nil, nil }),
				vzenClusterSourceRecord,
			),
		},
	}

	records, err := reader.List(context.Background(), resources.ProductZIA, resourceVZENClusters)
	if err != nil {
		t.Fatalf("SDKReader.List(zia, vzen-clusters) error = %v, want nil", err)
	}
	got := projectOneRecord(t, resources.ProductZIA, resourceVZENClusters, records)
	assertNoCanaries(t, "vzen-clusters", got, nodeCanary)
	if got["ipSecEnabled"] != true {
		t.Errorf("projected vzen-clusters ipSecEnabled = %v, want true", got["ipSecEnabled"])
	}
}

func TestReaderListVZENNodesProjectsSDKShapeThroughAllowList(t *testing.T) {
	t.Parallel()

	reader := &SDKReader{
		cfg: validReaderConfig(),
		handlers: map[resourceKey]resourceHandler{
			{product: resources.ProductZIA, name: resourceVZENNodes}: newListGetHandler(
				resourceVZENNodes,
				func(context.Context) ([]vzennodes.VZENNodes, error) {
					return []vzennodes.VZENNodes{
						{
							ID:                            906,
							ZGatewayID:                    11,
							Name:                          "VZEN node",
							Status:                        "ENABLED",
							InProduction:                  true,
							IPAddress:                     "198.51.100.20",
							SubnetMask:                    "255.255.255.0",
							DefaultGateway:                "198.51.100.1",
							Type:                          "SMALL",
							IPSecEnabled:                  true,
							OnDemandSupportTunnelEnabled:  false,
							EstablishSupportTunnelEnabled: false,
							LoadBalancerIPAddress:         "198.51.100.30",
							DeploymentMode:                "STANDALONE",
							ClusterName:                   "VZEN cluster",
							VzenSkuType:                   "SMALL",
						},
					}, nil
				},
				intIDGetter(func(context.Context, int) (*vzennodes.VZENNodes, error) { return nil, nil }),
				vzenNodeSourceRecord,
			),
		},
	}

	records, err := reader.List(context.Background(), resources.ProductZIA, resourceVZENNodes)
	if err != nil {
		t.Fatalf("SDKReader.List(zia, vzen-nodes) error = %v, want nil", err)
	}
	got := projectOneRecord(t, resources.ProductZIA, resourceVZENNodes, records)
	if got["zgatewayId"] != 11 {
		t.Errorf("projected vzen-nodes zgatewayId = %v, want 11", got["zgatewayId"])
	}
}

func TestReaderListDLPICAPServersProjectsSDKShapeThroughAllowList(t *testing.T) {
	t.Parallel()

	const canary = "dlp-icap-psk-canary"
	reader := &SDKReader{
		cfg: validReaderConfig(),
		handlers: map[resourceKey]resourceHandler{
			{product: resources.ProductZIA, name: resourceDLPICAPServers}: newListGetHandler(
				resourceDLPICAPServers,
				func(context.Context) ([]dlpicapservers.DLPICAPServers, error) {
					return []dlpicapservers.DLPICAPServers{
						{
							ID:     1003,
							Name:   "ICAP server psk=" + canary,
							URL:    "icap://icap.example.invalid/scan",
							Status: "ENABLED",
						},
					}, nil
				},
				intIDGetter(func(context.Context, int) (*dlpicapservers.DLPICAPServers, error) { return nil, nil }),
				dlpICAPServerSourceRecord,
			),
		},
	}

	records, err := reader.List(context.Background(), resources.ProductZIA, resourceDLPICAPServers)
	if err != nil {
		t.Fatalf("SDKReader.List(zia, dlp-icap-servers) error = %v, want nil", err)
	}
	got := projectOneRecord(t, resources.ProductZIA, resourceDLPICAPServers, records)
	assertNoCanaries(t, "dlp-icap-servers", got, canary)
	if got["status"] != "ENABLED" {
		t.Errorf("projected dlp-icap-servers status = %v, want ENABLED", got["status"])
	}
}

func TestDeferredZIABatchProjectionBoundaries(t *testing.T) {
	t.Parallel()

	const canary = "synthetic-sensitive-canary"
	tests := []struct {
		name            string
		resource        string
		record          resources.SourceRecord
		standardPresent []string
		standardAbsent  []string
		shareAbsent     []string
	}{
		{
			name:     "dlp engine expression is standard-only",
			resource: resourceDLPEngines,
			record: resources.NewSourceRecord(map[string]any{
				"id":                   1,
				"name":                 "Engine",
				"engineExpression":     "dict(123)",
				"customDlpEngine":      true,
				"predefinedEngineName": "Predefined",
			}),
			standardPresent: []string{"engineExpression"},
			shareAbsent:     []string{"engineExpression"},
		},
		{
			name:     "dlp dictionary detector content is always dropped",
			resource: resourceDLPDictionaries,
			record: resources.NewSourceRecord(map[string]any{
				"id":                    2,
				"name":                  "Dictionary",
				"phrases":               []map[string]any{{"phrase": canary}},
				"patterns":              []map[string]any{{"pattern": canary}},
				"exactDataMatchDetails": []map[string]any{{"secondaryFieldMatchOn": canary}},
			}),
			standardPresent: []string{"name"},
			standardAbsent:  []string{"phrases", "patterns", "exactDataMatchDetails"},
			shareAbsent:     []string{"phrases", "patterns", "exactDataMatchDetails"},
		},
		{
			name:     "edm schema data model is standard-only or dropped",
			resource: resourceDLPEDMSchemas,
			record: resources.NewSourceRecord(map[string]any{
				"schemaId":         3,
				"projectName":      "EDM Project",
				"filename":         "edm.csv",
				"tokenList":        []map[string]any{{"name": canary}},
				"modifiedBy":       map[string]any{"id": 1, "name": canary},
				"schedule":         map[string]any{"scheduleType": "DAILY"},
				"fileUploadStatus": "READY",
			}),
			standardPresent: []string{"projectName", "filename"},
			standardAbsent:  []string{"tokenList", "modifiedBy", "schedule"},
			shareAbsent:     []string{"projectName", "filename", "tokenList", "modifiedBy", "schedule"},
		},
		{
			name:     "idm profile host and username are standard-only",
			resource: resourceDLPIDMProfiles,
			record: resources.NewSourceRecord(map[string]any{
				"profileId":      4,
				"profileName":    "Profile",
				"host":           "idm.example.invalid",
				"profileDirPath": "/srv/idm",
				"userName":       "svc-idm",
				"modifiedBy":     map[string]any{"id": 1, "name": canary},
				"uploadStatus":   "COMPLETE",
			}),
			standardPresent: []string{"profileName", "host", "profileDirPath", "userName"},
			standardAbsent:  []string{"modifiedBy"},
			shareAbsent:     []string{"profileName", "host", "profileDirPath", "userName", "modifiedBy"},
		},
		{
			name:     "web dlp rule criteria is standard-only and recursive subrules drop",
			resource: resourceDLPWebRules,
			record: resources.NewSourceRecord(map[string]any{
				"id":                   5,
				"name":                 "Web DLP",
				"externalAuditorEmail": "auditor@example.invalid",
				"receiver":             map[string]any{"id": 1, "name": "Receiver", "tenant": map[string]any{"name": canary}},
				"users":                []map[string]any{{"id": 1, "name": "User"}},
				"subRules":             []map[string]any{{"name": canary}},
				"state":                "ENABLED",
			}),
			standardPresent: []string{"externalAuditorEmail", "receiver", "users"},
			standardAbsent:  []string{"subRules"},
			shareAbsent:     []string{"externalAuditorEmail", "receiver", "users", "subRules"},
		},
		{
			name:     "web dlp rule receiver without tenant does not panic",
			resource: resourceDLPWebRules,
			record: dlpWebRuleSourceRecord(dlpwebrules.WebDLPRules{
				ID:       5,
				Name:     "Web DLP",
				Receiver: &dlpwebrules.Receiver{ID: 1, Name: "Receiver", Type: "INCIDENT_RECEIVER"},
			}),
			standardPresent: []string{"receiver"},
			shareAbsent:     []string{"receiver"},
		},
		{
			name:     "c2c receiver auth details are dropped",
			resource: resourceC2CIncidentRcvs,
			record: resources.NewSourceRecord(map[string]any{
				"id":                6,
				"name":              "Receiver",
				"onboardableEntity": map[string]any{"id": 1, "name": "Entity", "tenantAuthorizationInfo": map[string]any{"clientSecret": canary}},
				"lastValidationMsg": map[string]any{"errorMsg": canary},
				"lastModifiedBy":    map[string]any{"id": 1, "name": canary},
			}),
			standardPresent: []string{"onboardableEntity"},
			standardAbsent:  []string{"lastValidationMsg", "lastModifiedBy"},
			shareAbsent:     []string{"onboardableEntity", "lastValidationMsg", "lastModifiedBy"},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			standard := projectOneRecordInMode(t, resources.ProductZIA, tc.resource, redact.ModeStandard, []resources.SourceRecord{tc.record})
			for _, field := range tc.standardPresent {
				if _, ok := standard[field]; !ok {
					t.Errorf("standard projected %s missing %s", tc.resource, field)
				}
			}
			for _, field := range tc.standardAbsent {
				if _, ok := standard[field]; ok {
					t.Errorf("standard projected %s includes %s, want dropped", tc.resource, field)
				}
			}
			assertNoCanaries(t, tc.resource+" standard", standard, canary)

			for _, mode := range []redact.Mode{redact.ModeShare, redact.ModeParanoid} {
				got := projectOneRecordInMode(t, resources.ProductZIA, tc.resource, mode, []resources.SourceRecord{tc.record})
				for _, field := range tc.shareAbsent {
					if _, ok := got[field]; ok {
						t.Errorf("%s projected %s includes %s, want dropped", mode, tc.resource, field)
					}
				}
				assertNoCanaries(t, tc.resource+" "+string(mode), got, canary)
			}
		})
	}
}

func TestZIAListOnlyStaticBatchProjectionBoundaries(t *testing.T) {
	t.Parallel()

	const canary = "synthetic-sensitive-canary"
	tests := []struct {
		name            string
		resource        string
		record          resources.SourceRecord
		standardPresent []string
		standardAbsent  []string
		shareAbsent     []string
	}{
		{
			name:     "browser isolation profile URL is standard-only",
			resource: resourceBrowserIsolation,
			record: browserIsolationProfileSourceRecord(browserisolation.CBIProfile{
				ID:             "profile-id",
				Name:           "Isolation",
				URL:            "https://isolation.example.invalid",
				DefaultProfile: true,
			}),
			standardPresent: []string{"id", "url"},
			shareAbsent:     []string{"id", "url"},
		},
		{
			name:     "edm lite token list is dropped",
			resource: resourceDLPEDMLite,
			record: dlpEDMLiteSourceRecord(dlpedmlite.DLPEDMLite{
				Schema:    dlpedmlite.SchemaIDNameExtension{ID: 1, Name: "Schema", ExternalID: "external-schema"},
				TokenList: []dlpedmlite.TokenList{{Name: canary, Type: "TEXT", OriginalColumn: 1}},
			}),
			standardPresent: []string{"schema"},
			standardAbsent:  []string{"tokenList"},
			shareAbsent:     []string{"schema", "tokenList"},
		},
		{
			name:     "dc exclusion datacenter reference is standard-only",
			resource: resourceDCExclusions,
			record: dcExclusionSourceRecord(dcexclusions.DCExclusions{
				DcID:        1,
				Description: "maintenance window",
				DcName:      &ziacommon.IDNameExtensions{ID: 2, Name: "Datacenter"},
			}),
			standardPresent: []string{"dcName"},
			shareAbsent:     []string{"description", "dcName"},
		},
		{
			name:     "sub-cloud nested topology is standard-only and admin identity drops",
			resource: resourceSubClouds,
			record: subCloudSourceRecord(subclouds.SubClouds{
				ID:   1,
				Name: "Subcloud",
				Dcs:  []subclouds.DCs{{ID: 2, Name: "DC", Country: "US"}},
				Exclusions: []subclouds.Exclusions{{
					Datacenter:       &ziacommon.IDNameExtensions{ID: 3, Name: "Datacenter"},
					LastModifiedUser: &ziacommon.IDNameExtensions{ID: 4, Name: canary},
					Country:          "US",
				}},
			}),
			standardPresent: []string{"dcs", "exclusions"},
			shareAbsent:     []string{"dcs", "exclusions"},
		},
		{
			name:     "ipv6 config prefixes are standard-only",
			resource: resourceIPv6Config,
			record: ipv6ConfigSourceRecord(ipv6config.IPv6Config{
				IpV6Enabled: true,
				DnsPrefix:   "2001:db8:64::/96",
				NatPrefixes: []ipv6config.IPv6ConfigPrefix{{ID: 1, Name: "NAT64", PrefixMask: "2001:db8::/64"}},
			}),
			standardPresent: []string{"dnsPrefix", "natPrefixes"},
			shareAbsent:     []string{"dnsPrefix", "natPrefixes"},
		},
		{
			name:     "ipv6 dns64 prefix mask is standard-only",
			resource: resourceIPv6DNS64Prefix,
			record: ipv6ConfigPrefixSourceRecord(ipv6config.IPv6ConfigPrefix{
				ID:         1,
				Name:       "DNS64",
				PrefixMask: "2001:db8:64::/96",
			}),
			standardPresent: []string{"prefixMask"},
			shareAbsent:     []string{"prefixMask"},
		},
		{
			name:     "ipv6 nat64 prefix mask is standard-only",
			resource: resourceIPv6NAT64Prefix,
			record: ipv6ConfigPrefixSourceRecord(ipv6config.IPv6ConfigPrefix{
				ID:         1,
				Name:       "NAT64",
				PrefixMask: "2001:db8::/64",
			}),
			standardPresent: []string{"prefixMask"},
			shareAbsent:     []string{"prefixMask"},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			standard := projectOneRecordInMode(t, resources.ProductZIA, tc.resource, redact.ModeStandard, []resources.SourceRecord{tc.record})
			for _, field := range tc.standardPresent {
				if _, ok := standard[field]; !ok {
					t.Errorf("standard projected %s missing %s", tc.resource, field)
				}
			}
			for _, field := range tc.standardAbsent {
				if _, ok := standard[field]; ok {
					t.Errorf("standard projected %s includes %s, want dropped", tc.resource, field)
				}
			}
			assertNoCanaries(t, tc.resource+" standard", standard, canary)

			for _, mode := range []redact.Mode{redact.ModeShare, redact.ModeParanoid} {
				got := projectOneRecordInMode(t, resources.ProductZIA, tc.resource, mode, []resources.SourceRecord{tc.record})
				for _, field := range tc.shareAbsent {
					if _, ok := got[field]; ok {
						t.Errorf("%s projected %s includes %s, want dropped", mode, tc.resource, field)
					}
				}
				assertNoCanaries(t, tc.resource+" "+string(mode), got, canary)
			}
		})
	}
}

func TestZIASaaSCloudConfigBatchProjectionBoundaries(t *testing.T) {
	t.Parallel()

	const canary = "synthetic-sensitive-canary"
	tests := []struct {
		name            string
		resource        string
		record          resources.SourceRecord
		standardPresent []string
		standardAbsent  []string
		shareAbsent     []string
	}{
		{
			name:     "pac file content and URLs are standard-only",
			resource: resourcePACFiles,
			record: pacFileSourceRecord(pacfiles.PACFileConfig{
				ID:         1,
				Name:       "PAC",
				Domain:     "example.invalid",
				PACUrl:     "https://pac.example.invalid/proxy.pac",
				PACContent: "function FindProxyForURL() { return \"DIRECT\"; }",
				PACSubURL:  "obfuscated.example.invalid",
				LastModifiedBy: pacfiles.LastModifiedBy{
					ID:   2,
					Name: canary,
				},
			}),
			standardPresent: []string{"domain", "pacUrl", "pacContent", "pacSubURL"},
			standardAbsent:  []string{"lastModifiedBy"},
			shareAbsent:     []string{"domain", "pacUrl", "pacContent", "pacSubURL", "lastModifiedBy"},
		},
		{
			name:     "cloud application policy remains shareable",
			resource: resourceCloudAppPolicy,
			record: cloudApplicationPolicySourceRecord(cloudapplications.CloudApplications{
				App:        "APP",
				AppName:    "Application",
				Parent:     "PARENT",
				ParentName: "Parent",
			}),
			standardPresent: []string{"app", "appName", "parent", "parentName"},
		},
		{
			name:     "cloud application ssl policy remains shareable",
			resource: resourceCloudAppSSLPol,
			record: cloudApplicationPolicySourceRecord(cloudapplications.CloudApplications{
				App:        "APP",
				AppName:    "Application",
				Parent:     "PARENT",
				ParentName: "Parent",
			}),
			standardPresent: []string{"app", "appName", "parent", "parentName"},
		},
		{
			name:     "domain profile custom domains are standard-only",
			resource: resourceDomainProfiles,
			record: domainProfileSourceRecord(saassecurityapi.DomainProfiles{
				ProfileID:     1,
				ProfileName:   "Domain profile",
				CustomDomains: []string{"example.invalid"},
			}),
			standardPresent: []string{"customDomains"},
			shareAbsent:     []string{"description", "customDomains", "predefinedEmailDomains"},
		},
		{
			name:     "casb tombstone text is standard-only",
			resource: resourceCASBTombstones,
			record: casbTombstoneTemplateSourceRecord(saassecurityapi.QuarantineTombstoneLite{
				ID:          1,
				Name:        "Tombstone",
				Description: "template text",
			}),
			standardPresent: []string{"description"},
			shareAbsent:     []string{"description"},
		},
		{
			name:     "casb email label description is standard-only",
			resource: resourceCASBEmailLabels,
			record: casbEmailLabelSourceRecord(saassecurityapi.CasbEmailLabel{
				ID:        1,
				Name:      "Label",
				LabelDesc: "label text",
			}),
			standardPresent: []string{"labelDesc"},
			shareAbsent:     []string{"labelDesc"},
		},
		{
			name:     "casb tenant identifiers are standard-only",
			resource: resourceCASBTenants,
			record: casbTenantSourceRecord(saassecurityapi.CasbTenants{
				TenantID:           1,
				EnterpriseTenantID: "enterprise-tenant",
				TenantName:         "Tenant",
				SaaSApplication:    "SaaS",
			}),
			standardPresent: []string{"enterpriseTenantId", "tenantName"},
			shareAbsent:     []string{"lastTenantValidationTime", "enterpriseTenantId", "tenantName", "zscalerAppTenantId"},
		},
		{
			name:     "casb dlp rule identifiers and admin identity are constrained",
			resource: resourceCASBDLPRules,
			record: casbDLPRuleSourceRecord(casbdlprules.CasbDLPRules{
				ID:                   1,
				Name:                 "CASB DLP",
				BucketOwner:          "owner@example.invalid",
				ExternalAuditorEmail: "auditor@example.invalid",
				Domains:              []string{"example.invalid"},
				LastModifiedBy:       &ziacommon.IDNameExtensions{ID: 2, Name: canary},
				Receiver: &casbdlprules.Receiver{
					ID:   3,
					Name: "Receiver",
					Tenant: &ziacommon.IDNameExtensions{
						ID:   4,
						Name: "Tenant",
					},
				},
			}),
			standardPresent: []string{"bucketOwner", "externalAuditorEmail", "domains", "receiver"},
			standardAbsent:  []string{"lastModifiedBy"},
			shareAbsent:     []string{"bucketOwner", "externalAuditorEmail", "domains", "lastModifiedBy", "receiver"},
		},
		{
			name:     "casb malware rule identifiers and admin identity are constrained",
			resource: resourceCASBMalwareRules,
			record: casbMalwareRuleSourceRecord(casbmalwarerules.CasbMalwareRules{
				ID:                   1,
				Name:                 "CASB Malware",
				QuarantineLocation:   "quarantine://folder",
				ScanInboundEmailLink: "https://mail.example.invalid",
				LastModifiedBy:       &ziacommon.IDNameExtensions{ID: 2, Name: canary},
			}),
			standardPresent: []string{"quarantineLocation", "scanInboundEmailLink"},
			standardAbsent:  []string{"lastModifiedBy"},
			shareAbsent:     []string{"quarantineLocation", "scanInboundEmailLink", "lastModifiedBy"},
		},
		{
			name:     "browser control targeting and profile URL are standard-only",
			resource: resourceBrowserControl,
			record: browserControlSettingsSourceRecord(browsercontrolsettings.BrowserControlSettings{
				PluginCheckFrequency: "DAILY",
				BypassPlugins:        []string{"PLUGIN"},
				SmartIsolationUsers: []ziacommon.IDNameExtensions{
					{ID: 1, Name: "User"},
				},
				SmartIsolationProfile: browsercontrolsettings.SmartIsolationProfile{
					ID:   "profile-id",
					Name: "Profile",
					URL:  "https://isolation.example.invalid",
				},
			}),
			standardPresent: []string{"pluginCheckFrequency", "bypassPlugins", "smartIsolationUsers", "smartIsolationProfile"},
			shareAbsent:     []string{"bypassPlugins", "smartIsolationUsers", "smartIsolationProfile"},
		},
		{
			name:     "supported browser versions remain shareable",
			resource: resourceSupportedBrowsers,
			record: structSourceRecord(securebrowsing.SupportedBrowserVersion{
				BrowserType:   "CHROME",
				Versions:      []string{"120"},
				OlderVersions: []string{"119"},
			}),
			standardPresent: []string{"browserType", "versions", "olderVersions"},
		},
		{
			name:     "ftp control URLs are standard-only",
			resource: resourceFTPControl,
			record: structSourceRecord(ftpcontrolpolicy.FTPControlPolicy{
				FtpOverHttpEnabled: true,
				FtpEnabled:         true,
				UrlCategories:      []string{"BUSINESS_AND_ECONOMY"},
				Urls:               []string{"ftp.example.invalid"},
			}),
			standardPresent: []string{"ftpOverHttpEnabled", "ftpEnabled", "urlCategories", "urls"},
			shareAbsent:     []string{"urls"},
		},
		{
			name:     "remote assistance settings remain shareable",
			resource: resourceRemoteAssistance,
			record: structSourceRecord(remoteassistance.RemoteAssistance{
				ViewOnlyUntil:       1,
				FullAccessUntil:     2,
				UsernameObfuscated:  true,
				DeviceInfoObfuscate: true,
			}),
			standardPresent: []string{"viewOnlyUntil", "fullAccessUntil", "usernameObfuscated", "deviceInfoObfuscate"},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			standard := projectOneRecordInMode(t, resources.ProductZIA, tc.resource, redact.ModeStandard, []resources.SourceRecord{tc.record})
			for _, field := range tc.standardPresent {
				if _, ok := standard[field]; !ok {
					t.Errorf("standard projected %s missing %s", tc.resource, field)
				}
			}
			for _, field := range tc.standardAbsent {
				if _, ok := standard[field]; ok {
					t.Errorf("standard projected %s includes %s, want dropped", tc.resource, field)
				}
			}
			assertNoCanaries(t, tc.resource+" standard", standard, canary)

			for _, mode := range []redact.Mode{redact.ModeShare, redact.ModeParanoid} {
				got := projectOneRecordInMode(t, resources.ProductZIA, tc.resource, mode, []resources.SourceRecord{tc.record})
				for _, field := range tc.shareAbsent {
					if _, ok := got[field]; ok {
						t.Errorf("%s projected %s includes %s, want dropped", mode, tc.resource, field)
					}
				}
				assertNoCanaries(t, tc.resource+" "+string(mode), got, canary)
			}
		})
	}
}

func TestZIAAdminGovernanceProjectionBoundaries(t *testing.T) {
	t.Parallel()

	const canary = "synthetic-admin-secret-canary"
	tests := []struct {
		name            string
		resource        string
		record          resources.SourceRecord
		standardPresent []string
		standardAbsent  []string
		shareAbsent     []string
	}{
		{
			name:     "admin user identifiers are standard-only and material is dropped",
			resource: resourceAdminUsers,
			record: ziaAdminUserSourceRecord(ziaadminusers.AdminUsers{
				ID:                1,
				LoginName:         "admin@example.invalid",
				UserName:          "Admin User",
				Email:             "admin@example.invalid",
				Comments:          "admin note",
				Password:          canary,
				AdminScopeType:    "ORG",
				IsPasswordExpired: true,
				Role: &ziaadminusers.Role{
					ID:         2,
					Name:       "Admin role",
					Extensions: map[string]interface{}{"secret": canary},
				},
				ExecMobileAppTokens: []ziaadminusers.ExecMobileAppTokens{
					{Token: canary, DeviceName: "device"},
				},
			}),
			standardPresent: []string{"loginName", "userName", "email", "comments", "role"},
			standardAbsent:  []string{"password", "isPasswordExpired", "execMobileAppTokens"},
			shareAbsent:     []string{"loginName", "userName", "email", "comments", "password", "role", "execMobileAppTokens"},
		},
		{
			name:     "admin role permission maps are dropped",
			resource: resourceAdminRoles,
			record: ziaAdminRoleSourceRecord(ziaadminroles.AdminRoles{
				ID:                    1,
				Name:                  "Admin role",
				PolicyAccess:          "READ_WRITE",
				Permissions:           []string{"POLICY"},
				FeaturePermissions:    map[string]interface{}{"feature": canary},
				ExtFeaturePermissions: map[string]interface{}{"external": canary},
			}),
			standardPresent: []string{"name", "policyAccess", "permissions"},
			standardAbsent:  []string{"featurePermissions", "extFeaturePermissions"},
			shareAbsent:     []string{"policyAccess", "permissions", "featurePermissions", "extFeaturePermissions"},
		},
		{
			name:     "password expiry settings are shareable policy",
			resource: resourcePasswordExpiry,
			record: structSourceRecord(ziaadminusers.PasswordExpiry{
				PasswordExpirationEnabled: true,
				PasswordExpiryDays:        90,
			}),
			standardPresent: []string{"passwordExpirationEnabled", "passwordExpiryDays"},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			standard := projectOneRecordInMode(t, resources.ProductZIA, tc.resource, redact.ModeStandard, []resources.SourceRecord{tc.record})
			for _, field := range tc.standardPresent {
				if _, ok := standard[field]; !ok {
					t.Errorf("standard projected %s missing %s", tc.resource, field)
				}
			}
			for _, field := range tc.standardAbsent {
				if _, ok := standard[field]; ok {
					t.Errorf("standard projected %s includes %s, want dropped", tc.resource, field)
				}
			}
			assertNoCanaries(t, tc.resource+" standard", standard, canary)

			for _, mode := range []redact.Mode{redact.ModeShare, redact.ModeParanoid} {
				got := projectOneRecordInMode(t, resources.ProductZIA, tc.resource, mode, []resources.SourceRecord{tc.record})
				for _, field := range tc.shareAbsent {
					if _, ok := got[field]; ok {
						t.Errorf("%s projected %s includes %s, want dropped", mode, tc.resource, field)
					}
				}
				assertNoCanaries(t, tc.resource+" "+string(mode), got, canary)
			}
		})
	}
}

func TestReaderGetLocationRejectsNonNumericID(t *testing.T) {
	t.Parallel()

	reader := &SDKReader{
		cfg: validReaderConfig(),
		handlers: map[resourceKey]resourceHandler{
			{product: resources.ProductZIA, name: resourceLocations}: fakeLocationsResourceHandler(fakeZIALocationClient{}),
		},
	}

	_, err := reader.Get(context.Background(), resources.ProductZIA, "locations", "not-a-number")
	if !errors.Is(err, ErrInvalidResourceID) {
		t.Fatalf("SDKReader.Get(non-numeric id) error = %v, want ErrInvalidResourceID", err)
	}
}

func TestReaderGetRuleLabelDispatchesByResource(t *testing.T) {
	t.Parallel()

	reader := &SDKReader{
		cfg: validReaderConfig(),
		handlers: map[resourceKey]resourceHandler{
			{product: resources.ProductZIA, name: resourceRuleLabels}: fakeRuleLabelsResourceHandler(fakeZIARuleLabelsClient{
				label: &rulelabels.RuleLabels{
					ID:                  789,
					Name:                "Outbound",
					ReferencedRuleCount: 3,
				},
			}),
		},
	}

	record, err := reader.Get(context.Background(), resources.ProductZIA, "rule-labels", "789")
	if err != nil {
		t.Fatalf("SDKReader.Get(zia, rule-labels, 789) error = %v, want nil", err)
	}
	spec, ok := resources.FindSpec(resources.ProductZIA, "rule-labels")
	if !ok {
		t.Fatal("FindSpec(zia, rule-labels) ok = false, want true")
	}
	projected, _, err := resources.ProjectRecord(spec, redact.ModeStandard, record)
	if err != nil {
		t.Fatalf("ProjectRecord(zia rule-labels) error = %v, want nil", err)
	}
	got := projected.Fields()
	if got["id"] != 789 {
		t.Errorf("projected rule-label id = %v, want 789", got["id"])
	}
	if got["name"] != "Outbound" {
		t.Errorf("projected rule-label name = %v, want Outbound", got["name"])
	}
	if err := resources.AssertRenderedSubset(spec, redact.ModeStandard, got); err != nil {
		t.Errorf("AssertRenderedSubset(projected rule label) error = %v, want nil", err)
	}
}

func TestReaderGetLocationGroupDispatchesByResource(t *testing.T) {
	t.Parallel()

	reader := &SDKReader{
		cfg: validReaderConfig(),
		handlers: map[resourceKey]resourceHandler{
			{product: resources.ProductZIA, name: resourceLocationGroups}: fakeLocationGroupsResourceHandler(fakeZIALocationGroupsClient{
				group: &locationgroups.LocationGroup{
					ID:        987,
					Name:      "Branch group",
					GroupType: "STATIC_GROUP",
				},
			}),
		},
	}

	record, err := reader.Get(context.Background(), resources.ProductZIA, "location-groups", "987")
	if err != nil {
		t.Fatalf("SDKReader.Get(zia, location-groups, 987) error = %v, want nil", err)
	}
	spec, ok := resources.FindSpec(resources.ProductZIA, "location-groups")
	if !ok {
		t.Fatal("FindSpec(zia, location-groups) ok = false, want true")
	}
	projected, _, err := resources.ProjectRecord(spec, redact.ModeStandard, record)
	if err != nil {
		t.Fatalf("ProjectRecord(zia location-groups) error = %v, want nil", err)
	}
	got := projected.Fields()
	if got["id"] != 987 {
		t.Errorf("projected location-group id = %v, want 987", got["id"])
	}
	if got["name"] != "Branch group" {
		t.Errorf("projected location-group name = %v, want Branch group", got["name"])
	}
	if got["groupType"] != "STATIC_GROUP" {
		t.Errorf("projected location-group groupType = %v, want STATIC_GROUP", got["groupType"])
	}
	if err := resources.AssertRenderedSubset(spec, redact.ModeStandard, got); err != nil {
		t.Errorf("AssertRenderedSubset(projected location-group) error = %v, want nil", err)
	}
}

func TestReaderGetStaticIPDispatchesByResource(t *testing.T) {
	t.Parallel()

	reader := &SDKReader{
		cfg: validReaderConfig(),
		handlers: map[resourceKey]resourceHandler{
			{product: resources.ProductZIA, name: resourceStaticIPs}: fakeStaticIPsResourceHandler(fakeZIAStaticIPsClient{
				staticIP: &staticips.StaticIP{
					ID:        321,
					IpAddress: "192.0.2.44",
				},
			}),
		},
	}

	record, err := reader.Get(context.Background(), resources.ProductZIA, "static-ips", "321")
	if err != nil {
		t.Fatalf("SDKReader.Get(zia, static-ips, 321) error = %v, want nil", err)
	}
	spec, ok := resources.FindSpec(resources.ProductZIA, "static-ips")
	if !ok {
		t.Fatal("FindSpec(zia, static-ips) ok = false, want true")
	}
	projected, _, err := resources.ProjectRecord(spec, redact.ModeStandard, record)
	if err != nil {
		t.Fatalf("ProjectRecord(zia static-ips) error = %v, want nil", err)
	}
	got := projected.Fields()
	if got["id"] != 321 {
		t.Errorf("projected static-ip id = %v, want 321", got["id"])
	}
	if got["ipAddress"] != "192.0.2.44" {
		t.Errorf("projected static-ip ipAddress = %v, want 192.0.2.44", got["ipAddress"])
	}
	if err := resources.AssertRenderedSubset(spec, redact.ModeStandard, got); err != nil {
		t.Errorf("AssertRenderedSubset(projected static-ip) error = %v, want nil", err)
	}
}

func TestReaderListZPAServerGroupsProjectsSDKShapeThroughAllowList(t *testing.T) {
	t.Parallel()

	const (
		freeTextCanary = "psk=server-group-canary-value"
		nestedCanary   = "server-group-nested-canary"
	)
	reader := &SDKReader{
		cfg: validReaderConfig(),
		handlers: map[resourceKey]resourceHandler{
			{product: resources.ProductZPA, name: resourceZPAServerGroups}: newListGetHandler(
				resourceZPAServerGroups,
				func(context.Context) ([]zpaservergroup.ServerGroup, error) {
					return []zpaservergroup.ServerGroup{{
						ID:               "sg-1",
						Name:             "server-group",
						Description:      freeTextCanary,
						Enabled:          true,
						ConfigSpace:      "DEFAULT",
						DynamicDiscovery: true,
						Applications: []zpaservergroup.Applications{{
							ID:   "app-1",
							Name: nestedCanary,
						}},
					}}, nil
				},
				func(context.Context, string) (*zpaservergroup.ServerGroup, error) { return nil, nil },
				jsonSourceRecord[zpaservergroup.ServerGroup],
			),
		},
	}

	records, err := reader.List(context.Background(), resources.ProductZPA, resourceZPAServerGroups)
	if err != nil {
		t.Fatalf("SDKReader.List(zpa, server-groups) error = %v, want nil", err)
	}
	spec, ok := resources.FindSpec(resources.ProductZPA, resourceZPAServerGroups)
	if !ok {
		t.Fatalf("FindSpec(zpa, %s) ok = false, want true", resourceZPAServerGroups)
	}
	projected, reports, err := resources.ProjectRecords(spec, redact.ModeStandard, records)
	if err != nil {
		t.Fatalf("ProjectRecords(zpa server-groups) error = %v, want nil", err)
	}
	gotRecords := projected.Records()
	if len(gotRecords) != 1 {
		t.Fatalf("ProjectRecords(zpa server-groups) records length = %d, want 1", len(gotRecords))
	}
	got := gotRecords[0].Fields()
	if got["id"] != "sg-1" {
		t.Errorf("projected server-group id = %v, want sg-1", got["id"])
	}
	description, ok := got["description"].(string)
	if !ok || !strings.Contains(description, "<REDACTED:SECRET>") || strings.Contains(description, "server-group-canary-value") {
		t.Errorf("projected server-group description = %v, want redacted canary value", got["description"])
	}
	for _, field := range []string{"applications", "configSpace", "dynamicDiscovery"} {
		if _, ok := got[field]; ok {
			t.Errorf("projected server-group includes %s, want dropped", field)
		}
	}
	if strings.Contains(fmt.Sprint(got), nestedCanary) {
		t.Errorf("projected server-group = %v, want nested canary absent", got)
	}
	if err := resources.AssertRenderedSubset(spec, redact.ModeStandard, got); err != nil {
		t.Errorf("AssertRenderedSubset(projected server-group) error = %v, want nil", err)
	}
	if len(reports) != 1 {
		t.Fatalf("ProjectRecords(zpa server-groups) reports length = %d, want 1", len(reports))
	}
	assertReportContains(t, reports[0].DroppedFields, "applications")
	assertReportContains(t, reports[0].DroppedFields, "configSpace")
	assertReportContains(t, reports[0].RedactedFields, "description")
}

func TestReaderListZPAApplicationSegmentsProjectsSDKShapeThroughReferenceAllowList(t *testing.T) {
	t.Parallel()

	const (
		freeTextCanary = "psk=application-segment-canary-value"
		nestedCanary   = "nested-application-segment-secret-canary"
	)
	reader := SDKReader{
		handlers: map[resourceKey]resourceHandler{
			{product: resources.ProductZPA, name: resourceZPAAppSegments}: newListGetHandler(
				resourceZPAAppSegments,
				func(context.Context) ([]zpaapplicationsegment.ApplicationSegmentResource, error) {
					return []zpaapplicationsegment.ApplicationSegmentResource{{
						ID:                        "app-seg-1",
						Name:                      "Payroll application",
						Description:               freeTextCanary,
						Enabled:                   true,
						DomainNames:               []string{"payroll.example.internal"},
						TCPPortRanges:             []string{"443"},
						UDPPortRanges:             []string{"5000"},
						APIProtectionEnabled:      true,
						AutoAppProtectEnabled:     true,
						ADPEnabled:                true,
						BypassOnReauth:            true,
						DoubleEncrypt:             true,
						InspectTrafficWithZia:     true,
						BypassType:                "NEVER",
						HealthCheckType:           "DEFAULT",
						IcmpAccessType:            "PING",
						HealthReporting:           "ON_ACCESS",
						PassiveHealthEnabled:      true,
						IpAnchored:                true,
						FQDNDnsCheck:              true,
						TCPKeepAlive:              "1",
						IsCnameEnabled:            true,
						SelectConnectorCloseToApp: true,
						RestrictionType:           "NONE",
						UseInDrMode:               true,
						CreationTime:              "1700000000000",
						ModifiedTime:              "1700000100000",
						SegmentGroupID:            nestedCanary,
						SegmentGroupName:          "Payroll segment group",
						MicroTenantID:             nestedCanary,
						MicroTenantName:           "Default microtenant",
						ModifiedBy:                nestedCanary,
						AppRecommendationId:       nestedCanary,
						Applications:              nestedCanary,
						ConfigSpace:               nestedCanary,
						ShareToMicrotenants:       []string{nestedCanary},
						TCPAppPortRange:           []zpacommon.NetworkPorts{{From: "1", To: "65535"}},
						UDPAppPortRange:           []zpacommon.NetworkPorts{{From: "1", To: "65535"}},
						ServerGroups: []zpaservergroup.ServerGroup{{
							ID:   "sg-1",
							Name: "Payroll servers",
							Servers: []zpaappservercontroller.ApplicationServer{{
								ID:      "server-1",
								Name:    nestedCanary,
								Address: nestedCanary,
							}},
							AppConnectorGroups: []zpaappconnectorgroup.AppConnectorGroup{{
								ID:       "connector-group-1",
								Name:     nestedCanary,
								Location: nestedCanary,
							}},
						}},
						ClientlessApps: []zpaapplicationsegmentbrowseraccess.ClientlessApps{{
							ID:     "clientless-1",
							Name:   nestedCanary,
							Domain: nestedCanary,
						}},
						SharedMicrotenantDetails: zpaapplicationsegment.SharedMicrotenantDetails{
							SharedFromMicrotenant: zpaapplicationsegment.SharedFromMicrotenant{
								ID:   nestedCanary,
								Name: nestedCanary,
							},
							SharedToMicrotenants: []zpaapplicationsegment.SharedToMicrotenant{{
								ID:   nestedCanary,
								Name: nestedCanary,
							}},
						},
						ZPNERID: &zpacommon.ZPNERID{
							ID:        nestedCanary,
							ZIAErName: nestedCanary,
						},
						Tags: []zpaapplicationsegment.Tag{{
							Namespace: zpacommon.CommonSummary{ID: "ns-1", Name: nestedCanary},
							TagKey:    zpacommon.CommonSummary{ID: "key-1", Name: nestedCanary},
							TagValue:  zpacommon.CommonIDName{ID: "value-1", Name: nestedCanary},
							Origin:    nestedCanary,
						}},
					}}, nil
				},
				func(context.Context, string) (*zpaapplicationsegment.ApplicationSegmentResource, error) {
					return nil, nil
				},
				applicationSegmentSourceRecord,
			),
		},
	}

	records, err := reader.List(context.Background(), resources.ProductZPA, resourceZPAAppSegments)
	if err != nil {
		t.Fatalf("SDKReader.List(zpa, application-segments) error = %v, want nil", err)
	}
	spec, ok := resources.FindSpec(resources.ProductZPA, resourceZPAAppSegments)
	if !ok {
		t.Fatalf("FindSpec(zpa, %s) ok = false, want true", resourceZPAAppSegments)
	}

	projected, reports, err := resources.ProjectRecords(spec, redact.ModeStandard, records)
	if err != nil {
		t.Fatalf("ProjectRecords(zpa application-segments, standard) error = %v, want nil", err)
	}
	gotRecords := projected.Records()
	if len(gotRecords) != 1 {
		t.Fatalf("ProjectRecords(zpa application-segments, standard) records length = %d, want 1", len(gotRecords))
	}
	got := gotRecords[0].Fields()
	if got["id"] != "app-seg-1" {
		t.Errorf("projected application-segment id = %v, want app-seg-1", got["id"])
	}
	if got["name"] != "Payroll application" {
		t.Errorf("projected application-segment name = %v, want Payroll application", got["name"])
	}
	description, ok := got["description"].(string)
	if !ok || !strings.Contains(description, "<REDACTED:SECRET>") || strings.Contains(description, "application-segment-canary-value") {
		t.Errorf("projected application-segment description = %v, want redacted canary value", got["description"])
	}
	serverGroups := mustProjectedList(t, got, "serverGroups")
	if len(serverGroups) != 1 {
		t.Fatalf("projected application-segment serverGroups length = %d, want 1", len(serverGroups))
	}
	serverGroup, ok := serverGroups[0].(map[string]any)
	if !ok {
		t.Fatalf("projected application-segment serverGroups[0] = %T, want map[string]any", serverGroups[0])
	}
	if serverGroup["id"] != "sg-1" || serverGroup["name"] != "Payroll servers" {
		t.Errorf("projected application-segment serverGroups[0] = %v, want id/name reference", serverGroup)
	}
	for _, field := range []string{"servers", "appConnectorGroups", "description", "enabled", "microtenantName"} {
		if _, ok := serverGroup[field]; ok {
			t.Errorf("projected application-segment serverGroups[0] includes %s, want reference only", field)
		}
	}
	for _, field := range []string{
		"modifiedBy",
		"segmentGroupId",
		"microtenantId",
		"clientlessApps",
		"sharedMicrotenantDetails",
		"tags",
		"zpnErId",
		"shareToMicrotenants",
		"tcpPortRange",
		"udpPortRange",
	} {
		if _, ok := got[field]; ok {
			t.Errorf("projected application-segment includes %s, want dropped", field)
		}
	}
	if strings.Contains(fmt.Sprint(got), nestedCanary) {
		t.Errorf("projected application-segment = %v, want nested canary absent", got)
	}
	if err := resources.AssertRenderedSubset(spec, redact.ModeStandard, got); err != nil {
		t.Errorf("AssertRenderedSubset(projected application-segment standard) error = %v, want nil", err)
	}
	if len(reports) != 1 {
		t.Fatalf("ProjectRecords(zpa application-segments) reports length = %d, want 1", len(reports))
	}
	assertReportContains(t, reports[0].DroppedFields, "clientlessApps")
	assertReportContains(t, reports[0].DroppedFields, "sharedMicrotenantDetails")
	assertReportContains(t, reports[0].DroppedFields, "tags")
	assertReportContains(t, reports[0].DroppedFields, "zpnErId")
	assertReportContains(t, reports[0].RedactedFields, "description")

	shareGot := projectApplicationSegmentMode(t, spec, records, redact.ModeShare)
	for _, field := range []string{
		"description",
		"domainNames",
		"tcpPortRanges",
		"udpPortRanges",
		"apiProtectionEnabled",
		"inspectTrafficWithZia",
		"doubleEncrypt",
		"adpEnabled",
		"autoAppProtectEnabled",
		"bypassOnReauth",
	} {
		if _, ok := shareGot[field]; ok {
			t.Errorf("share-mode projected application-segment includes %s, want dropped", field)
		}
	}
	if shareGot["name"] != "Payroll application" {
		t.Errorf("share-mode projected application-segment name = %v, want Payroll application", shareGot["name"])
	}
	shareServerGroup := mustProjectedList(t, shareGot, "serverGroups")[0].(map[string]any)
	if shareServerGroup["name"] != "Payroll servers" {
		t.Errorf("share-mode projected application-segment serverGroups[0].name = %v, want Payroll servers", shareServerGroup["name"])
	}

	paranoidGot := projectApplicationSegmentMode(t, spec, records, redact.ModeParanoid)
	for _, field := range []string{"name", "segmentGroupName", "microtenantName"} {
		if _, ok := paranoidGot[field]; ok {
			t.Errorf("paranoid-mode projected application-segment includes %s, want dropped", field)
		}
	}
	paranoidServerGroup := mustProjectedList(t, paranoidGot, "serverGroups")[0].(map[string]any)
	if paranoidServerGroup["id"] != "sg-1" {
		t.Errorf("paranoid-mode projected application-segment serverGroups[0].id = %v, want sg-1", paranoidServerGroup["id"])
	}
	if _, ok := paranoidServerGroup["name"]; ok {
		t.Errorf("paranoid-mode projected application-segment serverGroups[0] includes name, want id-only reference")
	}
}

func TestZPAApplicationSegmentServerGroupReferenceDoesNotOutExposeServerGroups(t *testing.T) {
	t.Parallel()

	appSpec, ok := resources.FindSpec(resources.ProductZPA, resourceZPAAppSegments)
	if !ok {
		t.Fatalf("FindSpec(zpa, %s) ok = false, want true", resourceZPAAppSegments)
	}
	serverGroupSpec, ok := resources.FindSpec(resources.ProductZPA, resourceZPAServerGroups)
	if !ok {
		t.Fatalf("FindSpec(zpa, %s) ok = false, want true", resourceZPAServerGroups)
	}
	appServerGroups := mustFieldSpec(t, appSpec.Fields, "serverGroups")
	if _, ok := fieldSpecByName(appServerGroups.Fields, "servers"); ok {
		t.Fatalf("zpa/application-segments serverGroups models servers, want id/name reference only")
	}
	standalone := fieldSpecMap(serverGroupSpec.Fields)
	for _, nested := range appServerGroups.Fields {
		topLevel, ok := standalone[nested.JSONField()]
		if !ok {
			t.Errorf("zpa/application-segments serverGroups.%s has no zpa/server-groups field", nested.JSONField())
			continue
		}
		for _, mode := range []redact.Mode{redact.ModeStandard, redact.ModeShare, redact.ModeParanoid} {
			if nested.AllowedIn(mode) && !topLevel.AllowedIn(mode) {
				t.Errorf("zpa/application-segments serverGroups.%s allowed in %s but zpa/server-groups.%s is not", nested.JSONField(), mode, topLevel.JSONField())
			}
		}
	}
}

func TestReaderListZPAServiceEdgeGroupsProjectsSDKShapeThroughAllowList(t *testing.T) {
	t.Parallel()

	const nestedCanary = "nested-service-edge-canary"
	reader := SDKReader{
		handlers: map[resourceKey]resourceHandler{
			{product: resources.ProductZPA, name: resourceZPAServiceGrps}: newListGetHandler(
				resourceZPAServiceGrps,
				func(context.Context) ([]zpaserviceedgegroup.ServiceEdgeGroup, error) {
					return []zpaserviceedgegroup.ServiceEdgeGroup{{
						ID:          "seg-1",
						Name:        "Service edge group",
						Description: "psk=service-edge-group-canary-value",
						Enabled:     true,
						ServiceEdges: []zpaserviceedgecontroller.ServiceEdgeController{{
							ID:                  "edge-1",
							Name:                nestedCanary,
							ProvisioningKeyName: nestedCanary,
						}},
						EnrollmentCertID: nestedCanary,
					}}, nil
				},
				func(context.Context, string) (*zpaserviceedgegroup.ServiceEdgeGroup, error) { return nil, nil },
				jsonSourceRecord[zpaserviceedgegroup.ServiceEdgeGroup],
			),
		},
	}

	records, err := reader.List(context.Background(), resources.ProductZPA, resourceZPAServiceGrps)
	if err != nil {
		t.Fatalf("SDKReader.List(zpa, service-edge-groups) error = %v, want nil", err)
	}
	spec, ok := resources.FindSpec(resources.ProductZPA, resourceZPAServiceGrps)
	if !ok {
		t.Fatalf("FindSpec(zpa, %s) ok = false, want true", resourceZPAServiceGrps)
	}
	projected, reports, err := resources.ProjectRecords(spec, redact.ModeStandard, records)
	if err != nil {
		t.Fatalf("ProjectRecords(zpa service-edge-groups) error = %v, want nil", err)
	}
	gotRecords := projected.Records()
	if len(gotRecords) != 1 {
		t.Fatalf("ProjectRecords(zpa service-edge-groups) records length = %d, want 1", len(gotRecords))
	}
	got := gotRecords[0].Fields()
	if got["id"] != "seg-1" {
		t.Errorf("projected service-edge-group id = %v, want seg-1", got["id"])
	}
	description, ok := got["description"].(string)
	if !ok || !strings.Contains(description, "<REDACTED:SECRET>") || strings.Contains(description, "service-edge-group-canary-value") {
		t.Errorf("projected service-edge-group description = %v, want redacted canary value", got["description"])
	}
	for _, field := range []string{"serviceEdges", "enrollmentCertId"} {
		if _, ok := got[field]; ok {
			t.Errorf("projected service-edge-group includes %s, want dropped", field)
		}
	}
	if strings.Contains(fmt.Sprint(got), nestedCanary) {
		t.Errorf("projected service-edge-group = %v, want nested canary absent", got)
	}
	if err := resources.AssertRenderedSubset(spec, redact.ModeStandard, got); err != nil {
		t.Errorf("AssertRenderedSubset(projected service-edge-group) error = %v, want nil", err)
	}
	if len(reports) != 1 {
		t.Fatalf("ProjectRecords(zpa service-edge-groups) reports length = %d, want 1", len(reports))
	}
	assertReportContains(t, reports[0].DroppedFields, "serviceEdges")
	assertReportContains(t, reports[0].DroppedFields, "enrollmentCertId")
	assertReportContains(t, reports[0].RedactedFields, "description")
}

func TestReaderListZPAAppConnectorsProjectsSDKShapeThroughAllowList(t *testing.T) {
	t.Parallel()

	const nestedCanary = "nested-app-connector-secret-canary"
	reader := SDKReader{
		handlers: map[resourceKey]resourceHandler{
			{product: resources.ProductZPA, name: resourceZPAAppConnectors}: newListGetHandler(
				resourceZPAAppConnectors,
				func(context.Context) ([]zpaappconnectorcontroller.AppConnector, error) {
					return []zpaappconnectorcontroller.AppConnector{{
						ID:                               "app-connector-1",
						Name:                             "App connector",
						Description:                      "psk=app-connector-canary-value",
						Enabled:                          true,
						AppConnectorGroupID:              nestedCanary,
						AppConnectorGroupName:            "App connector group",
						ApplicationStartTime:             "1700000000000",
						AssistantVersion:                 zpaappconnectorcontroller.AssistantVersion{ID: "assistant-1", BrokerId: nestedCanary},
						ControlChannelStatus:             "CONNECTED",
						CreationTime:                     "1700000100000",
						CtrlBrokerName:                   "Broker",
						CurrentVersion:                   "1.2.3",
						EnrollmentCert:                   map[string]any{"name": nestedCanary},
						ExpectedUpgradeTime:              "1700000200000",
						ExpectedVersion:                  "1.2.4",
						Fingerprint:                      nestedCanary,
						IPACL:                            "198.51.100.10",
						IssuedCertID:                     nestedCanary,
						LastBrokerConnectTime:            "1700000300000",
						LastBrokerConnectTimeDuration:    "100",
						LastBrokerDisconnectTime:         "1700000400000",
						LastBrokerDisconnectTimeDuration: "200",
						LastUpgradeTime:                  "1700000500000",
						Latitude:                         "37.3387",
						Location:                         "San Jose, CA",
						Longitude:                        "-121.8853",
						MicroTenantID:                    nestedCanary,
						MicroTenantName:                  "Microtenant",
						ModifiedBy:                       "admin-1",
						ModifiedTime:                     "1700000600000",
						Platform:                         "linux",
						PlatformDetail:                   "el8",
						PreviousVersion:                  "1.2.2",
						PrivateIP:                        "10.0.0.20",
						ProvisioningKeyID:                nestedCanary,
						ProvisioningKeyName:              nestedCanary,
						PublicIP:                         "203.0.113.20",
						ReadOnly:                         true,
						RestrictionType:                  "CUSTOMER",
						RuntimeOS:                        "linux",
						SargeVersion:                     "1.2.3",
						UpgradeAttempt:                   nestedCanary,
						UpgradeStatus:                    "COMPLETE",
						ZPNSubModuleUpgrade:              []zpacommon.ZPNSubModuleUpgrade{{ID: "module-1", EntityGid: nestedCanary}},
						ZscalerManaged:                   true,
					}}, nil
				},
				func(context.Context, string) (*zpaappconnectorcontroller.AppConnector, error) {
					return nil, nil
				},
				jsonSourceRecord[zpaappconnectorcontroller.AppConnector],
			),
		},
	}

	records, err := reader.List(context.Background(), resources.ProductZPA, resourceZPAAppConnectors)
	if err != nil {
		t.Fatalf("SDKReader.List(zpa, app-connectors) error = %v, want nil", err)
	}
	spec, ok := resources.FindSpec(resources.ProductZPA, resourceZPAAppConnectors)
	if !ok {
		t.Fatalf("FindSpec(zpa, %s) ok = false, want true", resourceZPAAppConnectors)
	}
	projected, reports, err := resources.ProjectRecords(spec, redact.ModeStandard, records)
	if err != nil {
		t.Fatalf("ProjectRecords(zpa app-connectors) error = %v, want nil", err)
	}
	gotRecords := projected.Records()
	if len(gotRecords) != 1 {
		t.Fatalf("ProjectRecords(zpa app-connectors) records length = %d, want 1", len(gotRecords))
	}
	got := gotRecords[0].Fields()
	if got["id"] != "app-connector-1" {
		t.Errorf("projected app-connector id = %v, want app-connector-1", got["id"])
	}
	if got["location"] != "San Jose, CA" {
		t.Errorf("projected app-connector location = %v, want San Jose, CA", got["location"])
	}
	description, ok := got["description"].(string)
	if !ok || !strings.Contains(description, "<REDACTED:SECRET>") || strings.Contains(description, "app-connector-canary-value") {
		t.Errorf("projected app-connector description = %v, want redacted canary value", got["description"])
	}
	for _, field := range []string{
		"appConnectorGroupId",
		"assistantVersion",
		"enrollmentCert",
		"fingerprint",
		"issuedCertId",
		"microtenantId",
		"provisioningKeyId",
		"provisioningKeyName",
		"upgradeAttempt",
		"zpnSubModuleUpgradeList",
		"zscalerManaged",
	} {
		if _, ok := got[field]; ok {
			t.Errorf("projected app-connector includes %s, want dropped", field)
		}
	}
	if strings.Contains(fmt.Sprint(got), nestedCanary) {
		t.Errorf("projected app-connector = %v, want nested canary absent", got)
	}
	if err := resources.AssertRenderedSubset(spec, redact.ModeStandard, got); err != nil {
		t.Errorf("AssertRenderedSubset(projected app-connector) error = %v, want nil", err)
	}
	if len(reports) != 1 {
		t.Fatalf("ProjectRecords(zpa app-connectors) reports length = %d, want 1", len(reports))
	}
	assertReportContains(t, reports[0].DroppedFields, "appConnectorGroupId")
	assertReportContains(t, reports[0].DroppedFields, "assistantVersion")
	assertReportContains(t, reports[0].DroppedFields, "enrollmentCert")
	assertReportContains(t, reports[0].DroppedFields, "fingerprint")
	assertReportContains(t, reports[0].DroppedFields, "issuedCertId")
	assertReportContains(t, reports[0].DroppedFields, "microtenantId")
	assertReportContains(t, reports[0].DroppedFields, "provisioningKeyId")
	assertReportContains(t, reports[0].DroppedFields, "provisioningKeyName")
	assertReportContains(t, reports[0].DroppedFields, "upgradeAttempt")
	assertReportContains(t, reports[0].DroppedFields, "zpnSubModuleUpgradeList")
	assertReportContains(t, reports[0].DroppedFields, "zscalerManaged")
	assertReportContains(t, reports[0].RedactedFields, "description")
}

func TestReaderListZPAServiceEdgesProjectsSDKShapeThroughAllowList(t *testing.T) {
	t.Parallel()

	const nestedCanary = "nested-service-edge-secret-canary"
	reader := SDKReader{
		handlers: map[resourceKey]resourceHandler{
			{product: resources.ProductZPA, name: resourceZPAServiceEdges}: newListGetHandler(
				resourceZPAServiceEdges,
				func(context.Context) ([]zpaserviceedgecontroller.ServiceEdgeController, error) {
					return []zpaserviceedgecontroller.ServiceEdgeController{{
						ID:                  "edge-1",
						Name:                "Service edge",
						Description:         "psk=service-edge-canary-value",
						Enabled:             true,
						ProvisioningKeyName: nestedCanary,
						EnrollmentCert: map[string]interface{}{
							"name": nestedCanary,
						},
						PrivateBrokerVersion: zpaserviceedgecontroller.PrivateBrokerVersion{
							ID:       "broker-1",
							TunnelId: nestedCanary,
						},
					}}, nil
				},
				func(context.Context, string) (*zpaserviceedgecontroller.ServiceEdgeController, error) {
					return nil, nil
				},
				jsonSourceRecord[zpaserviceedgecontroller.ServiceEdgeController],
			),
		},
	}

	records, err := reader.List(context.Background(), resources.ProductZPA, resourceZPAServiceEdges)
	if err != nil {
		t.Fatalf("SDKReader.List(zpa, service-edges) error = %v, want nil", err)
	}
	spec, ok := resources.FindSpec(resources.ProductZPA, resourceZPAServiceEdges)
	if !ok {
		t.Fatalf("FindSpec(zpa, %s) ok = false, want true", resourceZPAServiceEdges)
	}
	projected, reports, err := resources.ProjectRecords(spec, redact.ModeStandard, records)
	if err != nil {
		t.Fatalf("ProjectRecords(zpa service-edges) error = %v, want nil", err)
	}
	gotRecords := projected.Records()
	if len(gotRecords) != 1 {
		t.Fatalf("ProjectRecords(zpa service-edges) records length = %d, want 1", len(gotRecords))
	}
	got := gotRecords[0].Fields()
	if got["id"] != "edge-1" {
		t.Errorf("projected service-edge id = %v, want edge-1", got["id"])
	}
	description, ok := got["description"].(string)
	if !ok || !strings.Contains(description, "<REDACTED:SECRET>") || strings.Contains(description, "service-edge-canary-value") {
		t.Errorf("projected service-edge description = %v, want redacted canary value", got["description"])
	}
	for _, field := range []string{"enrollmentCert", "privateBrokerVersion", "provisioningKeyName"} {
		if _, ok := got[field]; ok {
			t.Errorf("projected service-edge includes %s, want dropped", field)
		}
	}
	if strings.Contains(fmt.Sprint(got), nestedCanary) {
		t.Errorf("projected service-edge = %v, want nested canary absent", got)
	}
	if err := resources.AssertRenderedSubset(spec, redact.ModeStandard, got); err != nil {
		t.Errorf("AssertRenderedSubset(projected service-edge) error = %v, want nil", err)
	}
	if len(reports) != 1 {
		t.Fatalf("ProjectRecords(zpa service-edges) reports length = %d, want 1", len(reports))
	}
	assertReportContains(t, reports[0].DroppedFields, "enrollmentCert")
	assertReportContains(t, reports[0].DroppedFields, "privateBrokerVersion")
	assertReportContains(t, reports[0].DroppedFields, "provisioningKeyName")
	assertReportContains(t, reports[0].RedactedFields, "description")
}

func TestReaderListZPACloudConnectorGroupsProjectsSDKShapeThroughAllowList(t *testing.T) {
	t.Parallel()

	const nestedCanary = "nested-cloud-connector-secret-canary"
	reader := SDKReader{
		handlers: map[resourceKey]resourceHandler{
			{product: resources.ProductZPA, name: resourceZPACloudConnGrps}: newListGetHandler(
				resourceZPACloudConnGrps,
				func(context.Context) ([]zpacloudconnectorgroup.CloudConnectorGroup, error) {
					return []zpacloudconnectorgroup.CloudConnectorGroup{{
						ID:            "ccg-1",
						Name:          "Cloud connector group",
						Description:   "psk=cloud-connector-group-canary-value",
						Enabled:       true,
						GeolocationID: nestedCanary,
						CloudConnectors: []zpacloudconnectorgroup.CloudConnectors{{
							ID:          "connector-1",
							Name:        nestedCanary,
							Fingerprint: nestedCanary,
							SigningCert: map[string]any{
								"name": nestedCanary,
							},
						}},
					}}, nil
				},
				func(context.Context, string) (*zpacloudconnectorgroup.CloudConnectorGroup, error) {
					return nil, nil
				},
				jsonSourceRecord[zpacloudconnectorgroup.CloudConnectorGroup],
			),
		},
	}

	records, err := reader.List(context.Background(), resources.ProductZPA, resourceZPACloudConnGrps)
	if err != nil {
		t.Fatalf("SDKReader.List(zpa, cloud-connector-groups) error = %v, want nil", err)
	}
	spec, ok := resources.FindSpec(resources.ProductZPA, resourceZPACloudConnGrps)
	if !ok {
		t.Fatalf("FindSpec(zpa, %s) ok = false, want true", resourceZPACloudConnGrps)
	}
	projected, reports, err := resources.ProjectRecords(spec, redact.ModeStandard, records)
	if err != nil {
		t.Fatalf("ProjectRecords(zpa cloud-connector-groups) error = %v, want nil", err)
	}
	gotRecords := projected.Records()
	if len(gotRecords) != 1 {
		t.Fatalf("ProjectRecords(zpa cloud-connector-groups) records length = %d, want 1", len(gotRecords))
	}
	got := gotRecords[0].Fields()
	if got["id"] != "ccg-1" {
		t.Errorf("projected cloud-connector-group id = %v, want ccg-1", got["id"])
	}
	description, ok := got["description"].(string)
	if !ok || !strings.Contains(description, "<REDACTED:SECRET>") || strings.Contains(description, "cloud-connector-group-canary-value") {
		t.Errorf("projected cloud-connector-group description = %v, want redacted canary value", got["description"])
	}
	for _, field := range []string{"cloudConnectors", "geoLocationId"} {
		if _, ok := got[field]; ok {
			t.Errorf("projected cloud-connector-group includes %s, want dropped", field)
		}
	}
	if strings.Contains(fmt.Sprint(got), nestedCanary) {
		t.Errorf("projected cloud-connector-group = %v, want nested canary absent", got)
	}
	if err := resources.AssertRenderedSubset(spec, redact.ModeStandard, got); err != nil {
		t.Errorf("AssertRenderedSubset(projected cloud-connector-group) error = %v, want nil", err)
	}
	if len(reports) != 1 {
		t.Fatalf("ProjectRecords(zpa cloud-connector-groups) reports length = %d, want 1", len(reports))
	}
	assertReportContains(t, reports[0].DroppedFields, "cloudConnectors")
	assertReportContains(t, reports[0].DroppedFields, "geoLocationId")
	assertReportContains(t, reports[0].RedactedFields, "description")
}

func TestReaderListZPACloudConnectorsProjectsSDKShapeThroughAllowList(t *testing.T) {
	t.Parallel()

	const nestedCanary = "nested-cloud-connector-secret-canary"
	reader := SDKReader{
		handlers: map[resourceKey]resourceHandler{
			{product: resources.ProductZPA, name: resourceZPACloudConns}: newListGetHandler(
				resourceZPACloudConns,
				func(context.Context) ([]zpacloudconnector.CloudConnector, error) {
					return []zpacloudconnector.CloudConnector{{
						ID:                     "cloud-connector-1",
						Name:                   "Cloud connector",
						Description:            "psk=cloud-connector-canary-value",
						Enabled:                true,
						EdgeConnectorGroupID:   nestedCanary,
						EdgeConnectorGroupName: "Cloud connector group",
						Fingerprint:            nestedCanary,
						IpAcl:                  []string{"198.51.100.10"},
						IssuedCertID:           nestedCanary,
						EnrollmentCert: map[string]any{
							"name": nestedCanary,
						},
					}}, nil
				},
				func(context.Context, string) (*zpacloudconnector.CloudConnector, error) {
					return nil, nil
				},
				jsonSourceRecord[zpacloudconnector.CloudConnector],
			),
		},
	}

	records, err := reader.List(context.Background(), resources.ProductZPA, resourceZPACloudConns)
	if err != nil {
		t.Fatalf("SDKReader.List(zpa, cloud-connectors) error = %v, want nil", err)
	}
	spec, ok := resources.FindSpec(resources.ProductZPA, resourceZPACloudConns)
	if !ok {
		t.Fatalf("FindSpec(zpa, %s) ok = false, want true", resourceZPACloudConns)
	}
	projected, reports, err := resources.ProjectRecords(spec, redact.ModeStandard, records)
	if err != nil {
		t.Fatalf("ProjectRecords(zpa cloud-connectors) error = %v, want nil", err)
	}
	gotRecords := projected.Records()
	if len(gotRecords) != 1 {
		t.Fatalf("ProjectRecords(zpa cloud-connectors) records length = %d, want 1", len(gotRecords))
	}
	got := gotRecords[0].Fields()
	if got["id"] != "cloud-connector-1" {
		t.Errorf("projected cloud-connector id = %v, want cloud-connector-1", got["id"])
	}
	if got["edgeConnectorGroupName"] != "Cloud connector group" {
		t.Errorf("projected cloud-connector edgeConnectorGroupName = %v, want Cloud connector group", got["edgeConnectorGroupName"])
	}
	description, ok := got["description"].(string)
	if !ok || !strings.Contains(description, "<REDACTED:SECRET>") || strings.Contains(description, "cloud-connector-canary-value") {
		t.Errorf("projected cloud-connector description = %v, want redacted canary value", got["description"])
	}
	for _, field := range []string{"edgeConnectorGroupId", "enrollmentCert", "fingerprint", "issuedCertId"} {
		if _, ok := got[field]; ok {
			t.Errorf("projected cloud-connector includes %s, want dropped", field)
		}
	}
	if strings.Contains(fmt.Sprint(got), nestedCanary) {
		t.Errorf("projected cloud-connector = %v, want nested canary absent", got)
	}
	if err := resources.AssertRenderedSubset(spec, redact.ModeStandard, got); err != nil {
		t.Errorf("AssertRenderedSubset(projected cloud-connector) error = %v, want nil", err)
	}
	if len(reports) != 1 {
		t.Fatalf("ProjectRecords(zpa cloud-connectors) reports length = %d, want 1", len(reports))
	}
	assertReportContains(t, reports[0].DroppedFields, "edgeConnectorGroupId")
	assertReportContains(t, reports[0].DroppedFields, "enrollmentCert")
	assertReportContains(t, reports[0].DroppedFields, "fingerprint")
	assertReportContains(t, reports[0].DroppedFields, "issuedCertId")
	assertReportContains(t, reports[0].RedactedFields, "description")
}

func TestReaderListZPAPostureProfilesProjectsSDKShapeThroughAllowList(t *testing.T) {
	t.Parallel()

	const nestedCanary = "nested-posture-profile-secret-canary"
	reader := SDKReader{
		handlers: map[resourceKey]resourceHandler{
			{product: resources.ProductZPA, name: resourceZPAPostureProfs}: newListGetHandler(
				resourceZPAPostureProfs,
				func(context.Context) ([]zpapostureprofile.PostureProfile, error) {
					return []zpapostureprofile.PostureProfile{{
						ID:                             "posture-1",
						Name:                           "Posture profile",
						Domain:                         "example.internal",
						ApplyToMachineTunnelEnabled:    true,
						CRLCheckEnabled:                true,
						NonExportablePrivateKeyEnabled: true,
						PostureType:                    "cert",
						PostureudID:                    nestedCanary,
						RootCert:                       nestedCanary,
						ZscalerCustomerID:              nestedCanary,
					}}, nil
				},
				func(context.Context, string) (*zpapostureprofile.PostureProfile, error) {
					return nil, nil
				},
				jsonSourceRecord[zpapostureprofile.PostureProfile],
			),
		},
	}

	records, err := reader.List(context.Background(), resources.ProductZPA, resourceZPAPostureProfs)
	if err != nil {
		t.Fatalf("SDKReader.List(zpa, posture-profiles) error = %v, want nil", err)
	}
	spec, ok := resources.FindSpec(resources.ProductZPA, resourceZPAPostureProfs)
	if !ok {
		t.Fatalf("FindSpec(zpa, %s) ok = false, want true", resourceZPAPostureProfs)
	}
	projected, reports, err := resources.ProjectRecords(spec, redact.ModeStandard, records)
	if err != nil {
		t.Fatalf("ProjectRecords(zpa posture-profiles) error = %v, want nil", err)
	}
	gotRecords := projected.Records()
	if len(gotRecords) != 1 {
		t.Fatalf("ProjectRecords(zpa posture-profiles) records length = %d, want 1", len(gotRecords))
	}
	got := gotRecords[0].Fields()
	if got["id"] != "posture-1" {
		t.Errorf("projected posture-profile id = %v, want posture-1", got["id"])
	}
	if got["domain"] != "example.internal" {
		t.Errorf("projected posture-profile domain = %v, want example.internal", got["domain"])
	}
	for _, field := range []string{"nonExportablePrivateKeyEnabled", "postureUdid", "rootCert", "zscalerCustomerId"} {
		if _, ok := got[field]; ok {
			t.Errorf("projected posture-profile includes %s, want dropped", field)
		}
	}
	if strings.Contains(fmt.Sprint(got), nestedCanary) {
		t.Errorf("projected posture-profile = %v, want nested canary absent", got)
	}
	if err := resources.AssertRenderedSubset(spec, redact.ModeStandard, got); err != nil {
		t.Errorf("AssertRenderedSubset(projected posture-profile) error = %v, want nil", err)
	}
	if len(reports) != 1 {
		t.Fatalf("ProjectRecords(zpa posture-profiles) reports length = %d, want 1", len(reports))
	}
	assertReportContains(t, reports[0].DroppedFields, "nonExportablePrivateKeyEnabled")
	assertReportContains(t, reports[0].DroppedFields, "postureUdid")
	assertReportContains(t, reports[0].DroppedFields, "rootCert")
	assertReportContains(t, reports[0].DroppedFields, "zscalerCustomerId")
}

func TestReaderListZPACBIZPAProfilesProjectsSDKShapeThroughAllowList(t *testing.T) {
	t.Parallel()

	const nestedCanary = "nested-cbi-zpa-profile-secret-canary"
	reader := SDKReader{
		handlers: map[resourceKey]resourceHandler{
			{product: resources.ProductZPA, name: resourceZPACBIZPAProfs}: newListGetHandler(
				resourceZPACBIZPAProfs,
				func(context.Context) ([]zpacbizpaprofile.ZPAProfiles, error) {
					return []zpacbizpaprofile.ZPAProfiles{{
						ID:           "cbi-zpa-profile-1",
						Name:         "CBI ZPA profile",
						Description:  "psk=cbi-zpa-profile-canary-value",
						Enabled:      true,
						CreationTime: "1700000000000",
						ModifiedBy:   "admin-1",
						ModifiedTime: "1700000100000",
						CBIProfileID: "cbi-profile-1",
						CBITenantID:  nestedCanary,
						CBIURL:       "https://cbi.example.invalid/profile",
					}}, nil
				},
				func(context.Context, string) (*zpacbizpaprofile.ZPAProfiles, error) {
					return nil, nil
				},
				jsonSourceRecord[zpacbizpaprofile.ZPAProfiles],
			),
		},
	}

	records, err := reader.List(context.Background(), resources.ProductZPA, resourceZPACBIZPAProfs)
	if err != nil {
		t.Fatalf("SDKReader.List(zpa, cbi-zpa-profiles) error = %v, want nil", err)
	}
	spec, ok := resources.FindSpec(resources.ProductZPA, resourceZPACBIZPAProfs)
	if !ok {
		t.Fatalf("FindSpec(zpa, %s) ok = false, want true", resourceZPACBIZPAProfs)
	}
	projected, reports, err := resources.ProjectRecords(spec, redact.ModeStandard, records)
	if err != nil {
		t.Fatalf("ProjectRecords(zpa cbi-zpa-profiles) error = %v, want nil", err)
	}
	gotRecords := projected.Records()
	if len(gotRecords) != 1 {
		t.Fatalf("ProjectRecords(zpa cbi-zpa-profiles) records length = %d, want 1", len(gotRecords))
	}
	got := gotRecords[0].Fields()
	if got["id"] != "cbi-zpa-profile-1" {
		t.Errorf("projected cbi-zpa-profile id = %v, want cbi-zpa-profile-1", got["id"])
	}
	if got["cbiProfileId"] != "cbi-profile-1" {
		t.Errorf("projected cbi-zpa-profile cbiProfileId = %v, want cbi-profile-1", got["cbiProfileId"])
	}
	description, ok := got["description"].(string)
	if !ok || !strings.Contains(description, "<REDACTED:SECRET>") || strings.Contains(description, "cbi-zpa-profile-canary-value") {
		t.Errorf("projected cbi-zpa-profile description = %v, want redacted canary value", got["description"])
	}
	if _, ok := got["cbiTenantId"]; ok {
		t.Errorf("projected cbi-zpa-profile includes cbiTenantId, want dropped")
	}
	if strings.Contains(fmt.Sprint(got), nestedCanary) {
		t.Errorf("projected cbi-zpa-profile = %v, want nested canary absent", got)
	}
	if err := resources.AssertRenderedSubset(spec, redact.ModeStandard, got); err != nil {
		t.Errorf("AssertRenderedSubset(projected cbi-zpa-profile) error = %v, want nil", err)
	}
	if len(reports) != 1 {
		t.Fatalf("ProjectRecords(zpa cbi-zpa-profiles) reports length = %d, want 1", len(reports))
	}
	assertReportContains(t, reports[0].DroppedFields, "cbiTenantId")
	assertReportContains(t, reports[0].RedactedFields, "description")
}

func TestReaderListZPAC2CIPRangesProjectsSDKShapeThroughAllowList(t *testing.T) {
	t.Parallel()

	const nestedCanary = "nested-c2c-ip-range-secret-canary"
	reader := SDKReader{
		handlers: map[resourceKey]resourceHandler{
			{product: resources.ProductZPA, name: resourceZPAC2CIPRanges}: newListGetHandler(
				resourceZPAC2CIPRanges,
				func(context.Context) ([]zpac2cipranges.IPRanges, error) {
					return []zpac2cipranges.IPRanges{{
						ID:            "c2c-ip-range-1",
						Name:          "C2C IP range",
						Description:   "psk=c2c-ip-range-canary-value",
						Enabled:       true,
						AvailableIps:  "8",
						CountryCode:   "US",
						CreationTime:  "1700000000000",
						CustomerId:    nestedCanary,
						IpRangeBegin:  "198.51.100.10",
						IpRangeEnd:    "198.51.100.20",
						IsDeleted:     "false",
						LatitudeInDb:  "37.3387",
						Location:      "San Jose, CA",
						LocationHint:  "West",
						LongitudeInDb: "-121.8853",
						ModifiedBy:    "admin-1",
						ModifiedTime:  "1700000100000",
						SccmFlag:      true,
						SubnetCidr:    "198.51.100.0/24",
						TotalIps:      "16",
						UsedIps:       "8",
					}}, nil
				},
				func(context.Context, string) (*zpac2cipranges.IPRanges, error) {
					return nil, nil
				},
				jsonSourceRecord[zpac2cipranges.IPRanges],
			),
		},
	}

	records, err := reader.List(context.Background(), resources.ProductZPA, resourceZPAC2CIPRanges)
	if err != nil {
		t.Fatalf("SDKReader.List(zpa, c2c-ip-ranges) error = %v, want nil", err)
	}
	spec, ok := resources.FindSpec(resources.ProductZPA, resourceZPAC2CIPRanges)
	if !ok {
		t.Fatalf("FindSpec(zpa, %s) ok = false, want true", resourceZPAC2CIPRanges)
	}
	projected, reports, err := resources.ProjectRecords(spec, redact.ModeStandard, records)
	if err != nil {
		t.Fatalf("ProjectRecords(zpa c2c-ip-ranges) error = %v, want nil", err)
	}
	gotRecords := projected.Records()
	if len(gotRecords) != 1 {
		t.Fatalf("ProjectRecords(zpa c2c-ip-ranges) records length = %d, want 1", len(gotRecords))
	}
	got := gotRecords[0].Fields()
	if got["id"] != "c2c-ip-range-1" {
		t.Errorf("projected c2c-ip-range id = %v, want c2c-ip-range-1", got["id"])
	}
	if got["subnetCidr"] != "198.51.100.0/24" {
		t.Errorf("projected c2c-ip-range subnetCidr = %v, want 198.51.100.0/24", got["subnetCidr"])
	}
	if got["availableIps"] != "8" {
		t.Errorf("projected c2c-ip-range availableIps = %v, want 8", got["availableIps"])
	}
	description, ok := got["description"].(string)
	if !ok || !strings.Contains(description, "<REDACTED:SECRET>") || strings.Contains(description, "c2c-ip-range-canary-value") {
		t.Errorf("projected c2c-ip-range description = %v, want redacted canary value", got["description"])
	}
	if _, ok := got["customerId"]; ok {
		t.Errorf("projected c2c-ip-range includes customerId, want dropped")
	}
	if strings.Contains(fmt.Sprint(got), nestedCanary) {
		t.Errorf("projected c2c-ip-range = %v, want nested canary absent", got)
	}
	if err := resources.AssertRenderedSubset(spec, redact.ModeStandard, got); err != nil {
		t.Errorf("AssertRenderedSubset(projected c2c-ip-range) error = %v, want nil", err)
	}
	if len(reports) != 1 {
		t.Fatalf("ProjectRecords(zpa c2c-ip-ranges) reports length = %d, want 1", len(reports))
	}
	assertReportContains(t, reports[0].DroppedFields, "customerId")
	assertReportContains(t, reports[0].RedactedFields, "description")
}

func TestReaderListZPAConfigOverridesProjectsSDKShapeThroughAllowList(t *testing.T) {
	t.Parallel()

	const nestedCanary = "nested-config-override-secret-canary"
	reader := SDKReader{
		handlers: map[resourceKey]resourceHandler{
			{product: resources.ProductZPA, name: resourceZPAConfigOvrds}: newListGetHandler(
				resourceZPAConfigOvrds,
				func(context.Context) ([]zpaconfigoverride.ConfigOverrides, error) {
					return []zpaconfigoverride.ConfigOverrides{{
						BrokerName:     "Broker",
						ConfigKey:      nestedCanary,
						ConfigValue:    "psk=" + nestedCanary,
						ConfigValueInt: "12345",
						CustomerId:     nestedCanary,
						CustomerName:   "Customer",
						Description:    "psk=config-override-canary-value",
						TargetGid:      nestedCanary,
						TargetName:     "Target",
						TargetType:     "BROKER",
					}}, nil
				},
				func(context.Context, string) (*zpaconfigoverride.ConfigOverrides, error) {
					return nil, nil
				},
				jsonSourceRecord[zpaconfigoverride.ConfigOverrides],
			),
		},
	}

	records, err := reader.List(context.Background(), resources.ProductZPA, resourceZPAConfigOvrds)
	if err != nil {
		t.Fatalf("SDKReader.List(zpa, config-overrides) error = %v, want nil", err)
	}
	spec, ok := resources.FindSpec(resources.ProductZPA, resourceZPAConfigOvrds)
	if !ok {
		t.Fatalf("FindSpec(zpa, %s) ok = false, want true", resourceZPAConfigOvrds)
	}
	projected, reports, err := resources.ProjectRecords(spec, redact.ModeStandard, records)
	if err != nil {
		t.Fatalf("ProjectRecords(zpa config-overrides) error = %v, want nil", err)
	}
	gotRecords := projected.Records()
	if len(gotRecords) != 1 {
		t.Fatalf("ProjectRecords(zpa config-overrides) records length = %d, want 1", len(gotRecords))
	}
	got := gotRecords[0].Fields()
	if got["brokerName"] != "Broker" {
		t.Errorf("projected config-overrides brokerName = %v, want Broker", got["brokerName"])
	}
	if got["targetType"] != "BROKER" {
		t.Errorf("projected config-overrides targetType = %v, want BROKER", got["targetType"])
	}
	description, ok := got["description"].(string)
	if !ok || !strings.Contains(description, "<REDACTED:SECRET>") || strings.Contains(description, "config-override-canary-value") {
		t.Errorf("projected config-overrides description = %v, want redacted canary value", got["description"])
	}
	for _, field := range []string{"configKey", "configValue", "configValueInt", "customerId", "targetGid"} {
		if _, ok := got[field]; ok {
			t.Errorf("projected config-overrides includes %s, want dropped", field)
		}
	}
	if strings.Contains(fmt.Sprint(got), nestedCanary) {
		t.Errorf("projected config-overrides = %v, want nested canary absent", got)
	}
	if err := resources.AssertRenderedSubset(spec, redact.ModeStandard, got); err != nil {
		t.Errorf("AssertRenderedSubset(projected config-overrides) error = %v, want nil", err)
	}
	if len(reports) != 1 {
		t.Fatalf("ProjectRecords(zpa config-overrides) reports length = %d, want 1", len(reports))
	}
	assertReportContains(t, reports[0].DroppedFields, "configKey")
	assertReportContains(t, reports[0].DroppedFields, "configValue")
	assertReportContains(t, reports[0].DroppedFields, "configValueInt")
	assertReportContains(t, reports[0].DroppedFields, "customerId")
	assertReportContains(t, reports[0].DroppedFields, "targetGid")
	assertReportContains(t, reports[0].RedactedFields, "description")
}

func TestReaderListZidentityGroupsProjectsSDKShapeThroughAllowList(t *testing.T) {
	t.Parallel()

	const descriptionCanary = "zidentity-group-description-canary"
	reader := SDKReader{
		handlers: map[resourceKey]resourceHandler{
			{product: resources.ProductZidentity, name: resourceZidentityGroups}: newListGetHandler(
				resourceZidentityGroups,
				func(context.Context) ([]zidgroups.Groups, error) {
					return []zidgroups.Groups{{
						ID:                        "group-1",
						Name:                      "Engineering",
						Description:               "temporary psk=" + descriptionCanary,
						Source:                    "SCIM",
						IsDynamicGroup:            true,
						DynamicGroup:              true,
						AdminEntitlementEnabled:   true,
						ServiceEntitlementEnabled: true,
						IDP: &zidcommon.IDNameDisplayName{
							ID:          "idp-1",
							Name:        "Corporate IDP",
							DisplayName: "Corporate IDP display",
						},
					}}, nil
				},
				func(context.Context, string) (*zidgroups.Groups, error) {
					return nil, nil
				},
				zidentityGroupSourceRecord,
			),
		},
	}

	records, err := reader.List(context.Background(), resources.ProductZidentity, resourceZidentityGroups)
	if err != nil {
		t.Fatalf("SDKReader.List(zidentity, groups) error = %v, want nil", err)
	}
	spec, ok := resources.FindSpec(resources.ProductZidentity, resourceZidentityGroups)
	if !ok {
		t.Fatalf("FindSpec(zidentity, %s) ok = false, want true", resourceZidentityGroups)
	}
	projected, reports, err := resources.ProjectRecords(spec, redact.ModeStandard, records)
	if err != nil {
		t.Fatalf("ProjectRecords(zidentity groups, standard) error = %v, want nil", err)
	}
	got := projected.Records()[0].Fields()
	if got["name"] != "Engineering" || got["source"] != "SCIM" || got["adminEntitlementEnabled"] != true {
		t.Errorf("projected zidentity group = %v, want directory fields preserved", got)
	}
	description, ok := got["description"].(string)
	if !ok || !strings.Contains(description, "<REDACTED:SECRET>") || strings.Contains(description, descriptionCanary) {
		t.Errorf("projected zidentity group description = %v, want redacted canary", got["description"])
	}
	idp, ok := got["idp"].(map[string]any)
	if !ok {
		t.Fatalf("projected zidentity group idp = %T, want map[string]any", got["idp"])
	}
	if idp["id"] != "idp-1" || idp["name"] != "Corporate IDP" || idp["displayName"] != "Corporate IDP display" {
		t.Errorf("projected zidentity group idp = %v, want id/name/displayName", idp)
	}
	if err := resources.AssertRenderedSubset(spec, redact.ModeStandard, got); err != nil {
		t.Errorf("AssertRenderedSubset(projected zidentity group standard) error = %v, want nil", err)
	}
	assertReportContains(t, reports[0].RedactedFields, "description")

	shareProjected, shareReports, err := resources.ProjectRecords(spec, redact.ModeShare, records)
	if err != nil {
		t.Fatalf("ProjectRecords(zidentity groups, share) error = %v, want nil", err)
	}
	shareGot := shareProjected.Records()[0].Fields()
	if shareGot["name"] != "Engineering" || shareGot["source"] != "SCIM" {
		t.Errorf("projected zidentity group share = %v, want directory fields preserved", shareGot)
	}
	if _, ok := shareGot["description"]; ok {
		t.Errorf("projected zidentity group share includes description, want dropped")
	}
	if err := resources.AssertRenderedSubset(spec, redact.ModeShare, shareGot); err != nil {
		t.Errorf("AssertRenderedSubset(projected zidentity group share) error = %v, want nil", err)
	}
	assertReportContains(t, shareReports[0].DroppedFields, "description")
}

func TestReaderListZidentityUsersProjectsSDKShapeThroughAllowList(t *testing.T) {
	t.Parallel()

	const customCanary = "zidentity-user-custom-canary"
	reader := SDKReader{
		handlers: map[resourceKey]resourceHandler{
			{product: resources.ProductZidentity, name: resourceZidentityUsers}: newListGetHandler(
				resourceZidentityUsers,
				func(context.Context) ([]zidusers.Users, error) {
					return []zidusers.Users{{
						ID:             "user-1",
						Source:         "SCIM",
						LoginName:      "jane.doe@example.internal",
						DisplayName:    "Jane Doe",
						FirstName:      "Jane",
						LastName:       "Doe",
						PrimaryEmail:   "jane.doe@example.internal",
						SecondaryEmail: "jane.alt@example.internal",
						Status:         true,
						Department: &zidcommon.IDNameDisplayName{
							ID:          "dept-1",
							Name:        "Engineering",
							DisplayName: "Engineering display",
						},
						IDP: &zidcommon.IDNameDisplayName{
							ID:          "idp-1",
							Name:        "Corporate IDP",
							DisplayName: "Corporate IDP display",
						},
						CustomAttrsInfo: map[string]interface{}{
							"employeeCode": customCanary,
						},
					}}, nil
				},
				func(context.Context, string) (*zidusers.Users, error) {
					return nil, nil
				},
				zidentityUserSourceRecord,
			),
		},
	}

	records, err := reader.List(context.Background(), resources.ProductZidentity, resourceZidentityUsers)
	if err != nil {
		t.Fatalf("SDKReader.List(zidentity, users) error = %v, want nil", err)
	}
	spec, ok := resources.FindSpec(resources.ProductZidentity, resourceZidentityUsers)
	if !ok {
		t.Fatalf("FindSpec(zidentity, %s) ok = false, want true", resourceZidentityUsers)
	}
	projected, reports, err := resources.ProjectRecords(spec, redact.ModeStandard, records)
	if err != nil {
		t.Fatalf("ProjectRecords(zidentity users, standard) error = %v, want nil", err)
	}
	got := projected.Records()[0].Fields()
	wantStandardFields := map[string]any{
		"source":         "SCIM",
		"loginName":      "jane.doe@example.internal",
		"displayName":    "Jane Doe",
		"firstName":      "Jane",
		"lastName":       "Doe",
		"primaryEmail":   "jane.doe@example.internal",
		"secondaryEmail": "jane.alt@example.internal",
		"status":         true,
	}
	for field, want := range wantStandardFields {
		if got[field] != want {
			t.Errorf("ProjectRecords(zidentity users, standard) field %s = %v, want %v", field, got[field], want)
		}
	}
	department, ok := got["department"].(map[string]any)
	if !ok {
		t.Fatalf("ProjectRecords(zidentity users, standard) department = %T, want map[string]any", got["department"])
	}
	if department["id"] != "dept-1" || department["name"] != "Engineering" || department["displayName"] != "Engineering display" {
		t.Errorf("ProjectRecords(zidentity users, standard) department = %v, want id/name/displayName", department)
	}
	if _, ok := got["customAttrsInfo"]; ok {
		t.Errorf("ProjectRecords(zidentity users, standard) includes customAttrsInfo, want dropped")
	}
	if strings.Contains(fmt.Sprint(got), customCanary) {
		t.Errorf("ProjectRecords(zidentity users, standard) = %v, want custom canary absent", got)
	}
	if err := resources.AssertRenderedSubset(spec, redact.ModeStandard, got); err != nil {
		t.Errorf("AssertRenderedSubset(projected zidentity user standard) error = %v, want nil", err)
	}
	assertReportContains(t, reports[0].DroppedFields, "customAttrsInfo")

	personalFields := []string{"loginName", "displayName", "firstName", "lastName", "primaryEmail", "secondaryEmail"}
	for _, mode := range []redact.Mode{redact.ModeShare, redact.ModeParanoid} {
		projected, reports, err := resources.ProjectRecords(spec, mode, records)
		if err != nil {
			t.Fatalf("ProjectRecords(zidentity users, %s) error = %v, want nil", mode, err)
		}
		got := projected.Records()[0].Fields()
		for _, field := range personalFields {
			if _, ok := got[field]; ok {
				t.Errorf("ProjectRecords(zidentity users, %s) includes %s, want dropped", mode, field)
			}
			assertReportContains(t, reports[0].DroppedFields, field)
		}
		if _, ok := got["customAttrsInfo"]; ok {
			t.Errorf("ProjectRecords(zidentity users, %s) includes customAttrsInfo, want dropped", mode)
		}
		if strings.Contains(fmt.Sprint(got), "jane.doe@example.internal") || strings.Contains(fmt.Sprint(got), "Jane Doe") {
			t.Errorf("ProjectRecords(zidentity users, %s) = %v, want personal fields absent", mode, got)
		}
		if err := resources.AssertRenderedSubset(spec, mode, got); err != nil {
			t.Errorf("AssertRenderedSubset(projected zidentity user %s) error = %v, want nil", mode, err)
		}
		assertReportContains(t, reports[0].DroppedFields, "customAttrsInfo")
	}
}

func TestReaderListZidentityResourceServersProjectsSDKShapeThroughAllowList(t *testing.T) {
	t.Parallel()

	const (
		descriptionCanary = "resource-server-description-canary-value"
		nestedCanary      = "resource-server-nested-canary"
	)
	reader := SDKReader{
		handlers: map[resourceKey]resourceHandler{
			{product: resources.ProductZidentity, name: resourceZidentityResourceServers}: newListGetHandler(
				resourceZidentityResourceServers,
				func(context.Context) ([]zidresourceservers.ResourceServers, error) {
					return []zidresourceservers.ResourceServers{{
						ID:          "rs-1",
						Name:        "Payroll API",
						DisplayName: "Payroll API display",
						Description: "temporary psk=" + descriptionCanary,
						PrimaryAud:  "api://payroll.internal.example",
						DefaultApi:  true,
						ServiceScopes: []zidresourceservers.ServiceScopes{{
							Service: zidresourceservers.Service{
								ID:          "svc-1",
								Name:        "Identity service",
								DisplayName: "Identity service display",
								CloudName:   "cloud-" + nestedCanary,
								OrgName:     "org-" + nestedCanary,
							},
							Scopes: []zidresourceservers.Scopes{{
								ID:   "scope-1",
								Name: "read:payroll",
							}},
						}},
					}}, nil
				},
				func(context.Context, string) (*zidresourceservers.ResourceServers, error) {
					return nil, nil
				},
				zidentityResourceServerSourceRecord,
			),
		},
	}

	records, err := reader.List(context.Background(), resources.ProductZidentity, resourceZidentityResourceServers)
	if err != nil {
		t.Fatalf("SDKReader.List(zidentity, resource-servers) error = %v, want nil", err)
	}
	spec, ok := resources.FindSpec(resources.ProductZidentity, resourceZidentityResourceServers)
	if !ok {
		t.Fatalf("FindSpec(zidentity, %s) ok = false, want true", resourceZidentityResourceServers)
	}
	projected, reports, err := resources.ProjectRecords(spec, redact.ModeStandard, records)
	if err != nil {
		t.Fatalf("ProjectRecords(zidentity resource-servers, standard) error = %v, want nil", err)
	}
	gotRecords := projected.Records()
	if len(gotRecords) != 1 {
		t.Fatalf("ProjectRecords(zidentity resource-servers, standard) records length = %d, want 1", len(gotRecords))
	}
	got := gotRecords[0].Fields()
	if got["id"] != "rs-1" {
		t.Errorf("projected resource-server id = %v, want rs-1", got["id"])
	}
	description, ok := got["description"].(string)
	if !ok || !strings.Contains(description, "<REDACTED:SECRET>") || strings.Contains(description, descriptionCanary) {
		t.Errorf("projected resource-server description = %v, want redacted canary value", got["description"])
	}
	serviceScopes := mustProjectedList(t, got, "serviceScopes")
	if len(serviceScopes) != 1 {
		t.Fatalf("projected resource-server serviceScopes length = %d, want 1", len(serviceScopes))
	}
	scopeBlock, ok := serviceScopes[0].(map[string]any)
	if !ok {
		t.Fatalf("projected resource-server serviceScopes[0] = %T, want map[string]any", serviceScopes[0])
	}
	service, ok := scopeBlock["service"].(map[string]any)
	if !ok {
		t.Fatalf("projected resource-server serviceScopes[0].service = %T, want map[string]any", scopeBlock["service"])
	}
	if service["id"] != "svc-1" || service["name"] != "Identity service" || service["displayName"] != "Identity service display" {
		t.Errorf("projected resource-server service = %v, want id/name/displayName reference", service)
	}
	for _, field := range []string{"cloudName", "orgName"} {
		if _, ok := service[field]; ok {
			t.Errorf("projected resource-server service includes %s, want dropped", field)
		}
	}
	if strings.Contains(fmt.Sprint(got), nestedCanary) {
		t.Errorf("projected resource-server = %v, want nested canary absent", got)
	}
	if err := resources.AssertRenderedSubset(spec, redact.ModeStandard, got); err != nil {
		t.Errorf("AssertRenderedSubset(projected resource-server standard) error = %v, want nil", err)
	}
	if len(reports) != 1 {
		t.Fatalf("ProjectRecords(zidentity resource-servers) reports length = %d, want 1", len(reports))
	}
	assertReportContains(t, reports[0].RedactedFields, "description")

	shareProjected, _, err := resources.ProjectRecords(spec, redact.ModeShare, records)
	if err != nil {
		t.Fatalf("ProjectRecords(zidentity resource-servers, share) error = %v, want nil", err)
	}
	shareRecords := shareProjected.Records()
	if len(shareRecords) != 1 {
		t.Fatalf("ProjectRecords(zidentity resource-servers, share) records length = %d, want 1", len(shareRecords))
	}
	shareGot := shareRecords[0].Fields()
	for _, field := range []string{"description", "primaryAud", "serviceScopes"} {
		if _, ok := shareGot[field]; ok {
			t.Errorf("share-mode projected resource-server includes %s, want dropped", field)
		}
	}
}

func TestReadAllZidentityPagesRejectsRepeatedPage(t *testing.T) {
	t.Parallel()

	fullPage := make([]string, zidentityPageLimit)
	for i := range fullPage {
		fullPage[i] = fmt.Sprintf("record-%d", i)
	}
	calls := 0
	_, err := readAllZidentityPages(context.Background(), func(_ context.Context, offset, limit int) (zidentityPage[string], error) {
		calls++
		if limit != zidentityPageLimit {
			t.Fatalf("readAllZidentityPages limit = %d, want %d", limit, zidentityPageLimit)
		}
		return zidentityPage[string]{
			records:    fullPage,
			pageOffset: 0,
			nextLink:   fmt.Sprintf("/admin/api/v1/users?offset=%d&limit=%d", offset+len(fullPage), limit),
		}, nil
	})
	if err == nil {
		t.Fatal("readAllZidentityPages repeated page error = nil, want error")
	}
	if !strings.Contains(err.Error(), "pagination did not advance") {
		t.Fatalf("readAllZidentityPages repeated page error = %q, want pagination did not advance", err)
	}
	if calls != 2 {
		t.Fatalf("readAllZidentityPages repeated page calls = %d, want 2", calls)
	}
}

func TestReaderUnsupportedResourceFailsClosed(t *testing.T) {
	t.Parallel()

	reader := &SDKReader{cfg: validReaderConfig()}

	_, err := reader.List(context.Background(), resources.ProductZPA, "applications")
	if !errors.Is(err, ErrUnsupportedResource) {
		t.Fatalf("SDKReader.List(zpa, applications) error = %v, want ErrUnsupportedResource", err)
	}
}

func TestReaderListOnlyHandlerRejectsGet(t *testing.T) {
	t.Parallel()

	const resourceName = "list-only-test"
	reader := &SDKReader{
		cfg: validReaderConfig(),
		handlers: map[resourceKey]resourceHandler{
			{product: resources.ProductZIA, name: resourceName}: newListOnlyHandler(
				resourceName,
				func(context.Context) ([]rulelabels.RuleLabels, error) {
					return []rulelabels.RuleLabels{{
						ID:   10,
						Name: "List only",
					}}, nil
				},
				ruleLabelSourceRecord,
			),
		},
	}

	records, err := reader.List(context.Background(), resources.ProductZIA, resourceName)
	if err != nil {
		t.Fatalf("SDKReader.List(zia, %s) error = %v, want nil", resourceName, err)
	}
	if len(records) != 1 {
		t.Fatalf("SDKReader.List(zia, %s) records length = %d, want 1", resourceName, len(records))
	}
	_, err = reader.Get(context.Background(), resources.ProductZIA, resourceName, "10")
	if !errors.Is(err, ErrUnsupportedResource) {
		t.Fatalf("SDKReader.Get(zia, %s, 10) error = %v, want ErrUnsupportedResource", resourceName, err)
	}
}

func TestReaderSingletonHandlerListsOneRecordAndRejectsGet(t *testing.T) {
	t.Parallel()

	const resourceName = "singleton-test"
	reader := &SDKReader{
		cfg: validReaderConfig(),
		handlers: map[resourceKey]resourceHandler{
			{product: resources.ProductZIA, name: resourceName}: newSingletonHandler(
				resourceName,
				func(context.Context) (*rulelabels.RuleLabels, error) {
					return &rulelabels.RuleLabels{
						ID:   11,
						Name: "Singleton",
					}, nil
				},
				ruleLabelSourceRecord,
			),
		},
	}

	records, err := reader.List(context.Background(), resources.ProductZIA, resourceName)
	if err != nil {
		t.Fatalf("SDKReader.List(zia, %s) error = %v, want nil", resourceName, err)
	}
	if len(records) != 1 {
		t.Fatalf("SDKReader.List(zia, %s) records length = %d, want 1", resourceName, len(records))
	}
	_, err = reader.Get(context.Background(), resources.ProductZIA, resourceName, "11")
	if !errors.Is(err, ErrUnsupportedResource) {
		t.Fatalf("SDKReader.Get(zia, %s, 11) error = %v, want ErrUnsupportedResource", resourceName, err)
	}
}

func TestSDKSessionCloseIsIdempotent(t *testing.T) {
	t.Parallel()

	calls := 0
	session := &SDKSession{
		cleanup: func() {
			calls++
		},
	}

	session.Close()
	session.Close()

	if calls != 1 {
		t.Fatalf("SDKSession.Close cleanup calls = %d, want 1", calls)
	}
}

func TestReaderNormalizesSDKErrors(t *testing.T) {
	t.Parallel()

	const leaked = "sdk-client-secret"
	reader := &SDKReader{
		cfg: validReaderConfig(),
		handlers: map[resourceKey]resourceHandler{
			{product: resources.ProductZIA, name: resourceLocations}: fakeLocationsResourceHandler(fakeZIALocationClient{
				err: errors.New("raw SDK error containing " + leaked),
			}),
		},
	}

	_, err := reader.List(context.Background(), resources.ProductZIA, "locations")
	if !errors.Is(err, ErrLiveAccessFailed) {
		t.Fatalf("SDKReader.List(error) error = %v, want ErrLiveAccessFailed", err)
	}
	if strings.Contains(err.Error(), leaked) {
		t.Errorf("SDKReader.List(error) error = %q, want no leaked SDK error content", err.Error())
	}
}

func TestReaderNormalizedSDKErrorPreservesSafeStatusCodeOnly(t *testing.T) {
	t.Parallel()

	const leaked = "client_secret=raw-sdk-body"
	reader := &SDKReader{
		cfg: validReaderConfig(),
		handlers: map[resourceKey]resourceHandler{
			{product: resources.ProductZIA, name: resourceLocations}: fakeLocationsResourceHandler(fakeZIALocationClient{
				err: &sdkerrorx.ErrorResponse{
					Response: &http.Response{
						StatusCode: http.StatusForbidden,
						Status:     "403 Forbidden",
					},
					Message: leaked,
				},
			}),
		},
	}

	_, err := reader.List(context.Background(), resources.ProductZIA, "locations")
	if !errors.Is(err, ErrLiveAccessFailed) {
		t.Fatalf("SDKReader.List(status error) error = %v, want ErrLiveAccessFailed", err)
	}
	if !strings.Contains(err.Error(), "status 403") {
		t.Errorf("SDKReader.List(status error) error = %q, want safe status code", err.Error())
	}
	if strings.Contains(err.Error(), leaked) {
		t.Errorf("SDKReader.List(status error) error = %q, want no leaked SDK error content", err.Error())
	}
}

func validReaderConfig() ReaderConfig {
	return ReaderConfig{
		ClientID:         secret.New("zscalerctl-client-id"),
		ClientSecret:     secret.New("zscalerctl-client-secret"),
		VanityDomain:     "zscalerctl-vanity",
		ZPACustomerID:    "zscalerctl-zpa-customer-id",
		ZPAMicrotenantID: "zscalerctl-zpa-microtenant-id",
		Timeout:          time.Second,
	}
}

func validLegacyReaderConfig() ReaderConfig {
	return ReaderConfig{
		AuthMode: AuthModeZIALegacy,
		ZIALegacy: ZIALegacyConfig{
			Username: secret.New("zscalerctl-zia-admin@example.invalid"),
			Password: secret.New("zscalerctl-zia-password"),
			APIKey:   secret.New("zscalerctl-zia-api-key"),
			Cloud:    "zscalerthree",
		},
		Timeout: time.Second,
	}
}

func mustFindSpec(t *testing.T, product resources.Product, name string) resources.ResourceSpec {
	t.Helper()
	spec, ok := resources.FindSpec(product, name)
	if !ok {
		t.Fatalf("FindSpec(%s, %s) ok = false, want true", product, name)
	}
	return spec
}

type fakeZIALocationClient struct {
	locations []locationmanagement.Locations
	location  *locationmanagement.Locations
	err       error
}

func (f fakeZIALocationClient) ListLocations(context.Context) ([]locationmanagement.Locations, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.locations, nil
}

func (f fakeZIALocationClient) GetLocation(context.Context, int) (*locationmanagement.Locations, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.location, nil
}

type fakeZIALocationGroupsClient struct {
	groups []locationgroups.LocationGroup
	group  *locationgroups.LocationGroup
	err    error
}

func (f fakeZIALocationGroupsClient) ListLocationGroups(context.Context) ([]locationgroups.LocationGroup, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.groups, nil
}

func (f fakeZIALocationGroupsClient) GetLocationGroup(context.Context, int) (*locationgroups.LocationGroup, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.group, nil
}

type fakeZIARuleLabelsClient struct {
	labels []rulelabels.RuleLabels
	label  *rulelabels.RuleLabels
	err    error
}

func (f fakeZIARuleLabelsClient) ListRuleLabels(context.Context) ([]rulelabels.RuleLabels, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.labels, nil
}

func (f fakeZIARuleLabelsClient) GetRuleLabel(context.Context, int) (*rulelabels.RuleLabels, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.label, nil
}

type fakeZIAStaticIPsClient struct {
	staticIPs []staticips.StaticIP
	staticIP  *staticips.StaticIP
	err       error
}

func (f fakeZIAStaticIPsClient) ListStaticIPs(context.Context) ([]staticips.StaticIP, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.staticIPs, nil
}

func (f fakeZIAStaticIPsClient) GetStaticIP(context.Context, int) (*staticips.StaticIP, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.staticIP, nil
}

type fakeZIAGRETunnelsClient struct {
	tunnels []gretunnels.GreTunnels
	tunnel  *gretunnels.GreTunnels
	err     error
}

func (f fakeZIAGRETunnelsClient) ListGRETunnels(context.Context) ([]gretunnels.GreTunnels, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.tunnels, nil
}

func (f fakeZIAGRETunnelsClient) GetGRETunnel(context.Context, int) (*gretunnels.GreTunnels, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.tunnel, nil
}

func fakeLocationsResourceHandler(client fakeZIALocationClient) resourceHandler {
	return newListGetHandler(
		resourceLocations,
		client.ListLocations,
		intIDGetter(client.GetLocation),
		locationSourceRecord,
	)
}

func fakeLocationGroupsResourceHandler(client fakeZIALocationGroupsClient) resourceHandler {
	return newListGetHandler(
		resourceLocationGroups,
		client.ListLocationGroups,
		intIDGetter(client.GetLocationGroup),
		locationGroupSourceRecord,
	)
}

func fakeRuleLabelsResourceHandler(client fakeZIARuleLabelsClient) resourceHandler {
	return newListGetHandler(
		resourceRuleLabels,
		client.ListRuleLabels,
		intIDGetter(client.GetRuleLabel),
		ruleLabelSourceRecord,
	)
}

func fakeStaticIPsResourceHandler(client fakeZIAStaticIPsClient) resourceHandler {
	return newListGetHandler(
		resourceStaticIPs,
		client.ListStaticIPs,
		intIDGetter(client.GetStaticIP),
		staticIPSourceRecord,
	)
}

func fakeGRETunnelsResourceHandler(client fakeZIAGRETunnelsClient) resourceHandler {
	return newListGetHandler(
		resourceGRETunnels,
		client.ListGRETunnels,
		intIDGetter(client.GetGRETunnel),
		greTunnelSourceRecord,
	)
}

func projectOneRecord(t *testing.T, product resources.Product, name string, records []resources.SourceRecord) map[string]any {
	t.Helper()

	return projectOneRecordInMode(t, product, name, redact.ModeStandard, records)
}

func projectOneRecordInMode(t *testing.T, product resources.Product, name string, mode redact.Mode, records []resources.SourceRecord) map[string]any {
	t.Helper()

	spec, ok := resources.FindSpec(product, name)
	if !ok {
		t.Fatalf("FindSpec(%s, %s) ok = false, want true", product, name)
	}
	projected, _, err := resources.ProjectRecords(spec, mode, records)
	if err != nil {
		t.Fatalf("ProjectRecords(%s %s) error = %v, want nil", product, name, err)
	}
	if len(projected.Records()) != 1 {
		t.Fatalf("ProjectRecords(%s %s) records = %d, want 1", product, name, len(projected.Records()))
	}
	got := projected.Records()[0].Fields()
	if err := resources.AssertRenderedSubset(spec, mode, got); err != nil {
		t.Fatalf("AssertRenderedSubset(projected %s %s SDK shape) error = %v, want nil", product, name, err)
	}
	return got
}

func projectApplicationSegmentMode(
	t *testing.T,
	spec resources.ResourceSpec,
	records []resources.SourceRecord,
	mode redact.Mode,
) map[string]any {
	t.Helper()

	projected, _, err := resources.ProjectRecords(spec, mode, records)
	if err != nil {
		t.Fatalf("ProjectRecords(zpa application-segments, %s) error = %v, want nil", mode, err)
	}
	if len(projected.Records()) != 1 {
		t.Fatalf("ProjectRecords(zpa application-segments, %s) records length = %d, want 1", mode, len(projected.Records()))
	}
	got := projected.Records()[0].Fields()
	if err := resources.AssertRenderedSubset(spec, mode, got); err != nil {
		t.Fatalf("AssertRenderedSubset(projected application-segment %s) error = %v, want nil", mode, err)
	}
	return got
}

func mustProjectedList(t *testing.T, record map[string]any, field string) []any {
	t.Helper()

	value, ok := record[field]
	if !ok {
		t.Fatalf("projected record missing %s, want list", field)
	}
	items, ok := value.([]any)
	if !ok {
		t.Fatalf("projected record %s = %T, want []any", field, value)
	}
	return items
}

func mustFieldSpec(t *testing.T, fields []resources.FieldSpec, name string) resources.FieldSpec {
	t.Helper()

	field, ok := fieldSpecByName(fields, name)
	if !ok {
		t.Fatalf("field specs missing %s", name)
	}
	return field
}

func fieldSpecByName(fields []resources.FieldSpec, name string) (resources.FieldSpec, bool) {
	for _, field := range fields {
		if field.JSONField() == name {
			return field, true
		}
	}
	return resources.FieldSpec{}, false
}

func fieldSpecMap(fields []resources.FieldSpec) map[string]resources.FieldSpec {
	out := make(map[string]resources.FieldSpec, len(fields))
	for _, field := range fields {
		out[field.JSONField()] = field
	}
	return out
}

func assertNoCanaries(t *testing.T, resource string, record map[string]any, canaries ...string) {
	t.Helper()

	body := fmt.Sprint(record)
	for _, canary := range canaries {
		if strings.Contains(body, canary) {
			t.Errorf("projected %s = %#v, want no %q", resource, record, canary)
		}
	}
}

func assertReportContains(t *testing.T, got []string, want string) {
	t.Helper()

	for _, item := range got {
		if item == want {
			return
		}
	}
	t.Errorf("projection report fields = %v, want %q", got, want)
}

func toString(value any) string {
	if value == nil {
		return ""
	}
	text, _ := value.(string)
	return text
}
