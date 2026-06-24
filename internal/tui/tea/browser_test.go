package tea

import (
	"strings"
	"testing"

	bubbletea "github.com/charmbracelet/bubbletea"

	"github.com/dvmrry/zscalerctl/internal/output"
	"github.com/dvmrry/zscalerctl/internal/tui/data"
)

func TestBrowserModelInitialSelection(t *testing.T) {
	m := NewBrowserModel(output.Style{}, data.NewFakeBrowserData())
	if got := m.SelectedIndex(); got != 0 {
		t.Errorf("initial selected index = %d, want 0", got)
	}
	if got := m.ActivePane(); got != "left" {
		t.Errorf("initial active pane = %q, want left", got)
	}
}

func TestBrowserModelDownMovesSelection(t *testing.T) {
	m := NewBrowserModel(output.Style{}, data.NewFakeBrowserData())
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
	m := NewBrowserModel(output.Style{}, data.NewFakeBrowserData())
	updated, _ := m.Update(bubbletea.KeyMsg{Type: bubbletea.KeyUp})
	m2 := updated.(BrowserModel)
	if got := m2.SelectedIndex(); got != 0 {
		t.Errorf("up from top selection = %d, want 0", got)
	}
}

func TestBrowserModelDownStopsAtBottom(t *testing.T) {
	m := NewBrowserModel(output.Style{}, data.NewFakeBrowserData())
	for i := 0; i < len(m.items)+2; i++ {
		updated, _ := m.Update(bubbletea.KeyMsg{Type: bubbletea.KeyDown})
		m = updated.(BrowserModel)
	}
	if got := m.SelectedIndex(); got != len(m.items)-1 {
		t.Errorf("down past bottom selection = %d, want %d", got, len(m.items)-1)
	}
}

func TestBrowserModelTabSwitchesPane(t *testing.T) {
	m := NewBrowserModel(output.Style{}, data.NewFakeBrowserData())
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
	m := NewBrowserModel(output.Style{}, data.NewFakeBrowserData())
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
		m := NewBrowserModel(output.Style{}, data.NewFakeBrowserData())
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
	m := NewBrowserModel(output.Style{}, data.NewFakeBrowserData())
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
	m := NewBrowserModel(output.Style{}, data.NewFakeBrowserData())
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
	m := NewBrowserModel(output.Style{}, data.NewFakeBrowserData())
	// Select connectors (index 7): zia, locations, url-filtering-rules, forwarding-rules,
	// settings, zpa, app-segments, connectors.
	for i := 0; i < 7; i++ {
		m = step(m, bubbletea.KeyMsg{Type: bubbletea.KeyDown})
	}
	view := m.View()
	if !strings.Contains(view, "connectors") {
		t.Errorf("View() = %q, want selected resource name", view)
	}
	if !strings.Contains(view, "connector list unavailable") {
		t.Errorf("View() = %q, want error message", view)
	}
	if !strings.Contains(view, "Error loading resource") {
		t.Errorf("View() = %q, want error header", view)
	}
}

func TestBrowserDataFakeFixture(t *testing.T) {
	data := data.NewFakeBrowserData()
	if len(data.Products) != 3 {
		t.Errorf("len(Products) = %d, want 3", len(data.Products))
	}
	var zia, zpa, zcc bool
	for _, p := range data.Products {
		switch p.Name {
		case "zia":
			zia = true
			if len(p.Resources) != 4 {
				t.Errorf("zia resources = %d, want 4", len(p.Resources))
			}
		case "zpa":
			zpa = true
			if len(p.Resources) != 2 {
				t.Errorf("zpa resources = %d, want 2", len(p.Resources))
			}
		case "zcc":
			zcc = true
			if len(p.Resources) != 1 {
				t.Errorf("zcc resources = %d, want 1", len(p.Resources))
			}
		}
	}
	if !zia || !zpa || !zcc {
		t.Errorf("missing expected products: zia=%v zpa=%v zcc=%v", zia, zpa, zcc)
	}
	// settings is the long-record resource.
	var settings bool
	for _, r := range data.Products[0].Resources {
		if r.Name == "settings" && len(r.Records) == 1 && len(r.Records[0].Fields) > 5 {
			settings = true
		}
	}
	if !settings {
		t.Errorf("settings long-record resource missing or malformed")
	}
}

func TestBrowserDataContractStates(t *testing.T) {
	data := data.BrowserData{
		Products: []data.ProductNode{
			{
				Name: "test",
				Resources: []data.ResourceNode{
					{Name: "normal", Product: "test", Records: []data.RecordSummary{{ID: "1", Name: "A"}}},
					{Name: "empty", Product: "test", Empty: true},
					{Name: "error", Product: "test", Error: "boom"},
				},
			},
		},
	}
	m := NewBrowserModel(output.Style{}, data)

	// normal
	m = step(m, bubbletea.KeyMsg{Type: bubbletea.KeyDown})
	view := m.View()
	if !strings.Contains(view, "A") {
		t.Errorf("normal view = %q, want record name A", view)
	}

	// empty
	m = step(m, bubbletea.KeyMsg{Type: bubbletea.KeyDown})
	view = m.View()
	if !strings.Contains(view, "No records") {
		t.Errorf("empty view = %q, want No records", view)
	}

	// error
	m = step(m, bubbletea.KeyMsg{Type: bubbletea.KeyDown})
	view = m.View()
	if !strings.Contains(view, "boom") {
		t.Errorf("error view = %q, want error message", view)
	}
}

func TestBrowserModelRendersDataFields(t *testing.T) {
	data := data.BrowserData{
		Products: []data.ProductNode{
			{
				Name: "test",
				Resources: []data.ResourceNode{
					{
						Name:    "fields",
						Product: "test",
						Records: []data.RecordSummary{
							{ID: "1", Name: "A", Fields: []data.KV{{Key: "region", Value: "us-east"}}},
						},
					},
				},
			},
		},
	}
	m := NewBrowserModel(output.Style{}, data)
	m = step(m, bubbletea.KeyMsg{Type: bubbletea.KeyDown})
	view := m.View()
	if !strings.Contains(view, "region: us-east") {
		t.Errorf("view = %q, want generic field", view)
	}
}

func TestBrowserModelHelpOverlay(t *testing.T) {
	m := NewBrowserModel(output.Style{}, data.NewFakeBrowserData())
	updated, _ := m.Update(bubbletea.KeyMsg{Type: bubbletea.KeyRunes, Runes: []rune{'?'}})
	m2 := updated.(BrowserModel)
	if !m2.ShowingHelp() {
		t.Errorf("ShowingHelp() = false, want true")
	}
	view := m2.View()
	if !strings.Contains(view, "Keyboard help") {
		t.Errorf("View() = %q, want help overlay", view)
	}

	updated, _ = m2.Update(bubbletea.KeyMsg{Type: bubbletea.KeyDown})
	m3 := updated.(BrowserModel)
	if m3.ShowingHelp() {
		t.Errorf("ShowingHelp() = true after dismiss, want false")
	}
}

func TestBrowserModelStatusBar(t *testing.T) {
	m := NewBrowserModel(output.Style{}, data.NewFakeBrowserData())
	view := m.View()
	if !strings.Contains(view, "zia · 1/10") {
		t.Errorf("View() = %q, want selected index status", view)
	}
	m = step(m, bubbletea.KeyMsg{Type: bubbletea.KeyDown})
	view = m.View()
	if !strings.Contains(view, "zia / locations · 2/10") {
		t.Errorf("View() = %q, want resource path status", view)
	}
	if !strings.Contains(view, "3 records") {
		t.Errorf("View() = %q, want record count", view)
	}
}

func TestBrowserModelLongRecordScroll(t *testing.T) {
	data := data.BrowserData{
		Products: []data.ProductNode{
			{
				Name: "test",
				Resources: []data.ResourceNode{
					{
						Name:    "scroll",
						Product: "test",
						Records: []data.RecordSummary{
							{ID: "1", Name: "First", Status: "active", Detail: "first", Fields: []data.KV{
								{Key: "a", Value: "1"}, {Key: "b", Value: "2"}, {Key: "c", Value: "3"},
								{Key: "d", Value: "4"}, {Key: "e", Value: "5"}, {Key: "f", Value: "6"},
							}},
							{ID: "2", Name: "Second", Status: "active"},
						},
					},
				},
			},
		},
	}
	m := NewBrowserModel(output.Style{}, data)
	m = step(m, bubbletea.KeyMsg{Type: bubbletea.KeyDown}) // select resource
	m = step(m, bubbletea.KeyMsg{Type: bubbletea.KeyTab})  // focus right pane
	initialScroll := m.ScrollOffset()
	if initialScroll < 0 {
		t.Errorf("initial ScrollOffset() = %d, want >= 0", initialScroll)
	}
	m = step(m, bubbletea.KeyMsg{Type: bubbletea.KeyDown}) // move to second record
	if got := m.ScrollOffset(); got <= initialScroll {
		t.Errorf("ScrollOffset() = %d, want > %d after moving to lower record", got, initialScroll)
	}
	view := m.View()
	if !strings.Contains(view, "Second") {
		t.Errorf("View() = %q, want second record name", view)
	}
}

func step(m BrowserModel, msg bubbletea.Msg) BrowserModel {
	updated, _ := m.Update(msg)
	return updated.(BrowserModel)
}
