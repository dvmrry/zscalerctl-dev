package resources

func catalogZidentity() ResourceCatalog {
	return ResourceCatalog{
		{
			Product:    ProductZidentity,
			Name:       "groups",
			Operations: ReadOperations(),
			Fields: []FieldSpec{
				operationalField("id", allModes()),
				tenantConfigField("name", standardShareModes()),
				freeTextField("description", "Zidentity group description"),
				tenantConfigField("source", standardShareModes()),
				operationalField("isDynamicGroup", allModes()),
				operationalField("dynamicGroup", allModes()),
				tenantConfigField("adminEntitlementEnabled", standardShareModes()),
				tenantConfigField("serviceEntitlementEnabled", standardShareModes()),
				idNameDisplayNameField("idp", standardShareModes()),
			},
		},
		{
			Product:    ProductZidentity,
			Name:       "users",
			Operations: ReadOperations(),
			Fields: []FieldSpec{
				operationalField("id", allModes()),
				tenantConfigField("source", standardShareModes()),
				sensitiveIdentifierField("loginName"),
				sensitiveIdentifierField("displayName"),
				sensitiveIdentifierField("firstName"),
				sensitiveIdentifierField("lastName"),
				sensitiveIdentifierField("primaryEmail"),
				sensitiveIdentifierField("secondaryEmail"),
				operationalField("status", allModes()),
				idNameDisplayNameField("department", standardShareModes()),
				idNameDisplayNameField("idp", standardShareModes()),
				secretField("customAttrsInfo"),
			},
		},
		{
			Product:    ProductZidentity,
			Name:       "resource-servers",
			Operations: ReadOperations(),
			Fields: []FieldSpec{
				operationalField("id", allModes()),
				tenantConfigField("name", standardShareModes()),
				tenantConfigField("displayName", standardShareModes()),
				freeTextField("description", "Zidentity resource server description"),
				sensitiveIdentifierField("primaryAud"),
				operationalField("defaultApi", allModes()),
				{
					Name:           "serviceScopes",
					Classification: ClassTenantConfig,
					AllowedModes:   standardOnlyMode(),
					Fields: []FieldSpec{
						{
							Name:           "service",
							Classification: ClassTenantConfig,
							AllowedModes:   standardOnlyMode(),
							Fields: []FieldSpec{
								operationalField("id", allModes()),
								tenantConfigField("name", standardShareModes()),
								tenantConfigField("displayName", standardShareModes()),
							},
						},
						{
							Name:           "scopes",
							Classification: ClassTenantConfig,
							AllowedModes:   standardOnlyMode(),
							Fields: []FieldSpec{
								operationalField("id", allModes()),
								tenantConfigField("name", standardShareModes()),
							},
						},
					},
				},
			},
		},
	}
}
