package resources

// ZCC reference targets; see fieldReferences in references.go.
var zccFieldReferences = map[string]map[string]referenceTargets{
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
		"appServiceIds":             "predefined-ip-apps and web-app-services both list app services; which one the ids belong to is not verified",
		"bypassAppIds":              "predefined-ip-apps and web-app-services both list app services; which one the ids belong to is not verified",
		"bypassCustomAppIds":        "custom-ip-apps and process-based-apps both list custom apps; which one the ids belong to is not verified",
		"deviceGroupIds":            "no ZCC device group resource in the catalog",
		"deviceGroups":              "no ZCC device group resource in the catalog",
		"disasterRecovery.policyId": "the owning profile's policy id; not verified as a link",
		"groupIds":                  "no ZCC user group resource in the catalog",
		"groups":                    "no ZCC user group resource in the catalog",
		"notificationTemplateId":    "no notification template resource in the catalog",
		"policyExtension.generateCliPasswordContract.policyId":    "the owning profile's policy id; not verified as a link",
		"policyExtension.locationRulesetPolicies.offTrusted":      "a per-network policy override; target resource not verified",
		"policyExtension.locationRulesetPolicies.splitVpnTrusted": "a per-network policy override; target resource not verified",
		"policyExtension.locationRulesetPolicies.trusted":         "a per-network policy override; target resource not verified",
		"policyExtension.locationRulesetPolicies.vpnTrusted":      "a per-network policy override; target resource not verified",
		"userIds":            "no ZCC user resource in the catalog",
		"users":              "no ZCC user resource in the catalog",
		"users.companyId":    "the tenant's own id, not a record id",
		"ziaPostureConfigId": "no ZIA posture config resource in the catalog",
	},
	"zcc/company-info": {
		"devicePostureFrequency.postureId": "no ZCC device posture resource in the catalog",
		"masterCustomerId":                 "the tenant's own id, not a record id",
		"orgId":                            "the tenant's own id, not a record id",
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
