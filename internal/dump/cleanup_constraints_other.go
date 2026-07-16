//go:build !darwin && !linux

package dump

import "os"

func validateRemovalConstraints(*os.File, os.FileInfo) error { return nil }
