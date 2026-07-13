//go:build windows

package dump

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/redact"
)

func TestPublishContextWindowsPublishesNewDestination(t *testing.T) {
	parent := t.TempDir()
	dir := filepath.Join(parent, "dump")
	if err := PublishContext(context.Background(), dir, redact.ModeStandard, Result{}, false); err != nil {
		t.Fatalf("PublishContext(%q, new destination) error = %v, want nil", dir, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "manifest.json")); err != nil {
		t.Fatalf("os.Stat(manifest.json) error = %v, want nil", err)
	}
	assertNoWindowsPublicationLeftovers(t, parent)
}

func TestPublishContextWindowsForceFailsClosedWithoutExchange(t *testing.T) {
	parent := t.TempDir()
	dir := filepath.Join(parent, "dump")
	if err := PublishContext(context.Background(), dir, redact.ModeStandard, Result{}, false); err != nil {
		t.Fatalf("PublishContext(%q, initial) error = %v, want nil", dir, err)
	}
	oldInfo, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("os.Stat(%q) error = %v, want nil", dir, err)
	}

	err = PublishContext(context.Background(), dir, redact.ModeShare, Result{}, true)
	if !errors.Is(err, ErrAtomicReplaceUnsupported) {
		t.Fatalf("PublishContext(%q, force) error = %v, want ErrAtomicReplaceUnsupported", dir, err)
	}
	newInfo, statErr := os.Stat(dir)
	if statErr != nil || !os.SameFile(oldInfo, newInfo) {
		t.Errorf("forced target after unsupported exchange = (%v, %v), want unchanged", newInfo, statErr)
	}
	assertNoWindowsPublicationLeftovers(t, parent)
}

func assertNoWindowsPublicationLeftovers(t *testing.T, parent string) {
	t.Helper()
	for _, pattern := range []string{".zscalerctl-staging-*", ".zscalerctl-replaced-*"} {
		matches, err := filepath.Glob(filepath.Join(parent, pattern))
		if err != nil {
			t.Fatalf("filepath.Glob(%q) error = %v, want nil", pattern, err)
		}
		if len(matches) != 0 {
			t.Errorf("publication leftovers for %q = %v, want none", pattern, matches)
		}
	}
}
