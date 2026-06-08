package zscaler

import (
	"context"

	zsdk "github.com/zscaler/zscaler-sdk-go/v3/zscaler"
	zidgroups "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zid/services/groups"
	zidresourceservers "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zid/services/resource_servers"
	zidusers "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zid/services/users"

	"github.com/dvmrry/zscalerctl/internal/resources"
)

func addZidentityHandlers(m map[resourceKey]resourceHandler, client sdkClient) {
	entries := map[resourceKey]resourceHandler{
		{product: resources.ProductZidentity, name: resourceZidentityGroups}: newListGetHandler(
			resourceZidentityGroups,
			sdkProductList(resources.ProductZidentity, client, func(ctx context.Context, service *zsdk.Service) ([]zidgroups.Groups, error) {
				return zidentityListAll[zidgroups.Groups](ctx, service, zidentityGroupsEndpoint)
			}),
			sdkProductStringGet(resources.ProductZidentity, client, func(ctx context.Context, service *zsdk.Service, id string) (*zidgroups.Groups, error) {
				return zidgroups.Get(ctx, service, id)
			}),
			zidentityGroupSourceRecord,
		),
		{product: resources.ProductZidentity, name: resourceZidentityUsers}: newListGetHandler(
			resourceZidentityUsers,
			sdkProductList(resources.ProductZidentity, client, func(ctx context.Context, service *zsdk.Service) ([]zidusers.Users, error) {
				return zidentityListAll[zidusers.Users](ctx, service, zidentityUsersEndpoint)
			}),
			sdkProductStringGet(resources.ProductZidentity, client, func(ctx context.Context, service *zsdk.Service, id string) (*zidusers.Users, error) {
				return zidusers.GetUser(ctx, service, id)
			}),
			zidentityUserSourceRecord,
		),
		{product: resources.ProductZidentity, name: resourceZidentityResourceServers}: newListGetHandler(
			resourceZidentityResourceServers,
			sdkProductList(resources.ProductZidentity, client, func(ctx context.Context, service *zsdk.Service) ([]zidresourceservers.ResourceServers, error) {
				return zidentityListAll[zidresourceservers.ResourceServers](ctx, service, zidentityResourceServersEndpoint)
			}),
			sdkProductStringGet(resources.ProductZidentity, client, func(ctx context.Context, service *zsdk.Service, id string) (*zidresourceservers.ResourceServers, error) {
				return zidresourceservers.Get(ctx, service, id)
			}),
			zidentityResourceServerSourceRecord,
		),
	}
	for k, v := range entries {
		m[k] = v
	}
}
