#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

module="github.com/charmbracelet/bubbletea"
tea_pkg="github.com/dvmrry/zscalerctl/internal/tui/tea"

for target in ./cmd/zscalerctl ./internal/cli ./internal/tui; do
  if [[ ! -d "$target" ]]; then
    continue
  fi

  deps=$(go list -deps "$target" 2>/dev/null || true)

  if printf '%s\n' "$deps" | grep -qxF "$module"; then
    echo "unexpected transitive dependency: $target -> $module" >&2
    exit 1
  fi

  if printf '%s\n' "$deps" | grep -qxF "$tea_pkg"; then
    echo "unexpected transitive dependency: $target -> $tea_pkg" >&2
    exit 1
  fi
done

# The shell script wrapper must also pass.
bash scripts/verify-tui-import-boundary.sh
