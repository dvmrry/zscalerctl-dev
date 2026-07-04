# Forward Roadmap

Baseline: `380ebb5` (post PR #91). Authored 2026-07-03; revised same day after
external model review (GPT-5.5 Pro) — revisions noted inline. This document is
the sequencing source of truth; individual PRs still follow the gates, semver
labels, and surface rules in
[DEV_PUBLIC_SURFACE_MODEL.md](DEV_PUBLIC_SURFACE_MODEL.md),
[VERSIONING.md](VERSIONING.md), and [RELEASE_CHECKLIST.md](RELEASE_CHECKLIST.md).

## Standing decisions

- **Fang is rejected** for the supported CLI: self-described experimental,
  owns frozen dispatch surfaces (help, errors, completions, version), ambient
  terminal probing conflicts with the explicit color policy. Styled help is
  implemented in-repo on Lip Gloss v2. Revisit only if Fang reaches a stable
  major version and a spike under experiment rules passes the Workstream B
  boundary criteria. Not up for re-debate at roadmap reviews.
- **Lip Gloss v2** (stable) is the 1.x styling foundation; its manual color
  model matches the pinned-profile discipline `internal/output` already
  enforces.
- **Local MCP server** is planned as an experiment (posture change from the
  earlier "not planned" note in ARCHITECTURE.md). Justification: the official
  `modelcontextprotocol/go-sdk` is v1.0.0 with a formal compatibility
  guarantee, and MCP-only hosts that cannot shell out are a concrete
  integration need. A threat-model gate (D0) precedes any MCP code.
- **Wails v3 is alpha.** GUI work starts as an isolated experiment on v3;
  stability is re-checked at its promotion checkpoint; Wails v2 is the
  documented fallback. Alpha dependencies never enter the root module.
- **Bubble Tea TUI remains optional** (the previously shelved track). If
  explored, it is a human CLI-layer adapter only: Cobra remains the command
  surface, no core/runtime replacement, no supported automation surface,
  post-1.0 only.

## Operating rules

1. Frontends and adapters consume seams; they never define them. Missing
   capability → candidate-seam PR first.
2. Experiments live in nested modules under `experiments/` with their own
   `go.mod`, forbidden-import tests (pattern:
   `experiments/stdio-machine-adapter/main_test.go`), and zero root-module
   dependency changes until promotion.
3. Every PR: one concern, one semver label, `make check` green, golden moves
   recorded in `surface_changes.md`.
4. Machine surfaces (JSON, NDJSON, envelopes, exit codes, manifest, schemas)
   never change as a side effect of presentation work.
5. Trusted runtime assembly stays in `internal/runtime`; presentation layers
   receive projected/redacted records and narrow capabilities only.
6. No frontend invents its own resource semantics, error taxonomy, redaction
   behavior, or progress model. Those exist once, in core.

## Phase 0 — Roadmap PR

Land this file as `docs(roadmap)` via a normal reviewed PR (`semver:none`).

## Phase 1 — Public 1.0 release closeout (release actions only)

Status: the code prerequisites are already merged — #88 (`not_found` kind →
exit 4), #89 (manifest schema + full error-kind fixtures), #90 (NDJSON policy
gate, exit-7 golden, surface-manifest hardening), #91 (`deadline_exceeded` →
exit 5, `canceled` → exit 1, envelope kind vocabulary enumerated in
`error.schema.json`, manifest golden schema-validated).

1. Final validation on the selected dev commit: `make release-check`,
   `make live-smoke` (with credentials), golden surface suite.
2. Close/supersede public dvmrry/zscalerctl#206 as dry-run evidence.
3. Squash-promote the dev baseline to `dvmrry/zscalerctl` as one deliberate
   commit; release notes point to the dev repo for history; never delete or
   move existing public tags.
4. Re-validate in the public repo per RELEASE_CHECKLIST promotion section.
5. Tag `v1.0.0` via manual `workflow_dispatch bump=major`.

Nothing else gates the tag. (Revised per external review: adapter-enablement
cleanup that was previously bundled here moved to Phase 2.) Owner decision
2026-07-04: dev `main` continues shipping 0.x minors until the backlog is
done; Phase 2+ merges are NOT gated on the public 1.0 tag — promotion, when
chosen, picks a validated baseline commit.

## Phase 2 — Adapter-enablement cleanup (pre-adapter-promotion)

1. **`refactor(cli)`: url-lookup behind runtime.** Retire `resourceReader`;
   remove `internal/cli`'s last `internal/zscaler` import; then add
   `internal/cli → no internal/zscaler` as a mechanical rule in
   `scripts/verify-core-boundaries.sh`. `semver:patch`.
2. **`refactor(runtime)`: error-currency cleanup.** Delete the
   `runtimeErrorFromMachineExecution` load-error substitution
   (`internal/runtime/runtime.go:300`) **paired in the same PR** with a
   `machine.ErrorKindLiveAccessFailed → exit 5` case in `exitCodeForError`.
   Hand-traced 2026-07-03: deleting the substitution alone regresses live
   failures from exit 5 to exit 1; with the paired mapping, exit code and
   envelope `kind` are preserved and only the envelope `message` text changes
   (declared non-stable). Tests must pin exit 5 + kind for a live failure
   through the machine path. Result: `MachineError` becomes the single error
   currency for machine paths; adapters get typed errors in every failure
   mode. `semver:minor`.
3. **`docs`: error-vocabulary mapping table** in `docs/cli/machine-contract.md`
   covering all five layers (exit codes, stderr envelope kinds, machine
   `MachineError.Kind`, internal Go sentinels, schema-enumerated kinds) so no
   future adapter invents a further taxonomy. `semver:none`.
4. **`refactor(cli)`: split `app.go`** (~2.6k lines) by responsibility:
   orchestration (`app.go`), command construction (`commands_*.go`), human
   rendering (`render_*.go`), error mapping (`errors.go`), format/NDJSON
   policy (`format_policy.go`). Mechanical moves only; boundary and golden
   tests pass after each move; not interleaved with Phase 3 PRs.
   `semver:none`.

## Phase 3 — Human CLI polish (can run parallel to Phase 2 if PRs stay small)

Hard requirements for the whole phase: the existing color policy
(`--color auto|always|never`, `--no-color`, `NO_COLOR`, `TERM=dumb`, TTY
detection, spinner gating) is preserved exactly; styling is human UX and never
machine contract; `verify-pty-escape-clean` and no-color byte-cleanliness hold
throughout.

1. **`test(cli)`: pretty/table golden expansion** (representative list, get,
   singleton show, `schema list`) — baseline before restyling. `semver:none`.
2. **`chore(output)`: Lip Gloss v2 migration** contained to `internal/output`;
   pinned profile derived from resolved `Style`; no-color bytes identical
   (goldens unchanged); dependency review + vendor. `semver:patch`.
3. **`feat(cli)`: styled help/usage** via owned Cobra `HelpFunc`/`UsageFunc`
   rendering through the redacting writers; plain text when color is off;
   verify `gen-cli-docs`/introspect unaffected. `semver:minor`.
4. **`feat(cli)`: per-shape pretty polish** (list tables with explicit
   truncation policy and no width-dependent field meaning; key-value cards for
   get/show; styled doctor/auth). Same projected/redacted records,
   presentation only. `semver:minor`.
5. **`feat(cli)`: dump progress styling** over the existing
   `DumpProgressFunc` (stderr, TTY-only). `semver:minor`.
6. **(Optional) Bubble Tea experiment** — only under experiment rules, scoped
   as a human browse/explore adapter consuming safe seams; see promotion
   criteria table. Post-1.0 only.

## Phase 4 — Operation event stream (the keystone shared enabler)

1. **Design checkpoint first** (revised per external review — do not go
   straight from the machine-contract.md design sketch to code). The design
   note must answer: event ordering guarantees; cancellation semantics;
   deadline behavior; backpressure; partial-error semantics; the redaction
   boundary (events carry value-free metadata and projected records only);
   whether event schemas are candidate or supported; and how one-shot
   `Execute` is reconstructed from the stream.
2. **Core implementation** as a candidate seam:
   `started`/`progress`/`record`/`warning`/`completed`/`failed|canceled`.
3. **One-shot `Execute` reconstructed from events** — Execute remains the
   stable adapter; events are additive underneath. The CLI/machine contract
   does not change.
4. **Dump progress migrates onto events** (replacing the bespoke
   `DumpProgressFunc` seam or adapting it on top).
5. **Tests** for cancel/deadline/partial-error event sequences.
`semver:minor` (candidate seam).

## Phase 5 — MCP experiment

- **D0 `docs(security)`: MCP threat model + redaction posture — gates all MCP
  code.** Must decide: default redaction mode for MCP (tenant data enters
  model context — evaluate defaulting to `share` rather than `standard`);
  whether raw tenant identifiers are allowed in tool results; what tool
  descriptions may expose (resource names/fields from the catalog are public
  project data; tenant values are not); tool-confusion/overbreadth prevention
  (narrow per-operation tools, no generic "query" tool); logging policy
  (value-free, mirroring dump errors); tools-only vs prompts/resources
  (start tools-only); host allowlist stance; local-stdio-only forever or
  initially. THREAT_MODEL.md addendum. `semver:none`.
- **D1 `experiment(mcp)`: nested module `experiments/mcp-server`.** Official
  `modelcontextprotocol/go-sdk` v1.x; **stdio transport only, no network
  listener**; tools map 1:1 onto the machine contract (`manifest`,
  `schema_list`, `resource_list`, `resource_get`, `resource_show`) with
  read-only annotations; args translate to `machine.Request`; consumes
  `internal/runtime` + `internal/machine` + `internal/machineio` only;
  forbidden-import tests; `ZSCALERCTL_*` env credentials; SDK dependency
  stays in the nested module. Dump over MCP is out of scope until Phase 4
  events exist.
- **D2: host workflow proof + go/no-go.** Promotion requires beating
  skill+CLI for a real workflow in a concrete MCP-only host — "it works" is
  not promotion criteria. Record the decision either way.
- **D3 (if go): candidate promotion.** Machine envelope schemas/versioning
  (request/response/error schemas, version-field decision, `machineio` decode
  errors wrapped as `MachineError`) land **before** this step — that work is
  required for adapter promotion, not for the experiment and not for public
  CLI 1.0. Separate release binary (`zscalerctl-mcp`); never merged into the
  root CLI dependency graph.

## Phase 6 — Wails experiment

1. **v3 stability re-check** at start; v2 fallback documented.
2. **Design note + seam gap analysis**: session/runtime lifetime in a
   long-lived process (start per-action `NewMachine`, matching CLI token
   posture; a bounded session cache is a later, reviewed change), capability
   set vs existing seams.
3. **Nested module spike `experiments/wails-gui`.** Gate (revised per external
   review): the spike uses **static/fixture data only until Phase 4 events
   exist** — the GUI must not invent its own progress model for live
   long-running operations. Read-only browser first: doctor/onboarding,
   catalog tree, list/get/show with filters/fields, redaction-mode selector
   (never off).
4. **Bindings rule (hard):** the frontend receives projected/redacted JSON
   only — no source records, no credentials, no secret refs, no SDK clients,
   no config secrets, no raw response payloads. Everything in the frontend
   bundle is public.
5. **Dump + diff** via Phase 4 events; restrictive-permission dump dirs via
   the existing writer.
6. **Packaging/promotion checkpoint**: code signing/notarization, security
   review addendum (clipboard, screenshots, local caches, logging of projected
   data), and the public-vs-dev-tool decision.

## Skill workstream (rolling, alongside phases)

- **Post-1.0 refresh**: #88/#91 exit-code and kind changes, manifest schema
  pointer, recipes (filters/fields/ndjson, error-kind → next-action table,
  timeout semantics). `scripts/sync-agents-skill.sh` + `verify-agents-skill`.
- **Fixture-based skill eval** (lightweight, no live tenant): given task →
  expected command plan; given error envelope → expected diagnosis; given
  discovery need → expected manifest/schema command; given
  `not_found`/`deadline_exceeded`/`canceled` → expected next action.
- **If MCP ships**: a "CLI vs MCP vs dump/diff vs manual operator step"
  routing section — the skill is the bridge between human docs, agent
  recipes, and adapters.

## Promotion criteria per experiment

| Experiment | Go | No-go / kill | Promotion requires | Release-surface impact |
| --- | --- | --- | --- | --- |
| MCP server | A concrete MCP-only host workflow beats skill+CLI | No host need after evaluation window; security posture unresolvable; maintenance cost exceeds value | D0 threat model, envelope schemas (D3 prereqs), promotion checklist, separate binary | New release artifact; machine envelope becomes supported |
| Wails GUI | Read-only browse + dump/diff demonstrably useful; v3 stable or v2 fallback acceptable | v3 never stabilizes and v2 port cost exceeds value; security review fails | Signing/notarization, security addendum, seam-only consumption proven | New artifact or stays dev-repo tool (explicit decision) |
| Bubble Tea TUI | Operator demand for terminal browse/explore beyond pretty CLI | Pretty CLI (Phase 3) proves sufficient | Same experiment→candidate→supported path; Cobra surface unchanged | None until promoted |

Experiments do not become permanent by inertia: each gets an evaluation
window ending in a recorded go/no-go.

## Dependency-risk table

| Dependency | Status (2026-07) | Risk | Policy |
| --- | --- | --- | --- |
| Lip Gloss v2 | Stable | Styling behavior drift | Root dep allowed only after Phase 3 golden baseline exists |
| MCP Go SDK | v1.0.0, compat guarantee | Tenant data in model context; evolving spec around streamable servers | Nested module until D3; stdio only |
| Wails v3 | Alpha | API churn, packaging churn | Nested module only; stability re-check before promotion; v2 fallback |
| Bubble Tea | Mature | TUI test complexity | Optional post-1.0 experiment only |
| zscaler-sdk-go | Vendored, renovate-gated | Upstream discovery/logging drift | Existing sdk-boundary runbook per bump |

## Decision checkpoints (record outcomes here or as ADRs)

1. **Phase 1:** public 1.0 promotion executed — the next release action.
2. **Phase 2.2:** error-currency cleanup lands only with the paired exit-5
   mapping (hand-trace above is the evidence).
3. **Phase 4.1:** event-stream design checkpoint sign-off before code.
4. **Phase 5 D0/D2:** MCP threat model accepted; go/no-go with host evidence.
5. **Phase 6.1/6.6:** Wails v3 vs v2; public-vs-dev-tool.
6. **Pagination** (`Input.Options` extension): design note only when MCP host
   or GUI demonstrates need — not before.

## Review prompts for external model passes

- Does any workstream let presentation or adapter code bypass projection,
  redaction, or the redacting writers?
- Is the Phase 4 event vocabulary sufficient for dump progress *and* MCP
  progress notifications without a second event model?
- Does the MCP tool surface leak anything the CLI JSON surface does not, and
  is the D0 redaction default right given model-context exposure?
- Is per-action runtime construction in the GUI acceptable for token
  lifetime, and if a session cache is added, where does it live so the
  frontend never holds it?
- What breaks in this plan if Wails v3 never stabilizes?
