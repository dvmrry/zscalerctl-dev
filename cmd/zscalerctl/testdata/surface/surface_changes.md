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
| *(empty — no intentional deltas yet; this is the pre-Cobra baseline)* | | | | |

---

**Workflow:** when a Cobra migration phase changes a golden, do NOT just run
`-update` silently.  Instead:

1. Run `go test ./cmd/zscalerctl/... -run "TestGoldenSurface|TestCommandTreeInventory" -update`
2. Review the diff (`git diff testdata/surface/`).
3. Confirm each delta is intentional.
4. Add a row to the table above.
5. Commit golden files and this file in the same commit as the implementation.
