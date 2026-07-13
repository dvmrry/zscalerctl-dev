//go:build darwin

package dump

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/redact"
	"golang.org/x/sys/unix"
)

func TestPublishContextRejectsImmutableArtifactBeforeExchange(t *testing.T) {
	parent := t.TempDir()
	dir := filepath.Join(parent, "dump")
	entry := projectedDumpEntry(t, "zia", "locations", nil)
	if err := Write(dir, redact.ModeStandard, Result{Entries: []ResourceDump{entry}}); err != nil {
		t.Fatalf("Write(%q) error = %v, want nil", dir, err)
	}
	resourcePath := filepath.Join(dir, "resources", "zia", "locations.json")
	if err := unix.Chflags(resourcePath, unix.UF_IMMUTABLE); err != nil {
		t.Fatalf("unix.Chflags(%q, UF_IMMUTABLE) error = %v, want nil", resourcePath, err)
	}
	defer unix.Chflags(resourcePath, 0)
	oldInfo, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("os.Stat(%q) error = %v, want nil", dir, err)
	}
	resourceBefore := readFile(t, resourcePath)

	err = PublishContext(context.Background(), dir, redact.ModeShare, Result{}, true)
	if !errors.Is(err, ErrUnsafePath) {
		t.Fatalf("PublishContext(%q, immutable target) error = %v, want ErrUnsafePath", dir, err)
	}
	newInfo, statErr := os.Stat(dir)
	if statErr != nil || !os.SameFile(oldInfo, newInfo) {
		t.Errorf("target after immutable rejection = (%v, %v), want unchanged", newInfo, statErr)
	}
	if got := readFile(t, resourcePath); got != resourceBefore {
		t.Errorf("immutable resource after rejected replacement changed\n got: %s\nwant: %s", got, resourceBefore)
	}
	matches, globErr := filepath.Glob(filepath.Join(parent, ".zscalerctl-staging-*"))
	if globErr != nil {
		t.Fatalf("filepath.Glob(staging) error = %v, want nil", globErr)
	}
	if len(matches) != 0 {
		t.Errorf("staging directories after immutable rejection = %v, want none", matches)
	}
}

func TestPublishContextRejectsDeleteDenyACLBeforeExchange(t *testing.T) {
	parent := t.TempDir()
	dir := filepath.Join(parent, "dump")
	entry := projectedDumpEntry(t, "zia", "locations", nil)
	if err := Write(dir, redact.ModeStandard, Result{Entries: []ResourceDump{entry}}); err != nil {
		t.Fatalf("Write(%q) error = %v, want nil", dir, err)
	}
	if output, err := exec.Command("chmod", "+a", "everyone deny delete_child", dir).CombinedOutput(); err != nil {
		t.Fatalf("chmod(+a deny delete_child, %q) error = %v, output=%s", dir, err, output)
	}
	defer exec.Command("chmod", "-N", dir).Run()
	oldInfo, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("os.Stat(%q) error = %v, want nil", dir, err)
	}
	manifestBefore := readFile(t, filepath.Join(dir, "manifest.json"))

	err = PublishContext(context.Background(), dir, redact.ModeShare, Result{}, true)
	if !errors.Is(err, ErrUnsafePath) {
		t.Fatalf("PublishContext(%q, delete-deny ACL) error = %v, want ErrUnsafePath", dir, err)
	}
	newInfo, statErr := os.Stat(dir)
	if statErr != nil || !os.SameFile(oldInfo, newInfo) {
		t.Errorf("target after ACL rejection = (%v, %v), want unchanged", newInfo, statErr)
	}
	if got := readFile(t, filepath.Join(dir, "manifest.json")); got != manifestBefore {
		t.Errorf("manifest after rejected ACL replacement changed\n got: %s\nwant: %s", got, manifestBefore)
	}
	matches, globErr := filepath.Glob(filepath.Join(parent, ".zscalerctl-staging-*"))
	if globErr != nil {
		t.Fatalf("filepath.Glob(staging) error = %v, want nil", globErr)
	}
	if len(matches) != 0 {
		t.Errorf("staging directories after ACL rejection = %v, want none", matches)
	}
}

func TestFailedPublicationDoesNotRetainDataInDiscardOnCleanupPreflightFailure(t *testing.T) {
	parent := t.TempDir()
	destination := filepath.Join(parent, "dump")
	if err := Write(destination, redact.ModeStandard, Result{}); err != nil {
		t.Fatalf("Write(%q, existing destination) error = %v, want nil", destination, err)
	}
	var stagingPath string
	var hookErr error
	err := publishContextWithHooks(
		context.Background(),
		destination,
		redact.ModeShare,
		Result{},
		false,
		publishContextHooks{
			beforeStagingCleanupRelocate: func(path string) {
				stagingPath = path
				hookErr = os.Chmod(path, 0o500)
			},
		},
	)
	if hookErr != nil {
		t.Fatalf("staging cleanup preflight hook error = %v, want nil", hookErr)
	}
	if !errors.Is(err, ErrUnsafeOverwrite) {
		t.Fatalf("publishContextWithHooks(existing destination) error = %v, want ErrUnsafeOverwrite", err)
	}
	if stagingPath == "" {
		t.Fatal("staging path = empty, want hook invocation")
	}
	defer os.Chmod(stagingPath, dirPerm)
	if _, statErr := os.Stat(filepath.Join(stagingPath, "manifest.json")); statErr != nil {
		t.Errorf("restored process staging manifest error = %v, want nil", statErr)
	}
	discards, globErr := filepath.Glob(filepath.Join(parent, ".zscalerctl-discard-*"))
	if globErr != nil {
		t.Fatalf("filepath.Glob(discard) error = %v, want nil", globErr)
	}
	if len(discards) != 1 {
		t.Fatalf("discard directories = %v, want one empty quarantine", discards)
	}
	entries, readErr := os.ReadDir(discards[0])
	if readErr != nil {
		t.Fatalf("os.ReadDir(%q) error = %v, want nil", discards[0], readErr)
	}
	if len(entries) != 0 {
		t.Errorf("discard directory entries = %v, want empty", entries)
	}
}
