package resources

func catalogZCC() ResourceCatalog {
	return ResourceCatalog{
		{
			Product:    ProductZCC,
			Name:       "fail-open-policy",
			Operations: ListOperations(),
			Fields: []FieldSpec{
				operationalField("id", allModes()),
				operationalField("active", allModes()),
				operationalField("enableFailOpen", allModes()),
				operationalField("enableCaptivePortalDetection", allModes()),
				operationalField("captivePortalWebSecDisableMinutes", allModes()),
				operationalField("enableStrictEnforcementPrompt", allModes()),
				operationalField("strictEnforcementPromptDelayMinutes", allModes()),
				freeTextField("strictEnforcementPromptMessage", "ZCC fail-open strict enforcement prompt message"),
				operationalField("enableWebSecOnProxyUnreachable", allModes()),
				operationalField("enableWebSecOnTunnelFailure", allModes()),
				operationalField("tunnelFailureRetryCount", allModes()),
				operationalField("createdBy", allModes()),
				operationalField("editedBy", allModes()),
				secretField("companyId"),
			},
		},
	}
}
