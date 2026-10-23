# Builder Handoff

## Intent

feat: say why a live read failed (error envelope v2, dump error v2)

Three fresh LLM agents drove the CLI against a live non-production tenant
(2026-10-14). Every live failure surfaced as "resource read failed" with no
reason, and `--log-level debug` printed nothing, so they spent most of their
retries guessing between permissions, timeouts and bugs. The reader already
knew the HTTP status, deadlines and the page walker's refusals, but every
site that built a machine error rebuilt it from a bare sentinel.

`normalizeLiveError` now classifies each live failure into a closed,
value-free reason (unauthorized, forbidden, not_found, rate_limited,
server_error, client_error, transport, timeout, pagination_incomplete,
decode_failed, not_subscribed, unknown) with the HTTP status and a retry
hint. `MachineError` carries it as `Detail` (`json:"-"`), attached through
one method, `AttachFailure`, at every site that builds a live_access_failed
or deadline_exceeded error from a reader error; `FailureDetailOf` reads it
everywhere. The stderr error envelope (`zscalerctl.error.v2`) and
`errors.ndjson` records (`zscalerctl.dump.error.v2`) gain optional `reason`,
`status` and `retryable`, and the message names the reason. Readers still
accept v1 error records. The stdio wire is unchanged: its adapter emits only
static failure kinds.

Live result: `zia url-categories`, `dlp-dictionaries` and `network-services`
list failures, previously indistinguishable from permission errors, reported
`pagination_incomplete`, which led to the separate unpaged-endpoint fix.

## Base / Head

- Base: `integration/2026-10-09` at `badd8b4`.
- Reviewed heads: `b606fcc` (round 1), `8cdf17a` (round 2).
- Final head: the nit commit after `8cdf17a` plus this artifact.

## Files Changed

- `.agents/skills/zscalerctl/SKILL.md`, `AGENTS.md`, `README.md`,
  `docs/cli/agent-machine-workflow.md`, `skills/zscalerctl/SKILL.md`
- `docs/schema/README.md`, `docs/schema/error.schema.json`,
  `docs/schema/dump-error.schema.json`
- `cmd/zscalerctl/main.go`, `cmd/zscalerctl/main_test.go`
- `internal/cli/app_test.go`
- `internal/dump/artifact.go`, `internal/dump/dump.go`, `internal/dump/dump_test.go`
- `internal/enginewire/adapter/adapter_test.go`
- `internal/machine/engine_dump.go`, `internal/machine/executor.go`,
  `internal/machine/failure.go`, `internal/machine/failure_test.go`,
  `internal/machine/types.go`
- `internal/runtime/dump.go`, `internal/runtime/dump_engine.go`,
  `internal/runtime/engine.go`, `internal/runtime/failure_detail_test.go`,
  `internal/runtime/url_lookup.go`
- `internal/zscaler/failure.go`, `internal/zscaler/failure_test.go`,
  `internal/zscaler/reader.go`, `internal/zscaler/reader_zia.go`

## Tests Run

- `go test ./...` (Go 1.26.9; macOS ACL tests need `/bin/chmod` first in
  `PATH`), `verify-docs`, `verify-cli-docs`, `verify-surface-changes-manifest`,
  `verify-machine-contract`: pass.
- Live binary against the non-production tenant: the three list failures above
  report `reason: pagination_incomplete`; a ZIA dump's `errors.ndjson` carried
  25 `pagination_incomplete` records and one `client_error` with status 400.

## Review History

Reviewed with no repository access by a fresh-context local model given the
diff, surrounding source and call sites, and this repository's reviewer
prompt and template: Strata serving Qwen3.8-Flash-Next (IQ3_S) on gm01. The
reviewer could not run commands.

- Round 1, request changes. Blocking: `AttachFailure` rewrites
  `MachineError.Message`, so the stdio protocol might not be byte-identical.
  Refuted with evidence and pinned: the stdio adapter turns every machine
  error into a static failure kind and dump failures into
  product/resource/phase/kind, and `machineio` has no non-test caller
  (`TestLiveFailureDetailDoesNotReachTheStdioWire`). Applied non-blocking
  items: `FailureDetailOf` reads `MachineError.Detail`; 408 and network
  timeouts are `timeout`; wiring tests for every runtime boundary and an
  end-to-end dump test; a schema-enum drift test.
- Round 2, approve with nits (below). Applied: `NOT_SUBSCRIBED` is a sentinel
  matched with `errors.Is`; `FailureDetailOf` reads `MachineError` values too;
  the two timeout constants are tested equal.

Accepted and not changed: `machine.DumpResourceError` is never marshalled
(the adapter copies four fields; pinned by the stdio test), so its `Detail`
needs no tag; message sanitization at boundaries is unchanged (fixed strings
plus a value-free reason).

# Adversarial Review

Fresh-context reviewer: Strata (Qwen3.8-Flash-Next IQ3_S, gm01), read-only, no command execution, 2026-10-14 (round 2)

Editor's note: a quoted test canary shaped like an assigned client secret is shown as `client_secret=<test-canary>` so the docs check and secret scanners accept this record; the review text is otherwise verbatim.

## Blocking Findings

None. The round-1 blocking stdio concern is not reproduced by the supplied artifacts.

The supplied evidence supports the builder’s claim for the stdio wire:

- `internal/enginewire/adapter/errors.go` `ToOperationError` converts a `machine.MachineError` into either a canceled outcome or a static wire failure:
  - `context.Canceled` / `machine.ErrorKindCanceled` -> `ErrorConversion{Canceled: true}`
  - `context.DeadlineExceeded` -> `NonCredentialFailure{Kind: FailureDeadlineExceeded}`
  - missing credentials -> `MissingCredentialsFailure{Kind, Missing}`
  - otherwise -> `NonCredentialFailure{Kind: operationFailureKind(...)}`

  It does not copy `MachineError.Message`, `Detail`, `Reason`, `Status`, or `Retryable`.

- `ToDumpSummary` converts each `machine.DumpResourceError` to only:
  - `Product`
  - `Resource`
  - `Phase`
  - `Kind`

  It does not copy `Detail`.

- The new `TestLiveFailureDetailDoesNotReachTheStdioWire` pins both behaviors:
  - a live-access `MachineError` with `Message` and `Detail` becomes `enginewire.NonCredentialFailure{Kind: enginewire.FailureLiveAccess}`
  - a dump failure with `Detail` is marshalled without `rate_limited`, `429`, `retryable`, or `Detail`

I did not find a path in the supplied diff where `MachineError.Message` or `MachineError.Detail` is serialized to a machine protocol. The conclusion does depend on the supplied `rg` result that `machineio.ExecuteJSON` / `machineio.EncodeResponse` have no non-test callers; that result is not independently re-runnable from the prompt, but it is concrete evidence and no contradicting path appears in the diff.

## Non-Blocking Risks

### 1. `NOT_SUBSCRIBED` classification is now substring-based and can be too broad

- file/path: `internal/zscaler/failure.go`
- concrete issue: `classifyFailure` now uses:
  ```go
  case err != nil && strings.Contains(err.Error(), "NOT_SUBSCRIBED"):
  ```
  instead of a prefix match or sentinel match.
- why it matters: any error whose rendered message happens to contain `NOT_SUBSCRIBED` will be classified as subscription-not-enabled, even if the more specific failure is transport, decode, pagination, or another wrapped condition. This can change `reason`, `retryable`, and potentially CLI exit-code behavior.
- how to reproduce or verify: construct a wrapped error that is a network error or decode error but whose message contains `NOT_SUBSCRIBED`, then call `classifyFailure` or the public normalize path.
- test that would catch it: add a table case where a `*net.OpError` or decode error wraps a message containing `NOT_SUBSCRIBED`, and assert the intended classification. Better: introduce a sentinel error such as `ErrNotSubscribed` and use `errors.Is`, then test both wrapped and unwrapped sentinel behavior.

### 2. The new wrapped `NOT_SUBSCRIBED` test proves classification but not CLI exit-code or envelope behavior

- file/path: `internal/zscaler/failure_test.go`
- concrete issue: the new test:
  ```go
  {"wrapped not subscribed", fmt.Errorf("read: %w", errors.New("NOT_SUBSCRIBED: subscription is required")), FailureNotSubscribed, 0, false},
  ```
  verifies only the low-level failure reason/status/retryable tuple.
- why it matters: if CLI exit-code mapping or error-envelope behavior depends on `reason`, wrapping can change user-visible exit status. The builder notes that 403 with `NOT_SUBSCRIBED` stays forbidden because the reader converts the subscription error to a status-free error before classification, but the supplied diff does not show that reader conversion.
- how to reproduce or verify: run a CLI-level test where the reader returns a wrapped `NOT_SUBSCRIBED` error and assert exit code, stdout, stderr, and error envelope.
- test that would catch it: a CLI test asserting that a wrapped subscription error yields the expected `not_subscribed` reason and expected exit code, plus a reader test asserting that a 403 subscription response is converted to a status-free subscription error before classification.

### 3. `FailureDetailOf` now reads `MachineError.Detail`, but the helper only matches pointer `MachineError` and can shadow context deadline classification

- file/path: `internal/machine/failure.go`
- concrete issue: the new code is:
  ```go
  var machineErr *MachineError
  if errors.As(err, &machineErr) && machineErr.Detail.Reason != "" {
      return machineErr.Detail
  }
  if errors.Is(err, context.DeadlineExceeded) {
      return FailureDetail{Reason: FailureReasonTimeout, Retryable: true}
  }
  ```
  This only matches `*MachineError`, whereas the adapter has an explicit helper that handles both pointer and value `MachineError`. It also returns a non-empty `MachineError.Detail` before checking `context.DeadlineExceeded`.
- why it matters: if a value `MachineError` is wrapped, `FailureDetailOf` may ignore its detail. If a `MachineError` wraps a context deadline but already has a non-empty, non-timeout `Detail.Reason`, the timeout classification can be shadowed.
- how to reproduce or verify: wrap `machine.MachineError{Detail: ...}` as a value, and separately wrap `context.DeadlineExceeded` inside a `MachineError` with `Detail.Reason: "unknown"`.
- test that would catch it: add tests for:
  - wrapped value `machine.MachineError` with detail
  - wrapped `context.DeadlineExceeded` inside `MachineError` with empty detail
  - wrapped `context.DeadlineExceeded` inside `MachineError` with non-empty non-timeout detail, asserting the intended precedence

### 4. Runtime boundary tests pin detail but not message sanitization for all boundary constructors

- file/path: `internal/runtime/failure_detail_test.go`
- concrete issue: the test asserts `Detail` for:
  - `resourceReadRuntimeBoundaryError`
  - `dumpRuntimeBoundaryError`
  - `urlLookupBoundaryError`

  but it only asserts the described `Message` for `dumpMachineError`:
  ```go
  if dumpErr.Detail != want || dumpErr.Message != "resource read failed: forbidden (HTTP 403)" {
  ```
- why it matters: the safety claim is that the new described `MachineError.Message` is value-free. If another boundary constructor sets `MachineError.Message` from `source.Error()`, CLI error envelopes could leak raw source text while still passing this test.
- how to reproduce or verify: inspect each boundary constructor for direct use of `source.Error()` or `err.Error()` in `Message`. The supplied diff does not include those constructor bodies.
- test that would catch it: in the same test, assert for each boundary error that `machineErr.Message` does not contain the sentinel source text, for example:
  ```go
  if strings.Contains(machineErr.Message, "zscaler API request failed") ||
     strings.Contains(machineErr.Message, "list zia/locations") {
      t.Errorf("boundary %s leaked source message: %q", name, machineErr.Message)
  }
  ```

### 5. The dump end-to-end redaction test checks only `errors.ndjson`, not stdout/stderr

- file/path: `internal/cli/app_test.go`
- concrete issue: `TestDumpErrorRecordsCarryTheReadFailureReason` uses a reader error whose message is:
  ```go
  "client_secret=<test-canary>"
  ```
  and asserts the exact `errors.ndjson` line. It does not assert that stdout or stderr omit the raw sentinel.
- why it matters: the comment says “and nothing else,” but the test only checks one artifact. If the CLI writes the reader error to stderr or stdout, the test would still pass.
- how to reproduce or verify: run the test with an assertion that `out.String()` and `errOut.String()` do not contain `client_secret`.
- test that would catch it: add:
  ```go
  if strings.Contains(out.String(), "client_secret") ||
     strings.Contains(errOut.String(), "client_secret") {
      t.Fatalf("raw reader error leaked to CLI output")
  }
  ```

### 6. `machine.DumpResourceError.Detail` remains untagged; safety depends on the type never being directly marshalled

- file/path: `internal/machine` / `internal/enginewire/adapter/errors.go` evidence
- concrete issue: `ToDumpSummary` only copies four fields, but `machine.DumpResourceError.Detail` is described as untagged. If any code directly marshals `machine.DumpResult` or `machine.DumpResourceError`, the untagged `Detail` field will serialize.
- why it matters: the current stdio safety depends on the adapter being the only serialization path and on `machineio` having no non-test callers. A future wiring change could bypass the adapter and leak detail.
- how to reproduce or verify: call `json.Marshal` on a `machine.DumpResult` containing a `DumpResourceError` with detail.
- test that would catch it: either add `json:"-"` to `Detail` and assert it is omitted, or add a test that direct marshalling of machine dump types never emits `Detail`, `Reason`, `Status`, or `Retryable`.

### 7. The schema drift test is useful but narrow

- file/path: `internal/zscaler/failure_test.go`
- concrete issue: `TestFailureReasonsMatchPublishedSchemas` checks that the `reason` enum in `docs/schema/error.schema.json` and `docs/schema/dump-error.schema.json` equals `failureReasons`.
- why it matters: this prevents enum drift, but it does not prove that every reason is actually emitted by `classifyFailure`, that the schema files are the schemas used by the CLI/dump writer, or that status/retryable types match runtime behavior.
- how to reproduce or verify: add a reason to `failureReasons` and the schema without wiring it into `classifyFailure`; the current test would still pass.
- test that would catch it: table-drive `classifyFailure` across every reason, or add a generated/contract test that validates actual CLI/dump error records against the schema.

### 8. Timeout vocabulary is duplicated without an equality test

- file/path: `internal/machine/failure.go`, `internal/zscaler/failure.go`
- concrete issue: `machine.FailureReasonTimeout` and `zscaler.FailureTimeout` both appear to be `"timeout"`, but the new schema test only covers the zscaler list.
- why it matters: if one constant changes, CLI/dump reason values and stdio/engine classification can drift silently.
- how to reproduce or verify: change one constant and run tests; current tests may not catch it.
- test that would catch it: add a simple equality assertion:
  ```go
  if zscaler.FailureTimeout != machine.FailureReasonTimeout { ... }
  ```

## Machine Contract Review

- Did JSON, NDJSON, error envelope, exit code, dump, diff, schema, manifest, or introspection behavior change?

  The fix commit itself mostly adds tests and small helper changes. The runtime-visible behavior that changed is that `machine.FailureDetailOf` now reads `MachineError.Detail`, and `cmd/zscalerctl/main.go` no longer has a special case for that. This makes CLI error envelopes and dump error records use the same detail-resolution path.

  The new CLI test pins dump `errors.ndjson` to schema `zscalerctl.dump.error.v2` with:
  ```json
  {"schema":"zscalerctl.dump.error.v2","product":"zia","name":"rule-labels","operation":"list","kind":"list_failed","reason":"rate_limited","status":429,"retryable":true}
  ```

  The stdio wire is pinned as unchanged by `TestLiveFailureDetailDoesNotReachTheStdioWire`.

- Are contract deltas intentional and documented?

  The dump error v2 fields and schema enum drift test make the machine-contract intent clearer. The stdio narrowing is also pinned. However, the supplied review inputs do not include the process-doc baseline or the original b606fcc contract docs, so I cannot independently confirm that the CLI/dump contract change is documented in the intended baseline.

- Are generated docs, schemas, and fixtures in sync with runtime behavior?

  The builder reports `verify-docs`, `verify-cli-docs`, `verify-surface-changes-manifest`, and `verify-machine-contract` pass on `8cdf17a`. The fix diff itself does not show generated artifact changes. The `.agents` copy claim cannot be source-verified from the supplied diff. The new schema enum test is a good guard, but it only checks reason enums.

## Safety Review

- Did redaction, projection, field coverage, or sensitive-data classification change?

  Yes, in a controlled direction: `FailureDetailOf` now propagates `MachineError.Detail` to consumers that previously may have ignored it. The new tests show that value-free detail is preserved while raw reader messages are not written to dump error records.

- Can a narrowed field set widen output or reveal previously dropped data?

  For the stdio wire, no path found in the supplied diff. `ToOperationError` emits static failure kinds, and `ToDumpSummary` emits only product/resource/phase/kind.

  For CLI/dump artifacts, output is intentionally widened to include `reason`, `status`, and `retryable` where available. The new tests show those fields are value-free for the exercised paths.

- Are dump and diff artifacts still sanitized but treated as confidential?

  The new dump test shows a classified read failure writes only schema/product/name/operation/kind/reason/status/retryable to `errors.ndjson`, and the sentinel raw secret does not appear in that artifact. The test should be strengthened to assert stdout/stderr also omit the sentinel.

## Generated Artifact Review

- Were generated files intentionally regenerated?

  The fix diff does not include generated file changes. The builder says `.agents` is regenerated by `scripts/sync-agents-skill.sh` and checked by `verify-docs`.

- Are deltas explained?

  The commit message explains the fix and the added tests. It does not show generated artifact deltas because none are present in the fix diff.

- Any suspicious drops, additions, ordering changes, or count changes?

  None visible in the supplied diff. The new schema drift test prevents reason enum drift, but it does not prove all generated docs/skill copies are current.

## Verdict

Verdict: approve with nits
