#!/usr/bin/env bash
set -euo pipefail

# Guard the vendor patch that disables Bubble Tea v1.x package-init terminal
# probing. Bubble Tea's upstream tea_init.go calls lipgloss.HasDarkBackground()
# in init(), which emits OSC/DSR sequences before main() and can hang failure
# paths such as `zscalerctl-tui --live --profile <invalid>`.
#
# This script fails if the patched init probe returns after `go mod vendor` or a
# dependency refresh. It is included in `make check`.

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

init_file="vendor/github.com/charmbracelet/bubbletea/tea_init.go"

if [[ ! -f "$init_file" ]]; then
  echo "missing Bubble Tea init file: $init_file" >&2
  exit 1
fi

if grep -vE '^[[:space:]]*//' "$init_file" | grep -qF "HasDarkBackground()"; then
  echo "Bubble Tea vendor patch missing: $init_file still calls HasDarkBackground() in init()" >&2
  echo "Re-apply the zscalerctl vendor patch that disables the startup terminal probe." >&2
  exit 1
fi

if grep -vE '^[[:space:]]*//' "$init_file" | grep -qE "_[[:space:]]*=[[:space:]]*lipgloss\.HasDarkBackground\(\)"; then
  echo "Bubble Tea vendor patch missing: $init_file still invokes the HasDarkBackground probe" >&2
  exit 1
fi

echo "Bubble Tea vendor patch OK: no startup terminal probe in init()"
