//go:build !zscalerctl_engine_testhooks

package dump

import "os"

func runPublicationTestHook(string) error { return nil }

func registerRootPathForTestHooks(*os.Root, string) {}

func rootPathForTestHooks(root *os.Root) string { return root.Name() }

func runInventoryDirectoryTestHook(string, string) {}

func runInventoryAfterReadDirTestHook(string, string) {}
