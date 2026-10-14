package resources

// ZPA reference targets; see fieldReferences in references.go.
var zpaFieldReferences = map[string]map[string]ResourceRef{
	"zpa/app-connector-groups": {
		"connectors":       zpaRef("app-connectors"),
		"microtenantId":    zpaRef("microtenants"),
		"serverGroups":     zpaRef("server-groups"),
		"versionProfileId": zpaRef("version-profiles"),
	},
	"zpa/app-connectors": {
		"appConnectorGroupId": zpaRef("app-connector-groups"),
		"microtenantId":       zpaRef("microtenants"),
	},
	"zpa/app-servers": {
		"appServerGroupIds": zpaRef("server-groups"),
		"microtenantId":     zpaRef("microtenants"),
	},
	"zpa/application-segments": {
		"microtenantId":       zpaRef("microtenants"),
		"segmentGroupId":      zpaRef("segment-groups"),
		"serverGroups":        zpaRef("server-groups"),
		"shareToMicrotenants": zpaRef("microtenants"),
	},
	"zpa/browser-access": {
		"microtenantId":  zpaRef("microtenants"),
		"segmentGroupId": zpaRef("segment-groups"),
		"serverGroups":   zpaRef("server-groups"),
	},
	"zpa/cloud-connector-groups": {
		"cloudConnectors": zpaRef("cloud-connectors"),
	},
	"zpa/inspection-app-segments": {
		"microtenantId":  zpaRef("microtenants"),
		"segmentGroupId": zpaRef("segment-groups"),
		"serverGroups":   zpaRef("server-groups"),
	},
	"zpa/machine-groups": {
		"microtenantId": zpaRef("microtenants"),
	},
	"zpa/pra-app-segments": {
		"microtenantId":  zpaRef("microtenants"),
		"segmentGroupId": zpaRef("segment-groups"),
		"serverGroups":   zpaRef("server-groups"),
	},
	"zpa/segment-groups": {
		"applications":  zpaRef("application-segments"),
		"microtenantId": zpaRef("microtenants"),
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
		"microtenantId":      zpaRef("microtenants"),
		"serviceEdgeGroupId": zpaRef("service-edge-groups"),
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
		"enrollmentCertId": "no certificate resource in the catalog",
		"geoLocationId":    "no geolocation resource in the catalog",
		"siteId":           "no site resource in the catalog",
	},
	"zpa/app-connectors": {
		"issuedCertId":      "no certificate resource in the catalog",
		"provisioningKeyId": "no provisioning key resource in the catalog",
	},
	"zpa/application-segments": {
		"appRecommendationId": "no app recommendation resource in the catalog",
		"zpnErId":             "no extranet resource in the catalog",
	},
	"zpa/branch-connectors": {
		"branchConnectorGroupId": "no branch connector group resource in the catalog",
		"edgeConnectorGroupId":   "which group resource the id belongs to is not verified",
		"issuedCertId":           "no certificate resource in the catalog",
	},
	"zpa/browser-access": {
		"appRecommendationId": "no app recommendation resource in the catalog",
		"zpnErId":             "no extranet resource in the catalog",
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
		"appRecommendationId": "no app recommendation resource in the catalog",
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
		"appRecommendationId": "no app recommendation resource in the catalog",
		"zpnErId":             "no extranet resource in the catalog",
	},
	"zpa/service-edge-groups": {
		"enrollmentCertId": "no certificate resource in the catalog",
		"geoLocationId":    "no geolocation resource in the catalog",
		"siteId":           "no site resource in the catalog",
	},
	"zpa/service-edges": {
		"issuedCertId":      "no certificate resource in the catalog",
		"provisioningKeyId": "no provisioning key resource in the catalog",
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
		"customScopeCustomerIds":        "customer ids the profile is scoped to, not records in this tenant",
		"customScopeRequestCustomerIds": "customer ids the profile is scoped to, not records in this tenant",
		"customerId":                    "a customer id, not a record id",
	},
}
