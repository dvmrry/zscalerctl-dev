package resources

// ZIdentity reference targets; see fieldReferences in references.go.
var zidentityFieldReferences = map[string]map[string][]ResourceRef{
	"zidentity/users": {
		"groups": zidentityRef("groups"),
	},
}

var zidentityUnmappedReferences = map[string]map[string]string{
	"zidentity/groups": {
		"idp": "no identity provider resource in the catalog",
	},
	"zidentity/resource-servers": {
		"serviceScopes.scopes":  "no scope resource in the catalog",
		"serviceScopes.service": "no Zscaler service resource in the catalog",
	},
	"zidentity/users": {
		"department": "no Zidentity department resource in the catalog",
		"idp":        "no identity provider resource in the catalog",
	},
}
