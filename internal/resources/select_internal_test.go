package resources

import (
	"reflect"
	"testing"
)

// TestSelectDoesNotAliasStoredValues inspects the stored state directly:
// ProjectedRecord.Fields copies on read, so a test through the public API
// cannot see aliasing.
func TestSelectDoesNotAliasStoredValues(t *testing.T) {
	t.Parallel()

	groups := []any{map[string]any{"id": 10, "name": "a"}}
	owner := map[string]any{"name": "bob"}
	record := ProjectedRecord{fields: map[string]any{"groups": groups, "owner": owner}}

	for _, keys := range [][]string{{"groups", "owner"}, {"groups.id", "owner.name"}} {
		selected := record.Select(keys).fields
		if reflect.ValueOf(selected["groups"]).Pointer() == reflect.ValueOf(groups).Pointer() {
			t.Errorf("Select(%v) groups shares the stored slice", keys)
		}
		if reflect.ValueOf(selected["owner"]).Pointer() == reflect.ValueOf(owner).Pointer() {
			t.Errorf("Select(%v) owner shares the stored map", keys)
		}
		element := selected["groups"].([]any)[0].(map[string]any)
		if reflect.ValueOf(element).Pointer() == reflect.ValueOf(groups[0]).Pointer() {
			t.Errorf("Select(%v) groups[0] shares the stored map", keys)
		}
	}
}
