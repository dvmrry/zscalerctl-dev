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

// zccPageSize is the per-page size for paginated ZCC reads. ZCC PAPI clamps
// page size to MaxPageSize (5000), so 1000 (the SDK's own ReadAllPages default)
// is honored by the server and short-page termination stays valid.
const zccPageSize = 1000

// zccPaginate walks every page of a ZCC list endpoint, mirroring the SDK's
// ReadAllPages contract: advance pages until one returns fewer than a full page.
// Several ZCC by-company list endpoints otherwise return only the first page
// and silently truncate large tenants.
func zccPaginate[T any](ctx context.Context, fetchPage func(ctx context.Context, page, pageSize int) ([]T, error)) ([]T, error) {
	var all []T
	for page := 1; ; page++ {
		items, err := fetchPage(ctx, page, zccPageSize)
		if err != nil {
			return nil, err
		}
		all = append(all, items...)
		if len(items) < zccPageSize {
			break
		}
	}
	return all, nil
}

func addZCCHandlers(m map[resourceKey]resourceHandler, client sdkClient) {
	entries := map[resourceKey]resourceHandler{
		{product: resources.ProductZCC, name: resourceZCCFailOpenPolicy}: newListOnlyHandler(
			resourceZCCFailOpenPolicy,
			sdkProductList(resources.ProductZCC, client, func(ctx context.Context, service *zsdk.Service) ([]zccfailopen.WebFailOpenPolicy, error) {
				// Fail-open policy is a per-company singleton list; one full page covers it.
				return zccfailopen.GetFailOpenPolicy(ctx, service, zccPageSize)
			}),
			structSourceRecord[zccfailopen.WebFailOpenPolicy],
		),
		{product: resources.ProductZCC, name: resourceZCCFwdProfiles}: newListOnlyHandler(
			resourceZCCFwdProfiles,
			sdkProductList(resources.ProductZCC, client, func(ctx context.Context, service *zsdk.Service) ([]zccfwdprofile.ForwardingProfile, error) {
				return zccPaginate(ctx, func(ctx context.Context, page, pageSize int) ([]zccfwdprofile.ForwardingProfile, error) {
					return zccfwdprofile.GetForwardingProfileByCompanyID(ctx, service, "", &page, &pageSize)
				})
			}),
			structSourceRecord[zccfwdprofile.ForwardingProfile],
		),
		{product: resources.ProductZCC, name: resourceZCCTrustedNets}: newListOnlyHandler(
			resourceZCCTrustedNets,
			sdkProductList(resources.ProductZCC, client, func(ctx context.Context, service *zsdk.Service) ([]zcctrustednet.TrustedNetwork, error) {
				return zccPaginate(ctx, func(ctx context.Context, page, pageSize int) ([]zcctrustednet.TrustedNetwork, error) {
					resp, _, err := zcctrustednet.GetMultipleTrustedNetworks(ctx, service, "", "", &page, &pageSize)
					if err != nil {
						return nil, err
					}
					return resp.TrustedNetworkContracts, nil
				})
			}),
			structSourceRecord[zcctrustednet.TrustedNetwork],
		),
		{product: resources.ProductZCC, name: resourceZCCWebAppServices}: newListOnlyHandler(
			resourceZCCWebAppServices,
			sdkProductList(resources.ProductZCC, client, func(ctx context.Context, service *zsdk.Service) ([]zccwebappsvc.WebAppService, error) {
				return zccPaginate(ctx, func(ctx context.Context, page, pageSize int) ([]zccwebappsvc.WebAppService, error) {
					return zccwebappsvc.GetWebAppServices(ctx, service, "", &page, &pageSize)
				})
			}),
			structSourceRecord[zccwebappsvc.WebAppService],
		),
		{product: resources.ProductZCC, name: resourceZCCAppProfiles}: newListOnlyHandler(
			resourceZCCAppProfiles,
			sdkProductList(resources.ProductZCC, client, func(ctx context.Context, service *zsdk.Service) ([]zccappprofiles.ApplicationProfile, error) {
				return zccPaginate(ctx, func(ctx context.Context, page, pageSize int) ([]zccappprofiles.ApplicationProfile, error) {
					resp, _, err := zccappprofiles.GetApplicationProfiles(ctx, service, "", "", "", &page, &pageSize)
					if err != nil {
						return nil, err
					}
					return resp.Policies, nil
				})
			}),
			structSourceRecord[zccappprofiles.ApplicationProfile],
		),
		{product: resources.ProductZCC, name: resourceZCCCustomIPApps}: newListOnlyHandler(
			resourceZCCCustomIPApps,
			sdkProductList(resources.ProductZCC, client, func(ctx context.Context, service *zsdk.Service) ([]zcccustomip.CustomIPApp, error) {
				return zccPaginate(ctx, func(ctx context.Context, page, pageSize int) ([]zcccustomip.CustomIPApp, error) {
					resp, _, err := zcccustomip.GetCustomIPApps(ctx, service, "", &page, &pageSize)
					if err != nil {
						return nil, err
					}
					return resp.CustomAppContracts, nil
				})
			}),
			structSourceRecord[zcccustomip.CustomIPApp],
		),
		{product: resources.ProductZCC, name: resourceZCCPredefIPApps}: newListOnlyHandler(
			resourceZCCPredefIPApps,
			sdkProductList(resources.ProductZCC, client, func(ctx context.Context, service *zsdk.Service) ([]zccpredefip.PredefinedIPApp, error) {
				return zccPaginate(ctx, func(ctx context.Context, page, pageSize int) ([]zccpredefip.PredefinedIPApp, error) {
					resp, _, err := zccpredefip.GetPredefinedIPApps(ctx, service, "", &page, &pageSize)
					if err != nil {
						return nil, err
					}
					return resp.AppServiceContracts, nil
				})
			}),
			structSourceRecord[zccpredefip.PredefinedIPApp],
		),
		{product: resources.ProductZCC, name: resourceZCCProcessApps}: newListOnlyHandler(
			resourceZCCProcessApps,
			sdkProductList(resources.ProductZCC, client, func(ctx context.Context, service *zsdk.Service) ([]zccprocessapps.ProcessBasedApp, error) {
				return zccPaginate(ctx, func(ctx context.Context, page, pageSize int) ([]zccprocessapps.ProcessBasedApp, error) {
					resp, _, err := zccprocessapps.GetProcessBasedApps(ctx, service, "", &page, &pageSize)
					if err != nil {
						return nil, err
					}
					return resp.AppIdentities, nil
				})
			}),
			structSourceRecord[zccprocessapps.ProcessBasedApp],
		),
		{product: resources.ProductZCC, name: resourceZCCDevices}: newListOnlyHandler(
			resourceZCCDevices,
			sdkProductList(resources.ProductZCC, client, func(ctx context.Context, service *zsdk.Service) ([]zccdevices.GetDevices, error) {
				// devices.GetAll paginates via the SDK's ReadAllPages.
				return zccdevices.GetAll(ctx, service, "", "")
			}),
			structSourceRecord[zccdevices.GetDevices],
		),
		{product: resources.ProductZCC, name: resourceZCCAdminRoles}: newListOnlyHandler(
			resourceZCCAdminRoles,
			sdkProductList(resources.ProductZCC, client, func(ctx context.Context, service *zsdk.Service) ([]zccadminroles.AdminRole, error) {
				// GetAdminRoles paginates via the SDK's ReadAllPages.
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
