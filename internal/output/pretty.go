package output

import (
	"io"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/muesli/termenv"
)

// The pretty renderer is a presentation overlay over data that has already been
// allow-list projected and redacted upstream. It introduces no new data path:
// it only styles strings the caller has already produced, and the result is
// still passed through Renderer.WriteText (the final redaction byte-scan) before
// it reaches stdout. When color is disabled the output is byte-clean (no ANSI
// escapes), so it stays safe to capture or diff.

// prettyRenderer builds a lipgloss renderer whose color profile is pinned from
// the resolved Style instead of lipgloss auto-detecting the environment. This
// keeps pretty output deterministic and honors --color / NO_COLOR / TTY exactly
// as the rest of the tool does.
func prettyRenderer(style Style) *lipgloss.Renderer {
	r := lipgloss.NewRenderer(io.Discard)
	r.SetColorProfile(prettyProfile(style))
	return r
}

func prettyProfile(style Style) termenv.Profile {
	switch {
	case !style.Color:
		return termenv.Ascii
	case style.Color256:
		return termenv.ANSI256
	default:
		return termenv.ANSI
	}
}

// RenderRecordsPretty renders a list of records as a bordered table. headers and
// each row share the same column order; callers pass values already formatted to
// strings.
func RenderRecordsPretty(headers []string, rows [][]string, style Style) SafeText {
	r := prettyRenderer(style)
	headerStyle := r.NewStyle().Bold(true).Padding(0, 1)
	cellStyle := r.NewStyle().Padding(0, 1)
	if style.Color {
		headerStyle = headerStyle.Foreground(prettyAccent(style))
	}

	t := table.New().
		Border(lipgloss.NormalBorder()).
		BorderStyle(prettyBorderStyle(r, style)).
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
	r := prettyRenderer(style)
	headerStyle := r.NewStyle().Bold(true).Padding(0, 1)
	keyStyle := r.NewStyle().Bold(true).Padding(0, 1)
	cellStyle := r.NewStyle().Padding(0, 1)
	if style.Color {
		headerStyle = headerStyle.Foreground(prettyAccent(style))
		keyStyle = keyStyle.Foreground(prettyAccent(style))
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
		BorderStyle(prettyBorderStyle(r, style)).
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

func prettyAccent(style Style) lipgloss.Color {
	if style.Color256 {
		return lipgloss.Color("45")
	}
	return lipgloss.Color("6")
}

func prettyBorderStyle(r *lipgloss.Renderer, style Style) lipgloss.Style {
	s := r.NewStyle()
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
