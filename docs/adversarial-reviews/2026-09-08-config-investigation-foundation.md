# Builder Handoff

## Intent

Allow repeated configuration investigations over a validated saved dump using
ordinary resource reads. Add targeted, config-free field semantics for a small
reviewed pilot and a synthetic evaluator for common agent workflow mistakes.
Record the URL over-redaction investigation without shipping the rejected
runtime prototype. Tenant state remains read-only.

## Base / Head

Base commit: `2b350c94680b160d4059af886de1fdce147d6887` (`origin/main`, after PR #131).
Head branch: `feature/config-investigation-foundation`.
Review input: the tracked diff and added source, tests, fixtures, and documents
in the shared working tree against that base, including the fixes below.
Process baseline: `origin/main:docs/adversarial-review.md`, the builder handoff
template, and the fresh-context reviewer run prompt.

## Files Changed

- `internal/diff/collection.go`, its tests, and shared `diff.go` admission:
  immutable saved collections, bounded admission, physical payload validation,
  safe numeric parse errors, projected list/get/show and safe provenance.
- `internal/cli/snapshot.go`, its tests, app routing, product dispatch, global
  flags and introspection: `--from-dump` with preflight before filesystem access
  and routing before configuration/provider/client construction.
- `internal/resources/semantics*.go`, candidate semantics JSON Schema,
  `internal/cli/commands_schema_describe*.go`, parent registration and format
  tests: config-free discovery for one resource.
- `evals/agent-workflows/`: six synthetic tasks, corpus, saved fixture, task and
  trace contracts, evaluator, examples and regression tests; wired into Makefile
  and the CI unit job via `make test`.
- CLI goldens and surface-change manifest, generated CLI reference, README,
  architecture/threat notes, formal scope, URL study, man page, AGENTS, CLI
  machine workflow, canonical skill and generated agent copy.

No SDK, dependency, resource coverage, catalog classification, or runtime
redactor change is part of this increment.

## Source Inputs Consulted

Existing dump metadata/inventory validation and writer operation choice; diff
admission and projection; projected-loader/getter interfaces; machine executor,
error types and exit mapping; catalog and field mode rules; vendored Zscaler SDK
models for locations, URL filtering policies and rule labels; existing redaction
scanner/final writer; CLI generation and machine contract fixtures. Synthetic
fixtures contain no tenant data. The handoff describes claims to verify, not
independent evidence.

## Generated Artifacts

```sh
make gen-cli-docs
scripts/sync-agents-skill.sh
go test ./cmd/zscalerctl -run 'TestGoldenSurface|TestCommandTreeInventory' -update
```

Expected delta: one global flag (`--from-dump`, 15 to 16 global flags), one
`schema describe` command, conditional saved-read effects on virtual resource
read commands, updated help/parent usage, and process fixtures for schema
describe, saved pages and rejected source scope. The surface-change manifest
explains the deltas. `schema list`, machine resource capabilities, field coverage,
existing read shapes and exit-code definitions remain unchanged. The new
semantics schema is a separate candidate contract. Invalid overflowing numbers
now produce value-free diagnostics in both collection admission and diff;
successful diff behavior and error classification are preserved.

## Invariants Claimed

- Saved reads require complete, valid artifacts admitted under the stored mode
  and current catalog. Unknown scope, absent IDs, invalid artifacts and empty
  successful collections remain distinguishable.
- Saved routing does not load configuration, resolve credentials, run providers,
  construct an SDK client or contact Zscaler. Explicit output retains its existing
  local-write effect.
- Projection and field narrowing cannot widen the allow-list. Returned data and
  catalog copies cannot mutate the admitted in-memory collection.
- Schema describe exposes only reviewed metadata allowed in the active mode.
  Resources outside the three-resource pilot report `not_reviewed`; collection
  ordering stays `unknown` rather than guessed.
- The evaluator binds successful read evidence to requests and the fictional
  corpus. It validates submitted traces; it does not run an agent or establish
  an agent performance baseline.
- Runtime redaction is byte-identical to the base. URL findings are diagnostic.

## Tests Run

Focused core, CLI, metadata and evaluator tests passed during implementation and
review. The final integrated command and result are recorded below after the
independent recheck. All validation is offline; no tenant key or live API call
was used.

## Known Deferrals

- URL over-redaction remains. A global component-scanning prototype was rejected
  because it created new opaque-token blind spots; see URL_REDACTION_STUDY.md.
- The 256 MiB cap bounds serialized resource data, not RSS. Full decoding or
  projection can delay cancellation observation. No hard time/memory budget is
  promised.
- Separate CLI invocations reload disk. No cross-call filesystem snapshot,
  upstream atomic snapshot, cryptographic provenance, or partial-dump reads.
- Semantic metadata is a small candidate pilot, not policy simulation or a
  public library freeze. Future SDK changes require deliberate metadata review.
- Real-agent reliability and tenant behavior are unmeasured. The evaluator is
  only as trustworthy as externally captured traces and their runner.

## Review Focus

Review source routing/effects before credential or filesystem work; hostile
artifact shape, numbers, identities, paths, byte limits and redaction admission;
missing versus empty results; cancellation/error context; exact metadata versus
SDK/catalog evidence; generated runtime/docs/schema parity.

For trace evaluators, mutate successful evidence independently from the answer:
source, contact status, operation, IDs, requested fields, exit code, values and
secret keys. Mutate task/corpus expectations independently. Exercise optional
contract fields, type/range bounds and unknown properties, not just required
happy-path fields. Fixture examples cannot establish autonomous-agent accuracy.

# Adversarial Review

Fresh-context reviewer: `/root/review_saved_collection`, `/root/review_saved_cli`,
`/root/review_semantics_evals`, and `/root/review_url_refinement`. Each was created
with no inherited implementation conversation, using `gpt-5.6-luna` at `max`
reasoning. Reviewers inspected source and ran independent checks; none
implemented fixes. The builder applied confirmed fixes and requested narrow
rechecks of the findings and resulting changes.

## Blocking Findings and Resolutions

1. **Saved physical payload shape.** Shared diff normalization accepted list
   objects and show arrays, so malformed input could pass admission or fail late.
   Collection-only admission now mirrors the dump writer: show requires an
   object; list requires an array, including list-backed singletons. Compare
   keeps its legacy shape handling. `TestLoadCollectionEnforcesWriterPayloadShape`
   and independent malformed/valid artifact probes verify rejection and controls.

2. **Artifact numeric value echo.** The JSON number conversion error included
   the full overflowing lexeme. Shared validation now returns a value-free
   sentinel. Collection, Compare and CLI no-echo tests verify invalid-dump/usage
   classification, no payload disclosure and no output on rejection. The core
   reviewer independently reproduced safe rejection after the fix.

3. **Blank get ID before filesystem access.** Arity validation accepted an empty
   or whitespace-only ID. Preflight now requires a nonempty trimmed ID before
   opening the artifact. `TestFromDumpInvalidInvocationPrecedesFilesystemReads`
   and rebuilt-binary probes verify exit 2 even with a missing dump path.

4. **Saved admission context error contract.** Raw context errors bypassed the
   machine error mapping. The saved route now maps cancellation/deadline to fixed
   safe machine errors with operation/product/resource and preserved context
   sentinels. `TestFromDumpContextErrorsUseMachineBoundary` and independent probes
   verify kinds, `errors.Is`, no emitted output and exit mapping (1/5).

5. **Schema describe argument metadata.** Runtime exact-two arity was absent
   from introspection. The command now declares `exact:2`; a regression checks
   policy and N, and regenerated docs/goldens were reviewed. Independent runtime
   and introspection probes agree.

6. **Evaluator false passes and contract gaps.** Earlier scoring accepted
   failed or mismatched reads as supporting evidence; malformed fixtures could
   raise KeyError; corpus values were not used as an oracle; requested fields,
   saved IDs, secret fields and unknown task properties were insufficiently
   checked. Shared request matching now binds source/contact/operation/resource/
   ID/fields; supporting reads must succeed. Corpus and task validation binds
   expectations and record values; explicit saved-field allowlists and recursive
   secret-key rejection cover event and answer evidence. Malformed inputs return
   invalid/exit 2. Regression mutations cover each reported case. Optional trace
   metadata types and nonnegative finite duration bounds are also rechecked
   against the published trace contract.

7. **URL prototype safety regressions.** Component splitting reduced benign URL
   false positives but exposed opaque spans caught by the baseline. The runtime
   change was withdrawn. The reviewer verified the baseline redactor, absent
   prototype test file, successful redaction suite and clean Go package graph.
   A transient missing study link was fixed; scope and study ship together.

## Non-Blocking Risks

Cancellation latency during full decoding/projection remains documented in the
scope. Serialized bytes are not a process-memory bound. Semantics require future
SDK/catalog evidence review. No real-agent result is claimed. Future URL changes
need differential coverage for explicit credentials, opaque components, percent
encoding, userinfo, nested URLs, JSON escapes, NDJSON, final writer, idempotence
and all modes before reconsidering a runtime refinement.

## Machine, Safety and Generated Artifact Review

Saved reads retain the existing renderer/narrowing path and safely distinguish
missing resources from empty lists. No secret-field/classification expansion or
SDK reader rewiring occurs. Existing dump/diff schema and successful read shapes
remain; saved shape enforcement is admission-specific. The numeric diagnostic
change is intentional. Schema describe is a separate config-free contract,
verified for mode filtering and unreviewed resources. Pilot descriptions for
all 27 fields were checked against the vendored SDK and catalog. Generated CLI
docs, goldens and skill copies were regenerated and drift-checked.

## Independent Recheck Results

- Saved collection core: approve with nits; shape and numeric disclosure findings
  resolved; cancellation latency remains a documented limitation.
- Saved CLI: approve; blank ID and context error findings resolved.
- URL investigation: approve with nits for diagnostic-only scope; no runtime
  prototype is approved or included.
- Semantics/evaluator: approve; all reported evidence and contract mutations
  rechecked, including optional metadata, malformed-input CLI exit 2, six valid
  traces, config-free mode/arity probes and 23 evaluator tests. The documented
  fixture-oracle limitation remains; no autonomous-agent measurement is claimed.
  A final bounded recheck also approved the CI unit job invoking `make test`,
  preserving the Go suite and adding the evaluator suite.


Verdict: approve with nits

All four independent review scopes have cleared after the resolution loop.
The combined verdict retains the documented cancellation, metadata maintenance,
evaluator-measurement and URL follow-up limitations above.

## Integrated Verification

`make verify-adversarial-review` and full `make check` passed with this approved
artifact present. The aggregate includes all Go tests and race tests, 23 Python
evaluator tests, vet, root/tools vulnerability scans, staticcheck, license checks,
Semgrep, secret scanning and allowlist checks, toolchain/client verification,
SDK/core/experiment/machine boundaries, generated docs and surface checks,
release/scaffold/script checks and skill synchronization. Local execution log:
`scratch/core-investigation/make-check-approved.log` (ignored, not a shipped
artifact). Earlier integration runs found the resolved issues described above;
they are superseded by this final successful run.

The built candidate also passed synthetic bounded-list, direct-get and
singleton-show reads from `evals/agent-workflows/fixtures/dumps/synthetic-small`,
and targeted JSON semantics discovery. Runtime redaction was independently
verified unchanged from the base commit. No live keys or tenant calls were used.
