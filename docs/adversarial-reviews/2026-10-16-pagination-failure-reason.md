# Builder Handoff

## Intent

fix(zscaler): classify every page walker's refusal as pagination_incomplete

Only ZIA's page walker wrapped its completeness guards in a sentinel, so a
ZPA, ZCC, ZTW or ZIdentity list refused for the same reasons (page ceiling,
declared total changing, repeated pages, missing continuation) reported
reason `unknown`, which tells an agent nothing about whether to retry. Seen
live: `zidentity resource-servers list` failed with `unknown`. The sentinel is
now `errPaginationValidation`, shared by every walker and by the
network-applications single-page read, so the error envelope says
`pagination_incomplete`, not retryable.

## Base / Head

- Base: `integration/2026-10-09` at `b82c562`.
- Reviewed head: `c3fe20c` (round 2).
- Final head: this artifact on top of `c3fe20c`.

## Files Changed

- `internal/zscaler/reader_zia.go` (sentinel), `failure.go`,
  `reader_zpa_pagination.go`, `reader_zcc.go`, `reader_ztw.go`,
  `reader_zidentity_pagination.go`, `reader.go`
- tests: `failure_test.go`, `reader_pagination_test.go`, `reader_test.go`

## Tests Run

- `go test ./...`, `go vet ./internal/zscaler`: pass.
- Live, non-production tenant: `zidentity resource-servers list` now reports
  `pagination_incomplete` instead of `unknown`.

## Known Gaps

- Which ZIdentity guard rejects `resource-servers` is not yet known: the live
  error deliberately drops the guard message. Finding out needs a value-free
  diagnostic (the guard messages hold only counts and offsets) or operator
  help.
- Drift guards (a declared total changing between pages) are classified not
  retryable, matching ZIA's precedent, although a concurrent admin edit could
  make a retry succeed.
- `parseZPATotalPages` sends a malformed quoted value to `decode_failed` and a
  missing or invalid value to `pagination_incomplete`; both are not retryable.

## Review History

Strata on gm01 remained stopped, so the review ran in a fresh-context Claude
subagent with no conversation history and read-only repository access.

- Round 1, request changes: the ZTW page ceiling and the network-applications
  full-page refusal were still unwrapped; the sentinel carried a stale
  ZIA-only comment; the test covered one guard per walker and skipped
  `zccPaginateWithTotalCount`. All acted on.
- Round 2, approve with nits (below). The reviewer parsed every error literal
  in `internal/zscaler`, multi-line calls included, and found no completeness
  refusal left unwrapped.

# Adversarial Review

Fresh-context reviewer: Claude subagent (no conversation history, read-only repository access), 2026-10-16 (round 2)

## Blocking Findings

None. HTTP status is still classified first; fetch, SDK, context and JSON
errors are not wrapped; every sentinel site maps to `pagination_incomplete`,
not retryable, consistent with ZIA.

## Non-Blocking Risks

1. nit: the network-applications check is a source-text match that pins the
   wrap, not the behavior; a behavioral test needs a 5000-record server.
   Accepted, matching the file's existing source-test style.
2. nit: the `decode_failed` / `pagination_incomplete` split in
   `parseZPATotalPages` is cosmetic, since both are not retryable. Kept.

## Verdict

Verdict: approve with nits
