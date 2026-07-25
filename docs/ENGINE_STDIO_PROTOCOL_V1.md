# Local Stdio Engine Protocol v1 — Candidate Implementation

Status: CANDIDATE IMPLEMENTATION. The Go codec/host and zero-runtime-dependency
TypeScript reference client implement this cross-language local transport over
the accepted Go engine. The command and client are not packaged in releases and
the protocol is not yet a supported surface. This checkpoint does not change or
promote the supported CLI, `machine.v1`, or existing machine JSON envelopes.

The normative JSON shapes are:

- [engine-stdio-bootstrap.schema.json](schema/engine-stdio-bootstrap.schema.json)
- [engine-stdio-v1.schema.json](schema/engine-stdio-v1.schema.json)

Those version-specific shapes are immutable once this checkpoint lands. A
semantic or structural incompatibility requires a new offered protocol version
and schema file.

The schemas use non-resolving immutable URN identities rather than branch URLs:
`urn:zscalerctl:engine-stdio:bootstrap:1` and
`urn:zscalerctl:engine-stdio:protocol:1`. Clients bundle the bootstrap schema.
After negotiation, `ready.schema` carries the selected schema URN and the
lowercase SHA-256 of its exact checked-in bytes; clients compare both against
their bundled copy before accepting any request lifecycle frame. A schema hash
mismatch is an incompatible host, not permission to fetch replacement schema
bytes from the network.

## Boundary

```text
Ink / Ratatui / GUI
        |
        | strict NDJSON over child stdin/stdout
        v
stdio host -> runtime.Engine -> projection/redaction -> official Zscaler SDK
```

The candidate host is `cmd/zscalerctl-engine`. Build it explicitly for local
consumer development:

```sh
go build -o ./zscalerctl-engine ./cmd/zscalerctl-engine
```

It accepts only process-start policy flags: `--profile`, `--config`,
`--timeout`, `--redaction`, and `--no-cache`. Credentials remain inherited
environment/config inputs to the trusted Go runtime and never cross the wire.
Consumers must keep stdin open until the active request reaches a terminal;
stdin EOF is a session-shutdown request, not an end-of-batch marker.

The protocol is a child-process stdio API. There is no listener, HTTP server,
WebSocket, daemon, or credential API. The host inherits its environment and
receives profile, config path, HTTP timeout, redaction, and cache-bypass policy
only at process start. Changing one restarts the host.

Credentials, environment entries, secret references, provider commands,
tokens, SDK clients/models, HTTP data, source records, raw backend errors, and
local paths never appear in server frames. Stderr is silent by default and may
carry only closed, value-free diagnostic codes when explicitly enabled.

The bootstrap protocol identifier is `zscalerctl.engine.stdio`. Wire version
`1` is independent from in-process `engine.v1` discovery and supported CLI
`machine.v1`.

## Capability set

| Capability | Operations | Delivery |
| --- | --- | --- |
| `engine.manifest` | `manifest` | completed manifest result |
| `catalog.schema` | `list` | one `catalog_resource` item per resource |
| `status.inspect` | `doctor`, `auth_status`, `config_status` | completed typed status result |
| `zia.url_lookup` | `lookup` | one `url_classification` item per answer |
| `resources.read` | `list`, `get`, `show` | one `projected_record` item per record |
| `dump.write` | `dump` | progress, value-free warnings, completed dump summary |
| `diff.compare` | `diff` | progress, bounded diff items, completed diff summary |

Config editing, credentials, arbitrary queries, raw API calls, tenant mutation,
server-held cursors, and session control are absent from v1.

The lifecycle matrix is closed and stateful:

| Capability | Operation | Semantic item kinds | Progress | Warning | Completed result kind |
| --- | --- | --- | --- | --- | --- |
| `engine.manifest` | `manifest` | none | no | no | `engine_manifest` |
| `catalog.schema` | `list` | `catalog_resource` | no | no | `catalog_summary` |
| `status.inspect` | `doctor` | none | no | no | `doctor_status` |
| `status.inspect` | `auth_status` | none | no | no | `auth_status` |
| `status.inspect` | `config_status` | none | no | no | `config_status` |
| `zia.url_lookup` | `lookup` | `url_classification` | no | no | `url_lookup_summary` |
| `resources.read` | `list` | `projected_record` | no | no | `resource_read_summary` |
| `resources.read` | `get` | `projected_record` | no | no | `resource_read_summary` |
| `resources.read` | `show` | `projected_record` | no | no | `resource_read_summary` |
| `dump.write` | `dump` | none | yes | yes | `dump_summary` |
| `diff.compare` | `diff` | `diff_resource`, `diff_added`, `diff_removed`, `diff_field_change` | yes | no | `diff_summary` |

For an accepted request, `started` must repeat its exact capability/operation
pair. Inline and fragmented item kinds, progress, warnings, and a successful
result must then match that row. `failed` and `canceled` are the only terminal
alternatives. The frame schema closes every individual shape; the session
validator enforces this request-relative matrix. Any mismatch is a fatal
protocol violation, never an unknown extension to ignore.

## Bootstrap and negotiation

Bootstrap frames have a permanent 64 KiB byte limit and maximum JSON depth 8.
The server always writes LF; client bootstrap input may use LF or CRLF. The
bootstrap shapes and interpretation never change when another engine protocol
version is added.

The first server frame is `hello`. Its `versions` are bounded opaque ASCII
tokens in server-preference order. A client ignores tokens it does not support,
selects the first mutually supported token, and echoes both protocol and
version in `initialize`.

```json
{"type":"hello","protocol":"zscalerctl.engine.stdio","versions":["1"],"bootstrap":{"frame_bytes":65536,"json_depth":8}}
{"type":"initialize","protocol":"zscalerctl.engine.stdio","version":"1"}
```

If there is no common version, a conforming client sends the immutable bootstrap
rejection frame instead of guessing a version:

```json
{"type":"reject","protocol":"zscalerctl.engine.stdio","reason":"unsupported_protocol"}
```

The server then writes one static fatal `unsupported_protocol` frame and exits.
An unsupported `initialize` receives the same response. EOF before either
`initialize` or `reject` means the client abandoned negotiation; the server
exits without claiming a negotiated outcome.

A successful initialization produces `ready` under the selected v1 schema.
`ready` identifies the immutable schema bytes, build, negotiated limits, and
explicitly converted engine manifest. The following is a structurally complete
minimal frame; a build advertises every capability it actually provides.

```json
{"type":"ready","protocol":"zscalerctl.engine.stdio","version":"1","schema":{"id":"urn:zscalerctl:engine-stdio:protocol:1","sha256":"6cba5a8170e538bd6eacde38c84526873f691421d6dc5f57cacfbd5f9438c522"},"server":{"name":"zscalerctl-engine","version":"dev"},"limits":{"client_frame_bytes":1048576,"server_frame_bytes":1048576,"json_depth":64,"aggregate_item_bytes":67108864,"fragment_chunk_bytes":524288,"url_count":1024,"read_field_count":1024,"read_filter_count":1024,"product_selector_count":16,"resource_selector_count":4096,"path_bytes":32768,"control_string_bytes":8192},"engine":{"version":"engine.v1","tenant_read_only":true,"capabilities":[{"name":"engine.manifest","operations":["manifest"],"input":"none","result":"engine_manifest","tenant_read_only":true,"effects":[]}]}}
```

No request is accepted before `ready`. Unknown bootstrap fields, duplicate
keys, invalid UTF-8, an unterminated final line, or another frame type are fatal.

## V1 limits and JSON rules

Negotiated v1 limits are fixed:

| Limit | Value |
| --- | ---: |
| client frame | 1 MiB |
| server frame | 1 MiB |
| JSON depth | 64 |
| aggregate semantic item | 64 MiB |
| raw fragment chunk | 512 KiB |
| URLs per lookup | 1,024 |
| fields or filters per read | 1,024 each |
| product selectors | 16 |
| resource selectors | 4,096 |
| path bytes | 32,768 |
| general control string bytes | 8,192 |

`aggregate semantic item` is a per-item limit, not a total response limit. V1
has no total-response byte field, and its atomic whole-response barrier means
host memory can scale with the admitted item count and the number of fragmented
payloads. Consumers must not interpret the 64 MiB value as a process RSS bound;
the candidate benchmark records this limitation explicitly.

All structural integers (`id`, `seq`, item IDs, chunk indexes, lengths, and
counters) are integral and in `0..9007199254740991`; request IDs begin at 1.
Collection-specific minimum/maximum values are in the v1 schema.

Framing and parsing rules:

- One UTF-8 JSON object per line. Server output is LF. Client input accepts LF
  or exactly one CR immediately before LF; bare CR is invalid.
- A final unterminated line, empty line, BOM, top-level non-object, trailing
  JSON, invalid scalar value, or unpaired surrogate escape is fatal.
- Root object depth is 1; every nested object or array increments depth.
- Duplicate object keys are rejected after escape decoding, case-sensitively.
- Unknown fields are rejected in every control object.
- Input byte limits are enforced before a line reader can grow an unbounded
  buffer. Node/Bun clients split bytes on `0x0A` before fatal UTF-8 decoding.
- Structural strings reject C0/C1 controls, Unicode format runes, NUL, invalid
  UTF-8, and values beyond their byte limits. Paths are accepted only as input
  and never echoed.
- Dynamic object keys at every nesting depth reject the same C0/C1 controls and
  Unicode format runes. Dynamic values remain lossless, untrusted data: clients
  must apply sink-specific escaping at terminal, HTML, log, and similar
  presentation boundaries rather than altering the wire value.

Dynamic projected/diff JSON numbers retain their exact RFC 8259 decimal lexeme.
They are not coerced through `float64`. Go uses `json.Number`; TypeScript uses a
lossless parser and exposes `WireNumber`; Rust retains the source lexeme or an
equivalent arbitrary-precision representation. Structural numbers are then
validated and converted to safe native integers.

## Client requests

A request is a closed capability/operation union, not an option map:

```json
{"type":"request","id":1,"capability":"resources.read","operation":"list","input":{"product":"zia","resource":"locations","fields":[],"filters":[],"search":""}}
```

IDs are strictly increasing positive safe integers for the process lifetime,
including rejected requests. The host stores only the greatest observed ID, so
the rule prevents ABA cancellation races without unbounded ID retention.

Operation inputs are exact:

- `engine.manifest/manifest`, `catalog.schema/list`, and status operations have
  no `input` member.
- `resources.read/list`: product/resource plus required arrays `fields` and
  `filters` and required `search`; `record_id` is forbidden.
- `resources.read/get`: product/resource/record_id plus required `fields`;
  filters and search are forbidden.
- `resources.read/show`: product/resource plus required `fields`; record ID,
  filters, and search are forbidden.
- `zia.url_lookup/lookup`: non-empty bounded `urls`.
- `dump.write/dump`: output directory, product/resource selector arrays, and
  explicit `continue_on_error`/`force` booleans.
- `diff.compare/diff`: old/new directories, selector arrays, and explicit
  `ignore_operational`/`allow_partial` booleans.

The common envelope is decoded first; its raw input is then strictly decoded as
the exact operation DTO before dispatch. Raw input never becomes a downstream
generic map.

Only one operation runs. A fully decoded and validated second request receives:

```json
{"type":"request_rejected","id":2,"reason":"busy"}
```

It has no sequence number and starts no lifecycle. Malformed input remains
session-fatal even while busy. The reference client queues and normally does
not exercise rejection.

Cancellation names the active request:

```json
{"type":"cancel","id":1}
```

It is idempotent. An unknown, inactive, or outcome-committed ID is ignored.
Cancellation wins only when the coordinator linearizes it before an outcome
commit. A success commit occurs after whole-response serialization preflight
and before the first semantic item; after it, cancellation is ignored and the
precommitted success stream wins.

`dump.write` also has an earlier internal effect boundary around the atomic
directory publication attempt. Cancellation before that boundary prevents the
attempt. Cancellation selected while the atomic operation is in flight is
deferred: a failed attempt releases it, while a successful attempt commits the
local effect and cancellation can no longer replace success. The request and
process stay alive through forced-replacement quarantine cleanup, even if
normal cancel or shutdown watchdog durations elapse. A broken output transport
after this boundary is an indeterminate delivery failure, not a canceled
filesystem operation. Protocol v1 has no separate wire acknowledgment for
either commit boundary.

## Wire lifecycle

Engine and wire lifecycles are distinct.

Session states:

```text
bootstrap -> ready -> closing -> dead
```

Request states:

```text
idle -> accepted -> running
running -> failed_committed   -> terminal_write_pending -> terminal_written -> idle
running -> canceled_committed -> terminal_write_pending -> terminal_written -> idle
running -> success_committed  -> streaming_success -> terminal_write_pending
        -> terminal_written -> idle
```

The coordinator emits the sole wire `started`. It suppresses and validates
internal engine `started`/terminal events, converts only safe non-terminal
events, reconciles the typed method's returned result/error with the internal
terminal, and commits exactly one wire outcome. Runtime-construction errors
before an internal stream become wire `failed` outcomes. Outcome commit is the
ordinary cancellation linearization point. Dump's internal effect commit can
make cancellation non-winning earlier, while the request remains active through
cleanup and `terminal_written`. A request remains active for busy rejection
purposes through `terminal_written`.

The decoder stops granting new reads once a terminal is queued. One read permit
may already be blocked in the decoder; if that complete valid request arrives
before terminal submission it is classified `busy`, while a request arriving
during the terminal write is held until the writer acknowledges the complete
terminal. This removes the transport race where a client can observe the full
terminal bytes and send its next request before the coordinator selects the
writer acknowledgment. Malformed input remains fatal in either interval.

While the session is writable, the coordinator attempts one complete terminal
write. Delivery cannot be guaranteed across a broken pipe, fatal session input,
or process termination. EOF before a complete terminal is an indeterminate
transport failure; it is never interpreted as operation success/failure.

Every accepted request begins with:

```json
{"type":"started","id":1,"seq":1,"capability":"resources.read","operation":"list"}
```

Subsequent frames increment `seq`. `completed`, `failed`, or `canceled` is the
last committed request frame. No request frame follows it.

Fatal session errors preempt an active request: the coordinator cancels it,
commits no further request outcome, attempts one fatal protocol frame if the
stream remains writable, and the client discards all in-flight state.

## Success barrier and atomic item delivery

Every item-producing operation has a whole-response success/serialization
barrier. Before the first semantic item, the worker must finish the engine
operation, convert and defensively copy every admitted item, validate every DTO,
compute each exact deterministic JSON byte length and fragment plan, compute
fragment digests, validate the operation-specific summary, and continue
observing cancellation. No semantic item is submitted to the writer during
this preflight.

The coordinator then linearizes either cancellation/failure or success. A
preflight cancellation or failure emits zero semantic items. An oversized item
fails with `response_too_large` before any item is visible. On success commit,
the immutable item plans and terminal result are fixed; cancellation is ignored
from that point and the coordinator emits the complete item stream followed by
`completed`.

The post-commit encoder is a deterministic second pass over the copied DTOs.
Any impossible size/digest mismatch or encoding failure after success commit is
a fatal session `internal` error, and a write failure is an indeterminate
transport failure. Neither can be reclassified as request `failed` or
`canceled` after visible items.

Resource reads are therefore atomic: a failed or canceled list/get/show emits
zero `projected_record` items. Current upstream pagination, projection,
verification, filtering, and search complete before preflight. Page-by-page
early records would expose partial results and therefore require a new protocol
version or a separate explicitly non-atomic capability. Catalog, URL lookup,
and diff semantic items use the same barrier. Value-free dump/diff progress and
dump warnings are provisional and may precede a failed or canceled terminal;
dump has no semantic item stream.

SDK/token reuse may remain internal only if result, effects, invalidation,
cancellation, and one-active-request semantics remain identical. Wire cursors,
result sessions, or drill-down handles are protocol changes.

## Items and fragmentation

Small semantic items use an inline `item` frame. Kinds are:

- `catalog_resource`
- `url_classification`
- `projected_record`
- `diff_resource`
- `diff_added`
- `diff_removed`
- `diff_field_change`

Resource records are converted only from trusted engine events; dynamic values
are defensively copied and encoded by the transport's closed value encoder.
Dump record events are acknowledged and suppressed before DTO conversion,
sizing, logging, or writer submission.

Diff item order follows the admitted report: one `diff_resource`, then added,
removed, and one item per changed field. Old/new dump references use fixed
`side: old|new` labels and omit caller paths. Protocol v1 emits a
`diff_resource` only when both sides supplied records and the resource was
actually compared. When `allow_partial` admits a selected resource whose
collection failed, the in-process report retains its explanatory note while
the v1 stream omits that note-only entry so `resources_compared` remains
literal and reconcilable.

The `diff_resource.identity` discriminant controls its following items.
`get_key` requires `field`; `singleton` and `content_hash` forbid it. Added and
removed records use `key` references for `get_key` and `singleton` (the latter
uses the literal key `singleton`) and `hash` references for `content_hash`.
`diff_field_change` is valid only for `get_key` and `singleton`; content-hash
changes are represented as a remove/add pair. Key and hash references are
mutually exclusive.

If an item cannot fit one server frame, it uses a contiguous fragment sequence:

```json
{"type":"item_begin","id":1,"seq":2,"item_id":1,"kind":"projected_record","encoding":"json","bytes":9000000}
{"type":"item_chunk","id":1,"seq":3,"item_id":1,"index":0,"data":"...base64..."}
{"type":"item_end","id":1,"seq":4,"item_id":1,"chunks":18,"sha256":"..."}
```

The payload bytes are exactly the JSON object that would occupy the inline
`item` member. Item IDs increase within a request. Begin/chunks/end are not
interleaved with another item/progress/warning. Whole-response preflight uses a
bounded counting encoder, then the post-commit pass streams through the chunker
and verifies the precomputed digest; it never allocates a second aggregate-sized
JSON buffer merely to discover an item's size.

Chunk `data` is canonical RFC 4648 base64 produced by Standard Encoding: no
whitespace, no omitted required padding, and no nonzero trailing bits. A client
strictly decodes and then requires byte-for-byte equality with re-encoding.
Each decoded chunk is 1..524,288 bytes. Every nonfinal chunk is exactly 524,288
bytes; the final chunk is 1..524,288 bytes. Indexes are contiguous from zero,
`item_end.chunks` equals `ceil(item_begin.bytes / 524288)`, the final index is
`chunks - 1`, and the sum of decoded chunk lengths equals `item_begin.bytes`.
The maximum 64 MiB item therefore has exactly 128 chunks.

An item is valid only after `item_end`, exact length/chunk count/digest checks,
lossless strict JSON decode, and validation against the schema selected by
`kind`. Partial items are discarded on fatal session error or EOF. The success
barrier means an operation terminal failure or cancellation cannot follow a
partial semantic item. An item beyond the aggregate limit produces an operation
`failed(response_too_large)` before success commit, not a session protocol
error.

## Progress and warnings

Dump and diff progress means “this catalog resource is about to begin,” matching
the engine's current pre-resource event:

```json
{"type":"progress","id":1,"seq":2,"phase":"resource_started","current":1,"total":3,"product":"zia","resource":"locations"}
```

Dump warnings are value-free and separate outer capability operation `dump`
from resource phase:

```json
{"type":"warning","id":1,"seq":3,"warning":{"product":"zia","resource":"locations","phase":"list","kind":"list_failed"}}
```

Closed phases are `list`, `show`, `project`, and `validate`; kinds are
`list_failed`, `show_failed`, `projection_failed`, and `subset_failed`. Final
dump failures exactly match warning frames in order and content.

On a successful dump, every selected resource is accounted for exactly once:
`resources_written + warning_count` equals the final progress total. Diff
progress likewise counts selected resources, but a resource empty on both
sides or skipped because partial collection failed is omitted from the wire
report. Consequently, successful
`resources_compared` may be less than, but never greater than, the final diff
progress total.

## Terminal results

There are no ambiguous generic completion counters. Every successful request
has an operation-specific result discriminant:

- `engine_manifest`
- `catalog_summary`
- `doctor_status`
- `auth_status`
- `config_status`
- `url_lookup_summary`
- `resource_read_summary`
- `dump_summary`
- `diff_summary`

All item-producing summaries include `stream_items_emitted`, counting semantic
items rather than fragment frames.

Dump summary contains records/resources written, warning count, partial flag,
effective redaction, and the exact value-free failure list. It contains no
output path or records.

Diff summary contains schema, fixed old/new side metadata without paths,
resources compared/with drift, record counts, and semantic item count. It is
converted only from admitted `DiffResult`.

## Error boundary

Wire operation errors contain no message, Go cause, backend text, path, rejected
selector, or provider detail. They are a closed kind plus an optional allow-
listed missing-credential variable list. The client owns presentation text.

Operation kinds include the existing machine taxonomy plus
`response_too_large`. Unknown Go errors map to static `internal`; the adapter
never serializes `error.Error()`. `canceled` is valid only in a canceled terminal.

Protocol errors use a separate closed vocabulary (`protocol_violation`,
`unsupported_protocol`, `frame_too_large`, `internal`) and are fatal. Busy uses
`request_rejected`, not the protocol-error or request-terminal families.

Caller product/resource values never echo in errors. Canonical catalog product
and resource names may appear only in successful item/progress frames and
value-free dump warnings.

## Concurrency and backpressure

The host uses:

1. one bounded decoder;
2. one operation worker;
3. one coordinator owning state, cancellation linearization, IDs, and sequence;
4. one writer owning stdout, with a one-frame queue and per-write acknowledgment.

The coordinator performs no blocking I/O. It retains at most one pending frame
while still selecting input cancellation/shutdown. The synchronous engine sink
rendezvous is bounded and returns a static transport-closing error when the
session closes. Every goroutine has a cancel and join path; no request/event
queue is unbounded.

The writer uses a full-write loop. A short/error write is unrecoverable because
it may leave a truncated frame. POSIX hosts ignore/register SIGPIPE before the
first stdout write and handle EPIPE as broken output.

## EOF, signals, and exit behavior

Stdin EOF is the portable Node/Bun/Rust shutdown mechanism. The host closes
stdin on a local signal to unblock decoding and closes stdout when abandoning a
blocked writer. Graceful join has a five-second ceiling; after that the process
forces a nonzero exit because Go cannot safely kill an arbitrary blocked
goroutine.

The in-process `enginehost.Host` does not take ownership of an arbitrary caller
writer on a graceful return; its output-close hook exists to unblock a failed
transport. The executable process owns its duplicated stdio descriptors, and
process exit supplies the consumer-visible stdout EOF.

| Cause | Behavior | Exit |
| --- | --- | ---: |
| clean EOF, idle | join and close | 0 |
| clean EOF, active | cancel, attempt terminal, join | 0 if joined/written; 1 otherwise |
| fatal bootstrap/protocol input | attempt fatal protocol error, cancel/join | 2 |
| internal coordinator/codec failure | cancel/join | 1 |
| broken/blocked stdout | cancel, close output, join/force | 1 |
| POSIX SIGINT | graceful cancel/join, second signal forces | 130 |
| POSIX SIGTERM | graceful cancel/join, second signal forces | 143 |
| Windows interrupt | graceful cancel/join | 130 |

Windows does not promise POSIX SIGTERM or second-signal semantics.

## Codec architecture

Wire DTOs live in a transport-owned package and embed no in-process engine
type. Converters explicitly map and copy every allowed field. Direct
`machine.Event`, result, SDK, config, or error serialization is forbidden.

The strict tokenizer enforces bytes, UTF-8, depth, decoded duplicate keys, and
scalar validity before typed decoding. Typed control decoders reject unknown
fields and trailing values. Dynamic values use a lossless parser/encoder.
Outbound control frames encode into a bounded buffer before one full-write
submission; large item payloads use the preflight/fragment stream.

The Go reference DTO/codec checkpoint is implemented in
`internal/enginewire`. The transport package is standard-library-only and its
engine adapter is isolated in a child package. It covers all four bootstrap
and 31 v1 root frame branches, rejects case-variant keys, bounds numeric work,
pins the checked-in schema identity and byte hash, and preserves exact JSON
numbers in diff ingestion while retaining value-based numeric equality and
hashing. `internal/enginehost` implements the single-session coordinator,
preflight/commit barrier, operation matrix, cancellation ceiling, and joined
decoder/worker/writer lifecycle. `cmd/zscalerctl-engine` is the narrow candidate
process adapter with platform-specific signal and interruptible-stdio handling.
`clients/typescript` supplies the strict independent parser/codec, stateful
request queue, full operation matrix, fragment/diff reconciliation,
cancellation handling, and no-shell Node process adapter.

## Conformance and promotion

The current language-neutral corpus contains 20 codec cases and five framing
cases. It covers bootstrap roots, representative v1 request/server roots,
strict parser and structural-number failures, exact-number preservation,
base64 boundaries, and NDJSON split/limit behavior. Both the Go codec and the
zero-runtime-dependency TypeScript client consume those same checked-in
fixtures.

The Go host and TypeScript client currently exercise operation and lifecycle
behavior in separate implementation-specific suites. Together those suites
cover all 11 operation pairs, request queuing, sequence and terminal rules,
busy rejection, fragments, diff and dump reconciliation, cancellation around
success commit, EOF, signals, broken pipes, callback isolation, and a real
credential-free Go process. Those lifecycle and process cases are not yet
shared transcripts. macOS and Linux run real process lifecycle tests; Windows
currently has build proof. Native Windows lifecycle runs remain a promotion
gate.

Before promotion, the checked-in shared corpus must expand to cover every exact
operation input/result/item family and the lifecycle negatives now exercised
only by implementation-specific tests. That includes no-version `reject`,
wrong schema hash, invalid capability/operation/item/result pairings, invalid
diff identity/reference combinations, cancellation races, terminal ordering,
EOF/signals/broken pipes, forbidden-value attempts, and oversized later-item
failure before semantic output.

Reproducible benchmarks record startup-to-hello, handshake, started, first item,
completion, cancellation, fragmentation throughput, and peak RSS. The initial
machine-specific baseline and exact commands are in
[ENGINE_STDIO_BENCHMARKS.md](ENGINE_STDIO_BENCHMARKS.md).

The initial language-neutral corpus under
`internal/enginewire/testdata/conformance` drives the Go and TypeScript codec
and framer tests. Lifecycle races, fragment sequences, signal handling, and
process joins remain independently checked by the executable host and client
suites until they are promoted into shared fixtures.

Promotion remains blocked on a second independent client (planned Rust),
cross-platform lifecycle proof, immutable schema validation, CLI and consumer
conformance, full project gates, and fresh-context compatibility/security
review. The first TUI may consume the candidate protocol before promotion, but
cannot redefine it.
