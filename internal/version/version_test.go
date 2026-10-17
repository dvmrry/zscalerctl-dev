package version

import (
	"runtime/debug"
	"testing"
)

func TestCurrentUsesInjectedVersion(t *testing.T) {
	oldVersion := version
	t.Cleanup(func() { version = oldVersion })

	version = "v1.2.3"

	if got := Current().Version; got != "1.2.3" {
		t.Errorf("Current().Version = %q, want 1.2.3", got)
	}
}

func TestCurrentDefaultsToDev(t *testing.T) {
	oldVersion := version
	t.Cleanup(func() { version = oldVersion })

	version = "dev"

	if got := Current().Version; got == "" {
		t.Error("Current().Version is empty, want dev or Go module version")
	}
}

func TestCommitAndDateFallBackToStampedVCS(t *testing.T) {
	oldCommit, oldDate := commit, date
	t.Cleanup(func() { commit, date = oldCommit, oldDate })
	commit, date = "unknown", "unknown"

	info := &debug.BuildInfo{Settings: []debug.BuildSetting{
		{Key: "vcs.revision", Value: "0123456789abcdef"},
		{Key: "vcs.time", Value: "2026-01-02T03:04:05Z"},
		{Key: "vcs.modified", Value: "true"},
	}}
	if got := effectiveCommit(info); got != "0123456789abcdef-dirty" {
		t.Errorf("effectiveCommit(stamped, modified) = %q, want revision-dirty", got)
	}
	if got := effectiveDate(info); got != "2026-01-02T03:04:05Z" {
		t.Errorf("effectiveDate(stamped) = %q, want vcs.time", got)
	}
	if got := effectiveCommit(nil); got != "unknown" {
		t.Errorf("effectiveCommit(no build info) = %q, want unknown", got)
	}

	commit, date = "fedcba", "2026-09-09T00:00:00Z"
	if got := effectiveCommit(info); got != "fedcba" {
		t.Errorf("effectiveCommit(injected) = %q, want the injected commit", got)
	}
	if got := effectiveDate(info); got != "2026-09-09T00:00:00Z" {
		t.Errorf("effectiveDate(injected) = %q, want the injected date", got)
	}
}
