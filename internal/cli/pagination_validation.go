package cli

import (
	"fmt"

	"github.com/dvmrry/zscalerctl/internal/output"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

// validatePaginationInvocation applies the CLI-only page contract before any
// command RunE can load configuration or construct a reader. It deliberately
// validates the invocation shape rather than changing the shared machine
// request: collection still runs to completion, and pagination is a view over
// the projected and filtered result.
func validatePaginationInvocation(opts globalOptions, rest []string, catalog resources.ResourceCatalog) error {
	if !opts.limitSet && !opts.offsetSet {
		return nil
	}
	if !opts.limitSet {
		return UsageError{Message: "--offset requires --limit"}
	}
	if opts.limit <= 0 {
		return UsageError{Message: "--limit must be positive"}
	}
	if opts.offset < 0 {
		return UsageError{Message: "--offset must be nonnegative"}
	}
	if opts.format != output.FormatJSON {
		return UsageError{Message: fmt.Sprintf("--limit requires JSON output; got --format %s", opts.format)}
	}
	if len(rest) != 3 || !knownProductCommand(rest[0], catalog) || rest[2] != "list" {
		return UsageError{Message: "--limit applies to list operations only; use it with \"<product> <resource> list\""}
	}
	return nil
}
