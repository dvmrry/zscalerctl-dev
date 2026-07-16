//go:build zscalerctl_engine_testhooks

package dump

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const publicationTestHookDirEnv = "ZSCALERCTL_ENGINE_TEST_HOOK_DIR"

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
