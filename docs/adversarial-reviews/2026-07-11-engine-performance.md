# Builder Handoff

## Intent

Measure and reduce two independently reported engine costs without weakening
the tenant-data or local-artifact safety model:

- redundant collection/map copies and repeated allowed-field compilation at
  the typed resource-result to final CLI verification boundary
- whole-resource retention while a narrowly scoped diff validates unselected
  resource bodies

Keep every supported CLI, diff-report, manifest, error, exit-code, schema,
machine.v1, and generated-artifact contract unchanged.

## Base / Head

Base commit: 22ecb7c7027171decdbfe254d6f425898ae9396f

Head branch: feature/engine-performance

Reviewed implementation head: 28d1a1e40852714e0dd3db21893dac49720efed8

Review scope: 22ecb7c7027171decdbfe254d6f425898ae9396f..28d1a1e40852714e0dd3db21893dac49720efed8

## Files Changed

Benchmarks and CLI compatibility regression:

- internal/cli/machine_read_bench_test.go
- internal/cli/app_test.go

Diff streaming validation, bounds/error handling, tests, fuzz oracle, and
benchmarks:

- internal/diff/diff.go
- internal/diff/diff_test.go
- internal/diff/diff_bench_test.go

Immutable typed result and compiled projected-field validator:

- internal/machine/engine_types.go
- internal/resources/resources.go

Review artifact:

- docs/adversarial-reviews/2026-07-11-engine-performance.md

## Source Inputs Consulted

- AGENTS.md and the adversarial-review workflow, handoff, run prompt, and
  report templates from origin/main at b0597df
- the accepted engine diff and projection review artifacts at the base commit
- ResourceReadResult, ProjectedRecords, ProjectedRecord, final CLI
  verification, rendering, and event-copy boundaries
- diff load/admission, os.Root, regular-file, max+1 size, context, runtime
  adapter, and Cobra compatibility paths
- Go 1.26.5 encoding/json Decoder, Token, More, and scanner behavior
- the independent Fable performance nits that motivated measurement, treated
  as hypotheses rather than evidence

## Generated Artifacts

None. No generated CLI document, schema, manifest fixture, machine.v1 fixture,
golden, field-coverage artifact, release artifact, or generated agent skill
changed.

## Expected Delta

- ResourceReadResult retains and returns ProjectedRecords by value. The type's
  private slice/map state remains accessible to callers only through
  defensive-copy accessors.
- Rendered-subset verification compiles the mode-filtered nested field tree
  once per batch and reads package-private immutable projected state directly.
- Selected diff resources retain the existing full bounded read, JSON decode,
  record-count check, catalog/mode admission, and idempotent projection path.
- Valid unselected resources are parsed one array record at a time, fully
  shape/count checked, and discarded without entering loadedDump.resources.
- Non-EOF reader failures remain sticky across json.Decoder.More and win over
  parser results. Context retains highest precedence.
- Malformed unselected bodies replay the historical bounded decoder only to
  preserve the existing local CLI diagnostic. Valid bodies never take that
  path.

## Invariants Claimed

- No caller can mutate ResourceReadResult state through its returned
  ProjectedRecords value.
- Unknown, nested unknown, mode-disallowed, unsupported, cyclic, NaN/Inf, and
  otherwise unrenderable projected values remain fail-closed.
- Unselected values cannot enter reports, results, events, or errors.
- Every unselected body remains completely parsed. Syntax errors win over
  later shape checks; reader errors win over syntax; context wins over both.
- Path locality, os.Root binding, regular-file checks, initial stat cap, max+1
  growth cap, and manifest record counts remain enforced.
- JSON acceptance/rejection matches the historical json.Unmarshal(any) path,
  including null/scalar rejection, nested-array record rejection, invalid
  UTF-8/surrogate handling, duplicate keys, trailing data, and float overflow.
- Existing invalid-dump adapter text, CLI output, report bytes, schemas,
  effects, and exit classifications are unchanged.

## Tests Run

Builder verification, all passing at the final reviewed tree unless noted:

- `env -u GOFLAGS go test ./internal/diff ./internal/resources ./internal/machine ./internal/cli`
- `env -u GOFLAGS go test -race -count=1 ./internal/diff ./internal/resources ./internal/machine ./internal/cli`
- focused structural-parity, cancellation, malformed-diagnostic, path,
  symlink, nonregular-file, oversized-file, count, and one-shot reader-error
  regressions
- repeated differential fuzzing of the streaming scanner against
  json.Unmarshal(any), including runs of 10, 10, 5, and 5 seconds and more
  than one million aggregate executions
- `env -u GOFLAGS go vet ./...`
- gofmt and `git diff --check`
- `env -u GOFLAGS make check` before the review/fix loop, including the full
  repository test/race, vulnerability, staticcheck, license, docs, Semgrep,
  Gitleaks, contract, surface, boundary, release, and skill-sync gates
- `env -u GOFLAGS make check` again on the final approved tree after adding
  this review artifact; exit 0 across the same complete gate set

Fresh-context reviewers independently ran focused tests, race tests, fuzzing,
vet, formatting checks, and benchmark probes.

## Benchmark Evidence

Apple M1, Go 1.26.5, deterministic fixtures, setup outside timed regions:

- Typed result/final verification, 1,000 records: median approximately 3.53 ms
  to 1.41 ms; 3.82 MB to 186 KB allocated; 34,021 to 13,027 allocations.
- Narrow diff, 64 selected plus 10,000 unselected records read from both sides:
  median approximately 41.6 ms to 39.7 ms; 34.0 MB to 25.6 MB allocated;
  allocation count effectively unchanged.
- Direct 100,000-record unselected parser: full decode median approximately
  174.6 ms and 162.2 MB allocated; streaming count approximately 181.0 ms and
  118.3 MB allocated.
- Final precompiled one-operation process measurement: full decode maximum RSS
  168,034,304 bytes; streaming count 44,564,480 bytes.

## Known Deferrals

- Selected resources still require full read/decode because their admitted
  values must be compared.
- One enormous top-level object, one enormous array record, or one enormous
  string can still delay cancellation within that value.
- Malformed unselected bodies intentionally pay the historical bounded decode
  cost to preserve supported local diagnostic text; the low-live-memory path
  applies to valid structurally parseable bodies.
- A concurrent file growth after initial stat is bounded by max+1 and covered
  by source logic, but no deterministic race hook was added solely to force
  that timing window.
- Benchmarks are machine-specific and intended for relative regression use.

## Review Focus

- private-state immutability and every possible aliasing route
- nested mode-filtered field validation and unsupported projected values
- Token/Decode/More interaction, syntax/shape/count parity, and number handling
- reader, parser, context, size, and diagnostic precedence
- path, symlink, regular-file, descriptor, and max+1 behavior
- value leakage and CLI/runtime adapter compatibility
- benchmark fixture construction, timer boundaries, and retained-result bias

# Finding Resolution

## Malformed unselected JSON changed CLI diagnostics

Finding: the streaming decoder emitted `unexpected EOF` and
`unexpected trailing JSON value` where the existing Cobra adapter exposed
`unexpected end of JSON input` and the standard library's
`invalid character ... after top-level value` diagnostic.

Root cause: the differential oracle compared acceptance and shape, but not the
legacy adapter's intentionally preserved local error text.

Fix: malformed inputs replay json.Unmarshal(any) through the same opened
regular-file descriptor after bounded drain/size checks. Valid inputs never
enter replay.

Regression test: `TestDiffPreservesUnselectedMalformedJSONDiagnostics` pins
both exact messages through `App.Run` on a narrow diff.

Verification: the test was added first and reproduced both failures, then
passed after the fix. Focused, race, and fuzz tests passed, and both reviewers
rechecked the change.

## Diagnostic replay could mask a transient reader error

Finding: a decoder error is not necessarily a JSON-content error. Broad replay
could reread after a transient I/O failure and replace the historical read
error with a parser diagnostic.

Root cause: replay was gated only on a non-nil scanner error.

Fix: `isJSONContentError` admits only known JSON error families. Any other
scanner error immediately returns as an invalid-dump read failure, before
drain, size reinterpretation, or replay.

Regression test: a one-shot reader failure during record decode must remain
the exact reader sentinel and must not classify as JSON content.

Verification: focused and race tests passed; both reviewers rechecked the
classification and precedence.

## json.Decoder.More could swallow a reader error

Finding: Decoder.More returns only bool. A one-shot error immediately before
the closing array bracket could collapse to false, after which Token retried
successfully and the scan returned nil.

Root cause: the scanner trusted More to preserve errors across decoder reads.

Fix: scanResourceJSON owns a sticky reader that latches the first non-EOF
failure. Context wins first; then the latched read failure overrides any
decoder result, including simultaneous malformed bytes. The failure wrapper is
explicitly non-replayable.

Regression tests: one-shot failures during Decode and immediately before `]`
both remain visible; a reader returning malformed bytes with `n > 0` and an
error proves the read error wins over syntax.

Verification: the pre-`]` case was added first and reproduced the silent
success, then passed after the sticky-reader fix. Focused, package, race, vet,
formatting, CLI-diagnostic, and fuzz checks passed. Both reviewers approved the
final correction.

# Adversarial Review

Fresh-context reviewer: Parfit (gpt-5.6-luna, max,
019f519f-4b78-7ab1-bd17-e5e4677b43ab) and Lorentz (gpt-5.6-terra, xhigh,
019f519f-492c-7bc1-9ad6-8e5773526865)

Process baseline: origin/main at b0597dfb8e673a06d99995e6e1360cfcc709f0a8

Review scope: 22ecb7c7027171decdbfe254d6f425898ae9396f..28d1a1e40852714e0dd3db21893dac49720efed8

Both reviewers were fresh-context, read-only, and did not implement fixes.
Lorentz initially approved the measured immutable-boundary and streaming
design, then found the transient read-error replay problem during recheck.
Parfit found the malformed diagnostic drift and Decoder.More swallowed-error
path. Each requested change was independently reproduced by the builder,
fixed, regression-tested, and returned to both original reviewers. Both final
rechecks approved with no remaining findings.

## Blocking Findings

The three resolved findings are recorded in Finding Resolution. There are no
unresolved blockers.

## Non-Blocking Risks

The known bounded-value cancellation and malformed-input diagnostic replay
costs are documented above. Neither reviewer reported an unresolved
non-blocking finding after the final recheck.

## Machine Contract Review

Valid diff report bytes, dump manifests, machine.v1, schemas, flags, error
taxonomy, and exit classifications are unchanged. Historical invalid local
dump diagnostics are explicitly regression-pinned through the CLI adapter.

## Safety Review

The reviewers confirmed private projected-state immutability, compiled nested
allow-list equivalence, fail-closed projected-value checks, selected-resource
admission, unselected-value discard, complete structural parsing, sticky read
errors, context/read/parser precedence, root-relative access, regular-file and
size bounds, and no report/error value leakage.

## Generated Artifact Review

No generated artifact changed. The repository-wide docs, schema, machine,
surface, boundary, release, and skill-sync checks passed before final approval.

Verdict: approve
