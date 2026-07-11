package runtime

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"unicode"

	"github.com/dvmrry/zscalerctl/internal/browser"
	"github.com/dvmrry/zscalerctl/internal/config"
	"github.com/dvmrry/zscalerctl/internal/machine"
	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
	"github.com/dvmrry/zscalerctl/internal/zscaler"
)

const urlLookupCommandName = "url-lookup"

// URLLookup is the trusted live ZIA URL lookup facade. It retains only a
// narrow lookup reader and the effective redactor, never config or credentials.
type URLLookup struct {
	reader   zscalerURLLookupReader
	redactor redact.Redactor
}

type zscalerURLLookupReader interface {
	URLLookup(context.Context, []string) ([]zscaler.URLClassification, error)
}

// The live SDK reader must keep satisfying the lookup capability.
var _ zscalerURLLookupReader = (*zscaler.SDKReader)(nil)

// NewURLLookup loads effective config, resolves credentials, and constructs an
// SDK-backed URL lookup facade. Engine.LookupURL should be preferred when raw
// request validation must happen before config access.
func NewURLLookup(ctx context.Context, opts Options) (*URLLookup, error) {
	ctx = nonNilContext(ctx)
	if err := ctx.Err(); err != nil {
		return nil, urlLookupBoundaryError(err)
	}
	env := append([]string(nil), opts.Env...)
	loadConfig := opts.loadConfig
	if loadConfig == nil {
		loadConfig = config.LoadConfig
	}
	cfg, err := loadConfig(env, config.LoadOptions{
		Profile:    opts.Profile,
		ConfigPath: opts.ConfigPath,
	})
	if err != nil {
		return nil, urlLookupBoundaryError(err)
	}
	return NewURLLookupFromConfig(ctx, cfg, opts)
}

// NewURLLookupFromConfig resolves credentials from an already-loaded effective
// config and constructs an SDK-backed URL lookup facade.
func NewURLLookupFromConfig(
	ctx context.Context,
	cfg config.Config,
	opts Options,
) (*URLLookup, error) {
	ctx = nonNilContext(ctx)
	if err := ctx.Err(); err != nil {
		return nil, urlLookupBoundaryError(err)
	}
	reader, err := newReaderFromConfig(ctx, &cfg, opts)
	if err != nil {
		return nil, urlLookupBoundaryError(err)
	}
	return NewURLLookupFromReader(reader, cfg.Defaults.Redaction)
}

// NewURLLookupFromReader constructs a URL lookup facade around an
// already-trusted read-only record reader and an explicit output redaction
// mode. It is intended for trusted adapters and tests that own reader creation.
func NewURLLookupFromReader(reader browser.RecordReader, mode redact.Mode) (*URLLookup, error) {
	if reader == nil {
		return nil, urlLookupBoundaryError(browser.ErrMissingReader)
	}
	lookupReader, ok := reader.(zscalerURLLookupReader)
	if !ok {
		return nil, urlLookupBoundaryError(unsupportedURLLookupError())
	}
	return &URLLookup{reader: lookupReader, redactor: redact.New(mode)}, nil
}

// Lookup validates, normalizes, executes, and sanitizes one typed URL lookup
// request. It performs one synchronous reader call and preserves response order
// and duplicates.
func (l *URLLookup) Lookup(
	ctx context.Context,
	req machine.URLLookupRequest,
) (machine.URLLookupResult, error) {
	if l == nil {
		return machine.URLLookupResult{}, errors.New("url lookup runtime is nil")
	}
	if l.reader == nil {
		return machine.URLLookupResult{}, urlLookupBoundaryError(unsupportedURLLookupError())
	}
	ctx = nonNilContext(ctx)
	urls, err := prepareURLLookupRequest(ctx, req)
	if err != nil {
		return machine.URLLookupResult{}, err
	}
	return l.lookupPrepared(ctx, urls)
}

func (l *URLLookup) lookupPrepared(
	ctx context.Context,
	urls []string,
) (machine.URLLookupResult, error) {
	classifications, err := l.reader.URLLookup(ctx, append([]string(nil), urls...))
	if err != nil {
		return machine.URLLookupResult{}, urlLookupBoundaryError(err)
	}
	if err := ctx.Err(); err != nil {
		return machine.URLLookupResult{}, urlLookupBoundaryError(err)
	}
	sanitized, err := urlClassificationsFromZscaler(l.redactor, classifications)
	if err != nil {
		return machine.URLLookupResult{}, err
	}
	return machine.NewURLLookupResult(sanitized), nil
}

func prepareURLLookupRequest(
	ctx context.Context,
	req machine.URLLookupRequest,
) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, urlLookupBoundaryError(err)
	}
	if len(req.URLs) == 0 {
		return nil, urlLookupUsageError()
	}
	urls := make([]string, len(req.URLs))
	for i, raw := range req.URLs {
		normalized, ok := normalizeLookupURL(raw)
		if !ok {
			return nil, urlLookupUsageError()
		}
		urls[i] = normalized
	}
	return urls, nil
}

func normalizeLookupURL(raw string) (string, bool) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", false
	}
	for _, ch := range value {
		if unicode.IsControl(ch) || unicode.Is(unicode.Cf, ch) {
			return "", false
		}
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Opaque != "" {
		return "", false
	}
	parsed.User = nil
	parsed.RawQuery = ""
	parsed.ForceQuery = false
	parsed.Fragment = ""
	normalized := parsed.String()
	return normalized, strings.TrimSpace(normalized) != ""
}

func urlClassificationsFromZscaler(
	r redact.Redactor,
	classifications []zscaler.URLClassification,
) ([]machine.URLClassification, error) {
	out := make([]machine.URLClassification, len(classifications))
	for i, classification := range classifications {
		normalizedURL, ok := normalizeLookupURL(classification.URL)
		if !ok {
			return nil, invalidURLLookupResponseError()
		}
		out[i] = machine.URLClassification{
			URL:                          sanitizeEngineString(r, normalizedURL),
			Classifications:              sanitizeURLLookupStrings(r, classification.Classifications),
			SecurityAlertClassifications: sanitizeURLLookupStrings(r, classification.SecurityAlertClassifications),
			Application:                  sanitizeEngineString(r, classification.Application),
		}
	}
	return out, nil
}

func sanitizeURLLookupStrings(r redact.Redactor, values []string) []string {
	out := make([]string, len(values))
	for i, value := range values {
		out[i] = sanitizeEngineString(r, value)
	}
	return out
}

func nonNilContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func urlLookupUsageError() error {
	return &machine.MachineError{
		Kind:      machine.ErrorKindUsage,
		Message:   "one or more valid URLs are required",
		Missing:   []string{"urls"},
		Operation: machine.OperationLookup,
		Product:   string(resources.ProductZIA),
		Resource:  urlLookupCommandName,
	}
}

func invalidURLLookupResponseError() error {
	return newBoundaryError(&machine.MachineError{
		Kind:      machine.ErrorKindLiveAccessFailed,
		Message:   "lookup zia/url-lookup returned an invalid response",
		Operation: machine.OperationLookup,
		Product:   string(resources.ProductZIA),
		Resource:  urlLookupCommandName,
	}, zscaler.ErrLiveAccessFailed)
}

func urlLookupBoundaryError(err error) error {
	if err == nil {
		return nil
	}
	machineErr := &machine.MachineError{}
	sentinel := error(nil)
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		setURLLookupErrorContext(machineErr)
		machineErr.Kind = machine.ErrorKindDeadlineExceeded
		machineErr.Message = "request deadline exceeded"
		sentinel = context.DeadlineExceeded
	case errors.Is(err, context.Canceled):
		setURLLookupErrorContext(machineErr)
		machineErr.Kind = machine.ErrorKindCanceled
		machineErr.Message = "request canceled"
		sentinel = context.Canceled
	case errors.Is(err, config.ErrInvalidConfig):
		machineErr.Kind = machine.ErrorKindInvalidConfig
		machineErr.Message = "invalid configuration"
		sentinel = config.ErrInvalidConfig
	case errors.Is(err, zscaler.ErrInvalidProxyConfig):
		machineErr.Kind = machine.ErrorKindInvalidProxyConfig
		machineErr.Message = "invalid proxy configuration"
		sentinel = zscaler.ErrInvalidProxyConfig
	case errors.Is(err, zscaler.ErrMissingCredentials):
		machineErr.Kind = machine.ErrorKindMissingCredentials
		machineErr.Message, sentinel = sanitizedMissingCredentials(err)
		var missingErr *zscaler.MissingCredentialsError
		if errors.As(sentinel, &missingErr) {
			machineErr.Missing = append([]string(nil), missingErr.Missing...)
		}
	case errors.Is(err, zscaler.ErrUnsupportedResource):
		machineErr.Kind = machine.ErrorKindUnsupportedResource
		machineErr.Message = "unsupported zscaler resource: zia/url-lookup"
		sentinel = zscaler.ErrUnsupportedResource
	case errors.Is(err, zscaler.ErrLiveAccessFailed):
		setURLLookupErrorContext(machineErr)
		machineErr.Kind = machine.ErrorKindLiveAccessFailed
		machineErr.Message = "lookup zia/url-lookup failed"
		sentinel = zscaler.ErrLiveAccessFailed
	case errors.Is(err, browser.ErrMissingReader):
		setURLLookupErrorContext(machineErr)
		machineErr.Kind = machine.ErrorKindInternal
		machineErr.Message = "URL lookup reader is not configured"
		sentinel = browser.ErrMissingReader
	default:
		setURLLookupErrorContext(machineErr)
		machineErr.Kind = machine.ErrorKindInternal
		machineErr.Message = "URL lookup runtime failed"
	}
	return newBoundaryError(machineErr, sentinel)
}

func setURLLookupErrorContext(machineErr *machine.MachineError) {
	machineErr.Operation = machine.OperationLookup
	machineErr.Product = string(resources.ProductZIA)
	machineErr.Resource = urlLookupCommandName
}

func sanitizedMissingCredentials(err error) (string, error) {
	var missingErr *zscaler.MissingCredentialsError
	if !errors.As(err, &missingErr) {
		return zscaler.ErrMissingCredentials.Error(), zscaler.ErrMissingCredentials
	}
	missing := make([]string, 0, len(missingErr.Missing))
	seen := map[string]bool{}
	for _, name := range missingErr.Missing {
		if !isKnownCredentialName(name) || seen[name] {
			continue
		}
		seen[name] = true
		missing = append(missing, name)
	}
	if len(missing) == 0 {
		return zscaler.ErrMissingCredentials.Error(), zscaler.ErrMissingCredentials
	}
	safeErr := &zscaler.MissingCredentialsError{Missing: missing}
	return safeErr.Error(), safeErr
}

func isKnownCredentialName(name string) bool {
	switch name {
	case config.EnvClientID,
		config.EnvClientSecret,
		config.EnvClientSecretFile,
		config.EnvVanityDomain,
		config.EnvCloud,
		config.EnvZIAUsername,
		config.EnvZIAPassword,
		config.EnvZIAPasswordFile,
		config.EnvZIAAPIKey,
		config.EnvZIAAPIKeyFile,
		config.EnvZIACloud:
		return true
	default:
		return false
	}
}

func unsupportedURLLookupError() error {
	return fmt.Errorf("%w: %s/%s", zscaler.ErrUnsupportedResource, resources.ProductZIA, urlLookupCommandName)
}
