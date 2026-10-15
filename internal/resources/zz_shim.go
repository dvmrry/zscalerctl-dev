package resources

// TEMPORARY shim so the tree compiles while catalog files are reclassified.
func secretField(name string) FieldSpec { return secretWithReason(name, SecretUnstructured) }
func secretFields(names ...string) []FieldSpec {
	out := make([]FieldSpec, 0, len(names))
	for _, n := range names {
		out = append(out, secretField(n))
	}
	return out
}
