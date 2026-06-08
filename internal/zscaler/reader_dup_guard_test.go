package zscaler

import (
	"testing"

	"github.com/dvmrry/zscalerctl/internal/resources"
)

// TestAddHandlerPanicsOnDuplicateKey locks in the safety property restored by
// addHandler: after the per-product registry split, a duplicate {product,name}
// across product files must fail loudly at construction instead of silently
// overwriting (the old single map literal caught this at compile time).
func TestAddHandlerPanicsOnDuplicateKey(t *testing.T) {
	m := map[resourceKey]resourceHandler{}
	key := resourceKey{product: resources.ProductZIA, name: "dup-key-guard-test"}

	addHandler(m, key, nil) // first registration is fine

	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on duplicate handler registration")
		}
	}()
	addHandler(m, key, nil) // duplicate must panic
}
