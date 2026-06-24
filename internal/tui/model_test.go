package tui_test

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dvmrry/zscalerctl/internal/tui"
)

func TestStaticModelView(t *testing.T) {
	model := tui.NewStaticModel("Inventory", "locations", "rules")

	if got, want := model.View(), "Inventory\nlocations\nrules\n"; got != want {
		t.Errorf("StaticModel.View() = %q, want %q", got, want)
	}
}

func TestStaticModelTracksWindowSize(t *testing.T) {
	model := tui.NewStaticModel("Inventory")

	updated, cmd := model.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	if cmd != nil {
		t.Fatalf("StaticModel.Update(WindowSizeMsg) command = %v, want nil", cmd)
	}
	sized, ok := updated.(tui.StaticModel)
	if !ok {
		t.Fatalf("StaticModel.Update(WindowSizeMsg) model = %T, want tui.StaticModel", updated)
	}
	if gotWidth, gotHeight := sized.Size(); gotWidth != 120 || gotHeight != 40 {
		t.Errorf("StaticModel.Size() = (%d, %d), want (120, 40)", gotWidth, gotHeight)
	}
}

func TestStaticModelQuitKeys(t *testing.T) {
	for _, msg := range []tea.KeyMsg{
		{Type: tea.KeyCtrlC},
		{Type: tea.KeyRunes, Runes: []rune{'q'}},
	} {
		model := tui.NewStaticModel("Inventory")

		_, cmd := model.Update(msg)
		if cmd == nil {
			t.Fatalf("StaticModel.Update(%q) command = nil, want quit command", msg.String())
		}
		if got := cmd(); got != tea.Quit() {
			t.Errorf("StaticModel.Update(%q) command() = %#v, want %#v", msg.String(), got, tea.Quit())
		}
	}
}
