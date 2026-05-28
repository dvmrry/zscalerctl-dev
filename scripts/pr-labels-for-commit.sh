#!/usr/bin/env bash
set -euo pipefail

repo="${1:-}"
sha="${2:-}"

if [[ -z "$repo" || -z "$sha" ]]; then
	echo "usage: pr-labels-for-commit.sh <owner/repo> <commit-sha>" >&2
	exit 2
fi

labels_from_commit_prs() {
	local pr_json
	if [[ -n "${ZSCALERCTL_PR_JSON:-}" ]]; then
		pr_json="$ZSCALERCTL_PR_JSON"
	else
		pr_json="$(gh api \
			-H "Accept: application/vnd.github+json" \
			"/repos/${repo}/commits/${sha}/pulls")"
	fi
	printf '%s\n' "$pr_json" | jq -r 'if length == 0 then empty else .[0].labels[].name end'
}

commit_message() {
	if [[ -n "${ZSCALERCTL_COMMIT_MESSAGE:-}" ]]; then
		printf '%s\n' "$ZSCALERCTL_COMMIT_MESSAGE"
		return
	fi
	git show -s --format=%B "$sha"
}

labels_from_pr_number() {
	local pr_number="$1"
	local pr_json
	if [[ -n "${ZSCALERCTL_PR_VIEW_JSON:-}" ]]; then
		pr_json="$ZSCALERCTL_PR_VIEW_JSON"
	else
		pr_json="$(gh pr view "$pr_number" --repo "$repo" --json labels)"
	fi
	printf '%s\n' "$pr_json" | jq -r '.labels[].name'
}

labels="$(labels_from_commit_prs 2>/dev/null || true)"
if [[ -n "$labels" ]]; then
	printf '%s\n' "$labels"
	exit 0
fi

pr_number="$(commit_message | sed -nE 's/.*\(#([0-9]+)\).*/\1/p' | tail -n 1)"
if [[ -z "$pr_number" ]]; then
	echo "could not resolve pull request labels for commit $sha" >&2
	exit 1
fi

labels_from_pr_number "$pr_number"
