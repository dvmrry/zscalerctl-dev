# Builder Handoff

## Intent

Add the final planned typed in-process read operation to the common Go engine:
compare two existing local dump artifacts through diff.compare, return a closed
admitted report, and route the existing Cobra diff command through that engine
operation without changing supported CLI, diff-report, schema, golden,
exit-code, or machine.v1 surfaces.

Treat selected dump records as untrusted at the engine boundary. Validate and
copy exact selectors before filesystem access, structurally parse every
manifest body, admit selected values only after catalog/redaction verification
and idempotent re-projection, emit deterministic lifecycle events, and keep
rendering, global --output, detail, and exit 7 as adapter policy.

## Base / Head

Base commit: 8c83078

Initial implementation head: bf1367e

Reviewed fix head: 8dd5e5b

Review scope: 8c83078..8dd5e5b

## Files Changed

Design and contract documentation:

- docs/ENGINE_API_DESIGN.md
- docs/ENGINE_CAPABILITY_MODEL.md
- docs/cli/machine-contract.md

CLI adapter and regressions:

- internal/cli/app_test.go
- internal/cli/cobra_diff_test.go
- internal/cli/commands_dump_diff.go
- internal/cli/dump_diff.go

Diff parser, admission, comparison, copying, cancellation, and tests:

- internal/diff/diff.go
- internal/diff/diff_test.go

Typed machine capability model:

- internal/machine/engine_diff.go
- internal/machine/engine_diff_test.go
- internal/machine/engine_manifest.go
- internal/machine/engine_manifest_test.go
- internal/machine/types.go

Trusted runtime and regressions:

- internal/runtime/diff_engine.go
- internal/runtime/diff_engine_test.go
- internal/runtime/runtime_test.go

## Source Inputs Consulted

- AGENTS.md and the adversarial-review workflow, handoff, run prompt, and
  report templates at the explicit base 8c83078
- the accepted engine API and capability checkpoints
- the existing diff report, comparison semantics, CLI adapter, command tests,
  and frozen drift golden
- dump manifest v2 types, writer behavior, and published schema
- catalog validation, rendered-subset verification, projection, redaction, and
  projected-value copying boundaries
- the common event stream, typed boundary errors, dump capability patterns,
  CLI error envelope, exit mapping, and Go 1.26.5 os.Root behavior

## Generated Artifacts

None. No supported JSON schema, dump fixture, machine.v1 fixture,
introspection golden, command-surface golden, generated CLI document,
field-coverage artifact, release artifact, or generated skill changed.

## Expected Delta

- Add candidate engine.v1 capability diff.compare, operation diff, typed
  request, closed report result, and an always-possible local-filesystem read
  effect.
- Add runtime.Engine.Diff as a config-free local operation.
- Resolve and copy canonical product/resource selectors and validate the
  tenant-read-only catalog before reading either path.
- Structurally parse and count-check every manifest body; apply catalog,
  redaction-mode, and idempotence admission only to selected records.
- Emit deterministic started/progress/terminal events and static typed errors.
- Preserve Cobra JSON/table/detail rendering, global --output, unsupported
  formats, legacy local-input messages, and --fail-on-drift exit 7.
- Preserve report schema and bytes, including resources:null for an empty
  report.

## Invariants Claimed

- diff.compare is advertised only under the same valid, duplicate-free,
  tenant-read-only list/show catalog condition as dump.write.
- Diff never loads config, resolves a provider, constructs an SDK reader,
  executes a process, or contacts Zscaler.
- Request/catalog validation, selection, copying, and finished-context handling
  occur before filesystem access.
- Every manifest body remains structurally parsed and record-count checked.
  Unselected values are discarded and cannot enter a report.
- Selected records enter a report only after exact catalog/mode subset
  verification and idempotent re-projection/redaction. Unknown, nested unknown,
  secret, mode-disallowed, unsafe, or non-idempotent selected values fail
  closed.
- Raw parser, file, callback, and attacker errors do not cross the typed result
  or terminal event. The private legacy adapter retains only sanitized Cobra
  local-input messages and safe sentinels.
- DiffResult recursively copies mutable report values at construction and
  access while preserving nil-versus-empty JSON semantics.
- Caller mutation cannot alter prepared selectors. Event delivery is
  synchronous and terminal-exactly-once.
- Roots and files close when cancellation lands immediately after open.
- Supported diff bytes/schema, CLI behavior, machine.v1, schemas, goldens, and
  generated artifacts are unchanged.

## Tests Run

Builder verification, all passing at the applicable reviewed heads:

- focused non-cached tests over internal/diff, internal/machine,
  internal/runtime, internal/cli, and cmd/zscalerctl
- strict-admission tests for unknown, nested unknown, secret, mode-disallowed,
  non-idempotent, duplicate-resource, and attacker-canary inputs
- scoped-selection tests proving unselected catalog-unadmitted fields do not
  fail or leak, selected fields fail closed, and structurally invalid
  unselected bodies still fail
- result deep-copy, nil-slice JSON, request mutation, invalid catalog,
  cancellation, progress, sink failure, and config-free execution tests
- affected tests repeatedly at -count=20 and under the race detector
- go vet, staticcheck, format, docs, CLI-doc, core-boundary,
  machine-contract, and surface-manifest checks
- the frozen diff-fail-on-drift golden
- clean env -u GOFLAGS make test
- clean env -u GOFLAGS make check at bf1367e and final head 8dd5e5b,
  including repository-wide test/race, vulnerability, license, contract,
  secret, workflow, release, generated-artifact, and skill-sync gates

Fresh-context reviewers additionally ran focused diff, runtime, machine, and
CLI tests and inspected the exact source ranges and contract artifacts.

## Known Deferrals

- cancellation is observed before and after, but not during, one
  json.Unmarshal or one recursive projection/canonicalization traversal; each
  resource file remains bounded at 512 MiB
- versioned stdio DTOs, framing, codecs, cancellation frames, and clients
- frontend, MCP, Wails, Ink/OpenTUI, Ratatui, and GUI adapters
- config initialization as a separately reviewed local-effectful capability
- a public Go package or promoted engine wire contract
- redesign of historical number precision, aggregate streaming, or identity
  semantics

## Review Focus

- selected-record admission and routes for unselected or attacker values into
  reports, errors, or events
- scoped selection compatibility and whole-manifest structural integrity
- capability truth, validation/cancellation ordering, and config-free behavior
- recursive copying, nil-versus-empty bytes, paths, handle cleanup, parser
  bounds, progress, sink failure, and terminal lifecycle
- Cobra JSON/table/detail/output/error/exit compatibility and frozen goldens
- machine.v1, schema, generated-doc, and artifact drift

# Finding Resolution

## Scoped diff admitted unselected resource values

Finding: the initial implementation loaded and strictly admitted both dump
artifacts before resolving --products or --resources. A catalog-known but
unselected resource containing an unadmitted field could newly fail a scoped
diff.

Root cause: structural parsing and catalog/redaction admission were combined in
readResource, and selectedSpecs ran only after loadDump.

Fix: resolve selected specs before filesystem access and pass exact keys to
loadDump. Every successful manifest body is still opened, structurally decoded,
and record-count checked. Only selected records pass through admitRecords and
are retained; unselected values are discarded.

Regression test: direct tests prove a selected keyed change succeeds with an
unselected clientSecret canary and that an unselected non-object record still
returns ErrInvalidDump. A Cobra --resources zia/locations test proves only the
selected change renders with no canary. The selected unadmitted-field test
continues to fail closed.

Verification: focused, repeated, race, vet, staticcheck, docs, contract,
boundary, surface, and full make check runs passed. The original reviewer
rechecked bf1367e..8dd5e5b and approved with no new blocker.

# Adversarial Review

Fresh-context reviewer: Bohr (gpt-5.6-terra, xhigh,
019f50fb-918e-7be2-9499-c0bfecb7935e)

Independent final-head reviewer: Boyle (gpt-5.6-terra, xhigh,
019f5116-9ed6-7872-884c-1940cd62a91d)

Process baseline: explicit base commit 8c83078

Review scope: 8c83078..8dd5e5b

Both cited reviewers were read-only and did not implement the change. Bohr
reviewed the initial implementation, found the scoped-admission compatibility
regression, and requested changes. After remediation, Bohr rechecked the exact
fix delta and approved with no new blocker. Boyle independently reviewed the
full final range and approved with one non-blocking responsiveness nit.

## Blocking Findings

The review found the scoped-admission defect described above. The builder
verified it from source, fixed the root cause, added direct and Cobra
regressions, reran focused/race/full gates, and returned the exact fix head for
recheck. Both final reviews found no unresolved blocker.

## Non-Blocking Risks

Cancellation checks surround filesystem reads, per-record admission,
comparison loops, callbacks, and terminal delivery. One maximum-size JSON value
can still complete json.Unmarshal or one recursive projection/canonicalization
traversal before observing cancellation. Resource files are bounded at 512
MiB. Both reviewers treated this as future responsiveness hardening, not a
correctness, compatibility, or data-leak blocker.

## Machine Contract Review

The candidate manifest advertises executable diff.compare with its exact
operation, input, result, read-only posture, and local-read effect. Requests,
results, events, and candidate discovery reject direct wire serialization.
Supported machine.v1, report schema/bytes, CLI rendering, global output, error
taxonomy, and exit 7 did not change.

## Safety Review

The final reviews confirmed pre-read validation and copying, config-free
execution, whole-manifest structural parsing, selected-record admission,
idempotent redaction, static typed errors/events, recursive report copying,
context classifications, handle cleanup, root-relative access, and
sink/terminal behavior. No attacker canary crossed a selected failure or
unselected success boundary.

## Generated Artifact Review

No frozen or generated artifact changed. CLI-doc, machine-contract, schema,
surface, boundary, release-artifact, and generated-skill checks passed.

Verdict: approve with nits
