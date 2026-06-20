# CLI Payoff Features: agent `introspect` + progress spinners

Status: **Design** — the dep-light payoffs the surface-preserving Cobra migration deferred. Two independent features: (A) a machine-readable `introspect` command for agents, (B) TTY-gated progress spinners for long operations. Validated against a 5-slice ecosystem survey (2026-06-20): agent-oriented CLI introspection is **greenfield** — kubectl/helm/gh/docker/hugo/cosign/goreleaser/gitleaks ship zero runtime command-tree dumps, and Cobra has no `GenJSON`. Field choices borrow from the only emerging precedents (CLIspec draft, `major/jira-agent`, Datadog Pup).

**Explicit non-goals:** Charm **fang** (deferred — adds Charm deps, restyles help/errors, and renders errors so the no-leak boundary would need a dedicated re-proof); per-command `--help --json` (every agent-native tool surveyed rejected it for a dedicated subcommand); shipping an MCP server (the `introspect` JSON is *shaped* to be MCP-`tools/list`-compatible, but we do not ship a server).

## 1. Context

The Cobra migration (merged to `zscalerctl-dev` main) replaced the hand-rolled dispatch with a uniform, typed, introspectable `*cobra.Command` tree and Cobra-generated completion. That uniform tree is the enabling substrate for both features here: `introspect` walks it (so it cannot drift from the real binary), and the spinner work plugs into the now-consistent command lifecycle. Neither feature was possible cheaply before the migration.

## 2. Constraints (preserved invariants)

- **Machine-first default.** stdout = data; stderr = diagnostics/progress. Spinners write ONLY to stderr and emit **zero bytes** in non-interactive/piped mode, so the data path and every existing golden are untouched.
- **No-leak.** `introspect` emits only *static structure* (command paths, flags, descriptions, the static resource catalog) — never tenant data — rendered through `redact.ModeStandard` like `version`. Spinner text is restricted to catalog resource names + fixed strings, and is always cleared before any stdout data or stderr error envelope.
- **Minimal-dependency.** Zero new runtime dependencies. Both features use only the Go stdlib and the Cobra public API (`Commands()`, `NonInheritedFlags()`, `InheritedFlags()`, `flag.Value.Type()`, `cmd.Annotations`).
- **Boundary unchanged.** `cmd/zscalerctl/main.go:run()` and the 12-case `exitCodeForError` (exit codes 0–7) are untouched.

## 3. Feature A — `introspect`

A **visible**, **config-free** Cobra command that emits a one-shot machine-readable map of the entire CLI surface. Visible (not hidden) because the tool must be *self-describing*: a hidden discovery command is circular (you must read AGENTS.md to learn the command that exists so you don't have to read docs). `introspect` (not `schema`) because `schema list` already means the resource catalog — overloading it would confuse.

### 3a. Output shape (define our own; ecosystem has no standard)

Output follows the CLI's standard auto-format convention (the same machine-first/pretty-on-TTY behavior as `version` and `schema`): when stdout is **piped/non-interactive** — the agent case — it emits **JSON** with no flag required; on an **interactive TTY** it renders a human-readable tree. `--format json` forces JSON anywhere (e.g. an agent allocating a PTY); `--format table`/`pretty` force the tree; `ndjson` is rejected (single document, not a record stream). `--format auto` is collapsed to the concrete format by `resolveFormat` before the command runs, so the command itself only ever sees json/table/pretty/ndjson. One call returns the full tree (zscalerctl's surface is bounded; no lazy per-command form needed). Net effect: agents (piped) always get JSON without opting in, while a human running it interactively gets the readable tree.

```jsonc
{
  "$schema": "https://raw.githubusercontent.com/dvmrry/zscalerctl/main/docs/schema/introspect.schema.json",
  "introspect_version": "1",        // contract version at the OUTERMOST key (integer-string)
  "cli_version": "<semver>",
  "read_only": true,                // CLI-wide guarantee — the headline agent-safety signal
  "global_flags": [                 // the 13 mirrored globals (globalFlagDefs), fully described ONCE here
    { "name": "format", "shorthand": "", "type": "string", "default": "auto",
      "usage": "...", "enum": ["auto","table","json","ndjson","pretty"] }
    // ... config, profile, output, timeout, redaction, no-cache, color, no-color, log-level, fields, filter, search
  ],
  "commands": [
    {
      "path": "zia locations list", // space-joined; maps to an MCP tool name via space→.
      "short": "...", "long": "...",
      "aliases": [], "hidden": false, "deprecated": "",
      "mutating": false,            // per-command (always false today; future-proofs a write cmd)
      "args": { "policy": "exact|range|arbitrary|none", "n": 0, "valid_values": [] },
      "flags": [                    // LOCAL flags (NonInheritedFlags)
        { "name": "...", "shorthand": "", "type": "string", "default": "",
          "required": false, "usage": "...", "enum": [] }
      ],
      "inherited_flags": [ "format", "filter", "fields", "..." ],  // names only — the globals; not re-described per command
      "output_fields": [ "id", "name", "..." ]   // reads only: the projected fields agents can --fields/--filter on
    }
  ],
  "catalog": { "products": ["zia","zpa","ztw","zcc","zidentity"],
               "resources": [ { "product": "zia", "name": "locations", "ops": ["list","get","show"], "fields": ["..."] } ] },
  "exit_codes": [                   // the error contract, programmatically consumable
    { "code": 0, "kind": "ok" },
    { "code": 2, "kind": "usage", "retryable": false, "description": "..." },
    { "code": 3, "kind": "missing_credentials", "retryable": false, "description": "..." }
    // ... 1,4,5,6,7
  ]
}
```

Field rationale (each traced to a precedent or a zscalerctl-specific need):
- `read_only` + per-command `mutating` — the single highest-value agent-safety annotation (CLIspec + agent-CLI research converged on it). For a read-only tool, `read_only:true` advertises the guarantee in one field.
- `flags` vs `inherited_flags` split — agents must not re-document `--profile`/`--filter`/`--format` per command; the persistent globals are listed by name once per command and fully described under a top-level... (see below).
- `type` from `flag.Value.Type()` — the only runtime source; cobra's own `GenYamlTree` drops it (the anti-pattern the survey flagged).
- `output_fields` — what an agent can pass to `--fields` or reference in `--filter` (populated from the projected model's JSON field names; zscalerctl-specific, validated by CLIspec's `output_fields`).
- `exit_codes` — the 0–7 model + kinds, so agents handle errors without scraping text.
- hidden/deprecated commands are **included with flags set** (not filtered out) so agents have the complete picture and can deliberately avoid them.
- A top-level `global_flags` block fully describes the 13 mirrored globals once (so `inherited_flags` per command stays name-only). Derived from `globalFlagDefs` (the single source of truth).

### 3b. Derivation (dep-free, DRY with docs-gen)

Refactor the existing `scripts/gen-cli-docs.go` tree-walk into a shared introspection function (e.g. `internal/cli.introspectTree(root) IntrospectDoc`) that both the runtime `introspect` command (emits JSON) and `gen-cli-docs` (emits markdown) consume — **one tree-walk, cannot drift**. The walk uses `BuildCommandTree` + `resources.Catalog()` + `globalFlagDefs`; `args.policy` comes from each command's `Args`/usage; `mutating` from `cmd.Annotations["introspect/mutating"]` (absent ⇒ false). Config-free: the catalog is static, so `introspect` needs no creds and never calls `LoadConfig`/the reader/network — exit 0 always (no-auth-required, per the jira-agent precedent: agents discover before they authenticate).

### 3c. Published schema + discoverability

Commit `docs/schema/introspect.schema.json` (mirroring the existing published `config.schema.json`), referenced by the `$schema` key, with a drift gate (the runtime output validates against it, like the config-schema gate). Add an **AGENTS.md** line instructing agents to "run `zscalerctl introspect` first for the machine-readable command + catalog map." `introspect` gets a generated `docs/cli` page like every other command.

## 4. Feature B — progress spinners

A small dep-free progress indicator on **stderr** for long-running work, invisible in machine/pipeline use.

### 4a. The `Spinner` (internal/output)

A braille-frame (`⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏`) indicator using `\r` carriage-return overwrite, `Start()/Update(text)/Stop()`. Writes to the App's stderr. **Active iff ALL hold:** stderr is a TTY (new `a.stderrTTY`, captured at `New` from the raw `*os.File` like `stdoutTTY`) AND `--log-level == off` (else diagnostic log lines clash with the `\r` line) AND color mode ≠ `never` (respects the user's plain-output intent). When inactive, every method is a no-op → zero bytes. `Stop()` clears the line (`\r` + spaces + `\r`) before any subsequent output.

### 4b. Wiring

- **`dump` — determinate.** `collectDump` gains an optional progress callback invoked per resource; the dump command renders `⠹ [3/12] zia/locations`. Cleared on completion; the existing stderr status (`dump written: …` / partial-dump notice) prints after. The known resource set comes from the already-resolved `--products`/`--resources` selection.
- **single-call reads — indeterminate.** `list`/`get`/`show`, `zia url-lookup`, and `doctor` wrap the in-flight reader/API call in `⠋ contacting Zscaler…`, cleared before the projected record(s) or an error envelope reach stdout/stderr.

### 4c. No-leak

Spinner text is restricted to catalog resource identifiers (`zia/locations`) and fixed strings — never record values, tenant identifiers, or credential material — and is always cleared before stdout data or the error envelope. (It does not flow through `redact.NewWriter`; the constraint is enforced at the call sites by only ever passing catalog names + literals.)

## 5. Research basis (2026-06-20 ecosystem survey)

Agent-oriented CLI introspection is novel; no de-facto schema exists. Borrowed: CLIspec's `mutating`/`output_fields`/`errors`-catalog shape + version-key; `jira-agent`'s public-API tree walk + "call it first" AGENTS.md instruction; Datadog Pup's dedicated-subcommand + JSON-default. Avoided anti-patterns: cobra `GenYamlTree` (drops flag types), build-time generation wired as a runtime command (drift), flat path arrays that lose hierarchy/inherited-flag distinction, and JSON-behind-a-required-flag (a footgun for agents that call with no flags).

## 6. Verification

- **`introspect` golden** through the real boundary: `introspect` (json) + `introspect --pretty`, scrubbed for `cli_version`/SHA, asserted leak-free (structure + catalog names only).
- **Schema drift gate:** the runtime `introspect` JSON validates against the committed `docs/schema/introspect.schema.json`; CI fails on divergence (mirror the config-schema gate).
- **DRY gate:** a test asserting `introspect` and `gen-cli-docs` enumerate the identical command set (shared `introspectTree`), so docs and the agent map cannot diverge.
- **introspect config-free / no-reader:** runs with empty env + a reader that fails if constructed → exits 0 with the full map (proves no creds/network).
- **Spinner tests:** zero output when stderr is not a TTY, when `--log-level != off`, and when `--color never` (the three gates); a forced-TTY path asserts frames render + the line is cleared; a no-leak test asserts only catalog names/literals are ever written.
- `make check` + `-race` + the windows-config + the binary golden harness all stay green; both features are additive (machine default unchanged).

## 7. Phases

**Phase 1 — `introspect`.** (a) Extract the shared `introspectTree` walk (refactor `gen-cli-docs`); (b) `newIntrospectCmd` (config-free, JSON default + pretty, ModeStandard, `read_only`/`mutating`/flag-split/`output_fields`/`exit_codes`/`global_flags`); (c) wire `isMigrated`/`execCobra`; (d) commit `docs/schema/introspect.schema.json` + the drift + DRY gates; (e) golden + config-free tests; (f) AGENTS.md + the generated doc page. Gate + push to `zscalerctl-dev` + PR (real CI).

**Phase 2 — spinners.** (a) `internal/output.Spinner` + `a.stderrTTY`; (b) the three-gate activation; (c) `collectDump` progress callback + the determinate dump render; (d) the indeterminate single-call wrap; (e) the gating + no-leak tests. Gate + PR.

Each phase: subagent-driven (fresh implementer + spec & code-quality review per task), `make check`/`-race` green, real-CI-on-`zscalerctl-dev` validation (the migration's lesson: local-green ≠ CI-green), and an adversarial pass before merge.
