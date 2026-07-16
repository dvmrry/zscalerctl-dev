# Builder Handoff

## Intent

Add the first typed, in-process local-engine capability model without changing
the supported CLI or `machine.v1` wire contracts. Introduce config-free
`engine.v1` discovery, typed resource list/get/show requests and results,
secret-free execution settings, and CLI dogfooding of the typed resource-read
path. Harden projected-record copying so callers and future frontends cannot
retain aliases, execute methods hidden in arbitrary Go values, bypass catalog
projection, or serialize internal engine envelopes directly.

## Base / Head

Base commit: `de05d98`

Initial implementation head: `a528f29`

Reviewed fix head: `432d265`

Review scope: `de05d98..432d265`

## Files Changed

Design and contract documentation:

- `docs/ENGINE_API_DESIGN.md`
- `docs/ENGINE_CAPABILITY_MODEL.md`
- `docs/ROADMAP.md`
- `docs/cli/machine-contract.md`

Typed engine, runtime, and CLI adapter:

- `internal/machine/engine_manifest.go`
- `internal/machine/engine_manifest_test.go`
- `internal/machine/engine_read.go`
- `internal/machine/engine_read_test.go`
- `internal/machine/engine_types.go`
- `internal/machine/executor.go`
- `internal/machine/executor_test.go`
- `internal/machine/json_contract_test.go`
- `internal/machine/types.go`
- `internal/machineio/machineio_test.go`
- `internal/runtime/runtime.go`
- `internal/runtime/runtime_test.go`
- `internal/cli/app.go`
- `internal/cli/app_internal_test.go`

Projection and defensive-copy boundary:

- `internal/resources/resources.go`
- `internal/resources/resources_test.go`

## Source Inputs Consulted

- `AGENTS.md` and the adversarial-review workflow/templates
- `docs/ENGINE_API_DESIGN.md`, `docs/EVENT_STREAM_DESIGN.md`, and the Phase
  4.5 roadmap
- the static resource catalog, projection/redaction implementation, browser
  service, machine executor/event path, runtime assembly, and CLI render path
- frozen `machine.v1` fixtures and schemas, introspection schemas/goldens,
  generated CLI documentation, field-coverage artifacts, and generated skill
- the C2C incident-receiver source adapter containing safe siblings alongside
  secret-classified SDK structs

## Generated Artifacts

None. No schema, supported machine fixture, CLI golden, generated CLI
documentation, field-coverage artifact, or generated skill changed.

## Expected Delta

- Add candidate `engine.v1` discovery in Go only.
- Add typed `ResourceReadRequest`, `ResourceReadResult`, and
  `ExecutionSettings` values that reject direct JSON.
- Derive resource-read operations from the catalog but advertise only the
  executable `list`, `get`, and `show` set.
- Run typed reads through the existing synchronous event/executor path.
- Route Cobra list/get/show through the typed runtime method while retaining
  final catalog/redaction verification before rendering.
- Remove the candidate generic `Input.Options` escape hatch; strict decoding
  rejects it as unknown.
- Establish a closed, defensively copied projected-value domain.

## Invariants Claimed

- Supported `machine.v1`, introspection v1/v2, CLI JSON/NDJSON/table/pretty,
  stderr envelopes, exit codes, schemas, and dump/diff artifacts are unchanged.
- Engine request/result/settings/manifest values have no direct wire format.
- Runtime settings contain no environment entries, credentials, secret refs,
  tokens, or resolved values.
- Resource results contain only projected, redacted records and return fresh
  collection/value copies.
- Cycles, structs, pointers, typed maps, nested typed containers, complex
  numbers, non-finite floats, invalid `json.Number`, and process-like values
  fail closed.
- Unsupported children in exact source maps are quarantined independently:
  catalog-secret or unallowed children may be discarded, but a marker reaching
  any allowed scalar or structured field fails with
  `ErrInvalidProjectedValue`.
- Named source scalars and scalar sequences are normalized to method-free
  built-ins before projected storage. Already-projected constructors reject
  named values, preventing custom `MarshalJSON`, `MarshalText`, or `String`
  methods from running during JSON, NDJSON, or text output.

## Tests Run

Builder verification, all passing at `432d265`:

- focused projection/copy/quarantine/method regressions with `-count=20`
- `go test ./...`
- `go test -race ./internal/resources ./internal/machine ./internal/runtime ./internal/cli ./internal/zscaler`
- `go vet ./...`
- `go run honnef.co/go/tools/cmd/staticcheck@v0.7.0 ./...`
- `git diff --check`
- `env -u GOFLAGS make check` (exit 0), including full repository race,
  vulnerability, static-analysis, docs, schema/contract, boundary, secret,
  workflow, release, generated-artifact, and skill-sync gates

Final fresh-context reviewer verification, all passing:

- focused regressions with `-count=20`
- targeted resource race tests
- fresh `-count=1` downstream tests for `internal/zscaler`,
  `internal/browser`, `internal/runtime`, and `internal/cli`
- targeted `gofmt -d`, `go vet`, and `git diff --check`

## Known Deferrals

- typed catalog/schema and sanitized status capabilities
- typed ZIA URL lookup
- typed dump and diff, including artifact-level dump completion
- versioned stdio DTOs, codecs, framing, cancellation, and reference clients
- frontend, MCP, Wails, Ink/OpenTUI, or Ratatui adapters
- public Go API or transport promotion

## Review Focus

- operation advertisement versus executable typed operations
- direct-JSON rejection and supported `machine.v1` immutability
- caller mutation, nested aliases, cycles, and reflection panics
- projection/redaction bypass through arbitrary Go values or methods
- quarantine behavior at allowed versus catalog-dropped fields
- CLI final verification and generated/supported artifact drift

# Finding Resolution

## Engine manifest over-advertised catalog read operations

Finding: any catalog operation marked `CapabilityRead` was advertised even
though typed `Read` executes only `list`, `get`, and `show`.

Fix: filter catalog-derived operations through the same closed executable
predicate used by `Read`.

Regression: a synthetic future read operation remains absent while a supported
list operation is advertised.

## Projected values retained mutable aliases

Finding: the initial copier handled only selected slice types and could retain
caller-owned mutable values.

Fix: recursively copy the admitted value domain and return fresh result,
record, map, sequence, manifest-operation, and effect collections.

Regression: supported boolean, float, byte, integer, string, interface, map,
and record collections are mutated on both sides of the boundary.

## Arbitrary object graphs escaped the copy boundary

Finding: cycles could recurse indefinitely; structs with mutable unexported
state, exported structs, pointers, complex/non-finite values, and invalid
numbers could alias, bypass modeled projection, or fail only during rendering.

Fix: replace arbitrary object-graph cloning with a closed projected-value
domain, cycle detection, finite-number validation, and fail-closed markers.

Regression: each rejected family passes through raw assertion, unchecked and
verified construction, source projection, and JSON failure checks without
emitting canary bytes.

## Whole-map rejection dropped safe C2C siblings

Finding: a source `map[string]any` containing an unsupported secret SDK struct
caused the entire modeled map to be quarantined, dropping safe sibling fields.

Fix: quarantine unsupported exact-map children independently. Projection may
discard secret/unallowed children while preserving safe siblings; any surviving
marker is rejected by final verification.

Regression: the real C2C incident-receiver adapter and a focused nested-map
test retain safe fields, omit the secret child, and reject an unsupported
allowed child.

## Structured fields could silently discard quarantine markers

Finding: modeled structured dispatch ran before direct marker detection, so an
unsupported value at an allowed structured field returned `include=false` and
reported success.

Fix: detect a direct marker before structured dispatch while leaving per-key
catalog filtering inside modeled maps.

Regression: top-level and nested allowed structured fields now require
`ErrInvalidProjectedValue`; discarded secret children still succeed.

## Named scalars could execute rendering methods

Finding: reflect-kind validation preserved named bools, numbers, and scalar
sequences. Their custom JSON, text, or string methods could change output shape,
emit unscanned content, return errors, or panic.

Fix: separate private source copying from projected-output copying. Normalize
named source scalars/sequences to exact built-ins during sanitization and reject
them at already-projected boundaries.

Regression: method-bearing int, bool, named slice, and slice-of-named-int values
are tested through raw/verified/unchecked boundaries, JSON, NDJSON, source
projection, and text formatting with no method output.

# Adversarial Review

Fresh-context reviewer: Franklin (`gpt-5.6-luna`, max, `019f4f86-8805-7153-92ea-58b52c592be4`)

Process baseline: `de05d98` (`AGENTS.md` and adversarial-review docs)

Review scope: `de05d98..432d265`

The reviewer was read-only, did not implement the change, inspected source and
tests as evidence, independently reproduced the boundary defects above, and
rechecked each fix in the same fresh context.

## Blocking Findings

The initial and intermediate reviews found the operation-advertisement,
defensive-copy, object-graph, structured-marker, and method-bearing-scalar
defects documented above and requested changes. Final re-review of `432d265`
found every blocker resolved and no remaining actionable finding or nit.

## Non-Blocking Risks

None reported.

## Machine Contract Review

The supported `machine.v1` manifest, request/response fixtures, CLI output,
schemas, introspection, error envelopes, exit codes, and generated artifacts
remain unchanged. Candidate engine types reject direct JSON, and strict legacy
decoding rejects the removed generic options field.

## Safety Review

The final review confirmed exact operation filtering, private source versus
projected copy domains, safe normalization, cycle quarantine, independent map
child handling, and final CLI verification. No mutable alias or custom named
scalar method was found to reach projected output.

## Generated Artifact Review

No generated or frozen artifact changed. Contract, docs, surface, field
coverage, and generated-skill gates passed with zero intentional artifact
delta.

Verdict: approve
