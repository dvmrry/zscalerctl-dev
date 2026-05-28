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

got="$(run_script '[]')"
if [[ -n "$got" ]]; then
	echo "pr-labels-for-commit empty PR list = $got, want empty" >&2
	exit 1
fi

if run_script 'not-json' >"$tmp_dir/out" 2>"$tmp_dir/err"; then
	echo "pr-labels-for-commit accepted invalid JSON, want failure" >&2
	exit 1
fi
