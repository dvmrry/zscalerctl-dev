# Builder Handoff

## Intent

fix(runtime): close cancellation, env-ref and SDK log contract gaps

- cmd: credential providers keep context cancellation identity, and the CLI
  reports a cancellation during runtime construction as canceled rather than
  missing_credentials.
- Environment-variable secret references are snapshotted before SDK env
  clearing.
- The null-sink JSON formatter and SDK debug logging no longer carry raw
  values: every text-like argument in a forwarded SDK log format is replaced
  unless it is a strict number, Go duration or HTTP-date (Retry-After values
  in parse warnings and rate-limit summaries included).
- The cmd-provider cancellation test uses load-tolerant waits.


## Base / Head

- Base: `main` at `50135e4`.
- Head reviewed: `10b3bb6` on `pr/runtime-contract` (one squashed commit). This artifact
  is added to that commit afterwards and changes no reviewed code.

## Files Changed

- `cmd/zscalerctl/main.go`
- `cmd/zscalerctl/main_test.go`
- `docs/adversarial-reviews/2026-10-08-runtime-contract.md`
- `internal/cli/app.go`
- `internal/config/load.go`
- `internal/runtime/runtime_test.go`
- `internal/secretref/resolver.go`
- `internal/secretref/resolver_test.go`
- `internal/zscaler/reader.go`
- `internal/zscaler/sdklog.go`
- `internal/zscaler/sdklog_test.go`

## Tests Run

- `make check` (gofmt, vet, `make test` and `make race` including the
  zscalerctl_engine_testhooks runs, docs, surface manifest, machine contract and
  the other repository gates); `make vuln`: no reachable vulnerabilities.
- gitleaks with `.gitleaks.toml` over `main..HEAD`: no leaks.

## Review History

The change was built on per-area branches, each reviewed by fresh-context
reviewers (GPT-6.1 Sol, then GPT-6 Astra) until approved, and then reviewed
again as this single squashed commit against main. The review below is that
final review.

# Adversarial Review

Fresh-context reviewer: gpt-6-astra (Codex CLI), read-only, 2026-10-09

## Blocking Findings

None found under the maintainer’s severity bar.

Reviewed `main` (`50135e4`) through `10b3bb6` using Git, the three process documents from `main`, and locally available SDK v3.8.48 source. No files were edited.

The supplied prior report lists **no blocking findings**, so there are no prior blocking items requiring a fixed/not-fixed disposition.

## Non-Blocking Risks

- **Prior environment-fallback limitation: not fixed; pre-existing.** `Engine.Read` calls `NewMachine`, which reloads configuration. At `internal/config/load.go:85`, `profileSecretEnv` snapshots the supplied environment or falls back to `os.LookupEnv`. If `Options.Env` omits `ZSCALER_SANDBOX_TOKEN`, the first live read can resolve it, but SDK configuration clears the process variable. The second read cannot recover it. The new test supplies the variable explicitly and therefore avoids this limitation. Qualify the unconditional “later reads” claim at `internal/zscaler/reader.go:1545`.

- **Prior parent-deadline coverage gap: not fixed.** The added provider integration tests exercise cancellation, not a parent deadline. Source tracing supports the intended behavior: `resolveCmd` returns `ctx.Err()` before checking its own timeout; runtime wrapping preserves that identity; CLI classification selects `deadline_exceeded` and exit `5`. Add an integration test asserting these results and a separate assertion that a provider-only timeout remains `missing_credentials`/exit `3`. The existing provider-timeout test checks timing and message text.

## Machine Contract Review

- Provider cancellation now retains context identity through credential-resolution wrapping. Resource CLI construction returns the parent context error, and the command boundary emits `canceled`/exit `1`. Engine resource reads prioritize the same sentinel when constructing machine errors.
- **Snapshot precedence follow-up: fixed by source inspection.** `profileSecretEnv` prefers the supplied environment; `Resolver.lookupEnv` now checks that snapshot before the process environment. With snapshot value `from-snapshot` and process value `from-process`, resolution selects `from-snapshot`. The added resolver test covers this conflict and process fallback.
- Null-sink opening failures now use `errorFormat`, preserving JSON envelopes for JSON, NDJSON, and piped automatic output.
- Resource output shapes, command signatures, schemas, manifests, introspection, and dump/diff formats are unchanged.
- `git diff --check main...10b3bb6` passed. Builder-reported builds, tests, race checks, and generated-document checks remain unverified because execution was unavailable.

## Safety Review

- SDK log arguments are sanitized before formatting, then the rendered message is scanned. Inspected SDK call sites use argument types handled by the sanitizer; malformed header strings and error text are replaced while supported numeric/date/duration values remain available.
- Request/response dump and authentication-failure exclusions remain intact.
- Projection, field classification, narrowing, and resource allow-lists are unchanged. No output-widening path was identified.
- Dump/diff sanitization and confidential-inventory treatment are unchanged.

## Generated Artifact Review

No generated documentation, schemas, fixtures, coverage reports, or skill copies changed. No command-tree or catalog delta requiring regeneration was identified.

The added `docs/adversarial-reviews/2026-10-08-runtime-contract.md` records approval of `3222e6a`. Its “changes no reviewed code” statement does not describe the subsequent resolver-precedence change present at `10b3bb6`. Preserve it as historical evidence and append this re-review before treating the artifact as approval of the current head.

## Verdict

Verdict: approve with nits