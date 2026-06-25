#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT

check_package() {
  local label="$1"
  local package="$2"
  local deps_file_env="$3"
  local forbidden_re="$4"
  local guidance="$5"
  local deps_file="$tmp_dir/${label//[^A-Za-z0-9]/_}.deps"
  local matches

  if [[ -n "${!deps_file_env:-}" ]]; then
    cat "${!deps_file_env}" >"$deps_file"
  else
    go list -deps -mod=vendor "$package" >"$deps_file"
  fi

  matches="$(grep -E "$forbidden_re" "$deps_file" || true)"
  if [[ -n "$matches" ]]; then
    echo "verify-core-boundaries: $label imports forbidden dependencies:" >&2
    sed 's/^/  /' <<<"$matches" >&2
    echo "$guidance" >&2
    exit 1
  fi
}

check_package \
  "cmd/zscalerctl" \
  "./cmd/zscalerctl" \
  "ZSCALERCTL_CMD_DEPS_FILE" \
  '(^|/)(github\.com/charmbracelet/(bubbletea|bubbles)|github\.com/wailsapp/wails|internal/tui|vite|react)(/|$)' \
  "cmd/zscalerctl must remain the normal CLI binary; UI runtimes belong outside this dependency graph."

check_package \
  "internal/browser" \
  "./internal/browser" \
  "ZSCALERCTL_BROWSER_DEPS_FILE" \
  '(^|/)(github\.com/spf13/cobra|github\.com/charmbracelet/(bubbletea|bubbles|lipgloss)|github\.com/wailsapp/wails|internal/cli)(/|$)' \
  "internal/browser must remain UI-agnostic and Cobra-free; future UI code should consume it without reversing the dependency direction."

echo "verify-core-boundaries: PASS"
