# Builder Handoff

## Intent

Remediate the validated adversarial findings and implement the highest-value
adjacent hardening:

1. Remove a broad Gitleaks private-key fixture allowlist that could suppress a
   real key inside a crafted composite detector match.
2. Publish introspection v2 effect metadata so tenant-read-only is not mistaken
   for locally side-effect-free.
3. Preserve released introspection v1 schema validation while giving v2 an
   immutable version-specific schema.
4. Enforce Go 1.26.5 as the build minimum after reachable standard-library
   vulnerabilities were reported under 1.26.4.
5. Restore Gitleaks CI compatibility with GitHub's Node-24 runner and make the
   new history regression genuinely full-history.
6. Record the larger resource/catalog file split as future mechanical work
   rather than mixing it into this remediation.

## Base / Head

Base branch or commit: `b0597dfb8e673a06d99995e6e1360cfcc709f0a8`
on local `main`.

Head branch or commit: the uncommitted working tree containing this artifact.

Process-doc baseline: `b0597dfb8e673a06d99995e6e1360cfcc709f0a8`.
The user did not name a PR or remote branch; the local base is one commit ahead
of `origin/main` and contains the adversarial-review workflow used here.

## Files Changed

Security and CI:

- `.gitleaks.toml`, `.gitleaksignore`
- `scripts/test-gitleaks-allowlist.sh`
- `scripts/verify-gitleaks-history-policy.sh`
- `scripts/test-verify-gitleaks-history-policy.sh`
- `scripts/verify-go-toolchain.sh`
- `scripts/test-verify-go-toolchain.sh`
- `scripts/verify-actions-pinned.sh`
- `scripts/test-verify-actions-pinned.sh`
- `Makefile`, `.github/workflows/*.yml`
- root, tools, and experiment `go.mod` files
- `experiments/stdio-machine-adapter/go.sum`
- security/dependency/install/script documentation

Effect contract and tests:

- `internal/cli/introspect.go`, `globalflags.go`
- command constructors for config/auth/schema, core, dump/diff, and products
- `internal/cli/introspect_test.go`
- `internal/cli/introspect_effects_internal_test.go`
- `internal/cli/introspect_schema_test.go`
- `internal/cli/app_test.go`, `agent_docs_test.go`
- `docs/schema/introspect-v2.schema.json`
- the frozen legacy `docs/schema/introspect.schema.json` remains byte-identical
  and therefore has no final diff
- agent/operator/machine-contract/versioning/manpage guidance

Fixtures and generated artifacts:

- runtime-assembled PEM test fixtures in redaction/resource tests
- `docs/cli/zscalerctl.md`
- the canonical and generated zscalerctl skills
- introspection JSON/pretty goldens and inherited-output help goldens
- `cmd/zscalerctl/testdata/surface/surface_changes.md`
- `docs/ROADMAP.md`

The authoritative exact list is `git status --short`; reviewers inspected the
actual base-to-working-tree diff rather than treating this inventory as
evidence.

## Source Inputs Consulted

- Gitleaks v8.30.1 built-in private-key detector behavior and the original
  `.gitleaks.toml` allowlist.
- A generated RSA-2048 key alone and inside the original unclosed fake-prefix
  composite bypass shape.
- Exact historical Gitleaks commit/path/rule/line fingerprints.
- Cobra's command tree, canonical global flags, output writer, config loader,
  dump/diff paths, resource runtime, secret providers, keyring helpers, and
  Windows owner-permission helper.
- Released v0.15.1 introspection output and schema.
- Local `govulncheck` results for Go 1.26.4 and 1.26.5.
- Official Gitleaks Action v3.0.0 tag and action metadata; commit
  `e0c47f4f8be36e29cdc102c57e68cb5cbf0e8d1e` declares Node 24.
- Project review, schema, surface, dependency, and generated-artifact policies.

## Generated Artifacts

- `docs/cli/zscalerctl.md`: `make gen-cli-docs`
- `.agents/skills/zscalerctl/SKILL.md`:
  `bash scripts/sync-agents-skill.sh`
- CLI surface goldens:
  `go test -mod=vendor ./cmd/zscalerctl/... -run '^TestGoldenSurface$' -update`
- `surface_changes.md` is handwritten and explains both intentional golden
  families.

No resource catalog, field-coverage, dump, diff, projection, or `machine.v1`
artifact changed.

## Expected Delta

- CLI command paths: unchanged at 292.
- Catalog operations: unchanged at 271.
- `machine.v1`, exit codes, error envelopes, resource output fields, and
  production redaction/projection behavior: unchanged.
- Introspection: v1 to v2 with required sorted `effects`; all 292 conservative
  `mutating` values are true.
- Effect totals: 279 local reads across 278 commands, 293 writes, 2 deletes,
  273 network effects, and 274 process effects. Conditions total 276 `always`,
  294 `flag_set`, and 551 `configuration_dependent`.
- Network/provider-capable commands: 271 catalog reads plus `dump` and
  `zia url-lookup`. Windows `config init` adds the remaining platform-dependent
  process effect.
- Released v1 schema SHA-256 remains
  `029a8b56b478d4b2af4ef69188d4b727e1ebbc63cf2de693a5c3d73754f83b23`.
- V2 output, schema `$id`, and `$schema` constant use
  `docs/schema/introspect-v2.schema.json`.
- All modules enforce `go 1.26.5`; redundant `toolchain` directives are gone.
- The experiment's pre-existing tidy drift moves `x/sys` 0.44.0 to 0.45.0.
- Both Gitleaks Action uses move from Node-20 v2.3.9 to SHA-pinned Node-24
  v3.0.0.

## Invariants Claimed

- A real private key is detected in a test path and in the composite bypass
  shape; no private-key content/path exception remains.
- Every history ignore is exact `commit:path:rule:line`, names an available
  commit, and is checked only in a full repository checkout.
- Generated key material remains in owner-only temporary storage, is fully
  redacted from reports, and is removed on exit.
- Effect metadata is strict, sorted, deduplicated, centrally derived from
  intrinsic command and global-flag metadata, and tied to available flags.
- `configuration_dependent` means effective config, environment, provider, or
  platform may enable an effect; consumers treat it as possible unless pinned.
- Config/status commands do not falsely claim provider execution; credentialed
  live commands do. `config init` discloses its Windows `icacls` helper.
- Configured process execution participates in conservative `mutating`.
- Human introspection shows the same effects and condition semantics as JSON.
- Published schema URLs are immutable: v1 remains frozen and future contracts
  get new versioned paths.
- Go 1.26.4 cannot build/list the module, and workflow/toolchain drift cannot
  silently lower the security floor.
- Normalized v1/v2 golden comparison shows no unexplained command, catalog, or
  pre-existing field drift.

## Tests Run

Focused and regression checks passed:

- exploit reproduction against the original Gitleaks config
- `make verify-gitleaks-allowlist secret-scan`
- full 508-commit Gitleaks history scan
- `make verify-go-toolchain verify-actions-pinned verify-script-registry`
- `GOTOOLCHAIN=go1.26.4 go list -mod=vendor ./...` rejected as required
- `go mod tidy -diff` and `go mod verify` in root, tools, and experiment
- experiment module tests
- `go test -mod=vendor ./internal/cli ./cmd/zscalerctl`
- focused redaction/resource tests
- golden regeneration followed by golden verification
- docs, schema, machine-contract, skill-sync, surface-manifest, boundary,
  release, Semgrep, secret, vulnerability, vet, staticcheck, unit, and race gates
- `git diff --check` and changed-file control-character scan

The complete non-review gate set passed before artifact creation. The final
`make check` is run after this approved artifact is present because the
adversarial-review verifier is itself part of that target.

## Known Deferrals

- The resource/catalog file split is roadmap-only and intentionally not mixed
  into the security/contract change.
- Metadata-only filesystem probes used to make a declared write safe are not
  duplicated as read effects; v2 documents pre-existing content reads.
- Historical exact fingerprints remain because Git history is not rewritten.
- GitHub-hosted workflow execution cannot be reproduced locally; action SHA,
  runtime metadata, workflow policy, and local equivalents were verified.

## Review Focus

- Attack every Gitleaks suppression and history assumption.
- Verify the Go patch floor is enforced for source consumers and each CI step.
- Compare every effect class to actual config, provider, platform, filesystem,
  and network paths.
- Validate v1 compatibility and v2 schema identity.
- Normalize the large golden diff and look for unexplained drops or reordering.
- Verify docs, pretty output, schema, and generated skill tell the same safety
  story.

# Adversarial Review

Fresh-context reviewer: `/root/security_reviewer`, `/root/contract_reviewer`, and `/root/holistic_reviewer`

## Blocking Findings

The initial reviews requested changes for:

- Go 1.26.5 being advisory instead of the strict module minimum.
- SHA-pinned Gitleaks Action v2.3.9 using a retired Node-20 runtime.
- The new history control running in a shallow CI checkout.
- Introspection omitting local reads and configured/provider process execution.
- V2 replacing the floating schema URL embedded in already-released v1 output.

The first recheck found one additional same-class omission: Windows
`config init` executes the fixed `icacls` permission helper. The security gate
recheck also found that global workflow-key counts could be spoofed instead of
proving every setup-go step had its own pin.

## Resolution And Recheck

Each finding mapped to root cause, fix, regression, and verification:

1. All module `go` directives are 1.26.5; the new gate rejects older compilers,
   synchronized downgrades, unknown nested modules, redundant toolchain lines,
   and missing/stale workflow pins.
2. Both Gitleaks actions use official SHA-pinned Node-24 v3.0.0; the action gate
   rejects the retired Node-20 SHA.
3. CI fetches full history; history policy rejects shallow clones, global or
   malformed ignores, and missing commits before scanning.
4. V2 adds local-read/process kinds and configuration-dependent conditions with
   exact command/catalog coverage and an observed provider-execution test.
5. V1 schema is byte-frozen; v2 has a new immutable path and identity tests.
6. `config init` now advertises platform-dependent process execution and the
   golden/count tests require it.
7. Setup-go verification parses each complete direct or named step, requires a
   direct `with.go-version` child, and rejects unrelated, commented, env, or
   block-scalar decoys.

The same reviewers rechecked only their addressed findings and affected
surfaces. Contract and holistic reviewers returned `approve`; the security
reviewer returned `approve` after the final named-step/block-scalar recheck.

## Non-Blocking Risks

None remain in the reviewed scope.

## Machine Contract Review

`machine.v1`, resource fields, exit codes, error envelopes, dump/diff contracts,
and tenant-read-only behavior are unchanged. Introspection v2 is an intentional
supported-surface change with an immutable schema path. Released v1 output
continues to validate against its embedded schema with zero errors, and current
v2 output validates against its new schema with zero errors.

## Safety Review

The private-key bypass is closed without weakening runtime redaction tests.
Local reads, writes, deletes, network access, configured provider execution,
and fixed platform-helper execution are now visible to agents. No production
redaction, projection, field-classification, or tenant mutation path changed.

## Generated Artifact Review

The large JSON golden is wholly explained by the versioned schema URL and
effect arrays. Pretty output now renders the same effects. Command paths,
catalog operations, output fields, skill copies, generated CLI docs, and all
unrelated golden fields were checked for drift.

Verdict: approve
