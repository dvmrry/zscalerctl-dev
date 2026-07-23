# Builder Handoff

## Intent

Remove every production reader path that reaches the vendored SDK's unbounded
ZIA, ZPA, ZTW, or ZCC list paginators. Preserve each endpoint's existing API
version, path, query defaults, page size, post-filter, error propagation,
projection, and redaction behavior while adding finite page ceilings and
fail-closed completion checks.

## Base / Head

- Base: `6d5c9030568cf4ce12565e61c782103e9ce2f363`
- Initial reviewed head: `d2a2e52e501f7d1a7b75317c9cead6045fb66a25`
- Final reviewed runtime head: `64ba648b0e509a60aa97867c2a56c449d2ee1c18`
- Branch: `feature/audit-getall-completeness`
- Process baseline: `origin/main` at the base commit

## Files Changed

- `internal/zscaler/reader_zia.go`
- `internal/zscaler/reader_zpa.go`
- `internal/zscaler/reader_zpa_pagination.go`
- `internal/zscaler/reader_ztw.go`
- `internal/zscaler/reader_zcc.go`
- `internal/zscaler/reader_pagination_test.go`
- `internal/zscaler/reader_zpa_pagination_test.go`
- `internal/zscaler/reader_sdk_pagination_guard_test.go`
- `docs/SDK_LIST_COMPLETENESS_AUDIT.md`
- `docs/SDK_PAGINATION_VALIDATION.md`
- `docs/ZPA_PAGINATION_VALIDATION.md`
- `docs/README.md`
- `docs/RESOURCES.md`

## Source Inputs Consulted

- Every production SDK service call in `internal/zscaler` for ZIA, ZPA, ZTW,
  ZCC, and Zidentity.
- Vendored `zscaler-sdk-go/v3` `v3.8.38` implementations behind those calls.
- ZIA `common.ReadAllPages` and `ReadPage`, including endpoint-specific page
  sizes and query construction.
- ZPA `GetAllPagesGeneric*`, response envelopes, API versions, microtenant
  injection, and application-subtype post-filters.
- ZTW `common.ReadAllPages`, `ReadPage`, `ReadResource`, and legacy/OneAPI
  product routing.
- ZCC `ReadAllPages`, `ReadPage`, and existing per-page list methods.
- Existing Zidentity offset pagination and its regression tests.
- The machine capability manifest for exact downstream resource names.
- Official Zscaler URL-filtering pagination guidance for the previously
  observed 100-record boundary.

## Generated Artifacts

None changed. CLI docs, schemas, machine manifests, introspection fixtures,
field-coverage artifacts, surface goldens, and generated agent-skill copies
remain unchanged.

## Expected Delta

- ZIA SDK short-page lists use `getZIAAllPages` with a 1,000-page ceiling.
- ZIA users retain `pageSize=10000` plus `sortBy=name&sortOrder=asc`.
- ZIA firewall filtering rules retain `pageSize=5000`.
- ZIA location groups retain `fetchLocations=true`.
- ZIA sublocation and device `get` scans now use bounded list paths.
  Sublocation `get` retains the SDK's ordered early return and tolerance for
  an inaccessible parent while propagating caller cancellation.
- ZIA browser-isolation list retains the SDK's status-free `NOT_SUBSCRIBED`
  error wording.
- ZPA's 24 catalog lists use a local 500-record metadata paginator that fixes
  the first response's `totalPages`, rejects missing/malformed/drifting
  metadata, empty or repeated declared pages, and declarations above 1,000.
- ZPA methods that accepted `service.WithMicroTenant` retain that explicit
  request override; generic methods retain client-level scoping.
- ZPA browser-access, inspection, and PRA views retain their SDK subtype
  filters after complete collection.
- ZTW's 19 paginated resources use a local 1,000-record, 1,000-page adapter
  and `ReadResource` so legacy and OneAPI product routing remain correct.
- All ten ZCC lists use bounded pagination; devices and fail-open policy keep
  `pageSize=1000`, while admin roles preserve the SDK's `pageSize=50`.
- Any page, metadata, ceiling, or post-collection filter error returns no
  partial record slice.
- A source-aware test scans every production Go file, resolves direct SDK
  calls into the buildable vendored package files, follows same-package helper
  calls, and rejects `ReadAllPages*` or `GetAllPagesGeneric*`.
- Resource names, operation availability, projected fields, JSON/NDJSON
  shapes, schemas, error envelopes, and exit codes are unchanged.

## Invariants Claimed

- Raw SDK records still pass through the existing source adapters and
  fail-closed catalog projection before rendered or dump output.
- No new field is collected, projected, or rendered.
- Redaction modes and output narrowing are unchanged.
- List and dump continue to share the same resource handlers.
- Existing query defaults, service-level sorts, microtenant scoping, API
  versions, and endpoint paths are preserved.
- Context cancellation and per-request timeouts still reach the SDK client.
- Tests and validation notes contain no credentials or tenant payloads.

## Tests Run

- Focused ordinary tests for ZIA, ZPA, ZTW, ZCC, service-scoped microtenant
  propagation, ZCC admin-role page-size parity, sublocation early return, the
  browser-isolation error adapter, and the source-aware gate.
- Focused race tests for the same pagination paths.
- `go test -mod=vendor ./internal/zscaler -count=1`.
- Full ordinary and race Go suites.
- `go vet`, staticcheck, root/tools vulnerability scans, licenses, semgrep,
  gitleaks, toolchain, TypeScript-client, SDK/core/experiment boundaries,
  machine-contract, docs, CLI-doc drift, surface-manifest, CI-credential, and
  action-pin gates.
- The Go error-flow anti-pattern checker over every changed Go file.
- `git diff --check`.

`make check` reached the required adversarial-review gate with every preceding
gate passing. The final complete run is intentionally deferred until an
approved review artifact exists.

## Known Deferrals

- No credentialed tenant was used. The operator will validate representative
  counts and unique IDs with `docs/SDK_PAGINATION_VALIDATION.md` and
  `docs/ZPA_PAGINATION_VALIDATION.md`.
- Eight ZIA list endpoints remain single reads because neither vendored source
  nor current evidence proves a pagination contract: admin roles, DC
  exclusions, source/destination IP groups, NSS servers, sandbox rules, SSL
  inspection rules, and time windows.
- ZTW admin roles and the documented raw-array/singleton ZPA endpoints remain
  single reads.
- This change does not upgrade or patch the vendored SDK.

## Review Focus

Attack endpoint/API-version/query/page-size equivalence against each replaced
SDK function; ZPA `totalPages` interpretation and microtenant scoping; ZPA
subtype filtering; ZTW legacy product routing; exact-multiple and empty-page
behavior; partial-result suppression; response/error fidelity; ceiling and
resource-exhaustion behavior; JMESPath timing; source-guard false negatives;
list/dump handler reuse; projection/redaction boundaries; downstream command
safety; and any undocumented machine-contract or generated-artifact drift.

# Adversarial Review

Fresh-context reviewer: Gibbs
(`019f907d-aeca-77a2-8b9a-70671d74615d`)

## Initial Blocking Findings And Resolution

1. **ZPA service-scoped microtenant overrides were dropped.** The initial
   adapter relied only on client-level request injection, while 17 replaced
   SDK methods explicitly honored `service.WithMicroTenant`. The final adapter
   separates those 17 service-scoped methods from the seven generic
   client-scoped methods. A real transport test sets conflicting service and
   client values and requires the service override on scoped requests.
2. **ZCC admin-role page size changed from 50 to 1,000.** Short-page
   termination at 1,000 could silently stop on a server-capped 50-record first
   page. The final handler preserves the SDK default of 50. A real transport
   test returns 50 records followed by a second page and requires both
   requests.
3. **ZIA sublocation `get` depended on unrelated later parents.** The initial
   implementation collected every parent's sublocations before scanning, so a
   later failure could suppress an already available match. The final
   implementation scans bounded parent pages in order, returns immediately on
   a match, tolerates an inaccessible earlier parent as the SDK did, and still
   propagates caller cancellation.

## Initial Non-Blocking Findings And Resolution

1. Unquoted integral-decimal ZPA `totalPages` values such as `1.0` were
   rejected even though the SDK accepted them through its float decoding path.
   The parser now preserves that behavior while continuing to reject
   fractional values and quoted decimals.
2. Replacing the browser-isolation SDK wrapper bypassed its status-free
   `NOT_SUBSCRIBED` mapping. The local bounded adapter now retains the exact
   safe operator message and has focused regression coverage.

The same reviewer rechecked only the resolution commit
`64ba648b0e509a60aa97867c2a56c449d2ee1c18`. No findings remained.

## Machine Contract Review

- Resource names, operations, projected fields, JSON and NDJSON framing,
  schemas, introspection, the machine manifest, and exit-code definitions are
  unchanged.
- List and dump still share the same handlers. Complete page sets pass through
  the existing typed source adapters and fail-closed catalog projection.
- Service-scoped ZPA reads retain their former request scope, ZCC admin roles
  retain their 50-record page contract, and sublocation `get` retains its
  successful early-return behavior.

## Safety Review

Projection, field classifications, redaction, and narrowing are unchanged.
Paginator failures return no partial records. The microtenant correction
prevents a service-scoped request from silently reading the client-default
microtenant instead. Validation scripts emit only resource labels, counts, and
ID cardinalities and enable pipeline failure propagation.

## Generated Artifact Review

No generated CLI reference, schema, capability manifest, catalog, surface
golden, field-coverage artifact, or agent-skill copy changed. Focused
formatting, documentation, SDK/core boundary, machine-contract, and SDK
inventory gates passed.

## Independent Verification

The reviewer independently passed `git diff --check`, formatting checks,
`go vet -mod=vendor ./internal/zscaler`, and focused ordinary and race tests.
The builder separately passed the complete ordinary and race repository suites
on the final runtime head. No credentialed tenant was used.

Verdict: approve
