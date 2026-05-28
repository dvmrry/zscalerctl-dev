#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT

run_script() {
	local json="$1"
	ZSCALERCTL_PR_JSON="$json" "$repo_root/scripts/pr-labels-for-commit.sh" dvmrry/zscalerctl deadbeef
}

got="$(run_script '[{"labels":[{"name":"semver:minor"},{"name":"security"}]}]')"
want=$'semver:minor\nsecurity'
if [[ "$got" != "$want" ]]; then
	echo "pr-labels-for-commit labels = $got, want $want" >&2
	exit 1
fi

got="$(
	ZSCALERCTL_PR_JSON='[]' \
		ZSCALERCTL_COMMIT_MESSAGE='Fix release PR label extraction (#7)' \
		ZSCALERCTL_PR_VIEW_JSON='{"labels":[{"name":"semver:minor"}]}' \
		"$repo_root/scripts/pr-labels-for-commit.sh" dvmrry/zscalerctl deadbeef
)"
if [[ "$got" != "semver:minor" ]]; then
	echo "pr-labels-for-commit fallback labels = $got, want semver:minor" >&2
	exit 1
fi

if ZSCALERCTL_PR_JSON='[]' \
	ZSCALERCTL_COMMIT_MESSAGE='regular commit without pr number' \
	"$repo_root/scripts/pr-labels-for-commit.sh" dvmrry/zscalerctl deadbeef >"$tmp_dir/out" 2>"$tmp_dir/err"; then
	echo "pr-labels-for-commit accepted commit without PR number, want failure" >&2
	exit 1
fi
if ! grep -q "could not resolve pull request labels" "$tmp_dir/err"; then
	echo "pr-labels-for-commit failed without expected fallback error" >&2
	cat "$tmp_dir/err" >&2
	exit 1
fi

if run_script 'not-json' >"$tmp_dir/out" 2>"$tmp_dir/err"; then
	echo "pr-labels-for-commit accepted invalid JSON, want failure" >&2
	exit 1
fi
