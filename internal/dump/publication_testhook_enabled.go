//go:build zscalerctl_engine_testhooks

package dump

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// rootPaths maps roots opened through OpenRootDirectory, whose Name is a
// descriptor link, back to the path the caller opened, for test hooks.
var rootPaths sync.Map

func registerRootPathForTestHooks(root *os.Root, path string) {
	rootPaths.Store(root, path)
}

func rootPathForTestHooks(root *os.Root) string {
	if path, ok := rootPaths.Load(root); ok {
		return path.(string)
	}
	return root.Name()
}

const publicationTestHookDirEnv = "ZSCALERCTL_ENGINE_TEST_HOOK_DIR"

var inventoryDirectoryTestHook func(rootName, path string)
var inventoryAfterReadDirTestHook func(rootName, path string)

// SetInventoryDirectoryTestHook installs an in-process hook that inventory
// traversal calls after a directory handle is opened and validated and before
// that handle is enumerated. It exists only in test-hook builds; the returned
// function restores the previous hook. Callers must not run admissions
// concurrently with installing or restoring the hook.
func SetInventoryDirectoryTestHook(hook func(rootName, path string)) (restore func()) {
	previous := inventoryDirectoryTestHook
	inventoryDirectoryTestHook = hook
	return func() { inventoryDirectoryTestHook = previous }
}

func runInventoryDirectoryTestHook(rootName, path string) {
	if hook := inventoryDirectoryTestHook; hook != nil {
		hook(rootName, path)
	}
}

// SetInventoryAfterReadDirTestHook installs an in-process hook that artifact
// inventory calls after a directory handle is enumerated and before its entries
// are inspected. It exists only in test-hook builds; the returned function
// restores the previous hook. Callers must not run admissions concurrently with
// installing or restoring the hook.
func SetInventoryAfterReadDirTestHook(hook func(rootName, path string)) (restore func()) {
	previous := inventoryAfterReadDirTestHook
	inventoryAfterReadDirTestHook = hook
	return func() { inventoryAfterReadDirTestHook = previous }
}

func runInventoryAfterReadDirTestHook(rootName, path string) {
	if hook := inventoryAfterReadDirTestHook; hook != nil {
		hook(rootName, path)
	}
}

func runPublicationTestHook(stage string) error {
	dir := os.Getenv(publicationTestHookDirEnv)
	if dir == "" {
		return nil
	}
	if !filepath.IsAbs(dir) || filepath.Clean(dir) != dir {
		return fmt.Errorf("invalid dump publication test-hook directory")
	}
	reached := filepath.Join(dir, stage+".reached")
	if err := os.WriteFile(reached, []byte(stage), filePerm); err != nil {
		return fmt.Errorf("write dump publication test hook: %w", err)
	}
	release := filepath.Join(dir, stage+".release")
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for range ticker.C {
		if _, err := os.Stat(release); err == nil {
			return nil
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("inspect dump publication test hook: %w", err)
		}
	}
	return nil
}
