# Dependency Policy

`zscalerctl` treats dependencies as part of the credentialed trusted computing
base. The project vendors Go modules so reviewers can inspect the exact code
used by CI and release builds.

## Required Checks

Run these checks before merging dependency changes:

```sh
go mod tidy
go mod vendor
git diff --exit-code -- go.mod go.sum vendor
make fmt-check
go test -mod=vendor ./...
go test -race -mod=vendor ./...
go vet -mod=vendor ./...
bash scripts/verify-docs.sh
bash scripts/verify-semgrep.sh
bash scripts/verify-ci-no-live-creds.sh
bash scripts/test-verify-ci-no-live-creds.sh
bash scripts/verify-actions-pinned.sh
bash scripts/test-verify-actions-pinned.sh
make vuln
make verify-licenses
go run honnef.co/go/tools/cmd/staticcheck@v0.7.0 ./...
gitleaks dir .
gitleaks git .
bash scripts/verify-sdk-boundary.sh
bash scripts/test-verify-sdk-boundary.sh
```

`make vuln` runs govulncheck at the version pinned by `GOVULNCHECK_VERSION` in
the Makefile, so local runs and CI scan with the same tool version.
`govulncheck` must report no reachable vulnerabilities. Non-reachable findings
in required modules require a written review note before release.

## SDK Upgrade Runbook

The Zscaler SDK is the highest-risk dependency because it receives credentials
and performs network I/O. For every `github.com/zscaler/zscaler-sdk-go/v3`
version bump:

1. Read the SDK changelog or diff for auth, logging, cache, retry, proxy,
   request construction, and error handling changes.
2. Re-run `bash scripts/verify-sdk-boundary.sh` and
   `bash scripts/test-verify-sdk-boundary.sh`.
   Re-run `bash scripts/verify-semgrep.sh` so the credential escape-hatch rule
   stays active.
3. Confirm the adapter still builds the SDK configuration manually and does not
   call `zscaler.NewConfiguration`.
4. Confirm OneAPI and ZIA legacy adapters do not read environment variables,
   local config files, SDK log flags, proxy settings, or cache settings from
   ambient state when supplied manually constructed configuration.
5. Confirm live reader errors remain normalized before reaching CLI output.
6. Re-check whether legacy ZIA client cleanup can safely call the SDK `Close`
   method. Version 3.8.37 can deadlock on `Close`, so the current adapter avoids
   that call for short-lived CLI operations.
7. Run the shape-registry tests. The registry diff is the complete
   field-review checklist for the bump: any new SDK response field fails
   `TestReviewedSDKShapesMatchCatalogOrIgnoredRegistry` until it is classified
   in the catalog or excluded with a recorded reason. Then run
   `make field-coverage` to regenerate
   [FIELD_COVERAGE.md](FIELD_COVERAGE.md) (a stale report fails
   `TestFieldCoverageReportIsCurrent`), and keep its deferred column at zero —
   that column is the watchdog for fields parked without a final decision.
8. Run the required checks above.

Do not add a new resource in the same change as an SDK bump unless the SDK bump
is required for that resource and the review explicitly covers both changes.

## Presentation Dependencies

The `pretty` output renderer uses `github.com/charmbracelet/lipgloss` (with its
transitive `muesli/termenv`, `charmbracelet/colorprofile`, and `charmbracelet/x`
modules). These are terminal-styling libraries, not part of the credentialed
network path: they receive no credentials, perform no network or filesystem I/O,
and only style strings that have already been allow-list projected and redacted.

Two properties keep them low-risk and must hold on upgrade:

- The renderer pins its color profile explicitly (`SetColorProfile`) and renders
  through an `io.Discard`-backed renderer, so lipgloss/termenv never auto-detect
  the terminal or probe it with escape sequences. Color is driven solely by the
  existing `--color` / `NO_COLOR` / TTY logic.
- `colorprofile` contains the one subprocess path in this dependency set (it
  shells out to `tmux info` in `colorprofile.Tmux`/`Detect`). That path is
  unreachable from our usage: the only consumer, `charmbracelet/x/cellbuf`, uses
  `colorprofile` for type constants and color conversion, never detection. On
  upgrade, re-confirm no reachable call to `colorprofile.Detect`, `.Env`, or
  `.Tmux`, and that `termenv` still imports no `os/exec` or `net`.

## Machine Output Terminal Cleanliness

Machine-readable output must stay free of terminal control sequences even when
stdout is a real TTY. `make verify-pty-escape-clean` builds the normal
`zscalerctl` binary, runs representative JSON commands inside a pseudo-terminal,
and verifies that they emit valid JSON with no ESC bytes. Keep this guard when
changing terminal styling dependencies or startup paths.

## Renovate Policy

Renovate keeps Go dependencies and GitHub Actions current, but it does not
automerge updates. GitHub Actions must remain pinned to full commit SHAs with an
inline version comment so Renovate can update the digest while preserving a
human-readable tag.

Release SBOM generation uses `cyclonedx-gomod` installed by `go install` from
the committed `tools/go.mod` and `tools/go.sum` instead of a GitHub Action.
The committed `tools/go.sum` is the integrity control for that tool: the `go`
command verifies every module download against those hashes at install time,
and Renovate's `gomod` manager auto-discovers `tools/go.mod` to keep the
version current. GitHub Action SHA pinning does not apply to that dependency
path.

The Zscaler SDK package is handled separately from routine dependency updates.
Renovate requires dependency dashboard approval for SDK bumps and annotates those
PRs with the SDK upgrade runbook requirement.

### Dependency Update Ownership

- Root `go.mod`: Renovate-managed and vendored (tidy plus vendor refresh).
- `tools/` module: Renovate-managed for routine bumps but intentionally not
  vendored; a `renovate.json` package rule skips vendoring so no `tools/vendor`
  directory is created. GitHub Dependabot security alerts may also open PRs
  against it.
- Semver labels: dependency PRs that touch only `tools/` get `semver:none`;
  root-module bumps get `semver:patch`.

## Updating Semgrep

Semgrep is pinned in three places that must stay in sync:

1. Edit the version in `.github/requirements/semgrep.in`.
2. Regenerate `.github/requirements/semgrep.txt` with the `uv pip compile`
   command in the comment at the top of that file.
3. Update `SEMGREP_VERSION` in the Makefile to the same version.

## Advisory Scanners

CodeQL, OSV-Scanner, and gosec provide breadth signal on top of the project
specific gates. They start as advisory workflows instead of merge blockers.
gosec runs on every pull request and produces SARIF. CodeQL runs its analyzer
on pull requests and `main`.

Security analyzers and code-scanning upload are separate concerns:

- Scanner execution failure is a workflow failure.
- Scanner findings are advisory until a rule class is triaged and promoted.
- SARIF upload is attempted only when the repository exposes GitHub code
  scanning to the workflow token; upload unavailability is a repository
  capability warning, not a source-code failure.
- Development repositories that do not have code scanning enabled still keep
  SARIF or CodeQL results as short-lived workflow artifacts for review.

Promote a finding class to a blocking gate only after it is triaged and the rule
is known to be stable for this repository.

## Finding Remediation Policy

These thresholds are enforced by CI, not aspirational:

- **Dependency vulnerabilities (SCA):** zero tolerance in called code paths —
  any `govulncheck` finding blocks merge and release (`make vuln` runs in PR CI
  and in `make release-check`). Advisories in uncalled build tooling are
  remediated by version bump at the next opportunity.
- **Dependency integrity and licensing:** dependencies are hash-verified
  (`go.sum`), vendored for review, and must carry an Apache-2.0-compatible
  license. `make verify-licenses` enforces the shipped binary's dependency
  license allow-list with `go-licenses`; incompatible licenses block CI.
- **Non-exploitable advisories (VEX):** when an advisory affects a declared
  dependency but not the shipped binary (for example, build-only tooling in
  `tools/`), a `not_affected` statement is recorded in the repository's
  [.openvex.json](../.openvex.json) (OpenVEX format) alongside the remediating
  bump. Statements are reviewed like code.
- **Static analysis (SAST):** blocking tools (`go vet`, staticcheck, semgrep,
  secret scan) must report zero findings to merge. Advisory tools (gosec,
  CodeQL) feed the Security tab; their findings are triaged, and suppressions
  must be declared in code with a justification (`#nosec <rule> -- reason`).

## Blocking Gates And Fuzzing

Secret scanning is a merge blocker, not advisory: `make secret-scan` (part of
`make check`) runs gitleaks over the working tree with the same `.gitleaks.toml`
config as CI's `secret-scan` job, so an allowlist gap or a real leak is caught
locally.

Fuzzing uses the standard two-tier model, **not** a live-exploration blocking
gate:

- **Regression (blocking, deterministic):** `go test ./...` (the `unit` CI job)
  runs every fuzz target's seed corpus — the inline `f.Add` seeds plus any
  committed `testdata/fuzz/<target>/*` crash inputs — as ordinary tests. A
  committed crash input that still reproduces fails the build deterministically.
- **Discovery (advisory):** the weekly `fuzz.yml` workflow runs live
  `-fuzz` exploration and uploads any newly discovered crash input as an
  artifact. When the fuzzer finds a real bug, commit that input under
  `testdata/fuzz/` so the regression tier then catches it forever.

A short live-`-fuzz` job was previously wired as a blocking PR gate; it was
removed because live exploration is nondeterministic (it can fail an unrelated
PR on a freshly mutated input whose reproduction is not committed) and because a
fuzz-harness false positive can block real work. `make fuzz-smoke` remains as a
local discovery helper.
