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
- **Sanitized, fail-closed dumps and local drift reports** — releases ship checksums, per-target SBOMs, and provenance attestations.
- **Stable automation contract** — documented exit codes and JSON error envelopes.

Reviewed read/list/show coverage spans **ZIA, ZPA, ZTW, ZCC, and Zidentity**. The catalog is the source of truth:

```sh
zscalerctl --format json schema list
```

See [docs/RESOURCES.md](docs/RESOURCES.md) for the resource reference and [docs/RESOURCE_QUEUE.md](docs/RESOURCE_QUEUE.md) for deferred and queued work.

## Install

Release archives for macOS, Linux, and Windows include checksums, CycloneDX SBOMs, and GitHub provenance attestations. See [docs/INSTALL.md](docs/INSTALL.md) for verification, credentials, proxy, completions, and platform notes.

With Go 1.26.6 or newer (no checkout needed):

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

# Compare two existing dumps for drift
zscalerctl diff ./scratch-live-dump-old ./scratch-live-dump-new --fail-on-drift
```

Output defaults to `--format auto`: a terminal gets the human-readable `pretty` view, while a pipe, redirect, or `--output` file gets JSON, so automation is the default surface without a flag. Force it either way with `--format json` or `--format pretty` (or `--format table` for the tab-separated form). The `pretty` view is a styled overlay of the same sanitized data — it adds no fields and passes through the same redaction. Use `--output <path>` to create or replace a restricted regular file with one command's output; use `dump --out <dir>` for dump directories (the two are intentionally not combined). Dump refuses to overwrite by default; add `--force` only to replace an existing zscalerctl dump directory. Agents should inspect the structured `effects` in `zscalerctl --format json introspect` before authorizing local reads, writes, network access, or configured provider execution.

The examples above are written for interactive use. Scripts and agents should pass `--format json` explicitly rather than rely on auto-detection — a PTY-based harness can read as a terminal and receive the `pretty` view. Dump directories and diff reports contain sanitized but still confidential tenant inventory; keep them in ignored scratch paths and do not paste payloads into tickets or chats. `diff` compares two dump directories you already collected; use cron, CI, or another external scheduler if you want recurring drift checks. The agent-oriented guide is in [AGENTS.md](AGENTS.md).

For `--output`, publication uses a same-directory rename and is atomic on
Unix. Windows does not have an atomic replacement guarantee; see
[Go’s rename contract](https://pkg.go.dev/os#Rename). On Windows, file mode
`0600` does not restrict ACLs: use an output directory whose ACL already limits
access to the intended account. See [platform permissions](docs/INSTALL.md).

## Saved configuration investigations

Use an existing complete dump with normal resource commands:

```sh
zscalerctl --format json --from-dump ./scratch-live-dump --fields id,name --limit 20 zia locations list
zscalerctl --format json --from-dump ./scratch-live-dump zia locations get 12345
zscalerctl --format json schema describe zia url-filtering-rules
```

`--from-dump` bypasses configuration, credentials, secret providers and network
access. It preserves the stored redaction mode, rejects partial or invalid
artifacts, and distinguishes missing resources from empty lists. It cannot be
combined with `--profile` or `--config`, and an explicitly different redaction
mode is rejected. Each invocation admits the complete saved collection with a
256 MiB aggregate serialized-resource limit (not a decoded-memory limit), then
applies the normal narrowing options. Separate calls reload the directory.

`schema describe <product> <resource>` is a config-free semantics pilot for
ZIA locations, URL filtering rules and rule labels. It describes selected
catalog fields and confirmed references; other known resources explicitly
report `not_reviewed`. It does not change the existing `schema list` contract.
These development capabilities must be discovered on the installed binary
before use. See the [agent workflow](docs/cli/agent-machine-workflow.md),
[implementation scope and follow-on experiments](docs/CORE_INVESTIGATION_SCOPE.md),
and [synthetic agent evaluation foundation](evals/agent-workflows/README.md).

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

ZIA legacy credentials are supported for ZIA resources. Legacy, profile, proxy, Windows, and secret-file details live in [docs/INSTALL.md](docs/INSTALL.md). Environment variables remain the highest-precedence configuration path; optional owner-only profiles are for local operator convenience. Corporate proxy use is opt-in via `ZSCALERCTL_PROXY_FROM_ENV=true`.

Zidentity on OneAPI `gov`/`govus` clouds is currently unsupported because the
SDK's admin routing is not correct for those clouds. The CLI rejects those
reads before authentication or API access. See [cloud limitations](docs/INSTALL.md#configure-credentials).

## Automation contract

Exit codes are stable for scripting:

| Code | Meaning |
| --- | --- |
| `0` | Success |
| `1` | Internal, canceled, or unclassified failure |
| `2` | Usage or argument error |
| `3` | Missing or invalid credentials |
| `4` | Product/resource not found or unsupported |
| `5` | Live Zscaler API access failure, including machine request deadline exceeded |
| `6` | Partial dump written (inspect `manifest.json` and `errors.ndjson`) |
| `7` | Drift detected when `diff --fail-on-drift` is used |

Configuration and proxy errors (an invalid `ZSCALERCTL_*` value) map to `2`; machine `not_found` maps to `4`, `deadline_exceeded` maps to `5`, and `canceled` maps to `1`. With `--format json` — or the default `auto` when stdout is not a terminal — a failing command emits a redacted envelope on stderr:

```json
{ "error": { "kind": "missing_credentials", "message": "missing zscaler API credentials" } }
```

List results can be narrowed in-process: `--filter key=value` keeps records whose rendered field equals the value, `--filter key~value` matches a case-insensitive substring, repeated filters must all match (AND), and `--search term` keeps records where any rendered field value contains the term. Filters and search apply to `list` only and run after projection and redaction. Unknown `--filter` or `--fields` names fail with usage exit `2` before configuration or credentials are loaded. A known field suppressed by redaction stays omitted or nonmatching; narrowing cannot widen the sanitized output. Discover exact names with `--format json schema list`. No matches is success with an empty result.

```sh
zscalerctl zia locations list --filter country=US --filter name~branch
```

For agents, use explicit JSON and bounded output when the installed binary
advertises the page flags:

```sh
set -euo pipefail
zscalerctl --format json --fields id,name --limit 20 --offset 0 zia locations list |
  jq '{records, pagination}'
```

`--limit` must be positive; `--offset` must be nonnegative and requires
`--limit`. They apply to JSON resource lists only. The opt-in envelope contains
`records` and `pagination` with returned/matched counts, `has_more`,
`next_offset`, and `collection_complete`. Without `--limit`, JSON lists remain
arrays. Paging happens after complete collection and filtering; it limits
rendered output, not API work or memory, and separate calls are not a stable
snapshot. NDJSON likewise emits records after collection completes.

Check `zscalerctl version` and `--format json introspect` before using a skill
with an older binary. This checkout's updated skill and paging flags are not
automatically present in a previously published release. The
[agent workflow](docs/cli/agent-machine-workflow.md) includes targeted discovery,
active-mode field validation, and compatible JSON summaries for older binaries.

## Security posture

- Defensive administration only — not an exploitation, credential-discovery, bypass, or traffic-interception tool.
- Primary leak control is **allow-list projection** into safe view records; redaction and secret scanning are defense-in-depth, not a license to render raw API responses.
- **v1 ships no write commands and no generic raw API executor.**
- The read-only guarantee is tenant-scoped. `config init`, `dump --out`, and
  global `--output` have explicit local filesystem effects described by
  `zscalerctl introspect` version 2. Commands that load configuration may read
  local config or secret files, and live reads may execute an operator-configured
  credential provider; those possibilities are also explicit effects.

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
