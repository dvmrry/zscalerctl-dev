package runtime

import (
	"strings"
	"unicode"

	"github.com/dvmrry/zscalerctl/internal/machine"
	"github.com/dvmrry/zscalerctl/internal/redact"
)

// boundaryError exposes a static MachineError and one separately supplied safe
// sentinel. It never retains the original backend error.
type boundaryError struct {
	machineErr *machine.MachineError
	sentinel   error
}

func (e *boundaryError) Error() string { return e.machineErr.Error() }

func (e *boundaryError) Unwrap() []error {
	return []error{e.machineErr, e.sentinel}
}

func newBoundaryError(machineErr *machine.MachineError, sentinel error) error {
	if sentinel == nil {
		return machineErr
	}
	return &boundaryError{machineErr: machineErr, sentinel: sentinel}
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
