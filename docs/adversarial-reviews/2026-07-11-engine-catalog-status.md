# Builder Handoff

## Intent

Extend the candidate in-process Go engine with config-free typed catalog
discovery and config-backed, SDK-free doctor/auth/config status capabilities.
Route the existing CLI commands through those typed results without changing
their supported JSON keys, table/pretty shapes, schemas, or generated
artifacts. Fail closed at catalog, status-value, error, and cancellation
boundaries before future local frontends consume them.

## Base / Head

Base commit: `e23389a`

Initial implementation head: `6d04dde`

Reviewed fix heads: `f6e431e`, `3204124`, `83f38a5`

Review scope: `e23389a..83f38a5`

## Files Changed

Design and contract documentation:

- `docs/ENGINE_API_DESIGN.md`
- `docs/ENGINE_CAPABILITY_MODEL.md`
- `docs/cli/machine-contract.md`

CLI adapter and regressions:

- `internal/cli/app_test.go`
- `internal/cli/commands_config_schema_auth.go`
- `internal/cli/commands_core.go`
- `internal/cli/status.go`

Typed machine capability model:

- `internal/machine/engine_catalog.go`
- `internal/machine/engine_catalog_status_test.go`
- `internal/machine/engine_manifest.go`
- `internal/machine/engine_manifest_test.go`
- `internal/machine/engine_status.go`
- `internal/machine/executor.go`
- `internal/machine/manifest.go`
- `internal/machine/types.go`

Trusted runtime assembly and regressions:

- `internal/runtime/catalog.go`
- `internal/runtime/catalog_test.go`
- `internal/runtime/engine.go`
- `internal/runtime/engine_test.go`
- `internal/runtime/runtime.go`
- `internal/runtime/runtime_test.go`
- `internal/runtime/status.go`
- `internal/runtime/status_test.go`

## Source Inputs Consulted

- `AGENTS.md` and the adversarial-review workflow, run prompt, handoff, and
  report templates
- the accepted engine API and capability design checkpoints
- the static resource catalog and its tenant-read-only assertion
- config loading, safe config views, secret-source/provider interfaces, proxy
  validation, redaction, output rendering, and command-boundary error mapping
- the supported `machine.v1` manifest/schema fixtures, introspection schemas and
  goldens, generated CLI docs, surface fixtures, and generated skill gates

## Generated Artifacts

None. No supported schema, machine fixture, introspection golden, CLI surface
golden, generated CLI documentation, field-coverage artifact, release artifact,
or generated skill changed.

## Expected Delta

- Add candidate `engine.v1` `catalog.schema` and `status.inspect` capabilities
  as in-process Go values only.
- Add `runtime.Engine`, typed catalog/status requests and closed results, and a
  status inspector that retains only precomputed safe views.
- Keep catalog/manifest discovery config-, filesystem-, provider-, SDK-,
  process-, and network-free.
- Preserve existing CLI status output shapes while removing terminal-control
  runes and raw configuration/backend detail before values or errors cross the
  engine boundary.
- Return operation-specific canceled/deadline machine errors before CLI status
  commands access config when their context is already finished.

## Invariants Claimed

- The engine rejects injected catalogs containing any tenant-mutating
  operation. Lower-level discovery suppresses catalog/resource-read
  capabilities for such a catalog instead of advertising an unexecutable path.
- Catalog snapshots recursively copy operation, mode, and nested-field slices.
- Typed catalog/status requests, results, and the engine manifest reject direct
  JSON; no candidate wire protocol is introduced.
- Status construction never resolves deferred secret providers, constructs an
  SDK reader, executes a process, or contacts Zscaler.
- Status strings are redacted and normalize Unicode control and format runes.
- Config, option, proxy, context, and unknown-operation failures expose static
  `MachineError` values and retain only safe sentinel identity.
- Unknown status operations are rejected before config loading and never echo
  caller-controlled operation text.
- Existing status JSON keys, human output structure, error kinds/exit mapping,
  schemas, and generated artifacts remain unchanged. Finished-context handling
  now conforms to the documented canceled/deadline taxonomy.

## Tests Run

Builder verification, all passing at the applicable reviewed fix heads:

- `go test -count=20 ./internal/machine ./internal/runtime ./internal/cli`
- `go test -count=20 ./internal/runtime ./internal/cli`
- `go test -count=1 ./internal/machine ./internal/runtime ./internal/cli ./cmd/zscalerctl`
- `go test -race -count=1 ./internal/machine ./internal/runtime ./internal/cli ./cmd/zscalerctl`
- `git diff --check`
- repeated clean `env -u GOFLAGS make check` runs, including repository-wide
  tests/race, vet, staticcheck, vulnerability scans, docs, schema/contract,
  boundary, secret, workflow, release, surface, generated-artifact, and skill
  synchronization gates

Final fresh-context reviewer verification included non-cached status/CLI and
machine/command tests, targeted race, CLI-doc drift, machine-contract, and PTY
escape-clean checks.

## Known Deferrals

- typed ZIA URL lookup
- typed dump and diff, including artifact-level dump completion and strict
  diff-field admission
- versioned stdio DTOs, framing, codecs, cancellation, and reference clients
- frontend, MCP, Wails, Ink/OpenTUI, Ratatui, or GUI adapters
- public Go API or transport promotion

## Review Focus

- advertised capabilities versus executable behavior for empty, read-only, and
  mixed mutating catalogs
- config-free discovery and defensive catalog copying
- status provider/SDK/process/network absence
- status result closure, secret redaction, ANSI/C1/bidi/control injection, and
  malicious provider scheme strings
- raw config path, provider, backend, and proxy error leakage
- unknown-operation and finished-context ordering before config access
- operation, error-kind, exit-code, JSON/table/pretty, schema, and generated
  artifact compatibility

# Finding Resolution

## Mixed catalogs over-advertised executable capabilities

Finding: `EngineManifestFromCatalog` advertised catalog and resource-read
capabilities for a catalog that `DiscoverCatalog` rejected as mutating.

Fix: `runtime.NewEngine` rejects mutating injected catalogs, and lower-level
manifest derivation omits catalog/resource-read capabilities when the catalog
is not tenant-read-only.

Regression: synthetic mixed catalogs are rejected at construction and expose
only independent manifest/status discovery at the lower-level derivation seam.

## Status values admitted terminal-control and format runes

Finding: redaction alone allowed newline, ANSI escape, C1, bidi, and other
Unicode-format runes in profile, cloud, and provider-scheme strings.

Fix: status strings pass through redaction and then normalize all Unicode
control and `Cf` runes to spaces before entering a result.

Regression: malicious values are exercised through runtime results and CLI
JSON/table/pretty output for doctor, auth status, and config show.

## Loader and option errors escaped raw details

Finding: status config-loader and option errors could retain paths, provider or
backend text, and unknown status operations were echoed after config loading.

Fix: static status boundary errors preserve only safe config/context sentinels;
unknown operations are validated before loading and omit the untrusted value.
CLI status config-load failures use the same boundary.

Regression: generic and invalid-config canaries, unknown operations, load
counts, sentinel identity, and CLI config paths are covered directly.

## Invalid proxy status lacked stable machine classification

Finding: doctor retained a safely worded proxy sentinel but returned it without
a `MachineError`, so typed callers lost stable kind and operation fields.

Fix: proxy-validation failures become static
`invalid_proxy_config`/`doctor` machine errors while preserving only
`zscaler.ErrInvalidProxyConfig`.

Regression: both `StatusInspector` and `Engine.InspectStatus` assert machine
classification, operation, sentinel identity, and absence of proxy canaries.

## CLI cancellation occurred after config access

Finding: doctor, auth status, and config show loaded config before observing an
already-canceled or expired context; doctor also returned a raw formatted
cancellation error.

Fix: each RunE validates arity, maps a finished context to the typed status
error before `LoadConfig`, and doctor no longer bypasses the inspector.

Regression: canceled/deadline contexts are crossed with an invalid canary
config path for all three commands, proving the context machine error wins and
no config error/path is retained.

# Adversarial Review

Fresh-context reviewer: Ampere (`gpt-5.6-terra`, xhigh, `019f5039-1305-7ab2-88aa-3a885c2a5c63`)

Initial fresh-context finding review: Ptolemy (`gpt-5.6-luna`, xhigh,
`019f4ff7-ef12-7e60-8cd8-76179482bf58`)

Process baseline: `e23389a` (`AGENTS.md` and adversarial-review docs)

Review scope: `e23389a..83f38a5`

Both reviewers were read-only and did not implement the change. Findings were
verified and fixed by the builder, then the final reviewer independently
inspected the final head and rechecked the last cancellation blocker.

## Blocking Findings

The initial reviews found the mutating-catalog advertisement, status-control,
raw-error/unknown-operation, and invalid-proxy-classification defects described
above. Final review found that CLI status commands still observed cancellation
after config loading and that doctor bypassed structured cancellation. Re-review
of `83f38a5` confirmed all blockers resolved and no remaining blocking finding.

## Non-Blocking Risks

An earlier review noted that direct users of the lower-level
`NewStatusInspector` choose an operation only after construction; the
before-load unknown-operation guarantee therefore belongs to
`Engine.InspectStatus`. It also noted a theoretical custom `SecretSource` whose
dynamic kind is `reflect.UnsafePointer`; canonical providers do not use that
shape. Neither affected the final verdict.

## Machine Contract Review

The supported `machine.v1` manifest/schema, introspection schemas, status JSON
keys, CLI human output structure, error-kind/exit taxonomy, and generated
artifacts remain unchanged. Finished CLI status contexts now consistently emit
the already-documented `canceled` or `deadline_exceeded` classification with
the selected operation.

## Safety Review

The final review confirmed tenant-read-only catalog gating, recursive catalog
copies, status JSON closure, provider/SDK/process/network absence, value
redaction and terminal-control normalization, static error boundaries, safe
sentinel identity, unknown-operation ordering, invalid-proxy classification,
and pre-config cancellation handling.

## Generated Artifact Review

No frozen or generated artifact changed. CLI-doc, machine-contract, PTY escape,
surface, schema, release, and skill synchronization checks passed.

Verdict: approve
