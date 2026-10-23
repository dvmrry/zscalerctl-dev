# Builder Handoff

## Intent

fix(zscaler): route ZPATWO OneAPI traffic to production hosts

ZPATWO names a ZPA service cloud, not a OneAPI cloud. Passed to the SDK, it
built `zsloginzpatwo.net` for the token and `api.zpatwo.zsapi.net` for every
product call; neither host has a DNS record. The token-host rewrite fixed only
the first request, so a tenant configured as `ZPATWO` could read nothing. The
reader now maps `ZPATWO` (any case) to `PRODUCTION` in `oneAPICloud`, where it
builds the OneAPI SDK configuration, removes the rewrite transport, and no
longer refuses Zidentity on ZPATWO. This supersedes PR #116, which made the
same correction against `main` before Zidentity moved to the OneAPI gateway.

## Base / Head

- Base: `integration/2026-10-09` at `07d54f1`.
- Reviewed head: `f9bfe56`.
- Final head: the review nit fix and this artifact.

## Files Changed

- `internal/zscaler/reader.go`, `reader_test.go`,
  `reader_zidentity_gov_guard_test.go`
- `README.md`, `docs/INSTALL.md`, `docs/THREAT_MODEL.md`,
  `docs/ARCHITECTURE.md`, `docs/schema/config.schema.json`, `man/zscalerctl.1`

## Tests Run

- `go test ./...`, `verify-docs`, `verify-cli-docs`,
  `verify-surface-changes-manifest`, `verify-machine-contract`,
  `verify-typescript-client`, `verify-script-registry`, `git diff --check`:
  pass.
- Mutation: with the mapping removed, the routing test fails.
- DNS: `api.zpatwo.zsapi.net` and `zsloginzpatwo.net` have no record;
  `api.zsapi.net` resolves.
- Live, non-production tenant whose ZPA portal is on ZPATWO, profile cloud
  `PRODUCTION`: ZPA application segments (29) and Zidentity groups (15) read
  through the production hosts.

## Known Gaps

- `auth status`, `doctor` and `config show` display the configured cloud
  (`ZPATWO`), not the effective OneAPI cloud.
- The mapping is fixed: if Zscaler ever serves a ZPATWO OneAPI gateway, the
  CLI keeps using production.

## Review History

Strata on gm01 remained stopped, so the review ran in a fresh-context Claude
subagent with no conversation history and read-only repository access.

- Round 1, approve with nits (below). Acted on: THREAT_MODEL wording.

# Adversarial Review

Fresh-context reviewer: Claude subagent (no conversation history, read-only repository access), 2026-10-16 (round 1)

## Blocking Findings

None. `newSDKConfiguration` is the only OneAPI SDK configuration builder;
per-call, session and dump services all pass through it. Legacy ZIA reads
`cfg.ZIALegacy.Cloud`, and no code imports the SDK's legacy ZPA or SCIM
clients, whose `config.zpatwo.net` and `scim1.zpatwo.net` hosts are
untouched. In the vendored SDK the cloud string only selects hosts. The token
goes to the same host the removed rewrite sent it to; bearer-token calls go to
the gateway every production tenant uses.

## Non-Blocking Risks

1. minor: status and doctor show `cloud: ZPATWO` while the SDK uses
   production hosts. Listed in Known Gaps.
2. minor: the mapping is hard-coded. Documented in the code comment and the
   ARCHITECTURE departures table.
3. nit: circular THREAT_MODEL wording. Fixed.
4. nit: the routing test checks token, ZPA and Zidentity hosts but not ZIA,
   ZTW or ZCC; all use the same base-URL function.

## Verdict

Verdict: approve with nits
