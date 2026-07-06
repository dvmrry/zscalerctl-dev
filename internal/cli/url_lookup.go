package cli

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/dvmrry/zscalerctl/internal/config"
	"github.com/dvmrry/zscalerctl/internal/output"
	machineruntime "github.com/dvmrry/zscalerctl/internal/runtime"
)

// urlLookupCommandName is the zia diagnostic verb for URL category lookups.
// It is a natural-verb diagnostic like doctor or auth status, not a catalog
// resource: it has no list/get/show operations and no schema-registry entry.
const urlLookupCommandName = "url-lookup"

const urlLookupUsageMessage = "usage: zscalerctl zia url-lookup <url> [url...]"

// urlLookupResult is the hand-built output-safe view of one lookup answer.
// Each field is copied explicitly from the adapter struct — no raw struct
// passthrough — and rendered through the normal renderer so redaction applies.
type urlLookupResult struct {
	URL                          string   `json:"url"`
	Classifications              []string `json:"classifications"`
	SecurityAlertClassifications []string `json:"security_alert_classifications"`
	Application                  string   `json:"application"`
}

type urlLookupResults []urlLookupResult

func (urlLookupResults) OutputSafe() {}

var urlLookupFieldOrder = []string{"url", "classifications", "security_alert_classifications", "application"}

func (a *App) runURLLookup(ctx context.Context, cfg config.Config, opts globalOptions, args []string) error {
	if len(args) == 0 {
		return UsageError{Message: urlLookupUsageMessage}
	}
	lookupURLs := make([]string, 0, len(args))
	for _, rawURL := range args {
		if strings.HasPrefix(rawURL, "-") {
			return UsageError{Message: fmt.Sprintf("url-lookup takes no flags (%q); global flags go before the command\n%s", rawURL, urlLookupUsageMessage)}
		}
		sanitized := sanitizeLookupURL(rawURL)
		if strings.TrimSpace(sanitized) == "" {
			return UsageError{Message: urlLookupUsageMessage}
		}
		lookupURLs = append(lookupURLs, sanitized)
	}
	lookup, err := a.urlLookup(ctx, cfg, opts)
	if err != nil {
		return err
	}
	classifications, err := callWithSpinner(a, opts, "contacting Zscaler", func() ([]machineruntime.URLClassification, error) {
		return lookup.Lookup(ctx, lookupURLs)
	})
	if err != nil {
		return err
	}
	results := newURLLookupResults(classifications)
	if opts.format == output.FormatJSON {
		return a.renderer(cfg, opts).WriteJSON(a.out, results)
	}
	if opts.format != output.FormatTable && opts.format != output.FormatPretty {
		return rejectUnsupportedFormat("zia url-lookup", opts.format)
	}
	if opts.format == output.FormatPretty {
		return a.renderer(cfg, opts).WriteText(a.out, output.RenderRecordsPretty(urlLookupFieldOrder, urlLookupRows(results), a.style(opts)))
	}
	return a.renderer(cfg, opts).WriteText(a.out, renderURLLookupTable(results, a.style(opts)))
}

// newURLLookupResults copies the adapter results into the output-safe view,
// normalizing nil slices to empty ones so the JSON shape is stable
// (classification fields always render as arrays, never null).
func newURLLookupResults(classifications []machineruntime.URLClassification) urlLookupResults {
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

func sanitizeLookupURL(raw string) string {
	value := strings.TrimSpace(raw)
	parsed, err := url.Parse(value)
	if err != nil {
		// Can't reliably strip secrets from an unparseable value, so reject it
		// (empty → caller emits a value-free usage error) rather than forward the
		// raw query/fragment/userinfo to the API or the rendered output.
		return ""
	}
	// Drop everything that can carry secrets or PII before the URL reaches the
	// API or the rendered output: userinfo credentials (user:pass@), the query
	// string, and the fragment. Host and path are kept — they determine the
	// category.
	parsed.User = nil
	parsed.RawQuery = ""
	parsed.ForceQuery = false
	parsed.Fragment = ""
	return parsed.String()
}

func nonNilStrings(values []string) []string {
	out := make([]string, 0, len(values))
	return append(out, values...)
}

func (a *App) urlLookup(
	ctx context.Context,
	cfg config.Config,
	opts globalOptions,
) (*machineruntime.URLLookup, error) {
	if a.reader != nil {
		return machineruntime.NewURLLookupFromReader(a.reader)
	}
	return machineruntime.NewURLLookupFromConfig(ctx, cfg, machineruntime.Options{
		Timeout:    opts.timeout,
		DiagLogger: a.sdkDiagLogger(opts),
	})
}
