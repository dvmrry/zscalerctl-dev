package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type StaticModel struct {
	title  string
	body   []string
	width  int
	height int
}

var _ tea.Model = StaticModel{}

func NewStaticModel(title string, body ...string) StaticModel {
	return StaticModel{
		title: title,
		body:  append([]string(nil), body...),
	}
}

func (m StaticModel) Init() tea.Cmd {
	return nil
}

func (m StaticModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}
	return m, nil
}

func (m StaticModel) View() string {
	var lines []string
	if m.title != "" {
		lines = append(lines, m.title)
	}
	lines = append(lines, m.body...)
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n") + "\n"
}

func (m StaticModel) Size() (int, int) {
	return m.width, m.height
}
