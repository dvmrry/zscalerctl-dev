# Cobra CLI Migration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace `zscalerctl`'s hand-rolled command dispatch (`internal/cli/app.go`, ~2.4k LOC, + 449-line `completion.go`) with a Cobra command tree, surface-preserving plus Cobra's free inline wins, without weakening the machine-first/no-leak ethos or the exit-code contract.

**Architecture:** A Cobra root command holds all persistent/global flags; subcommands live in small per-group files. Cobra's auto error/usage printing is fully silenced; every command's `RunE` returns an error, and one top-level handler routes errors through the existing redactor and maps them to exit codes 0–7. Built thin-slice: foundation + no-leak handler proven on `version`+`doctor`, then command groups one PR at a time, then completion, then docs.

**Tech Stack:** Go, `spf13/cobra` + `spf13/pflag` (new deps), existing `internal/redact`, `internal/config`, `internal/zscaler`, the resource catalog, and the existing `lipgloss`/`termenv` pretty stack.

---

## Reference: spec & locked decisions

Spec: `docs/superpowers/specs/2026-06-20-cobra-cli-migration-design.md`. Locked: Cobra; surface-preserving refactor + inline wins; thin-slice phasing; golden-snapshot + agentic-coverage gates; **no command renames**, **spinners deferred**, **no 1.0 history-reset**, **rename-to-`zctl` deferred to 1.0**. Keep the §5 handler fang-retrofit-friendly (errors redacted at the boundary; exit code = pure function of error type).

## Conventions for every task

- **No `Co-Authored-By` lines in commits** (strict repo rule).
- **Spike-then-land:** experiment in the public `dvmrry/zscalerctl-dev` fork; each phase lands as a clean, squash-merged PR to `main` in `dvmrry/zscalerctl`.
- **Value-free:** no real tenant identifiers / live-runtime artifacts / real credentials in committed fixtures.
- **TDD:** write the failing test, see it fail, implement minimally, see it pass, commit.
- **Frequent commits** at each green step.

## Per-phase gate (run before opening each phase PR)

- [ ] `go test -mod=vendor ./...` green
- [ ] Golden CLI-surface snapshot: every delta vs the committed baseline reviewed and re-blessed as **intentional** (`go test ./internal/cli -run TestGoldenSurface -update` only after manual diff review)
- [ ] Agentic-coverage eval (DAV-10) ≥ baseline floor
- [ ] `make check` green (gofmt, vet, staticcheck, govulncheck, semgrep, gitleaks, verify-docs, verify-actions-pinned, sync-agents-skill, verify-release-artifacts)
- [ ] `CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build ./...` (and the `windows-config` CI job) green
- [ ] PR opened against `main`, `semver:minor` (behavior shifts via inline wins), draft until the gate passes

---

## Phase 0 — Spike sandbox + golden-surface baseline

**Goal:** a private-to-churn workspace and a frozen record of today's exact CLI surface, so every later change is provably intentional.

### Task 0.1: Create the spike sandbox

- [ ] **Step 1: Fork to the public dev sandbox**

```bash
gh repo fork dvmrry/zscalerctl --clone=false --fork-name zscalerctl-dev
# if same-owner fork is rejected, seed a fresh repo instead:
#   gh repo create dvmrry/zscalerctl-dev --public --source=. --remote=dev --push
```

- [ ] **Step 2: Verify it exists and carries full history**

Run: `gh repo view dvmrry/zscalerctl-dev --json name,isFork,visibility`
Expected: `name=zscalerctl-dev`, public, fork (or seeded clone with full `git log`).

(No commit — infra only. The sandbox is throwaway, deleted at 1.0.)

### Task 0.2: Golden CLI-surface snapshot harness

**Files:**
- Create: `internal/cli/golden_surface_test.go`
- Create: `internal/cli/testdata/surface/` (generated `.golden` files)

- [ ] **Step 1: Write the snapshot test (initially capturing today's hand-rolled surface)**

```go
package cli_test

// TestGoldenSurface freezes the user-visible CLI surface: for each invocation,
// it records combined stdout, stderr, and exit code into a .golden file. Run
// with -update to re-bless after a reviewed, intentional change. A failing diff
// means the surface changed; the diff must be reviewed as intentional or reverted.
func TestGoldenSurface(t *testing.T) {
	cases := []struct{ name string; args []string; env []string }{
		{"root_help", []string{"--help"}, nil},
		{"no_args", nil, nil},
		{"version", []string{"version"}, nil},
		{"doctor_help", []string{"doctor", "--help"}, nil},
		{"dump_help", []string{"dump", "--help"}, nil},
		{"diff_help", []string{"diff", "--help"}, nil},
		{"list_help", []string{"list", "--help"}, nil},
		{"get_help", []string{"get", "--help"}, nil},
		{"show_help", []string{"show", "--help"}, nil},
		{"config_help", []string{"config", "--help"}, nil},
		{"config_init_help", []string{"config", "init", "--help"}, nil},
		{"schema_help", []string{"schema", "--help"}, nil},
		{"auth_help", []string{"auth", "--help"}, nil},
		{"auth_status_help", []string{"auth", "status", "--help"}, nil},
		{"completion_help", []string{"completion", "--help"}, nil},
		{"unknown_command", []string{"frobnicate"}, nil},
		{"unknown_flag", []string{"version", "--nonexistent"}, nil},
		{"missing_creds", []string{"list", "users"}, nil}, // no config/env → exit 3, redacted
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			var out, errOut bytes.Buffer
			app := cli.New(&out, &errOut, tc.env)
			code := app.Main(context.Background(), tc.args) // returns the resolved exit code
			got := fmt.Sprintf("=== exit: %d\n=== stdout:\n%s\n=== stderr:\n%s", code, out.String(), errOut.String())
			golden := filepath.Join("testdata", "surface", tc.name+".golden")
			if *update {
				_ = os.WriteFile(golden, []byte(got), 0o644)
				return
			}
			want, err := os.ReadFile(golden)
			if err != nil { t.Fatalf("read golden %s: %v (run -update to create)", golden, err) }
			if got != string(want) {
				t.Errorf("surface changed for %q:\n--- want\n%s\n--- got\n%s", tc.name, want, got)
			}
		})
	}
}

var update = flag.Bool("update", false, "update golden surface files")
```

- [ ] **Step 2: Add a thin `App.Main` exit-code entrypoint if one is not already exposed**

If `internal/cli` does not already expose a function returning the resolved exit code (vs calling `os.Exit`), add one so the test can assert codes. Check `cmd/zscalerctl/main.go` — the exit-code resolution must be callable without exiting. (If it already returns a code, reuse it.)

- [ ] **Step 3: Generate the baseline**

Run: `go test ./internal/cli -run TestGoldenSurface -update`
Then `go test ./internal/cli -run TestGoldenSurface` → PASS (baseline matches itself).

- [ ] **Step 4: Review the generated `.golden` files**

Manually read each `testdata/surface/*.golden`. Confirm none contains a real secret/tenant value (they should be help text, usage, and redacted errors). This is the frozen contract.

- [ ] **Step 5: Commit**

```bash
git add internal/cli/golden_surface_test.go internal/cli/testdata/surface/
git commit -m "test(cli): freeze golden CLI-surface baseline before Cobra migration"
```

---

## Phase 1 — Foundation: Cobra root + persistent flags + the no-leak/exit handler

**Goal:** stand up the Cobra root, all global flags, and the single error→redactor→exit-code chokepoint, and migrate `version` + `doctor` onto it. Proves the load-bearing §5 design on a tiny surface. The golden surface for these commands must match (or change only intentionally).

**Files:**
- Create: `internal/cli/root.go` (root command + persistent flags + the execute/error handler)
- Create: `internal/cli/exitcode.go` (error→exit-code mapping, pure function)
- Create: `internal/cli/cmd_version.go`, `internal/cli/cmd_doctor.go`
- Modify: `cmd/zscalerctl/main.go` (call the new entrypoint)
- Test: `internal/cli/exitcode_test.go`, `internal/cli/root_test.go`

### Task 1.1: Add Cobra and vendor it

- [ ] **Step 1: Add the dependency**

Run: `go get github.com/spf13/cobra@latest && go mod tidy && go mod vendor`
Expected: `spf13/cobra`, `spf13/pflag`, `inconshreveable/mousetrap` appear in `go.mod`/`vendor/`.

- [ ] **Step 2: Verify license + vuln gates accept it**

Run: `make verify-licenses && make vuln`
Expected: PASS (Cobra is Apache-2.0; no advisories).

- [ ] **Step 3: Commit**

```bash
git add go.mod go.sum vendor/
git commit -m "build: vendor spf13/cobra for the CLI migration"
```

### Task 1.2: Exit-code mapping (pure function — keeps the fang seam clean)

- [ ] **Step 1: Write the failing test**

```go
func TestExitCodeForError(t *testing.T) {
	cases := []struct{ name string; err error; want int }{
		{"nil", nil, 0},
		{"internal", errors.New("boom"), 1},
		{"usage", cli.UsageError{Message: "bad flag"}, 2},
		{"invalid_config", config.ErrInvalidConfig, 2},
		{"missing_creds", zscaler.ErrMissingCredentials, 3},
		{"not_found", cli.ErrNotFound, 4},
		{"live_fail", cli.ErrLiveAccessFailed, 5},
		{"partial_dump", cli.ErrPartialDump, 6},
		{"drift", cli.ErrDrift, 7},
	}
	for _, tc := range cases {
		if got := cli.ExitCodeForError(tc.err); got != tc.want {
			t.Errorf("ExitCodeForError(%s) = %d, want %d", tc.name, got, tc.want)
		}
	}
}
```

- [ ] **Step 2: Run to verify it fails** — Run: `go test ./internal/cli -run TestExitCodeForError` → FAIL (`ExitCodeForError` undefined).

- [ ] **Step 3: Implement `ExitCodeForError`** in `internal/cli/exitcode.go` — a pure `func ExitCodeForError(err error) int` using `errors.Is`/`errors.As` against the existing sentinel errors, mirroring the codes the current dispatch returns. Map Cobra/pflag flag-parse + unknown-command errors to `2` (detect via `UsageError` wrapping at the parse layer).

- [ ] **Step 4: Run to verify it passes** — `go test ./internal/cli -run TestExitCodeForError` → PASS.

- [ ] **Step 5: Commit** — `git commit -am "cli: pure error→exit-code mapping (0–7)"`

### Task 1.3: Root command + the silenced error handler

- [ ] **Step 1: Write the failing test (errors are silenced by Cobra and redacted by us)**

```go
func TestRootSilencesAndRedactsErrors(t *testing.T) {
	var out, errOut bytes.Buffer
	app := cli.New(&out, &errOut, nil)
	// A command whose RunE returns an error containing a secret-shaped token.
	code := app.Main(context.Background(), []string{"doctor", "--inject-test-error=AKIA_secret_like_token_1234567890"})
	if code == 0 { t.Fatal("want non-zero exit") }
	if strings.Contains(errOut.String(), "AKIA_secret_like_token_1234567890") {
		t.Errorf("secret-shaped token leaked to stderr: %q", errOut.String())
	}
	if !strings.Contains(errOut.String(), "<REDACTED") && /* entropy rule */ !redactedSomehow(errOut.String()) {
		t.Errorf("error not routed through redactor: %q", errOut.String())
	}
}
```

(The `--inject-test-error` flag is a hidden test-only flag on a throwaway command, or assert via a real error path that carries a high-entropy token. Prefer a real path if available.)

- [ ] **Step 2: Run to verify it fails** — FAIL (no root yet).

- [ ] **Step 3: Implement the root in `internal/cli/root.go`**

```go
func (a *App) newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "zscalerctl",
		Short:         "Read-only Zscaler configuration CLI",
		SilenceErrors: true, // we print, not Cobra
		SilenceUsage:  true, // no usage dump on error
	}
	// Persistent (global) flags, defined ONCE, inherited by all subcommands.
	pf := root.PersistentFlags()
	pf.String("format", "", "output format (json|yaml|table|...)")
	pf.String("profile", "", "config profile to select")
	pf.String("config", "", "config file path")
	pf.Duration("timeout", 0, "per-request timeout")
	pf.String("redaction", "", "redaction mode (standard|share|paranoid)")
	pf.StringSlice("fields", nil, "projection fields")
	pf.String("filter", "", "filter expression")
	pf.String("search", "", "search expression")
	pf.String("output", "", "output destination")
	pf.Bool("no-cache", false, "disable per-session cache")
	pf.Bool("color", false, "force color")
	pf.Bool("no-color", false, "disable color")
	pf.String("log-level", "", "log level")
	root.AddCommand(a.newVersionCmd(), a.newDoctorCmd())
	return root
}

// Main is the single entrypoint: build root, execute, route the error through
// the redactor, and resolve the exit code. Never calls os.Exit (returns the code).
func (a *App) Main(ctx context.Context, args []string) int {
	root := a.newRootCmd()
	root.SetArgs(args)
	root.SetOut(a.out)             // help → machine-first stdout
	root.SetErr(io.Discard)        // Cobra never writes errors; we do
	err := root.ExecuteContext(ctx)
	if err != nil {
		a.writeError(err)          // existing redacting writer → ScanRenderedString
	}
	return ExitCodeForError(err)
}
```

`cmd/zscalerctl/main.go` becomes: `os.Exit(cli.New(os.Stdout, os.Stderr, os.Environ()).Main(ctx, os.Args[1:]))`, preserving `muteProcessOutput` around it.

- [ ] **Step 4: Run to verify it passes** — `go test ./internal/cli -run TestRootSilencesAndRedactsErrors` → PASS.

- [ ] **Step 5: Commit** — `git commit -am "cli: Cobra root with silenced, redacted error handling + 0–7 exit mapping"`

### Task 1.4: Migrate `version` and `doctor`

- [ ] **Step 1:** Write `internal/cli/cmd_version.go` (`newVersionCmd`) and `cmd_doctor.go` (`newDoctorCmd`) as Cobra commands whose `RunE` calls the *existing* `runVersion`/`runDoctor` logic (extracted, not rewritten), reading persistent flags via the cobra flag set.
- [ ] **Step 2:** Run the existing version/doctor behavior tests → PASS (behavior unchanged).
- [ ] **Step 3:** `go test ./internal/cli -run TestGoldenSurface` → review the `version*`/`doctor*`/`root_help`/`unknown_*` diffs. Re-bless intentional inline-win changes (did-you-mean on `unknown_command`; cleaner per-command help) with `-update` after manual review; investigate anything unexpected.
- [ ] **Step 4:** Full `go test ./...` → PASS.
- [ ] **Step 5: Commit** — `git commit -am "cli: migrate version + doctor onto Cobra root"`

### Task 1.5: Phase-1 gate + PR

- [ ] Run the **Per-phase gate** checklist above.
- [ ] Open the Phase-1 PR (`semver:minor`). Body: links the spec, lists the re-blessed surface diffs as intentional inline wins.

---

## Phase 2 — Resources: `list` / `get` / `show` + dynamic catalog completion

**Files:** `internal/cli/cmd_resources.go` (the three commands sharing catalog plumbing), `internal/cli/completion_args.go` (`ValidArgsFunc` over the catalog), tests alongside.

**Tasks (each TDD, following the Phase-1 pattern — failing test → minimal impl → green → commit):**
- [ ] `newListCmd`/`newGetCmd`/`newShowCmd` with `RunE` delegating to the existing resource-reader logic; resource arg validated against the catalog.
- [ ] `ValidArgsFunc` returning catalog resource names/keys for dynamic completion (test: returns expected names; unknown prefix → empty).
- [ ] Golden-surface diffs for `list`/`get`/`show` re-blessed as intentional.
- [ ] Behavior tests (projection, `--fields`/`--filter`/`--search`) pass unchanged.
- [ ] Phase gate + PR.

## Phase 3 — `dump` + `diff`

**Files:** `internal/cli/cmd_dump.go`, `internal/cli/cmd_diff.go`, tests.

**Tasks:**
- [ ] `newDumpCmd`/`newDiffCmd` delegating to existing `runDump`/`runDiff`; exit codes 6 (partial-dump) and 7 (drift) flow through `ExitCodeForError`.
- [ ] Golden-surface diffs re-blessed; dump/diff behavior tests pass.
- [ ] Phase gate + PR.

## Phase 4 — `config` + `schema` + `auth` + `<product> help`

**Files:** `internal/cli/cmd_config.go` (`config` parent + `init`, `show`), `cmd_schema.go` (`schema show`), `cmd_auth.go` (`auth status`), `cmd_product.go` (product help topics), tests.

**Tasks:**
- [ ] Parent/child command groups (`config init` keeps its exit-2 refuse + owner-only write; `auth status`/`doctor`/`version` keep natural verbs — no renames).
- [ ] Golden-surface diffs re-blessed; existing config/schema/auth tests pass.
- [ ] Phase gate + PR.

## Phase 5 — Completion overhaul

**Files:** delete `internal/cli/completion.go` (449 lines); add `newCompletionCmd` (Cobra-generated bash/zsh/fish/powershell) wired to the `ValidArgsFunc` hooks from Phase 2.

**Tasks:**
- [ ] Replace the hand-written completion with `cobra`'s generator; keep the dynamic catalog completion via `ValidArgsFunc`.
- [ ] Test: generated completion scripts are non-empty for all four shells; dynamic completion returns catalog names.
- [ ] Retire and remove the old `completion_internal_test.go` cases that asserted the hand-rolled shell strings; replace with generation-smoke tests.
- [ ] Phase gate + PR.

## Phase 6 — Docs: Markdown/manpage generation + drift check

**Files:** `internal/cli/docs_gen.go` (or a `tools/gen-docs` main using `cobra/doc`), generated `docs/cli/*.md` + `man/*.1`, a CI drift check (`scripts/verify-cli-docs.sh`) wired into `make check`.

**Tasks:**
- [ ] Generate Markdown + manpages from the command tree via `spf13/cobra/doc`.
- [ ] CI drift check: regenerate in CI, `git diff --exit-code` the generated docs + completions vs committed — fail on drift (so README ⇄ manpage ⇄ completion ⇄ surface can't silently diverge).
- [ ] Update `AGENTS.md`/skill to point agents at the generated reference + the introspectable tree.
- [ ] Phase gate + PR. **Migration complete.**

---

## Self-review

**Spec coverage:** §1 motivation (intro); §2 Cobra (Task 1.1); §3 scope/non-goals (conventions + phase boundaries; renames/spinners/history-reset excluded); §4 command tree (Phases 1–4); §5 no-leak/exit handler (Tasks 1.2–1.3, the load-bearing detail with code); §6 completion (Phase 5 + Phase 2 `ValidArgsFunc`); §7 inline wins (re-blessed in golden-surface diffs per phase); §8 gates (Phase 0 harness + the per-phase gate); §9 workflow (conventions + Task 0.1); §10 phases (Phases 1–6); §11 risks (no-leak proven first in Phase 1; deps gated Task 1.1; doc drift Phase 6); §12 testing (golden surface + behavior tests + agentic eval + make check). All covered.

**Placeholders:** Phases 2–6 are scoped task lists, not vague TODOs — each names its files, the delegated existing logic, the gate, and references the fully-detailed Phase-1 pattern (failing test → minimal impl → green → commit). They are expanded into per-step checkboxes when each phase starts, after the Phase-1 spike proves the exact Cobra wiring. This decomposition is deliberate (later phases mechanically repeat Phase 1 against different command groups).

**Type consistency:** `App.Main(ctx, args) int`, `cli.New(out, err, env)`, `ExitCodeForError(err) int`, `newRootCmd`/`new<Cmd>Cmd`, `ValidArgsFunc`, the sentinel errors (`UsageError`, `ErrNotFound`, `ErrLiveAccessFailed`, `ErrPartialDump`, `ErrDrift`, `config.ErrInvalidConfig`, `zscaler.ErrMissingCredentials`) are used consistently across tasks. Note: confirm the exact sentinel names against the current code during Task 1.2 (the mapping must mirror what the existing dispatch returns) and adjust the table to match.
