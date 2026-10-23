# Builder Handoff

## Intent

feat(filter): match nested fields element by element with dotted paths

A filter on a nested list matched its Go-formatted rendering
("map[id:11 name:APP_INFOSEC_DEV]"), so `groups~_DEV` also matched
`APP_INFOSEC_DEV`, key names matched substrings, and `groups=X` could never
match. `--filter` (and machine `filters[].field`) now accepts a dotted path
ending at a nested leaf field (`groups.name`, `groups.id`); it matches when
any value the path reaches does, element by element. Paths are validated
against the nested catalog before any read and walk the projected record,
so fields dropped by projection or redaction still match nothing. Top-level
keys keep whole-value matching, `--fields` stays top-level, and catalog
validation rejects field names containing `.`. Repeated dotted filters are
record-level AND and may be satisfied by different elements.

Reported downstream during a group-membership reporting task (2026-10-13).

## Base / Head

- Base: `integration/2026-10-09` at `9bb0344`.
- Reviewed heads: `8425d1c` (round 1), `3ecd443` (round 2).
- Final head: `8f86734`, which after round 2 changes only tests, README and
  the man page (round-2 nits 3, 4 and 6 below). This artifact is added
  afterwards and changes no reviewed code.

## Files Changed

- `.agents/skills/zscalerctl/SKILL.md`
- `README.md`
- `cmd/zscalerctl/testdata/surface/surface_changes.md`
- `docs/cli/agent-machine-workflow.md`
- `docs/cli/zscalerctl.md`
- `internal/cli/filter_search_test.go`
- `internal/cli/globalflags.go`
- `internal/cli/render_help.go`
- `internal/machine/executor_test.go`
- `internal/resources/resources.go`
- `internal/resources/resources_test.go`
- `man/zscalerctl.1`
- `skills/zscalerctl/SKILL.md`
- 17 surface goldens under `cmd/zscalerctl/testdata/surface/` (the `--filter` usage line only)

## Tests Run

- `go test ./...`, `gofmt -l`, `go vet` on changed packages (Go 1.26.9): pass.
- `scripts/verify-surface-changes-manifest.sh`, `scripts/verify-cli-docs.sh`,
  `scripts/sync-agents-skill.sh --check`: pass.
- New resource tests were run against the base `resources.go` and failed (6
  failures) before the change.
- `git diff -U0` over the 17 goldens: every changed line is the `--filter`
  usage string (15 Cobra help lines, the introspect JSON `usage`, the
  introspect-pretty line).

## Review History

Reviewed with no repository access by a fresh-context local model given the
diff, surrounding source and this repository's reviewer prompt and template:
Strata 0.1.43.1 serving Qwen3.8-Flash-Next (IQ3_S) on gm01. The reviewer
could not run commands; it traced code by hand.

- Round 1 (`8425d1c`): approve with nits. Addressed in `3ecd443`: dotted
  paths must end at a leaf field (a path ending at an object rendered the
  whole object); tests for two-level paths, dotted catalog names at every
  depth, and the machine executor; docs point to `schema list`, which already
  lists nested fields.
- Round 2 (`3ecd443`): approve with nits, below. Addressed in `8f86734`: nit
  3 (man page states the leaf rule and top-level exception), nit 4 (test and
  docs pin independent matching of repeated dotted filters), nit 6 (executor
  test asserts no loader call for an unknown path). Nits 1 and 2 are covered
  by the golden check above and by `schema list` output; nit 5 relies on
  projection normalizing nested lists to `[]any`.

# Adversarial Review

Fresh-context reviewer: Strata 0.1.43.1 (Qwen3.8-Flash-Next IQ3_S, gm01), read-only, no command execution, 2026-10-13 (round 2)

Baseline confirmation: the supplied review inputs identify base `9bb0344` on `integration/2026-10-09` and head `3ecd443` (`8425d1c` feature + `3ecd443` follow-up). No separate process-doc baseline artifact was included in the review inputs, so I used the supplied base/head as the diff baseline. That is an evidence gap, not a source-verifiable defect.

Review basis: the branch diff, shown source snippets, and tests. I cannot run commands; reported passing `go test`, `verify-surface-changes-manifest.sh`, `verify-cli-docs.sh`, and `sync-agents-skill.sh --check` are treated as reported evidence, not independently re-run.

## Blocking Findings

No blocking finding can be traced through the supplied diff and source artifacts.

The core follow-up behavior appears correct:

- `validFilterPath` accepts top-level catalog fields and dotted paths through nested catalog fields, and rejects dotted paths that end at a field with nested catalog fields.
- `projectedFilterMatches` keeps top-level whole-value matching, while dotted paths traverse projected values and match any reached leaf value.
- `projectedPathValues` traverses projected maps/lists and returns no values when projected fields are missing, preserving the post-projection/redaction invariant.
- `validateFields` rejects dotted field names, preventing ambiguous projection-report/filter paths.
- Tests cover:
  - nested exact/substring matching,
  - two-level object paths,
  - top-level list whole-value matching,
  - non-leaf nested path rejection,
  - `--fields` dotted-path rejection,
  - redaction/share/paranoid dropped-field nonmatching,
  - dotted field-name spec validation at top level, nested, and two levels deep,
  - machine executor dotted filter match and usage error.

## Non-Blocking Risks

1. **Golden diff evidence is still not directly visible.**

   - File/path: `cmd/zscalerctl/testdata/surface/surface_changes.md`; actual golden files not shown.
   - Concrete issue: the row claims 17 help/introspect goldens changed only in the `--filter` usage string, but the golden diffs themselves are not included.
   - Why it matters: goldens freeze CLI help, completion help, and introspection output. A hidden extra line, ordering change, or count change could be a contract drift.
   - How to reproduce or verify: inspect `git diff -U0 9bb0344..3ecd443 -- <golden paths>` and confirm every changed line is the expected `--filter` usage text.
   - Test that would catch it: existing golden tests plus `verify-surface-changes-manifest.sh` / `verify-cli-docs.sh`. The commands are reported passing, but the diff evidence is still not in this review packet.

2. **Machine schema discoverability for dotted filter keys is not evidenced in this diff.**

   - File/path: machine schema/introspection source not shown; docs updated in `docs/cli/agent-machine-workflow.md`, `skills/zscalerctl/SKILL.md`, and README.
   - Concrete issue: prose now tells users that dotted paths are supported and that `schema list` shows nested fields, but the diff does not show a machine schema description change or a schema-list test proving nested field discovery.
   - Why it matters: machine clients should discover valid `Filter.Field` semantics from machine-readable introspection/schema, not only human docs. If `schema list` does not actually expose nested fields, or if machine schema guidance is stale, agents may construct invalid filters.
   - How to reproduce or verify: run `zscalerctl --format json schema list` or inspect the machine schema/introspection output for `zia/users` and confirm nested `groups` fields and filter-field guidance.
   - Test that would catch it: a schema-list JSON test or machine schema test asserting nested fields are present and, ideally, that `filter.field` guidance mentions dotted leaf paths.

3. **Man page wording is less precise than README/agent docs about the leaf rule and top-level exception.**

   - File/path: `man/zscalerctl.1`
   - Concrete issue: the man page says a dotted path matches when any list element it reaches matches, but it does not explicitly say the path must end at a leaf field or that top-level keys keep whole-value matching. README and agent docs do state those rules.
   - Why it matters: users may try `--filter owner.manager=...` or similar non-leaf nested paths and receive exit 2. The behavior is correct, but the man page underdocuments the validation rule.
   - How to reproduce or verify: compare `man/zscalerctl.1` with `validFilterPath` and `TestNestedFilterPathsValidateAgainstNestedCatalog`, which rejects `owner.manager` but accepts top-level `owner`.
   - Test that would catch it: a docs consistency test or golden/doc check comparing man/README/agent docs for the leaf/top-level exception wording.

4. **Repeated nested filters are record-level, not same-element correlated.**

   - File/path: `internal/resources/resources.go`, `projectedFilterMatches` / `projectedPathValues`
   - Concrete issue: each dotted filter independently matches any value reached by that path. Two filters on nested fields can be satisfied by different elements. For example, a user with groups `[{name:"APP_DEV", id:10}, {name:"OTHER", id:11}]` would match both `--filter groups.name=APP_DEV --filter groups.id=11`, even though no single group has both name `APP_DEV` and id `11`.
   - Why it matters: agents or users may expect nested filters to describe the same nested object. The current semantics are defensible as record-level AND, but they can produce surprising false positives when used for exact nested-object membership.
   - How to reproduce or verify: add a projected record with two groups, one matching name and another matching id, then run `FilterProjectedRecords` with both filters.
   - Test that would catch it: an explicit test documenting cross-element AND behavior, or a new test if same-element correlated matching is desired.

5. **`projectedPathValues` relies on projection normalized nested slices to `[]any`.**

   - File/path: `internal/resources/resources.go`, `projectedPathValues`
   - Concrete issue: the type switch handles `[]any` and `[]string`, but not `[]map[string]any`, `[]int`, or other concrete slice types. The current projection code normalizes nested `[]map[string]any` to `[]any`, so the current behavior is okay, but the matcher itself is fragile if projection drift leaves concrete slices in projected fields.
   - Why it matters: future projection changes could silently make nested paths match whole slices instead of elements, or fail to traverse maps.
   - How to reproduce or verify: call `FilterProjectedRecords` with a projected field containing `[]map[string]any` or `[]int` for a valid nested leaf path.
   - Test that would catch it: a resources test that either normalizes those slice types defensively or asserts projected values passed to filtering are always normalized.

6. **Machine invalid-path test does not explicitly prove the loader was not called.**

   - File/path: `internal/machine/executor_test.go`, `TestExecutorFiltersNestedPathsElementByElement`
   - Concrete issue: the test asserts invalid dotted filter returns a usage `MachineError`, but it reuses an executor after a successful request and does not assert a zero loader-call count for the invalid request.
   - Why it matters: the stated machine contract is validation before live read. The test proves error kind, but not “before loader” as strongly as the CLI test does with `failingResourceReader`.
   - How to reproduce or verify: inspect `fakeBrowserLoader`; if it does not track calls, the test cannot prove the loader was skipped.
   - Test that would catch it: use a loader that fails on any call or records call count, then assert invalid dotted filters produce usage error with zero loader calls.

## Machine Contract Review

- CLI/machine help surface changed:
  - `--filter` usage string changed in `internal/cli/globalflags.go`.
  - Rendered global usage changed in `internal/cli/render_help.go`.
  - `surface_changes.md` adds a row for 17 goldens covering help commands and `introspect` / `introspect-pretty`.
  - The row explains the delta as dotted nested paths, element-wise matching, top-level whole-value matching, and unchanged exit codes.

- Exit-code behavior:
  - Unknown dotted filter keys are validated through `ValidateNarrowOptions` and return `UnknownFilterError`.
  - CLI test asserts `errors.Is(err, cli.ErrUsage)` for `groups.notAField=x` before a failing reader is used.
  - Machine executor test asserts usage `MachineError` for `groups.notAField`.
  - No new exit-code surface is introduced.

- JSON/NDJSON/error envelopes:
  - No JSON shape change is shown.
  - NDJSON behavior is not touched.
  - Error envelope kind remains usage for unknown filter fields.
  - `--fields` dotted paths remain rejected as unknown fields, preserving the top-level output projection contract.

- Introspection/schema:
  - Introspection usage strings changed via goldens/manifest.
  - The machine-readable schema or `schema list` nested-field discovery is asserted only by docs and builder-reported tests in this packet. That is a review gap, but no concrete contract defect is visible.

- Manifest/surface docs:
  - `surface_changes.md` delta is present and the golden count matches the listed 17 commands.
  - If the actual golden diff is clean, this is adequate. The missing golden diff remains the main evidence gap.

## Safety Review

- Narrow-only invariant:
  - Filter validation is independent of redaction mode, as documented.
  - Matching runs against projected fields only.
  - `projectedPathValues` returns no values for missing projected keys, so fields dropped by projection/redaction cannot match.
  - `TestNestedFilterPathsCannotReachDroppedFields` verifies:
    - share mode drops `groups.name` and `groups.comments`,
    - paranoid mode drops `groups`,
    - filters on those dropped nested fields return no records.

- Output narrowing:
  - `--fields` remains top-level only.
  - `TestNestedFilterPathsValidateAgainstNestedCatalog` verifies `--fields groups.name` returns `ErrUnknownField`.
  - Nested filters can use projected fields omitted from `--fields`, but that is consistent with the existing model where filtering runs after projection/redaction and can narrow records using sanitized projected fields.

- Sensitive-data classification:
  - No classification changes are shown.
  - Nested fields carry their own `Classification` and `AllowedModes` in the test spec.
  - Dotted field names are rejected in spec validation, preventing ambiguous projection-report paths and filter-path parsing.

- Redaction:
  - No new redaction bypass is visible.
  - Nested paths do not consult source records directly; they traverse projected maps/lists.
  - If a nested parent or child is dropped, the path reaches no values and cannot match.

## Generated Artifact Review

- Generated docs and agent skill:
  - `docs/cli/agent-machine-workflow.md`, `skills/zscalerctl/SKILL.md`, and `README.md` were updated consistently with dotted-path behavior.
  - The generated `.agents` copy and `docs/cli/zscalerctl.md` are excluded from the shown diff, but `sync-agents-skill.sh --check` and `verify-cli-docs.sh` are reported passing.

- Man page:
  - `man/zscalerctl.1` was updated with the dotted-path behavior.
  - The man page wording is slightly less complete than README/agent docs about leaf-rule/top-level exception behavior.

- Goldens:
  - Goldens are not shown in the review packet.
  - The surface manifest claims only the `--filter` usage line changed across 17 goldens.
  - This is the main generated-artifact evidence gap. The reported verification commands reduce risk, but the diff itself should be visible in a future review packet if the goal is to prove no hidden golden drift.

## Verdict

Verdict: approve with nits
