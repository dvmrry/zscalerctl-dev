# Builder Handoff

## Intent

Add a bounded terminal-motion system to PR #112's unsupported Bun/OpenTUI
experiment, give its welcome surface a responsive Poison FIGlet identity, and
use the transcript area for a truthful data-reactive operation scene. Motion
must remain presentation-only and must not alter the Go engine, stdio protocol,
workspace adapter contract, result snapshots, or tenant data.

## Base / Head

- Delta base: `f5fe3b9850ec523f732be0430f30dcbe611e22e1`
- Initial candidate: `d51ed52480048a513d9212fb548f95e54048ebe6`
- Final reviewed source head:
  `46ae7bbd8b76030515fe5d8f6e384afdccd65247`
- Branch: `agent/opentui-agent-tui` (draft PR #112)
- Process-doc baseline:
  `b0597dfb8e673a06d99995e6e1360cfcc709f0a8`
- Final base-to-head binary patch SHA-256:
  `0cf47fe92fed6777fd1a6775d0ce0585059f72b689d09c2b118c290979d6963f`

Implementation commits after the reviewed base:

1. `d51ed52480048a513d9212fb548f95e54048ebe6` — motion system and Poison
   identity.
2. `10bb13fccc19d8fcea6ac61c9627d4a9a9b16660` — bind welcome timers to
   renderable geometry and keep compact progress on one fixed row.
3. `61fb09ac2b6ad43fe129328dcc6114244e38bacc` — prove complete artwork,
   stale callbacks, focus, and compact counter geometry at the renderable
   boundary.
4. `46ae7bbd8b76030515fe5d8f6e384afdccd65247` — pause welcome motion unless
   the complete ten-row banner is inside the actual transcript viewport.

## Files Changed

The reviewed source delta changes only `experiments/opentui-agent-tui/**`:

- Runtime and presentation: `src/App.tsx`, `src/commands.ts`, `src/options.ts`,
  `src/main.tsx`, `src/motion.ts`, `src/useMotion.ts`, `src/brand.ts`, and the
  composer, operation-indicator, operation-scene, transcript, and welcome
  components.
- Removed: `src/useSpinnerFrame.ts`, superseded by the shared motion provider.
- Tests: app, brand, command, documentation, operation-scene, option, and
  spinner coverage.
- Documentation and attribution: `README.md` and `THIRD_PARTY_NOTICES.md`.

This review artifact is the only file outside the experiment added after the
approved source recheck.

No Go, TypeScript client, engine protocol, schema, resource catalog,
projection/redaction, fixture data, theme asset, lockfile, or workflow file
changed.

## Source Inputs Consulted

- Installed OpenTUI 0.4.3 source and declarations for renderer scheduling,
  ScrollBox viewport geometry, frame callbacks, timelines, post-processing,
  hit-grid behavior, and test clocks.
- Existing workspace operation lifecycle, cancellation, progress semantics,
  responsive transcript/rail layout, and picker-purpose routing.
- The prior animation deferral in
  `2026-07-15-opentui-agent-tui-experiment.md`, requiring delayed display,
  reduced-motion fallback, and a bounded frame rate.
- FIGlet 2.2.5 `poison.flf` from the trusted local Homebrew distribution and
  the pinned FIGlet.js 1.11.0 catalog.

## Generated Artifacts

No repository generator ran and no generated contract artifact changed.

`src/brand.ts` contains a one-time render from
`figlet -w 100 -f poison zscalerctl`. Blank outer rows and trailing padding
were removed. Its normalized ten-row display is pinned at SHA-256
`378583226322ee0c7923ca6f5280679189036aba65969c4f5bf45b453b934072`
and attributed in `THIRD_PARTY_NOTICES.md`. No FIGlet dependency or font file
is shipped.

## Expected Delta

- Startup option: `--motion full|reduced|off`, default `full`.
- Local command: `/motion [full|reduced|off|list]`; bare/list opens the
  existing floating picker. Valid motion controls remain available while an
  operation is active and never dispatch to the workspace adapter.
- At most one shared repeating timer. Full cadence is 120 ms, reduced cadence
  is 420 ms, and off creates no repeating timer.
- A 1.8-second welcome color sweep only while all ten Poison rows fit and are
  fully inside the actual transcript viewport. Hidden or partially clipped
  time does not consume the sweep; returning the complete banner starts a
  fresh duration unless keyboard interaction already dismissed startup motion.
- An ephemeral operation scene after a 320 ms delay, using only existing
  sanitized context fields and truthful completed/total progress. It is not a
  transcript entry or result snapshot.
- Existing spinner choice and every workspace/engine command remain unchanged.

## Invariants Claimed

- Motion is session-local presentation state, not adapter-owned context or
  historical result state.
- No motion command can reach `workspace.execute`.
- The scene is invisible for operations completing before 320 ms, disappears
  on success, cancellation, or error, and cannot be revived by late progress.
- Displayed counters mean completed work units; animation never invents
  elapsed progress or streamed records.
- Reduced mode retains slow liveness but freezes decorative travel. Off mode
  creates no repeating timer while retaining static labels and counters.
- Every animated sequence has fixed terminal-cell geometry. Compact progress
  remains one row with a visible completed/total separator through
  `Number.MAX_SAFE_INTEGER`.
- Banner motion changes spans/colors only. All ten 97-cell rows must be fully
  contained in the ScrollBox viewport before welcome motion owns a timer.
- Motion uses ordinary React renderables, not post-processing, so geometry and
  mouse hit targets cannot diverge.
- Adapter-owned transport, authority, and operation labels are sanitized again
  at the operation-scene boundary.
- No credentialed tenant access is needed or performed.

## Tests Run

- Final `cd experiments/opentui-agent-tui && bun run check`: 121 passed, one
  expected opt-in integration skip, zero failed, 2,764 assertions.
- Final real-process integration against a built Go engine: one passed, zero
  failed, 175 assertions; config-free only, with no credentials or tenant
  contact.
- Final `env -u GOFLAGS make check`: exit 0, including normal/race tests, vet,
  vulnerability and static analysis, secret and policy checks, machine
  contracts, experiment boundaries, generated-artifact checks, and skill sync.
- `make verify-experiment-boundaries verify-release-artifacts`: pass.
- `git diff --check`: pass.
- Frozen Bun install: no lockfile changes.

Focused rendering checks cover all ten exact Poison rows at 104 and 120
columns, transcript focus, absence at 103 and 121 columns, 41/42-row height
transitions, stale timer callbacks, long sticky-bottom content, partial row
clipping, fresh duration after hidden time, and compact operation renderables
at 12, 20, 40, and 51 columns in full and off modes.

## Known Deferrals

- The permanent long-transcript regression uses direct ScrollBox movement and
  initial long content. The final reviewer independently verified real mouse
  wheel input, mouse focus, post-mount sticky growth, banner unmount, and
  deduplicated publication, but those exact paths are not all committed as
  separate regression tests. This is an accepted non-blocking test-coverage
  nit, not an observed product defect.
- No global CRT, glitch, bloom, post-process, geometry-moving transition, or
  animation gallery is enabled.
- No persisted TUI motion preference, model transport, streamed record
  protocol, relationship graph, OpenAPI endpoint, shell execution, or tenant
  mutation is introduced.

## Review Focus

Reviewers were asked to attack timer ownership and stale callbacks, actual
ScrollBox visibility, renderer-live behavior, 97-cell and responsive geometry,
compact maximum-safe counters, operation lifecycle races, local-command
isolation, sanitized/truthful data, Poison provenance, and supported contract
or generated-artifact drift.

# Adversarial Review

## Reviewers

- Initial read-only reviewer: Codex subagent `Avicenna`, session
  `019f669b-532e-7cb3-ae8c-8112816ed96a`.
- Final fresh-context read-only reviewer: Codex subagent `Arendt`, session
  `019f6813-d00e-7e01-a651-5ced0b864b1f`.

Neither reviewer edited the worktree. The initial reviewer exhausted its
session usage during the last assertion-only recheck, so the complete final
diff was assigned to Arendt from a blank context rather than treating the
interrupted pass as approval.

## Initial Blocking Findings

Avicenna found two product defects in the initial candidate:

1. Layouts that selected the literal fallback still ran and consumed the
   welcome sweep clock because timer eligibility ignored whether 97 cells
   actually fit.
2. Large valid progress counters wrapped and collided with labels in the
   compact operation scene, changing its claimed one-row geometry.

The fixes made the exact banner branch and viewport dimensions part of timer
eligibility, and pre-budgeted compact label/counter fields inside one
non-wrapping renderable. Regression tests exercise both responsive breakpoints
and counters through `Number.MAX_SAFE_INTEGER`.

Avicenna's follow-up found two test-quality gaps: a slash assertion could be
satisfied by the `zia/locations` label, and a 92-cell banner prefix did not
prove all 97-cell rows. The tests were strengthened with a slash-free label,
explicit counter suffix, direct scene/text dimensions, every exact Poison row,
proven transcript focus, and retained stale callbacks.

## Final Fresh-Context Finding

Arendt reviewed the complete three-commit candidate and reproduced one further
product defect: a long sticky-bottom transcript could move the banner entirely
outside the viewport while width/height eligibility kept its interval and
timeout active.

The corrective delta
`61fb09ac2b6ad43fe129328dcc6114244e38bacc..46ae7bbd8b76030515fe5d8f6e384afdccd65247`
has SHA-256
`38165e9c0f6ed76562174246a1981100c5eb7495a70586e6803da2bd7d2d1fae`.
It identifies the ten-row banner renderable, measures full containment against
the actual ScrollBox viewport after layout, deduplicates visibility
publication, and requires that signal before welcome motion activates.

## Focused Recheck

Arendt independently verified at 104×42 with an 80-line transcript:

- initially hidden: zero intervals, zero timeouts, zero live requests;
- mouse wheel to top: all ten rows, one interval, one timeout, 1,800 ms left;
- mouse focus: all ten 97-cell rows remained contained with viewport padding;
- one clipped row: interval and timeout both cleared;
- after 1,700 ms hidden: restoration restarted the full 1,800 ms;
- post-mount sticky growth and banner unmount published hidden;
- forced renders produced no duplicate publication or live request;
- zero-height/deferred growth allocated no transient stale timer.

The reviewer also reran the responsive, stale-callback, motion-mode,
compact-counter, operation lifecycle, real-process, experiment-boundary,
release-artifact, diff-hygiene, and root checks. No blocker remained.

## Non-Blocking Risks

The committed viewport test does not permanently exercise every mouse,
post-mount growth, unmount, and publication-count path used in the independent
probe. Current behavior passed those probes. Adding each as a dedicated test is
a future hardening opportunity.

## Machine Contract Review

No supported Go CLI, engine protocol, TypeScript client, JSON/NDJSON behavior,
error envelope, exit code, dump/diff contract, schema, manifest,
introspection surface, generated CLI document, or release surface changed.
`/motion` remains local and never reaches the adapter.

## Safety Review

No redaction, projection, field coverage, credential, secret-provider, or
tenant-read-only behavior changed. Viewport visibility is local presentation
state and never enters context, transcript history, snapshots, or adapter
results. Existing sanitization and truthful-progress checks pass.

## Generated Artifact Review

No generated file, lockfile, dependency declaration, font asset, fixture, or
release artifact changed. Poison source, hash, attribution, and absence of a
runtime font dependency were independently checked.

## Verdict

Approve with nits.
