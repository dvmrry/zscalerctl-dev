package runtime

import (
	"context"
	"errors"
	"fmt"

	"github.com/dvmrry/zscalerctl/internal/browser"
	"github.com/dvmrry/zscalerctl/internal/config"
	"github.com/dvmrry/zscalerctl/internal/resources"
	"github.com/dvmrry/zscalerctl/internal/zscaler"
)

const urlLookupCommandName = "url-lookup"

// URLClassification is the runtime-owned URL lookup result shape.
type URLClassification struct {
	URL                          string
	Classifications              []string
	SecurityAlertClassifications []string
	Application                  string
}

// URLLookup is the trusted live ZIA URL lookup facade.
type URLLookup struct {
	reader zscalerURLLookupReader
}

type zscalerURLLookupReader interface {
	URLLookup(context.Context, []string) ([]zscaler.URLClassification, error)
}

// The live SDK reader must keep satisfying the lookup capability.
var _ zscalerURLLookupReader = (*zscaler.SDKReader)(nil)

// NewURLLookup loads runtime config, resolves credentials, constructs the
// SDK-backed read-only reader, and returns a URL lookup facade.
func NewURLLookup(ctx context.Context, opts Options) (*URLLookup, error) {
	if ctx == nil {
		ctx = context.Background()
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
		return nil, err
	}
	return NewURLLookupFromConfig(ctx, cfg, opts)
}

// NewURLLookupFromConfig resolves credentials from an already-loaded effective
// config, constructs the SDK-backed read-only reader, and returns a URL lookup
// facade.
func NewURLLookupFromConfig(
	ctx context.Context,
	cfg config.Config,
	opts Options,
) (*URLLookup, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	reader, err := newReaderFromConfig(ctx, &cfg, opts)
	if err != nil {
		return nil, err
	}
	return NewURLLookupFromReader(reader)
}

// NewURLLookupFromReader constructs a URL lookup facade around an
// already-trusted read-only record reader.
func NewURLLookupFromReader(reader browser.RecordReader) (*URLLookup, error) {
	if reader == nil {
		return nil, browser.ErrMissingReader
	}
	lookupReader, ok := reader.(zscalerURLLookupReader)
	if !ok {
		return nil, unsupportedURLLookupError()
	}
	return &URLLookup{reader: lookupReader}, nil
}

// Lookup resolves the URL category classifications for the given URLs.
func (l *URLLookup) Lookup(ctx context.Context, urls []string) ([]URLClassification, error) {
	if l == nil {
		return nil, errors.New("url lookup runtime is nil")
	}
	if l.reader == nil {
		return nil, unsupportedURLLookupError()
	}
	if ctx == nil {
		ctx = context.Background()
	}
	classifications, err := l.reader.URLLookup(ctx, urls)
	if err != nil {
		return nil, err
	}
	return urlClassificationsFromZscaler(classifications), nil
}

func unsupportedURLLookupError() error {
	return fmt.Errorf("%w: %s/%s", zscaler.ErrUnsupportedResource, resources.ProductZIA, urlLookupCommandName)
}

func urlClassificationsFromZscaler(classifications []zscaler.URLClassification) []URLClassification {
	out := make([]URLClassification, 0, len(classifications))
	for _, classification := range classifications {
		out = append(out, URLClassification{
			URL:                          classification.URL,
			Classifications:              append([]string(nil), classification.Classifications...),
			SecurityAlertClassifications: append([]string(nil), classification.SecurityAlertClassifications...),
			Application:                  classification.Application,
		})
	}
	return out
}
