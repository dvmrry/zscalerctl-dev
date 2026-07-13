package dump

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/dvmrry/zscalerctl/internal/resources"
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

type publishReplacementHooks struct {
	beforeExchange       func()
	beforeQuarantineMove func()
	exchange             func(string, string) (bool, error)
}

func publishReplacingDirectoryContext(
	ctx context.Context,
	stagingDir string,
	dir string,
	allowOwnedDump bool,
) error {
	return publishReplacingDirectoryWithHooks(ctx, stagingDir, dir, allowOwnedDump, publishReplacementHooks{
		exchange: exchangeDirectories,
	})
}

func publishReplacingDirectoryWithHooks(
	ctx context.Context,
	stagingDir string,
	dir string,
	allowOwnedDump bool,
	hooks publishReplacementHooks,
) error {
	ctx = contextOrBackground(ctx)
	if hooks.exchange == nil {
		hooks.exchange = exchangeDirectories
	}
	if allowOwnedDump {
		if err := rejectDangerousForceTargetContext(ctx, dir); err != nil {
			return err
		}
	}
	info, err := os.Lstat(dir)
	if errors.Is(err, os.ErrNotExist) {
		return publishDirectoryNoReplace(stagingDir, dir)
	}
	if err != nil {
		return replacementTargetError(allowOwnedDump, dir, "inspect dump directory", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return replacementTargetError(allowOwnedDump, dir, "target is a symlink", nil)
	}
	target, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return replacementTargetError(allowOwnedDump, dir, "resolve target symlinks", err)
	}
	if allowOwnedDump {
		if err := rejectDangerousForceTargetContext(ctx, target); err != nil {
			return err
		}
	}
	info, err = os.Lstat(target)
	if err != nil {
		return replacementTargetError(allowOwnedDump, dir, "inspect resolved dump directory", err)
	}
	if !info.IsDir() {
		return replacementTargetError(allowOwnedDump, dir, "target is not a directory", nil)
	}
	root, err := os.OpenRoot(target)
	if err != nil {
		return replacementTargetError(allowOwnedDump, dir, "open dump directory", err)
	}
	defer root.Close()
	openedInfo, err := root.Stat(".")
	if err != nil {
		return fmt.Errorf("%w: inspect opened dump directory for --force: %v", ErrUnsafePath, err)
	}
	emptyPlan, hasFiles, err := inspectDirectoryTreeContext(ctx, root)
	if err != nil {
		return err
	}
	if hasFiles && !allowOwnedDump {
		return fmt.Errorf("%w: %s", ErrUnsafeOverwrite, filepath.Join(dir, "manifest.json"))
	}
	cleanupPlan := emptyPlan
	if hasFiles {
		cleanupPlan, err = validateExistingDumpRootContext(ctx, root, target)
		if err != nil {
			return err
		}
	}
	if err := preflightCleanupPlan(root, cleanupPlan); err != nil {
		return err
	}
	if err := preflightCleanupParent(target); err != nil {
		return err
	}
	if err := preflightCleanupParent(target); err != nil {
		return err
	}
	currentInfo, err := os.Lstat(target)
	if err != nil || !os.SameFile(openedInfo, currentInfo) {
		return fmt.Errorf("%w: --force target changed during validation", ErrUnsafePath)
	}

	if err := checkContext(ctx); err != nil {
		return err
	}
	if hooks.beforeExchange != nil {
		hooks.beforeExchange()
	}
	supported, err := hooks.exchange(stagingDir, target)
	if err != nil {
		return fmt.Errorf("atomically replace dump directory: %w", err)
	}
	if !supported {
		return fmt.Errorf("%w: existing destination %s", ErrAtomicReplaceUnsupported, dir)
	}
	swappedInfo, statErr := os.Lstat(stagingDir)
	if statErr != nil || !os.SameFile(openedInfo, swappedInfo) {
		return rollbackExchangedDirectory(
			hooks.exchange,
			stagingDir,
			target,
			fmt.Errorf("%w: --force target changed during atomic replacement", ErrUnsafePath),
		)
	}
	postPlan, postErr := replacementCleanupPlanContext(ctx, root, stagingDir, hasFiles)
	if postErr != nil {
		return rollbackExchangedDirectory(hooks.exchange, stagingDir, target, postErr)
	}
	if err := preflightCleanupPlan(root, postPlan); err != nil {
		return rollbackExchangedDirectory(hooks.exchange, stagingDir, target, err)
	}
	if err := preflightCleanupPlan(root, postPlan); err != nil {
		return rollbackExchangedDirectory(hooks.exchange, stagingDir, target, err)
	}
	rollbackSafe, cleanupErr := removeReplacedDumpRoot(
		stagingDir,
		openedInfo,
		postPlan,
		hooks.beforeQuarantineMove,
	)
	if cleanupErr != nil && rollbackSafe {
		return rollbackExchangedDirectory(hooks.exchange, stagingDir, target, cleanupErr)
	}
	return cleanupErr
}

func replacementTargetError(allowOwnedDump bool, dir, action string, cause error) error {
	if !allowOwnedDump {
		return fmt.Errorf("%w: %s", ErrUnsafeOverwrite, filepath.Join(dir, "manifest.json"))
	}
	if cause != nil {
		return fmt.Errorf("%w: %s for --force: %v", ErrUnsafePath, action, cause)
	}
	return fmt.Errorf("%w: --force %s", ErrUnsafePath, action)
}

func inspectDirectoryTreeContext(
	ctx context.Context,
	root *os.Root,
) (artifactCleanupPlan, bool, error) {
	hasFiles := false
	dirs := map[string]struct{}{".": {}}
	identities := make(map[string]os.FileInfo)
	err := fs.WalkDir(root.FS(), ".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := checkContext(ctx); err != nil {
			return err
		}
		path = filepath.ToSlash(path)
		if entry.IsDir() {
			dirs[path] = struct{}{}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			identities[path] = info
		} else {
			hasFiles = true
		}
		return nil
	})
	if err != nil {
		return artifactCleanupPlan{}, false, fmt.Errorf("inspect existing dump directory: %w", err)
	}
	return newArtifactCleanupPlan(nil, dirs, identities), hasFiles, nil
}

// removeReplacedDumpRoot atomically relocates the entire validated root into a
// new private quarantine before deleting anything. A same-name substitution
// anywhere in the tree is detected after relocation; the whole root is then
// restored without deleting the replacement. The bool result reports whether
// the directory exchange can still be rolled back.
func removeReplacedDumpRoot(
	path string,
	openedInfo os.FileInfo,
	plan artifactCleanupPlan,
	beforeFirstMove func(),
) (bool, error) {
	parent := filepath.Dir(path)
	quarantine, err := os.MkdirTemp(parent, ".zscalerctl-cleanup-*")
	if err != nil {
		return true, fmt.Errorf("create dump cleanup quarantine: %w", err)
	}
	if err := os.Chmod(quarantine, dirPerm); err != nil {
		return true, fmt.Errorf("chmod dump cleanup quarantine: %w", err)
	}
	quarantineInfo, err := os.Lstat(quarantine)
	if err != nil {
		return true, fmt.Errorf("inspect dump cleanup quarantine: %w", err)
	}
	if err := validateAbsoluteCleanupEntry(quarantine, quarantineInfo, true); err != nil {
		return true, fmt.Errorf("validate dump cleanup quarantine: %w", err)
	}

	quarantinedRoot := filepath.Join(quarantine, "root")
	if beforeFirstMove != nil {
		beforeFirstMove()
	}
	if err := renameNoReplace(path, quarantinedRoot); err != nil {
		return true, fmt.Errorf("%w: relocate validated dump root: %v", ErrUnsafePath, err)
	}
	restoreRoot := func() error {
		return renameNoReplace(quarantinedRoot, path)
	}
	movedInfo, err := os.Lstat(quarantinedRoot)
	if err != nil || !os.SameFile(openedInfo, movedInfo) {
		restoreErr := restoreRoot()
		return restoreErr == nil, fmt.Errorf("%w: relocated dump root changed identity; restore: %v", ErrUnsafePath, restoreErr)
	}
	if err := validateQuarantinedCleanupPlan(quarantinedRoot, openedInfo, plan); err != nil {
		restoreErr := restoreRoot()
		return restoreErr == nil, fmt.Errorf("validate relocated dump root: %w; restore: %v", err, restoreErr)
	}
	for _, name := range plan.files {
		entryPath := filepath.Join(quarantinedRoot, filepath.FromSlash(name))
		if err := validateAbsoluteCleanupEntry(entryPath, plan.identities[name], false); err != nil {
			return false, fmt.Errorf("validate quarantined dump file: %w", err)
		}
		if err := os.Remove(entryPath); err != nil {
			return false, fmt.Errorf("remove quarantined dump file: %w", err)
		}
	}
	for _, name := range plan.dirs {
		entryPath := filepath.Join(quarantinedRoot, filepath.FromSlash(name))
		if err := validateAbsoluteCleanupEntry(entryPath, plan.identities[name], true); err != nil {
			return false, fmt.Errorf("validate quarantined dump directory: %w", err)
		}
		if err := os.Remove(entryPath); err != nil {
			return false, fmt.Errorf("remove quarantined dump directory: %w", err)
		}
	}
	if err := validateAbsoluteCleanupEntry(quarantinedRoot, openedInfo, true); err != nil {
		return false, fmt.Errorf("validate empty quarantined dump root: %w", err)
	}
	if err := os.Remove(quarantinedRoot); err != nil {
		return false, fmt.Errorf("remove quarantined dump root: %w", err)
	}
	// Never unlink the public quarantine pathname: a hostile writable parent
	// could substitute that entry after any identity check. Once confidential
	// contents are gone, leaving an empty 0700 directory is harmless.
	return false, nil
}

func validateQuarantinedCleanupPlan(
	root string,
	rootInfo os.FileInfo,
	plan artifactCleanupPlan,
) error {
	if err := validateAbsoluteCleanupEntry(root, rootInfo, true); err != nil {
		return err
	}
	for _, name := range plan.files {
		if err := validateAbsoluteCleanupEntry(
			filepath.Join(root, filepath.FromSlash(name)),
			plan.identities[name],
			false,
		); err != nil {
			return err
		}
	}
	for _, name := range plan.dirs {
		if err := validateAbsoluteCleanupEntry(
			filepath.Join(root, filepath.FromSlash(name)),
			plan.identities[name],
			true,
		); err != nil {
			return err
		}
	}
	return nil
}

func preflightCleanupParent(path string) error {
	parent := filepath.Dir(path)
	info, err := os.Lstat(parent)
	if err != nil {
		return fmt.Errorf("%w: inspect dump parent for cleanup: %v", ErrUnsafePath, err)
	}
	if err := validateAbsoluteCleanupEntry(parent, info, true); err != nil {
		return fmt.Errorf("%w: dump parent cannot support atomic cleanup: %v", ErrUnsafePath, err)
	}
	if info.Mode().Perm()&0o300 != 0o300 {
		return fmt.Errorf("%w: dump parent is not owner-writable", ErrUnsafePath)
	}
	return nil
}

func validateAbsoluteCleanupEntry(path string, want os.FileInfo, wantDirectory bool) error {
	if want == nil {
		return fmt.Errorf("%w: missing cleanup identity", ErrUnsafePath)
	}
	info, err := os.Lstat(path)
	if err != nil || !os.SameFile(want, info) {
		return fmt.Errorf("%w: cleanup path changed", ErrUnsafePath)
	}
	if wantDirectory != info.IsDir() || (!wantDirectory && !info.Mode().IsRegular()) {
		return fmt.Errorf("%w: cleanup path changed type", ErrUnsafePath)
	}
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("%w: open cleanup path: %v", ErrUnsafePath, err)
	}
	openedInfo, statErr := file.Stat()
	if statErr != nil || !os.SameFile(want, openedInfo) {
		_ = file.Close()
		return fmt.Errorf("%w: cleanup path changed while opening", ErrUnsafePath)
	}
	constraintErr := validateRemovalConstraints(file, openedInfo)
	closeErr := file.Close()
	if constraintErr != nil {
		return constraintErr
	}
	return closeErr
}

func replacementCleanupPlanContext(
	ctx context.Context,
	root *os.Root,
	dir string,
	hadFiles bool,
) (artifactCleanupPlan, error) {
	if hadFiles {
		return validateExistingDumpRootContext(ctx, root, dir)
	}
	plan, hasFiles, err := inspectDirectoryTreeContext(ctx, root)
	if err != nil {
		return artifactCleanupPlan{}, err
	}
	if hasFiles {
		return artifactCleanupPlan{}, fmt.Errorf(
			"%w: existing empty dump directory changed before replacement",
			ErrUnsafePath,
		)
	}
	return plan, nil
}

func rollbackExchangedDirectory(
	exchange func(string, string) (bool, error),
	stagingDir string,
	target string,
	cause error,
) error {
	supported, rollbackErr := exchange(stagingDir, target)
	if rollbackErr != nil || !supported {
		return fmt.Errorf("%w; atomic rollback failed: %v", cause, rollbackErr)
	}
	return cause
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
	cleanupPlan, err := validateExistingDumpRootContext(ctx, root, target)
	if err != nil {
		return err
	}
	if err := preflightCleanupPlan(root, cleanupPlan); err != nil {
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
	if err := root.Close(); err != nil {
		return fmt.Errorf("close validated dump root before cleanup: %w", err)
	}
	rollbackSafe, cleanupErr := removeReplacedDumpRoot(target, openedInfo, cleanupPlan, nil)
	if cleanupErr != nil {
		return cleanupErr
	}
	if rollbackSafe {
		return fmt.Errorf("%w: validated dump cleanup did not commit", ErrUnsafePath)
	}
	if err := os.Mkdir(target, dirPerm); err != nil {
		return fmt.Errorf("recreate empty validated dump directory: %w", err)
	}
	recreatedInfo, err := os.Lstat(target)
	if err != nil {
		return fmt.Errorf("inspect recreated dump directory: %w", err)
	}
	if hooks.beforeFinalCheck != nil {
		hooks.beforeFinalCheck()
	}
	currentInfo, err = os.Lstat(target)
	if contextErr := checkContext(ctx); contextErr != nil {
		return contextErr
	}
	if err != nil || !os.SameFile(recreatedInfo, currentInfo) {
		return fmt.Errorf("%w: --force target changed after clearing", ErrUnsafePath)
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

func validateExistingDumpRootContext(
	ctx context.Context,
	root *os.Root,
	dir string,
) (artifactCleanupPlan, error) {
	artifact, err := ValidateArtifactRootContext(ctx, root)
	if err != nil {
		if contextErr := checkContext(ctx); contextErr != nil {
			return artifactCleanupPlan{}, contextErr
		}
		return artifactCleanupPlan{}, fmt.Errorf(
			"%w: --force target %s is not a zscalerctl dump directory",
			ErrUnsafePath,
			dir,
		)
	}
	if artifact.Manifest.Status != "complete" {
		return artifactCleanupPlan{}, fmt.Errorf(
			"%w: --force target %s is not a complete zscalerctl dump directory",
			ErrUnsafePath,
			dir,
		)
	}
	for _, resource := range artifact.Manifest.Resources {
		if resource.Status != "ok" {
			continue
		}
		spec, ok := resources.FindSpec(resources.Product(resource.Product), resource.Name)
		if !ok || resource.Shape != ManifestResourceShape(spec) {
			return artifactCleanupPlan{}, fmt.Errorf(
				"%w: --force target %s does not match the current resource catalog",
				ErrUnsafePath,
				dir,
			)
		}
	}
	return artifact.cleanup, nil
}

func preflightCleanupPlan(root *os.Root, plan artifactCleanupPlan) error {
	for _, dir := range append([]string{"."}, plan.dirs...) {
		info, err := validateCleanupEntry(root, plan, dir, true)
		if err != nil {
			return err
		}
		if info.Mode().Perm()&0o300 != 0o300 {
			return fmt.Errorf("%w: cleanup directory %s is not owner-writable", ErrUnsafePath, dir)
		}
	}
	for _, name := range plan.files {
		if _, err := validateCleanupEntry(root, plan, name, false); err != nil {
			return err
		}
	}
	return nil
}

func validateCleanupEntry(
	root *os.Root,
	plan artifactCleanupPlan,
	name string,
	wantDirectory bool,
) (os.FileInfo, error) {
	want, ok := plan.identities[name]
	if !ok || want == nil {
		return nil, fmt.Errorf("%w: cleanup identity missing for %s", ErrUnsafePath, name)
	}
	info, err := root.Lstat(name)
	if err != nil {
		return nil, fmt.Errorf("%w: inspect cleanup path %s: %v", ErrUnsafePath, name, err)
	}
	if !os.SameFile(want, info) {
		return nil, fmt.Errorf("%w: cleanup path %s changed after validation", ErrUnsafePath, name)
	}
	if wantDirectory != info.IsDir() || (!wantDirectory && !info.Mode().IsRegular()) {
		return nil, fmt.Errorf("%w: cleanup path %s changed type", ErrUnsafePath, name)
	}
	file, err := root.Open(name)
	if err != nil {
		return nil, fmt.Errorf("%w: open cleanup path %s: %v", ErrUnsafePath, name, err)
	}
	openedInfo, statErr := file.Stat()
	if statErr != nil || !os.SameFile(want, openedInfo) {
		_ = file.Close()
		return nil, fmt.Errorf("%w: cleanup path %s changed while opening", ErrUnsafePath, name)
	}
	constraintErr := validateRemovalConstraints(file, openedInfo)
	closeErr := file.Close()
	if constraintErr != nil {
		return nil, fmt.Errorf("%w: cleanup path %s cannot be removed: %v", ErrUnsafePath, name, constraintErr)
	}
	if closeErr != nil {
		return nil, fmt.Errorf("%w: close cleanup path %s: %v", ErrUnsafePath, name, closeErr)
	}
	return openedInfo, nil
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
