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
| `zia-locations-list-help` | Resource-specific field list: "usage: zscalerctl zia locations list\|get" + fields | Cobra `zia` command help (same as `zia-help`) | Products take resource/op as positional args; Cobra intercepts `--help` before RunE so resource-specific help is no longer reachable via `--help`. Resource usage still shown in arity UsageErrors. Exit code stays 0. | `help-text` |

---

**Workflow:** when a Cobra migration phase changes a golden, do NOT just run
`-update` silently.  Instead:

1. Run `go test ./cmd/zscalerctl/... -run "TestGoldenSurface|TestCommandTreeInventory" -update`
2. Review the diff (`git diff testdata/surface/`).
3. Confirm each delta is intentional.
4. Add a row to the table above.
5. Commit golden files and this file in the same commit as the implementation.
