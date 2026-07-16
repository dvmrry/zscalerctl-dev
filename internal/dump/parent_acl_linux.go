//go:build linux

package dump

import "os"

// Linux POSIX ACL write grants are reflected in the group-mode mask checked by
// validateStablePublicationParent.
func validateStableNamespaceACLFile(*os.File) error { return nil }
