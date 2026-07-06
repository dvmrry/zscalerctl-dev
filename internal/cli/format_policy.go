package cli

import (
	"fmt"
	"strings"

	"github.com/dvmrry/zscalerctl/internal/output"
)

// RequestedFormatRaw returns the --format value as parsed
// (auto/table/json/ndjson/pretty), defaulting to auto, without resolving auto
// against a TTY. The error renderer in main uses it so error output follows the
// same format the data path will use.
func RequestedFormatRaw(args []string) output.Format {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			return output.FormatAuto
		}
		name, hasValue := flagName(arg)
		if name != "format" {
			continue
		}
		value := ""
		if hasValue {
			_, after, _ := strings.Cut(arg, "=")
			value = after
		} else if i+1 < len(args) {
			value = args[i+1]
		}
		if f, err := output.ParseFormat(value); err == nil {
			return f
		}
		return output.FormatAuto
	}
	return output.FormatAuto
}

// rejectUnsupportedFormat returns an error when command does not support the
// given format. JSON is handled separately (fast-path) before this guard, so
// only non-table/non-pretty formats reach here.
func rejectUnsupportedFormat(command string, format output.Format) error {
	return UsageError{Message: fmt.Sprintf("%s does not support %s output yet", command, format)}
}

// resolveFormat collapses the auto format to a concrete one at the point where
// the destination is known: a real stdout TTY (and no --output file) gets the
// pretty human renderer, everything else (pipe, redirect, --output file) gets
// json so pipelines stay machine-parseable. Explicit --format choices pass
// through untouched.
func (a *App) resolveFormat(opts globalOptions) output.Format {
	if opts.format != output.FormatAuto {
		return opts.format
	}
	if a.stdoutTTY && opts.output == "" {
		return output.FormatPretty
	}
	return output.FormatJSON
}
