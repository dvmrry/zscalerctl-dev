# zscalerctl

[![CI](https://github.com/dvmrry/zscalerctl/actions/workflows/ci.yml/badge.svg)](https://github.com/dvmrry/zscalerctl/actions/workflows/ci.yml)
[![CodeQL](https://github.com/dvmrry/zscalerctl/actions/workflows/codeql.yml/badge.svg)](https://github.com/dvmrry/zscalerctl/actions/workflows/codeql.yml)
[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/dvmrry/zscalerctl/badge)](https://scorecard.dev/viewer/?uri=github.com/dvmrry/zscalerctl)
[![OpenSSF Best Practices](https://www.bestpractices.dev/projects/13176/badge)](https://www.bestpractices.dev/projects/13176)
[![Release](https://img.shields.io/github/v/release/dvmrry/zscalerctl)](https://github.com/dvmrry/zscalerctl/releases)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.26-00ADD8)](go.mod)

> Unofficial, security-first, **read-only** CLI for authorized Zscaler administrators — safe configuration query, inventory, and sanitized exports.

A single Go binary, agentic/pipeline-first by design, so one reviewed command can replace duplicated API snippets across your workflows. Not affiliated with, endorsed by, or sponsored by Zscaler.

## What you get

- **Read-only by design** — no write commands and no raw API executor (and none planned for v1).
- **Agentic-first output** — deterministic JSON whenever output is piped or redirected; a styled `pretty` view on a terminal, both rendered from the same sanitized data.
- **Explicit auth only** — reads only `ZSCALERCTL_*` config; never the Zscaler SDK's own env vars or files.
- **Leak-resistant** — allow-list projection into safe views, with every SDK field classified or deliberately excluded on the record, test-enforced ([docs/FIELD_COVERAGE.md](docs/FIELD_COVERAGE.md)); redaction and secret scanning as defense-in-depth for values.
- **Sanitized, fail-closed dumps** — releases ship checksums, per-target SBOMs, and provenance attestations.
- **Stable automation contract** — documented exit codes and JSON error envelopes.

Reviewed read/list/show coverage spans **ZIA, ZPA, ZTW, ZCC, and Zidentity**. The catalog is the source of truth:

```sh
zscalerctl --format json schema list
```

See [docs/RESOURCES.md](docs/RESOURCES.md) for the resource reference and [docs/RESOURCE_QUEUE.md](docs/RESOURCE_QUEUE.md) for deferred and queued work.

## Install

Release archives for macOS, Linux, and Windows include checksums, CycloneDX SBOMs, and GitHub provenance attestations. See [docs/INSTALL.md](docs/INSTALL.md) for verification, credentials, proxy, completions, and platform notes.

With a Go toolchain (no checkout needed):

```sh
go install github.com/dvmrry/zscalerctl/cmd/zscalerctl@latest
zscalerctl version
```

From a checkout (rerun after every `git pull` — the binary on PATH does not
update itself):

```sh
go install ./cmd/zscalerctl
```

## Quick start

```sh
# Inspect local config without contacting Zscaler
zscalerctl doctor
zscalerctl auth status

# Browse the reviewed catalog
zscalerctl schema list

# Read resources
zscalerctl zia locations list
zscalerctl zpa server-groups list
zscalerctl ztw workload-groups list

# Diagnostic lookup: which categories does this domain/URL resolve to?
zscalerctl zia url-lookup example.com

# Write a sanitized, fail-closed dump
zscalerctl dump --products zia --out ./scratch-live-dump
```

Output defaults to `--format auto`: a terminal gets the human-readable `pretty` view, while a pipe, redirect, or `--output` file gets JSON, so automation is the default surface without a flag. Force it either way with `--format json` or `--format pretty` (or `--format table` for the tab-separated form). The `pretty` view is a styled overlay of the same sanitized data — it adds no fields and passes through the same redaction. Use `--output <path>` to write a single command's output to a restricted file; use `dump --out <dir>` for dump directories (the two are intentionally not combined).

The examples above are written for interactive use. Scripts and agents should pass `--format json` explicitly rather than rely on auto-detection — a PTY-based harness can read as a terminal and receive the `pretty` view. Dump directories contain sanitized but still confidential tenant inventory; keep them in ignored scratch paths and do not paste dump payloads into tickets or chats. The agent-oriented guide is in [AGENTS.md](AGENTS.md).

## Authentication

OneAPI is the default. The CLI reads only explicit `ZSCALERCTL_*` values:

```sh
export ZSCALERCTL_CLIENT_ID=<client-id>
export ZSCALERCTL_CLIENT_SECRET_FILE=/path/to/owner-only/secret-file
export ZSCALERCTL_VANITY_DOMAIN=<vanity-domain>
export ZSCALERCTL_CLOUD=PRODUCTION
export ZSCALERCTL_ZPA_CUSTOMER_ID=<zpa-customer-id>          # ZPA resources only
export ZSCALERCTL_ZPA_MICROTENANT_ID=<zpa-microtenant-id>   # optional, ZPA microtenants
```

ZIA legacy credentials are supported for ZIA resources. Legacy, proxy, Windows, and secret-file details live in [docs/INSTALL.md](docs/INSTALL.md). Configuration is environment-only (no config file). Corporate proxy use is opt-in via `ZSCALERCTL_PROXY_FROM_ENV=true`.

## Automation contract

Exit codes are stable for scripting:

| Code | Meaning |
| --- | --- |
| `0` | Success |
| `1` | Internal or unclassified failure |
| `2` | Usage or argument error |
| `3` | Missing or invalid credentials |
| `4` | Product/resource not found or unsupported |
| `5` | Live Zscaler API access failure |
| `6` | Partial dump written (inspect `manifest.json` and `errors.ndjson`) |

Configuration and proxy errors (an invalid `ZSCALERCTL_*` value) map to `2`. With `--format json` — or the default `auto` when stdout is not a terminal — a failing command emits a redacted envelope on stderr:

```json
{ "error": { "kind": "missing_credentials", "message": "missing zscaler API credentials" } }
```

List results can be narrowed in-process: `--filter key=value` keeps records whose rendered field equals the value, `--filter key~value` matches a case-insensitive substring, repeated filters must all match (AND), and `--search term` keeps records where any rendered field value contains the term. Both apply to `list` operations only (anywhere else is a usage error, exit `2`) and run strictly after projection and redaction, so they can narrow but never widen the sanitized output — a dropped or secret field name simply matches nothing. No matches is success: exit `0` with an empty array/table.

```sh
zscalerctl zia locations list --filter country=US --filter name~branch
```

## Security posture

- Defensive administration only — not an exploitation, credential-discovery, bypass, or traffic-interception tool.
- Primary leak control is **allow-list projection** into safe view records; redaction and secret scanning are defense-in-depth, not a license to render raw API responses.
- **v1 ships no write commands and no generic raw API executor.**

Full model: [docs/THREAT_MODEL.md](docs/THREAT_MODEL.md) · [docs/DATA_CLASSIFICATION.md](docs/DATA_CLASSIFICATION.md).

## Documentation

**Usage**
- [AGENTS.md](AGENTS.md) — cold-start guide for AI agents driving the CLI
- [skills/zscalerctl/](skills/zscalerctl/SKILL.md) — canonical installable agent skill; [`.agents/skills/zscalerctl/`](.agents/skills/zscalerctl/SKILL.md) is a generated discovery copy kept in sync by `scripts/sync-agents-skill.sh --check`
- [docs/INSTALL.md](docs/INSTALL.md) — install, verify, configure
- [docs/RESOURCES.md](docs/RESOURCES.md) — enabled resource reference
- [docs/RESOURCE_QUEUE.md](docs/RESOURCE_QUEUE.md) — deferred / queued / excluded state

**Security & governance**
- [docs/THREAT_MODEL.md](docs/THREAT_MODEL.md)
- [docs/DATA_CLASSIFICATION.md](docs/DATA_CLASSIFICATION.md)
- [docs/ZSCALER_SENSITIVE_DATA.md](docs/ZSCALER_SENSITIVE_DATA.md)
- [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)

**Project**
- [docs/VERSIONING.md](docs/VERSIONING.md) · [docs/DEPENDENCY_POLICY.md](docs/DEPENDENCY_POLICY.md) · [docs/RELEASE_CHECKLIST.md](docs/RELEASE_CHECKLIST.md)
- [docs/SDK_SURFACE_INVENTORY.md](docs/SDK_SURFACE_INVENTORY.md) · [docs/ZSCALER_PRODUCT_SCOPE_PLAN.md](docs/ZSCALER_PRODUCT_SCOPE_PLAN.md) · [docs/SCRIPTS.md](docs/SCRIPTS.md)

## Development

```sh
make check        # full gate: tests, vet, vuln, staticcheck, semgrep, secret scan, doc + registry verifiers
make live-smoke   # validate the live-smoke resource manifest (artifacts to a temp dir)
```

## Contributing

Open an issue to discuss a change first, then submit a pull request against
`main`. Every PR must pass `make check` and carry exactly one `semver:*` label;
new functionality must include tests. Security-sensitive reports go through
[SECURITY.md](SECURITY.md), not the public tracker.

License: [Apache-2.0](LICENSE).
