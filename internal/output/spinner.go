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
// from different goroutines.
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

	s.wg.Add(1)
	go s.run()
}

// Update swaps the text displayed beside the spinning frame. Safe to call
// concurrently. No-op when the spinner is inactive or already stopped.
func (s *Spinner) Update(text string) {
	if !s.active {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped {
		return
	}
	s.text = text
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
	s.wg.Wait()

	// Clear the last rendered line: \r + spaces wide enough to erase + \r.
	s.mu.Lock()
	w := s.lastWidth
	s.mu.Unlock()
	if w < 1 {
		w = 1
	}
	fmt.Fprintf(s.w, "\r%s\r", strings.Repeat(" ", w))
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

// redraw writes a single spinner frame to the writer. It is called from the
// goroutine only, but reads shared state under the mutex.
func (s *Spinner) redraw() {
	s.mu.Lock()
	frame := brailleFrames[s.frame%len(brailleFrames)]
	text := s.text
	s.frame++
	s.mu.Unlock()

	line := fmt.Sprintf("%c %s", frame, text)
	s.mu.Lock()
	s.lastWidth = len([]rune(line))
	s.mu.Unlock()

	fmt.Fprintf(s.w, "\r%s", line)
}
