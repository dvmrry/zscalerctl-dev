package output

import (
	"fmt"
	"strings"
)

type Style struct {
	Color    bool
	Color256 bool
	// Width is the terminal column count, or 0 when unknown. Only the pretty
	// renderer uses it, to wrap tables wider than the terminal; other renderers
	// ignore it.
	Width int
}

func NewStyle(color, color256 bool) Style {
	return Style{Color: color, Color256: color256}
}

func (s Style) Key(value string) string {
	if !s.Color {
		return value
	}
	if s.Color256 {
		return "\x1b[38;5;245m" + value + "\x1b[0m"
	}
	return "\x1b[36m" + value + "\x1b[0m"
}

func (s Style) Value(kind, value string) string {
	if !s.Color {
		return value
	}
	switch strings.ToLower(kind) {
	case "ok", "success":
		return s.paint(value, "32", "40")
	case "warning":
		return s.paint(value, "33", "220")
	case "error":
		return s.paint(value, "31", "196")
	case "mode":
		return s.paint(value, "36", "45")
	default:
		return value
	}
}

func (s Style) paint(value, basic, color256 string) string {
	if s.Color256 {
		return fmt.Sprintf("\x1b[38;5;%sm%s\x1b[0m", color256, value)
	}
	return fmt.Sprintf("\x1b[%sm%s\x1b[0m", basic, value)
}
