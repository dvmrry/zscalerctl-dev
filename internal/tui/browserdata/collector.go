// Package browserdata adapts safe, already-projected resource records into the
// BrowserData view model consumed by the TUI browser.
package browserdata

import (
	"context"
	"fmt"

	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
	tui_tea "github.com/dvmrry/zscalerctl/internal/tui/tea"
)

// RecordReader provides source records for a resource. It is a deliberately
// narrow interface so the collector can be tested with fake readers and does not
// depend on the live Zscaler client.
type RecordReader interface {
	List(ctx context.Context, product resources.Product, resource string) ([]resources.SourceRecord, error)
	Show(ctx context.Context, product resources.Product, resource string) (resources.SourceRecord, error)
}

// CollectOptions controls which products and resources are collected and how
// resource errors are handled.
type CollectOptions struct {
	Products        []resources.Product
	Resources       []string
	ContinueOnError bool
}

// Collector coordinates catalog selection, source-record collection, projection,
// and conversion into BrowserData. It does not import Bubble Tea and does not
// load config or credentials.
type Collector struct {
	Catalog resources.ResourceCatalog
	Reader  RecordReader
	Mode    redact.Mode
}

// Collect iterates the selected resources, reads and projects their records,
// and returns a BrowserData view model. When ContinueOnError is true, a resource
// error becomes an error state on the corresponding resource node; otherwise the
// first error is returned immediately.
func (c *Collector) Collect(ctx context.Context, opts CollectOptions) (tui_tea.BrowserData, error) {
	mode := c.Mode
	if mode == "" {
		mode = redact.ModeStandard
	}
	filtered := filterCatalog(c.Catalog, opts.Products, opts.Resources)
	results := make(map[string]collectionResult, len(filtered))
	for _, spec := range filtered {
		if err := ctx.Err(); err != nil {
			return tui_tea.BrowserData{}, err
		}
		res, err := c.collectResource(ctx, spec, mode)
		if err != nil {
			if opts.ContinueOnError {
				results[specKey(spec)] = collectionResult{err: err}
				continue
			}
			return tui_tea.BrowserData{}, err
		}
		results[specKey(spec)] = res
	}
	return Build(filtered, staticSource(results))
}

func (c *Collector) collectResource(ctx context.Context, spec resources.ResourceSpec, mode redact.Mode) (collectionResult, error) {
	var sourceRecords []resources.SourceRecord
	var err error
	switch {
	case spec.SupportsReadOperation("list"):
		sourceRecords, err = c.Reader.List(ctx, spec.Product, spec.Name)
	case spec.SupportsReadOperation("show"):
		var rec resources.SourceRecord
		rec, err = c.Reader.Show(ctx, spec.Product, spec.Name)
		if err == nil {
			sourceRecords = []resources.SourceRecord{rec}
		}
	default:
		return collectionResult{}, nil
	}
	if err != nil {
		return collectionResult{}, err
	}
	if len(sourceRecords) == 0 {
		return collectionResult{}, nil
	}
	projected, _, err := resources.ProjectRecordsAndVerify(spec, mode, sourceRecords)
	if err != nil {
		return collectionResult{}, err
	}
	return collectionResult{records: projected.Records()}, nil
}

func filterCatalog(catalog resources.ResourceCatalog, products []resources.Product, resourceNames []string) resources.ResourceCatalog {
	productSet := make(map[resources.Product]bool, len(products))
	for _, p := range products {
		productSet[p] = true
	}
	resourceSet := make(map[string]bool, len(resourceNames))
	for _, r := range resourceNames {
		resourceSet[r] = true
	}
	out := make(resources.ResourceCatalog, 0, len(catalog))
	for _, spec := range catalog {
		if len(productSet) > 0 && !productSet[spec.Product] {
			continue
		}
		if len(resourceSet) > 0 && !resourceSet[spec.Name] {
			continue
		}
		out = append(out, spec)
	}
	return out
}

type collectionResult struct {
	records []resources.ProjectedRecord
	err     error
}

type staticSource map[string]collectionResult

func (s staticSource) ProjectedRecords(spec resources.ResourceSpec) ([]resources.ProjectedRecord, error) {
	res := s[specKey(spec)]
	return res.records, res.err
}

func specKey(spec resources.ResourceSpec) string {
	return fmt.Sprintf("%s/%s", spec.Product, spec.Name)
}
