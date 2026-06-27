// Package runtime assembles trusted read-only execution stacks for adapters.
//
// This package owns config loading, secret resolution, SDK reader construction,
// browser projection, and machine execution wiring. Transport adapters can use
// it without importing the Cobra CLI adapter or duplicating raw runtime setup.
package runtime

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/dvmrry/zscalerctl/internal/browser"
	"github.com/dvmrry/zscalerctl/internal/config"
	"github.com/dvmrry/zscalerctl/internal/machine"
	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
	"github.com/dvmrry/zscalerctl/internal/zscaler"
)

type configLoader func([]string, config.LoadOptions) (config.Config, error)
type readerFactory func(zscaler.ReaderConfig) (browser.RecordReader, error)

// Options configures construction of a read-only machine runtime.
type Options struct {
	Env        []string
	Profile    string
	ConfigPath string

	Timeout time.Duration

	Redaction    redact.Mode
	RedactionSet bool
	NoCache      bool

	Catalog    resources.ResourceCatalog
	DiagLogger *slog.Logger

	loadConfig configLoader
	newReader  readerFactory
}

// Machine is the trusted read-only machine execution facade.
type Machine struct {
	executor  machine.Executor
	catalog   resources.ResourceCatalog
	redaction redact.Mode
}

// NewMachine loads runtime config, resolves credentials, constructs the
// SDK-backed read-only reader, and returns a machine execution facade.
func NewMachine(ctx context.Context, opts Options) (*Machine, error) {
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
	if err := applyOptions(&cfg, opts); err != nil {
		return nil, err
	}
	readerConfig, err := readerConfigFromConfig(ctx, cfg, opts)
	if err != nil {
		return nil, err
	}
	newReader := opts.newReader
	if newReader == nil {
		newReader = func(cfg zscaler.ReaderConfig) (browser.RecordReader, error) {
			return zscaler.NewReader(cfg)
		}
	}
	reader, err := newReader(readerConfig)
	if err != nil {
		return nil, err
	}
	return newMachineFromReader(reader, catalogFromOptions(opts.Catalog), cfg.Defaults.Redaction), nil
}

// Execute runs one machine request through the assembled read-only stack.
func (m *Machine) Execute(ctx context.Context, req machine.Request) (machine.Response, error) {
	if m == nil {
		return machine.Response{}, errors.New("machine runtime is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return m.executor.Execute(ctx, req)
}

// Manifest returns the capability manifest for the runtime catalog.
func (m *Machine) Manifest() machine.Manifest {
	if m == nil {
		return machine.ManifestFromCatalog(nil)
	}
	return machine.ManifestFromCatalog(m.catalog)
}

// Catalog returns a copy of the runtime catalog.
func (m *Machine) Catalog() resources.ResourceCatalog {
	if m == nil {
		return resources.ResourceCatalog{}
	}
	return copyCatalog(m.catalog)
}

// Redaction returns the effective redaction mode used by the runtime.
func (m *Machine) Redaction() redact.Mode {
	if m == nil {
		return redact.ModeStandard
	}
	return redact.EffectiveMode(m.redaction)
}

func applyOptions(cfg *config.Config, opts Options) error {
	if opts.RedactionSet {
		mode, err := redact.ParseMode(string(opts.Redaction))
		if err != nil {
			return fmt.Errorf("%w: %w", config.ErrInvalidConfig, err)
		}
		cfg.Defaults.Redaction = mode
	}
	if opts.NoCache {
		cfg.Defaults.NoCache = true
	}
	if opts.Timeout < 0 {
		return fmt.Errorf("%w: timeout must be positive", config.ErrInvalidConfig)
	}
	return nil
}

func readerConfigFromConfig(
	ctx context.Context,
	cfg config.Config,
	opts Options,
) (zscaler.ReaderConfig, error) {
	clientSecret, err := cfg.Credentials.ClientSecret.Resolve(ctx)
	if err != nil {
		return zscaler.ReaderConfig{}, fmt.Errorf("%w: %w (while resolving the client secret)", zscaler.ErrMissingCredentials, err)
	}
	ziaPassword, err := cfg.ZIALegacy.Password.Resolve(ctx)
	if err != nil {
		return zscaler.ReaderConfig{}, fmt.Errorf("%w: %w (while resolving the ZIA legacy password)", zscaler.ErrMissingCredentials, err)
	}
	ziaAPIKey, err := cfg.ZIALegacy.APIKey.Resolve(ctx)
	if err != nil {
		return zscaler.ReaderConfig{}, fmt.Errorf("%w: %w (while resolving the ZIA legacy API key)", zscaler.ErrMissingCredentials, err)
	}
	return zscaler.ReaderConfig{
		ClientID:         cfg.Credentials.ClientID,
		ClientSecret:     clientSecret,
		VanityDomain:     cfg.VanityDomain,
		Cloud:            cfg.Cloud,
		ZPACustomerID:    cfg.ZPA.CustomerID,
		ZPAMicrotenantID: cfg.ZPA.MicrotenantID,
		AuthMode:         zscaler.AuthMode(cfg.EffectiveAuthMode()),
		ZIALegacy: zscaler.ZIALegacyConfig{
			Username: cfg.ZIALegacy.Username,
			Password: ziaPassword,
			APIKey:   ziaAPIKey,
			Cloud:    cfg.ZIALegacy.Cloud,
		},
		Timeout: opts.Timeout,
		NoCache: cfg.Defaults.NoCache,
		Proxy: zscaler.ProxyConfig{
			URL:             cfg.Proxy.URL,
			FromEnvironment: cfg.Proxy.FromEnvironment,
		},
		DiagLogger: opts.DiagLogger,
	}, nil
}

func newMachineFromReader(
	reader browser.RecordReader,
	catalog resources.ResourceCatalog,
	mode redact.Mode,
) *Machine {
	catalog = copyCatalog(catalog)
	mode = redact.EffectiveMode(mode)
	service := browser.Service{
		Catalog: catalog,
		Reader:  reader,
		Mode:    mode,
	}
	return &Machine{
		executor: machine.Executor{
			Browser:   service,
			Catalog:   catalog,
			Redaction: mode,
		},
		catalog:   catalog,
		redaction: mode,
	}
}

func catalogFromOptions(catalog resources.ResourceCatalog) resources.ResourceCatalog {
	if catalog != nil {
		return copyCatalog(catalog)
	}
	return resources.Catalog()
}

func copyCatalog(catalog resources.ResourceCatalog) resources.ResourceCatalog {
	out := append(resources.ResourceCatalog(nil), catalog...)
	if out == nil {
		return resources.ResourceCatalog{}
	}
	return out
}
