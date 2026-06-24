package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dvmrry/zscalerctl/internal/config"
	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
	"github.com/dvmrry/zscalerctl/internal/secret"
	"github.com/dvmrry/zscalerctl/internal/secretref"
	"github.com/dvmrry/zscalerctl/internal/tui"
	"github.com/dvmrry/zscalerctl/internal/tui/browserdata"
	"github.com/dvmrry/zscalerctl/internal/tui/data"
	"github.com/dvmrry/zscalerctl/internal/zscaler"
)

// fakeSecretSource is a test SecretSource that resolves to a fixed secret or error.
type fakeSecretSource struct {
	scheme string
	sec    secret.Secret
	err    error
}

func (f fakeSecretSource) Scheme() string                                 { return f.scheme }
func (f fakeSecretSource) IsConfigured() bool                             { return f.scheme != "" }
func (f fakeSecretSource) Resolve(context.Context) (secret.Secret, error) { return f.sec, f.err }

func oneAPIConfig() config.Config {
	return config.Config{
		Credentials: config.Credentials{
			ClientID:     secret.New("client-id"),
			ClientSecret: fakeSecretSource{scheme: "fake", sec: secret.New("client-secret")},
		},
		VanityDomain: "example",
		Cloud:        "zscalerthree",
		ZIALegacy: config.ZIALegacyCredentials{
			Password: secretref.Unset(),
			APIKey:   secretref.Unset(),
		},
		Defaults: config.Defaults{
			Redaction: redact.ModeStandard,
		},
	}
}

func legacyConfig() config.Config {
	return config.Config{
		Credentials: config.Credentials{
			ClientSecret: secretref.Unset(),
		},
		ZIALegacy: config.ZIALegacyCredentials{
			Username: secret.New("user@example"),
			Password: fakeSecretSource{scheme: "fake", sec: secret.New("password")},
			APIKey:   fakeSecretSource{scheme: "fake", sec: secret.New("api-key")},
			Cloud:    "zscalerthree",
		},
		Defaults: config.Defaults{
			Redaction: redact.ModeStandard,
		},
	}
}

// panicSecretSource is a SecretSource that fails the test if Resolve is called.
// It is used to prove that credentials not required by the selected auth mode
// are never resolved.
type panicSecretSource struct{}

func (panicSecretSource) Scheme() string     { return "panic" }
func (panicSecretSource) IsConfigured() bool { return false }
func (panicSecretSource) Resolve(context.Context) (secret.Secret, error) {
	panic("unused secret was resolved")
}

func oneAPIConfigWithoutLegacySecrets() config.Config {
	cfg := oneAPIConfig()
	cfg.ZIALegacy = config.ZIALegacyCredentials{
		Password: panicSecretSource{},
		APIKey:   panicSecretSource{},
	}
	return cfg
}

func legacyConfigWithoutClientSecret() config.Config {
	cfg := legacyConfig()
	cfg.Credentials.ClientSecret = panicSecretSource{}
	cfg.Credentials.ClientID = secret.Secret{}
	cfg.VanityDomain = ""
	cfg.Cloud = ""
	return cfg
}

// recordingReader implements browserdata.RecordReader and records every call.
type recordingReader struct {
	records map[string][]resources.SourceRecord
	errors  map[string]error
	calls   []readerCall
}

type readerCall struct {
	op       string
	product  string
	resource string
}

func (r *recordingReader) List(ctx context.Context, product resources.Product, resource string) ([]resources.SourceRecord, error) {
	key := fmt.Sprintf("%s/%s", product, resource)
	r.calls = append(r.calls, readerCall{op: "List", product: string(product), resource: resource})
	if err := r.errors[key]; err != nil {
		return nil, err
	}
	return r.records[key], nil
}

func (r *recordingReader) Show(ctx context.Context, product resources.Product, resource string) (resources.SourceRecord, error) {
	key := fmt.Sprintf("%s/%s", product, resource)
	r.calls = append(r.calls, readerCall{op: "Show", product: string(product), resource: resource})
	if err := r.errors[key]; err != nil {
		return resources.SourceRecord{}, err
	}
	recs := r.records[key]
	if len(recs) == 0 {
		return resources.SourceRecord{}, errors.New("singleton not found")
	}
	return recs[0], nil
}

func newRecordingReader() *recordingReader {
	return &recordingReader{
		records: make(map[string][]resources.SourceRecord),
		errors:  make(map[string]error),
	}
}

// fakeProgram implements programRunner and records that it was run.
type fakeProgram struct {
	model  tea.Model
	opts   []tea.ProgramOption
	called bool
	runErr error
}

func (p *fakeProgram) Run() (tea.Model, error) {
	p.called = true
	return p.model, p.runErr
}

func fakeProgramFactory(p *fakeProgram) func(model tea.Model, opts ...tea.ProgramOption) programRunner {
	return func(model tea.Model, opts ...tea.ProgramOption) programRunner {
		p.model = model
		p.opts = opts
		return p
	}
}

// alwaysEnabledGate returns an enabled eligibility decision, letting tests
// bypass the real TTY gate so they can focus on later failure points.
func alwaysEnabledGate(tui.Options) tui.Eligibility {
	return tui.Eligibility{Enabled: true}
}

func alwaysDisabledGate(reason string) func(tui.Options) tui.Eligibility {
	return func(tui.Options) tui.Eligibility {
		return tui.Eligibility{Enabled: false, Reason: reason}
	}
}

func runTest(t *testing.T, deps dependencies, args []string) error {
	t.Helper()
	return run(context.Background(), deps, args, nil, true, true, true, 80)
}

func TestGateFailureNoProgram(t *testing.T) {
	prog := &fakeProgram{}
	deps := dependencies{
		gateChecker: alwaysDisabledGate("stdin is not interactive"),
		loadConfig:  config.LoadConfig,
		newReader:   defaultDependencies().newReader,
		newProgram:  fakeProgramFactory(prog),
	}

	err := runTest(t, deps, []string{"--live"})
	if err == nil {
		t.Fatal("expected error for disabled gate, got nil")
	}
	if prog.called {
		t.Error("program should not be launched when gate disables TUI")
	}
}

func TestConfigFailureNoProgram(t *testing.T) {
	prog := &fakeProgram{}
	deps := dependencies{
		gateChecker: alwaysEnabledGate,
		loadConfig: func([]string, config.LoadOptions) (config.Config, error) {
			return config.Config{}, errors.New("config file not found")
		},
		newReader:  defaultDependencies().newReader,
		newProgram: fakeProgramFactory(prog),
	}

	err := runTest(t, deps, []string{"--live"})
	if err == nil || !errors.Is(err, errors.New("config file not found")) && err.Error() != "config file not found" {
		t.Fatalf("expected config error, got %v", err)
	}
	if prog.called {
		t.Error("program should not be launched when config fails to load")
	}
}

func TestCredentialFailureNoProgram(t *testing.T) {
	prog := &fakeProgram{}
	cfg := oneAPIConfig()
	cfg.Credentials.ClientSecret = fakeSecretSource{scheme: "fake", err: errors.New("missing client secret")}

	deps := dependencies{
		gateChecker: alwaysEnabledGate,
		loadConfig: func([]string, config.LoadOptions) (config.Config, error) {
			return cfg, nil
		},
		newReader:  defaultDependencies().newReader,
		newProgram: fakeProgramFactory(prog),
	}

	err := runTest(t, deps, []string{"--live"})
	if err == nil {
		t.Fatal("expected credential error, got nil")
	}
	if prog.called {
		t.Error("program should not be launched when credential resolution fails")
	}
}

func TestReaderFailureNoProgram(t *testing.T) {
	prog := &fakeProgram{}
	deps := dependencies{
		gateChecker: alwaysEnabledGate,
		loadConfig: func([]string, config.LoadOptions) (config.Config, error) {
			return oneAPIConfig(), nil
		},
		newReader: func(context.Context, zscaler.ReaderConfig) (browserdata.RecordReader, error) {
			return nil, errors.New("reader creation failed")
		},
		newProgram: fakeProgramFactory(prog),
	}

	err := runTest(t, deps, []string{"--live"})
	if err == nil || err.Error() != "reader creation failed" {
		t.Fatalf("expected reader error, got %v", err)
	}
	if prog.called {
		t.Error("program should not be launched when reader creation fails")
	}
}

func TestCollectorFailureNoProgram(t *testing.T) {
	prog := &fakeProgram{}
	reader := newRecordingReader()
	reader.errors["zia/locations"] = errors.New("api error")

	deps := dependencies{
		gateChecker: alwaysEnabledGate,
		loadConfig: func([]string, config.LoadOptions) (config.Config, error) {
			return oneAPIConfig(), nil
		},
		newReader: func(context.Context, zscaler.ReaderConfig) (browserdata.RecordReader, error) {
			return reader, nil
		},
		newProgram: fakeProgramFactory(prog),
	}

	err := runTest(t, deps, []string{"--live", "--products", "zia", "--resources", "locations"})
	if err == nil || err.Error() != "api error" {
		t.Fatalf("expected collector error, got %v", err)
	}
	if prog.called {
		t.Error("program should not be launched when collector fails and continue-on-error is false")
	}
}

func TestCollectorContinueOnErrorLaunches(t *testing.T) {
	prog := &fakeProgram{}
	reader := newRecordingReader()
	reader.errors["zia/locations"] = errors.New("api error")
	reader.records["zia/url-filtering-rules"] = []resources.SourceRecord{
		resources.NewSourceRecord(map[string]any{"id": 1, "name": "rule1"}),
	}

	deps := dependencies{
		gateChecker: alwaysEnabledGate,
		loadConfig: func([]string, config.LoadOptions) (config.Config, error) {
			return oneAPIConfig(), nil
		},
		newReader: func(context.Context, zscaler.ReaderConfig) (browserdata.RecordReader, error) {
			return reader, nil
		},
		newProgram: fakeProgramFactory(prog),
	}

	err := runTest(t, deps, []string{"--live", "--products", "zia", "--resources", "locations,url-filtering-rules", "--continue-on-error"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !prog.called {
		t.Fatal("program should be launched when continue-on-error is true")
	}
	model, ok := prog.model.(interface{ Data() data.BrowserData })
	if !ok {
		t.Fatal("program model does not expose Data()")
	}
	browserData := model.Data()
	if len(browserData.Products) == 0 {
		t.Fatal("expected at least one product in BrowserData")
	}
	hasError := false
	hasRecords := false
	for _, p := range browserData.Products {
		for _, r := range p.Resources {
			if r.Error != "" {
				hasError = true
			}
			if len(r.Records) > 0 {
				hasRecords = true
			}
		}
	}
	if !hasError {
		t.Error("expected a resource error node in BrowserData")
	}
	if !hasRecords {
		t.Error("expected a resource with records in BrowserData")
	}
}

func TestLiveSuccessLaunches(t *testing.T) {
	prog := &fakeProgram{}
	reader := newRecordingReader()
	reader.records["zia/locations"] = []resources.SourceRecord{
		resources.NewSourceRecord(map[string]any{"id": 1, "name": "HQ"}),
	}

	deps := dependencies{
		gateChecker: alwaysEnabledGate,
		loadConfig: func([]string, config.LoadOptions) (config.Config, error) {
			return oneAPIConfig(), nil
		},
		newReader: func(context.Context, zscaler.ReaderConfig) (browserdata.RecordReader, error) {
			return reader, nil
		},
		newProgram: fakeProgramFactory(prog),
	}

	err := runTest(t, deps, []string{"--live", "--products", "zia", "--resources", "locations"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !prog.called {
		t.Fatal("program should be launched on live success")
	}

	if len(reader.calls) == 0 {
		t.Fatal("reader should have been called")
	}
	found := false
	for _, c := range reader.calls {
		if c.op == "List" && c.product == "zia" && c.resource == "locations" {
			found = true
		}
	}
	if !found {
		t.Errorf("reader calls = %v, want List zia/locations", reader.calls)
	}
}

func TestLegacyZIALiveSuccessLaunches(t *testing.T) {
	prog := &fakeProgram{}
	reader := newRecordingReader()
	reader.records["zia/locations"] = []resources.SourceRecord{
		resources.NewSourceRecord(map[string]any{"id": 1, "name": "LegacyHQ"}),
	}

	deps := dependencies{
		gateChecker: alwaysEnabledGate,
		loadConfig: func([]string, config.LoadOptions) (config.Config, error) {
			return legacyConfig(), nil
		},
		newReader: func(context.Context, zscaler.ReaderConfig) (browserdata.RecordReader, error) {
			return reader, nil
		},
		newProgram: fakeProgramFactory(prog),
	}

	err := runTest(t, deps, []string{"--live", "--products", "zia", "--resources", "locations"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !prog.called {
		t.Fatal("program should be launched on legacy ZIA live success")
	}
}

func TestOneAPIWithoutLegacySecretsLaunches(t *testing.T) {
	prog := &fakeProgram{}
	reader := newRecordingReader()
	reader.records["zia/locations"] = []resources.SourceRecord{
		resources.NewSourceRecord(map[string]any{"id": 1, "name": "HQ"}),
	}
	var gotCfg zscaler.ReaderConfig

	deps := dependencies{
		gateChecker: alwaysEnabledGate,
		loadConfig: func([]string, config.LoadOptions) (config.Config, error) {
			return oneAPIConfigWithoutLegacySecrets(), nil
		},
		newReader: func(ctx context.Context, cfg zscaler.ReaderConfig) (browserdata.RecordReader, error) {
			gotCfg = cfg
			return reader, nil
		},
		newProgram: fakeProgramFactory(prog),
	}

	err := runTest(t, deps, []string{"--live", "--products", "zia", "--resources", "locations"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !prog.called {
		t.Fatal("program should be launched when OneAPI credentials are present and legacy secrets are absent")
	}
	if gotCfg.AuthMode != zscaler.AuthModeOneAPI {
		t.Errorf("AuthMode = %q, want %q", gotCfg.AuthMode, zscaler.AuthModeOneAPI)
	}
	if !gotCfg.ClientSecret.IsSet() {
		t.Error("OneAPI reader config should have a resolved client secret")
	}
	if gotCfg.ZIALegacy.Password.IsSet() || gotCfg.ZIALegacy.APIKey.IsSet() {
		t.Error("OneAPI reader config should not have resolved ZIA legacy secrets")
	}
}

func TestLegacyZIAWithoutClientSecretLaunches(t *testing.T) {
	prog := &fakeProgram{}
	reader := newRecordingReader()
	reader.records["zia/locations"] = []resources.SourceRecord{
		resources.NewSourceRecord(map[string]any{"id": 1, "name": "LegacyHQ"}),
	}
	var gotCfg zscaler.ReaderConfig

	deps := dependencies{
		gateChecker: alwaysEnabledGate,
		loadConfig: func([]string, config.LoadOptions) (config.Config, error) {
			return legacyConfigWithoutClientSecret(), nil
		},
		newReader: func(ctx context.Context, cfg zscaler.ReaderConfig) (browserdata.RecordReader, error) {
			gotCfg = cfg
			return reader, nil
		},
		newProgram: fakeProgramFactory(prog),
	}

	err := runTest(t, deps, []string{"--live", "--products", "zia", "--resources", "locations"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !prog.called {
		t.Fatal("program should be launched when ZIA legacy credentials are present and client secret is absent")
	}
	if gotCfg.AuthMode != zscaler.AuthModeZIALegacy {
		t.Errorf("AuthMode = %q, want %q", gotCfg.AuthMode, zscaler.AuthModeZIALegacy)
	}
	if gotCfg.ClientSecret.IsSet() {
		t.Error("ZIA legacy reader config should not have a resolved client secret")
	}
	if !gotCfg.ZIALegacy.Password.IsSet() || !gotCfg.ZIALegacy.APIKey.IsSet() {
		t.Error("ZIA legacy reader config should have resolved password and api key")
	}
}

func TestLegacyZIAMissingPasswordFailsBeforeTUI(t *testing.T) {
	prog := &fakeProgram{}
	cfg := legacyConfig()
	cfg.ZIALegacy.Password = fakeSecretSource{scheme: "fake", err: errors.New("missing zia password")}

	deps := dependencies{
		gateChecker: alwaysEnabledGate,
		loadConfig: func([]string, config.LoadOptions) (config.Config, error) {
			return cfg, nil
		},
		newReader: func(context.Context, zscaler.ReaderConfig) (browserdata.RecordReader, error) {
			return nil, errors.New("reader should not be reached")
		},
		newProgram: fakeProgramFactory(prog),
	}

	err := runTest(t, deps, []string{"--live"})
	if err == nil {
		t.Fatal("expected credential error, got nil")
	}
	if prog.called {
		t.Error("program should not be launched when a required ZIA legacy secret is missing")
	}
}

func TestProductResourceFiltersPassed(t *testing.T) {
	prog := &fakeProgram{}
	reader := newRecordingReader()
	reader.records["zia/locations"] = []resources.SourceRecord{
		resources.NewSourceRecord(map[string]any{"id": 1, "name": "HQ"}),
	}
	reader.records["zpa/application-segments"] = []resources.SourceRecord{
		resources.NewSourceRecord(map[string]any{"id": 2, "name": "app"}),
	}

	deps := dependencies{
		gateChecker: alwaysEnabledGate,
		loadConfig: func([]string, config.LoadOptions) (config.Config, error) {
			return oneAPIConfig(), nil
		},
		newReader: func(context.Context, zscaler.ReaderConfig) (browserdata.RecordReader, error) {
			return reader, nil
		},
		newProgram: fakeProgramFactory(prog),
	}

	err := runTest(t, deps, []string{"--live", "--products", "zia", "--resources", "locations"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, c := range reader.calls {
		if c.product != "zia" {
			t.Errorf("reader called for product %q, want only zia", c.product)
		}
		if c.resource != "locations" {
			t.Errorf("reader called for resource %q, want only locations", c.resource)
		}
	}
}

func TestFixtureModeLaunches(t *testing.T) {
	prog := &fakeProgram{}
	deps := dependencies{
		gateChecker: alwaysEnabledGate,
		loadConfig:  config.LoadConfig,
		newReader:   defaultDependencies().newReader,
		newProgram:  fakeProgramFactory(prog),
	}

	err := runTest(t, deps, []string{"--fixture"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !prog.called {
		t.Fatal("program should be launched in fixture mode")
	}
	model, ok := prog.model.(interface{ Data() data.BrowserData })
	if !ok {
		t.Fatal("program model does not expose Data()")
	}
	if len(model.Data().Products) == 0 {
		t.Error("fixture BrowserData should contain products")
	}
}

func TestCollectorFixtureModeLaunches(t *testing.T) {
	prog := &fakeProgram{}
	deps := dependencies{
		gateChecker: alwaysEnabledGate,
		loadConfig:  config.LoadConfig,
		newReader:   defaultDependencies().newReader,
		newProgram:  fakeProgramFactory(prog),
	}

	err := runTest(t, deps, []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !prog.called {
		t.Fatal("program should be launched in default collector-fixture mode")
	}
}

func TestFixtureAndLiveAreMutuallyExclusive(t *testing.T) {
	prog := &fakeProgram{}
	deps := dependencies{
		gateChecker: alwaysEnabledGate,
		loadConfig:  config.LoadConfig,
		newReader:   defaultDependencies().newReader,
		newProgram:  fakeProgramFactory(prog),
	}

	err := runTest(t, deps, []string{"--fixture", "--live"})
	if err == nil {
		t.Fatal("expected error for mutually exclusive flags")
	}
	if prog.called {
		t.Error("program should not be launched when flags conflict")
	}
}

func TestFormatJSONDisablesTUI(t *testing.T) {
	prog := &fakeProgram{}
	deps := dependencies{
		gateChecker: tui.Evaluate,
		loadConfig:  config.LoadConfig,
		newReader:   defaultDependencies().newReader,
		newProgram:  fakeProgramFactory(prog),
	}

	err := run(context.Background(), deps, []string{"--live", "--format", "json"}, nil, true, true, true, 80)
	if err == nil {
		t.Fatal("expected error for machine format")
	}
	if prog.called {
		t.Error("program should not be launched when machine format is requested")
	}
}

func TestColorNeverDisablesTUI(t *testing.T) {
	prog := &fakeProgram{}
	deps := dependencies{
		gateChecker: tui.Evaluate,
		loadConfig:  config.LoadConfig,
		newReader:   defaultDependencies().newReader,
		newProgram:  fakeProgramFactory(prog),
	}

	err := run(context.Background(), deps, []string{"--live", "--color", "never"}, nil, true, true, true, 80)
	if err == nil {
		t.Fatal("expected error for disabled color")
	}
	if prog.called {
		t.Error("program should not be launched when color is disabled")
	}
}

// blockingReader never returns until its context is cancelled. It is used to
// test the live-collection timeout without contacting a real API.
type blockingReader struct{}

func (blockingReader) List(ctx context.Context, product resources.Product, resource string) ([]resources.SourceRecord, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func (blockingReader) Show(ctx context.Context, product resources.Product, resource string) (resources.SourceRecord, error) {
	<-ctx.Done()
	return resources.SourceRecord{}, ctx.Err()
}

func TestLiveTimeoutNoProgram(t *testing.T) {
	prog := &fakeProgram{}
	deps := dependencies{
		gateChecker: alwaysEnabledGate,
		loadConfig: func([]string, config.LoadOptions) (config.Config, error) {
			return oneAPIConfig(), nil
		},
		newReader: func(context.Context, zscaler.ReaderConfig) (browserdata.RecordReader, error) {
			return blockingReader{}, nil
		},
		newProgram: fakeProgramFactory(prog),
	}

	start := time.Now()
	err := run(context.Background(), deps, []string{"--live", "--timeout", "50ms", "--products", "zia", "--resources", "locations"}, nil, true, true, true, 80)
	elapsed := time.Since(start)
	if err == nil || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context deadline exceeded, got %v", err)
	}
	if prog.called {
		t.Error("program should not be launched when live collection times out")
	}
	if elapsed > 500*time.Millisecond {
		t.Fatalf("timeout took too long: %v", elapsed)
	}
}

func TestVerboseLiveMilestones(t *testing.T) {
	prog := &fakeProgram{}
	reader := newRecordingReader()
	reader.records["zia/locations"] = []resources.SourceRecord{
		resources.NewSourceRecord(map[string]any{"id": 1, "name": "HQ"}),
	}
	var logs []string

	deps := dependencies{
		gateChecker: alwaysEnabledGate,
		loadConfig: func([]string, config.LoadOptions) (config.Config, error) {
			return oneAPIConfig(), nil
		},
		newReader: func(context.Context, zscaler.ReaderConfig) (browserdata.RecordReader, error) {
			return reader, nil
		},
		newProgram: fakeProgramFactory(prog),
		verboseLog: func(format string, args ...any) { logs = append(logs, fmt.Sprintf(format, args...)) },
	}

	err := run(context.Background(), deps, []string{"--live", "--verbose", "--products", "zia", "--resources", "locations"}, nil, true, true, true, 80)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !prog.called {
		t.Fatal("program should be launched")
	}

	want := []string{
		"checking terminal eligibility",
		"terminal eligibility passed",
		"loading config profile",
		"resolved auth mode",
		"resolving credentials",
		"building reader",
		"reader ready",
		"collecting",
		"launching TUI",
	}
	joined := strings.Join(logs, "\n")
	for _, w := range want {
		if !strings.Contains(joined, w) {
			t.Errorf("verbose logs missing %q; logs = %v", w, logs)
		}
	}
	// Sanity: no secret values should appear in the logs. The oneAPI config uses
	// "client-secret" as the resolved secret value.
	if strings.Contains(joined, "client-secret") || strings.Contains(joined, "password") || strings.Contains(joined, "api-key") {
		t.Errorf("verbose logs appear to contain a secret value: %v", logs)
	}
}

func TestVerboseFixtureMilestones(t *testing.T) {
	prog := &fakeProgram{}
	var logs []string

	deps := dependencies{
		gateChecker: alwaysEnabledGate,
		loadConfig:  config.LoadConfig,
		newReader:   defaultDependencies().newReader,
		newProgram:  fakeProgramFactory(prog),
		verboseLog:  func(format string, args ...any) { logs = append(logs, fmt.Sprintf(format, args...)) },
	}

	err := run(context.Background(), deps, []string{"--fixture", "--verbose"}, nil, true, true, true, 80)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !prog.called {
		t.Fatal("program should be launched in fixture mode")
	}

	joined := strings.Join(logs, "\n")
	if !strings.Contains(joined, "checking terminal eligibility") {
		t.Error("verbose logs missing gate milestone")
	}
	if !strings.Contains(joined, "launching TUI") {
		t.Error("verbose logs missing launch milestone")
	}
	// Fixture mode should not log live-specific milestones.
	for _, forbidden := range []string{"loading config profile", "resolving credentials", "building reader", "collecting"} {
		if strings.Contains(joined, forbidden) {
			t.Errorf("fixture mode logged live milestone %q", forbidden)
		}
	}
}
