# Builder Handoff

## Intent

What this change is supposed to accomplish.

## Base / Head

Base branch or commit:
Head branch or commit:

## Files Changed

List code, tests, fixtures, generated artifacts, docs, and scripts.

## Source Inputs Consulted

List SDK source files, resource catalog entries, schemas, specs, generated
fixtures, snapshots, docs, or other source artifacts used.

## Generated Artifacts

Which generated files changed, exact regeneration command, and whether changes
are expected.

## Expected Delta

Expected count/diff summary. Examples:

- Machine manifest resources: unchanged.
- Generated CLI docs: updated for one new flag.
- Field coverage counts: +1 classified field, no ignored-field count changes.
- Surface goldens: usage text changed intentionally for one command.

## Invariants Claimed

Examples:

- Existing resource readers are unchanged.
- No raw SDK records reach rendered output.
- Redaction mode behavior is unchanged.
- Error envelope shape and exit codes are unchanged.
- Generated CLI docs remain in sync with the Cobra command tree.
- Golden fixture drift is intentional and explained.

## Tests Run

Exact commands and result.

## Known Deferrals

What this PR intentionally does not solve.

## Review Focus

Specific areas reviewer should attack.
