# Builder Handoff

## Intent

feat(schema): document how ZIA rule criteria combine

Agents read a rule with no users, groups or locations as "applies to
everyone" without knowing how criteria combine. The operator asked that the
semantics come from Zscaler's documentation rather than be litigated case by
case. Each ZIA policy's help page has a "Policy Execution" section:
categories are ANDed, criteria within a category have their own operator,
and "selecting no value ignores the criterion in the policy evaluation". The
operators differ by policy (URL filtering lets the admin choose AND or OR
between users, groups and departments and the API does not return the
choice; bandwidth control ANDs locations with location groups; cloud app
control ORs device trust level with devices and user risk).

`schema describe` now carries `matching` for twelve ZIA rule resources,
transcribed from those pages and cited by URL: ANDed groups of catalog
fields with operator `or` or `admin_selected`; `unplaced_criteria` for
criteria fields the documentation does not place; `not_rendered`, the match
fields the requested redaction mode omits; `not_read` for documented criteria
the API does not return; and `notes` for modifiers. The published semantics
schema gains `ruleMatching` and `criteriaGroup`; text output prints them.

## Base / Head

- Base: `integration/2026-10-09` at `d845b95`.
- Reviewed heads: `12cf3b7` (round 1), `573af84` (round 2).
- Final head: this artifact on top of `573af84`.

## Files Changed

- `internal/resources/rule_matching.go` (new), `internal/resources/semantics.go`
- `internal/resources/rule_matching_test.go` (new),
  `internal/resources/semantics_schema_test.go`
- `internal/cli/commands_schema_describe.go`,
  `internal/cli/commands_schema_describe_test.go`
- `docs/schema/resource-semantics-v1.schema.json`
- `AGENTS.md`, `skills/zscalerctl/SKILL.md` (+ `.agents` copy),
  `docs/cli/agent-machine-workflow.md`,
  `cmd/zscalerctl/testdata/surface/surface_changes.md`

## Tests Run

- `go test ./...`, `gofmt -l`, `go vet`, `verify-docs`, `verify-cli-docs`,
  `verify-surface-changes-manifest`, `verify-machine-contract`,
  `verify-typescript-client`: pass.
- Tests pin: every criteria-like catalog field of a covered resource is
  grouped or unplaced; every modifier field is explained by a note;
  `not_rendered` equals exactly the match fields the mode omits (users only
  in paranoid for URL rules); URL filtering's identity group is
  `admin_selected`; resources without documented logic carry no `matching`;
  the published schema matches the Go types.
- Offline check against a saved share-mode list of 153 URL rules from the
  non-production tenant: no rule lacks criteria; 143 list all 17 request
  methods, which led to the "every value does not restrict" note.

## Known Gaps

- Only twelve ZIA rule resources; CASB, ZTW and the remaining ZIA policies
  carry no `matching`.
- `matching` describes the documented logic. It does not evaluate a rule
  against traffic or resolve the `admin_selected` operator, which the API does
  not return.

## Review History

Reviewed by Strata (Qwen3.8-Flash-Next IQ3_S on gm01) with no repository
access, given the code and verbatim excerpts of each Zscaler policy page.

- Round 1, request changes: (1) the logic text claimed other catalog fields
  are not criteria while notes named modifiers; (2) `resCategories` was
  placed in forwarding and IPS destination groups the documentation does not
  name. Fixed in `573af84`: the logic text defers to notes, a test requires
  every modifier field to be explained by a note, and `resCategories` moved
  to `unplaced_criteria`.
- Round 2, approve with nits (below).

# Adversarial Review

Fresh-context reviewer: Strata (Qwen3.8-Flash-Next IQ3_S, gm01), read-only, no command execution, 2026-10-15 (round 2)

## Blocking Findings

None. The two Round-1 blockers are addressed by the diff.

For blocker 1, `internal/resources/rule_matching.go` replaces the absolute claim:

```text
Other catalog fields are not match criteria.
```

with:

```text
A field named in notes modifies matching as the note says (an exclusion flag or a validity window); other catalog fields are not documented as match criteria.
```

This no longer contradicts notes that describe `excludeSrcCountries` and `enforceTimeValidity` as modifiers. The added test in `internal/resources/rule_matching_test.go` requires every documented rule resource that has one of these catalog modifier fields to have a note naming that field:

```go
var modifierFields = []string{"excludeSrcCountries", "enforceTimeValidity", "excludedUsers", "excludedGroups", "excludedDepartments"}
```

The cloud-app-control definition gains the missing validity note. The firewall definition already has the `excludeSrcCountries` note.

For blocker 2, `resCategories` is removed from the `destination` group for:

```text
zia/forwarding-rules
zia/ips-policies
```

and moved into `unplaced_criteria` for both. That matches the Round-1 requirement: if the cited documentation does not place response categories in the destination expression, the field should not be represented as part of the documented destination group.

## Non-Blocking Risks

1. `modifierFields` is hand-maintained.

   - **file/path**: `internal/resources/rule_matching_test.go`
   - **issue**: The test only checks a fixed list of modifier fields. Future notes may name another field that modifies matching, but the test would not catch it.
   - **why it matters**: The logic sentence now says any field named in notes modifies matching. If a future note names a field that is neither in `groups`, `unplaced_criteria`, nor `modifierFields`, the contract can drift again.
   - **verify**: Grep `ruleMatchingDefinitions` for notes containing field names and compare against `groups`, `unplaced_criteria`, and `modifierFields`.
   - **test that would catch it**: Extract candidate field identifiers from notes and require each named field to be either represented in `groups`/`unplaced_criteria` or covered by the note-modifier rule.

2. `sourceCountries` is still represented as a normal positive source-group member for firewall rules.

   - **file/path**: `internal/resources/rule_matching.go`
   - **issue**: For firewall rules, `sourceCountries` appears in an `or` source group, while the note says that when `excludeSrcCountries` is true, `sourceCountries` lists countries the rule does not apply to.
   - **why it matters**: The structured group alone can still be misleading if a consumer reads `matching.groups` without reading notes. The new logic sentence mitigates this, but the representation remains non-conditional.
   - **verify**: Inspect the firewall definition’s source group and note.
   - **test that would catch it**: A future test could fail if a field appears in a positive group while a note describes conditional exclusion semantics, unless the group or field carries conditional/negative metadata.

3. The parenthetical examples in the logic sentence are narrow.

   - **file/path**: `internal/resources/rule_matching.go`
   - **issue**: The sentence says notes describe “an exclusion flag or a validity window”, but `excludedUsers`, `excludedGroups`, and `excludedDepartments` are exclusion fields rather than flags.
   - **why it matters**: This is wording risk, not a contract failure, because the test requires those fields to be explained by notes.
   - **verify**: Search notes for `excludedUsers`, `excludedGroups`, and `excludedDepartments`.
   - **test that would catch it**: A wording test could require notes to use a consistent modifier vocabulary.

## Machine Contract Review

- `RuleMatchingLogic` changed. This is an intentional machine-readable contract delta.
- `zia/forwarding-rules` and `zia/ips-policies` changed:
  - `resCategories` removed from `destination` groups.
  - `resCategories` added to `unplaced_criteria`.
- `zia/cloud-app-control-rules` gains a note for `enforceTimeValidity`.
- The added test strengthens the contract by tying catalog modifier fields to notes.
- No evidence in the diff that generated docs, schemas, goldens, or fixtures were regenerated. Since the handoff says tests and verify scripts pass, treat this as likely in sync, but confirm if generated artifacts are derived from these definitions.

## Safety Review

- No redaction, projection, sensitive-data classification, or output-narrowing behavior changed.
- Moving `resCategories` from documented groups to `unplaced_criteria` is conservative and does not widen output.
- No sensitive field is newly exposed.

## Generated Artifact Review

- No generated files appear in the diff.
- If `RuleMatchingLogic`, notes, groups, or `unplaced_criteria` feed generated schemas, docs, or goldens, those artifacts should be regenerated and reviewed.
- The diff itself explains the source deltas: notes, logic text, and `resCategories` placement.

## Verdict

Verdict: approve with nits
