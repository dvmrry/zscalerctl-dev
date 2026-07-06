package cli

import (
	"github.com/dvmrry/zscalerctl/internal/resources"
)

func knownProducts(catalog resources.ResourceCatalog) []resources.Product {
	// Derive from the enabled catalog so help and command dispatch always reflect
	// the products that actually have resources, instead of a hardcoded list that
	// drifts as batches merge.
	seen := make(map[resources.Product]bool)
	var products []resources.Product
	for _, spec := range catalog {
		if !seen[spec.Product] {
			seen[spec.Product] = true
			products = append(products, spec.Product)
		}
	}
	return products
}

func knownProductCommand(name string, catalog resources.ResourceCatalog) bool {
	for _, product := range knownProducts(catalog) {
		if name == string(product) {
			return true
		}
	}
	return false
}

func isRunnableCommand(name string, catalog resources.ResourceCatalog) bool {
	switch name {
	case "doctor", "auth", "config", "schema", "dump", "diff":
		return true
	default:
		return knownProductCommand(name, catalog)
	}
}

// isKnownCommand reports whether name is one of the top-level commands the
// dispatch switch in runParsed recognizes. The --fields guard uses it so that
// an unrecognized token — for example a product name a value-taking flag
// swallowed — still reaches the dispatch's more specific swallowed-product hint
// instead of the generic --fields usage error.
func isKnownCommand(name string, catalog resources.ResourceCatalog) bool {
	switch name {
	case "help", "version", "completion", "introspect", "machine":
		return true
	}
	return isRunnableCommand(name, catalog)
}

func productNames(products []resources.Product) []string {
	names := make([]string, len(products))
	for i, product := range products {
		names[i] = string(product)
	}
	return names
}

func productReadOperationNames(product resources.Product, catalog resources.ResourceCatalog) []string {
	seen := make(map[string]bool)
	for _, spec := range catalog {
		if spec.Product != product {
			continue
		}
		for _, op := range spec.Operations {
			if op.Capability == resources.CapabilityRead {
				seen[op.Name] = true
			}
		}
	}

	var names []string
	for _, name := range []string{"list", "get", "show"} {
		if seen[name] {
			names = append(names, name)
		}
	}
	return names
}

func readOperationNames(spec resources.ResourceSpec) []string {
	var names []string
	for _, op := range spec.Operations {
		if op.Capability == resources.CapabilityRead {
			names = append(names, op.Name)
		}
	}
	return names
}
