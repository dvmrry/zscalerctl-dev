// Package tea holds the Bubble Tea runtime model for the isolated TUI demo.
package tea

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/dvmrry/zscalerctl/internal/output"
	"github.com/dvmrry/zscalerctl/internal/tui/data"
)

const defaultResourceLoadTimeout = 30 * time.Second

// ResourceLoader loads one resource on demand for lazy live browsing.
type ResourceLoader interface {
	LoadResource(ctx context.Context, product, resource string) data.ResourceNode
}

// ResourceLoaderFunc adapts a function into a ResourceLoader.
type ResourceLoaderFunc func(ctx context.Context, product, resource string) data.ResourceNode

// LoadResource calls f(ctx, product, resource).
func (f ResourceLoaderFunc) LoadResource(ctx context.Context, product, resource string) data.ResourceNode {
	return f(ctx, product, resource)
}

// BrowserModel is a product/resource browser that renders a neutral
// BrowserData view model. Live loading is injected through ResourceLoader so
// config, credential, and reader ownership stay outside the Bubble Tea package.
type BrowserModel struct {
	style       output.Style
	width       int
	height      int
	data        data.BrowserData
	items       []browserItem
	idx         int
	active      string // "left" or "right"
	rIdx        int
	scroll      int
	showHelp    bool
	exitKey     string
	loader      ResourceLoader
	loadTimeout time.Duration
}

// browserItem is a single row in the left navigation pane.
type browserItem struct {
	name    string
	kind    string // "product" or "resource"
	depth   int
	state   data.ResourceState
	records []data.RecordSummary
	empty   bool
	err     string
	Product string
}

func (i browserItem) effectiveState() data.ResourceState {
	if i.state != "" {
		return i.state
	}
	if i.err != "" {
		return data.ResourceStateError
	}
	return data.ResourceStateLoaded
}

type resourceLoadedMsg struct {
	product string
	name    string
	node    data.ResourceNode
}

var _ tea.Model = BrowserModel{}

// NewBrowserModel returns a browser model that renders the supplied BrowserData.
func NewBrowserModel(style output.Style, browserData data.BrowserData) BrowserModel {
	return BrowserModel{
		style:  style,
		data:   browserData,
		items:  flattenBrowserData(browserData),
		idx:    0,
		active: "left",
	}
}

// NewLazyBrowserModel returns a browser model that loads resources on demand.
func NewLazyBrowserModel(style output.Style, browserData data.BrowserData, loader ResourceLoader, loadTimeout time.Duration) BrowserModel {
	m := NewBrowserModel(style, browserData)
	m.loader = loader
	m.loadTimeout = loadTimeout
	return m
}

// Data returns the BrowserData currently being rendered.
func (m BrowserModel) Data() data.BrowserData {
	return m.data
}

func (m BrowserModel) Init() tea.Cmd {
	return nil
}

func (m BrowserModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.showHelp {
		if key, ok := msg.(tea.KeyMsg); ok {
			m.showHelp = false
			// Quit keys should pass through and exit immediately.
			if key.String() == "ctrl+c" || key.String() == "esc" || key.String() == "q" {
				m.exitKey = key.String()
				return m, tea.Quit
			}
			return m, nil
		}
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc", "q":
			m.exitKey = msg.String()
			return m, tea.Quit
		case "?":
			m.showHelp = true
		case "up":
			if m.active == "left" {
				if m.idx > 0 {
					m.idx--
					m.rIdx = 0
					m.scroll = 0
				}
			} else {
				if m.rIdx > 0 {
					m.rIdx--
					m.adjustScrollToRecord()
				}
			}
		case "down":
			if m.active == "left" {
				if m.idx < len(m.items)-1 {
					m.idx++
					m.rIdx = 0
					m.scroll = 0
				}
			} else {
				if m.rIdx < len(m.items[m.idx].records)-1 {
					m.rIdx++
					m.adjustScrollToRecord()
				}
			}
		case "tab":
			if m.active == "left" {
				m.active = "right"
				if m.rIdx >= len(m.items[m.idx].records) {
					m.rIdx = 0
				}
				m.adjustScrollToRecord()
			} else {
				m.active = "left"
			}
		case "enter":
			if m.selectedItem().kind == "resource" {
				return m.startResourceLoad(false)
			}
			if m.active == "right" {
				m.resetRecordSelection()
			}
		case "r":
			return m.startResourceLoad(true)
		}
	case resourceLoadedMsg:
		m.applyResourceNode(msg.node)
		m.resetRecordSelection()
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}
	return m, nil
}

func (m BrowserModel) selectedItem() browserItem {
	if len(m.items) == 0 || m.idx < 0 || m.idx >= len(m.items) {
		return browserItem{}
	}
	return m.items[m.idx]
}

func (m BrowserModel) startResourceLoad(refresh bool) (BrowserModel, tea.Cmd) {
	item := m.selectedItem()
	if item.kind != "resource" || m.loader == nil {
		return m, nil
	}
	switch item.effectiveState() {
	case data.ResourceStateLoading:
		return m, nil
	case data.ResourceStateLoaded:
		if !refresh {
			return m, nil
		}
	}

	loading := data.ResourceNode{
		Product: item.Product,
		Name:    item.name,
		State:   data.ResourceStateLoading,
	}
	m.applyResourceNode(loading)
	m.resetRecordSelection()
	return m, m.loadResourceCmd(item.Product, item.name)
}

func (m BrowserModel) loadResourceCmd(product, name string) tea.Cmd {
	loader := m.loader
	timeout := m.loadTimeout
	if timeout <= 0 {
		timeout = defaultResourceLoadTimeout
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		node := loader.LoadResource(ctx, product, name)
		node = normalizeResourceNode(node, product, name)
		return resourceLoadedMsg{
			product: product,
			name:    name,
			node:    node,
		}
	}
}

func normalizeResourceNode(node data.ResourceNode, product, name string) data.ResourceNode {
	if node.Product == "" {
		node.Product = product
	}
	if node.Name == "" {
		node.Name = name
	}
	switch node.EffectiveState() {
	case data.ResourceStateUnloaded, data.ResourceStateLoading, data.ResourceStateLoaded, data.ResourceStateError:
		node.State = node.EffectiveState()
	default:
		node.State = data.ResourceStateLoaded
	}
	if node.State == data.ResourceStateError && node.Error == "" {
		node.Error = "resource load failed"
	}
	if node.State == data.ResourceStateLoaded && len(node.Records) == 0 {
		node.Empty = true
	}
	return node
}

func (m *BrowserModel) applyResourceNode(node data.ResourceNode) {
	node = normalizeResourceNode(node, node.Product, node.Name)
	for pIdx := range m.data.Products {
		if m.data.Products[pIdx].Name != node.Product {
			continue
		}
		for rIdx := range m.data.Products[pIdx].Resources {
			if m.data.Products[pIdx].Resources[rIdx].Name == node.Name {
				m.data.Products[pIdx].Resources[rIdx] = node
				m.items = flattenBrowserData(m.data)
				if m.idx >= len(m.items) {
					m.idx = len(m.items) - 1
				}
				if m.idx < 0 {
					m.idx = 0
				}
				return
			}
		}
	}
}

func (m *BrowserModel) resetRecordSelection() {
	m.rIdx = 0
	m.scroll = 0
}

func (m *BrowserModel) adjustScrollToRecord() {
	records := m.items[m.idx].records
	if m.rIdx < 0 {
		m.rIdx = 0
	}
	if len(records) == 0 {
		m.rIdx = 0
		m.scroll = 0
		return
	}
	if m.rIdx >= len(records) {
		m.rIdx = len(records) - 1
	}
	m.scroll = m.recordStartLine(m.rIdx)
}

func (m BrowserModel) recordStartLine(rIdx int) int {
	item := m.items[m.idx]
	if item.kind != "resource" || rIdx < 0 || rIdx >= len(item.records) {
		return 0
	}
	start := 2 // title + blank line
	for i := 0; i < rIdx && i < len(item.records); i++ {
		rec := item.records[i]
		start++ // record header
		if rec.Detail != "" {
			start++
		}
		start += len(rec.Fields)
	}
	return start
}

func (m BrowserModel) View() string {
	width, height := m.dimensions()
	r := browserRenderer(m.style)

	statusHeight := 1
	footerHeight := 2
	bodyHeight := height - statusHeight - footerHeight
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

	if m.showHelp {
		body = m.renderHelpOverlay(r, width, height-statusHeight-1)
	}

	status := m.renderStatus(r, width)
	footer := m.renderFooter(r, width)
	return body + "\n" + status + "\n" + footer + "\n"
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

// ScrollOffset reports the right-pane line scroll offset.
func (m BrowserModel) ScrollOffset() int {
	return m.scroll
}

// ShowingHelp reports whether the help overlay is visible.
func (m BrowserModel) ShowingHelp() bool {
	return m.showHelp
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
		Width(width - 2).
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

	content := m.rightPaneContent(r)
	visible := m.visibleLines(content, height-2)

	return style.Render(strings.Join(visible, "\n"))
}

func (m BrowserModel) rightPaneContent(r *lipgloss.Renderer) []string {
	if len(m.items) == 0 {
		return []string{
			r.NewStyle().Bold(true).Render("No resources"),
			"",
			browserEmptyStyle(m.style).Render("No resources match the current filters."),
		}
	}
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

	case item.effectiveState() == data.ResourceStateUnloaded:
		lines = append(lines, "")
		lines = append(lines, browserEmptyStyle(m.style).Render("Resource not loaded"))
		lines = append(lines, "")
		lines = append(lines, browserEmptyStyle(m.style).Render("Press enter to load this resource."))

	case item.effectiveState() == data.ResourceStateLoading:
		lines = append(lines, "")
		lines = append(lines, browserLoadingStyle(m.style).Render("Loading resource..."))
		lines = append(lines, "")
		lines = append(lines, browserEmptyStyle(m.style).Render("The API call is running for this resource only."))

	case item.effectiveState() == data.ResourceStateError:
		lines = append(lines, "")
		lines = append(lines, browserErrorStyle(m.style).Render("Error loading resource"))
		lines = append(lines, browserErrorStyle(m.style).Render(item.err))
		lines = append(lines, "")
		lines = append(lines, browserEmptyStyle(m.style).Render("Press enter to retry or r to refresh."))

	case item.empty || len(item.records) == 0:
		lines = append(lines, "")
		lines = append(lines, browserEmptyStyle(m.style).Render("No records for this resource"))
		lines = append(lines, "")
		lines = append(lines, browserEmptyStyle(m.style).Render("Press r to refresh or select a different resource."))

	default:
		for i, rec := range item.records {
			recLine := fmt.Sprintf("  %s (id=%s, status=%s)", rec.Name, rec.ID, rec.Status)
			if i == m.rIdx && m.active == "right" {
				recLine = browserSelectedStyle(m.style).Render(recLine)
			}
			lines = append(lines, recLine)
			if rec.Detail != "" {
				lines = append(lines, "    "+rec.Detail)
			}
			for _, f := range rec.Fields {
				lines = append(lines, fmt.Sprintf("    %s: %s", f.Key, f.Value))
			}
		}
	}

	return lines
}

func (m BrowserModel) visibleLines(lines []string, maxLines int) []string {
	if maxLines <= 0 {
		return []string{}
	}
	if m.scroll < 0 {
		m.scroll = 0
	}
	if m.scroll >= len(lines) {
		m.scroll = len(lines) - 1
		if m.scroll < 0 {
			m.scroll = 0
		}
	}
	end := m.scroll + maxLines
	if end > len(lines) {
		end = len(lines)
	}
	return lines[m.scroll:end]
}

func (m BrowserModel) renderStatus(r *lipgloss.Renderer, width int) string {
	if len(m.items) == 0 {
		return r.NewStyle().
			Width(width - 2).
			Render("no resources")
	}
	item := m.items[m.idx]
	var selected string
	if item.kind == "product" {
		selected = item.name
	} else {
		selected = item.Product + " / " + item.name
	}
	status := fmt.Sprintf("%s · %d/%d", selected, m.idx+1, len(m.items))
	if item.kind == "resource" {
		switch item.effectiveState() {
		case data.ResourceStateUnloaded:
			status += " · unloaded"
		case data.ResourceStateLoading:
			status += " · loading"
		case data.ResourceStateError:
			status += " · error"
		default:
			status += fmt.Sprintf(" · %d records", len(item.records))
		}
	}
	return r.NewStyle().
		Width(width - 2).
		Render(status)
}

func (m BrowserModel) renderFooter(r *lipgloss.Renderer, width int) string {
	help := "↑/↓ move · tab switch · enter load/select · r refresh · ? help · esc/q quit"
	return r.NewStyle().
		Width(width - 2).
		Render(help)
}

func (m BrowserModel) renderHelpOverlay(r *lipgloss.Renderer, width, height int) string {
	helpText := strings.Join([]string{
		"Keyboard help",
		"",
		"↑ / down    move selection",
		"tab         switch active pane",
		"enter       load selected resource or reset record selection",
		"r           refresh selected resource",
		"?           toggle this help",
		"q / esc     quit",
		"ctrl+c      quit",
		"",
		"Press any key to close.",
	}, "\n")

	panel := r.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(browserBorderColor(m.style, true)).
		Padding(1, 2).
		Render(helpText)

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, panel)
}

func flattenBrowserData(browserData data.BrowserData) []browserItem {
	var items []browserItem
	for _, p := range browserData.Products {
		items = append(items, browserItem{name: p.Name, kind: "product", depth: 0})
		for _, r := range p.Resources {
			items = append(items, browserItem{
				name:    r.Name,
				kind:    "resource",
				depth:   1,
				state:   r.EffectiveState(),
				records: r.Records,
				empty:   r.Empty,
				err:     r.Error,
				Product: r.Product,
			})
		}
	}
	return items
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

func browserLoadingStyle(style output.Style) lipgloss.Style {
	s := lipgloss.NewStyle()
	if style.Color {
		s = s.Foreground(browserAccent(style))
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
