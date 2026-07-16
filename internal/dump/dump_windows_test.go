//go:build windows

package dump

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/fileperm"
	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
	"golang.org/x/sys/windows"
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

func TestPublishContextWindowsInheritsRestrictedParentDACL(t *testing.T) {
	parent := t.TempDir()
	user := os.Getenv("USERNAME")
	if user == "" {
		t.Skip("USERNAME not set; cannot configure restricted parent DACL")
	}
	systemDir, err := windows.GetSystemDirectory()
	if err != nil {
		t.Fatalf("windows.GetSystemDirectory() error = %v, want nil", err)
	}
	icacls := filepath.Join(systemDir, "icacls.exe")
	out, err := exec.Command( // #nosec G204 -- absolute Windows system binary, fixed flags, test temp path and process username.
		icacls,
		parent,
		"/inheritance:r",
		"/grant:r",
		user+":(OI)(CI)F",
	).CombinedOutput()
	if err != nil {
		t.Fatalf("restrict test parent DACL error = %v, want nil; output=%s", err, out)
	}

	dir := filepath.Join(parent, "dump")
	entry := projectedDumpEntry(t, resources.ProductZIA, "locations", []resources.SourceRecord{
		resources.NewSourceRecord(map[string]any{"id": 1, "name": "HQ", "description": ""}),
	})
	result := Result{
		Entries: []ResourceDump{entry},
		Errors: []ResourceError{
			NewResourceError(resources.ProductZIA, "rule-labels", "list", "live_access_failed"),
		},
	}
	if err := PublishContext(context.Background(), dir, redact.ModeStandard, result, false); err != nil {
		t.Fatalf("PublishContext(%q, restricted parent) error = %v, want nil", dir, err)
	}
	for _, path := range []string{
		dir,
		filepath.Join(dir, "resources"),
		filepath.Join(dir, "resources", "zia"),
		filepath.Join(dir, "resources", "zia", "locations.json"),
		filepath.Join(dir, "manifest.json"),
		filepath.Join(dir, "redaction_report.json"),
		filepath.Join(dir, "errors.ndjson"),
	} {
		file, err := fileperm.OpenOwnerOnly(path)
		if err != nil {
			t.Errorf("fileperm.OpenOwnerOnly(%q) error = %v, want nil", path, err)
			continue
		}
		if err := file.Close(); err != nil {
			t.Errorf("Close(%q) error = %v, want nil", path, err)
		}
	}
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
