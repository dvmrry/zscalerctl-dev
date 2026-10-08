//go:build !zscalerctl_engine_testhooks

package dump

func runPublicationTestHook(string) error { return nil }

func runInventoryDirectoryTestHook(string, string) {}

func runInventoryAfterReadDirTestHook(string, string) {}
