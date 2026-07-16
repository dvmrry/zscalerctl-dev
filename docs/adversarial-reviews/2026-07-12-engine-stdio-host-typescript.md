# Builder Handoff

## Intent

Complete the candidate local stdio engine API checkpoint without changing the
supported CLI: add the long-lived Go host and narrow child-process command, then
prove the cross-language boundary with a strict zero-runtime-dependency
TypeScript client that can drive every accepted engine operation.

## Base / Head

- Change base: `ca333ef6f9c34b44b6b4b1c8c06f01fb62daac52`
- Head: working tree on `feature/stdio-engine-api`
- Process-doc baseline: `origin/main` at
  `b0597dfb8e673a06d99995e6e1360cfcc709f0a8`

## Files Changed

- `internal/enginehost/*.go`
- `cmd/zscalerctl-engine/*.go`
- `internal/enginewire/{framing.go,server.go,codec_test.go}`
- `internal/runtime/{engine.go,runtime_test.go,dump.go,dump_engine.go}`
- `clients/typescript/{package.json,README.md,src/*.ts,test/*.test.ts}`
- `scripts/{verify-typescript-client.sh,verify-core-boundaries.sh,test-verify-core-boundaries.sh}`
- `.github/workflows/ci.yml`, `.gitignore`, `Makefile`
- `docs/{ENGINE_STDIO_PROTOCOL_V1.md,ENGINE_STDIO_BENCHMARKS.md,ENGINE_API_DESIGN.md,ROADMAP.md,DEPENDENCY_POLICY.md,DEV_PUBLIC_SURFACE_MODEL.md,README.md,SCRIPTS.md}`
- this review artifact

## Source Inputs Consulted

- immutable `docs/schema/engine-stdio-bootstrap.schema.json` and
  `docs/schema/engine-stdio-v1.schema.json`
- accepted protocol design and checked-in Go conformance corpora
- `internal/enginewire` DTO, codec, adapter, and validator sources
- `internal/machine` typed engine operations, event lifecycle, manifest, and
  error taxonomy
- `internal/runtime` config/credential lifetime, reader construction, dump,
  diff, URL lookup, status, and projected-read boundaries
- resource projection/redaction and dump/diff result types
- Node stable erasable-TypeScript documentation and the upstream
  `actions/setup-node` release/tag source

## Generated Artifacts

None. The immutable protocol schemas, CLI reference, field coverage, machine
goldens, and supported surface goldens are unchanged. The benchmark report is
hand-recorded regression evidence from deterministic benchmark code.

## Expected Delta

- Supported `zscalerctl` commands, flags, help, completion, CLI goldens,
  machine manifest, introspection, schema-list output, JSON errors, and exit
  codes: unchanged.
- Candidate-only `cmd/zscalerctl-engine`: added in source, not added to release
  archives. It accepts only `--profile`, `--config`, `--timeout`,
  `--redaction`, and `--no-cache`.
- Candidate stdio host: all 11 capability/operation pairs, one active request,
  strict sequencing, whole-response success preflight, bounded per-item
  fragmentation, cancellation, backpressure, EOF/signal/broken-pipe handling,
  and joined goroutine/process lifecycle.
- Candidate TypeScript client: private Node >=24.12 source package with no
  runtime/optional/peer dependencies, strict independent parser/codec,
  one-request queue, all 11 typed methods, fragment/diff reconciliation,
  cancellation, immutable event values, and a no-shell absolute-path process
  adapter.
- Typed runtime reads now convert recognized construction failures such as
  missing credentials into the existing sanitized `MachineError` taxonomy;
  unknown in-process construction errors retain their original identity.
- Protocol schemas and their advertised hashes: byte-for-byte unchanged.

## Invariants Claimed

- No operation can mutate Zscaler tenant state.
- Credentials, secret refs, provider output, SDK clients/models, raw backend
  errors, source records, and caller paths never cross the wire.
- Runtime configuration and provider execution remain process-start or trusted
  Go-runtime concerns; the protocol has no credential API.
- Capability advertisement gates dispatch before runtime construction.
- A failed, canceled, or oversized item-producing operation emits zero semantic
  items; success commits only after all items and the terminal validate.
- Request IDs are strictly increasing; sequences are contiguous; one terminal
  ends each accepted lifecycle; fatal protocol errors discard in-flight state.
- Fragment chunks are canonical base64, contiguous, exactly sized, bounded to
  512 KiB each and 64 MiB per item, and reconciled by exact byte count, chunk
  count, SHA-256, strict lossless JSON, and item-kind DTO.
- Dump warnings and diff item groups reconcile exactly with completion
  summaries; dynamic JSON numbers keep their source lexemes.
- Host goroutines have cancellation and join paths. POSIX duplicated stdio is
  nonblocking and close-on-exec so provider subprocesses cannot inherit the
  protocol descriptors.
- TypeScript protocol source imports no filesystem/network/TLS/worker APIs,
  uses no dynamic code or native `JSON.parse`, and cannot select the engine via
  `PATH`.
- No total-response memory bound is claimed: v1 is atomic, the 64 MiB limit is
  per item, and both host/client memory can scale with admitted response size.

## Tests Run

- `env -u GOFLAGS go test -mod=vendor ./internal/enginewire ./internal/enginewire/adapter ./internal/enginehost ./internal/runtime ./cmd/zscalerctl-engine`: pass
- `go test -race ./cmd/zscalerctl-engine ./internal/enginehost -count=5`: pass
- `go test ./internal/enginehost ./internal/enginewire ./cmd/zscalerctl-engine -count=10`: pass
- `GOOS=linux go test -c` and `GOOS=windows go test -c` for the candidate
  command/host packages: pass
- strict temporary TypeScript compiler run with `--strict`,
  `--noUnusedLocals`, and `--noUnusedParameters`: pass
- `bash scripts/verify-typescript-client.sh`: pass, including all 35 shared-
  corpus, focused, stateful-client, and real Go process tests
- `bash scripts/verify-core-boundaries.sh`: pass
- `bash scripts/test-verify-core-boundaries.sh`: pass
- `bash scripts/verify-docs.sh`: pass
- `bash scripts/verify-script-registry.sh`: pass
- `bash scripts/verify-actions-pinned.sh`: pass
- `git diff --check`: pass

- `env -u GOFLAGS make check`: pass after the complete review/fix/re-review
  loop

## Known Deferrals

- Rust remains the second independent client required for protocol promotion.
- Native Windows process lifecycle proof remains a promotion gate; Windows
  build proof is present.
- The host command and TypeScript client remain candidate-only and are not
  release-packaged or supported surfaces.
- V1 intentionally has no cursor/page stream and no total-response byte cap;
  changing atomic resource delivery requires a new capability or protocol
  version.
- CI executes TypeScript directly with Node's stable type stripping; adding a
  compiler dependency to CI requires a separate pinned, integrity-locked
  dependency review.

## Review Focus

- Attack coordinator cancellation/terminal linearization, decoder permits,
  blocked writer/engine joins, signal/EOF races, goroutine leaks, fd ownership,
  and child descriptor inheritance.
- Find any path where semantic output can precede failed/canceled/oversized
  outcomes, item preflight can diverge from post-commit encoding, or provisional
  events can contradict terminals.
- Attack runtime-to-wire error mapping for leakage, lost taxonomy, forbidden
  credential names, and raw/path-bearing errors.
- Treat the TypeScript child as hostile: malformed/huge numbers, counts, depth,
  base64, fragments, sequences, item/result pairings, diff groups, output after
  terminal, process exit, stderr pressure, callback mutation, and cancellation
  races.
- Check TypeScript queue/close/write serialization for deadlocks, unhandled
  promises, operations sent after failure, and transport reuse after fatal
  state.
- Verify the process adapter cannot acquire arbitrary args, shell, PATH-selected
  executables, filesystem/network authority, or credential transport surface.
- Verify documentation and CI claims exactly match what runs, including action
  pins, dependency policy, release exclusion, platform proof, and memory limits.

# Adversarial Review

Fresh-context reviewer: Wegener (`019f5834-a234-7dc1-807d-4ed0bc1ed220`, Sol xhigh) and Maxwell (`019f5834-a3e5-79a3-a2d2-918aff43b99d`, Luna max)

Both reviewers inspected the working tree read-only, reported request-changes
findings, and independently re-reviewed the fixes. Neither reviewer implemented
or modified the change.

## Blocking Findings

1. The TypeScript client required diff progress total to equal
   `resources_compared`, rejecting a valid selected resource empty on both
   sides. The client now permits compared resources to be less than selected
   progress while retaining exact reconciliation with emitted resource items.
   Runtime and real-process regressions prove the empty-resource behavior.
2. The Go host and TypeScript client did not require every selected dump
   resource to resolve to exactly one success or warning. Both now enforce
   `resources_written + warning_count == selected total`, with under-count,
   over-count, and excess-warning regressions.
3. The protocol document claimed lifecycle behavior was already covered by
   shared transcripts. It now states the exact shared corpus (20 codec and five
   framing cases), separates implementation-specific lifecycle/process suites,
   and makes shared lifecycle expansion a promotion gate.
4. `EngineClient.close()` could wait forever for a child that retained stdout,
   and canceled requests had no independent client-side ceiling. Bounded,
   configurable close and cancel watchdogs now abort a nonconforming transport;
   held-open stdout and withheld-terminal regressions prove both paths.
5. The initial cancel-watchdog fix remained armed after a semantic item proved
   the host had committed success. It could therefore abort a valid long stream
   after a late cancel. Inline items and valid fragment begins now mark success
   committed before callbacks, clear an armed timer, and suppress later cancel
   requests. The regression delays completion beyond the watchdog and covers
   both cancel-before-first-item and cancel-from-item-callback races.
6. Go and TypeScript accepted duplicate `diff_field_change` identities for the
   same product, resource, key, and field. Both preflight implementations now
   reject duplicates, with regressions in each language.
7. Public TypeScript JSON/frame encoders could recursively follow cyclic values
   until stack exhaustion. Both generic and typed encoders now enforce depth and
   ancestor-cycle checks before recursion, with public-API cycle regressions.
8. The ordered-JSON helper initially rejected duplicate keys only at
   construction but retained caller-owned mutable entry tuples. A caller could
   mutate a validated key and emit duplicates later. It now snapshots and
   freezes every tuple plus the outer array; the exact post-construction
   mutation exploit is covered.

The reviewers also confirmed fixes for the original lower-severity findings:
invalid method input is an asynchronously normalized request error without ID
consumption; invalid transport output is normalized and aborted inside
bootstrap handling; an impossible busy rejection is session-fatal for the
serialized client; and the README no longer claims unverified Bun compatibility.

Both final narrow re-reviews found no adjacent blocker. Wegener independently
reproduced cancellation just before the first item and confirmed that the item
disarmed the timer. Maxwell independently reproduced the ordered-entry mutation
and confirmed the copied key remained stable. Their final read-only client runs
passed all 35 tests.

## Non-Blocking Risks

- V1 remains response-atomic with a per-item limit but no total-response memory
  bound; host and client memory can scale with admitted response size.
- Native Windows process lifecycle proof and a second independent client remain
  promotion gates. The candidate is intentionally not release-packaged.
- Shared lifecycle transcripts remain deferred to protocol promotion; current
  Go and TypeScript suites exercise those paths independently.

## Machine Contract Review

The supported CLI, `machine.v1`, introspection, machine manifest, CLI schema,
JSON errors, exit codes, and release artifacts are unchanged. The new command,
wire protocol, and TypeScript package remain explicitly candidate-only. The
reviewers verified all 11 candidate operation pairs, sequence/terminal rules,
fragment and diff reconciliation, dump conservation, strict numeric/base64/JSON
handling, process shutdown, and the corrected documentation claims.

## Safety Review

No credential or raw runtime/backend error is added to the wire. The process
adapter still requires an absolute executable, uses no shell, exposes only the
five process policy flags, and inherits credential lifetime inside the trusted
Go process. Projection/redaction remains the source of resource values;
item-producing failures and cancellations remain response-atomic. Watchdog
failure aborts the entire child rather than reusing an indeterminate session.

## Generated Artifact Review

The immutable bootstrap/v1 schemas, schema hashes, CLI reference, field
coverage, machine goldens, and supported surface goldens are unchanged. The
new benchmark report is hand-recorded evidence, not generated supported
surface. Shared fixture counts were verified directly as 20 codec and five
framing cases.

## Verdict

Verdict: approve
