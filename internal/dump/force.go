package dump

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// PrepareOutputDir validates an existing output directory and, when force is
// true, clears it only when it is an owned zscalerctl dump directory.
func PrepareOutputDir(ctx context.Context, dir string, force bool) error {
	return prepareOutputDir(ctx, dir, force, prepareOutputDirHooks{})
}

type prepareOutputDirHooks struct {
	beforeClear      func()
	beforeFinalCheck func()
}

// prepareOutputDir accepts per-call test-only boundary callbacks around the
// destructive phase. This avoids mutable package hooks in production and makes
// replacement-race tests deterministic.
func prepareOutputDir(ctx context.Context, dir string, force bool, hooks prepareOutputDirHooks) error {
	ctx = contextOrBackground(ctx)
	if err := checkContext(ctx); err != nil {
		return err
	}
	if !force {
		return nil
	}
	if strings.TrimSpace(dir) == "" {
		return fmt.Errorf("%w: missing dump directory", ErrUnsafePath)
	}
	if err := rejectDangerousForceTargetContext(ctx, dir); err != nil {
		return err
	}
	if err := checkContext(ctx); err != nil {
		return err
	}
	info, err := os.Lstat(dir)
	if contextErr := checkContext(ctx); contextErr != nil {
		return contextErr
	}
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("%w: inspect dump directory for --force: %v", ErrUnsafePath, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%w: --force target %s is a symlink", ErrUnsafePath, dir)
	}
	if err := checkContext(ctx); err != nil {
		return err
	}
	target, err := filepath.EvalSymlinks(dir)
	if contextErr := checkContext(ctx); contextErr != nil {
		return contextErr
	}
	if err != nil {
		return fmt.Errorf("%w: resolve --force target symlinks: %v", ErrUnsafePath, err)
	}
	if err := rejectDangerousForceTargetContext(ctx, target); err != nil {
		return err
	}
	if err := checkContext(ctx); err != nil {
		return err
	}
	info, err = os.Lstat(target)
	if contextErr := checkContext(ctx); contextErr != nil {
		return contextErr
	}
	if err != nil {
		return fmt.Errorf("%w: inspect resolved dump directory for --force: %v", ErrUnsafePath, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%w: --force target %s is not a directory", ErrUnsafePath, dir)
	}
	empty, err := isDirEmptyContext(ctx, target)
	if err != nil {
		return err
	}
	if empty {
		return nil
	}
	root, err := os.OpenRoot(target)
	if contextErr := checkContext(ctx); contextErr != nil {
		if root != nil {
			_ = root.Close()
		}
		return contextErr
	}
	if err != nil {
		return fmt.Errorf("%w: open dump directory for --force: %v", ErrUnsafePath, err)
	}
	defer root.Close()

	openedInfo, err := root.Stat(".")
	if contextErr := checkContext(ctx); contextErr != nil {
		return contextErr
	}
	if err != nil {
		return fmt.Errorf("%w: inspect opened dump directory for --force: %v", ErrUnsafePath, err)
	}
	if !openedInfo.IsDir() {
		return fmt.Errorf("%w: --force target %s is not a directory", ErrUnsafePath, dir)
	}
	if err := validateExistingDumpRootContext(ctx, root, target); err != nil {
		return err
	}
	if err := checkContext(ctx); err != nil {
		return err
	}
	currentInfo, err := os.Lstat(target)
	if contextErr := checkContext(ctx); contextErr != nil {
		return contextErr
	}
	if err != nil || !os.SameFile(openedInfo, currentInfo) {
		return fmt.Errorf("%w: --force target changed during validation", ErrUnsafePath)
	}
	if hooks.beforeClear != nil {
		hooks.beforeClear()
	}
	// Clear through the still-open directory root, not through target's path.
	// On supported desktop/server platforms the root remains bound to the
	// validated directory if that path is concurrently renamed or replaced, so
	// an unvalidated replacement can never be recursively deleted.
	if err := clearDumpRootContext(ctx, root); err != nil {
		return err
	}
	if err := checkContext(ctx); err != nil {
		return err
	}
	if hooks.beforeFinalCheck != nil {
		hooks.beforeFinalCheck()
	}
	currentInfo, err = os.Lstat(target)
	if contextErr := checkContext(ctx); contextErr != nil {
		return contextErr
	}
	if err != nil || !os.SameFile(openedInfo, currentInfo) {
		return fmt.Errorf("%w: --force target changed after clearing", ErrUnsafePath)
	}
	// Leave the identity-checked directory in place for WriteContext to reuse.
	// Removing even an empty pathname would reopen a final substitution race in
	// which an unvalidated regular file could be deleted after the check.
	return nil
}

func rejectDangerousForceTargetContext(ctx context.Context, dir string) error {
	if err := checkContext(ctx); err != nil {
		return err
	}
	abs, err := filepath.Abs(dir)
	if contextErr := checkContext(ctx); contextErr != nil {
		return contextErr
	}
	if err != nil {
		return fmt.Errorf("%w: resolve --force target: %v", ErrUnsafePath, err)
	}
	clean := filepath.Clean(abs)
	if err := checkContext(ctx); err != nil {
		return err
	}
	cwd, err := os.Getwd()
	if contextErr := checkContext(ctx); contextErr != nil {
		return contextErr
	}
	if err != nil {
		return fmt.Errorf("%w: resolve current directory: %v", ErrUnsafePath, err)
	}
	if clean == filepath.Clean(cwd) {
		return fmt.Errorf("%w: --force target cannot be the current directory", ErrUnsafePath)
	}
	if filepath.Dir(clean) == clean {
		return fmt.Errorf("%w: --force target cannot be the filesystem root", ErrUnsafePath)
	}
	if err := checkContext(ctx); err != nil {
		return err
	}
	home, err := os.UserHomeDir()
	if contextErr := checkContext(ctx); contextErr != nil {
		return contextErr
	}
	if err == nil && home != "" && clean == filepath.Clean(home) {
		return fmt.Errorf("%w: --force target cannot be the home directory", ErrUnsafePath)
	}
	return nil
}

func isDirEmptyContext(ctx context.Context, dir string) (bool, error) {
	if err := checkContext(ctx); err != nil {
		return false, err
	}
	entries, err := os.ReadDir(dir)
	if contextErr := checkContext(ctx); contextErr != nil {
		return false, contextErr
	}
	if err != nil {
		return false, fmt.Errorf("%w: inspect dump directory for --force: %v", ErrUnsafePath, err)
	}
	return len(entries) == 0, nil
}

func validateExistingDumpRootContext(ctx context.Context, root *os.Root, dir string) error {
	if err := checkContext(ctx); err != nil {
		return err
	}
	info, err := root.Lstat("manifest.json")
	if contextErr := checkContext(ctx); contextErr != nil {
		return contextErr
	}
	if errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("%w: --force target %s is not a zscalerctl dump directory", ErrUnsafePath, dir)
	}
	if err != nil {
		return fmt.Errorf("%w: inspect dump manifest for --force: %v", ErrUnsafePath, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%w: --force target manifest is a symlink", ErrUnsafePath)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%w: --force target manifest is not a regular file", ErrUnsafePath)
	}
	if info.Size() > 1<<20 {
		return fmt.Errorf("%w: --force target manifest is too large", ErrUnsafePath)
	}
	if err := checkContext(ctx); err != nil {
		return err
	}
	body, err := root.ReadFile("manifest.json")
	if contextErr := checkContext(ctx); contextErr != nil {
		return contextErr
	}
	if err != nil {
		return fmt.Errorf("%w: read dump manifest for --force: %v", ErrUnsafePath, err)
	}
	if err := checkContext(ctx); err != nil {
		return err
	}
	var manifest struct {
		Schema string `json:"schema"`
	}
	if err := json.Unmarshal(body, &manifest); err != nil {
		return fmt.Errorf("%w: --force target %s is not a zscalerctl dump directory", ErrUnsafePath, dir)
	}
	if err := checkContext(ctx); err != nil {
		return err
	}
	if !strings.HasPrefix(manifest.Schema, "zscalerctl.dump.manifest.") {
		return fmt.Errorf("%w: --force target %s is not a zscalerctl dump directory", ErrUnsafePath, dir)
	}
	return nil
}

func clearDumpRootContext(ctx context.Context, root *os.Root) error {
	if err := checkContext(ctx); err != nil {
		return err
	}
	dir, err := root.Open(".")
	if err != nil {
		return fmt.Errorf("%w: open dump directory contents for --force: %v", ErrUnsafePath, err)
	}
	names, readErr := dir.Readdirnames(-1)
	closeErr := dir.Close()
	if readErr != nil {
		return fmt.Errorf("%w: read dump directory contents for --force: %v", ErrUnsafePath, readErr)
	}
	if closeErr != nil {
		return fmt.Errorf("%w: close dump directory contents for --force: %v", ErrUnsafePath, closeErr)
	}
	if err := checkContext(ctx); err != nil {
		return err
	}
	// Once clearing begins it is intentionally uninterruptible between root
	// entries. Returning on cancellation mid-phase could strand a partially
	// deleted artifact; cancellation is checked immediately before and after.
	for _, name := range names {
		if err := root.RemoveAll(name); err != nil {
			return fmt.Errorf("%w: clear dump directory for --force: %v", ErrUnsafePath, err)
		}
	}
	if err := checkContext(ctx); err != nil {
		return err
	}
	return nil
}
