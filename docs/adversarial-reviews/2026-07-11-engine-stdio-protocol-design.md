# Builder Handoff

## Intent

Define the immutable candidate local stdio engine bootstrap and v1 wire
contract before transport/process code. The contract must be implementable by
Go, TypeScript, and a later Rust client without exposing credentials, paths,
raw errors, source records, or internal engine types.

## Base / Head

- Base: `824cf0e`
- Head: working tree on `feature/stdio-engine-api`
- Process baseline: `origin/main` at `b0597df`

## Files Changed

- `docs/ENGINE_STDIO_PROTOCOL_V1.md`
- `docs/schema/engine-stdio-bootstrap.schema.json`
- `docs/schema/engine-stdio-v1.schema.json`
- `docs/ENGINE_API_DESIGN.md`
- `docs/ROADMAP.md`
- this review artifact

## Source Inputs Consulted

- accepted engine, capability, event, and runtime designs
- typed machine request/result/event source
- runtime engine dump, diff, URL, status, and resource-read implementations
- dump/diff published schemas and resource value domain
- fresh-context protocol reviews by Beauvoir and Hubble

## Generated Artifacts

None. The two JSON schemas are hand-authored normative source artifacts.

## Expected Delta

- Candidate documentation/schema surface only.
- Supported CLI, `machine.v1`, JSON/NDJSON, error envelopes, exit codes,
  dump/diff artifacts, dependencies, and binaries remain unchanged.
- Bootstrap and v1 schema URNs plus exact-byte hashes become immutable design
  inputs for the transport implementation.

## Invariants Claimed

- Bootstrap remains version-independent and negotiates only opaque offered
  version tokens; no common version has an explicit client rejection frame.
- Every operation request and server frame/result/item family is closed, and
  the stateful operation matrix closes cross-frame combinations.
- Structural integers stay within the JavaScript exact-integer range; dynamic
  numbers retain exact decimal lexemes.
- Large semantic items use bounded, canonical-base64, checksummed
  fragmentation with a separate aggregate limit.
- Every semantic item stream passes whole-response preflight before success
  commit; failed and canceled resource reads expose zero record items.
- Process-lifetime monotonic IDs prevent cancellation ABA races without an
  unbounded retained ID set.
- Session/request state, outcome commit, terminal write, busy rejection, and
  cancellation linearization are distinct.
- Local paths, raw errors, rejected selectors, credentials, and dump records
  do not cross the wire.
- Every host goroutine has a bounded cancellation and join path.

## Tests Run

- Both schemas parse with strict duplicate-key rejection and `jq -e`.
- Every local `$ref` resolves to an existing `$defs` entry.
- Every typed object schema with properties has an explicit
  `additionalProperties` rule.
- Parsed-schema assertions verify all 11 accepted `started` pairs, exact diff
  unions, the closed missing-credential enum, and bootstrap rejection shape.
- Base64 boundary assertions accept canonical decoded sizes 1, 2, 3, 524287,
  and 524288 and reject 524289.
- The complete `ready` fixture advertises the actual v1 schema SHA-256:
  `6cba5a8170e538bd6eacde38c84526873f691421d6dc5f57cacfbd5f9438c522`.
- `bash scripts/verify-docs.sh`: pass.
- `make docs-cli-check`: pass.
- `make fmt-check`: pass.
- `git diff --check`: pass.

## Known Deferrals

- DTOs, codecs, transcript tooling, host process, and clients are the next
  implementation slices.
- Aggregate items beyond 64 MiB fail with `response_too_large`.
- A Rust reference client remains a promotion gate, not a first-TUI gate.

## Finding Resolution

The first exploratory fresh-context review identified ten architecture gaps:
incomplete DTOs, unsafe negotiation, ambiguous JSON/numbers, oversized valid
results, conflated lifecycles, busy/cancel ABA races, unjoined goroutines,
ambiguous dump counts, path/error leaks, and atomic-read/page-streaming
contradiction. The candidate contract now defines complete frame schemas,
immutable bootstrap, lossless number handling, fragmentation, separate state
machines, monotonic IDs, bounded process topology, operation-specific results,
closed safe converters, and atomic resource reads.

The formal reviewer then found five remaining blockers and two nits:

1. Negotiation and identity: added client `reject`, abandoned-handshake EOF,
   immutable schema URNs, and `ready` schema ID plus exact-byte SHA-256.
2. Atomicity: added whole-response conversion/validation/size/digest preflight
   and a success-commit barrier before any semantic item.
3. Cross-frame combinations: closed `started` to 11 pairs and added the
   operation-to-item/progress/warning/result matrix.
4. Fragment bounds: split padded/unpadded schema branches and defined canonical
   RFC 4648 decoding, exact decoded sizes, indexes, lengths, and chunk counts.
5. Diff unions: split identities and record references into exact disjoint
   schemas and documented their stateful correlation.
6. Nits: replaced the invalid abbreviated `ready` example and broad credential
   variable regex with a complete frame and eight-name allow-list.

# Adversarial Review

Fresh-context reviewer: Hubble (`gpt-5.6-sol`, high,
`019f5487-3280-73e3-9685-93830840a1f2`)

The reviewer was read-only and inspected the working-tree documents and parsed
schemas rather than accepting the builder handoff as evidence.

## Blocking Findings

The initial review requested changes for the five issues recorded above. The
focused re-review independently confirmed every correction. No blocking
findings remain.

## Non-Blocking Risks

None within the candidate-design scope. Implementation and cross-platform
process behavior remain explicitly gated before protocol promotion.

## Machine Contract Review

- The selected schema digest was independently recalculated and matches
  `ready`.
- `started` contains exactly the 11 legal capability/operation pairs.
- The stateful matrix closes item, progress, warning, and result legality.
- Diff identities/references and base64 fragment forms are disjoint.
- The existing supported CLI and `machine.v1` contract do not change.

## Safety Review

Whole-response preflight prevents request failure or cancellation after a
visible semantic item. Paths, raw errors, credentials, source records, and dump
records remain outside the wire contract. Missing credential details are
limited to the eight names emitted by the trusted engine boundary.

## Generated Artifact Review

No generated artifact changed. The hand-authored schemas parse, local
references resolve, control objects remain closed, and the design/roadmap
documents accurately describe the candidate contract.

## Verdict

Verdict: approve
