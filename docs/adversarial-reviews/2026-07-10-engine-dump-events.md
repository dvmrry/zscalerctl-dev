# Builder Handoff

## Intent

Migrate live dump collection from the candidate `DumpProgressFunc` callback to
the shared synchronous operation-event lifecycle. Keep the existing buffered
`dump.Result` and every supported CLI, error, and on-disk artifact contract
unchanged while making dump progress, projected records, partial warnings, and
terminal state available to future in-process adapters.

## Base / Head

Base commit: `4a8e397`

Initial implementation head: `9521200`

Reviewed fix head: `cc4311d`

Review scope: `4a8e397..cc4311d`. The review-artifact commit also corrects one
reviewer-identified roadmap reference from the removed callback to the shared
event stream; it does not change runtime behavior.

## Files Changed

Design and roadmap documentation:

- `docs/EVENT_STREAM_DESIGN.md`
- `docs/ROADMAP.md`

Code and tests:

- `internal/machine/events.go`
- `internal/machine/events_test.go`
- `internal/machine/events_internal_test.go`
- `internal/machine/executor.go`
- `internal/runtime/dump.go`
- `internal/runtime/dump_memory_test.go`
- `internal/runtime/runtime_test.go`
- `internal/cli/dump_diff.go`
- `internal/cli/dump_progress_test.go`

## Source Inputs Consulted

- `AGENTS.md` and the adversarial-review workflow/templates
- `docs/EVENT_STREAM_DESIGN.md`, `docs/ENGINE_API_DESIGN.md`, and
  `docs/ROADMAP.md`
- machine event/executor tests and frozen machine-contract fixtures
- runtime dump collection, product-session reuse, and error propagation
- CLI spinner, info logging, partial-dump, force, cancellation, and dump
  artifact tests
- `dump.Result`, `ResourceDump`, `ResourceError`, manifest, and writer code
- `resources.ProjectedRecord` immutability and boundary-copy behavior
- the existing large-tenant projection/write memory baseline

## Generated Artifacts

None. No schema, machine fixture, CLI golden, generated CLI documentation,
field-coverage artifact, or generated skill changed.

## Expected Delta

- Add a reusable candidate `machine.EventStream` lifecycle controller and run
  `Executor.ExecuteStream` through it without changing one-shot reconstruction.
- Add `DumpCollector.CollectStream`; keep `Collect` as the one-shot adapter over
  the same event-producing path.
- Remove `DumpProgressFunc` and migrate the in-repo CLI consumer to progress
  events.
- Emit `started`, per-resource `progress`, projected `record`, value-free
  `warning`, and one `completed`/`failed`/`canceled` terminal event.
- Keep collection sequential, synchronous, and backpressured; start no
  goroutines in production code.
- Keep dump writing buffered and all supported output/file formats unchanged.

## Invariants Claimed

- Fatal dump returns preserve the original Go error identity. If terminal
  delivery also fails, the return joins the original error with a sanitized
  delivery error; sink error and panic values do not leak.
- Record events contain only post-projection, post-redaction, post-verification
  `ProjectedRecord` values.
- Warning metadata matches `errors.ndjson` product/resource/operation/kind and
  never includes a backend error value.
- `started.Total`/`progress.Total` count selected resources;
  `completed.Resources` counts successful resources (including zero-record
  success); `Records` counts record events; `Warnings` counts warnings.
- Existing resource order, progress timing, product-session reuse/cleanup,
  context behavior, partial-dump behavior, CLI diagnostics, and dump artifacts
  are unchanged.
- Direct event JSON remains forbidden and no supported machine or CLI contract
  is promoted or changed.
- Event delivery retains only a record wrapper, not another projected payload
  generation.

## Tests Run

All passed at the reviewed fix head:

- focused event/dump/CLI regressions with `-count=20`
- terminal sink-error and sink-panic reproducer with `-count=20`
- `go test ./...`
- `go test -race ./internal/machine ./internal/runtime ./internal/cli`
- full repository `go test -race -mod=vendor ./...`
- `go vet ./...`
- staticcheck, machine-contract, docs, CLI-doc, core/experiment-boundary,
  surface-manifest, secret, vulnerability, license, action-pin, release, script,
  and skill-sync gates
- accumulating-sink memory baseline repeatedly below the `4x` ceiling
- deterministic event-copy guard: one allocation and eight bytes per record
  wrapper with a 4,096-integer payload
- `env -u GOFLAGS make check` at `cc4311d` (exit 0)

## Known Deferrals

- true page/record-at-a-time SDK streaming and a streaming dump writer
- typed dump capability input/result promotion and a common engine manifest
- stdio/wire DTOs, codecs, schemas, cancellation frames, and reference clients
- frontend, MCP, Wails, Ink/OpenTUI, or Ratatui adapters
- public Go API or protocol promotion
- explicit precedence when both the operation and terminal-delivery errors are
  `MachineError` values; current SDK dump readers do not return that type
- external transport treatment of caller-supplied typed `MachineError` sink
  failures; events remain a trusted, in-process candidate seam
- a dedicated manifest-plus-completion-counter rejection case (the source
  rejects it; mixed-payload tests cover the same fail-closed path)

## Review Focus

- terminal-exactly-once behavior under sink error/panic
- original sentinel identity versus sanitized terminal delivery failure
- cancellation/deadline and partial-warning semantics
- projection/redaction boundary and retained-event aliasing/allocation
- completion counter meaning for list, show, zero-record, and partial cases
- product-session cleanup and CLI progress/log timing
- supported machine, CLI, schema, generated, and dump artifact drift

# Finding Resolution

## Terminal delivery replaced the original fatal error

Finding: when a sink failed or panicked while receiving a fatal terminal event,
`finishDumpStreamFailure` returned only the sanitized delivery failure. The
original reader/context error no longer satisfied `errors.Is`, contradicting
the stated dump contract.

Fix: return `errors.Join(resultErr, deliveryErr)`. The operation error remains
discoverable, the sanitized delivery `MachineError` remains discoverable, and
the raw sink error/panic value is absent.

Regression: both sink-error and sink-panic cases assert original sentinel
identity, sanitized delivery classification, one terminal attempt, no sink
value leakage, and exactly one product-session close. The fresh reviewer reran
the reproducer twenty times.

## Completion-count ambiguity

Finding: `completed.Resources` counted successful entries, but the design did
not distinguish selected, attempted, and successful resources.

Fix: document selected totals and successful completion counters in the event
type and design. Existing/new tests cover record lists, show resources,
zero-record resources, and partial warnings.

## Event payload exclusivity

Finding: lifecycle ordering was enforced, but a producer could attach record,
error, manifest, progress, or completion fields to the wrong event kind.

Fix: fail closed on mixed payloads and manifest completions carrying resource
counters. Table tests verify malformed progress, record, warning, and completed
events produce one sanitized failed terminal.

## Memory-baseline confidence

Finding: the real collect-path heap test sampled asynchronously, so one payload
clone might hide below the generous peak ceiling.

Fix: retain the real accumulating-sink peak test and add deterministic
allocation accounting at the exact event-copy boundary. A large record event
copies only its immutable wrapper (`1 alloc/op`, `8 bytes/op`), not its map or
typed-slice payload.

## Removed-callback roadmap wording

Finding: a future CLI styling item still named `DumpProgressFunc` after this
slice removed it.

Fix: point that roadmap item at the shared operation event stream.

# Adversarial Review

Fresh-context reviewer: Lorentz (`gpt-5.6-luna`, max, `019f4ef0-0581-7512-856c-a1cca150c36b`)

Process baseline: `4a8e397` (`AGENTS.md` and adversarial-review docs)

Review scope: `4a8e397..cc4311d`

The reviewer was read-only, started without the builder context, inspected the
tree/diff as evidence, and independently ran the relevant tests and gates.

## Blocking Findings

The initial review found the terminal-delivery/original-error loss documented
above and requested changes. Re-review of `cc4311d` independently reproduced
the repaired behavior twenty times and found no remaining blocker.

## Non-Blocking Risks

- If a future dump operation error is itself a `MachineError`, ordinary
  `errors.As` traversal over the joined error returns the first matching value;
  current SDK dump reader paths do not return this type.
- A trusted sink that deliberately returns a typed `MachineError` supplies its
  own value-free message; generic errors and panics are sanitized by the core.
- The real heap baseline remains sampling-based, supplemented by the exact
  deterministic allocation guard.
- Manifest-plus-counter rejection is source-enforced but lacks its own named
  test case.

None changes a supported contract or blocks this candidate in-process slice.

## Machine Contract Review

No JSON, NDJSON, error-envelope, exit-code, dump, diff, schema, manifest,
introspection, or generated CLI artifact changed. Direct event JSON still
fails closed. Machine-contract, CLI-doc, surface, and full repository gates
passed.

## Safety Review

Projection and verification precede every record event. Warning and terminal
events carry static messages and value-free metadata, never backend values.
The terminal-delivery regression additionally proves sink error/panic values do
not enter the joined return. The event-copy allocation guard prevents a hidden
retained payload generation.

## Generated Artifact Review

No generated artifact changed or required regeneration. Frozen schema,
machine-fixture, CLI-surface, generated-doc, field-coverage, and generated-skill
paths have zero intentional delta.

## Final Re-review

The reviewer independently passed the terminal-delivery reproducer, mixed
payload tests, allocation guard, CLI progress tests, memory baseline, full Go
tests, focused race, vet, staticcheck, machine-contract, docs, boundary, and
surface gates. The original blocker is resolved and no new blocker was found.

Verdict: approve with nits
