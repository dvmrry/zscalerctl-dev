package tui_test

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/output"
	"github.com/dvmrry/zscalerctl/internal/tui"
)

func TestEvaluateRequiresExplicitRequestAndInteractiveStreams(t *testing.T) {
	base := tui.Options{
		Requested: true,
		StdinTTY:  true,
		StdoutTTY: true,
		StderrTTY: true,
		Format:    output.FormatAuto,
		ColorMode: output.ColorAuto,
		Env:       []string{"TERM=xterm-256color"},
	}

	tests := []struct {
		name string
		opts tui.Options
		want tui.Eligibility
	}{
		{
			name: "eligible",
			opts: base,
			want: tui.Eligibility{Enabled: true},
		},
		{
			name: "not requested",
			opts: with(base, func(opts *tui.Options) {
				opts.Requested = false
			}),
			want: tui.Eligibility{Reason: tui.ReasonNotRequested},
		},
		{
			name: "stdin not tty",
			opts: with(base, func(opts *tui.Options) {
				opts.StdinTTY = false
			}),
			want: tui.Eligibility{Reason: tui.ReasonStdinNotTTY},
		},
		{
			name: "stdout not tty",
			opts: with(base, func(opts *tui.Options) {
				opts.StdoutTTY = false
			}),
			want: tui.Eligibility{Reason: tui.ReasonStdoutNotTTY},
		},
		{
			name: "stderr not tty",
			opts: with(base, func(opts *tui.Options) {
				opts.StderrTTY = false
			}),
			want: tui.Eligibility{Reason: tui.ReasonStderrNotTTY},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tui.Evaluate(tt.opts); got != tt.want {
				t.Errorf("Evaluate(%+v) = %+v, want %+v", tt.opts, got, tt.want)
			}
		})
	}
}

func TestEvaluateRejectsMachineFormats(t *testing.T) {
	for _, format := range []output.Format{output.FormatJSON, output.FormatNDJSON} {
		opts := eligibleOptions()
		opts.Format = format

		want := tui.Eligibility{Reason: tui.ReasonMachineFormat}
		if got := tui.Evaluate(opts); got != want {
			t.Errorf("Evaluate(format=%q) = %+v, want %+v", format, got, want)
		}
	}
}

func TestEvaluateRejectsPlainTerminalSignals(t *testing.T) {
	tests := []struct {
		name string
		opts tui.Options
		want tui.Eligibility
	}{
		{
			name: "color never",
			opts: with(eligibleOptions(), func(opts *tui.Options) {
				opts.ColorMode = output.ColorNever
			}),
			want: tui.Eligibility{Reason: tui.ReasonColorDisabled},
		},
		{
			name: "NO_COLOR",
			opts: with(eligibleOptions(), func(opts *tui.Options) {
				opts.Env = []string{"TERM=xterm-256color", "NO_COLOR=1"}
				opts.ColorMode = output.ColorAlways
			}),
			want: tui.Eligibility{Reason: tui.ReasonColorDisabled},
		},
		{
			name: "TERM dumb",
			opts: with(eligibleOptions(), func(opts *tui.Options) {
				opts.Env = []string{"TERM=dumb"}
				opts.ColorMode = output.ColorAlways
			}),
			want: tui.Eligibility{Reason: tui.ReasonDumbTerminal},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tui.Evaluate(tt.opts); got != tt.want {
				t.Errorf("Evaluate(%+v) = %+v, want %+v", tt.opts, got, tt.want)
			}
		})
	}
}

func eligibleOptions() tui.Options {
	return tui.Options{
		Requested: true,
		StdinTTY:  true,
		StdoutTTY: true,
		StderrTTY: true,
		Format:    output.FormatAuto,
		ColorMode: output.ColorAuto,
		Env:       []string{"TERM=xterm-256color"},
	}
}

func with(opts tui.Options, apply func(*tui.Options)) tui.Options {
	apply(&opts)
	return opts
}

func TestEvaluateRejectsOutputPath(t *testing.T) {
	opts := with(eligibleOptions(), func(opts *tui.Options) {
		opts.OutputPath = "/tmp/out.json"
	})
	want := tui.Eligibility{Reason: tui.ReasonOutputPath}
	if got := tui.Evaluate(opts); got != want {
		t.Errorf("Evaluate(outputPath) = %+v, want %+v", got, want)
	}
}

func TestDecideLaunchMirrorsEvaluate(t *testing.T) {
	opts := tui.LaunchOptions{
		Requested:  true,
		StdinTTY:   true,
		StdoutTTY:  true,
		StderrTTY:  true,
		Format:     output.FormatAuto,
		ColorMode:  output.ColorAuto,
		OutputPath: "",
		Env:        []string{"TERM=xterm-256color"},
	}
	if got := tui.DecideLaunch(opts); got != (tui.LaunchDecision{Enabled: true}) {
		t.Errorf("DecideLaunch(eligible) = %+v, want enabled", got)
	}

	opts.OutputPath = "/tmp/out.json"
	want := tui.LaunchDecision{Reason: tui.ReasonOutputPath}
	if got := tui.DecideLaunch(opts); got != want {
		t.Errorf("DecideLaunch(outputPath) = %+v, want %+v", got, want)
	}
}

func TestGatePackageDoesNotImportBubbleTea(t *testing.T) {
	cmd := exec.Command("go", "list", "-f", "{{join .Imports \"\\n\"}}", "github.com/dvmrry/zscalerctl/internal/tui")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go list internal/tui: %v\n%s", err, out)
	}
	if strings.Contains(string(out), "charm.land/bubbletea/v2") {
		t.Fatalf("internal/tui imports charm.land/bubbletea/v2; the gate package must stay independent\n%s", out)
	}
}
