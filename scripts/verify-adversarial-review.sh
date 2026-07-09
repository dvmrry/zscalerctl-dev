#!/usr/bin/env bash
set -euo pipefail

repo_root="${ZSCALERCTL_REPO_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
cd "$repo_root"

base_ref="${ZSCALERCTL_BASE_REF:-origin/main}"
review_dir="${ZSCALERCTL_ADVERSARIAL_REVIEW_DIR:-docs/adversarial-reviews}"

if ! git rev-parse --quiet --verify "$base_ref" >/dev/null; then
	if [[ "$base_ref" == "origin/main" ]]; then
		echo "base ref $base_ref not found; fetching origin/main" >&2
		git fetch origin main:refs/remotes/origin/main
	else
		echo "base ref $base_ref not found" >&2
		exit 1
	fi
fi

changed_files="$(git diff --name-only "$base_ref" --)"
untracked_files="$(git ls-files --others --exclude-standard)"
if [[ -n "$untracked_files" ]]; then
	changed_files="${changed_files}"$'\n'"${untracked_files}"
fi

is_review_artifact() {
	local path="$1"
	[[ "$path" == "$review_dir/"*.md && "$path" != "$review_dir/README.md" ]]
}

is_high_risk_path() {
	local path="$1"

	case "$path" in
		AGENTS.md | Makefile | docs/SCRIPTS.md)
			return 0
			;;
		.codex/hooks.json | .codex/hooks/*)
			return 0
			;;
		.github/workflows/*)
			return 0
			;;
		cmd/zscalerctl/* | internal/cli/* | internal/runtime/* | internal/resources/*)
			return 0
			;;
		internal/zscaler/* | internal/dump/* | internal/diff/*)
			return 0
			;;
		internal/machine/* | internal/machineio/* | internal/output/*)
			return 0
			;;
		internal/redact/* | internal/secret/* | internal/secretref/* | internal/credentials/*)
			return 0
			;;
		docs/cli/zscalerctl.md | docs/FIELD_COVERAGE.md | docs/field-coverage.json)
			return 0
			;;
		docs/schema/* | cmd/zscalerctl/testdata/surface/*)
			return 0
			;;
		docs/adversarial-review*.md | docs/review-handoff-template.md)
			return 0
			;;
		skills/zscalerctl/* | .agents/skills/zscalerctl/*)
			return 0
			;;
		scripts/verify-adversarial-review.sh | scripts/test-verify-adversarial-review.sh)
			return 0
			;;
		scripts/gen-cli-docs.go | scripts/scaffold-resource.sh | scripts/sdk-surface-inventory.go | scripts/sync-agents-skill.sh)
			return 0
			;;
	esac

	return 1
}

high_risk_files=()
review_artifacts=()

while IFS= read -r path; do
	[[ -n "$path" ]] || continue
	if is_review_artifact "$path"; then
		review_artifacts+=("$path")
	elif is_high_risk_path "$path"; then
		high_risk_files+=("$path")
	fi
done <<<"$changed_files"

if ((${#high_risk_files[@]} == 0)); then
	exit 0
fi

if ((${#review_artifacts[@]} == 0)); then
	{
		echo "error: high-risk files changed without an adversarial-review artifact."
		echo
		echo "High-risk files:"
		printf '  - %s\n' "${high_risk_files[@]}"
		echo
		echo "Add an approved review artifact under $review_dir/ with:"
		echo "  - # Builder Handoff"
		echo "  - # Adversarial Review"
		echo "  - Fresh-context reviewer:"
		echo "  - Verdict: approve or Verdict: approve with nits"
	} >&2
	exit 1
fi

valid_artifact=""
artifact_errors=()

for artifact in "${review_artifacts[@]}"; do
	if [[ ! -f "$artifact" ]]; then
		artifact_errors+=("$artifact: file does not exist")
		continue
	fi
	if ! grep -q '^# Builder Handoff' "$artifact"; then
		artifact_errors+=("$artifact: missing # Builder Handoff")
		continue
	fi
	if ! grep -q '^# Adversarial Review' "$artifact"; then
		artifact_errors+=("$artifact: missing # Adversarial Review")
		continue
	fi
	if ! grep -q '^Fresh-context reviewer:' "$artifact"; then
		artifact_errors+=("$artifact: missing Fresh-context reviewer:")
		continue
	fi
	verdict_count="$(grep -Eic '^Verdict:' "$artifact" || true)"
	if [[ "$verdict_count" != "1" ]]; then
		artifact_errors+=("$artifact: expected exactly one Verdict line")
		continue
	fi
	if ! grep -Eiq '^Verdict: approve( with nits)?$' "$artifact"; then
		artifact_errors+=("$artifact: missing approved Verdict")
		continue
	fi
	valid_artifact="$artifact"
	break
done

if [[ -n "$valid_artifact" ]]; then
	exit 0
fi

{
	echo "error: high-risk files changed, but no approved adversarial-review artifact was found."
	echo
	if ((${#artifact_errors[@]} > 0)); then
		echo "Review artifact problems:"
		printf '  - %s\n' "${artifact_errors[@]}"
		echo
	fi
	echo "High-risk files:"
	printf '  - %s\n' "${high_risk_files[@]}"
} >&2
exit 1
