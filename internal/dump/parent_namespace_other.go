//go:build !darwin && !linux

package dump

// Windows dump publication relies on the documented restricted-parent DACL
// model. Existing-directory replacement is unsupported there, so the POSIX
// namespace and sticky-directory checks do not apply.
func validateStablePublicationParent(parent string) (string, error) { return parent, nil }
