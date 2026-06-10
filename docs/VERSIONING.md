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
`semver:major` is reserved until after `v1.0.0`; cutting `v1.0.0` itself is a
deliberate manual step (see [Automation](#automation)).

Machine-readable output schemas are part of the release contract. This includes
dump manifests, redaction reports, partial-dump error records, and the stderr
JSON error envelope emitted by a failing command under JSON output. The
published JSON Schemas for these artifacts live in [schema/](schema/) and carry
versioned `schema` ids; drift tests keep them in sync with the emitting structs.
Backward-compatible schema additions are minor releases; incompatible schema
changes are breaking changes.

Process exit codes are also part of the contract: `0` success, `1` internal
error, `2` usage (including invalid CLI flags, an invalid resource id, an
invalid `ZSCALERCTL_*` configuration value, and invalid proxy configuration),
`3` missing or invalid credentials, `4` resource not found or unsupported
(including a product/resource the tenant is not entitled to, or a get-by-ID whose
ID does not exist), `5` live API access failure, and `6` partial dump. Changing
the meaning of an exit code is a breaking change.

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

### Cutting v1.0.0

While the latest release is `0.x`, both the semver-label check and
`next-version.sh` reject a major bump unless `ZSCALERCTL_ALLOW_MAJOR_ZERO=true`.
Nothing sets that on the normal push/label path, so a `semver:major` label alone
can never produce `1.0.0`.

To cut `1.0.0`, manually run the release workflow (Actions → release → Run
workflow) with bump `major`. That manual-dispatch path — and only that path —
sets `ZSCALERCTL_ALLOW_MAJOR_ZERO=true`, so the `0.x → 1.0.0` bump is always a
deliberate human action. The release is built and tagged inside Actions, which is
also what produces the provenance attestations; pushing a `v1.0.0` tag by hand
would skip them. After `1.0.0`, the guard no longer applies and `semver:major`
releases follow the normal label flow.
