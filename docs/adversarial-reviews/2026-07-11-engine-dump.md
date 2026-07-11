# Builder Handoff

## Intent

Add a typed in-process dump capability to the common Go engine and route the
existing Cobra `dump` command through it without changing supported CLI,
artifact, schema, or machine surfaces. Validate and copy the complete request
before config or live access, emit only projected lifecycle events, retain only
value-free summary metadata, make artifact writing context-aware, and bind
`--force` clearing to the validated dump-directory identity.

## Base / Head

Base commit: `0467c3a`

Initial implementation head: `41efd37`

Reviewed fix heads: `74c9e26`, `f86ac91`

Review scope: `0467c3a..f86ac91`

## Files Changed

Command-boundary regressions:

- `cmd/zscalerctl/main_test.go`

Design and contract documentation:

- `docs/ENGINE_API_DESIGN.md`
- `docs/ENGINE_CAPABILITY_MODEL.md`
- `docs/cli/machine-contract.md`

CLI adapter and regressions:

- `internal/cli/app_test.go`
- `internal/cli/commands_dump_diff.go`
- `internal/cli/dump_diff.go`
- `internal/cli/dump_progress_test.go`

Dump artifact and force boundary:

- `internal/dump/dump.go`
- `internal/dump/dump_test.go`
- `internal/dump/force.go`
- `internal/dump/force_test.go`

Typed machine capability model:

- `internal/machine/engine_dump.go`
- `internal/machine/engine_dump_test.go`
- `internal/machine/engine_manifest.go`
- `internal/machine/engine_manifest_test.go`
- `internal/machine/types.go`

Trusted runtime and regressions:

- `internal/runtime/dump.go`
- `internal/runtime/dump_engine.go`
- `internal/runtime/dump_engine_test.go`
- `internal/runtime/runtime_test.go`

## Source Inputs Consulted

- `AGENTS.md` and the adversarial-review workflow, handoff, run prompt, and
  report templates from `origin/main` at `b0597df`
- the accepted engine API and capability checkpoints
- the existing dump collector, projected event stream, redaction and final
  subset verifier, dump writer, force helper, and CLI adapter
- Go 1.26.5 `os.Root`, `os.SameFile`, `Root.RemoveAll`, and release-platform
  implementations for macOS, Linux, and Windows
- supported dump manifest/diff schemas, dump and command goldens, CLI error and
  exit mapping, machine manifests, introspection fixtures, field coverage,
  generated docs, release artifacts, and generated skill gates

## Generated Artifacts

None. No supported JSON schema, dump fixture, `machine.v1` fixture,
introspection golden, command-surface golden, generated CLI document,
field-coverage artifact, release artifact, or generated skill changed.

## Expected Delta

- Add candidate `engine.v1` capability `dump.write`, operation `dump`, typed
  request, closed summary result, and conservative local-read/write/delete,
  network, and process effects.
- Add `runtime.Engine.Dump` plus an injected-reader collector seam over the
  existing official-SDK collection path.
- Validate exact canonical product/resource selectors before config, provider,
  reader, network, or filesystem mutation.
- Emit synchronous started/progress/projected-record/warning/terminal events;
  emit completion only after the entire artifact is finalized.
- Make the existing writer context-aware, finalize each file through a
  same-directory temporary file, and write `manifest.json` last.
- Move force handling into the dump package and clear only a validated owned
  dump through its held open root.
- Keep Cobra flag parsing, spinner/status prose, partial exit 6, legacy local
  error messages, global output policy, and rendering as adapter behavior.

## Invariants Claimed

- `dump.write` is advertised only for a valid, duplicate-free,
  tenant-read-only catalog with an executable list/show resource; advertised
  effects match actual execution.
- Request and catalog validation, copying, and finished-context handling occur
  before config/provider/SDK work and before local mutation.
- Selection is exact, duplicate-free, catalog-ordered, and immune to caller or
  sink mutation after the started event.
- Raw SDK records never cross the engine seam. Record events occur only after
  catalog projection, redaction, and final subset verification.
- Fatal config/provider/reader/backend/output errors expose static
  `MachineError` values and safe sentinels only; partial warnings and artifact
  errors are value-free.
- `DumpResult` carries only counts, effective redaction, and copied value-free
  failures. It carries no output path, record, config, credential, provider,
  SDK value, writer state, or raw error.
- Force clearing performs no deletion before successful collection, rejects
  dangerous or unowned targets, recursively clears only through the validated
  held `os.Root`, performs no pathname deletion, and detects directory or file
  substitution before returning to the writer.
- Context-aware writing cleans temporary files. A canceled or failed partial
  write never leaves a manifest that claims completion or grants force
  ownership.
- Existing dump artifact bytes/schema, CLI stdout/stderr and status prose,
  actionable force/overwrite messages, optional error-envelope context, exit
  codes, supported `machine.v1`, schemas, goldens, and generated artifacts are
  unchanged.

## Tests Run

Builder verification, all passing at the applicable reviewed heads:

- non-cached focused tests over `internal/machine`, `internal/dump`,
  `internal/runtime`, `internal/cli`, and `cmd/zscalerctl`
- focused new dump, force, cancellation, selector, error, result-copy, and
  manifest tests repeatedly at `-count=20`
- final force-boundary tests at `-count=100`
- final output-phase canceled/deadline tests at `-count=50`
- race-enabled tests over dump, runtime, machine, CLI, and command packages
- Windows/amd64 and Linux/amd64 dump-package cross-compilation; native macOS
  test and race execution
- `git diff --check`
- `scripts/sync-agents-skill.sh --check`
- repeated clean `env -u GOFLAGS make check` runs, including final head
  `f86ac91`; repository-wide tests/race, vet, staticcheck, vulnerability scans,
  docs, machine-contract/schema, core/SDK/experiment boundaries, secret scan,
  workflow, surface, release, generated-artifact, and skill-sync gates passed

Fresh-context reviewers additionally ran focused force/runtime/CLI regressions,
source and Go runtime portability inspection, contract/golden drift checks, and
full `make check` verification.

## Known Deferrals

- typed local diff and strict diff-record admission
- versioned stdio DTOs, framing, codecs, cancellation frames, and reference
  clients
- frontend, MCP, Wails, Ink/OpenTUI, Ratatui, or GUI adapters
- config initialization as a separately reviewed local-effectful capability
- a public Go package or promoted engine wire contract

## Review Focus

- capability advertisement versus executable behavior and conservative effects
- validation/cancellation ordering before config, live access, and mutation
- catalog and request copying, exact selector admission, and catalog order
- raw record/backend/config/path/credential leakage through results, events,
  returned errors, sink failures, sentinels, or partial artifacts
- terminal-exactly-once behavior and completion only after manifest finalization
- force ownership, path/symlink checks, directory-identity substitution races,
  context behavior, and old-artifact preservation before the delete boundary
- temp cleanup, manifest-last behavior, file permissions, and artifact bytes
- Cobra prose, JSON envelope fields/messages, exit codes, schemas, goldens, and
  generated-artifact drift

# Finding Resolution

## Force validation and recursive deletion were separated by a pathname race

Finding: the initial refactor validated an owned directory through `os.Root`,
closed that root, and later called `os.RemoveAll(target)`. A same-UID actor could
replace the validated path with an unrelated directory before recursive
deletion.

Root cause: destructive work was resolved again through the pathname instead
of remaining bound to the validated directory identity.

Fix: retain the opened root, capture its identity, compare it with the path
before and after clearing, and recursively clear entries only with
`root.RemoveAll(name)`.

Regression test: a deterministic pre-clear boundary renames the validated dump
and installs a non-dump replacement directory; the operation returns
`ErrUnsafePath` and preserves the replacement file and directory.

Verification: repeated focused and race tests confirmed only the validated
open root is cleared.

## Typed output sanitization erased actionable Cobra messages

Finding: the first typed output boundary reduced force-refusal and overwrite
errors to `dump output failed`, changing plain stderr and JSON
`error.message`.

Root cause: the engine correctly discarded raw local errors, but the legacy
Cobra adapter had no separately sanitized compatibility channel.

Fix: `dumpOutputError` exposes only the static typed boundary through ordinary
`Error`, `Unwrap`, `errors.Is`, and `errors.As`, while retaining one
standard-redacted and control-normalized compatibility message in a private
field. `LegacyDumpAdapterError` is an explicit Cobra-only opt-in that restores
that message and safe sentinel without `MachineError` context.

Regression test: runtime tests prove the typed error remains path-free; CLI
tests assert exact rendered overwrite/force messages and sentinels; command
tests assert those actionable messages survive text and JSON rendering without
machine optional fields.

Verification: the first recheck confirmed ordinary force/overwrite message
compatibility was restored.

## Final pathname removal could delete a substituted regular file

Finding: after identity-bound recursive clearing, a final `os.Remove(target)`
could still delete an unvalidated regular file substituted after the final
identity check.

Root cause: even non-recursive pathname deletion reopened a time-of-check to
time-of-use window.

Fix: remove final pathname deletion entirely. The validated directory remains
empty and is reused by `WriteContext`; no force code deletes `target` by path.

Regression test: a deterministic post-clear boundary renames the validated
empty directory and installs a non-empty regular file at `target`; the
operation returns `ErrUnsafePath` and preserves the file bytes.

Verification: the final recheck confirmed no pathname deletion remains and
both directory and regular-file substitutions survive.

## Legacy message adaptation erased context machine classifications

Finding: the compatibility adapter handled every output error, including
`context.Canceled` and `context.DeadlineExceeded`, replacing their
`MachineError` with a bare context sentinel. JSON kinds and process exit mapping
would regress to `internal`/1.

Root cause: legacy message adaptation was selected before checking whether the
typed output failure carried context semantics.

Fix: `LegacyDumpAdapterError` declines canceled and deadline-exceeded errors,
leaving the original typed error, operation, kind, and sentinel intact.

Regression test: runtime tests cover both context kinds directly. A
deterministic successful one-resource reader finishes the supplied context
before output preparation, and `App.Run` must return the correct
`canceled`/`deadline_exceeded` `MachineError`, operation `dump`, and sentinel
without creating an artifact.

Verification: the final reviewer confirmed JSON classification and exit
mapping remain the existing kind-driven behavior.

# Adversarial Review

Fresh-context reviewer: Locke (`gpt-5.6-terra`, xhigh,
`019f50ae-c8bb-7381-aafd-d52bf4e41042`)

Independent final-head reviewer: Planck (`gpt-5.6-luna`, xhigh,
`019f50ce-f973-70b1-96db-a26cb7819136`)

Process baseline: `origin/main` at `b0597df`

Review scope: `0467c3a..f86ac91`

Both cited reviewers were read-only and did not implement the change. Locke
reviewed the initial implementation, found the recursive force race and CLI
message regression, then found the final pathname deletion and context-kind
regressions during the fix recheck. Locke's final recheck of
`74c9e26..f86ac91` approved with no blockers or nits. Planck independently
reviewed the final remediation and approved with two non-blocking coverage
nits.

## Blocking Findings

The reviews found the four force/error-boundary defects described above. The
builder independently verified each from source, fixed the root cause, added a
deterministic regression, reran focused/race/full gates, and returned the exact
fix head for recheck. The final reviews found no unresolved blocker.

## Non-Blocking Risks

Planck's cross-target test matrix stopped after Linux when its isolated host ran
out of disk; the builder's Windows/Linux cross-compilation and native macOS
tests passed, and the reviewer inspected the Go 1.26.5 handle-backed `os.Root`
implementations. Planck also noted there is no new single command-level test
that creates an output-phase context error and simultaneously asserts both its
JSON envelope and process exit; the deterministic `App.Run` test proves the
typed error survives the adapter, while existing command tests independently
freeze the same `MachineError` JSON-kind and exit mappings. Neither reviewer
considered these blocking.

## Machine Contract Review

The candidate engine manifest advertises an executable `dump.write` capability
with conservative effects. Typed request/result closure, defensive copying,
static errors, event lifecycle, partial counts, and CLI adaptation were
verified. Supported dump bytes/schema, `machine.v1`, introspection, error
envelopes, exit taxonomy, and generated command surfaces did not change.

## Safety Review

The final reviews confirmed pre-config request validation, tenant-read-only
catalog enforcement, projected/redacted/verified events, value-free errors and
results, manifest-last finalization, temp cleanup, context handling, force
ownership checks, identity-bound recursive clearing with no pathname deletion,
and preservation of substituted objects. No backend, path, credential, or
record canary crossed the typed boundary.

## Generated Artifact Review

No frozen or generated artifact changed. CLI-doc, machine-contract, schema,
surface, field-coverage, release-artifact, and generated-skill checks passed.

Verdict: approve with nits
