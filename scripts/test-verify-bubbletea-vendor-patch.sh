#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

init_file="vendor/github.com/charmbracelet/bubbletea/tea_init.go"

# The real repository must satisfy the guard.
bash scripts/verify-bubbletea-vendor-patch.sh

# Negative test: temporarily restore the upstream probe and verify the guard fails.
tmp_backup="$(mktemp)"
trap 'rm -f "$tmp_backup"' EXIT
cp "$init_file" "$tmp_backup"

trap 'cp "$tmp_backup" "$init_file"; rm -f "$tmp_backup"' EXIT

cat >"$init_file" <<'GO'
package tea

import (
	"github.com/charmbracelet/lipgloss"
)

func init() {
	_ = lipgloss.HasDarkBackground()
}
GO

if bash scripts/verify-bubbletea-vendor-patch.sh >/dev/null 2>&1; then
  echo "verify-bubbletea-vendor-patch accepted the upstream HasDarkBackground probe" >&2
  exit 1
fi

# Restore the patched file before the trap runs.
cp "$tmp_backup" "$init_file"
