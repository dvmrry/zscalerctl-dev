package zscaler

import (
	"context"
	"fmt"

	zsdk "github.com/zscaler/zscaler-sdk-go/v3/zscaler"
	ztwadminroles "github.com/zscaler/zscaler-sdk-go/v3/zscaler/ztw/services/adminuserrolemgmt/adminroles"
	ztwadminusers "github.com/zscaler/zscaler-sdk-go/v3/zscaler/ztw/services/adminuserrolemgmt/adminusers"
	ztwdnsgateway "github.com/zscaler/zscaler-sdk-go/v3/zscaler/ztw/services/dns_gateway"
	ztwecgroup "github.com/zscaler/zscaler-sdk-go/v3/zscaler/ztw/services/ecgroup"
	ztwziaforwardinggateway "github.com/zscaler/zscaler-sdk-go/v3/zscaler/ztw/services/forwarding_gateways/zia_forwarding_gateway"
	ztwlocation "github.com/zscaler/zscaler-sdk-go/v3/zscaler/ztw/services/locationmanagement/location"
	ztwlocationtemplate "github.com/zscaler/zscaler-sdk-go/v3/zscaler/ztw/services/locationmanagement/locationtemplate"
	ztwaccountgroups "github.com/zscaler/zscaler-sdk-go/v3/zscaler/ztw/services/partner_integrations/account_groups"
	ztwpubliccloudinfo "github.com/zscaler/zscaler-sdk-go/v3/zscaler/ztw/services/partner_integrations/public_cloud_info"
	ztwforwardingrules "github.com/zscaler/zscaler-sdk-go/v3/zscaler/ztw/services/policy_management/forwarding_rules"
	ztwtrafficdnsrules "github.com/zscaler/zscaler-sdk-go/v3/zscaler/ztw/services/policy_management/traffic_dns_rules"
	ztwtrafficlogrules "github.com/zscaler/zscaler-sdk-go/v3/zscaler/ztw/services/policy_management/traffic_log_rules"
	ztwipdestinationgroups "github.com/zscaler/zscaler-sdk-go/v3/zscaler/ztw/services/policyresources/ipdestinationgroups"
	ztwipgroups "github.com/zscaler/zscaler-sdk-go/v3/zscaler/ztw/services/policyresources/ipgroups"
	ztwipsourcegroups "github.com/zscaler/zscaler-sdk-go/v3/zscaler/ztw/services/policyresources/ipsourcegroups"
	ztwnetworkservicegroups "github.com/zscaler/zscaler-sdk-go/v3/zscaler/ztw/services/policyresources/networkservicegroups"
	ztwnetworkservices "github.com/zscaler/zscaler-sdk-go/v3/zscaler/ztw/services/policyresources/networkservices"
	ztwzparesources "github.com/zscaler/zscaler-sdk-go/v3/zscaler/ztw/services/policyresources/zparesources"
	ztwpubliccloudaccount "github.com/zscaler/zscaler-sdk-go/v3/zscaler/ztw/services/provisioning/public_cloud_account"
	ztwworkloadgroups "github.com/zscaler/zscaler-sdk-go/v3/zscaler/ztw/services/workload_groups"

	"github.com/dvmrry/zscalerctl/internal/resources"
)

func addZTWHandlers(m map[resourceKey]resourceHandler, client sdkClient) {
	entries := map[resourceKey]resourceHandler{
		{product: resources.ProductZTW, name: resourceWorkloadGroups}: newListGetHandler(
			resourceWorkloadGroups,
			sdkProductList(resources.ProductZTW, client, func(ctx context.Context, service *zsdk.Service) ([]ztwworkloadgroups.WorkloadGroup, error) {
				return ztwworkloadgroups.GetAll(ctx, service)
			}),
			sdkProductGet(resources.ProductZTW, client, func(ctx context.Context, service *zsdk.Service, id int) (*ztwworkloadgroups.WorkloadGroup, error) {
				return ztwworkloadgroups.Get(ctx, service, id)
			}),
			ztwWorkloadGroupSourceRecord,
		),
		{product: resources.ProductZTW, name: resourcePublicCloudAccts}: newListGetHandler(
			resourcePublicCloudAccts,
			sdkProductList(resources.ProductZTW, client, func(ctx context.Context, service *zsdk.Service) ([]ztwpubliccloudaccount.PublicCloudAccountDetails, error) {
				return ztwpubliccloudaccount.GetAll(ctx, service)
			}),
			sdkProductGet(resources.ProductZTW, client, func(ctx context.Context, service *zsdk.Service, id int) (*ztwpubliccloudaccount.PublicCloudAccountDetails, error) {
				return ztwpubliccloudaccount.GetAccountID(ctx, service, id)
			}),
			ztwPublicCloudAccountSourceRecord,
		),
		{product: resources.ProductZTW, name: resourceDNSGateways}: newListGetHandler(
			resourceDNSGateways,
			sdkProductList(resources.ProductZTW, client, func(ctx context.Context, service *zsdk.Service) ([]ztwdnsgateway.DNSGateway, error) {
				return ztwdnsgateway.GetAll(ctx, service)
			}),
			sdkProductGet(resources.ProductZTW, client, func(ctx context.Context, service *zsdk.Service, id int) (*ztwdnsgateway.DNSGateway, error) {
				return ztwdnsgateway.Get(ctx, service, id)
			}),
			ztwDNSGatewaySourceRecord,
		),
		{product: resources.ProductZTW, name: resourceForwardingGWs}: newListGetHandler(
			resourceForwardingGWs,
			sdkProductList(resources.ProductZTW, client, func(ctx context.Context, service *zsdk.Service) ([]ztwziaforwardinggateway.ECGateway, error) {
				return ztwziaforwardinggateway.GetAll(ctx, service)
			}),
			sdkProductGet(resources.ProductZTW, client, func(ctx context.Context, service *zsdk.Service, id int) (*ztwziaforwardinggateway.ECGateway, error) {
				gateway, _, err := ztwziaforwardinggateway.Get(ctx, service, id)
				return gateway, err
			}),
			ztwForwardingGatewaySourceRecord,
		),
		{product: resources.ProductZTW, name: resourceECGroups}: newListGetHandler(
			resourceECGroups,
			sdkProductList(resources.ProductZTW, client, func(ctx context.Context, service *zsdk.Service) ([]ztwecgroup.EcGroup, error) {
				return ztwecgroup.GetAll(ctx, service)
			}),
			sdkProductGet(resources.ProductZTW, client, func(ctx context.Context, service *zsdk.Service, id int) (*ztwecgroup.EcGroup, error) {
				return ztwecgroup.Get(ctx, service, id)
			}),
			ztwECGroupSourceRecord,
		),
		{product: resources.ProductZTW, name: resourceIPSourceGroups}: newListGetHandler(
			resourceIPSourceGroups,
			sdkProductList(resources.ProductZTW, client, func(ctx context.Context, service *zsdk.Service) ([]ztwipsourcegroups.IPSourceGroups, error) {
				return ztwipsourcegroups.GetAll(ctx, service)
			}),
			sdkProductGet(resources.ProductZTW, client, func(ctx context.Context, service *zsdk.Service, id int) (*ztwipsourcegroups.IPSourceGroups, error) {
				return ztwipsourcegroups.Get(ctx, service, id)
			}),
			ztwIPSourceGroupSourceRecord,
		),
		{product: resources.ProductZTW, name: resourceIPDestGroups}: newListGetHandler(
			resourceIPDestGroups,
			sdkProductList(resources.ProductZTW, client, func(ctx context.Context, service *zsdk.Service) ([]ztwipdestinationgroups.IPDestinationGroups, error) {
				return ztwipdestinationgroups.GetAll(ctx, service)
			}),
			sdkProductGet(resources.ProductZTW, client, func(ctx context.Context, service *zsdk.Service, id int) (*ztwipdestinationgroups.IPDestinationGroups, error) {
				return ztwipdestinationgroups.Get(ctx, service, id)
			}),
			ztwIPDestinationGroupSourceRecord,
		),
		{product: resources.ProductZTW, name: resourceIPGroups}: newListGetHandler(
			resourceIPGroups,
			sdkProductList(resources.ProductZTW, client, func(ctx context.Context, service *zsdk.Service) ([]ztwipgroups.IPGroups, error) {
				return ztwipgroups.GetAll(ctx, service)
			}),
			sdkProductGet(resources.ProductZTW, client, func(ctx context.Context, service *zsdk.Service, id int) (*ztwipgroups.IPGroups, error) {
				return ztwipgroups.Get(ctx, service, id)
			}),
			ztwIPGroupSourceRecord,
		),
		{product: resources.ProductZTW, name: resourceNetworkServices}: newListGetHandler(
			resourceNetworkServices,
			sdkProductList(resources.ProductZTW, client, func(ctx context.Context, service *zsdk.Service) ([]ztwnetworkservices.NetworkServices, error) {
				return ztwnetworkservices.GetAllNetworkServices(ctx, service)
			}),
			sdkProductGet(resources.ProductZTW, client, func(ctx context.Context, service *zsdk.Service, id int) (*ztwnetworkservices.NetworkServices, error) {
				return ztwnetworkservices.Get(ctx, service, id)
			}),
			ztwNetworkServiceSourceRecord,
		),
		{product: resources.ProductZTW, name: resourceNetworkSvcGroups}: newListGetHandler(
			resourceNetworkSvcGroups,
			sdkProductList(resources.ProductZTW, client, func(ctx context.Context, service *zsdk.Service) ([]ztwnetworkservicegroups.NetworkServiceGroups, error) {
				return ztwnetworkservicegroups.GetAllNetworkServiceGroups(ctx, service)
			}),
			sdkProductGet(resources.ProductZTW, client, func(ctx context.Context, service *zsdk.Service, id int) (*ztwnetworkservicegroups.NetworkServiceGroups, error) {
				return ztwnetworkservicegroups.GetNetworkServiceGroups(ctx, service, id)
			}),
			ztwNetworkServiceGroupSourceRecord,
		),
		{product: resources.ProductZTW, name: resourceAdminUsers}: newListGetHandler(
			resourceAdminUsers,
			sdkProductList(resources.ProductZTW, client, func(ctx context.Context, service *zsdk.Service) ([]ztwadminusers.AdminUsers, error) {
				return ztwadminusers.GetAllAdminUsers(ctx, service)
			}),
			sdkProductGet(resources.ProductZTW, client, func(ctx context.Context, service *zsdk.Service, id int) (*ztwadminusers.AdminUsers, error) {
				return ztwadminusers.GetAdminUsers(ctx, service, id)
			}),
			ztwAdminUserSourceRecord,
		),
		{product: resources.ProductZTW, name: resourceAdminRoles}: newListGetHandler(
			resourceAdminRoles,
			sdkProductList(resources.ProductZTW, client, func(ctx context.Context, service *zsdk.Service) ([]ztwadminroles.AdminRoles, error) {
				return ztwadminroles.GetAllAdminRoles(ctx, service)
			}),
			sdkProductGet(resources.ProductZTW, client, func(ctx context.Context, service *zsdk.Service, id int) (*ztwadminroles.AdminRoles, error) {
				return ztwadminroles.Get(ctx, service, id)
			}),
			ztwAdminRoleSourceRecord,
		),
		{product: resources.ProductZTW, name: resourceLocations}: newListGetHandler(
			resourceLocations,
			sdkProductList(resources.ProductZTW, client, func(ctx context.Context, service *zsdk.Service) ([]ztwlocation.Locations, error) {
				return ztwlocation.GetAll(ctx, service)
			}),
			sdkProductGet(resources.ProductZTW, client, func(ctx context.Context, service *zsdk.Service, id int) (*ztwlocation.Locations, error) {
				return ztwlocation.GetLocation(ctx, service, id)
			}),
			ztwLocationSourceRecord,
		),
		{product: resources.ProductZTW, name: resourceLocationTmpls}: newListGetHandler(
			resourceLocationTmpls,
			sdkProductList(resources.ProductZTW, client, func(ctx context.Context, service *zsdk.Service) ([]ztwlocationtemplate.LocationTemplate, error) {
				return ztwlocationtemplate.GetAll(ctx, service)
			}),
			sdkProductGet(resources.ProductZTW, client, func(ctx context.Context, service *zsdk.Service, id int) (*ztwlocationtemplate.LocationTemplate, error) {
				return ztwlocationtemplate.Get(ctx, service, id)
			}),
			ztwLocationTemplateSourceRecord,
		),
		{product: resources.ProductZTW, name: resourceAccountGroups}: newListGetHandler(
			resourceAccountGroups,
			sdkProductList(resources.ProductZTW, client, func(ctx context.Context, service *zsdk.Service) ([]ztwaccountgroups.AccountGroups, error) {
				return ztwaccountgroups.GetAllAccountGroups(ctx, service)
			}),
			sdkProductGet(resources.ProductZTW, client, func(ctx context.Context, service *zsdk.Service, id int) (*ztwaccountgroups.AccountGroups, error) {
				groups, err := ztwaccountgroups.GetAccountGroup(ctx, service, id)
				if err != nil {
					return nil, err
				}
				if len(groups) == 0 {
					return nil, fmt.Errorf("%w: empty ztw account group response", ErrLiveAccessFailed)
				}
				return &groups[0], nil
			}),
			ztwAccountGroupSourceRecord,
		),
		{product: resources.ProductZTW, name: resourcePublicCloudInfo}: newListGetHandler(
			resourcePublicCloudInfo,
			sdkProductList(resources.ProductZTW, client, func(ctx context.Context, service *zsdk.Service) ([]ztwpubliccloudinfo.PublicCloudInfo, error) {
				return ztwpubliccloudinfo.GetAllPublicCloudInfo(ctx, service)
			}),
			sdkProductGet(resources.ProductZTW, client, func(ctx context.Context, service *zsdk.Service, id int) (*ztwpubliccloudinfo.PublicCloudInfo, error) {
				return ztwpubliccloudinfo.GetPublicCloudInfo(ctx, service, id)
			}),
			ztwPublicCloudInfoSourceRecord,
		),
		{product: resources.ProductZTW, name: resourceZTWZPAAppSegs}: newListOnlyHandler(
			resourceZTWZPAAppSegs,
			sdkProductList(resources.ProductZTW, client, func(ctx context.Context, service *zsdk.Service) ([]ztwzparesources.ZPAApplicationSegment, error) {
				return ztwzparesources.GetZPAApplicationSegments(ctx, service)
			}),
			ztwZPAApplicationSegmentSourceRecord,
		),
		{product: resources.ProductZTW, name: resourceForwardingRules}: newListGetHandler(
			resourceForwardingRules,
			sdkProductList(resources.ProductZTW, client, func(ctx context.Context, service *zsdk.Service) ([]ztwforwardingrules.ForwardingRules, error) {
				return ztwforwardingrules.GetAll(ctx, service)
			}),
			sdkProductGet(resources.ProductZTW, client, func(ctx context.Context, service *zsdk.Service, id int) (*ztwforwardingrules.ForwardingRules, error) {
				return ztwforwardingrules.Get(ctx, service, id)
			}),
			ztwForwardingRuleSourceRecord,
		),
		{product: resources.ProductZTW, name: resourceTrafficDNSRules}: newListGetHandler(
			resourceTrafficDNSRules,
			sdkProductList(resources.ProductZTW, client, func(ctx context.Context, service *zsdk.Service) ([]ztwtrafficdnsrules.ECDNSRules, error) {
				return ztwtrafficdnsrules.GetAll(ctx, service)
			}),
			sdkProductGet(resources.ProductZTW, client, func(ctx context.Context, service *zsdk.Service, id int) (*ztwtrafficdnsrules.ECDNSRules, error) {
				return ztwtrafficdnsrules.Get(ctx, service, id)
			}),
			ztwTrafficDNSRuleSourceRecord,
		),
		{product: resources.ProductZTW, name: resourceTrafficLogRules}: newListGetHandler(
			resourceTrafficLogRules,
			sdkProductList(resources.ProductZTW, client, func(ctx context.Context, service *zsdk.Service) ([]ztwtrafficlogrules.ECTrafficLogRules, error) {
				return ztwtrafficlogrules.GetAll(ctx, service)
			}),
			sdkProductGet(resources.ProductZTW, client, func(ctx context.Context, service *zsdk.Service, id int) (*ztwtrafficlogrules.ECTrafficLogRules, error) {
				return ztwtrafficlogrules.Get(ctx, service, id)
			}),
			ztwTrafficLogRuleSourceRecord,
		),
	}
	for k, v := range entries {
		addHandler(m, k, v)
	}
}
