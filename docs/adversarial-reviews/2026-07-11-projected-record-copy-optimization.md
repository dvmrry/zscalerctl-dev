# Builder Handoff

## Intent

Reduce CPU time and allocation volume in projected-record JSON rendering,
filter/search, single-record verification, and count-only consumers without
weakening projection, redaction, output safety, mutation isolation, or machine
contracts.

Keep conservative copying for collections reconstructed through a public or
otherwise unproven boundary. Preserve the trusted immutable collection across
the in-process machine event path so the measured JSON optimization applies to
the normal CLI read flow rather than only to a synthetic constructor path.

## Base / Head

Base commit: `2eb03535057af3a4334458bd7440bdf9f1112a70`

Head branch: `feature/engine-performance`

Reviewed implementation head: `bc65c2f`

Review scope: `2eb03535057af3a4334458bd7440bdf9f1112a70..bc65c2f`

## Files Changed

Projected-record ownership, encoding, filtering, verification, tests, fuzzing,
and benchmarks:

- `internal/resources/resources.go`
- `internal/resources/resources_test.go`
- `internal/resources/project_verify_internal_test.go`
- `internal/resources/resources_fuzz_test.go`
- `internal/resources/resources_bench_test.go`

Typed machine-result preservation, event invariants, tests, and actual-path
benchmark:

- `internal/machine/engine_read.go`
- `internal/machine/engine_read_internal_test.go`
- `internal/machine/events.go`
- `internal/machine/events_internal_test.go`
- `internal/machine/executor.go`
- `internal/cli/machine_read_bench_test.go`

Count-only consumers:

- `internal/cli/render_records.go`
- `internal/dump/dump.go`

Review artifact:

- `docs/adversarial-reviews/2026-07-11-projected-record-copy-optimization.md`

## Source Inputs Consulted

- `AGENTS.md` and the adversarial-review workflow, handoff, run prompt, and
  report templates from `origin/main` at `b0597df`
- `ProjectedRecord`, `ProjectedRecords`, source/projection constructors,
  copy/quarantine helpers, final subset verification, filters, and renderers
- `ResourceReadResult`, `Executor.Read`, `Executor.ExecuteStream`, event
  lifecycle validation, event sink copying, runtime adapters, and CLI final
  verification
- dump payload construction and pretty-render allocation/count paths
- Go 1.26.5 `encoding/json` map, custom-marshaler, error-wrapping, empty-slice,
  `json.Number`, and unsupported-value behavior
- the previously approved engine-performance review and benchmark conventions

## Generated Artifacts

None. No generated CLI document, JSON Schema, manifest fixture, machine.v1
fixture, golden, field-coverage artifact, release artifact, or generated agent
skill changed.

## Expected Delta

- `ProjectedRecords` tracks an unexported collection-level isolation invariant.
  Copying/projection constructors and field selection establish it; zero values
  and `NewProjectedRecords` remain conservative.
- Isolated collection JSON builds a shallow `[]map[string]any` encoder view and
  reads private maps without recursively copying them. Unisolated collections
  retain the historical `Fields()` copy/quarantine path.
- Empty and zero-value collections still encode as `[]`.
- Filtering reads private fields only for isolated collections. Unisolated
  collections retain copy/quarantine semantics, and filtered output preserves
  the input isolation state.
- `ProjectRecordAndVerify` validates the just-created package-private projected
  map without a redundant defensive copy.
- `ProjectedRecords.Len` exposes only count; it does not expose a slice or map.
- Resource completion events carry an unexported `ResourceReadResult` pointer
  for in-package typed adapters while public event records and counters remain
  unchanged. The sink receives a copied immutable result wrapper.
- `Executor.Read` counts the same record events, verifies typed-result and
  completion counts, and returns the original immutable typed result instead of
  reconstructing it through the conservative public collection constructor.
- Typed completion requires matching record count, exactly one completed
  resource, and no warnings.

## Invariants Claimed

- Public constructors and accessors continue to copy caller-owned or returned
  slices, maps, and nested mutable values.
- A hand-built or otherwise unproven private record cannot invoke a custom JSON
  marshaler or custom stringer through collection JSON or filter/search.
- Unknown, secret, mode-disallowed, cyclic, NaN/Inf, invalid `json.Number`,
  method-bearing, and otherwise invalid projected values remain fail-closed.
- Supported JSON bytes, map-key ordering, empty encoding, and legacy invalid
  value error chains are unchanged.
- Filter/search matching, output order, and field narrowing are unchanged.
- Final CLI subset verification and final output redaction remain in place.
- Public event fields, event order, record events, completion counters, machine
  DTOs, error envelopes, exit codes, schemas, manifests, introspection, dump and
  diff shapes, generated docs, and goldens are unchanged.

## Tests Run

Builder verification, passing on the applicable reviewed tree unless noted:

- deterministic A/B benchmarks were added before each corresponding production
  change and run with the same code, fixture, machine, and 500 ms benchtime for
  ten samples per side
- focused projected JSON, unsupported-value, legacy-error-chain, filtering,
  mutation-isolation, event, typed-result, and completion-counter regressions
- `env -u GOFLAGS go test ./...`
- `env -u GOFLAGS go test -race -count=1 ./internal/machine ./internal/cli ./internal/resources ./internal/runtime ./internal/browser`
- `env -u GOFLAGS go test -race -count=1 ./internal/resources ./internal/dump ./internal/cli`
- `env -u GOFLAGS go vet ./...`
- gofmt and `git diff --check`
- builder differential fuzzing of direct projected JSON against the historical
  defensive-copy encoder: 773,723 executions across two runs
- fresh reviewer differential fuzzing: 398,331 executions across two runs
- `env -u GOFLAGS make check` before the review/fix loop, including normal and
  race tests, vet, vulnerability scans, staticcheck, Semgrep, Gitleaks,
  contracts, boundaries, release checks, and skill sync
- final approved-tree `env -u GOFLAGS make check` with this artifact present:
  exit 0 across the same complete gate set

## Benchmark Evidence

Apple M1, Go 1.26.5, deterministic 1,000-record fixtures, setup outside timed
regions, retained results, ten samples per side, median values:

- Projected collection indented JSON: 3,695,109 to 2,501,058.5 ns/op;
  2,264,607 to 1,244,478.5 B/op; 33,023 to 16,016 allocations/op. This is
  approximately 32.3% faster with 45.0% fewer allocated bytes and 51.5% fewer
  allocations.
- Filter plus search: 1,305,840 to 177,478 ns/op; 978,762 to 33,251 B/op;
  18,321 to 1,339.5 allocations/op. This is approximately 86.4% faster with
  96.6% fewer allocated bytes and 92.7% fewer allocations.
- Single-record project plus verify: 12,020 to 10,816.5 ns/op; 6,688 to
  5,752 B/op; 94 to 77 allocations/op. This is approximately 10.0% faster with
  14.0% fewer allocated bytes and 18.1% fewer allocations.
- Actual typed CLI JSON core path (`Executor.Read` -> final CLI verification ->
  `json.MarshalIndent`): 5,562,150 to 4,039,526.5 ns/op; 2,504,558 to
  1,502,436 B/op; 48,068 to 31,055 allocations/op. This is approximately 27.4%
  faster with 40.0% fewer allocated bytes and 35.4% fewer allocations.

Benchmarks are machine-specific and are intended for relative regression use.

## Known Deferrals

- Reflection/type-switch optimization in projected-value verification remains a
  separate, fuzz-heavy change.
- Selected-diff batch admission still performs redundant projection/copy work.
- Redactor clean-buffer aliasing, unsafe byte/string conversion, streaming
  around whole-document redaction, and global validator caches remain out of
  scope.
- The actual-path benchmark intentionally ends at indented JSON encoding; it
  does not include final redaction scanning or destination I/O, which are
  unchanged by this patch.

## Review Focus

- every constructor, copy, selection, filtering, event, and result path that can
  establish, preserve, lose, or falsely claim isolated private state
- malformed same-package construction, custom method invocation, cyclic and
  unsupported values, exact JSON bytes, and error-chain parity
- filter/search mutation and alias behavior for isolated and conservative paths
- event payload visibility, sink wrapper copying, record order/count parity,
  typed completion validation, and manifest/nonterminal rejection
- whether the actual CLI read path reaches the measured fast path
- benchmark setup/timer boundaries, retained results, and comparative validity
- generated and machine-contract drift

# Finding Resolution

## Malformed private state bypassed the historical quarantine

Finding: Direct collection JSON could invoke a custom marshaler, and direct
filter/search could invoke a custom stringer, when a same-package test or future
internal bug hand-built a `ProjectedRecord` with malformed private state.

Root cause: The first fast path assumed all `ProjectedRecords` values came from
copying/projection constructors, but the zero value and conservative public
`NewProjectedRecords` path did not prove that invariant.

Fix: `ProjectedRecords` now carries an unexported collection-level isolation
marker. Proven constructors establish it. Unproven collections use the old
`Fields()` copy/quarantine path for JSON and filtering. Isolated JSON encodes a
shallow `[]map[string]any` view, preserving the legacy container and error chain
without recursively copying nested values.

Regression tests: The custom canary JSON and search tests were added first and
failed on `d271287`, then passed on `c1224ee`. A separate test compares the exact
fast-path invalid-value error string with the historical encoder.

Verification: Focused normal/race tests, exact-error regression, differential
fuzzing, vet, formatting, and benchmarks passed. The original reviewer rechecked
and considered these findings resolved.

## Normal machine reads lost the isolation marker

Finding: `Executor.Read` rebuilt event records with `NewProjectedRecords`, which
correctly marked them conservative. Ordinary CLI list JSON therefore used the
legacy copy path, while the original resource benchmark measured only a
preconstructed isolated collection.

Root cause: The shared event path emitted typed records individually but did not
preserve the original immutable `ResourceReadResult` for the typed in-process
adapter.

Fix: Resource completion carries the original typed result in an unexported
event field. The event sink copies the result wrapper. `Executor.Read` still
consumes/counts public record events, verifies all counts, and returns the typed
result without public reconstruction.

Regression tests and benchmark: Internal event tests pin typed-result presence
and wrapper copying. Existing routing, narrowing, empty, error, defensive, event,
runtime, and browser tests pass. `BenchmarkMachineReadJSONPath`, added and run
before the production event change, directly measures `Executor.Read` through
final CLI verification and JSON encoding and demonstrates the real-path delta.

Verification: Full Go tests, targeted race tests, vet, formatting, and the
actual-path A/B passed. The reviewer reproduced the source path, ran focused and
race tests plus a bounded benchmark probe, and marked the blocker resolved.

## Typed completion consistency was enforced only by the consumer

Finding: `Executor.Read` rejected typed-result/count mismatch, but
`EventStream.Complete` itself did not reject an inconsistent in-package typed
completion.

Root cause: Producer-side lifecycle validation had been extended for payload
placement but not for typed result length, resource count, or warning count.

Fix: `EventStream.Complete` now requires typed-result length to equal
`event.Records`, `Resources` to equal one, and `Warnings` to equal zero before
delivery.

Regression test: `TestEventStreamRejectsTypedResultWithInconsistentCounters`
pins producer rejection and the internal diagnostic.

Verification: Focused machine/CLI tests, machine race, vet, formatting, and
diff checks passed. The reviewer performed a final focused recheck and approved.

# Adversarial Review

Fresh-context reviewer: Tesla (`gpt-5.6-luna`, max,
`019f522b-a5e2-7ab3-ac5e-f37838a79dd9`)

Process baseline: `origin/main` at
`b0597dfb8e673a06d99995e6e1360cfcc709f0a8`

Review scope: `2eb03535057af3a4334458bd7440bdf9f1112a70..bc65c2f`

The reviewer was fresh-context, read-only, and did not implement fixes. Its
initial review approved with nits but identified malformed-state and exact-error
gaps. After those were reproduced and fixed, the reviewer found that the primary
machine read path lost the isolation marker and requested changes. After the
typed event/result fix, it approved with a producer-side consistency nit. The
final focused fix and regression test were rechecked and approved.

## Blocking Findings

The machine-path benchmark/marker finding was the blocking finding. It is mapped
through root cause, fix, regression benchmark/tests, and verification above.
There are no unresolved blockers.

## Non-Blocking Risks

The malformed-state/error-chain and producer-side count-validation nits were
resolved during the review loop. No unresolved non-blocking finding remains.

## Machine Contract Review

Supported JSON bytes and errors, empty collections, filtering order, public
event fields/order/counters, NDJSON, dump counts, error envelopes, exits,
schemas, manifests, introspection, and CLI surfaces remain unchanged. The typed
event payload is unexported and `Event` remains explicitly non-serializable.

## Safety Review

Public mutable boundaries still copy. Direct private reads require a proven
collection-level isolation invariant; otherwise the historical quarantine path
is used. The machine event sink copies its private result wrapper, the projected
collection remains immutable through public APIs, completion counts are checked
by producer and consumer, final CLI verification remains, and final redaction is
untouched.

## Generated Artifact Review

No generated artifact changed. Repository docs, schema, machine, surface,
boundary, release, and skill-sync gates are covered by `make check`.

## Verdict

`approve`
