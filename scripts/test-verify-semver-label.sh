#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT

run_good() {
	local labels="$1"
	ZSCALERCTL_PR_LABELS="$labels" "$repo_root/scripts/verify-semver-label.sh"
}

run_bad() {
	local labels="$1"
	local want="$2"
	if ZSCALERCTL_PR_LABELS="$labels" "$repo_root/scripts/verify-semver-label.sh" >"$tmp_dir/out" 2>"$tmp_dir/err"; then
		echo "verify-semver-label accepted labels: $labels" >&2
		exit 1
	fi
	if ! grep -q "$want" "$tmp_dir/err"; then
		echo "verify-semver-label failed without expected message: $want" >&2
		cat "$tmp_dir/err" >&2
		exit 1
	fi
}

# verify-semver-label.sh reads the surrounding repo's tags for its major-label
# guard, so run every case from a temp repo with a controlled tag history.
# Without this isolation the pre-1.0 expectations below would invert once the
# zscalerctl repo itself gains a v1.x tag.
repo="$(mktemp -d "$tmp_dir/repo.XXXXXX")"
cd "$repo"
git init -q
git config user.email "test@example.invalid"
git config user.name "zscalerctl test"
git commit --allow-empty -m initial >/dev/null
git tag v0.1.0

run_good "dependencies,semver:patch"
run_good "semver:minor"
run_good "semver:none"

run_bad "" "exactly one semver label"
run_bad "semver:patch,semver:minor" "exactly one semver label"
run_bad "semver:major" "reserved for post-1.0"

# The explicit escape hatch the release workflow's major dispatch uses to cut
# v1.0.0 from a 0.x history.
(
	export ZSCALERCTL_ALLOW_MAJOR_ZERO=true
	run_good "semver:major"
)

git tag v1.0.0
run_good "semver:major"
