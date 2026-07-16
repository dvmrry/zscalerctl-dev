# MCP Server Threat Model And Redaction Posture (Roadmap Phase 5 D0)

Status: OWNER-ACCEPTED D0 candidate as of 2026-07-15. D1 implementation is
blocked unless this exact policy delta has an approved fresh-context review
artifact. Satisfying that condition completes D0; it does not promote an MCP
server into the supported v1 surface.

This document extends [THREAT_MODEL.md](THREAT_MODEL.md) for the specific risk
shift an MCP server introduces. The base model's guarantees (tenant-read-only,
allow-list projection, redaction before values leave the trusted engine, and
sanitized errors) remain mandatory.

## What actually changes

CLI output goes to a terminal or pipe selected by the operator. MCP tool output
goes into **model context**: it is transmitted to an LLM provider, may be
retained in host transcripts or logs, may sync to a vendor cloud, and is
interpreted by a model that treats text as potential instructions. Four new
threat classes follow:

1. **Context exfiltration.** Tenant configuration values leave the immediate
   admin context by design. The model provider, host logs, and downstream agent
   actions become additional recipients. Mitigation is minimization and bounded
   disclosure, not access control.
2. **Prompt injection through tenant data.** Free-text tenant fields are
   writable by tenant administrators and enter model context. A hostile or
   compromised co-administrator can plant text that a host model may interpret
   as instructions. zscalerctl cannot make data non-instructional for every
   host; it can remove high-risk free text by default and constrain what any
   confused follow-up call can do.
3. **Agent-mediated misuse.** A model may select the wrong tool, hallucinate
   arguments, repeat calls, or request results too large for useful review.
   Mitigations are a closed tool registry, strict typed inputs, call and result
   budgets, one active operation, and tenant-read-only construction.
4. **Host-controlled process and environment.** The stdio host spawns the
   server and owns its environment and lifetime. It can observe injected
   credentials and can substitute or wrap the binary. The experiment therefore
   uses the same release-integrity expectations as the CLI, and the official
   MCP Go SDK is a new trusted-computing-base entry with an exact reviewed pin.

Credentials do not cross the MCP tool protocol. The host still supplies them to
the server process, as it does for agent CLI use. Projection and redaction occur
inside the trusted Go engine before a tenant value reaches the MCP adapter.

## Accepted policy decisions

**D1 — MCP defaults to `share` redaction.** The adapter forces `share` even when
the ordinary CLI, profile, or engine default would be `standard`. `standard` is
available only through an explicit MCP-specific server-start setting;
`paranoid` is also allowed. The model cannot select redaction mode per call, an
invalid startup value fails startup, and no unredacted mode exists. A model
provider is an authorized recipient outside the immediate admin context, which
is exactly the audience `share` protects.

**D2 — Sensitive identifiers require explicit `standard` opt-in.** Existing
catalog classifications remain authoritative. Fields classified
`sensitive_identifier` are available only when their catalog modes and D1 both
permit `standard`. This does not remove operational record IDs that the catalog
deliberately allows in `share` or `paranoid`; those IDs remain necessary to
correlate otherwise-safe records.

**D3 — Tool definitions contain public adapter metadata only.** Names,
descriptions, and input schemas may contain release-static products, resources,
operations, and field names admitted by D4. Tenant values, effective
configuration, paths, credential state, and runtime errors never enter tool
definitions.

**D4 — A frozen MCP registry, not the engine catalog, defines exposure.** The
nested experiment owns a versioned deny-by-default registry containing exact
tool names and product/resource/operation pairs. It does not auto-expand when
the engine catalog grows. At startup, every registry entry must still exist in
the engine catalog, be tenant-read-only, and carry the expected effects; any
drift fails startup. A committed fixture enumerates the complete registry and a
test rejects additions, removals, duplicates, and unsupported operations.

The initial tenant-bearing registry contains only the current catalog's 23
singleton `show` pairs: 21 ZIA, one ZCC, and one ZTW. ZPA has no current `show`
pair. Zidentity and every collection or ID lookup are excluded.
`server_capabilities` and `catalog_schema` are generated from this MCP registry,
not emitted as the raw engine manifest or full engine catalog, so excluded
capabilities and resources are not advertised.

Here, singleton means the engine's `show` path returns exactly one projected
record. The registry does not infer eligibility from `ResourceSpec.Shape` or
`EffectiveShape()`, whose default is list-shaped even for the current `show`
entries. Standard MCP `tools/list` is generated from the same frozen registry;
it cannot reveal an unregistered tool or raw engine capability.

The exact D1 tools are:

| MCP tool | Engine source | Boundary |
| --- | --- | --- |
| `server_capabilities` | MCP registry verified against `engine.manifest` | Config-free public metadata for this server's tools and effects only |
| `catalog_schema` | MCP registry joined to `catalog.schema` | Config-free metadata for the admitted `show` resources only; no-argument output is a compact index and field detail requires product/resource scope |
| `doctor` | `status.inspect` (`doctor`) | Sanitized config-dependent local read; no provider resolution, process execution, or network access |
| `auth_status` | `status.inspect` (`auth_status`) | Sanitized config-dependent local read; no provider resolution, process execution, or network access |
| `config_status` | `status.inspect` (`config_status`) | Sanitized config-dependent local read; no provider resolution, process execution, or network access |
| `resource_show` | `resources.read` (`show`) | One projected/redacted singleton; config-dependent local read, network access, and fixed-provider helper effects remain possible |

There is no generic `execute`, capability selector, arbitrary engine request,
shell/CLI passthrough, `resource_list`, or `resource_get`. The adapter never
constructs SDK clients or handles source records.

**D5 — Logging is value-free.** Server logs may include tool name, admitted
product/resource, safe error kind, counts, duration, and session-local request
ID. They never include tenant values, fields requested by the model, paths,
effective configuration values, provider output, credentials, raw protocol
frames, or raw errors. MCP SDK debug/protocol logging and tracing are disabled
unless an implementation proves their sink is value-free. The default
destination is stderr and the default level is warn.

**D6 — Synchronous tools only.** D1 exposes no MCP prompts, resources, roots,
elicitation, sampling, server-originated logging messages, or task capability.
It does not advertise `tasks.requests.tools.call` or tool
`execution.taskSupport`. For a negotiated protocol that requires a receiver
without task capability to ignore task-augmentation metadata, the server
processes that request as an ordinary synchronous tool call: it creates no task
state and returns no task handle. Unsupported `tasks/*` methods remain protocol
errors. Each omitted feature requires a new decision and review before use.

**D7 — The stdio host is the trust decision.** Stdio has no independently
authenticatable peer: the process that starts the server is the host. A host
allow-list would not add a security boundary. Documentation must state that the
server should run only under a host trusted with the selected redaction mode's
tenant data.

**D8 — Local stdio and reviewed protocol versions only.** The experiment has no
TCP, HTTP, SSE, streamable-HTTP, Unix-socket, or other listener. Adding any
listener requires a new D-series decision covering authentication,
authorization, multi-client isolation, and transport security; it is not an
implementation toggle.

D1 pins a stable, non-prerelease MCP Go SDK release. The implementation records
and tests every protocol version that pin can negotiate for the D1 feature set.
Transport/schema argument failures and tool-execution failures follow the
negotiated version's official error rules, with a committed fixture for each
admitted version; a version without a proven mapping is disabled.
An SDK bump or newly admitted protocol version requires review of tool schemas,
result/error behavior, stdio framing, and security-sensitive transport changes
before it is enabled.

**D9 — MCP annotations are advisory; explicit DTOs carry results.** Tools set
accurate `readOnlyHint`, `destructiveHint`, and `openWorldHint` values, but
clients may ignore them. Config-free discovery and local status tools use
`readOnlyHint: true`, `destructiveHint: false`, and `openWorldHint: false`.
`resource_show` uses the same read/destructive hints and `openWorldHint: true`.
D12's unconditional `cmd:` prohibition is required for the read-only claim.

Every tool has a committed MCP-specific output schema whose `oneOf` branches
cover both the success and tool-error objects below. The adapter does not
serialize internal engine result types directly. Successful
`structuredContent` is always an object of this form:

```json
{"schema":"zscalerctl.mcp.<tool>.v1","ok":true,"result":{}}
```

The tool-specific `result` object is one of:

- `server_capabilities`: `tools`, admitted `products`, and registry/effect
  metadata;
- `catalog_schema`: a `resources` array containing only admitted metadata;
- each status tool: one sanitized `status` object for its fixed operation;
- `resource_show`: `product`, `resource`, `operation`, and an exact one-element
  `records` array.

Tool execution failures use:

```json
{"schema":"zscalerctl.mcp.<tool>.v1","ok":false,"error":{"kind":"...","message":"..."}}
```

Their `CallToolResult` sets `isError: true`. Budget exhaustion, oversize output,
semantic input rejection after handler admission, cancellation, deadlines, and
engine/business errors all use this value-free form and conform to the error
branch of the tool's output schema. Malformed/unsupported JSON-RPC, an unknown
tool, unknown or wrongly typed arguments, missing required arguments, invalid
schema enums, or another failure before handler admission uses the negotiated
protocol version's value-free JSON-RPC error (including `-32602` where
required), not a `CallToolResult`. Task metadata follows D6 instead of being
treated as a tool argument.

For protocol compatibility, `TextContent.text` is the canonical compact JSON
serialization of the exact `structuredContent` object. Fixtures validate both
representations and every output schema. A fixed server-authored description
warns that tenant content is untrusted data. No tenant value is interpolated
into that warning, and any namespaced metadata marker is defense-in-depth only.

**D10 — One active operation and finite session budgets.** The adapter inherits
the engine's synchronous operation and bounded retry behavior. It admits at most
one operation at a time and defaults to 100 registered tool calls per stdio
session, with a server-start range of 1-1000 and no unlimited value. A call
consumes one unit when its registered handler admits it; semantic argument
errors, cancellation, and timeouts do not refund it. Exhaustion uses the D9
tool-error envelope and does not reuse the machine `usage` error kind.

The bounded transport separately defaults to 1000 inbound frame attempts per
session, with a server-start range of 1-10000 and no unlimited value. It charges
a unit before frame or JSON-RPC validation, so invalid UTF-8, duplicate-key,
depth, trailing-data, oversize, malformed-method, unknown-tool, pre-admission
argument, and unsupported-task-method attempts cannot bypass accounting. Any
invalid or oversize frame closes the session immediately after a value-free
protocol error when a response is possible; the server does not attempt stream
resynchronization. A task-augmented tool request that D6 requires the server to
process normally also consumes a tool-call unit at handler admission. On
message-budget exhaustion the server likewise emits a value-free protocol error
when possible and closes the session. Unknown or invalid calls never reach an
engine or tenant. Starting a new process resets both budgets.

**D11 — Host transcripts are sensitive data stores.** Documentation must tell
operators to choose hosts whose transcript retention and cloud synchronization
they understand, protect transcript directories like sanitized dump output,
and apply the model provider agreement appropriate for their tenant data.
zscalerctl cannot enforce host or provider retention.

**D12 — Security-sensitive authority is fixed at process start, and `cmd:` is
forbidden.** MCP tool arguments cannot choose a profile, config path, redaction
mode, timeout, cache policy, provider, output path, call budget, or result
limit. Those are operator-controlled server-start settings. The experiment
unconditionally disables `cmd:` secret providers; there is no MCP override.
Environment, owner-only file, and platform keyring sources remain subject to
the base credential policy. The model can never supply or alter provider argv.

**D13 — Complete MCP results are bounded and atomic.** Before emitting any
result content, the adapter constructs and exactly encodes the complete
`CallToolResult`, including structured content, duplicate text content, and
metadata. The default maximum is 256 KiB and the hard server-start maximum is
2 MiB. Tools have no per-call override. An oversize result returns only the
small static D9 error envelope; no tenant-bearing partial content is emitted.

`resource_show` returns exactly one record. A singleton that cannot fit within
the configured complete-result ceiling is intentionally unavailable through D1
MCP; the operator must use the CLI or another local engine consumer. This is a
deliberate context-safety tradeoff evaluated again at D2, not silent
truncation. Config-free discovery uses the same bounds, and its compact/scoped
behavior keeps the current catalog practical.

D13 bounds disclosure and adapter serialization. It does not claim to cap the
size of the one upstream API response. Collection `list` and potentially
list-backed `get` operations are excluded because the current engine can fully
buffer those collections before an adapter can enforce a limit. They cannot be
registered until an engine design bounds collection before source-record
accumulation and provides reviewed pagination/continuation semantics.

**D14 — Inputs are bounded before SDK decoding or engine work.** The stdio
transport rejects any complete inbound JSON-RPC message over 64 KiB before the
MCP SDK allocates an unbounded payload. It also rejects invalid UTF-8, duplicate
JSON keys, nesting deeper than 32, and trailing data. If the pinned SDK cannot
enforce those checks before ordinary decoding, D1 supplies a bounded transport
wrapper; this is a launch gate. Every frame attempt, including an empty line,
consumes D10's transport budget before these checks. Every framing failure,
including an over-limit frame without a terminating newline, terminates the
session after bounded draining or immediate close; later bytes can never reach
a handler.

Registered tools reject unknown argument keys before engine construction.
Config-free/status tools have no tenant-controlled string arguments.
`catalog_schema` accepts only registry-backed product/resource enums.
`resource_show` accepts one registry-backed pair plus at most 128 unique `fields`;
each field must be an admitted catalog field and at most 128 UTF-8 bytes. No D1
tool accepts a free-form ID, filter, search string, path, URL, provider, or
operation. Schema/transport rejection occurs before handler admission and uses
the D9 version-specific protocol-error path. A decoded field that fails
registry-dependent semantic validation consumes a tool-call unit and uses the
D9 tool-error envelope. Both paths occur before config/provider/network work and
never echo the rejected value.

## Explicitly out of scope for D1

- `resource_list` and `resource_get`, until collection is bounded before source
  accumulation and pagination/continuation semantics are reviewed.
- Zidentity resources; adding any product/resource/operation requires a frozen
  registry delta and security review rather than catalog auto-expansion.
- `zia.url_lookup`: it sends model-supplied URL material to an external service
  and needs a separate caller-data disclosure decision.
- `dump.write`: it performs long-running network reads and local
  write/delete/publication effects, and candidate stdio v1 has no wire-visible
  dump commit marker.
- `diff.compare`: it lets a model select local filesystem paths and may produce
  large tenant-bearing reports.
- Config initialization, arbitrary filesystem access, shell execution, any
  tenant-mutating operation, generic engine/CLI passthrough, and every network
  transport.

These exclusions are policy decisions, not merely missing adapter wiring. Each
requires a new threat-model decision before entering the MCP registry.

## D0 completion and D1 acceptance gates

The owner has accepted the policy direction. D0 completes only when this
addendum, the base threat model, architecture, and roadmap agree and an approved
fresh-context review artifact covers the exact source delta. The D1 experiment
brief and implementation must inherit D1-D14 and additionally prove:

- exact stable MCP SDK and negotiated protocol-version pins;
- an exact frozen registry fixture and fail-closed startup reconciliation with
  engine capabilities, catalog operations, effects, and field modes;
- forbidden-import tests keeping the adapter above the trusted engine and out
  of SDK, credential, source-record, CLI, and unrelated transport internals;
- no linked or started network listener and no generic request passthrough;
- process-level tests for `share` enforcement, unconditional `cmd:` denial,
  one-operation admission, both budgets, per-version task behavior and
  protocol/tool error fixtures, strict input framing, output preflight,
  schema-conforming success/error objects, exact structured/text DTO
  equivalence, and no partial data;
- with transport budget one, every invalid-UTF-8, duplicate-key, excessive-depth,
  trailing-data, empty/malformed, and oversize frame class consumes the unit,
  closes the session, and prevents a following valid handler call;
- fakes that fail if a D1 tenant tool invokes `List` or `Get`, plus an exact
  assertion that only the 23 reviewed `Show` operation pairs are admitted and
  return one record regardless of catalog shape defaults;
- standard `tools/list`, `server_capabilities`, and `catalog_schema` expose only
  the same frozen registry;
- malformed inputs, dependency diagnostics, and adapter errors cannot leak
  arguments or tenant values;
- the experiment remains unreleased and unsupported until the separate D2 host
  workflow proof and promotion decision.
