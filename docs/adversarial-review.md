# Post-Build Adversarial Review

Use adversarial review for high-risk agent-built changes that can silently
alter the CLI contract, tenant-data safety model, generated artifacts, or
resource coverage. The goal is to stop those changes at "ready for
adversarial review" instead of letting the builder self-approve.

The workflow now has an enforcement gate:
`scripts/verify-adversarial-review.sh` fails high-risk changes unless they
commit an approved review artifact under
[adversarial-reviews/](adversarial-reviews/). The gate is wired into
the project Codex `Stop` hook, `make verify-adversarial-review`, and
`make check`.

The gate is deliberately mechanical. It proves that high-risk changes include
a fresh-context review artifact; it does not try to score review quality.

The Stop hook is configured in `.codex/hooks.json` and runs
`.codex/hooks/adversarial_review_stop.sh` when Codex is about to finish a turn.
If high-risk changes lack an approved artifact, the hook blocks completion and
prints the required review loop.

## Invocation Rules

Treat "adversarial review" as a workflow term. When a user asks for an
adversarial review, do not silently substitute a stricter manual review.

- Run a fresh-context reviewer if a sub-agent, formal review workflow, or
  equivalent independent context is available.
- If no independent reviewer is available, say so clearly. The gate still
  requires an approved review artifact before high-risk changes can pass.
- For named PRs, branches, issues, or roadmap items, inspect the process docs
  from `origin/main` or the explicitly requested baseline before relying on the
  current checkout. Local branches may be stale or may not contain the current
  workflow rules.
- Do not claim the workflow completed unless the independent reviewer was
  actually run and its verdict is reported.

## Core Contract

For high-risk changes:

- The builder must not self-approve.
- The builder stops at "ready for adversarial review."
- The builder produces a review handoff using
  [review-handoff-template.md](review-handoff-template.md).
- The reviewer must run in a fresh Codex context: a new thread or reviewer
  agent that did not implement the change and does not share the builder's
  implementation conversation state.
- The reviewer should start from
  [adversarial-review-run-prompt.md](adversarial-review-run-prompt.md) so the
  review stance, evidence rules, and output format are explicit.
- The reviewer may inspect the diff, handoff, source artifacts, and test
  commands.
- The reviewer must not rely on the builder's implementation summary as
  evidence.
- The reviewer must not implement fixes.
- The reviewer assumes the change is wrong.
- Findings must be source-verifiable.
- For especially high-risk changes, run two independent fresh-context reviews
  before final acceptance.
- Fixes must map: finding -> root cause -> fix -> regression test ->
  verification.
- Final acceptance comes after the review/fix loop.

## When This Applies

Use adversarial review for changes touching:

- CLI parsing, command routing, help, completion, generated CLI docs, or
  command-surface goldens.
- Machine-readable contracts: JSON/NDJSON output, error envelopes, exit codes,
  `machine manifest`, `introspect`, `schema list`, dump manifests, diff
  reports, or published JSON Schemas.
- Resource catalog entries, reader wiring, source-record projection, adapter
  matching, entitlement/deferred-resource status, or coverage accounting.
- Redaction allow-lists, sensitive-data classification, field coverage,
  secret-handling boundaries, or output narrowing semantics.
- Generated artifacts such as `docs/cli/zscalerctl.md`,
  `docs/FIELD_COVERAGE.md`, `docs/field-coverage.json`, schemas, golden
  fixtures, surface-change manifests, or generated agent skill copies.
- Canonical skill changes under `skills/zscalerctl/` and the generated
  `.agents/skills/zscalerctl/` copy.

Routine docs-only edits, typo fixes, or narrow README updates do not need the
full process unless they alter behavior claims, process rules, machine
contracts, or generated-output interpretation.

## Review Resolution Loop

When review finds issues:

1. Builder fixes confirmed findings.
2. Builder reports:

```md
Finding:
Root cause:
Fix:
Regression test:
Verification:
```

3. Reviewer rechecks only the addressed findings, the changed surface
   introduced by the fix, and any generated artifacts affected by the fix.

The reviewer still does not implement fixes.

If review catches a repeatable class of miss, update the handoff, run prompt,
or checklist so the next reviewer looks for that class explicitly.

## Rollout Plan

1. Land docs/templates, `AGENTS.md` guidance, and the local verifier.
2. Add the project Codex Stop hook so the verifier runs automatically before
   Codex completes an implementation turn.
3. Use the workflow on the next CLI-surface, schema, resource-wiring,
   redaction/projection, field-coverage, or generated-artifact change.
4. After 2-3 reviewed changes, identify checks agents routinely miss.
5. Promote only proven checks into the mechanical verifier.

## Later Hard-Gate Candidates

Good candidates:

- Parser, command-surface, or generator code changed without relevant tests or
  recorded verification.
- Generated artifacts changed without regeneration notes.
- Golden fixtures changed without a surface-change or source-delta
  explanation.
- Field coverage, schema, manifest, dump, or diff artifacts changed without
  count/contract explanation.
- `skills/zscalerctl/` changed without syncing and checking the generated
  `.agents/skills/zscalerctl/` copy.

Avoid initially:

- Semantic "quality" gates.
- Broad "review was good enough" checks.
- Anything likely to become noisy or fake-authoritative.

## Success Criteria

The pilot works if:

- builders consistently emit useful handoffs
- reviewers find concrete issues or explicitly verify invariants
- generated artifact drift becomes easier to audit
- agents stop treating their own implementation summary as approval
- the process adds clarity without becoming heavy ceremony
