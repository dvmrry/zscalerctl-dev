package output_test

import (
	"fmt"
	"testing"
	"unicode"

	"github.com/dvmrry/zscalerctl/internal/output"
)

func TestTerminalCellEscapesControlsAndUnicodeFormatRunesVisibly(t *testing.T) {
	t.Parallel()

	input := "plain 한글 e\u0301\n\r\t\x1b\x7f\u0085\u00ad\u200b\u200d\u202e\u2060\ufeff\U000e0041"
	want := `plain 한글 é\n\r\t\x1b\x7f\u0085\u00ad\u200b\u200d\u202e\u2060\ufeff\U000e0041`
	if got := output.TerminalCell(input); got != want {
		t.Fatalf("TerminalCell() = %q, want %q", got, want)
	}
}

func TestTerminalCellPreservesPrintableText(t *testing.T) {
	t.Parallel()

	const value = "zia/locations branch-01_name 東京 e\u0301"
	if got := output.TerminalCell(value); got != value {
		t.Fatalf("TerminalCell(%q) = %q, want unchanged", value, got)
	}
}

func TestTerminalCellEscapesEveryUnicodeFormatRune(t *testing.T) {
	t.Parallel()

	const wantFormatRuneCount = 170
	formatRuneCount := 0
	for r := rune(0); r <= unicode.MaxRune; r++ {
		if !unicode.Is(unicode.Cf, r) {
			continue
		}
		formatRuneCount++
		want := fmt.Sprintf(`\u%04x`, r)
		if r > 0xffff {
			want = fmt.Sprintf(`\U%08x`, r)
		}
		if got := output.TerminalCell(string(r)); got != want {
			t.Errorf("TerminalCell(U+%04X) = %q, want visible escape %q", r, got, want)
		}
	}
	if formatRuneCount != wantFormatRuneCount {
		t.Errorf("unicode.Cf rune count = %d, want %d for the pinned Go toolchain", formatRuneCount, wantFormatRuneCount)
	}
}
