# zscalerctl Threat Model

`zscalerctl` is an unofficial, security-first CLI for querying Zscaler
configuration and producing controlled, sanitized exports. Its design goal is to
help authorized administrators understand their own tenant configuration without
creating a credential leak, data leak, or abuse-enabling tool.

This project is defensive administration software. It is not an exploitation,
credential discovery, policy bypass, traffic interception, or offensive
reconnaissance tool.

Review stamp: last reviewed on 2026-06-16 against
`github.com/zscaler/zscaler-sdk-go/v3 v3.8.38`. Re-review this threat model on
every Zscaler SDK version bump.

## Security Objectives

- Never print, log, store, commit, or export API credentials, bearer tokens,
  client secrets, private keys, cookies, session values, or authorization
  headers.
- Treat sanitized dumps as still sensitive.
- Default to read-only behavior.
- Make future write operations explicit, gated, auditable, and separate from
  read/query/dump flows.
- Prefer fail-closed data handling over broad API response dumping.
- Keep outputs deterministic enough for audit and diff workflows without
  weakening privacy guarantees.
- Keep the project understandable enough that reviewers can inspect its safety
  controls without reverse engineering the code.

## Non-Goals

The tool must not provide:

- Credential harvesting, discovery, brute forcing, or validation against
  unauthorized tenants.
- Bypass or evasion workflows for Zscaler controls.
- Packet capture, traffic interception, decryption, or user activity
  surveillance.
- Exploit checks, vulnerability scanning, or attack path generation.
- Unbounded endpoint crawling outside a declared resource catalog.
- Raw unaudited API dumps as the default behavior.
- A mode that disables safety redaction.
- A hidden or implicit write path.

If a future feature blurs these boundaries, it must be discussed and documented
before implementation.

## Trust Boundaries

Primary trust boundaries:

- Local user and shell environment.
- Local config files and secret files.
- Zscaler API credentials and token material.
- Official Zscaler Go SDK.
- Zscaler API responses.
- Terminal stdout/stderr.
- Files written by dump/export commands.
- CI logs and test artifacts.

The SDK is part of the trusted computing base. Wrapping it improves project
structure, but does not remove the need to audit SDK credential loading,
debug logging, cache behavior, retry behavior, and error rendering.
SDK version bumps must re-verify these properties and must pass `govulncheck`.

Go modules are vendored so CI and release builds use inspectable dependency
source. Dependency changes must refresh `vendor/` and pass the dependency policy
checks.

## Threat Actors

The design considers these actors:

- A local observer who can see shell history, process arguments, terminal
  output, environment variables, logs, or files.
- A recipient of a shared sanitized dump.
- A CI or automation system that captures stdout, stderr, logs, or artifacts.
- An administrator who accidentally includes secrets or sensitive tenant data in
  free-text fields such as descriptions or comments.
- A future maintainer who adds a new resource and accidentally exposes raw SDK
  fields.
- A compromised or surprising dependency that changes output fields, logging,
  or error behavior.

The tool does not attempt to defend against a fully compromised workstation,
kernel-level attacker, or malicious administrator with valid tenant access.

## Primary Controls

The primary leak-prevention control is allow-list projection:

- API responses are ingested into internal models.
- Each supported resource declares a safe view model.
- Only explicitly allowed fields may be rendered.
- Fields outside the allow-list are dropped fail-closed — and none are left
  undecided: every emitted field is explicitly classified, and every
  non-emitted SDK field is deliberately excluded with a recorded reason
  ([FIELD_COVERAGE.md](FIELD_COVERAGE.md)).
- Free-text fields are excluded, redacted, or scanned before output.

This coverage claim is test-enforced, not aspirational. The shape-registry
tests fail the build when an SDK response field is neither classified in the
catalog nor excluded with a recorded reason, and
`TestFieldCoverageReportIsCurrent` fails the build when the committed
coverage report drifts from the code, so a new SDK field cannot ship
undecided.

Output redaction is defense-in-depth for values, not the field-selection
mechanism. Classification decides which fields render; every renderer still
passes the final bytes through redaction and secret scanning to catch
secret-shaped values pasted into otherwise-safe fields.

## Secret Handling

Secrets must not be accepted as ordinary CLI arguments because arguments can
appear in shell history and process listings.

Allowed secret sources:

- Environment variables using the `ZSCALERCTL_*` namespace.
- Secret files with strict owner-only permissions.
- Structured `cmd` references in owner-only profile files. The command is
  executed as an argv array with no shell interpretation, has a bounded timeout
  plus a 2-second WaitDelay grace period to force-close misbehaving background
  forks, and must return the secret on stdout. Provider stderr is summarized without
  content so helper failures cannot leak token material through error messages.
  Operators can disable this provider entirely with
  `ZSCALERCTL_DISALLOW_CMD=true`.
- OS keychain references through `keyring:<service>/<key>`. This provider is
  read-only and value-free on failure. macOS reads with the absolute
  `/usr/bin/security` helper (`-w` primary; a second `-g` call only to
  disambiguate hex-looking output, whose stderr is captured but never surfaced
  raw), Linux reads with `secret-tool` using bounded
  no-shell execution and a `ZSCALERCTL_*`-scrubbed environment, and Windows
  reads directly through `CredReadW` from `advapi32.dll` loaded with
  `NewLazySystemDLL`. Locked or unavailable keychains fail with bounded,
  actionable errors; headless workflows should keep using `env:`, `file:`, or
  structured `cmd:` providers.

Config files should contain non-secret profile settings only. Commands such as
`config show`, `doctor`, and `auth status` must render through secret-safe
types and must never reveal token material.

## Read-Only Posture

Version 1 is read-only by design. Commands should use read/query/list/get/dump
semantics and avoid mutation-capable API calls.

Future write support, if added, must use:

- Separate command verbs.
- Explicit feature enablement.
- Resource allow-lists.
- Dry-run output.
- Human confirmation for interactive use.
- Clear audit logging that does not include secrets.

Read-only credentials are recommended, but the program must not rely on
credential scope as its only safety boundary.

## Dump Handling

Sanitized dumps remain sensitive because policy rules, topology, application
segments, locations, and access patterns can reveal tenant security posture.

Dump commands must:

- Write with restrictive permissions.
- Refuse unsafe overwrites by default.
- Replace an existing dump only through explicit `dump --force`, and only when
  the target already validates as a zscalerctl dump directory or is empty.
- Use deterministic structure where possible.
- Include a manifest and redaction report.
- Avoid original secret values in reports.
- Mark partial dumps explicitly and put only value-free failure metadata in
  `errors.ndjson`.
- Avoid raw API response archives unless a future design explicitly justifies
  and protects them.

## Redaction Modes

Initial modes:

- `standard`: intended for local operational use. Emits only allow-listed fields
  and redacts secret-shaped values, including bare high-entropy tokens in
  emitted string fields. Free-text fields are standard-only catalog exceptions
  and must carry a `standard_free_text_reason`.
- `share`: intended for sharing outside the immediate admin context. It removes
  identifier and free-text fields by projection — fields classified as sensitive
  identifiers (users, login names, emails, host/tenant identity) and free-text
  fields are standard-only, so they are dropped entirely rather than emitted. As
  defense-in-depth on the fields that do render, it additionally byte-masks
  email addresses and IPv4 addresses. Note that the byte-scan covers only emails
  and IPv4; protection for domains, hostnames, and tenant identifiers comes from
  the projection allow-list (their fields being classified standard-only), not
  from a content scanner — so correct field classification is the control. The
  catalog validator enforces that bare identifier-named fields stay standard-only
  unless explicitly justified.
- `paranoid`: intended for maximum minimization. It may sacrifice cross-dump
  diffability until a safe tokenization and key-management design exists.

There is no `off` mode.

## Scope Decisions

The enabled resource catalog is derived from the compiled catalog and published
in [RESOURCES.md](RESOURCES.md). Deferred and queued SDK surfaces are tracked in
[RESOURCE_QUEUE.md](RESOURCE_QUEUE.md).

`paranoid` mode does not promise cross-dump diffability until a safe
tokenization and key-management design exists.

## Mandatory CI Checks

`make check` is the authoritative gate; the list below is representative, not
exhaustive (see [DEPENDENCY_POLICY.md](DEPENDENCY_POLICY.md) for the full set).

- `go test ./...` and `go test -race ./...`
- `go vet ./...`, `make fmt-check`
- `govulncheck ./...` and staticcheck
- semgrep (credential escape-hatch and SDK-boundary rules)
- secret scan (gitleaks over the working tree locally / the gitleaks action in CI)
- `bash scripts/verify-sdk-boundary.sh`
- `bash scripts/test-verify-sdk-boundary.sh`
- the doc, no-live-creds, actions-pinned, and registry verifiers
