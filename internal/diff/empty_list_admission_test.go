package diff

import (
	"context"
	"reflect"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

// TestAdmitRecordsKeepsSavedEmptyLists pins that a saved dump record's []
// survives admission: projection renders [] only for a list the API returned
// empty, so a stored [] is deliberate, and admission's re-projection must not
// drop it (which would also fail its equality check and reject the dump).
func TestAdmitRecordsKeepsSavedEmptyLists(t *testing.T) {
	t.Parallel()

	spec, ok := resources.Catalog().FindSpec(resources.ProductZIA, "users")
	if !ok {
		t.Fatal("zia/users not in catalog")
	}
	saved := []map[string]any{
		{"id": float64(1), "groups": []any{}, "authMethods": []any{}},
		{"id": float64(2), "groups": []any{map[string]any{"id": float64(10), "name": "OPS"}}},
	}
	admitted, err := admitRecords(context.Background(), spec, redact.ModeStandard, saved)
	if err != nil {
		t.Fatalf("admitRecords(saved [] lists) error = %v", err)
	}
	if !reflect.DeepEqual(admitted, saved) {
		t.Errorf("admitted = %#v, want the saved records unchanged %#v", admitted, saved)
	}
}
