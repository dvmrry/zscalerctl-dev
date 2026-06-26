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

ui_runtime_re='github\.com/charmbracelet/(bubbletea|bubbles)|github\.com/wailsapp/wails|vite|react|internal/tui'
cli_rendering_re='github\.com/spf13/cobra|github\.com/charmbracelet/lipgloss|internal/(cli|output)'
raw_runtime_re='internal/(config|credentials|secret|secretref|zscaler)'

check_package \
  "cmd/zscalerctl" \
  "./cmd/zscalerctl" \
  "ZSCALERCTL_CMD_DEPS_FILE" \
  "(^|/)(${ui_runtime_re})(/|$)" \
  "cmd/zscalerctl must remain the normal CLI binary; UI runtimes belong outside this dependency graph."

check_package \
  "internal/browser" \
  "./internal/browser" \
  "ZSCALERCTL_BROWSER_DEPS_FILE" \
  "(^|/)(${ui_runtime_re}|${cli_rendering_re}|${raw_runtime_re})(/|$)" \
  "internal/browser must remain an overlay-facing projected-record seam: no CLI/UI/rendering packages and no raw config, secret, credential, or SDK adapter packages."

check_package \
  "internal/machine" \
  "./internal/machine" \
  "ZSCALERCTL_MACHINE_DEPS_FILE" \
  "(^|/)(${ui_runtime_re}|${cli_rendering_re}|${raw_runtime_re})(/|$)" \
  "internal/machine must remain transport-neutral and projected-record only: no CLI/UI/rendering packages and no raw config, secret, credential, or SDK adapter packages."

check_package \
  "internal/machineio" \
  "./internal/machineio" \
  "ZSCALERCTL_MACHINEIO_DEPS_FILE" \
  "(^|/)(${ui_runtime_re}|${cli_rendering_re}|${raw_runtime_re})(/|$)" \
  "internal/machineio must remain a machine JSON adapter helper: no CLI/UI/rendering packages and no raw config, secret, credential, or SDK adapter packages."

echo "verify-core-boundaries: PASS"
