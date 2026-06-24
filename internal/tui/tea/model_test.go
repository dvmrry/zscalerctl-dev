package tea_test

import (
	"strings"
	"testing"

	bubbletea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/dvmrry/zscalerctl/internal/output"
	tea "github.com/dvmrry/zscalerctl/internal/tui/tea"
)

func TestDemoModelViewTracksWindowSize(t *testing.T) {
	model := tea.NewDemoModel(output.Style{})

	updated, cmd := model.Update(bubbletea.WindowSizeMsg{Width: 60, Height: 16})
	if cmd != nil {
		t.Fatalf("DemoModel.Update(WindowSizeMsg) command = %v, want nil", cmd)
	}
	sized, ok := updated.(tea.DemoModel)
	if !ok {
		t.Fatalf("DemoModel.Update(WindowSizeMsg) model = %T, want tea.DemoModel", updated)
	}
	if gotWidth, gotHeight := sized.Size(); gotWidth != 60 || gotHeight != 16 {
		t.Errorf("DemoModel.Size() = (%d, %d), want (60, 16)", gotWidth, gotHeight)
	}

	view := sized.View().Content
	if !strings.Contains(view, "terminal: 60x16") {
		t.Errorf("DemoModel.View().Content = %q, want terminal dimensions", view)
	}
	for _, line := range strings.Split(strings.TrimSuffix(view, "\n"), "\n") {
		if got := lipgloss.Width(line); got > 60 {
			t.Errorf("lipgloss.Width(%q) = %d, want <= 60", line, got)
		}
	}
}

func TestDemoModelColorRendering(t *testing.T) {
	colorView := tea.NewDemoModel(output.Style{Color: true, Color256: true}).View().Content
	if !strings.Contains(colorView, "\x1b[") {
		t.Errorf("DemoModel.View().Content with color = %q, want ANSI escape", colorView)
	}

	plainView := tea.NewDemoModel(output.Style{}).View().Content
	if strings.Contains(plainView, "\x1b[") {
		t.Errorf("DemoModel.View().Content without color = %q, want no ANSI escape", plainView)
	}
	if !strings.Contains(plainView, "style: monochrome render") {
		t.Errorf("DemoModel.View().Content without color = %q, want monochrome label", plainView)
	}
}

func TestDemoModelQuitKeys(t *testing.T) {
	tests := []struct {
		name string
		msg  bubbletea.Msg
		want string
	}{
		{name: "q", msg: keyText("q"), want: "q"},
		{name: "esc", msg: keyCode(bubbletea.KeyEsc), want: "esc"},
		{name: "ctrl c", msg: keyCtrlC(), want: "ctrl+c"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := tea.NewDemoModel(output.Style{})

			updated, cmd := model.Update(tt.msg)
			if cmd == nil {
				t.Fatalf("DemoModel.Update(%q) command = nil, want quit command", tt.want)
			}
			if got := cmd(); got != bubbletea.Quit() {
				t.Errorf("DemoModel.Update(%q) command() = %#v, want %#v", tt.want, got, bubbletea.Quit())
			}
			quitModel, ok := updated.(tea.DemoModel)
			if !ok {
				t.Fatalf("DemoModel.Update(%q) model = %T, want tea.DemoModel", tt.want, updated)
			}
			if got := quitModel.ExitKey(); got != tt.want {
				t.Errorf("DemoModel.ExitKey() = %q, want %q", got, tt.want)
			}
		})
	}
}

func keyText(s string) bubbletea.KeyPressMsg {
	runes := []rune(s)
	code := rune(0)
	if len(runes) > 0 {
		code = runes[0]
	}
	return bubbletea.KeyPressMsg(bubbletea.Key{Text: s, Code: code})
}

func keyCode(code rune) bubbletea.KeyPressMsg {
	return bubbletea.KeyPressMsg(bubbletea.Key{Code: code})
}

func keyCtrlC() bubbletea.KeyPressMsg {
	return bubbletea.KeyPressMsg(bubbletea.Key{Code: 'c', Mod: bubbletea.ModCtrl})
}
