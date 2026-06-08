package zscaler

import (
	"context"
	"fmt"
	"net/http"

	zsdk "github.com/zscaler/zscaler-sdk-go/v3/zscaler"
	zpaappconnectorcontroller "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/appconnectorcontroller"
	zpaappconnectorgroup "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/appconnectorgroup"
	zpaapplicationsegment "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/applicationsegment"
	zpaappservercontroller "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/appservercontroller"
	zpac2cipranges "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/c2c_ip_ranges"
	zpacloudconnector "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/cloud_connector"
	zpacloudconnectorgroup "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/cloud_connector_group"
	zpacbizpaprofile "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/cloudbrowserisolation/cbizpaprofile"
	zpaconfigoverride "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/config_override"
	zpamachinegroup "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/machinegroup"
	zpapostureprofile "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/postureprofile"
	zpasegmentgroup "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/segmentgroup"
	zpaservergroup "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/servergroup"
	zpaserviceedgecontroller "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/serviceedgecontroller"
	zpaserviceedgegroup "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/serviceedgegroup"
	zpatrustednetwork "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/trustednetwork"

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
		m[k] = v
	}
}
