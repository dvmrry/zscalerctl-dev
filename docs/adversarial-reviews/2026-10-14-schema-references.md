# Builder Handoff

## Intent

feat(schema): catalog reference targets and `schema references`

Three fresh LLM agents drove the CLI against a live non-production tenant
(2026-10-14). None of them could answer "which rules use this location" or
"which server groups does this connector group serve" without reading every
resource and guessing which id field meant what. Reference fields now carry
their targets in the catalog (`FieldSpec.References`, a list, rendered by
`schema list`). `schema references <product> <resource>` is the config-free
reverse index: each entry names the referring resource and field, the
`--filter` path that holds the id (`match`), the modes that render it, and
every resource the field's ids may belong to (`targets`). A reference no mode
renders is listed with no match and no modes.

Targets live in per-product tables (`references_<product>.go`), each checked
against the vendored SDK. Reference-shaped fields with no verified target are
listed in `unmappedReferences` with the reason. A test keeps every
reference-shaped field in the catalog, nested ones included, in one of the
two tables. The semantics pilot takes its reference from the catalog and
reports one only when a field has a single target.

A ZIA rule's `locations` (and a location group's members) may name a
sublocation, so those fields target both `zia/locations` and
`zia/sublocations`. ZIA's `zpaAppSegments.externalId` and
`zpaServerGroup.externalId` are the ZPA-side ids (per the SDK) and target
`zpa/application-segments` and `zpa/server-groups`.

While checking the reverse index's match paths, a filter bug turned up. A
top-level `--filter` compared the whole rendered value, so
`--filter urlCategories=X` missed every rule listing more than one category.
A top-level list now also matches when any one element equals the value. That
only adds matches: every whole-value match still matches.

## Base / Head

- Base: `integration/2026-10-09` at `f1b7625`.
- Reviewed heads: `c3b2d7c` (round 1), `406ba65` (round 2).
- Final head: the round-2 nit commit plus this artifact.

## Files Changed

- `.agents/skills/zscalerctl/SKILL.md`
- `AGENTS.md`
- `README.md`
- `cmd/zscalerctl/testdata/surface/introspect-pretty.stdout.golden`
- `cmd/zscalerctl/testdata/surface/introspect.stdout.golden`
- `cmd/zscalerctl/testdata/surface/inventory.golden`
- `cmd/zscalerctl/testdata/surface/schema-bare.stderr.golden`
- `cmd/zscalerctl/testdata/surface/schema-help.stdout.golden`
- `cmd/zscalerctl/testdata/surface/surface_changes.md`
- `docs/cli/agent-machine-workflow.md`
- `docs/cli/zscalerctl.md`
- `internal/cli/commands_config_schema_auth.go`
- `internal/cli/commands_schema_references.go`
- `internal/cli/commands_schema_references_test.go`
- `internal/cli/introspect_test.go`
- `internal/cli/ndjson_policy_test.go`
- `internal/resources/references.go`
- `internal/resources/references_test.go`
- `internal/resources/references_zcc.go`
- `internal/resources/references_zia.go`
- `internal/resources/references_zia_rules.go`
- `internal/resources/references_zidentity.go`
- `internal/resources/references_zpa.go`
- `internal/resources/references_ztw.go`
- `internal/resources/resources.go`
- `internal/resources/resources_test.go`
- `internal/resources/semantics.go`
- `internal/resources/semantics_test.go`
- `man/zscalerctl.1`
- `skills/zscalerctl/SKILL.md`
- `cmd/zscalerctl/testdata/surface/*.golden` (schema-help, schema-bare, inventory, introspect, introspect-pretty)

## Tests Run

- `go test ./...` (Go 1.26.9, `/bin/chmod` first in `PATH` for the macOS ACL
  tests), `verify-docs`, `verify-cli-docs`,
  `verify-surface-changes-manifest`, `verify-machine-contract`: pass.
- No live run: the live key was unavailable. Whether rule `locations` in
  this tenant actually holds sublocation ids has not been checked. The
  two-target model is correct either way.

## Known Gaps

- The catalog has no field or key types, so a mapping whose id type cannot
  match the target (integer vs. UUID) is caught only by review. Such
  demotions are pinned by `TestDemotedReferencesStayUnmapped`.
- `references` metadata documents relationships that are classified secret
  (most ZPA relationship ids); reclassifying them is separate work.

## Review History

Reviewed with no repository access by a fresh-context local model given the
diff, the model code in full, a sample of the tables, and this repository's
reviewer prompt and template: Strata serving Qwen3.8-Flash-Next (IQ3_S) on
gm01. The reviewer could not run commands. A first attempt that included every
table entry spent its whole budget walking the tables and produced no review;
later rounds sent the model code in full and a table sample.

- Round 1, request changes. Blocking: (1) rule `locations` claimed
  `zia/locations`, which lists parent locations only, though the field may
  name sublocations. Fixed by multi-target references. (2) The coverage test
  stopped at reference objects, and the tables had no check that a target can
  hold the field's ids. The coverage descent is fixed; the 29 nested fields it
  found are classified, and demoted mappings are pinned. A type check is not
  added because the catalog has no type model. Non-blocking: the `schema
  describe` delta is now in `surface_changes.md`. Flat-list `match` paths
  assumed membership filtering, which `--filter` did not do; fixed in
  `--filter`. The no-mode text no longer says "secret". `pretty` stays text,
  as for `schema describe`.
- Round 2, approve with nits (below). Applied: top-level list matching is one
  level deep, as documented; a duplicate target is a table error; the
  cross-product mappings are pinned by a test; the docs say "a reference no
  mode renders".

Accepted and not changed: the `--filter` help line does not mention list
membership; the man page, README and agent docs do. Changing it would touch
17 surface goldens for one clause. `schema list`'s `references` shape changes
only within this unreleased branch.

# Adversarial Review

Fresh-context reviewer: Strata (Qwen3.8-Flash-Next IQ3_S, gm01), read-only, no command execution, 2026-10-14 (round 2)

Assume the change is wrong. Do not implement fixes. Find concrete ways it
could lie, drop data, overclaim, weaken tests, bypass redaction, change machine
contracts, or silently change generated artifacts.

## Blocking Findings

None.

The two round-1 blockers are fixed in a way I can verify from the diff and
tests:

1. **ZIA `locations` overclaim fixed by multi-target references**

   - `FieldSpec.References` is now `[]ResourceRef` in
     `internal/resources/resources.go`.
   - ZIA rule `locations` fields and `zia/location-groups.locations` now map to
     both `zia/locations` and `zia/sublocations`:
     - `internal/resources/references_zia.go`
     - `internal/resources/references_zia_rules.go`
   - `ReferencesTo` now includes a site if the queried target is contained in
     the field’s target list:
     ```go
     if !slices.Contains(field.References, target) {
         return
     }
     ```
   - `ReferenceSite` now carries `Targets`, so `schema references` does not
     imply a single target:
     ```go
     Targets []ResourceRef `json:"targets"`
     ```
   - Tests pin the important behavior:
     - `TestCatalogFieldsCarryReferences` expects
       `url-filtering-rules.locations` to have `zia/locations` and
       `zia/sublocations`.
     - `TestReferencesToListsMatchPathsAndModes` expects the same field to appear
       under both reverse-index targets and to carry both targets.
     - `TestSchemaReferencesTextNamesFilterOrSecret` checks the text output for
       `zia sublocations` includes:
       ```text
       zia url-filtering-rules	--filter locations.id=<id>	standard	id is one of zia/locations, zia/sublocations
       ```
     - `TestDescribeSemanticsReferenceTargetsAndCollectionOrdering` now expects
       `url-filtering-rules.locations` to have no single reference target.

   The pilot’s single-target rule is also consistent: `DescribeSemantics` only
   reports a reference when `len(field.References) == 1`.

2. **Reference coverage descent fixed and demotions pinned**

   - `referenceShapedPaths` no longer stops at a reference object. It appends a
     reference-shaped path and then descends into children:
     ```go
     if hasID || (idNamed && len(field.Fields) == 0) {
         out = append(out, path)
     }
     out = append(out, referenceShapedPaths(field.Fields, path+".")...)
     ```
   - The diff adds unmapped entries for many newly discovered nested
     `externalId`/`zpaId` children, so the expanded coverage does not silently
     drop fields.
   - `TestDemotedReferencesStayUnmapped` pins the round-1 demotions:
     - `zia/url-filtering-rules.cbiProfileId`
     - `zia/casb-dlp-rules.cloudAppTenants`
     - `zia/casb-malware-rules.cloudAppTenants`
     - `zia/casb-malware-rules.cloudAppTenantIds`
     - `ztw/traffic-dns-rules.zpaIpGroup`

   This directly addresses the round-1 coverage hole where nested references
   inside reference objects were unchecked.

## Non-Blocking Risks

1. **Top-level list filter matching is broader than the prose in some edge
   cases**

   - File: `internal/resources/resources.go`
   - Concrete issue:
     ```go
     values = append([]any{value}, projectedPathValues(value, nil)...)
     ```
     For a top-level list, this adds:
     - each element of a flat list;
     - each object element of a list of objects;
     - leaf values from nested lists because `projectedPathValues` recurses into
       `[]any`.

     For a list of lists, the code matches leaf values but not the inner list
     as an element. For example, if a projected field were:
     ```json
     {"matrix": [["a", "b"], ["c"]]}
     ```
     then `--filter matrix=c` matches, but `--filter matrix=["c"]` does not match
     the inner list element. The docs say “a top-level list also matches when any
     one element does,” which is not exactly what the code does for nested lists.

   - Why it matters:
     The new behavior is useful for flat id lists such as `urlCategories`, but
     the implementation is more general than the documented rule. If any catalog
     field renders a list of lists, the CLI could match a leaf where the docs
     imply element matching, or fail to match an inner-list element where the
     docs imply it should match.

   - How to reproduce or verify:
     Add a synthetic projected record with a top-level list of lists and run
     filter cases:
     ```go
     records := []map[string]any{
         {"matrix": []any{[]any{"a", "b"}, []any{"c"}}},
     }
     ```
     Then test:
     ```go
     ProjectedFilter{Field: "matrix", Value: "c"}
     ProjectedFilter{Field: "matrix", Value: "[\"c\"]"}
     ```
     The first currently matches because of recursive flattening; the second
     does not match the inner list element.

   - Test that would catch it:
     A table test in `internal/resources/resources_test.go` that defines whether
     top-level list matching is “one level only” or “recursive leaf matching.”
     If the intended contract is one-level element matching, add a case for a
     list-of-lists field and assert the inner list element behavior.

2. **CLI help/surface documentation may not fully reflect the filter behavior**

   - File:
     - `man/zscalerctl.1`
     - `README.md`
     - `docs/cli/agent-machine-workflow.md`
     - `skills/zscalerctl/SKILL.md`
     - `cmd/zscalerctl/testdata/surface/surface_changes.md`
   - Concrete issue:
     The man page and user docs now say top-level lists also match when any one
     element does. However, the existing `surface_changes.md` help row still
     says:
     ```text
     Top-level keys keep whole-value matching.
     ```
     That remains true but is now incomplete. The diff also does not show a
     cobra help-text change or a help golden update.

   - Why it matters:
     The primary user is an LLM agent. If `--help` or generated help goldens
     still describe only whole-value matching, agents may not know that
     `--filter urlCategories=X` now matches membership in a list.

   - How to reproduce or verify:
     Run the help command and inspect the `--filter` description:
     ```sh
     zscalerctl zia url-filtering-rules list --help
     ```
     Also inspect surface goldens for `--filter` help text.

   - Test that would catch it:
     A surface/help golden or test asserting the help text includes something
     like:
     ```text
     a top-level list also matches when any one element does
     ```

3. **`schema list` JSON `references` changed from object to array; baseline needs
   confirmation**

   - File: `internal/resources/resources.go`
   - Concrete issue:
     `FieldSpec.References` changed from:
     ```go
     References *ResourceRef `json:"references,omitempty"`
     ```
     to:
     ```go
     References []ResourceRef `json:"references,omitempty"`
     ```
     So `schema list --format json` emits:
     ```json
     "references": [{"product": "zia", "resource": "locations"}, ...]
     ```
     instead of an object.

   - Why it matters:
     This is a machine-contract change for `schema list`. If `schema references`
     or the `references` key was already present on the process-doc baseline,
     this is breaking. If the entire schema-reference feature is still
     unreleased, it is acceptable as an in-PR contract change.

   - How to reproduce or verify:
     Confirm the process-doc baseline. If `origin/main` already contains
     `FieldSpec.references`, this needs a breaking-change note or compatibility
     path. If the feature is new on this branch, the updated
     `surface_changes.md` row is sufficient.

   - Test that would catch it:
     A `schema list --format json` golden or machine-contract fixture that pins
     the `references` shape. The absence of a golden change in this commit is
     only safe if no such golden exists or if the previous shape was never on
     the baseline.

4. **New cross-product mappings are not pinned by tests or visible source
   artifacts**

   - Files:
     - `internal/resources/references_zia.go`
     - `internal/resources/references_zia_rules.go`
   - Concrete issue:
     The commit adds mappings such as:
     ```go
     "zia/zpa-gateways": {
         "zpaServerGroup.externalId": zpaRef("server-groups"),
         "zpaAppSegments.externalId": zpaRef("application-segments"),
     },
     ```
     and many rule mappings:
     ```go
     "zpaAppSegments.externalId": zpaRef("application-segments"),
     ```
     The tests pin that these paths are classified and that their targets exist,
     but they do not pin that the target is semantically correct. For example,
     `FieldReferenceErrors` only checks that `zpa/application-segments` is a
     catalog resource; it does not verify the SDK relationship.

   - Why it matters:
     The round-1 blocker was an overclaim in the reference table. New mappings
     can reintroduce that class of bug. The builder says these were verified
     against the SDK, but the diff does not include the SDK artifact or a test
     that pins the specific semantic mapping.

   - How to reproduce or verify:
     Search for a test or artifact that asserts:
     ```go
     fieldReferences["zia/zpa-gateways"]["zpaAppSegments.externalId"]
     ```
     targets `zpa/application-segments` because the SDK documents that field as
     the ZPA-side id. I did not find such a pin in the diff.

   - Test that would catch it:
     A narrow table test for the newly added cross-product mappings, similar to
     `TestDemotedReferencesStayUnmapped`, would at least pin the intended targets
     and make accidental regressions visible. This is not blocking because the
     model is correct and the coverage test prevents silent classification
     holes, but it leaves the same hand-verification risk that caused round 1.

5. **Text output no longer says “secret,” but docs still use that wording**

   - File:
     - `internal/cli/commands_schema_references.go`
     - `AGENTS.md`
     - `docs/cli/agent-machine-workflow.md`
     - `skills/zscalerctl/SKILL.md`
     - `man/zscalerctl.1`
   - Concrete issue:
     Text output changed from:
     ```text
     secret; not rendered in any mode
     ```
     to:
     ```text
     not rendered in any mode
     ```
     The man page now says “A reference no mode renders is listed without a
     match path,” but `AGENTS.md`, the agent workflow doc, and the skill still
     say “A secret reference is listed with no match and no modes.”

   - Why it matters:
     Minor consistency issue. The JSON contract is clear: `match` omitted and
     `allowed_modes` empty. The text and docs should use the same concept.

   - How to reproduce or verify:
     Run:
     ```sh
     zscalerctl --format table schema references zia admin-users
     ```
     and compare the wording with the skill/docs.

   - Test that would catch it:
     A docs-sync test or a text-output test asserting the phrase “no mode”
     rather than “secret.”

6. **Duplicate targets are not mechanically rejected**

   - File: `internal/resources/references.go`
   - Concrete issue:
     `FieldReferenceErrors` checks that every target exists and that the list is
     not empty, but it does not reject duplicate targets such as:
     ```go
     ziaRef("locations", "locations")
     ```
     That would produce a `references` array with duplicate entries and text
     like:
     ```text
     id is one of zia/locations, zia/locations
     ```

   - Why it matters:
     Low likelihood from hand-edited tables, but it is a table-contract hole.
     The multi-target model should probably enforce distinct targets.

   - How to reproduce or verify:
     Temporarily add a duplicate target to a table and run:
     ```sh
     go test ./internal/resources
     ```
     It will not fail merely because the target is duplicated.

   - Test that would catch it:
     Add a check in `FieldReferenceErrors` that target slices have no
     duplicates.

## Machine Contract Review

- **`schema list` JSON**
  - `references` changed from object to array.
  - This is intentional for multi-target references.
  - If the feature is not already on the process-doc baseline, this is an
    in-PR contract refinement.
  - If it is already on the baseline, this is breaking and needs a compatibility
    note.

- **`schema references` JSON**
  - `referred_by[].targets` is added.
  - This is additive and useful.
  - A field with multiple targets now appears under each target query, and the
    `targets` array makes the ambiguity explicit.

- **`schema describe` semantics**
  - `reference` is now omitted for fields with multiple targets.
  - This is a contract narrowing from round-1 behavior, but it is the correct
    fix for the overclaim.
  - The delta is documented in `surface_changes.md`.

- **Text output**
  - `schema references` text now includes `id is one of ...` for multi-target
    fields.
  - No-mode references no longer say “secret.”
  - Text output is not the primary machine contract, but agents do read it.

- **`--filter` behavior**
  - Top-level list filters now match the whole rendered value and any element.
  - This is user-visible and documented in README, man, skill, and agent docs.
  - The implementation is broader than the prose in edge cases such as
    list-of-lists and list-of-objects exact matching.

- **NDJSON, error envelopes, exit codes**
  - No exit-code changes are visible.
  - No NDJSON or error-envelope changes are visible.
  - `schema references` still exits 4 for unknown resources according to the
    updated man page and prior contract.

## Safety Review

- **Redaction/projection**
  - `--filter` still runs after projection and redaction.
  - The new list-element matching uses projected values only.
  - It does not widen output or reveal omitted fields.
  - It can narrow list results more effectively, but only over data already
    rendered in the selected mode.

- **Sensitive data**
  - Reference metadata is static catalog data, not tenant values.
  - Multi-target references do not expose ids or names.
  - The new mappings may point agents to other product resources, but that is a
    catalog relationship, not a redaction bypass.

- **Filter validation**
  - The diff does not change filter-name validation.
  - Filters on omitted or secret fields should still fail before configuration
    or credentials are loaded, consistent with the existing README contract.

## Generated Artifact Review

- **Docs updated**
  - `AGENTS.md`
  - `README.md`
  - `docs/cli/agent-machine-workflow.md`
  - `skills/zscalerctl/SKILL.md`
  - `man/zscalerctl.1`

  These are in sync with the new multi-target and filter behavior.

- **Surface process doc updated**
  - `cmd/zscalerctl/testdata/surface/surface_changes.md`
  - The schema row now explains:
    - `references` array;
    - multiple targets;
    - `schema describe` no-single-reference behavior;
    - top-level list filter membership.

- **Generated goldens**
  - The diff shows no golden changes.
  - That is plausible if no golden covers the changed JSON/text outputs.
  - It is a review gap if `schema list`, `schema references`, or `--filter` help
    goldens exist. The builder’s reported `verify-cli-docs`,
    `verify-surface-changes-manifest`, and `verify-machine-contract` results are
    not proof by themselves, but the absence of golden diffs is not enough to
    block unless such goldens are known to exist.

## Verdict

Verdict: approve with nits
