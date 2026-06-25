// Package launcher is the bridge that evaluates TUI eligibility and collects
// BrowserData from a caller-supplied collector. It is intentionally Bubble-free:
// it does not import charm.land/bubbletea/v2 or internal/tui/tea.
// Terminal program construction is delegated to isolated TUI entrypoints (e.g.
// scripts/tui-demo.go, scripts/tui-browser-demo.go, or a future cmd/zscalerctl-tui).
package launcher

import (
	"context"
	"fmt"

	"github.com/dvmrry/zscalerctl/internal/output"
	"github.com/dvmrry/zscalerctl/internal/tui"
	"github.com/dvmrry/zscalerctl/internal/tui/browserdata"
	"github.com/dvmrry/zscalerctl/internal/tui/data"
)

// Config describes the terminal context and user request for the TUI launch
// layer. The caller is responsible for detecting whether stdin/stdout/stderr
// are interactive TTYs and for supplying the terminal streams that the eventual
// TUI runtime will own.
type Config struct {
	Requested      bool
	StdinTTY       bool
	StdoutTTY      bool
	StderrTTY      bool
	Format         output.Format
	ColorMode      output.ColorMode
	OutputPath     string
	Env            []string
	Collector      *browserdata.Collector
	CollectOptions browserdata.CollectOptions
}

// LaunchError reports that the TUI was disabled by a launch gate. It carries the
// gate reason so the command boundary can render a clear usage error.
type LaunchError struct {
	Reason string
}

func (e LaunchError) Error() string {
	return fmt.Sprintf("tui disabled: %s", e.Reason)
}

func launchOptions(cfg Config) tui.LaunchOptions {
	return tui.LaunchOptions{
		Requested:  cfg.Requested,
		StdinTTY:   cfg.StdinTTY,
		StdoutTTY:  cfg.StdoutTTY,
		StderrTTY:  cfg.StderrTTY,
		Format:     cfg.Format,
		ColorMode:  cfg.ColorMode,
		OutputPath: cfg.OutputPath,
		Env:        cfg.Env,
	}
}

// CheckGate evaluates the TUI launch eligibility gates for the supplied config.
// It returns a LaunchError if the TUI should not launch. Callers can use this
// to reject --format json/ndjson, --output, non-TTY, and color-disabled
// invocations before any config, credential, or reader work.
func CheckGate(cfg Config) error {
	decision := tui.DecideLaunch(launchOptions(cfg))
	if !decision.Enabled {
		return LaunchError{Reason: decision.Reason}
	}
	return nil
}

// CollectBrowserData evaluates the TUI launch gates and collects a BrowserData
// view model from the supplied collector. If a gate disables the TUI, it returns
// a LaunchError (which the caller can wrap as a usage error). If no collector is
// supplied, it falls back to the fixture collector used by the isolated demo.
func CollectBrowserData(ctx context.Context, cfg Config) (data.BrowserData, error) {
	if err := CheckGate(cfg); err != nil {
		return data.BrowserData{}, err
	}

	collector := cfg.Collector
	collectOpts := cfg.CollectOptions
	if collector == nil {
		collector = browserdata.NewCollectorFixture()
		collectOpts = browserdata.CollectOptions{ContinueOnError: true}
	}
	return collector.Collect(ctx, collectOpts)
}
