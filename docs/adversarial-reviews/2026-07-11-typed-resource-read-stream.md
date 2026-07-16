# Builder Handoff

## Intent

Expose catalog resource events through typed `ResourceReadRequest` methods on
`machine.Executor`, `runtime.Machine`, and `runtime.Engine`. A future transport
adapter no longer needs to construct the legacy generic request envelope for
streamed reads. Existing one-shot, compatibility, CLI, and machine behavior
remain unchanged.

## Base / Head

- Base: `eca6959`
- Head: working tree on `feature/stdio-engine-api`
- Process baseline: `origin/main` at `b0597df`

## Files Changed

- `internal/machine/engine_read.go`
- `internal/machine/engine_read_test.go`
- `internal/machine/executor.go`
- `internal/machine/engine_manifest.go`
- `internal/runtime/runtime.go`
- `internal/runtime/engine.go`
- `internal/runtime/runtime_test.go`
- `docs/ENGINE_API_DESIGN.md`
- `docs/ENGINE_CAPABILITY_MODEL.md`
- this review artifact

## Source Inputs Consulted

- accepted engine, capability, and event design documents
- existing executor event lifecycle and machine contract tests
- runtime engine/machine construction paths
- CLI resource adapter and core-boundary verifier

## Generated Artifacts

None.

## Expected Delta

- New candidate in-process `ReadStream` methods at executor, machine, and
  engine layers.
- One exported internal-package predicate for the closed list/get/show family.
- No supported CLI, JSON/NDJSON, machine manifest, schema, error envelope,
  exit code, dump/diff artifact, golden, or dependency change.

## Invariants Claimed

- Typed reads cannot execute manifest or another capability.
- Invalid operations reject config-free before provider or reader effects.
- One-shot `Read` reconstructs its result from the same typed stream path.
- Existing event ordering, terminal, cancellation, sink, projection,
  redaction, and narrowing behavior remains shared with `ExecuteStream`.
- Caller input slices are copied before the first sink callback.

## Tests Run

- Focused typed stream and runtime-preflight tests: pass.
- `go test ./internal/machine ./internal/runtime -count=1`: pass.
- `go test -race ./internal/machine ./internal/runtime -count=1`: pass.
- `go test ./... -count=1`: pass.
- `go vet ./...`: pass.
- machine-contract, core-boundary, docs, formatting, and diff checks: pass.

## Known Deferrals

- Resource reads remain atomic: upstream pagination/projection completes before
  record events are emitted.
- Runtime-construction failures happen before the in-process engine stream;
  a future wire coordinator owns its separate complete wire lifecycle.
- No wire DTO or serialization surface is introduced.

## Review Focus

- Non-read routing and pre-effect validation.
- One-shot parity and private result reconstruction.
- Input aliasing, cancellation, deadline, sink error/panic, empty result, and
  terminal-event behavior.
- Runtime construction/nil behavior and supported-surface compatibility.

## Finding Resolution

Finding: `Engine.ReadStream` constructed a live runtime before rejecting a
non-read operation.

Root cause: The canonical read-operation allow-list was private to the machine
package, so the engine reached it only after `NewMachine`.

Fix: Export the closed predicate as `machine.IsResourceReadOperation`. Both
typed engine methods route invalid operations through the config-free executor
rejection path. The unnecessary construction-error wrapper was also removed so
stream and one-shot methods preserve exact error parity.

Regression test: Engine typed calls with `manifest` assert zero config/reader
effects and `unsupported_operation`; construction-error parity is pinned;
typed cancellation, deadline, sink-error, and sink-panic behavior is exercised
directly.

Verification: Focused, race, full-repository, vet, contract, boundary, docs,
formatting, and diff checks all pass.

# Adversarial Review

Fresh-context reviewer: Herschel (`gpt-5.6-terra`, high,
`019f5465-9a5b-75a1-a681-00adbfe46dc7`)

The reviewer confirmed base `eca6959`, the current working tree, and process
baseline `origin/main` at `b0597df`. No files were edited.

## Blocking Findings

The initial review found one blocker: invalid typed operations reached live
runtime construction before the machine allow-list rejected them. The focused
re-review confirmed the finding is resolved. No blocking findings remain.

## Non-Blocking Risks

The reviewer noted that the temporary handoff omitted `executor.go` and
`engine_manifest.go` from its file list. The list was corrected before this
artifact was written; no code change was required.

## Machine Contract Review

- Both typed engine methods now preflight through the shared operation
  predicate and config-free rejection path.
- Stream rejection emits `started` followed by
  `failed(unsupported_operation)`; one-shot returns the same typed error.
- Construction failures preserve the original `NewMachine` error.
- No supported machine or CLI surface changed.

## Safety Review

Invalid typed operations cause no config load, provider resolution, or reader
construction. Supported reads retain the existing projected/redacted event
path. Direct typed tests cover cancellation, deadline, sink errors, and sink
panics with one value-free terminal outcome.

## Generated Artifact Review

No generated artifact changed or requires regeneration. The candidate design
documents match the implementation and do not promote a public surface.

## Verdict

Verdict: approve with nits
