# Builder Handoff

## Intent

feat(engine): carry live failure reasons on the stdio wire; drop dump error v1

The CLI error envelope and dump `errors.ndjson` say why a live read failed
(`reason`, HTTP `status`, `retryable`), but the stdio engine wire carried only
`live_access_failed`: an engine client could not tell a missing API role from
a rate limit. `MachineError.Detail` had been hidden from the wire to keep it
unchanged; zscalerctl is pre-1.0 with no external consumers, so the wire now
carries it.

`machine` owns the closed reason vocabulary. `enginewire.NonCredentialFailure`
gains optional `reason`/`status`/`retryable`, valid only on
`live_access_failed`; `DumpFailure` gains the same, valid only on `list`/`show`.
One Go validator, the TypeScript client decoder and the JSON schema enforce
the same rules (closed reason, status 100..599, status/retryable only with a
reason). The adapter copies the machine detail and drops one the wire would
reject. The engine-stdio schema digest is updated in Go, the protocol doc and
the TypeScript client. The dump error v1 reader is removed.

## Base / Head

- Base: `integration/2026-10-09` at `28eb6ae`.
- Reviewed heads: `9291b4f` (round 1), `bd15cb2` (round 2).
- Final head: this artifact on top of `bd15cb2`.

## Files Changed

- `.agents/skills/zscalerctl/SKILL.md`
- `clients/typescript/src/{client,codec,constants,types}.ts`,
  `clients/typescript/test/{focused.test.ts,fixtures/stderr-flood-engine.cjs}`
- `docs/ENGINE_STDIO_PROTOCOL_V1.md`
- `docs/cli/agent-machine-workflow.md`
- `docs/schema/dump-error.schema.json`
- `docs/schema/engine-stdio-v1.schema.json`
- `internal/dump/artifact.go`, `internal/dump/dump_test.go`
- `internal/enginewire/{data,server,types}.go`, `internal/enginewire/server_test.go`
- `internal/enginewire/adapter/{errors,results}.go`, `internal/enginewire/adapter/adapter_test.go`
- `internal/machine/failure.go`, `internal/machine/failure_vocabulary_test.go`
- `internal/zscaler/failure.go`, `internal/zscaler/failure_test.go`
- `skills/zscalerctl/SKILL.md`

## Tests Run

- `go test ./...` (Go 1.26.9, `/bin/chmod` first in `PATH`), `gofmt -l`,
  `go vet`, `verify-docs`, `verify-cli-docs`,
  `verify-surface-changes-manifest`, `verify-machine-contract`: pass.
- `scripts/verify-typescript-client.sh`: 42/42, including the real
  `zscalerctl-engine` process test.
- `tsc --noEmit --strict` over `clients/typescript/src`: no errors in the
  client's own code (the remaining errors are missing `@types/node` in this
  environment).

## Known Gaps

- The repo's TypeScript verification runs `node --test`, which strips types
  without checking them; a strict `tsc` step would need `@types/node` and is
  not part of the verifier.

## Review History

Reviewed with no repository access by a fresh-context local model given the
diff and this repository's reviewer prompt and template: Strata serving
Qwen3.8-Flash-Next (IQ3_S) on gm01. The reviewer could not run commands.

- Round 1, request changes. Blocker: the TypeScript `warningEqual` read the
  detail fields on `project`/`validate` dump failures, which did not declare
  them (a strict `tsc` error that `node --test` does not catch). Those
  variants now intersect `NoFailureDetail`. Also: the vocabulary sync test now
  checks the TypeScript client's `FAILURE_REASONS`, and the dump test name
  says v1 is rejected.
- Round 2, approve with nits (below).

Accepted and not changed: an explicit `"retryable": false` is accepted by the
TypeScript client and the schema while Go never emits it (`omitempty`); Go
validates a struct, so it cannot see an explicit `"status": 0`, which it never
emits; `deadline_exceeded` carries no reason on the wire because the kind
already says it.

# Adversarial Review

Fresh-context reviewer: Strata (Qwen3.8-Flash-Next IQ3_S, gm01), read-only, no command execution, 2026-10-15 (round 2)

## Blocking Findings

None.

The round-1 blocker is resolved by the type-level change itself: `DumpFailure` now declares `reason`, `status`, and `retryable` on every variant. The live-read variants keep `FailureDetail`, while `project` and `validate` intersect `NoFailureDetail`, whose properties are typed as `undefined`. That makes property access in `warningEqual` type-safe without weakening the list/show variants.

The diff does not weaken tests or machine contracts; it only narrows TypeScript types and adds a vocabulary-sync check.

## Non-Blocking Risks

1. **TypeScript vocabulary sync only checks `codec.ts`, not `types.ts`.**

   - file/path: `internal/machine/failure_vocabulary_test.go`, `clients/typescript/src/types.ts`, `clients/typescript/src/codec.ts`
   - concrete issue: the new test parses `const FAILURE_REASONS = new Set([...])` from `codec.ts`, but it does not parse the `FailureReason` union in `types.ts`. If `types.ts` drifts from the Go vocabulary while `codec.ts` remains correct, this test may still pass.
   - why it matters: the TypeScript public type is also a machine-facing contract. A future edit could add or remove a reason in the exported union without failing the vocabulary sync test.
   - how to reproduce or verify: add a bogus reason to `FailureReason` in `types.ts`, leave `codec.ts` unchanged, and run `go test ./internal/machine -run TestFailureReasonsMatchEveryCarrier`.
   - test that would catch it: parse the `FailureReason` union from `types.ts` and compare it to the same sorted Go list, or make `FAILURE_REASONS` typed as `Set<FailureReason>` and add a type-level test.

2. **`NoFailureDetail` types absence as `undefined`, but runtime rejection is not proven by this diff.**

   - file/path: `clients/typescript/src/types.ts`, `clients/typescript/src/codec.ts`
   - concrete issue: the TypeScript type says `project` and `validate` failures do not carry `reason`, `status`, or `retryable`. If the codec accepts such fields on those failure kinds and casts them to `DumpFailure`, the runtime object can violate the declared type.
   - why it matters: TypeScript types are only a compile-time contract. A malformed or future wire frame could pass runtime parsing and then be treated as detail-free by client code.
   - how to reproduce or verify: construct a TypeScript test that parses a project failure such as:
     ```json
     {"product":"zpa","resource":"server","phase":"project","kind":"projection_failed","reason":"unauthorized"}
     ```
     and assert the codec rejects it.
   - test that would catch it: a TS codec test for project/validate failure frames rejecting `reason`, `status`, and `retryable`.

3. **The reported `tsc` evidence is not clean in the provided environment.**

   - file/path: `clients/typescript/src/*`
   - concrete issue: the handoff says strict `tsc --noEmit` reports no errors in `client.ts`, `types.ts`, and `codec.ts`, but remaining errors are only missing `@types/node` in the environment. That is probably enough for this narrow type fix, but it is not a fully green typecheck.
   - why it matters: this change is specifically about a strict TypeScript compilation issue. A clean CI typecheck is the right guard.
   - how to reproduce or verify: run `tsc --noEmit --strict` in an environment with Node types installed, or run the repo’s TypeScript verification script in CI.
   - test that would catch it: CI job that fails if `tsc --noEmit --strict` reports any TypeScript client source errors.

## Machine Contract Review

- The diff changes no Go runtime behavior, no JSON/NDJSON encoding, no exit codes, no schemas, no manifests, and no introspection output.
- The TypeScript `DumpFailure` contract is narrowed: `project` and `validate` failures are now declared detail-free.
- If the published JSON schema allows failure detail on all dump failure kinds, then the TypeScript type and schema may diverge. This diff does not show schema changes, so that should be verified separately.
- The vocabulary sync test now covers the TypeScript codec’s reason set, which strengthens machine-contract consistency but still does not cover the TypeScript exported union type.

## Safety Review

- No redaction, projection, field coverage, or sensitive-data classification behavior changes.
- The change does not widen output or reveal previously dropped data.
- It makes the TypeScript type stricter for non-live dump failures, which is safety-positive if the runtime codec enforces it.

## Generated Artifact Review

- No generated files, schemas, fixtures, or docs are changed.
- The only test changes are a clearer dump test name and an added TypeScript vocabulary check.
- No suspicious artifact drops, additions, ordering changes, or count changes are present in the diff.

## Verdict

Verdict: approve with nits
