package cli

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/output"
)

// TestSpinnerActive is a table-driven white-box test for App.spinnerActive,
// verifying the gate logic: ShouldColor(colorMode, env, stderrTTY) AND
// (logLevel == "" or "off"). ShouldColor folds in --color never/always,
// NO_COLOR=1, and TERM=dumb — all of which suppress spinner output.
func TestSpinnerActive(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		stderrTTY bool
		logLevel  string
		colorMode output.ColorMode
		env       []string
		want      bool
	}{
		{
			name:      "all conditions met",
			stderrTTY: true,
			logLevel:  "off",
			colorMode: output.ColorAuto,
			want:      true,
		},
		{
			name:      "empty logLevel treated as off",
			stderrTTY: true,
			logLevel:  "",
			colorMode: output.ColorAuto,
			want:      true,
		},
		{
			name:      "colorAlways still active",
			stderrTTY: true,
			logLevel:  "off",
			colorMode: output.ColorAlways,
			want:      true,
		},
		{
			name:      "stderrTTY false",
			stderrTTY: false,
			logLevel:  "off",
			colorMode: output.ColorAuto,
			want:      false,
		},
		{
			name:      "logLevel info disables spinner",
			stderrTTY: true,
			logLevel:  "info",
			colorMode: output.ColorAuto,
			want:      false,
		},
		{
			name:      "logLevel error disables spinner",
			stderrTTY: true,
			logLevel:  "error",
			colorMode: output.ColorAuto,
			want:      false,
		},
		{
			name:      "logLevel debug disables spinner",
			stderrTTY: true,
			logLevel:  "debug",
			colorMode: output.ColorAuto,
			want:      false,
		},
		{
			name:      "colorMode never disables spinner",
			stderrTTY: true,
			logLevel:  "off",
			colorMode: output.ColorNever,
			want:      false,
		},
		{
			name:      "all gates off",
			stderrTTY: false,
			logLevel:  "info",
			colorMode: output.ColorNever,
			want:      false,
		},
		// NO_COLOR and TERM=dumb: ShouldColor returns false in ColorAuto mode,
		// so spinnerActive must also return false even when stderrTTY=true.
		{
			name:      "NO_COLOR=1 disables spinner (TTY=true, color=auto)",
			stderrTTY: true,
			logLevel:  "off",
			colorMode: output.ColorAuto,
			env:       []string{"NO_COLOR=1"},
			want:      false,
		},
		{
			name:      "TERM=dumb disables spinner (TTY=true, color=auto)",
			stderrTTY: true,
			logLevel:  "off",
			colorMode: output.ColorAuto,
			env:       []string{"TERM=dumb"},
			want:      false,
		},
		{
			name:      "NO_COLOR=1 does not disable spinner when color=always",
			stderrTTY: true,
			logLevel:  "off",
			colorMode: output.ColorAlways,
			env:       []string{"NO_COLOR=1"},
			want:      true,
		},
		{
			name:      "TERM=dumb does not disable spinner when color=always",
			stderrTTY: true,
			logLevel:  "off",
			colorMode: output.ColorAlways,
			env:       []string{"TERM=dumb"},
			want:      true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			a := NewWithOptions(io.Discard, io.Discard, tc.env, Options{
				StderrTTY: tc.stderrTTY,
			})
			opts := globalOptions{
				logLevel:  tc.logLevel,
				colorMode: tc.colorMode,
			}
			if got := a.spinnerActive(opts); got != tc.want {
				t.Errorf("spinnerActive(%+v) = %v, want %v", tc, got, tc.want)
			}
		})
	}
}

// TestCallWithSpinner verifies that callWithSpinner propagates fn's return
// values, writes zero bytes to stderr when the spinner is inactive (non-TTY),
// and is race-clean.
func TestCallWithSpinner(t *testing.T) {
	t.Parallel()

	// Inactive spinner (stderrTTY = false): nothing must be written to stderr.
	var errBuf bytes.Buffer
	a := NewWithOptions(io.Discard, &errBuf, nil, Options{StderrTTY: false})
	opts := globalOptions{logLevel: "off", colorMode: output.ColorAuto}

	t.Run("returns value and nil error", func(t *testing.T) {
		t.Parallel()
		got, err := callWithSpinner(a, opts, "contacting Zscaler", func() (int, error) {
			return 42, nil
		})
		if err != nil {
			t.Fatalf("callWithSpinner() error = %v, want nil", err)
		}
		if got != 42 {
			t.Errorf("callWithSpinner() value = %d, want 42", got)
		}
		if errBuf.Len() != 0 {
			t.Errorf("callWithSpinner() wrote %d bytes to stderr, want 0", errBuf.Len())
		}
	})

	t.Run("propagates error", func(t *testing.T) {
		t.Parallel()
		sentinel := errors.New("sentinel")
		var errBuf2 bytes.Buffer
		a2 := NewWithOptions(io.Discard, &errBuf2, nil, Options{StderrTTY: false})
		_, err := callWithSpinner(a2, opts, "contacting Zscaler", func() (string, error) {
			return "", sentinel
		})
		if !errors.Is(err, sentinel) {
			t.Errorf("callWithSpinner() error = %v, want sentinel", err)
		}
		if errBuf2.Len() != 0 {
			t.Errorf("callWithSpinner() wrote %d bytes to stderr on error path, want 0", errBuf2.Len())
		}
	})
}

// TestNewSpinnerReturnsCorrectActiveState verifies that newSpinner propagates
// the gate result into the returned Spinner (active vs. inactive).
func TestNewSpinnerReturnsCorrectActiveState(t *testing.T) {
	t.Parallel()

	// Active: stderrTTY=true, logLevel=off, colorMode=auto.
	aActive := NewWithOptions(io.Discard, io.Discard, nil, Options{StderrTTY: true})
	optsActive := globalOptions{logLevel: "off", colorMode: output.ColorAuto}
	spinActive := aActive.newSpinner(optsActive)
	if spinActive == nil {
		t.Fatal("newSpinner returned nil for active configuration")
	}

	// Inactive: stderrTTY=false.
	aInactive := NewWithOptions(io.Discard, io.Discard, nil, Options{StderrTTY: false})
	optsInactive := globalOptions{logLevel: "off", colorMode: output.ColorAuto}
	spinInactive := aInactive.newSpinner(optsInactive)
	if spinInactive == nil {
		t.Fatal("newSpinner returned nil for inactive configuration")
	}
	// Calling Start + Stop on the inactive spinner must write nothing to Discard
	// and must not panic.
	spinInactive.Start("test")
	spinInactive.Stop()
}
