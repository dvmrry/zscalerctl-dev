# Builder Handoff

## Intent

Close three independently reproduced boundaries without making supported
machine data lossy:

1. visibly escape C0, DEL, C1, and every Unicode `Cf` rune at resource and diff
   terminal sinks while preserving filter/search matching and JSON/NDJSON;
2. constrain legacy ZIA credential traffic to reviewed cloud origins, reject
   unsupported clouds before resolving provider-backed secrets, and prevent
   redirect replay of the authentication body;
3. apply the existing structural-key policy recursively in the Go stdio codec
   and keep the TypeScript decoder and OpenTUI terminal sink aligned to the
   same pinned Unicode table.

## Base / Head

- Base: `origin/main` at
  `ed026199a4f76b96a24e9aab56cdbf87fb2741fa`
- Initial reviewed head:
  `2228e1751f4cb09d083e422721937d2ffcdb60ca`
- Finding-resolution head:
  `2b30a830e96e7623551fcba8ad2b70dfee196926`
- Branch: `feature/output-auth-wire-hardening`
- Process baseline: `origin/main` at the base commit

## Files Changed

Terminal presentation and matching:

- `internal/output/terminal.go`
- `internal/output/terminal_test.go`
- `internal/cli/render_records.go`
- `internal/cli/dump_diff.go`
- `internal/cli/app_internal_test.go`
- `internal/cli/diff_detail_sanitization_test.go`
- `internal/resources/resources.go`
- `internal/resources/resources_test.go`

Legacy ZIA routing and secret lifecycle:

- `internal/zscaler/reader.go`
- `internal/zscaler/reader_test.go`
- `internal/runtime/runtime.go`
- `internal/runtime/runtime_test.go`

Wire and TypeScript/OpenTUI parity:

- `internal/enginewire/value.go`
- `internal/enginewire/codec_test.go`
- `clients/typescript/src/codec.ts`
- `clients/typescript/src/index.ts`
- `clients/typescript/src/unicode.ts`
- `clients/typescript/test/focused.test.ts`
- `experiments/opentui-agent-tui/src/text.ts`
- `experiments/opentui-agent-tui/src/transcript.ts`
- `experiments/opentui-agent-tui/test/text.test.ts`
- `experiments/opentui-agent-tui/test/transcript-model.test.ts`

Behavior and security documentation:

- `docs/ENGINE_STDIO_PROTOCOL_V1.md`
- `docs/INSTALL.md`
- `docs/THREAT_MODEL.md`
- `docs/schema/config.schema.json`

## Source Inputs Consulted

- Go 1.26.5 `unicode.Cf` (`unicode.Version == 15.0.0`).
- Go `net/http` redirect behavior and `http.ErrUseLastResponse`.
- The vendored legacy ZIA SDK client, including credential POST creation,
  configured-client retention, initial authentication, and refresh paths.
- Existing resource projection, matching, diff rendering, wire structural
  validation, and TypeScript decoder paths.
- Zscaler-owned legacy cloud-name documentation and the vendored SDK's exact
  host mapping.
- Adversarial-review process documents from the recorded `origin/main` base.

## Generated Artifacts

No generated artifact or frozen schema byte changed. The immutable engine
stdio v1 schema and hash, CLI docs, command-surface goldens, manifests,
field-coverage artifacts, and generated agent-skill copy are unchanged. The
config schema description and protocol/security prose were edited directly.

## Expected Delta

- Human resource table/pretty/key-value output and human diff output visibly
  escape C0, DEL, C1, and Unicode 15 `Cf` runes after projection and matching.
- JSON, NDJSON, dump/diff report values, and stdio dynamic values remain
  lossless.
- Filters and search retain their existing normalized matching representation;
  presentation escapes are not part of the query language.
- Legacy ZIA accepts only the nine documented, case-insensitive cloud names
  after trimming and maps them to exact HTTPS origins.
- Unsupported legacy clouds fail with the existing missing-credentials class,
  without echoing the input and before password or API-key providers resolve.
- The legacy SDK client does not follow HTTP redirects, so 307/308 cannot
  replay the authentication body to a second request.
- Dynamic object keys are validated recursively in Go and TypeScript. Dynamic
  values remain raw until a consumer-specific presentation sink.
- The TypeScript client owns one Unicode 15 format table shared by its codec
  and the OpenTUI experiment.
- Machine capability counts, resource counts, field coverage, CLI command
  surface, error envelopes, exit-code numbers, and frozen schemas are unchanged.

## Findings And Resolution

### Credential-bearing redirects

- Finding: Go's default redirect policy could replay the SDK's
  `bytes.NewReader` credential POST after a 307 or 308.
- Root cause: the legacy SDK authentication client had no `CheckRedirect`
  policy.
- Fix: the configured client returns `http.ErrUseLastResponse`, preserving the
  original response without issuing the redirect request.
- Regression: two real `httptest` origins require zero destination requests for
  both 307 and 308. Before the fix, the destination received the full POST body.

### Provider resolution before cloud validation

- Finding: production runtime construction resolved legacy password and API-key
  sources before `NewReader` rejected an unsupported cloud.
- Root cause: the closed cloud map was enforced only after runtime secret
  materialization.
- Fix: `ValidateLegacyZIACloud` reuses the closed map and runs before either
  provider resolution.
- Regression: recording providers must receive zero calls; the cloud error must
  retain its class, hide the input, and not be masked by a provider error.

### Incomplete TypeScript/OpenTUI Unicode tables

- Finding: copied tables omitted U+110BD, U+110CD, and U+13430-U+1343F.
- Root cause: independent hand-maintained range lists had drifted from Go's
  Unicode 15 table.
- Fix: one pinned table in `clients/typescript/src/unicode.ts` is consumed by
  both the codec and OpenTUI.
- Regression: exhaustive walks require all 170 Unicode 15 `Cf` points to be
  rejected in nested wire keys, preserved as dynamic values, and sanitized at
  the terminal sink.

## Invariants Claimed

- No raw SDK record bypasses projection/redaction.
- No credential value, rejected cloud value, redirect target, or backend prose
  is added to output or errors.
- Initial and refreshed legacy sessions use the redirect-refusing client.
- Valid legacy-cloud secret provider errors and cancellation identities remain
  matchable.
- Terminal encoding occurs after matching and never alters machine data.
- Empty dynamic keys remain allowed; only forbidden controls/formats at nested
  depths are newly rejected.
- Error envelope shape and numeric exit-code contracts are unchanged.

## Tests Run

Before the fixes, focused regressions demonstrated:

- 307 and 308 reached a second origin with the complete POST body;
- the password provider ran and masked unsupported-cloud rejection;
- TypeScript decoding and OpenTUI first failed at omitted U+110BD.

After the fixes:

- Focused Go, TypeScript, and OpenTUI regressions: pass.
- `go test -race ./internal/zscaler ./internal/runtime -count=1`: pass.
- OpenTUI `bun run check`: 123 pass, one intentional live-engine skip, and
  TypeScript 7.0.2 typecheck pass.
- `make fmt-check verify-typescript-client docs-check verify-core-boundaries
  verify-experiment-boundaries verify-machine-contract`: pass.
- `env -u GOFLAGS make check`: every target through surface accounting passed;
  it stopped only at the intentionally absent review artifact.
- All targets after that gate were run separately and passed: PTY escape check,
  release automation/artifacts, catalog draft, resource scaffold, SDK surface
  inventory, script registry, and generated skill sync.
- `git diff --check`: pass.

## Known Deferrals

- No credentialed tenant smoke was run. Routing, redirect, secret-resolution,
  codec, and presentation boundaries were validated without credentials or an
  external network endpoint.
- This change does not add a branded safe-string type to every experimental
  frontend leaf; it documents the sink obligation and closes the demonstrated
  OpenTUI paths.

# Adversarial Review

Fresh-context reviewer: Hooke (`019f974a-2bc4-79a2-a9b5-1fb9c9b43a55`)

Reviewed head: `2b30a830e96e7623551fcba8ad2b70dfee196926`

## Initial Blocking Findings

The reviewer independently found three blockers at the initial head:

1. the allowlist constrained only the initial legacy origin because Go could
   follow a cross-origin 307/308 with the credential body intact;
2. runtime construction resolved deferred legacy secrets before reaching the
   unsupported-cloud check;
3. the TypeScript and OpenTUI format tables omitted real Unicode 15 `Cf` code
   points, invalidating complete parity and terminal-safety claims.

The builder reproduced all three before changing code and supplied a separate
resolution commit with focused regressions.

## Resolution Recheck

The same reviewer rechecked only the three findings, the resolution delta, and
directly introduced surface:

- Redirect finding closed. The vendored SDK retains the configured client for
  both initial and refresh authentication. Focused 307/308 tests returned the
  original response and observed no destination request or body.
- Provider-order finding closed. Every production legacy construction path
  funnels through validation before password/API-key resolution. Unsupported
  clouds caused zero provider calls; valid-cloud errors and cancellation
  identities remained preserved.
- Unicode finding closed. One table now drives both consumers. An independent
  exhaustive probe reported 170/170 nested keys rejected, 170/170 dynamic
  values preserved through decode/encode, and 170/170 terminal values
  sanitized.
- No directly introduced blocker remained.

## Machine Contract Review

JSON, NDJSON, dynamic wire values, immutable schema bytes/hash, capability
counts, resource counts, error envelopes, and exit-code numbers are unchanged.
The new recursive key rejection aligns Go with the documented TypeScript
behavior; it does not rewrite dynamic values.

## Safety Review

Human resource/diff output is inert after matching, while machine data remains
lossless. Legacy cloud input cannot derive a destination, run a secret provider
before rejection, or cause a redirect replay. Existing projection, redaction,
and narrowing boundaries remain intact.

## Generated Artifact Review

No generated artifact changed. The reviewer confirmed the immutable engine
schema hash is identical at base and implementation head.

## Independent Verification

The reviewer passed `git diff --check`, affected-file formatting, focused Go
vet and race tests, TypeScript focused tests, OpenTUI typecheck and terminal
tests, and an independent Go-versus-TypeScript Unicode-table comparison. Head,
branch, origin, and worktree cleanliness were rechecked at completion.

Verdict: approve
