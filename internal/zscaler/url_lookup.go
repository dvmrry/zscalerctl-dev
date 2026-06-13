package zscaler

import (
	"context"
	"errors"
	"fmt"

	zsdk "github.com/zscaler/zscaler-sdk-go/v3/zscaler"
	"github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/urlcategories"

	"github.com/dvmrry/zscalerctl/internal/resources"
)

// urlLookupName is the value-free label used in normalized url-lookup errors.
// url-lookup is a diagnostic verb, not a catalog resource, so the label exists
// only for error context and exit-code classification.
const urlLookupName = "url-lookup"

// URLClassification is the adapter-owned result of a ZIA URL lookup. Every
// field is copied explicitly from the SDK response struct so the CLI never
// holds (or renders) a raw SDK type.
type URLClassification struct {
	URL                          string
	Classifications              []string
	SecurityAlertClassifications []string
	Application                  string
}

// URLLookup resolves the URL category classifications for the given URLs via
// the ZIA urlLookup endpoint. The endpoint uses POST but performs a pure
// query: the request body is the list of URLs to classify and no tenant state
// is created, modified, or deleted (see "Diagnostic lookups" in
// docs/ARCHITECTURE.md).
func (r *SDKReader) URLLookup(ctx context.Context, urls []string) ([]URLClassification, error) {
	if r == nil {
		return nil, fmt.Errorf("%w: %s/%s", ErrUnsupportedResource, resources.ProductZIA, urlLookupName)
	}
	client := sdkClient{services: perCallService{cfg: r.cfg}}
	call := ziaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]urlcategories.URLClassification, error) {
		return urlcategories.GetURLLookup(ctx, service, urls)
	})
	return lookupURLClassifications(ctx, call)
}

// lookupURLClassifications runs a lookup call and applies the same error
// discipline as the resource read paths: missing credentials pass through for
// exit-code 3 classification, every other failure is normalized to a
// value-free error so SDK response or error content never reaches output.
func lookupURLClassifications(
	ctx context.Context,
	call func(context.Context) ([]urlcategories.URLClassification, error),
) ([]URLClassification, error) {
	results, err := call(ctx)
	if err != nil {
		if errors.Is(err, ErrMissingCredentials) {
			return nil, err
		}
		return nil, normalizeLiveError(ctx, "lookup", resources.ProductZIA, urlLookupName, err)
	}
	return urlClassificationsFromSDK(results), nil
}

// urlClassificationsFromSDK copies each SDK response field explicitly into the
// adapter-owned struct. All four SDK fields (url, urlClassifications,
// urlClassificationsWithSecurityAlert, application) are mapped; nothing is
// dropped silently and nothing is synthesized.
func urlClassificationsFromSDK(results []urlcategories.URLClassification) []URLClassification {
	out := make([]URLClassification, 0, len(results))
	for _, result := range results {
		out = append(out, URLClassification{
			URL:                          result.URL,
			Classifications:              append([]string(nil), result.URLClassifications...),
			SecurityAlertClassifications: append([]string(nil), result.URLClassificationsWithSecurityAlert...),
			Application:                  result.Application,
		})
	}
	return out
}
