#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

modules=(
  "charm.land/bubbletea/v2"
  "charm.land/bubbles/v2"
)
tea_pkg="github.com/dvmrry/zscalerctl/internal/tui/tea"

targets=(
  ./cmd/zscalerctl
  ./internal/cli
  ./internal/tui
  ./internal/tui/data
  ./internal/tui/browserdata
  ./internal/tui/launcher
)

for target in "${targets[@]}"; do
  if [[ ! -d "$target" ]]; then
    continue
  fi

  deps=$(go list -deps "$target" 2>/dev/null || true)

  for module in "${modules[@]}"; do
    if printf '%s\n' "$deps" | grep -qxF "$module"; then
      echo "unexpected transitive dependency: $target -> $module" >&2
      exit 1
    fi
  done

  if printf '%s\n' "$deps" | grep -qxF "$tea_pkg"; then
    echo "unexpected transitive dependency: $target -> $tea_pkg" >&2
    exit 1
  fi
done

# The shell script wrapper must also pass on the real repository.
bash scripts/verify-tui-import-boundary.sh

# Negative test: temporarily introduce a forbidden transitive dependency into
# internal/cli and verify that the verifier catches it. This proves the script
# is not just passing because the repository is already clean.
bad_file="internal/cli/zscalerctl_tui_bad_import.go"
cleanup() {
  rm -f "$bad_file"
}
trap cleanup EXIT

cat >"$bad_file" <<'GO'
package cli

import _ "github.com/dvmrry/zscalerctl/internal/tui/tea"
GO

bad_out="$(mktemp)"
bad_err="$(mktemp)"
trap 'rm -f "$bad_file" "$bad_out" "$bad_err"' EXIT

if bash scripts/verify-tui-import-boundary.sh >"$bad_out" 2>"$bad_err"; then
  echo "verify-tui-import-boundary accepted a forbidden transitive dependency" >&2
  exit 1
fi

if ! grep -qF "internal/tui/tea" "$bad_err"; then
  echo "verify-tui-import-boundary did not report the forbidden package" >&2
  cat "$bad_err" >&2
  exit 1
fi

# Clean up the bad file before the trap removes it, so the real repository check
# above is not contaminated.
rm -f "$bad_file"
