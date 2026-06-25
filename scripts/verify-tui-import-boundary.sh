#!/usr/bin/env bash
set -euo pipefail

# Enforce the TUI import boundary: Bubble Tea, Bubbles widgets, and the Bubble
# Tea runtime package (internal/tui/tea) must not be transitively imported by
# normal CLI startup packages (cmd/zscalerctl, internal/cli) or by the
# Bubble-free TUI packages (internal/tui, internal/tui/data,
# internal/tui/browserdata, internal/tui/launcher). They are allowed only in
# isolated TUI entrypoints such as cmd/zscalerctl-tui, scripts/tui-demo.go, and
# scripts/tui-browser-demo.go.
#
# This script uses `go list -deps` rather than a textual import scan so that a
# transitive dependency through any intermediate package is caught. The boundary
# keeps the normal CLI Bubble Tea-free and prevents future TUI runtime side
# effects from leaking into JSON/NDJSON, completion, introspection, or regular
# command startup paths.

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

modules=(
  "${ZSCALERCTL_TUI_BUBBLETEA_MODULE:-charm.land/bubbletea/v2}"
  "${ZSCALERCTL_TUI_BUBBLES_MODULE:-charm.land/bubbles/v2}"
)
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

  for module in "${modules[@]}"; do
    if printf '%s\n' "$deps" | grep -qxF "$module"; then
      echo "forbidden transitive dependency: $target -> $module" >&2
      fail=1
    fi
  done

  if printf '%s\n' "$deps" | grep -qxF "$tea_pkg"; then
    echo "forbidden transitive dependency: $target -> $tea_pkg" >&2
    fail=1
  fi
done

exit "$fail"
