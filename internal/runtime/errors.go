package runtime

import (
	"strings"
	"unicode"

	"github.com/dvmrry/zscalerctl/internal/machine"
	"github.com/dvmrry/zscalerctl/internal/redact"
)

func newBoundaryError(machineErr *machine.MachineError, sentinel error) error {
	return machine.ErrorWithCause(machineErr, sentinel)
}

func sanitizeEngineString(r redact.Redactor, value string) string {
	value, _ = r.ScanRenderedString(value)
	return strings.Map(func(ch rune) rune {
		if unicode.IsControl(ch) || unicode.Is(unicode.Cf, ch) {
			return ' '
		}
		return ch
	}, value)
}
