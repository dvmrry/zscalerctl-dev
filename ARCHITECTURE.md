# zscalerctl Architecture

`zscalerctl` is a Go CLI for authorized Zscaler administrators to query
configuration, inspect inventory, and create sanitized dumps. The canonical
repo, module, and binary name is `zscalerctl`. Users may define a local `zctl`
alias, but `zscalerctl` remains the shipped command name for clarity in logs,
docs, package managers, and support output.

Implementation should continue to follow the M0 safety baseline in this
document, `THREAT_MODEL.md`, and `DATA_CLASSIFICATION.md`.

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

Planned layout:

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
internal/diff/
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
- Finite retry behavior.
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

Resource fields must be classified before they can render. Non-secret fields
must explicitly list the redaction modes in which they are allowed. Secret-class
fields are never renderable. The projection harness must be able to prove that
rendered records are a subset of the declared allow-list before a resource is
considered ready.

Resource authors must check `ZSCALER_SENSITIVE_DATA.md` before allowing a new
field. Generic names such as `value` are not automatically safe; their
classification depends on the resource schema and endpoint context.

Allowed fields are still scanned before rendering. This catches realistic tenant
data mistakes such as a labeled pre-shared key, PEM key, JWT, provisioning key,
or authorization header pasted into a resource name or description. Scanned
values use typed markers such as `<REDACTED:SECRET>` and
`<REDACTED:PROVISIONING_KEY>` so the operator can tell that data was
intentionally obscured. This scanner is a backstop for self-describing secret
shapes; it must not be treated as a substitute for field allow-list projection.
Free-text fields receive one extra backstop: a conservative high-entropy token
scan for bare unlabeled secret material pasted into administrator-controlled
text. Canonical UUIDs and contextual git commit SHAs are preserved, but other
long hashes or thumbprints may be redacted; the scan does not guarantee
detection of every short unlabeled secret.

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

Initial commands:

```text
zscalerctl doctor
zscalerctl auth status
zscalerctl config show
zscalerctl zia <resource> list|get
zscalerctl zpa <resource> list|get
zscalerctl dump --products zia,zpa --out ./dump
zscalerctl diff ./dump-a ./dump-b
zscalerctl schema list
zscalerctl completion bash|zsh|fish
```

Completion output is static public project data. Generating completions must not
read credential files, initialize the live reader, or contact Zscaler.

Initial global flags:

```text
--profile
--format table|json|yaml|ndjson
--output
--timeout
--redaction standard|share|paranoid
--no-cache
```

There is no `--redaction off`.

## Config And Credentials

Configuration precedence:

```text
CLI non-secret flags > ZSCALERCTL_* environment > profile config > defaults
```

Credential sources:

- `ZSCALERCTL_*` environment variables.
- Strict-permission secret files.
- Future keychain integration.

Secrets should not be accepted as ordinary CLI arguments.

Profile config files should be owner-only. The tool should refuse to load
profile or secret files with unsafe permissions.

The project must define how `ZSCALERCTL_*` variables interact with SDK-native
variables such as product-specific or OneAPI variables before implementation.

## Output And Dumps

Human-readable commands may default to stable tables. Machine-readable workflows
should ask for an explicit format such as JSON or NDJSON.

Pretty output should remain script-safe:

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

By default, dump collection aborts before writing files when any selected
resource fails. `--continue-on-error` is the explicit opt-in for partial dumps:
successful resources are written, `manifest.json` is marked `partial`, failed
resources are represented as `status: error` manifest entries, and
`errors.ndjson` contains value-free error records with product, resource,
operation, and error kind. Credential/session creation failures remain fatal
because they indicate that live read access itself is not trustworthy.

## Redaction Modes

`standard` is for local operational use. It emits allow-listed fields and uses
secret redaction, free-text high-entropy token scanning, and final byte scanning
as backstops.

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

## M0 Decisions Required Before Code

- First supported products and resources.
- OneAPI-first versus legacy-auth support in version 1.
- Exact `ZSCALERCTL_*` credential variable names.
- Which future free-text fields, beyond currently modeled descriptions, may be
  emitted in `standard` mode. New free-text fields must retain scanner backstop
  coverage and catalog-driven canaries.
- Whether `paranoid` mode supports diffs in version 1.
- Required CI gates before public release.
- License.
- Module path, expected to be `github.com/dvmrry/zscalerctl`.
