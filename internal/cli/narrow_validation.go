package cli

import (
	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

// validateProductNarrowing performs the catalog-only portion of product
// command validation before RunE loads config or constructs a reader. The
// normal runParsed gates already scope --filter/--search/--fields to resource
// reads; this check validates the names used by those options while the
// process is still config- and credential-free.
func validateProductNarrowing(
	product resources.Product,
	args []string,
	catalog resources.ResourceCatalog,
	opts globalOptions,
) error {
	if len(args) < 2 || (len(opts.fields) == 0 && len(opts.filters) == 0) {
		return nil
	}
	op := args[1]
	if op != "list" && op != "get" && op != "show" {
		return nil
	}
	spec, ok := catalog.FindSpec(product, args[0])
	if !ok {
		// Resource lookup and its established not-found diagnostic belong to
		// runProduct. Leave that path intact when the resource itself is not in
		// the catalog.
		return nil
	}
	filters := make([]resources.ProjectedFilter, 0, len(opts.filters))
	for _, filter := range opts.filters {
		filters = append(filters, resources.ProjectedFilter{
			Field:     filter.key,
			Value:     filter.value,
			Substring: filter.substring,
		})
	}
	if err := resources.ValidateNarrowOptions(spec, resources.NarrowOptions{
		Fields:  opts.fields,
		Filters: filters,
	}); err != nil {
		// App.Run is also used directly by in-process callers, so sanitize the
		// diagnostic here rather than relying only on cmd/zscalerctl's final
		// error writer. Field names are client-controlled and are reflected in
		// unknown-name diagnostics.
		message, _ := redact.New(redact.ModeStandard).ScanRenderedString(err.Error())
		return UsageError{Message: message}
	}
	return nil
}
