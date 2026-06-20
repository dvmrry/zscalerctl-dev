package cli

import (
	"io"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/output"
)

// TestSpinnerActive is a table-driven white-box test for App.spinnerActive,
// verifying the three-gate logic: stderrTTY AND (logLevel == "" or "off") AND
// colorMode != ColorNever.
func TestSpinnerActive(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		stderrTTY bool
		logLevel  string
		colorMode output.ColorMode
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
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			a := NewWithOptions(io.Discard, io.Discard, nil, Options{
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
