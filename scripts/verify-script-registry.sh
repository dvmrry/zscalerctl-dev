#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

registry="${ZSCALERCTL_SCRIPT_REGISTRY:-docs/SCRIPTS.md}"
scripts_dir="${ZSCALERCTL_SCRIPTS_DIR:-scripts}"

if [[ ! -f "$registry" ]]; then
  echo "missing script registry: $registry" >&2
  exit 1
fi

if [[ ! -d "$scripts_dir" ]]; then
  echo "missing scripts directory: $scripts_dir" >&2
  exit 1
fi

tmp_actual="$(mktemp)"
tmp_registered="$(mktemp)"
tmp_missing="$(mktemp)"
tmp_stale="$(mktemp)"
cleanup() {
  rm -f "$tmp_actual" "$tmp_registered" "$tmp_missing" "$tmp_stale"
}
trap cleanup EXIT

find "$scripts_dir" -maxdepth 1 -type f -exec basename {} \; |
  awk '{ print "scripts/" $0 }' |
  LC_ALL=C sort >"$tmp_actual"
awk -F'`' '/^\| `scripts\// { print $2 }' "$registry" | LC_ALL=C sort -u >"$tmp_registered"

comm -23 "$tmp_actual" "$tmp_registered" >"$tmp_missing"
comm -13 "$tmp_actual" "$tmp_registered" >"$tmp_stale"

fail=0

if [[ -s "$tmp_missing" ]]; then
  echo "scripts missing from registry:" >&2
  sed 's/^/  /' "$tmp_missing" >&2
  fail=1
fi

if [[ -s "$tmp_stale" ]]; then
  echo "registry entries with no matching script:" >&2
  sed 's/^/  /' "$tmp_stale" >&2
  fail=1
fi

if (( fail != 0 )); then
  exit 1
fi
