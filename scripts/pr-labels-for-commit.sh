#!/usr/bin/env bash
set -euo pipefail

repo="${1:-}"
sha="${2:-}"

if [[ -z "$repo" || -z "$sha" ]]; then
	echo "usage: pr-labels-for-commit.sh <owner/repo> <commit-sha>" >&2
	exit 2
fi

if [[ -n "${ZSCALERCTL_PR_JSON:-}" ]]; then
	pr_json="$ZSCALERCTL_PR_JSON"
else
	pr_json="$(gh api \
		-H "Accept: application/vnd.github+json" \
		"/repos/${repo}/commits/${sha}/pulls")"
fi

printf '%s\n' "$pr_json" | jq -r 'if length == 0 then empty else .[0].labels[].name end'
