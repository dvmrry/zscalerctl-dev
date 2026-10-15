# Builder Handoff

## Intent

feat(zia): read locations-lite so predefined locations resolve

Rule `locations` references can name predefined locations such as Road Warrior
(id -3; the operator: users connected through Zscaler Client Connector), which
GET /locations does not return, so `refs check` reported them as not found.
GET /locations/lite returns them, so `zia locations-lite` becomes a list
resource instead of a declared gap, and rule `locations` reference targets gain
it. The operator approved showing what an id is ("just showing the config in a
way that makes sense"), not modeling what it does.

## Base / Head

- Base: `integration/2026-10-09` at `27797bb`.
- Reviewed head: `0c3d6e9`.
- Final head: review-nit fixes and this artifact on top of `0c3d6e9`.

## Files Changed

- `internal/resources/catalog_zia.go`, `gaps_table.go`, `references_zia.go`,
  `references_zia_rules.go`, `refs.go`
- `internal/zscaler/reader.go`, `reader_zia.go`, `schema_zia_test.go`
- tests: `locations_lite_test.go`, `reader_locations_lite_test.go`, gap,
  reference, refs and introspect tests
- `docs/RESOURCES.md`, `docs/FIELD_COVERAGE.md`, `docs/field-coverage.json`,
  goldens, `surface_changes.md`, `AGENTS.md`, `skills/zscalerctl/SKILL.md`
  (+ `.agents` copy), `docs/cli/agent-machine-workflow.md`

## Tests Run

- `go test ./...`, `gofmt -l`, `go vet`, `verify-docs`, `verify-cli-docs`,
  `verify-surface-changes-manifest`, `verify-machine-contract`,
  `verify-typescript-client`: pass.
- Live, non-production tenant: `zia locations-lite list` returned 3 records,
  including `{"id": -3, "name": "Road Warrior"}`; `refs check zia
  url-filtering-rules` no longer reports the 8 references to -3 (remaining:
  one source IP group, 135 `lastModifiedBy` references to 2 admin ids).

## Known Gaps

- `zia location-lite` (the removed gap's alias) now gets a plain unsupported
  resource error.
- If `/locations/lite` cannot be read (permissions, or a dump saved before
  this resource existed), `refs` lists every rule `locations` field as
  `target_not_checked` rather than comparing against the other two lists.

## Review History

Strata on gm01 remained stopped, so the review ran in a fresh-context Claude
subagent with no conversation history and read-only repository access.

- Round 1, approve with nits (below). Acted on after review: the false "SDK has
  no model" claim (comment corrected, unread SDK keys documented); the mirror
  test now pins `zappSslScanEnabled` against `zappSSLScanEnabled`; `refs`
  treats zero as unset, fixing a false "not found" for `parentId: 0`; stale
  two-target wording in docs and test messages.

# Adversarial Review

Fresh-context reviewer: Claude subagent (no conversation history, read-only repository access), 2026-10-15 (round 1)

## Blocking Findings

None. I checked every candidate below against the code or a test, and none rises to blocking:

- **Mode leaks:** none. Every lite field uses the same classification and modes as on zia/locations (`TestLocationsLiteFieldsMirrorLocations`). Only `id` renders in paranoid. In share, the rule `locations` ids and the lite ids are both renderable, so the comparison in `targetIDs` is valid.
- **Invented false/0:** none for booleans or `parentId`. They are `*bool`/`*int64` and are emitted only when non-nil (`liteLocationSourceRecord`). The test shows an omitted `ipsControl`/`parentId` stays absent and an explicit `false`/`0` stays present.
- **Pagination:** `getZIAAllPages[ziaLiteLocation]` goes through `ziaWalkPages`. That covers short pages, page-ignoring endpoints (probe), repeated pages, and duplicate identities. `paginationRecordIdentity` treats `-3` as a real identity, because only `0`/`null`/`""` are identity-less. `int64` decodes `-3` exactly, and `refIDKey` normalizes int/int64/float64/json.Number, so `-3` in a rule matches `-3` in lite. The scratch test confirmed a rule with `locations:[{id:-3},{id:101}]` now reports nothing not found.
- **Wrong-list matching:** none. `CheckRefs` takes the union of target ids. Location and sublocation ids share one namespace, and lite is a subset of locations plus predefined ids, so adding it cannot produce a false match.

## Non-Blocking Risks

1. **The "SDK has no model" claim is false.**
   - Where: `internal/zscaler/reader_zia.go:1675` comment, and the commit message.
   - Issue: zscaler-sdk-go v3.8.55 ships `zia/services/location/locationlite.LocationLite` for `/zia/api/v1/locations/lite`.
   - The custom struct is still the right choice. The SDK model uses plain `bool` and `ParentID int` with omitempty, so it would invent `false`/`0`. The comment should give that as the reason instead of claiming no model exists.
   - Fields silently dropped (fail-closed, so not a leak): the SDK model documents keys that `ziaLiteLocation` does not decode: `other6SubLocation`, `ipv6Enabled`, `subLocScopeEnabled`, `subLocScope`, `subLocScopeValues`, `subLocAccIds`, plus `city` under `enableIOT`. These are dropped at decode with no unmodeled-key signal.
   - Coverage overstated: FIELD_COVERAGE reports `locations-lite 19/19 100%` as "exported SDK fields". That is measured against a self-authored struct, so the reviewed-shape entry in `schema_zia_test.go` is tautological and will never catch drift.
   - Verified by reading `~/go/pkg/mod/github.com/zscaler/zscaler-sdk-go/v3@v3.8.55/zscaler/zia/services/location/locationlite/locationlite.go`.
2. **Same setting, two key spellings.**
   - Where: `internal/resources/catalog_zia.go:2331`, `internal/resources/locations_lite_test.go:276`.
   - Issue: zia/locations renders `zappSSLScanEnabled` (the SDK tag), while lite renders `zappSslScanEnabled` (the wire key). The mirror test skips this field instead of mapping it, so its classification is the one lite field that is not pinned to zia/locations.
   - Classification matches today: tenantConfig, standard and share.
   - Why it matters: an agent comparing the two views sees different keys for the same setting.
3. **`parentId: 0` is reported as a missing reference.**
   - Where: `internal/resources/references_zia.go:112`.
   - Issue: `refs check zia locations-lite` reports `parentId=0 not in zia/locations` for any top-level location where the API sends `parentId: 0`. That is because `refIDKey(0)` returns `"0", true`.
   - The scratch test reproduced it for lite. The same false positive already exists on `refs check zia locations`, since `locationSourceRecord` always emits the SDK `int` `parentId`.
   - Not introduced by this commit, but lite inherits it. The fix belongs in the refs model (treat 0 as "no reference" for parent-id fields, or skip zero ids), not here.
4. **When lite cannot be read, refs skips the `locations` field on all 14 sites.**
   - Where: `internal/resources/refs.go` (`CheckRefs`: "One unread target means a miss could be in it: compare nothing").
   - Issue: if `/locations/lite` fails to read, every rule `locations` field becomes `target_not_checked`. Causes include a role without access, a pagination-validation failure, or a `--from-dump` saved before this resource existed.
   - The scratch test confirmed it: `references_checked: 0`, with `locations-lite read_failed` and `locations target_not_checked` listed.
   - It is visible and fail-closed, which is correct by design, but it is a coverage regression in those cases. Older dumps lose all rule-location checks until re-dumped. This deserves a sentence in RESOURCES.md or SKILL.md.
5. **Stale agent-facing text.**
   - `skills/zscalerctl/SKILL.md:352` still says a field "whose ids may belong to either of two resources lists both (a ZIA rule's `locations` may name a location or a sublocation)". It now has three targets.
   - Test failure messages still say two targets: `internal/resources/references_test.go:126` ("want zia/locations and zia/sublocations") and `internal/cli/commands_schema_references_test.go:66` ("with both targets").
6. **Absent `id` renders as 0.** `ID int64` is a plain value, so an absent `id` would render as `0`. This matches every other reader, and the API always sends an id, so it is a nit.
7. **The `zia location-lite` alias is gone.** Removing the gap also removed its alias, and resource specs have no alias field. `zia location-lite list` now returns a generic "unsupported resource" error instead of pointing at `zia locations-lite`. Minor, given the pre-1.0 stance.
8. **No handler-level test.** Only `liteLocationSourceRecord` is unit-tested. The endpoint path and paginator wiring are exercised only by the one live read.
9. **What the default lite view returns.** Per the SDK `GetAllFilterOptions` docs, the unfiltered `/locations/lite` excludes sublocations and parent locations that have sublocations. RESOURCES.md already says it is "not a superset". Stating which records the default view excludes would let an agent not misread an absent location as unconfigured.

## Verdict

Verdict: approve with nits
