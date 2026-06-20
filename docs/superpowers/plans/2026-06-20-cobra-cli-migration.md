# Cobra CLI Migration Implementation Plan (revised v2.1)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Replace `zscalerctl`'s hand-rolled dispatch with a Cobra command tree, surface-preserving + bounded inline wins, with zero no-leak / exit-code / global-flag regression.

**Architecture (corrected — see spec v2):** `cmd/zscalerctl/main.go:run()` (mute, panic-recovery, raw-arg `errorFormat`, 12-case `exitCodeForError`, partial-dump suppression, redacting `writeError`) **stays unchanged**. `App.Run(ctx,args) error` keeps its signature; it parses globals via the existing `parseGlobal` (canonical), then routes migrated commands through a Cobra root (`SilenceErrors`/`SilenceUsage`, returning `UsageError`-wrapped sentinels) and un-migrated commands through the existing `runParsed`. Cobra owns the command tree + per-command local flags + help/completion/doc generation; globals are mirrored as persistent flags for help/completion/introspection only (parsed by `parseGlobal`, not Cobra).

**v2.1 refinements (Codex + Gemini validation — see spec v2.1):** `redact.NewWriter` **full-buffers** (Task 0.2, not line-buffer) and completion scripts bypass it; the §5 hybrid **re-inserts `--help`/`--`** that `splitGlobalArgs` strips and gates the legacy help early-return to un-migrated commands; **`--output` is preserved from Phase 1** (the `App.Run` wrapper wraps all output — add `version --output` tests in Phase 1, not Phase 4); **ban `MarkFlagRequired`/flag-groups** (validate in `RunE` → `UsageError`), custom `Args` funcs wrap `UsageError`, unknown-command is intercepted post-`Execute`, and `ValidArgsFunc` uses `ShellCompDirectiveError`; Phase 5 routes Cobra's `__complete` commands as config-free; the golden harness splits into binary-boundary goldens + in-process fake-reader behavior tests; §5b/§5c tables now include `dump` (config redaction) and `config init` (config-free/text). (Gemini confirmed Phase 3's `--`-after-local-flags already matches Cobra's default — no change there.)

**Tech Stack:** Go, `spf13/cobra`+`pflag` (pinned), existing `internal/redact`/`config`/`zscaler`/`output`/catalog.

---

## Reference & conventions

Spec: `docs/superpowers/specs/2026-06-20-cobra-cli-migration-design.md` (v2). **No `Co-Authored-By`.** Spike in public `dvmrry/zscalerctl-dev`; clean squash-merged PRs to `main`. Value-free fixtures. TDD; frequent commits.

**Per-phase gate (before each phase PR):**
- [ ] `go test -mod=vendor ./...` green; the ~115 existing `App.Run` behavior tests pass unchanged.
- [ ] Golden boundary snapshot: every delta reviewed + re-blessed as intentional (via the `surface_changes` manifest); exit codes asserted in Go, never `-update`-blessed.
- [ ] Agentic-coverage eval (DAV-10) ≥ floor.
- [ ] `make check` green.
- [ ] The **corrected** `windows-config` CI job (now covering `internal/cli`) green; `CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build ./...` green.
- [ ] PR vs `main`, `semver:minor`, draft until the gate passes.

---

## Phase 0 — Sandbox + `redact.NewWriter` + real-boundary golden harness

### Task 0.1: Spike sandbox
- [ ] `gh repo fork dvmrry/zscalerctl --fork-name zscalerctl-dev --clone=false` (fallback: `gh repo create dvmrry/zscalerctl-dev --public --source=. --push`). Verify `gh repo view dvmrry/zscalerctl-dev`. Throwaway; deleted at 1.0.

### Task 0.2: Build `redact.NewWriter` (required by §5a — does not exist yet)
**Files:** Create `internal/redact/writer.go`, `internal/redact/writer_test.go`.
- [ ] **Step 1: Failing test** — a writer that redacts across write boundaries:
```go
func TestRedactingWriterRedactsAcrossWrites(t *testing.T) {
	var buf bytes.Buffer
	w := redact.NewWriter(&buf, redact.ModeStandard)
	io.WriteString(w, "token=AKIA")          // pattern split across writes
	io.WriteString(w, "abcdef1234567890XYZ\n")
	w.(io.Closer).Close()                    // flush
	if strings.Contains(buf.String(), "AKIAabcdef1234567890XYZ") {
		t.Fatalf("secret leaked across write boundary: %q", buf.String())
	}
}
```
- [ ] **Step 2:** Run → FAIL (`NewWriter` undefined).
- [ ] **Step 3:** Implement `NewWriter(io.Writer, Mode) io.WriteCloser` that **full-buffers** (accumulates ALL writes) and redacts the whole buffer via `Redactor.ScanRenderedString` on `Close` — NOT line-by-line (line buffering misses *multi-line* secrets like private keys, and per-write splitting breaks patterns). The §5 handler `defer w.Close()`s around `Execute` so a final partial line flushes (Cobra never closes its writers). Completion-script generation does NOT use this writer (its bytes must stay valid shell — §5a).
- [ ] **Step 4:** Run → PASS. Also test: a non-secret line passes through unchanged; a **multi-line** secret (a PEM `-----BEGIN PRIVATE KEY-----`…`-----END…` block) is redacted; and a final write with **no trailing `\n`** still flushes on `Close`.
- [ ] **Step 5:** Commit `feat(redact): line-buffering redacting io.Writer for Cobra output`.

### Task 0.3: Golden harness through the REAL `cmd/zscalerctl` boundary
**Files:** Create `cmd/zscalerctl/golden_surface_test.go`, `cmd/zscalerctl/testdata/surface/`.
- [ ] **Step 1:** Build the test binary once (`go test` can `go build -o` a temp binary, or use `os/exec` on a `go run`). Write `TestGoldenSurface` that, for each case, execs the binary with `args`+`env`, captures stdout/stderr/exit, **scrubs** non-determinism (git SHA, build date, abs paths, `time=…`), asserts `wantCode` in Go, and snapshots only the scrubbed stdout/stderr:
```go
cases := []struct{ name string; args []string; env []string; wantCode int }{
	{"root_help", []string{"--help"}, nil, 0},
	{"version", []string{"version"}, nil, 0},
	{"unknown_command", []string{"frobnicate"}, nil, 2},
	{"unknown_flag", []string{"version", "--nope"}, nil, 2},
	{"zia_locations_list_help", []string{"zia", "locations", "list", "--help"}, nil, 0},
	{"missing_creds", []string{"zia", "locations", "list"}, nil, 3}, // valid tree, no creds → 3
	{"ndjson_on_version_rejected", []string{"--format", "ndjson", "version"}, nil, 2},
	// ... one case per command × {json,ndjson,table,pretty} boundary (§5c)
}
```
- [ ] **Step 2:** Add a **command-tree-inventory golden** (after Phase 1 the inventory is the Cobra tree; in Phase 0 it's the hand-rolled surface captured the same way) and a `surface_changes.md` manifest template.
- [ ] **Step 3:** `go test ./cmd/zscalerctl -run TestGoldenSurface -update`; review every `.golden` for secrets/tenant values; re-run → PASS.
- [ ] **Step 4:** Commit `test(cli): freeze golden CLI surface through the real boundary before Cobra`.

---

## Phase 1 — Foundation + hybrid dispatch

### Task 1.1: Vendor a pinned Cobra
- [x] Pinned `github.com/spf13/cobra@v1.10.2` (latest stable as of 2026-06-20). Transitive deps: `github.com/spf13/pflag v1.0.9` (direct — used by globalflags.go), `github.com/inconshreveable/mousetrap v1.1.0` (indirect, Windows-only). No `go-md2man` in vendor (not imported at build time). `make verify-licenses && make vuln` → PASS.

### Task 1.2: Global-flag source-of-truth + Cobra mirror (§4a)
**Files:** Create `internal/cli/globalflags.go`, `globalflags_test.go`.
- [x] **Steps 1–5 DONE.** `internal/cli/globalflags.go` defines `globalFlagDefs []globalFlagDef` (13 entries, alphabetical), `defineGlobalFlags(fs *flag.FlagSet, filterVar *repeatableFlag) globalFlagPointers` (called by `parseGlobal`), `registerGlobalPersistentFlags(fs *pflag.FlagSet)`, and `applyGlobalPersistentFlags(cmd *cobra.Command)`. `parseGlobal` now calls `defineGlobalFlags` rather than registering flags inline. `TestCobraGlobalsMirrorParseGlobal` cross-checks name, kind, and default across both sides by calling the actual functions; type for filter normalises `""` vs `"[]"` (stdlib repeatableFlag vs pflag StringArray empty default). All 115 existing `internal/cli` tests pass.

### Task 1.3: Cobra root + redacting writers + error mapping (§5, §5a, §8)
**Files:** Create `internal/cli/root.go`, `root_test.go`.
- [ ] **Step 1: Failing test** — a `RunE` error carrying a high-entropy token does not leak through the boundary, and a flag error exits 2:
```go
func TestRootRedactsAndMapsErrors(t *testing.T) { /* exec binary: a command returning a token-bearing error → stderr redacted; `version --nope` → exit 2 */ }
```
- [ ] **Step 2:** FAIL. **Step 3:** Build `newRootCmd`: `SilenceErrors`,`SilenceUsage`,`TraverseChildren:true`,`EnablePrefixMatching=false`,`CompletionOptions{DisableDefaultCmd:true}`,`SuggestionsMinimumDistance=2`,`SetFlagErrorFunc(→UsageError)`; `root.SetOut(redact.NewWriter(a.out,ModeStandard))`/`SetErr` likewise; detect TTY/width from the raw `*os.File` (captured on `App`) BEFORE wrapping. Make every `Args`/validation return `UsageError`. **Step 4:** PASS. **Step 5:** Commit.

### Task 1.4: Hybrid dispatch in `App.Run`
**Files:** Modify `internal/cli/app.go` (`Run`/`runParsed`).
- [ ] **Step 1: Failing test** — `version` routes through Cobra while an un-migrated command still works:
```go
func TestHybridDispatch(t *testing.T) { /* version → Cobra path (assert via a marker); `zia locations list` (no creds) → still exit 3 via legacy */ }
```
- [ ] **Step 2:** FAIL. **Step 3:** In `runParsed`, after `parseGlobal`, add: `if isMigrated(rest[0]) { return a.execCobra(ctx, opts, rest) }` else fall through to the existing switch. `execCobra` builds the root, sets args to `rest`, injects `opts`+`ctx`, executes, returns the (wrapped) error. `isMigrated` starts as `{version,doctor}` and grows per phase. **Step 4:** PASS; all existing tests still green. **Step 5:** Commit.

### Task 1.5: Migrate `version` + `doctor`
- [ ] `newVersionCmd`/`newDoctorCmd` whose `RunE` calls the extracted `runVersion`/`runDoctor` logic, reading globals from `opts`. **Preserve their differing redaction sources** (§5b: `version`→`ModeStandard`; `doctor`→`a.renderer(cfg,opts)`) and format allowlists (§5c). Behavior tests pass; golden diffs for `version`/`doctor`/`root_help`/`unknown_*` re-blessed via the manifest. Commit.

### Task 1.6: Extend `windows-config` CI to cover `internal/cli` (§9)
- [ ] Modify `.github/workflows/ci.yml` `windows-config`: add `go build ./...` and `go test ./internal/cli` (or a smoke: `version`/`doctor`/`config init --help`/`completion bash`). Commit.

### Task 1.7: Parse-error → exit-2 coverage
- [ ] Tests asserting exit 2 for: unknown command, unknown flag, bad flag value, missing flag value, required-flag-missing, and `Args` validation — all via the boundary. Commit.

### Task 1.8: Gate + PR (Phase-1 gate above).

---

## Phase 2 — Products + resources + `zia url-lookup`
**Files:** `internal/cli/cmd_products.go`, `completion_args.go`, `cmd_url_lookup.go`.
- [ ] Product commands `zia/zpa/ztw/zcc/zidentity` taking `<resource> <list|get|show> [id]` args, `RunE` delegating to the extracted `runProduct` logic; arity enforced per `app.go:824-839`.
- [ ] `ValidArgsFunc` over the catalog (catalog-only, §6); test it runs with empty env/no config and returns names with no reader/LoadConfig/network.
- [ ] `zia url-lookup <url> [url...]` as an explicit non-catalog child preserving URL query/userinfo/fragment stripping; handle its `--help` in `RunE` if `DisableFlagParsing` is used (else validate args in `RunE`).
- [ ] Preserve per-command format allowlist (`json|ndjson|table|pretty` for reads; `url-lookup` rejects ndjson) + redaction source (`cfg.Defaults.Redaction`). Add `isMigrated` entries. Golden + behavior tests; gate + PR.

## Phase 3 — `dump` + `diff`
**Files:** `internal/cli/cmd_dump.go`, `cmd_diff.go`.
- [ ] Declare each runner's **local** flags as Cobra local flags (dump: `--out`,`--products`,`--resources`,`--continue-on-error`,`--force`; diff: `--products`,`--resources`,`--ignore-operational`,`--detail`,`--allow-partial`,`--fail-on-drift`); `RunE` reads them via `cmd.Flags().Get*` and calls the extracted runner with a constructed options struct (never raw argv). `--out` ≠ global `--output`.
- [ ] Preserve text-only format (ndjson rejected) + exit 6/7 flow through `exitCodeForError`. Decide `--`-after-local-flags (document or stitch `cmd.Flags().Args()`). Golden + behavior tests; gate + PR.

## Phase 4 — `config` + `schema list` + `auth status` + `--output` wrapper
**Files:** `internal/cli/cmd_config.go`, `cmd_schema.go`, `cmd_auth.go`.
- [ ] `config init|show`, `schema list` (NOT show), `auth status`. Keep config-free commands config-free; shared narrowing-validation in root `PersistentPreRunE`; config-load lazy inside `RunE` (no child `PersistentPreRunE` — avoid shadowing).
- [ ] Model **`--output`** as the boundary execution wrapper (buffer stdout → owner-only atomic file → reject with `dump` → suppress color for file sinks → still emit diff on drift), retaining its tests. Golden + behavior tests; gate + PR.

## Phase 5 — Completion overhaul
**Files:** relocate flag inventory out of `completion.go` first; then delete it.
- [ ] **Before deletion:** move `completionFlags`/`completionDumpFlags`/`completionDiffFlags` to `internal/cli/surface.go` (or derive from the Cobra tree) and re-point `man_test.go` + `agent_docs_test.go` at the new source. Verify both drift gates still compile + pass.
- [ ] Replace hand-written completion with Cobra-generated four-shell completion + the catalog `ValidArgsFunc`; remove `CompletionOptions.DisableDefaultCmd`.
- [ ] **Adapt (don't delete)** the completion security tests (`TestCompletionScriptsDoNotReadCredentialFilesOrUseReader`, url-lookup coverage, log-level values, catalog/product coverage, PowerShell parsing) to assert against Cobra output. Gate + PR.

## Phase 6 — Docs/manpages + drift + packaging
**Files:** `tools/gen-docs` (or a hidden `docs-gen`), generated `docs/cli/*.md` + `man/*.1`, `scripts/verify-cli-docs.sh`.
- [ ] Generate md/man via `spf13/cobra/doc` with `DisableAutoGenTag = true` (else the timestamp footer breaks the drift check). Ensure `url-lookup` is a real Cobra command so it gets a page.
- [ ] Rewrite `TestManPageDocumentsFlagsAndCommands` to assert against the generated multi-file tree (retiring the hand-written `man/zscalerctl.1` subset test); update `.goreleaser.yaml` `archives.files`, `release.yml`, and `verify-release-artifacts.sh` to package + verify the generated man/completion/doc set.
- [ ] Add the regenerate-and-`git diff --exit-code` CI drift check; update `AGENTS.md`/skill to point at the generated reference. **Migration complete.** Gate + PR.

---

## Self-review

**Spec coverage:** §3 scope (conventions + non-goals); §4 tree (Phases 2–4) + §4a globals (Task 1.2); §5 boundary/hybrid (Tasks 1.3–1.4, no `App.Main`); §5a redacting writers + `redact.NewWriter` (Task 0.2, 1.3) + raw-`*os.File` TTY; §5b redaction source (Task 1.5 + per phase, with tests); §5c format allowlist (golden per command×format, §9/0.3); §5d zctl preserved; §6 completion catalog-only (Phase 5 + Task 2 ValidArgsFunc test); §7 inline wins (golden re-bless); §8 root settings (Task 1.3); §9 gates — real-boundary harness (0.3), windows-config fix (1.6), wantCode/scrub/tree-inventory/manifest; §10 phases (0–6); §11 risks; §12 testing. All covered.

**Placeholders:** Phase 0/1 are executable TDD; Phases 2–6 are scoped tasks naming files, the extracted runner, the preserved contracts, and the gate — expanded to per-step checkboxes when each starts, applying the Phase-1 pattern. The pinned Cobra version is a named TODO resolved in Task 1.1 during the spike (not a vague placeholder).

**Type consistency:** `App.Run(ctx,args) error` (unchanged, ~115 sites), `run()` (boundary, unchanged), `redact.NewWriter(io.Writer,Mode) io.WriteCloser`, `newRootCmd`/`new<Cmd>Cmd`, `globalFlagSpecs`, `isMigrated`, `execCobra`, `UsageError{}` (`errors.Is`→`cli.ErrUsage`), the 12-case `exitCodeForError` (in `main.go`, not duplicated). Confirm exact sentinel + helper names against the code in Task 1.3/1.4.
