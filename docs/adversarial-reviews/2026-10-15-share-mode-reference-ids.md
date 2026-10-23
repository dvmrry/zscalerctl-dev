# Builder Handoff

## Intent

feat(resources): reference ids render in share mode; names keep their modes

Rule scope (users, groups, departments, locations, location groups, devices,
IP groups, labels, override/excluded users and groups, and the other ZIA/ZTW
`{id, name, extensions}` reference lists) was standard-only, so in the
default share mode every rule looked unconditional. The operator decided
reference ids are fine to share. `idNameExtensionsField`'s mode argument now
bounds the referenced object's name; the list itself also renders in share
mode, so its ids do. Extensions stay secret; paranoid is unchanged.

## Base / Head

- Base: `integration/2026-10-09` at `2fdcef8`.
- Reviewed head: `ee3c6ec` (round 1; an earlier attempt exhausted its budget
  without a review).
- Final head: this artifact on top of `ee3c6ec`.

## Files Changed

- `internal/resources/resources.go` (`idNameExtensionsField`, `withShareMode`)
- `internal/resources/references_test.go`,
  `internal/cli/commands_schema_references_test.go`
- 11 reader test files under `internal/zscaler/`
- `skills/zscalerctl/SKILL.md` (+ `.agents` copy),
  `cmd/zscalerctl/testdata/surface/surface_changes.md`

## Tests Run

- `go test ./...`, `gofmt -l`, `go vet`, `verify-docs`, `verify-cli-docs`,
  `verify-surface-changes-manifest`, `verify-machine-contract`,
  `verify-typescript-client`: pass.
- Live, default share mode: `zia url-filtering-rules list`: 115 of 153 rules
  show scope criteria as id-only elements (the other 38 are the rules with no
  criteria); no element carries a name; every reference id is an integer
  (users 54, groups 119, location groups 75, locations 17, source IP groups
  13, device groups 4, header action profiles 1). The stderr notice lists
  the omitted `*.name` fields.

## Review History

Reviewed by Strata (Qwen3.8-Flash-Next IQ3_S on gm01), no repository access.

- Round 1, approve with nits (below). Its evidence gap (whether any id value
  is itself identifying) was closed with the live id-type check above.

# Adversarial Review

Fresh-context reviewer: Strata (Qwen3.8-Flash-Next IQ3_S, gm01), read-only, no command execution, 2026-10-15 (round 1)

## Blocking Findings

None.

The changed helper does not, by itself, put a `name` into share mode unless the caller explicitly passes share in `nameModes`. The live non-production sample and the updated reader tests show share-mode reference elements as id-only, with no `name` present. For standard-only reference lists, the parent list is widened to `standard, share`, while the child `name` remains standard-only, and paranoid remains absent unless `nameModes` includes paranoid.

The main unresolved issue is evidence coverage, not a demonstrated leak: the non-test diff does not include the ~208 call sites, generated schema/golden artifacts, or id-value samples for all reference-list paths. That is a review gap, but it is not a concrete blocking finding unless a call site is shown passing `share` to `nameModes` or an id value is shown to be an email/hostname/other identifying value.

## Non-Blocking Risks

- `internal/resources/resources.go`
  - `idNameExtensionsField` trusts `nameModes` exactly. If any call site passes `standardShareModes()` or another list containing `redact.ModeShare`, the child `name` will render in share mode.
  - This is source-verifiable from the helper:
    ```go
    AllowedModes: withShareMode(nameModes),
    ...
    tenantConfigField("name", nameModes),
    ```
  - Reproduce/verify: inspect every call site of `idNameExtensionsField`, or add a table-driven test over all reference-list schema paths asserting that `name` is share-eligible only for explicitly allowlisted paths.
  - Test that would catch it: a schema/redaction test that walks all `idNameExtensionsField` paths and asserts either:
    ```go
    !contains(field.name.allowedModes, redact.ModeShare)
    ```
    for normal reference lists, or documents the exact exceptions.

- `internal/resources/resources.go`
  - `operationalField("id", allModes())` makes the leaf `id` mode-eligible everywhere, relying on parent-field gating to hide it in modes where the reference list is not allowed.
  - This is fine if the redaction engine intersects parent and child modes, but it means the machine contract depends on parent gating being correct.
  - Reproduce/verify: dump a standard-only reference list with `--redaction paranoid` and assert the whole list is absent, not merely that names are absent.
  - Test that would catch it: a paranoid dump test for a known reference list where the list itself is not paranoid-allowed, asserting no `id` keys appear.

- `skills/zscalerctl/SKILL.md`
  - The wording says “names, emails and other identifying values need `--redaction standard`”. That is accurate for `name` and secret projections, but it does not explicitly state that reference ids are intentionally visible in share mode for every reference list.
  - This is not a leak by itself, but it could mislead reviewers if a future reference id is email-like or hostname-like.
  - Reproduce/verify: run `zscalerctl --format json schema references zia url-filtering-rules` and confirm the documented id-only share shape matches schema output.
  - Test that would catch it: a docs/golden test comparing SKILL examples to schema output for at least one reference-list path.

## Machine Contract Review

- JSON shape change is intentional: reference-list elements in share mode now render as id-only objects.
- Schema/introspection modes for these reference-list paths change from standard-only to standard+share.
- Exit-code behavior is not changed by the diff.
- Error envelopes are not changed by the diff.
- NDJSON/dump/diff behavior follows the redaction mode change: share output can now contain reference ids where it previously omitted the whole list.
- The SKILL delta documents the new share-mode id-only reference behavior.
- Review gap: the diff does not show generated schema goldens or fixture deltas. Provided tests pass, but the reviewer cannot see the corresponding generated artifacts in the non-test diff.

## Safety Review

- `name` is bounded by `nameModes`; it is not unconditionally added to share by the helper.
- `extensions` remains an unstructured/secret projection and is not widened by the parent list mode change.
- `id` is now visible in share for reference lists because the parent list is widened to include share.
- For the observed ZIA URL-filtering-rules sample, share-mode criteria elements contain only `id`, with zero `name` values.
- Paranoid behavior is unchanged for standard-only call sites: the helper adds share, not paranoid. If a call site already passed paranoid, name visibility in paranoid could change because the child `name` now uses `nameModes` instead of the prior fixed `standardShareModes()`. No such call site is shown in the diff.

## Generated Artifact Review

- `skills/zscalerctl/SKILL.md` was intentionally updated to describe the new share-mode reference-id behavior.
- The SKILL wording is broadly consistent with the change.
- No other generated docs, schemas, or goldens appear in the non-test diff. If generated schema goldens exist outside test files, they should be included or their absence explained.

## Verdict

Verdict: approve with nits
