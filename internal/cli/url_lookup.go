package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/dvmrry/zscalerctl/internal/machine"
	"github.com/dvmrry/zscalerctl/internal/output"
	"github.com/dvmrry/zscalerctl/internal/redact"
	machineruntime "github.com/dvmrry/zscalerctl/internal/runtime"
)

// urlLookupCommandName is the zia diagnostic verb for URL category lookups.
// It is a natural-verb diagnostic like doctor or auth status, not a catalog
// resource: it has no list/get/show operations and no schema-registry entry.
const urlLookupCommandName = "url-lookup"

const urlLookupUsageMessage = "usage: zscalerctl zia url-lookup <url> [url...]"

// urlLookupResult is the adapter-owned output view. Values have already crossed
// the engine redaction and control-normalization boundary; this explicit copy
// preserves the supported CLI JSON field names and order.
type urlLookupResult struct {
	URL                          string   `json:"url"`
	Classifications              []string `json:"classifications"`
	SecurityAlertClassifications []string `json:"security_alert_classifications"`
	Application                  string   `json:"application"`
}

type urlLookupResults []urlLookupResult

func (urlLookupResults) OutputSafe() {}

var urlLookupFieldOrder = []string{"url", "classifications", "security_alert_classifications", "application"}

func (a *App) runURLLookup(ctx context.Context, opts globalOptions, args []string) error {
	if len(args) == 0 {
		return UsageError{Message: urlLookupUsageMessage}
	}
	for _, rawURL := range args {
		if strings.HasPrefix(rawURL, "-") {
			return UsageError{Message: fmt.Sprintf("url-lookup takes no flags (%q); global flags go before the command\n%s", rawURL, urlLookupUsageMessage)}
		}
	}
	if opts.format != output.FormatJSON && opts.format != output.FormatTable && opts.format != output.FormatPretty {
		return rejectUnsupportedFormat("zia url-lookup", opts.format)
	}

	result, err := callWithSpinner(a, opts, "contacting Zscaler", func() (machine.URLLookupResult, error) {
		return a.lookupURL(ctx, opts, machine.URLLookupRequest{URLs: append([]string(nil), args...)})
	})
	if err != nil {
		return cliErrorFromURLLookup(err)
	}
	results := newURLLookupResults(result.Classifications())
	renderer := output.NewRenderer(redact.New(redact.ModeStandard))
	if opts.format == output.FormatJSON {
		return renderer.WriteJSON(a.out, results)
	}
	if opts.format == output.FormatPretty {
		return renderer.WriteText(a.out, output.RenderRecordsPretty(urlLookupFieldOrder, urlLookupRows(results), a.style(opts)))
	}
	return renderer.WriteText(a.out, renderURLLookupTable(results, a.style(opts)))
}

// newURLLookupResults copies engine results into the supported output DTO and
// keeps empty classification fields as arrays rather than null.
func newURLLookupResults(classifications []machine.URLClassification) urlLookupResults {
	results := make(urlLookupResults, 0, len(classifications))
	for _, classification := range classifications {
		results = append(results, urlLookupResult{
			URL:                          classification.URL,
			Classifications:              nonNilStrings(classification.Classifications),
			SecurityAlertClassifications: nonNilStrings(classification.SecurityAlertClassifications),
			Application:                  classification.Application,
		})
	}
	return results
}

func nonNilStrings(values []string) []string {
	out := make([]string, 0, len(values))
	return append(out, values...)
}

func (a *App) lookupURL(
	ctx context.Context,
	opts globalOptions,
	req machine.URLLookupRequest,
) (machine.URLLookupResult, error) {
	if a.reader != nil {
		mode := redact.ModeStandard
		if opts.redactionSet {
			mode = opts.redaction
		}
		lookup, err := machineruntime.NewURLLookupFromReader(a.reader, mode)
		if err != nil {
			return machine.URLLookupResult{}, err
		}
		return lookup.Lookup(ctx, req)
	}
	engine, err := machineruntime.NewEngine(machineruntime.Options{
		Env:          a.env,
		Profile:      opts.profile,
		ConfigPath:   opts.configPath,
		Timeout:      opts.timeout,
		Redaction:    opts.redaction,
		RedactionSet: opts.redactionSet,
		NoCache:      opts.noCache,
		Catalog:      a.resourceCatalog(),
		DiagLogger:   a.sdkDiagLogger(opts),
	})
	if err != nil {
		return machine.URLLookupResult{}, err
	}
	return engine.LookupURL(ctx, req)
}

func cliErrorFromURLLookup(err error) error {
	if err == nil {
		return nil
	}
	machineErr, ok := asMachineError(err)
	if ok && machineErr.Kind == machine.ErrorKindUsage {
		return UsageError{Message: urlLookupUsageMessage}
	}
	return err
}
