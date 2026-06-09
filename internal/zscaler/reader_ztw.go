package zscaler

import (
	"context"
	"fmt"

	zsdk "github.com/zscaler/zscaler-sdk-go/v3/zscaler"
	ztwactivation "github.com/zscaler/zscaler-sdk-go/v3/zscaler/ztw/services/activation"
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
		{product: resources.ProductZTW, name: resourceZTWActivationStat}: newSingletonHandler(
			resourceZTWActivationStat,
			sdkProductShow(resources.ProductZTW, client, ztwactivation.GetActivationStatus),
			structSourceRecord[ztwactivation.ECAdminActivation],
		),
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

func ztwWorkloadGroupSourceRecord(group ztwworkloadgroups.WorkloadGroup) resources.SourceRecord {
	fields := map[string]any{
		"id":               group.ID,
		"name":             group.Name,
		"description":      group.Description,
		"expression":       group.Expression,
		"lastModifiedTime": group.LastModifiedTime,
	}
	addZTWIDNameExtensionsPtr(fields, "lastModifiedBy", group.LastModifiedBy)
	if len(group.WorkloadTagExpression.ExpressionContainers) > 0 {
		fields["expressionJson"] = ztwWorkloadTagExpressionSource(group.WorkloadTagExpression)
	}
	return resources.NewSourceRecord(fields)
}

func ztwPublicCloudAccountSourceRecord(account ztwpubliccloudaccount.PublicCloudAccountDetails) resources.SourceRecord {
	return resources.NewSourceRecord(map[string]any{
		"id":         account.ID,
		"accountId":  account.AccountID,
		"platformId": account.PlatformID,
	})
}

func ztwDNSGatewaySourceRecord(gateway ztwdnsgateway.DNSGateway) resources.SourceRecord {
	fields := map[string]any{
		"id":                           gateway.ID,
		"name":                         gateway.Name,
		"dnsGatewayType":               gateway.DNSGatewayType,
		"ecDnsGatewayOptionsPrimary":   gateway.ECDnsGatewayOptionsPrimary,
		"ecDnsGatewayOptionsSecondary": gateway.ECDnsGatewayOptionsSecondary,
		"failureBehavior":              gateway.FailureBehavior,
		"primaryIp":                    gateway.PrimaryIP,
		"secondaryIp":                  gateway.SecondaryIP,
		"lastModifiedTime":             gateway.LastModifiedTime,
	}
	addZTWCommonIDNameExternalIDPtr(fields, "lastModifiedBy", gateway.LastModifiedBy)
	return resources.NewSourceRecord(fields)
}

func ztwForwardingGatewaySourceRecord(gateway ztwziaforwardinggateway.ECGateway) resources.SourceRecord {
	fields := map[string]any{
		"id":                           gateway.ID,
		"name":                         gateway.Name,
		"description":                  gateway.Description,
		"failClosed":                   gateway.FailClosed,
		"manualPrimary":                gateway.ManualPrimary,
		"manualSecondary":              gateway.ManualSecondary,
		"primaryType":                  gateway.PrimaryType,
		"secondaryType":                gateway.SecondaryType,
		"type":                         gateway.Type,
		"failureBehavior":              gateway.FailureBehavior,
		"dnsGatewayType":               gateway.DNSGatewayType,
		"primaryIp":                    gateway.PrimaryIP,
		"secondaryIp":                  gateway.SecondaryIP,
		"ecDnsGatewayOptionsPrimary":   gateway.ECDNSGatewayOptionsPrimary,
		"ecDnsGatewayOptionsSecondary": gateway.ECDNSGatewayOptionsSecondary,
		"lastModifiedTime":             gateway.LastModifiedTime,
	}
	addZTWCommonIDNameExternalIDPtr(fields, "subcloudPrimary", gateway.SubCloudPrimary)
	addZTWCommonIDNameExternalIDPtr(fields, "subcloudSecondary", gateway.SubCloudSecondary)
	addZTWIDNameExtensionsPtr(fields, "lastModifiedBy", gateway.LastModifiedBy)
	return resources.NewSourceRecord(fields)
}

func ztwECGroupSourceRecord(group ztwecgroup.EcGroup) resources.SourceRecord {
	fields := map[string]any{
		"id":                    group.ID,
		"name":                  group.Name,
		"desc":                  group.Description,
		"deployType":            group.DeployType,
		"platform":              group.Platform,
		"awsAvailabilityZone":   group.AWSAvailabilityZone,
		"azureAvailabilityZone": group.AzureAvailabilityZone,
		"maxEcCount":            group.MaxEcCount,
		"tunnelMode":            group.TunnelMode,
	}
	addStringSlice(fields, "status", group.Status)
	addZTWCommonIDNameExternalIDPtr(fields, "location", group.Location)
	addZTWCommonIDNameExternalIDPtr(fields, "provTemplate", group.ProvTemplate)
	if len(group.ECVMs) > 0 {
		fields["ecVMs"] = len(group.ECVMs)
	}
	return resources.NewSourceRecord(fields)
}

func ztwIPSourceGroupSourceRecord(group ztwipsourcegroups.IPSourceGroups) resources.SourceRecord {
	fields := map[string]any{
		"id":             group.ID,
		"name":           group.Name,
		"description":    group.Description,
		"creatorContext": group.CreatorContext,
		"isNonEditable":  group.IsNonEditable,
	}
	addStringSlice(fields, "ipAddresses", group.IPAddresses)
	return resources.NewSourceRecord(fields)
}

func ztwIPDestinationGroupSourceRecord(group ztwipdestinationgroups.IPDestinationGroups) resources.SourceRecord {
	fields := map[string]any{
		"id":            group.ID,
		"name":          group.Name,
		"description":   group.Description,
		"type":          group.Type,
		"isNonEditable": group.IsNonEditable,
	}
	addStringSlice(fields, "addresses", group.Addresses)
	addStringSlice(fields, "ipCategories", group.IPCategories)
	addStringSlice(fields, "countries", group.Countries)
	return resources.NewSourceRecord(fields)
}

func ztwIPGroupSourceRecord(group ztwipgroups.IPGroups) resources.SourceRecord {
	fields := map[string]any{
		"id":             group.ID,
		"name":           group.Name,
		"description":    group.Description,
		"creatorContext": group.CreatorContext,
		"isNonEditable":  group.IsNonEditable,
		"extranetIpPool": group.ExtranetIPPool,
		"isPredefined":   group.IsPredefined,
	}
	addStringSlice(fields, "ipAddresses", group.IPAddresses)
	return resources.NewSourceRecord(fields)
}

func ztwNetworkServiceSourceRecord(service ztwnetworkservices.NetworkServices) resources.SourceRecord {
	fields := map[string]any{
		"id":             service.ID,
		"name":           service.Name,
		"description":    service.Description,
		"tag":            service.Tag,
		"type":           service.Type,
		"isNameL10nTag":  service.IsNameL10nTag,
		"creatorContext": service.CreatorContext,
	}
	addZTWNetworkPorts(fields, "srcTcpPorts", service.SrcTCPPorts)
	addZTWNetworkPorts(fields, "destTcpPorts", service.DestTCPPorts)
	addZTWNetworkPorts(fields, "srcUdpPorts", service.SrcUDPPorts)
	addZTWNetworkPorts(fields, "destUdpPorts", service.DestUDPPorts)
	return resources.NewSourceRecord(fields)
}

func ztwNetworkServiceGroupSourceRecord(group ztwnetworkservicegroups.NetworkServiceGroups) resources.SourceRecord {
	fields := map[string]any{
		"id":             group.ID,
		"name":           group.Name,
		"description":    group.Description,
		"creatorContext": group.CreatorContext,
	}
	if len(group.Services) > 0 {
		fields["services"] = ztwNetworkServiceRefsSource(group.Services)
	}
	return resources.NewSourceRecord(fields)
}

func ztwAdminUserSourceRecord(user ztwadminusers.AdminUsers) resources.SourceRecord {
	fields := map[string]any{
		"id":                          user.ID,
		"loginName":                   user.LoginName,
		"userName":                    user.UserName,
		"email":                       user.Email,
		"comments":                    user.Comments,
		"disabled":                    user.Disabled,
		"password":                    user.Password,
		"pwdLastModifiedTime":         user.PasswordLastModifiedTime,
		"isNonEditable":               user.IsNonEditable,
		"isPasswordLoginAllowed":      user.IsPasswordLoginAllowed,
		"isPasswordExpired":           user.IsPasswordExpired,
		"isAuditor":                   user.IsAuditor,
		"isSecurityReportCommEnabled": user.IsSecurityReportCommEnabled,
		"isServiceUpdateCommEnabled":  user.IsServiceUpdateCommEnabled,
		"isProductUpdateCommEnabled":  user.IsProductUpdateCommEnabled,
		"isExecMobileAppEnabled":      user.IsExecMobileAppEnabled,
		"adminScopeType":              user.AdminScopeType,
	}
	addZTWIDNameExtensionsSlice(fields, "adminScopescopeGroupMemberEntities", user.AdminScopeGroupMemberEntities)
	addZTWIDNameExtensionsSlice(fields, "adminScopeScopeEntities", user.AdminScopeEntities)
	if user.Role != nil {
		fields["role"] = ztwAdminUserRoleSource(user.Role)
	}
	if len(user.ExecMobileAppTokens) > 0 {
		fields["execMobileAppTokens"] = ztwExecMobileAppTokensSource(user.ExecMobileAppTokens)
	}
	return resources.NewSourceRecord(fields)
}

func ztwAdminRoleSourceRecord(role ztwadminroles.AdminRoles) resources.SourceRecord {
	fields := map[string]any{
		"id":                 role.ID,
		"rank":               role.Rank,
		"name":               role.Name,
		"policyAccess":       role.PolicyAccess,
		"alertingAccess":     role.AlertingAccess,
		"dashboardAccess":    role.DashboardAccess,
		"reportAccess":       role.ReportAccess,
		"analysisAccess":     role.AnalysisAccess,
		"usernameAccess":     role.UsernameAccess,
		"adminAcctAccess":    role.AdminAcctAccess,
		"deviceInfoAccess":   role.DeviceInfoAccess,
		"isAuditor":          role.IsAuditor,
		"isNonEditable":      role.IsNonEditable,
		"logsLimit":          role.LogsLimit,
		"roleType":           role.RoleType,
		"featurePermissions": role.FeaturePermissions,
	}
	addStringSlice(fields, "permissions", role.Permissions)
	return resources.NewSourceRecord(fields)
}

func ztwLocationSourceRecord(location ztwlocation.Locations) resources.SourceRecord {
	fields := map[string]any{
		"id":                                  location.ID,
		"name":                                location.Name,
		"parentId":                            location.ParentID,
		"enforceBandwidthControl":             location.EnforceBandwidthControl,
		"upBandwidth":                         location.UpBandwidth,
		"dnBandwidth":                         location.DnBandwidth,
		"overrideUpBandwidth":                 location.OverrideUpBandwidth,
		"overrideDnBandwidth":                 location.OverrideDnBandwidth,
		"sharedUpBandwidth":                   location.SharedUpBandwidth,
		"sharedDownBandwidth":                 location.SharedDownBandwidth,
		"unusedUpBandwidth":                   location.UnusedUpBandwidth,
		"country":                             location.Country,
		"state":                               location.State,
		"language":                            location.Language,
		"tz":                                  location.TZ,
		"authRequired":                        location.AuthRequired,
		"sslScanEnabled":                      location.SSLScanEnabled,
		"zappSSLScanEnabled":                  location.ZappSSLScanEnabled,
		"xffForwardEnabled":                   location.XFFForwardEnabled,
		"otherSubLocation":                    location.OtherSubLocation,
		"other6SubLocation":                   location.Other6SubLocation,
		"ecLocation":                          location.ECLocation,
		"surrogateIP":                         location.SurrogateIP,
		"idleTimeInMinutes":                   location.IdleTimeInMinutes,
		"displayTimeUnit":                     location.DisplayTimeUnit,
		"surrogateIPEnforcedForKnownBrowsers": location.SurrogateIPEnforcedForKnownBrowsers,
		"surrogateRefreshTimeInMinutes":       location.SurrogateRefreshTimeInMinutes,
		"surrogateRefreshTimeUnit":            location.SurrogateRefreshTimeUnit,
		"ofwEnabled":                          location.OFWEnabled,
		"ipsControl":                          location.IPSControl,
		"aupEnabled":                          location.AUPEnabled,
		"cautionEnabled":                      location.CautionEnabled,
		"aupBlockInternetUntilAccepted":       location.AUPBlockInternetUntilAccepted,
		"aupForceSslInspection":               location.AUPForceSSLInspection,
		"aupTimeoutInDays":                    location.AUPTimeoutInDays,
		"profile":                             location.Profile,
		"description":                         location.Description,
		"ipv6Enabled":                         location.IPv6Enabled,
		"ipv6Dns64Prefix":                     location.IPv6Dns64Prefix,
		"kerberosAuth":                        location.KerberosAuth,
		"digestAuthEnabled":                   location.DigestAuthEnabled,
		"childCount":                          location.ChildCount,
		"matchInChild":                        location.MatchInChild,
		"excludeFromDynamicGroups":            location.ExcludeFromDynamicGroups,
		"excludeFromManualGroups":             location.ExcludeFromManualGroups,
	}
	addStringSlice(fields, "ipAddresses", location.IPAddresses)
	if len(location.Ports) > 0 {
		fields["ports"] = location.Ports
	}
	if len(location.VPNCredentials) > 0 {
		fields["vpnCredentials"] = location.VPNCredentials
	}
	addZTWCommonIDNameExternalIDSlice(fields, "virtualZens", location.VirtualZens)
	addZTWCommonIDNameExternalIDSlice(fields, "virtualZenClusters", location.VirtualZenClusters)
	addZTWCommonIDNameExternalIDSlice(fields, "staticLocationGroups", location.StaticLocationGroups)
	addZTWCommonIDNameExternalIDSlice(fields, "dynamiclocationGroups", location.DynamiclocationGroups)
	addZTWCommonIDNamePtr(fields, "publicCloudAccountId", location.PublicCloudAccountID)
	if location.VPCInfo.CloudProvider != "" || location.VPCInfo.CloudMeta.ID != 0 || location.VPCInfo.CloudMeta.Name != "" {
		fields["vpcInfo"] = location.VPCInfo
	}
	return resources.NewSourceRecord(fields)
}

func ztwLocationTemplateSourceRecord(template ztwlocationtemplate.LocationTemplate) resources.SourceRecord {
	fields := map[string]any{
		"id":          template.ID,
		"name":        template.Name,
		"desc":        template.Description,
		"editable":    template.Editable,
		"lastModTime": template.LastModTime,
	}
	if template.LocationTemplateDetails != nil {
		fields["template"] = ztwLocationTemplateDetailsSource(template.LocationTemplateDetails)
	}
	addZTWCommonIDNameExternalIDPtr(fields, "lastModUid", template.LastModUid)
	return resources.NewSourceRecord(fields)
}

func ztwLocationTemplateDetailsSource(details *ztwlocationtemplate.LocationTemplateDetails) map[string]any {
	fields := map[string]any{
		"templatePrefix":                      details.TemplatePrefix,
		"xffForwardEnabled":                   details.XFFForwardEnabled,
		"authRequired":                        details.AuthRequired,
		"cautionEnabled":                      details.CautionEnabled,
		"aupEnabled":                          details.AupEnabled,
		"aupTimeoutInDays":                    details.AupTimeoutInDays,
		"ofwEnabled":                          details.OFWEnabled,
		"ipsControl":                          details.IPSControl,
		"enforceBandwidthControl":             details.EnforceBandwidthControl,
		"upBandwidth":                         details.UpBandwidth,
		"dnBandwidth":                         details.DnBandwidth,
		"displayTimeUnit":                     details.DisplayTimeUnit,
		"idleTimeInMinutes":                   details.IdleTimeInMinutes,
		"surrogateIPEnforcedForKnownBrowsers": details.SurrogateIPEnforcedForKnownBrowsers,
		"surrogateRefreshTimeUnit":            details.SurrogateRefreshTimeUnit,
		"surrogateRefreshTimeInMinutes":       details.SurrogateRefreshTimeInMinutes,
		"surrogateIP":                         details.SurrogateIP,
		"editable":                            details.Editable,
	}
	addZTWCommonIDNameExternalIDPtr(fields, "lastModUid", details.LastModUid)
	return fields
}

func ztwAccountGroupSourceRecord(group ztwaccountgroups.AccountGroups) resources.SourceRecord {
	fields := map[string]any{
		"id":          group.ID,
		"name":        group.Name,
		"description": group.Description,
		"cloudType":   group.CloudType,
	}
	addZTWIDNameExtensionsSlice(fields, "publicCloudAccounts", group.PublicCloudAccounts)
	addZTWIDNameExtensionsSlice(fields, "cloudConnectorGroups", group.CloudConnectorGroups)
	return resources.NewSourceRecord(fields)
}

func ztwPublicCloudInfoSourceRecord(info ztwpubliccloudinfo.PublicCloudInfo) resources.SourceRecord {
	fields := map[string]any{
		"id":           info.ID,
		"name":         info.Name,
		"cloudType":    info.CloudType,
		"externalId":   info.ExternalID,
		"lastModTime":  info.LastModTime,
		"lastSyncTime": info.LastSyncTime,
	}
	addZTWIDNameExtensionsSlice(fields, "accountGroups", info.AccountGroups)
	addZTWCommonIDNameExternalIDPtr(fields, "lastModUser", info.LastModUser)
	if len(info.RegionStatus) > 0 {
		fields["regionStatus"] = ztwRegionStatusSliceSource(info.RegionStatus)
	}
	if len(info.SupportedRegions) > 0 {
		fields["supportedRegions"] = ztwSupportedRegionsSliceSource(info.SupportedRegions)
	}
	if info.AccountDetails != nil {
		fields["accountDetails"] = info.AccountDetails
	}
	return resources.NewSourceRecord(fields)
}

func ztwZPAApplicationSegmentSourceRecord(segment ztwzparesources.ZPAApplicationSegment) resources.SourceRecord {
	return resources.NewSourceRecord(map[string]any{
		"id":          segment.ID,
		"name":        segment.Name,
		"description": segment.Description,
		"zpaId":       segment.ZpaID,
		"deleted":     segment.Deleted,
	})
}

func ztwForwardingRuleSourceRecord(rule ztwforwardingrules.ForwardingRules) resources.SourceRecord {
	fields := map[string]any{
		"id":                     rule.ID,
		"name":                   rule.Name,
		"accessControl":          rule.AccessControl,
		"description":            rule.Description,
		"type":                   rule.Type,
		"order":                  rule.Order,
		"rank":                   rule.Rank,
		"forwardMethod":          rule.ForwardMethod,
		"defaultRule":            rule.DefaultRule,
		"wanSelection":           rule.WanSelection,
		"state":                  rule.State,
		"blockResponseCode":      rule.BlockResponseCode,
		"lastModifiedTime":       rule.LastModifiedTime,
		"sourceIpGroupExclusion": rule.SourceIpGroupExclusion,
		"zpaBrokerRule":          rule.ZPABrokerRule,
	}
	addStringSlice(fields, "srcIps", rule.SrcIps)
	addStringSlice(fields, "destAddresses", rule.DestAddresses)
	addStringSlice(fields, "destIpCategories", rule.DestIpCategories)
	addStringSlice(fields, "resCategories", rule.ResCategories)
	addStringSlice(fields, "destCountries", rule.DestCountries)
	addStringSlice(fields, "sourceCountries", rule.SourceCountries)
	addStringSlice(fields, "nwApplications", rule.NwApplications)
	addZTWIDNameExtensionsSlice(fields, "locations", rule.Locations)
	addZTWIDNameExtensionsSlice(fields, "locationGroups", rule.LocationsGroups)
	addZTWIDNameExtensionsSlice(fields, "ecGroups", rule.ECGroups)
	addZTWIDNameExtensionsSlice(fields, "departments", rule.Departments)
	addZTWIDNameExtensionsSlice(fields, "groups", rule.Groups)
	addZTWIDNameExtensionsSlice(fields, "users", rule.Users)
	addZTWIDNameExtensionsPtr(fields, "lastModifiedBy", rule.LastModifiedBy)
	addZTWIDNameExtensionsSlice(fields, "srcIpGroups", rule.SrcIpGroups)
	addZTWIDNameExtensionsSlice(fields, "srcIpv6Groups", rule.SrcIpv6Groups)
	addZTWIDNameExtensionsSlice(fields, "destIpGroups", rule.DestIpGroups)
	addZTWIDNameExtensionsSlice(fields, "destIpv6Groups", rule.DestIpv6Groups)
	addZTWIDNameExtensionsSlice(fields, "nwServices", rule.NwServices)
	addZTWIDNameExtensionsSlice(fields, "nwServiceGroups", rule.NwServiceGroups)
	addZTWIDNameExtensionsSlice(fields, "labels", rule.Labels)
	addZTWIDNameExtensionsSlice(fields, "nwApplicationGroups", rule.NwApplicationGroups)
	addZTWIDNameExtensionsSlice(fields, "appServiceGroups", rule.AppServiceGroups)
	addZTWIDNameExtensionsSlice(fields, "srcWorkloadGroups", rule.SrcWorkloadGroups)
	addZTWCommonIDNamePtr(fields, "proxyGateway", rule.ProxyGateway)
	addZTWZPAApplicationSegments(fields, "zpaApplicationSegments", rule.ZPAApplicationSegments)
	addZTWZPAApplicationSegmentGroups(fields, "zpaApplicationSegmentGroups", rule.ZPAApplicationSegmentGroups)
	return resources.NewSourceRecord(fields)
}

func ztwTrafficDNSRuleSourceRecord(rule ztwtrafficdnsrules.ECDNSRules) resources.SourceRecord {
	fields := map[string]any{
		"id":               rule.ID,
		"name":             rule.Name,
		"description":      rule.Description,
		"type":             rule.Type,
		"action":           rule.Action,
		"order":            rule.Order,
		"rank":             rule.Rank,
		"state":            rule.State,
		"predefined":       rule.Predefined,
		"defaultRule":      rule.DefaultRule,
		"lastModifiedTime": rule.LastModifiedTime,
	}
	addStringSlice(fields, "srcIps", rule.SrcIps)
	addStringSlice(fields, "destAddresses", rule.DestAddresses)
	addZTWIDNameExtensionsSlice(fields, "locations", rule.Locations)
	addZTWIDNameExtensionsSlice(fields, "locationGroups", rule.LocationsGroups)
	addZTWIDNameExtensionsSlice(fields, "ecGroups", rule.ECGroups)
	addZTWIDNameExtensionsSlice(fields, "srcIpGroups", rule.SrcIpGroups)
	addZTWIDNameExtensionsSlice(fields, "destIpGroups", rule.DestIpGroups)
	addZTWCommonIDNamePtr(fields, "dnsGateway", rule.DNSGateway)
	addZTWCommonIDNamePtr(fields, "zpaIpGroup", rule.ZPAIPGroup)
	addZTWIDNameExtensionsPtr(fields, "lastModifiedBy", rule.LastModifiedBy)
	return resources.NewSourceRecord(fields)
}

func ztwTrafficLogRuleSourceRecord(rule ztwtrafficlogrules.ECTrafficLogRules) resources.SourceRecord {
	fields := map[string]any{
		"id":               rule.ID,
		"name":             rule.Name,
		"description":      rule.Description,
		"order":            rule.Order,
		"rank":             rule.Rank,
		"state":            rule.State,
		"type":             rule.Type,
		"forwardMethod":    rule.ForwardMethod,
		"defaultRule":      rule.DefaultRule,
		"lastModifiedTime": rule.LastModifiedTime,
	}
	addZTWIDNameExtensionsSlice(fields, "locations", rule.Locations)
	addZTWCommonIDNamePtr(fields, "proxyGateway", rule.ProxyGateway)
	addZTWIDNameExtensionsPtr(fields, "lastModifiedBy", rule.LastModifiedBy)
	addZTWIDNameExtensionsSlice(fields, "ecGroups", rule.ECGroups)
	return resources.NewSourceRecord(fields)
}
