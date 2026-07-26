#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
verifier="$repo_root/scripts/verify-node-toolchain.sh"

tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT

make_fixture() {
	local dir="$1"
	mkdir -p "$dir/.github/workflows"
	printf '24.15.0\n' >"$dir/.node-version"
	for workflow in ci release; do
		cat >"$dir/.github/workflows/$workflow.yml" <<'YAML'
name: fixture
on: [push]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/setup-node@example
        with:
          node-version-file: '.node-version'
          package-manager-cache: false
YAML
	done
}

run_verify() {
	local fixture="$1"
	ZSCALERCTL_REPO_ROOT="$fixture" \
	ZSCALERCTL_NODE_VERSION_FILE="$fixture/.node-version" \
	ZSCALERCTL_CI_WORKFLOW="$fixture/.github/workflows/ci.yml" \
	ZSCALERCTL_RELEASE_WORKFLOW="$fixture/.github/workflows/release.yml" \
		"$verifier"
}

good="$tmpdir/good"
make_fixture "$good"
run_verify "$good"

stale_version="$tmpdir/stale-version"
make_fixture "$stale_version"
printf '22.23.1\n' >"$stale_version/.node-version"
if run_verify "$stale_version" >"$tmpdir/stale.out" 2>"$tmpdir/stale.err"; then
	echo "verify-node-toolchain accepted a stale shared Node version" >&2
	exit 1
fi
grep -q 'does not match the reviewed runtime pin' "$tmpdir/stale.err"

multiline_version="$tmpdir/multiline-version"
make_fixture "$multiline_version"
printf '24.15\n.0\n' >"$multiline_version/.node-version"
if run_verify "$multiline_version" >"$tmpdir/multiline.out" 2>"$tmpdir/multiline.err"; then
	echo "verify-node-toolchain accepted a split multiline Node version" >&2
	exit 1
fi
grep -q 'does not match the reviewed runtime pin' "$tmpdir/multiline.err"

missing_release_setup="$tmpdir/missing-release-setup"
make_fixture "$missing_release_setup"
cat >"$missing_release_setup/.github/workflows/release.yml" <<'YAML'
name: release
on: [push]
jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - run: make release-check
YAML
if run_verify "$missing_release_setup" >"$tmpdir/missing.out" 2>"$tmpdir/missing.err"; then
	echo "verify-node-toolchain accepted a release workflow without setup-node" >&2
	exit 1
fi
grep -q 'no setup-node steps found' "$tmpdir/missing.err"

literal_version="$tmpdir/literal-version"
make_fixture "$literal_version"
perl -0pi -e "s/node-version-file: '\\.node-version'/node-version: '24.15.0'/" "$literal_version/.github/workflows/release.yml"
if run_verify "$literal_version" >"$tmpdir/literal.out" 2>"$tmpdir/literal.err"; then
	echo "verify-node-toolchain accepted a duplicated literal release Node version" >&2
	exit 1
fi
grep -q 'missing with.node-version-file' "$tmpdir/literal.err"

wrong_version_file="$tmpdir/wrong-version-file"
make_fixture "$wrong_version_file"
perl -0pi -e 's/node-version-file: '\''\.node-version'\''/node-version-file: '\''.release-node-version'\''/' "$wrong_version_file/.github/workflows/release.yml"
if run_verify "$wrong_version_file" >"$tmpdir/wrong-file.out" 2>"$tmpdir/wrong-file.err"; then
	echo "verify-node-toolchain accepted a divergent release Node version file" >&2
	exit 1
fi
grep -q 'does not match shared Node version file' "$tmpdir/wrong-file.err"

cache_enabled="$tmpdir/cache-enabled"
make_fixture "$cache_enabled"
perl -0pi -e 's/package-manager-cache: false/package-manager-cache: true/' "$cache_enabled/.github/workflows/release.yml"
if run_verify "$cache_enabled" >"$tmpdir/cache.out" 2>"$tmpdir/cache.err"; then
	echo "verify-node-toolchain accepted package-manager caching in the release job" >&2
	exit 1
fi
grep -q 'package-manager-cache must be the literal boolean false' "$tmpdir/cache.err"

comment_spoof="$tmpdir/comment-spoof"
make_fixture "$comment_spoof"
cat >"$comment_spoof/.github/workflows/release.yml" <<'YAML'
name: release
on: [push]
jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      # - uses: actions/setup-node@example
      #   with: {node-version-file: '.node-version', package-manager-cache: false}
      - run: make release-check
YAML
if run_verify "$comment_spoof" >"$tmpdir/comment.out" 2>"$tmpdir/comment.err"; then
	echo "verify-node-toolchain accepted a commented-out release setup-node step" >&2
	exit 1
fi
grep -q 'no setup-node steps found' "$tmpdir/comment.err"
