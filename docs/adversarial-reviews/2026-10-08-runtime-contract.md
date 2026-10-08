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
- Head reviewed: `3222e6a` on `pr/runtime-contract` (one squashed commit). This artifact
  is added to that commit afterwards and changes no reviewed code.

## Files Changed

- `cmd/zscalerctl/main.go`
- `cmd/zscalerctl/main_test.go`
- `internal/cli/app.go`
- `internal/config/load.go`
- `internal/runtime/runtime_test.go`
- `internal/secretref/resolver.go`
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

Fresh-context reviewer: gpt-6-astra (Codex CLI), read-only, 2026-10-08

## Blocking Findings

None found under the maintainer’s severity bar.

Reviewed `main` (`50135e4`) to `3222e6a` through Git, using the three process documents from `main`. Inspected changed source, tests, runtime callers, and the locally available pinned SDK v3.8.48. No files were edited.

## Non-Blocking Risks

- **Environment fallback has a remaining pre-existing limitation.** At `internal/config/load.go:85`, snapshots belong to each newly loaded config. If an embedded caller omits `ZSCALER_SANDBOX_TOKEN` from `Options.Env` and relies on the process environment, the first read can resolve it, but SDK construction clears it. The second `Engine.Read` reloads configuration and cannot recover that value. The new test supplies the variable in `Options.Env`, covering the normal CLI/engine-host path. Qualify the unconditional “later reads” comment in `internal/zscaler/reader.go:1545` and document the supplied-environment requirement.

- **Parent-deadline coverage is incomplete.** The added cancellation tests exercise `context.Canceled`, while the new branches also change `context.DeadlineExceeded` handling. Add a provider test with a parent deadline shorter than the provider timeout, asserting deadline identity, `deadline_exceeded`, and CLI exit `5`. Separately assert that a provider’s own timeout remains a credential failure.

## Machine Contract Review

- Runtime-construction cancellation now produces `canceled`/exit `1`; parent deadline errors produce `deadline_exceeded`/exit `5`. These align with the existing machine-error mappings.
- Failure to open the null sink now honors JSON, NDJSON, and automatic piped error formatting through the existing `errorFormat` function.
- Resource JSON/NDJSON shapes, command signatures, schemas, manifests, introspection, and dump/diff formats are unchanged.
- The environment fallback preserves explicitly referenced credentials across SDK environment clearing when the host supplies its environment snapshot.
- Builder test results remain unverified: Go builds and test execution were unavailable. `git diff --check main...3222e6a` passed.

## Safety Review

- SDK logging now replaces unsafe text arguments before formatting and scans the resulting message. The inspected pinned SDK calls use argument types handled by this sanitizer.
- Existing request/response-dump and authentication-failure exclusions remain intact.
- Projection, field classification, field narrowing, and resource allow-lists are unchanged; no widening path was introduced.
- Dump/diff sanitization and their confidential-inventory treatment are unchanged.

## Generated Artifact Review

No generated files, fixtures, schemas, coverage reports, or skill copies changed. No command-tree or catalog delta requiring regeneration was identified. Generated checks could not be independently executed.

## Verdict

Verdict: approve with nits