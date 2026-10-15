package resources

// ZPA reference targets; see fieldReferences in references.go.
var zpaFieldReferences = map[string]map[string][]ResourceRef{
	"zpa/app-connector-groups": {
		"connectors":                           zpaRef("app-connectors"),
		"microtenantId":                        zpaRef("microtenants"),
		"npAssistantGroup.appConnectorGroupId": zpaRef("app-connector-groups"),
		"npAssistantGroup.lanSubnets.appConnectorGroupId": zpaRef("app-connector-groups"),
		"serverGroups":     zpaRef("server-groups"),
		"versionProfileId": zpaRef("version-profiles"),
	},
	"zpa/app-connectors": {
		"appConnectorGroupId":                  zpaRef("app-connector-groups"),
		"assistantVersion.appConnectorGroupId": zpaRef("app-connector-groups"),
		"microtenantId":                        zpaRef("microtenants"),
	},
	"zpa/app-servers": {
		"appServerGroupIds": zpaRef("server-groups"),
		"microtenantId":     zpaRef("microtenants"),
	},
	"zpa/application-segments": {
		"clientlessApps.microtenantId": zpaRef("microtenants"),
		"microtenantId":                zpaRef("microtenants"),
		"segmentGroupId":               zpaRef("segment-groups"),
		"serverGroups":                 zpaRef("server-groups"),
		"shareToMicrotenants":          zpaRef("microtenants"),
		"sharedMicrotenantDetails.sharedFromMicrotenant": zpaRef("microtenants"),
		"sharedMicrotenantDetails.sharedToMicrotenants":  zpaRef("microtenants"),
	},
	"zpa/browser-access": {
		"clientlessApps.microtenantId": zpaRef("microtenants"),
		"microtenantId":                zpaRef("microtenants"),
		"segmentGroupId":               zpaRef("segment-groups"),
		"serverGroups":                 zpaRef("server-groups"),
		"sharedMicrotenantDetails.sharedFromMicrotenant": zpaRef("microtenants"),
		"sharedMicrotenantDetails.sharedToMicrotenants":  zpaRef("microtenants"),
	},
	"zpa/cloud-connector-groups": {
		"cloudConnectors": zpaRef("cloud-connectors"),
	},
	"zpa/inspection-app-segments": {
		"inspectionApps.microtenantId": zpaRef("microtenants"),
		"microtenantId":                zpaRef("microtenants"),
		"segmentGroupId":               zpaRef("segment-groups"),
		"serverGroups":                 zpaRef("server-groups"),
		"sharedMicrotenantDetails.sharedFromMicrotenant": zpaRef("microtenants"),
		"sharedMicrotenantDetails.sharedToMicrotenants":  zpaRef("microtenants"),
	},
	"zpa/machine-groups": {
		"machines.machineGroupId": zpaRef("machine-groups"),
		"machines.microtenantId":  zpaRef("microtenants"),
		"microtenantId":           zpaRef("microtenants"),
	},
	"zpa/microtenants": {
		"user.microtenantId": zpaRef("microtenants"),
	},
	"zpa/pra-app-segments": {
		"microtenantId":         zpaRef("microtenants"),
		"praApps.microtenantId": zpaRef("microtenants"),
		"segmentGroupId":        zpaRef("segment-groups"),
		"serverGroups":          zpaRef("server-groups"),
		"sharedMicrotenantDetails.sharedFromMicrotenant": zpaRef("microtenants"),
		"sharedMicrotenantDetails.sharedToMicrotenants":  zpaRef("microtenants"),
	},
	"zpa/segment-groups": {
		"applicationNames": zpaRef("application-segments"),
		"applications":     zpaRef("application-segments"),
		"microtenantId":    zpaRef("microtenants"),
	},
	"zpa/server-groups": {
		"appConnectorGroups": zpaRef("app-connector-groups"),
		"applications":       zpaRef("application-segments"),
		"microtenantId":      zpaRef("microtenants"),
		"servers":            zpaRef("app-servers"),
	},
	"zpa/service-edge-groups": {
		"microtenantId":    zpaRef("microtenants"),
		"serviceEdges":     zpaRef("service-edges"),
		"trustedNetworks":  zpaRef("trusted-networks"),
		"versionProfileId": zpaRef("version-profiles"),
	},
	"zpa/service-edges": {
		"microtenantId": zpaRef("microtenants"),
		"privateBrokerVersion.serviceEdgeGroupId": zpaRef("service-edge-groups"),
		"serviceEdgeGroupId":                      zpaRef("service-edge-groups"),
	},
	"zpa/user-portal-aups": {
		"microtenantId": zpaRef("microtenants"),
	},
	"zpa/user-portal-links": {
		"microtenantId": zpaRef("microtenants"),
		"userPortalId":  zpaRef("user-portals"),
		"userPortals":   zpaRef("user-portals"),
	},
	"zpa/user-portals": {
		"microtenantId": zpaRef("microtenants"),
	},
}

var zpaUnmappedReferences = map[string]map[string]string{
	"zpa/app-connector-groups": {
		"enrollmentCertId":                          "no certificate resource in the catalog",
		"npAssistantGroup":                          "the group's own network-presence settings, not a pointer",
		"npAssistantGroup.lanSubnets":               "the group's own LAN subnet entries, not a pointer",
		"npAssistantGroup.lanSubnets.npDnsNsRecord": "the subnet's own DNS NS record, not a pointer",
		"geoLocationId":                             "no geolocation resource in the catalog",
		"siteId":                                    "no site resource in the catalog",
	},
	"zpa/app-connectors": {
		"assistantVersion":          "the connector's own runtime version record, not a pointer",
		"assistantVersion.brokerId": "no broker resource in the catalog",
		"zpnSubModuleUpgradeList":   "the connector's own sub-module upgrade records, not a pointer",
		"issuedCertId":              "no certificate resource in the catalog",
		"provisioningKeyId":         "no provisioning key resource in the catalog",
	},
	"zpa/application-segments": {
		"appRecommendationId":          "no app recommendation resource in the catalog",
		"clientlessApps":               "the segment's own browser-access app entries, not a pointer",
		"clientlessApps.certificateId": "no certificate resource in the catalog",
		"tags.namespace":               "no tag namespace resource in the catalog",
		"tags.tagKey":                  "no tag key resource in the catalog",
		"tags.tagValue":                "no tag value resource in the catalog",
		"zpnErId":                      "no extranet resource in the catalog",
		"zpnErId.ziaErId":              "no ZIA extranet resource in the catalog",
		"zpnErId.ziaOrgId":             "a ZIA org id, not a record id",
	},
	"zpa/branch-connectors": {
		"branchConnectorGroupId": "no branch connector group resource in the catalog",
		"edgeConnectorGroupId":   "which group resource the id belongs to is not verified",
		"issuedCertId":           "no certificate resource in the catalog",
	},
	"zpa/browser-access": {
		"appRecommendationId":          "no app recommendation resource in the catalog",
		"clientlessApps":               "the segment's own browser-access app entries, not a pointer",
		"clientlessApps.certificateId": "no certificate resource in the catalog",
		"zpnErId":                      "no extranet resource in the catalog",
		"zpnErId.ziaErId":              "no ZIA extranet resource in the catalog",
		"zpnErId.ziaOrgId":             "a ZIA org id, not a record id",
	},
	"zpa/c2c-ip-ranges": {
		"customerId": "a customer id, not a record id",
	},
	"zpa/cbi-zpa-profiles": {
		"cbiProfileId": "a Cloud Browser Isolation profile id; match to zia/browser-isolation-profiles is not verified",
		"cbiTenantId":  "a Cloud Browser Isolation tenant id, not a record id",
	},
	"zpa/cloud-connector-groups": {
		"geoLocationId": "no geolocation resource in the catalog",
		"ziaOrgId":      "a ZIA organization id, not a record id",
	},
	"zpa/cloud-connectors": {
		"edgeConnectorGroupId": "which group resource the id belongs to is not verified",
		"issuedCertId":         "no certificate resource in the catalog",
	},
	"zpa/config-overrides": {
		"customerId": "a customer id, not a record id",
	},
	"zpa/inspection-app-segments": {
		"appRecommendationId":                    "no app recommendation resource in the catalog",
		"commonAppsDto.appsConfig":               "the segment's own app config entries, not a pointer",
		"commonAppsDto.appsConfig.appId":         "the SDK does not say which resource the id belongs to",
		"commonAppsDto.appsConfig.certificateId": "no certificate resource in the catalog",
		"commonAppsDto.appsConfig.inspectAppId":  "no inspection app resource in the catalog",
		"inspectionApps":                         "the segment's own inspection app entries, not a pointer",
		"inspectionApps.appId":                   "the SDK does not say which resource the id belongs to",
		"inspectionApps.certificateId":           "no certificate resource in the catalog",
	},
	"zpa/machine-groups": {
		"machines":                "the group's own machine entries; no machine resource in the catalog",
		"machines.issuedCertId":   "no certificate resource in the catalog",
		"machines.machineTokenId": "no machine provisioning token resource in the catalog",
	},
	"zpa/microtenants": {
		"roles":           "no ZPA admin role resource in the catalog",
		"user":            "the microtenant's admin user; no ZPA admin user resource in the catalog",
		"user.customerId": "a customer id, not a record id",
		"user.roleId":     "no ZPA admin role resource in the catalog",
		"user.tokenId":    "an MFA token identifier, not a catalog record",
	},
	"zpa/isolation-profiles": {
		"isolationProfileId": "a Cloud Browser Isolation profile id; match to zia/browser-isolation-profiles is not verified",
		"isolationTenantId":  "a Cloud Browser Isolation tenant id, not a record id",
	},
	"zpa/posture-profiles": {
		"masterCustomerId":  "a customer id, not a record id",
		"zscalerCustomerId": "a customer id, not a record id",
	},
	"zpa/pra-app-segments": {
		"appRecommendationId":               "no app recommendation resource in the catalog",
		"commonAppsDto.appsConfig":          "the segment's own app config entries, not a pointer",
		"commonAppsDto.appsConfig.appId":    "the SDK does not say which resource the id belongs to",
		"commonAppsDto.appsConfig.praAppId": "no PRA app resource in the catalog",
		"praApps":                           "the segment's own PRA app entries, not a pointer",
		"praApps.appId":                     "the SDK does not say which resource the id belongs to",
		"praApps.certificateId":             "no certificate resource in the catalog",
		"zpnErId":                           "no extranet resource in the catalog",
		"zpnErId.ziaErId":                   "no ZIA extranet resource in the catalog",
		"zpnErId.ziaOrgId":                  "a ZIA org id, not a record id",
	},
	"zpa/server-groups": {
		"extranetDTO.locationDTO":                   "a ZIA location used by the extranet; match to zia/locations is not verified",
		"extranetDTO.locationGroupDTO":              "a ZIA location group used by the extranet; match to a ZIA resource is not verified",
		"extranetDTO.locationGroupDTO.ziaLocations": "a ZIA location used by the extranet; match to zia/locations is not verified",
		"extranetDTO.zpnErId":                       "no extranet resource in the catalog",
	},
	"zpa/service-edge-groups": {
		"enrollmentCertId": "no certificate resource in the catalog",
		"geoLocationId":    "no geolocation resource in the catalog",
		"siteId":           "no site resource in the catalog",
	},
	"zpa/service-edges": {
		"privateBrokerVersion":                         "the service edge's own runtime version record, not a pointer",
		"privateBrokerVersion.brokerId":                "no broker resource in the catalog",
		"privateBrokerVersion.zpnSubModuleUpgradeList": "the service edge's own sub-module upgrade records, not a pointer",
		"issuedCertId":                                 "no certificate resource in the catalog",
		"provisioningKeyId":                            "no provisioning key resource in the catalog",
	},
	"zpa/trusted-networks": {
		"masterCustomerId": "a customer id, not a record id",
		"networkId":        "the record's own network id, not a pointer to another record",
	},
	"zpa/user-portal-links": {
		"applicationId": "the SDK does not say which resource the id belongs to",
	},
	"zpa/user-portals": {
		"certificateId": "no certificate resource in the catalog",
	},
	"zpa/version-profiles": {
		"customScopeCustomerIds":                          "customer ids the profile is scoped to, not records in this tenant",
		"customScopeRequestCustomerIds":                   "customer ids the profile is scoped to, not records in this tenant",
		"customerId":                                      "a customer id, not a record id",
		"customScopeCustomerIds.customerId":               "a customer id, not a record id",
		"customScopeRequestCustomerIds.addCustomerIds":    "customer ids the profile is scoped to, not records in this tenant",
		"customScopeRequestCustomerIds.deleteCustomerIds": "customer ids the profile is scoped to, not records in this tenant",
		"versions":            "the profile's own per-platform version entries, not a pointer",
		"versions.customerId": "a customer id, not a record id",
	},
}
