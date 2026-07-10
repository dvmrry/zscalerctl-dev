#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

github_dir="${ZSCALERCTL_GITHUB_DIR:-.github}"

if [[ ! -d "$github_dir" ]]; then
	exit 0
fi

scan_dir="$github_dir/workflows"
if [[ ! -d "$scan_dir" ]]; then
	# Preserve the old override behavior for callers that point the variable at
	# a fixture directory containing YAML files directly.
	scan_dir="$github_dir"
fi

if [[ "${ZSCALERCTL_GITHUB_DIR+x}" == x ]]; then
	github_dir_abs="$(cd "$github_dir" && pwd)"
	github_dir_parent="$(dirname "$github_dir_abs")"
	if [[ "${github_dir_abs##*/}" == ".github" ]]; then
		local_root="$(cd "$github_dir_abs/.." && pwd)"
	elif [[ "${github_dir_abs##*/}" == "workflows" && "${github_dir_parent##*/}" == ".github" ]]; then
		local_root="$(cd "$github_dir_parent/.." && pwd)"
	else
		local_root="$github_dir_abs"
	fi
else
	local_root="$repo_root"
fi

scan_dir_abs="$(cd "$scan_dir" && pwd)"
go run -mod=vendor "$repo_root/scripts/verify-workflow-policies.go" \
	--mode actions \
	--scan-dir "$scan_dir_abs" \
	--repo-root "$local_root"
