// Package launcher is the bridge that evaluates TUI eligibility, collects
// fixture BrowserData, and launches the Bubble Tea runtime. It does not import
// github.com/charmbracelet/bubbletea directly; terminal program construction is
// delegated to internal/tui/tea.
package launcher

import (
	"context"
	"fmt"
	"io"

	"github.com/dvmrry/zscalerctl/internal/output"
	"github.com/dvmrry/zscalerctl/internal/tui"
	"github.com/dvmrry/zscalerctl/internal/tui/browserdata"
	tui_tea "github.com/dvmrry/zscalerctl/internal/tui/tea"
)

// Config describes the terminal context and user request for the TUI launch
// layer. The caller (e.g. a Cobra command) is responsible for detecting whether
// stdin/stdout/stderr are interactive TTYs and for supplying the terminal
// streams that Bubble Tea will own.
type Config struct {
	Requested  bool
	StdinTTY   bool
	StdoutTTY  bool
	StderrTTY  bool
	Format     output.Format
	ColorMode  output.ColorMode
	OutputPath string
	Env        []string
	Input      io.Reader
	Output     io.Writer
}

// LaunchError reports that the TUI was disabled by a launch gate. It carries the
// gate reason so the command boundary can render a clear usage error.
type LaunchError struct {
	Reason string
}

func (e LaunchError) Error() string {
	return fmt.Sprintf("tui disabled: %s", e.Reason)
}

// LaunchBrowser evaluates the TUI launch gates, collects a fixture BrowserData,
// and runs the Bubble Tea browser. If a gate disables the TUI, it returns a
// LaunchError (which the caller can wrap as a usage error). It does not load
// config, resolve credentials, or contact Zscaler.
func LaunchBrowser(ctx context.Context, cfg Config) error {
	opts := tui.LaunchOptions{
		Requested:  cfg.Requested,
		StdinTTY:   cfg.StdinTTY,
		StdoutTTY:  cfg.StdoutTTY,
		StderrTTY:  cfg.StderrTTY,
		Format:     cfg.Format,
		ColorMode:  cfg.ColorMode,
		OutputPath: cfg.OutputPath,
		Env:        cfg.Env,
	}
	decision := tui.DecideLaunch(opts)
	if !decision.Enabled {
		return LaunchError{Reason: decision.Reason}
	}

	collector := browserdata.NewCollectorFixture()
	browserData, err := collector.Collect(ctx, browserdata.CollectOptions{ContinueOnError: true})
	if err != nil {
		return err
	}

	style := output.NewStyle(
		output.ShouldColor(cfg.ColorMode, cfg.Env, cfg.StdoutTTY),
		output.Supports256Color(cfg.Env),
	)
	if cfg.StdoutTTY {
		style.Width = output.TerminalWidth(cfg.Output)
	}

	return tui_tea.RunProgram(ctx, cfg.Input, cfg.Output, tui_tea.NewBrowserModel(style, browserData))
}
