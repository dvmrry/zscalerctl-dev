# Design: Migrate the zscalerctl CLI dispatch to Cobra (revised)

Status: **Design v2.1 — v2 (full `app.go`/`main.go` surface inventory + ~50 findings across 5 families) refined by a Codex + Gemini validation pass: both validated the core architecture; ~9 mechanics fixes folded inline (full-buffer redactor, help/`--` routing, error-wrapping mechanics, `--output` as a Phase-1 invariant, the `__complete` route, ban required-flags, completed §5b/§5c tables, split golden harness).**
Date: 2026-06-20
Owner: Dave (maintainer)
Precedes: a phased implementation plan; spike in public `dvmrry/zscalerctl-dev`; clean phased PRs to `main`.

> **Why a v2:** the v1 draft was written from an incomplete read of the real CLI surface. Cross-family review (Codex, Gemini, Kimi, Deepseek, GLM) plus a direct code inventory corrected several fundamentals: the real command tree is product-scoped, the no-leak/exit boundary already exists in `cmd/zscalerctl/main.go` (and stays), the global-flag surface is too nuanced to re-parse in pflag, and per-command redaction mode and format allowlists are non-uniform and security-relevant. This document is the corrected design.

## 1. Context & motivation

`zscalerctl` is a read-only, security-first Go CLI. Its dispatch is hand-rolled: `internal/cli/app.go` (~2.4k LOC) routes via nested switches; `internal/cli/completion.go` (449 LOC) hand-writes four shells. Each new command adds another manual routing branch, completion case, and doc-drift surface. We migrate to Cobra **pre-1.0** (no external consumers; clean revert point) to get per-command help, an introspectable command tree, generated completions, generated docs/manpages, and standard flag/error hooks — **without** changing observable behavior beyond a bounded set of opt-in "inline wins."

## 2. Framework: Cobra

Decision unchanged from v1: Cobra (`spf13/cobra` + `spf13/pflag`, + `inconshreveable/mousetrap` on Windows). It uniquely covers the maintainer's goals (4-shell completion gen, md/man doc gen, introspectable tree, ecosystem). Kong/urfave/fang were considered; Cobra wins for these goals. **Pin a reviewed Cobra version during the Phase-0 spike and record the exact version before vendoring — do not `go get @latest` in the plan.**

## 3. Scope & non-goals

**In scope:** reimplement the **exact current surface** on Cobra, plus Cobra's no-extra-work inline wins (per-command `--help`, did-you-mean, aliases, examples, introspectable tree). Replace hand-written completion with Cobra-generated + dynamic catalog hooks. Add doc/manpage generation + a CI drift check.

**Non-goals / explicitly deferred:**
- No command renames; `list/get/show` stay resource-only verbs under products.
- No braille spinners (TTY-gated polish, later).
- No 1.0 history-reset / `zctl` rename (1.0-time; `zctl` persona is preserved as-is — see §5d).
- No behavior redesign beyond the bounded inline wins.
- **Globals are NOT re-parsed by pflag** (see §4a) — a deliberate risk-reduction, not laziness.

## 4. Command tree (the real surface)

The tree is **product-scoped**. Root `zscalerctl`:
- **Product commands** `zia`, `zpa`, `ztw`, `zcc`, `zidentity` — each takes positional args `<resource> <list|get|show> [id]` (catalog-driven; `runProduct`, `app.go:804`). Resource/op completion via `ValidArgsFunc` over the catalog.
- **`zia url-lookup <url> [url...]`** — a diagnostic verb under `zia`, NOT a catalog resource (`app.go:812`, `url_lookup.go`). Out-of-catalog; strict trailing-token handling (see §5c).
- **Flat commands:** `doctor`, `auth status`, `config init|show`, `schema list` (NOT `show`), `dump`, `diff`, `version`, `completion`.

Operation arity is enforced (`list`/`show` take no id; `get` takes one), matching `runProduct` exactly. `zctl` stays a thin alias (§5d). **No top-level `list/get/show`** — that was the v1 error.

### 4a. Global flag handling (key architectural decision)

The global surface (`parseGlobal` + `splitGlobalArgs`, `app.go:412`) has behavior pflag cannot reproduce: positional extraction (globals before/between/after the subcommand), single-dash long flags (`-format`, `-profile`, …), comma-separated `--fields` (`parseFieldsList`), repeatable `--filter` (a `Var`), the `redactionSet` "was-it-provided" sentinel, `--no-color` overriding tri-state `--color`, and `30s`/`auto`/`off` defaults.

**Decision: keep `parseGlobal`/`splitGlobalArgs` as the canonical global parser, unchanged.** Cobra persistent flags mirroring the globals are registered on the root **for help, completion, and introspection only** — Cobra never parses them, because `App.Run` strips globals via `splitGlobalArgs` before handing command args to Cobra, and `RunE` reads globals from the already-parsed `globalOptions` (carried on the `App`/context), not from `cmd.Flags()`. A single source-of-truth table (name → type → default → help → completion values) drives both `parseGlobal` and the Cobra persistent-flag registration; a test asserts they cannot drift.

**Consequence:** this eliminates an entire risk class — every global-flag finding (single-dash, positional, `--format=auto`, tri-state `--color`, repeatable `--filter`, comma `--fields`, one-way `--no-cache`, flag-defaults-clobbering-config via `redactionSet`, `-format`-as-short-cluster) is moot, because global parsing is byte-identical to today. The cost is that the global-flag *help text* is now Cobra-rendered (an intentional inline win, captured in the golden snapshot).

## 5. No-leak & exit-code boundary (the load-bearing part — corrected)

**The boundary already exists in `cmd/zscalerctl/main.go:run()` and stays unchanged:** it acquires `muteProcessOutput` (swaps `os.Stdout`/`os.Stderr` → `/dev/null`), recovers panics → exit 1, captures the **original** writers as parameters *before* muting, computes `errorFormat(args, stdout)` from **raw args** (so a parse error renders in the requested format), calls `app.Run(ctx, args)`, maps the returned error via the **12-case** `exitCodeForError`, suppresses the duplicate partial-dump error in non-JSON modes, and renders via `writeError` (always `redact.ModeStandard`).

**The migration changes only `App.Run`'s internals:**
- `App.Run` still has signature `(ctx, args) error` (preserving ~115 test call sites). It calls `parseGlobal` (§4a), then for a **migrated** command builds and executes the Cobra root (`SilenceErrors: true`, `SilenceUsage: true`) and returns the `RunE` error; for an **un-migrated** command it calls the existing `runParsed` logic. The hybrid routes on the already-parsed command token (`rest[0]`), so globals are never stripped by Cobra (they're already parsed). Each phase leaves the tool fully working.
- **Error → sentinel wrapping (precise mechanics):** Cobra/pflag errors are plain; map them to `UsageError` so `exitCodeForError` returns 2. (a) `root.SetFlagErrorFunc(… → UsageError{…})` for flag-parse errors. (b) Use **custom `Args` funcs** that wrap failures in `UsageError` — do NOT use `cobra.ExactArgs`/`MinimumNArgs` (they return plain errors). (c) **Ban `MarkFlagRequired` and flag-groups** — their validation bypasses `SetFlagErrorFunc`; enforce required/mutually-exclusive locals inside `RunE`/`PreRunE` returning `UsageError`. (d) Cobra "unknown command" has no in-tree hook — intercept it by string-match on the error returned *after* `Execute` and wrap it. (e) `ValidArgsFunc` returns `(…, ShellCompDirective)`, not an error — signal failure with `ShellCompDirectiveError`, never `UsageError`. `UsageError{}` already satisfies `errors.Is(err, cli.ErrUsage)`; the 12-case map stays solely in `main.go`.
- **Help (`-h`/`--help`) and `--`:** `splitGlobalArgs` consumes `-h`/`--help` into `opts.help` and drops the `--` sentinel. For a migrated command, **re-insert `--help`** (when `opts.help`) and **re-insert `--`** into the argv handed to Cobra, so Cobra's own help fires and positionals after `--` survive; gate the legacy `if opts.help` early-return in `runParsed` to un-migrated commands only. Root `--help` stays legacy-rendered (complete) during the transition; Cobra owns root help only once the tree is fully migrated.
- **`App.Main` is NOT introduced.** (v1 error.) The production entrypoint stays `cmd/zscalerctl/main.go:run`.

### 5a. Cobra output must not bypass the redactor

Cobra prints help/usage and completion scripts to its configured writers, bypassing the data-path redactor. Wrap them: `root.SetOut(redact.NewWriter(out, redact.ModeStandard))` and `SetErr` likewise. **`redact.NewWriter` does not exist yet** — it must be built (`internal/redact/writer.go`): a **full-buffer** `io.WriteCloser` that accumulates all output and redacts on `Close` (line-buffering misses *multi-line* secrets like private keys, and per-write splitting breaks patterns). The §5 handler must `defer w.Close()` around `Execute` to flush — Cobra never closes its writers, so a final line without `\n` would otherwise be dropped. **Completion-script generation must BYPASS the redactor** (its bytes must stay valid shell, exactly as generated, and use the existing completion `Redactor.Bytes` path); only help/usage/errors route through `NewWriter`. **TTY/width must be detected from the raw `*os.File` BEFORE wrapping** (`output.IsTerminal`/`TerminalWidth` need an `*os.File`); the redactor wrapper would report non-TTY/zero-width and kill the pretty persona. Decide and test help-text over-redaction (help examples may contain pattern-shaped strings); default to redacting help and add a no-over-redaction test on the committed help text.

### 5b. Per-command redaction source is a preserved contract

Redaction mode is **non-uniform** and must be preserved exactly (a single renderer factory would silently narrow redaction on tenant data — a no-leak regression):
- **`redact.ModeStandard` hardcoded:** `version`, `diff`, `completion`, and all command-boundary error envelopes (`main.go:writeError`).
- **`cfg.Defaults.Redaction` (the resolved `--redaction`/env/profile mode):** `doctor`, `auth status`, `config show`, `schema list`, `zia url-lookup`, **`dump`** (dump files + status output), and resource reads (`runProduct` → `ProjectRecords/RecordAndVerify(spec, cfg.Defaults.Redaction, …)`).
- **No redaction / fixed text:** `config init` writes a value-free template, is config-free, has a local `--force`, and does not consult `--format` today — preserve all of that.

The spec carries this table; each Cobra `RunE` sources its renderer from the same place as today; a redaction-invariant test exists per group.

### 5c. Per-command format allowlist is a preserved contract

The `--format` allowlist is per-command and asymmetric:
- **`json|ndjson|table|pretty`:** `list/get/show` (ndjson is the documented SIEM stream).
- **`json|table|pretty` (ndjson rejected via `rejectUnsupportedFormat`):** `version`, `doctor`, `auth status`, `config show`, `schema list`, `zia url-lookup`, `diff`.
- **text-only (ndjson rejected early; other formats pass to a text writer):** `completion`, `dump`.
- **format-agnostic:** `config init` ignores `--format` today (writes a fixed text template); preserve that — add no new format rejection unless deliberately re-blessed in the manifest.

Each Cobra `RunE` replicates its `rejectUnsupportedFormat` guard (or a shared helper keyed by command class); a golden case per command × `{json,ndjson,table,pretty}` boundary verifies it.

### 5d. `zctl` persona

`App.New` has no argv0 input; format resolution checks `stdoutTTY`/`--output`/`--format`. The migration **preserves current single-binary behavior**; the `zctl` rename/persona is a 1.0-time decision (§3). The migration must not assume the binary name changes.

## 6. Completion

Cobra generates bash/zsh/fish/powershell from the tree, retiring `completion.go`. Dynamic resource/op completion via `ValidArgsFunc` over the catalog. **Hard constraint:** every `ValidArgsFunc` may read only the static catalog (`resources.Catalog()`/`FindSpec`) — it must never construct the reader, resolve secret refs, call `config.LoadConfig`, or dial the API (a live call during `<TAB>` would resolve credentials and could leak). A test runs completion with empty env/no config and asserts catalog names with no error. The existing completion security tests (no-credential-leak, catalog/product coverage, url-lookup, log-level values, PowerShell parsing) are **adapted to Cobra output, not deleted** (§8). Completion output always uses `ModeStandard`.

## 7. Inline wins (bounded)

Per-command `--help`, did-you-mean (`SuggestionsMinimumDistance`), aliases, `Example` blocks, and the introspectable tree. The global-flag help is now Cobra-rendered. Everything else (deprecation frameworks, restructured commands) is a follow-up.

## 8. Cobra configuration reference (exact root settings)

To preserve the surface, the root command sets: `SilenceErrors: true`, `SilenceUsage: true`, `TraverseChildren: true` (so globals can appear between product and resource args), `EnablePrefixMatching = false` (legacy is exact-match; `doc`≠`doctor`), `CompletionOptions.DisableDefaultCmd: true` (Phase 1–4; removed in Phase 5 when we own completion), `SetFlagErrorFunc` (→ `UsageError`), and on the doc generator `DisableAutoGenTag = true` (Cobra's timestamp footer would make the doc-drift check always fail). Decide `--version`/`-v` deliberately: today only the `version` command exists; leave `root.Version` unset unless `--version` is intentionally added and re-blessed. Disable/override Cobra's auto `help` command to avoid conflict with the existing `help` handling. Do NOT use `MarkFlagRequired`/flag-group validation (see §5). When Cobra completion is enabled (Phase 5), route Cobra's hidden `__complete`/`__completeNoDesc` commands as config-free Cobra paths in the hybrid — they must not fall to legacy unknown-command handling.

## 9. Verification gates (both stay green, corrected)

- **Golden CLI-surface snapshot — through the REAL boundary.** The harness must exercise `cmd/zscalerctl/main.go:run` (mute + panic-recovery + raw-arg `errorFormat` + 12-case exit + partial-dump suppression), NOT an internal entrypoint — preferably by exec'ing the built test binary (in-process testing fights the global `os.Stdout` swap). Each case asserts **`wantCode` explicitly in Go** (never let `-update` bless an exit-code change) and snapshots only stdout/stderr, **scrubbed** for SHAs/dates/abs-paths/`time=`. Cases use the real tree (`zia locations list`, not `list users`). Add a **command-tree-inventory golden** (paths, aliases, flags, defaults, hidden, examples, args policy) + a `surface_changes` manifest (case, old, new, reason, category) as the durable intentional-vs-accidental gate. **Two test layers:** the exec'd-binary harness covers only the *boundary* surface (help, usage, errors, exit codes) — it cannot inject a fake reader, so successful `list/get/show`/`dump`/`url-lookup` *output* stays in **in-process behavior tests** (which CAN inject a fake `ResourceReader`).
- **Agentic-coverage eval (DAV-10).** Per-phase floor; the introspectable tree should raise it.
- **`windows-config` CI must cover `internal/cli`.** Today it runs only `fileperm`/`secretref`/`keyring`, so the "windows-config green" gate proves nothing about the migration. Phase 1 extends it to `go build ./...` + a Windows `internal/cli` smoke (`version`, `doctor`, `config init --help`, `completion bash`). Mousetrap's double-click guard is a documented no-op for this read-only CLI.

## 10. Phases (each ships a working tool)

**Phase 0 — Sandbox + real-boundary golden harness.** Create `dvmrry/zscalerctl-dev`; build `internal/redact` `NewWriter` (needed by §5a); stand up the golden harness through the exec'd binary with explicit `wantCode` + scrubbing + the tree-inventory golden; freeze the baseline.
**Phase 1 — Foundation + hybrid.** Cobra root with all §8 settings; the global-flag mirror + sync test (§4a); the redacting writers + raw-`*os.File` TTY detection (§5a); `SetFlagErrorFunc`/`UsageError` mapping (§5); the hybrid dispatch in `App.Run` (migrated→Cobra, else `runParsed`); extend `windows-config` CI (§9); migrate `version` + `doctor` (note their redaction sources differ — §5b). **The existing `App.Run` `--output` wrapper wraps ALL output, so migrated commands run inside it from Phase 1 — add `version --output`/`doctor --output` tests now; `--output` is NOT deferred to Phase 4.** Prove parse-error→exit-2 for every class.
**Phase 2 — Products + resources.** `zia/zpa/ztw/zcc/zidentity` with `<resource> <op>` args + catalog `ValidArgsFunc` (catalog-only, §6); `zia url-lookup` as an explicit non-catalog child (§5c); preserve per-command format allowlist + redaction source. Product help lands here.
**Phase 3 — `dump` + `diff`.** Declare each runner's **local** flags (`--out`/`--products`/`--resources`/`--continue-on-error`/`--force`; `--ignore-operational`/`--detail`/`--allow-partial`/`--fail-on-drift`) as Cobra local flags and call the extracted runner with a constructed options struct — never delegate raw argv (Cobra would strip/reject them). `--out` (dump-local) ≠ `--output` (global wrapper). Decide `--`-after-local-flags behavior (legacy parses locals after `--`; Cobra makes them positional) — document as an intentional change or stitch `cmd.Flags().Args()` back.
**Phase 4 — `config` + `schema list` + `auth status`.** (The `--output` security wrapper — buffer stdout → owner-only atomic file → reject with `dump` → suppress color for file sinks → still emit diff on drift — is already preserved as the `App.Run` wrapper from Phase 1; Phase 4 only adds these commands inside it.) Keep config-free commands config-free; shared narrowing-validation in root `PersistentPreRunE` with config-load lazy inside `RunE` (avoid the PersistentPreRunE-shadowing trap — no child `PersistentPreRunE`).
**Phase 5 — Completion overhaul.** Before deleting `completion.go`, **relocate the flag inventory** (`completionFlags`/`completionDumpFlags`/`completionDiffFlags`) to a non-deleted file or derive it from the Cobra tree — `man_test.go` and `agent_docs_test.go` source their drift inventory from it. Replace with Cobra-generated completion + the catalog `ValidArgsFunc`; **adapt** (don't delete) the completion security tests; remove `DisableDefaultCmd`.
**Phase 6 — Docs/manpages + drift + packaging.** Generate md/man via `cobra/doc` with `DisableAutoGenTag = true`; rewrite `man_test.go` to assert against the **generated multi-file** tree (replacing the hand-written `man/zscalerctl.1` subset test); update `.goreleaser.yaml`, `release.yml`, and `verify-release-artifacts.sh` to package + verify the generated man/completion/doc set; add the regenerate-and-`git diff --exit-code` CI drift check.

## 11. Risks & mitigations

- **No-leak regression (highest):** preserved per-command redaction source (§5b) + redacting Cobra writers (§5a) + redaction-invariant tests + the golden boundary harness.
- **Global-flag regression:** eliminated by keeping `parseGlobal` (§4a) + the mirror sync test.
- **Mid-migration breakage:** the hybrid (§5) keeps every phase working; the boundary is unchanged.
- **Dynamic completion leak/live-call:** the catalog-only `ValidArgsFunc` constraint (§6) + a no-env test.
- **CI false-confidence:** `windows-config` extended to cover `internal/cli` (§9); the doc-drift check pinned with `DisableAutoGenTag`.
- **Dep add:** pinned, vendored, gated.

## 12. Testing

Golden boundary harness (§9); the ~115 existing `App.Run` behavior tests pass unchanged; per-command unit tests; parse-error→exit-code tests for every class; redaction-invariant tests per group (§5b); format-allowlist golden per command×format (§5c); the global-flag mirror sync test (§4a); the completion catalog-only/no-leak tests (§6); the man/agent-docs drift gates re-pointed (Phase 5/6); agentic-coverage eval; `make check` + the corrected `windows-config` job green per phase.
