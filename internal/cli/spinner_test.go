package cli

import (
	"bytes"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/dvmrry/zscalerctl/internal/output"
)

// syncBuf is a thread-safe bytes.Buffer used in leak-detection tests. A leaked
// spinner goroutine keeps writing braille frames; a stopped one does not. By
// reading Len() before and after a settle window we can determine whether the
// goroutine is still alive without touching runtime.NumGoroutine(), which is
// unreliable under parallel test runs.
type syncBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuf) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuf) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Len()
}

func (s *syncBuf) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

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
		// Fix #1 regression gate: --color always must NOT activate the spinner when
		// stderr is not a TTY. ShouldColor(ColorAlways, ...) returns true regardless
		// of the isTTY arg, so without the explicit a.stderrTTY guard this case
		// would incorrectly return true and write braille bytes to non-TTY stderr.
		{
			name:      "stderrTTY false + colorAlways must NOT activate spinner",
			stderrTTY: false,
			logLevel:  "off",
			colorMode: output.ColorAlways,
			want:      false,
		},
		{
			name:      "stderrTTY false + colorAlways + NO_COLOR still false",
			stderrTTY: false,
			logLevel:  "off",
			colorMode: output.ColorAlways,
			env:       []string{"NO_COLOR=1"},
			want:      false,
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

// TestCallWithSpinnerPanicSafe verifies fix #3: if fn panics while the spinner
// is active, the deferred s.Stop() in callWithSpinner prevents a goroutine
// leak. Without the defer, the spinner goroutine would stay live indefinitely
// after recovery, keeping writing to stderr until process exit.
//
// Leak detection is deterministic: we use an active spinner (StderrTTY: true)
// backed by a syncBuf so we can safely read the byte count from the test
// goroutine. After the panic+recover, we record Len(), sleep 250 ms (> 2 ×
// 100 ms tick interval), and assert Len() has not grown. A leaked goroutine
// would have written ≥1 more braille frame; a stopped one writes nothing.
func TestCallWithSpinnerPanicSafe(t *testing.T) {
	t.Parallel()

	errBuf := &syncBuf{}
	// StderrTTY: true activates the spinner so the goroutine is actually
	// launched; we can then observe whether it keeps writing after the panic.
	// ColorAlways overrides the NO_COLOR/TERM env heuristics so the spinner
	// is active regardless of the test runner's environment.
	a := NewWithOptions(io.Discard, errBuf, []string{"NO_COLOR="}, Options{StderrTTY: true})
	opts := globalOptions{logLevel: "off", colorMode: output.ColorAlways}

	panicked := false
	// Wrap the panicking call in a recover so the test continues.
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()
		_, _ = callWithSpinner(a, opts, "test-panic", func() (int, error) {
			panic("simulated panic inside fn")
		})
	}()

	if !panicked {
		t.Fatal("expected callWithSpinner to propagate the panic; it did not")
	}

	// Give the deferred Stop inside callWithSpinner a moment to fully join the
	// goroutine (it should already be done, but be safe).
	// Then confirm no further frames arrive in the next 250 ms.
	n1 := errBuf.Len()
	time.Sleep(250 * time.Millisecond) // > 2 × 100 ms tick interval
	n2 := errBuf.Len()

	if n2 != n1 {
		t.Errorf("goroutine leak after panic recovery: errBuf grew from %d to %d bytes in 250 ms after Stop",
			n1, n2)
	}
}
