#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

github_dir="${ZSCALERCTL_GITHUB_DIR:-.github}"

if [[ ! -d "$github_dir" ]]; then
	exit 0
fi

fail=0

# This exact gitleaks-action release embeds Node 20, which GitHub-hosted
# runners reject without an insecure-runtime opt-out after 2026-06-02. Keep the
# denylist source-verifiable and narrow; current workflows use the Node-24 v3
# release instead.
retired_node_runtime_actions=(
	"gitleaks/gitleaks-action@ff98106e4c7b2bc287b24eaf42907196329070c7"
)

while IFS= read -r -d '' file; do
	line_no=0
	while IFS= read -r line || [[ -n "$line" ]]; do
		line_no=$((line_no + 1))
		if [[ ! "$line" =~ ^[[:space:]]*(-[[:space:]]*)?uses:[[:space:]]*[\"\']?([^[:space:]#\"\']+)[\"\']?(.*)$ ]]; then
			continue
		fi

		ref="${BASH_REMATCH[2]}"
		trailing="${BASH_REMATCH[3]}"

		if [[ "$ref" == ./* ]]; then
			continue
		fi

		if [[ ! "$ref" =~ @[0-9a-fA-F]{40}$ ]]; then
			echo "$file:$line_no external action is not pinned to a full commit SHA: $ref" >&2
			fail=1
			continue
		fi

		for retired in "${retired_node_runtime_actions[@]}"; do
			if [[ "$ref" == "$retired" ]]; then
				echo "$file:$line_no action uses a retired Node runtime: $ref" >&2
				fail=1
			fi
		done

		if [[ ! "$trailing" =~ \#[[:space:]]*v?[0-9][A-Za-z0-9._-]* ]]; then
			echo "$file:$line_no SHA-pinned action is missing a Renovate version comment: $ref" >&2
			fail=1
		fi
	done <"$file"
done < <(find "$github_dir" -type f \( -name '*.yml' -o -name '*.yaml' \) -print0)

if (( fail != 0 )); then
	exit 1
fi
