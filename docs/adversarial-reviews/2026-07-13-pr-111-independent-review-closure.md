# Builder Handoff

## Intent

Close the four defects reported by the independent review of PR #111 at
`89b715a`: partial-diff reconciliation, dump publication/cancellation
linearization, TypeScript bootstrap lifecycle, and active-catalog force
validation. Preserve the supported CLI and the immutable stdio v1 schemas
while making the candidate host and reference client truthful about local
filesystem commit.

## Base / Head

- Reviewed base: `89b715ad98f6f37898ccd4ce765d6e651f920764`
- Head: reviewed uncommitted working tree on `feature/stdio-engine-api`
- Process baseline: `origin/main` at
  `b0597dfb8e673a06d99995e6e1360cfcc709f0a8`
- Pull request: #111

## Files Changed

- Partial-diff state and wire conversion under `internal/diff/` and
  `internal/enginewire/adapter/`
- Dump publication, force validation, test hooks, and active-catalog wiring
  under `internal/dump/` and `internal/runtime/`
- Operation-scoped effect arbitration under `internal/effectcommit/` and
  `internal/enginehost/`
- Process-level host regressions and the purpose-built dump test engine under
  `cmd/zscalerctl-engine/` and `internal/enginehost/testdata/`
- TypeScript startup, process, cancellation, stderr, fixtures, tests, and
  consumer documentation under `clients/typescript/`
- Candidate protocol behavior documentation and core-boundary verification
  under `docs/ENGINE_STDIO_PROTOCOL_V1.md` and `scripts/`
- This review artifact

## Source Inputs Consulted

- The supplied commit-pinned independent review of PR #111 at `89b715a`
- `internal/diff` collection-state and summary construction
- `internal/dump` staged publication, atomic exchange, quarantine, and
  complete-artifact admission paths
- `internal/runtime` injected catalog, dump selection, collection, and
  publication path
- `internal/enginehost` decoder, coordinator, worker, writer, cancellation,
  shutdown, and process-exit lifecycle
- The immutable bootstrap and v1 protocol schemas plus the candidate protocol
  documentation
- Node child-process lifecycle and the existing strict TypeScript transport
  abstraction

## Generated Artifacts

None. The immutable bootstrap/v1 schemas and hashes, supported CLI reference,
machine manifest, introspection, field coverage, surface goldens, and release
artifacts are unchanged. The added JavaScript helpers and Go test engine are
purpose-built test fixtures, not generated supported surface.

## Expected Delta

- A selected partial-dump resource whose collection failed remains a note-only
  entry in the in-process report but is omitted from the immutable v1 stream;
  `resources_compared` therefore continues to count only real comparisons.
- Atomic new-destination rename and existing-directory exchange are bracketed
  by an operation-scoped effect boundary. Cancellation wins before admission,
  but cannot report `canceled` or kill confidential cleanup after publication
  committed.
- Shutdown has no deadline only while an atomic effect result or committed
  worker cleanup is genuinely in flight. A failed effect or completed worker
  restores the ordinary bounded shutdown deadline.
- TypeScript bootstrap accepts a bounded timeout and `AbortSignal`; process
  startup failure aborts and awaits the direct child before rejection.
- The reference process adapter drains and discards stderr with constant
  memory. Stderr volume and read-side errors cannot kill a committed dump.
- Protocol v1 has no wire-visible effect marker, so the reference client sends
  dump cancellation but does not apply its own dump cancel/close kill watchdog.
- Forced replacement uses the same snapshotted active catalog that advertised,
  selected, and collected the dump resources.

## Invariants Claimed

- No operation can mutate Zscaler tenant state.
- Projection, redaction, credentials, secret-provider execution, raw SDK
  values, backend errors, and caller-path handling are unchanged.
- A destination that has committed is never described by a `canceled`
  terminal; output failure after commit is a transport/delivery failure.
- Confidential old-artifact cleanup cannot be terminated by the ordinary
  operation-cancel or shutdown watchdog.
- Failed publication attempts and post-cleanup output stalls retain bounded
  host termination.
- Default and injected engines use one catalog consistently for capability,
  selection, collection, artifact shape, and destructive force admission.
- Stdio v1 schemas and advertised hashes remain byte-for-byte unchanged.
- The candidate host command and TypeScript client remain excluded from release
  archives and are not supported CLI surface.

## Tests Run

- `env -u GOFLAGS make check`: pass after the complete fix/re-review loop
- `go test -race ./...`: pass
- `go test -race ./internal/enginehost ./cmd/zscalerctl-engine`: pass
- `go test -race ./internal/diff ./internal/dump ./internal/runtime ./internal/enginewire/adapter ./internal/effectcommit`: pass
- Five repeated race-enabled process runs covering partial diff, cancellation
  immediately after new-destination commit, and cancellation during force
  cleanup blocked beyond five seconds: pass
- `bash scripts/verify-typescript-client.sh`: pass, 40/40
- Focused TypeScript bootstrap, abort, stderr-flood, and committed-dump
  cancellation tests: pass, 5/5
- `bash scripts/verify-core-boundaries.sh`: pass
- `bash scripts/test-verify-core-boundaries.sh`: pass
- Windows amd64 cross-compilation of the engine command, dump, runtime, and
  engine-host test packages: pass
- `git diff --check`: pass

## Known Deferrals

- Protocol v1 cannot expose the filesystem commit boundary to third-party
  clients. The reference client therefore trades a bounded wait against an
  untrusted engine for preserving committed dump cleanup. A future protocol
  version should add an explicit effect-commit acknowledgment.
- Existing-directory atomic replacement remains intentionally unsupported on
  Windows and fails closed. Native Windows CI remains the execution proof for
  the supported new-destination path; no live tenant credentials are required.
- The stdio host and TypeScript client remain candidate-only pending the
  protocol's existing promotion gates.

## Review Focus

- Race cancellation, EOF, malformed input, writer failure, and process shutdown
  against effect begin, effect finish, publication, cleanup, outcome, and worker
  completion.
- Verify every path that suppresses a deadline later restores one when killing
  the worker becomes safe.
- Verify note-only partial resources cannot become counted v1 comparison items.
- Verify injected/restricted catalogs govern both positive and negative force
  admission.
- Treat bootstrap helpers and stderr as hostile process behavior and prove no
  hidden child survives startup rejection.

# Adversarial Review

Fresh-context reviewer: Volta (`019f5e84-c60d-7d20-9a87-2543e58495b5`, Luna xhigh) and Dalton (`019f5e84-c7ee-7440-8ffb-4c98519fd1e3`, Luna max)

Both reviewers inspected the actual working tree read-only, ran independent
focused tests, and did not implement or modify the change. The supplied
independent PR review first identified four defects; the builder implemented
their union and submitted the resulting tree for fresh adversarial review.

## Independent PR Findings Resolved

1. Partial collection failures were emitted as ordinary `diff_resource` items
   even though the summary did not count them. `ResourceDiff` now retains
   private comparison state, and the v1 adapter omits note-only skipped entries.
   Core, process, and TypeScript regressions prove a same-scope
   `allow_partial=true` comparison completes with zero compared resources.
2. Filesystem publication committed before protocol cancellation did. The new
   effect runner brackets only atomic rename/exchange, lets the coordinator
   arbitrate concurrent input before admission, and protects committed cleanup
   until the worker exits. Process hooks prove a post-publication cancel returns
   success and a force cleanup blocked beyond the production watchdog remains
   alive until released.
3. TypeScript bootstrap could wait forever while hiding a live child. One
   overall startup gate now bounds hello, negotiation writes, rejection, and
   ready; abort uses the same gate, and `spawnEngine` awaits child completion
   after termination. Silent, hello-without-ready, abort, and PID-liveness
   regressions cover the lifecycle.
4. Force admission consulted the global catalog instead of the engine's active
   catalog. Publication now receives and snapshots the collector catalog.
   Regressions prove a custom absent-from-global resource can replace its own
   artifact and a restricted catalog rejects an otherwise global artifact
   without changing it.

## Fresh-Review Findings Resolved

1. Volta found that an already-decoded cancellation and effect-begin message
   could both be ready, allowing select order to admit the effect first. Effect
   begin now drains the single already-authorized decode result through normal
   input handling before acknowledgment. The regression proves the queued
   cancellation wins while preserving the canceled-terminal outcome channel.
2. Dalton found that cancel followed by shutdown during a failed effect could
   leave shutdown with neither its own timer nor a selectable operation timer.
   Shutdown now takes precedence after failure and immediately restores its
   deadline; graceful, fatal, and transport combinations are covered directly.
3. Dalton found that successful effect protection remained sticky after worker
   cleanup, allowing blocked terminal/fatal output to wait forever. Protection
   now means committed effect plus live worker, worker completion rearms any
   suppressed shutdown timer, and an integration regression proves both
   graceful and fatal blocked writers end with `ErrJoinTimeout`.
4. Dalton found the TypeScript adapter's former 64 KiB stderr bound could
   blindly `SIGKILL` an active committed dump. Stderr is now continuously
   drained and discarded without retention or process authority. A helper emits
   more than 64 KiB after `started` and still completes the dump.

## Final Reviewer Verification

- Volta: `go test -race -count=50 ./internal/enginehost` and the engine command
  process suite passed; no blocking finding remained.
- Dalton: repeated race-enabled engine-host tests, the engine command process
  suite, the complete 40-test TypeScript verifier, and diff checks passed; all
  three assigned blockers were closed.
- Both reviewers confirmed that cancellation remains concurrent only when the
  input has not yet been decoded, and that the unbounded active-dump client
  watchdog is an intentional, accurately documented protocol-v1 limitation.

## Non-Blocking Risks

- A malicious or permanently wedged engine can make a reference-client dump
  wait indefinitely after cancellation or close. Without a v1 wire-visible
  commit marker, killing it on a client timer would recreate the confidential
  partial-cleanup defect. This is explicitly documented and deferred to a new
  protocol capability/version.
- Native Windows behavior still depends on required CI execution. The local
  cross-build is clean, and no credentialed tenant access is involved.

## Machine Contract Review

The supported CLI, `machine.v1`, JSON/NDJSON output, error envelopes, exit
codes, introspection, schema list, and release artifacts are unchanged. The
candidate v1 stream now accurately excludes resources that were not compared;
the in-process diff retains its explanatory note. Protocol schemas and hashes
remain immutable.

## Safety Review

The reviewers found no projection, redaction, credential, or secret-provider
change. The effect boundary narrows process-kill authority around local dump
publication and confidential cleanup; it does not broaden tenant or filesystem
operations. Active-catalog force validation now closes both the custom-catalog
false rejection and restricted-catalog over-authorization paths.

## Generated Artifact Review

No generated supported artifact changed. The immutable stdio schemas and
hashes, CLI docs/goldens, field coverage, machine fixtures, and release surface
remain unchanged. Added fixtures exercise hostile process and publication
timing only.

Verdict: approve with nits
