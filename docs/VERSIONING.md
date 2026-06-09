# Versioning Policy

`zscalerctl` uses semantic versioning for release tags.

Release tags are the source of truth and use the `vMAJOR.MINOR.PATCH` form,
such as `v0.1.0`. Release builds inject that tag into `zscalerctl version`.

## Before 1.0.0

Before `v1.0.0`, the project uses `v0.MINOR.PATCH`.

- `v0.1.0`: first security-reviewed, read-only release.
- `v0.2.0`, `v0.3.0`, and later `0.x` minor releases: meaningful feature
  expansion or behavior change, including new resources, new auth modes, new
  output formats, dump schema changes, command or flag changes, or default
  redaction behavior changes.
- `v0.1.1`, `v0.1.2`, and later patch releases: fixes and hardening that
  preserve the current command, output, and resource surface, including
  deadlock fixes, redaction bug fixes, dependency updates, docs corrections,
  CI/security gate improvements, and performance fixes that do not change
  output semantics.

Breaking changes are allowed in `0.x` minor releases, but not in patch releases.
`semver:major` is reserved until after `v1.0.0`.

Machine-readable output schemas are part of the release contract. This includes
dump manifests, redaction reports, and partial-dump error records. The published
JSON Schemas for these artifacts live in [schema/](schema/) and carry versioned
`schema` ids; a drift test keeps them in sync with the emitting structs.
Backward-compatible schema additions are minor releases; incompatible schema
changes are breaking changes.

## After 1.0.0

- Major: breaking contract changes, including removing or renaming commands,
  flags, environment variables, resources, or fields; incompatible JSON or dump
  schema changes; auth/config precedence changes; weakened security guarantees;
  or changed read-only guarantees.
- Minor: backward-compatible capability, including added resources, classified
  fields, commands, flags, output modes, or supported auth paths.
- Patch: bug fixes, security fixes, documentation fixes, dependency updates,
  and internal hardening with no intended contract expansion. A patch may redact
  more aggressively if it fixes a leak risk, but the release notes must call it
  out because automation may observe the changed output.

## Automation

Every pull request must have exactly one semver label:

- `semver:patch`
- `semver:minor`
- `semver:major`
- `semver:none`

The semver label check fails if the label is missing or ambiguous. While the
latest release is `0.x`, `semver:major` also fails.

On merge to `main`, the release workflow reads the merged pull request's semver
label, computes the next tag from the latest `v*` tag, runs release gates, builds
artifacts, and creates the GitHub release. `semver:none` skips release creation.
