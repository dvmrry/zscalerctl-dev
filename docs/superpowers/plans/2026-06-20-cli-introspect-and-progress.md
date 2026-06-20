# CLI Payoff Features (introspect + spinners) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Realize the two dep-light payoffs the Cobra migration deferred — a machine-readable `introspect` command for agents, and TTY-gated progress spinners for long ops — with zero new runtime deps and zero machine-default surface change.

**Architecture:** Both build on the migrated Cobra tree. `introspect` walks the live `*cobra.Command` tree via a shared `introspectTree` function (also consumed by the existing `gen-cli-docs`, so docs + the agent map can't drift) and emits a JSON surface map; it's a config-free Cobra command routed through the existing `isMigrated`/`execCobra` hybrid. Spinners are a dep-free `internal/output.Spinner` written to stderr, gated off unless interactive, wired into `dump` (determinate) and single-call reads (indeterminate).

**Tech Stack:** Go, `spf13/cobra`+`pflag` (already vendored), stdlib only (no new deps). Spec: `docs/superpowers/specs/2026-06-20-cli-introspect-and-progress-design.md`.

**Conventions:** No `Co-Authored-By`. TDD; frequent commits. Run `make fmt-check` + `staticcheck ./...` before each commit (CI runs both — a clean local `go test` is NOT sufficient, per the migration's CI lessons). Work on branch `cli-introspect-progress`; push to `dvmrry/zscalerctl-dev` + open a PR per phase so real Linux/Windows CI validates (local-green ≠ CI-green).

**Per-phase gate (before each phase PR):**
- [ ] `go test -mod=vendor ./...` green; `go test -race -mod=vendor ./internal/cli/ ./cmd/zscalerctl/` clean.
- [ ] `staticcheck ./...` clean; `go vet`/`make fmt-check` clean; `GOOS=windows go vet -mod=vendor ./...` compiles.
- [ ] `make check` green (incl docs-cli-check, secret-scan, verify-licenses, vuln).
- [ ] Golden boundary snapshot: deltas reviewed + re-blessed via `surface_changes.md`; wantCodes asserted in Go.
- [ ] Push to `zscalerctl-dev`; PR; real CI (all jobs incl windows-config + race shards) green before merge.

---

## Phase 1 — `introspect`

### Task 1.1: Shared `introspectTree` walk + doc types

**Files:** Create `internal/cli/introspect.go`, `internal/cli/introspect_test.go`. Modify `scripts/gen-cli-docs.go` (consume the shared walk).

- [ ] **Step 1 — Failing test.** In `introspect_test.go`, build the tree and assert the doc's shape:
```go
func TestIntrospectTree(t *testing.T) {
	a := New(io.Discard, io.Discard, nil)
	doc := a.introspectTree() // walks BuildCommandTree(a)
	if doc.IntrospectVersion != "1" { t.Fatalf("version = %q", doc.IntrospectVersion) }
	if !doc.ReadOnly { t.Fatal("read_only must be true") }
	// every command is non-mutating today
	for _, c := range doc.Commands {
		if c.Mutating { t.Fatalf("command %q marked mutating in a read-only CLI", c.Path) }
	}
	// a known command exists with split flags
	loc := findCmd(doc, "zia locations list")
	if loc == nil { t.Fatal("missing zia locations list") }
	if len(loc.OutputFields) == 0 { t.Errorf("list command should expose output_fields") }
	if !contains(loc.InheritedFlags, "format") { t.Errorf("globals should appear as inherited_flags") }
	// global_flags described once, from globalFlagDefs (13)
	if len(doc.GlobalFlags) != len(globalFlagDefs) { t.Fatalf("global_flags = %d want %d", len(doc.GlobalFlags), len(globalFlagDefs)) }
	// exit codes catalog present (0..7)
	if len(doc.ExitCodes) == 0 { t.Fatal("exit_codes catalog missing") }
}
```
- [ ] **Step 2.** Run `go test -mod=vendor ./internal/cli/ -run TestIntrospectTree` → FAIL (undefined).
- [ ] **Step 3 — Types + walk.** In `introspect.go` define the doc types matching spec §3a: `IntrospectDoc{Schema, IntrospectVersion, CLIVersion, ReadOnly, GlobalFlags []FlagDoc, Commands []CommandDoc, Catalog CatalogDoc, ExitCodes []ExitCodeDoc}`; `CommandDoc{Path, Short, Long, Aliases, Hidden, Deprecated, Mutating, Args ArgsDoc, Flags []FlagDoc, InheritedFlags []string, OutputFields []string}`; `FlagDoc{Name, Shorthand, Type, Default, Required, Usage, Enum}`; `ArgsDoc{Policy, N, ValidValues}`; `CatalogDoc{Products []string, Resources []ResourceDoc}`; `ExitCodeDoc{Code int, Kind, Retryable bool, Description}`. All JSON-tagged + `OutputSafe()` markers as needed. Implement `(a *App) introspectTree() IntrospectDoc`: walk `BuildCommandTree(a)` recursively; per command use `cmd.CommandPath()` (strip the root name → space-joined path), `cmd.Short/Long/Aliases/Hidden/Deprecated`, `cmd.Annotations["introspect/mutating"]=="true"` for `Mutating`, `cmd.NonInheritedFlags()` → `Flags`, `cmd.InheritedFlags()` → `InheritedFlags` (names), flag `Type` from `pflag.Flag.Value.Type()`, `Enum` from the `globalFlagDefs` completion values / registered completions where known. `GlobalFlags` from `globalFlagDefs`. `OutputFields` for product read commands from the catalog spec's projected field names. `Catalog` from `resources.Catalog()`. `ExitCodes` a static table mirroring `main.go:exitCodeForError`/`errorKind` (0 ok,1 internal,2 usage,3 missing_credentials,4 not_found,5 live_access_failed,6 partial_dump,7 drift_detected) — keep it a hand-maintained table in `introspect.go` with a comment pointing at `main.go` (it is the documented contract).
- [ ] **Step 4.** Run the test → PASS.
- [ ] **Step 5 — DRY refactor.** Refactor `scripts/gen-cli-docs.go` to render markdown FROM `introspectTree()` (or a shared lower-level walk both call), so the doc generator and the runtime command share one enumeration. Run `bash scripts/verify-cli-docs.sh` → still passes (regenerate `docs/cli/` if the shared walk changes ordering; review the diff is benign).
- [ ] **Step 6 — Commit.** `feat(cli): shared introspectTree walk (commands+flags+catalog+exit-codes)`.

### Task 1.2: `newIntrospectCmd` + dispatch wiring

**Files:** Modify `internal/cli/introspect.go` (add the command), `internal/cli/app.go` (`isMigrated`, `execCobra` registration). Test: `internal/cli/introspect_test.go`.

- [ ] **Step 1 — Failing test.** `introspect` via `App.Run` emits valid JSON with the expected top-level keys; `--format ndjson introspect` → ErrUsage; runs config-free (no creds):
```go
func TestIntrospectCommand(t *testing.T) {
	var out bytes.Buffer
	a := New(&out, io.Discard, nil) // no env/creds
	if err := a.Run(context.Background(), []string{"introspect"}); err != nil { t.Fatal(err) }
	var doc map[string]any
	if err := json.Unmarshal(out.Bytes(), &doc); err != nil { t.Fatalf("not JSON: %v", err) }
	for _, k := range []string{"introspect_version","read_only","commands","global_flags","catalog","exit_codes"} {
		if _, ok := doc[k]; !ok { t.Errorf("missing key %q", k) }
	}
}
func TestIntrospectRejectsNDJSON(t *testing.T) { /* --format ndjson introspect → errors.Is(err, ErrUsage) */ }
```
- [ ] **Step 2.** Run → FAIL.
- [ ] **Step 3 — Implement.** `newIntrospectCmd(opts globalOptions) *cobra.Command` (Use `"introspect"`, neutral Short, config-free RunE). RunE: reject ndjson (`rejectUnsupportedFormat("introspect", opts.format)`); set `doc.CLIVersion = version.Current().Version`; render: `json` → `output.NewRenderer(redact.New(redact.ModeStandard)).WriteJSON(a.out, doc)`; `table`/`pretty` → a human tree text via `a.renderer`-style ModeStandard writer; default = json (machine-first). Add `"introspect"` to `isMigrated`; `execCobra` `root.AddCommand(a.newIntrospectCmd(opts))`. Do NOT call `LoadConfig`/construct a reader.
- [ ] **Step 4.** Run → PASS; full `go test ./internal/cli/...` green.
- [ ] **Step 5 — Commit.** `feat(cli): introspect command (config-free JSON surface map)`.

### Task 1.3: Published schema + drift + DRY gates

**Files:** Create `docs/schema/introspect.schema.json`, `scripts/verify-introspect-schema.sh`, `scripts/test-verify-introspect-schema.sh`. Modify `Makefile`, `.github/workflows/ci.yml`, `docs/SCRIPTS.md`.

- [ ] **Step 1.** Hand-author `docs/schema/introspect.schema.json` (JSON Schema draft-07, mirroring the existing `config.schema.json` style) describing the IntrospectDoc shape (required: introspect_version, read_only, commands, global_flags, catalog, exit_codes; per-command + per-flag shapes). Point the runtime `$schema` at its raw GitHub URL.
- [ ] **Step 2 — Drift gate test (RED).** Add `internal/cli/introspect_test.go::TestIntrospectMatchesPublishedSchema` validating `introspectTree()` JSON against `docs/schema/introspect.schema.json` (use the same JSON-schema validator the config-schema test uses — check `internal/config` tests for the helper). Run → adjust schema/doc until PASS.
- [ ] **Step 3 — DRY gate.** Add `TestIntrospectAndDocsAgree` asserting the command paths in `introspectTree()` equal the set `gen-cli-docs` emits (both from the shared walk). Run → PASS.
- [ ] **Step 4 — verifier + self-test + CI.** `scripts/verify-introspect-schema.sh` (regenerate runtime introspect → validate against the committed schema → fail on drift) + `scripts/test-verify-introspect-schema.sh` (pass + corrupt-fail cases), mirroring the `verify-cli-docs` pair. Wire both into the `Makefile` `check` aggregate and the `.github/workflows/ci.yml` `verify-gates` job. Register in `docs/SCRIPTS.md`. Run `make verify-script-registry` → PASS.
- [ ] **Step 5 — Commit.** `feat(docs): publish introspect.schema.json + drift/DRY gates`.

### Task 1.4: Golden + config-free no-leak coverage

**Files:** Modify `cmd/zscalerctl/golden_surface_test.go` (+ testdata goldens).

- [ ] **Step 1.** Add golden cases `introspect` (json, wantCode 0) and `introspect-pretty` (`["introspect","--pretty"]` or `--format pretty`, wantCode 0). The scrubber must scrub `cli_version` (pseudo-version) + the `$schema` SHA if any — confirm the existing version/pseudo-version scrub covers it; extend if needed. `--format ndjson introspect` → add to `TestParseErrorsExitTwo` (wantCode 2).
- [ ] **Step 2.** `go test ./cmd/zscalerctl/ -run TestGoldenSurface -update`; **open every new golden** and confirm zero tenant data — only command/flag names, descriptions, catalog names, exit-code text. Re-run without `-update` → PASS. Record in `surface_changes.md` (new `introspect`/`introspect-pretty` cases, category new-surface).
- [ ] **Step 3 — config-free proof.** `internal/cli` test: run `introspect` with empty env + a `ResourceReader` stub that `t.Fatal`s if any method is called → must succeed with the full map (proves no reader/LoadConfig/network).
- [ ] **Step 4 — Commit.** `test(cli): freeze introspect golden + prove config-free`.

### Task 1.5: AGENTS.md signpost + generated doc page

**Files:** Modify `AGENTS.md`, `skills/zscalerctl/SKILL.md`; regenerate `docs/cli/`.

- [ ] **Step 1.** Add an AGENTS.md line (in the existing CLI-reference section): "Run `zscalerctl introspect` first for the machine-readable command + catalog map (JSON; `read_only`, flags, args, output_fields, exit codes)." Mirror a pointer into `skills/zscalerctl/SKILL.md`; re-sync the agents skill (`sync-agents-skill.sh` if present).
- [ ] **Step 2.** Regenerate `docs/cli/` (now includes the `introspect` page); `bash scripts/verify-cli-docs.sh` + `make verify-agents-skill` → PASS.
- [ ] **Step 3 — Commit.** `docs: AGENTS.md points agents at introspect; generated page`.

### Task 1.6: Phase-1 gate + PR
- [ ] Run the full per-phase gate (above). Push `cli-introspect-progress` to `zscalerctl-dev`; open a PR (label `semver:minor`); confirm ALL CI jobs green (windows-config + race shards + static-analysis + secret-scan). Adversarial pass, then your call to merge.

---

## Phase 2 — progress spinners

### Task 2.1: `Spinner` + `stderrTTY` + three-gate activation

**Files:** Create `internal/output/spinner.go`, `internal/output/spinner_test.go`. Modify `internal/cli/app.go` (`App` struct: add `stderrTTY bool`; set in `New`).

- [ ] **Step 1 — Failing tests.** A `Spinner` is a no-op (zero bytes) when inactive, and writes/clears when active:
```go
func TestSpinnerInactiveWritesNothing(t *testing.T) {
	var buf bytes.Buffer
	s := output.NewSpinner(&buf, false) // active=false
	s.Start("contacting Zscaler"); s.Update("zia/locations"); s.Stop()
	if buf.Len() != 0 { t.Fatalf("inactive spinner wrote %q", buf.String()) }
}
func TestSpinnerActiveWritesAndClears(t *testing.T) {
	var buf bytes.Buffer
	s := output.NewSpinner(&buf, true)
	s.Start("working"); s.Stop()
	out := buf.String()
	if !strings.Contains(out, "\r") { t.Fatal("expected carriage-return overwrite") }
	if !strings.HasSuffix(out, "\r"+strings.Repeat(" ", len("⠋ working"))+"\r") && !strings.HasSuffix(out, "\r") { /* line cleared */ }
}
```
- [ ] **Step 2.** Run → FAIL.
- [ ] **Step 3 — Implement.** `NewSpinner(w io.Writer, active bool) *Spinner`; braille frames `[]rune("⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏")`; `Start(text)`, `Update(text)`, `Stop()`. When `active==false`, all methods return immediately (no write). When active, render `\r<frame> <text>` (a background goroutine ticking ~100ms via `time.Ticker`, OR a synchronous redraw on Update — choose the goroutine approach with a `done` channel + mutex; `Stop` signals done, joins, clears the line `\r`+spaces+`\r`). Keep it small + race-free (the `-race` gate will check).
- [ ] **Step 4.** Run `go test -race ./internal/output/ -run Spinner` → PASS (no race).
- [ ] **Step 5 — App wiring.** In `internal/cli/app.go` add `stderrTTY bool` to `App`; in `New`, set `stderrTTY = output.IsTerminal(stderr)` from the raw stderr (BEFORE any wrapping), mirroring how `stdoutTTY` is captured. Add a helper `(a *App) newSpinner(opts globalOptions) *output.Spinner` returning `output.NewSpinner(a.err, a.spinnerActive(opts))` where `spinnerActive = a.stderrTTY && opts.logLevel=="off" && opts.colorMode != output.ColorNever`. Add a `TestSpinnerActiveGating` table test asserting all three gates (each false → inactive).
- [ ] **Step 6 — Commit.** `feat(output): dep-free braille Spinner + three-gate activation`.

### Task 2.2: `dump` determinate progress

**Files:** Modify `internal/cli/app.go` (`collectDump` + `runDumpWithOptions`). Test: `internal/cli/cobra_dump_test.go`.

- [ ] **Step 1 — Failing test.** With an active spinner sink (a buffer + active=true via a test seam) a multi-resource dump emits per-resource progress containing the resource names, and NOTHING on stdout; with inactive, zero progress bytes. (Inject the spinner or assert via a recording sink.)
- [ ] **Step 2.** Run → FAIL.
- [ ] **Step 3 — Implement.** Give `collectDump` an optional `progress func(done, total int, product, resource string)` parameter (nil-safe). `runDumpWithOptions` builds the spinner (`a.newSpinner(opts)`), computes `total` from the resolved selection, and passes a callback that calls `spinner.Update(fmt.Sprintf("[%d/%d] %s/%s", done, total, product, resource))`. `spinner.Start("dumping")` before the loop; `spinner.Stop()` (clears the line) before the existing stderr status line. Progress text is catalog names only — no record data.
- [ ] **Step 4.** Run → PASS; existing dump tests + goldens unchanged (machine mode = inactive spinner = no output).
- [ ] **Step 5 — Commit.** `feat(cli): determinate dump progress (stderr, gated)`.

### Task 2.3: single-call indeterminate spinner

**Files:** Modify `internal/cli/app.go` (`runProduct` read paths) + `internal/cli/url_lookup.go` (`runURLLookup`) + the doctor path. Test: existing per-command tests.

- [ ] **Step 1 — Failing test.** A `list`/`get`/`show` (and `url-lookup`, `doctor`) with an active spinner emits an in-flight indeterminate frame to the spinner sink during the reader call and clears it before the projected output; stdout carries ONLY the projected data.
- [ ] **Step 2.** Run → FAIL.
- [ ] **Step 3 — Implement.** Wrap the reader call (`reader.List/Get/Show`, `lookupReader.URLLookup`, doctor's status build) in `s := a.newSpinner(opts); s.Start("contacting Zscaler"); defer s.Stop()` so the frame shows only while the API call is in flight and is cleared before rendering. Ensure `Stop()` happens BEFORE any write to `a.out` (data) or the error path.
- [ ] **Step 4.** Run → PASS; goldens/behavior tests unchanged (inactive in tests/machine mode).
- [ ] **Step 5 — Commit.** `feat(cli): in-flight spinner for single-call reads (gated)`.

### Task 2.4: gating + no-leak tests

**Files:** `internal/cli/*_test.go`.

- [ ] **Step 1.** Add explicit tests: (a) dump/list with `stderrTTY=false` (default in tests) → zero spinner bytes; (b) with `--log-level info` → zero spinner bytes (gate); (c) with `--color never` → zero spinner bytes (gate); (d) no-leak: drive an active spinner through a dump with a fake reader returning records containing secret-shaped values, assert the spinner sink contains ONLY catalog names + literals, never a record value.
- [ ] **Step 2.** Run → PASS. `go test -race` on `internal/cli` + `internal/output` clean.
- [ ] **Step 3 — Commit.** `test(cli): spinner gating + no-leak coverage`.

### Task 2.5: Phase-2 gate + PR
- [ ] Full per-phase gate; push; PR (`semver:minor`); all CI green; adversarial pass; merge.

---

## Self-review

**Spec coverage:** §3 introspect → Tasks 1.1–1.5 (tree-walk + command + schema/gates + golden + AGENTS); §3a shape (read_only/mutating/flag-split/output_fields/exit_codes/global_flags) → Task 1.1 types + 1.3 schema; §3b dep-free+DRY → 1.1 Step 5 + 1.3 DRY gate; §3c schema+signpost → 1.3 + 1.5; §4 spinners → Tasks 2.1–2.4 (Spinner+gating, dump determinate, single-call indeterminate, no-leak); §5 research basis → reflected in field choices; §6 verification → the gates + golden + config-free + gating/no-leak tests; §2 constraints (machine-first, no-leak, minimal-dep, boundary) enforced per task. All covered.

**Placeholders:** test bodies are sketches to be completed during TDD against the real symbols; every task names exact files, the real codebase symbols (`BuildCommandTree`, `globalFlagDefs`, `isMigrated`/`execCobra`, `collectDump`, `runDumpWithOptions`, `output.IsTerminal`, `rejectUnsupportedFormat`, `redact.ModeStandard`), and the gate. No vague "add error handling".

**Type consistency:** `introspectTree() IntrospectDoc`, `IntrospectDoc`/`CommandDoc`/`FlagDoc`/`ArgsDoc`/`CatalogDoc`/`ExitCodeDoc`, `NewSpinner(io.Writer, bool) *Spinner` with `Start/Update/Stop`, `a.stderrTTY`, `a.newSpinner(opts)`, `a.spinnerActive(opts)` — used consistently across tasks.
