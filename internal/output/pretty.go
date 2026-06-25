package output

import (
	"image/color"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
)

// The pretty renderer is a presentation overlay over data that has already been
// allow-list projected and redacted upstream. It introduces no new data path:
// it only styles strings the caller has already produced, and the result is
// still passed through Renderer.WriteText (the final redaction byte-scan) before
// it reaches stdout. When color is disabled the output is byte-clean (no ANSI
// escapes), so it stays safe to capture or diff.

// RenderRecordsPretty renders a list of records as a bordered table. headers and
// each row share the same column order; callers pass values already formatted to
// strings.
func RenderRecordsPretty(headers []string, rows [][]string, style Style) SafeText {
	headerStyle := lipgloss.NewStyle().Padding(0, 1)
	cellStyle := lipgloss.NewStyle().Padding(0, 1)
	if style.Color {
		headerStyle = headerStyle.Bold(true).Foreground(prettyAccent(style))
	}

	t := table.New().
		Border(lipgloss.NormalBorder()).
		BorderStyle(prettyBorderStyle(style)).
		Headers(headers...).
		Rows(rows...).
		StyleFunc(func(row, _ int) lipgloss.Style {
			if row == table.HeaderRow {
				return headerStyle
			}
			return cellStyle
		})

	return renderPrettyTable(t, style)
}

// RenderRecordPretty renders a single record as a bordered key/value table. It
// is the show/get counterpart to RenderRecordsPretty.
func RenderRecordPretty(rows []KV, style Style) SafeText {
	return RenderKeyValuesPretty(rows, style)
}

// RenderKeyValuesPretty renders general key/value rows as a bordered table.
func RenderKeyValuesPretty(rows []KV, style Style) SafeText {
	headerStyle := lipgloss.NewStyle().Padding(0, 1)
	keyStyle := lipgloss.NewStyle().Padding(0, 1)
	cellStyle := lipgloss.NewStyle().Padding(0, 1)
	if style.Color {
		headerStyle = headerStyle.Bold(true).Foreground(prettyAccent(style))
		keyStyle = keyStyle.Bold(true).Foreground(prettyAccent(style))
	}

	tableRows := make([][]string, 0, len(rows))
	for _, row := range rows {
		if row.Kind == "" {
			row.Kind = row.Key
		}
		tableRows = append(tableRows, []string{row.Key, style.Value(row.Kind, row.Value)})
	}

	t := table.New().
		Border(lipgloss.NormalBorder()).
		BorderStyle(prettyBorderStyle(style)).
		Headers("field", "value").
		Rows(tableRows...).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				return headerStyle
			}
			if col == 0 {
				return keyStyle
			}
			return cellStyle
		})

	return renderPrettyTable(t, style)
}

func prettyAccent(style Style) color.Color {
	if style.Color256 {
		return lipgloss.Color("45")
	}
	return lipgloss.Color("6")
}

func prettyBorderStyle(style Style) lipgloss.Style {
	s := lipgloss.NewStyle()
	if !style.Color {
		return s
	}
	if style.Color256 {
		return s.Foreground(lipgloss.Color("240"))
	}
	return s.Foreground(lipgloss.Color("8"))
}

func renderPrettyTable(t *table.Table, style Style) SafeText {
	rendered := t.Render()
	// Only constrain the width when the natural table would overflow the
	// terminal; otherwise a narrow table would be stretched to fill the screen.
	// With a width set, lipgloss wraps cell contents (wrap defaults to true)
	// instead of letting a long value run off the line and smear the borders.
	if style.Width > 0 && lipgloss.Width(rendered) > style.Width {
		rendered = t.Width(style.Width).Render()
	}
	return NewSafeText(rendered + "\n")
}
