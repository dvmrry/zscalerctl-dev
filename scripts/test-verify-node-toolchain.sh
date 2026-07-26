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
        if: steps.version.outputs.release == 'true'
YAML
}

run_verify() {
	local fixture="$1"
	"$verifier" \
		--repo-root "$fixture" \
		--node-version-file "$fixture/.node-version" \
		--node-version-ref '.node-version' \
		--ci-workflow "$fixture/.github/workflows/ci.yml" \
		--release-workflow "$fixture/.github/workflows/release.yml"
}

for variable in \
	ZSCALERCTL_REPO_ROOT \
	ZSCALERCTL_NODE_VERSION_FILE \
	ZSCALERCTL_NODE_VERSION_REF \
	ZSCALERCTL_CI_WORKFLOW \
	ZSCALERCTL_RELEASE_WORKFLOW; do
	if grep -q "$variable" "$verifier"; then
		echo "verify-node-toolchain reads production path override $variable" >&2
		exit 1
	fi
done

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
        if: steps.version.outputs.release == 'true'
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
grep -q 'input.*node-version.*must not override the shared node-version-file' "$tmpdir/override.err"

mixed_case_version="$tmpdir/mixed-case-version"
make_fixture "$mixed_case_version"
perl -0pi -e "s/node-version-file: '\.node-version'/node-version-file: '.node-version'\n          Node-Version: '22.23.1'/" "$mixed_case_version/.github/workflows/release.yml"
if run_verify "$mixed_case_version" >"$tmpdir/mixed-case-version.out" 2>"$tmpdir/mixed-case-version.err"; then
	echo "verify-node-toolchain accepted a mixed-case node-version override" >&2
	exit 1
fi
grep -q 'input.*Node-Version.*must not override' "$tmpdir/mixed-case-version.err"

mixed_case_version_file="$tmpdir/mixed-case-version-file"
make_fixture "$mixed_case_version_file"
perl -0pi -e "s/node-version-file: '\.node-version'/node-version-file: '.node-version'\n          Node-Version-File: '.decoy-node-version'/" "$mixed_case_version_file/.github/workflows/release.yml"
if run_verify "$mixed_case_version_file" >"$tmpdir/mixed-case-version-file.out" 2>"$tmpdir/mixed-case-version-file.err"; then
	echo "verify-node-toolchain accepted a case-colliding node-version-file input" >&2
	exit 1
fi
grep -q 'input.*Node-Version-File.*must use the exact lowercase key node-version-file' "$tmpdir/mixed-case-version-file.err"

mixed_case_cache="$tmpdir/mixed-case-cache"
make_fixture "$mixed_case_cache"
perl -0pi -e 's/package-manager-cache: false/package-manager-cache: false\n          Package-Manager-Cache: true/' "$mixed_case_cache/.github/workflows/release.yml"
if run_verify "$mixed_case_cache" >"$tmpdir/mixed-case-cache.out" 2>"$tmpdir/mixed-case-cache.err"; then
	echo "verify-node-toolchain accepted a case-colliding package-manager-cache input" >&2
	exit 1
fi
grep -q 'input.*Package-Manager-Cache.*must use the exact lowercase key package-manager-cache' "$tmpdir/mixed-case-cache.err"

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
        if: steps.version.outputs.release == 'true'
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
        if: steps.version.outputs.release == 'true'
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

skipped_release_gate="$tmpdir/skipped-release-gate"
make_fixture "$skipped_release_gate"
perl -0pi -e "s/if: steps\.version\.outputs\.release == 'true'/if: false/" "$skipped_release_gate/.github/workflows/release.yml"
if run_verify "$skipped_release_gate" >"$tmpdir/skipped-release-gate.out" 2>"$tmpdir/skipped-release-gate.err"; then
	echo "verify-node-toolchain accepted a skipped release-check consumer" >&2
	exit 1
fi
grep -q 'Node consumer.*make release-check.*must use the literal release condition' "$tmpdir/skipped-release-gate.err"

wrong_release_consumer="$tmpdir/wrong-release-consumer"
make_fixture "$wrong_release_consumer"
perl -0pi -e "s/- run: make release-check\n        if: steps\.version\.outputs\.release == 'true'/- run: make verify-typescript-client/" "$wrong_release_consumer/.github/workflows/release.yml"
if run_verify "$wrong_release_consumer" >"$tmpdir/wrong-release-consumer.out" 2>"$tmpdir/wrong-release-consumer.err"; then
	echo "verify-node-toolchain accepted a release workflow without make release-check" >&2
	exit 1
fi
grep -q 'required run.*make release-check.*was not found' "$tmpdir/wrong-release-consumer.err"

conditional_ci_consumer="$tmpdir/conditional-ci-consumer"
make_fixture "$conditional_ci_consumer"
perl -0pi -e 's/- run: bash scripts\/verify-typescript-client\.sh/- run: bash scripts\/verify-typescript-client.sh\n        if: false/' "$conditional_ci_consumer/.github/workflows/ci.yml"
if run_verify "$conditional_ci_consumer" >"$tmpdir/conditional-ci-consumer.out" 2>"$tmpdir/conditional-ci-consumer.err"; then
	echo "verify-node-toolchain accepted a conditional TypeScript consumer" >&2
	exit 1
fi
grep -q 'Node consumer.*verify-typescript-client.*must be unconditional' "$tmpdir/conditional-ci-consumer.err"

release_custom_shell="$tmpdir/release-custom-shell"
make_fixture "$release_custom_shell"
perl -0pi -e "s/if: steps\.version\.outputs\.release == 'true'/if: steps.version.outputs.release == 'true'\n        shell: \/bin\/true \{0\}/" "$release_custom_shell/.github/workflows/release.yml"
if run_verify "$release_custom_shell" >"$tmpdir/release-custom-shell.out" 2>"$tmpdir/release-custom-shell.err"; then
	echo "verify-node-toolchain accepted a no-op custom shell on make release-check" >&2
	exit 1
fi
grep -q 'Node policy run.*make release-check.*must use the runner.*default shell' "$tmpdir/release-custom-shell.err"

ci_custom_shell="$tmpdir/ci-custom-shell"
make_fixture "$ci_custom_shell"
perl -0pi -e 's/- run: make verify-node-toolchain/- run: make verify-node-toolchain\n        shell: \/bin\/true \{0\}/' "$ci_custom_shell/.github/workflows/ci.yml"
if run_verify "$ci_custom_shell" >"$tmpdir/ci-custom-shell.out" 2>"$tmpdir/ci-custom-shell.err"; then
	echo "verify-node-toolchain accepted a no-op custom shell on its CI self-check" >&2
	exit 1
fi
grep -q 'Node policy run.*make verify-node-toolchain.*must use the runner.*default shell' "$tmpdir/ci-custom-shell.err"

workflow_run_defaults="$tmpdir/workflow-run-defaults"
make_fixture "$workflow_run_defaults"
perl -0pi -e 's/on: \[push\]\n/on: [push]\ndefaults:\n  run:\n    shell: \/bin\/true \{0\}\n/' "$workflow_run_defaults/.github/workflows/release.yml"
if run_verify "$workflow_run_defaults" >"$tmpdir/workflow-run-defaults.out" 2>"$tmpdir/workflow-run-defaults.err"; then
	echo "verify-node-toolchain accepted a workflow-level no-op default shell" >&2
	exit 1
fi
grep -q 'Node policy workflows must not override run defaults' "$tmpdir/workflow-run-defaults.err"

job_run_defaults="$tmpdir/job-run-defaults"
make_fixture "$job_run_defaults"
perl -0pi -e 's/    runs-on: ubuntu-latest\n/    runs-on: ubuntu-latest\n    defaults:\n      run:\n        shell: \/bin\/true \{0\}\n/' "$job_run_defaults/.github/workflows/release.yml"
if run_verify "$job_run_defaults" >"$tmpdir/job-run-defaults.out" 2>"$tmpdir/job-run-defaults.err"; then
	echo "verify-node-toolchain accepted a job-level no-op default shell" >&2
	exit 1
fi
grep -q 'job.*release.*must not override run defaults' "$tmpdir/job-run-defaults.err"

alternate_working_directory="$tmpdir/alternate-working-directory"
make_fixture "$alternate_working_directory"
perl -0pi -e "s/if: steps\.version\.outputs\.release == 'true'/if: steps.version.outputs.release == 'true'\n        working-directory: clients\/typescript/" "$alternate_working_directory/.github/workflows/release.yml"
if run_verify "$alternate_working_directory" >"$tmpdir/alternate-working-directory.out" 2>"$tmpdir/alternate-working-directory.err"; then
	echo "verify-node-toolchain accepted an alternate release-gate working directory" >&2
	exit 1
fi
grep -q 'Node policy run.*make release-check.*must use the repository root working directory' "$tmpdir/alternate-working-directory.err"

step_environment="$tmpdir/step-environment"
make_fixture "$step_environment"
perl -0pi -e "s/if: steps\.version\.outputs\.release == 'true'/if: steps.version.outputs.release == 'true'\n        env:\n          PATH: \/tmp\/decoy/" "$step_environment/.github/workflows/release.yml"
if run_verify "$step_environment" >"$tmpdir/step-environment.out" 2>"$tmpdir/step-environment.err"; then
	echo "verify-node-toolchain accepted a release-gate step environment override" >&2
	exit 1
fi
grep -q 'Node policy run.*make release-check.*must not define environment overrides' "$tmpdir/step-environment.err"

workflow_environment="$tmpdir/workflow-environment"
make_fixture "$workflow_environment"
perl -0pi -e 's/on: \[push\]\n/on: [push]\nenv:\n  PATH: \/tmp\/decoy\n/' "$workflow_environment/.github/workflows/release.yml"
if run_verify "$workflow_environment" >"$tmpdir/workflow-environment.out" 2>"$tmpdir/workflow-environment.err"; then
	echo "verify-node-toolchain accepted a workflow-level environment override" >&2
	exit 1
fi
grep -q 'Node policy workflows must not define workflow-level environment overrides' "$tmpdir/workflow-environment.err"

job_environment="$tmpdir/job-environment"
make_fixture "$job_environment"
perl -0pi -e 's/    runs-on: ubuntu-latest\n/    runs-on: ubuntu-latest\n    env:\n      PATH: \/tmp\/decoy\n/' "$job_environment/.github/workflows/release.yml"
if run_verify "$job_environment" >"$tmpdir/job-environment.out" 2>"$tmpdir/job-environment.err"; then
	echo "verify-node-toolchain accepted a job-level environment override" >&2
	exit 1
fi
grep -q 'job.*release.*must not define environment overrides' "$tmpdir/job-environment.err"

release_container="$tmpdir/release-container"
make_fixture "$release_container"
perl -0pi -e 's/    runs-on: ubuntu-latest\n/    runs-on: ubuntu-latest\n    container:\n      image: ghcr.io\/example\/noop:latest\n      env:\n        PATH: \/opt\/decoy:\/usr\/bin:\/bin\n/' "$release_container/.github/workflows/release.yml"
if run_verify "$release_container" >"$tmpdir/release-container.out" 2>"$tmpdir/release-container.err"; then
	echo "verify-node-toolchain accepted a container on the release policy job" >&2
	exit 1
fi
grep -q 'job.*release.*key.*container.*is not allowed' "$tmpdir/release-container.err"

ci_gate_container="$tmpdir/ci-gate-container"
make_fixture "$ci_gate_container"
perl -0pi -e 's/  verify-gates:\n    runs-on: ubuntu-latest\n/  verify-gates:\n    runs-on: ubuntu-latest\n    container:\n      image: ghcr.io\/example\/noop:latest\n/' "$ci_gate_container/.github/workflows/ci.yml"
if run_verify "$ci_gate_container" >"$tmpdir/ci-gate-container.out" 2>"$tmpdir/ci-gate-container.err"; then
	echo "verify-node-toolchain accepted a container on its CI self-check job" >&2
	exit 1
fi
grep -q 'job.*verify-gates.*key.*container.*is not allowed' "$tmpdir/ci-gate-container.err"

alternate_runner="$tmpdir/alternate-runner"
make_fixture "$alternate_runner"
perl -0pi -e 's/runs-on: ubuntu-latest/runs-on: self-hosted/' "$alternate_runner/.github/workflows/release.yml"
if run_verify "$alternate_runner" >"$tmpdir/alternate-runner.out" 2>"$tmpdir/alternate-runner.err"; then
	echo "verify-node-toolchain accepted a non-reviewed release runner" >&2
	exit 1
fi
grep -q 'job.*release.*must use the literal runner ubuntu-latest' "$tmpdir/alternate-runner.err"

setup_environment="$tmpdir/setup-environment"
make_fixture "$setup_environment"
perl -0pi -e 's/- uses: actions\/setup-node@example/- uses: actions\/setup-node@example\n        env:\n          RUNNER_TOOL_CACHE: \/tmp\/decoy-cache/' "$setup_environment/.github/workflows/release.yml"
if run_verify "$setup_environment" >"$tmpdir/setup-environment.out" 2>"$tmpdir/setup-environment.err"; then
	echo "verify-node-toolchain accepted a setup-node environment override" >&2
	exit 1
fi
grep -q 'setup-node step key.*env.*is not allowed' "$tmpdir/setup-environment.err"

setup_mirror="$tmpdir/setup-mirror"
make_fixture "$setup_mirror"
perl -0pi -e 's/node-version-file: '\''\.node-version'\''/node-version-file: '\''.node-version'\''\n          mirror: https:\/\/example.invalid\/node/' "$setup_mirror/.github/workflows/release.yml"
if run_verify "$setup_mirror" >"$tmpdir/setup-mirror.out" 2>"$tmpdir/setup-mirror.err"; then
	echo "verify-node-toolchain accepted an unreviewed setup-node mirror" >&2
	exit 1
fi
grep -q 'setup-node input.*mirror.*is not in the reviewed allowlist' "$tmpdir/setup-mirror.err"

environment_redirect="$tmpdir/environment-redirect"
make_fixture "$environment_redirect"
cat >"$environment_redirect/.github/workflows/release.yml" <<'YAML'
name: release
on: [push]
jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - run: make release-check
        if: steps.version.outputs.release == 'true'
YAML
if ZSCALERCTL_REPO_ROOT="$good" \
	ZSCALERCTL_NODE_VERSION_FILE="$good/.node-version" \
	ZSCALERCTL_NODE_VERSION_REF='.node-version' \
	ZSCALERCTL_CI_WORKFLOW="$good/.github/workflows/ci.yml" \
	ZSCALERCTL_RELEASE_WORKFLOW="$good/.github/workflows/release.yml" \
	run_verify "$environment_redirect" >"$tmpdir/environment-redirect.out" 2>"$tmpdir/environment-redirect.err"; then
	echo "verify-node-toolchain allowed environment variables to redirect validation to decoy files" >&2
	exit 1
fi
grep -q 'no direct setup-node steps found' "$tmpdir/environment-redirect.err"

environment_pin_redirect="$tmpdir/environment-pin-redirect"
make_fixture "$environment_pin_redirect"
printf '22.23.1\n' >"$environment_pin_redirect/.node-version"
printf '24.15.0\n' >"$environment_pin_redirect/.decoy-node-version"
if ZSCALERCTL_NODE_VERSION_FILE="$environment_pin_redirect/.decoy-node-version" \
	run_verify "$environment_pin_redirect" >"$tmpdir/environment-pin-redirect.out" 2>"$tmpdir/environment-pin-redirect.err"; then
	echo "verify-node-toolchain allowed an environment variable to redirect the reviewed Node pin" >&2
	exit 1
fi
grep -q 'does not match the reviewed runtime pin' "$tmpdir/environment-pin-redirect.err"

missing_ci_wiring="$tmpdir/missing-ci-wiring"
make_fixture "$missing_ci_wiring"
perl -0pi -e 's/  verify-gates:\n    runs-on: ubuntu-latest\n    steps:\n      - run: make verify-node-toolchain\n//' "$missing_ci_wiring/.github/workflows/ci.yml"
if run_verify "$missing_ci_wiring" >"$tmpdir/missing-ci-wiring.out" 2>"$tmpdir/missing-ci-wiring.err"; then
	echo "verify-node-toolchain accepted CI without invoking its own gate" >&2
	exit 1
fi
grep -q 'required run.*make verify-node-toolchain.*was not found' "$tmpdir/missing-ci-wiring.err"

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
        if: steps.version.outputs.release == 'true'
YAML
if run_verify "$comment_spoof" >"$tmpdir/comment.out" 2>"$tmpdir/comment.err"; then
	echo "verify-node-toolchain accepted a commented-out release setup-node step" >&2
	exit 1
fi
grep -q 'no direct setup-node steps found' "$tmpdir/comment.err"
