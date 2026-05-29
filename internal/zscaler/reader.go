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
	ziacommon "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/common"
	"github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/location/locationgroups"
	"github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/location/locationmanagement"
	rulelabels "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/rule_labels"
	"github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/sslinspection"
	gretunnels "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/trafficforwarding/gretunnels"
	staticips "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/trafficforwarding/staticips"
	"github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/urlcategories"

	"github.com/dvmrry/zscalerctl/internal/resources"
	"github.com/dvmrry/zscalerctl/internal/secret"
)

var (
	ErrMissingCredentials  = errors.New("missing zscaler API credentials")
	ErrUnsupportedResource = errors.New("unsupported zscaler resource")
	ErrInvalidResourceID   = errors.New("invalid zscaler resource id")
	ErrLiveAccessFailed    = errors.New("zscaler API request failed")
)

const defaultTimeout = 30 * time.Second

const (
	resourceLocations      = "locations"
	resourceLocationGroups = "location-groups"
	resourceRuleLabels     = "rule-labels"
	resourceStaticIPs      = "static-ips"
	resourceGRETunnels     = "gre-tunnels"
	resourceSublocations   = "sublocations"
	resourceSSLRules       = "ssl-inspection-rules"
	resourceURLCategories  = "url-categories"
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
	Timeout      time.Duration
	NoCache      bool
}

type ZIALegacyConfig struct {
	Username secret.Secret
	Password secret.Secret
	APIKey   secret.Secret
	Cloud    string
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
		Transport: directTransport(),
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
		Transport: directTransport(),
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

func directTransport() http.RoundTripper {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	return transport
}

func validateReaderConfig(cfg ReaderConfig) error {
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

func idNameExtensionsSource(value *ziacommon.IDNameExtensions) map[string]any {
	fields := map[string]any{
		"id":   value.ID,
		"name": value.Name,
	}
	if len(value.Extensions) > 0 {
		fields["extensions"] = value.Extensions
	}
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
		fields := map[string]any{
			"id":   value.ID,
			"name": value.Name,
		}
		if value.Parent != "" {
			fields["parent"] = value.Parent
		}
		out = append(out, fields)
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
		if len(values[i].Extensions) > 0 {
			fields["extensions"] = values[i].Extensions
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
	if len(value.Extensions) > 0 {
		fields["extensions"] = value.Extensions
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
	if len(value.Extensions) > 0 {
		fields["extensions"] = value.Extensions
	}
	return fields
}

func staticIPLastModifiedBySource(value *staticips.LastModifiedBy) map[string]any {
	fields := map[string]any{
		"id":   value.ID,
		"name": value.Name,
	}
	if len(value.Extensions) > 0 {
		fields["extensions"] = value.Extensions
	}
	return fields
}

func greManagedBySource(value *gretunnels.ManagedBy) map[string]any {
	fields := map[string]any{
		"id":   value.ID,
		"name": value.Name,
	}
	if len(value.Extensions) > 0 {
		fields["extensions"] = value.Extensions
	}
	return fields
}

func greLastModifiedBySource(value *gretunnels.LastModifiedBy) map[string]any {
	fields := map[string]any{
		"id":   value.ID,
		"name": value.Name,
	}
	if len(value.Extensions) > 0 {
		fields["extensions"] = value.Extensions
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
