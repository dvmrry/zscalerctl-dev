package output_test

import (
	"testing"

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
