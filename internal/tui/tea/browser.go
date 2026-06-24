// Package tea holds the Bubble Tea runtime model for the isolated TUI demo.
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

// BrowserModel is a static/fake-data product/resource browser used to prove the
// TUI product shape before connecting to real CLI data. It contains no config,
// credential, network, or live reader dependencies.
type BrowserModel struct {
	style   output.Style
	width   int
	height  int
	items   []browserItem
	idx     int
	active  string // "left" or "right"
	rIdx    int
	exitKey string
}

// browserItem is a single row in the left navigation pane.
type browserItem struct {
	name    string
	kind    string // "product" or "resource"
	depth   int
	records []browserRecord
	empty   bool
	err     string
}

// browserRecord is a fake record shown in the right pane.
type browserRecord struct {
	name   string
	id     string
	status string
	detail string
}

var _ tea.Model = BrowserModel{}

// NewBrowserModel returns a static browser model with fake data.
func NewBrowserModel(style output.Style) BrowserModel {
	return BrowserModel{
		style:  style,
		items:  fakeBrowserItems(),
		idx:    0,
		active: "left",
	}
}

func (m BrowserModel) Init() tea.Cmd {
	return nil
}

func (m BrowserModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc", "q":
			m.exitKey = msg.String()
			return m, tea.Quit
		case "up":
			if m.active == "left" {
				if m.idx > 0 {
					m.idx--
					m.rIdx = 0
				}
			} else {
				if m.rIdx > 0 {
					m.rIdx--
				}
			}
		case "down":
			if m.active == "left" {
				if m.idx < len(m.items)-1 {
					m.idx++
					m.rIdx = 0
				}
			} else {
				if m.rIdx < len(m.items[m.idx].records)-1 {
					m.rIdx++
				}
			}
		case "tab":
			if m.active == "left" {
				m.active = "right"
				if m.rIdx >= len(m.items[m.idx].records) {
					m.rIdx = 0
				}
			} else {
				m.active = "left"
			}
		case "enter":
			// Enter confirms the current selection; on the right pane this resets
			// the record index to the top of the current list.
			if m.active == "right" {
				m.rIdx = 0
			}
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}
	return m, nil
}

func (m BrowserModel) View() string {
	width, height := m.dimensions()
	r := browserRenderer(m.style)

	footerHeight := 2
	bodyHeight := height - footerHeight
	if bodyHeight < 6 {
		bodyHeight = 6
	}

	leftWidth, rightWidth, stacked := browserPaneWidths(width)

	var leftPane, rightPane string
	if stacked {
		leftHeight := bodyHeight / 2
		if leftHeight < 5 {
			leftHeight = 5
		}
		rightHeight := bodyHeight - leftHeight
		if rightHeight < 4 {
			rightHeight = 4
			leftHeight = bodyHeight - rightHeight
		}
		leftPane = m.renderLeftPane(r, leftWidth, leftHeight)
		rightPane = m.renderRightPane(r, rightWidth, rightHeight)
	} else {
		leftPane = m.renderLeftPane(r, leftWidth, bodyHeight)
		rightPane = m.renderRightPane(r, rightWidth, bodyHeight)
	}

	var body string
	if stacked {
		body = lipgloss.JoinVertical(lipgloss.Top, leftPane, rightPane)
	} else {
		body = lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane)
	}

	footer := m.renderFooter(r, width)
	return body + "\n" + footer + "\n"
}

// ActivePane reports which pane currently has focus.
func (m BrowserModel) ActivePane() string {
	return m.active
}

// SelectedIndex reports the selected left-pane index.
func (m BrowserModel) SelectedIndex() int {
	return m.idx
}

// RecordIndex reports the selected right-pane record index.
func (m BrowserModel) RecordIndex() int {
	return m.rIdx
}

// ExitKey returns the key that requested shutdown, if any.
func (m BrowserModel) ExitKey() string {
	return m.exitKey
}

// Size returns the most recent terminal dimensions reported by Bubble Tea.
func (m BrowserModel) Size() (int, int) {
	return m.width, m.height
}

func (m BrowserModel) dimensions() (int, int) {
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

func (m BrowserModel) renderLeftPane(r *lipgloss.Renderer, width, height int) string {
	style := r.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(browserBorderColor(m.style, m.active == "left")).
		Padding(0, 1).
		Width(width - 2). // subtract border
		Height(height - 2)

	var lines []string
	title := r.NewStyle().Bold(true).Render("Products / Resources")
	lines = append(lines, title, "")

	for i, item := range m.items {
		prefix := strings.Repeat("  ", item.depth)
		label := prefix + item.name
		if item.kind == "product" {
			label = r.NewStyle().Bold(true).Render(label)
		} else {
			label = r.NewStyle().Render(label)
		}
		if i == m.idx {
			label = browserSelectedStyle(m.style).Render(label)
		}
		lines = append(lines, label)
	}

	return style.Render(strings.Join(lines, "\n"))
}

func (m BrowserModel) renderRightPane(r *lipgloss.Renderer, width, height int) string {
	style := r.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(browserBorderColor(m.style, m.active == "right")).
		Padding(0, 1).
		Width(width - 2).
		Height(height - 2)

	item := m.items[m.idx]
	var lines []string

	title := r.NewStyle().Bold(true).Render(item.name)
	lines = append(lines, title, "")

	switch {
	case item.kind == "product":
		resourceCount := 0
		for i := m.idx + 1; i < len(m.items) && m.items[i].depth > 0; i++ {
			resourceCount++
		}
		lines = append(lines, fmt.Sprintf("Product: %s", item.name))
		lines = append(lines, fmt.Sprintf("Resources: %d", resourceCount))

	case item.err != "":
		lines = append(lines, browserErrorStyle(m.style).Render("Error: "+item.err))

	case item.empty:
		lines = append(lines, browserEmptyStyle(m.style).Render("No records"))

	case len(item.records) == 0:
		lines = append(lines, browserEmptyStyle(m.style).Render("No records"))

	default:
		for i, rec := range item.records {
			recLine := fmt.Sprintf("  %s (id=%s, status=%s)", rec.name, rec.id, rec.status)
			if i == m.rIdx && m.active == "right" {
				recLine = browserSelectedStyle(m.style).Render(recLine)
			}
			lines = append(lines, recLine)
			if rec.detail != "" {
				lines = append(lines, "    "+rec.detail)
			}
		}
	}

	return style.Render(strings.Join(lines, "\n"))
}

func (m BrowserModel) renderFooter(r *lipgloss.Renderer, width int) string {
	help := "↑/↓ move · tab switch pane · enter select · esc/q quit"
	return r.NewStyle().
		Width(width - 2).
		Render(help)
}

func fakeBrowserItems() []browserItem {
	return []browserItem{
		{name: "zia", kind: "product", depth: 0},
		{
			name:    "locations",
			kind:    "resource",
			depth:   1,
			records: []browserRecord{{name: "HQ", id: "123", status: "active", detail: "US East"}, {name: "Branch", id: "124", status: "active", detail: "EU West"}, {name: "Remote", id: "125", status: "inactive", detail: "APAC"}},
		},
		{
			name:    "url-filtering-rules",
			kind:    "resource",
			depth:   1,
			records: []browserRecord{{name: "Social", id: "501", status: "active", detail: "block social"}, {name: "Streaming", id: "502", status: "active", detail: "allow streaming"}},
		},
		{name: "forwarding-rules", kind: "resource", depth: 1, empty: true},
		{name: "zpa", kind: "product", depth: 0},
		{
			name:    "app-segments",
			kind:    "resource",
			depth:   1,
			records: []browserRecord{{name: "Engineering", id: "901", status: "active", detail: "10 apps"}, {name: "Finance", id: "902", status: "active", detail: "5 apps"}},
		},
		{name: "connectors", kind: "resource", depth: 1, err: "connector list unavailable"},
		{name: "zcc", kind: "product", depth: 0},
		{name: "devices", kind: "resource", depth: 1, empty: true},
	}
}

func browserPaneWidths(width int) (left, right int, stacked bool) {
	if width < 70 {
		return width, width, true
	}
	left = width / 3
	if left < 24 {
		left = 24
	}
	right = width - left
	return left, right, false
}

func browserRenderer(style output.Style) *lipgloss.Renderer {
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

func browserSelectedStyle(style output.Style) lipgloss.Style {
	s := lipgloss.NewStyle()
	if style.Color {
		s = s.Background(browserAccent(style)).Foreground(lipgloss.Color("0"))
	}
	return s
}

func browserEmptyStyle(style output.Style) lipgloss.Style {
	s := lipgloss.NewStyle()
	if style.Color {
		s = s.Foreground(lipgloss.Color("245"))
	}
	return s
}

func browserErrorStyle(style output.Style) lipgloss.Style {
	s := lipgloss.NewStyle()
	if style.Color {
		s = s.Foreground(lipgloss.Color("196"))
	}
	return s
}

func browserAccent(style output.Style) lipgloss.Color {
	if style.Color256 {
		return lipgloss.Color("45")
	}
	return lipgloss.Color("6")
}

func browserBorderColor(style output.Style, active bool) lipgloss.Color {
	if !style.Color {
		return ""
	}
	if active {
		if style.Color256 {
			return lipgloss.Color("45")
		}
		return lipgloss.Color("6")
	}
	if style.Color256 {
		return lipgloss.Color("240")
	}
	return lipgloss.Color("8")
}
