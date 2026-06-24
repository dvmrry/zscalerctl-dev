// Command zscalerctl-tui is an experimental standalone TUI browser binary.
//
// It is intentionally separate from the normal zscalerctl binary so that
// Bubble Tea (which runs terminal probing at package init) is never linked into
// the main CLI. This binary may import internal/tui/tea and Bubble Tea freely.
//
// Modes:
//
//   - --live: load config, resolve credentials, build a real resource reader, and
//     collect live tenant data before launching the TUI.
//   - --collector-fixture: use the fake-reader-backed collector fixture (default).
//   - --fixture: use the hard-coded static fixture.
//
// Usage:
//
//	go run ./cmd/zscalerctl-tui [--live] [--collector-fixture] [--fixture] [--products <list>] [--resources <list>] [--continue-on-error] [--profile <name>] [--config <path>] [--color auto|always|never] [--format auto|table|pretty|json|ndjson]
//
// The default mode is --collector-fixture. Live mode requires ZSCALERCTL_*
// credentials or a config profile that provides them.
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

	liveFlag := flags.Bool("live", false, "load config, resolve credentials, and collect live tenant data")
	fixtureFlag := flags.Bool("fixture", false, "use the hard-coded fake fixture")
	collectorFixtureFlag := flags.Bool("collector-fixture", false, "use the fake-reader-backed collector fixture")
	productsFlag := flags.String("products", "", "comma-separated list of products to include")
	resourcesFlag := flags.String("resources", "", "comma-separated list of resources to include")
	continueOnErrorFlag := flags.Bool("continue-on-error", false, "continue collecting after a resource error")
	profileFlag := flags.String("profile", "", "config profile name")
	configPathFlag := flags.String("config", "", "config file path")
	colorFlag := flags.String("color", string(output.ColorAuto), "color mode: auto, always, never")
	formatFlag := flags.String("format", string(output.FormatAuto), "output format gate: auto, table, pretty, json, ndjson")

	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments: %v", flags.Args())
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

	collectOpts := collectOptionsFromFlags(*productsFlag, *resourcesFlag, *continueOnErrorFlag)

	var browserData data.BrowserData
	switch {
	case *liveFlag:
		browserData, err = collectLive(ctx, deps, env, *profileFlag, *configPathFlag, collectOpts)
	case *fixtureFlag:
		browserData = data.NewFakeBrowserData()
	default:
		// Default mode: collector fixture.
		browserData, err = collectFixture(ctx, collectOpts)
	}
	if err != nil {
		return err
	}

	style := output.NewStyle(
		output.ShouldColor(colorMode, env, stdoutTTY),
		output.Supports256Color(env),
	)
	if width > 0 {
		style.Width = width
	} else {
		style.Width = 80
	}

	p := deps.newProgram(
		tui_tea.NewBrowserModel(style, browserData),
		tea.WithContext(ctx),
		tea.WithInput(os.Stdin),
		tea.WithOutput(os.Stdout),
	)
	_, err = p.Run()
	return err
}

func collectOptionsFromFlags(productsStr, resourcesStr string, continueOnError bool) browserdata.CollectOptions {
	opts := browserdata.CollectOptions{ContinueOnError: continueOnError}
	if productsStr != "" {
		for _, p := range strings.Split(productsStr, ",") {
			opts.Products = append(opts.Products, resources.Product(strings.TrimSpace(p)))
		}
	}
	if resourcesStr != "" {
		for _, r := range strings.Split(resourcesStr, ",") {
			opts.Resources = append(opts.Resources, strings.TrimSpace(r))
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

func collectLive(
	ctx context.Context,
	deps dependencies,
	env []string,
	profile string,
	configPath string,
	collectOpts browserdata.CollectOptions,
) (data.BrowserData, error) {
	cfg, err := deps.loadConfig(env, config.LoadOptions{Profile: profile, ConfigPath: configPath})
	if err != nil {
		return data.BrowserData{}, err
	}

	authMode := cfg.EffectiveAuthMode()
	var clientSecret, ziaPassword, ziaAPIKey secret.Secret

	switch authMode {
	case config.AuthModeZIALegacy:
		ziaPassword, err = cfg.ZIALegacy.Password.Resolve(ctx)
		if err != nil {
			return data.BrowserData{}, fmt.Errorf("resolve zia password: %w", err)
		}
		ziaAPIKey, err = cfg.ZIALegacy.APIKey.Resolve(ctx)
		if err != nil {
			return data.BrowserData{}, fmt.Errorf("resolve zia api key: %w", err)
		}
	default:
		// OneAPI is the default auth mode.
		clientSecret, err = cfg.Credentials.ClientSecret.Resolve(ctx)
		if err != nil {
			return data.BrowserData{}, fmt.Errorf("resolve client secret: %w", err)
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

	reader, err := deps.newReader(ctx, readerCfg)
	if err != nil {
		return data.BrowserData{}, err
	}

	mode := cfg.Defaults.Redaction
	if mode == "" {
		mode = redact.ModeStandard
	}
	collector := &browserdata.Collector{
		Catalog: resources.Catalog(),
		Reader:  reader,
		Mode:    mode,
	}
	return collector.Collect(ctx, collectOpts)
}
