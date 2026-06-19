package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestDefaultConfigPathWindowsUsesLocalAppData asserts the Windows platform
// default resolves under %LOCALAPPDATA% (non-roamed, local fixed drive) rather
// than %APPDATA%/Roaming. It injects goos="windows" + an env map so the branch
// is exercised on any host, including this non-Windows CI box.
func TestDefaultConfigPathWindowsUsesLocalAppData(t *testing.T) {
	t.Parallel()

	local := `C:\Users\ops\AppData\Local`
	env := map[string]string{"LOCALAPPDATA": local}
	got := defaultConfigPath("windows", env)
	want := filepath.Join(local, "zscalerctl", "config.yaml")
	if got != want {
		t.Fatalf("defaultConfigPath(windows) = %q, want %q", got, want)
	}
}

// TestDefaultConfigPathWindowsFallsBackWhenLocalAppDataEmpty asserts that with
// LOCALAPPDATA unset the Windows branch falls back to os.UserConfigDir() rather
// than panicking or returning a bare relative path.
func TestDefaultConfigPathWindowsFallsBackWhenLocalAppDataEmpty(t *testing.T) {
	t.Parallel()

	got := defaultConfigPath("windows", map[string]string{})
	if dir, err := os.UserConfigDir(); err == nil && dir != "" {
		want := filepath.Join(dir, "zscalerctl", "config.yaml")
		if got != want {
			t.Fatalf("defaultConfigPath(windows, empty) = %q, want %q", got, want)
		}
		return
	}
	want := filepath.Join(".config", "zscalerctl", "config.yaml")
	if got != want {
		t.Fatalf("defaultConfigPath(windows, empty) = %q, want %q", got, want)
	}
}

// TestDefaultConfigPathNonWindowsIgnoresLocalAppData asserts the non-Windows
// default is unchanged (os.UserConfigDir, e.g. ~/.config) and never consults
// LOCALAPPDATA even when it happens to be set.
func TestDefaultConfigPathNonWindowsIgnoresLocalAppData(t *testing.T) {
	t.Parallel()

	env := map[string]string{"LOCALAPPDATA": `C:\should\be\ignored`}
	got := defaultConfigPath("linux", env)
	if dir, err := os.UserConfigDir(); err == nil && dir != "" {
		want := filepath.Join(dir, "zscalerctl", "config.yaml")
		if got != want {
			t.Fatalf("defaultConfigPath(linux) = %q, want %q", got, want)
		}
		return
	}
	want := filepath.Join(".config", "zscalerctl", "config.yaml")
	if got != want {
		t.Fatalf("defaultConfigPath(linux) = %q, want %q", got, want)
	}
}
