#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT

init_repo() {
	local dir="$1"
	mkdir -p "$dir"
	cd "$dir"
	git init -q
	git checkout -q -b main
	git config user.email "test@example.com"
	git config user.name "Test"
	printf '# Test\n' >README.md
	mkdir -p internal/cli docs/adversarial-reviews
	printf 'package cli\n' >internal/cli/app.go
	printf '# Review artifacts\n' >docs/adversarial-reviews/README.md
	git add -A
	git commit -q -m "base"
}

run_gate() {
	local dir="$1"
	ZSCALERCTL_BASE_REF=HEAD \
		ZSCALERCTL_REPO_ROOT="$dir" \
		bash "$repo_root/scripts/verify-adversarial-review.sh"
}

run_stop_hook() {
	local dir="$1"
	ZSCALERCTL_BASE_REF=HEAD \
		ZSCALERCTL_REPO_ROOT="$dir" \
		bash "$repo_root/.codex/hooks/adversarial_review_stop.sh"
}

write_approved_review() {
	mkdir -p docs/adversarial-reviews
	cat >docs/adversarial-reviews/test-review.md <<'EOF'
# Builder Handoff

Intent: test high-risk change.

# Adversarial Review

Fresh-context reviewer: codex-subagent

Verdict: approve
EOF
}

init_repo "$tmp_dir/clean"
run_gate "$tmp_dir/clean"

init_repo "$tmp_dir/low-risk-doc"
cd "$tmp_dir/low-risk-doc"
printf 'more docs\n' >>README.md
run_gate "$tmp_dir/low-risk-doc"

init_repo "$tmp_dir/high-risk-no-review"
cd "$tmp_dir/high-risk-no-review"
printf '// changed\n' >>internal/cli/app.go
if run_gate "$tmp_dir/high-risk-no-review" >"$tmp_dir/out" 2>"$tmp_dir/err"; then
	echo "verify-adversarial-review accepted a high-risk change without review" >&2
	exit 1
fi
if ! grep -q "high-risk files changed without an adversarial-review artifact" "$tmp_dir/err"; then
	echo "verify-adversarial-review failed without the expected missing-artifact message" >&2
	cat "$tmp_dir/err" >&2
	exit 1
fi

init_repo "$tmp_dir/high-risk-approved"
cd "$tmp_dir/high-risk-approved"
printf '// changed\n' >>internal/cli/app.go
write_approved_review
run_gate "$tmp_dir/high-risk-approved"

init_repo "$tmp_dir/high-risk-request-changes"
cd "$tmp_dir/high-risk-request-changes"
printf '// changed\n' >>internal/cli/app.go
write_approved_review
perl -0pi -e 's/Verdict: approve/Verdict: request changes/' docs/adversarial-reviews/test-review.md
if run_gate "$tmp_dir/high-risk-request-changes" >"$tmp_dir/out" 2>"$tmp_dir/err"; then
	echo "verify-adversarial-review accepted a request-changes verdict" >&2
	exit 1
fi
if ! grep -q "missing approved Verdict" "$tmp_dir/err"; then
	echo "verify-adversarial-review failed without the expected verdict message" >&2
	cat "$tmp_dir/err" >&2
	exit 1
fi

init_repo "$tmp_dir/high-risk-missing-fresh-context"
cd "$tmp_dir/high-risk-missing-fresh-context"
printf '// changed\n' >>internal/cli/app.go
write_approved_review
perl -0pi -e 's/Fresh-context reviewer: codex-subagent/Reviewer: codex-subagent/' docs/adversarial-reviews/test-review.md
if run_gate "$tmp_dir/high-risk-missing-fresh-context" >"$tmp_dir/out" 2>"$tmp_dir/err"; then
	echo "verify-adversarial-review accepted a review without fresh-context evidence" >&2
	exit 1
fi
if ! grep -q "missing Fresh-context reviewer" "$tmp_dir/err"; then
	echo "verify-adversarial-review failed without the expected fresh-context message" >&2
	cat "$tmp_dir/err" >&2
	exit 1
fi

init_repo "$tmp_dir/high-risk-mixed-verdict"
cd "$tmp_dir/high-risk-mixed-verdict"
printf '// changed\n' >>internal/cli/app.go
write_approved_review
cat >>docs/adversarial-reviews/test-review.md <<'EOF'

Verdict: request changes
EOF
if run_gate "$tmp_dir/high-risk-mixed-verdict" >"$tmp_dir/out" 2>"$tmp_dir/err"; then
	echo "verify-adversarial-review accepted a mixed-verdict review" >&2
	exit 1
fi
if ! grep -q "expected exactly one Verdict line" "$tmp_dir/err"; then
	echo "verify-adversarial-review failed without the expected mixed-verdict message" >&2
	cat "$tmp_dir/err" >&2
	exit 1
fi

init_repo "$tmp_dir/untracked-high-risk"
cd "$tmp_dir/untracked-high-risk"
printf 'package cli\n' >internal/cli/new_command.go
if run_gate "$tmp_dir/untracked-high-risk" >"$tmp_dir/out" 2>"$tmp_dir/err"; then
	echo "verify-adversarial-review accepted an untracked high-risk file without review" >&2
	exit 1
fi
if ! grep -q "internal/cli/new_command.go" "$tmp_dir/err"; then
	echo "verify-adversarial-review failed without naming the untracked high-risk file" >&2
	cat "$tmp_dir/err" >&2
	exit 1
fi

for path in \
	internal/redact/redact.go \
	internal/machine/manifest.go \
	internal/machineio/machineio.go \
	internal/output/output.go \
	internal/secret/secret.go \
	internal/secretref/ref.go \
	internal/credentials/credentials.go; do
	name="${path//\//-}"
	init_repo "$tmp_dir/high-risk-$name"
	cd "$tmp_dir/high-risk-$name"
	mkdir -p "$(dirname "$path")"
	printf 'package fixture\n' >"$path"
	if run_gate "$tmp_dir/high-risk-$name" >"$tmp_dir/out" 2>"$tmp_dir/err"; then
		echo "verify-adversarial-review accepted high-risk path $path without review" >&2
		exit 1
	fi
	if ! grep -q "$path" "$tmp_dir/err"; then
		echo "verify-adversarial-review failed without naming high-risk path $path" >&2
		cat "$tmp_dir/err" >&2
		exit 1
	fi
done

init_repo "$tmp_dir/hook-config-no-review"
cd "$tmp_dir/hook-config-no-review"
mkdir -p .codex/hooks
printf '{}\n' >.codex/hooks.json
if run_gate "$tmp_dir/hook-config-no-review" >"$tmp_dir/out" 2>"$tmp_dir/err"; then
	echo "verify-adversarial-review accepted hook config changes without review" >&2
	exit 1
fi
if ! grep -q ".codex/hooks.json" "$tmp_dir/err"; then
	echo "verify-adversarial-review failed without naming .codex/hooks.json" >&2
	cat "$tmp_dir/err" >&2
	exit 1
fi

init_repo "$tmp_dir/stop-hook-no-review"
cd "$tmp_dir/stop-hook-no-review"
mkdir -p .codex/hooks
printf '#!/usr/bin/env bash\n' >.codex/hooks/adversarial_review_stop.sh
if run_gate "$tmp_dir/stop-hook-no-review" >"$tmp_dir/out" 2>"$tmp_dir/err"; then
	echo "verify-adversarial-review accepted hook script changes without review" >&2
	exit 1
fi
if ! grep -q ".codex/hooks/adversarial_review_stop.sh" "$tmp_dir/err"; then
	echo "verify-adversarial-review failed without naming the hook script" >&2
	cat "$tmp_dir/err" >&2
	exit 1
fi

init_repo "$tmp_dir/fetch-source"
cd "$tmp_dir/fetch-source"
git clone -q --bare . "$tmp_dir/fetch-origin.git"
git clone -q "$tmp_dir/fetch-origin.git" "$tmp_dir/fetch-work"
cd "$tmp_dir/fetch-work"
git remote set-branches origin feature-only
git update-ref -d refs/remotes/origin/main
printf '// changed\n' >>internal/cli/app.go
write_approved_review
ZSCALERCTL_REPO_ROOT="$tmp_dir/fetch-work" \
	bash "$repo_root/scripts/verify-adversarial-review.sh"
if ! git rev-parse --quiet --verify origin/main >/dev/null; then
	echo "verify-adversarial-review did not fetch origin/main as a remote-tracking ref" >&2
	exit 1
fi

init_repo "$tmp_dir/worktree-source"
cd "$tmp_dir/worktree-source"
git worktree add -q "$tmp_dir/linked-worktree"
cd "$tmp_dir/linked-worktree"
printf '// changed\n' >>internal/cli/app.go
if run_stop_hook "$tmp_dir/linked-worktree" >"$tmp_dir/out" 2>"$tmp_dir/err"; then
	echo "adversarial review Stop hook accepted an unreviewed linked-worktree change" >&2
	exit 1
fi
if ! grep -q "Adversarial review is required" "$tmp_dir/err"; then
	echo "Stop hook failed without the expected linked-worktree review message" >&2
	cat "$tmp_dir/err" >&2
	exit 1
fi
