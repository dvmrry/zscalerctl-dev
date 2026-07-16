# Builder Handoff

## Intent

Establish the first candidate local-engine slice: one synchronous in-process
operation event stream shared by `machine.Executor` and the trusted runtime.
Existing one-shot `Execute` is reconstructed from events with no supported
CLI or machine-output drift. Record the accepted local-engine architecture for
future typed operations and a local stdio protocol, without promoting a public
Go API or network service.

## Base / Head

Base commit: `f57c577`

Reviewed implementation head: `ed1535f`

The local integration base contains the pending reviewed PR stacks #106, #108,
#109, and #110. Review scope was limited to `f57c577..ed1535f`.

## Files Changed

Architecture and contract documentation:

- `docs/ENGINE_API_DESIGN.md`
- `docs/EVENT_STREAM_DESIGN.md`
- `docs/ROADMAP.md`
- `docs/DEV_PUBLIC_SURFACE_MODEL.md`
- `docs/cli/machine-contract.md`

Code and tests:

- `internal/machine/events.go`
- `internal/machine/events_test.go`
- `internal/machine/events_internal_test.go`
- `internal/machine/executor.go`
- `internal/machine/types.go`
- `internal/runtime/runtime.go`
- `internal/runtime/runtime_test.go`
- `internal/resources/resources.go`
- `internal/resources/resources_test.go`

## Source Inputs Consulted

- adversarial-review process and templates from `origin/main`
- the event-stream design from `origin/docs/event-stream-design`
- the MCP threat model from `origin/docs/mcp-threat-model`
- existing machine contract, public-surface model, architecture, and roadmap
- machine request/response/error fixtures and manifest schema gate
- runtime machine and dump assembly
- `resources.ProjectedRecord` and `ProjectedRecords` construction/copy behavior
- the one-shot stdio machine-adapter experiment

## Generated Artifacts

None. No schema, machine fixture, CLI golden, generated CLI documentation,
field-coverage artifact, or generated skill changed.

## Expected Delta

- Add candidate-only `machine.Event`, `EventSink`, and
  `Executor.ExecuteStream`.
- Add candidate-only `runtime.Machine.ExecuteStream` forwarding.
- Reconstruct the existing `machine.Response` from stream events.
- Carry the config-free manifest on successful manifest completion so manifest
  responses are reconstructed too.
- Reject direct JSON marshal and unmarshal of the in-process event type.
- Preserve `machine.v1`, all request/response/error fixtures, CLI goldens,
  error envelopes, exit codes, dump/diff formats, and schemas byte-for-byte.
- Add no goroutine, channel, transport, network listener, or UI dependency.

## Invariants Claimed

- A non-nil sink receives exactly one `started` callback first and exactly one
  attempted terminal callback last.
- No callback occurs after terminal.
- Sink errors and panics are converted to machine-safe failures; panic and raw
  error values do not cross the seam.
- Terminal delivery failure is not retried.
- Cancellation maps to `canceled`; deadlines map to
  `failed/deadline_exceeded`.
- Records cross only as projected, redacted, verified
  `resources.ProjectedRecord` values.
- Manifest completion contains config-free catalog metadata only.
- One-shot empty records and empty manifest capabilities preserve their
  previous non-nil empty-array shapes.
- The private invalid-resource-ID sentinel cause survives reconstruction.

## Tests Run

All passed:

- targeted finding regressions with `-count=20`
- `go test ./internal/machine ./internal/runtime ./internal/resources`
- `go test -race ./internal/machine ./internal/runtime ./internal/resources`
- `go vet ./internal/machine ./internal/runtime ./internal/resources`
- `make verify-machine-contract`
- `make docs-check`
- Go error-flow and pre-review formatting/vet checks
- `env -u GOFLAGS make check` at `ed1535f` (exit 0)

## Known Deferrals

- dump event migration and `DumpProgressFunc` removal
- warning events for multi-resource operations
- true page/record-at-a-time SDK streaming
- capability-specific typed inputs/results and engine capability manifest
- stdio DTOs, codecs, lifecycle, and schemas
- TypeScript/Rust clients and frontend/MCP/Wails experiments
- public Go API or protocol promotion

## Review Focus

- one-shot response equivalence for records, manifest, metadata, and every
  machine-error path
- terminal state under sink errors, typed-nil errors, and panics
- cancellation/deadline classification
- error, panic, source-record, and tenant-value leakage
- retained-event pointer/slice/map isolation
- direct JSON event serialization
- candidate-versus-supported documentation and frozen generated surfaces

# Finding Resolution

## Typed-nil sink error

Finding: A sink returning a typed-nil `*MachineError` could fall through a
second `errors.As` traversal and panic after `started` without terminal
delivery.

Root cause: the successful pointer match was combined with a non-nil check, so
a nil matched pointer was treated as no match.

Fix: any successful `*MachineError` match now returns immediately; typed nil is
converted to the static `internal/event sink failed` error.

Regression test: `TestExecutorExecuteStreamTypedNilMachineErrorEmitsFailure`
asserts `[started, failed]`, no panic, and a sanitized internal error.

Verification: repeated targeted test, focused race/vet, and full `make check`
passed.

## Empty manifest array identity

Finding: copying a non-nil empty capability slice with append-to-nil changed
JSON from `[]` to `null`.

Root cause: the boundary copy did not preserve nil-versus-empty slice identity.

Fix: manifest and machine-error slices use `slices.Clone`.

Regression test: `TestExecutorExecuteManifestPreservesEmptyCapabilities`
compares reconstructed output with `ManifestFromCatalog` and asserts
`"capabilities":[]`.

Verification: repeated targeted test, machine-contract gate, and full
`make check` passed.

## Retained typed-slice isolation

Finding: the pre-existing projected-value copier handled `[]string` but not the
real `[]int` port fields, weakening retained-event isolation.

Root cause: `copyAny` returned typed integer slices by reference.

Fix: `[]int` and `[]string` use `slices.Clone`.

Regression tests: the resource reconstruction test mutates input/output port
slices, and `TestExecutorExecuteStreamRecordFieldsAreDefensiveCopies` mutates a
retained event view and verifies the original remains unchanged.

Verification: repeated targeted tests, resource race test, and full
`make check` passed.

# Adversarial Review

Fresh-context reviewer: McClintock (`gpt-5.6-sol`, ultra, `019f4ec2-6b61-78f2-8752-a93a4dc6ca28`) and Dirac (`gpt-5.6-luna`, max, `019f4ec2-6d8c-7681-8f1f-b1f084e673b9`)

Process baseline: `origin/main`

Review scope: `f57c577..ed1535f`

Both reviewers were read-only and did not share the builder implementation
context. McClintock owned event/error state semantics and one-shot equivalence;
Dirac owned supported contracts, security boundaries, generated artifacts, and
documentation accuracy.

## Blocking Findings

The initial code-semantics review found the typed-nil sink panic and empty
manifest array regression documented above. Both were fixed with regression
tests. Focused re-review found no remaining blocking findings.

## Non-Blocking Risks

The initial reviews identified typed `[]int` aliasing, caller-controlled
pre-validation resource selectors, caller-goroutine wording, source-boundary
wording, terminal sink-error coverage, and direct JSON unmarshal hardening.
The final delta addressed each item. Focused re-review found no remaining risk
requiring a change in this slice.

## Machine Contract Review

The supported CLI continues to call one-shot `Machine.Execute`. No CLI command,
JSON/NDJSON output, stderr envelope, exit code, schema, fixture, or
`machine.v1` artifact changed. The machine-contract gate passed after the
fixes. Empty resource and manifest array identity is explicitly covered.

## Safety Review

Events receive projected records from the catalog-backed browser service; no
source-record type, unprojected map, credential, SDK client, raw backend error,
or panic value crosses the seam. Manifest completion is config-free catalog
metadata. Direct event JSON marshal and unmarshal fail closed. Sink callbacks
remain synchronous and the core starts no goroutines.

## Generated Artifact Review

No generated artifact changed and no regeneration was required. Frozen CLI,
schema, and machine-fixture paths have zero diff in the review scope.

## Final Re-review

McClintock independently reran the five focused machine regressions and the
resource-copy regression and found no remaining issue. Dirac independently
reran focused race/vet checks, verified the documentation resolutions and
generated-surface scope, and found no remaining issue. Both approved the final
delta without nits.

Verdict: approve
