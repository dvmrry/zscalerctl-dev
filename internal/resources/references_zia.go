package resources

// ZIA reference targets for resources other than rules and policies (those
// are in references_zia_rules.go); see fieldReferences in references.go.
var ziaFieldReferences = map[string]map[string]ResourceRef{
	"zia/admin-users": {
		"role": ziaRef("admin-roles"),
	},
	"zia/browser-control-settings": {
		"smartIsolationGroups":  ziaRef("groups"),
		"smartIsolationProfile": ziaRef("browser-isolation-profiles"),
		"smartIsolationUsers":   ziaRef("users"),
	},
	"zia/cloud-app-control": {
		"cbiProfile":          ziaRef("browser-isolation-profiles"),
		"cloudAppInstances":   ziaRef("cloud-app-instances"),
		"cloudAppRiskProfile": ziaRef("risk-profiles"),
		"departments":         ziaRef("departments"),
		"deviceGroups":        ziaRef("device-groups"),
		"devices":             ziaRef("devices"),
		"groups":              ziaRef("groups"),
		"labels":              ziaRef("rule-labels"),
		"locationGroups":      ziaRef("location-groups"),
		"locations":           ziaRef("locations"),
		"tenancyProfileIds":   ziaRef("tenancy-restriction-profiles"),
		"users":               ziaRef("users"),
	},
	"zia/devices": {
		"ownerUserId": ziaRef("users"),
	},
	"zia/dlp-dictionaries": {
		"dictTemplateId": ziaRef("dlp-dictionaries"),
	},
	"zia/dlp-edm-schemas-lite": {
		"schema": ziaRef("dlp-edm-schemas"),
	},
	"zia/locations": {
		"dynamiclocationGroups": ziaRef("location-groups"),
		"parentId":              ziaRef("locations"),
		"staticLocationGroups":  ziaRef("location-groups"),
		"virtualZenClusters":    ziaRef("vzen-clusters"),
		"virtualZens":           ziaRef("vzen-nodes"),
	},
	"zia/network-service-groups": {
		"services": ziaRef("network-services"),
	},
	"zia/nss-feeds": {
		"dlpDictionaries": ziaRef("dlp-dictionaries"),
		"dlpEngines":      ziaRef("dlp-engines"),
		"nwServices":      ziaRef("network-services"),
	},
	"zia/proxy-gateways": {
		"primaryProxy":   ziaRef("proxies"),
		"secondaryProxy": ziaRef("proxies"),
	},
	"zia/risk-profiles": {
		"modifiedBy": ziaRef("admin-users"),
	},
	"zia/sublocations": {
		"dynamiclocationGroups": ziaRef("location-groups"),
		"parentId":              ziaRef("locations"),
		"staticLocationGroups":  ziaRef("location-groups"),
		"virtualZenClusters":    ziaRef("vzen-clusters"),
		"virtualZens":           ziaRef("vzen-nodes"),
	},
	"zia/tenancy-restriction-profiles": {
		"lastModifiedUserId": ziaRef("admin-users"),
	},
	"zia/users": {
		"department": ziaRef("departments"),
		"groups":     ziaRef("groups"),
	},
	"zia/vzen-clusters": {
		"virtualZenNodes": ziaRef("vzen-nodes"),
	},
}

var ziaUnmappedReferences = map[string]map[string]string{
	"zia/admin-users": {
		"adminScopeScopeEntities":            "target depends on the admin scope type (departments, locations or location groups)",
		"adminScopescopeGroupMemberEntities": "holds location and sublocation ids; zia/locations lists parent locations only",
	},
	"zia/browser-control-settings": {
		"smartIsolationProfileId": "an integer id; browser-isolation-profiles is keyed by UUID",
	},
	"zia/c2c-incident-receivers": {
		"onboardableEntity": "an onboarded SaaS tenant; the match to casb-tenants is not verified",
	},
	"zia/casb-tenants": {
		"enterpriseTenantId": "the SaaS vendor's tenant id, not a catalog record",
		"tenantId":           "the record's own id",
		"zscalerAppTenantId": "target not documented in the SDK",
	},
	"zia/cloud-app-control": {
		"browserEunTemplateId": "no end-user notification template resource in the catalog",
		"eunTemplateId":        "no end-user notification template resource in the catalog",
		"timeWindows":          "time-windows and time-intervals both list rule time ranges; which one the ids belong to is not verified",
		"validityTimeZoneId":   "a time zone name, not a record id",
	},
	"zia/cloud-app-instances": {
		"instanceId": "the record's own id",
	},
	"zia/custom-file-types": {
		"fileTypeId": "a file type id shared with predefined types, not a catalog record",
	},
	"zia/dc-exclusions": {
		"dcName": "no data center resource in the catalog",
	},
	"zia/dedicated-ip-gateways": {
		"primaryDataCenter":   "no data center resource in the catalog",
		"secondaryDataCenter": "no data center resource in the catalog",
	},
	"zia/departments": {
		"idpId": "no identity provider resource in the catalog",
	},
	"zia/dlp-edm-schemas": {
		"edmClient": "no Index Tool resource in the catalog",
		"schemaId":  "the record's own id",
	},
	"zia/dlp-idm-profile-lite": {
		"clientVm":  "no Index Tool resource in the catalog",
		"profileId": "the record's own id",
	},
	"zia/dlp-idm-profiles": {
		"idmClient": "no Index Tool resource in the catalog",
		"profileId": "the record's own id",
	},
	"zia/domain-profiles": {
		"profileId": "the record's own id",
	},
	"zia/eusa-status": {
		"version": "an EUSA version, not a catalog record",
	},
	"zia/gre-tunnels": {
		"primaryDestVip":   "no GRE virtual IP resource in the catalog",
		"secondaryDestVip": "no GRE virtual IP resource in the catalog",
	},
	"zia/groups": {
		"idpId": "no identity provider resource in the catalog",
	},
	"zia/ipv6-config": {
		"natPrefixes": "embedded prefix records; whether they match ipv6-nat64-prefixes is not verified",
	},
	"zia/location-groups": {
		"dynamicLocationGroupCriteria.managedBy": "a predefined SD-WAN partner, not a catalog record",
		"locations":                              "holds location and sublocation ids; zia/locations lists parent locations only",
	},
	"zia/locations": {
		"extranet":       "no extranet resource in the catalog",
		"extranetDns":    "no extranet resource in the catalog",
		"extranetIpPool": "no extranet resource in the catalog",
		"subLocAccIds":   "cloud account ids for the sublocation scope, not catalog records",
	},
	"zia/nss-feeds": {
		"clientId":              "an OAuth client id, not a catalog record",
		"externalCollaborators": "no SaaS collaborator resource in the catalog",
		"externalOwners":        "no SaaS collaborator resource in the catalog",
		"internalCollaborators": "no SaaS collaborator resource in the catalog",
		"itsmObjectType":        "a predefined ITSM object type, not a catalog record",
		"rules":                 "rule type depends on the feed log type",
		"urlCategories":         "integer ids; url-categories is keyed by string id",
	},
	"zia/nss-servers": {
		"icapSvrId": "target not documented in the SDK",
	},
	"zia/org-information": {
		"orgId":       "a tenant id, not a catalog record",
		"zpaTenantId": "a tenant id, not a catalog record",
	},
	"zia/proxies": {
		"cert": "no root certificate resource in the catalog",
	},
	"zia/risk-profiles": {
		"customTags": "no cloud application tag resource in the catalog",
	},
	"zia/static-ips": {
		"city": "a geographic city id, not a catalog record",
	},
	"zia/sub-clouds": {
		"dcs":                   "no data center resource in the catalog",
		"exclusions.datacenter": "no data center resource in the catalog",
	},
	"zia/sublocations": {
		"extranet":       "no extranet resource in the catalog",
		"extranetDns":    "no extranet resource in the catalog",
		"extranetIpPool": "no extranet resource in the catalog",
		"subLocAccIds":   "cloud account ids for the sublocation scope, not catalog records",
	},
	"zia/url-categories": {
		"scopes.ScopeEntities":            "target depends on the admin scope type (departments, locations or location groups)",
		"scopes.scopeGroupMemberEntities": "holds location and sublocation ids; zia/locations lists parent locations only",
	},
	"zia/vzen-nodes": {
		"zgatewayId": "a Zscaler service gateway id, not a catalog record",
	},
	"zia/zpa-gateways": {
		"zpaAppSegments": "the ZPA id is in externalId; id is ZIA-internal",
		"zpaServerGroup": "the ZPA id is in externalId; id is ZIA-internal",
		"zpaTenantId":    "a tenant id, not a catalog record",
	},
}
