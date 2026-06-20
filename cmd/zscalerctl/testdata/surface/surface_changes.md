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

---

**Workflow:** when a Cobra migration phase changes a golden, do NOT just run
`-update` silently.  Instead:

1. Run `go test ./cmd/zscalerctl/... -run "TestGoldenSurface|TestCommandTreeInventory" -update`
2. Review the diff (`git diff testdata/surface/`).
3. Confirm each delta is intentional.
4. Add a row to the table above.
5. Commit golden files and this file in the same commit as the implementation.
