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
	"time"

	sdkcache "github.com/zscaler/zscaler-sdk-go/v3/cache"
	sdklogger "github.com/zscaler/zscaler-sdk-go/v3/logger"
	zsdk "github.com/zscaler/zscaler-sdk-go/v3/zscaler"
	sdkzia "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia"
	ziacommon "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/common"
	"github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/location/locationmanagement"
	rulelabels "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/rule_labels"

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
	resourceLocations  = "locations"
	resourceRuleLabels = "rule-labels"
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

type ziaLocationClient interface {
	ListLocations(context.Context) ([]locationmanagement.Locations, error)
	GetLocation(context.Context, int) (*locationmanagement.Locations, error)
}

type ziaRuleLabelsClient interface {
	ListRuleLabels(context.Context) ([]rulelabels.RuleLabels, error)
	GetRuleLabel(context.Context, int) (*rulelabels.RuleLabels, error)
}

type resourceKey struct {
	product resources.Product
	name    string
}

type resourceHandler interface {
	List(context.Context) ([]resources.SourceRecord, error)
	Get(context.Context, string) (resources.SourceRecord, error)
}

var (
	_ resourceHandler = ziaLocationsHandler{}
	_ resourceHandler = ziaRuleLabelsHandler{}
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
	ziaClient := sdkZIAClient{cfg: cfg}
	return &SDKReader{
		cfg: cfg,
		handlers: map[resourceKey]resourceHandler{
			{product: resources.ProductZIA, name: resourceLocations}: ziaLocationsHandler{
				client: sdkZIALocationClient{sdkZIAClient: ziaClient},
			},
			{product: resources.ProductZIA, name: resourceRuleLabels}: ziaRuleLabelsHandler{
				client: sdkZIARuleLabelsClient{sdkZIAClient: ziaClient},
			},
		},
	}, nil
}

func (r *SDKReader) List(ctx context.Context, product resources.Product, name string) ([]resources.SourceRecord, error) {
	handler, err := r.handler(product, name)
	if err != nil {
		return nil, err
	}
	records, err := handler.List(ctx)
	if err != nil {
		return nil, normalizeLiveError(ctx, "list", product, name)
	}
	return records, nil
}

func (r *SDKReader) Get(ctx context.Context, product resources.Product, name string, id string) (resources.SourceRecord, error) {
	handler, err := r.handler(product, name)
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

func (r *SDKReader) handler(product resources.Product, name string) (resourceHandler, error) {
	if r == nil {
		return nil, fmt.Errorf("%w: %s/%s", ErrUnsupportedResource, product, name)
	}
	handler, ok := r.handlers[resourceKey{product: product, name: name}]
	if !ok {
		return nil, fmt.Errorf("%w: %s/%s", ErrUnsupportedResource, product, name)
	}
	return handler, nil
}

type ziaLocationsHandler struct {
	client ziaLocationClient
}

func (h ziaLocationsHandler) List(ctx context.Context) ([]resources.SourceRecord, error) {
	locations, err := h.client.ListLocations(ctx)
	if err != nil {
		return nil, err
	}
	records := make([]resources.SourceRecord, 0, len(locations))
	for _, location := range locations {
		records = append(records, locationSourceRecord(location))
	}
	return records, nil
}

func (h ziaLocationsHandler) Get(ctx context.Context, id string) (resources.SourceRecord, error) {
	locationID, err := parsePositiveIntID(id)
	if err != nil {
		return resources.SourceRecord{}, err
	}
	location, err := h.client.GetLocation(ctx, locationID)
	if err != nil {
		return resources.SourceRecord{}, err
	}
	if location == nil {
		return resources.SourceRecord{}, errors.New("empty sdk location response")
	}
	return locationSourceRecord(*location), nil
}

type ziaRuleLabelsHandler struct {
	client ziaRuleLabelsClient
}

func (h ziaRuleLabelsHandler) List(ctx context.Context) ([]resources.SourceRecord, error) {
	labels, err := h.client.ListRuleLabels(ctx)
	if err != nil {
		return nil, err
	}
	records := make([]resources.SourceRecord, 0, len(labels))
	for _, label := range labels {
		records = append(records, ruleLabelSourceRecord(label))
	}
	return records, nil
}

func (h ziaRuleLabelsHandler) Get(ctx context.Context, id string) (resources.SourceRecord, error) {
	labelID, err := parsePositiveIntID(id)
	if err != nil {
		return resources.SourceRecord{}, err
	}
	label, err := h.client.GetRuleLabel(ctx, labelID)
	if err != nil {
		return resources.SourceRecord{}, err
	}
	if label == nil {
		return resources.SourceRecord{}, errors.New("empty sdk rule label response")
	}
	return ruleLabelSourceRecord(*label), nil
}

func parsePositiveIntID(id string) (int, error) {
	parsed, err := strconv.Atoi(id)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("%w: %q", ErrInvalidResourceID, id)
	}
	return parsed, nil
}

type sdkZIAClient struct {
	cfg ReaderConfig
}

type sdkZIALocationClient struct {
	sdkZIAClient
}

func (c sdkZIALocationClient) ListLocations(ctx context.Context) ([]locationmanagement.Locations, error) {
	service, cleanup, err := c.service(ctx)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	return locationmanagement.GetAll(ctx, service)
}

func (c sdkZIALocationClient) GetLocation(ctx context.Context, id int) (*locationmanagement.Locations, error) {
	service, cleanup, err := c.service(ctx)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	return locationmanagement.GetLocation(ctx, service, id)
}

type sdkZIARuleLabelsClient struct {
	sdkZIAClient
}

func (c sdkZIARuleLabelsClient) ListRuleLabels(ctx context.Context) ([]rulelabels.RuleLabels, error) {
	service, cleanup, err := c.service(ctx)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	return rulelabels.GetAll(ctx, service)
}

func (c sdkZIARuleLabelsClient) GetRuleLabel(ctx context.Context, id int) (*rulelabels.RuleLabels, error) {
	service, cleanup, err := c.service(ctx)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	return rulelabels.Get(ctx, service, id)
}

func (c sdkZIAClient) service(ctx context.Context) (*zsdk.Service, func(), error) {
	if c.cfg.AuthMode == AuthModeZIALegacy {
		return c.legacyService(ctx)
	}
	cfg := newSDKConfiguration(ctx, c.cfg)
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

func (c sdkZIAClient) legacyService(ctx context.Context) (*zsdk.Service, func(), error) {
	ziaCfg, err := newLegacyZIAConfiguration(ctx, c.cfg)
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
