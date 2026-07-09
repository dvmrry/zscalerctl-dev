# Builder Handoff

## Intent

Enforce the post-build adversarial review workflow for high-risk changes. The
previous version was guidance-only, which let agents treat "adversarial review"
as a stricter manual review instead of a required fresh-context workflow.

## Base / Head

Base branch or commit: `origin/main` / `779b563`
Head branch or commit: local `main` working tree after the enforcement-hook
change.

## Files Changed

- `AGENTS.md`
- `Makefile`
- `docs/README.md`
- `docs/SCRIPTS.md`
- `docs/adversarial-review.md`
- `docs/adversarial-review-run-prompt.md`
- `docs/adversarial-review-template.md`
- `docs/review-handoff-template.md`
- `docs/adversarial-reviews/README.md`
- `scripts/verify-adversarial-review.sh`
- `scripts/test-verify-adversarial-review.sh`

## Source Inputs Consulted

- Existing Makefile verifier patterns.
- Existing `scripts/verify-surface-changes-manifest.sh` diff-base and
  untracked-file handling.
- Existing script registry requirements in `docs/SCRIPTS.md`.
- Fresh-context adversarial review feedback on the first hook implementation.

## Generated Artifacts

None. All files are handwritten Markdown, shell, Makefile, and workflow
changes.

## Expected Delta

- `make verify-adversarial-review` runs a new verifier and self-test.
- `make check` runs the new gate.
- High-risk paths fail without an approved artifact under
  `docs/adversarial-reviews/`.
- Review artifacts must include builder handoff and adversarial review
  sections, a fresh-context reviewer line, and exactly one approved verdict.
- Request-changes or mixed-verdict artifacts fail.

## Invariants Claimed

- No runtime CLI behavior changed.
- No generated CLI docs, schemas, field coverage artifacts, or golden fixtures
  changed.
- The gate is mechanical: it proves a review artifact is present, not that the
  review was high quality.
- The gate detects untracked high-risk files for local validation.
- The default `origin/main` base is fetched as a remote-tracking ref when
  missing.

## Tests Run

- `make docs-check`: passed.
- `make verify-script-registry`: passed.
- `git diff --check`: passed.
- `bash scripts/test-verify-adversarial-review.sh`: passed.
- `bash scripts/verify-adversarial-review.sh`: failed before this artifact was
  added, as expected, because high-risk files changed without an approved
  artifact.

## Known Deferrals

- The gate does not score review quality.
- The gate does not enforce two independent reviews for especially high-risk
  changes.
- The high-risk path list is intentionally conservative and can be widened as
  new miss classes are found.

## Review Focus

Verify the hook is actually enforced in local checks, high-risk path detection
matches the documented policy, artifact validation cannot pass a
request-changes review, and default base-ref handling works when `origin/main`
is missing in shallow or stale local checkouts.

# Adversarial Review

Fresh-context reviewer: Hubble (`019f44dc-86f4-7ff3-87d5-4c46c866f4b5`)

## Blocking Findings

The first review found three blocking issues:

- Default-base verification could fail mechanically because fetching
  `origin main` did not create `refs/remotes/origin/main`.
- The high-risk matcher missed redaction, machine-contract, output, and
  secret-handling surfaces.
- Mixed artifacts could pass if they contained both request-changes and
  approved verdict lines.

The builder fixed all three and added regression coverage.

## Recheck

The fresh-context reviewer rechecked only the prior findings and fix surface.
The reviewer confirmed:

- default `origin/main` fetch now creates `refs/remotes/origin/main`
- previously missed high-risk directories now block without an artifact
- mixed request-changes plus approved artifacts are rejected
- `bash scripts/test-verify-adversarial-review.sh` passes
- the top-level verifier still fails as expected until this approved artifact
  is added

## Non-Blocking Risks

Artifact validation is intentionally mechanical and can still be gamed by bad
faith text. That is accepted for this first enforced hook because the local
gate can reliably require that a fresh-context review artifact exists before
`make check` passes.

## Machine Contract Review

No runtime machine contract changed. The gate now treats machine-contract
directories such as `internal/machine/` and `internal/machineio/` as high-risk.

## Safety Review

No redaction, projection, credential, or secret-handling behavior changed. The
gate now treats `internal/redact/`, `internal/secret/`,
`internal/secretref/`, and `internal/credentials/` as high-risk.

## Generated Artifact Review

No generated artifacts changed.

Verdict: approve
