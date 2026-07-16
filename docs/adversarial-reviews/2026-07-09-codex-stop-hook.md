# Builder Handoff

## Intent

Wire adversarial review into the Codex implementation lifecycle with a
project-local `Stop` hook. The hook should run automatically before Codex
finishes a turn, call the local adversarial-review verifier, and block
completion when high-risk changes lack an approved fresh-context review
artifact.

## Base / Head

Base branch or commit: local `main` after `cc5b775`.
Head branch or commit: working tree with `.codex/` Stop hook wiring.

## Files Changed

- `.codex/hooks.json`
- `.codex/hooks/adversarial_review_stop.sh`
- `AGENTS.md`
- `docs/adversarial-review.md`
- `docs/adversarial-reviews/2026-07-09-codex-stop-hook.md`
- `scripts/verify-adversarial-review.sh`
- `scripts/test-verify-adversarial-review.sh`

## Source Inputs Consulted

- Codex manual hook documentation for project-local hooks, `Stop` events, and
  command hook handlers.
- Existing local adversarial-review verifier.
- Existing hook trust and project `.codex/` behavior documented by Codex.
- Fresh-context adversarial review of the first Stop hook implementation.

## Generated Artifacts

None. All files are handwritten JSON, shell, and Markdown.

## Expected Delta

- Codex loads `.codex/hooks.json` when the project `.codex/` layer is trusted.
- The `Stop` hook runs `.codex/hooks/adversarial_review_stop.sh` before Codex
  completes a turn.
- The hook script invokes `scripts/verify-adversarial-review.sh`.
- If the verifier fails, the hook exits nonzero and prints the required
  review loop.
- `.codex/hooks.json` and `.codex/hooks/*` are high-risk paths that require a
  review artifact when changed.

## Invariants Claimed

- No runtime CLI behavior changed.
- No generated artifacts changed.
- The hook remains local-only; no CI workflow is modified.
- Linked Git worktrees are handled correctly.
- The hook does not try to spawn reviewers itself; it blocks completion and
  requires Codex to run the fresh-context review loop.

## Tests Run

- `bash .codex/hooks/adversarial_review_stop.sh`: passed with the approved
  artifact present.
- `make verify-adversarial-review`: passed.
- `make docs-check`: passed.
- `git diff --check`: passed.

## Known Deferrals

- Codex command hooks cannot directly invoke the in-app sub-agent tool.
- Hook trust must be reviewed by Codex when the project `.codex/` layer changes.

## Review Focus

Verify the Stop hook actually runs in Codex-linked worktrees, blocks rather
than silently no-ops when review is required, keeps `.codex/` hook files in
the high-risk path set, and does not reintroduce CI/server-side enforcement.

# Adversarial Review

Fresh-context reviewer: Kepler (`019f4508-48f6-7a80-9b8a-553ac34f4b12`)

## Blocking Findings

The first review found one blocking issue:

- `.codex/hooks/adversarial_review_stop.sh` silently exited in linked
  worktrees because `.git` is a file, not a directory.

The builder fixed the hook to use `git rev-parse --is-inside-work-tree` after
resolving and entering the repository root.

## Recheck

The fresh-context reviewer rechecked only the linked-worktree fix and
regression coverage. The reviewer confirmed:

- the Stop hook now invokes `scripts/verify-adversarial-review.sh` in this
  linked worktree
- an unreviewed high-risk change in a linked worktree fails with the expected
  review-required message
- `make verify-adversarial-review` passes

## Non-Blocking Risks

Project-local hooks require Codex hook trust before they run. That is accepted
because hook trust is the Codex harness-level review mechanism for local hook
code.

## Machine Contract Review

No CLI machine contract changed.

## Safety Review

No redaction, projection, credential, or tenant-data behavior changed. This is
process enforcement only.

## Generated Artifact Review

No generated artifacts changed.

Verdict: approve
