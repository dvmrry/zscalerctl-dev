// Command zscalerctl-tui is an experimental standalone TUI browser binary.
//
// It is intentionally separate from the normal zscalerctl binary so that
// Bubble Tea (which runs terminal probing at package init) is never linked into
// the main CLI. This binary may import internal/tui/tea and Bubble Tea freely.
//
// Modes:
//
//   - --live: load config, resolve credentials, build a real resource reader, and
//     launch with a catalog. Records are loaded on demand when selected.
//   - --collector-fixture: use the fake-reader-backed collector fixture (default).
//   - --fixture: use the hard-coded static fixture.
//
// Usage:
//
//	go run ./cmd/zscalerctl-tui [--live] [--collector-fixture] [--fixture] [--products <list>] [--resources <list>] [--continue-on-error] [--profile <name>] [--config <path>] [--timeout 30s] [--verbose] [--color auto|always|never] [--format auto|table|pretty|json|ndjson]
//
// The default mode is --collector-fixture. Live mode requires ZSCALERCTL_*
// credentials or a config profile that provides them. --products and
// --resources limit the visible live catalog; they do not preload records.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dvmrry/zscalerctl/internal/config"
	"github.com/dvmrry/zscalerctl/internal/output"
	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
	"github.com/dvmrry/zscalerctl/internal/secret"
	"github.com/dvmrry/zscalerctl/internal/tui"
	"github.com/dvmrry/zscalerctl/internal/tui/browserdata"
	"github.com/dvmrry/zscalerctl/internal/tui/data"
	tui_tea "github.com/dvmrry/zscalerctl/internal/tui/tea"
	"github.com/dvmrry/zscalerctl/internal/zscaler"
)

func main() {
	if err := run(
		context.Background(),
		defaultDependencies(),
		os.Args[1:],
		os.Environ(),
		output.IsTerminal(os.Stdin),
		output.IsTerminal(os.Stdout),
		output.IsTerminal(os.Stderr),
		output.TerminalWidth(os.Stdout),
	); err != nil {
		fmt.Fprintf(os.Stderr, "zscalerctl-tui: %v\n", err)
		os.Exit(2)
	}
}

// programRunner is the small interface the binary needs from a Bubble Tea
// program. It keeps the dependency injection surface narrow and testable.
type programRunner interface {
	Run() (tea.Model, error)
}

// dependencies are the externally observable seams used by run. Tests replace
// these with fakes so gate/config/credential/reader/collector failures and
// success can be asserted without a real TTY or Zscaler tenant.
type dependencies struct {
	gateChecker func(opts tui.Options) tui.Eligibility
	loadConfig  func(environ []string, opts config.LoadOptions) (config.Config, error)
	newReader   func(ctx context.Context, cfg zscaler.ReaderConfig) (browserdata.RecordReader, error)
	newProgram  func(model tea.Model, opts ...tea.ProgramOption) programRunner
	// verboseLog is an optional test override for verbose diagnostic output.
	// When nil, --verbose writes to os.Stderr.
	verboseLog func(format string, args ...any)
}

func defaultDependencies() dependencies {
	return dependencies{
		gateChecker: tui.Evaluate,
		loadConfig:  config.LoadConfig,
		newReader: func(ctx context.Context, cfg zscaler.ReaderConfig) (browserdata.RecordReader, error) {
			return zscaler.NewReader(cfg)
		},
		newProgram: func(model tea.Model, opts ...tea.ProgramOption) programRunner {
			return tea.NewProgram(model, opts...)
		},
	}
}

func run(ctx context.Context, deps dependencies, args []string, env []string, stdinTTY, stdoutTTY, stderrTTY bool, width int) error {
	flags := flag.NewFlagSet("zscalerctl-tui", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)

	liveFlag := flags.Bool("live", false, "load config, resolve credentials, and lazily browse live tenant data")
	fixtureFlag := flags.Bool("fixture", false, "use the hard-coded fake fixture")
	collectorFixtureFlag := flags.Bool("collector-fixture", false, "use the fake-reader-backed collector fixture")
	productsFlag := flags.String("products", "", "comma-separated list of products to include")
	resourcesFlag := flags.String("resources", "", "comma-separated list of resources to include")
	continueOnErrorFlag := flags.Bool("continue-on-error", false, "continue collecting after a resource error")
	profileFlag := flags.String("profile", "", "config profile name")
	configPathFlag := flags.String("config", "", "config file path")
	timeoutFlag := flags.Duration("timeout", 30*time.Second, "timeout for each live resource load (e.g. 30s, 2m)")
	verboseFlag := flags.Bool("verbose", false, "print pre-launch diagnostics to stderr")
	colorFlag := flags.String("color", string(output.ColorAuto), "color mode: auto, always, never")
	formatFlag := flags.String("format", string(output.FormatAuto), "output format gate: auto, table, pretty, json, ndjson")

	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments: %v", flags.Args())
	}

	logVerbose := func(format string, args ...any) {}
	if *verboseFlag {
		if deps.verboseLog != nil {
			logVerbose = deps.verboseLog
		} else {
			logVerbose = func(format string, args ...any) {
				fmt.Fprintf(os.Stderr, "zscalerctl-tui: "+format+"\n", args...)
			}
		}
	}

	modeCount := 0
	if *liveFlag {
		modeCount++
	}
	if *fixtureFlag {
		modeCount++
	}
	if *collectorFixtureFlag {
		modeCount++
	}
	if modeCount > 1 {
		return fmt.Errorf("--live, --fixture, and --collector-fixture are mutually exclusive")
	}

	colorMode, err := output.ParseColorMode(*colorFlag)
	if err != nil {
		return err
	}
	format, err := output.ParseFormat(*formatFlag)
	if err != nil {
		return err
	}

	logVerbose("checking terminal eligibility")
	eligibility := deps.gateChecker(tui.Options{
		Requested:  true,
		StdinTTY:   stdinTTY,
		StdoutTTY:  stdoutTTY,
		StderrTTY:  stderrTTY,
		Format:     format,
		ColorMode:  colorMode,
		OutputPath: "",
		Env:        env,
	})
	if !eligibility.Enabled {
		return fmt.Errorf("disabled: %s", eligibility.Reason)
	}
	logVerbose("terminal eligibility passed")

	collectOpts := collectOptionsFromFlags(*productsFlag, *resourcesFlag, *continueOnErrorFlag)

	var browserData data.BrowserData
	var resourceLoader tui_tea.ResourceLoader
	switch {
	case *liveFlag:
		logVerbose("loading config profile %q", *profileFlag)
		browserData, resourceLoader, err = prepareLive(ctx, deps, env, *profileFlag, *configPathFlag, collectOpts, logVerbose)
	case *fixtureFlag:
		browserData = data.NewFakeBrowserData()
	default:
		// Default mode: collector fixture.
		browserData, err = collectFixture(ctx, collectOpts)
	}
	if err != nil {
		return err
	}

	logVerbose("launching TUI")
	style := output.NewStyle(
		output.ShouldColor(colorMode, env, stdoutTTY),
		output.Supports256Color(env),
	)
	if width > 0 {
		style.Width = width
	} else {
		style.Width = 80
	}

	var model tea.Model
	if resourceLoader != nil {
		model = tui_tea.NewLazyBrowserModel(style, browserData, resourceLoader, *timeoutFlag)
	} else {
		model = tui_tea.NewBrowserModel(style, browserData)
	}

	p := deps.newProgram(model, tea.WithContext(ctx), tea.WithInput(os.Stdin), tea.WithOutput(os.Stdout))
	_, err = p.Run()
	return err
}

func collectOptionsFromFlags(productsStr, resourcesStr string, continueOnError bool) browserdata.CollectOptions {
	opts := browserdata.CollectOptions{ContinueOnError: continueOnError}
	if productsStr != "" {
		for _, p := range strings.Split(productsStr, ",") {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			opts.Products = append(opts.Products, resources.Product(p))
		}
	}
	if resourcesStr != "" {
		for _, r := range strings.Split(resourcesStr, ",") {
			r = strings.TrimSpace(r)
			if r == "" {
				continue
			}
			opts.Resources = append(opts.Resources, r)
		}
	}
	return opts
}

func collectFixture(ctx context.Context, opts browserdata.CollectOptions) (data.BrowserData, error) {
	collector := browserdata.NewCollectorFixture()
	// The fixture reader intentionally omits some resources; continue on error so
	// the demo still shows the resources that are present.
	fixtureOpts := opts
	fixtureOpts.ContinueOnError = true
	return collector.Collect(ctx, fixtureOpts)
}

func prepareLive(
	ctx context.Context,
	deps dependencies,
	env []string,
	profile string,
	configPath string,
	collectOpts browserdata.CollectOptions,
	logVerbose func(format string, args ...any),
) (data.BrowserData, tui_tea.ResourceLoader, error) {
	cfg, err := deps.loadConfig(env, config.LoadOptions{Profile: profile, ConfigPath: configPath})
	if err != nil {
		return data.BrowserData{}, nil, err
	}

	authMode := cfg.EffectiveAuthMode()
	logVerbose("resolved auth mode %q", authMode)
	logVerbose("resolving credentials for auth mode %q", authMode)
	var clientSecret, ziaPassword, ziaAPIKey secret.Secret

	switch authMode {
	case config.AuthModeZIALegacy:
		ziaPassword, err = cfg.ZIALegacy.Password.Resolve(ctx)
		if err != nil {
			return data.BrowserData{}, nil, fmt.Errorf("resolve zia password: %w", err)
		}
		ziaAPIKey, err = cfg.ZIALegacy.APIKey.Resolve(ctx)
		if err != nil {
			return data.BrowserData{}, nil, fmt.Errorf("resolve zia api key: %w", err)
		}
	default:
		// OneAPI is the default auth mode.
		clientSecret, err = cfg.Credentials.ClientSecret.Resolve(ctx)
		if err != nil {
			return data.BrowserData{}, nil, fmt.Errorf("resolve client secret: %w", err)
		}
	}

	readerCfg := zscaler.ReaderConfig{
		ClientID:         cfg.Credentials.ClientID,
		ClientSecret:     clientSecret,
		VanityDomain:     cfg.VanityDomain,
		Cloud:            cfg.Cloud,
		ZPACustomerID:    cfg.ZPA.CustomerID,
		ZPAMicrotenantID: cfg.ZPA.MicrotenantID,
		AuthMode:         zscaler.AuthMode(authMode),
		ZIALegacy: zscaler.ZIALegacyConfig{
			Username: cfg.ZIALegacy.Username,
			Password: ziaPassword,
			APIKey:   ziaAPIKey,
			Cloud:    cfg.ZIALegacy.Cloud,
		},
		Timeout:    30 * time.Second,
		NoCache:    cfg.Defaults.NoCache,
		DiagLogger: nil,
		Proxy: zscaler.ProxyConfig{
			URL:             cfg.Proxy.URL,
			FromEnvironment: cfg.Proxy.FromEnvironment,
		},
	}

	logVerbose("building reader")
	reader, err := deps.newReader(ctx, readerCfg)
	if err != nil {
		return data.BrowserData{}, nil, err
	}
	logVerbose("reader ready")

	mode := cfg.Defaults.Redaction
	if mode == "" {
		mode = redact.ModeStandard
	}
	catalog := browserdata.CatalogForOptions(resources.Catalog(), collectOpts)
	collector := &browserdata.Collector{
		Catalog: catalog,
		Reader:  reader,
		Mode:    mode,
	}

	resourcesDesc := "all visible resources"
	if len(collectOpts.Resources) > 0 {
		resourcesDesc = strings.Join(collectOpts.Resources, ", ")
	}
	logVerbose("building unloaded catalog for %s", resourcesDesc)
	browserData := browserdata.BuildUnloadedCatalog(catalog)
	logVerbose("prepared %d products for lazy loading", len(browserData.Products))
	return browserData, collector, nil
}
