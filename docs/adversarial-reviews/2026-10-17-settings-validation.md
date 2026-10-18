# Builder Handoff

## Intent

fix(config): report unusable settings as invalid_config, and doctor readiness

An unsupported `ZSCALERCTL_CLOUD`, a vanity domain that is not one DNS
label, an unreviewed legacy `ZSCALERCTL_ZIA_CLOUD` or an unknown auth mode
failed as `missing_credentials` (exit 3) with "missing zscaler API
credentials:" in front of the real reason. `doctor` printed `Status: OK`
whatever the configuration. The reader, runtime and status views each judged
readiness their own way, so they could disagree.

- `zscaler.ValidateSettings` is the one check of the non-secret settings.
  Reader construction, runtime and status all call it. A failure is an
  `InvalidSettingsError` wrapping `ErrInvalidSettings`: kind
  `invalid_config`, exit 2. Its reason is one of a fixed set of constants
  that never include the configured value, so machine boundaries pass the
  message through.
- `config.Config.MissingCredentials` names the unset required values by
  presence alone. Runtime runs the settings check, then this, before
  resolving any secret, so no provider runs for a read certain to fail.
- Status derives `doctor` status (`OK` or `not ready`), `credentials`
  (`configured`, `partial`, `invalid settings`, `not configured`) and
  `live_api` from one typed credential state built from the same two checks.
  An unusable setting wins over missing values everywhere.

## Base / Head

- Base: `integration/2026-10-09` at `88f752f`.
- Reviewed head: staged tree on `fix/settings-validation` (round 2).
- Final head: round-2 nit fixes and this artifact.

## Files Changed

- `internal/zscaler/reader.go`, `url_lookup.go`, `admin_audit_logs.go`
- `internal/config/config.go` (`MissingCredentials`)
- `internal/runtime/runtime.go`, `status.go`, `engine.go`, `dump.go`,
  `dump_engine.go`, `url_lookup.go`
- `cmd/zscalerctl/main.go`, `internal/cli/render_records.go`,
  `internal/cli/introspect.go`
- tests: `reader_test.go`, `runtime_test.go`, `status_test.go`,
  `invalid_settings_test.go`, `main_test.go`, `app_test.go`; doctor and
  introspect goldens
- `AGENTS.md`, `skills/zscalerctl/SKILL.md` (+ `.agents` copy),
  `docs/INSTALL.md`, `docs/cli/machine-contract.md`

## Tests Run

- `PATH=/bin:$PATH go test ./...`, `verify-docs`, `verify-cli-docs`,
  `verify-surface-changes-manifest`, `verify-machine-contract`,
  `verify-typescript-client`, skill sync: pass.
- Offline with synthetic values: a read with `ZSCALERCTL_CLOUD=zscalertwo`
  returns `invalid_config`, exit 2, with the cloud reason; `doctor` says
  `not ready`, and `OK` with `PRODUCTION`.
- Mutations: disabling the new boundary case in `engine.go` or `main.go`
  fails its test.

## Known Gaps

- `doctor` still exits 0 when it reports `not ready`; the status field is the
  signal.
- Full legacy credentials plus a `ZSCALERCTL_CLOUD` value make the CLI infer
  OneAPI, so the user sees the OneAPI cloud error rather than a legacy hint
  (unchanged behaviour).

## Review History

Strata on gm01 remained stopped, so the review ran in fresh-context Claude
subagents with no conversation history and read-only repository access.

- Round 1, request changes. Blocking: with `zia-legacy` and no legacy cloud,
  a `cmd:` secret provider ran twice before the read failed (the base ran it
  0 times); OneAPI already resolved the secret with a missing vanity domain
  or client id. Also: status said `not configured` when only an invalid cloud
  was set while a read said `invalid_config`; no test pinned the five new
  boundary mappings; a comment overstated who can build the error; the
  introspect exit-2 text did not mention settings. Acted on:
  `MissingCredentials` runs before any secret is resolved, with a test that
  the provider runs 0 times; the settings check runs first in status; tests
  cover every boundary mapping and the CLI envelope; comment and text fixed.
- Round 2, approve with nits (below). The reviewer counted provider runs
  against base and head builds: 0 on head for every missing or invalid case.

# Adversarial Review

Fresh-context reviewer: Claude subagent (no conversation history, read-only repository access), 2026-10-17 (round 2)

## Blocking Findings

None. All round-1 findings re-verified as fixed. `MissingCredentials` agrees
with `Configured()` and the reader's own check in names, order and
whitespace handling; every caller normalizes nil secret sources first. No
caller relied on getting a reader config with incomplete credentials.

## Non-Blocking Risks

1. nit: the reader checked missing values before settings, the opposite of
   runtime and status. Fixed: settings first.
2. nit: `machine-contract.md` repeated the round-1 overstatement. Fixed.
3. nit: in the partial state `live_api` listed every required variable.
   Fixed: it lists only the missing ones.
4. minor: `doctor` exits 0 when not ready. Left; Known Gaps.
5. minor: legacy credentials plus a OneAPI cloud infer OneAPI. Pre-existing;
   Known Gaps.

## Verdict

Verdict: approve with nits
