// Package tui contains the disabled-by-default terminal UI foundation.
package tui

import (
	"strings"

	"github.com/dvmrry/zscalerctl/internal/output"
)

type Eligibility struct {
	Enabled bool
	Reason  string
}

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
