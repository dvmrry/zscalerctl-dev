package dump

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadArtifactFileRejectsOversizedOpenedHandle(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.json")
	if err := os.WriteFile(path, []byte("{} "), 0o600); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v", path, err)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatalf("os.OpenRoot(%q) error = %v", dir, err)
	}
	defer root.Close()

	_, err = readArtifactFile(context.Background(), root, "manifest.json", 2)
	if !errors.Is(err, ErrInvalidArtifact) || !strings.Contains(err.Error(), "too large") {
		t.Fatalf("readArtifactFile(oversized handle) error = %v, want ErrInvalidArtifact with size context", err)
	}
}

func TestReadArtifactFileRejectsGrowthAfterHandleStat(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "manifest.json")
	if err := os.WriteFile(path, []byte("{}"), 0o600); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v", path, err)
	}
	file, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		t.Fatalf("os.OpenFile(%q) error = %v", path, err)
	}
	defer file.Close()

	handle := &growingArtifactHandle{
		File: file,
		grow: func() error {
			_, err := file.WriteAt([]byte(" "), 2)
			return err
		},
	}
	_, err = readArtifactFileHandle(context.Background(), handle, "manifest.json", 2)
	if !errors.Is(err, ErrInvalidArtifact) || !strings.Contains(err.Error(), "too large") {
		t.Fatalf("readArtifactFileHandle(growing file) error = %v, want ErrInvalidArtifact with size context", err)
	}
}

type growingArtifactHandle struct {
	*os.File
	grow func() error
}

func (handle *growingArtifactHandle) Stat() (os.FileInfo, error) {
	info, err := handle.File.Stat()
	if err != nil {
		return nil, err
	}
	if handle.grow != nil {
		grow := handle.grow
		handle.grow = nil
		if err := grow(); err != nil {
			return nil, err
		}
	}
	return info, nil
}
