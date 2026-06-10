#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT

repo="$(mktemp -d "$tmp_dir/repo.XXXXXX")"
cd "$repo"
git init -q
git config user.email "test@example.invalid"
git config user.name "zscalerctl test"
git commit --allow-empty -m initial >/dev/null

assert_next() {
	local bump="$1"
	local want="$2"
	local got
	got="$("$repo_root/scripts/next-version.sh" "$bump")"
	if [[ "$got" != "$want" ]]; then
		echo "next-version.sh $bump = $got, want $want" >&2
		exit 1
	fi
}

assert_next minor v0.1.0
assert_next patch v0.0.1

git tag v0.1.0
assert_next patch v0.1.1
assert_next minor v0.2.0

if "$repo_root/scripts/next-version.sh" major >"$tmp_dir/out" 2>"$tmp_dir/err"; then
	echo "next-version.sh accepted major bump before 1.0" >&2
	exit 1
fi
if ! grep -q "reserved for post-1.0" "$tmp_dir/err"; then
	echo "next-version.sh major failed without expected message" >&2
	cat "$tmp_dir/err" >&2
	exit 1
fi

# The explicit escape hatch the release workflow's major dispatch uses to cut
# v1.0.0 from a 0.x history.
got="$(ZSCALERCTL_ALLOW_MAJOR_ZERO=true "$repo_root/scripts/next-version.sh" major)"
if [[ "$got" != "v1.0.0" ]]; then
	echo "next-version.sh major with ZSCALERCTL_ALLOW_MAJOR_ZERO=true = $got, want v1.0.0" >&2
	exit 1
fi

git tag v1.2.3
assert_next major v2.0.0

none_out="$("$repo_root/scripts/next-version.sh" none)"
if [[ -n "$none_out" ]]; then
	echo "next-version.sh none = $none_out, want empty output" >&2
	exit 1
fi
