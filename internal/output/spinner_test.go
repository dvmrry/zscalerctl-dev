package output_test

import (
	"bytes"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dvmrry/zscalerctl/internal/output"
)

// TestSpinnerInactiveWritesNothing verifies that an inactive spinner emits zero
// bytes regardless of how many methods are called.
func TestSpinnerInactiveWritesNothing(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	s := output.NewSpinner(&buf, false)
	s.Start("contacting")
	s.Update("zia/locations")
	s.Stop()

	if buf.Len() != 0 {
		t.Errorf("inactive spinner wrote %d bytes (%q), want 0", buf.Len(), buf.String())
	}
}

// TestSpinnerActiveWritesAndClears verifies that an active spinner:
//   - writes something containing a carriage-return (the \r redrawn line),
//   - ends with a cleared-line suffix: \r + spaces + \r.
func TestSpinnerActiveWritesAndClears(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	s := output.NewSpinner(&buf, true)
	s.Start("working")
	// Give the ticker at least one fire so we have output to inspect.
	time.Sleep(200 * time.Millisecond)
	s.Stop()

	got := buf.String()
	if !strings.Contains(got, "\r") {
		t.Errorf("active spinner output %q does not contain \\r", got)
	}
	// The clear-line suffix is \r<spaces>\r — assert it ends with \r.
	if len(got) == 0 || got[len(got)-1] != '\r' {
		t.Errorf("active spinner output %q does not end with \\r (cleared line)", got)
	}
}

// TestSpinnerConcurrencyRace exercises concurrent Start + Update + Stop to
// verify there are no data races under -race.
func TestSpinnerConcurrencyRace(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	s := output.NewSpinner(&buf, true)
	s.Start("initial")
	for i := 0; i < 5; i++ {
		s.Update("step")
		time.Sleep(20 * time.Millisecond)
	}
	s.Stop()
	// Second Stop must be a safe no-op.
	s.Stop()
}

// TestSpinnerStopWithoutStartSafe verifies calling Stop before Start does not
// panic or deadlock.
func TestSpinnerStopWithoutStartSafe(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	s := output.NewSpinner(&buf, true)
	s.Stop() // no Start — must not panic/deadlock
	if buf.Len() != 0 {
		t.Errorf("Stop-without-Start wrote %d bytes, want 0", buf.Len())
	}
}

// TestSpinnerUpdateAfterStopSafe verifies that Update called after Stop does
// not panic.
func TestSpinnerUpdateAfterStopSafe(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	s := output.NewSpinner(&buf, true)
	s.Start("text")
	s.Stop()
	s.Update("after-stop") // must not panic
}

// TestSpinnerStartStopConcurrentNoLeak verifies fix #2: when Start and Stop
// race (e.g. Stop called immediately after Start from a different goroutine),
// the wg.Add(1)-under-lock ordering guarantees that wg.Wait() in Stop always
// joins the goroutine that run() executes. Without the fix, Stop could observe
// started==true, call wg.Wait() on a zero counter, return immediately, and
// leave an orphan goroutine.
//
// Must be run with -race to catch data races.
func TestSpinnerStartStopConcurrentNoLeak(t *testing.T) {
	t.Parallel()

	runtime.GC()
	time.Sleep(20 * time.Millisecond)
	baseline := runtime.NumGoroutine()

	const iterations = 50
	for i := 0; i < iterations; i++ {
		var buf bytes.Buffer
		s := output.NewSpinner(&buf, true)

		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			s.Start("concurrent")
		}()
		go func() {
			defer wg.Done()
			s.Stop()
		}()
		wg.Wait()
		// Ensure any goroutine launched by Start has exited: call Stop again
		// (idempotent) and give the scheduler a moment.
		s.Stop()
	}

	// Allow goroutines to finish.
	runtime.GC()
	time.Sleep(100 * time.Millisecond)

	after := runtime.NumGoroutine()
	const tolerance = 3
	if after > baseline+tolerance {
		t.Errorf("goroutine leak after %d concurrent Start/Stop pairs: baseline=%d, after=%d (delta %d > tolerance %d)",
			iterations, baseline, after, after-baseline, tolerance)
	}
}

// TestSpinnerPanicInCallerNoLeak verifies fix #3: if the caller of a spinner
// panics while the spinner goroutine is running, a properly deferred Stop
// prevents the goroutine from leaking. This test simulates that scenario
// directly at the Spinner level (without going through callWithSpinner).
func TestSpinnerPanicInCallerNoLeak(t *testing.T) {
	t.Parallel()

	runtime.GC()
	time.Sleep(20 * time.Millisecond)
	baseline := runtime.NumGoroutine()

	var buf bytes.Buffer
	s := output.NewSpinner(&buf, true)

	// Simulate a caller that starts the spinner, defers Stop (the fix), and
	// then panics. The outer recover ensures the test continues.
	func() {
		defer func() { recover() }() //nolint:errcheck // intentional panic recovery
		s.Start("work")
		defer s.Stop() // fix #3: covers panic in the caller
		panic("simulated caller panic")
	}()

	// Spinner goroutine must have been joined by the deferred Stop.
	runtime.GC()
	time.Sleep(50 * time.Millisecond)

	after := runtime.NumGoroutine()
	const tolerance = 3
	if after > baseline+tolerance {
		t.Errorf("goroutine leak after panic+deferred Stop: baseline=%d, after=%d (delta %d > tolerance %d)",
			baseline, after, after-baseline, tolerance)
	}
}
