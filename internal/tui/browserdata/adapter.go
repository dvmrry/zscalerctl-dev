// Package browserdata adapts safe, already-projected resource records into the
// BrowserData view model consumed by the TUI browser. It does not import Bubble
// Tea, config, credentials, or network clients.
package browserdata

import (
	"fmt"
	"sort"

	"github.com/dvmrry/zscalerctl/internal/resources"
	tui_tea "github.com/dvmrry/zscalerctl/internal/tui/tea"
)

// ProjectedRecordSource provides already-projected records for a resource spec.
// The adapter is intentionally decoupled from live readers so it can be tested
// and demoed with fake sources.
type ProjectedRecordSource interface {
	ProjectedRecords(spec resources.ResourceSpec) ([]resources.ProjectedRecord, error)
}

// Build converts a resource catalog and projected records into a BrowserData view
// model. The catalog order is preserved. A source error becomes a display-only
// error on the corresponding resource node; an empty result becomes an empty
// state.
func Build(catalog resources.ResourceCatalog, src ProjectedRecordSource) (tui_tea.BrowserData, error) {
	var products []tui_tea.ProductNode
	var current *tui_tea.ProductNode
	for _, spec := range catalog {
		if spec.Name == "" {
			continue
		}
		if current == nil || current.Name != string(spec.Product) {
			products = append(products, tui_tea.ProductNode{Name: string(spec.Product)})
			current = &products[len(products)-1]
		}
		node, err := buildResourceNode(spec, src)
		if err != nil {
			return tui_tea.BrowserData{}, err
		}
		current.Resources = append(current.Resources, node)
	}
	return tui_tea.BrowserData{Products: products}, nil
}

func buildResourceNode(spec resources.ResourceSpec, src ProjectedRecordSource) (tui_tea.ResourceNode, error) {
	node := tui_tea.ResourceNode{
		Product: string(spec.Product),
		Name:    spec.Name,
	}
	records, err := src.ProjectedRecords(spec)
	if err != nil {
		node.Error = err.Error()
		return node, nil
	}
	if len(records) == 0 {
		node.Empty = true
		return node, nil
	}
	for _, rec := range records {
		node.Records = append(node.Records, buildRecordSummary(spec, rec))
	}
	return node, nil
}

func buildRecordSummary(spec resources.ResourceSpec, rec resources.ProjectedRecord) tui_tea.RecordSummary {
	fields := rec.Fields()

	idKey := spec.EffectiveGetKey()
	if idKey == "" {
		idKey = "id"
	}
	id := fieldString(fields, idKey)
	name := fieldString(fields, "name")
	status := fieldString(fields, "status")
	if status == "" {
		status = "active"
	}
	detail := fieldString(fields, "description")

	summary := tui_tea.RecordSummary{
		ID:     id,
		Name:   name,
		Status: status,
		Detail: detail,
	}

	var keys []string
	for k := range fields {
		if k == idKey || k == "name" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		summary.Fields = append(summary.Fields, tui_tea.KV{
			Key:   k,
			Value: fmt.Sprintf("%v", fields[k]),
		})
	}
	return summary
}

func fieldString(fields map[string]any, key string) string {
	if v, ok := fields[key]; ok {
		return fmt.Sprintf("%v", v)
	}
	return ""
}
