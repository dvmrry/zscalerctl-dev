// Package tui contains the disabled-by-default terminal UI eligibility gate.
//
// This package must not import github.com/charmbracelet/bubbletea. Bubble Tea
// v1.x runs package-init terminal probing (Lip Gloss background detection) that
// can emit OSC/cursor queries before the CLI has a chance to evaluate this
// gate. Keeping the gate free of Bubble Tea imports lets normal zscalerctl
// startup paths load and evaluate eligibility without side effects. The actual
// Bubble Tea model lives in the sibling internal/tui/tea package and is only
// reached from explicit demo/runtime entry points.
package tui

import (
	"strings"

	"github.com/dvmrry/zscalerctl/internal/output"
)

// Eligibility reports whether a TUI session may start and, if not, why it was
// disabled.
type Eligibility struct {
	Enabled bool
	Reason  string
}

// Options describe the terminal context used to evaluate TUI eligibility.
type Options struct {
	Requested bool
	StdinTTY  bool
	StdoutTTY bool
	StderrTTY bool
	Format    output.Format
	ColorMode output.ColorMode
	Env       []string
}

const (
	ReasonNotRequested  = "tui not requested"
	ReasonStdinNotTTY   = "stdin is not interactive"
	ReasonStdoutNotTTY  = "stdout is not interactive"
	ReasonStderrNotTTY  = "stderr is not interactive"
	ReasonMachineFormat = "machine output format requested"
	ReasonColorDisabled = "terminal styling disabled"
	ReasonDumbTerminal  = "TERM=dumb"
)

// Evaluate returns an Eligibility decision for the supplied terminal context.
// It is pure: it reads no state beyond opts and performs no terminal I/O.
func Evaluate(opts Options) Eligibility {
	if !opts.Requested {
		return disabled(ReasonNotRequested)
	}
	if !opts.StdinTTY {
		return disabled(ReasonStdinNotTTY)
	}
	if !opts.StdoutTTY {
		return disabled(ReasonStdoutNotTTY)
	}
	if !opts.StderrTTY {
		return disabled(ReasonStderrNotTTY)
	}
	if opts.Format == output.FormatJSON || opts.Format == output.FormatNDJSON {
		return disabled(ReasonMachineFormat)
	}
	if envValue(opts.Env, "NO_COLOR") != "" {
		return disabled(ReasonColorDisabled)
	}
	if envValue(opts.Env, "TERM") == "dumb" {
		return disabled(ReasonDumbTerminal)
	}
	if !output.ShouldColor(opts.ColorMode, opts.Env, opts.StdoutTTY) {
		return disabled(ReasonColorDisabled)
	}
	return Eligibility{Enabled: true}
}

func disabled(reason string) Eligibility {
	return Eligibility{Reason: reason}
}

func envValue(env []string, key string) string {
	prefix := key + "="
	for _, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			return strings.TrimPrefix(entry, prefix)
		}
	}
	return ""
}
