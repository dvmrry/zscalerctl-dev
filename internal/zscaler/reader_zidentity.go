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
		addHandler(m, k, v)
	}
}

func zidentityGroupSourceRecord(group zidgroups.Groups) resources.SourceRecord {
	fields := map[string]any{
		"id":                        group.ID,
		"name":                      group.Name,
		"description":               group.Description,
		"source":                    group.Source,
		"isDynamicGroup":            group.IsDynamicGroup,
		"dynamicGroup":              group.DynamicGroup,
		"adminEntitlementEnabled":   group.AdminEntitlementEnabled,
		"serviceEntitlementEnabled": group.ServiceEntitlementEnabled,
	}
	if group.IDP != nil {
		fields["idp"] = zidIDNameDisplayNameSource(group.IDP)
	}
	return resources.NewSourceRecord(fields)
}

func zidentityUserSourceRecord(user zidusers.Users) resources.SourceRecord {
	fields := map[string]any{
		"id":             user.ID,
		"source":         user.Source,
		"loginName":      user.LoginName,
		"displayName":    user.DisplayName,
		"firstName":      user.FirstName,
		"lastName":       user.LastName,
		"primaryEmail":   user.PrimaryEmail,
		"secondaryEmail": user.SecondaryEmail,
		"status":         user.Status,
	}
	if user.Department != nil {
		fields["department"] = zidIDNameDisplayNameSource(user.Department)
	}
	if user.IDP != nil {
		fields["idp"] = zidIDNameDisplayNameSource(user.IDP)
	}
	if len(user.CustomAttrsInfo) > 0 {
		fields["customAttrsInfo"] = copyStringAnyMap(user.CustomAttrsInfo)
	}
	return resources.NewSourceRecord(fields)
}

func zidentityResourceServerSourceRecord(server zidresourceservers.ResourceServers) resources.SourceRecord {
	fields := map[string]any{
		"id":          server.ID,
		"name":        server.Name,
		"displayName": server.DisplayName,
		"description": server.Description,
		"primaryAud":  server.PrimaryAud,
		"defaultApi":  server.DefaultApi,
	}
	if len(server.ServiceScopes) > 0 {
		fields["serviceScopes"] = zidentityServiceScopesSource(server.ServiceScopes)
	}
	return resources.NewSourceRecord(fields)
}
