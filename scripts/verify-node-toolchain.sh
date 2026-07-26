#!/usr/bin/env bash
set -euo pipefail

script_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
repo_root="${ZSCALERCTL_REPO_ROOT:-$script_root}"
repo_root="$(cd "$repo_root" && pwd -P)"
cd "$repo_root"

node_version_file="${ZSCALERCTL_NODE_VERSION_FILE:-.node-version}"
node_version_ref="${ZSCALERCTL_NODE_VERSION_REF:-.node-version}"
ci_workflow="${ZSCALERCTL_CI_WORKFLOW:-.github/workflows/ci.yml}"
release_workflow="${ZSCALERCTL_RELEASE_WORKFLOW:-.github/workflows/release.yml}"

# Exact reviewed runtime pin. The TypeScript client separately enforces its
# feature floor (Node >=24.12); keeping the CI/release pin here prevents a
# synchronized workflow downgrade from approving itself.
required_version="24.15.0"

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
			--node-version-file "$node_version_ref" \
			"$@"
	)
}

verify_workflow \
	"$ci_workflow" \
	--required-run "make verify-node-toolchain" \
	--required-run-job "verify-gates"
verify_workflow "$release_workflow"
