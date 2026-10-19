# Builder Handoff

## Intent

fix(zscaler): scan-based gets report misses as not found; singletons keep []

- A get the API cannot answer directly is answered by reading the complete
  list and matching the ID. The list call succeeds, so a miss has no HTTP 404
  to classify downstream; four getters returned a plain "not found" error,
  which `normalizeLiveError` turned into a live-access failure (exit 5, not 4):
  ZIA `devices` and `sublocations` (local getters), ZIA
  `dlp-idm-profile-lite` (SDK `GetDLPProfileLiteID`) and ZPA
  `cbi-zpa-profiles` (SDK `Get`). New shared helpers `scanGetByIntID`,
  `scanGetByStringID` and `scanGet` own the not-found sentinel;
  `ziaSDKListGetByIntID` wraps them. `devices`, `dlp-idm-profile-lite` and
  `cbi-zpa-profiles` scan the same list their `list` operation reads, so a get
  finds a record exactly when `list` shows it (dlp-idm-profile-lite previously
  sent `?activeOnly=false` in one unpaginated read).
- `sublocations` searches every parent and tolerates a parent it cannot read.
  A miss is not-found only when every parent was read; otherwise the skipped
  parent's error is returned (exit 5), preferring a non-404 skip over a 404.
- `newSingletonHandler` did not wrap its mapper in `preserveEmptyLists`, so a
  list a singleton API returned as `[]` rendered as absent. All three handler
  constructors now wrap. (The 2026-10-15 confirmed-empty-lists artifact's note
  that other handlers keep dropping empty lists no longer holds.)

## Base / Head

- Base: `integration/2026-10-09` at `14b1ae1`.
- Reviewed head: working tree on `fix/reader-scan-miss-and-singletons`.
- Final head: the reviewed change, round-2 nits, and this artifact.

## Files Changed

- `internal/zscaler/reader.go` (scan-get helpers, singleton wrap)
- `internal/zscaler/reader_zia.go` (devices, dlp-idm-profile-lite,
  sublocation search; `getZIADeviceByID` removed)
- `internal/zscaler/reader_zpa.go` (cbi-zpa-profiles)
- `internal/zscaler/empty_lists_test.go` (`TestHandlersPreserveEmptyLists`)
- `internal/zscaler/reader_pagination_test.go`
  (`TestScanGetsReportMissesAsNotFound`)

## Tests Run

- `PATH=/bin:$PATH go test ./...`, `make fmt-check`: pass. Reviewer ran
  `-race ./internal/zscaler`: pass.
- Mutations: with the base `reader_zia.go`/`reader_zpa.go` all four miss
  assertions fail; dropping the skipped-parent return fails the 403-parent
  assertion; reverting the singleton wrap fails only the singleton case.

## Known Gaps

- ZTW `account-groups` maps an empty `[]` get response to a live-access
  failure; whether ZTW answers an unknown ID that way is unverified.
- Nested and pointer-to-slice lists remain unconfirmed (unchanged).

## Review History

Strata on gm01 remained stopped, so the review ran in a fresh-context Claude
subagent with no conversation history and read-only repository access.

- Round 1, request changes: the sublocation search tolerates unreadable
  parents, so the new sentinel turned "may be under a skipped parent" into a
  false not-found (exit 4); also two more scan-gets with plain errors
  (dlp-idm-profile-lite, cbi-zpa-profiles). Fixed with the skipped-parent rule
  and the shared scan-get helpers.
- Round 2, approve with nits: a parent 404 kept as the first skip could hide a
  later 403; no hit assertions. Both taken.

# Adversarial Review

Fresh-context reviewer: Claude subagent (no conversation history, read-only repository access), 2026-10-19 (round 2)

## Blocking Findings

None. The skipped-parent rule returns the remembered error (exit 5 with its
real status) and the sentinel only after a complete scan; cancellation and
pagination validation still win. dlp-idm-profile-lite's `idOf` matches the
catalog `GetKey: "profileId"`, and `intIDGetter` keeps id validation.
`scanGetByStringID` keeps `zpaSDKStringGet`'s trim/empty validation before
service acquisition. The helper refactor preserves per-call service
acquisition. Tests, `-race`, gofmt and vet pass.

## Non-Blocking Risks

1. A remembered parent 404 normalizes to not-found on a get; only the first
   skip was kept, so a later 403 parent could be hidden. (Taken: a non-404
   skip replaces a 404.)
2. dlp-idm-profile-lite get now sends no `activeOnly`, like `list`; consistent
   with list, assuming the API's default is false as the SDK does.
3. The test checks only misses; a hit assertion would pin the ID field.
   (Taken.)
4. ZTW `account-groups` empty-response mapping and the stale historical
   artifact line remain, as noted.

## Verdict

Verdict: approve with nits
