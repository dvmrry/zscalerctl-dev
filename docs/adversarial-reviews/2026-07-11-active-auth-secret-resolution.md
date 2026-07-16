# Builder Handoff

## Intent

Resolve and retain only the credential family selected by the effective
authentication mode. OneAPI runtime construction must not resolve configured
ZIA legacy password or API-key providers; ZIA legacy runtime construction must
not resolve the OneAPI client-secret provider.

Direct `config.Config` callers must receive classified errors rather than a
panic when secret-source interfaces are nil or typed nil.

## Base / Head

- Base: `27ddc0b`
- Head: working tree on `feature/stdio-engine-api`
- Process baseline: `origin/main` at `b0597df`
- Local and baseline adversarial-review documents had matching SHA-256 hashes.

## Files Changed

- `internal/runtime/runtime.go`
- `internal/runtime/runtime_test.go`
- `internal/runtime/status.go`
- this review artifact

## Source Inputs Consulted

- `internal/config/config.go`: effective mode and credential-family behavior
- `internal/secretref/source.go`: deferred source resolution
- `internal/zscaler/reader.go`: reader configuration and auth validation
- `internal/runtime/status.go`: existing nil/typed-nil source normalization
- adversarial-review process documents from `origin/main`

## Generated Artifacts

None.

## Expected Delta

- OneAPI resolves only its client-secret source and only OneAPI fields enter
  `zscaler.ReaderConfig`.
- ZIA legacy resolves only password and API-key sources and only legacy fields
  enter `zscaler.ReaderConfig`.
- Nil and typed-nil sources normalize to `secretref.Unset` before auth-mode
  inference or resolution.
- CLI output, machine contracts, status shapes, config precedence, redaction,
  catalog behavior, and effect metadata remain unchanged.

## Invariants Claimed

- Effective auth mode is computed once after source normalization.
- Active-provider failures retain missing-credentials and context identities.
- Inactive deferred providers cannot perform file, keyring, or process effects
  through `SecretSource.Resolve`.
- No credential value enters logs, errors, fixtures, or machine output.

## Tests Run

- Focused active/inactive, explicit/inferred, nil/typed-nil, cancellation,
  deadline, status, and invalid-mode tests: pass.
- `go test ./internal/runtime -count=1`: pass.
- `go test -race ./internal/runtime -count=1`: pass.
- `go test ./... -count=1`: pass.
- `go vet ./...`: pass.
- `git diff --check`: pass.

The optional Go error-style helper reported one pre-existing bare return in
`internal/runtime/engine.go`; this change did not touch that line.

## Known Deferrals

- Legacy authentication remains supported.
- Environment `*_FILE` values remain eagerly read during config loading; this
  change is scoped to deferred `SecretSource.Resolve` calls and runtime
  credential retention.
- No stdio protocol or frontend surface is introduced.

## Review Focus

- Explicit and inferred OneAPI/ZIA-legacy modes.
- Nil and typed-nil active and inactive source behavior.
- Active-provider error and context classification.
- Inactive credential retention and provider side effects.
- Machine-contract and effect-metadata compatibility.

## Finding Resolution

Finding: Direct configs could panic on nil or typed-nil secret sources.

Root cause: Runtime construction called auth inference or source resolution
before normalizing interface-valued sources. The status path already performed
the normalization, but live runtime construction did not share it.

Fix: Reuse the reflection-safe normalizer before inference, resolve only the
selected family, and populate only that family's reader fields.

Regression test: The explicit/inferred by OneAPI/legacy matrix covers nil and
typed-nil active/inactive sources; missing active sources and invalid modes
return classified errors; cancellation and deadline identities are preserved.

Verification: Focused, package, race, full-repository, vet, formatting, and
diff checks listed above pass.

# Adversarial Review

Fresh-context reviewer: Goodall (`gpt-5.6-sol`, ultra,
`019f5444-e2f4-7151-8caa-cfd0d887912b`)

Process baseline was confirmed against `origin/main` at `b0597df`. The reviewer
did not edit files and worked from the diff and source artifacts rather than
the builder summary.

## Blocking Findings

The initial review found one blocker: direct configs could panic during
effective-mode inference or active-source resolution when a secret-source
interface was nil or held a typed-nil pointer. This was especially material for
a future long-lived stdio host without CLI top-level panic recovery.

The focused re-review confirmed the finding is resolved. No blocking findings
remain.

## Non-Blocking Risks

None within the re-review scope.

## Machine Contract Review

- Source normalization now occurs before auth inference and resolution.
- Explicit and inferred modes no longer panic.
- Only the active family resolves and enters `ReaderConfig`.
- Missing active sources and invalid direct auth modes retain
  `ErrMissingCredentials` classification.
- Active cancellation and deadline failures retain their context identity.
- No JSON, exit-code, schema, manifest, or effect-metadata change occurred.

## Safety Review

Inactive credential families remain zero-valued in `ReaderConfig`. No
redaction, projection, logging, or secret-output behavior changed. The status
change only renames and reuses an existing unexported normalizer; status tests
remain green.

## Generated Artifact Review

No generated artifact changed or requires regeneration.

## Verdict

Verdict: approve
