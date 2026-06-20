package output

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
)

// brailleFrames is the animation sequence for the spinner. Each rune is a
// Braille pattern that advances one step per tick.
var brailleFrames = []rune("⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏")

// spinnerTickInterval is the redraw interval. 100 ms gives ~10 fps, which is
// smooth enough without being CPU-intensive.
const spinnerTickInterval = 100 * time.Millisecond

// Spinner is a dep-free, TTY-gated, stderr-only progress indicator.
// When inactive (active == false from NewSpinner) every method is a no-op and
// zero bytes are written. When active, Start launches a goroutine that redraws
// \r<frame> <text> on each tick; Stop joins the goroutine and clears the line.
//
// Spinner is safe for concurrent use: Start, Update, and Stop may be called
// from different goroutines. The invariant is that every write to w happens
// while holding mu, and every write first checks stopped under the same lock,
// so a concurrent Update during or after Stop will never emit a stray frame.
type Spinner struct {
	w      io.Writer
	active bool

	mu        sync.Mutex
	text      string
	frame     int
	started   bool
	stopped   bool
	lastWidth int // width of the last rendered line (frame + space + text)

	stopCh chan struct{}
	wg     sync.WaitGroup
}

// NewSpinner returns a new Spinner that writes to w.
// When active is false every method is a no-op and w is never written.
func NewSpinner(w io.Writer, active bool) *Spinner {
	return &Spinner{
		w:      w,
		active: active,
		stopCh: make(chan struct{}),
	}
}

// Start begins the spinner animation, displaying text next to the frame.
// It writes the current frame immediately (synchronously, under the mutex)
// before launching the ticker goroutine, so the text is always visible
// without waiting for the first tick interval.
// Calling Start on an already-started or inactive spinner is a no-op.
func (s *Spinner) Start(text string) {
	if !s.active {
		return
	}
	s.mu.Lock()
	if s.started || s.stopped {
		s.mu.Unlock()
		return
	}
	s.text = text
	s.started = true
	s.mu.Unlock()

	// Write the first frame synchronously so the text appears immediately.
	s.redraw()

	s.wg.Add(1)
	go s.run()
}

// Update swaps the text displayed beside the spinning frame and immediately
// redraws to make the new text visible without waiting for the next tick.
// Safe to call concurrently. No-op when the spinner is inactive or already stopped.
func (s *Spinner) Update(text string) {
	if !s.active {
		return
	}
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return
	}
	s.text = text
	s.mu.Unlock()

	// Redraw immediately so the new text appears without waiting for the next tick.
	s.redraw()
}

// Stop signals the spinner goroutine to exit and waits for it to finish, then
// clears the line so no partial spinner text precedes subsequent output.
// Stop is idempotent: calling it multiple times or calling it before Start is safe.
func (s *Spinner) Stop() {
	if !s.active {
		return
	}
	s.mu.Lock()
	if !s.started || s.stopped {
		s.mu.Unlock()
		return
	}
	s.stopped = true
	s.mu.Unlock()

	close(s.stopCh)
	// IMPORTANT: mu must NOT be held here; the ticker goroutine calls redraw()
	// which acquires mu. Holding mu across wg.Wait() would deadlock.
	s.wg.Wait()

	// The goroutine is done. Re-acquire mu for the clear-line write so that
	// any concurrent Update's redraw (which sees stopped=true and returns early)
	// cannot interleave with our clear. Re-check lastWidth under the same lock.
	s.mu.Lock()
	w := s.lastWidth
	if w < 1 {
		w = 1
	}
	fmt.Fprintf(s.w, "\r%s\r", strings.Repeat(" ", w))
	s.mu.Unlock()
}

// run is the goroutine launched by Start. It redraws the spinner line on every
// tick until stopCh is closed.
func (s *Spinner) run() {
	defer s.wg.Done()

	ticker := time.NewTicker(spinnerTickInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.redraw()
		}
	}
}

// redraw writes a single spinner frame to the writer under the mutex, so it is
// safe to call from both the ticker goroutine and the caller of Start/Update.
// The entire read-compute-write sequence is protected to prevent interleaved
// output when Update triggers an immediate redraw while the goroutine is ticking.
// If stopped is true (Stop has been called) the write is skipped, ensuring no
// stray frame can land after the clear-line that Stop writes.
func (s *Spinner) redraw() {
	s.mu.Lock()
	if !s.active || s.stopped {
		s.mu.Unlock()
		return
	}
	frame := brailleFrames[s.frame%len(brailleFrames)]
	text := s.text
	s.frame++
	line := fmt.Sprintf("%c %s", frame, text)
	s.lastWidth = len([]rune(line))
	// Write to w while holding the mutex so concurrent redraws cannot interleave,
	// and so Stop's clear-line write (also under mu) cannot interleave with us.
	fmt.Fprintf(s.w, "\r%s", line)
	s.mu.Unlock()
}
