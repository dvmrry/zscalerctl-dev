package zscaler

import (
	"context"
	"fmt"

	zsdk "github.com/zscaler/zscaler-sdk-go/v3/zscaler"
	zccadminroles "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zcc/services/admin_roles"
	zccappprofiles "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zcc/services/application_profiles"
	zcccommon "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zcc/services/common"
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
// page size to MaxPageSize (5000), so 1000 is honored by the server and
// preserves the prior devices.GetAll request size.
const zccPageSize = 1000

// zccMaxPages is a fail-closed ceiling on the number of pages zccPaginate will
// fetch. Termination otherwise relies entirely on the server returning a
// short final page; an endpoint that keeps returning a persistently-full page
// would loop until --timeout fires on every request. The ceiling mirrors the
// zidentity/networkApplications guards: at zccPageSize=1000 it admits up to a
// million records before erroring, so it never trips on a real tenant, but it
// converts a pathological infinite loop into a visible, descriptive error.
const zccMaxPages = 1000

// zccPaginate walks every page of a ZCC list endpoint, mirroring the SDK's
// ReadAllPages contract: advance pages until one returns fewer than a full page.
// Several ZCC by-company list endpoints otherwise return only the first page
// and silently truncate large tenants.
func zccPaginate[T any](ctx context.Context, fetchPage func(ctx context.Context, page, pageSize int) ([]T, error)) ([]T, error) {
	var all []T
	for page := 1; ; page++ {
		if page > zccMaxPages {
			return nil, fmt.Errorf("zcc pagination exceeded the ceiling of %d pages (%d records); the endpoint kept returning full pages, so completeness cannot be guaranteed", zccMaxPages, len(all))
		}
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

func getZCCAllPages[T any](
	ctx context.Context,
	service *zsdk.Service,
	endpoint string,
	params zcccommon.QueryParams,
) ([]T, error) {
	items, err := zccPaginate(ctx, func(ctx context.Context, page, pageSize int) ([]T, error) {
		params.Page = page
		params.PageSize = pageSize
		return zcccommon.ReadPage[T](ctx, service.Client, endpoint, params)
	})
	if err != nil {
		return nil, err
	}
	items, err = zsdk.ApplyJMESPathFromContext(ctx, items)
	if err != nil {
		return nil, fmt.Errorf("apply zcc list filter: %w", err)
	}
	return items, nil
}

func addZCCHandlers(m map[resourceKey]resourceHandler, client sdkClient) {
	entries := map[resourceKey]resourceHandler{
		{product: resources.ProductZCC, name: resourceZCCFailOpenPolicy}: newListOnlyHandler(
			resourceZCCFailOpenPolicy,
			sdkProductList(resources.ProductZCC, client, func(ctx context.Context, service *zsdk.Service) ([]zccfailopen.WebFailOpenPolicy, error) {
				return getZCCAllPages[zccfailopen.WebFailOpenPolicy](
					ctx,
					service,
					"/zcc/papi/public/v1/webFailOpenPolicy/listByCompany",
					zcccommon.QueryParams{},
				)
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
				return getZCCAllPages[zccdevices.GetDevices](
					ctx,
					service,
					"/zcc/papi/public/v1/getDevices",
					zcccommon.QueryParams{},
				)
			}),
			structSourceRecord[zccdevices.GetDevices],
		),
		{product: resources.ProductZCC, name: resourceZCCAdminRoles}: newListOnlyHandler(
			resourceZCCAdminRoles,
			sdkProductList(resources.ProductZCC, client, func(ctx context.Context, service *zsdk.Service) ([]zccadminroles.AdminRole, error) {
				return getZCCAllPages[zccadminroles.AdminRole](
					ctx,
					service,
					"/zcc/papi/public/v1/getAdminRoles",
					zcccommon.QueryParams{},
				)
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
