# Builder Handoff

## Intent

test(catalog): fail on catalog paths no reader emits

A catalog path no reader fills is listed by `schema list` and accepted by
`--fields`, `--filter` and `--count-by`, yet always comes back empty, so an
agent reads "no value" where the field cannot be read at all. The previous
change removed 38 such paths (a `parent` child under every `{id, name}`
reference, ZCC `webAppConfig`, IPS `ruleText`). `TestCatalogPathsAreEmitted`
keeps it from recurring: it runs every handler's record mapper on the
synthetic samples the field type generator uses and requires each field that
renders in any mode, nested ones included, to appear in at least one sample
projected in standard mode. Secret fields render in no mode and are skipped.

## Base / Head

- Base: `integration/2026-10-09` at `6244478`.
- Reviewed head: working tree on `test/catalog-paths-emitted` (round 1).
- Final head: round-1 nit fixes and this artifact.

## Files Changed

- `internal/zscaler/catalog_paths_emitted_test.go` (new)
- `internal/zscaler/field_types_test.go` (`sampleCatalog` extracted from
  `buildFieldTypes`; generated output unchanged)

## Tests Run

- `go test ./...`, `go vet ./internal/zscaler`, `gofmt -l`: pass;
  `TestFieldTypesAreCurrent` passes with `field_types_gen.go` untouched.
- Mutations: a `parent` child under `idNameField` fails 22 paths; a
  share-only child fails 35; a bogus top-level field fails with its path.

## Known Gaps

- A path the reader fills only from particular content (a branch the samples
  never take) would fail and need sampling or a reasoned skip; none exists
  today.

## Review History

Strata on gm01 remained stopped, so the review ran in a fresh-context Claude
subagent with no conversation history and read-only repository access.

- Round 1, approve with nits (below). Acted on: the check now covers fields
  rendering in any mode, not only standard; `sampleCatalog` has a doc
  comment; the redundant secret check went with the mode change.

# Adversarial Review

Fresh-context reviewer: Claude subagent (no conversation history, read-only repository access), 2026-10-17 (round 1)

## Blocking Findings

None. All 166 catalog specs have a sampled handler, so none is skipped. The
projector copies only source keys (structured leaves, childless parents and
unconfirmed empty lists drop), so presence after projection is not vacuous.
One always-nil path (`zcc/application-profiles/onNetPolicy`, an
`interface{}` SDK field) is genuinely mapped.

## Non-Blocking Risks

1. minor: only standard-mode fields were checked, so a share-only or
   paranoid-only field would pass unemitted. Fixed.
2. nit: the secret check duplicated `AllowedIn`. Removed.
3. nit: `sampleCatalog` lacked a doc comment. Added.

## Verdict

Verdict: approve with nits
