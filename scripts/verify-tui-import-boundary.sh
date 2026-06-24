#!/usr/bin/env bash
set -euo pipefail

# Enforce the TUI import boundary: Bubble Tea must not be imported by normal
# CLI startup packages (cmd/, internal/cli/) or by the gate-only internal/tui
# package. It is allowed only in the demo/runtime package internal/tui/tea and
# in the demo entry point scripts/tui-demo.go.
#
# The boundary exists because Bubble Tea v1.x runs package-init terminal
# probing (via Lip Gloss background detection) that can emit OSC/cursor queries
# before the CLI has a chance to evaluate its own TUI eligibility gate. If any
# package on the normal startup path imports Bubble Tea, that probe may run for
# every zscalerctl invocation, regardless of whether the TUI was requested.

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

module="${ZSCALERCTL_TUI_BUBBLETEA_MODULE:-github.com/charmbracelet/bubbletea}"
cmd_dir="${ZSCALERCTL_TUI_CMD_DIR:-cmd}"
cli_dir="${ZSCALERCTL_TUI_CLI_DIR:-internal/cli}"
gate_dir="${ZSCALERCTL_TUI_GATE_DIR:-internal/tui}"
tea_dir="${ZSCALERCTL_TUI_TEA_DIR:-internal/tui/tea}"

fail=0

check_dir() {
  local dir="$1"
  if [[ ! -d "$dir" ]]; then
    return
  fi

  local abs_tea_dir=""
  if [[ -d "$tea_dir" ]]; then
    abs_tea_dir="$(cd "$tea_dir" && pwd)"
  fi

  while IFS= read -r -d '' file; do
    local abs_file
    abs_file="$(cd "$(dirname "$file")" && pwd)/$(basename "$file")"
    if [[ -n "$abs_tea_dir" && "$abs_file" == "$abs_tea_dir"* ]]; then
      continue
    fi
    if grep -nE "^[[:space:]]*(import[[:space:]]+([a-zA-Z_][a-zA-Z0-9_]*[[:space:]]+)?|[a-zA-Z_][a-zA-Z0-9_]*[[:space:]]+)?\"$module\"" "$file" >/dev/null; then
      echo "forbidden Bubble Tea import: $file" >&2
      fail=1
    fi
  done < <(find "$dir" -name '*.go' -print0)
}

check_dir "$cmd_dir"
check_dir "$cli_dir"
check_dir "$gate_dir"

exit "$fail"
