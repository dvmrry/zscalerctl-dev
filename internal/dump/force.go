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
// true, removes it only when it is an owned zscalerctl dump directory.
func PrepareOutputDir(ctx context.Context, dir string, force bool) error {
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
	if err := validateExistingDumpDirContext(ctx, target); err != nil {
		return err
	}
	if err := checkContext(ctx); err != nil {
		return err
	}
	// The target was resolved after rejecting a final symlink. If a same-host
	// actor swaps the directory after validation, RemoveAll on a symlink removes
	// the link itself, not its target; the command still refuses cwd/home/root
	// after symlink resolution before reaching this point.
	if err := os.RemoveAll(target); err != nil {
		if contextErr := checkContext(ctx); contextErr != nil {
			return contextErr
		}
		return fmt.Errorf("%w: remove dump directory for --force: %v", ErrUnsafePath, err)
	}
	if err := checkContext(ctx); err != nil {
		return err
	}
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

func validateExistingDumpDirContext(ctx context.Context, dir string) error {
	if err := checkContext(ctx); err != nil {
		return err
	}
	root, err := os.OpenRoot(dir)
	if contextErr := checkContext(ctx); contextErr != nil {
		return contextErr
	}
	if err != nil {
		return fmt.Errorf("%w: open dump directory for --force: %v", ErrUnsafePath, err)
	}
	defer root.Close()

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
