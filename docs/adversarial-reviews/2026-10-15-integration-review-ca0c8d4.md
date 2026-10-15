# Builder Handoff

## Intent

fix: address integration review at ca0c8d4

An external review of `integration/2026-10-09` at `ca0c8d4` found:

- High: a stdio dump failed whenever a resource failed with a reason. The
  streamed warning and the completed summary built the wire `DumpFailure`
  separately and only the summary carried the detail, so the host's
  warning/summary equality rejected every partial dump with a classified
  failure. Both now call `adapter.DumpFailure`, the one conversion.
- Medium: `*url.Error` implements `net.Error`, so an untrusted TLS certificate,
  a host that does not resolve, a redirect loop and proxy authentication read
  as retryable `transport`. They are now non-retryable.
- Low: HTTP status is kept only in 100..599 (body status as fallback);
  browser-isolation NOT_SUBSCRIBED is recognized only on a non-retryable 4xx;
  an empty or repeated `--count-by` is a usage error (repeats counted by the
  parser through a counted `flag.Value`); the count document gains
  `truncated`.

Left for later, by design: compatibility signalling, schema validation of
emitted output, two pagination edge cases, and the profile name in failure
envelopes.

## Base / Head

- Base: `integration/2026-10-09` at `d81f738`.
- Reviewed heads: `4ee4581` (round 1), `56fd690` (round 2).
- Final head: this artifact on top of `56fd690`.

## Files Changed

- `.agents/skills/zscalerctl/SKILL.md`, `AGENTS.md`, `README.md`,
  `docs/cli/agent-machine-workflow.md`, `man/zscalerctl.1`,
  `skills/zscalerctl/SKILL.md`
- `clients/typescript/test/session.test.ts`
- `internal/cli/{app,count_output,globalflags}.go`, `internal/cli/{count_by_test,snapshot_test}.go`
- `internal/enginehost/events.go`, `internal/enginehost/operation_test.go`
- `internal/enginewire/adapter/results.go`
- `internal/zscaler/{failure,reader,reader_zia}.go`,
  `internal/zscaler/{failure_test,reader_pagination_test}.go`

## Tests Run

- `go test ./...` (Go 1.26.9, `/bin/chmod` first in `PATH`), `gofmt -l`,
  `go vet`, `verify-docs`, `verify-cli-docs`,
  `verify-surface-changes-manifest`, `verify-machine-contract`,
  `verify-typescript-client` (42/42, real engine): pass.
- The host-path dump-with-403 test fails with the reported
  "dump warnings do not match completed summary" when the `events.go` change
  is reverted.

## Review History

Reviewed with no repository access by a fresh-context local model given the
diff, the external review section for `ca0c8d4`, and this repository's
reviewer prompt and template: Strata serving Qwen3.8-Flash-Next (IQ3_S) on
gm01. The reviewer could not run commands.

- Round 1, request changes. Blocker: the first repeat detector scanned raw
  args, so a flag-shaped value of another flag (`--search --count-by`) was
  counted as a repeat. `--count-by` is now a counted `flag.Value`; a test
  pins the case. Also: NOT_SUBSCRIBED excludes 408; `sdkStatusCode` falls
  back to the body status when the response status is out of range.
- Round 2, approve with nits (below).

Accepted and not changed: the redirect and proxy-authentication checks match
net/http's untyped error text (no typed error exists); x509 errors are matched
as the value types the standard library returns; `count-by` keeps its empty
default.

# Adversarial Review

Fresh-context reviewer: Strata (Qwen3.8-Flash-Next IQ3_S, gm01), read-only, no command execution, 2026-10-15 (round 2)

## Blocking Findings

None found.

The round-1 blocker is resolved: `--count-by` repeat detection is now parser-owned via `countedStringFlag.sets` rather than raw-arg scanning. A flag-shaped value consumed by another string flag, e.g. `--search --count-by`, is not counted as a `--count-by` set. Genuine repeats and `--count-by=value` repeats still increment `sets` and produce the same usage error.

## Non-Blocking Risks

1. **`globalflags.go` ignores `d.defaultVal` for `count-by`**

   - file/path: `internal/cli/globalflags.go`
   - concrete issue: the `count-by` special case calls `fs.Var(counted, d.name, d.usage)` and skips the normal `fs.String(d.name, d.defaultVal, d.usage)` path. If `globalFlagDefs` contains a non-empty default for `count-by`, that default is now silently lost.
   - why it matters: `parseGlobal` later uses `countByFlag.value`, so a default that previously affected `globalOptions.countBy` would disappear without a visible parser change.
   - how to reproduce or verify: inspect `globalFlagDefs` for `count-by`. If its `defaultVal` is non-empty, compare old and new `parseGlobal` output.
   - test that would catch it: a `parseGlobal` table test asserting the expected default `countBy` when no `--count-by` is passed, or asserting `globalFlagDefs` contains no non-empty default for `count-by`.

2. **The new regression test is indirect**

   - file/path: `internal/cli/count_by_test.go`
   - concrete issue: `TestCountBySetOnceIsNotARepeat` runs `app.Run` and only asserts that the failure is not `cli.ErrUsage`.
   - why it matters: it proves the user-visible command is not rejected as a usage error, but it does not directly pin that `--search` received `--count-by` and that `countBy` was set exactly once.
   - how to reproduce or verify: call the parser directly and inspect parsed global options / counted flag state.
   - test that would catch it: a direct `parseGlobal` test asserting `search == "--count-by"` and `countByFlag.sets == 1` for `--count-by groups.id --search --count-by`.

3. **`sdkStatusCode` fallback has no direct test in the diff**

   - file/path: `internal/zscaler/reader.go`
   - concrete issue: the new logic falls back to `sdkErr.Parsed.Status` when `sdkErr.Response.StatusCode` is invalid, where the old code returned `0` if the response status was present but out of range.
   - why it matters: this changes error classification and therefore machine-visible error behavior when an SDK error has an invalid response status but a valid parsed body status.
   - how to reproduce or verify: construct an SDK-style error with `Response.StatusCode = 0` or `600` and `Parsed.Status = 408`, `429`, or `400`, then call `sdkStatusCode` and `normalizeZIABrowserIsolationListError`.
   - test that would catch it: add table tests for invalid/zero response status with valid parsed status, including subscription text and 408/429/400 outcomes.

4. **Error-code behavior changes should be reflected in docs/goldens if they exist**

   - file/path: `internal/zscaler/reader_zia.go`
   - concrete issue: `NOT_SUBSCRIBED` no longer applies to 408 when the subscription text appears, and `sdkStatusCode` may now use body status when response status is invalid.
   - why it matters: `NOT_SUBSCRIBED` is a machine-visible classification, and retryability is a contract.
   - how to reproduce or verify: check any docs, error-code references, goldens, or integration fixtures mentioning `NOT_SUBSCRIBED`, 408, or 429 retryability.
   - test that would catch it: a golden/docs consistency test or updated error-code documentation test.

5. **Process/handoff gap for repeatable flags**

   - concrete issue: the original bug came from scanning raw args to detect repeats.
   - why it matters: the same class of bug can appear for any repeated flag if future code counts occurrences by text scanning instead of parser state.
   - how to catch next time: builder handoff should require parser-owned repeat counters for repeatable flags and direct tests with flag-shaped values consumed by other flags, such as `--search --count-by`, `--filter --count-by`, or `--count-by --search`.

## Machine Contract Review

- CLI surface: `--count-by` repeat rejection remains a usage error, but the false positive for `--search --count-by` is fixed.
- Exit-code behavior: unchanged for valid commands and genuine repeated `--count-by`.
- JSON/NDJSON output: no direct output shape change in this diff; `countBy` parsing remains via `parseFieldsList`.
- Error envelope / classification: intentional change: 408 with subscription text is no longer classified as `NOT_SUBSCRIBED`; invalid response status can now fall back to parsed body status.
- Generated docs/help: no generated help, schema, manifest, dump, diff, or fixture changes are visible in the diff. If docs/goldens describe `NOT_SUBSCRIBED` retryability, they likely need a small update.

## Safety Review

- No redaction, projection, field coverage, sensitive-data classification, or output-narrowing behavior changed.
- `--count-by` field narrowing behavior is unchanged except for the potential default-loss risk noted above.
- The `NOT_SUBSCRIBED` classification change does not expose sensitive data, but it does change retryability semantics.

## Generated Artifact Review

- No generated artifacts, agent-skill copies, schemas, goldens, or fixtures appear in the diff.
- No suspicious generated deltas are present.
- If this repo maintains generated CLI docs or error-code docs, they should be checked for consistency with the new 408 and fallback behavior.

## Verdict

Verdict: approve with nits
