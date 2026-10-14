package resources

import (
	"fmt"
	"slices"
	"strings"

	"github.com/dvmrry/zscalerctl/internal/redact"
)

// CountByPathError reports a --count-by path that is not a field of the
// resource, or that ends at an object rather than a leaf value.
type CountByPathError struct {
	Product  Product
	Resource string
	Field    string
	NotLeaf  bool
}

func (e CountByPathError) Error() string {
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
	for _, path := range paths {
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
// a user in three groups adds one to each. A record with no value at a path
// counts under nil, so the counts always cover every record. With several
// paths, a record counts once per combination of its values. Rows are ordered
// by count, largest first, then by value.
func CountProjectedRecords(records ProjectedRecords, paths []string) []CountRow {
	type entry struct {
		values []any
		count  int
	}
	counts := map[string]*entry{}
	for _, record := range records.Records() {
		fields := record.Fields()
		combos := [][]any{{}}
		for _, path := range paths {
			values := distinctPathValues(fields, path)
			next := make([][]any, 0, len(combos)*len(values))
			for _, combo := range combos {
				for _, value := range values {
					next = append(next, append(slices.Clone(combo), value))
				}
			}
			combos = next
		}
		for _, combo := range combos {
			key := countKey(combo)
			if e, ok := counts[key]; ok {
				e.count++
			} else {
				counts[key] = &entry{values: combo, count: 1}
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

// compareCountValues orders equal counts by value, with a missing value (nil)
// after every present one.
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
		if c := strings.Compare(FormatProjectedValue(a[i]), FormatProjectedValue(b[i])); c != 0 {
			return c
		}
	}
	return 0
}

func distinctPathValues(fields map[string]any, path string) []any {
	var reached []any
	if value, ok := fields[path]; ok && !strings.Contains(path, ".") {
		reached = projectedPathValues(value, nil)
	} else {
		reached = projectedPathValues(fields, strings.Split(path, "."))
	}
	seen := map[string]bool{}
	var out []any
	for _, value := range reached {
		key := countKey([]any{value})
		if !seen[key] {
			seen[key] = true
			out = append(out, value)
		}
	}
	if len(out) == 0 {
		return []any{nil}
	}
	return out
}

func countKey(values []any) string {
	parts := make([]string, len(values))
	for i, value := range values {
		if value == nil {
			parts[i] = "\x00"
			continue
		}
		parts[i] = FormatProjectedValue(value)
	}
	return strings.Join(parts, "\x1f")
}
