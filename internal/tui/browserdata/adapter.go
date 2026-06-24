// Package browserdata adapts safe, already-projected resource records into the
// data.BrowserData view model consumed by the TUI browser. It does not import Bubble
// Tea, config, credentials, or network clients.
package browserdata

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"

	"github.com/dvmrry/zscalerctl/internal/resources"
	"github.com/dvmrry/zscalerctl/internal/tui/data"
)

// ProjectedRecordSource provides already-projected records for a resource spec.
// The adapter is intentionally decoupled from live readers so it can be tested
// and demoed with fake sources.
type ProjectedRecordSource interface {
	ProjectedRecords(spec resources.ResourceSpec) ([]resources.ProjectedRecord, error)
}

// Build converts a resource catalog and projected records into a data.BrowserData view
// model. The catalog order is preserved. A source error becomes a display-only
// error on the corresponding resource node; an empty result becomes an empty
// state.
func Build(catalog resources.ResourceCatalog, src ProjectedRecordSource) (data.BrowserData, error) {
	var products []data.ProductNode
	var current *data.ProductNode
	for _, spec := range catalog {
		if spec.Name == "" {
			continue
		}
		if current == nil || current.Name != string(spec.Product) {
			products = append(products, data.ProductNode{Name: string(spec.Product)})
			current = &products[len(products)-1]
		}
		node, err := buildResourceNode(spec, src)
		if err != nil {
			return data.BrowserData{}, err
		}
		current.Resources = append(current.Resources, node)
	}
	return data.BrowserData{Products: products}, nil
}

// BuildUnloadedCatalog converts a resource catalog into a BrowserData view
// model without reading records. Live TUI mode uses this so first paint only
// depends on config, credentials, reader construction, and catalog filtering.
func BuildUnloadedCatalog(catalog resources.ResourceCatalog) data.BrowserData {
	var products []data.ProductNode
	var current *data.ProductNode
	for _, spec := range catalog {
		if spec.Name == "" {
			continue
		}
		if current == nil || current.Name != string(spec.Product) {
			products = append(products, data.ProductNode{Name: string(spec.Product)})
			current = &products[len(products)-1]
		}
		current.Resources = append(current.Resources, data.ResourceNode{
			Product: string(spec.Product),
			Name:    spec.Name,
			State:   data.ResourceStateUnloaded,
		})
	}
	return data.BrowserData{Products: products}
}

func buildResourceNode(spec resources.ResourceSpec, src ProjectedRecordSource) (data.ResourceNode, error) {
	node := data.ResourceNode{
		Product: string(spec.Product),
		Name:    spec.Name,
		State:   data.ResourceStateLoaded,
	}
	records, err := src.ProjectedRecords(spec)
	if err != nil {
		node.State = data.ResourceStateError
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

func buildRecordSummary(spec resources.ResourceSpec, rec resources.ProjectedRecord) data.RecordSummary {
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

	summary := data.RecordSummary{
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
		summary.Fields = append(summary.Fields, data.KV{
			Key:   k,
			Value: formatProjectedValue(fields[k]),
		})
	}
	return summary
}

func formatProjectedValue(v any) string {
	if v == nil {
		return "null"
	}
	if s, ok := v.(string); ok {
		return s
	}
	value := reflect.ValueOf(v)
	for value.Kind() == reflect.Pointer || value.Kind() == reflect.Interface {
		if value.IsNil() {
			return "null"
		}
		value = value.Elem()
	}
	switch value.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice, reflect.Struct:
		if b, err := json.MarshalIndent(v, "", "  "); err == nil {
			return string(b)
		}
	}
	return fmt.Sprintf("%v", v)
}

func fieldString(fields map[string]any, key string) string {
	if v, ok := fields[key]; ok {
		return fmt.Sprintf("%v", v)
	}
	return ""
}
