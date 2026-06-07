#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

tmpdir="$(mktemp -d)"
cleanup() {
  rm -rf "$tmpdir"
}
trap cleanup EXIT

scripts_dir="$tmpdir/scripts"
mkdir -p "$scripts_dir"

touch "$scripts_dir/alpha.sh" "$scripts_dir/beta.go"

registry="$tmpdir/SCRIPTS.md"
cat >"$registry" <<'EOF'
# Script Registry

| Script | Category | Called by | Validation |
| --- | --- | --- | --- |
| `scripts/alpha.sh` | verify | test | test |
| `scripts/beta.go` | dev | test | test |
EOF

ZSCALERCTL_SCRIPT_REGISTRY="$registry" \
  ZSCALERCTL_SCRIPTS_DIR="$scripts_dir" \
  bash scripts/verify-script-registry.sh

cat >"$registry" <<'EOF'
# Script Registry

| Script | Category | Called by | Validation |
| --- | --- | --- | --- |
| `scripts/alpha.sh` | verify | test | test |
EOF

missing_out="$tmpdir/missing.out"
if ZSCALERCTL_SCRIPT_REGISTRY="$registry" \
  ZSCALERCTL_SCRIPTS_DIR="$scripts_dir" \
  bash scripts/verify-script-registry.sh >"$missing_out" 2>&1; then
  echo "verify-script-registry succeeded with an unregistered script" >&2
  exit 1
fi

if ! grep -q "scripts/beta.go" "$missing_out"; then
  echo "missing-script failure did not name scripts/beta.go" >&2
  cat "$missing_out" >&2
  exit 1
fi

cat >"$registry" <<'EOF'
# Script Registry

| Script | Category | Called by | Validation |
| --- | --- | --- | --- |
| `scripts/alpha.sh` | verify | test | test |
| `scripts/beta.go` | dev | test | test |
| `scripts/gamma.sh` | stale | test | test |
EOF

stale_out="$tmpdir/stale.out"
if ZSCALERCTL_SCRIPT_REGISTRY="$registry" \
  ZSCALERCTL_SCRIPTS_DIR="$scripts_dir" \
  bash scripts/verify-script-registry.sh >"$stale_out" 2>&1; then
  echo "verify-script-registry succeeded with a stale registry entry" >&2
  exit 1
fi

if ! grep -q "scripts/gamma.sh" "$stale_out"; then
  echo "stale-entry failure did not name scripts/gamma.sh" >&2
  cat "$stale_out" >&2
  exit 1
fi
