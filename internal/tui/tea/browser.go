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
	left        viewportState
	active      string // "left" or "right"
	right       viewportState
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
				m.moveLeft(-1)
			} else {
				m.moveRight(-1)
			}
		case "down":
			if m.active == "left" {
				m.moveLeft(1)
			} else {
				m.moveRight(1)
			}
		case "pgup", "pageup":
			if m.active == "left" {
				m.pageLeft(-1)
			} else {
				m.pageRight(-1)
			}
		case "pgdown", "pagedown":
			if m.active == "left" {
				m.pageLeft(1)
			} else {
				m.pageRight(1)
			}
		case "home":
			if m.active == "left" {
				m.homeLeft()
			} else {
				m.homeRight()
			}
		case "end":
			if m.active == "left" {
				m.endLeft()
			} else {
				m.endRight()
			}
		case "tab":
			if m.active == "left" {
				m.active = "right"
				m.clampViewports()
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
		m.clampViewports()
	}
	return m, nil
}

func (m BrowserModel) selectedItem() browserItem {
	if len(m.items) == 0 || m.left.Selected < 0 || m.left.Selected >= len(m.items) {
		return browserItem{}
	}
	return m.items[m.left.Selected]
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
				m.clampViewports()
				return
			}
		}
	}
}

func (m *BrowserModel) resetRecordSelection() {
	m.right = viewportState{}
	m.clampViewports()
}

func (m *BrowserModel) moveLeft(delta int) {
	before := m.left.Selected
	m.left.Move(delta, len(m.items), m.leftViewportHeight())
	if m.left.Selected != before {
		m.resetRecordSelection()
	}
}

func (m *BrowserModel) pageLeft(delta int) {
	before := m.left.Selected
	m.left.Page(delta, len(m.items), m.leftViewportHeight())
	if m.left.Selected != before {
		m.resetRecordSelection()
	}
}

func (m *BrowserModel) homeLeft() {
	before := m.left.Selected
	m.left.Home(len(m.items), m.leftViewportHeight())
	if m.left.Selected != before {
		m.resetRecordSelection()
	}
}

func (m *BrowserModel) endLeft() {
	before := m.left.Selected
	m.left.End(len(m.items), m.leftViewportHeight())
	if m.left.Selected != before {
		m.resetRecordSelection()
	}
}

func (m *BrowserModel) moveRight(delta int) {
	m.right.Move(delta, len(m.selectedRecords()), m.rightViewportHeight())
	m.ensureRightSelectionVisible()
}

func (m *BrowserModel) pageRight(delta int) {
	m.right.Page(delta, len(m.selectedRecords()), m.rightViewportHeight())
	m.ensureRightSelectionVisible()
}

func (m *BrowserModel) homeRight() {
	m.right.Home(len(m.selectedRecords()), m.rightViewportHeight())
	m.ensureRightSelectionVisible()
}

func (m *BrowserModel) endRight() {
	m.right.End(len(m.selectedRecords()), m.rightViewportHeight())
	m.ensureRightSelectionVisible()
}

func (m *BrowserModel) clampViewports() {
	m.left.Clamp(len(m.items), m.leftViewportHeight())
	m.right.Clamp(len(m.selectedRecords()), m.rightViewportHeight())
	m.ensureRightSelectionVisible()
}

func (m BrowserModel) selectedRecords() []data.RecordSummary {
	item := m.selectedItem()
	if item.kind != "resource" {
		return nil
	}
	return item.records
}

func (m *BrowserModel) ensureRightSelectionVisible() {
	item := m.selectedItem()
	if item.kind != "resource" || len(item.records) == 0 {
		m.right = viewportState{}
		return
	}
	m.right.Clamp(len(item.records), m.rightViewportHeight())
	budget := m.rightRecordLineBudget()
	if budget <= 0 || m.right.Offset >= m.right.Selected {
		return
	}
	for m.right.Offset < m.right.Selected &&
		recordBlockLineCount(item.records, m.right.Offset, m.right.Selected) > budget {
		m.right.Offset++
	}
}

func (m BrowserModel) leftViewportHeight() int {
	return leftViewportHeight(m.geometry().leftHeight)
}

func (m BrowserModel) rightViewportHeight() int {
	return rightViewportHeight(m.geometry().rightHeight)
}

func (m BrowserModel) rightRecordLineBudget() int {
	return rightRecordLineBudget(m.geometry().rightHeight)
}

func recordBlockLineCount(records []data.RecordSummary, start, end int) int {
	if start < 0 {
		start = 0
	}
	if end >= len(records) {
		end = len(records) - 1
	}
	if start > end || len(records) == 0 {
		return 0
	}
	count := 0
	for i := start; i <= end; i++ {
		count += recordLineCount(records[i])
	}
	return count
}

func recordLineCount(rec data.RecordSummary) int {
	count := 1
	if rec.Detail != "" {
		count++
	}
	count += len(rec.Fields)
	return count
}

func (m BrowserModel) View() string {
	g := m.geometry()
	r := browserRenderer(m.style)

	var leftPane, rightPane string
	if g.stacked {
		leftPane = m.renderLeftPane(r, g.leftWidth, g.leftHeight)
		rightPane = m.renderRightPane(r, g.rightWidth, g.rightHeight)
	} else {
		leftPane = m.renderLeftPane(r, g.leftWidth, g.leftHeight)
		rightPane = m.renderRightPane(r, g.rightWidth, g.rightHeight)
	}

	var body string
	if g.stacked {
		body = lipgloss.JoinVertical(lipgloss.Top, leftPane, rightPane)
	} else {
		body = lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane)
	}

	if m.showHelp {
		body = m.renderHelpOverlay(r, g.width, g.bodyHeight)
	}

	status := m.renderStatus(r, g.width)
	footer := m.renderFooter(r, g.width)
	return body + "\n" + status + "\n" + footer + "\n"
}

// ActivePane reports which pane currently has focus.
func (m BrowserModel) ActivePane() string {
	return m.active
}

// SelectedIndex reports the selected left-pane index.
func (m BrowserModel) SelectedIndex() int {
	return m.left.Selected
}

// RecordIndex reports the selected right-pane record index.
func (m BrowserModel) RecordIndex() int {
	return m.right.Selected
}

// ScrollOffset reports the right-pane record viewport offset.
func (m BrowserModel) ScrollOffset() int {
	return m.right.Offset
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

type browserGeometry struct {
	width       int
	height      int
	bodyHeight  int
	leftWidth   int
	rightWidth  int
	leftHeight  int
	rightHeight int
	stacked     bool
}

func (m BrowserModel) geometry() browserGeometry {
	width, height := m.dimensions()
	bodyHeight := height - 3
	if bodyHeight < 0 {
		bodyHeight = 0
	}
	leftWidth, rightWidth, stacked := browserPaneWidths(width)
	g := browserGeometry{
		width:      width,
		height:     height,
		bodyHeight: bodyHeight,
		leftWidth:  leftWidth,
		rightWidth: rightWidth,
		stacked:    stacked,
	}
	if stacked {
		g.leftHeight = bodyHeight / 2
		g.rightHeight = bodyHeight - g.leftHeight
	} else {
		g.leftHeight = bodyHeight
		g.rightHeight = bodyHeight
	}
	return g
}

func (m BrowserModel) renderLeftPane(r *lipgloss.Renderer, width, height int) string {
	contentWidth := paneContentWidth(width)
	contentHeight := paneContentHeight(height)
	style := r.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(browserBorderColor(m.style, m.active == "left")).
		Padding(0, 1).
		Width(contentWidth).
		Height(contentHeight)

	var lines []string
	appendStyledLine(&lines, contentHeight, contentWidth, r.NewStyle().Bold(true), "Products / Resources")
	appendFittedLine(&lines, contentHeight, contentWidth, "")

	start, end := m.left.VisibleRange(len(m.items), leftViewportHeight(height))
	for i := start; i < end; i++ {
		item := m.items[i]
		prefix := strings.Repeat("  ", item.depth)
		label := prefix + item.name
		if item.kind == "product" {
			label = r.NewStyle().Bold(true).Render(fitText(label, contentWidth))
		} else {
			label = r.NewStyle().Render(fitText(label, contentWidth))
		}
		if i == m.left.Selected {
			label = browserSelectedStyle(m.style).Render(label)
		}
		lines = append(lines, label)
	}

	return style.Render(strings.Join(lines, "\n"))
}

func (m BrowserModel) renderRightPane(r *lipgloss.Renderer, width, height int) string {
	contentWidth := paneContentWidth(width)
	contentHeight := paneContentHeight(height)
	style := r.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(browserBorderColor(m.style, m.active == "right")).
		Padding(0, 1).
		Width(contentWidth).
		Height(contentHeight)

	content := m.rightPaneContent(r, contentWidth, contentHeight)

	return style.Render(strings.Join(content, "\n"))
}

func (m BrowserModel) rightPaneContent(r *lipgloss.Renderer, width, maxLines int) []string {
	if len(m.items) == 0 {
		var lines []string
		appendStyledLine(&lines, maxLines, width, r.NewStyle().Bold(true), "No resources")
		appendFittedLine(&lines, maxLines, width, "")
		appendStyledLine(&lines, maxLines, width, browserEmptyStyle(m.style), "No resources match the current filters.")
		return lines
	}
	item := m.items[m.left.Selected]
	var lines []string

	appendStyledLine(&lines, maxLines, width, r.NewStyle().Bold(true), item.name)
	appendFittedLine(&lines, maxLines, width, "")

	switch {
	case item.kind == "product":
		resourceCount := 0
		for i := m.left.Selected + 1; i < len(m.items) && m.items[i].depth > 0; i++ {
			resourceCount++
		}
		appendFittedLine(&lines, maxLines, width, fmt.Sprintf("Product: %s", item.name))
		appendFittedLine(&lines, maxLines, width, fmt.Sprintf("Resources: %d", resourceCount))

	case item.effectiveState() == data.ResourceStateUnloaded:
		appendFittedLine(&lines, maxLines, width, "")
		appendStyledLine(&lines, maxLines, width, browserEmptyStyle(m.style), "Resource not loaded")
		appendFittedLine(&lines, maxLines, width, "")
		appendStyledLine(&lines, maxLines, width, browserEmptyStyle(m.style), "Press enter to load this resource.")

	case item.effectiveState() == data.ResourceStateLoading:
		appendFittedLine(&lines, maxLines, width, "")
		appendStyledLine(&lines, maxLines, width, browserLoadingStyle(m.style), "Loading resource...")
		appendFittedLine(&lines, maxLines, width, "")
		appendStyledLine(&lines, maxLines, width, browserEmptyStyle(m.style), "The API call is running for this resource only.")

	case item.effectiveState() == data.ResourceStateError:
		appendFittedLine(&lines, maxLines, width, "")
		appendStyledLine(&lines, maxLines, width, browserErrorStyle(m.style), "Error loading resource")
		appendStyledLine(&lines, maxLines, width, browserErrorStyle(m.style), item.err)
		appendFittedLine(&lines, maxLines, width, "")
		appendStyledLine(&lines, maxLines, width, browserEmptyStyle(m.style), "Press enter to retry or r to refresh.")

	case item.empty || len(item.records) == 0:
		appendFittedLine(&lines, maxLines, width, "")
		appendStyledLine(&lines, maxLines, width, browserEmptyStyle(m.style), "No records for this resource")
		appendFittedLine(&lines, maxLines, width, "")
		appendStyledLine(&lines, maxLines, width, browserEmptyStyle(m.style), "Press r to refresh or select a different resource.")

	default:
		start := m.rightVisibleRecordStart(item, maxLines)
		for i := start; i < len(item.records) && len(lines) < maxLines; i++ {
			rec := item.records[i]
			recLine := fmt.Sprintf("  %s (id=%s, status=%s)", rec.Name, rec.ID, rec.Status)
			if i == m.right.Selected && m.active == "right" {
				recLine = browserSelectedStyle(m.style).Render(fitText(recLine, width))
			} else {
				recLine = fitText(recLine, width)
			}
			appendLine(&lines, maxLines, recLine)
			if rec.Detail != "" {
				appendFittedLine(&lines, maxLines, width, "    "+rec.Detail)
			}
			for _, f := range rec.Fields {
				appendFittedLine(&lines, maxLines, width, fmt.Sprintf("    %s: %s", f.Key, f.Value))
			}
		}
	}

	return lines
}

func (m BrowserModel) rightVisibleRecordStart(item browserItem, maxLines int) int {
	start, _ := m.right.VisibleRange(len(item.records), rightViewportHeight(maxLines+2))
	if start > m.right.Selected {
		start = m.right.Selected
	}
	budget := maxLines - 2
	if budget < 1 {
		budget = maxLines
	}
	for start < m.right.Selected && recordBlockLineCount(item.records, start, m.right.Selected) > budget {
		start++
	}
	if start < 0 {
		return 0
	}
	return start
}

func (m BrowserModel) renderStatus(r *lipgloss.Renderer, width int) string {
	if len(m.items) == 0 {
		return r.NewStyle().
			Width(lineWidth(width)).
			Render(fitText("no resources", lineWidth(width)))
	}
	item := m.items[m.left.Selected]
	var selected string
	if item.kind == "product" {
		selected = item.name
	} else {
		selected = item.Product + " / " + item.name
	}
	status := fmt.Sprintf("%s · %d/%d", selected, m.left.Selected+1, len(m.items))
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
	statusWidth := lineWidth(width)
	return r.NewStyle().
		Width(statusWidth).
		Render(fitText(status, statusWidth))
}

func (m BrowserModel) renderFooter(r *lipgloss.Renderer, width int) string {
	help := "up/down move · enter load · r refresh · esc/q quit · ? help · pgup/pgdown page · home/end jump · tab switch"
	footerWidth := lineWidth(width)
	return r.NewStyle().
		Width(footerWidth).
		Render(fitText(help, footerWidth))
}

func (m BrowserModel) renderHelpOverlay(r *lipgloss.Renderer, width, height int) string {
	helpText := strings.Join([]string{
		"Keyboard help",
		"",
		"up / down       move selection",
		"pgup / pgdown   page selection",
		"home / end      jump to boundary",
		"tab             switch active pane",
		"enter           load selected resource or reset record selection",
		"r               refresh selected resource",
		"?               toggle this help",
		"q / esc         quit",
		"ctrl+c          quit",
		"",
		"Press any key to close.",
	}, "\n")

	panel := r.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(browserBorderColor(m.style, true)).
		Padding(1, 2).
		MaxWidth(maxInt(0, width-2)).
		MaxHeight(maxInt(0, height)).
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

func paneContentWidth(width int) int {
	return maxInt(0, width-4)
}

func paneContentHeight(height int) int {
	return maxInt(0, height-2)
}

func leftViewportHeight(paneHeight int) int {
	return maxInt(0, paneContentHeight(paneHeight)-2)
}

func rightViewportHeight(paneHeight int) int {
	return maxInt(1, rightRecordLineBudget(paneHeight))
}

func rightRecordLineBudget(paneHeight int) int {
	return maxInt(0, paneContentHeight(paneHeight)-2)
}

func lineWidth(width int) int {
	return maxInt(0, width-2)
}

func appendLine(lines *[]string, maxLines int, line string) bool {
	if maxLines <= 0 || len(*lines) >= maxLines {
		return false
	}
	*lines = append(*lines, line)
	return len(*lines) < maxLines
}

func appendFittedLine(lines *[]string, maxLines, width int, line string) bool {
	return appendLine(lines, maxLines, fitText(line, width))
}

func appendStyledLine(lines *[]string, maxLines, width int, style lipgloss.Style, line string) bool {
	return appendLine(lines, maxLines, style.Render(fitText(line, width)))
}

func fitText(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= width {
		return s
	}
	if width <= 3 {
		return strings.Repeat(".", width)
	}
	target := width - 3
	var b strings.Builder
	used := 0
	for _, r := range s {
		cellWidth := lipgloss.Width(string(r))
		if used+cellWidth > target {
			break
		}
		b.WriteRune(r)
		used += cellWidth
	}
	return b.String() + "..."
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
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
