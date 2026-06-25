#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT

cat >"$tmp_dir/cmd-good.deps" <<'EOF'
github.com/dvmrry/zscalerctl/cmd/zscalerctl
github.com/dvmrry/zscalerctl/internal/browser
github.com/dvmrry/zscalerctl/internal/cli
github.com/charmbracelet/lipgloss
github.com/spf13/cobra
EOF

cat >"$tmp_dir/browser-good.deps" <<'EOF'
github.com/dvmrry/zscalerctl/internal/browser
github.com/dvmrry/zscalerctl/internal/resources
github.com/dvmrry/zscalerctl/internal/redact
EOF

cat >"$tmp_dir/machine-good.deps" <<'EOF'
github.com/dvmrry/zscalerctl/internal/machine
EOF

cat >"$tmp_dir/cmd-bad.deps" <<'EOF'
github.com/dvmrry/zscalerctl/cmd/zscalerctl
github.com/charmbracelet/bubbletea
github.com/dvmrry/zscalerctl/internal/browser
EOF

cat >"$tmp_dir/browser-bad.deps" <<'EOF'
github.com/dvmrry/zscalerctl/internal/browser
github.com/dvmrry/zscalerctl/internal/cli
github.com/charmbracelet/lipgloss
github.com/spf13/cobra
EOF

cat >"$tmp_dir/machine-bad.deps" <<'EOF'
github.com/dvmrry/zscalerctl/internal/machine
github.com/dvmrry/zscalerctl/internal/output
github.com/charmbracelet/lipgloss
github.com/spf13/cobra
EOF

ZSCALERCTL_CMD_DEPS_FILE="$tmp_dir/cmd-good.deps" \
ZSCALERCTL_BROWSER_DEPS_FILE="$tmp_dir/browser-good.deps" \
ZSCALERCTL_MACHINE_DEPS_FILE="$tmp_dir/machine-good.deps" \
  "$repo_root/scripts/verify-core-boundaries.sh" >/dev/null

if ZSCALERCTL_CMD_DEPS_FILE="$tmp_dir/cmd-bad.deps" \
  ZSCALERCTL_BROWSER_DEPS_FILE="$tmp_dir/browser-good.deps" \
  ZSCALERCTL_MACHINE_DEPS_FILE="$tmp_dir/machine-good.deps" \
  "$repo_root/scripts/verify-core-boundaries.sh" >"$tmp_dir/cmd.out" 2>"$tmp_dir/cmd.err"; then
  echo "verify-core-boundaries accepted a CLI dependency on Bubble Tea" >&2
  cat "$tmp_dir/cmd.out" >&2
  cat "$tmp_dir/cmd.err" >&2
  exit 1
fi

if ! grep -q "cmd/zscalerctl imports forbidden dependencies" "$tmp_dir/cmd.err"; then
  echo "verify-core-boundaries failed without the expected CLI boundary message" >&2
  cat "$tmp_dir/cmd.err" >&2
  exit 1
fi

if ZSCALERCTL_CMD_DEPS_FILE="$tmp_dir/cmd-good.deps" \
  ZSCALERCTL_BROWSER_DEPS_FILE="$tmp_dir/browser-bad.deps" \
  ZSCALERCTL_MACHINE_DEPS_FILE="$tmp_dir/machine-good.deps" \
  "$repo_root/scripts/verify-core-boundaries.sh" >"$tmp_dir/browser.out" 2>"$tmp_dir/browser.err"; then
  echo "verify-core-boundaries accepted browser dependencies on CLI/UI packages" >&2
  cat "$tmp_dir/browser.out" >&2
  cat "$tmp_dir/browser.err" >&2
  exit 1
fi

if ZSCALERCTL_CMD_DEPS_FILE="$tmp_dir/cmd-good.deps" \
  ZSCALERCTL_BROWSER_DEPS_FILE="$tmp_dir/browser-good.deps" \
  ZSCALERCTL_MACHINE_DEPS_FILE="$tmp_dir/machine-bad.deps" \
  "$repo_root/scripts/verify-core-boundaries.sh" >"$tmp_dir/machine.out" 2>"$tmp_dir/machine.err"; then
  echo "verify-core-boundaries accepted machine dependencies on CLI/UI/rendering packages" >&2
  cat "$tmp_dir/machine.out" >&2
  cat "$tmp_dir/machine.err" >&2
  exit 1
fi

if ! grep -q "internal/machine imports forbidden dependencies" "$tmp_dir/machine.err"; then
  echo "verify-core-boundaries failed without the expected machine boundary message" >&2
  cat "$tmp_dir/machine.err" >&2
  exit 1
fi

if ! grep -q "internal/browser imports forbidden dependencies" "$tmp_dir/browser.err"; then
  echo "verify-core-boundaries failed without the expected browser boundary message" >&2
  cat "$tmp_dir/browser.err" >&2
  exit 1
fi
