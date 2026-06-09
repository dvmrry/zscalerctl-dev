package zscaler

import (
	"context"
	"fmt"
	"net/http"

	zsdk "github.com/zscaler/zscaler-sdk-go/v3/zscaler"
	zpaappconnectorcontroller "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/appconnectorcontroller"
	zpaappconnectorgroup "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/appconnectorgroup"
	zpaapplicationsegment "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/applicationsegment"
	zpaappsegmentba "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/applicationsegmentbrowseraccess"
	zpaappsegmentinspection "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/applicationsegmentinspection"
	zpaappsegmentpra "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/applicationsegmentpra"
	zpaappservercontroller "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/appservercontroller"
	zpabranchconnector "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/branch_connector"
	zpac2cipranges "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/c2c_ip_ranges"
	zpaclienttypes "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/clienttypes"
	zpacloudconnector "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/cloud_connector"
	zpacloudconnectorgroup "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/cloud_connector_group"
	zpacbibanner "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/cloudbrowserisolation/cbibannercontroller"
	zpacbiprofile "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/cloudbrowserisolation/cbiprofilecontroller"
	zpacbiregions "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/cloudbrowserisolation/cbiregions"
	zpacbizpaprofile "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/cloudbrowserisolation/cbizpaprofile"
	zpaisolationprofile "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/cloudbrowserisolation/isolationprofile"
	zpaconfigoverride "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/config_override"
	zpaversionprofile "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/customerversionprofile"
	zpainspectioncustom "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/inspectioncontrol/inspection_custom_controls"
	zpainspectionpredef "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/inspectioncontrol/inspection_predefined_controls"
	zpainspectionprofile "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/inspectioncontrol/inspection_profile"
	zpamachinegroup "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/machinegroup"
	zpamicrotenants "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/microtenants"
	zpaplatforms "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/platforms"
	zpapostureprofile "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/postureprofile"
	zpapraapproval "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/privilegedremoteaccess/praapproval"
	zpapraconsole "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/privilegedremoteaccess/praconsole"
	zpapraportal "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/privilegedremoteaccess/praportal"
	zpasegmentgroup "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/segmentgroup"
	zpaservergroup "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/servergroup"
	zpaserviceedgecontroller "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/serviceedgecontroller"
	zpaserviceedgegroup "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/serviceedgegroup"
	zpataggroup "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/tag_controller/tag_group"
	zpatagkey "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/tag_controller/tag_key"
	zpatagnamespace "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/tag_controller/tag_namespace"
	zpatrustednetwork "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/trustednetwork"
	zpauserportalaup "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/userportal/aup"
	zpauserportal "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/userportal/portal_controller"
	zpauserportallink "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/userportal/portal_link"

	"github.com/dvmrry/zscalerctl/internal/resources"
)

func addZPAHandlers(m map[resourceKey]resourceHandler, client sdkClient) {
	entries := map[resourceKey]resourceHandler{
		{product: resources.ProductZPA, name: resourceZPAServerGroups}: newListGetHandler(
			resourceZPAServerGroups,
			zpaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]zpaservergroup.ServerGroup, *http.Response, error) {
				return zpaservergroup.GetAll(ctx, service)
			}),
			zpaSDKStringGet(client, func(ctx context.Context, service *zsdk.Service, id string) (*zpaservergroup.ServerGroup, *http.Response, error) {
				return zpaservergroup.Get(ctx, service, id)
			}),
			jsonSourceRecord[zpaservergroup.ServerGroup],
		),
		{product: resources.ProductZPA, name: resourceZPAMicrotenants}: newListGetHandler(
			resourceZPAMicrotenants,
			zpaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]zpamicrotenants.MicroTenant, *http.Response, error) {
				return zpamicrotenants.GetAll(ctx, service)
			}),
			zpaSDKStringGet(client, func(ctx context.Context, service *zsdk.Service, id string) (*zpamicrotenants.MicroTenant, *http.Response, error) {
				return zpamicrotenants.Get(ctx, service, id)
			}),
			jsonSourceRecord[zpamicrotenants.MicroTenant],
		),
		{product: resources.ProductZPA, name: resourceZPAVersionProfiles}: newListOnlyHandler(
			resourceZPAVersionProfiles,
			zpaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]zpaversionprofile.CustomerVersionProfile, *http.Response, error) {
				return zpaversionprofile.GetAll(ctx, service)
			}),
			jsonSourceRecord[zpaversionprofile.CustomerVersionProfile],
		),
		{product: resources.ProductZPA, name: resourceZPAClientTypes}: newSingletonHandler(
			resourceZPAClientTypes,
			zpaSDKShow(client, func(ctx context.Context, service *zsdk.Service) (*zpaclienttypes.ClientTypes, *http.Response, error) {
				return zpaclienttypes.GetAllClientTypes(ctx, service)
			}),
			jsonSourceRecord[zpaclienttypes.ClientTypes],
		),
		{product: resources.ProductZPA, name: resourceZPAPlatforms}: newSingletonHandler(
			resourceZPAPlatforms,
			zpaSDKShow(client, func(ctx context.Context, service *zsdk.Service) (*zpaplatforms.Platforms, *http.Response, error) {
				return zpaplatforms.GetAllPlatforms(ctx, service)
			}),
			jsonSourceRecord[zpaplatforms.Platforms],
		),
		{product: resources.ProductZPA, name: resourceZPAInspectionProfiles}: newListGetHandler(
			resourceZPAInspectionProfiles,
			zpaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]zpainspectionprofile.InspectionProfile, *http.Response, error) {
				return zpainspectionprofile.GetAll(ctx, service)
			}),
			zpaSDKStringGet(client, func(ctx context.Context, service *zsdk.Service, id string) (*zpainspectionprofile.InspectionProfile, *http.Response, error) {
				return zpainspectionprofile.Get(ctx, service, id)
			}),
			jsonSourceRecord[zpainspectionprofile.InspectionProfile],
		),
		{product: resources.ProductZPA, name: resourceZPAInspectionCustomCtl}: newListGetHandler(
			resourceZPAInspectionCustomCtl,
			zpaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]zpainspectioncustom.InspectionCustomControl, *http.Response, error) {
				return zpainspectioncustom.GetAll(ctx, service)
			}),
			zpaSDKStringGet(client, func(ctx context.Context, service *zsdk.Service, id string) (*zpainspectioncustom.InspectionCustomControl, *http.Response, error) {
				return zpainspectioncustom.Get(ctx, service, id)
			}),
			jsonSourceRecord[zpainspectioncustom.InspectionCustomControl],
		),
		{product: resources.ProductZPA, name: resourceZPAInspectionPredefinedCtl}: newListGetHandler(
			resourceZPAInspectionPredefinedCtl,
			zpaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]zpainspectionpredef.PredefinedControls, *http.Response, error) {
				items, err := zpainspectionpredef.GetAll(ctx, service, "")
				return items, nil, err
			}),
			zpaSDKStringGet(client, func(ctx context.Context, service *zsdk.Service, id string) (*zpainspectionpredef.PredefinedControls, *http.Response, error) {
				return zpainspectionpredef.Get(ctx, service, id)
			}),
			jsonSourceRecord[zpainspectionpredef.PredefinedControls],
		),
		{product: resources.ProductZPA, name: resourceZPATagGroups}: newListGetHandler(
			resourceZPATagGroups,
			zpaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]zpataggroup.TagGroup, *http.Response, error) {
				return zpataggroup.GetAll(ctx, service)
			}),
			zpaSDKStringGet(client, func(ctx context.Context, service *zsdk.Service, id string) (*zpataggroup.TagGroup, *http.Response, error) {
				return zpataggroup.Get(ctx, service, id)
			}),
			jsonSourceRecord[zpataggroup.TagGroup],
		),
		{product: resources.ProductZPA, name: resourceZPATagKeys}: newListOnlyHandler(
			resourceZPATagKeys,
			zpaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]zpatagkey.TagKey, *http.Response, error) {
				// Tag keys are namespace-scoped; enumerate namespaces and gather each
				// namespace's keys (per-namespace errors are skipped).
				namespaces, resp, err := zpatagnamespace.GetAll(ctx, service)
				if err != nil {
					return nil, resp, err
				}
				var all []zpatagkey.TagKey
				for i := range namespaces {
					keys, _, err := zpatagkey.GetAll(ctx, service, namespaces[i].ID)
					if err != nil {
						continue
					}
					all = append(all, keys...)
				}
				return all, nil, nil
			}),
			jsonSourceRecord[zpatagkey.TagKey],
		),
		{product: resources.ProductZPA, name: resourceZPATagNamespaces}: newListGetHandler(
			resourceZPATagNamespaces,
			zpaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]zpatagnamespace.Namespace, *http.Response, error) {
				return zpatagnamespace.GetAll(ctx, service)
			}),
			zpaSDKStringGet(client, func(ctx context.Context, service *zsdk.Service, id string) (*zpatagnamespace.Namespace, *http.Response, error) {
				return zpatagnamespace.Get(ctx, service, id)
			}),
			jsonSourceRecord[zpatagnamespace.Namespace],
		),
		{product: resources.ProductZPA, name: resourceZPACBIBanners}: newListGetHandler(
			resourceZPACBIBanners,
			zpaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]zpacbibanner.CBIBannerController, *http.Response, error) {
				return zpacbibanner.GetAll(ctx, service)
			}),
			zpaSDKStringGet(client, func(ctx context.Context, service *zsdk.Service, id string) (*zpacbibanner.CBIBannerController, *http.Response, error) {
				return zpacbibanner.Get(ctx, service, id)
			}),
			jsonSourceRecord[zpacbibanner.CBIBannerController],
		),
		{product: resources.ProductZPA, name: resourceZPACBIProfiles}: newListGetHandler(
			resourceZPACBIProfiles,
			zpaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]zpacbiprofile.IsolationProfile, *http.Response, error) {
				return zpacbiprofile.GetAll(ctx, service)
			}),
			zpaSDKStringGet(client, func(ctx context.Context, service *zsdk.Service, id string) (*zpacbiprofile.IsolationProfile, *http.Response, error) {
				return zpacbiprofile.Get(ctx, service, id)
			}),
			jsonSourceRecord[zpacbiprofile.IsolationProfile],
		),
		{product: resources.ProductZPA, name: resourceZPACBIRegions}: newListOnlyHandler(
			resourceZPACBIRegions,
			zpaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]zpacbiregions.CBIRegions, *http.Response, error) {
				return zpacbiregions.GetAll(ctx, service)
			}),
			jsonSourceRecord[zpacbiregions.CBIRegions],
		),
		{product: resources.ProductZPA, name: resourceZPAIsolationProfiles}: newListOnlyHandler(
			resourceZPAIsolationProfiles,
			zpaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]zpaisolationprofile.IsolationProfile, *http.Response, error) {
				return zpaisolationprofile.GetAll(ctx, service)
			}),
			jsonSourceRecord[zpaisolationprofile.IsolationProfile],
		),
		{product: resources.ProductZPA, name: resourceZPABranchConnectors}: newListOnlyHandler(
			resourceZPABranchConnectors,
			zpaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]zpabranchconnector.BranchConnector, *http.Response, error) {
				return zpabranchconnector.GetAll(ctx, service)
			}),
			jsonSourceRecord[zpabranchconnector.BranchConnector],
		),
		{product: resources.ProductZPA, name: resourceZPAPRAApprovals}: newListGetHandler(
			resourceZPAPRAApprovals,
			zpaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]zpapraapproval.PrivilegedApproval, *http.Response, error) {
				return zpapraapproval.GetAll(ctx, service)
			}),
			zpaSDKStringGet(client, func(ctx context.Context, service *zsdk.Service, id string) (*zpapraapproval.PrivilegedApproval, *http.Response, error) {
				return zpapraapproval.Get(ctx, service, id)
			}),
			jsonSourceRecord[zpapraapproval.PrivilegedApproval],
		),
		{product: resources.ProductZPA, name: resourceZPAPRAConsoles}: newListGetHandler(
			resourceZPAPRAConsoles,
			zpaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]zpapraconsole.PRAConsole, *http.Response, error) {
				return zpapraconsole.GetAll(ctx, service)
			}),
			zpaSDKStringGet(client, func(ctx context.Context, service *zsdk.Service, id string) (*zpapraconsole.PRAConsole, *http.Response, error) {
				return zpapraconsole.Get(ctx, service, id)
			}),
			jsonSourceRecord[zpapraconsole.PRAConsole],
		),
		{product: resources.ProductZPA, name: resourceZPAPRAPortals}: newListGetHandler(
			resourceZPAPRAPortals,
			zpaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]zpapraportal.PRAPortal, *http.Response, error) {
				return zpapraportal.GetAll(ctx, service)
			}),
			zpaSDKStringGet(client, func(ctx context.Context, service *zsdk.Service, id string) (*zpapraportal.PRAPortal, *http.Response, error) {
				return zpapraportal.Get(ctx, service, id)
			}),
			jsonSourceRecord[zpapraportal.PRAPortal],
		),
		{product: resources.ProductZPA, name: resourceZPAUserPortals}: newListGetHandler(
			resourceZPAUserPortals,
			zpaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]zpauserportal.UserPortalController, *http.Response, error) {
				return zpauserportal.GetAll(ctx, service)
			}),
			zpaSDKStringGet(client, func(ctx context.Context, service *zsdk.Service, id string) (*zpauserportal.UserPortalController, *http.Response, error) {
				return zpauserportal.Get(ctx, service, id)
			}),
			jsonSourceRecord[zpauserportal.UserPortalController],
		),
		{product: resources.ProductZPA, name: resourceZPAUserPortalAups}: newListGetHandler(
			resourceZPAUserPortalAups,
			zpaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]zpauserportalaup.UserPortalAup, *http.Response, error) {
				return zpauserportalaup.GetAll(ctx, service)
			}),
			zpaSDKStringGet(client, func(ctx context.Context, service *zsdk.Service, id string) (*zpauserportalaup.UserPortalAup, *http.Response, error) {
				return zpauserportalaup.Get(ctx, service, id)
			}),
			jsonSourceRecord[zpauserportalaup.UserPortalAup],
		),
		{product: resources.ProductZPA, name: resourceZPAUserPortalLinks}: newListGetHandler(
			resourceZPAUserPortalLinks,
			zpaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]zpauserportallink.UserPortalLink, *http.Response, error) {
				return zpauserportallink.GetAll(ctx, service)
			}),
			zpaSDKStringGet(client, func(ctx context.Context, service *zsdk.Service, id string) (*zpauserportallink.UserPortalLink, *http.Response, error) {
				return zpauserportallink.Get(ctx, service, id)
			}),
			jsonSourceRecord[zpauserportallink.UserPortalLink],
		),
		{product: resources.ProductZPA, name: resourceZPABrowserAccess}: newListGetHandler(
			resourceZPABrowserAccess,
			zpaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]zpaappsegmentba.BrowserAccess, *http.Response, error) {
				return zpaappsegmentba.GetAll(ctx, service)
			}),
			zpaSDKStringGet(client, func(ctx context.Context, service *zsdk.Service, id string) (*zpaappsegmentba.BrowserAccess, *http.Response, error) {
				return zpaappsegmentba.Get(ctx, service, id)
			}),
			jsonSourceRecord[zpaappsegmentba.BrowserAccess],
		),
		{product: resources.ProductZPA, name: resourceZPAInspectionAppSegments}: newListGetHandler(
			resourceZPAInspectionAppSegments,
			zpaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]zpaappsegmentinspection.AppSegmentInspection, *http.Response, error) {
				return zpaappsegmentinspection.GetAll(ctx, service)
			}),
			zpaSDKStringGet(client, func(ctx context.Context, service *zsdk.Service, id string) (*zpaappsegmentinspection.AppSegmentInspection, *http.Response, error) {
				return zpaappsegmentinspection.Get(ctx, service, id)
			}),
			jsonSourceRecord[zpaappsegmentinspection.AppSegmentInspection],
		),
		{product: resources.ProductZPA, name: resourceZPAPRAAppSegments}: newListGetHandler(
			resourceZPAPRAAppSegments,
			zpaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]zpaappsegmentpra.AppSegmentPRA, *http.Response, error) {
				return zpaappsegmentpra.GetAll(ctx, service)
			}),
			zpaSDKStringGet(client, func(ctx context.Context, service *zsdk.Service, id string) (*zpaappsegmentpra.AppSegmentPRA, *http.Response, error) {
				return zpaappsegmentpra.Get(ctx, service, id)
			}),
			jsonSourceRecord[zpaappsegmentpra.AppSegmentPRA],
		),
		{product: resources.ProductZPA, name: resourceZPASegmentGroups}: newListGetHandler(
			resourceZPASegmentGroups,
			zpaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]zpasegmentgroup.SegmentGroup, *http.Response, error) {
				return zpasegmentgroup.GetAll(ctx, service)
			}),
			zpaSDKStringGet(client, func(ctx context.Context, service *zsdk.Service, id string) (*zpasegmentgroup.SegmentGroup, *http.Response, error) {
				return zpasegmentgroup.Get(ctx, service, id)
			}),
			jsonSourceRecord[zpasegmentgroup.SegmentGroup],
		),
		{product: resources.ProductZPA, name: resourceZPAAppSegments}: newListGetHandler(
			resourceZPAAppSegments,
			zpaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]zpaapplicationsegment.ApplicationSegmentResource, *http.Response, error) {
				return zpaapplicationsegment.GetAll(ctx, service)
			}),
			zpaSDKStringGet(client, func(ctx context.Context, service *zsdk.Service, id string) (*zpaapplicationsegment.ApplicationSegmentResource, *http.Response, error) {
				return zpaapplicationsegment.Get(ctx, service, id)
			}),
			applicationSegmentSourceRecord,
		),
		{product: resources.ProductZPA, name: resourceZPAAppConnectors}: newListGetHandler(
			resourceZPAAppConnectors,
			zpaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]zpaappconnectorcontroller.AppConnector, *http.Response, error) {
				return zpaappconnectorcontroller.GetAll(ctx, service)
			}),
			zpaSDKStringGet(client, func(ctx context.Context, service *zsdk.Service, id string) (*zpaappconnectorcontroller.AppConnector, *http.Response, error) {
				return zpaappconnectorcontroller.Get(ctx, service, id)
			}),
			jsonSourceRecord[zpaappconnectorcontroller.AppConnector],
		),
		{product: resources.ProductZPA, name: resourceZPAConnectorGrps}: newListGetHandler(
			resourceZPAConnectorGrps,
			zpaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]zpaappconnectorgroup.AppConnectorGroup, *http.Response, error) {
				return zpaappconnectorgroup.GetAll(ctx, service)
			}),
			zpaSDKStringGet(client, func(ctx context.Context, service *zsdk.Service, id string) (*zpaappconnectorgroup.AppConnectorGroup, *http.Response, error) {
				return zpaappconnectorgroup.Get(ctx, service, id)
			}),
			jsonSourceRecord[zpaappconnectorgroup.AppConnectorGroup],
		),
		{product: resources.ProductZPA, name: resourceZPAAppServers}: newListGetHandler(
			resourceZPAAppServers,
			zpaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]zpaappservercontroller.ApplicationServer, *http.Response, error) {
				return zpaappservercontroller.GetAll(ctx, service)
			}),
			zpaSDKStringGet(client, func(ctx context.Context, service *zsdk.Service, id string) (*zpaappservercontroller.ApplicationServer, *http.Response, error) {
				return zpaappservercontroller.Get(ctx, service, id)
			}),
			jsonSourceRecord[zpaappservercontroller.ApplicationServer],
		),
		{product: resources.ProductZPA, name: resourceZPAMachineGroups}: newListGetHandler(
			resourceZPAMachineGroups,
			zpaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]zpamachinegroup.MachineGroup, *http.Response, error) {
				return zpamachinegroup.GetAll(ctx, service)
			}),
			zpaSDKStringGet(client, func(ctx context.Context, service *zsdk.Service, id string) (*zpamachinegroup.MachineGroup, *http.Response, error) {
				return zpamachinegroup.Get(ctx, service, id)
			}),
			jsonSourceRecord[zpamachinegroup.MachineGroup],
		),
		{product: resources.ProductZPA, name: resourceZPATrustedNets}: newListGetHandler(
			resourceZPATrustedNets,
			zpaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]zpatrustednetwork.TrustedNetwork, *http.Response, error) {
				return zpatrustednetwork.GetAll(ctx, service)
			}),
			zpaSDKStringGet(client, func(ctx context.Context, service *zsdk.Service, id string) (*zpatrustednetwork.TrustedNetwork, *http.Response, error) {
				return zpatrustednetwork.Get(ctx, service, id)
			}),
			jsonSourceRecord[zpatrustednetwork.TrustedNetwork],
		),
		{product: resources.ProductZPA, name: resourceZPAServiceGrps}: newListGetHandler(
			resourceZPAServiceGrps,
			zpaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]zpaserviceedgegroup.ServiceEdgeGroup, *http.Response, error) {
				return zpaserviceedgegroup.GetAll(ctx, service)
			}),
			zpaSDKStringGet(client, func(ctx context.Context, service *zsdk.Service, id string) (*zpaserviceedgegroup.ServiceEdgeGroup, *http.Response, error) {
				return zpaserviceedgegroup.Get(ctx, service, id)
			}),
			jsonSourceRecord[zpaserviceedgegroup.ServiceEdgeGroup],
		),
		{product: resources.ProductZPA, name: resourceZPAServiceEdges}: newListGetHandler(
			resourceZPAServiceEdges,
			zpaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]zpaserviceedgecontroller.ServiceEdgeController, *http.Response, error) {
				return zpaserviceedgecontroller.GetAll(ctx, service)
			}),
			zpaSDKStringGet(client, func(ctx context.Context, service *zsdk.Service, id string) (*zpaserviceedgecontroller.ServiceEdgeController, *http.Response, error) {
				return zpaserviceedgecontroller.Get(ctx, service, id)
			}),
			jsonSourceRecord[zpaserviceedgecontroller.ServiceEdgeController],
		),
		{product: resources.ProductZPA, name: resourceZPACloudConnGrps}: newListGetHandler(
			resourceZPACloudConnGrps,
			zpaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]zpacloudconnectorgroup.CloudConnectorGroup, *http.Response, error) {
				return zpacloudconnectorgroup.GetAll(ctx, service)
			}),
			zpaSDKStringGet(client, func(ctx context.Context, service *zsdk.Service, id string) (*zpacloudconnectorgroup.CloudConnectorGroup, *http.Response, error) {
				return zpacloudconnectorgroup.Get(ctx, service, id)
			}),
			jsonSourceRecord[zpacloudconnectorgroup.CloudConnectorGroup],
		),
		{product: resources.ProductZPA, name: resourceZPACloudConns}: newListGetHandler(
			resourceZPACloudConns,
			zpaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]zpacloudconnector.CloudConnector, *http.Response, error) {
				return zpacloudconnector.GetAll(ctx, service)
			}),
			func(context.Context, string) (*zpacloudconnector.CloudConnector, error) {
				return nil, fmt.Errorf("%w: %s/%s get", ErrUnsupportedResource, resources.ProductZPA, resourceZPACloudConns)
			},
			jsonSourceRecord[zpacloudconnector.CloudConnector],
		),
		{product: resources.ProductZPA, name: resourceZPAPostureProfs}: newListGetHandler(
			resourceZPAPostureProfs,
			zpaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]zpapostureprofile.PostureProfile, *http.Response, error) {
				return zpapostureprofile.GetAll(ctx, service)
			}),
			zpaSDKStringGet(client, func(ctx context.Context, service *zsdk.Service, id string) (*zpapostureprofile.PostureProfile, *http.Response, error) {
				return zpapostureprofile.Get(ctx, service, id)
			}),
			jsonSourceRecord[zpapostureprofile.PostureProfile],
		),
		{product: resources.ProductZPA, name: resourceZPACBIZPAProfs}: newListGetHandler(
			resourceZPACBIZPAProfs,
			zpaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]zpacbizpaprofile.ZPAProfiles, *http.Response, error) {
				return zpacbizpaprofile.GetAll(ctx, service)
			}),
			zpaSDKStringGet(client, func(ctx context.Context, service *zsdk.Service, id string) (*zpacbizpaprofile.ZPAProfiles, *http.Response, error) {
				return zpacbizpaprofile.Get(ctx, service, id)
			}),
			jsonSourceRecord[zpacbizpaprofile.ZPAProfiles],
		),
		{product: resources.ProductZPA, name: resourceZPAC2CIPRanges}: newListGetHandler(
			resourceZPAC2CIPRanges,
			zpaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]zpac2cipranges.IPRanges, *http.Response, error) {
				items, resp, err := zpac2cipranges.GetAll(ctx, service)
				if err != nil {
					return nil, resp, err
				}
				out := make([]zpac2cipranges.IPRanges, 0, len(items))
				for _, item := range items {
					if item != nil {
						out = append(out, *item)
					}
				}
				return out, resp, nil
			}),
			zpaSDKStringGet(client, func(ctx context.Context, service *zsdk.Service, id string) (*zpac2cipranges.IPRanges, *http.Response, error) {
				return zpac2cipranges.Get(ctx, service, id)
			}),
			jsonSourceRecord[zpac2cipranges.IPRanges],
		),
		{product: resources.ProductZPA, name: resourceZPAConfigOvrds}: newListGetHandler(
			resourceZPAConfigOvrds,
			zpaSDKList(client, func(ctx context.Context, service *zsdk.Service) ([]zpaconfigoverride.ConfigOverrides, *http.Response, error) {
				return zpaconfigoverride.GetAll(ctx, service)
			}),
			zpaSDKStringGet(client, func(ctx context.Context, service *zsdk.Service, id string) (*zpaconfigoverride.ConfigOverrides, *http.Response, error) {
				return zpaconfigoverride.Get(ctx, service, id)
			}),
			jsonSourceRecord[zpaconfigoverride.ConfigOverrides],
		),
	}
	for k, v := range entries {
		addHandler(m, k, v)
	}
}
