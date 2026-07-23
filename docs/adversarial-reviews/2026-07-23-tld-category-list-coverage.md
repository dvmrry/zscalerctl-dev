# Builder Handoff

## Intent

Make `zia url-categories list` and ZIA dumps discover configured TLD categories
that were already retrievable by `get <id>`, without changing count-only list
payloads, projection/redaction, direct get behavior, or the bounded page
ceiling.

## Base / Head Reviewed

- Base: `2be8061bd7406f72146823999d22b3b3f11720f8`
- Reviewed head: `06b96250129c1e69e7fe72b31c24af0be1a3f35d`
- Branch: `feature/fix-tld-category-list`
- Process baseline: `origin/main` at the base commit

## Expected Delta

- The URL-category list query adds `type=ALL`.
- List and dump results include `URL_CATEGORY` and `TLD_CATEGORY` records.
- `includeOnlyUrlKeywordCounts=true`, `page=1`, `pageSize=5000`, and the
  fail-closed 5,000-record ceiling remain unchanged.
- `get <id>` remains the detail/member path.
- The existing URL-category adapter, catalog allow-list, redaction modes,
  output schema, resource names, and exit behavior remain unchanged.

## Builder Verification

- The focused transport regression failed before the fix because the outgoing
  query had no `type` value.
- Focused post-fix ordinary and race tests passed.
- Full Go and full race suites passed.
- Vet, staticcheck, docs, generated CLI docs, SDK/core/experiment/machine
  boundaries, surface manifest, secret scans, release checks, and skill-sync
  checks passed.
- The root vulnerability scan passed. The unchanged tools module still has the
  separate baseline `GO-2026-5970` finding.
- No credentials or live tenant were used.

# Adversarial Review

Fresh-context reviewer: Sartre (`019f8f0e-4301-73a3-ab3f-efb817b2e164`)

## Blocking Findings

None.

## Non-Blocking Risks

- No credentialed API call was performed. Actual tenant behavior remains the
  downstream validation boundary.
- The reviewer context's focused tests did not execute because its offline
  toolchain verification stopped before compilation. This was an environment
  failure, not a test failure; the builder's ordinary and race executions
  passed against the reviewed head.

## Machine Contract Review

- Vendored `GetAll` documents `ALL` as returning every category type, and the
  production request adds exactly `type=ALL`.
- Vendored `ReadPage` preserves existing query values while adding
  `page=1&pageSize=5000`.
- `includeOnlyUrlKeywordCounts=true` remains present, preserving count-only
  list semantics.
- The 5,000-record fail-closed check is unchanged.
- Direct get remains wired to `urlcategories.Get` with the same source-record
  adapter.
- Dump collection uses the same handler map and calls `List` before projection.
- The transport regression captures the final OneAPI request and verifies all
  four query parameters plus both `URL_CATEGORY` and `TLD_CATEGORY` decoding.
- The projection regression independently verifies that both types survive
  adaptation and that the TLD count field survives the allow-list.

## Safety Review

The URL-category source mapper and catalog remain unchanged. TLD records inherit
the same allow-list, redaction modes, sensitive member-field restrictions,
secret scanning, and rendered-subset verification as ordinary categories.

## Generated Artifact Review

Only the intentional `docs/RESOURCES.md` behavior clarification changed.
Catalog definitions, field-coverage artifacts, schemas, fixtures, resource
names, and generated CLI documentation are unchanged.

Verdict: approve
