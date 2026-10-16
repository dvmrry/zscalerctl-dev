# Builder Handoff

## Intent

fix(zscaler): accept only OneAPI gateway clouds in ZSCALERCTL_CLOUD

`ZSCALERCTL_CLOUD` names the OneAPI gateway, not the cloud a tenant's
products run on. Tenants on zscalertwo, zscalerthree, ZPATWO and other
product clouds all use `PRODUCTION` (operator report; the non-production
tenant is on ZPATWO and reads through PRODUCTION). The SDK builds the OAuth
host, which receives the client secret, and the API host from any string; for
product cloud names neither host exists. This replaces the ZPATWO-only
mapping merged earlier today with a closed list (`PRODUCTION`, `ALPHA`,
`BETA`, `GOV`, `GOVUS`, any case), as legacy ZIA already has. Other values are
rejected before the client secret is resolved, with an error that says to use
PRODUCTION, and `auth status`/`doctor` report `invalid cloud`.

## Base / Head

- Base: `integration/2026-10-09` at `b28f7b6`.
- Reviewed heads: `0507eba` (round 1), `7b493ae` (round 2).
- Final head: the round-2 test nit and this artifact.

## Files Changed

- `internal/zscaler/reader.go`, `reader_test.go`,
  `reader_zidentity_gov_guard_test.go`
- `internal/runtime/runtime.go`, `runtime_test.go`, `status.go`,
  `status_test.go`, `internal/cli/config_init.go`
- `README.md`, `docs/INSTALL.md`, `docs/THREAT_MODEL.md`,
  `docs/ARCHITECTURE.md`, `docs/schema/config.schema.json`,
  `man/zscalerctl.1`, `examples/zscalerctl.env.example`

## Tests Run

- `go test ./...`, `verify-docs`, `verify-cli-docs`,
  `verify-surface-changes-manifest`, `verify-machine-contract`,
  `verify-typescript-client`, `verify-script-registry`, `gofmt`,
  `git diff --check`: pass.
- DNS: `api.zsapi.net`, `api.alpha.zsapi.net`, `api.beta.zsapi.net`,
  `api.zscalergov.net`, `api.zscalergov.us` and the matching token hosts
  resolve; `api.<x>.zsapi.net` has no record for zscaler, zscalerone,
  zscalertwo, zscalerthree, zscloud, zscalerbeta, zscalerten, zscalergov,
  zpatwo or preview.
- Live, non-production tenant (ZPATWO, profile `PRODUCTION`): ZPA read OK.

## Known Gaps

- `doctor` prints `Status: OK` while credentials are `invalid cloud`, as it
  already did for `not configured`.
- An invalid cloud reuses `ErrMissingCredentials`, as legacy ZIA does.

## Review History

Strata on gm01 remained stopped, so the review ran in a fresh-context Claude
subagent with no conversation history and read-only repository access.

- Round 1, request changes. Blocking: ALPHA is a real OneAPI cloud (SDK
  README; `api.alpha.zsapi.net` and `zsloginalpha.net` resolve) and the list
  rejected it, while the docs claimed only four gateways exist. Minor: auth
  status said "configured" for an invalid cloud; the routing test lacked GOV
  and GOVUS. Acted on: ALPHA added, docs say "supported", routing pins for
  ALPHA/GOV/GOVUS, `invalid cloud` status for OneAPI and legacy ZIA.
- Round 2, approve with nits (below). Acted on: the status test now checks
  each mode's message by variable name.

# Adversarial Review

Fresh-context reviewer: Claude subagent (no conversation history, read-only repository access), 2026-10-16 (round 2)

## Blocking Findings

None. Round-1 findings re-verified as fixed. `NewReader` →
`validateReaderConfig` is the only way to build a SDK service, and runtime
validates before the only client-secret resolution. The error does not echo
the input. `credentials` and `live_api` are free-form strings in the engine
schema and TypeScript codec, so `invalid cloud` breaks no contract; doctor
goldens cover only the unconfigured case.

## Non-Blocking Risks

1. nit: `doctor` prints `Status: OK` with an invalid cloud (pre-existing for
   `not configured`). Listed in Known Gaps.
2. nit: the status test did not tell the OneAPI and legacy messages apart.
   Fixed.
3. nit: `ErrMissingCredentials` for a set-but-unsupported value; consistent
   with legacy ZIA.

## Verdict

Verdict: approve with nits
