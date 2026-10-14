package resources

// ZTW reference targets; see fieldReferences in references.go.
var ztwFieldReferences = map[string]map[string]ResourceRef{
	"ztw/account-groups": {
		"cloudConnectorGroups": ztwRef("ec-groups"),
	},
	"ztw/admin-users": {
		"role": ztwRef("admin-roles"),
	},
	"ztw/ec-groups": {
		"location": ztwRef("locations"),
	},
	"ztw/forwarding-rules": {
		"destIpGroups":           ztwRef("ip-destination-groups"),
		"ecGroups":               ztwRef("ec-groups"),
		"locations":              ztwRef("locations"),
		"nwServiceGroups":        ztwRef("network-service-groups"),
		"nwServices":             ztwRef("network-services"),
		"proxyGateway":           ztwRef("forwarding-gateways"),
		"srcIpGroups":            ztwRef("ip-source-groups"),
		"srcWorkloadGroups":      ztwRef("workload-groups"),
		"zpaApplicationSegments": ztwRef("zpa-application-segments"),
	},
	"ztw/locations": {
		"parentId": ztwRef("locations"),
	},
	"ztw/network-service-groups": {
		"services": ztwRef("network-services"),
	},
	"ztw/public-cloud-info": {
		"accountGroups": ztwRef("account-groups"),
	},
	"ztw/traffic-dns-rules": {
		"destIpGroups": ztwRef("ip-destination-groups"),
		"dnsGateway":   ztwRef("dns-gateways"),
		"ecGroups":     ztwRef("ec-groups"),
		"locations":    ztwRef("locations"),
		"srcIpGroups":  ztwRef("ip-source-groups"),
	},
	"ztw/traffic-log-rules": {
		"ecGroups":     ztwRef("ec-groups"),
		"locations":    ztwRef("locations"),
		"proxyGateway": ztwRef("forwarding-gateways"),
	},
}

var ztwUnmappedReferences = map[string]map[string]string{
	"ztw/account-groups": {
		"publicCloudAccounts": "public-cloud-accounts and public-cloud-info both list cloud accounts; which one the ids belong to is not verified",
	},
	"ztw/admin-users": {
		"adminScopeScopeEntities":            "departments, locations or location groups depending on adminScopeType",
		"adminScopescopeGroupMemberEntities": "locations of a location group; ZTW has no location group resource to scope them",
	},
	"ztw/ec-groups": {
		"provTemplate": "no provisioning template resource in the catalog",
	},
	"ztw/forwarding-gateways": {
		"subcloudPrimary":   "not applicable to Cloud & Branch Connector per the SDK",
		"subcloudSecondary": "not applicable to Cloud & Branch Connector per the SDK",
	},
	"ztw/forwarding-rules": {
		"appServiceGroups":            "no app service group resource in the catalog",
		"departments":                 "no ZTW user directory resource in the catalog",
		"destIpv6Groups":              "no ZTW IPv6 group resource in the catalog",
		"groups":                      "no ZTW user directory resource in the catalog",
		"labels":                      "no ZTW rule label resource in the catalog",
		"locationGroups":              "no ZTW location group resource in the catalog",
		"nwApplicationGroups":         "no network application group resource in the catalog",
		"srcIpv6Groups":               "no ZTW IPv6 group resource in the catalog",
		"users":                       "no ZTW user directory resource in the catalog",
		"zpaApplicationSegmentGroups": "no ZPA application segment group resource in the ZTW catalog",
	},
	"ztw/locations": {
		"dynamiclocationGroups": "not applicable to Cloud & Branch Connector per the SDK",
		"publicCloudAccountId":  "public-cloud-accounts and public-cloud-info both list cloud accounts; which one the ids belong to is not verified",
		"staticLocationGroups":  "not applicable to Cloud & Branch Connector per the SDK",
		"virtualZenClusters":    "not applicable to Cloud & Branch Connector per the SDK",
		"virtualZens":           "not applicable to Cloud & Branch Connector per the SDK",
	},
	"ztw/public-cloud-accounts": {
		"accountId":  "a cloud provider account or subscription id, not a record id",
		"platformId": "a cloud platform name (AWS or Azure), not a record id",
	},
	"ztw/public-cloud-info": {
		"externalId":       "an external id issued for the cloud account, not a record id",
		"regionStatus":     "a cloud region id, not a catalog record",
		"supportedRegions": "a cloud region id, not a catalog record",
	},
	"ztw/traffic-dns-rules": {
		"zpaIpGroup":     "SDK comment is copied from proxyGateway; target unverified",
		"locationGroups": "no ZTW location group resource in the catalog",
	},
	"ztw/zpa-application-segments": {
		"zpaId": "the ZPA tenant id per the SDK, not a record id",
	},
}
