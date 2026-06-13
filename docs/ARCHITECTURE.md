# zscalerctl Architecture

`zscalerctl` is a Go CLI for authorized Zscaler administrators to query
configuration, inspect inventory, and create sanitized dumps. The canonical
repo, module, and binary name is `zscalerctl`. Users may define a local `zctl`
alias, but `zscalerctl` remains the shipped command name for clarity in logs,
docs, package managers, and support output.

Implementation should continue to follow the M0 safety baseline in this
document, [THREAT_MODEL.md](THREAT_MODEL.md), and
[DATA_CLASSIFICATION.md](DATA_CLASSIFICATION.md).

## Design Principles

- Defensive administration only.
- Read-only by default.
- No secret exposure.
- No raw API dumping by default.
- Allow-list projection before redaction.
- One output pipeline for all formats.
- Scriptable and agent-friendly before interactive.
- Explicit, inspectable behavior over clever implicit behavior.
- Small adapter around the official SDK rather than SDK calls spread through
  command handlers.

## Primary Use Case

The primary use case is CLI and agentic automation: replace duplicated Python
snippets spread across pipelines and workflows with one reviewed, tested,
security-first command.

This project should stay shell-native:

- Stable exit codes.
- Predictable stdout for data.
- Predictable stderr for diagnostics.
- Explicit machine formats.
- Explicit color policy: `--color auto|always|never`, `--no-color`, and
  `NO_COLOR` support.
- Deterministic output for dumps and diffs.
- No interactive prompts in default automation paths.

Polished human output is useful, but a TUI is not a goal until a concrete
workflow proves it is needed.

## Initial Package Shape

Package layout:

```text
cmd/zscalerctl/
internal/cli/
internal/config/
internal/credentials/
internal/secret/
internal/zscaler/
internal/resources/
internal/redact/
internal/output/
internal/dump/
internal/version/
internal/livesmoke/
```

Test-only security helpers should live in `_test` packages or testdata that is
not imported by the production binary.

## SDK Boundary

The official SDK, `github.com/zscaler/zscaler-sdk-go/v3`, should be used behind
an internal adapter.

The CLI should not call SDK methods directly. Command handlers should depend on
small reader interfaces owned by this project.

The adapter is responsible for:

- Explicit SDK configuration from `zscalerctl` config.
- Avoiding surprising SDK config discovery. The live reader builds the SDK
  configuration directly from `ZSCALERCTL_*` values instead of calling SDK
  constructors that read the SDK's own environment variables or local config
  files.
- Explicit HTTP client and request timeouts.
- Direct outbound HTTP by default. The adapter disables ambient
  `HTTP_PROXY`/`HTTPS_PROXY` discovery; proxy support must be added as an
  explicit `zscalerctl` setting if it becomes necessary.
- Finite retry behavior, including rate-limit handling (see Rate Limits And
  Pacing below).
- Context cancellation.
- Cache policy. SDK response caching is currently disabled for all live reads.
- Product-specific clients.
- Normalizing SDK errors before they reach user-facing output.

The SDK remains part of the trusted computing base and must be reviewed for
debug logging, token caching, environment variable behavior, and error contents.
On every SDK version bump, re-verify that `NewOneAPIClient` does not perform
additional environment, file, proxy, cache, or logging discovery when supplied a
manually constructed configuration.
The dependency policy requires vendored modules and a CI check for this SDK
boundary invariant.

The live reader creates an SDK service per single-resource CLI operation and
closes it after the call. Dump commands create one SDK service per selected
product that supports sessions and close it when collection finishes, avoiding
repeated authentication while keeping token lifetime bounded to the command.

## Rate Limits And Pacing

`zscalerctl` keeps its request rate within tenant limits through two explicit,
deliberately conservative mechanisms rather than a custom throttler:

- **Per-request retry/backoff.** The adapter configures the SDK rate-limit
  policy explicitly for both auth paths: `MaxRetries = 2`, `RetryWaitMin = 1s`,
  `RetryWaitMax = 3s` (bounded exponential backoff, honoring `Retry-After` /
  rate-limit responses), plus `MaxSessionNotValidRetries = 1` for OneAPI. A
  request that exhausts these retries surfaces as a normalized live-access
  error, not an infinite stall.
- **Sequential collection.** `dump` reads one product/resource at a time in
  catalog order — there is no client-side fan-out or concurrency. Serialization
  is itself the pacing strategy: at most one in-flight request per command, so
  bursts cannot exceed what the per-request backoff already absorbs.

This is the project's rate-limit policy as of v1: correctness and staying under
limits are favored over dump throughput. Parallel collection would require an
explicit future setting paired with rate-aware throttling and per-tenant
concurrency caps; it is intentionally not enabled by default.

## Data Flow

```text
CLI command
  -> config and credential loader
  -> Zscaler adapter
  -> resource reader
  -> resource-specific safe view projection
  -> redaction and final byte scanning
  -> renderer
  -> stdout, stderr, or restricted file writer
```

Raw SDK responses must not be sent directly to renderers.

The code enforces this with separate source and projected record types. Resource
readers return opaque source records. The projection layer consumes those source
records and produces projected records. Renderers accept projected or otherwise
explicitly safe output types, not arbitrary maps or raw strings.

## Resource Model

Each supported resource should define:

- Product, such as ZIA or ZPA.
- Resource name.
- Supported operations, initially list and get.
- Pagination behavior.
- Cache behavior.
- Safe view struct.
- Field classification.
- Sort key for deterministic output.
- Redaction behavior for each supported mode.

Unknown resources and unknown fields are excluded by default.

Resource readers should map the SDK response shape into source records without
using the reader as a second safety allow-list. They may normalize SDK field
names and pointer types into JSON-shaped source data, but fields should not be
omitted merely because they are sensitive or not currently rendered. The
resource catalog and projection layer are the auditable allow-list; the reader
is an adapter, not a sanitizer.

Mapped SDK struct shapes are reviewed by reflection tests. Every exported SDK
JSON field for a mapped resource or nested helper must either correspond to a
catalog-classified field or appear in an explicit ignored-field registry with a
reason. SDK bumps and new resources must update that registry intentionally, so
new SDK response fields cannot drift in silently.

Nested API objects are excluded unless the resource spec explicitly models their
nested fields. An allowed top-level field does not implicitly allow every child
field inside a map or list of maps.

When a nested API object has, or plausibly should have, its own dedicated
resource, parent resources should render only a reviewed `id`/`name` reference
instead of re-expanding that object's child graph. The dedicated resource owns
the authoritative field classification for that object; parent resources show
the association without creating a second, potentially more permissive
allow-list.

Resource fields must be classified before they can render. Non-secret fields
must explicitly list the redaction modes in which they are allowed. Secret-class
fields are never renderable. The projection harness must be able to prove that
rendered records are a subset of the declared allow-list before a resource is
considered ready.

Resource authors must check
[ZSCALER_SENSITIVE_DATA.md](ZSCALER_SENSITIVE_DATA.md) before allowing a new
field. Generic names such as `value` are not automatically safe; their
classification depends on the resource schema and endpoint context.

Allowed fields are still scanned before rendering. This catches realistic tenant
data mistakes such as a labeled pre-shared key, PEM key, JWT, provisioning key,
or authorization header pasted into a resource name or description. Scanned
values use typed markers such as `<REDACTED:SECRET>` and
`<REDACTED:PROVISIONING_KEY>` so the operator can tell that data was
intentionally obscured. This scanner is a backstop for self-describing secret
shapes; it must not be treated as a substitute for field allow-list projection.
Rendered string values receive one extra backstop: a conservative high-entropy
token scan for bare unlabeled secret material. Canonical UUIDs are preserved
everywhere. In `standard` mode, structured rendered strings also preserve compact
UUIDs and 40/64 character hex fingerprints; `share` and `paranoid` redact those
fingerprint-shaped values. Free-text prose preserves git commit SHAs only when
nearby words identify them as git revisions. The scan does not guarantee
detection of every short unlabeled secret below the 32-character entropy floor,
or of every hex-shaped secret in a rendering `standard` field.

## Secret-Safe Types

Credential values should flow through secret-safe types where practical.

The secret type should avoid revealing the wrapped value through:

- `String()`
- `GoString()`
- JSON marshaling
- text marshaling
- structured logging
- error formatting

This does not make Go memory zeroization perfect. It does make accidental output
leaks harder and easier to test.

## Command Model

Commands:

```text
zscalerctl doctor
zscalerctl auth status
zscalerctl config show
zscalerctl <product> <resource> list|get|show   # ops vary by resource
zscalerctl zia url-lookup <url> [url...]        # diagnostic lookup
zscalerctl dump --products zia,zpa --out ./scratch-live-dump
zscalerctl schema list
zscalerctl version
zscalerctl completion bash|zsh|fish|powershell
```

`diff` (compare two dump directories) is planned, not yet implemented. There is
no code for it yet.

### Diagnostic lookups

Some diagnostics answer a question about live data without reading a tenant
resource. `zia url-lookup` is the first instance: it resolves the URL category
classifications for the URLs the caller supplies. These are natural-verb
diagnostics like `doctor` and `auth status`, not catalog resources — they have
no list/get/show operations and no schema-registry entry, and their output is a
hand-built output-safe struct rendered through the normal renderer so redaction
still applies.

Posture decision: diagnostic verbs may call query-only endpoints regardless of
HTTP method. The urlLookup endpoint uses POST, but the request body is purely
the query input (the URLs to classify) and the call creates, modifies, and
deletes nothing. Diagnostic verbs must never call endpoints that mutate tenant
state, whatever the method. This does not change the read-only guarantees in
THREAT_MODEL.md, which already frames the posture as read/query semantics
rather than HTTP verbs.

Completion output is static public project data. Generating completions must not
read credential files, initialize the live reader, or contact Zscaler.

Global flags:

```text
--profile
--format auto|table|json|pretty
--output
--timeout
--redaction standard|share|paranoid
--color auto|always|never
--no-color
--no-cache
--log-level off|error|warn|info|debug
--fields a,b,c
```

There is no `--redaction off`.

## Config And Credentials

Configuration precedence:

```text
CLI non-secret flags > ZSCALERCTL_* environment > defaults
```

Credential sources:

- `ZSCALERCTL_*` environment variables.
- Strict-permission secret files.
- Future keychain integration.

Secrets should not be accepted as ordinary CLI arguments.

Secret files (the `*_FILE` variables) must be owner-only; the tool refuses to
load a secret file with unsafe permissions. Configuration is environment-only
today — there is no profile config file source.

Resolved: `ZSCALERCTL_*` variables never interact with SDK-native variables.
The adapter constructs SDK configuration explicitly from `ZSCALERCTL_*` values
and avoids the SDK constructors that discover the SDK's own environment
variables or config files; SDK-native variables the SDK consults at request
time are neutralized at client construction.
`scripts/verify-sdk-boundary.sh` regression-checks this boundary against the
vendored SDK.

## Output And Dumps

The default format is `auto`: it resolves to the human-readable `pretty`
renderer when stdout is a terminal and to `json` otherwise (pipe, redirect, or
`--output` file), so interactive use is readable while pipelines and agents get
deterministic JSON without passing a flag. `--format` can override it with an
explicit `table`, `json`, or `pretty`.

The `pretty` renderer is a presentation overlay only: it consumes the same
already-projected, already-redacted records as every other format and adds no
new data path. Its output still passes through the final redaction byte-scan
before stdout, and it is byte-clean (no ANSI escapes) whenever color is off.

Pretty output remains script-safe:

- No color unless output is a TTY.
- Respect `NO_COLOR` and provide `--no-color` before adding color.
- Support common 256-color terminals when color is enabled.
- Never depend on color alone to communicate status or risk.
- No spinners or progress animations on machine-readable output.
- No layout that changes field meaning based on terminal width.
- Prefer compact, keyboard-friendly CLI ergonomics over multi-pane or
  editor-like interaction models.
- Data stays on stdout; diagnostics stay on stderr.

Dump layout:

```text
dump/
  manifest.json
  redaction_report.json
  resources/
    zia/
      <resource>.json
    zpa/
      <resource>.json
  errors.ndjson        # partial dumps only
```

Dump writers should:

- Use restrictive permissions.
- Refuse unsafe overwrite by default.
- Write temporary files and rename atomically.
- Avoid partial successful output looking complete.
- Include enough manifest data to support review and diff workflows.

Large tenants: dump output is fully buffered in memory. List collection
accumulates every record, output marshals the entire payload, and the final
redaction pass byte-scans a second full copy of the serialized bytes, so
expect peak memory of several times the serialized dump size (the pinned
baseline observes roughly 7-8x once record and projection overhead is
included). `TestLargeTenantDumpBaseline` in `internal/dump` pins this
baseline with a synthetic multi-thousand-record tenant and fails if peak heap
growth ever exceeds 20x the serialized output size.

By default, dump collection aborts before writing files when any selected
resource fails. `--continue-on-error` is the explicit opt-in for partial dumps:
successful resources are written, `manifest.json` is marked `partial`, failed
resources are represented as `status: error` manifest entries, and
`errors.ndjson` contains value-free error records with product, resource,
operation, and error kind. Credential/session creation failures remain fatal
because they indicate that live read access itself is not trustworthy.

## Redaction Modes

`standard` is for local operational use. It emits allow-listed fields and uses
secret redaction, rendered-string high-entropy token scanning, and final byte scanning
as backstops. Free-text fields are standard-only catalog exceptions: each one
must include a `standard_free_text_reason`, keep catalog-driven canary coverage,
and stay out of `share` and `paranoid` unless a future tokenization design
explicitly changes that policy.

`share` is for sharing with an authorized recipient outside the immediate admin
context. It removes or masks sensitive identifiers and high-risk free text.

`paranoid` is for maximum minimization. Version 1 should not promise cross-dump
diffability in paranoid mode unless key management and tokenization are designed
first.

## Future Writes

Writes are out of scope for the initial implementation. If added later, they
must use separate verbs, explicit enablement, allow-lists, dry-run behavior, and
confirmation. Read and write paths must not be accidentally interchangeable at
the command layer.

## Foundation Decisions

The implementation is read-only and uses allow-listed projections, explicit
redaction modes, SDK boundary checks, and live-smoke promotion gates. ZIA
legacy auth remains supported for the ZIA surface already proven with that auth
mode. OneAPI is the expansion path for ZPA, ZTW, and other product families.

`paranoid` mode does not promise stable cross-dump diffability unless a future
tokenization and key-management design makes that safe.
