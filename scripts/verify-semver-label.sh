#!/usr/bin/env bash
set -euo pipefail

labels="${ZSCALERCTL_PR_LABELS:-${*:-}}"
allow_major_zero="${ZSCALERCTL_ALLOW_MAJOR_ZERO:-false}"

normalized="$(printf '%s\n' "$labels" | tr ', ' '\n\n' | sed '/^$/d')"
semver_labels="$(printf '%s\n' "$normalized" | grep -E '^semver:(patch|minor|major|none)$' || true)"
count="$(printf '%s\n' "$semver_labels" | sed '/^$/d' | wc -l | tr -d ' ')"

print_policy_note() {
	cat >&2 <<'EOF'
SemVer label policy:
  semver:none is only for inert changes: docs-only, tests-only, comments, formatting, or non-shipped metadata.
  Runtime behavior, execution paths, release gates, security controls, candidate seams, adapter helpers, output behavior, and machine-contract changes are at least semver:patch.
  See docs/VERSIONING.md#automation.
EOF
}

if [[ "$count" != "1" ]]; then
	echo "pull request must have exactly one semver label: semver:patch, semver:minor, semver:major, or semver:none" >&2
	if [[ -n "$semver_labels" ]]; then
		printf 'found semver labels:\n%s\n' "$semver_labels" >&2
	else
		echo "found semver labels: none" >&2
	fi
	print_policy_note
	exit 1
fi

label="$(printf '%s\n' "$semver_labels" | sed -n '1p')"
if [[ "$label" == "semver:none" ]]; then
	print_policy_note
fi

if [[ "$label" != "semver:major" ]]; then
	exit 0
fi

latest_tag="$(git tag -l 'v[0-9]*.[0-9]*.[0-9]*' --sort=-v:refname 2>/dev/null | sed -n '1p' || true)"
major=0
if [[ "$latest_tag" =~ ^v([0-9]+)\.[0-9]+\.[0-9]+$ ]]; then
	major="${BASH_REMATCH[1]}"
fi

if [[ "$major" == "0" && "$allow_major_zero" != "true" ]]; then
	echo "semver:major is reserved for post-1.0 releases; use semver:minor for breaking 0.x changes" >&2
	exit 1
fi
