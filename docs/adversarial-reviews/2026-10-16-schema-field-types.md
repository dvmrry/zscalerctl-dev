# Builder Handoff

## Intent

feat(schema): give schema list fields a JSON type

Each catalog field, nested ones included, may carry `type` (string, integer,
number, boolean, object, or array<T>) in `schema list`, so an agent knows
whether `groups` is a list before writing a filter, `--count-by` or a jq path.
The type is what zscalerctl emits, observed rather than declared:
`TestFieldTypesAreCurrent` runs every resource handler's own record mapper on
synthetic SDK values, projects the records through the catalog, and types a
path only when every sample that emits it agrees. It writes
`internal/resources/field_types_gen.go`; plain `go test` fails on drift, and
`make field-types` regenerates. Secret fields, values the reader derives from
content, structured values under catalog leaves, and `interface{}` SDK fields
stay untyped.

Also fixes the skill's `--fields` preflight, which compared requested names
with top-level fields only and so rejected the dotted paths merged earlier
today.

## Base / Head

- Base: `integration/2026-10-09` at `716142e`; rebased onto `b28f7b6`, then `4259208` (no
  generated-type change).
- Reviewed head: staged tree on `feat/schema-field-types` (round 2).
- Final head: round-2 nit fixes and this artifact.

## Files Changed

- `internal/resources/resources.go` (`FieldSpec.Type`), `field_types.go`,
  `field_types_gen.go` (generated), `field_types_test.go`
- `internal/zscaler/field_types_test.go` (generator, test-only sampling
  methods on the three generic handlers)
- `Makefile` (`field-types`), `AGENTS.md`, `skills/zscalerctl/SKILL.md`
  (+ `.agents` copy)

## Tests Run

- `go test ./...`, `verify-docs`, `verify-cli-docs`,
  `verify-surface-changes-manifest`, `verify-machine-contract`,
  `verify-typescript-client`, `verify-script-registry`, skill sync: pass.
- 4,574 catalog paths typed. Checked against a saved dev-tenant dump: 0
  mismatches over 1,684,131 values (redaction markers aside).
- The types agree with every hand-reviewed type in `schema describe`.

## Known Gaps

- The engine stdio catalog (`enginewire.CatalogField`) does not carry `type`;
  its schema is unchanged.
- Catalog paths that never render (the `parent` object under references,
  found by this sampler) were removed in a separate change before this one
  landed; a committed invariant for that follows this change.

## Review History

Strata on gm01 remained stopped, so the review ran in a fresh-context Claude
subagent with no conversation history and read-only repository access.

- Round 1, request changes. Types were derived from SDK structs, but many
  readers are hand-written mappers: `ztw/ec-groups/ecVMs` was typed
  `array<object>` while the reader emits a count. Also typed: leaves the
  projector drops, `.parent` strings with nested children, and secret fields.
  The tests compared the generator only with itself. Acted on: the generator
  now samples the real mappers and projects them, so it is pinned by reader
  output; `TestFieldTypesFitCatalogShape` enforces that a field with nested
  fields is untyped, object or array<object>; pins cover ecVMs and untyped
  cases; docs reworded.
- Round 2, approve with nits (below). The reviewer fuzzed all 166 handlers
  with 400 random samples each (nil/empty/1–3 element lists and maps, nil
  pointers, varied ints, floats and strings including 6,887 literals from the
  reader and catalog source, interface fields set to each kind) in all three
  modes: no kind mismatch on any typed path. Acted on: docs say any typed
  field may be `null`; the generator fails if a mapper yields no sample.

# Adversarial Review

Fresh-context reviewer: Claude subagent (no conversation history, read-only repository access), 2026-10-16 (round 2)

## Blocking Findings

None. All round-1 findings re-verified as fixed.

## Non-Blocking Risks

1. minor: a typed field can still be `null` (nil pointers and slices; 8 paths
   observed), which the sampler never produces. Docs now say so.
2. nit: `mapSample` swallowed panics silently. The generator now fails when
   a mapper yields no sample.
3. nit: the `.parent` catalog modeling bug remains (out of scope; listed in
   Known Gaps).

## Verdict

Verdict: approve with nits
