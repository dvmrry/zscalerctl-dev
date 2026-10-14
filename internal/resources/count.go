package resources

import (
	"cmp"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/dvmrry/zscalerctl/internal/redact"
)

// CountByPathError reports a --count-by path that is not a field of the
// resource, or that ends at an object rather than a leaf value.
type CountByPathError struct {
	Product   Product
	Resource  string
	Field     string
	NotLeaf   bool
	Duplicate bool
}

func (e CountByPathError) Error() string {
	if e.Duplicate {
		return fmt.Sprintf("--count-by: %q is listed more than once", e.Field)
	}
	if e.NotLeaf {
		return fmt.Sprintf("--count-by: %q of %s/%s is an object; count by one of its fields, such as %s.id", e.Field, e.Product, e.Resource, e.Field)
	}
	return fmt.Sprintf("--count-by: %q is not a field of %s/%s", e.Field, e.Product, e.Resource)
}

// Unwrap returns the sentinel error for errors.Is checks.
func (e CountByPathError) Unwrap() error { return ErrUnknownField }

// ValidateCountByPaths checks --count-by paths against the catalog alone, so
// it can run before configuration is loaded: each path must be a field or a
// dotted path through nested fields, and must end at a leaf.
func ValidateCountByPaths(spec ResourceSpec, paths []string) error {
	seen := map[string]bool{}
	for _, path := range paths {
		if seen[path] {
			return CountByPathError{Product: spec.Product, Resource: spec.Name, Field: path, Duplicate: true}
		}
		seen[path] = true
		if !validFilterPath(spec.Fields, path) {
			return CountByPathError{Product: spec.Product, Resource: spec.Name, Field: path}
		}
		if field, ok := fieldByJSONName(spec.Fields, path); ok && len(field.Fields) > 0 {
			return CountByPathError{Product: spec.Product, Resource: spec.Name, Field: path, NotLeaf: true}
		}
	}
	return nil
}

// ValidateCountByPathsInMode adds the mode check: counting by a field the mode
// never renders would put every record under "missing", which reads as data.
func ValidateCountByPathsInMode(spec ResourceSpec, mode redact.Mode, paths []string) error {
	if err := ValidateCountByPaths(spec, paths); err != nil {
		return err
	}
	for _, path := range paths {
		availability, _ := FieldPathAvailability(spec, path)
		if !availability.RenderedIn(mode) {
			return SuppressedFieldError{
				Flag:         "--count-by",
				Product:      spec.Product,
				Resource:     spec.Name,
				Field:        path,
				Mode:         mode,
				Secret:       availability.Secret,
				RenderableIn: availability.RenderableIn,
			}
		}
	}
	return nil
}

// CountRow is one combination of counted values and the number of records
// carrying it. A nil value means the record had no value at that path.
type CountRow struct {
	Values []any
	Count  int
}

// CountProjectedRecords groups already projected (and filtered) records by
// the values at each path. A path reaching a list counts each distinct element
// once per record, so counting users by groups.id gives members per group and
// a user in three groups adds one to each. Paths through the same list are
// read from the same element: groups.id with groups.name pairs each group's
// id with its own name, never with a sibling's. Paths on independent branches
// combine every value of one with every value of the other, since each pair
// is true of the record. A record with no value at a path counts under nil,
// so the counts always cover every record. Rows are ordered by count, largest
// first, then by value.
func CountProjectedRecords(records ProjectedRecords, paths []string) []CountRow {
	type entry struct {
		values []any
		count  int
	}
	counts := map[string]*entry{}
	for _, record := range records.Records() {
		fields := record.Fields()
		segments := make([][]string, len(paths))
		for i, path := range paths {
			if _, ok := fields[path]; ok {
				segments[i] = []string{path}
			} else {
				segments[i] = strings.Split(path, ".")
			}
		}
		for _, tuple := range pathTuples(fields, segments) {
			key := countKey(tuple)
			if e, ok := counts[key]; ok {
				e.count++
			} else {
				counts[key] = &entry{values: tuple, count: 1}
			}
		}
	}
	rows := make([]CountRow, 0, len(counts))
	for _, e := range counts {
		rows = append(rows, CountRow{Values: e.values, Count: e.count})
	}
	slices.SortFunc(rows, func(a, b CountRow) int {
		if a.Count != b.Count {
			return b.Count - a.Count
		}
		return compareCountValues(a.Values, b.Values)
	})
	return rows
}

// pathTuples returns the distinct value tuples that value holds for paths
// (each relative to value), one value per path, nil where a path reaches
// nothing. Paths sharing a first segment are resolved together inside it, so
// a list is walked once and its elements are never mixed.
func pathTuples(value any, paths [][]string) [][]any {
	if items, ok := listItems(value); ok {
		var out [][]any
		seen := map[string]bool{}
		for _, item := range items {
			for _, tuple := range pathTuples(item, paths) {
				if key := countKey(tuple); !seen[key] {
					seen[key] = true
					out = append(out, tuple)
				}
			}
		}
		if len(out) == 0 {
			return [][]any{make([]any, len(paths))}
		}
		return out
	}
	object, _ := value.(map[string]any)
	tuples := [][]any{make([]any, len(paths))}
	var order []string
	groups := map[string][]int{}
	for i, path := range paths {
		if len(path) == 0 {
			for _, tuple := range tuples {
				tuple[i] = value
			}
			continue
		}
		if _, ok := groups[path[0]]; !ok {
			order = append(order, path[0])
		}
		groups[path[0]] = append(groups[path[0]], i)
	}
	for _, name := range order {
		indexes := groups[name]
		child, ok := object[name]
		if !ok {
			continue
		}
		rest := make([][]string, len(indexes))
		for j, i := range indexes {
			rest[j] = paths[i][1:]
		}
		sub := pathTuples(child, rest)
		next := make([][]any, 0, len(tuples)*len(sub))
		for _, tuple := range tuples {
			for _, part := range sub {
				combined := slices.Clone(tuple)
				for j, i := range indexes {
					combined[i] = part[j]
				}
				next = append(next, combined)
			}
		}
		tuples = next
	}
	return tuples
}

func listItems(value any) ([]any, bool) {
	switch v := value.(type) {
	case []any:
		return v, true
	case []string:
		items := make([]any, len(v))
		for i, item := range v {
			items[i] = item
		}
		return items, true
	}
	return nil, false
}

// compareCountValues orders equal counts by value: numbers numerically,
// other values by their rendered text, and a missing value (nil) after every
// present one.
func compareCountValues(a, b []any) int {
	for i := range a {
		switch {
		case a[i] == nil && b[i] == nil:
			continue
		case a[i] == nil:
			return 1
		case b[i] == nil:
			return -1
		}
		if x, ok := countNumber(a[i]); ok {
			if y, ok := countNumber(b[i]); ok {
				if c := cmp.Compare(x, y); c != 0 {
					return c
				}
				continue
			}
		}
		if c := strings.Compare(FormatProjectedValue(a[i]), FormatProjectedValue(b[i])); c != 0 {
			return c
		}
	}
	return 0
}

func countNumber(value any) (float64, bool) {
	switch v := value.(type) {
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case float64:
		return v, true
	case json.Number:
		f, err := v.Float64()
		return f, err == nil
	}
	return 0, false
}

// countKey identifies a value tuple by its JSON encoding, so 1 and "1" (or a
// value containing any separator) stay distinct.
func countKey(values []any) string {
	encoded, err := json.Marshal(values)
	if err != nil {
		return fmt.Sprintf("%#v", values)
	}
	return string(encoded)
}
