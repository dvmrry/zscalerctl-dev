#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT

mkdir -p "$tmp_dir/cmd" "$tmp_dir/internal/cli" "$tmp_dir/internal/tui" "$tmp_dir/internal/tui/tea"

cat >"$tmp_dir/cmd/main.go" <<'GO'
package main
import "os"
func main() { os.Exit(0) }
GO

cat >"$tmp_dir/internal/cli/root.go" <<'GO'
package cli
func Root() {}
GO

cat >"$tmp_dir/internal/tui/gate.go" <<'GO'
package tui
func Evaluate() bool { return true }
GO

cat >"$tmp_dir/internal/tui/tea/model.go" <<'GO'
package tea
import "github.com/charmbracelet/bubbletea"
func New() bubbletea.Model { return nil }
GO

ZSCALERCTL_TUI_CMD_DIR="$tmp_dir/cmd" \
ZSCALERCTL_TUI_CLI_DIR="$tmp_dir/internal/cli" \
ZSCALERCTL_TUI_GATE_DIR="$tmp_dir/internal/tui" \
ZSCALERCTL_TUI_TEA_DIR="$tmp_dir/internal/tui/tea" \
  bash scripts/verify-tui-import-boundary.sh

cat >"$tmp_dir/internal/tui/gate_bad.go" <<'GO'
package tui
import _ "github.com/charmbracelet/bubbletea"
func Bad() {}
GO

bad_err="$tmp_dir/bad.err"
if ZSCALERCTL_TUI_CMD_DIR="$tmp_dir/cmd" \
ZSCALERCTL_TUI_CLI_DIR="$tmp_dir/internal/cli" \
ZSCALERCTL_TUI_GATE_DIR="$tmp_dir/internal/tui" \
ZSCALERCTL_TUI_TEA_DIR="$tmp_dir/internal/tui/tea" \
  bash scripts/verify-tui-import-boundary.sh >"$tmp_dir/bad.out" 2>"$bad_err"; then
  echo "verify-tui-import-boundary accepted a forbidden Bubble Tea import" >&2
  exit 1
fi

if ! grep -qF "gate_bad.go" "$bad_err"; then
  echo "verify-tui-import-boundary did not report the bad file" >&2
  cat "$bad_err" >&2
  exit 1
fi

# The real repository must also satisfy the boundary.
bash scripts/verify-tui-import-boundary.sh
