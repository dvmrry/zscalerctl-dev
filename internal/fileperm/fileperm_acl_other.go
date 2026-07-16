//go:build !darwin

package fileperm

import "os"

func validateOwnerOnlyACLPath(string) error {
	return nil
}

func validateOwnerOnlyACLFile(*os.File) error {
	return nil
}
