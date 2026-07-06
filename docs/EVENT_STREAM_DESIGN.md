# Operation Event Stream — Design Checkpoint (Roadmap Phase 4.1)

Status: DRAFT for owner review. This is the design checkpoint required by
[ROADMAP.md](ROADMAP.md) Phase 4 before any event-stream code is written. It
answers the questions machine-contract.md's "Streaming And Progress Direction"
section deliberately left open. Items marked **DECISION** need an owner call;
everything else is a recommendation with rationale.

## Goals and non-goals

Goals: one internal event model that serves (a) dump progress, (b) future GUI
progress and incremental record display, and (c) future MCP progress
notifications — so no frontend ever invents its own progress semantics.

Non-goals (unchanged from machine-contract.md): no transport choice, no
supported JSON event schema, no CLI streaming command, no change to one-shot
`Execute`, JSON/NDJSON output, envelopes, exit codes, or dump file formats.

## Shape: synchronous callback, not channels or iterators

```go
// package machine (safe seam) — types only, no emission logic

type EventKind string

const (
    EventStarted   EventKind = "started"
    EventProgress  EventKind = "progress"
    EventRecord    EventKind = "record"
    EventWarning   EventKind = "warning"
    EventCompleted EventKind = "completed"
    EventFailed    EventKind = "failed"
    EventCanceled  EventKind = "canceled"
)

type Event struct {
    Kind     EventKind
    Product  string
    Resource string
    // Done/Total for progress; value-free counters only.
    Done, Total int
    // Record is set only on EventRecord: one already-projected,
    // already-redacted, already-verified record.
    Record *resources.ProjectedRecord
    // Err is set only on warning/failed/canceled: a MachineError
    // (sanitized kind + value-free message), never a raw SDK error.
    Err *MachineError
    // Counts on completed: records, resources, warnings.
    Records, Resources, Warnings int
}

type EventSink func(Event) error
```

The producer calls the sink synchronously on its own goroutine.

Why callback over channel/iterator:

- **Backpressure is inherent.** The producer blocks until the consumer
  returns. In the Wails case the blocked goroutine is the backend Go
  goroutine servicing the call — never the JS/UI thread — and that backend
  can drain into its own channel and emit to the frontend asynchronously. No buffering policy, no dropped-event policy, no goroutine
  lifecycle to manage or leak. This matches the project's "explicit,
  inspectable behavior" principle and the sequential-collection pacing policy.
- **Ordering is trivial.** Single producer goroutine, synchronous calls:
  events arrive in program order. No reordering is possible by construction.
- **Cancellation composes.** A sink returning a non-nil error stops the
  operation (producer wraps it and aborts); context cancellation is still
  honored between SDK calls exactly as today. Two stop paths, both explicit.
- Precedent: `runtime.DumpProgressFunc` already works this way; this design
  generalizes it rather than introducing a second model.

An `iter.Seq`-based adapter can be layered on later without changing the
producer; do not start there.

## Required answers

1. **Ordering guarantees.** Exactly one `started` first; zero or more
   `progress`/`record`/`warning` in program order; exactly one terminal event
   (`completed`, `failed`, or `canceled`) last. After a terminal event the
   sink is never called again. The producer's emission wrapper recovers a
   panicking sink and converts it to a terminal `failed` with
   `Kind: internal` — without this, a sink panic would unwind the producer
   and the terminal-exactly-once property would be silently false
   (adversarial-review finding, 2026-07-06). Records for a given resource are emitted in
   the same order the projected reader returns them (catalog order across
   resources, matching today's dump collection order).
2. **Cancellation semantics.** `ctx.Done()` between operations → terminal
   `canceled` with `MachineError{Kind: canceled}`. Sink error → terminal
   `failed` wrapping the sink's error value-free (kind `internal` unless the
   sink returned a `MachineError`). Sink panic → recovered, terminal
   `failed{internal}` (see 1); the panic value is never placed in the event. The CLI exit-code mapping is unchanged
   (canceled → 1), because the CLI consumes the same terminal kinds the
   one-shot path produces today.
3. **Deadline behavior.** Per-request deadlines surface exactly as post-#91
   semantics: terminal `failed` with `Kind: deadline_exceeded` (CLI exit 5).
   No new timeout mechanism; the SDK adapter's per-request timeout stands.
4. **Backpressure.** Synchronous sink = producer speed is consumer speed.
   Deliberately no internal buffering. A slow GUI must do its own buffering
   on its side of the seam; the core stays allocation-light and honest.
5. **Partial-error semantics.** `warning` events carry the same value-free
   fields as dump's `errors.ndjson` records (product, resource, operation,
   kind — never message payloads from the backend). `completed.Warnings`
   counts them. Continue-on-error remains an option on the operation, not a
   property of the stream.
6. **Redaction boundary.** `record` events carry `resources.ProjectedRecord`
   only — the same post-projection, post-verification type machine responses
   are built from. `started`/`progress`/`completed` are value-free by type
   construction (ints and catalog names only). There is no code path from a
   source record to an event.
7. **Schema status.** Candidate, in-process only. Event types live in
   `internal/machine` (safe seam; types + kinds), emission lives in
   `internal/runtime` (trusted). There is no committed wire form for events
   in v1. Note: omitting JSON tags is NOT a guard — encoding/json marshals
   exported untagged fields by name — so the mechanical guard is a unit test
   in `internal/machine` asserting no production code path marshals `Event`
   (grep-style check over non-test sources, same pattern as
   verify-machine-contract.sh's constructor scan). JSON serialization is
   added only by the future MCP-notification or supported-streaming
   promotion, with schemas and fixtures at that time.
8. **One-shot reconstruction.** `Executor.Execute` becomes: run the
   event-producing path with an accumulating sink; build `machine.Response`
   from accumulated records; map terminal `failed`/`canceled` to the
   existing `MachineError` returns. Equivalence proof: the existing machine
   contract golden fixtures (all nine error kinds + list/get/show/manifest)
   must pass unchanged against the reconstructed path — the fixtures ARE the
   equivalence test. `verify-machine-contract.sh` stays the gate.

## Dump integration

`DumpCollector.Collect` becomes an event producer; `DumpProgressFunc` is
reimplemented as a thin sink adapter (kept for one release, deprecated, then
removed — it is a candidate seam, so removal is `semver:minor` at most).
Dump file writing consumes `record` events per resource instead of a fully
accumulated slice **only if** the write path can stream marshaling; otherwise
buffering stays as-is and the memory-baseline work is a separate follow-up.
Note (adversarial-review finding): `TestLargeTenantDumpBaseline` calls the
projection and dump-write layers directly and never crosses
`DumpCollector.Collect` or `Executor.Execute`, so it is structurally blind
to this refactor. The dump-migration PR must add a peak-heap baseline over
the reconstructed collect path (an accumulating sink must not create a new
full-copy generation of records) in addition to keeping the existing gate.

## Test plan

- Unit: ordering property tests (terminal-exactly-once, no-events-after-
  terminal — including under a panicking sink), cancellation from both
  paths, deadline mapping, warning accounting, sink-error abort.
- Contract: existing golden fixtures over reconstructed Execute (no new
  fixtures — that is the point).
- Dump: progress-event sequence test replacing the DumpProgressFunc test;
  memory baseline unchanged.

## DECISION points for the owner

- **D1 — record batching.** `Record *ProjectedRecord` (one per event,
  recommended: simplest, backpressure-friendly) vs a batch slice per event.
  Recommendation: single record; batching is a consumer-side concern.
- **D2 — DumpProgressFunc deprecation window.** Remove immediately in the
  same PR (nothing external consumes candidate seams) vs keep one release.
  Recommendation: remove immediately; the CLI is the only consumer.
- **D3 — where warning events live for single-resource reads.** list/get/show
  currently fail atomically (no partials). Keep it that way (recommended) or
  allow warnings for pagination-level soft failures. Recommendation: atomic;
  warnings are a dump/multi-resource concept until a concrete need appears.

## Semver and sequencing

Types + executor reconstruction + runtime emission + dump migration can be
one PR series (3 small PRs: machine types+reconstruction, runtime emission,
dump migration), each `semver:minor` (candidate seam), each gated by the
existing contract and boundary checks. No supported surface changes in any of
them.
