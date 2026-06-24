package tui_test

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/dvmrry/zscalerctl/internal/output"
	"github.com/dvmrry/zscalerctl/internal/tui"
)

func TestDemoModelViewTracksWindowSize(t *testing.T) {
	model := tui.NewDemoModel(output.Style{})

	updated, cmd := model.Update(tea.WindowSizeMsg{Width: 60, Height: 16})
	if cmd != nil {
		t.Fatalf("DemoModel.Update(WindowSizeMsg) command = %v, want nil", cmd)
	}
	sized, ok := updated.(tui.DemoModel)
	if !ok {
		t.Fatalf("DemoModel.Update(WindowSizeMsg) model = %T, want tui.DemoModel", updated)
	}
	if gotWidth, gotHeight := sized.Size(); gotWidth != 60 || gotHeight != 16 {
		t.Errorf("DemoModel.Size() = (%d, %d), want (60, 16)", gotWidth, gotHeight)
	}

	view := sized.View()
	if !strings.Contains(view, "terminal: 60x16") {
		t.Errorf("DemoModel.View() = %q, want terminal dimensions", view)
	}
	for _, line := range strings.Split(strings.TrimSuffix(view, "\n"), "\n") {
		if got := lipgloss.Width(line); got > 60 {
			t.Errorf("lipgloss.Width(%q) = %d, want <= 60", line, got)
		}
	}
}

func TestDemoModelColorRendering(t *testing.T) {
	colorView := tui.NewDemoModel(output.Style{Color: true, Color256: true}).View()
	if !strings.Contains(colorView, "\x1b[") {
		t.Errorf("DemoModel.View() with color = %q, want ANSI escape", colorView)
	}

	plainView := tui.NewDemoModel(output.Style{}).View()
	if strings.Contains(plainView, "\x1b[") {
		t.Errorf("DemoModel.View() without color = %q, want no ANSI escape", plainView)
	}
	if !strings.Contains(plainView, "style: monochrome render") {
		t.Errorf("DemoModel.View() without color = %q, want monochrome label", plainView)
	}
}

func TestDemoModelQuitKeys(t *testing.T) {
	tests := []struct {
		name string
		msg  tea.KeyMsg
		want string
	}{
		{name: "q", msg: tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}, want: "q"},
		{name: "esc", msg: tea.KeyMsg{Type: tea.KeyEsc}, want: "esc"},
		{name: "ctrl c", msg: tea.KeyMsg{Type: tea.KeyCtrlC}, want: "ctrl+c"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := tui.NewDemoModel(output.Style{})

			updated, cmd := model.Update(tt.msg)
			if cmd == nil {
				t.Fatalf("DemoModel.Update(%q) command = nil, want quit command", tt.want)
			}
			if got := cmd(); got != tea.Quit() {
				t.Errorf("DemoModel.Update(%q) command() = %#v, want %#v", tt.want, got, tea.Quit())
			}
			quitModel, ok := updated.(tui.DemoModel)
			if !ok {
				t.Fatalf("DemoModel.Update(%q) model = %T, want tui.DemoModel", tt.want, updated)
			}
			if got := quitModel.ExitKey(); got != tt.want {
				t.Errorf("DemoModel.ExitKey() = %q, want %q", got, tt.want)
			}
		})
	}
}
