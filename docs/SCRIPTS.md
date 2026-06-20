# Script Registry

This registry records the ownership and validation surface for files in
`scripts/`. It is intentionally boring: every top-level script must appear here
so helper code cannot become mystery project infrastructure.

The registry is checked by `scripts/verify-script-registry.sh`, which verifies
that every top-level file under `scripts/` appears in the table and that every
registered path exists.

| Script | Category | Called by | Validation |
| --- | --- | --- | --- |
| `scripts/catalog-draft.go` | dev | Manual `go run`; exercised by `scripts/test-catalog-draft.sh` | `make verify-catalog-draft` |
| `scripts/live-smoke.go` | smoke | `make live-smoke`; manual live tenant validation | `go test ./internal/livesmoke/...` |
| `scripts/next-version.sh` | release | `.github/workflows/release.yml` | `scripts/test-next-version.sh` |
| `scripts/pr-labels-for-commit.sh` | release | `.github/workflows/release.yml` | `scripts/test-pr-labels-for-commit.sh` |
| `scripts/scaffold-resource.sh` | dev | `make scaffold-resource`; manual resource scaffolding | `scripts/test-scaffold-resource.sh` |
| `scripts/sdk-surface-inventory.go` | dev | `make sdk-surface-inventory`; manual SDK scouting | `scripts/test-sdk-surface-inventory.sh` |
| `scripts/sync-agents-skill.sh` | dev | `make verify-agents-skill`; manual `.agents` skill regeneration | `scripts/test-sync-agents-skill.sh` |
| `scripts/test-catalog-draft.sh` | test | `make verify-catalog-draft` | Self-contained fixture test |
| `scripts/test-next-version.sh` | test | `make verify-release-automation` | Self-contained release-helper test |
| `scripts/test-pr-labels-for-commit.sh` | test | `make verify-release-automation` | Self-contained release-helper test |
| `scripts/test-scaffold-resource.sh` | test | `make verify-resource-scaffold` | Self-contained scaffold test |
| `scripts/test-sdk-surface-inventory.sh` | test | `make verify-sdk-surface-inventory` | Self-contained inventory test |
| `scripts/test-sync-agents-skill.sh` | test | `make verify-agents-skill` | Self-contained generated-copy drift test |
| `scripts/test-verify-actions-pinned.sh` | test | `make verify-actions-pinned`; `.github/workflows/ci.yml` | Self-contained verifier test |
| `scripts/test-verify-ci-no-live-creds.sh` | test | `make verify-ci-no-live-creds`; `.github/workflows/ci.yml` | Self-contained verifier test |
| `scripts/test-verify-release-artifacts.sh` | test | `make verify-release-artifacts` | Self-contained verifier test |
| `scripts/test-verify-sdk-boundary.sh` | test | `make verify-sdk-boundary`; `.github/workflows/ci.yml` | Self-contained verifier test |
| `scripts/test-verify-script-registry.sh` | test | `make verify-script-registry` | Self-contained verifier test |
| `scripts/test-verify-semver-label.sh` | test | `make verify-release-automation` | Self-contained verifier test |
| `scripts/verify-actions-pinned.sh` | verify | `make verify-actions-pinned`; `.github/workflows/ci.yml` | `scripts/test-verify-actions-pinned.sh` |
| `scripts/verify-ci-no-live-creds.sh` | verify | `make verify-ci-no-live-creds`; `.github/workflows/ci.yml` | `scripts/test-verify-ci-no-live-creds.sh` |
| `scripts/gen-cli-docs.go` | dev | `make gen-cli-docs`; manual CLI-reference regeneration | `scripts/verify-cli-docs.sh` |
| `scripts/verify-cli-docs.sh` | verify | `make docs-cli-check`; `.github/workflows/ci.yml` | No companion test; checks committed docs/cli/zscalerctl.md matches the live tree |
| `scripts/verify-docs.sh` | verify | `make docs-check`; `.github/workflows/ci.yml` | No companion test yet; docs secret-pattern gate |
| `scripts/verify-licenses.sh` | verify | `make verify-licenses`; `.github/workflows/ci.yml` | `go-licenses` allow-list check for the shipped binary |
| `scripts/verify-release-artifacts.sh` | verify | `make verify-release-artifacts` | `scripts/test-verify-release-artifacts.sh` |
| `scripts/verify-sdk-boundary.sh` | verify | `make verify-sdk-boundary`; `.github/workflows/ci.yml` | `scripts/test-verify-sdk-boundary.sh` |
| `scripts/verify-script-registry.sh` | verify | `make verify-script-registry` | `scripts/test-verify-script-registry.sh` |
| `scripts/verify-semgrep.sh` | verify | `make semgrep-check`; `.github/workflows/ci.yml` | Semgrep rule fixtures under `semgrep/tests` |
| `scripts/verify-semver-label.sh` | verify | `.github/workflows/semver-label.yml`; `.github/workflows/release.yml` | `scripts/test-verify-semver-label.sh` |

The testdata directory under `scripts/` holds fixtures for script tests and is
intentionally not a registry entry because the verifier tracks top-level script
files, not fixture directories.
