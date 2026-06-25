// Package tea holds the Bubble Tea runtime model for the isolated TUI demo.
package tea

import (
	"context"
	"encoding/json"
	"fmt"
	"image/color"
	"io"
	"math"
	"regexp"
	"sort"
	"strings"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/table"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/dvmrry/zscalerctl/internal/output"
	"github.com/dvmrry/zscalerctl/internal/tui/data"
)

const (
	defaultResourceLoadTimeout = 30 * time.Second

	paneLeft   = "left"
	paneRight  = "right"
	paneDetail = "detail"

	recordStatusPlaceholder = "-"
)

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

// BrowserModel renders a neutral BrowserData view model. Live loading is
// injected through ResourceLoader so config, credential, and reader ownership
// stay outside the Bubble Tea package.
type BrowserModel struct {
	style  output.Style
	width  int
	height int
	data   data.BrowserData
	items  []browserItem

	active   string
	showHelp bool
	exitKey  string

	catalog    table.Model
	records    table.Model
	detailView viewport.Model
	help       help.Model
	keys       browserKeyMap

	// These mirrors preserve the existing tests' selection/offset readback
	// while Bubbles owns the actual table/viewport behavior.
	left   viewportState
	right  viewportState
	detail viewportState

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
	keys := newBrowserKeyMap()
	m := BrowserModel{
		style:      style,
		data:       browserData,
		items:      flattenBrowserData(browserData),
		active:     paneLeft,
		catalog:    newBrowserTable(style),
		records:    newBrowserTable(style),
		detailView: newBrowserViewport(),
		help:       newBrowserHelp(style),
		keys:       keys,
	}
	m.rebuildComponents(false)
	return m
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
		if key, ok := msg.(tea.KeyPressMsg); ok {
			m.showHelp = false
			if isQuitKey(key.String()) {
				m.exitKey = key.String()
				return m, tea.Quit
			}
			m.rebuildComponents(false)
			return m, nil
		}
	}

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		keyString := msg.String()
		switch keyString {
		case "ctrl+c", "esc", "q":
			m.exitKey = keyString
			return m, tea.Quit
		case "?":
			m.showHelp = true
			return m, nil
		case "left":
			m.focusPreviousColumn(false)
			m.rebuildComponents(false)
			return m, nil
		case "right":
			m.focusNextColumn(false)
			m.rebuildComponents(false)
			return m, nil
		case "tab":
			m.focusNextColumn(true)
			m.rebuildComponents(false)
			return m, nil
		case "shift+tab":
			m.focusPreviousColumn(true)
			m.rebuildComponents(false)
			return m, nil
		case "enter":
			return m.handleEnter()
		case "r":
			return m.startResourceLoad(true)
		}

		cmd := m.updateFocusedComponent(msg)
		return m, cmd

	case resourceLoadedMsg:
		m.applyResourceNode(msg.node)
		m.resetRecordSelection()
		if len(m.selectedRecords()) > 0 {
			m.active = paneRight
		}
		m.rebuildComponents(false)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.rebuildComponents(false)
	}
	return m, nil
}

func (m *BrowserModel) handleEnter() (BrowserModel, tea.Cmd) {
	switch m.active {
	case paneLeft:
		item := m.selectedItem()
		if item.kind == "resource" {
			if item.effectiveState() == data.ResourceStateLoaded {
				m.active = paneRight
				m.rebuildComponents(false)
				return *m, nil
			}
			return m.startResourceLoad(false)
		}
		m.focusNextColumn(false)
		m.rebuildComponents(false)
	case paneRight:
		if len(m.selectedRecords()) > 0 && m.geometry().splitDetails {
			m.active = paneDetail
			m.rebuildComponents(false)
		}
	case paneDetail:
		m.detailView.GotoTop()
		m.syncLegacyViewports()
	}
	return *m, nil
}

func (m *BrowserModel) updateFocusedComponent(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	switch m.active {
	case paneLeft:
		before := m.catalog.Cursor()
		m.catalog, cmd = m.catalog.Update(msg)
		if m.catalog.Cursor() != before {
			m.left.Selected = m.catalog.Cursor()
			m.resetRecordSelection()
		}
	case paneRight:
		before := m.records.Cursor()
		m.records, cmd = m.records.Update(msg)
		if m.records.Cursor() != before {
			m.right.Selected = m.records.Cursor()
			m.resetDetailScroll()
		}
	case paneDetail:
		if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
			switch keyMsg.String() {
			case "home":
				m.detailView.GotoTop()
			case "end":
				m.detailView.GotoBottom()
			default:
				m.detailView, cmd = m.detailView.Update(msg)
			}
		} else {
			m.detailView, cmd = m.detailView.Update(msg)
		}
		m.detail.Offset = m.detailView.YOffset()
	}
	m.rebuildComponents(false)
	return cmd
}

func (m BrowserModel) selectedItem() browserItem {
	if len(m.items) == 0 || m.left.Selected < 0 || m.left.Selected >= len(m.items) {
		return browserItem{}
	}
	return m.items[m.left.Selected]
}

func (m BrowserModel) selectedRecords() []data.RecordSummary {
	item := m.selectedItem()
	if item.kind != "resource" {
		return nil
	}
	return item.records
}

func (m BrowserModel) selectedRecord() (data.RecordSummary, bool) {
	records := m.selectedRecords()
	if len(records) == 0 || m.right.Selected < 0 || m.right.Selected >= len(records) {
		return data.RecordSummary{}, false
	}
	return records[m.right.Selected], true
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
	m.rebuildComponents(false)
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
				return
			}
		}
	}
}

func (m *BrowserModel) resetRecordSelection() {
	m.right = viewportState{}
	m.records.SetCursor(0)
	m.resetDetailScroll()
}

func (m *BrowserModel) resetDetailScroll() {
	m.detail = viewportState{}
	m.detailView.GotoTop()
}

func (m *BrowserModel) focusNextColumn(wrap bool) {
	panes := m.availablePanes()
	idx := paneIndex(panes, m.active)
	if idx < len(panes)-1 {
		m.active = panes[idx+1]
		return
	}
	if wrap && len(panes) > 0 {
		m.active = panes[0]
	}
}

func (m *BrowserModel) focusPreviousColumn(wrap bool) {
	panes := m.availablePanes()
	idx := paneIndex(panes, m.active)
	if idx > 0 {
		m.active = panes[idx-1]
		return
	}
	if wrap && len(panes) > 0 {
		m.active = panes[len(panes)-1]
	}
}

func (m BrowserModel) availablePanes() []string {
	if m.geometry().splitDetails {
		return []string{paneLeft, paneRight, paneDetail}
	}
	return []string{paneLeft, paneRight}
}

func paneIndex(panes []string, active string) int {
	for i, pane := range panes {
		if pane == active {
			return i
		}
	}
	return 0
}

func (m *BrowserModel) rebuildComponents(resetRecords bool) {
	m.items = flattenBrowserData(m.data)
	g := m.geometry()
	layout := m.componentLayout(g)
	if m.active == paneDetail && !g.splitDetails {
		m.active = paneRight
	}
	if len(m.items) == 0 {
		m.left = viewportState{}
	} else {
		m.left.Clamp(len(m.items), m.leftViewportHeight())
	}
	if resetRecords {
		m.right = viewportState{}
		m.detail = viewportState{}
	}

	m.catalog.SetColumns([]table.Column{{Title: "Products / Resources", Width: tableColumnWidth(paneContentWidth(g.leftWidth))}})
	m.catalog.SetRows(m.catalogRows())
	m.catalog.SetWidth(paneContentWidth(g.leftWidth))
	m.catalog.SetHeight(paneContentHeight(g.leftHeight))
	m.catalog.SetCursor(m.left.Selected)

	recordCols := recordColumns(tableColumnWidth(layout.recordsWidth))
	m.records.SetColumns(recordCols)
	m.records.SetRows(m.recordRows(recordCols))
	m.records.SetWidth(layout.recordsWidth)
	m.records.SetHeight(layout.recordsHeight)
	m.records.SetCursor(m.right.Selected)

	m.detailView.SetWidth(layout.detailWidth)
	m.detailView.SetHeight(layout.detailHeight)
	m.detailView.SetContent(m.detailContent(layout.detailWidth))
	m.detailView.SetYOffset(m.detail.Offset)

	m.help.SetWidth(lineWidth(g.width))
	m.setFocus()
	m.syncLegacyViewports()
}

func (m *BrowserModel) setFocus() {
	m.catalog.Blur()
	m.records.Blur()
	switch m.active {
	case paneLeft:
		m.catalog.Focus()
	case paneRight:
		m.records.Focus()
	}
}

func (m *BrowserModel) syncLegacyViewports() {
	m.left.Selected = m.catalog.Cursor()
	m.left.Offset = visibleOffset(m.left.Selected, len(m.items), m.leftViewportHeight())
	m.right.Selected = m.records.Cursor()
	m.right.Offset = visibleOffset(m.right.Selected, len(m.selectedRecords()), m.rightViewportHeight())
	m.detail.Offset = m.detailView.YOffset()
}

func visibleOffset(selected, total, height int) int {
	var v viewportState
	v.Selected = selected
	v.Clamp(total, height)
	return v.Offset
}

func (m BrowserModel) catalogRows() []table.Row {
	rows := make([]table.Row, 0, len(m.items))
	for _, item := range m.items {
		rows = append(rows, table.Row{catalogLabel(item)})
	}
	if len(rows) == 0 {
		return []table.Row{{"No resources"}}
	}
	return rows
}

func catalogLabel(item browserItem) string {
	if item.kind == "product" {
		return item.name
	}
	return strings.Repeat("  ", item.depth) + item.name
}

func (m BrowserModel) recordRows(columns []table.Column) []table.Row {
	item := m.selectedItem()
	switch {
	case len(m.items) == 0:
		return noticeRecordRows(columns, "No resources")
	case item.kind == "product":
		return noticeRecordRows(columns, "Select a resource")
	case item.effectiveState() == data.ResourceStateUnloaded:
		return noticeRecordRows(columns, "Resource not loaded", "Press enter to load")
	case item.effectiveState() == data.ResourceStateLoading:
		return noticeRecordRows(columns, "Loading resource...")
	case item.effectiveState() == data.ResourceStateError:
		return noticeRecordRows(columns, "Error loading resource", item.err)
	case item.empty || len(item.records) == 0:
		return noticeRecordRows(columns, "No records")
	}

	rows := make([]table.Row, 0, len(item.records))
	for _, rec := range item.records {
		rows = append(rows, table.Row{
			recordTitle(rec),
			rec.ID,
			recordStatus(rec.Status),
		})
	}
	return rows
}

func noticeRecordRows(columns []table.Column, messages ...string) []table.Row {
	rows := make([]table.Row, 0, len(messages))
	for _, message := range messages {
		row := make(table.Row, len(columns))
		if len(row) == 0 {
			row = table.Row{message}
		} else {
			row[0] = message
		}
		rows = append(rows, row)
	}
	if len(rows) == 0 {
		return []table.Row{{""}}
	}
	return rows
}

func recordColumns(width int) []table.Column {
	width = maxInt(0, width)
	if width < 22 {
		return []table.Column{
			{Title: "Records", Width: maxInt(0, width)},
			{Title: "ID", Width: 0},
			{Title: "Status", Width: 0},
		}
	}
	idWidth := 12
	if width < 42 {
		idWidth = 10
	}
	statusWidth := 8
	if width < 34 {
		statusWidth = 0
	}
	const minRecordNameWidth = 12
	if width < minRecordNameWidth+idWidth+statusWidth {
		statusWidth = 0
	}
	if width < minRecordNameWidth+idWidth {
		idWidth = 0
	}
	nameWidth := width - idWidth - statusWidth
	if nameWidth < 1 {
		nameWidth = width
		idWidth = 0
		statusWidth = 0
	}
	return []table.Column{
		{Title: "Records", Width: nameWidth},
		{Title: "ID", Width: idWidth},
		{Title: "Status", Width: statusWidth},
	}
}

func recordStatus(status string) string {
	status = strings.TrimSpace(status)
	if status == "" {
		return recordStatusPlaceholder
	}
	return status
}

func (m BrowserModel) detailContent(width int) string {
	if len(m.items) == 0 {
		return strings.Join([]string{
			"No resource",
			"",
			"No resources match the current filters.",
		}, "\n")
	}

	item := m.selectedItem()
	switch {
	case item.kind == "product":
		resourceCount := 0
		for i := m.left.Selected + 1; i < len(m.items) && m.items[i].depth > 0; i++ {
			resourceCount++
		}
		return strings.Join([]string{
			item.name,
			"",
			fmt.Sprintf("Product: %s", item.name),
			fmt.Sprintf("Resources: %d", resourceCount),
		}, "\n")
	case item.effectiveState() == data.ResourceStateUnloaded:
		return strings.Join([]string{
			item.name,
			"",
			"Resource not loaded",
			"",
			"Press enter to load this resource.",
		}, "\n")
	case item.effectiveState() == data.ResourceStateLoading:
		return strings.Join([]string{
			item.name,
			"",
			"Loading resource...",
			"",
			"The API call is running for this resource only.",
		}, "\n")
	case item.effectiveState() == data.ResourceStateError:
		return strings.Join([]string{
			item.name,
			"",
			"Error loading resource",
			item.err,
			"",
			"Press enter to retry or r to refresh.",
		}, "\n")
	case item.empty || len(item.records) == 0:
		return strings.Join([]string{
			item.name,
			"",
			"No records for this resource",
			"",
			"Press r to refresh or select a different resource.",
		}, "\n")
	}

	rec, ok := m.selectedRecord()
	if !ok {
		return item.name + "\n\nNo record selected."
	}

	var lines []string
	lines = append(lines, recordTitle(rec), "")
	appendRecordField(&lines, "id", rec.ID, width)
	appendRecordField(&lines, "status", rec.Status, width)
	appendRecordField(&lines, "description", rec.Detail, width)
	for _, f := range rec.Fields {
		appendRecordField(&lines, f.Key, f.Value, width)
	}
	if len(lines) == 2 {
		lines = append(lines, "No additional fields.")
	}
	return strings.Join(wrapDetailOutput(lines, width), "\n")
}

func appendRecordField(lines *[]string, keyName, value string, width int) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	valueLines := formatDetailValue(value, detailValueWidth(width))
	if len(valueLines) == 1 && !isStructuredDetailValue(value) && ansi.StringWidth(keyName+": "+valueLines[0]) <= width {
		*lines = append(*lines, keyName+": "+valueLines[0])
		return
	}
	*lines = append(*lines, keyName+":")
	for _, line := range valueLines {
		if line == "" {
			*lines = append(*lines, "")
			continue
		}
		*lines = append(*lines, "  "+line)
	}
}

func recordTitle(rec data.RecordSummary) string {
	if rec.Name != "" {
		return rec.Name
	}
	if rec.ID != "" {
		return rec.ID
	}
	return "(unnamed record)"
}

func (m BrowserModel) View() tea.View {
	g := m.geometry()
	layout := m.componentLayout(g)

	leftPane := m.renderPane(m.catalog.View(), paneLeft, g.leftWidth, g.leftHeight)
	rightContent := m.records.View()
	if !g.splitDetails && layout.detailHeight > 0 {
		rightContent += "\n" + m.detailView.View()
	}
	rightPane := m.renderPane(rightContent, paneRight, g.rightWidth, g.rightHeight)

	var body string
	if g.stacked {
		body = lipgloss.JoinVertical(lipgloss.Top, leftPane, rightPane)
	} else if g.splitDetails {
		detailPane := m.renderPane(m.detailView.View(), paneDetail, g.detailWidth, g.detailHeight)
		body = lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane, detailPane)
	} else {
		body = lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane)
	}

	if m.showHelp {
		body = m.renderHelpOverlay(g.width, g.bodyHeight)
	}

	status := m.renderStatus(g.width)
	footer := m.renderFooter(g.width)
	return tea.NewView(body + "\n" + status + "\n" + footer + "\n")
}

func (m BrowserModel) renderPane(content, pane string, width, height int) string {
	contentWidth := paneContentWidth(width)
	contentHeight := paneContentHeight(height)
	content = boundBlock(content, contentWidth, contentHeight)

	style := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		Padding(0, 1).
		Width(maxInt(0, width)).
		Height(maxInt(0, height))
	if m.style.Color {
		style = style.BorderForeground(browserBorderColor(m.style, m.active == pane))
	}
	return style.Render(content)
}

func (m BrowserModel) renderStatus(width int) string {
	if len(m.items) == 0 {
		return lipgloss.NewStyle().
			Width(lineWidth(width)).
			Render(fitText("no resources", lineWidth(width)))
	}
	item := m.selectedItem()
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
	return lipgloss.NewStyle().
		Width(statusWidth).
		Render(fitText(status, statusWidth))
}

func (m BrowserModel) renderFooter(width int) string {
	footerWidth := lineWidth(width)
	return lipgloss.NewStyle().
		Width(footerWidth).
		Render(fitText(m.help.View(m.keys), footerWidth))
}

func (m BrowserModel) renderHelpOverlay(width, height int) string {
	helpModel := m.help
	helpModel.ShowAll = true
	helpModel.SetWidth(maxInt(0, width-4))
	helpText := "Keyboard help\n\n" + helpModel.View(m.keys) + "\n\nPress any key to close."

	panelStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		Padding(1, 2).
		MaxWidth(maxInt(0, width-2)).
		MaxHeight(maxInt(0, height))
	if m.style.Color {
		panelStyle = panelStyle.BorderForeground(browserBorderColor(m.style, true))
	}
	panel := panelStyle.Render(helpText)
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, panel)
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
	width        int
	height       int
	bodyHeight   int
	leftWidth    int
	rightWidth   int
	detailWidth  int
	leftHeight   int
	rightHeight  int
	detailHeight int
	stacked      bool
	splitDetails bool
}

func (m BrowserModel) geometry() browserGeometry {
	width, height := m.dimensions()
	bodyHeight := height - 3
	if bodyHeight < 0 {
		bodyHeight = 0
	}
	leftWidth, rightWidth, detailWidth, stacked, splitDetails := browserPaneWidths(width)
	g := browserGeometry{
		width:        width,
		height:       height,
		bodyHeight:   bodyHeight,
		leftWidth:    leftWidth,
		rightWidth:   rightWidth,
		detailWidth:  detailWidth,
		stacked:      stacked,
		splitDetails: splitDetails,
	}
	if stacked {
		g.leftHeight = bodyHeight / 2
		g.rightHeight = bodyHeight - g.leftHeight
		g.detailHeight = 0
	} else {
		g.leftHeight = bodyHeight
		g.rightHeight = bodyHeight
		g.detailHeight = bodyHeight
	}
	return g
}

func (m BrowserModel) leftViewportHeight() int {
	return maxInt(0, m.catalog.Height())
}

func (m BrowserModel) rightViewportHeight() int {
	return maxInt(1, m.records.Height())
}

type componentLayout struct {
	recordsWidth  int
	recordsHeight int
	detailWidth   int
	detailHeight  int
}

func (m BrowserModel) componentLayout(g browserGeometry) componentLayout {
	if g.splitDetails {
		return componentLayout{
			recordsWidth:  paneContentWidth(g.rightWidth),
			recordsHeight: paneContentHeight(g.rightHeight),
			detailWidth:   paneContentWidth(g.detailWidth),
			detailHeight:  paneContentHeight(g.detailHeight),
		}
	}

	contentWidth := paneContentWidth(g.rightWidth)
	contentHeight := paneContentHeight(g.rightHeight)
	if contentHeight <= 6 {
		return componentLayout{
			recordsWidth:  contentWidth,
			recordsHeight: contentHeight,
			detailWidth:   contentWidth,
			detailHeight:  0,
		}
	}

	recordsHeight := contentHeight / 2
	if recordsHeight < 4 {
		recordsHeight = 4
	}
	if recordsHeight > 8 {
		recordsHeight = 8
	}
	detailHeight := contentHeight - recordsHeight - 1
	if detailHeight < 0 {
		detailHeight = 0
	}
	return componentLayout{
		recordsWidth:  contentWidth,
		recordsHeight: recordsHeight,
		detailWidth:   contentWidth,
		detailHeight:  detailHeight,
	}
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

func browserPaneWidths(width int) (left, right, detail int, stacked, splitDetails bool) {
	if width < 70 {
		return width, width, 0, true, false
	}
	if width >= 100 {
		left = width / 4
		if left < 24 {
			left = 24
		}
		right = width / 3
		if right < 30 {
			right = 30
		}
		detail = width - left - right
		if detail >= 30 {
			return left, right, detail, false, true
		}
	}
	left = width / 3
	if left < 24 {
		left = 24
	}
	right = width - left
	return left, right, 0, false, false
}

func paneContentWidth(width int) int {
	return maxInt(0, width-4)
}

func paneContentHeight(height int) int {
	return maxInt(0, height-2)
}

func tableColumnWidth(width int) int {
	if width <= 1 {
		return maxInt(0, width)
	}
	return width - 1
}

func lineWidth(width int) int {
	return maxInt(0, width-2)
}

func fitText(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if ansi.StringWidth(s) <= width {
		return s
	}
	return ansi.Truncate(s, width, "...")
}

func boundBlock(content string, width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	lines := strings.Split(strings.TrimSuffix(content, "\n"), "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	for i, line := range lines {
		lines[i] = fitText(line, width)
	}
	return strings.Join(lines, "\n")
}

func detailValueWidth(width int) int {
	return maxInt(1, width-2)
}

func formatDetailValue(value string, width int) []string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	if lines, ok := formatJSONDetailValue(trimmed, width); ok {
		return lines
	}
	if lines, ok := formatGoMapDetailValue(trimmed, width); ok {
		return lines
	}
	return wrapDetailText(trimmed, width)
}

func isStructuredDetailValue(value string) bool {
	trimmed := strings.TrimSpace(value)
	return strings.HasPrefix(trimmed, "{") ||
		strings.HasPrefix(trimmed, "[") ||
		strings.HasPrefix(trimmed, "map[")
}

func formatJSONDetailValue(value string, width int) ([]string, bool) {
	if !strings.HasPrefix(value, "{") && !strings.HasPrefix(value, "[") {
		return nil, false
	}
	decoder := json.NewDecoder(strings.NewReader(value))
	decoder.UseNumber()
	var decoded any
	if err := decoder.Decode(&decoded); err != nil {
		return nil, false
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, false
	}
	return formatStructuredValue(decoded, width), true
}

func formatStructuredValue(value any, width int) []string {
	switch v := value.(type) {
	case []any:
		if len(v) == 0 {
			return []string{"[]"}
		}
		var lines []string
		for _, item := range v {
			lines = append(lines, formatListItem(item, width)...)
		}
		return lines
	case map[string]any:
		if len(v) == 0 {
			return []string{"{}"}
		}
		return formatMapFields(v, width, nil)
	default:
		return wrapDetailText(formatScalar(v), width)
	}
}

func formatListItem(value any, width int) []string {
	switch v := value.(type) {
	case map[string]any:
		if summary := mapSummary(v); summary != "" {
			lines := wrapDetailText("- "+summary, width)
			extra := formatMapFields(v, detailValueWidth(width), summaryKeys())
			if len(extra) > 0 {
				lines = append(lines, indentDetailLines(extra)...)
			}
			return lines
		}
	case []any:
		lines := formatStructuredValue(v, detailValueWidth(width))
		if len(lines) == 0 {
			return []string{"- []"}
		}
		return prefixFirstAndIndent(lines, "- ", "  ", width)
	default:
		if isScalar(v) {
			return wrapDetailText("- "+formatScalar(v), width)
		}
	}
	lines := formatStructuredValue(value, detailValueWidth(width))
	if len(lines) == 0 {
		return []string{"-"}
	}
	return prefixFirstAndIndent(lines, "- ", "  ", width)
}

func formatMapFields(m map[string]any, width int, skip map[string]bool) []string {
	keys := sortedMapKeys(m)
	var lines []string
	for _, keyName := range keys {
		if skipSummaryField(skip, keyName) {
			continue
		}
		fieldValue := m[keyName]
		if isScalar(fieldValue) {
			lines = append(lines, wrapDetailText(keyName+": "+formatScalar(fieldValue), width)...)
			continue
		}
		lines = append(lines, keyName+":")
		lines = append(lines, indentDetailLines(formatStructuredValue(fieldValue, detailValueWidth(width)))...)
	}
	return lines
}

func mapSummary(m map[string]any) string {
	return summaryFromLookup(func(keyName string) (string, bool) {
		v, ok := m[keyName]
		if !ok || !isScalar(v) {
			return "", false
		}
		return formatScalar(v), true
	}, sortedMapKeys(m))
}

func sortedMapKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func formatGoMapDetailValue(value string, width int) ([]string, bool) {
	if !strings.HasPrefix(value, "map[") && !strings.HasPrefix(value, "[map[") {
		return nil, false
	}
	chunks := goMapChunks(value)
	if len(chunks) == 0 {
		return nil, false
	}
	lines := make([]string, 0, len(chunks))
	for _, chunk := range chunks {
		fields := parseGoMapFields(chunk)
		if len(fields) == 0 {
			return nil, false
		}
		summary := summaryFromLookup(func(keyName string) (string, bool) {
			v, ok := fields[keyName]
			return v, ok
		}, sortedStringMapKeys(fields))
		if summary == "" {
			var parts []string
			for _, keyName := range sortedStringMapKeys(fields) {
				parts = append(parts, keyName+"="+fields[keyName])
			}
			summary = strings.Join(parts, ", ")
		}
		lines = append(lines, wrapDetailText("- "+summary, width)...)
		for _, line := range stringMapExtraLines(fields, detailValueWidth(width), summaryKeys()) {
			lines = append(lines, "  "+line)
		}
	}
	return lines, true
}

func goMapChunks(value string) []string {
	var chunks []string
	for start := strings.Index(value, "map["); start >= 0; {
		contentStart := start + len("map[")
		depth := 1
		end := contentStart
		for end < len(value) && depth > 0 {
			switch value[end] {
			case '[':
				depth++
			case ']':
				depth--
			}
			end++
		}
		if depth != 0 {
			return nil
		}
		chunks = append(chunks, value[contentStart:end-1])
		next := strings.Index(value[end:], "map[")
		if next < 0 {
			break
		}
		start = end + next
	}
	return chunks
}

var goMapKeyPattern = regexp.MustCompile(`(?:^| )([A-Za-z_][A-Za-z0-9_]*):`)

func parseGoMapFields(chunk string) map[string]string {
	matches := goMapKeyPattern.FindAllStringSubmatchIndex(chunk, -1)
	if len(matches) == 0 {
		return nil
	}
	fields := make(map[string]string, len(matches))
	for i, match := range matches {
		keyName := chunk[match[2]:match[3]]
		valueStart := match[1]
		valueEnd := len(chunk)
		if i+1 < len(matches) {
			valueEnd = matches[i+1][0]
		}
		fields[keyName] = strings.TrimSpace(chunk[valueStart:valueEnd])
	}
	return fields
}

func sortedStringMapKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func stringMapExtraLines(m map[string]string, width int, skip map[string]bool) []string {
	var lines []string
	for _, keyName := range sortedStringMapKeys(m) {
		if skipSummaryField(skip, keyName) {
			continue
		}
		lines = append(lines, wrapDetailText(keyName+": "+m[keyName], width)...)
	}
	return lines
}

func summaryKeys() map[string]bool {
	return map[string]bool{
		"id":     true,
		"name":   true,
		"status": true,
	}
}

func skipSummaryField(skip map[string]bool, keyName string) bool {
	if len(skip) == 0 {
		return false
	}
	return skip[strings.ToLower(keyName)]
}

func summaryFromLookup(lookup func(string) (string, bool), fallbackKeys []string) string {
	name, hasName := firstLookupValue(lookup, "name", "Name")
	id, hasID := firstLookupValue(lookup, "id", "ID")
	status, hasStatus := firstLookupValue(lookup, "status", "Status")

	label := name
	if !hasName && hasID {
		label = id
	}
	var attrs []string
	if hasName && hasID {
		attrs = append(attrs, "id="+id)
	}
	if hasStatus {
		attrs = append(attrs, "status="+status)
	}
	if label != "" {
		if len(attrs) > 0 {
			return label + " (" + strings.Join(attrs, ", ") + ")"
		}
		return label
	}

	var parts []string
	for _, keyName := range fallbackKeys {
		if value, ok := lookup(keyName); ok {
			parts = append(parts, keyName+"="+value)
		}
		if len(parts) == 3 {
			break
		}
	}
	return strings.Join(parts, ", ")
}

func firstLookupValue(lookup func(string) (string, bool), keys ...string) (string, bool) {
	for _, keyName := range keys {
		value, ok := lookup(keyName)
		if ok {
			value = strings.TrimSpace(value)
			if value != "" {
				return value, true
			}
		}
	}
	return "", false
}

func isScalar(value any) bool {
	switch value.(type) {
	case nil, string, bool, json.Number, float64, float32, int, int64, int32, uint, uint64, uint32:
		return true
	default:
		return false
	}
}

func formatScalar(value any) string {
	switch v := value.(type) {
	case nil:
		return "null"
	case string:
		return v
	case json.Number:
		return v.String()
	case float64:
		if math.Trunc(v) == v {
			return fmt.Sprintf("%.0f", v)
		}
		return fmt.Sprintf("%g", v)
	case float32:
		f := float64(v)
		if math.Trunc(f) == f {
			return fmt.Sprintf("%.0f", f)
		}
		return fmt.Sprintf("%g", f)
	default:
		return fmt.Sprintf("%v", v)
	}
}

func indentDetailLines(lines []string) []string {
	return prefixFirstAndIndent(lines, "  ", "  ", 0)
}

func prefixFirstAndIndent(lines []string, firstPrefix, restPrefix string, width int) []string {
	if len(lines) == 0 {
		return nil
	}
	out := make([]string, 0, len(lines))
	for i, line := range lines {
		prefix := restPrefix
		if i == 0 {
			prefix = firstPrefix
		}
		if width > 0 {
			out = append(out, wrapDetailText(prefix+line, width)...)
			continue
		}
		out = append(out, prefix+line)
	}
	return out
}

func wrapDetailOutput(lines []string, width int) []string {
	if width <= 0 {
		return nil
	}
	var out []string
	for _, line := range lines {
		if ansi.StringWidth(line) <= width {
			out = append(out, line)
			continue
		}
		indent := leadingWhitespace(line)
		wrapped := wrapDetailText(strings.TrimSpace(line), maxInt(1, width-ansi.StringWidth(indent)))
		for _, wrappedLine := range wrapped {
			out = append(out, indent+wrappedLine)
		}
	}
	return out
}

func leadingWhitespace(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r != ' ' && r != '\t' {
			break
		}
		b.WriteRune(r)
	}
	return b.String()
}

func wrapDetailText(value string, width int) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return []string{""}
	}
	width = maxInt(1, width)
	var out []string
	for _, part := range strings.Split(value, "\n") {
		words := strings.Fields(part)
		if len(words) == 0 {
			out = append(out, "")
			continue
		}
		line := ""
		for _, word := range words {
			if ansi.StringWidth(word) > width {
				if line != "" {
					out = append(out, line)
					line = ""
				}
				chunks := splitWideWord(word, width)
				if len(chunks) == 0 {
					continue
				}
				out = append(out, chunks[:len(chunks)-1]...)
				line = chunks[len(chunks)-1]
				continue
			}
			if line == "" {
				line = word
				continue
			}
			if ansi.StringWidth(line)+1+ansi.StringWidth(word) <= width {
				line += " " + word
				continue
			}
			out = append(out, line)
			line = word
		}
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

func splitWideWord(word string, width int) []string {
	width = maxInt(1, width)
	var chunks []string
	var b strings.Builder
	used := 0
	for _, r := range word {
		cellWidth := ansi.StringWidth(string(r))
		if used > 0 && used+cellWidth > width {
			chunks = append(chunks, b.String())
			b.Reset()
			used = 0
		}
		b.WriteRune(r)
		used += cellWidth
	}
	if b.Len() > 0 {
		chunks = append(chunks, b.String())
	}
	return chunks
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func isQuitKey(s string) bool {
	return s == "ctrl+c" || s == "esc" || s == "q"
}

func newBrowserTable(style output.Style) table.Model {
	return table.New(
		table.WithFocused(false),
		table.WithKeyMap(browserTableKeyMap()),
		table.WithStyles(browserTableStyles(style)),
	)
}

func newBrowserViewport() viewport.Model {
	v := viewport.New()
	v.SoftWrap = true
	v.FillHeight = true
	return v
}

func newBrowserHelp(style output.Style) help.Model {
	h := help.New()
	if !style.Color {
		h.Styles = help.Styles{
			Ellipsis:       lipgloss.NewStyle(),
			ShortKey:       lipgloss.NewStyle(),
			ShortDesc:      lipgloss.NewStyle(),
			ShortSeparator: lipgloss.NewStyle(),
			FullKey:        lipgloss.NewStyle(),
			FullDesc:       lipgloss.NewStyle(),
			FullSeparator:  lipgloss.NewStyle(),
		}
	}
	return h
}

func browserTableKeyMap() table.KeyMap {
	return table.KeyMap{
		LineUp: key.NewBinding(
			key.WithKeys("up"),
			key.WithHelp("↑", "up"),
		),
		LineDown: key.NewBinding(
			key.WithKeys("down"),
			key.WithHelp("↓", "down"),
		),
		PageUp: key.NewBinding(
			key.WithKeys("pgup"),
			key.WithHelp("pgup", "page up"),
		),
		PageDown: key.NewBinding(
			key.WithKeys("pgdown"),
			key.WithHelp("pgdn", "page down"),
		),
		HalfPageUp:   key.NewBinding(key.WithDisabled()),
		HalfPageDown: key.NewBinding(key.WithDisabled()),
		GotoTop: key.NewBinding(
			key.WithKeys("home"),
			key.WithHelp("home", "top"),
		),
		GotoBottom: key.NewBinding(
			key.WithKeys("end"),
			key.WithHelp("end", "bottom"),
		),
	}
}

func browserTableStyles(style output.Style) table.Styles {
	styles := table.DefaultStyles()
	styles.Header = lipgloss.NewStyle()
	styles.Cell = lipgloss.NewStyle()
	styles.Selected = lipgloss.NewStyle()
	if style.Color {
		styles.Header = styles.Header.Bold(true).Foreground(browserAccent(style))
		styles.Selected = styles.Selected.Background(browserAccent(style)).Foreground(lipgloss.Color("0"))
	}
	return styles
}

func browserAccent(style output.Style) color.Color {
	if style.Color256 {
		return lipgloss.Color("45")
	}
	return lipgloss.Color("6")
}

func browserBorderColor(style output.Style, active bool) color.Color {
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

type browserKeyMap struct {
	Up      key.Binding
	Down    key.Binding
	PageUp  key.Binding
	PageDn  key.Binding
	Home    key.Binding
	End     key.Binding
	Left    key.Binding
	Right   key.Binding
	Enter   key.Binding
	Refresh key.Binding
	Help    key.Binding
	Quit    key.Binding
}

func newBrowserKeyMap() browserKeyMap {
	return browserKeyMap{
		Up:      key.NewBinding(key.WithKeys("up"), key.WithHelp("↑", "move")),
		Down:    key.NewBinding(key.WithKeys("down"), key.WithHelp("↓", "move")),
		PageUp:  key.NewBinding(key.WithKeys("pgup"), key.WithHelp("pgup", "page")),
		PageDn:  key.NewBinding(key.WithKeys("pgdown"), key.WithHelp("pgdn", "page")),
		Home:    key.NewBinding(key.WithKeys("home"), key.WithHelp("home", "top")),
		End:     key.NewBinding(key.WithKeys("end"), key.WithHelp("end", "bottom")),
		Left:    key.NewBinding(key.WithKeys("left", "shift+tab"), key.WithHelp("←", "pane")),
		Right:   key.NewBinding(key.WithKeys("right", "tab"), key.WithHelp("→", "pane")),
		Enter:   key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open")),
		Refresh: key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
		Help:    key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Quit:    key.NewBinding(key.WithKeys("q", "esc", "ctrl+c"), key.WithHelp("q/esc", "quit")),
	}
}

func (k browserKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Left, k.Right, k.Quit, k.Enter, k.Help}
}

func (k browserKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.PageUp, k.PageDn, k.Home, k.End},
		{k.Left, k.Right, k.Enter, k.Refresh, k.Help, k.Quit},
	}
}
