// Package tea holds the Bubble Tea runtime model for the isolated TUI demo.
//
// This package is the only place in the project that may import
// github.com/charmbracelet/bubbletea. Normal CLI startup packages (cmd/,
// internal/cli/) and the gate-only internal/tui package must not import it,
// because Bubble Tea's package initialization can emit terminal probes before
// the TUI eligibility gate is evaluated. The demo entry point in
// scripts/tui-demo.go imports this package and starts a program only after the
// gate has explicitly allowed it.
package tea

import (
	"fmt"
	"io"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/dvmrry/zscalerctl/internal/output"
)

// DemoModel is a development-only TUI model used to evaluate Bubble Tea
// rendering, sizing, and exit behavior before wiring any product command.
type DemoModel struct {
	style   output.Style
	width   int
	height  int
	exitKey string
}

var _ tea.Model = DemoModel{}

// NewDemoModel returns a minimal demo model with no config, credential, or
// network dependencies.
func NewDemoModel(style output.Style) DemoModel {
	return DemoModel{style: style}
}

func (m DemoModel) Init() tea.Cmd {
	return nil
}

func (m DemoModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc", "q":
			m.exitKey = msg.String()
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}
	return m, nil
}

func (m DemoModel) View() string {
	width, height := m.dimensions()
	r := demoRenderer(m.style)

	titleStyle := r.NewStyle().Bold(true)
	keyStyle := r.NewStyle().Bold(true)
	status := "Bubble Tea running"
	if m.style.Color {
		titleStyle = titleStyle.Foreground(demoAccent(m.style))
		keyStyle = keyStyle.Foreground(demoAccent(m.style))
		status = m.style.Value("success", status)
	}

	lines := []string{
		titleStyle.Render("zscalerctl TUI demo"),
		"",
		fmt.Sprintf("status: %s", status),
		fmt.Sprintf("terminal: %dx%d", width, height),
		fmt.Sprintf("style: %s", demoStyleLabel(m.style)),
		"",
		fmt.Sprintf("keys: %s quits | %s quits | %s quits",
			keyStyle.Render("q"),
			keyStyle.Render("esc"),
			keyStyle.Render("ctrl+c"),
		),
	}

	panel := r.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(demoBorderColor(m.style)).
		Padding(0, 1).
		Width(demoContentWidth(width)).
		Render(strings.Join(lines, "\n"))

	return panel + "\n"
}

// Size returns the most recent terminal dimensions reported by Bubble Tea.
func (m DemoModel) Size() (int, int) {
	return m.width, m.height
}

// ExitKey returns the key that requested shutdown, if any.
func (m DemoModel) ExitKey() string {
	return m.exitKey
}

func (m DemoModel) dimensions() (int, int) {
	width := m.width
	if width <= 0 {
		width = 80
	}
	height := m.height
	if height <= 0 {
		height = 24
	}
	return width, height
}

func demoContentWidth(terminalWidth int) int {
	if terminalWidth <= 0 {
		return 72
	}
	width := terminalWidth - 4
	if width < 12 {
		return 12
	}
	if width > 72 {
		return 72
	}
	return width
}

func demoStyleLabel(style output.Style) string {
	if !style.Color {
		return "monochrome render"
	}
	if style.Color256 {
		return "256-color render"
	}
	return "basic-color render"
}

func demoRenderer(style output.Style) *lipgloss.Renderer {
	r := lipgloss.NewRenderer(io.Discard)
	switch {
	case !style.Color:
		r.SetColorProfile(termenv.Ascii)
	case style.Color256:
		r.SetColorProfile(termenv.ANSI256)
	default:
		r.SetColorProfile(termenv.ANSI)
	}
	return r
}

func demoAccent(style output.Style) lipgloss.Color {
	if style.Color256 {
		return lipgloss.Color("45")
	}
	return lipgloss.Color("6")
}

func demoBorderColor(style output.Style) lipgloss.Color {
	if !style.Color {
		return ""
	}
	if style.Color256 {
		return lipgloss.Color("240")
	}
	return lipgloss.Color("8")
}
