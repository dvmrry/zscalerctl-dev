#!/usr/bin/env bash
set -euo pipefail

script_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
repo_root="$script_root"
node_version_file=".node-version"
node_version_ref=".node-version"
ci_workflow=".github/workflows/ci.yml"
release_workflow=".github/workflows/release.yml"

# Path flags exist only so the regression script can validate isolated fixture
# repositories. The production Make target passes no arguments, and environment
# variables cannot redirect the repository, pin, or workflow paths.

usage() {
	echo "usage: $0 [--repo-root DIR] [--node-version-file PATH] [--node-version-ref PATH] [--ci-workflow PATH] [--release-workflow PATH]" >&2
	exit 2
}

while (( $# > 0 )); do
	case "$1" in
	--repo-root | --node-version-file | --node-version-ref | --ci-workflow | --release-workflow)
		(( $# >= 2 )) || usage
		case "$1" in
		--repo-root) repo_root="$2" ;;
		--node-version-file) node_version_file="$2" ;;
		--node-version-ref) node_version_ref="$2" ;;
		--ci-workflow) ci_workflow="$2" ;;
		--release-workflow) release_workflow="$2" ;;
		esac
		shift 2
		;;
	*) usage ;;
	esac
done

repo_root="$(cd "$repo_root" && pwd -P)"
cd "$repo_root"

# Exact reviewed runtime pin. The TypeScript client separately enforces its
# feature floor (Node >=24.12); keeping the CI/release pin here prevents a
# synchronized workflow downgrade from approving itself.
required_version="24.15.0"
required_go_version="1.26.5"
required_ci_aggregator_run="/bin/bash scripts/require-ci-jobs.sh \"\${{ join(needs.*.result, ' ') }}\""

if [[ ! -f "$node_version_file" ]]; then
	echo "$node_version_file: shared Node version file not found" >&2
	exit 1
fi

node_version="$(cat "$node_version_file")"
if [[ "$node_version" != "$required_version" ]]; then
	echo "$node_version_file: Node version $node_version does not match the reviewed runtime pin $required_version" >&2
	exit 1
fi

verify_workflow() {
	local workflow="$1"
	shift
	if [[ ! -f "$workflow" ]]; then
		echo "$workflow: required Node workflow not found" >&2
		exit 1
	fi
	local workflow_path
	workflow_path="$(cd "$(dirname "$workflow")" && pwd -P)/$(basename "$workflow")"
	(
		cd "$script_root"
		go run -mod=vendor "$script_root/scripts/verify-workflow-policies.go" \
			--mode setup-node \
			--scan-dir "$workflow_path" \
			--repo-root "$repo_root" \
			--go-minimum "$required_go_version" \
			--node-version-file "$node_version_ref" \
			"$@"
	)
}

verify_workflow \
	"$ci_workflow" \
	--required-run "/usr/bin/make verify-node-toolchain" \
	--required-run-job "node-policy" \
	--required-dependent-job "required" \
	--required-dependent-needs "node-policy,typescript-client" \
	--required-dependent-job-if '${{ always() }}' \
	--required-dependent-run "$required_ci_aggregator_run"
verify_workflow \
	"$release_workflow" \
	--required-run "/usr/bin/make release-check" \
	--required-run-job "release-gate" \
	--required-dependent-job "release" \
	--required-dependent-needs "release-gate" \
	--required-job-set "release-gate,release"
