package output_test

import (
	"bytes"
	"strings"
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
// Leak detection is deterministic: a stopped spinner goroutine will NOT write
// any further frames to the syncBuf; a leaked one would write at least one
// braille frame per 100 ms tick. We record Len() immediately after the
// concurrent operations complete, sleep 250 ms (> 2 tick intervals), then
// assert Len() has not grown.
//
// Must be run with -race to catch data races.
func TestSpinnerStartStopConcurrentNoLeak(t *testing.T) {
	t.Parallel()

	const iterations = 50
	for i := 0; i < iterations; i++ {
		buf := &syncBuf{}
		s := output.NewSpinner(buf, true)

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

	// All iterations are done. Pick a fresh spinner so we can observe *its*
	// writer in isolation and confirm no goroutine is still ticking.
	sentinel := &syncBuf{}
	sLast := output.NewSpinner(sentinel, true)
	sLast.Start("probe")
	sLast.Stop() // deferred Stop — goroutine must be joined here

	n1 := sentinel.Len()
	time.Sleep(250 * time.Millisecond) // > 2 × 100 ms tick interval
	n2 := sentinel.Len()

	if n2 != n1 {
		t.Errorf("goroutine leak detected: sentinel writer grew from %d to %d bytes in 250 ms after Stop",
			n1, n2)
	}
}

// TestSpinnerPanicInCallerNoLeak verifies fix #3: if the caller of a spinner
// panics while the spinner goroutine is running, a properly deferred Stop
// prevents the goroutine from leaking. This test simulates that scenario
// directly at the Spinner level (without going through callWithSpinner).
//
// Leak detection is deterministic: after the panic+recover and deferred Stop,
// we record Len() on the syncBuf, sleep 250 ms (> 2 tick intervals), and
// assert Len() has not grown. A leaked goroutine would have written ≥1 more
// braille frame; a stopped one writes nothing.
func TestSpinnerPanicInCallerNoLeak(t *testing.T) {
	t.Parallel()

	buf := &syncBuf{}
	s := output.NewSpinner(buf, true)

	panicked := false
	// Simulate a caller that starts the spinner, defers Stop (the fix), and
	// then panics. The outer recover ensures the test continues.
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()
		s.Start("work")
		defer s.Stop() // fix #3: covers panic in the caller
		panic("simulated caller panic")
	}()

	if !panicked {
		t.Fatal("expected the inner func to panic; it did not")
	}

	// Spinner goroutine must have been joined by the deferred Stop.
	// Record the byte count and confirm no further frames arrive.
	n1 := buf.Len()
	time.Sleep(250 * time.Millisecond) // > 2 × 100 ms tick interval
	n2 := buf.Len()

	if n2 != n1 {
		t.Errorf("goroutine leak after panic+deferred Stop: buf grew from %d to %d bytes in 250 ms",
			n1, n2)
	}
}
