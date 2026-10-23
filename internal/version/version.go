package version

import (
	"runtime"
	"runtime/debug"
	"strings"
)

var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

type Info struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
	Date    string `json:"date"`
	Go      string `json:"go"`
	OS      string `json:"os"`
	Arch    string `json:"arch"`
}

func Current() Info {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		info = nil
	}
	return Info{
		Version: effectiveVersion(),
		Commit:  effectiveCommit(info),
		Date:    effectiveDate(info),
		Go:      runtime.Version(),
		OS:      runtime.GOOS,
		Arch:    runtime.GOARCH,
	}
}

func (Info) OutputSafe() {}

func effectiveVersion() string {
	if version != "" && version != "dev" {
		return strings.TrimPrefix(version, "v")
	}
	info, ok := debug.ReadBuildInfo()
	if !ok || info.Main.Version == "" || info.Main.Version == "(devel)" {
		if version == "" {
			return "dev"
		}
		return version
	}
	return strings.TrimPrefix(info.Main.Version, "v")
}

// effectiveCommit is the release build's injected commit, else the revision Go
// stamped from the checkout, marked "-dirty" for uncommitted changes. Go does
// not stamp builds from a linked git worktree, so those stay "unknown".
func effectiveCommit(info *debug.BuildInfo) string {
	if commit != "" && commit != "unknown" {
		return commit
	}
	revision := buildSetting(info, "vcs.revision")
	if revision == "" {
		return "unknown"
	}
	if buildSetting(info, "vcs.modified") == "true" {
		return revision + "-dirty"
	}
	return revision
}

// effectiveDate is the release build's injected date, else the commit time Go
// stamped from the checkout.
func effectiveDate(info *debug.BuildInfo) string {
	if date != "" && date != "unknown" {
		return date
	}
	if when := buildSetting(info, "vcs.time"); when != "" {
		return when
	}
	return "unknown"
}

func buildSetting(info *debug.BuildInfo, key string) string {
	if info == nil {
		return ""
	}
	for _, setting := range info.Settings {
		if setting.Key == key {
			return setting.Value
		}
	}
	return ""
}
