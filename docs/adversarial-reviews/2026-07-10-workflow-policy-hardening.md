# Builder Handoff

## Intent

Preserve published schema bytes across Windows checkouts and replace the
line-oriented GitHub Actions and `setup-go` policy checks with YAML-aware,
fail-closed validation.

## Base / Head

- Base commit: `85031bd35dd01d6fafcd3f2c3e5b95b29876fdb5`
- Reviewed head commit: `6e5959f`
- Process-document baseline: `origin/main`

The base is the independently approved security/effects candidate. This review
covers only the two checkout/policy hardening commits and the follow-up commit
that addressed the initial reviewer findings.

## Files Changed

- `.gitattributes`
- `docs/DEPENDENCY_POLICY.md`
- `docs/SCRIPTS.md`
- `scripts/verify-actions-pinned.sh`
- `scripts/test-verify-actions-pinned.sh`
- `scripts/verify-go-toolchain.sh`
- `scripts/test-verify-go-toolchain.sh`
- `scripts/verify-workflow-policies.go`

## Source Inputs Consulted

- Current workflows under `.github/workflows/`
- Published schemas under `docs/schema/`
- The vendored `gopkg.in/yaml.v3` syntax tree API
- Existing action-pin and Go-toolchain policy tests
- Fable's independently reproduced flow-style and local-action bypasses
- Adversarial-review instructions and templates from `origin/main`

## Generated Artifacts

None. No schema, CLI golden, machine manifest, generated documentation, or
workflow file changed.

## Expected Delta

- `docs/schema/*.json` is checked out with LF bytes on every platform.
- Existing block-style workflow policy remains enforced.
- Flow-style steps and jobs are now inspected.
- Local reusable workflows and composite actions are followed recursively.
- Every repository-local `action.yml` or `action.yaml` is inspected even when
  it is not currently referenced.
- The two policy modes share parsing and traversal but retain independent
  enforcement rules.
- CLI, runtime, machine-contract, schema-content, and tenant behavior remain
  unchanged.

## Invariants Claimed

- External actions and reusable workflows require a full 40-hex commit SHA and
  an inline version comment.
- The retired Gitleaks action is rejected case-insensitively.
- `actions/setup-go` is recognized case-insensitively and every occurrence must
  carry the literal policy minimum from `verify-go-toolchain.sh`.
- The Go minimum has one policy source of truth.
- Malformed YAML, dynamic executable structures, aliases, merge keys, duplicate
  keys, repository escapes, missing metadata, and dependency cycles fail closed.
- Unrelated nested input keys named `uses` or `steps` are not treated as
  executable nodes.
- Repository-root-relative local actions still resolve when the test override
  points at a fixture root, `.github`, or `.github/workflows`.
- The v1 introspection schema hash remains
  `029a8b56b478d4b2af4ef69188d4b727e1ebbc63cf2de693a5c3d73754f83b23`.

## Tests Run

- `env -u GOFLAGS make check` — pass at reviewed head, including race tests,
  govulncheck, staticcheck, Gitleaks, Semgrep, and all policy self-tests.
- `make verify-actions-pinned` — pass.
- `make verify-go-toolchain` — pass.
- `make verify-script-registry` — pass.
- `bash scripts/verify-docs.sh` — pass.
- `go test -mod=vendor ./...` — pass.
- Original flow-style action and stale flow-style `setup-go` bypass fixtures —
  rejected for the intended policy errors.
- Legitimate `with.steps`, `with.uses`, flow-style pinned action, nested local
  composite, local reusable workflow, and direct `.github/workflows` override
  fixtures — pass.
- Malformed-YAML fixture — fails cleanly without a Go `%!(EXTRA...)` formatting
  artifact.
- `git check-attr --cached text eol -- docs/schema/*.json` — `text` set and
  `eol=lf`.
- Disposable `core.autocrlf=true` checkout — v1 schema hash unchanged.
- `git diff --check` and `gofmt -d scripts/verify-workflow-policies.go` — clean.

## Known Deferrals

- The pre-existing JSON-redaction integrity defect and invalid-resource-ID exit
  mapping remain separate focused runtime fixes and block release, not this PR.
- Binding adversarial-review artifacts cryptographically to a reviewer identity
  and exact diff remains separate process hardening.

## Review Focus

- Valid GitHub Actions YAML forms and false positives
- Version-comment association
- Local action/reusable-workflow path resolution, symlinks, escapes, and cycles
- Case normalization and retired-action identity
- Separation of action-pin and `setup-go` policy modes
- Test assertions that could read stale stderr
- Windows schema checkout bytes and `.gitattributes` collateral

# Adversarial Review

Fresh-context reviewer: Terra high (`Peirce`, agent `019f4c0c-400e-7e33-ae20-0f300705dfc4`), reviewing `85031bd..6e5959f` read-only under the `origin/main` process rules.

## Review History

An initial independent Terra reviewer requested changes after reproducing two
issues: a direct `.github/workflows` override derived the wrong repository root,
and malformed YAML diagnostics supplied the parser error twice. The builder
reproduced both against the reviewed code, fixed them, added regression
assertions, and reran the focused and full suites. The final fresh-context
reviewer inspected the corrected head rather than accepting the fixes from the
builder summary.

## Blocking Findings

None remain.

## Non-Blocking Risks

None identified in the reviewed delta.

## Machine Contract Review

No CLI, JSON, schema-content, workflow, or generated-artifact contract changed.
The `.gitattributes` rule is scoped to the nine top-level JSON files under
`docs/schema/`.

## Safety Review

The reviewer verified fail-closed handling for malformed YAML, dynamic and
aliased structures, merge keys, duplicate keys, repository escapes, dependency
cycles, unpinned actions, retired-action casing, and stale case-varied
`setup-go`. The direct-workflows override and diagnostic regressions are covered
by focused tests.

## Generated Artifact Review

No generated content changed. Make targets, CI invocations, documentation, and
the script registry consistently reference the shared verifier; `yaml.v3` was
already committed and vendored.

## Coverage Ledger

| File | Disposition |
| --- | --- |
| `.gitattributes` | Reviewed; schema-only LF policy confirmed |
| `docs/DEPENDENCY_POLICY.md` | Reviewed; behavior matches implementation |
| `docs/SCRIPTS.md` | Reviewed; helper is registered |
| `scripts/verify-actions-pinned.sh` | Reviewed; wrapper and override roots confirmed |
| `scripts/test-verify-actions-pinned.sh` | Reviewed; positive and negative fixtures confirmed |
| `scripts/verify-go-toolchain.sh` | Reviewed; policy minimum remains authoritative |
| `scripts/test-verify-go-toolchain.sh` | Reviewed; structured and diagnostic fixtures confirmed |
| `scripts/verify-workflow-policies.go` | Reviewed; traversal and fail-closed boundaries confirmed |

Verdict: approve
