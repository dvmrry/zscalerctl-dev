# Builder Handoff

## Intent

fix(deps): require Go 1.26.9 and golang.org/x/net v0.60.0 for GO-2026-6617

govulncheck reports GO-2026-6617, an HTTP/2 HPACK encoder race in net/http
reachable from both modules, fixed in Go 1.26.9 and golang.org/x/net
v0.60.0. Raise the security floor to 1.26.9 in every module's go directive,
the verify-go-toolchain policy minimum and its self-test fixtures, the
node-toolchain expectations, every actions/setup-go pin, and the install
and dependency-policy docs; bump x/net in the tools module. govulncheck:
no reachable vulnerabilities in either module.


## Base / Head

- Base: `main` at `50135e4`.
- Head reviewed: `00d757d` on `pr/go-security` (one squashed commit). This artifact
  is added to that commit afterwards and changes no reviewed code.

## Files Changed

- `.github/workflows/ci.yml`
- `.github/workflows/codeql.yml`
- `.github/workflows/fuzz.yml`
- `.github/workflows/gosec.yml`
- `.github/workflows/release.yml`
- `README.md`
- `docs/DEPENDENCY_POLICY.md`
- `docs/INSTALL.md`
- `experiments/stdio-machine-adapter/go.mod`
- `go.mod`
- `scripts/test-verify-go-toolchain.sh`
- `scripts/test-verify-node-toolchain.sh`
- `scripts/verify-go-toolchain.sh`
- `scripts/verify-node-toolchain.sh`
- `tools/go.mod`
- `tools/go.sum`

## Tests Run

- `make check` (gofmt, vet, `make test` and `make race` including the
  zscalerctl_engine_testhooks runs, docs, surface manifest, machine contract and
  the other repository gates); `make vuln`: no reachable vulnerabilities.
- gitleaks with `.gitleaks.toml` over `main..HEAD`: no leaks.

## Review History

The change was built on per-area branches, each reviewed by fresh-context
reviewers (GPT-6.1 Sol, then GPT-6 Astra) until approved, and then reviewed
again as this single squashed commit against main. The review below is that
final review.

# Adversarial Review

Fresh-context reviewer: gpt-6-astra (Codex CLI), read-only, 2026-10-08

## Blocking Findings

None found in `main` (`50135e4`) → `00d757d`. Confirmed all three process documents from the requested `main` baseline.

## Non-Blocking Risks

- **Incomplete dependency handoff:** `tools/go.mod:53–61` also upgrades `x/crypto`, `x/mod`, `x/sync`, `x/sys`, `x/text`, and `x/tools`; `tools/go.sum` additionally updates `x/term`. Explain whether these accompany dependency resolution or were independently selected. Future dependency handoffs should enumerate the complete version delta.
- **Verification limits:** Upstream dependency source, advisory accuracy, build compatibility, and the claimed clean govulncheck results remain unverified. This environment permits neither network verification nor Go builds. Builder-reported tests were not treated as evidence.

## Machine Contract Review

No CLI implementation, JSON/NDJSON shape, error envelope, exit-code mapping, dump/diff format, schema, manifest, or introspection source changed.

The intentional source-build minimum increases from Go `1.26.6` to `1.26.9`, documented consistently in README, installation instructions, and dependency policy. All three module directives, both policy scripts, and all 15 workflow pins agree.

## Safety Review

No redaction, projection, field classification, output narrowing, or tenant-access code changed. No changed source path widens output or reveals dropped fields. Existing dump/diff sanitization and confidentiality boundaries remain unchanged.

The toolchain policy retains its independent minimum and exact-version checks. Updated negative fixtures retain rejection assertions.

## Generated Artifact Review

No generated CLI documentation, schemas, goldens, field-coverage reports, vendor files, or generated skills changed.

Changed tools requirements have corresponding archive and module checksum entries; their authenticity was not independently verified. Root dependency versions remain unchanged.

Read-only verification passed: four changed shell scripts parse with `bash -n`, version/checksum-entry consistency checks, and `git diff --check`. Generation and runtime tests were not run.

## Verdict

Verdict: approve with nits