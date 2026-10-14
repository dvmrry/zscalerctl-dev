package resources

// ZCC reference targets; see fieldReferences in references.go.
var zccFieldReferences = map[string]map[string]ResourceRef{
	"zcc/application-profiles": {
		"forwardingProfileId": zccRef("forwarding-profiles"),
	},
	"zcc/forwarding-profiles": {
		"trustedNetworkIds": zccRef("trusted-networks"),
	},
}

var zccUnmappedReferences = map[string]map[string]string{
	"zcc/admin-roles": {
		"companyId": "the tenant's own id, not a record id",
	},
	"zcc/application-profiles": {
		"appServiceIds":          "predefined-ip-apps and web-app-services both list app services; which one the ids belong to is not verified",
		"bypassAppIds":           "predefined-ip-apps and web-app-services both list app services; which one the ids belong to is not verified",
		"bypassCustomAppIds":     "custom-ip-apps and process-based-apps both list custom apps; which one the ids belong to is not verified",
		"deviceGroupIds":         "no ZCC device group resource in the catalog",
		"groupIds":               "no ZCC user group resource in the catalog",
		"notificationTemplateId": "no notification template resource in the catalog",
		"userIds":                "no ZCC user resource in the catalog",
		"ziaPostureConfigId":     "no ZIA posture config resource in the catalog",
	},
	"zcc/company-info": {
		"masterCustomerId": "the tenant's own id, not a record id",
		"orgId":            "the tenant's own id, not a record id",
	},
	"zcc/fail-open-policy": {
		"companyId": "the tenant's own id, not a record id",
	},
	"zcc/predefined-ip-apps": {
		"appSvcId": "an app service identifier with no verified catalog target",
	},
	"zcc/trusted-networks": {
		"companyId": "the tenant's own id, not a record id",
	},
	"zcc/web-app-services": {
		"appSvcId": "an app service identifier with no verified catalog target",
	},
}
