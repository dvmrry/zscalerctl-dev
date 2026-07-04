# Roadmap Execution Briefs

Companion to [ROADMAP.md](ROADMAP.md). This file exists because implementation
agents (Codex or otherwise) drift when handed broad context. The protocol and
briefs below are the countermeasure. Each brief is self-contained: it can be
pasted cold into a fresh agent session, or driven by a coordinator.

## Driving protocol

1. **One brief = one PR = one fresh agent session.** Never hand an agent the
   whole roadmap or a prior session's transcript. The brief is the entire
   task context.
2. **Mechanical done-criteria only.** An agent's claim of success is not
   evidence. The coordinator runs the listed gates after the agent stops and
   trusts only their output.
3. **Drift rule:** if the agent touches files outside the brief's list,
   modifies any machine surface (JSON/NDJSON output, stderr envelope, exit
   codes, schemas, completion, introspection, generated CLI docs) when the
   brief doesn't authorize it, or starts refactoring adjacent code — kill the
   session and restart with the same brief. Do not steer it back mid-session;
   that is how scope creep compounds.
4. **Golden discipline:** any golden change must be paired with a
   `cmd/zscalerctl/testdata/surface/surface_changes.md` entry in the same
   change (the verifier enforces this; agents forget it).
5. **Review pass before merge:** run an adversarial review (`/code-review` or
   a second agent) on the diff; the implementing agent never reviews itself.
6. **Sequencing:** merge in review order on the 0.x line — VERSIONING.md
   allows breaking changes in 0.x minors, and dev `main` keeps shipping minors
   until the work runs dry (owner decision 2026-07-04). The deliberate public
   promotion, whenever chosen, selects a validated baseline commit; it does
   not gate dev merges.

Universal constraints for every brief (paste into every agent prompt):

> Work only in the files listed. Do not modify JSON/NDJSON output, stderr
> error envelopes, exit codes, schemas, completion, introspection, or
> generated CLI docs unless the brief explicitly says so. Do not add
> dependencies. Do not touch `vendor/`. Run `gofmt`. All listed gates must
> pass. Label the PR with the stated semver label. No Co-Authored-By lines.

## Brief P3.1 — pretty/table golden expansion (test-only)

**Context:** zscalerctl freezes its CLI surface with process-boundary golden
tests (`cmd/zscalerctl/golden_surface_test.go`). Only `doctor --format pretty`
currently pins pretty output. Before restyling work begins, representative
pretty/table cases must be baselined.

**Task:** add golden-surface cases (same table-driven pattern as existing
cases) for: `zia locations list --format pretty`, `zia locations list
--format table`, a `get` in pretty (pick an existing offline-fixture-backed
case to mirror), a singleton `show` in pretty, and `schema list --format
pretty`. Use the existing fake-reader/offline fixtures already used by
neighboring cases — no live credentials. Regenerate goldens with
`go test ./cmd/zscalerctl/... -run TestGoldenSurface -update`, review the new
files by eye for redaction correctness, and add one `surface_changes.md`
entry (category: golden-added) describing the additions.

**Files:** `cmd/zscalerctl/golden_surface_test.go`,
`cmd/zscalerctl/testdata/surface/*` (new goldens),
`cmd/zscalerctl/testdata/surface/surface_changes.md`.

**Gates:** `go test -mod=vendor ./cmd/zscalerctl/...`,
`bash scripts/verify-surface-changes-manifest.sh`, `make fmt-check`.

**Semver:** `semver:none`.

## Brief P2.1 — url-lookup behind runtime + boundary rule

**Context:** `internal/cli` must stop importing `internal/zscaler`. The only
remaining use is `zia url-lookup`: `internal/cli/url_lookup.go` type-asserts a
`URLLookupReader` and `internal/cli/app.go` has an ~8-line `resourceReader()`
delegating to `machineruntime.NewReaderFromConfig`. All other reads go through
`internal/runtime` (`Machine`, `DumpCollector`).

**Task:** (1) add a URL-lookup entry point to `internal/runtime` following the
existing construction pattern (`New...FromConfig(ctx, cfg, opts)` plus a
`...FromReader` test constructor, mirroring `DumpCollector`); it wraps reader
construction and exposes only the lookup capability. (2) Rewire
`internal/cli/url_lookup.go` to consume it; delete `resourceReader` and the
`internal/zscaler` import from `internal/cli`. (3) Add
`github.com/dvmrry/zscalerctl/internal/zscaler` to a new `internal/cli` check
in `scripts/verify-core-boundaries.sh` (follow the existing `check_package`
pattern; forbid zscaler only — cli legitimately imports config/output/etc.).
(4) Keep the url-lookup error mapping and output byte-identical: existing
golden and unit tests must pass unchanged.

**Files:** `internal/runtime/` (new file, e.g. `urllookup.go` + test),
`internal/cli/url_lookup.go`, `internal/cli/app.go`,
`scripts/verify-core-boundaries.sh`, `scripts/test-verify-core-boundaries.sh`.

**Gates:** `go test -mod=vendor ./internal/cli/... ./internal/runtime/...
./cmd/zscalerctl/...`, `bash scripts/verify-core-boundaries.sh`,
`bash scripts/test-verify-core-boundaries.sh`, `make fmt-check`. Goldens must
not change.

**Semver:** `semver:patch`.

## Brief P2.2 — error-currency cleanup (paired change; do not split)

**Context:** `runtimeErrorFromMachineExecution`
(`internal/runtime/runtime.go:300`) substitutes the raw load error for the
sanitized `MachineError` when the kind is `live_access_failed`, so
`cmd/zscalerctl/main.go` can map it to exit 5 via
`errors.Is(err, zscaler.ErrLiveAccessFailed)`. **Hand-traced 2026-07-03:
deleting the substitution without a paired exit-code mapping regresses live
failures from exit 5 to exit 1.** The two changes must land together.

**Task:** (1) in `cmd/zscalerctl/main.go` `exitCodeForError`, add
`case machine.ErrorKindLiveAccessFailed: return exitLiveAccessFailure` to the
existing `MachineError` kind switch. (2) Delete the substitution:
`Machine.Execute` returns the executor's error unchanged; remove the
now-unused `loader.err` capture if nothing else uses it. (3) Update/extend
tests: `cmd/zscalerctl/main_test.go` pins exit 5 + envelope kind
`live_access_failed` for a machine-path live failure;
`internal/runtime/runtime_test.go` expectations that relied on the raw error
being returned must be updated to expect `*machine.MachineError`. (4) Envelope
`message` text for live failures will change from the adapter-normalized text
to the sanitized machine message — this is declared non-stable; confirm no
golden pins it (offline goldens cannot produce live failures).

**Files:** `internal/runtime/runtime.go`, `internal/runtime/runtime_test.go`,
`cmd/zscalerctl/main.go`, `cmd/zscalerctl/main_test.go`.

**Gates:** `go test -mod=vendor ./internal/runtime/... ./internal/cli/...
./cmd/zscalerctl/...`, `bash scripts/verify-machine-contract.sh`,
`make fmt-check`. Goldens must not change.

**Semver:** `semver:minor`.

## Brief P2.3 — error-vocabulary mapping table (docs only)

**Context:** five overlapping error vocabularies exist by design: process exit
codes (0–7), stderr envelope `kind` strings (enumerated in
`docs/schema/error.schema.json`), machine `MachineError.Kind`, internal Go
sentinels, and schema docs. Adapter authors need one table so no sixth
vocabulary appears.

**Task:** add a "Error vocabulary map" section to
`docs/cli/machine-contract.md`: one table with columns — scenario, machine
kind (if any), envelope kind, exit code, Go sentinel (internal reference),
notes. Cover: usage errors, unknown/unsupported resource, not_found,
missing credentials, invalid resource id, live access failure, deadline
exceeded, canceled, partial dump, drift detected, invalid config/proxy,
internal. Source of truth is code: `cmd/zscalerctl/main.go`
(`errorKind`, `exitCodeForError`) and `internal/machine/executor.go` (kind
constants). Verify every row against code before writing it. Do not change
any code or schema.

**Files:** `docs/cli/machine-contract.md`.

**Gates:** `bash scripts/verify-docs.sh`, `make docs-cli-check` (should be
unaffected), row-by-row spot check against `main.go`/`executor.go` by the
coordinator.

**Semver:** `semver:none`.

## Brief P3.1b — run() reader seam (follow-up to PR #93; land after it merges)

**Context:** PR #93's fixture helper (`runWithGoldenSurfaceFixture` in
`cmd/zscalerctl/golden_surface_test.go`) duplicates the error-handling tail of
`main.go run()` because `run(ctx, args, stdout, stderr, env)` has no reader
injection point. The goldens currently pin a copy of that logic.

**Task:** extract the body of `run()` after app construction into an
unexported `runApp(ctx, app *cli.App, args, stdout, stderr)` (exact split:
everything from `app.Run` through error writing/exit-code mapping); `run()`
builds the default app and delegates. Replace the duplicated logic in the test
helper with a `runApp` call (test file is package `main`). Behavior and all
goldens byte-identical.

**Files:** `cmd/zscalerctl/main.go`, `cmd/zscalerctl/golden_surface_test.go`.

**Gates:** `go test -mod=vendor ./cmd/zscalerctl/...`, `make fmt-check`;
`git status` shows no golden changes.

**Semver:** `semver:patch`. Hold merge until after the public 1.0 tag.

## Brief P3.2 — Lip Gloss v2 migration (after #93 merges)

**Context:** `internal/output/pretty.go` uses lipgloss v1.1.0 with a pinned
color profile (`prettyRenderer`) to defeat v1's environment auto-detection.
Lip Gloss v2 is stable and makes manual color handling the default.

**Task:** migrate `internal/output` to `github.com/charmbracelet/lipgloss/v2`
per its upgrade guide. Preserve exactly: profile pinned from the resolved
`Style` (no environment probing, no terminal queries), byte-clean output when
color is off, all existing golden files unchanged. Update `go.mod`/`go.sum`
and `vendor/` (`go mod vendor`). Remove the v1 dependency if nothing else
uses it.

**Files:** `internal/output/*.go`, `go.mod`, `go.sum`, `vendor/`.

**Gates:** `make check` (full), `bash scripts/verify-pty-escape-clean.sh`,
goldens byte-identical (`git status` clean under `cmd/zscalerctl/testdata`).

**Semver:** `semver:patch`. Hold merge until after the public 1.0 tag.

## Brief P3.3 — styled help via owned HelpFunc (after P3.2)

**Context:** Help output is plain Cobra text, golden-frozen. Fang was
rejected; styled help is implemented in-repo on Lip Gloss v2.

**Task:** add a custom `HelpFunc`/`UsageFunc` set on the root command in
`internal/cli`, rendering through `cmd.OutOrStdout()` (so redacting writers
stay in the loop). Styled only when color is enabled per the existing
resolved color policy; with color off, output may be restructured but must be
plain bytes (no ANSI). Regenerate help goldens with `-update`; add
`surface_changes.md` entries (category help-restyle) for every changed case.
Confirm `make docs-cli-check` still passes (generated docs read Cobra
metadata, not rendered help) — if it fails, stop and report rather than
regenerating docs.

**Files:** `internal/cli/` (new `help.go` + wiring), `internal/output/` (shared
styles if needed), `cmd/zscalerctl/testdata/surface/*` (help goldens),
`surface_changes.md`.

**Gates:** `go test -mod=vendor ./internal/cli/... ./cmd/zscalerctl/...`,
`bash scripts/verify-surface-changes-manifest.sh`,
`bash scripts/verify-pty-escape-clean.sh`, `make docs-cli-check`,
`make fmt-check`.

**Semver:** `semver:minor`. Hold merge until after the public 1.0 tag.

## Brief P2.4 — app.go split (mechanical; may be 2–3 stacked PRs)

**Context:** `internal/cli/app.go` is ~2.6k lines mixing orchestration,
command construction, rendering, and dump/diff wiring. Split by
responsibility with zero behavior change.

**Task:** move code (no rewrites, no renames beyond file placement) into:
`commands_*.go` (Cobra command construction), `render_*.go` (human rendering
helpers), `errors.go` (CLI error types/mapping), `format_policy.go`
(format/NDJSON gating), `dump_diff.go` (dump/diff orchestration), keeping
`app.go` as App.Run + dispatch. One responsibility per PR if split. The diff
for each PR must be pure moves verifiable by `git diff --color-moved`.

**Files:** `internal/cli/*.go` only. No test file changes except mirrored
moves.

**Gates:** full `make check` (this touches everything); goldens byte-identical;
`git diff --color-moved=dimmed-zebra` shows moves, not edits.

**Semver:** `semver:none`.
