package output_test

import (
	"bytes"
	"strings"
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
