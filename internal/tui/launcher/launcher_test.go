package launcher

import (
	"context"
	"errors"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/output"
	"github.com/dvmrry/zscalerctl/internal/tui"
)

func TestLaunchBrowserRequiresExplicitRequest(t *testing.T) {
	cfg := Config{
		Requested: true,
		StdinTTY:  true,
		StdoutTTY: true,
		StderrTTY: true,
		Format:    output.FormatAuto,
		ColorMode: output.ColorAuto,
		Env:       []string{"TERM=xterm-256color"},
	}
	cfg.Requested = false
	err := LaunchBrowser(context.Background(), cfg)
	var launchErr LaunchError
	if !errors.As(err, &launchErr) {
		t.Fatalf("error = %v, want LaunchError", err)
	}
	if launchErr.Reason != tui.ReasonNotRequested {
		t.Errorf("reason = %q, want %q", launchErr.Reason, tui.ReasonNotRequested)
	}
}

func TestLaunchBrowserRejectsNonTTY(t *testing.T) {
	base := Config{
		Requested: true,
		StdinTTY:  true,
		StdoutTTY: true,
		StderrTTY: true,
		Format:    output.FormatAuto,
		ColorMode: output.ColorAuto,
		Env:       []string{"TERM=xterm-256color"},
	}

	tests := []struct {
		name   string
		mutate func(*Config)
		want   string
	}{
		{"stdin not tty", func(c *Config) { c.StdinTTY = false }, tui.ReasonStdinNotTTY},
		{"stdout not tty", func(c *Config) { c.StdoutTTY = false }, tui.ReasonStdoutNotTTY},
		{"stderr not tty", func(c *Config) { c.StderrTTY = false }, tui.ReasonStderrNotTTY},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := base
			tt.mutate(&cfg)
			err := LaunchBrowser(context.Background(), cfg)
			var launchErr LaunchError
			if !errors.As(err, &launchErr) {
				t.Fatalf("error = %v, want LaunchError", err)
			}
			if launchErr.Reason != tt.want {
				t.Errorf("reason = %q, want %q", launchErr.Reason, tt.want)
			}
		})
	}
}

func TestLaunchBrowserRejectsMachineFormats(t *testing.T) {
	cfg := Config{
		Requested: true,
		StdinTTY:  true,
		StdoutTTY: true,
		StderrTTY: true,
		ColorMode: output.ColorAuto,
		Env:       []string{"TERM=xterm-256color"},
	}

	for _, format := range []output.Format{output.FormatJSON, output.FormatNDJSON} {
		cfg.Format = format
		err := LaunchBrowser(context.Background(), cfg)
		var launchErr LaunchError
		if !errors.As(err, &launchErr) {
			t.Fatalf("format=%q error = %v, want LaunchError", format, err)
		}
		if launchErr.Reason != tui.ReasonMachineFormat {
			t.Errorf("format=%q reason = %q, want %q", format, launchErr.Reason, tui.ReasonMachineFormat)
		}
	}
}

func TestLaunchBrowserRejectsOutputPath(t *testing.T) {
	cfg := Config{
		Requested:  true,
		StdinTTY:   true,
		StdoutTTY:  true,
		StderrTTY:  true,
		Format:     output.FormatAuto,
		ColorMode:  output.ColorAuto,
		OutputPath: "/tmp/out.json",
		Env:        []string{"TERM=xterm-256color"},
	}
	err := LaunchBrowser(context.Background(), cfg)
	var launchErr LaunchError
	if !errors.As(err, &launchErr) {
		t.Fatalf("error = %v, want LaunchError", err)
	}
	if launchErr.Reason != tui.ReasonOutputPath {
		t.Errorf("reason = %q, want %q", launchErr.Reason, tui.ReasonOutputPath)
	}
}

func TestLaunchBrowserRejectsColorDisabled(t *testing.T) {
	base := Config{
		Requested: true,
		StdinTTY:  true,
		StdoutTTY: true,
		StderrTTY: true,
		Format:    output.FormatAuto,
		ColorMode: output.ColorAuto,
	}

	tests := []struct {
		name string
		cfg  Config
		want string
	}{
		{
			name: "color never",
			cfg:  with(base, func(c *Config) { c.ColorMode = output.ColorNever }),
			want: tui.ReasonColorDisabled,
		},
		{
			name: "NO_COLOR",
			cfg: with(base, func(c *Config) {
				c.Env = []string{"TERM=xterm-256color", "NO_COLOR=1"}
				c.ColorMode = output.ColorAlways
			}),
			want: tui.ReasonColorDisabled,
		},
		{
			name: "TERM dumb",
			cfg:  with(base, func(c *Config) { c.Env = []string{"TERM=dumb"}; c.ColorMode = output.ColorAlways }),
			want: tui.ReasonDumbTerminal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := LaunchBrowser(context.Background(), tt.cfg)
			var launchErr LaunchError
			if !errors.As(err, &launchErr) {
				t.Fatalf("error = %v, want LaunchError", err)
			}
			if launchErr.Reason != tt.want {
				t.Errorf("reason = %q, want %q", launchErr.Reason, tt.want)
			}
		})
	}
}

func with(cfg Config, apply func(*Config)) Config {
	apply(&cfg)
	return cfg
}
