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

	"github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/alerts"
	authsettings "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/auth_settings"
	bandwidthclasses "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/bandwidth_control/bandwidth_classes"
	bandwidthcontrolrules "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/bandwidth_control/bandwidth_control_rules"
	cloudappinstances "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/cloud_app_instances"
	ziacommon "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/common"
	"github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/devicegroups"
	dlpicapservers "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/dlp/dlp_icap_servers"
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
	zpacloudconnectorgroup "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/cloud_connector_group"
	zpacbizpaprofile "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/cloudbrowserisolation/cbizpaprofile"
	zpapostureprofile "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/postureprofile"
	zpaservergroup "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/servergroup"
	zpaserviceedgecontroller "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/serviceedgecontroller"
	zpaserviceedgegroup "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/serviceedgegroup"

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

func validReaderConfig() ReaderConfig {
	return ReaderConfig{
		ClientID:     secret.New("zscalerctl-client-id"),
		ClientSecret: secret.New("zscalerctl-client-secret"),
		VanityDomain: "zscalerctl-vanity",
		Timeout:      time.Second,
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

	spec, ok := resources.FindSpec(product, name)
	if !ok {
		t.Fatalf("FindSpec(%s, %s) ok = false, want true", product, name)
	}
	projected, _, err := resources.ProjectRecords(spec, redact.ModeStandard, records)
	if err != nil {
		t.Fatalf("ProjectRecords(%s %s) error = %v, want nil", product, name, err)
	}
	if len(projected.Records()) != 1 {
		t.Fatalf("ProjectRecords(%s %s) records = %d, want 1", product, name, len(projected.Records()))
	}
	got := projected.Records()[0].Fields()
	if err := resources.AssertRenderedSubset(spec, redact.ModeStandard, got); err != nil {
		t.Fatalf("AssertRenderedSubset(projected %s %s SDK shape) error = %v, want nil", product, name, err)
	}
	return got
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
