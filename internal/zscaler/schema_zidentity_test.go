package zscaler

import (
	"reflect"

	"github.com/dvmrry/zscalerctl/internal/resources"

	zidcommon "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zid/services/common"
	zidgroups "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zid/services/groups"
	zidresourceservers "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zid/services/resource_servers"
	zidusers "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zid/services/users"
)

func reviewedSDKShapesZidentity() []sdkShapeReview {
	return []sdkShapeReview{
		{
			name:         "zidgroups.Groups",
			resource:     resources.ProductZidentity,
			resourceName: resourceZidentityGroups,
			typ:          reflect.TypeOf(zidgroups.Groups{}),
			catalogFields: []string{
				"id",
				"name",
				"description",
				"source",
				"isDynamicGroup",
				"dynamicGroup",
				"adminEntitlementEnabled",
				"serviceEntitlementEnabled",
				"idp",
			},
		},
		{
			name:         "zidcommon.IDNameDisplayName (groups)",
			resource:     resources.ProductZidentity,
			resourceName: resourceZidentityGroups,
			typ:          reflect.TypeOf(zidcommon.IDNameDisplayName{}),
			catalogFields: []string{
				"id",
				"name",
				"displayName",
			},
		},
		{
			name:         "zidusers.Users",
			resource:     resources.ProductZidentity,
			resourceName: resourceZidentityUsers,
			typ:          reflect.TypeOf(zidusers.Users{}),
			catalogFields: []string{
				"id",
				"source",
				"loginName",
				"displayName",
				"firstName",
				"lastName",
				"primaryEmail",
				"secondaryEmail",
				"status",
				"department",
				"idp",
				"customAttrsInfo",
			},
		},
		{
			name:         "zidcommon.IDNameDisplayName (users)",
			resource:     resources.ProductZidentity,
			resourceName: resourceZidentityUsers,
			typ:          reflect.TypeOf(zidcommon.IDNameDisplayName{}),
			catalogFields: []string{
				"id",
				"name",
				"displayName",
			},
		},
		{
			name:         "zidresourceservers.ResourceServers",
			resource:     resources.ProductZidentity,
			resourceName: resourceZidentityResourceServers,
			typ:          reflect.TypeOf(zidresourceservers.ResourceServers{}),
			catalogFields: []string{
				"id",
				"name",
				"displayName",
				"description",
				"primaryAud",
				"defaultApi",
				"serviceScopes",
			},
		},
		{
			name:         "zidresourceservers.ServiceScopes",
			resource:     resources.ProductZidentity,
			resourceName: resourceZidentityResourceServers,
			typ:          reflect.TypeOf(zidresourceservers.ServiceScopes{}),
			catalogFields: []string{
				"service",
				"scopes",
			},
		},
		{
			name:         "zidresourceservers.Service",
			resource:     resources.ProductZidentity,
			resourceName: resourceZidentityResourceServers,
			typ:          reflect.TypeOf(zidresourceservers.Service{}),
			catalogFields: []string{
				"id",
				"name",
				"displayName",
			},
			ignoredFields: ignoredBecause(
				"resource server service context can expose product or organization identifiers; classify deliberately before rendering",
				"cloudName",
				"orgName",
			),
		},
		{
			name:         "zidresourceservers.Scopes",
			resource:     resources.ProductZidentity,
			resourceName: resourceZidentityResourceServers,
			typ:          reflect.TypeOf(zidresourceservers.Scopes{}),
			catalogFields: []string{
				"id",
				"name",
			},
		},
	}
}
