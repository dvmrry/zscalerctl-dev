package runtime

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/diff"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

var (
	errSelectionInvalid = errors.New("invalid")
	errSelectionEmpty   = errors.New("empty")
	errSelectionCatalog = errors.New("catalog")
	errSelectionCancel  = errors.New("canceled")
)

// testSelectionErrors records which failure fired and the catalog sentinel.
func testSelectionErrors(sentinel *error) selectionErrors {
	return selectionErrors{
		boundary: func(error) error { return errSelectionCancel },
		catalog: func(err error) error {
			*sentinel = err
			return errSelectionCatalog
		},
		invalid: errSelectionInvalid,
		empty:   errSelectionEmpty,
	}
}

func selectionCatalog() resources.ResourceCatalog {
	getOnly := runtimeDumpListSpec(resources.ProductZPA, "get-only")
	getOnly.Operations = []resources.Operation{{Name: "get", Capability: resources.CapabilityRead}}
	return resources.ResourceCatalog{
		runtimeDumpListSpec(resources.ProductZIA, "users"),
		runtimeDumpListSpec(resources.ProductZPA, "segments"),
		runtimeDumpListSpec(resources.ProductZIA, "locations"),
		getOnly,
	}
}

func specKeys(specs []resources.ResourceSpec) []string {
	keys := make([]string, len(specs))
	for i, spec := range specs {
		keys[i] = string(spec.Product) + "/" + spec.Name
	}
	return keys
}

func TestSelectReadSpecsSelects(t *testing.T) {
	t.Parallel()

	catalog := selectionCatalog()
	for _, tc := range []struct {
		name      string
		products  []string
		selectors []resourceSelector
		want      []string
		selected  map[resources.Product]bool
		resources map[diff.ResourceKey]bool
	}{
		{
			name:     "everything readable, in catalog order",
			want:     []string{"zia/users", "zpa/segments", "zia/locations"},
			selected: map[resources.Product]bool{resources.ProductZIA: true, resources.ProductZPA: true},
		},
		{
			name:     "one product",
			products: []string{"zia"},
			want:     []string{"zia/users", "zia/locations"},
			selected: map[resources.Product]bool{resources.ProductZIA: true},
		},
		{
			name:      "one resource",
			selectors: []resourceSelector{{Product: "zia", Resource: "locations"}},
			want:      []string{"zia/locations"},
			selected:  map[resources.Product]bool{resources.ProductZIA: true, resources.ProductZPA: true},
			resources: map[diff.ResourceKey]bool{{Product: resources.ProductZIA, Name: "locations"}: true},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var sentinel error
			got, err := selectReadSpecs(context.Background(), catalog, tc.products, tc.selectors, testSelectionErrors(&sentinel))
			if err != nil {
				t.Fatalf("selectReadSpecs() error = %v, want nil", err)
			}
			if keys := specKeys(got.specs); !reflect.DeepEqual(keys, tc.want) {
				t.Fatalf("selectReadSpecs() specs = %v, want %v", keys, tc.want)
			}
			// diff filters products with this set; a lost set diffs every product.
			if !reflect.DeepEqual(got.products, tc.selected) {
				t.Fatalf("selectReadSpecs() products = %v, want %v", got.products, tc.selected)
			}
			if !reflect.DeepEqual(got.resources, tc.resources) {
				t.Fatalf("selectReadSpecs() resources = %v, want %v", got.resources, tc.resources)
			}
		})
	}
}

func TestSelectReadSpecsFailures(t *testing.T) {
	t.Parallel()

	duplicate := append(selectionCatalog(), runtimeDumpListSpec(resources.ProductZIA, "users"))
	broken := selectionCatalog()
	broken[0].Fields[0].AllowedModes = nil
	mutating := append(selectionCatalog(), resources.ResourceSpec{
		Product:    resources.ProductZIA,
		Name:       "mutating",
		Operations: []resources.Operation{{Name: "update", Capability: resources.CapabilityWrite}},
	})
	canceled, cancel := context.WithCancel(context.Background())
	cancel()

	for _, tc := range []struct {
		name         string
		ctx          context.Context
		catalog      resources.ResourceCatalog
		products     []string
		selectors    []resourceSelector
		want         error
		wantSentinel error
	}{
		{name: "mutating catalog", catalog: mutating, want: errSelectionCatalog, wantSentinel: resources.ErrMutatingOperation},
		{name: "invalid spec", catalog: broken, want: errSelectionCatalog, wantSentinel: resources.ErrInvalidResourceSpec},
		{name: "duplicate spec", catalog: duplicate, want: errSelectionCatalog},
		{name: "canceled", ctx: canceled, want: errSelectionCancel},
		{name: "blank product", products: []string{""}, want: errSelectionInvalid},
		{name: "padded product", products: []string{" zia"}, want: errSelectionInvalid},
		{name: "unknown product", products: []string{"zdx"}, want: errSelectionInvalid},
		{name: "repeated product", products: []string{"zia", "zia"}, want: errSelectionInvalid},
		{name: "blank resource", selectors: []resourceSelector{{Product: "zia"}}, want: errSelectionInvalid},
		{name: "padded resource", selectors: []resourceSelector{{Product: "zia", Resource: "users "}}, want: errSelectionInvalid},
		{name: "unknown resource", selectors: []resourceSelector{{Product: "zia", Resource: "nope"}}, want: errSelectionInvalid},
		{name: "unreadable resource", selectors: []resourceSelector{{Product: "zpa", Resource: "get-only"}}, want: errSelectionInvalid},
		{
			name:      "resource outside the selected products",
			products:  []string{"zpa"},
			selectors: []resourceSelector{{Product: "zia", Resource: "users"}},
			want:      errSelectionInvalid,
		},
		{
			name:      "repeated resource",
			selectors: []resourceSelector{{Product: "zia", Resource: "users"}, {Product: "zia", Resource: "users"}},
			want:      errSelectionInvalid,
		},
		{
			name:     "nothing readable",
			catalog:  resources.ResourceCatalog{selectionCatalog()[3]},
			products: []string{"zpa"},
			want:     errSelectionEmpty,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := tc.ctx
			if ctx == nil {
				ctx = context.Background()
			}
			catalog := tc.catalog
			if catalog == nil {
				catalog = selectionCatalog()
			}
			var sentinel error
			got, err := selectReadSpecs(ctx, catalog, tc.products, tc.selectors, testSelectionErrors(&sentinel))
			if err != tc.want {
				t.Fatalf("selectReadSpecs() error = %v, want %v", err, tc.want)
			}
			if sentinel != tc.wantSentinel {
				t.Fatalf("selectReadSpecs() catalog sentinel = %v, want %v", sentinel, tc.wantSentinel)
			}
			if got.specs != nil {
				t.Fatalf("selectReadSpecs() specs = %v on failure, want none", specKeys(got.specs))
			}
		})
	}
}
