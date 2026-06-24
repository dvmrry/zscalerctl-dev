#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT

surface_dir="cmd/zscalerctl/testdata/surface"
manifest="$surface_dir/surface_changes.md"

init_repo() {
	mkdir -p "$1/$surface_dir"
	cd "$1"
	git init -q
	git config user.email "test@example.com"
	git config user.name "Test"
	printf 'old-golden\n' >"$surface_dir/version.stdout.golden"
	printf 'old-manifest\n' >"$manifest"
	git add -A
	git commit -q -m "base"
}

init_repo "$tmp_dir/clean"

# No changes: should pass.
ZSCALERCTL_BASE_REF=HEAD \
	ZSCALERCTL_REPO_ROOT="$tmp_dir/clean" \
	"$repo_root/scripts/verify-surface-changes-manifest.sh"

init_repo "$tmp_dir/golden-only"
cd "$tmp_dir/golden-only"
printf 'new-golden\n' >"$surface_dir/version.stdout.golden"
if ZSCALERCTL_BASE_REF=HEAD \
	ZSCALERCTL_REPO_ROOT="$tmp_dir/golden-only" \
	"$repo_root/scripts/verify-surface-changes-manifest.sh" \
	>"$tmp_dir/out" 2>"$tmp_dir/err"; then
	echo "verify-surface-changes-manifest accepted a golden-only change" >&2
	cat "$tmp_dir/out" >&2
	cat "$tmp_dir/err" >&2
	exit 1
fi
if ! grep -q "surface golden files changed but $manifest was not updated" "$tmp_dir/err"; then
	echo "verify-surface-changes-manifest failed without the expected manifest message" >&2
	cat "$tmp_dir/err" >&2
	exit 1
fi

init_repo "$tmp_dir/golden-and-manifest"
cd "$tmp_dir/golden-and-manifest"
printf 'new-golden\n' >"$surface_dir/version.stdout.golden"
printf 'new-manifest\n' >"$manifest"
ZSCALERCTL_BASE_REF=HEAD \
	ZSCALERCTL_REPO_ROOT="$tmp_dir/golden-and-manifest" \
	"$repo_root/scripts/verify-surface-changes-manifest.sh"

# Only manifest changed: should pass.
init_repo "$tmp_dir/manifest-only"
cd "$tmp_dir/manifest-only"
printf 'new-manifest\n' >"$manifest"
ZSCALERCTL_BASE_REF=HEAD \
	ZSCALERCTL_REPO_ROOT="$tmp_dir/manifest-only" \
	"$repo_root/scripts/verify-surface-changes-manifest.sh"
