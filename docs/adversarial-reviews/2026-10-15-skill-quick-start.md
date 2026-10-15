# Builder Handoff

## Intent

docs(skill): quick start; name allowed_modes in schema references

Round-3 agent findings (2026-10-15): the skill led with long preflight shell
recipes, and agents wanted a short quick path; the `schema references` prose
said "the modes that render it" and an agent looked for a `modes` key, while
the output key is `allowed_modes`. The skill gains a quick start (product
help, a targeted schema lookup, multi-path `--count-by`, a nested `--filter`,
`schema references`, `schema gaps`, the default mode, and the exit codes in
the canonical wording), and the three agent docs name `allowed_modes`.

## Base / Head

- Base: `integration/2026-10-09` at `5c9a080`.
- Reviewed heads: `f3a006f` (round 1), the exit-code commit (round 2).
- Final head: this artifact.

## Files Changed

- `.agents/skills/zscalerctl/SKILL.md` (generated), `AGENTS.md`,
  `docs/cli/agent-machine-workflow.md`, `skills/zscalerctl/SKILL.md`

## Tests Run

- `verify-docs`, `verify-cli-docs`: pass. Quick-start commands checked against
  a built binary (product help lists resources; `schema list` is an array of
  entries with product/name; `schema references` entries carry
  `allowed_modes`).

## Review History

Reviewed by Strata (Qwen3.8-Flash-Next IQ3_S on gm01), no repository access.

- Round 1, request changes: the quick-start exit-code line paraphrased and
  omitted codes of the canonical table. Now it lists all eight in the
  canonical wording.
- Round 2, approve with nits (below).

# Adversarial Review

Fresh-context reviewer: Strata (Qwen3.8-Flash-Next IQ3_S, gm01), read-only, no command execution, 2026-10-15 (round 2)

## Blocking Findings

None.

The diff is docs-only and changes one quick-start exit-code summary. The round-1 blocker was that the summary omitted canonical exit-code entries. The new text lists all eight numeric codes: 0, 1, 2, 3, 4, 5, 6, and 7. It also replaces the prior incomplete code-4 wording with the canonical “not found or unsupported” phrase. No runtime CLI, JSON/NDJSON, error envelope, dump, diff, schema, manifest, redaction, or generated-artifact behavior is changed by the visible diff.

## Non-Blocking Risks

- `skills/zscalerctl/SKILL.md`
  - Issue: code 7 is abbreviated as `7 drift`, while the canonical wording supplied in the review input is stronger: “drift detected by `diff --fail-on-drift`”.
  - Why it matters: if this quick-start is consumed as a machine-contract summary, users or scripts may assume any drift-related failure returns 7, when the canonical contract may only define 7 for the explicit drift-gated diff path.
  - Verify: compare this line against the canonical exit-code table and the actual `diff --fail-on-drift` behavior.
  - Test: golden-doc or CLI contract test asserting that the quick-start exit-code line includes the canonical qualifier for 7, or at minimum that `7` is not documented without the `diff --fail-on-drift` condition if that condition is contractually required.

- `skills/zscalerctl/SKILL.md`
  - Issue: code 2 is documented as `2 usage`, not the canonical `2 usage error`.
  - Why it matters: minor wording drift in a contract summary can make docs-vs-table tests brittle.
  - Verify: compare against the canonical table.
  - Test: docs golden test that requires canonical exit-code phrases.

- `skills/zscalerctl/SKILL.md`
  - Issue: the new code-4 parenthetical, “a resource zscalerctl does not read names its gap”, is not evidenced by the diff or supplied source artifacts.
  - Why it matters: this could overclaim runtime error-text behavior. If unsupported or unread resources do not actually name a gap in the error envelope, the docs become misleading.
  - Verify: run the relevant unsupported/unread-resource command and inspect the error envelope and exit code.
  - Test: CLI integration test asserting exit code 4 and the documented gap-naming behavior for an unread/unsupported resource.

- Review input gap: no source artifacts, canonical table, or test commands were provided. This prevents verification that the SKILL.md wording is synchronized with the canonical source of truth.

## Machine Contract Review

- Did JSON, NDJSON, error envelope, exit code, dump, diff, schema, manifest, or introspection behavior change?
  - No runtime behavior changed in the diff. Only the documented exit-code summary changed.
- Are contract deltas intentional and documented?
  - The docs delta appears intentional and resolves the prior omission of codes 1 and 7.
  - The code-7 wording is still abbreviated relative to the canonical phrase supplied in the review input.
- Are generated docs, schemas, and fixtures in sync with runtime behavior?
  - Not verifiable from the provided inputs. If `skills/zscalerctl/SKILL.md` is generated or mirrors a canonical docs table, a sync test should catch drift.

## Safety Review

- Did redaction, projection, field coverage, or sensitive-data classification change?
  - No.
- Can a narrowed field set widen output or reveal previously dropped data?
  - No.
- Are dump and diff artifacts still sanitized but treated as confidential?
  - Not changed by this diff.

## Generated Artifact Review

- Were generated files intentionally regenerated?
  - The diff directly edits `skills/zscalerctl/SKILL.md`. If this file is generated from another source artifact, the source and regeneration path are not shown.
- Are deltas explained?
  - The visible delta is a docs correction and expansion of the exit-code summary.
- Any suspicious drops, additions, ordering changes, or count changes?
  - No suspicious drops. It adds codes 1 and 7 and expands codes 3 and 4. The only concern is wording precision for 7 and the unverified code-4 gap-naming claim.

## Verdict

Verdict: approve with nits
