# Builder Handoff

## Intent

fix(resources): drop catalog paths no reader can emit

Running every resource's record mapper on fully populated samples found 38
catalog paths that could never render; `schema list` advertised them and the
`--fields` preflight accepted them.

- `parent` under every id/name reference (35 paths): modeled as an
  `{id, name}` object, but the SDK field is a string and no saved record has
  one. Removed, with the matching reference-table rows.
- zcc/company-info `devicePostureFrequency`: a list of objects modeled as a
  leaf, so projection dropped it. Sub-fields are now modeled.
- zcc/company-info `webAppConfig` (about 185 portal flags) and
  zia/ips-signature-rules `ruleText` (never mapped; tests pin that): now
  deliberate exclusions instead of catalog entries.

## Base / Head

- Base: `integration/2026-10-09` at `07d54f1`.
- Reviewed head: `c33062c`.
- Final head: review-nit fixes and this artifact.

## Files Changed

- `internal/resources/resources.go`, `catalog_zcc.go`, `catalog_zia.go`,
  `references_zcc.go`, `references_zia.go`, `references_zia_rules.go`,
  `references_ztw.go`
- `internal/zscaler/reader_zia.go` (comment), `schema_zcc_test.go`,
  `schema_zia_test.go`, `reader_fields_ips_signature_rules_test.go`,
  `reader_fields_zcc_company_info_test.go` (new)
- `docs/RESOURCES.md`, generated `docs/FIELD_COVERAGE.md` and
  `docs/field-coverage.json`

## Tests Run

- `go test ./...`, `verify-docs`, `verify-cli-docs`,
  `verify-surface-changes-manifest`, `verify-machine-contract`,
  `verify-typescript-client`, `verify-script-registry`, `gofmt`: pass.
- Mutation: rendering `postureName` in paranoid fails the new mode test.
- Live, non-production tenant, share mode: `zcc company-info show` returns 25
  `devicePostureFrequency` entries, which were dropped before.

## Known Gaps

- The invariant that every non-secret catalog path is emitted by its mapper
  is not yet a committed test. It reuses the sampler in the pending
  field-types change and lands after it.

## Review History

Strata on gm01 remained stopped, so the review ran in a fresh-context Claude
subagent with no conversation history and read-only repository access.

- Round 1, approve with nits (below). Acted on: an SDK shape review for
  `zcccompany.DevicePostureFrequency`; a per-mode test for
  `devicePostureFrequency` and `webAppConfig`; the `webAppConfig` reason
  reworded to what is known (portal toggles plus some client settings, too
  broad to classify flag by flag); stale "secret" comments in the IPS tests.

# Adversarial Review

Fresh-context reviewer: Claude subagent (no conversation history, read-only repository access), 2026-10-16 (round 1)

## Blocking Findings

None. Only ZIA `common.IDName` carries `Parent`, as a string; the projector
already dropped it under the object-shaped spec, and an existing test pins
"no parent", so removing the path changes no output. The 35 removed
reference rows are exactly the `.parent` rows. A probe through all three
modes showed `devicePostureFrequency` keeps eight sub-fields in standard and
share and drops `postureName` in paranoid; `webAppConfig` is absent in every
mode.

## Non-Blocking Risks

1. minor: no SDK shape review for `DevicePostureFrequency`. Added.
2. minor: no committed per-mode test. Added.
3. minor: the `webAppConfig` reason overclaimed "not tenant policy".
   Reworded.
4. nit: stale "secret" comments for `ruleText`. Fixed.

## Verdict

Verdict: approve with nits
