# CLI Surface Change Log

This file records every **intentional** golden delta introduced during the Cobra
migration.  A golden diff means a visible change to the CLI surface (help text,
error wording, exit code, output shape, or the command-tree inventory).  Every
such change must be recorded here before the PR is merged.

Column definitions:

| Column   | Meaning |
|----------|---------|
| Case     | Golden file basename (e.g. `help-flag`, `inventory`) |
| Old      | What the golden contained before the change |
| New      | What the golden contains after the change |
| Reason   | One-line rationale |
| Category | `help-text`, `error-wording`, `exit-code`, `output-shape`, `command-added`, `command-removed`, `command-renamed`, `flag-added`, `flag-removed` |

## Changes

| Case | Old | New | Reason | Category |
|------|-----|-----|--------|----------|
| `doctor-pretty` | (none — new case) | Bordered two-column key/value table for `doctor --format pretty --color never` | Pretty key/value renderer now has a distinct human formatting surface from tabular `--format table`; case freezes the semver-visible output shape. | `new-surface` |
| `version-unknown-flag` | `"message": "usage: zscalerctl version"` (extra arg treated as positional, rejected by requireNoArgs) | `"message": "unknown flag: --nope"` (Cobra flag parsing rejects unknown flags before RunE) | Cobra flag parsing — better error; `--nope` is now correctly identified as an unknown flag rather than an extra positional argument. Exit code stays 2. Category: inline-win. | `error-wording` |
| `version-help` | (none) | Cobra help surface frozen (Usage line + inherited global persistent flags) | version migrated to Cobra (Task 1.5); `--help` now emits Cobra-formatted help including all global flags | `new-surface` |
| `doctor-help` | (none) | Cobra help surface frozen (Usage line + inherited global persistent flags) | doctor migrated to Cobra (Task 1.5.2); `--help` now emits Cobra-formatted help including all global flags | `new-surface` |
| `zia-help` | Legacy resource-list: "usage: zscalerctl zia <resource> list\|get\|show" + 102 resource names + diagnostics section | Cobra product help: short description + Usage line + global flags | Products migrated to Cobra (Phase 2a); `zia --help` now shows Cobra-formatted help. Resource enumeration via `zscalerctl --format json schema list` (hint preserved in ResourceNotFoundError). Exit code stays 0. | `help-text` |
| `zia-locations-list-help` | Resource-specific field list + "usage: zscalerctl zia locations list\|get" + fields → then Cobra zia help (same as `zia-help`) | Same Cobra zia help but now also lists "Available Commands: url-lookup …" and footer | Cascade of Phase 2a (zia→Cobra) plus Phase 2b (url-lookup subcommand); `zia locations list --help` still intercepts at the zia level. Exit code stays 0. | `help-text` |
| `zia-help` | "read zia resources" + Usage line + Flags + Global Flags (no subcommands listed) | Same but now includes "Available Commands: url-lookup …" and "Use … [command] --help" footer | url-lookup is now a real Cobra subcommand of zia (Phase 2b DisableFlagParsing); Cobra automatically lists it in zia's help. Exit code stays 0. | `help-text` |
| `zia-url-lookup-help` | (none — new case) | Cobra help surface for the new url-lookup subcommand: short description + Usage line + Flags + Global Flags | url-lookup migrated to Cobra subcommand (Phase 2b); `--help` now shows Cobra-formatted help. | `new-surface` |
| `zia-locations-list-help` | Cobra zia product help (same as `zia-help` case) | Resource-specific field list: "usage: zscalerctl zia locations list\|get" + fields block | Phase 2c SetHelpFunc restores legacy resource-specific help; `zia locations list --help` now shows the locations resource fields. Final state — reverts Phase 2a/2b re-bless. Exit code stays 0. | `help-text` |
| `zia-locations-help` | (none — new case) | Resource-specific field list: "usage: zscalerctl zia locations list\|get" + fields block | Phase 2c: new golden freezes `zia locations --help` path (without explicit op). Identical output to `zia-locations-list-help`. Exit code 0. | `new-surface` |

| `dump-help` | (none — new case) | Cobra help surface frozen: short description + Usage line + local flags (--out/--products/--resources/--continue-on-error/--force) + global flags | dump migrated to Cobra (Phase 3a); `dump --help` now emits Cobra-formatted help. Legacy `usage: zscalerctl dump --out <dir>` synopsis is replaced by Cobra's standard help format. No credential leaks. Exit code stays 0. | `new-surface` |
| `diff-help` | (none — new case) | Cobra help surface frozen: short description + Usage line + local flags (--products/--resources/--ignore-operational/--detail/--allow-partial/--fail-on-drift) + global flags | diff migrated to Cobra (Phase 3b); `diff --help` now emits Cobra-formatted help. Legacy `usage: zscalerctl diff <old-dump-dir> <new-dump-dir>` synopsis is replaced by Cobra's standard help format. No credential leaks. Exit code stays 0. | `new-surface` |

| `config-help` | (none — new case) | Cobra parent help: "manage zscalerctl configuration" + Available Commands: init, show + global flags. Bare `config` → exit 2 (was legacy UsageError "usage: zscalerctl config show"). | config migrated to Cobra parent+subcommand (Phase 4); parent RunE returns UsageError listing `<init\|show>`. | `new-surface` |
| `config-bare` | Legacy UsageError "usage: zscalerctl config show" (exit 2) | UsageError "usage: zscalerctl config <init\|show>" (exit 2); JSON error envelope on stderr. | config parent now has RunE with explicit UsageError listing both subcommands — inline win. Exit code stays 2. | `error-wording` |
| `config-init` | (none — new case) | Path written to `<TMPDIR>/zscalerctl/config.yaml` on stdout; next-steps hints on stderr. | config init migrated to Cobra subcommand (Phase 4); format-agnostic (ndjson accepted, exit 0). | `new-surface` |
| `config-init-help` | (none — new case) | Cobra subcommand help: "write a starter config file with owner-only permissions" + local --force flag + global flags. | config init migrated to Cobra (Phase 4). | `new-surface` |
| `config-show-help` | (none — new case) | Cobra subcommand help: "show the active configuration (redacted)" + global flags. | config show migrated to Cobra (Phase 4). | `new-surface` |
| `schema-help` | (none — new case) | Cobra parent help: "inspect the resource catalog schema" + Available Commands: list + global flags. | schema migrated to Cobra parent+subcommand (Phase 4); parent RunE returns UsageError "usage: zscalerctl schema list". | `new-surface` |
| `schema-bare` | Legacy UsageError "usage: zscalerctl schema list" (exit 2) | UsageError "usage: zscalerctl schema list" (exit 2); JSON error envelope on stderr. | Behavior preserved; now routed through Cobra parent RunE. Exit code stays 2. | `error-wording` |
| `schema-list-help` | Legacy scoped synopsis "usage: zscalerctl schema list" (used in TestHelpFlagsReturnUsage) | Cobra subcommand help: "list all catalog resources and their supported operations" + global flags. | schema list migrated to Cobra (Phase 4); --help now uses Cobra format. | `help-text` |
| `auth-help` | (none — new case) | Cobra parent help: "inspect authentication configuration" + Available Commands: status + global flags. | auth migrated to Cobra parent+subcommand (Phase 4); parent RunE returns UsageError "usage: zscalerctl auth status". | `new-surface` |
| `auth-bare` | Legacy UsageError "usage: zscalerctl auth status" (exit 2) | UsageError "usage: zscalerctl auth status" (exit 2); JSON error envelope on stderr. | Behavior preserved; now routed through Cobra parent RunE. Exit code stays 2. | `error-wording` |
| `auth-status-help` | (none — new case) | Cobra subcommand help: "show authentication status for the active profile" + global flags. | auth status migrated to Cobra (Phase 4). | `new-surface` |

| `completion-bash.stdout` | Hand-written 32-line bash script using `_zscalerctl()` + `complete -F _zscalerctl zscalerctl` | Cobra-generated 426-line bash V2 completion using `__start_zscalerctl` + `complete -F __start_zscalerctl zscalerctl`; full `__complete`-protocol integration; per-flag enum completion for `--log-level`, `--format`, `--color`, `--redaction` | completion migrated to Cobra (Phase 5b); `completion bash` now uses `cmd.GenBashCompletionV2`; hidden `__complete`/`__completeNoDesc` commands are Cobra built-ins | `output-shape` |

| `completion-help` | Legacy `writeHelp` printed only `usage: zscalerctl completion bash\|zsh\|fish\|powershell` (one-line synopsis) | Cobra completion group help: multi-line block listing bash/fish/powershell/zsh subcommands with short descriptions + global flags. Exit code stays 0. | completion migrated to Cobra (Phase 5b); `completion --help` now emits the Cobra group help surface rather than the legacy one-liner. Golden frozen in `completion-help.stdout.golden`. | `message/help-change` |
| `completion` (bare) | Legacy `runCompletion` returned `UsageError{Message: "usage: zscalerctl completion bash\|zsh\|fish\|powershell"}` (exit 2) when called with no shell argument | Cobra's built-in completion command shows the completion group help (list of supported shells) on stdout and exits 0 | Cobra's completion command group does not require exactly one positional arg; showing group help is more user-friendly; restoring exit-2 would require a custom completion command that risks the security-critical `__complete` path; pre-1.0, no consumers | `behavior-change (exit-code)` |
| `completion <unknown-shell>` (e.g. `elvish`) | Legacy `runCompletion` returned `UsageError{Message: "usage: zscalerctl completion bash\|zsh\|fish\|powershell"}` (exit 2) when given an unrecognised shell name | Cobra routes unknown subcommands of the built-in completion group to the group's help (same output as bare `completion`): lists bash/fish/powershell/zsh on stdout, exits 0, stderr empty | Same rationale as the bare case above; Cobra's completion routing is intentional and security-critical (`__complete`/`__completeNoDesc` must not be interfered with); pre-1.0, no consumers | `behavior-change (exit-code)` |
| Local flags: single-dash long form (e.g. `dump -out`, `diff -products`, `config init -force`) | Legacy `flag.FlagSet` accepted single-dash long flags (e.g. `dump -out <dir>` parsed successfully and exited 3 / credential-missing after flag parse) | pflag requires double-dash for long local flags; `dump -out <dir>` now fails flag parsing with `{"kind":"usage","message":"unknown shorthand flag: 'o' in -out"}` (exit 2); global flags still accept single-dash via the custom `parseGlobal` path | `--out` / `--products` / `--force` are POSIX-correct long-flag spellings; pflag does not support GNU-style single-dash-long local flags; pre-1.0, no consumers | `behavior-change (exit-code)` |
| Unknown double-dash flag message (e.g. `dump --bogus`) | Legacy stdlib `flag.FlagSet` reported `flag provided but not defined: -X` (the flag name was always single-dash in the message regardless of input) | pflag reports `unknown flag: --X` (preserves the double-dash from the user's input); exit code unchanged (2) | pflag error wording is clearer (preserves user's exact token); pre-1.0, no consumers | `message-change` |

| `introspect` | (none — new case) | JSON surface map: `$schema`, `introspect_version`, `cli_version` (`<VERSION>` after scrub), `read_only`, `global_flags`, `commands`, `catalog`, `exit_codes`. FormatAuto resolves to JSON in hermetic (non-TTY) env. Config-free: no credentials loaded. Exit code 0. | introspect command frozen (Task 1.4); `cli_version` is scrubbed by `rePseudoVersion` (verified by `TestScrubPseudoVersion`). | `new-surface` |
| `introspect-pretty` | (none — new case) | Human-readable tree: `zscalerctl CLI surface map` header + global-flags block + commands block + catalog summary + exit-codes block. `version: <VERSION>` scrubbed. Exit code 0. | `--format pretty` path of introspect frozen (Task 1.4). | `new-surface` |
| `introspect-ndjson-rejected` | (none — new case) | `{"error":{"kind":"usage","message":"introspect does not support ndjson output yet"}}` on stderr; stdout empty. Exit code 2. | Mirrors `version-ndjson-rejected`; introspect is a single document, not a record stream. | `new-surface` |
| `introspect` | No `completion_protocol` field | Added `completion_protocol`: ["__complete", "__completeNoDesc"] to expose hidden shell-completion protocol tokens | Surface completion protocol for agent discovery; no command tree changes | `output-shape` |
| `help` | Legacy `writeUsage` printed a custom global usage synopsis; `help` was not a real command in the tree | Cobra-native `help [command]` with standard Cobra help output; `zscalerctl help` lists `Available Commands` and `zscalerctl help <command>` works; `help` now appears in `introspect` and `docs/cli/zscalerctl.md` | Remove the hidden no-op help override and legacy `help` token dispatch; `help` is now the canonical Cobra help command. | `command-added` |
| `help-flag` | Legacy global usage omitted `help` and `introspect` from the commands list | Global usage now lists `help [command]` and `introspect` alongside other utility commands | `writeUsage` coverage gate (`TestWriteUsageCoversAllCobraCommands`) requires all live depth-1 utility commands to be documented | `help-text` |
| `inventory` | Hardcoded `topLevel` slice in `TestCommandTreeInventory` | `TopLevel` derived from live Cobra tree via `WalkCobraTree`; deterministic alphabetical order; no commands added or removed | Live-tree oracle prevents drift between command tree and inventory golden. | `output-shape` |
| `introspect` | 290 commands | 291 commands; added `browse` as a hidden command with `--tui` local flag | Experimental TUI wiring on `feature/tui`: `browse` is hidden, requires `--tui`, and is fixture-backed (no config, credentials, or network). Hidden commands are already exposed by the introspect schema; agents should ignore `hidden: true` commands. | `command-added` |
| `introspect-pretty` | 290 commands | 291 commands; `browse` shown as `[hidden]` with `--tui` | Same experimental TUI wiring as `introspect`; hidden commands appear in the pretty tree with a `[hidden]` marker. | `help-text` |
| `introspect` | `browse` has one local flag (`--tui`) | `browse` now has four local flags: `--tui`, `--products`, `--resources`, `--continue-on-error` | Real reader-backed TUI collection path: `browse --tui` now loads config, builds a reader, and runs the collector with product/resource filters and continue-on-error policy. | `flag-added` |
| `introspect-pretty` | `browse` shows only `--tui` | `browse` shows `--tui`, `--products`, `--resources`, `--continue-on-error` | Same real reader-backed collection path as `introspect`. | `help-text` |

---

**Workflow:** when a Cobra migration phase changes a golden, do NOT just run
`-update` silently.  Instead:

1. Run `go test ./cmd/zscalerctl/... -run "TestGoldenSurface|TestCommandTreeInventory" -update`
2. Review the diff (`git diff testdata/surface/`).
3. Confirm each delta is intentional.
4. Add a row to the table above.
5. Commit golden files and this file in the same commit as the implementation.
