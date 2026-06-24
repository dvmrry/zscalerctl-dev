package tea

import (
	"strings"
	"testing"

	bubbletea "github.com/charmbracelet/bubbletea"

	"github.com/dvmrry/zscalerctl/internal/output"
)

func TestBrowserModelInitialSelection(t *testing.T) {
	m := NewBrowserModel(output.Style{})
	if got := m.SelectedIndex(); got != 0 {
		t.Errorf("initial selected index = %d, want 0", got)
	}
	if got := m.ActivePane(); got != "left" {
		t.Errorf("initial active pane = %q, want left", got)
	}
}

func TestBrowserModelDownMovesSelection(t *testing.T) {
	m := NewBrowserModel(output.Style{})
	updated, _ := m.Update(bubbletea.KeyMsg{Type: bubbletea.KeyRunes, Runes: []rune{'j'}})
	// 'j' is not bound, so no change.
	m2, ok := updated.(BrowserModel)
	if !ok {
		t.Fatalf("Update returned %T, want BrowserModel", updated)
	}
	if got := m2.SelectedIndex(); got != 0 {
		t.Errorf("j changed selection to %d, want 0", got)
	}

	updated, _ = m2.Update(bubbletea.KeyMsg{Type: bubbletea.KeyDown})
	m3 := updated.(BrowserModel)
	if got := m3.SelectedIndex(); got != 1 {
		t.Errorf("down selection = %d, want 1", got)
	}
}

func TestBrowserModelUpStopsAtTop(t *testing.T) {
	m := NewBrowserModel(output.Style{})
	updated, _ := m.Update(bubbletea.KeyMsg{Type: bubbletea.KeyUp})
	m2 := updated.(BrowserModel)
	if got := m2.SelectedIndex(); got != 0 {
		t.Errorf("up from top selection = %d, want 0", got)
	}
}

func TestBrowserModelDownStopsAtBottom(t *testing.T) {
	m := NewBrowserModel(output.Style{})
	for i := 0; i < len(m.items)+2; i++ {
		updated, _ := m.Update(bubbletea.KeyMsg{Type: bubbletea.KeyDown})
		m = updated.(BrowserModel)
	}
	if got := m.SelectedIndex(); got != len(m.items)-1 {
		t.Errorf("down past bottom selection = %d, want %d", got, len(m.items)-1)
	}
}

func TestBrowserModelTabSwitchesPane(t *testing.T) {
	m := NewBrowserModel(output.Style{})
	updated, _ := m.Update(bubbletea.KeyMsg{Type: bubbletea.KeyTab})
	m2 := updated.(BrowserModel)
	if got := m2.ActivePane(); got != "right" {
		t.Errorf("tab active pane = %q, want right", got)
	}
	updated, _ = m2.Update(bubbletea.KeyMsg{Type: bubbletea.KeyTab})
	m3 := updated.(BrowserModel)
	if got := m3.ActivePane(); got != "left" {
		t.Errorf("tab again active pane = %q, want left", got)
	}
}

func TestBrowserModelRightPaneNavigation(t *testing.T) {
	m := NewBrowserModel(output.Style{})
	// Select locations (index 1), then tab, then down twice.
	m = step(m, bubbletea.KeyMsg{Type: bubbletea.KeyDown})
	m = step(m, bubbletea.KeyMsg{Type: bubbletea.KeyTab})
	m = step(m, bubbletea.KeyMsg{Type: bubbletea.KeyDown})
	m = step(m, bubbletea.KeyMsg{Type: bubbletea.KeyDown})

	if got := m.RecordIndex(); got != 2 {
		t.Errorf("right pane record index = %d, want 2", got)
	}
	if got := m.ActivePane(); got != "right" {
		t.Errorf("active pane = %q, want right", got)
	}
}

func TestBrowserModelQuitKeys(t *testing.T) {
	for _, key := range []bubbletea.KeyMsg{
		{Type: bubbletea.KeyRunes, Runes: []rune{'q'}},
		{Type: bubbletea.KeyEsc},
		{Type: bubbletea.KeyCtrlC},
	} {
		m := NewBrowserModel(output.Style{})
		updated, cmd := m.Update(key)
		if cmd == nil {
			t.Fatalf("Update(%q) command = nil, want quit command", key.String())
		}
		if got := cmd(); got != bubbletea.Quit() {
			t.Errorf("Update(%q) command() = %#v, want %#v", key.String(), got, bubbletea.Quit())
		}
		quitModel := updated.(BrowserModel)
		if got := quitModel.ExitKey(); got != key.String() {
			t.Errorf("ExitKey() = %q, want %q", got, key.String())
		}
	}
}

func TestBrowserModelViewContainsSelectedResource(t *testing.T) {
	m := NewBrowserModel(output.Style{})
	m = step(m, bubbletea.KeyMsg{Type: bubbletea.KeyDown})
	view := m.View()
	if !strings.Contains(view, "locations") {
		t.Errorf("View() = %q, want selected resource name", view)
	}
	if !strings.Contains(view, "HQ") {
		t.Errorf("View() = %q, want record name", view)
	}
}

func TestBrowserModelViewContainsEmptyState(t *testing.T) {
	m := NewBrowserModel(output.Style{})
	// Select forwarding-rules (index 3).
	m = step(m, bubbletea.KeyMsg{Type: bubbletea.KeyDown})
	m = step(m, bubbletea.KeyMsg{Type: bubbletea.KeyDown})
	m = step(m, bubbletea.KeyMsg{Type: bubbletea.KeyDown})
	view := m.View()
	if !strings.Contains(view, "forwarding-rules") {
		t.Errorf("View() = %q, want selected resource name", view)
	}
	if !strings.Contains(view, "No records") {
		t.Errorf("View() = %q, want empty state", view)
	}
}

func TestBrowserModelViewContainsErrorState(t *testing.T) {
	m := NewBrowserModel(output.Style{})
	// Select connectors (index 6): zia, locations, url-filtering-rules, forwarding-rules, zpa, app-segments, connectors.
	for i := 0; i < 6; i++ {
		m = step(m, bubbletea.KeyMsg{Type: bubbletea.KeyDown})
	}
	view := m.View()
	if !strings.Contains(view, "connectors") {
		t.Errorf("View() = %q, want selected resource name", view)
	}
	if !strings.Contains(view, "connector list unavailable") {
		t.Errorf("View() = %q, want error message", view)
	}
}

func step(m BrowserModel, msg bubbletea.Msg) BrowserModel {
	updated, _ := m.Update(msg)
	return updated.(BrowserModel)
}
