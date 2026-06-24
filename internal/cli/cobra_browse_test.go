package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/config"
	"github.com/dvmrry/zscalerctl/internal/resources"
	"github.com/dvmrry/zscalerctl/internal/tui/launcher"
	"github.com/dvmrry/zscalerctl/internal/zscaler"
)

func TestBrowseWithoutTUIReturnsUsageError(t *testing.T) {
	a := New(io.Discard, io.Discard, nil)
	err := a.Run(context.Background(), []string{"browse"})
	if err == nil {
		t.Fatal("browse without --tui: error = nil, want usage error")
	}
	if !errors.Is(err, ErrUsage) {
		t.Errorf("error = %v, want ErrUsage", err)
	}
	if !strings.Contains(err.Error(), "browse currently requires --tui") {
		t.Errorf("error = %q, want 'browse currently requires --tui'", err.Error())
	}
}

func TestBrowseTUIRequiresInteractiveTTY(t *testing.T) {
	a := New(io.Discard, io.Discard, nil)
	err := a.Run(context.Background(), []string{"browse", "--tui"})
	if err == nil {
		t.Fatal("browse --tui in non-TTY env: error = nil, want usage error")
	}
	if !errors.Is(err, ErrUsage) {
		t.Errorf("error = %v, want ErrUsage", err)
	}
	if !strings.Contains(err.Error(), "tui disabled") {
		t.Errorf("error = %q, want 'tui disabled'", err.Error())
	}
	var launchErr launcher.LaunchError
	if errors.As(err, &launchErr) {
		t.Errorf("error should be wrapped as UsageError, not bare LaunchError")
	}
}

func TestBrowseCommandIsKnownCommand(t *testing.T) {
	a := New(io.Discard, io.Discard, nil)
	if !isKnownCommand("browse", a.resourceCatalog()) {
		t.Error("browse is not recognized as a known command")
	}
	if !isRunnableCommand("browse", a.resourceCatalog()) {
		t.Error("browse is not recognized as a runnable command")
	}
}

func TestBrowseCommandIsHidden(t *testing.T) {
	a := New(io.Discard, io.Discard, nil)
	root := a.buildCommandTree(globalOptions{})
	for _, cmd := range root.Commands() {
		if cmd.Name() == "browse" {
			if !cmd.Hidden {
				t.Error("browse command should be hidden")
			}
			return
		}
	}
	t.Fatal("browse command not found in command tree")
}

func newBrowseApp(t *testing.T, reader ResourceReader, env []string) (*App, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	var out, errBuf bytes.Buffer
	a := NewWithOptions(&out, &errBuf, env, Options{
		Reader:    reader,
		StdinTTY:  true,
		StdoutTTY: true,
		StderrTTY: true,
	})
	return a, &out, &errBuf
}

// browseEnv returns an environment with XDG_CONFIG_HOME pointing at a temp dir so
// no ambient config file is picked up.
func browseEnv(t *testing.T) []string {
	t.Helper()
	return []string{"XDG_CONFIG_HOME=" + t.TempDir()}
}

func TestBrowseTUIGateRejectsFormatJSONBeforeConfig(t *testing.T) {
	t.Parallel()

	a, out, _ := newBrowseApp(t, nil, browseEnv(t))
	launched := false
	a.launchBrowser = func(context.Context, launcher.Config) error {
		launched = true
		return nil
	}
	err := a.Run(context.Background(), []string{"--format", "json", "browse", "--tui"})
	if err == nil {
		t.Fatal("--format json: error = nil, want usage error")
	}
	if !errors.Is(err, ErrUsage) {
		t.Errorf("error = %v, want ErrUsage", err)
	}
	if !strings.Contains(err.Error(), "tui disabled: machine output format requested") {
		t.Errorf("error = %q, want machine output format reason", err.Error())
	}
	if launched {
		t.Error("launchBrowser was called despite format gate")
	}
	if out.Len() != 0 {
		t.Errorf("stdout = %q, want empty", out.String())
	}
}

func TestBrowseTUIGateRejectsOutputBeforeConfig(t *testing.T) {
	t.Parallel()

	a, out, _ := newBrowseApp(t, nil, browseEnv(t))
	launched := false
	a.launchBrowser = func(context.Context, launcher.Config) error {
		launched = true
		return nil
	}
	err := a.Run(context.Background(), []string{"browse", "--tui", "--output", "/tmp/out.json"})
	if err == nil {
		t.Fatal("--output: error = nil, want usage error")
	}
	if !errors.Is(err, ErrUsage) {
		t.Errorf("error = %v, want ErrUsage", err)
	}
	if !strings.Contains(err.Error(), "tui disabled: output path is not supported for TUI") {
		t.Errorf("error = %q, want output path reason", err.Error())
	}
	if launched {
		t.Error("launchBrowser was called despite output gate")
	}
	if out.Len() != 0 {
		t.Errorf("stdout = %q, want empty", out.String())
	}
}

func TestBrowseTUIMissingCredentialsPreventsLaunch(t *testing.T) {
	t.Parallel()

	a, out, _ := newBrowseApp(t, nil, browseEnv(t))
	launched := false
	a.launchBrowser = func(context.Context, launcher.Config) error {
		launched = true
		return nil
	}
	err := a.Run(context.Background(), []string{"browse", "--tui"})
	if err == nil {
		t.Fatal("missing credentials: error = nil, want error")
	}
	if launched {
		t.Error("launchBrowser was called despite missing credentials")
	}
	if !errors.Is(err, zscaler.ErrMissingCredentials) {
		t.Errorf("error = %v, want ErrMissingCredentials", err)
	}
	if out.Len() != 0 {
		t.Errorf("stdout = %q, want empty", out.String())
	}
}

func TestBrowseTUIInvalidConfigPreventsLaunch(t *testing.T) {
	t.Parallel()

	a, out, _ := newBrowseApp(t, nil, browseEnv(t))
	launched := false
	a.launchBrowser = func(context.Context, launcher.Config) error {
		launched = true
		return nil
	}
	err := a.Run(context.Background(), []string{"--config", "/nonexistent/path.yaml", "browse", "--tui"})
	if err == nil {
		t.Fatal("bad config: error = nil, want error")
	}
	if launched {
		t.Error("launchBrowser was called despite bad config")
	}
	if !errors.Is(err, config.ErrInvalidConfig) {
		t.Errorf("error = %v, want ErrInvalidConfig", err)
	}
	if out.Len() != 0 {
		t.Errorf("stdout = %q, want empty", out.String())
	}
}

func TestBrowseTUIFakeReaderSuccessCollectsAndLaunches(t *testing.T) {
	t.Parallel()

	reader := browseFakeReader{
		list: []resources.SourceRecord{
			resources.NewSourceRecord(map[string]any{"id": "1", "name": "Alpha"}),
		},
	}
	a, out, _ := newBrowseApp(t, reader, browseEnv(t))

	var call launcher.Config
	a.launchBrowser = func(ctx context.Context, cfg launcher.Config) error {
		call = cfg
		return nil
	}

	err := a.Run(context.Background(), []string{"browse", "--tui"})
	if err != nil {
		t.Fatalf("fake reader success: error = %v, want nil", err)
	}
	if call.Collector == nil {
		t.Fatal("launchBrowser not called with a collector")
	}
	if call.Collector.Catalog == nil {
		t.Error("collector catalog is nil")
	}
	if call.Collector.Mode == "" {
		t.Error("collector redaction mode not set")
	}

	browserData, err := call.Collector.Collect(context.Background(), call.CollectOptions)
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	if len(browserData.Products) == 0 {
		t.Error("browserData has no products")
	}
	if out.Len() != 0 {
		t.Errorf("stdout = %q, want empty", out.String())
	}
}

func TestBrowseTUIFakeReaderErrorFailFast(t *testing.T) {
	t.Parallel()

	reader := browseFakeReader{err: errors.New("reader failure")}
	a, out, _ := newBrowseApp(t, reader, browseEnv(t))

	a.launchBrowser = func(ctx context.Context, cfg launcher.Config) error {
		// Run the real collection step so the fail-fast policy surfaces before
		// any Bubble Tea program is constructed.
		_, err := cfg.Collector.Collect(ctx, cfg.CollectOptions)
		return err
	}

	err := a.Run(context.Background(), []string{"browse", "--tui"})
	if err == nil {
		t.Fatal("fail-fast: error = nil, want error")
	}
	if !strings.Contains(err.Error(), "reader failure") {
		t.Errorf("error = %q, want 'reader failure'", err.Error())
	}
	if out.Len() != 0 {
		t.Errorf("stdout = %q, want empty", out.String())
	}
}

func TestBrowseTUIFakeReaderErrorContinues(t *testing.T) {
	t.Parallel()

	reader := browseFakeReader{err: errors.New("reader failure")}
	a, out, _ := newBrowseApp(t, reader, browseEnv(t))

	var call launcher.Config
	a.launchBrowser = func(ctx context.Context, cfg launcher.Config) error {
		call = cfg
		return nil
	}

	err := a.Run(context.Background(), []string{"browse", "--tui", "--continue-on-error"})
	if err != nil {
		t.Fatalf("continue-on-error: error = %v, want nil", err)
	}
	if call.Collector == nil {
		t.Fatal("launchBrowser not called with a collector")
	}
	if !call.CollectOptions.ContinueOnError {
		t.Error("ContinueOnError not passed to collector")
	}

	browserData, err := call.Collector.Collect(context.Background(), call.CollectOptions)
	if err != nil {
		t.Fatalf("collect with continue-on-error returned error: %v", err)
	}
	foundError := false
	for _, p := range browserData.Products {
		for _, r := range p.Resources {
			if r.Error != "" {
				foundError = true
			}
		}
	}
	if !foundError {
		t.Error("expected at least one resource with error state")
	}
	if out.Len() != 0 {
		t.Errorf("stdout = %q, want empty", out.String())
	}
}

// browseFakeReader satisfies ResourceReader. It is defined in package cli so it
// can be used to test the browse command's unexported launch hook.
type browseFakeReader struct {
	list []resources.SourceRecord
	get  resources.SourceRecord
	show resources.SourceRecord
	err  error
}

func (r browseFakeReader) List(context.Context, resources.Product, string) ([]resources.SourceRecord, error) {
	return r.list, r.err
}

func (r browseFakeReader) Get(context.Context, resources.Product, string, string) (resources.SourceRecord, error) {
	return r.get, r.err
}

func (r browseFakeReader) Show(context.Context, resources.Product, string) (resources.SourceRecord, error) {
	return r.show, r.err
}

func (r browseFakeReader) Session(context.Context, resources.Product) (zscaler.ResourceSession, error) {
	return nil, r.err
}
