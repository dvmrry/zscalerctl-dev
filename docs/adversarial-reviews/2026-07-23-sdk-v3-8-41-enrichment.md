# Builder Handoff

## Intent

Refresh `zscaler-sdk-go/v3` from `v3.8.38` to `v3.8.41` without enabling a
new tenant endpoint. Reconcile newly returned fields on four existing ZIA rule
resources through the fail-closed catalog, preserve bounded URL-rule
enrichment, and inventory new SDK packages for later live-smokeable batches.

## Base / Head

- Base: `ed026199a4f76b96a24e9aab56cdbf87fb2741fa`
- Initial reviewed head: `d27b95565101e70935c77282714e5991f42c5836`
- Final reviewed runtime head: `8f0401d1829b19454ff59073f1a3b8ee34bdb22c`
- Branch: `feature/sdk-v3.8.41-enrichment`
- Process baseline: `origin/main` at the base commit

## Files Changed

- Dependency/vendor: `go.mod`, `go.sum`, `vendor/modules.txt`, and the
  vendored `zscaler-sdk-go/v3` files selected by `go mod vendor`.
- Catalog/projection: `internal/resources/catalog_zia.go`,
  `internal/resources/resources.go`, `internal/zscaler/reader.go`, and
  `internal/zscaler/reader_zia.go`.
- Tests: `internal/zscaler/reader_fields_sdk_v3841_test.go`,
  `internal/zscaler/reader_sdk_v3841_test.go`,
  `internal/zscaler/reader_pagination_test.go`,
  `internal/zscaler/reader_test.go`, `internal/zscaler/schema_zia_test.go`, and
  `cmd/zscalerctl/golden_surface_test.go`.
- Generated artifacts: `docs/FIELD_COVERAGE.md`,
  `docs/field-coverage.json`, and 16 new help/table/pretty golden files under
  `cmd/zscalerctl/testdata/surface/`.
- Documentation/inventory: `docs/RESOURCES.md`, `docs/RESOURCE_QUEUE.md`,
  `docs/SDK_SURFACE_INVENTORY.md`, `docs/SDK_V3_8_41_VALIDATION.md`,
  `docs/THREAT_MODEL.md`, and `docs/ZSCALER_PRODUCT_SCOPE_PLAN.md`.
- Surface declaration:
  `cmd/zscalerctl/testdata/surface/surface_changes.md`.

## Source Inputs Consulted

- Full module-cache package and exported-type deltas from SDK `v3.8.38` to
  `v3.8.41`.
- Vendored SDK source for OneAPI routing, endpoint-application models, URL
  filtering, SSL inspection, firewall filtering, firewall DNS, IPS categories,
  and EUN additions.
- The tagged SDK's GOV/GOVUS test, README, changelog, and official PR #438.
- Existing zscalerctl catalogs, source adapters, bounded ZIA paginator,
  schema-shape review tests, redaction helpers, ZPATWO compatibility transport,
  and golden-surface harness.
- The adversarial-review process documents from `origin/main`.

## Generated Artifacts

- `docs/FIELD_COVERAGE.md` and `docs/field-coverage.json` were regenerated with
  `make field-coverage`.
- CLI docs were regenerated with `make gen-cli-docs` and remained
  byte-identical.
- Vendor was regenerated with `go mod tidy` and `go mod vendor`; the committed
  state passes `make verify-vendor`.
- New surface goldens were generated with
  `go test -mod=vendor ./cmd/zscalerctl -run TestGoldenSurface -update -count=1`
  and then verified without `-update`.

## Expected Delta

- SDK: `v3.8.38` to `v3.8.41`; no resource or operation count change.
- Exported SDK fields: `2979` to `2993`; classified fields: `2961` to `2975`;
  deliberate ignored fields remain `18`; deferred remains `0`.
- SSL inspection, firewall filtering, and firewall DNS rules gain compact
  standard-only endpoint-application and endpoint-application-group
  references. Only application resource ID/name/OS/type and group ID/name are
  copied to source records.
- URL filtering rules gain compact standard-only HTTP-header profile/action
  profile references.
- Firewall filtering and firewall DNS rules gain standard/share EUN template,
  EUN enablement, and context-shield exclusion fields.
- ISOLATE URL-rule `get` uses the existing bounded all-pages reader for optional
  CBI-profile enrichment. Non-context enrichment failure preserves the direct
  result; caller cancellation/deadline propagates.
- The tagged SDK's GOVUS OAuth implementation typo is corrected by a narrow
  transport rewrite to its documented `zidentitygovus.net` contract. OAuth and
  ZIA product routing are tested; GOV/GOVUS Zidentity `/admin` routing is
  explicitly not claimed without authoritative/live evidence.
- Twenty-three new service packages and new reads are inventoried but remain
  disabled.

## Invariants Claimed

- No new API endpoint is reachable through zscalerctl.
- Raw SDK records still pass through source adapters and fail-closed catalog
  projection before every renderer and consumer.
- Endpoint filenames, bundles, descriptions, versions, signature/modification
  metadata, nested inventories, HTTP-header extensions, CBI profile URLs,
  admin identities, and certificate material are not newly renderable.
- `--fields` remains narrowing-only.
- Existing resource names, operations, errors, exits, dump/diff formats,
  manifest schema, and introspection schema version are unchanged.
- The GOVUS rewrite matches only HTTPS, the configured vanity host, and the
  exact OAuth token path. Product, unrelated, and admin traffic is unchanged.
- Existing ZPATWO OAuth and Zidentity-admin compatibility remains intact.
- No credentialed tenant data or secret fixture was used.

## Tests Run

- Focused resource, projection, pagination, cancellation, cloud-routing,
  command, and CLI tests.
- Full ordinary Go suite and full race suite on the initial candidate.
- Focused GOV/GOVUS/ZPATWO and URL-enrichment race tests after review fixes.
- `go vet -mod=vendor ./...` and `make staticcheck`.
- Documentation, CLI-doc, SDK/core/experiment-boundary, machine-contract,
  surface-manifest, SDK-inventory, vendor, and field-coverage gates.
- Golden surface update followed by clean verification.
- `make fmt-check`, `jq empty docs/field-coverage.json`, and
  `git diff --check`.
- The tagged SDK's own `TestGetAuthURL` was run as a control and reproduced its
  internal GOVUS contradiction before the local compatibility fix.

The complete `make check` is run after this approved artifact is committed so
the adversarial-review gate evaluates the final branch state.

## Known Deferrals

- No credentialed tenant smoke was run. Optional read-only commands are in
  `docs/SDK_V3_8_41_VALIDATION.md`.
- No newly added service package is enabled. DNS application groups, HTTP
  header profiles, IPS categories, adaptive access, and DLP global options are
  queued separately; privacy- or secret-heavy additions remain held.
- GOV/GOVUS Zidentity `/admin` routing remains unclaimed and requires an
  authoritative host contract plus environment-specific smoke.

## Review Focus

Attack SDK-model reconciliation, nested projection and mode boundaries,
source/catalog key and type parity, URL-rule fallback semantics, GOVUS rewrite
scope, GOV/GOVUS and ZPATWO routing, generated field counts, help and human
renderers, vendor reproducibility, and queued-versus-supported claims.

# Adversarial Review

Fresh-context reviewer: Galileo
(`019f920b-2433-7803-8b43-05bdbc6eacdd`)

## Initial Blocking Findings And Resolution

1. **The GOVUS test blessed an upstream implementation typo.** The tagged SDK
   emits `zidentitygov.us` although its own test, README, and changelog require
   `zidentitygovus.net`. The existing identity compatibility transport now
   corrects only that exact GOVUS OAuth host/path. Lowercase and uppercase
   integration tests prove the documented OAuth host and government product
   gateway; boundary tests prove unrelated/admin traffic remains unchanged.
   Documentation limits the evidence to OAuth and ZIA product routing.
2. **The surface declaration omitted help and human renderers.** Four resource
   help goldens and four narrowed table/pretty cases now collectively render all
   14 new fields. The surface ledger enumerates those cases plus schema,
   introspection, JSON, and NDJSON effects.

The same reviewer rechecked only resolution commit
`8f0401d1829b19454ff59073f1a3b8ee34bdb22c`. No finding remained.

## Machine Contract Review

- Machine manifest, resource count, operations, error envelopes, exits, dump,
  diff, and introspection schema version remain unchanged.
- Schema and introspection output fields change only for the intended four ZIA
  resources.
- Help, table, pretty, JSON, and NDJSON deltas are now declared and
  regression-frozen where representative fixtures apply.
- The 23 new SDK packages remain queued rather than supported.

## Safety Review

Endpoint adapters copy only compact policy references. Excluded endpoint
inventory fields never reach source records. Nested projection recursively
allow-lists keys and scans rendered string leaves; share and paranoid omit the
standard-only parent fields. The reviewer found no path that exposes excluded
inventory or bypasses secret scanning. The GOVUS transport rewrite is exact in
scheme, host, vanity, and path and does not affect product or admin traffic.

## Generated Artifact Review

The 14-field increase exactly reconciles the SDK shape delta. Deliberate
exclusions remain 18. Field coverage, vendor reproduction, CLI docs, SDK
inventory, surface manifest, and all new goldens passed their drift gates.

## Independent Verification

The reviewer independently passed focused ordinary and race routing tests,
`TestGoldenSurface`, the surface-manifest gate, scoped vet, documentation and
machine-contract gates, field-coverage verification, formatting, and
`git diff --check`. The worktree remained clean. As a control, the reviewer
independently reproduced the tagged SDK's failing GOVUS auth-host test.

Verdict: approve
