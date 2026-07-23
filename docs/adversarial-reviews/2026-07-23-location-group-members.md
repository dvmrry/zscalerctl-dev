# Builder Handoff

## Intent

Fix `zia/location-groups` list and dump collection so associated locations and
sublocations are returned. The old list wiring explicitly sent
`fetchLocations=false`, even though the existing source adapter and catalog
already safely model reviewed member id/name references.

## Base / Head Reviewed

- Base: `36d700f1b5cf3ca6599a2a6fb9acdae0caeb1860`
- Initial reviewed head: `ade9a13c6148e9b56c9ef7e41a6819850045d5f8`
- Final reviewed head: `4587199f5c08bd954b5f0eaa1661d47657ab6b06`
- Branch: `feature/fix-location-group-members`
- Process baseline: `origin/main` at the base commit

## Files Changed

- `internal/zscaler/reader_zia.go`
- `internal/zscaler/reader_pagination_test.go`
- `internal/livesmoke/checks.go`
- `internal/livesmoke/checks_test.go`
- `internal/livesmoke/fake_runner_test.go`
- `internal/livesmoke/livesmoke_test.go`
- `docs/RESOURCES.md`
- `docs/LOCATION_GROUP_MEMBERS_VALIDATION.md`
- `docs/testdata/location-group-members-summary.jq`
- `scripts/testdata/test-location-group-members-validation.sh`
- `scripts/verify-docs.sh`

## Source Inputs Consulted

- Vendored SDK v3.8.38 location-groups implementation and
  `GetAllFilterOptions.FetchLocations` semantics.
- Vendored SDK common `ReadPage` and unbounded `ReadAllPages` implementations.
- Cached SDK v3.8.41 location-groups implementation; no relevant delta from
  v3.8.38.
- Existing ZIA bounded pagination helpers and location-group source adapter.
- Existing location-group catalog classifications and nested projection tests.
- Current live-smoke recursive deny checks and fake catalog fixtures.
- Zscaler's official Location Management API material, its 2025 release note
  for `fetchLocations`, and its location-group documentation describing
  independent parent-location and sublocation membership.

## Generated Artifacts

None changed. The resource catalog, schema, introspection, machine manifest,
field-coverage artifacts, CLI docs, surface goldens, and generated agent skill
copies are unchanged.

## Expected Delta

- `zia location-groups list` and focused/whole-product dump requests send
  `fetchLocations=true`.
- Collection walks one-based pages at the SDK-documented 1,000-record maximum
  and retains the existing 1,000-page fail-closed ceiling.
- Standard-mode records can now contain the already-cataloged `locations`
  array with member `id` and `name` only.
- Share and paranoid modes still omit `locations`.
- Nested SDK `extensions` maps and `lastModUser` remain denied and dropped.
- Direct get, dynamic-criteria classification, CLI syntax, JSON structure,
  schemas, exit codes, error envelopes, resource names, and all other readers
  remain unchanged.

## Invariants Claimed

- List and dump use the same handler and complete page walk.
- Raw SDK records still pass through source adaptation and fail-closed catalog
  projection before output.
- A later-page failure returns no partial slice.
- The reader preserves the API's flat id/name membership references and does
  not infer parent/child relationships.
- Context cancellation and request timeout propagation remain in the SDK
  client path.
- Live-smoke allowances are exact-path scoped and still reject nested
  extensions, admin identity references, and reviewed key names on wrong
  paths.
- The documented tenant check validates un-narrowed projected records before
  presenting a summary.
- Tests and documentation contain no credentials or tenant payloads.

## Builder Verification

The builder passed:

- Focused ordinary and race location-group pagination, projection, and
  live-smoke tests.
- Full ordinary and race Go suites and `go vet` after review fixes.
- Formatting, docs, executable documentation fixtures, generated CLI-doc
  drift, SDK/core boundaries, and script-registry checks after review fixes.
- Staticcheck, root/tools vulnerability scans, license, semgrep, secret,
  gitleaks-policy, toolchain, TypeScript-client, machine-contract,
  CI-credential, action-pin, surface-manifest, PTY, release, catalog/scaffold,
  SDK-inventory, script-registry, and agent-skill gates on the initial reviewed
  implementation.
- `git diff --check`.
- The final complete `make check`, including the approved adversarial-review
  gate, exited 0.

No credentials or live tenant were used.

## Known Deferrals

- The operator will perform credentialed tenant validation using
  `docs/LOCATION_GROUP_MEMBERS_VALIDATION.md`.
- The API's flat member references do not identify which members are parents
  versus sublocations; the downstream check uses known IDs/names rather than
  local inference.
- This focused fix does not change direct-get behavior or widen share/paranoid
  output.

## Review Focus

The reviewer was asked to attack endpoint/query construction, page-size and
termination semantics, flag persistence across pages, list/dump reuse,
projection of nested member fields, exact scope of live-smoke allowances,
extension/admin-identity leakage, redaction modes, downstream command safety,
test realism, stale behavior claims, and generated machine-surface drift.

# Adversarial Review

Fresh-context reviewer: Archimedes (`019f8fe0-9cb6-7e32-b315-74830dda7139`)

## Initial Blocking Findings And Resolution

1. The initial live-smoke exception allowed `city` and `managedBy` by resource
   and key name at every recursive depth, although the catalog permits them
   only under `dynamicLocationGroupCriteria`. This could let wrong-path fields
   such as `locations[].city` bypass the safety backstop.

   Root cause: the smoke traversal tracked values recursively but not their
   paths. The fix replaced key-only allowances with exact-path allowances,
   retained parent paths through arrays, and permits only
   `dynamicLocationGroupCriteria.city` and
   `dynamicLocationGroupCriteria.managedBy`. Unit fixtures cover correct and
   wrong paths; a full fake-runner regression proves both list and dump reject
   misplaced member `city` and `managedBy` fields.

2. The initial tenant-validation command narrowed fields and projected member
   summaries before claiming that `lastModUser` and member `extensions` were
   absent. It could therefore erase a canary and still look clean; the
   share-mode `jq` check also lacked `-e`.

   Root cause: presentation and validation were combined in the wrong order.
   The fix passes un-narrowed projected records through a committed `jq -e`
   filter that first rejects `lastModUser` and every member key other than
   `id`/`name`, then emits the summary. The PowerShell path performs equivalent
   assertions before display. Executable docs fixtures prove clean input
   succeeds and `lastModUser`, member extensions, member `city`, and
   share-mode locations fail. `docs-check` runs those fixtures.

The reviewer rechecked both resolutions and their changed surface. No blocking
or non-blocking findings remained.

## Non-Blocking Pagination Risk And Resolution

The initial endpoint-specific transport test used only one short page, so it
did not independently prove that `fetchLocations=true` persisted on page 2.
The final test forces a full 1,000-record first page and a short second page,
requires exactly two requests, verifies host, page, page size, and
`fetchLocations=true` on both, and checks second-page member decoding. The
documented focused regex now also runs all generic `TestZIAPaginate.*` tests.

## Machine Contract Review

- The endpoint, `fetchLocations` semantics, and 1,000-record maximum agree with
  the official API material and vendored SDK source.
- Vendored `ReadPage` preserves `fetchLocations=true` while adding one-based
  `page` and `pageSize` values.
- List and dump share the same handler; direct get remains wired to
  `GetLocationGroup`.
- JSON structure, schemas, manifests, introspection, exit codes, and error
  envelopes are unchanged. Standard-mode content intentionally starts
  populating the already-cataloged `locations` field.
- Machine-contract, surface-manifest, CLI-doc, and field-coverage checks passed.

## Safety Review

- Production projection restricts members to `id`/`name`, strips SDK
  extensions, and drops `lastModUser`.
- `locations` remains standard-only; existing standard/share/paranoid tests
  pass.
- Field narrowing occurs after projection and cannot widen output.
- Exact-path smoke allowances do not permit the reviewed key names under a
  member or another unreviewed nested object.
- The downstream summary filter fails closed before presentation.

## Generated Artifact Review

No generated artifacts changed, and no generated drift was detected.

## Independent Verification

Across the initial review and scoped recheck, the reviewer independently
passed focused ordinary/race tests, full ordinary/race suites, vet, formatting,
docs, generated CLI docs, machine-contract, surface-manifest, field-coverage,
script-registry, and `git diff --check`. Both review checkouts remained clean.

Verdict: approve
