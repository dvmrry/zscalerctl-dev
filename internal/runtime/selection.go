package runtime

import (
	"context"
	"strings"

	"github.com/dvmrry/zscalerctl/internal/diff"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

// readSelection is a validated product and resource selection over a
// read-only catalog. Dump and diff select the specs they read the same way.
type readSelection struct {
	// specs are copies of the selected list/show specs, in catalog order.
	specs    []resources.ResourceSpec
	products map[resources.Product]bool
	// resources is nil when no resource was named: every readable resource
	// of the selected products is selected.
	resources map[diff.ResourceKey]bool
}

// resourceSelector is a requested product/resource pair.
type resourceSelector struct {
	Product  string
	Resource string
}

// selectionErrors are an operation's own errors for each way a selection
// fails, so messages and kinds stay the operation's.
type selectionErrors struct {
	boundary func(error) error // the context ended
	catalog  func(error) error // the catalog is invalid: a sentinel, or nil for a duplicate spec
	invalid  error             // a product or resource that is blank, padded, unknown or repeated
	empty    error             // nothing readable was selected
}

// selectReadSpecs validates catalog and selects the list/show specs that
// products (all known products when empty) and selectors (every resource of
// the selected products when empty) name.
func selectReadSpecs(
	ctx context.Context,
	catalog resources.ResourceCatalog,
	products []string,
	selectors []resourceSelector,
	errs selectionErrors,
) (readSelection, error) {
	if err := resources.AssertReadOnly(catalog...); err != nil {
		return readSelection{}, errs.catalog(resources.ErrMutatingOperation)
	}

	knownProducts := make(map[resources.Product]bool)
	readable := make(map[diff.ResourceKey]bool)
	seenCatalog := make(map[diff.ResourceKey]bool)
	for _, spec := range catalog {
		if err := ctx.Err(); err != nil {
			return readSelection{}, errs.boundary(err)
		}
		if err := spec.Validate(); err != nil {
			return readSelection{}, errs.catalog(resources.ErrInvalidResourceSpec)
		}
		key := diff.ResourceKey{Product: spec.Product, Name: spec.Name}
		if seenCatalog[key] {
			return readSelection{}, errs.catalog(nil)
		}
		seenCatalog[key] = true
		knownProducts[spec.Product] = true
		if spec.SupportsReadOperation("list") || spec.SupportsReadOperation("show") {
			readable[key] = true
		}
	}

	selectedProducts := make(map[resources.Product]bool)
	if len(products) == 0 {
		for product := range knownProducts {
			selectedProducts[product] = true
		}
	} else {
		for _, rawProduct := range products {
			if err := ctx.Err(); err != nil {
				return readSelection{}, errs.boundary(err)
			}
			if rawProduct == "" || strings.TrimSpace(rawProduct) != rawProduct {
				return readSelection{}, errs.invalid
			}
			product := resources.Product(rawProduct)
			if !knownProducts[product] || selectedProducts[product] {
				return readSelection{}, errs.invalid
			}
			selectedProducts[product] = true
		}
	}

	selectedResources := make(map[diff.ResourceKey]bool)
	for _, selector := range selectors {
		if err := ctx.Err(); err != nil {
			return readSelection{}, errs.boundary(err)
		}
		if selector.Product == "" || selector.Resource == "" ||
			strings.TrimSpace(selector.Product) != selector.Product ||
			strings.TrimSpace(selector.Resource) != selector.Resource {
			return readSelection{}, errs.invalid
		}
		key := diff.ResourceKey{Product: resources.Product(selector.Product), Name: selector.Resource}
		if !selectedProducts[key.Product] || selectedResources[key] || !readable[key] {
			return readSelection{}, errs.invalid
		}
		selectedResources[key] = true
	}

	selection := readSelection{products: selectedProducts}
	if len(selectedResources) > 0 {
		selection.resources = selectedResources
	}
	for _, spec := range catalog {
		if err := ctx.Err(); err != nil {
			return readSelection{}, errs.boundary(err)
		}
		key := diff.ResourceKey{Product: spec.Product, Name: spec.Name}
		if !selectedProducts[key.Product] || !readable[key] {
			continue
		}
		if selection.resources != nil && !selection.resources[key] {
			continue
		}
		selection.specs = append(selection.specs, copyResourceSpec(spec))
	}
	if len(selection.specs) == 0 {
		return readSelection{}, errs.empty
	}
	return selection, nil
}
