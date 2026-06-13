package cli

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/dvmrry/zscalerctl/internal/config"
	"github.com/dvmrry/zscalerctl/internal/output"
	"github.com/dvmrry/zscalerctl/internal/resources"
	"github.com/dvmrry/zscalerctl/internal/zscaler"
)

// urlLookupCommandName is the zia diagnostic verb for URL category lookups.
// It is a natural-verb diagnostic like doctor or auth status, not a catalog
// resource: it has no list/get/show operations and no schema-registry entry.
const urlLookupCommandName = "url-lookup"

const urlLookupUsageMessage = "usage: zscalerctl zia url-lookup <url> [url...]"

// URLLookupReader is the optional reader capability behind zia url-lookup.
// It is deliberately separate from ResourceReader so adding the diagnostic
// does not widen the resource interface every fake must implement; the CLI
// type-asserts and reports a clean unsupported error when absent.
type URLLookupReader interface {
	URLLookup(ctx context.Context, urls []string) ([]zscaler.URLClassification, error)
}

// The live SDK reader must keep satisfying the optional lookup capability.
var _ URLLookupReader = (*zscaler.SDKReader)(nil)

// urlLookupResult is the hand-built output-safe view of one lookup answer
// (doctorStatus pattern). Each field is copied explicitly from the adapter
// struct — no raw struct passthrough — and rendered through the normal
// renderer so redaction applies.
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
	reader, err := a.resourceReader(cfg, opts)
	if err != nil {
		return err
	}
	lookupReader, ok := reader.(URLLookupReader)
	if !ok {
		return fmt.Errorf("%w: %s/%s", zscaler.ErrUnsupportedResource, resources.ProductZIA, urlLookupCommandName)
	}
	classifications, err := lookupReader.URLLookup(ctx, lookupURLs)
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
func newURLLookupResults(classifications []zscaler.URLClassification) urlLookupResults {
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
		return value
	}
	parsed.RawQuery = ""
	parsed.ForceQuery = false
	parsed.Fragment = ""
	return parsed.String()
}

func nonNilStrings(values []string) []string {
	out := make([]string, 0, len(values))
	return append(out, values...)
}

func urlLookupRows(results urlLookupResults) [][]string {
	rows := make([][]string, 0, len(results))
	for _, result := range results {
		rows = append(rows, []string{
			formatTableValue(result.URL),
			formatTableValue(result.Classifications),
			formatTableValue(result.SecurityAlertClassifications),
			formatTableValue(result.Application),
		})
	}
	return rows
}

func renderURLLookupTable(results urlLookupResults, style output.Style) output.SafeText {
	var body strings.Builder
	for i, field := range urlLookupFieldOrder {
		if i > 0 {
			body.WriteByte('\t')
		}
		body.WriteString(style.Key(field))
	}
	body.WriteByte('\n')
	for _, row := range urlLookupRows(results) {
		for i, cell := range row {
			if i > 0 {
				body.WriteByte('\t')
			}
			body.WriteString(style.Value(urlLookupFieldOrder[i], cell))
		}
		body.WriteByte('\n')
	}
	return output.NewSafeText(body.String())
}
