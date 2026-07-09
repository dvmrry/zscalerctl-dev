# Adversarial Review

Assume the change is wrong. Do not implement fixes. Find concrete ways it
could lie, drop data, overclaim, weaken tests, bypass redaction, change machine
contracts, or silently change generated artifacts.

## Blocking Findings

Each finding must include:

- file/path
- concrete issue
- why it matters
- how to reproduce or verify
- test that would catch it

## Non-Blocking Risks

Lower-confidence or future-hardening notes.

## Machine Contract Review

- Did JSON, NDJSON, error envelope, exit code, dump, diff, schema, manifest,
  or introspection behavior change?
- Are contract deltas intentional and documented?
- Are generated docs, schemas, and fixtures in sync with runtime behavior?

## Safety Review

- Did redaction, projection, field coverage, or sensitive-data classification
  change?
- Can a narrowed field set widen output or reveal previously dropped data?
- Are dump and diff artifacts still sanitized but treated as confidential?

## Generated Artifact Review

- Were generated files intentionally regenerated?
- Are deltas explained?
- Any suspicious drops, additions, ordering changes, or count changes?

## Verdict

One of:

- request changes
- approve with nits
- approve
