//go:build darwin || linux

package dump

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// validateStablePublicationParent establishes the pathname trust boundary used
// by directory publication, rollback, and quarantine moves. The immediate
// parent must be operator-owned and must not allow another principal to rename
// its entries. A writable ancestor is safe only when sticky-directory rules
// protect the next path component, which must also belong to the operator.
func validateStablePublicationParent(parent string) (string, error) {
	abs, err := filepath.Abs(parent)
	if err != nil {
		return "", fmt.Errorf("resolve dump parent: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", fmt.Errorf("resolve dump parent symlinks: %w", err)
	}
	resolved = filepath.Clean(resolved)

	childPath := resolved
	childInfo, err := stableNamespaceEntry(childPath)
	if err != nil {
		return "", err
	}
	if !ownedByCurrentUser(childInfo) {
		return "", errors.New("dump parent is not owned by the current user")
	}
	if childInfo.Mode().Perm()&0o022 != 0 {
		return "", errors.New("dump parent is group- or world-writable")
	}

	for {
		ancestorPath := filepath.Dir(childPath)
		if ancestorPath == childPath {
			return resolved, nil
		}
		ancestorInfo, err := stableNamespaceEntry(ancestorPath)
		if err != nil {
			return "", err
		}
		currentChildInfo, err := stableNamespaceEntry(childPath)
		if err != nil {
			return "", err
		}
		if !os.SameFile(childInfo, currentChildInfo) {
			return "", errors.New("dump parent ancestry changed during validation")
		}
		ancestorOwner, ok := namespaceOwnerUID(ancestorInfo)
		if !ok {
			return "", fmt.Errorf("owner identity for dump ancestor %s is unavailable", ancestorPath)
		}
		currentOwner := int64(os.Geteuid())
		if int64(ancestorOwner) != currentOwner && ancestorOwner != 0 {
			// A directory owner can change its mode and rename children even when
			// its current group/world mode bits are read-only. Only the operator
			// and the privileged system owner are inside this namespace boundary.
			return "", fmt.Errorf("dump ancestor %s is owned by another principal", ancestorPath)
		}
		if ancestorInfo.Mode().Perm()&0o022 != 0 {
			if ancestorInfo.Mode()&os.ModeSticky == 0 {
				return "", fmt.Errorf("writable ancestor %s is not sticky", ancestorPath)
			}
			// In a root-owned sticky directory such as /tmp, another principal
			// cannot rename a child owned by the operator. A current-user-owned
			// ancestor is already within the same-account trust boundary.
			if ancestorOwner == 0 && !ownedByCurrentUser(childInfo) {
				return "", fmt.Errorf("writable ancestor %s does not protect an operator-owned child", ancestorPath)
			}
		}
		childPath = ancestorPath
		childInfo = ancestorInfo
	}
}

func stableNamespaceEntry(path string) (os.FileInfo, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("inspect dump parent ancestry %s: %w", path, err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("dump parent ancestry %s is not a directory", path)
	}
	file, err := os.Open(path) // #nosec G304 -- caller-selected dump ancestor is opened only to bind identity and inspect metadata/ACLs.
	if err != nil {
		return nil, fmt.Errorf("open dump parent ancestry %s: %w", path, err)
	}
	openedInfo, statErr := file.Stat()
	if statErr != nil || !os.SameFile(info, openedInfo) {
		_ = file.Close()
		return nil, fmt.Errorf("dump parent ancestry %s changed while opening", path)
	}
	aclErr := validateStableNamespaceACLFile(file)
	closeErr := file.Close()
	if aclErr != nil {
		return nil, fmt.Errorf("dump parent ancestry %s has unsafe access controls: %w", path, aclErr)
	}
	if closeErr != nil {
		return nil, fmt.Errorf("close dump parent ancestry %s: %w", path, closeErr)
	}
	return openedInfo, nil
}

func ownedByCurrentUser(info os.FileInfo) bool {
	uid, ok := namespaceOwnerUID(info)
	return ok && int64(uid) == int64(os.Geteuid())
}

func namespaceOwnerUID(info os.FileInfo) (uint32, bool) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat == nil {
		return 0, false
	}
	return stat.Uid, true
}
