package tea

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	bubbletea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

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

func TestLazyBrowserModelLoadsSelectedResource(t *testing.T) {
	loader := &recordingResourceLoader{
		nodes: map[string]data.ResourceNode{
			"zia/locations": {
				Product: "zia",
				Name:    "locations",
				State:   data.ResourceStateLoaded,
				Records: []data.RecordSummary{{ID: "1", Name: "HQ", Status: "active"}},
			},
		},
	}
	m := NewLazyBrowserModel(output.Style{}, lazyBrowserData(), loader, time.Second)
	m = step(m, bubbletea.KeyMsg{Type: bubbletea.KeyDown})

	updated, cmd := m.Update(bubbletea.KeyMsg{Type: bubbletea.KeyEnter})
	if cmd == nil {
		t.Fatal("BrowserModel.Update(enter on unloaded resource) command = nil, want load command")
	}
	loading := updated.(BrowserModel)
	if len(loader.calls) != 0 {
		t.Fatalf("loader calls before command execution = %v, want none", loader.calls)
	}
	if got := loading.selectedItem().effectiveState(); got != data.ResourceStateLoading {
		t.Fatalf("selected resource state = %s, want %s", got, data.ResourceStateLoading)
	}
	if view := loading.View(); !strings.Contains(view, "Loading resource") {
		t.Errorf("loading View() = %q, want loading state", view)
	}

	msg := cmd()
	if got, want := len(loader.calls), 1; got != want {
		t.Fatalf("loader calls after command = %d, want %d", got, want)
	}
	updated, cmd = loading.Update(msg)
	if cmd != nil {
		t.Fatalf("BrowserModel.Update(resourceLoadedMsg) command = %v, want nil", cmd)
	}
	loaded := updated.(BrowserModel)
	if got := loaded.selectedItem().effectiveState(); got != data.ResourceStateLoaded {
		t.Fatalf("selected resource state = %s, want %s", got, data.ResourceStateLoaded)
	}
	if view := loaded.View(); !strings.Contains(view, "HQ") {
		t.Errorf("loaded View() = %q, want record name", view)
	}
}

func TestLazyBrowserModelCachesLoadedResourceUntilRefresh(t *testing.T) {
	loader := &recordingResourceLoader{
		nodes: map[string]data.ResourceNode{
			"zia/locations": {
				Product: "zia",
				Name:    "locations",
				State:   data.ResourceStateLoaded,
				Records: []data.RecordSummary{{ID: "1", Name: "HQ"}},
			},
		},
	}
	m := NewLazyBrowserModel(output.Style{}, lazyBrowserData(), loader, time.Second)
	m = step(m, bubbletea.KeyMsg{Type: bubbletea.KeyDown})

	updated, cmd := m.Update(bubbletea.KeyMsg{Type: bubbletea.KeyEnter})
	if cmd == nil {
		t.Fatal("first enter command = nil, want load command")
	}
	updated, _ = updated.(BrowserModel).Update(cmd())
	loaded := updated.(BrowserModel)
	if got := len(loader.calls); got != 1 {
		t.Fatalf("loader calls after first load = %d, want 1", got)
	}

	updated, cmd = loaded.Update(bubbletea.KeyMsg{Type: bubbletea.KeyEnter})
	if cmd != nil {
		t.Fatalf("second enter command = %v, want nil for cached loaded resource", cmd)
	}
	if got := len(loader.calls); got != 1 {
		t.Fatalf("loader calls after cached enter = %d, want 1", got)
	}

	updated, cmd = updated.(BrowserModel).Update(bubbletea.KeyMsg{Type: bubbletea.KeyRunes, Runes: []rune{'r'}})
	if cmd == nil {
		t.Fatal("refresh command = nil, want load command")
	}
	_, _ = updated.(BrowserModel).Update(cmd())
	if got := len(loader.calls); got != 2 {
		t.Fatalf("loader calls after refresh = %d, want 2", got)
	}
}

func TestLazyBrowserModelFailedResourceBecomesErrorState(t *testing.T) {
	loader := &recordingResourceLoader{
		nodes: map[string]data.ResourceNode{
			"zia/locations": {
				Product: "zia",
				Name:    "locations",
				State:   data.ResourceStateError,
				Error:   "api failed",
			},
		},
	}
	m := NewLazyBrowserModel(output.Style{}, lazyBrowserData(), loader, time.Second)
	m = step(m, bubbletea.KeyMsg{Type: bubbletea.KeyDown})

	updated, cmd := m.Update(bubbletea.KeyMsg{Type: bubbletea.KeyEnter})
	if cmd == nil {
		t.Fatal("enter command = nil, want load command")
	}
	updated, _ = updated.(BrowserModel).Update(cmd())
	errored := updated.(BrowserModel)
	if got := errored.selectedItem().effectiveState(); got != data.ResourceStateError {
		t.Fatalf("selected resource state = %s, want %s", got, data.ResourceStateError)
	}
	view := errored.View()
	if !strings.Contains(view, "Error loading resource") || !strings.Contains(view, "api failed") {
		t.Errorf("error View() = %q, want error state", view)
	}
}

func TestLazyBrowserModelSlowResourceTimesOut(t *testing.T) {
	loader := &recordingResourceLoader{waitForContext: true}
	m := NewLazyBrowserModel(output.Style{}, lazyBrowserData(), loader, 5*time.Millisecond)
	m = step(m, bubbletea.KeyMsg{Type: bubbletea.KeyDown})

	updated, cmd := m.Update(bubbletea.KeyMsg{Type: bubbletea.KeyEnter})
	if cmd == nil {
		t.Fatal("enter command = nil, want load command")
	}
	loading := updated.(BrowserModel)
	if view := loading.View(); !strings.Contains(view, "Loading resource") {
		t.Errorf("loading View() = %q, want loading state", view)
	}

	updated, _ = loading.Update(cmd())
	errored := updated.(BrowserModel)
	if got := errored.selectedItem().effectiveState(); got != data.ResourceStateError {
		t.Fatalf("selected resource state = %s, want %s", got, data.ResourceStateError)
	}
	if view := errored.View(); !strings.Contains(view, context.DeadlineExceeded.Error()) {
		t.Errorf("timeout View() = %q, want context deadline exceeded", view)
	}
}

func TestBrowserModelLeftViewportKeepsSelectionVisibleWithLongList(t *testing.T) {
	m := NewBrowserModel(output.Style{}, browserDataWithResources(200))
	m = step(m, bubbletea.WindowSizeMsg{Width: 80, Height: 16})
	m = step(m, bubbletea.KeyMsg{Type: bubbletea.KeyEnd})

	if got, want := m.SelectedIndex(), len(m.items)-1; got != want {
		t.Fatalf("SelectedIndex() = %d, want %d", got, want)
	}
	assertViewportSelectionVisible(t, "left viewport", m.left, len(m.items), m.leftViewportHeight())
	view := m.View()
	if !strings.Contains(view, "resource-199") {
		t.Errorf("View() = %q, want selected resource visible", view)
	}
	if strings.Count(view, "resource-") > 20 {
		t.Errorf("View() rendered %d resource rows, want bounded visible rows", strings.Count(view, "resource-"))
	}
}

func TestBrowserModelLeftViewportPageHomeEnd(t *testing.T) {
	m := NewBrowserModel(output.Style{}, browserDataWithResources(200))
	m = step(m, bubbletea.WindowSizeMsg{Width: 80, Height: 16})
	pageSize := m.leftViewportHeight()

	m = step(m, bubbletea.KeyMsg{Type: bubbletea.KeyPgDown})
	if got := m.SelectedIndex(); got != pageSize {
		t.Fatalf("SelectedIndex() after pgdown = %d, want %d", got, pageSize)
	}
	assertViewportSelectionVisible(t, "left viewport after pgdown", m.left, len(m.items), pageSize)

	m = step(m, bubbletea.KeyMsg{Type: bubbletea.KeyPgUp})
	if got := m.SelectedIndex(); got != 0 {
		t.Fatalf("SelectedIndex() after pgup = %d, want 0", got)
	}

	m = step(m, bubbletea.KeyMsg{Type: bubbletea.KeyEnd})
	if got, want := m.SelectedIndex(), len(m.items)-1; got != want {
		t.Fatalf("SelectedIndex() after end = %d, want %d", got, want)
	}

	m = step(m, bubbletea.KeyMsg{Type: bubbletea.KeyHome})
	if got := m.SelectedIndex(); got != 0 {
		t.Fatalf("SelectedIndex() after home = %d, want 0", got)
	}
}

func TestBrowserModelRightViewportBoundsLongRecordList(t *testing.T) {
	m := NewBrowserModel(output.Style{}, browserDataWithRecords(1000))
	m = step(m, bubbletea.WindowSizeMsg{Width: 120, Height: 32})
	m = step(m, bubbletea.KeyMsg{Type: bubbletea.KeyDown})
	m = step(m, bubbletea.KeyMsg{Type: bubbletea.KeyTab})
	m = step(m, bubbletea.KeyMsg{Type: bubbletea.KeyEnd})

	if got := m.RecordIndex(); got != 999 {
		t.Fatalf("RecordIndex() after end = %d, want 999", got)
	}
	if got := m.ScrollOffset(); got <= 0 {
		t.Fatalf("ScrollOffset() after end = %d, want > 0", got)
	}
	assertViewportSelectionVisible(t, "right viewport", m.right, len(m.selectedRecords()), m.rightViewportHeight())
	view := m.View()
	if !strings.Contains(view, "rec-0999") {
		t.Errorf("View() = %q, want selected record visible", view)
	}
	if strings.Contains(view, "rec-0000") {
		t.Errorf("View() = %q, want first record outside bounded viewport", view)
	}
	if strings.Count(view, "rec-") > 40 {
		t.Errorf("View() rendered %d records, want bounded visible records", strings.Count(view, "rec-"))
	}
	assertViewLineWidths(t, view, 120)
}

func TestBrowserModelRightViewportPageHomeEnd(t *testing.T) {
	m := NewBrowserModel(output.Style{}, browserDataWithRecords(1000))
	m = step(m, bubbletea.WindowSizeMsg{Width: 80, Height: 16})
	m = step(m, bubbletea.KeyMsg{Type: bubbletea.KeyDown})
	m = step(m, bubbletea.KeyMsg{Type: bubbletea.KeyTab})
	pageSize := m.rightViewportHeight()

	m = step(m, bubbletea.KeyMsg{Type: bubbletea.KeyPgDown})
	if got := m.RecordIndex(); got != pageSize {
		t.Fatalf("RecordIndex() after pgdown = %d, want %d", got, pageSize)
	}
	assertViewportSelectionVisible(t, "right viewport after pgdown", m.right, len(m.selectedRecords()), pageSize)

	m = step(m, bubbletea.KeyMsg{Type: bubbletea.KeyPgDown})
	if got, want := m.RecordIndex(), pageSize*2; got != want {
		t.Fatalf("RecordIndex() after second pgdown = %d, want %d", got, want)
	}

	m = step(m, bubbletea.KeyMsg{Type: bubbletea.KeyPgUp})
	if got := m.RecordIndex(); got != pageSize {
		t.Fatalf("RecordIndex() after pgup = %d, want %d", got, pageSize)
	}

	m = step(m, bubbletea.KeyMsg{Type: bubbletea.KeyEnd})
	if got := m.RecordIndex(); got != 999 {
		t.Fatalf("RecordIndex() after end = %d, want 999", got)
	}

	m = step(m, bubbletea.KeyMsg{Type: bubbletea.KeyHome})
	if got := m.RecordIndex(); got != 0 {
		t.Fatalf("RecordIndex() after home = %d, want 0", got)
	}
}

func TestBrowserModelWideLayoutSplitsRecordsAndDetails(t *testing.T) {
	m := NewBrowserModel(output.Style{}, data.NewFakeBrowserData())
	m = step(m, bubbletea.WindowSizeMsg{Width: 120, Height: 32})
	m = step(m, bubbletea.KeyMsg{Type: bubbletea.KeyDown})

	view := m.View()
	for _, want := range []string{"Products / Resources", "Records", "locations", "HQ", "US East"} {
		if !strings.Contains(view, want) {
			t.Fatalf("View() = %q, want %q in three-pane layout", view, want)
		}
	}
	if !strings.Contains(view, "id=123") {
		t.Fatalf("View() = %q, want record-list summary with id", view)
	}
	if strings.Contains(view, "id: 123") {
		t.Fatalf("View() = %q, want detail pane to avoid repeating record-list id", view)
	}
	if strings.Contains(view, "status: active") {
		t.Fatalf("View() = %q, want detail pane to avoid repeating record-list status", view)
	}
	assertViewLineWidths(t, view, 120)
}

func TestBrowserModelWideTabCyclesThroughDetails(t *testing.T) {
	m := NewBrowserModel(output.Style{}, data.NewFakeBrowserData())
	m = step(m, bubbletea.WindowSizeMsg{Width: 120, Height: 32})

	m = step(m, bubbletea.KeyMsg{Type: bubbletea.KeyTab})
	if got := m.ActivePane(); got != "right" {
		t.Fatalf("ActivePane() after first tab = %q, want right", got)
	}
	m = step(m, bubbletea.KeyMsg{Type: bubbletea.KeyTab})
	if got := m.ActivePane(); got != "detail" {
		t.Fatalf("ActivePane() after second tab = %q, want detail", got)
	}
	m = step(m, bubbletea.KeyMsg{Type: bubbletea.KeyTab})
	if got := m.ActivePane(); got != "left" {
		t.Fatalf("ActivePane() after third tab = %q, want left", got)
	}
}

func TestBrowserModelDetailViewportScrollsLargeSelectedBody(t *testing.T) {
	browserData := browserDataWithLargeRecordBody(40)
	m := NewBrowserModel(output.Style{}, browserData)
	m = step(m, bubbletea.WindowSizeMsg{Width: 120, Height: 16})
	m = step(m, bubbletea.KeyMsg{Type: bubbletea.KeyDown})
	m = step(m, bubbletea.KeyMsg{Type: bubbletea.KeyTab})
	m = step(m, bubbletea.KeyMsg{Type: bubbletea.KeyTab})

	if got := m.ActivePane(); got != "detail" {
		t.Fatalf("ActivePane() = %q, want detail", got)
	}
	m = step(m, bubbletea.KeyMsg{Type: bubbletea.KeyEnd})
	if got := m.detail.Offset; got <= 0 {
		t.Fatalf("detail offset after end = %d, want > 0", got)
	}
	view := m.View()
	if !strings.Contains(view, "field_39") {
		t.Fatalf("View() = %q, want final field visible after detail scroll", view)
	}
	assertViewLineWidths(t, view, 120)
}

func TestBrowserModelDetailWrapsStructuredFieldWithoutEllipsis(t *testing.T) {
	browserData := data.BrowserData{
		Products: []data.ProductNode{
			{
				Name: "zia",
				Resources: []data.ResourceNode{
					{
						Product: "zia",
						Name:    "rules",
						Records: []data.RecordSummary{
							{
								ID:     "12341235",
								Name:   "Rule",
								Status: "active",
								Fields: []data.KV{
									{Key: "dynamicLocationGroups", Value: "[\n  {\n    \"id\": \"1132433111\",\n    \"name\": \"Corporate User Traffic With A Long Name That Wraps\"\n  }\n]"},
								},
							},
						},
					},
				},
			},
		},
	}
	m := NewBrowserModel(output.Style{}, browserData)
	m = step(m, bubbletea.WindowSizeMsg{Width: 120, Height: 32})
	m = step(m, bubbletea.KeyMsg{Type: bubbletea.KeyDown})

	view := m.View()
	for _, want := range []string{"Rule (id=12341235", "status=active)", "dynamicLocationGroups:", `"name": "Corporate User Traffic`, "]"} {
		if !strings.Contains(view, want) {
			t.Fatalf("View() = %q, want %q", view, want)
		}
	}
	for _, notWant := range []string{"dynamicLocationGroups: [map[", "id: 12341235", "status: active", "..."} {
		if strings.Contains(view, notWant) {
			t.Fatalf("View() = %q, did not want %q", view, notWant)
		}
	}
	assertViewLineWidths(t, view, 120)
}

func TestBrowserModelLongFieldValuesFitPaneWidth(t *testing.T) {
	longValue := strings.Repeat("tenant-value-", 80)
	browserData := data.BrowserData{
		Products: []data.ProductNode{
			{
				Name: "zia",
				Resources: []data.ResourceNode{
					{
						Product: "zia",
						Name:    "long-values",
						Records: []data.RecordSummary{
							{
								ID:     "1",
								Name:   "record-with-a-very-long-display-name",
								Status: "active",
								Fields: []data.KV{{Key: "description", Value: longValue}},
							},
						},
					},
				},
			},
		},
	}
	m := NewBrowserModel(output.Style{}, browserData)
	m = step(m, bubbletea.WindowSizeMsg{Width: 60, Height: 16})
	m = step(m, bubbletea.KeyMsg{Type: bubbletea.KeyDown})

	view := m.View()
	assertViewLineWidths(t, view, 60)
	if !strings.Contains(view, "...") {
		t.Errorf("View() = %q, want long field truncation marker", view)
	}
	if !strings.Contains(view, "esc/q quit") {
		t.Errorf("View() = %q, want footer visible", view)
	}
}

func TestBrowserModelResizeClampsViewports(t *testing.T) {
	m := NewBrowserModel(output.Style{}, browserDataWithRecords(1000))
	m = step(m, bubbletea.WindowSizeMsg{Width: 120, Height: 32})
	m = step(m, bubbletea.KeyMsg{Type: bubbletea.KeyDown})
	m = step(m, bubbletea.KeyMsg{Type: bubbletea.KeyTab})
	m = step(m, bubbletea.KeyMsg{Type: bubbletea.KeyEnd})

	m = step(m, bubbletea.WindowSizeMsg{Width: 60, Height: 16})
	assertViewportSelectionVisible(t, "right viewport after resize", m.right, len(m.selectedRecords()), m.rightViewportHeight())
	assertViewLineWidths(t, m.View(), 60)
}

func TestBrowserModelSmallGeometryRendersResourceStates(t *testing.T) {
	browserData := data.BrowserData{
		Products: []data.ProductNode{
			{
				Name: "zia",
				Resources: []data.ResourceNode{
					{Product: "zia", Name: "unloaded", State: data.ResourceStateUnloaded},
					{Product: "zia", Name: "loading", State: data.ResourceStateLoading},
					{Product: "zia", Name: "errored", State: data.ResourceStateError, Error: "sanitized failure"},
				},
			},
		},
	}
	tests := []struct {
		name       string
		moves      int
		wantString string
	}{
		{name: "unloaded", moves: 1, wantString: "Resource not loaded"},
		{name: "loading", moves: 2, wantString: "Loading resource"},
		{name: "error", moves: 3, wantString: "Error loading resource"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewBrowserModel(output.Style{}, browserData)
			m = step(m, bubbletea.WindowSizeMsg{Width: 60, Height: 16})
			for i := 0; i < tt.moves; i++ {
				m = step(m, bubbletea.KeyMsg{Type: bubbletea.KeyDown})
			}
			view := m.View()
			if !strings.Contains(view, tt.wantString) {
				t.Errorf("View() = %q, want %q", view, tt.wantString)
			}
			assertViewLineWidths(t, view, 60)
		})
	}
}

func TestLazyBrowserModelViewportLoadsOnlySelectedResourceAndCaches(t *testing.T) {
	loader := &recordingResourceLoader{
		nodes: map[string]data.ResourceNode{
			"zia/resource-199": {
				Product: "zia",
				Name:    "resource-199",
				State:   data.ResourceStateLoaded,
				Records: []data.RecordSummary{{ID: "199", Name: "selected"}},
			},
		},
	}
	m := NewLazyBrowserModel(output.Style{}, browserDataWithResources(200), loader, time.Second)
	m = step(m, bubbletea.WindowSizeMsg{Width: 80, Height: 16})
	m = step(m, bubbletea.KeyMsg{Type: bubbletea.KeyEnd})

	updated, cmd := m.Update(bubbletea.KeyMsg{Type: bubbletea.KeyEnter})
	if cmd == nil {
		t.Fatal("BrowserModel.Update(enter on selected unloaded resource) command = nil, want load command")
	}
	if got := loader.calls; len(got) != 0 {
		t.Fatalf("loader calls before command = %v, want none", got)
	}
	loading := updated.(BrowserModel)
	if got := loading.selectedItem().effectiveState(); got != data.ResourceStateLoading {
		t.Fatalf("selected resource state after enter = %s, want %s", got, data.ResourceStateLoading)
	}

	updated, _ = loading.Update(cmd())
	loaded := updated.(BrowserModel)
	if got := len(loader.calls); got != 1 {
		t.Fatalf("loader calls after load = %d, want 1", got)
	}
	if got, want := loader.calls[0], "zia/resource-199"; got != want {
		t.Fatalf("loader call after load = %q, want %q", got, want)
	}

	updated, cmd = loaded.Update(bubbletea.KeyMsg{Type: bubbletea.KeyEnter})
	if cmd != nil {
		t.Fatalf("BrowserModel.Update(enter on cached resource) command = %v, want nil", cmd)
	}
	if got := len(loader.calls); got != 1 {
		t.Fatalf("loader calls after cached enter = %d, want 1", got)
	}

	updated, cmd = updated.(BrowserModel).Update(bubbletea.KeyMsg{Type: bubbletea.KeyRunes, Runes: []rune{'r'}})
	if cmd == nil {
		t.Fatal("BrowserModel.Update(r on cached resource) command = nil, want refresh command")
	}
	_, _ = updated.(BrowserModel).Update(cmd())
	if got := len(loader.calls); got != 2 {
		t.Fatalf("loader calls after refresh = %d, want 2", got)
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
	m = step(m, bubbletea.WindowSizeMsg{Width: 80, Height: 10})
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

func browserDataWithResources(count int) data.BrowserData {
	resources := make([]data.ResourceNode, count)
	for i := range resources {
		name := fmt.Sprintf("resource-%03d", i)
		resources[i] = data.ResourceNode{
			Product: "zia",
			Name:    name,
			State:   data.ResourceStateUnloaded,
		}
	}
	return data.BrowserData{
		Products: []data.ProductNode{
			{Name: "zia", Resources: resources},
		},
	}
}

func browserDataWithRecords(count int) data.BrowserData {
	records := make([]data.RecordSummary, count)
	for i := range records {
		records[i] = data.RecordSummary{
			ID:     fmt.Sprintf("%d", i),
			Name:   fmt.Sprintf("rec-%04d", i),
			Status: "active",
			Fields: []data.KV{
				{Key: "index", Value: fmt.Sprintf("%d", i)},
			},
		}
	}
	return data.BrowserData{
		Products: []data.ProductNode{
			{
				Name: "zia",
				Resources: []data.ResourceNode{
					{Product: "zia", Name: "locations", Records: records},
				},
			},
		},
	}
}

func browserDataWithLargeRecordBody(fieldCount int) data.BrowserData {
	fields := make([]data.KV, fieldCount)
	for i := range fields {
		fields[i] = data.KV{
			Key:   fmt.Sprintf("field_%02d", i),
			Value: fmt.Sprintf("value_%02d", i),
		}
	}
	return data.BrowserData{
		Products: []data.ProductNode{
			{
				Name: "zia",
				Resources: []data.ResourceNode{
					{
						Product: "zia",
						Name:    "large-body",
						Records: []data.RecordSummary{
							{
								ID:     "1",
								Name:   "Large body",
								Status: "active",
								Fields: fields,
							},
						},
					},
				},
			},
		},
	}
}

func assertViewportSelectionVisible(t *testing.T, label string, viewport viewportState, total, height int) {
	t.Helper()
	start, end := viewport.VisibleRange(total, height)
	if total == 0 {
		if viewport.Selected != 0 || viewport.Offset != 0 {
			t.Fatalf("%s = %+v for empty list, want zero selection and offset", label, viewport)
		}
		return
	}
	if viewport.Selected < start || viewport.Selected >= end {
		t.Fatalf("%s selected index = %d outside visible range [%d,%d) for height %d", label, viewport.Selected, start, end, height)
	}
}

func assertViewLineWidths(t *testing.T, view string, width int) {
	t.Helper()
	for lineNumber, line := range strings.Split(strings.TrimSuffix(view, "\n"), "\n") {
		if got := lipgloss.Width(line); got > width {
			t.Fatalf("View() line %d width = %d, want <= %d: %q", lineNumber+1, got, width, line)
		}
	}
}

func step(m BrowserModel, msg bubbletea.Msg) BrowserModel {
	updated, _ := m.Update(msg)
	return updated.(BrowserModel)
}

type recordingResourceLoader struct {
	nodes          map[string]data.ResourceNode
	waitForContext bool
	calls          []string
}

func (l *recordingResourceLoader) LoadResource(ctx context.Context, product, resource string) data.ResourceNode {
	key := product + "/" + resource
	l.calls = append(l.calls, key)
	if l.waitForContext {
		<-ctx.Done()
		return data.ResourceNode{
			Product: product,
			Name:    resource,
			State:   data.ResourceStateError,
			Error:   ctx.Err().Error(),
		}
	}
	if node, ok := l.nodes[key]; ok {
		return node
	}
	return data.ResourceNode{
		Product: product,
		Name:    resource,
		State:   data.ResourceStateLoaded,
		Empty:   true,
	}
}

func lazyBrowserData() data.BrowserData {
	return data.BrowserData{
		Products: []data.ProductNode{
			{
				Name: "zia",
				Resources: []data.ResourceNode{
					{Product: "zia", Name: "locations", State: data.ResourceStateUnloaded},
					{Product: "zia", Name: "url-filtering-rules", State: data.ResourceStateUnloaded},
				},
			},
		},
	}
}
