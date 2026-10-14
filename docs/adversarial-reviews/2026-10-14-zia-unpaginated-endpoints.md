# Builder Handoff

## Intent

fix(zscaler): read ZIA list endpoints that ignore pagination

On a live non-production tenant (2026-10-14), 25 of 26 ZIA dump failures
were the paginator refusing its own result. Those endpoints (url-categories,
dlp-dictionaries, network-services, dlp-web-rules, firewall-dns-rules and
others) return the whole collection on every request, whatever `page` and
`pageSize` say (a debug build measured 247 categories for a 5,000- and a
10-record request), so page 2 repeated page 1 and the repeated-page check
failed the read. Commit 08eb542 had found the opposite on another tenant
(urlCategories capped at 20 and paging), which is why a short first page is
never trusted.

When page 1 was short and page 2 holds the same records, the walker asks for
page 1 again with a page size one below what page 1 returned. Every record
coming back shows the endpoint ignores paging, and page 1 is accepted; fewer
means it honors sizes and may be capping, and the walk fails closed. Pages
match only record for record per identity, comparing exact values and
ignoring list-element order (networkServices shuffles its port lists on
every response). A changed value fails closed. Returned records are page 1
as received.

Live result: a ZIA dump goes from 26 failed resources to 2
(intermediate-ca-certificates, cloud-app-instances: separate issues), and the
recovered lists match the measured page sizes.

## Base / Head

- Base: `integration/2026-10-09` at `badd8b4`.
- Reviewed heads: `64790c6` (round 1), `6dad8dd` (round 2), `536b324`
  (round 3).
- Final head: `536b324` plus this artifact.

## Files Changed

- `docs/RESOURCES.md`
- `docs/SDK_LIST_COMPLETENESS_AUDIT.md`
- `internal/zscaler/reader_page_fingerprint.go`
- `internal/zscaler/reader_pagination_test.go`
- `internal/zscaler/reader_zia.go`

## Tests Run

- `go test ./...` (Go 1.26.9; macOS ACL tests need `/bin/chmod` first in
  `PATH`), `verify-docs`: pass.
- Debug builds against the tenant (counts and field names only): per-page
  sizes for every failing walk; a 10-record probe returning full collections;
  network-services page differences limited to list order.
- Live binary from this branch: url-categories 247, dlp-dictionaries 134,
  network-services 78 records; ZIA dump 26 failures to 2.

## Review History

Reviewed with no repository access by a fresh-context local model given the
diff, surrounding source and call sites, and this repository's reviewer
prompt and template: Strata serving Qwen3.8-Flash-Next (IQ3_S) on gm01. The
reviewer could not run commands; the builder reproduced each blocking finding
as a failing test before fixing it.

- Round 1, request changes. Blocking: pages matched on identity sets alone,
  so a record changed between requests would return page 1's stale values.
  Fixed by comparing records per identity field for field. Docs now say what
  the probe shows rather than "proves complete" and list the pinned limits
  (only a short page 1 is resolved; a fixed window ignoring page and size is
  indistinguishable and accepted; one-record pages; the probe is one request
  outside the page ceiling). The stricter comparison then failed live
  network-services; a debug build showed every difference was list order, so
  the comparison ignores list-element order.
- Round 2, request changes. Blocking: the canonical comparison decoded
  numbers as float64, so large integers differing by one compared equal.
  Fixed with `UseNumber`.
- Round 3, approve with nits (below).

Accepted and not changed: every ZIA walk decodes into typed SDK structs with
integer fields, so precision is not lost before comparison; list order is
deliberately treated as noise for recognizing a repeat (the worst case is
accepting page 1's ordering when only order changed between requests); exact
number literals err toward failing closed on formatting drift.

# Adversarial Review

Fresh-context reviewer: Strata (Qwen3.8-Flash-Next IQ3_S, gm01), read-only, no command execution, 2026-10-14 (round 3)

## Blocking Findings

None.

The previous blocking finding is closed for the code path shown: `canonicalRecord` no longer decodes the marshaled record into `any` with the default `encoding/json` behavior. It now uses `json.NewDecoder(bytes.NewReader(payload))` with `decoder.UseNumber()`, so numbers become `json.Number` values and keep their literal text through `canonicalValue` and the final `json.Marshal(canonical)`.

The added test is targeted and source-verifiable: it uses typed `int64` fields and list elements above `2^53`, changes the value between repeated pages, and expects `errZiaPaginationValidation`. Under the old default decode path, those large integers would collapse to the same `float64`; under the new path they should remain distinct.

I did not find a concrete, source-verifiable way introduced by this delta for a changed typed record to compare equal through `canonicalRecord`.

## Non-Blocking Risks

### 1. Precision can still be lost before `canonicalRecord` if pagination records are untyped maps

- **file/path:** `internal/zscaler/reader_page_fingerprint.go`, `canonicalRecord`
- **concrete issue:** `UseNumber()` is applied only after `json.Marshal(record)`. If `record` is already a `map[string]any` produced by ordinary `json.Unmarshal`, large integers were already decoded as `float64` before `canonicalRecord` sees them. `json.Marshal(record)` will then emit the rounded float representation, so two large integers that differ by one can still produce identical payloads.
- **why it matters:** the round-2 bug is a class, not just a local bug: any upstream JSON-to-Go decode can destroy integer precision before pagination comparison. The new test uses typed `int64` fields, so it does not prove the fix is safe for generic/map-backed pagination records.
- **how to reproduce or verify:** add a pagination test where `T` is `map[string]any` and the map value is a large `float64`, e.g.:

  ```go
  page1: {"id": 1, "value": float64(1000000000000000000)}
  page2: {"id": 1, "value": float64(1000000000000000001)}
  ```

  Those two float64 values are indistinguishable, so the pages may be treated as unchanged.
- **test that would catch it:** `TestZIAPaginateDistinguishesLargeNumbersInUntypedMaps`, using `map[string]any` pages and expecting fail-closed. If such maps are not used, audit the record types passed to `ziaPaginate` and document that all paginated records are typed with exact integer fields.

### 2. Sorting all list elements can make a genuinely changed record compare equal when list order is semantically meaningful

- **file/path:** `internal/zscaler/reader_page_fingerprint.go`, `canonicalValue`, `case []any`
- **concrete issue:** every JSON array is canonicalized as an unordered multiset. Two records that differ only by array order are treated as equal. This also applies recursively to nested arrays. For example:

  ```json
  {"id": 1, "ports": [80, 443]}
  ```

  and

  ```json
  {"id": 1, "ports": [443, 80]}
  ```

  canonicalize to the same record. Similarly:

  ```json
  {"id": 1, "pair": [[10, 20]]}
  ```

  and

  ```json
  {"id": 1, "pair": [[20, 10]]}
  ```

  canonicalize to the same record because inner arrays are sorted too.
- **why it matters:** this is intentional for the observed `networkServices` port-list reordering, but it is a source-verifiable way for pagination validation to treat a real tenant change as unchanged if any other paginated Zscaler resource uses ordered arrays or ordered tuples. The comparison model says “array order is noise”; if that assumption is wrong for even one resource, a changed page can pass as a repeated page.
- **how to reproduce or verify:** add a pagination test using an ordered list field:

  ```go
  page1: {id: 1, ordered: []int64{1, 2}}
  page2: {id: 1, ordered: []int64{2, 1}}
  ```

  Current `sameRecords` should treat these as the same page.
- **test that would catch it:** either add a test asserting fail-closed for ordered arrays if order is part of the contract, or add a resource-catalog/reader test proving that no paginated resource uses semantically ordered arrays and document the invariant that all arrays compared by `canonicalValue` are unordered sets.

### 3. Exact number literals can make numerically equal pages compare as changed

- **file/path:** `internal/zscaler/reader_page_fingerprint.go`, `canonicalRecord` and `canonicalValue`
- **concrete issue:** preserving literals means `1` and `1.0`, or `1e18` and `1000000000000000000`, are different canonical values if they arrive as `json.Number` or raw JSON. This is the safe direction for detecting changes, but it can cause false-positive repeat detection: an unchanged page may be treated as changed because the API or upstream decoder changed numeric formatting.
- **why it matters:** this can make unpaged-endpoint acceptance fail closed on harmless JSON formatting drift. It is not a data-loss issue, but it is a validation-availability risk.
- **how to reproduce or verify:** use a paginated record type that preserves JSON number literals, such as a map with `json.Number` values or a `json.RawMessage` field, and compare:

  ```json
  {"id": 1, "value": 1.0}
  ```

  against:

  ```json
  {"id": 1, "value": 1}
  ```

  Current canonicalization treats them as different.
- **test that would catch it:** decide the intended JSON numeric contract. If numeric equality is intended, add a test showing `1` and `1.0` do not cause pagination validation failure. If exact literal equality is intended, add a test documenting that formatting drift is treated as changed.

### 4. Review gap: missing `ziaPaginate` and record-type evidence

- **file/path:** `internal/zscaler/reader_pagination_test.go`, `internal/zscaler/reader_page_fingerprint.go`
- **concrete issue:** the supplied artifacts do not include the `ziaPaginate` implementation, the Zscaler reader wiring, or the concrete record types passed into pagination. Without those, I cannot prove whether the untyped-map precision risk or ordered-array risk is reachable in production.
- **why it matters:** the fix is narrow to `canonicalRecord`, but the correctness of repeated-page detection depends on where the compared JSON values come from and what record shapes are possible.
- **how to reproduce or verify:** run or inspect the full pagination path for all Zscaler readers that use `ziaPaginate`; list every record type and whether numbers are decoded as typed integers, `json.Number`, or `float64`.
- **test that would catch it:** a table-driven reader test for each paginated resource that changes a large integer or an ordered array and asserts fail-closed, or a static inventory test asserting that all pagination record types use exact integer fields and unordered arrays only.

### Repeatable class and process note

The repeatable class is: **JSON-to-Go numeric precision loss before a machine comparison is performed**. Future builder handoffs should include the origin of the bytes being compared, not just the comparison function. The reviewer prompt should explicitly ask: “Where was the JSON decoded before this comparison, and did that decode preserve exact integer literals?” A fix that only changes the final comparison function is incomplete if upstream decoding already collapsed large integers into `float64`.

## Machine Contract Review

- **JSON, NDJSON, error envelope, exit code, dump, diff, schema, manifest, introspection:** no public machine contract changes are visible in the supplied diff. The change affects internal pagination validation behavior.
- **Error behavior:** the new test uses the existing `errZiaPaginationValidation` sentinel. No new error envelope or exit-code behavior is introduced in the diff.
- **Canonicalization contract:** the internal contract is that list elements are compared as unordered multisets and numbers preserve their JSON literals. This is documented only in code comments. If this is a stable internal contract, it should be covered by tests for both unordered arrays and exact number literals.
- **Generated docs/schemas/fixtures:** not present in the diff. No generated artifacts, schemas, goldens, or completion fixtures appear to have changed.
- **Test evidence:** no test command output was supplied. The new test code is targeted, but I did not run it.

## Safety Review

- **Redaction/projection/field coverage/sensitive-data classification:** no redaction or projection behavior changed.
- **Sensitive data exposure:** `canonicalRecord` fingerprints record content internally; it does not emit record content. Sorting list elements does not widen output or reveal previously dropped data.
- **Dump/diff artifacts:** unchanged in the supplied diff.

## Generated Artifact Review

- **Generated files:** none in the supplied diff.
- **Generated docs or agent-skill copies:** none.
- **Fixture drift:** only the new pagination test fixture was added. No suspicious generated-artifact drops, additions, ordering changes, or count changes are visible.

## Verdict

Verdict: approve with nits
