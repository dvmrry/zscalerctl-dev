# Builder Handoff

## Intent

fix(zscaler): classify failed sign-ins and stop following sign-in redirects

A failed sign-in classified as reason `unknown` with no status. The OneAPI SDK
signs in eagerly in `NewOneAPIClient` and the legacy ZIA client lazily before
the first read; both report failure as text (`Failed to sign in the user ...,
got http status: 401, response body: ...`, `HTTP 401 Unauthorized: ...`),
dropping the status and the transport error chain. A rejected client secret,
a wrong vanity domain and a TLS-intercepting proxy were all `unknown`.

- `signInTransport` (`internal/zscaler/sign_in.go`) wraps the HTTP transport
  of both clients. For a POST to `/oauth2/v1/token` or
  `/api/v1/authenticatedSession` it turns any non-2xx response, or a transport
  error, into a typed `*signInError` returned from `RoundTrip`, and keeps the
  last one. Legacy returns transport errors unwrapped, so the type arrives as
  is; OneAPI flattens it (`%v`), so `newOneAPIService` restores the kept one.
  Installed in `perCallService.service` (via `newOneAPIService`) and
  `newLegacyZIAClient`.
- New failure reason `credentials_rejected` (sign-in refused with 400, 401 or
  403), in every copy of the vocabulary: `machine.FailureReasons`, the stdio
  wire list, the error, dump-error and stdio v1 schemas, and the TypeScript
  client. The v1 stdio schema digest is updated in Go, the protocol doc, the
  TypeScript constants and fixture, as in `9291b4f`. A reason crosses the
  machine boundary (`AttachFailure`); an operation label does not.
- `sdkStatusCode` reads the sign-in status. A get whose sign-in 404s is a live
  failure (exit 5), not a missing ID (exit 4).
- Failing every non-2xx sign-in response means a redirect from the token
  endpoint is never followed. Before, the OneAPI client (no `CheckRedirect`)
  followed it and re-posted the client secret to the `Location` host.
- Introspect: exit 3 adds an unresolvable secret; exit 5 is no longer
  code-level `retryable: true` (a rejected credential or forbidden read never
  succeeds on retry) and defers to each error's `reason` and `retryable`. The
  `ExitCodeDoc.Retryable` field and the v3 schema now define the flag that way.

## Base / Head

- Base: `integration/2026-10-09` at `e8f0143`.
- Reviewed head: working tree on `fix/sign-in-failure-reason`.
- Final head: the reviewed change, round-2 doc nits, and this artifact.

## Files Changed

- `internal/zscaler/sign_in.go` (new), `internal/zscaler/sign_in_test.go`
  (new), `internal/zscaler/reader.go`, `internal/zscaler/failure.go`,
  `internal/zscaler/failure_sdk_test.go`,
  `internal/zscaler/admin_audit_logs_test.go` (helper rename to
  `newLegacyZIATestService`)
- `internal/machine/failure.go`, `internal/enginewire/server.go`,
  `internal/enginewire/types.go`, `internal/cli/introspect.go`
- `docs/schema/error.schema.json`, `docs/schema/dump-error.schema.json`,
  `docs/schema/engine-stdio-v1.schema.json`,
  `docs/schema/introspect-v3.schema.json`, `docs/ENGINE_STDIO_PROTOCOL_V1.md`
- `clients/typescript/src/{codec,constants,types}.ts`,
  `clients/typescript/test/fixtures/stderr-flood-engine.cjs`
- `AGENTS.md`, `README.md`, `skills/zscalerctl/SKILL.md` (+ synced `.agents`
  copy), `docs/cli/agent-machine-workflow.md`
- Goldens `introspect`, `introspect-pretty`; `surface_changes.md` row

## Tests Run

- `PATH=/bin:$PATH go test ./...`: pass; reviewer ran `-race` on
  `internal/zscaler`, `internal/machine`, `internal/enginewire`: pass.
- `make fmt-check`, `make verify-agents-skill`, `make verify-typescript-client`,
  `ZSCALERCTL_BASE_REF=integration/2026-10-09 bash scripts/verify-surface-changes-manifest.sh`:
  pass.
- `TestSignInFailuresClassifyByOutcome`: 18 cases (9 per client), OneAPI and legacy, through
  the real SDK, asserting the classification, `machine.FailureDetailOf` and no
  echo of the client ID, username or response body. Without the legacy wiring
  all legacy cases fail.
- `TestSignInDoesNotFollowRedirects`: with the old `>= 400` condition the
  OneAPI client followed a 307 nine times.
- `TestSessionRetriesExhaustedMatchesTheRealSDKError` now runs through
  `newOneAPIService`.

## Known Gaps

- OneAPI sign-ins after construction (per-request refresh, session-invalid
  refresh, renewal ticker) are still flattened to text and classify as before.
  `perCallService` builds a fresh client per call; a long-lived `SDKSession`
  (dump) that outlives its token is affected.
- The wrapper makes net/http use the `req.Cancel` path for `Client.Timeout`, so
  a OneAPI sign-in timeout can read `unknown` instead of `deadline_exceeded`
  (never worse than before). Legacy sign-in timeouts are `deadline_exceeded`.
- A 3xx from a sign-in endpoint fails with its status and reason `unknown`.
- Sign-in detection is method plus path suffix, not host.

## Review History

Strata on gm01 remained stopped, so the review ran in a fresh-context Claude
subagent with no conversation history and read-only repository access.

- Round 1, request changes: the `authenticate` operation relabel never reached
  the CLI envelope, errors.ndjson or the wire (the machine boundary rebuilds the
  error with the request's operation), while the docs promised it. Replaced by
  the `credentials_rejected` reason. Also taken: never follow a sign-in
  redirect (a pre-existing secret re-post), nil body guard, session-refresh test
  through the wrapper, `retryable` wording in docs and the v3 schema.
- Round 2, approve with nits: missed reason lists in the protocol doc and two
  schema descriptions, softer `credentials_rejected` wording, and the manifest
  overclaiming `deadline_exceeded` for a OneAPI sign-in timeout. All taken.

# Adversarial Review

Fresh-context reviewer: Claude subagent (no conversation history, read-only repository access), 2026-10-19 (round 2)

## Blocking Findings

None. The round-1 blocker is resolved: `credentials_rejected` crosses the
machine boundary via `machineErrorFromLoadError` → `AttachFailure` →
`FailureDetailOf` and reaches the CLI envelope, errors.ndjson and the stdio
wire. The vocabulary copies agree. `go test ./...` and `-race` on
`internal/zscaler`, `internal/machine`, `internal/enginewire` pass.

## Non-Blocking Risks

1. A OneAPI sign-in timeout does not reliably become `deadline_exceeded`: with
   the transport wrapped, `Client.Timeout` also closes `req.Cancel`, and the
   recorded cause can be `net/http: request canceled`, which classifies as
   `unknown`. Not a regression; soften the manifest or detect a closed
   `request.Cancel`. (Manifest softened; recorded as a known gap.)
2. Reason lists missed in `ENGINE_STDIO_PROTOCOL_V1.md` and the
   `error`/`dump-error` schema descriptions. (Taken.)
3. `credentials_rejected` overclaims for 400/403, which can also be a malformed
   request, a locked account or API access not enabled. (Wording taken.)
4. The accepted gaps are honestly described; legacy `Client.Timeout`
   classifies as `deadline_exceeded`. (Corrected above.)
5. A sign-in 3xx is `unknown` with its status (acceptable); the redirect test's
   legacy half is guarded by `refuseLegacyZIARedirect` either way; detection
   does not check the host.

## Verdict

Verdict: approve with nits
