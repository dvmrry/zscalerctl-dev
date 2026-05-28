# zscalerctl Threat Model

`zscalerctl` is an unofficial, security-first CLI for querying Zscaler
configuration and producing controlled, sanitized exports. Its design goal is to
help authorized administrators understand their own tenant configuration without
creating a credential leak, data leak, or abuse-enabling tool.

This project is defensive administration software. It is not an exploitation,
credential discovery, policy bypass, traffic interception, or offensive
reconnaissance tool.

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
- Unknown fields are dropped by default.
- Free-text fields are excluded, redacted, or scanned before output.

Output redaction is defense-in-depth, not the main safety mechanism. Every
renderer should still pass final bytes through redaction and secret scanning,
but the normal rendered model should already exclude unsafe fields.

## Secret Handling

Secrets must not be accepted as ordinary CLI arguments because arguments can
appear in shell history and process listings.

Allowed secret sources:

- Environment variables using the `ZSCALERCTL_*` namespace.
- Secret files with strict owner-only permissions.
- A future OS keychain integration.

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
  emitted free-text fields.
- `share`: intended for sharing outside the immediate admin context. Adds
  stronger removal or masking of identifiers such as users, emails, IPs,
  domains, tenant identifiers, and free-text fields.
- `paranoid`: intended for maximum minimization. It may sacrifice cross-dump
  diffability until a safe tokenization and key-management design exists.

There is no `off` mode.

## Open Decisions

- Exact resource catalog for the first release.
- Whether `paranoid` mode supports cross-dump diffs in version 1.
- Which future free-text fields, beyond currently modeled descriptions, may be
  emitted in `standard` mode. New free-text fields must keep the scanner
  backstop and catalog-driven canary coverage.

## Mandatory CI Checks

- `go test ./...`
- `go test -race ./...`
- `go vet ./...`
- `make fmt-check`
- `govulncheck ./...`
- `go run honnef.co/go/tools/cmd/staticcheck@v0.7.0 ./...`
- `gitleaks detect`
- `bash scripts/verify-sdk-boundary.sh`
- `bash scripts/test-verify-sdk-boundary.sh`
