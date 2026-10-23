package resources

// applyFieldTypes sets FieldSpec.Type from generatedFieldTypes, which
// TestFieldTypesAreCurrent in internal/zscaler derives from the SDK models.
func applyFieldTypes(c ResourceCatalog) {
	for i := range c {
		prefix := specKey(c[i]) + "/"
		applyFieldTypesAt(c[i].Fields, prefix)
	}
}

func applyFieldTypesAt(fields []FieldSpec, prefix string) {
	for i := range fields {
		path := prefix + fields[i].JSONField()
		fields[i].Type = generatedFieldTypes[path]
		applyFieldTypesAt(fields[i].Fields, path+".")
	}
}
