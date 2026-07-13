package dump

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/dvmrry/zscalerctl/internal/resources"
)

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
	openedInfo, err := openRootEntryInfo(root, ".")
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
	stagedRoot, err := os.OpenRoot(stagingDir)
	if err != nil {
		return fmt.Errorf("%w: open staged dump directory: %v", ErrUnsafePath, err)
	}
	defer stagedRoot.Close()
	stagedInfo, err := openRootEntryInfo(stagedRoot, ".")
	if err != nil {
		return fmt.Errorf("%w: inspect staged dump directory: %v", ErrUnsafePath, err)
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
	publishedInfo, publishedStatErr := os.Lstat(target)
	if statErr != nil || !os.SameFile(openedInfo, swappedInfo) ||
		publishedStatErr != nil || !os.SameFile(stagedInfo, publishedInfo) {
		return fmt.Errorf("%w: replacement paths changed during atomic exchange; rollback refused", ErrUnsafePath)
	}
	postPlan, postErr := replacementCleanupPlanContext(ctx, root, stagingDir, hasFiles)
	if postErr != nil {
		return rollbackExchangedDirectory(
			hooks.exchange,
			stagingDir,
			target,
			openedInfo,
			stagedInfo,
			postErr,
		)
	}
	if err := preflightCleanupPlan(root, postPlan); err != nil {
		return rollbackExchangedDirectory(
			hooks.exchange,
			stagingDir,
			target,
			openedInfo,
			stagedInfo,
			err,
		)
	}
	rollbackSafe, cleanupErr := removeReplacedDumpRoot(
		ctx,
		root,
		stagingDir,
		openedInfo,
		hasFiles,
		hooks.beforeQuarantineMove,
	)
	if cleanupErr != nil && rollbackSafe {
		return rollbackExchangedDirectory(
			hooks.exchange,
			stagingDir,
			target,
			openedInfo,
			stagedInfo,
			cleanupErr,
		)
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
			pathInfo, err := entry.Info()
			if err != nil {
				return err
			}
			openedInfo, err := openRootEntryInfo(root, path)
			if err != nil {
				return err
			}
			if !os.SameFile(pathInfo, openedInfo) {
				return fmt.Errorf("%w: existing dump directory changed during inspection", ErrUnsafePath)
			}
			identities[path] = openedInfo
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
// new private quarantine before deleting anything. The root is fully
// revalidated after relocation, so a pre-move insertion or substitution is
// restored without deleting any content. The bool result reports whether the
// directory exchange can still be rolled back.
func removeReplacedDumpRoot(
	ctx context.Context,
	root *os.Root,
	path string,
	openedInfo os.FileInfo,
	hadFiles bool,
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
	plan, err := replacementCleanupPlanContext(ctx, root, quarantinedRoot, hadFiles)
	if err != nil {
		restoreErr := restoreRoot()
		return restoreErr == nil, fmt.Errorf("validate relocated dump root: %w; restore: %v", err, restoreErr)
	}
	if err := preflightCleanupPlan(root, plan); err != nil {
		restoreErr := restoreRoot()
		return restoreErr == nil, fmt.Errorf("preflight relocated dump root: %w; restore: %v", err, restoreErr)
	}
	// The quarantine is owner-private and has already been validated for ACL and
	// removal constraints. Other OS principals cannot traverse it; processes
	// running as the same account are part of the operator trust boundary.
	if err := clearDumpRootContext(context.Background(), root); err != nil {
		return false, fmt.Errorf("clear quarantined dump root: %w", err)
	}
	// Never unlink either public quarantine pathname: a hostile writable parent
	// could substitute an ancestor after any identity check. Once confidential
	// contents are gone, leaving an empty 0700 directory skeleton is harmless.
	return false, nil
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
	if _, err := validateStablePublicationParent(parent); err != nil {
		return fmt.Errorf("%w: dump parent namespace is not stable: %v", ErrUnsafePath, err)
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
	expectedStaging os.FileInfo,
	expectedTarget os.FileInfo,
	cause error,
) error {
	stagingInfo, stagingErr := os.Lstat(stagingDir)
	targetInfo, targetErr := os.Lstat(target)
	if stagingErr != nil || !os.SameFile(expectedStaging, stagingInfo) ||
		targetErr != nil || !os.SameFile(expectedTarget, targetInfo) {
		return fmt.Errorf("%w; atomic rollback refused because a replacement path changed", cause)
	}
	supported, rollbackErr := exchange(stagingDir, target)
	if rollbackErr != nil || !supported {
		return fmt.Errorf("%w; atomic rollback failed: %v", cause, rollbackErr)
	}
	restoredTarget, restoredTargetErr := os.Lstat(target)
	restoredStaging, restoredStagingErr := os.Lstat(stagingDir)
	if restoredTargetErr != nil || !os.SameFile(expectedStaging, restoredTarget) ||
		restoredStagingErr != nil || !os.SameFile(expectedTarget, restoredStaging) {
		return fmt.Errorf("%w; atomic rollback produced unexpected path identities", cause)
	}
	return cause
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
