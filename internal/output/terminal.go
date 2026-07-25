package output

import (
	"fmt"
	"strings"
	"unicode"
)

// TerminalCell visibly escapes runes that can alter terminal structure or
// presentation. JSON and other machine sinks intentionally remain lossless.
func TerminalCell(value string) string {
	var out strings.Builder
	out.Grow(len(value))
	for _, r := range value {
		switch {
		case r == '\n':
			out.WriteString(`\n`)
		case r == '\r':
			out.WriteString(`\r`)
		case r == '\t':
			out.WriteString(`\t`)
		case r < 0x20 || r == 0x7f:
			fmt.Fprintf(&out, `\x%02x`, r)
		case (r >= 0x80 && r <= 0x9f) || unicode.Is(unicode.Cf, r):
			if r <= 0xffff {
				fmt.Fprintf(&out, `\u%04x`, r)
			} else {
				fmt.Fprintf(&out, `\U%08x`, r)
			}
		default:
			out.WriteRune(r)
		}
	}
	return out.String()
}
