package zscaler

import (
	"context"

	zsdk "github.com/zscaler/zscaler-sdk-go/v3/zscaler"
	zccadminroles "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zcc/services/admin_roles"
	zccappprofiles "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zcc/services/application_profiles"
	zcccompany "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zcc/services/company"
	zcccustomip "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zcc/services/custom_ip_apps"
	zccdevices "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zcc/services/devices"
	zccfailopen "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zcc/services/failopen_policy"
	zccfwdprofile "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zcc/services/forwarding_profile"
	zccpredefip "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zcc/services/predefined_ip_apps"
	zccprocessapps "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zcc/services/process_based_apps"
	zcctrustednet "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zcc/services/trusted_network"
	zccwebappsvc "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zcc/services/web_app_service"

	"github.com/dvmrry/zscalerctl/internal/resources"
)

// zccProbePageSize bounds single-page ZCC reads. ZCC PAPI list endpoints page
// with skip/perPage; the inventory dump reads the first conservative page.
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
		{product: resources.ProductZCC, name: resourceZCCFwdProfiles}: newListOnlyHandler(
			resourceZCCFwdProfiles,
			sdkProductList(resources.ProductZCC, client, func(ctx context.Context, service *zsdk.Service) ([]zccfwdprofile.ForwardingProfile, error) {
				return zccfwdprofile.GetForwardingProfileByCompanyID(ctx, service, "", nil, nil)
			}),
			structSourceRecord[zccfwdprofile.ForwardingProfile],
		),
		{product: resources.ProductZCC, name: resourceZCCTrustedNets}: newListOnlyHandler(
			resourceZCCTrustedNets,
			sdkProductList(resources.ProductZCC, client, func(ctx context.Context, service *zsdk.Service) ([]zcctrustednet.TrustedNetwork, error) {
				resp, _, err := zcctrustednet.GetMultipleTrustedNetworks(ctx, service, "", "", nil, nil)
				if err != nil {
					return nil, err
				}
				return resp.TrustedNetworkContracts, nil
			}),
			structSourceRecord[zcctrustednet.TrustedNetwork],
		),
		{product: resources.ProductZCC, name: resourceZCCWebAppServices}: newListOnlyHandler(
			resourceZCCWebAppServices,
			sdkProductList(resources.ProductZCC, client, func(ctx context.Context, service *zsdk.Service) ([]zccwebappsvc.WebAppService, error) {
				return zccwebappsvc.GetWebAppServices(ctx, service, "", nil, nil)
			}),
			structSourceRecord[zccwebappsvc.WebAppService],
		),
		{product: resources.ProductZCC, name: resourceZCCAppProfiles}: newListOnlyHandler(
			resourceZCCAppProfiles,
			sdkProductList(resources.ProductZCC, client, func(ctx context.Context, service *zsdk.Service) ([]zccappprofiles.ApplicationProfile, error) {
				resp, _, err := zccappprofiles.GetApplicationProfiles(ctx, service, "", "", "", nil, nil)
				if err != nil {
					return nil, err
				}
				return resp.Policies, nil
			}),
			structSourceRecord[zccappprofiles.ApplicationProfile],
		),
		{product: resources.ProductZCC, name: resourceZCCCustomIPApps}: newListOnlyHandler(
			resourceZCCCustomIPApps,
			sdkProductList(resources.ProductZCC, client, func(ctx context.Context, service *zsdk.Service) ([]zcccustomip.CustomIPApp, error) {
				resp, _, err := zcccustomip.GetCustomIPApps(ctx, service, "", nil, nil)
				if err != nil {
					return nil, err
				}
				return resp.CustomAppContracts, nil
			}),
			structSourceRecord[zcccustomip.CustomIPApp],
		),
		{product: resources.ProductZCC, name: resourceZCCPredefIPApps}: newListOnlyHandler(
			resourceZCCPredefIPApps,
			sdkProductList(resources.ProductZCC, client, func(ctx context.Context, service *zsdk.Service) ([]zccpredefip.PredefinedIPApp, error) {
				resp, _, err := zccpredefip.GetPredefinedIPApps(ctx, service, "", nil, nil)
				if err != nil {
					return nil, err
				}
				return resp.AppServiceContracts, nil
			}),
			structSourceRecord[zccpredefip.PredefinedIPApp],
		),
		{product: resources.ProductZCC, name: resourceZCCProcessApps}: newListOnlyHandler(
			resourceZCCProcessApps,
			sdkProductList(resources.ProductZCC, client, func(ctx context.Context, service *zsdk.Service) ([]zccprocessapps.ProcessBasedApp, error) {
				resp, _, err := zccprocessapps.GetProcessBasedApps(ctx, service, "", nil, nil)
				if err != nil {
					return nil, err
				}
				return resp.AppIdentities, nil
			}),
			structSourceRecord[zccprocessapps.ProcessBasedApp],
		),
		{product: resources.ProductZCC, name: resourceZCCDevices}: newListOnlyHandler(
			resourceZCCDevices,
			sdkProductList(resources.ProductZCC, client, func(ctx context.Context, service *zsdk.Service) ([]zccdevices.GetDevices, error) {
				return zccdevices.GetAll(ctx, service, "", "")
			}),
			structSourceRecord[zccdevices.GetDevices],
		),
		{product: resources.ProductZCC, name: resourceZCCAdminRoles}: newListOnlyHandler(
			resourceZCCAdminRoles,
			sdkProductList(resources.ProductZCC, client, func(ctx context.Context, service *zsdk.Service) ([]zccadminroles.AdminRole, error) {
				return zccadminroles.GetAdminRoles(ctx, service)
			}),
			structSourceRecord[zccadminroles.AdminRole],
		),
		{product: resources.ProductZCC, name: resourceZCCCompanyInfo}: newSingletonHandler(
			resourceZCCCompanyInfo,
			sdkProductShow(resources.ProductZCC, client, zcccompany.GetCompanyInfo),
			structSourceRecord[zcccompany.CompanyInfo],
		),
	}
	for k, v := range entries {
		addHandler(m, k, v)
	}
}
