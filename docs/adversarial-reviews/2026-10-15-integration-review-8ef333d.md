# Builder Handoff

## Intent

test(zscaler): pin the session-retry match to the real SDK error

The two notes of an external review of `integration/2026-10-09` at
`8ef333d`:

- The exhausted-session-retry match was pinned to a hand-written copy of the
  SDK's error text. A test now drives a real SDK client, configured as the
  reader configures it (session retry cap 1), against an API that answers 403
  "Resource Access Blocked"; the SDK refreshes the token, gives up, and its
  own error must classify as `forbidden`, not retryable.
- A persistent 401 gets the same `forbidden` label because the SDK returns
  the error without the response. Accepted and documented in the code: both
  labels are non-retryable, and recovering the status would mean tracking
  every response in the transport.

## Base / Head

- Base: `integration/2026-10-09` at `8ef333d`.
- Reviewed head: `741614c` (round 1).
- Final head: this artifact on top of `741614c`.

## Files Changed

- `internal/zscaler/failure.go` (comment only)
- `internal/zscaler/failure_sdk_test.go`

## Tests Run

- `go test ./...` (Go 1.26.9, `/bin/chmod` first in `PATH`): pass.
- The new test under `-race -count=3`: pass (about 2 s per run, SDK retry
  backoff; the test is parallel).

## Review History

Reviewed with no repository access by a fresh-context local model given the
diff, the external review section, and this repository's reviewer prompt and
template: Strata serving Qwen3.8-Flash-Next (IQ3_S) on gm01. The reviewer
could not run commands.

- Round 1, approve with nits (below).

# Adversarial Review

Fresh-context reviewer: Strata (Qwen3.8-Flash-Next IQ3_S, gm01), read-only, no command execution, 2026-10-15 (round 1)

## Blocking Findings

None.

The diff is test-only plus a comment. It does not change classification logic, CLI behavior, JSON/NDJSON output, exit codes, schemas, manifests, generated artifacts, redaction, or projection behavior.

The new test does exercise a real SDK error path in a way that can fail if the vendored SDK changes its session-retry give-up text. The key mechanism is visible in the diff:

- `internal/zscaler/failure_sdk_test.go` builds a real `zsdk.NewOneAPIClient` and calls `zidusers.GetUser`.
- The transport returns `403 "Resource Access Blocked"` for non-token paths.
- The token path returns a fresh token.
- The test then requires `sessionRetriesExhausted(err)` and `classifyFailure(err, sdkStatusCode(err)) == (forbidden, false)`.

That is materially better than the prior hand-written error-text pin.

## Non-Blocking Risks

1. **The test does not explicitly prove the reader’s retry cap of 1.**

   - `file/path`: `internal/zscaler/failure_sdk_test.go`
   - Concrete issue: the test asserts only `apiCalls < 2` fails. It does not assert that the SDK configuration has `MaxSessionNotValidRetries == 1`, nor that the observed API call count matches the reader’s configured retry cap.
   - Why it matters: the commit message and test comment say the SDK client is “configured as the reader configures it.” If `newSDKConfiguration` is not the exact production path, or if the reader’s cap is set elsewhere and is changed accidentally, this test can still pass while the production wiring drifts.
   - How to reproduce or verify: inspect `newSDKConfiguration` and the reader construction path. If the reader sets `MaxSessionNotValidRetries` outside `newSDKConfiguration`, the test is not proving that production setting.
   - Test that would catch it: add an explicit assertion such as:

     ```go
     if sdkCfg.MaxSessionNotValidRetries != 1 {
         t.Fatalf("MaxSessionNotValidRetries = %d, want 1", sdkCfg.MaxSessionNotValidRetries)
     }
     ```

     and/or assert the final API call count matches the configured retry cap, e.g. `apiCalls == sdkCfg.MaxSessionNotValidRetries + 1`.

2. **The test does not explicitly prove token refresh occurred.**

   - `file/path`: `internal/zscaler/failure_sdk_test.go`
   - Concrete issue: the transport intercepts `/oauth2/v1/token`, but the test does not count token requests. It only counts API calls.
   - Why it matters: the comment claims the SDK “refreshes the token, gives up.” A future SDK or config change could produce the same give-up error through a different retry path, or the test could pass while the token endpoint is not actually being refreshed as expected.
   - How to reproduce or verify: add a `tokenCalls` counter and assert it is at least 2: initial token request plus refresh.
   - Test that would catch it:

     ```go
     if tokenCalls < 2 {
         t.Fatalf("token calls = %d, want token refresh after session invalid response", tokenCalls)
     }
     ```

3. **The token lifetime may be close to SDK background-renewal behavior.**

   - `file/path`: `internal/zscaler/failure_sdk_test.go`
   - Concrete issue: the token response uses `expires_in: 60`, with a comment saying this keeps expiry within the SDK’s one-minute renewal window so the test does not start a background renewal goroutine.
   - Why it matters: if the SDK’s renewal check is sensitive to elapsed test time, a token with a 60-second lifetime could potentially trigger renewal behavior under `-race -count=N`, increasing flakiness or making the test less deterministic.
   - How to reproduce or verify: inspect the vendored SDK renewal condition and run `go test ./internal/zscaler -run TestSessionRetriesExhaustedMatchesTheRealSDKError -race -count=10`. If token calls become nondeterministic or goroutines appear, adjust the token lifetime or assert exact token call counts.
   - Test that would catch it: exact token-call accounting, as above, would make unexpected background renewal visible.

4. **Accepting note 2 is reasonable for this change.**

   - `file/path`: `internal/zscaler/failure.go`
   - Concrete issue: the comment now states that a persistent 401 gets the same `forbidden` label as a persistent 403 because the SDK error does not preserve the status.
   - Why it matters: the reason label can be semantically slightly wrong, but the retryability is correct: both are non-retryable. Recovering 401 vs 403 would require transport-level status tracking or SDK changes, which is outside this test-only change.
   - How to verify: the current classification path only receives the SDK error and `sdkStatusCode(err)`. If the SDK does not expose the status, `classifyFailure` cannot distinguish the two.
   - Test that would catch future regression: if transport-level status tracking is ever added, add a 401-persistent case expecting `unauthorized`/non-retryable.

## Machine Contract Review

- Did JSON, NDJSON, error envelope, exit code, dump, diff, schema, manifest, or introspection behavior change?

  No. The diff changes only a comment in `internal/zscaler/failure.go` and adds `internal/zscaler/failure_sdk_test.go`.

- Are contract deltas intentional and documented?

  No machine contract deltas are present.

- Are generated docs, schemas, and fixtures in sync with runtime behavior?

  No generated artifacts, docs, schemas, or fixtures are touched.

## Safety Review

- Did redaction, projection, field coverage, or sensitive-data classification change?

  No.

- Can a narrowed field set widen output or reveal previously dropped data?

  No.

- Are dump and diff artifacts still sanitized but treated as confidential?

  No dump or diff artifact behavior is touched.

## Generated Artifact Review

- Were generated files intentionally regenerated?

  No generated files are touched.

- Are deltas explained?

  Not applicable.

- Any suspicious drops, additions, ordering changes, or count changes?

  None. The diff adds one test file and one comment-only expansion.

## Verdict

Verdict: approve with nits
