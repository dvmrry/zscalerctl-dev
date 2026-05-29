package zscaler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	ziacommon "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/common"
	"github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/location/locationgroups"
	"github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/location/locationmanagement"
	rulelabels "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/rule_labels"
	"github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/sslinspection"
	gretunnels "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/trafficforwarding/gretunnels"
	staticips "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/trafficforwarding/staticips"
	"github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/urlcategories"

	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
	"github.com/dvmrry/zscalerctl/internal/secret"
)

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

func TestReaderUnsupportedResourceFailsClosed(t *testing.T) {
	t.Parallel()

	reader := &SDKReader{cfg: validReaderConfig()}

	_, err := reader.List(context.Background(), resources.ProductZPA, "applications")
	if !errors.Is(err, ErrUnsupportedResource) {
		t.Fatalf("SDKReader.List(zpa, applications) error = %v, want ErrUnsupportedResource", err)
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

func toString(value any) string {
	if value == nil {
		return ""
	}
	text, _ := value.(string)
	return text
}
