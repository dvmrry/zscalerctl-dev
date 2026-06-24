#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

bubbletea_dir="vendor/charm.land/bubbletea/v2"
init_file="$bubbletea_dir/zscalerctl_bad_init_test.go"

# The real repository must satisfy the guard.
bash scripts/verify-bubbletea-vendor-patch.sh

# Negative test: temporarily introduce a startup probe and verify the guard fails.
cleanup() {
  rm -f "$init_file"
}
trap cleanup EXIT

cat >"$init_file" <<'GO'
package tea

import (
	"os"

	"charm.land/lipgloss/v2"
)

func init() {
	_ = lipgloss.HasDarkBackground(os.Stdin, os.Stdout)
}
GO

if bash scripts/verify-bubbletea-vendor-patch.sh >/dev/null 2>&1; then
  echo "verify-bubbletea-vendor-patch accepted a startup HasDarkBackground probe" >&2
  exit 1
fi
