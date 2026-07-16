package runtime

import (
	"context"
	"errors"

	"github.com/dvmrry/zscalerctl/internal/machine"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

// DiscoverCatalog returns a typed, config-free snapshot of static projected
// catalog metadata.
func DiscoverCatalog(
	ctx context.Context,
	catalog resources.ResourceCatalog,
) (machine.CatalogResult, error) {
	return (machine.Executor{Catalog: catalog}).DiscoverCatalog(ctx, machine.CatalogRequest{})
}

// DiscoverCatalog returns a typed snapshot of this runtime's catalog without
// invoking its live reader.
func (m *Machine) DiscoverCatalog(
	ctx context.Context,
	req machine.CatalogRequest,
) (machine.CatalogResult, error) {
	if m == nil {
		return machine.CatalogResult{}, errors.New("machine runtime is nil")
	}
	return (machine.Executor{Catalog: m.catalog}).DiscoverCatalog(ctx, req)
}
