# Builder Handoff

## Intent

Add a typed, in-process ZIA URL-classification capability to the common Go
engine and route the existing `zia url-lookup` Cobra command through it without
changing the supported CLI machine surface. Validate the complete batch before
config or live access, strip URL credential/query/fragment material on both
sides of the SDK boundary, and return only closed, defensively copied,
pre-redacted values and static errors.

## Base / Head

Base commit: `30c5b8e`

Initial implementation head: `484b7b2`

Reviewed fix heads: `5154141`, `d145738`, `de21132`, `0a8bf49`

Review scope: `30c5b8e..0a8bf49`

## Files Changed

Command-boundary regression:

- `cmd/zscalerctl/main_test.go`

Design and contract documentation:

- `docs/ENGINE_API_DESIGN.md`
- `docs/ENGINE_CAPABILITY_MODEL.md`
- `docs/cli/machine-contract.md`

CLI adapter and regressions:

- `internal/cli/app.go`
- `internal/cli/cobra_product_test.go`
- `internal/cli/commands_product.go`
- `internal/cli/url_lookup.go`
- `internal/cli/url_lookup_test.go`

Typed machine capability model:

- `internal/machine/engine_manifest.go`
- `internal/machine/engine_manifest_test.go`
- `internal/machine/engine_url_lookup.go`
- `internal/machine/engine_url_lookup_test.go`
- `internal/machine/executor.go`
- `internal/machine/types.go`

Trusted runtime and regressions:

- `internal/runtime/engine.go`
- `internal/runtime/errors.go`
- `internal/runtime/runtime_test.go`
- `internal/runtime/status.go`
- `internal/runtime/url_lookup.go`
- `internal/runtime/url_lookup_test.go`

## Source Inputs Consulted

- `AGENTS.md` and the repository adversarial-review workflow, handoff, run
  prompt, and report templates
- the accepted engine API and typed capability checkpoints
- the existing ZIA SDK URL-lookup reader and supported CLI adapter/output tests
- Go `net/url` parsing, escaping, host, path, and userinfo behavior
- config/provider construction, redaction, command-boundary error mapping, and
  core import-boundary gates
- the supported `machine.v1`, introspection, schema, generated CLI-doc, surface,
  field-coverage, release-artifact, and generated-skill gates

## Generated Artifacts

None. No published schema, supported `machine.v1` fixture, introspection
golden, command-surface golden, generated CLI document, field-coverage
artifact, release artifact, or generated skill changed.

## Expected Delta

- Add candidate `engine.v1` capability `zia.url_lookup`, operation `lookup`, a
  typed request, and a closed sanitized result.
- Add `runtime.Engine.LookupURL` and a narrow runtime facade over the existing
  official-SDK reader capability.
- Move the existing Cobra command onto the typed result while retaining its
  supported JSON field order and empty-array behavior, table/pretty rendering,
  usage, format policy, spinner behavior, stderr envelope, and exit mapping.
- Remove the final production `internal/cli` dependency on `internal/zscaler`.
- Define accepted URL syntax and fail-closed raw, parsed, and SDK-response
  normalization behavior.

## Invariants Claimed

- `zia.url_lookup` is advertised by `engine.v1` and executable through
  `runtime.Engine.LookupURL`; its effects conservatively include
  configuration-dependent local reads and process execution plus network
  access.
- The complete request is copied and validated before config loading, provider
  resolution, reader construction, or tenant contact.
- Accepted targets are hierarchical absolute URLs or the bare `host[/path]`
  form. Root-relative, scheme-relative, opaque, hostless scheme-only,
  malformed-escape, raw/escaped userinfo-like bare host, unsafe delimiter,
  invalid UTF-8, control, format, and non-ASCII-whitespace forms fail closed.
- Only surrounding ASCII spaces are trimmed. Retained decoded host/path
  components are revalidated after `net/url` unescaping; encoded ASCII `%20`
  paths remain valid.
- Userinfo, query, and fragment material is removed before the SDK call and
  removed again from SDK-returned URLs.
- Every returned string is redacted and control-normalized. A malformed SDK
  response returns a static `live_access_failed` error with no response value.
- Config, proxy, credential, unsupported, live, context, reader, and generic
  failures expose only static `MachineError` values and safe sentinels;
  credential names are allow-listed and copied.
- Requests and results reject direct JSON, result slices are recursively
  copied, and nil classification slices leave the result as non-nil empty
  arrays for adapter stability.
- Supported CLI and `machine.v1` surfaces and all generated artifacts remain
  unchanged.

## Tests Run

Builder verification, all passing at the applicable reviewed heads:

- focused URL request/response, runtime, CLI, and command tests with
  `-count=20`
- focused runtime/CLI race tests
- non-cached `go test` over `internal/runtime`, `internal/machine`,
  `internal/cli`, and `cmd/zscalerctl`
- `git diff --check`
- repeated clean `env -u GOFLAGS make check` runs, including a final run at
  `0a8bf49`; repository-wide tests/race, vet, staticcheck, vulnerability scans,
  docs, machine-contract/schema, core/SDK/experiment boundaries, secret scan,
  workflow, surface, release, generated-artifact, and skill-sync checks passed

Fresh-context verification additionally included targeted raw and escaped URL
probes through the real command boundary, focused count/race tests, and the
machine-contract, docs, core-boundary, surface, and generated-skill gates.

## Known Deferrals

- typed dump and diff capabilities
- a versioned stdio protocol, transport DTOs, framing, cancellation frames, and
  reference clients
- frontend, MCP, Wails, Ink/OpenTUI, Ratatui, or GUI adapters
- a public Go package or supported engine wire contract

## Review Focus

- capability advertisement versus executable behavior and effects
- pre-config validation ordering and whole-batch atomicity
- Go URL parser differences between absolute and bare-host forms
- literal, boundary-trimmed, percent-encoded, percent-decoded, invalid UTF-8,
  C0/C1/Cf/bidi, and Unicode-whitespace variants
- userinfo/query/fragment stripping on request and SDK response
- result copying, nil handling, redaction, and terminal-control safety
- raw config/provider/backend/path/credential error leakage and safe sentinel
  preservation
- CLI JSON/table/pretty/NDJSON/usage/envelope/exit compatibility
- supported and generated artifact drift

# Finding Resolution

## Boundary controls disappeared before validation

Finding: the initial normalizer called broad whitespace trimming before its
control scan, so leading or trailing C0/C1 characters could disappear and both
request and SDK-response values would be accepted.

Root cause: `strings.TrimSpace` removes several unsafe runes before later code
can observe them.

Fix: scan the original string before any trimming and accept only surrounding
ASCII spaces.

Regression test: request, direct runtime, engine-before-config, and SDK-response
tables cover leading/trailing C0 and C1 values and assert no reader/config call
or response canary leakage.

Verification: repeated focused tests/race and the first fresh-context recheck
confirmed the original bypass no longer reproduces.

## Bare-host userinfo was parsed as a path

Finding: `user@example.com` in the supported bare-host form is a relative path
to `net/url`, not `URL.User`, so the absolute-URL userinfo stripping did not
apply. Percent-encoded variants behaved equivalently.

Root cause: the broad relative-reference acceptance did not validate the first
bare-path segment as a host grammar.

Fix: validate and unescape the bare host segment independently, rejecting raw
or escaped userinfo/delimiter/control/invalid-UTF-8 forms. Root-relative,
scheme-relative, and hostless scheme-only references are explicitly rejected.

Regression test: raw and escaped bare userinfo, escaped separators/controls,
dot hosts, invalid UTF-8, and malformed references fail before reader/config;
the same SDK-response values fail with a static live sentinel.

Verification: the final reviewer confirmed all bare-host probes remain covered.

## Percent-decoded unsafe values bypassed the raw scan

Finding: `net/url.Parse` decoded path/host escapes after the raw scan, and
`URL.String` re-escaped them. Encoded C0, C1, bidi/format, invalid UTF-8, invalid
absolute hosts, and Unicode whitespace therefore reached config loading and
were accepted from SDK responses.

Root cause: the boundary validated only the original encoded representation,
not retained parsed components.

Fix: revalidate decoded paths and absolute/bare hosts for UTF-8, controls,
format runes, whitespace, and unsafe host delimiters before stripping transient
URL fields and serializing the canonical value.

Regression test: absolute and bare `%00`, `%C2%85`, `%E2%80%AE`, `%ff`, invalid
absolute-host, NBSP/U+2002 host/boundary, and equivalent SDK-response cases are
covered. A `%20` path and surrounding ASCII spaces remain positive controls.

Verification: the reviewer reproduced the original bypasses, requested the
fix, and then confirmed all but decoded non-ASCII path whitespace were closed.

## Decoded non-ASCII path whitespace remained

Finding: the first decoded-path predicate rejected invalid UTF-8 and control/Cf
runes but still admitted `%C2%A0` and `%E2%80%82`.

Root cause: Unicode separators are not control or format runes.

Fix: the decoded-path predicate now rejects all non-ASCII Unicode whitespace
while retaining ordinary encoded ASCII `%20` path spaces.

Regression test: absolute and bare NBSP/U+2002 requests fail before config;
equivalent SDK responses return static `live_access_failed`; `%20` remains
accepted.

Verification: the focused final recheck found no blocker or residual risk.

# Adversarial Review

Fresh-context reviewer: Kant (`gpt-5.6-terra`, xhigh,
`019f5083-8a3c-78a2-87d8-f843f2b90470`)

Initial fresh-context reviewer: Tesla (`gpt-5.6-luna`, xhigh,
`019f5057-99c5-7331-b513-847cdbb19a94`)

Process baseline: `30c5b8e`

Review scope: `30c5b8e..0a8bf49`

Both cited reviewers were read-only and did not implement the change. Tesla
found the pre-trim boundary-control bypass. Kant independently reviewed the
complete later head, reproduced the percent-decoding and Unicode-whitespace
classes through source and command-boundary probes, and performed focused
rechecks after each remediation.

## Blocking Findings

The reviews found the boundary-control, percent-decoded component, and decoded
non-ASCII-whitespace defects described above. The builder's parallel source
audit also found and fixed the bare-host userinfo ambiguity before final
acceptance. The final focused review at `0a8bf49` confirmed every blocker and
changed normalization surface resolved.

## Non-Blocking Risks

The reviewer noted that the mechanical artifact gate accepts any approved
artifact changed since its configured base and is not cryptographically bound
to a commit range. This artifact records the exact base, head, reviewer, and
finding-resolution chain; source verification remains the evidence.

## Machine Contract Review

The candidate `engine.v1` manifest advertises an executable
`zia.url_lookup` capability with conservative effects. Typed request/result
closure, defensive copying, static error mapping, CLI adapter behavior, and
pre-config validation were verified. The supported `machine.v1`,
introspection/schema, JSON/NDJSON, stderr-envelope, exit-code, and golden
surfaces did not change.

## Safety Review

The final review confirmed raw and decoded URL validation, absolute and bare
host policy, request/response credential/query/fragment removal, redaction and
control normalization, safe error boundaries, credential-name allow-listing,
and absence of a production CLI-to-SDK import. No raw probe or canary crossed
the result or error boundary.

## Generated Artifact Review

No generated or frozen artifact changed. CLI-doc, machine-contract, schema,
surface, field-coverage, release-artifact, and generated-skill checks passed.

Verdict: approve
