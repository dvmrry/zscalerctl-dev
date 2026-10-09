//go:build unix

package dump

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// withinDeadline fails the test if fn has not returned within a generous
// bound: a blocking open of a FIFO would wait for a writer forever.
func withinDeadline(t *testing.T, fn func() error) error {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- fn() }()
	select {
	case err := <-done:
		return err
	case <-time.After(20 * time.Second):
		t.Fatal("open blocked on a FIFO")
		return nil
	}
}

func TestOpenRootDirectoryRejectsFIFOWithoutBlocking(t *testing.T) {
	dir := t.TempDir()
	fifo := filepath.Join(dir, "fifo")
	if err := unix.Mkfifo(fifo, 0o600); err != nil {
		t.Skipf("mkfifo: %v", err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(fifo, link); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		path   string
		follow bool
	}{{fifo, false}, {fifo, true}, {link, true}, {link, false}} {
		err := withinDeadline(t, func() error {
			root, err := OpenRootDirectory(tc.path, tc.follow)
			if err == nil {
				_ = root.Close()
			}
			return err
		})
		if err == nil {
			t.Errorf("OpenRootDirectory(%s, follow=%t) error = nil, want an error", filepath.Base(tc.path), tc.follow)
		}
	}
}

func TestOpenRootDirectoryRejectsFIFOSubstitutedAfterCheck(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "dump")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	openRootDirectoryTestHook = func(path string) {
		if path != dir {
			return
		}
		if err := os.Remove(dir); err != nil {
			t.Errorf("remove: %v", err)
		}
		if err := unix.Mkfifo(dir, 0o600); err != nil {
			t.Errorf("mkfifo: %v", err)
		}
	}
	t.Cleanup(func() { openRootDirectoryTestHook = nil })

	err := withinDeadline(t, func() error {
		root, err := OpenRootDirectory(dir, false)
		if err == nil {
			_ = root.Close()
		}
		return err
	})
	if err == nil {
		t.Fatal("OpenRootDirectory() error = nil, want an error for the substituted FIFO")
	}
}

func TestOpenRootDirectoryFollowsSymlinksOnlyWhenAsked(t *testing.T) {
	base := t.TempDir()
	target := filepath.Join(base, "target")
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "marker"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(base, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if root, err := OpenRootDirectory(link, false); err == nil {
		_ = root.Close()
		t.Error("OpenRootDirectory(link, follow=false) error = nil, want an error")
	}
	root, err := OpenRootDirectory(link, true)
	if err != nil {
		t.Fatalf("OpenRootDirectory(link, follow=true) error = %v", err)
	}
	defer root.Close()
	if _, err := root.Stat("marker"); err != nil {
		t.Errorf("root.Stat(marker) error = %v, want the target directory", err)
	}
}

// The public artifact validator is one of the six former os.OpenRoot sites.
func TestValidateArtifactContextRejectsSubstitutedFIFO(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "dump")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	openRootDirectoryTestHook = func(path string) {
		if path != dir {
			return
		}
		_ = os.Remove(dir)
		_ = unix.Mkfifo(dir, 0o600)
	}
	t.Cleanup(func() { openRootDirectoryTestHook = nil })

	err := withinDeadline(t, func() error {
		_, err := ValidateArtifactContext(context.Background(), dir)
		return err
	})
	if err == nil {
		t.Fatal("ValidateArtifactContext() error = nil, want an error")
	}
}

// Every directory root in the dump and diff engines goes through
// OpenRootDirectory; a direct os.OpenRoot call can block on a FIFO.
func TestDumpAndDiffAvoidBlockingOpenRoot(t *testing.T) {
	for _, dir := range []string{".", filepath.Join("..", "diff")} {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range entries {
			name := entry.Name()
			if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") || strings.HasPrefix(name, "root_dir_") {
				continue
			}
			data, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil {
				t.Fatal(err)
			}
			for i, line := range strings.Split(string(data), "\n") {
				code, _, _ := strings.Cut(line, "//")
				if strings.Contains(code, "os.OpenRoot(") {
					t.Errorf("%s:%d calls os.OpenRoot; use OpenRootDirectory", filepath.Join(dir, name), i+1)
				}
			}
		}
	}
}
