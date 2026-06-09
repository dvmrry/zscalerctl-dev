package zscaler

import (
	"context"

	zsdk "github.com/zscaler/zscaler-sdk-go/v3/zscaler"
	zccfailopen "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zcc/services/failopen_policy"

	"github.com/dvmrry/zscalerctl/internal/resources"
)

// zccProbePageSize bounds the single-page fail-open policy probe read. Fail-open
// policy is effectively a per-company singleton, so one page is sufficient.
const zccProbePageSize = 100

func addZCCHandlers(m map[resourceKey]resourceHandler, client sdkClient) {
	entries := map[resourceKey]resourceHandler{
		{product: resources.ProductZCC, name: resourceZCCFailOpenPolicy}: newListOnlyHandler(
			resourceZCCFailOpenPolicy,
			sdkProductList(resources.ProductZCC, client, func(ctx context.Context, service *zsdk.Service) ([]zccfailopen.WebFailOpenPolicy, error) {
				return zccfailopen.GetFailOpenPolicy(ctx, service, zccProbePageSize)
			}),
			structSourceRecord[zccfailopen.WebFailOpenPolicy],
		),
	}
	for k, v := range entries {
		addHandler(m, k, v)
	}
}
