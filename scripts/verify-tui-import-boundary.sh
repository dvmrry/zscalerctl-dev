#!/usr/bin/env bash
set -euo pipefail

# Enforce the TUI import boundary: Bubble Tea and the Bubble Tea runtime package
# (internal/tui/tea) must not be transitively imported by normal CLI startup
# packages (cmd/zscalerctl, internal/cli) or by the Bubble-free TUI packages
# (internal/tui, internal/tui/data, internal/tui/browserdata, internal/tui/launcher).
# They are allowed only in isolated TUI entrypoints such as cmd/zscalerctl-tui,
# scripts/tui-demo.go, and scripts/tui-browser-demo.go.
#
# This script uses `go list -deps` rather than a textual import scan so that a
# transitive dependency through any intermediate package is caught. The boundary
# exists because Bubble Tea v1.x runs package-init terminal probing (via Lip Gloss
# background detection) that can emit OSC/cursor queries before the CLI has a
# chance to evaluate its own TUI eligibility gate. If any package on the normal
# startup path transitively imports Bubble Tea, that probe may run for every
# zscalerctl invocation, regardless of whether the TUI was requested.

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

module="${ZSCALERCTL_TUI_BUBBLETEA_MODULE:-github.com/charmbracelet/bubbletea}"
tea_pkg="${ZSCALERCTL_TUI_TEA_PKG:-github.com/dvmrry/zscalerctl/internal/tui/tea}"

targets=(
  ./cmd/zscalerctl
  ./internal/cli
  ./internal/tui
  ./internal/tui/data
  ./internal/tui/browserdata
  ./internal/tui/launcher
)

fail=0

for target in "${targets[@]}"; do
  if [[ ! -d "$target" ]]; then
    continue
  fi

  deps=$(go list -deps "$target" 2>/dev/null || true)

  if printf '%s\n' "$deps" | grep -qxF "$module"; then
    echo "forbidden transitive dependency: $target -> $module" >&2
    fail=1
  fi

  if printf '%s\n' "$deps" | grep -qxF "$tea_pkg"; then
    echo "forbidden transitive dependency: $target -> $tea_pkg" >&2
    fail=1
  fi
done

exit "$fail"
