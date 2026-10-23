//go:build linux || darwin

package dump

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

const (
	descriptorDepthChildEnv = "ZSCALERCTL_DESCRIPTOR_DEPTH_CHILD"
	descriptorDepthDirEnv   = "ZSCALERCTL_DESCRIPTOR_DEPTH_DIR"
	descriptorDepthLevels   = 96
	descriptorDepthLimit    = 64
)

// TestDirectoryWalksKeepDescriptorsBounded walks a tree deeper than the
// descriptor limit through --force inspection and artifact inventory. A walk
// that keeps every ancestor handle open runs out of descriptors (EMFILE); the
// walks must close each directory before descending, and --force inspection
// must refuse the tree as too deep (ErrUnsafePath) rather than fail with
// EMFILE. The limit is set in a child process so it cannot affect parallel
// tests.
func TestDirectoryWalksKeepDescriptorsBounded(t *testing.T) {
	if os.Getenv(descriptorDepthChildEnv) == "1" {
		runDescriptorDepthChild(t, os.Getenv(descriptorDepthDirEnv))
		return
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "marker.txt"), []byte("marker"), 0o600); err != nil {
		t.Fatalf("os.WriteFile(marker) error = %v", err)
	}
	chain := filepath.Join(append([]string{dir}, strings.Split(strings.Repeat("d/", descriptorDepthLevels), "/")...)...)
	if err := os.MkdirAll(chain, 0o700); err != nil {
		t.Fatalf("os.MkdirAll(%d-level chain) error = %v", descriptorDepthLevels, err)
	}

	cmd := exec.Command(os.Args[0], "-test.run=^TestDirectoryWalksKeepDescriptorsBounded$", "-test.count=1")
	cmd.Env = append(os.Environ(), descriptorDepthChildEnv+"=1", descriptorDepthDirEnv+"="+dir)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("descriptor-limited walk child failed: %v\n%s", err, output)
	}
}

func runDescriptorDepthChild(t *testing.T, dir string) {
	var limit syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &limit); err != nil {
		t.Fatalf("syscall.Getrlimit(RLIMIT_NOFILE) error = %v", err)
	}
	limit.Cur = descriptorDepthLimit
	if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &limit); err != nil {
		t.Fatalf("syscall.Setrlimit(RLIMIT_NOFILE=%d) error = %v", descriptorDepthLimit, err)
	}

	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatalf("os.OpenRoot(%q) error = %v", dir, err)
	}
	defer root.Close()

	if _, _, err := inspectDirectoryTreeContext(context.Background(), root); !errors.Is(err, ErrUnsafePath) ||
		!strings.Contains(err.Error(), "deeper than") {
		t.Fatalf("inspectDirectoryTreeContext(%d-level tree, %d descriptors) error = %v, want a too-deep ErrUnsafePath", descriptorDepthLevels, descriptorDepthLimit, err)
	}

	expectedFiles := map[string]struct{}{"marker.txt": {}}
	expectedDirs := map[string]struct{}{}
	path := ""
	for i := 0; i < descriptorDepthLevels; i++ {
		if path == "" {
			path = "d"
		} else {
			path += "/d"
		}
		expectedDirs[path] = struct{}{}
	}
	if _, err := validateArtifactInventory(context.Background(), root, expectedFiles, expectedDirs); err != nil {
		t.Fatalf("validateArtifactInventory(%d-level tree, %d descriptors) error = %v", descriptorDepthLevels, descriptorDepthLimit, err)
	}
}
