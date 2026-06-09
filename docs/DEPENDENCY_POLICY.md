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
go run golang.org/x/vuln/cmd/govulncheck@latest ./...
go run honnef.co/go/tools/cmd/staticcheck@v0.7.0 ./...
gitleaks dir .
gitleaks git .
bash scripts/verify-sdk-boundary.sh
bash scripts/test-verify-sdk-boundary.sh
```

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
7. Confirm `TestReviewedSDKShapesMatchCatalogOrIgnoredRegistry` still passes,
   and review any new SDK response fields it reports before classifying or
   ignoring them.
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

## Renovate Policy

Renovate keeps Go dependencies and GitHub Actions current, but it does not
automerge updates. GitHub Actions must remain pinned to full commit SHAs with an
inline version comment so Renovate can update the digest while preserving a
human-readable tag.

Release SBOM generation uses `cyclonedx-gomod` installed by `go install` at a
pinned Go module version instead of a GitHub Action. The Go checksum database
and Renovate custom manager are the integrity and freshness controls for that
tool; GitHub Action SHA pinning does not apply to that dependency path.

The Zscaler SDK package is handled separately from routine dependency updates.
Renovate requires dependency dashboard approval for SDK bumps and annotates those
PRs with the SDK upgrade runbook requirement.

## Advisory Scanners

CodeQL, OSV-Scanner, and gosec provide breadth signal on top of the project
specific gates. They start as advisory workflows instead of merge blockers.
gosec runs on every pull request and uploads its results as SARIF to the
Security tab (`continue-on-error`, so a finding surfaces without blocking the
build). Promote a finding class to a blocking gate only after it is triaged and
the rule is known to be stable for this repository.

## Blocking Gates

Secret scanning and the bounded fuzz smoke are merge blockers, not advisory.
`make secret-scan` (part of `make check`) runs gitleaks over the working tree
with the same `.gitleaks.toml` config as CI's `secret-scan` job, so an allowlist
gap or a real leak is caught locally. The `fuzz-smoke` CI job runs the redaction
and projection fuzz targets for a short bounded budget on every pull request so a
fuzzer-discovered no-leak bypass fails the build; the deeper weekly sweep lives
in `fuzz.yml`. `fuzz-smoke` is deliberately kept out of local `make check` to
keep the local loop fast.
