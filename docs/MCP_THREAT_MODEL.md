# MCP Server Threat Model And Redaction Posture (Roadmap Phase 5 D0)

Status: ACCEPTED for the isolated D1 experiment on 2026-07-15. This acceptance
does not promote an MCP server into the supported v1 surface. It authorizes only
the local stdio experiment constrained by D1-D13 below and still requires the
normal fresh-context review before MCP code lands.

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
   Mitigations are a closed tool allow-list, typed arguments, call and result
   budgets, one active operation, and tenant-read-only construction.
4. **Host-controlled process and environment.** The stdio host spawns the
   server and owns its environment and lifetime. It can observe injected
   credentials and can substitute or wrap the binary. The experiment therefore
   uses the same release-integrity expectations as the CLI, and the official
   MCP Go SDK is a new trusted-computing-base entry that receives an exact,
   reviewed version pin in the nested experiment module.

Credentials do not cross the MCP tool protocol. The host still supplies them to
the server process, as it does for agent CLI use. Projection and redaction occur
inside the trusted Go engine before a tenant value reaches the MCP adapter.

## Accepted decisions

**D1 — MCP defaults to `share` redaction.** The adapter forces `share` even when
the ordinary CLI, profile, or engine default would be `standard`. `standard` is
available only through an explicit MCP-specific server-start setting;
`paranoid` is also allowed. The model cannot select redaction mode per call, an
invalid startup value fails startup, and no unredacted mode exists. This is a
deliberate usability cost: a model provider is an authorized recipient outside
the immediate admin context, which is exactly the audience `share` protects.

**D2 — Raw tenant identifiers require explicit `standard` opt-in.** The adapter
does not invent a second field-classification system. Existing per-mode catalog
rules determine which identifiers survive, but the MCP-specific opt-in in D1 is
required before `standard` can apply.

**D3 — Tool definitions contain public catalog metadata only.** Names,
descriptions, and input schemas may contain release-static products, resource
names, operations, and field names. Tenant values, effective configuration,
paths, credential state, and runtime errors never enter tool definitions.

**D4 — The initial tool surface is closed and operation-specific.** There is no
generic `execute`, capability selector, arbitrary engine request, or shell/CLI
passthrough. The D1 allow-list is:

| MCP tool | Engine capability | Boundary |
| --- | --- | --- |
| `engine_manifest` | `engine.manifest` | Config-free public capability metadata |
| `catalog_schema` | `catalog.schema` | Config-free public catalog metadata; no-argument output is a compact product/resource/operation index, while field detail is scoped |
| `doctor` | `status.inspect` (`doctor`) | Sanitized config-dependent local read; no provider resolution, process execution, or network access |
| `auth_status` | `status.inspect` (`auth`) | Sanitized config-dependent local read; no provider resolution, process execution, or network access |
| `config_status` | `status.inspect` (`config`) | Sanitized config-dependent local read; no provider resolution, process execution, or network access |
| `resource_list` | `resources.read` (`list`) | Projected/redacted tenant values; config-dependent local read and possible network/provider effects |
| `resource_get` | `resources.read` (`get`) | Projected/redacted tenant values; config-dependent local read and possible network/provider effects |
| `resource_show` | `resources.read` (`show`) | Projected/redacted tenant values; config-dependent local read and possible network/provider effects |

The status tools are separate so the model cannot dynamically select a status
operation. Resource arguments may select only catalog-backed product/resource,
ID where applicable, and existing narrowing fields such as `filter`, `search`,
and `fields`. Every request is revalidated by the engine; the adapter never
constructs SDK clients or handles source records.

**D5 — Logging is value-free.** Server logs may include product, resource,
operation, safe error kind, counts, duration, and session-local request ID. They
never include tenant values, IDs supplied to `get`, filters, search strings,
paths, effective configuration values, provider output, credentials, or raw
errors. The default destination is stderr and the default level is warn.

**D6 — Tools only.** D1 exposes no MCP prompts, resources, roots, elicitation,
or sampling. Each is a separate capability and requires a new decision and
review before use.

**D7 — The stdio host is the trust decision.** Stdio has no independently
authenticatable peer: the process that starts the server is the host. A host
allow-list would not add a security boundary. Documentation must state that the
server should run only under a host trusted with the selected redaction mode's
tenant data.

**D8 — Local stdio only.** The experiment has no TCP, HTTP, SSE, streamable-HTTP,
Unix-socket, or other listener. Adding any listener requires a new D-series
decision covering authentication, authorization, multi-client isolation, and
transport security; it is not an implementation toggle.

**D9 — MCP annotations are advisory, not enforcement.** Tools set accurate
`readOnlyHint`, `destructiveHint`, and `openWorldHint` values, but clients may
ignore them. Config-free discovery and local status tools use `readOnlyHint:
true`, `destructiveHint: false`, and `openWorldHint: false`; live resource tools
use the same read/destructive hints and `openWorldHint: true`.

Tenant-bearing results use a declared output schema and `structuredContent`.
When protocol compatibility requires it, the same JSON is also serialized in a
separate text content block. A fixed server-authored tool description warns that
tenant content is untrusted data. No tenant value is interpolated into that
warning, no synthetic warning field is inserted into the machine result, and
any namespaced metadata marker is only defense-in-depth. The hard controls are
D1, D4, D10, D12, D13, and the engine's read-only/projection boundary.

**D10 — One active operation and a finite session call budget.** The adapter
inherits the engine's synchronous operation and bounded retry behavior. It
admits at most one operation at a time and defaults to 100 `tools/call` requests
per MCP session, with a server-start range of 1-1000 and no unlimited value.
Every decoded `tools/call` consumes one unit before tool or argument validation;
errors, cancellation, and timeouts do not refund it. Starting a new process
creates a new session budget. Exhaustion returns an MCP tool error and never
reuses the machine `usage` error kind.

**D11 — Host transcripts are sensitive data stores.** Documentation must tell
operators to choose hosts whose transcript retention and cloud synchronization
they understand, protect transcript directories like sanitized dump output,
and apply the model provider agreement appropriate for their tenant data.
zscalerctl cannot enforce host or provider retention.

**D12 — Security-sensitive authority is fixed at process start.** MCP tool
arguments cannot choose a profile, config path, redaction mode, timeout, cache
policy, provider command, output path, call budget, or result limit. Those are
operator-controlled server-start settings. `cmd:` secret providers are disabled
for MCP by default because an otherwise read-only model call could repeatedly
trigger local process execution. Enabling them requires an explicit
MCP-specific server-start opt-in; an existing global disallow setting wins on
conflict. The model can never supply or alter provider argv.

**D13 — Results are bounded and atomic at the MCP boundary.** Before emitting
any result content, the adapter serializes and checks the complete result.
Tenant-bearing tools default to at most 100 records and 256 KiB of serialized
JSON, whichever is reached first. `get` and `show` retain the same byte bound.
Server-start settings may raise these defaults only up to hard maxima of 1000
records and 2 MiB; tools have no per-call override. A result that exceeds either
bound fails as a value-free MCP tool error with no partial records and advises
the caller to narrow by product/resource, `filter`, `search`, or `fields`.

Config-free discovery is bounded by the same 2 MiB hard byte ceiling. The
default catalog response is the compact index described in D4 so discovery does
not inject the entire field catalog into context; detailed field metadata
requires product or resource scope. A future pagination or continuation design
must be specified and reviewed before replacing fail-closed oversize behavior.

## Explicitly out of scope for D1

- `zia.url_lookup`: it sends model-supplied URL material to an external service
  and needs a separate caller-data disclosure decision.
- `dump.write`: it performs long-running network reads and local
  write/delete/publication effects, and the candidate stdio v1 protocol has no
  wire-visible dump commit marker.
- `diff.compare`: it lets a model select local filesystem paths and may produce
  large tenant-bearing reports.
- Config initialization, arbitrary filesystem access, shell execution, any
  tenant-mutating operation, and any generic engine/CLI passthrough.
- Zidentity or ZDX catalog expansion and every network transport.

These exclusions are policy decisions, not missing-engine work. Each requires a
new threat-model decision before entering an MCP tool allow-list.

## D0 exit and D1 acceptance gates

D0 is complete when this accepted addendum, the base threat model, architecture,
and roadmap agree and the required fresh-context review artifact is approved.
The D1 experiment brief and implementation must inherit D1-D13 verbatim and
must additionally prove:

- an exact reviewed MCP SDK version is pinned in the nested module;
- forbidden-import tests keep the adapter above the trusted engine and out of
  SDK, credential, source-record, CLI, and transport internals;
- the tool allow-list is exact and no network listener is linked or started;
- redaction, command-provider policy, budgets, output preflight, and no-partial
  oversize failures have process-level tests;
- malformed inputs and adapter errors cannot leak arguments or tenant values;
- the experiment remains unreleased and unsupported until the separate D2 host
  workflow proof and promotion decision.
