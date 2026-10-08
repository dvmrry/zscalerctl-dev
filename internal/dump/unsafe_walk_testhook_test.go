//go:build unix && zscalerctl_engine_testhooks

package dump

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// TestInspectDirectoryTreeRejectsFIFOSwapBeforeEnumeration pauses the walk
// after the directory handle is opened and validated, swaps the directory for
// a FIFO, and requires the walk to fail closed. Before the handle-based walk
// this window was covered by fs.WalkDir re-opening the path with a blocking
// open, which stalls forever.
func TestInspectDirectoryTreeRejectsFIFOSwapBeforeEnumeration(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0o700); err != nil {
		t.Fatalf("os.Mkdir(%q) error = %v, want nil", sub, err)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatalf("os.OpenRoot(%q) error = %v, want nil", dir, err)
	}
	defer root.Close()

	reached := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	var releaseOnce sync.Once
	releaseHook := func() { releaseOnce.Do(func() { close(release) }) }
	restore := SetInventoryDirectoryTestHook(func(rootName, path string) {
		if filepath.Join(rootName, filepath.FromSlash(path)) != sub {
			return
		}
		once.Do(func() {
			close(reached)
			<-release
		})
	})
	t.Cleanup(restore)
	t.Cleanup(releaseHook)

	done := make(chan error, 1)
	go func() {
		_, _, err := inspectDirectoryTreeContext(context.Background(), root)
		done <- err
	}()
	select {
	case <-reached:
	case <-time.After(unsafeOpenTestDeadline):
		t.Fatal("inspectDirectoryTreeContext() did not reach the directory test hook")
	}
	if err := os.Rename(sub, sub+".validated"); err != nil {
		t.Fatalf("os.Rename(%q) error = %v, want nil", sub, err)
	}
	if err := unix.Mkfifo(sub, 0o600); err != nil {
		t.Fatalf("unix.Mkfifo(%q) error = %v, want nil", sub, err)
	}
	releaseHook()

	select {
	case err := <-done:
		if err == nil || !errors.Is(err, ErrUnsafePath) {
			t.Fatalf("inspectDirectoryTreeContext() error = %v, want ErrUnsafePath", err)
		}
	case <-time.After(unsafeOpenTestDeadline):
		t.Fatal("inspectDirectoryTreeContext() did not return promptly after the swap")
	}
}

// TestInspectDirectoryTreeCancellationReturnsPromptly checks that cancelling
// the context while the walk is paused still returns promptly once released.
func TestInspectDirectoryTreeCancellationReturnsPromptly(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0o700); err != nil {
		t.Fatalf("os.Mkdir(%q) error = %v, want nil", sub, err)
	}
	if err := os.WriteFile(filepath.Join(sub, "entry.txt"), []byte("x"), 0o600); err != nil {
		t.Fatalf("os.WriteFile(entry.txt) error = %v, want nil", err)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatalf("os.OpenRoot(%q) error = %v, want nil", dir, err)
	}
	defer root.Close()

	reached := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	var releaseOnce sync.Once
	releaseHook := func() { releaseOnce.Do(func() { close(release) }) }
	restore := SetInventoryDirectoryTestHook(func(rootName, path string) {
		if filepath.Join(rootName, filepath.FromSlash(path)) != sub {
			return
		}
		once.Do(func() {
			close(reached)
			<-release
		})
	})
	t.Cleanup(restore)
	t.Cleanup(releaseHook)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, _, err := inspectDirectoryTreeContext(ctx, root)
		done <- err
	}()
	select {
	case <-reached:
	case <-time.After(unsafeOpenTestDeadline):
		t.Fatal("inspectDirectoryTreeContext() did not reach the directory test hook")
	}
	cancel()
	releaseHook()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("inspectDirectoryTreeContext() error = %v, want context.Canceled", err)
		}
	case <-time.After(unsafeOpenTestDeadline):
		t.Fatal("inspectDirectoryTreeContext() did not return promptly after cancellation")
	}
}
