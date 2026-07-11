package dump

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/redact"
)

func TestPrepareOutputDirPreCanceledPreservesValidDump(t *testing.T) {
	t.Parallel()

	dir := validForceDumpDir(t)
	stalePath := filepath.Join(dir, "stale.txt")
	if err := os.WriteFile(stalePath, []byte("keep"), filePerm); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", stalePath, err)
	}
	manifestBefore := readFile(t, filepath.Join(dir, "manifest.json"))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := PrepareOutputDir(ctx, dir, true)
	if err != context.Canceled {
		t.Fatalf("PrepareOutputDir(%q, pre-canceled) error = %v, want identity %v", dir, err, context.Canceled)
	}
	if got := readFile(t, filepath.Join(dir, "manifest.json")); got != manifestBefore {
		t.Errorf("manifest after pre-canceled PrepareOutputDir = %q, want unchanged %q", got, manifestBefore)
	}
	if got := readFile(t, stalePath); got != "keep" {
		t.Errorf("stale file after pre-canceled PrepareOutputDir = %q, want %q", got, "keep")
	}
}

func TestPrepareOutputDirForceRemovesValidDump(t *testing.T) {
	t.Parallel()

	dir := validForceDumpDir(t)
	stalePath := filepath.Join(dir, "stale.txt")
	if err := os.WriteFile(stalePath, []byte("stale"), filePerm); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", stalePath, err)
	}

	if err := PrepareOutputDir(context.Background(), dir, true); err != nil {
		t.Fatalf("PrepareOutputDir(%q, force) error = %v, want nil", dir, err)
	}
	if _, err := os.Stat(dir); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("os.Stat(%q) error = %v, want os.ErrNotExist", dir, err)
	}
}

func TestPrepareOutputDirDoesNotDeleteReplacementDirectory(t *testing.T) {
	t.Parallel()

	dir := validForceDumpDir(t)
	validatedDir := dir + "-validated"
	replacementFile := filepath.Join(dir, "must-survive.txt")
	var hookErr error

	err := prepareOutputDir(context.Background(), dir, true, func() {
		if renameErr := os.Rename(dir, validatedDir); renameErr != nil {
			hookErr = renameErr
			return
		}
		if mkdirErr := os.Mkdir(dir, dirPerm); mkdirErr != nil {
			hookErr = mkdirErr
			return
		}
		hookErr = os.WriteFile(replacementFile, []byte("keep"), filePerm)
	})
	if hookErr != nil {
		t.Fatalf("replacement boundary setup error = %v, want nil", hookErr)
	}
	if !errors.Is(err, ErrUnsafePath) {
		t.Fatalf("prepareOutputDir(replaced target) error = %v, want ErrUnsafePath", err)
	}
	if got := readFile(t, replacementFile); got != "keep" {
		t.Errorf("replacement file after force race = %q, want %q", got, "keep")
	}
	if info, statErr := os.Stat(dir); statErr != nil || !info.IsDir() {
		t.Errorf("replacement directory after force race = (%v, %v), want existing directory", info, statErr)
	}
}

func TestPrepareOutputDirNoForceDoesNotInspect(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "missing", "dump")
	if err := PrepareOutputDir(context.Background(), dir, false); err != nil {
		t.Fatalf("PrepareOutputDir(%q, no force) error = %v, want nil", dir, err)
	}
	if _, err := os.Stat(dir); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("os.Stat(%q) error = %v, want os.ErrNotExist", dir, err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := PrepareOutputDir(ctx, "", false); err != context.Canceled {
		t.Fatalf("PrepareOutputDir(empty, pre-canceled no force) error = %v, want identity %v", err, context.Canceled)
	}
}

func TestPrepareOutputDirRejectsUnownedDirectory(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "not-a-dump")
	if err := os.Mkdir(dir, dirPerm); err != nil {
		t.Fatalf("os.Mkdir(%q) error = %v, want nil", dir, err)
	}
	notesPath := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(notesPath, []byte("keep"), filePerm); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", notesPath, err)
	}

	err := PrepareOutputDir(context.Background(), dir, true)
	if !errors.Is(err, ErrUnsafePath) {
		t.Fatalf("PrepareOutputDir(%q, unowned) error = %v, want ErrUnsafePath", dir, err)
	}
	if got := readFile(t, notesPath); got != "keep" {
		t.Errorf("unowned directory sentinel = %q, want %q", got, "keep")
	}
}

func TestPrepareOutputDirRejectsFinalSymlink(t *testing.T) {
	t.Parallel()

	target := validForceDumpDir(t)
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("os.Symlink(%q, %q) error = %v; symlinks unavailable", target, link, err)
	}

	err := PrepareOutputDir(context.Background(), link, true)
	if !errors.Is(err, ErrUnsafePath) {
		t.Fatalf("PrepareOutputDir(%q, symlink) error = %v, want ErrUnsafePath", link, err)
	}
	if _, err := os.Stat(filepath.Join(target, "manifest.json")); err != nil {
		t.Errorf("target manifest after rejected symlink = %v, want nil", err)
	}
}

func TestPrepareOutputDirLeavesEmptyDirectoryAlone(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "empty")
	if err := os.Mkdir(dir, dirPerm); err != nil {
		t.Fatalf("os.Mkdir(%q) error = %v, want nil", dir, err)
	}
	if err := PrepareOutputDir(context.Background(), dir, true); err != nil {
		t.Fatalf("PrepareOutputDir(%q, empty) error = %v, want nil", dir, err)
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		t.Errorf("os.Stat(%q) = (%v, %v), want existing directory", dir, info, err)
	}
}

func validForceDumpDir(t *testing.T) string {
	t.Helper()

	dir := filepath.Join(t.TempDir(), "dump")
	if err := Write(dir, redact.ModeStandard, Result{}); err != nil {
		t.Fatalf("Write(%q, empty result) error = %v, want nil", dir, err)
	}
	return dir
}
