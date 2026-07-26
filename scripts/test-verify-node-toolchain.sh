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
	cat >"$dir/.github/workflows/ci.yml" <<'YAML'
name: ci
on: [push]
jobs:
  typescript-client:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/setup-node@example
        with:
          node-version-file: '.node-version'
          package-manager-cache: false
      - run: bash scripts/verify-typescript-client.sh
  verify-gates:
    runs-on: ubuntu-latest
    steps:
      - run: make verify-node-toolchain
YAML
	cat >"$dir/.github/workflows/release.yml" <<'YAML'
name: release
on: [push]
jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/setup-node@example
        with:
          node-version-file: '.node-version'
          package-manager-cache: false
      - run: make release-check
YAML
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
grep -q 'no direct setup-node steps found' "$tmpdir/missing.err"

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

override_version="$tmpdir/override-version"
make_fixture "$override_version"
perl -0pi -e "s/node-version-file: '\.node-version'/node-version-file: '.node-version'\n          node-version: '22.23.1'/" "$override_version/.github/workflows/release.yml"
if run_verify "$override_version" >"$tmpdir/override.out" 2>"$tmpdir/override.err"; then
	echo "verify-node-toolchain accepted node-version overriding node-version-file" >&2
	exit 1
fi
grep -q 'node-version must not override the shared node-version-file' "$tmpdir/override.err"

conditional_setup="$tmpdir/conditional-setup"
make_fixture "$conditional_setup"
perl -0pi -e 's/- uses: actions\/setup-node@example/- uses: actions\/setup-node@example\n        if: false/' "$conditional_setup/.github/workflows/release.yml"
if run_verify "$conditional_setup" >"$tmpdir/conditional-setup.out" 2>"$tmpdir/conditional-setup.err"; then
	echo "verify-node-toolchain accepted a conditional release setup-node step" >&2
	exit 1
fi
grep -q 'setup-node must be unconditional' "$tmpdir/conditional-setup.err"

ignored_setup_failure="$tmpdir/ignored-setup-failure"
make_fixture "$ignored_setup_failure"
perl -0pi -e 's/- uses: actions\/setup-node@example/- uses: actions\/setup-node@example\n        continue-on-error: true/' "$ignored_setup_failure/.github/workflows/release.yml"
if run_verify "$ignored_setup_failure" >"$tmpdir/ignored-setup-failure.out" 2>"$tmpdir/ignored-setup-failure.err"; then
	echo "verify-node-toolchain accepted release setup-node with continue-on-error" >&2
	exit 1
fi
grep -q 'setup-node must not set continue-on-error' "$tmpdir/ignored-setup-failure.err"

late_setup="$tmpdir/late-setup"
make_fixture "$late_setup"
cat >"$late_setup/.github/workflows/release.yml" <<'YAML'
name: release
on: [push]
jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - run: make release-check
      - uses: actions/setup-node@example
        with:
          node-version-file: '.node-version'
          package-manager-cache: false
YAML
if run_verify "$late_setup" >"$tmpdir/late-setup.out" 2>"$tmpdir/late-setup.err"; then
	echo "verify-node-toolchain accepted setup-node after the release consumer" >&2
	exit 1
fi
grep -q 'must follow a valid unconditional direct setup-node step in the same job' "$tmpdir/late-setup.err"

different_job="$tmpdir/different-job"
make_fixture "$different_job"
cat >"$different_job/.github/workflows/release.yml" <<'YAML'
name: release
on: [push]
jobs:
  setup:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/setup-node@example
        with:
          node-version-file: '.node-version'
          package-manager-cache: false
  release:
    runs-on: ubuntu-latest
    steps:
      - run: make release-check
YAML
if run_verify "$different_job" >"$tmpdir/different-job.out" 2>"$tmpdir/different-job.err"; then
	echo "verify-node-toolchain accepted setup-node in a different job from the release consumer" >&2
	exit 1
fi
grep -q 'must follow a valid unconditional direct setup-node step in the same job' "$tmpdir/different-job.err"

conditional_release_job="$tmpdir/conditional-release-job"
make_fixture "$conditional_release_job"
perl -0pi -e 's/  release:\n    runs-on:/  release:\n    if: false\n    runs-on:/' "$conditional_release_job/.github/workflows/release.yml"
if run_verify "$conditional_release_job" >"$tmpdir/conditional-release-job.out" 2>"$tmpdir/conditional-release-job.err"; then
	echo "verify-node-toolchain accepted a conditional release job" >&2
	exit 1
fi
grep -q 'job.*release.*containing Node policy steps must be unconditional' "$tmpdir/conditional-release-job.err"

ignored_consumer_failure="$tmpdir/ignored-consumer-failure"
make_fixture "$ignored_consumer_failure"
perl -0pi -e 's/- run: make release-check/- run: make release-check\n        continue-on-error: true/' "$ignored_consumer_failure/.github/workflows/release.yml"
if run_verify "$ignored_consumer_failure" >"$tmpdir/ignored-consumer-failure.out" 2>"$tmpdir/ignored-consumer-failure.err"; then
	echo "verify-node-toolchain accepted release-check with continue-on-error" >&2
	exit 1
fi
grep -q 'Node consumer.*make release-check.*must not set continue-on-error' "$tmpdir/ignored-consumer-failure.err"

missing_ci_wiring="$tmpdir/missing-ci-wiring"
make_fixture "$missing_ci_wiring"
perl -0pi -e 's/  verify-gates:\n    runs-on: ubuntu-latest\n    steps:\n      - run: make verify-node-toolchain\n//' "$missing_ci_wiring/.github/workflows/ci.yml"
if run_verify "$missing_ci_wiring" >"$tmpdir/missing-ci-wiring.out" 2>"$tmpdir/missing-ci-wiring.err"; then
	echo "verify-node-toolchain accepted CI without invoking its own gate" >&2
	exit 1
fi
grep -q 'required unconditional run.*make verify-node-toolchain.*was not found' "$tmpdir/missing-ci-wiring.err"

wrong_ci_job="$tmpdir/wrong-ci-job"
make_fixture "$wrong_ci_job"
perl -0pi -e 's/  verify-gates:/  policy-gates:/' "$wrong_ci_job/.github/workflows/ci.yml"
if run_verify "$wrong_ci_job" >"$tmpdir/wrong-ci-job.out" 2>"$tmpdir/wrong-ci-job.err"; then
	echo "verify-node-toolchain accepted its CI invocation in the wrong job" >&2
	exit 1
fi
grep -q 'must be in workflow job.*verify-gates' "$tmpdir/wrong-ci-job.err"

conditional_ci_wiring="$tmpdir/conditional-ci-wiring"
make_fixture "$conditional_ci_wiring"
perl -0pi -e 's/- run: make verify-node-toolchain/- run: make verify-node-toolchain\n        if: false/' "$conditional_ci_wiring/.github/workflows/ci.yml"
if run_verify "$conditional_ci_wiring" >"$tmpdir/conditional-ci-wiring.out" 2>"$tmpdir/conditional-ci-wiring.err"; then
	echo "verify-node-toolchain accepted a conditional CI invocation of its own gate" >&2
	exit 1
fi
grep -q 'required run.*make verify-node-toolchain.*must be unconditional' "$tmpdir/conditional-ci-wiring.err"

conditional_ci_job="$tmpdir/conditional-ci-job"
make_fixture "$conditional_ci_job"
perl -0pi -e 's/  verify-gates:\n    runs-on:/  verify-gates:\n    if: false\n    runs-on:/' "$conditional_ci_job/.github/workflows/ci.yml"
if run_verify "$conditional_ci_job" >"$tmpdir/conditional-ci-job.out" 2>"$tmpdir/conditional-ci-job.err"; then
	echo "verify-node-toolchain accepted a conditional CI gate job" >&2
	exit 1
fi
grep -q 'job.*verify-gates.*containing Node policy steps must be unconditional' "$tmpdir/conditional-ci-job.err"

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
grep -q 'no direct setup-node steps found' "$tmpdir/comment.err"
