# Builder Handoff

## Intent

fix(cli): name an ignored HTTPS_PROXY on a timeout or transport failure

From the external re-check of `6e7040e`: `doctor` names an `HTTPS_PROXY` that
is set but not used, but a live read that then times out reports only
`deadline_exceeded`/`timeout`, so the cause is visible only from `doctor`.

- `config.EnvironmentProxyIgnored(environ)`: `HTTPS_PROXY`/`https_proxy` set,
  `ZSCALERCTL_PROXY_URL` unset and `ZSCALERCTL_PROXY_FROM_ENV` not true. It
  reads only those variables (no secret files). `environmentProxySet` is
  shared with `LoadEnv`, and `config.IgnoredProxyHint` with the doctor
  status, so the two say the same thing.
- `cli.withProxyHint`, applied once in `App.Run` on failure: when
  `machine.FailureDetailOf(err).Reason` is `timeout` (any deadline) or
  `transport` and the proxy is ignored, the error becomes
  `proxyHintError`, whose text appends ` (HTTPS_PROXY is set but not used;
  set ZSCALERCTL_PROXY_FROM_ENV=true to use it)`. It unwraps to the original
  error, so the envelope kind, reason, status, retryable and exit code are
  unchanged; only `message` gains the hint. Never the proxy value.

Not covered: the engine stdio host (it takes explicit config, no ambient
proxy), per-resource dump `errors.ndjson` records. An embedder that passes a
deadline context to a `--from-dump` read would see the hint on an offline
read; the CLI sets no context deadline, so it cannot reach that.

## Base / Head

- Base: `integration/2026-10-09` at `6e7040e`.
- Reviewed head: `fix/timeout-proxy-hint` (staged).

## Files Changed

- `internal/config/config.go`, `internal/runtime/status.go`
- `internal/cli/app.go`, `internal/cli/proxy_hint.go` (new),
  `internal/cli/proxy_hint_test.go` (new),
  `internal/cli/proxy_hint_internal_test.go` (new)
- `docs/INSTALL.md`

## Tests Run

- `PATH=/bin:$PATH go test ./...`, `make fmt-check`: pass.
- `TestLiveTimeoutNamesIgnoredEnvironmentProxy`: through `App.Run`, a timeout
  carries the hint only when the proxy is ignored (upper/lowercase), not with
  `ZSCALERCTL_PROXY_FROM_ENV=true`, an explicit proxy, no proxy, or a
  non-timeout failure; the kind stays `deadline_exceeded`, reason `timeout`,
  exit 5; the proxy URL is never echoed.
- `TestWithProxyHintCoversTransportFailuresOnce`: `transport` gets the hint
  once even if applied twice and still unwraps to the same `MachineError`;
  `forbidden` does not.
- `TestNewDoctorStatusNamesIgnoredEnvironmentProxy` still passes on the
  shared constant.

## Review History

- Round 1 (fresh-context Claude subagent), approve with nits. Taken: a test
  for an unparseable `ZSCALERCTL_PROXY_FROM_ENV` (in-package, since `App.Run`
  rejects it as `invalid_config` first) (the `err == nil` term
  survived mutation). Recorded: the `--from-dump` deadline case above; a DNS
  not-found from a wrong vanity domain also gets the hint, which stays true.

# Adversarial Review

Fresh-context reviewer: Claude subagent (no conversation history, read-only repository access), 2026-10-21 (round 1)

## Blocking Findings

None.

## Non-Blocking Risks

- The parse-error term of `EnvironmentProxyIgnored` was untested; `LoadEnv`
  rejects that value first in a real run. In-package test added.
- A deadline on a `--from-dump` read would get the hint; unreachable from the
  CLI (no context deadline). Recorded.
- Transport covers a DNS not-found from a wrong vanity domain; the hint is
  still true there. Accepted.

## Checked, Sound

- Error identity: `App.Run`'s only caller and the envelope, exit-code and
  kind code use `errors.Is`/`errors.As`; the wrapper keeps `MachineError`,
  `ErrorContexter`, `MissingCredentialsError`, `ResourceNotFoundError` and the
  sentinels reachable.
- End to end through `runApp` (json and table, all modes, deadline and
  transport): exit 5, kind/reason/retryable/operation/product/resource
  unchanged; the message carries the hint verbatim; the proxy host and any
  `user:password` in it never appear.
- Partial dump, drift, `--output` and notices unchanged; Ctrl-C never
  classifies as transport; secret-ref and keyring deadlines carry no reason.
- `EnvironmentProxyIgnored` agrees with `LoadEnv`; doctor and the error share
  `config.IgnoredProxyHint`.

## Tests Run

- `go test ./...`, `make fmt-check`, `go vet` (cli, config, runtime, cmd).
- Overlay envelope test; mutations m1-m5 and c1-c4, all killed except the
  parse-error term (now pinned).

Verdict: approve with nits
