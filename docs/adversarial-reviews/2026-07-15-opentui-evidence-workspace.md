# Builder Handoff

## Intent

Give PR #112's unsupported Bun/OpenTUI experiment a defensible use for the
conversation pane before any model integration: typed deterministic result
cards, exact clickable evidence, historical result restoration, and a bounded
manually pinned working set. Add command-aware summaries without promoting
arbitrary tenant fields into inferred insights.

## Base / Head

- Delta base: `e6e40733256afe6efad215decb12058c6f740303`
- Initial reviewed candidate:
  `d551e513bb05b564a3d3a3da43ec520dd7a0ff65`
- Corrective reviewed head:
  `9ce5959cd771d58c90849d13caa3aeff9a3afa1f`
- Branch: `agent/opentui-agent-tui` (draft PR #112)
- Process-doc baseline:
  `498840b4418c83d22e0a49e91c29351802f3e526`

## Files Changed

The source delta changes only `experiments/opentui-agent-tui/**`:

- typed transcript, summary, evidence, result-snapshot, and pin models;
- transcript and working-set presentation;
- JSON-tree stable render identities and fail-closed reveal behavior;
- application snapshot, search, focus, command, and lifetime handling;
- command-aware zscalerctl adapter summaries and semantic count labels;
- `/pin` and `/unpin` local commands;
- README boundaries and focused model, adapter, renderer, and interaction tests.

This review artifact is the only file outside the experiment added after the
approved source recheck.

## Source Inputs Consulted

- The project-neutral `WorkspaceAdapter` and `WorkspaceResult` contract.
- The existing tree/search model and its 800 visible-row and 5,000 searched-node
  bounds.
- Typed TypeScript stdio-client definitions for catalog resources, engine
  manifest, status results, resource reads, exact wire numbers, and diff
  summaries.
- Existing OpenTUI transcript, tree, picker, inspector, focus, mouse, and toast
  behavior.
- Adversarial-review process documents from the explicit baseline above.

## Generated Artifacts

None. The Bun lockfile, supported CLI docs, schemas, goldens, generated skills,
release artifacts, and root dependencies are unchanged.

## Expected Delta

- Supported Go CLI, stdio protocol, machine schemas, resource catalog,
  redaction/projection, dump/diff contracts, and generated artifacts: unchanged.
- Experimental local commands: add `/pin` and `/unpin`.
- Transcript representation: typed text, metric, facet, evidence, and action
  blocks backed by immutable result IDs instead of copied result trees.
- Working-set pins: session-memory only and capped at eight references.
- Catalog, manifest, status, read, lookup, and diff summaries: derived only
  from command, completion, and catalog metadata known to the adapter.
- Generic relationship inference and model-context assembly: deliberately
  absent.

## Invariants Claimed

- No operation can mutate Zscaler tenant state.
- Committed workspace data is copied and deeply frozen before it enters active
  state, transcript evidence, or the snapshot registry.
- Exact `WireNumber` lexemes survive the snapshot boundary.
- Historical evidence resolves only against its own result ID and exact
  own-property JSON path.
- Name/index sorting does not alter source paths or evidence identity.
- Search or evidence paths that cannot materialize inside the 800-row tree fail
  closed; Enter cannot close search as though an unavailable reveal succeeded.
- Result replacement during search resets the transaction baseline and keeps
  the visible match actionable.
- Counts are rendered only with an explicit semantic label; singleton status
  views do not invent a record count.
- `/pin` and `/unpin` remain local UI actions and do not create transcript
  turns.
- Transcript mouse actions stop propagation.
- SDK, engine, credential, projection, redaction, and error semantics are
  unchanged.

## Tests Run

- Final `cd experiments/opentui-agent-tui && bun run check`: 104 passed, one
  expected opt-in integration skip, zero failed, 2,138 assertions.
- Final real-process integration with a freshly built Go engine:
  one passed, zero failed, 175 assertions; config-free only, no credentials or
  live tenant contact.
- Final `env -u GOFLAGS make check`: exit 0, including normal/race tests, vet,
  vulnerability and static analysis, secret and policy checks, machine
  contracts, experiment boundaries, generated-artifact checks, and skill sync.
- Final `git diff --check`: pass.
- Final reviewed source-delta SHA-256:
  `9b72c63667a055a81e965ca229662e6925b79c2b15f6e0b34b27bb9b291ed82d`.
- Corrective-delta SHA-256:
  `e78375a6ef12a3a036768ca9976708829f9a1c3ac9a940eb22338ec4084d59a5`.

## Known Deferrals

- No local or remote model transport and no automatic model-context assembly.
- No relationship graph; names and `*id` fields are not joined generically.
- Transcript/result history remains session-memory and unbounded until
  `/clear`; pins are bounded to eight.
- No TUI dump operation; stdio v1 still lacks a wire-visible publication commit
  marker.

## Review Focus

The reviewer was asked to attack snapshot lifetime and identity, mutation after
adapter return, search replacement and transaction boundaries, row-limit
failure behavior, mouse/focus propagation, exact number and array-path
stability, summary leakage and overclaim, semantic count labels, and regressions
in existing OpenTUI and zscalerctl-adapter policy.

# Adversarial Review

Fresh-context reviewer: Codex subagent `Mill`, session
`019f6663-dc70-7910-816d-2f6c824397d0`, read-only against the isolated PR
worktree.

## Initial Blocking Findings

The reviewer found three source-verifiable blockers in the initial candidate:

1. Result snapshots retained mutable adapter-owned aliases, allowing a later
   mutation or array reorder to rewrite historical evidence while the
   transcript kept its original label.
2. If a result replaced data while search was open, the picker visibly fell
   back to the first new match while imperative action state retained the old
   path ID; the first Enter did nothing.
3. The generic card and context rail rendered every numeric context count as
   `Records`, even when the adapter supplied resources, capabilities,
   classifications, status results, or heterogeneous diff items.

The initial review independently reproduced the first two defects with real UI
probes, ran the experiment and repository gates, made no edits, and requested
changes.

## Resolution Mapping

### Mutable snapshot aliases

Root cause:
`App` stored adapter-owned `WireValue` objects directly in active state and the
snapshot registry.

Fix:
`snapshotWireValue` validates, deep-copies, cycle/depth rejects, and freezes
committed workspace data. Immutable `WireNumber` values retain their exact
lexemes. Initial and subsequent results use the committed copy consistently.

Regression tests:
Model tests mutate the source after commit, verify nested freezing and a huge
exact number, and reject a cycle. The renderer test mutates and reorders the
adapter's source before activating historical pinned evidence.

### Stale search selection after replacement

Root cause:
Rendered search selection had a current-result fallback, but keyboard and copy
actions used a stale old-result ID.

Fix:
A committed replacement resets search's result-qualified baseline and selected
ID. Rendered and imperative actions resolve the same current match, and reveal
still fails closed when the bounded tree cannot materialize it.

Regression test:
An in-flight operation is replaced while search remains open with the same
query at a different path. The first Enter reveals the new path and closes
search; an out-of-bound match still remains open after Enter.

### Incorrect generic record labels

Root cause:
`ContextState.records` was presented with an unconditional `Records` label.

Fix:
Presentation requires an explicit, sanitized `countLabel`. The zscalerctl
adapter supplies `Resources`, `Capabilities`, `Records`, `Classifications`, or
`Diff items` as appropriate, and omits counts for singleton status views.

Regression tests:
Transcript tests reject an implicit record label, and adapter tests cover
catalog, manifest, resource reads, lookup, diff, and doctor omission.

## Focused Recheck

The same reviewer inspected only
`d551e513bb05b564a3d3a3da43ec520dd7a0ff65..9ce5959cd771d58c90849d13caa3aeff9a3afa1f`,
reran TypeScript typechecking, 36 directly affected tests, and both original UI
probes. The mutation probe changed to `mutation_visible:false`; the search probe
changed to `search_open_after_first_enter:false`. All three findings closed,
no directly related regression was found, and the reviewer made no edits.

## Machine Contract Review

No Go CLI, stdio protocol, JSON/NDJSON contract, error envelope, exit code,
dump/diff schema, manifest schema, introspection surface, or generated contract
artifact changed. The entire behavioral delta remains inside the unsupported
OpenTUI experiment.

## Safety Review

No redaction, projection, field coverage, credential, secret-provider, or
tenant-read-only behavior changed. The frontend remains limited to sanitized
`WireValue` data, and the new commit boundary prevents adapter mutation from
silently rewriting historical evidence.

## Generated Artifact Review

No generated or release artifact changed. The Bun lockfile is absent from the
source delta, and diff hygiene passed.

Verdict: approve
