# Builder Handoff

## Intent

fix(zscaler): classify exhausted session retries; pin net/http error text

The two new low items of an external review of `integration/2026-10-09` at
`2651a86`:

- Since SDK v3.8.55 a 401/403 that stays session-invalid ("Resource Access
  Blocked") through every token refresh comes back as a plain
  "max SESSION_NOT_VALID retries exceeded" error with no response, so it
  classified as `unknown`. It is now `forbidden`, not retryable: the
  refreshed token was accepted and the request was still refused, which is
  an access verdict, as the 403 classified before the bump.
- The redirect-loop and proxy-407 checks in `permanentTransportFailure`
  match net/http's untyped error text. A test now produces both errors from
  net/http itself, so a Go release that rewords them fails the test.

## Base / Head

- Base: `integration/2026-10-09` at `2651a86`.
- Reviewed head: `9d76461` (round 1).
- Final head: this artifact on top of `9d76461`.

## Files Changed

- `internal/zscaler/failure.go`
- `internal/zscaler/failure_test.go`

## Tests Run

- `go test ./...` (Go 1.26.9, `/bin/chmod` first in `PATH`): pass.
- The reviewer's suggested check, `go test ./internal/zscaler -run
  'TestNormalizeLiveErrorClassifiesFailures|TestPermanentTransportFailureMatchesRealNetHTTPErrors'
  -race -count=20`: pass.

## Review History

Reviewed with no repository access by a fresh-context local model given the
diff, the SDK source that returns the session-retry error, the external review
section, and this repository's reviewer prompt and template: Strata serving
Qwen3.8-Flash-Next (IQ3_S) on gm01. The reviewer could not run commands.

- Round 1, approve with nits (below); the suggested race/count run passes.

# Adversarial Review

Fresh-context reviewer: Strata (Qwen3.8-Flash-Next IQ3_S, gm01), read-only, no command execution, 2026-10-15 (round 1)

## Blocking Findings

None high-confidence blocking findings from the supplied diff, source artifact, and test evidence.

The classification change is narrow and evidence-backed:

- `internal/zscaler/failure.go`: `sessionRetriesExhausted` only matches the SDK’s current give-up text:
  `max SESSION_NOT_VALID retries exceeded`.
- The supplied SDK v3.8.55 artifact confirms that after repeated session-invalid `401`/`403` responses, the SDK returns a plain `fmt.Errorf("max SESSION_NOT_VALID retries exceeded (%d), possible authentication issue")` without a usable response/status.
- `internal/zscaler/failure_test.go` adds a wrapped synthetic error case and asserts `FailureForbidden` / `retryable: false`.

The new net/http test is local-only and does not require external network:

- `httptest.NewServer` binds locally.
- The redirect-loop server redirects to itself.
- The proxy test uses a local `httptest` proxy and an explicit `http.ProxyURL`; the `https://api.example.test` target is not resolved because the transport issues `CONNECT` through the local proxy.

A reasonable verification is:

```bash
go test ./internal/zscaler -run 'TestNormalizeLiveErrorClassifiesFailures|TestPermanentTransportFailureMatchesRealNetHTTPErrors' -race -count=20
```

No source-verifiable defect in the changed surface was found that would justify blocking.

## Non-Blocking Risks

1. `internal/zscaler/failure.go` — SDK string drift remains possible.

   `sessionRetriesExhausted` uses:

   ```go
   strings.Contains(err.Error(), "max SESSION_NOT_VALID retries exceeded")
   ```

   This is correct for the supplied SDK v3.8.55 artifact, but it is not tied to an SDK error type and the new test uses a synthetic error, not a real SDK-generated error. A future SDK wording change would silently return to `unknown`.

   This is not blocking because the external review item explicitly concerns this SDK error and the supplied source artifact pins the current text. But it is a future-hardening risk.

   Test that would catch it: a test that exercises the SDK client path, or at least a golden constant copied from the SDK source with a comment/source link.

2. `internal/zscaler/failure.go` — `401` session-invalid exhaustion is now also classified as forbidden.

   The SDK artifact shows the session-invalid retry path can trigger on either:

   ```go
   resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden
   ```

   The new classifier cannot distinguish `401` from `403` because the SDK returns a status-free error. Returning `FailureForbidden` is defensible for the external review’s `403 Resource Access Blocked` case, but it may be imprecise for a persistent `401` session-invalid case.

   Why it matters: users or automation that distinguish forbidden from auth/session failures may receive a less precise reason than before the SDK bump.

   How to verify: inspect whether `classifyFailure` has a distinct `FailureUnauthorized` reason for status `401`; if so, this path collapses those cases into forbidden when no status is present.

   Test that would catch it: a classification test for a status-free SDK session-retry error that asserts the intended reason for `401`-originated session exhaustion, or an SDK wrapper that preserves the original status.

3. `internal/zscaler/failure.go` — broad `strings.Contains` could theoretically match unrelated wrapped errors.

   The phrase is SDK-specific, so real-world false positives are unlikely. But because the check is substring-based, any wrapped error whose final message contains the SDK phrase will be classified as forbidden.

   How to verify:

   ```go
   classifyFailure(fmt.Errorf("request failed: %w", errors.New("max SESSION_NOT_VALID retries exceeded (1)")), 0)
   ```

   Test that would catch it: a negative test asserting that unrelated transport or decode errors containing accidental text are not classified as forbidden, if this is considered a real risk.

4. `internal/zscaler/failure_test.go` — the redirect-loop test captures `loop` before assignment.

   The handler references `loop.URL` while `loop` is assigned after `httptest.NewServer` returns. This is safe for the current test because no request occurs until after assignment, but it is a little fragile and could confuse future edits.

   Safer pattern would be to create the server from a handler that closes over a separate base URL variable or use a helper that avoids self-reference before initialization.

5. Process-doc baseline is not included in the review inputs.

   The diff changes runtime classification behavior but does not touch docs, goldens, schemas, or generated artifacts. If docs or machine-contract notes enumerate failure reasons or exit-code mappings, they may need a separate note. This is a review-gap note, not a defect in the diff.

## Machine Contract Review

- Did JSON, NDJSON, error envelope, exit code, dump, diff, schema, manifest,
  or introspection behavior change?

  Yes, indirectly. `classifyFailure` now returns `FailureForbidden` instead of `FailureUnknown` for the SDK session-retry exhaustion error. If failure reason maps into JSON error envelopes or exit codes, that user-visible reason/exit-code class changes.

- Are contract deltas intentional and documented?

  The commit message explicitly states the intent: persistent session-invalid `401`/`403` after SDK retries should be forbidden and non-retryable. No docs/goldens are changed in this diff, so if there is a documented failure-reason matrix, it may need updating separately.

- Are generated docs, schemas, and fixtures in sync with runtime behavior?

  No generated artifacts are changed. There is no evidence of drift in the supplied diff.

## Safety Review

- Did redaction, projection, field coverage, or sensitive-data classification
  change?

  No. The change only classifies an error string. It does not alter output projection, redaction, or field coverage.

- Can a narrowed field set widen output or reveal previously dropped data?

  No. The SDK error text is already present in the error value; the change does not expose response bodies or additional fields.

- Are dump and diff artifacts still sanitized but treated as confidential?

  No dump/diff behavior changed.

## Generated Artifact Review

- Were generated files intentionally regenerated?

  No generated files are present in the diff.

- Are deltas explained?

  No generated deltas exist.

- Any suspicious drops, additions, ordering changes, or count changes?

  None.

## Verdict

Verdict: approve with nits
