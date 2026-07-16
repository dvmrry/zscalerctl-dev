//go:build darwin || linux

package dump

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/redact"
)

func TestPublishContextRejectsWritableParentBeforeStaging(t *testing.T) {
	t.Parallel()

	parent := filepath.Join(t.TempDir(), "shared")
	if err := os.Mkdir(parent, dirPerm); err != nil {
		t.Fatalf("os.Mkdir(%q) error = %v, want nil", parent, err)
	}
	if err := os.Chmod(parent, 0o770); err != nil {
		t.Fatalf("os.Chmod(%q, 0770) error = %v, want nil", parent, err)
	}
	destination := filepath.Join(parent, "dump")

	err := PublishContext(context.Background(), destination, redact.ModeStandard, Result{}, false)
	if !errors.Is(err, ErrUnsafePath) {
		t.Fatalf("PublishContext(%q, writable parent) error = %v, want ErrUnsafePath", destination, err)
	}
	if _, statErr := os.Lstat(destination); !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("os.Lstat(%q) error = %v, want os.ErrNotExist", destination, statErr)
	}
	assertNoPublicationDirectories(t, parent)
}

func TestPublishContextRejectsNonStickyWritableAncestor(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	shared := filepath.Join(root, "shared")
	parent := filepath.Join(shared, "private")
	if err := os.MkdirAll(parent, dirPerm); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v, want nil", parent, err)
	}
	if err := os.Chmod(shared, 0o777); err != nil {
		t.Fatalf("os.Chmod(%q, 0777) error = %v, want nil", shared, err)
	}
	destination := filepath.Join(parent, "dump")

	err := PublishContext(context.Background(), destination, redact.ModeStandard, Result{}, false)
	if !errors.Is(err, ErrUnsafePath) {
		t.Fatalf("PublishContext(%q, writable ancestor) error = %v, want ErrUnsafePath", destination, err)
	}
	if _, statErr := os.Lstat(destination); !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("os.Lstat(%q) error = %v, want os.ErrNotExist", destination, statErr)
	}
	assertNoPublicationDirectories(t, parent)
}

func TestPublishContextAllowsOwnedParentBelowStickyAncestor(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	sticky := filepath.Join(root, "sticky")
	parent := filepath.Join(sticky, "private")
	if err := os.MkdirAll(parent, dirPerm); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v, want nil", parent, err)
	}
	if err := os.Chmod(sticky, os.ModeSticky|0o777); err != nil {
		t.Fatalf("os.Chmod(%q, sticky 0777) error = %v, want nil", sticky, err)
	}
	destination := filepath.Join(parent, "dump")

	if err := PublishContext(context.Background(), destination, redact.ModeStandard, Result{}, false); err != nil {
		t.Fatalf("PublishContext(%q, sticky ancestor) error = %v, want nil", destination, err)
	}
}

func TestPublishContextUsesResolvedParentAfterSymlinkSwap(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	validatedParent := filepath.Join(root, "validated")
	replacementParent := filepath.Join(root, "replacement")
	for _, path := range []string{validatedParent, replacementParent} {
		if err := os.Mkdir(path, dirPerm); err != nil {
			t.Fatalf("os.Mkdir(%q) error = %v, want nil", path, err)
		}
	}
	link := filepath.Join(root, "parent-link")
	if err := os.Symlink(validatedParent, link); err != nil {
		t.Skipf("os.Symlink(%q, %q) error = %v; symlinks unavailable", validatedParent, link, err)
	}
	destination := filepath.Join(link, "dump")
	var hookErr error

	err := publishContextWithHooks(
		context.Background(),
		destination,
		redact.ModeStandard,
		Result{},
		false,
		publishContextHooks{
			afterParentValidation: func() {
				if removeErr := os.Remove(link); removeErr != nil {
					hookErr = removeErr
					return
				}
				hookErr = os.Symlink(replacementParent, link)
			},
		},
	)
	if hookErr != nil {
		t.Fatalf("parent symlink replacement hook error = %v, want nil", hookErr)
	}
	if err != nil {
		t.Fatalf("publishContextWithHooks(%q, parent symlink swap) error = %v, want nil", destination, err)
	}
	if _, statErr := os.Stat(filepath.Join(validatedParent, "dump", "manifest.json")); statErr != nil {
		t.Errorf("validated parent manifest error = %v, want nil", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(replacementParent, "dump")); !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("replacement parent destination error = %v, want os.ErrNotExist", statErr)
	}
}

func TestPublishContextForceRejectsWritableParentBeforeExchange(t *testing.T) {
	t.Parallel()

	parent := filepath.Join(t.TempDir(), "shared")
	if err := os.Mkdir(parent, dirPerm); err != nil {
		t.Fatalf("os.Mkdir(%q) error = %v, want nil", parent, err)
	}
	destination := filepath.Join(parent, "dump")
	if err := Write(destination, redact.ModeStandard, Result{}); err != nil {
		t.Fatalf("Write(%q) error = %v, want nil", destination, err)
	}
	before, err := os.Lstat(destination)
	if err != nil {
		t.Fatalf("os.Lstat(%q) error = %v, want nil", destination, err)
	}
	manifestBefore := readFile(t, filepath.Join(destination, "manifest.json"))
	if err := os.Chmod(parent, 0o770); err != nil {
		t.Fatalf("os.Chmod(%q, 0770) error = %v, want nil", parent, err)
	}

	err = PublishContext(context.Background(), destination, redact.ModeShare, Result{}, true)
	if !errors.Is(err, ErrUnsafePath) {
		t.Fatalf("PublishContext(%q, force in writable parent) error = %v, want ErrUnsafePath", destination, err)
	}
	after, statErr := os.Lstat(destination)
	if statErr != nil || !os.SameFile(before, after) {
		t.Errorf("forced target after rejection = (%v, %v), want original identity", after, statErr)
	}
	if manifestAfter := readFile(t, filepath.Join(destination, "manifest.json")); manifestAfter != manifestBefore {
		t.Errorf("manifest after rejected force changed\n got: %s\nwant: %s", manifestAfter, manifestBefore)
	}
	assertNoPublicationDirectories(t, parent)
}

func assertNoPublicationDirectories(t *testing.T, parent string) {
	t.Helper()
	for _, pattern := range []string{".zscalerctl-staging-*", ".zscalerctl-cleanup-*", ".zscalerctl-discard-*"} {
		matches, err := filepath.Glob(filepath.Join(parent, pattern))
		if err != nil {
			t.Fatalf("filepath.Glob(%q) error = %v, want nil", pattern, err)
		}
		if len(matches) != 0 {
			t.Errorf("publication leftovers for %q = %v, want none", pattern, matches)
		}
	}
}
