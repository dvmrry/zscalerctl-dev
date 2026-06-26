# Agent Guide

`zscalerctl` is a read-only CLI for querying Zscaler tenant configuration.
It is safe to explore: there are no write commands, so no invocation can
change tenant state. Worst case is a usage error with a helpful message.

## Skill locations

The canonical installable skill lives at `skills/zscalerctl/`. The
`.agents/skills/zscalerctl/` tree is a generated copy for agents that discover
repo-local `.agents` content; do not edit it directly. Run
`scripts/sync-agents-skill.sh` after changing the canonical skill, and
`scripts/sync-agents-skill.sh --check` to verify drift.

## CLI reference

The authoritative command and flag list is at
[docs/cli/zscalerctl.md](docs/cli/zscalerctl.md) — generated from the live
Cobra command tree, committed, and drift-gated in CI. Use it to look up exact
flag names, types, defaults, and subcommand signatures without running the CLI.
For agent and automation workflows, start with
[docs/cli/agent-machine-workflow.md](docs/cli/agent-machine-workflow.md).

## Discover, don't guess

Resource names are not guessable. Start agent read workflows with the
config-free machine capability manifest:

```sh
zscalerctl --format json machine manifest  # machine read capabilities; no config, credentials,
                                           # SDK client, or Zscaler API contact
zscalerctl introspect                      # full CLI map: commands, flags, args, output_fields,
                                           # exit codes, and the resource catalog
zscalerctl --format json schema list       # catalog-focused view: products, resources, ops, fields
zscalerctl zia --help                      # syntax fallback only after discovery
```

`machine manifest` carries `read_only: true` metadata and lists the
`resources.read` capabilities agents should use for `list`, `get`, and `show`.
`introspect` carries `read_only: true` at the top level and is the full CLI
surface map. Use `schema list` when you need catalog field metadata, and use
help only as a syntax fallback.

Then read with `list`, `get <id>`, or `show` (singletons):

```sh
zscalerctl --format json zia locations list
zscalerctl --format json zia locations get 12345
zscalerctl --format json zia advanced-settings show
zscalerctl dump --products zia --out ./scratch-live-dump   # sanitized whole-product export
zscalerctl --format json diff ./old-dump ./new-dump        # compare two existing dumps
```

A whole-tenant `dump` can run for minutes. Add `--log-level info` to follow it
on stderr: it emits a start event with the selected resource count, one event
per resource as it is read, and a completion summary with resource and error
counts (metadata only — never record values or secrets).

## Credentials

**Agents use `ZSCALERCTL_*` environment variables — not profiles.** Profiles and
secret providers (`env:`, `file:`, `keyring:`, `cmd:`) are operator ergonomics for
interactive local workflows; the right agent path is to inject credentials via
env vars, which always take precedence over any profile setting.

Configuration is `ZSCALERCTL_*` env-first, with optional owner-only YAML
profiles selected by `--profile` and `--config`. Env variables always win over
profiles, and the Zscaler SDK's own variables are never read. Profile secret
refs can point at `env:`, `file:`, `keyring:`, or structured `cmd:` providers; `cmd:` runs
an operator-specified argv directly with no shell and can be disabled with
`ZSCALERCTL_DISALLOW_CMD=true`. Do not invent or edit provider commands while
driving the CLI — ask the operator. Run
`zscalerctl doctor`: it reports exactly which variables or profile-backed
secret refs are set or missing without contacting Zscaler. The canonical env
set is in the [README](README.md#authentication)
(`ZSCALERCTL_CLIENT_ID`, `ZSCALERCTL_CLIENT_SECRET` or `..._FILE`,
`ZSCALERCTL_VANITY_DOMAIN`, `ZSCALERCTL_CLOUD`, plus
`ZSCALERCTL_ZPA_CUSTOMER_ID` for ZPA). **Values are operator- and
environment-specific: if doctor reports variables missing, ask your operator
to set them rather than inventing values or hunting through shell config.**

## Parse output, not prose

- Piped/redirected output is always deterministic JSON (`--format auto` is
  the default; force with `--format json`). For streaming a large `list` into a
  pipeline, `--format ndjson` emits one compact record per line (`jq -c`, SIEM
  ingest); it applies to resource `list`/`get`/`show` only.
- Do not parse `pretty` or `table` output in automation; those are human
  presentation formats.
- Failures emit a JSON envelope on stderr:
  `{"error": {"kind": "...", "message": "...", "product": "...", "resource": "..."}}`
- Exit codes are a stable contract: `0` ok, `1` internal, `2` usage,
  `3` credentials missing/invalid, `4` not found/unsupported (including a
  `get` of a nonexistent id), `5` live API failure, `6` partial dump,
  `7` drift detected when `diff --fail-on-drift` is used.
- Narrow output with `--fields a,b,c` (can only narrow, never widen).
- Bound each call with `--timeout 30s` — it caps each HTTP request (not the
  whole run), so a slow or unreachable tenant can't hang you indefinitely.

## Narrowing results

`list` operations narrow in-process (no `jq` needed); field names come from
`schema list`:

```sh
zscalerctl zia url-filtering-rules list --filter name~social        # substring, case-insensitive
zscalerctl zia locations list --filter country=US --filter name~hq  # exact + AND
zscalerctl zia locations list --search branch                       # any rendered field value
```

Both run after projection/redaction (narrow only, never widen; a secret or
dropped field name matches nothing) and an empty match is exit `0` with `[]`.
For richer queries, filter the JSON with `jq`:

```sh
zscalerctl zia url-filtering-rules list | jq '[.[] | select(.urlCategories // [] | index("SOCIAL_NETWORKING"))]'
```

## Boundaries

Output is sanitized by a fail-closed allow-list; secrets never render in any
mode. `--redaction` (`standard`, `share`, or `paranoid`) only tunes value-scrubbing
of rendered fields — it never widens the allow-list, so no mode can surface a
dropped or secret field. Do not try to recover dropped fields — absence is deliberate
([docs/FIELD_COVERAGE.md](docs/FIELD_COVERAGE.md)). Resources failing with
exit `4`/`5` on a live tenant may be entitlement-gated, not broken.
Dump directories and diff reports contain sanitized but still confidential
tenant inventory; use ignored scratch paths and do not paste payloads into
tickets or chats. `diff` only compares dump directories already on disk; it
does not schedule collection or contact Zscaler. List and array fields are
compared in order; reordering a list is reported as drift.
