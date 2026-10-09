package dump

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestForceInspectionBoundsTreeDepth accepts an existing dump nested exactly
// maxForceInspectionDepth levels deep and refuses one level more, before
// anything is published.
func TestForceInspectionBoundsTreeDepth(t *testing.T) {
	for _, tc := range []struct {
		levels int
		ok     bool
	}{{maxForceInspectionDepth, true}, {maxForceInspectionDepth + 1, false}} {
		dir := t.TempDir()
		chain := filepath.Join(append([]string{dir}, strings.Split(strings.TrimSuffix(strings.Repeat("d/", tc.levels), "/"), "/")...)...)
		if err := os.MkdirAll(chain, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(chain, "leaf.txt"), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		root, err := os.OpenRoot(dir)
		if err != nil {
			t.Fatal(err)
		}
		_, hasFiles, err := inspectDirectoryTreeContext(context.Background(), root)
		_ = root.Close()
		if tc.ok {
			if err != nil || !hasFiles {
				t.Errorf("inspect %d-level tree = hasFiles %t, error %v; want files and no error", tc.levels, hasFiles, err)
			}
			continue
		}
		if !errors.Is(err, ErrUnsafePath) {
			t.Errorf("inspect %d-level tree error = %v, want ErrUnsafePath", tc.levels, err)
		}
	}
}
