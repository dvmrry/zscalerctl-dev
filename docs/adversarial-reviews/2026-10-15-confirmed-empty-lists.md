# Builder Handoff

## Intent

fix: render [] only where the API returned an empty list

Empty lists were handled inconsistently and wrong in both directions: an empty
list of objects was always dropped (1,125 ZIA users with `"groups": []` read as
not set), while an empty list of plain values rendered `[]` even when the API
never sent the key (ZIA users' `authMethods` rendered `[]` on all 35,731 users;
the list endpoint does not return it). The SDK still distinguishes the two
after decoding (absent -> nil slice, `[]` -> empty non-nil); the mappers
discarded it.

The model is now one rule. Projection skips every empty list unless it carries
a confirmed-empty marker, which renders `[]` in a mode that renders the field
(skipped empty lists are not redaction drops). The generic list/get handlers
wrap their mapper with `preserveEmptyLists`: a top-level slice the API sent as
`[]` becomes a confirmed-empty list, but only for fields the mapper copies
element by element under their own JSON name. That is checked once per type:
probes with one and then two zero elements must map to lists of length one and
two. The marker never overrides a mapper's non-empty value. Saved dump records
are already projected output, so diff admission treats their empty lists as
confirmed (`NewSourceRecordFromProjected`).

So `[]` means the API said none; a missing list key means not sent, omitted by
the mode, or not modeled. Agent docs say so.

## Base / Head

- Base: `integration/2026-10-09` at `cc6a54a`.
- Reviewed heads: `d77e8bf` (round 1), `d496f8e` (round 2), `5b93cc7`
  (round 3), `c959ad3` (round 4).
- Final head: this artifact on top of `c959ad3`.

## Files Changed

- `.agents/skills/zscalerctl/SKILL.md`, `AGENTS.md`,
  `docs/cli/agent-machine-workflow.md`, `skills/zscalerctl/SKILL.md`
- `internal/diff/diff.go`, `internal/diff/empty_list_admission_test.go`
- `internal/resources/resources.go`, `internal/resources/empty_list_internal_test.go`
- `internal/zscaler/empty_lists.go`, `internal/zscaler/empty_lists_test.go`,
  `internal/zscaler/reader.go`

## Tests Run

- `go test ./...` (Go 1.26.9, `/bin/chmod` first in `PATH`), `gofmt -l`,
  `go vet`, `verify-docs`, `verify-cli-docs`,
  `verify-surface-changes-manifest`, `verify-machine-contract`: pass.
- Live, non-production tenant: a value-free probe over a full dump found
  3,010 explicit `[]` lists across 17 resources and 46,416 absent list keys;
  ZIA rule criteria are never sent as `[]`. zia users before/after: `groups`
  absent 1,125 -> `[]` 1,125; `authMethods` `[]` 35,731 -> absent. A branch
  dump reads back with `--from-dump` and diffs cleanly against itself; an
  older dump diffs against it with only that one-time change.
- `TestAdmitRecordsKeepsSavedEmptyLists` fails ("non-idempotent record")
  without the admission change.

## Known Gaps

- Nested lists and `[]*T` fields are never confirmed, so they stay absent when
  empty; the docs' "a missing list key does not prove none are set" covers it.
- Handlers outside the generic list/get handlers keep dropping empty lists.
- What a missing rule criterion means ("applies to all") is a separate
  catalog-semantics change.

## Review History

Reviewed with no repository access by a fresh-context local model given the
diff, the projection functions, and this repository's reviewer prompt and
template: Strata serving Qwen3.8-Flash-Next (IQ3_S) on gm01. The reviewer
could not run commands.

- Round 1, request changes: the identity probe accepted a key that was merely
  present. Now the probe element must come out as a non-empty list. Added a
  filter/count-by test for `[]`.
- Round 2, request changes: a constant non-empty list passed. The check is now
  differential (lengths one and two must track).
- Round 3, request changes: a mapper defaulting an empty input to a non-empty
  list. Not a defect: the marker never overrides a non-empty value; a test
  pins the reviewer's exact shape.
- Round 4, approve with nits (below).

# Adversarial Review

Fresh-context reviewer: Strata (Qwen3.8-Flash-Next IQ3_S, gm01), read-only, no command execution, 2026-10-15 (round 4)

## Blocking Findings

None.

The round-3 blocker is resolved by the guard in `SourceRecord.WithConfirmedEmptyList`.

Concrete evidence:

- `internal/zscaler/empty_lists.go` / `WithConfirmedEmptyList`:
  ```go
  if value, ok := r.fields[key]; ok && !isUnconfirmedEmptyList(value) {
      return r
  }
  ```
  The marker only writes `confirmedEmptyList{}` when the key is absent or holds an unconfirmed empty list. If the mapper has already written a non-empty value, the record is returned unchanged.

- `preserveEmptyLists` only calls `WithConfirmedEmptyList` when the input slice is non-nil and length zero:
  ```go
  if !list.IsNil() && list.Len() == 0 {
      record = record.WithConfirmedEmptyList(field.name)
  }
  ```

- The new test in `internal/zscaler/empty_lists_test.go` pins the reviewer’s exact shape:
  - API input: `Tags: []string{}`
  - mapper output: `tags = []string{"default"}`
  - preserved/wrapped output: `ListLen("tags") == 1`

If `WithConfirmedEmptyList` lacked the non-empty guard, the wrapper would replace the mapper’s `["default"]` with a confirmed empty marker and the new test would fail because `ListLen("tags")` would become `0`.

## Non-Blocking Risks

1. Missing helper definitions in the supplied artifacts  
   - `file/path`: `internal/zscaler/empty_lists.go`
   - concrete issue: the supplied source uses `isUnconfirmedEmptyList`, `confirmedEmptyList`, `SourceRecord.ListLen`, and `resources.NewSourceRecord`, but their definitions were not included.
   - why it matters: the guard’s correctness depends on `isUnconfirmedEmptyList` not misclassifying non-empty mapper output as unconfirmed empty.
   - how to verify: include definitions for those helpers in future handoffs.
   - test that would catch it: existing test catches the supplied `[]string{"default"}` case, but a table test over mapper-written scalar, map, nil, empty slice, and non-empty slice values would harden the contract.

2. New test asserts length, not value  
   - `file/path`: `internal/zscaler/empty_lists_test.go`
   - concrete issue: it asserts `ListLen("tags") == 1`, not that the field remains `[]string{"default"}`.
   - why it matters: length is enough to disprove a false empty claim, but value assertion would more directly pin “never overrides the mapper’s own value.”
   - how to verify: inspect the returned record’s `tags` field.
   - test that would catch it: assert the marker/value is not replaced, for example by checking the concrete `tags` value or a projection helper that exposes the list contents.

3. Identity detection is length-only  
   - `file/path`: `internal/zscaler/empty_lists.go`
   - concrete issue: `identityListFields` checks that mapped list length tracks probe length 1 and 2. It does not verify element values.
   - why it matters: a mapper that preserves length but transforms values can still be treated as identity. This does not create the round-3 false-empty-list blocker because the marker guard prevents false empty claims, but it can hide value-level mapper behavior.
   - how to verify: probe with distinguishable non-zero elements and compare projected values.
   - test that would catch it: value-aware identity probe, not just length.

4. Process-doc baseline was not supplied  
   - concrete issue: the review inputs give base/head commits but no process-doc baseline artifact.
   - why it matters: docs/goldens/schema sync cannot be fully audited from the supplied inputs.
   - how to verify: provide the process-doc baseline and generated-artifact diff in future rounds.
   - test that would catch it: golden/docs regeneration check.

## Machine Contract Review

- Did JSON, NDJSON, error envelope, exit code, exit-code behavior, dump, diff, schema, manifest, or introspection behavior change?
  - This head commit is test-only. It does not change CLI surface, help, completion, goldens, exit codes, JSON, NDJSON, error envelopes, schemas, manifests, or introspection output.
  - The underlying empty-list feature remains a machine-contract change: API `[]` can become a confirmed empty list when the mapper leaves the key absent or unconfirmed empty.

- Are contract deltas intentional and documented?
  - The test documents the important safety property: a mapper’s non-empty value is not overwritten by the confirmed-empty marker.
  - The supplied diff does not include docs/schema changes because there are none in this commit.

- Are generated docs, schemas, and fixtures in sync with runtime behavior?
  - No generated artifacts changed in this commit.
  - Because the process-doc baseline was not supplied, full sync cannot be independently confirmed from the review inputs.

## Safety Review

- Did redaction, projection, field coverage, or sensitive-data classification change?
  - No. The diff is test-only and does not touch redaction, projection, or sensitive-data classification.

- Can a narrowed field set widen output or reveal previously dropped data?
  - No. The confirmed-empty marker represents an empty list; it does not widen field coverage or reveal dropped data.

- Are dump and diff artifacts still sanitized but treated as confidential?
  - No dump/diff artifact changes are present.

## Generated Artifact Review

- Were generated files intentionally regenerated?
  - No generated files changed.

- Are deltas explained?
  - No generated deltas exist in the supplied diff.

- Any suspicious drops, additions, ordering changes, or count changes?
  - None observed. The only change is one added test case.

## Verdict

Verdict: approve with nits
