# Agent Guide

`zscalerctl` is a tenant-read-only CLI for querying Zscaler configuration: no
command can change tenant state. Some commands and flags do have explicit local
and process effects. Commands that load configuration may read config or secret
files. Live reads may execute an operator-configured `cmd:` provider or platform
keyring helper before contacting Zscaler. `config init` writes a config file,
`dump --out` writes a directory (and `--force` can replace a prior validated
dump), and global `--output` creates or replaces a regular file.
Inspect the structured `effects` in `zscalerctl --format json introspect` before
delegating commands to an agent.

## Skill locations

The canonical installable skill lives at `skills/zscalerctl/`. The
`.agents/skills/zscalerctl/` tree is a generated copy for agents that discover
repo-local `.agents` content; do not edit it directly. Run
`scripts/sync-agents-skill.sh` after changing the canonical skill, and
`scripts/sync-agents-skill.sh --check` to verify drift.

## Post-build adversarial review

For high-risk agent-built changes, stop at "ready for adversarial review"
instead of self-approving. Use the workflow in
[docs/adversarial-review.md](docs/adversarial-review.md), with the builder
handoff template in
[docs/review-handoff-template.md](docs/review-handoff-template.md) and the
fresh-context reviewer run prompt in
[docs/adversarial-review-run-prompt.md](docs/adversarial-review-run-prompt.md).
The reviewer reports with
[docs/adversarial-review-template.md](docs/adversarial-review-template.md).

This applies especially to CLI surface, schema, machine-contract,
redaction/projection, field-coverage, resource catalog/reader wiring, generated
artifact, golden fixture, and skill-sync changes. The reviewer must run in a
fresh Codex context that did not implement the change, must not rely on the
builder summary as evidence, and must not implement fixes.

Treat user requests for "adversarial review" as workflow invocations, not as a
request to personally review harder. If no fresh-context reviewer or formal
review workflow is available, say that clearly and do not present a manual
review as equivalent. For named PRs, branches, or issues, inspect the process
docs from `origin/main` or the explicitly requested baseline before relying on
the current checkout.

The workflow is enforced locally by the project Codex `Stop` hook in
`.codex/hooks.json`, plus `make verify-adversarial-review` and `make check`.
When high-risk files change, the hook blocks completion until there is an
approved artifact under `docs/adversarial-reviews/`; prose in this file is not
the enforcement layer.

Implementation completion protocol: after editing files, Codex should expect
the Stop hook to run before the final response. If the hook reports that
adversarial review is required, do not claim the task is complete. Generate a
builder handoff, spawn a fresh-context reviewer, fix/recheck confirmed
findings, add the approved review artifact, then rerun
`make verify-adversarial-review`.

## CLI reference

The authoritative command and flag list is at
[docs/cli/zscalerctl.md](docs/cli/zscalerctl.md) — generated from the live
Cobra command tree, committed, and drift-gated in CI. Use it to look up exact
flag names, types, defaults, and subcommand signatures without running the CLI.
For agent and automation workflows, start with
[docs/cli/agent-machine-workflow.md](docs/cli/agent-machine-workflow.md).

## Discover, don't guess

Resource names and newer flags are not guessable. Check the installed binary
before relying on the installed skill:

```sh
zscalerctl version
```

The current development workflow has the config-free machine capability
manifest and bounded JSON list pages. A public `v0.68.1` binary may predate
`machine manifest` and `--limit/--offset`; if the manifest is unsupported,
fall back to `introspect` and `schema list`, and do not pass flags those
surfaces do not advertise. These checkout docs describe capabilities and do
not imply that a distribution release has been published.

Start a current agent read workflow with the config-free machine capability
manifest:

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

The manifest and catalog are large. Extract one resource rather than printing
the complete documents:

```sh
set -euo pipefail
PRODUCT=zia
RESOURCE=locations
zscalerctl --format json machine manifest |
  jq -e -c --arg product "$PRODUCT" --arg resource "$RESOURCE" '
    [ .capabilities[]
      | select(.name == "resources.read"
               and .input.product == $product
               and .input.resource == $resource) ] as $matches
    | if ($matches | length) != 1
      then error("resource is not advertised by this binary")
      else $matches[0]
        | {resource: .input.resource, operations,
           shape: .meta.shape, get_key: (.meta.get_key // null)}
      end
  '

zscalerctl --format json schema list |
  jq -e -c --arg product "$PRODUCT" --arg resource "$RESOURCE" --arg mode standard '
    [ .[] | select(.product == $product and .name == $resource) ] as $matches
    | if ($matches | length) != 1 then error("resource is not in the catalog")
      else $matches[0]
        | {product, name, operations, get_key,
           fields: [.fields[]
             | select(.classification != "secret")
             | select(((.allowed_modes // []) | index($mode)) != null)
             | (.json_name // .name)]}
      end
  '
```

When the manifest command is unavailable, use the same exact product/resource
selection against `introspect`:

```sh
zscalerctl --format json introspect |
  jq -e -c --arg product "$PRODUCT" --arg resource "$RESOURCE" '
    [ .commands[]
      | select(.path == ($product + " " + $resource + " list")) ] as $matches
    | if ($matches | length) != 1 then error("resource list is not advertised")
      else $matches[0] | {path, inherited_flags, effects, output_fields}
      end
  '
```

The field query is a catalog preflight. Validate requested fields against the
active mode's renderable names before a live call; a known field that the mode
suppresses is accepted by the CLI but omitted. Secret fields never render.

Then read with separate `list`, `get <id>`, or `show` commands (singletons):

```sh
zscalerctl --format json --timeout 30s zia locations list
zscalerctl --format json --timeout 30s zia locations get 12345
zscalerctl --format json --timeout 30s zia advanced-settings show
zscalerctl dump --products zia --out ./scratch-live-dump
zscalerctl --format json diff ./old-dump ./new-dump
```

If the task already supplies a valid ID, use `get` directly. For a list
result, select a small field set and prefer a bounded JSON page when
`introspect` advertises `--limit` and `--offset`:

```sh
set -euo pipefail
zscalerctl --format json --timeout 30s --fields id,name \
  --filter 'name~hq' --limit 20 --offset 0 zia locations list |
  jq -e '{matched_count: .pagination.matched_count,
          returned_count: .pagination.returned_count,
          sample: .records, has_more: .pagination.has_more,
          next_offset: .pagination.next_offset,
          collection_complete: .pagination.collection_complete}'
```

`--limit N` is positive and `--offset K` is nonnegative and requires
`--limit`. They apply to JSON resource `list` only. A page is cut from one
complete projected/filtered collection, so `collection_complete` is true on
success; each invocation recollects and offsets do not provide a cross-call
snapshot. `matched_count` is after projection, redaction, and local filters.
This output bound does not reduce API calls or collection memory. If page flags
are unavailable, keep the complete array but pipe it to a count/sample summary
instead of emitting a bare large list.

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

Introspection marks provider-backed process execution as
`configuration_dependent`. Treat it as possible unless the effective config,
environment, provider choice, and platform are reviewed and pinned.

## Parse output, not prose

- Piped/redirected output is always deterministic JSON (`--format auto` is
  the default; force with `--format json`). For a complete record stream,
  `--format ndjson` emits one compact record per line (`jq -c`, SIEM
  ingest); it applies to resource `list`/`get`/`show` only, after
  collection and projection have completed.
- Do not parse `pretty` or `table` output in automation; those are human
  presentation formats.
- Failures emit a JSON envelope on stderr:
  `{"error": {"kind": "...", "message": "...", "product": "...", "resource": "..."}}`
- Exit codes are a stable contract: `0` ok, `1` internal, `2` usage,
  `3` credentials missing/invalid, `4` not found/unsupported (including a
  `get` of a nonexistent id), `5` live API failure, `6` partial dump,
  `7` drift detected when `diff --fail-on-drift` is used.
- Narrow resource `list`, `get`, and `show` output with
  `--fields a,b,c` (can only narrow, never widen). For a JSON resource
  `list`, `--limit N` emits a bounded `records`/`pagination` page and
  requires positive N; `--offset K` is nonnegative and requires `--limit`.
  These flags are list-only, JSON-only output views over a complete collected
  result. They do not reduce API calls or memory, and each invocation has no
  cross-call snapshot.
- Prefer stdout for agent reads. `--output PATH` is a local filesystem write
  that creates or replaces a restricted regular file; use it only
  when that side effect is explicitly intended. It is not valid with `dump`.
  Publication uses a same-directory rename and is atomic on Unix; Windows
  does not have an atomic replacement guarantee. On Windows, mode `0600`
  does not restrict ACLs; choose a destination directory with an appropriately
  restricted ACL.
- Bound each request with `--timeout 30s` — it caps each HTTP request, not the
  whole list or dump run. Use a separate process/orchestrator deadline when
  the total operation must be bounded.

## Narrowing results

`list` operations narrow in-process (no `jq` needed); field names come from
`schema list`. Unknown `--filter` and `--fields` names are usage errors (exit
2) before configuration or live reads. A known field omitted by projection or
redaction is simply unavailable: it cannot be recovered and does not match a
filter. An absent field is not false, empty, or unconfigured.

```sh
zscalerctl zia url-filtering-rules list --filter name~social        # substring, case-insensitive
zscalerctl zia locations list --filter country=US --filter name~hq  # exact + AND
zscalerctl zia locations list --search branch                       # any rendered field value
```

Filters and search run after projection/redaction (narrow only, never widen;
a secret or dropped field name matches nothing), and an empty match is exit
`0` with `[]`. For richer queries, filter the JSON with `jq`:

```sh
set -euo pipefail
zscalerctl --format json zia url-filtering-rules list |
  jq -e '[.[] | select(.urlCategories // [] | index("SOCIAL_NETWORKING"))]'
```

When using a bounded page, inspect `pagination.matched_count`,
`returned_count`, `has_more`, `next_offset`, and
`collection_complete` before interpreting `records`. Do not treat a page as a
tenant snapshot.

For NDJSON, remember that `head` does not avoid the full collection. Use a
nullable-field guard in downstream predicates, such as
`select((.name // "") | test("hq"; "i"))`.

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

## Read an existing collection

When `introspect` advertises `--from-dump`, use an operator-supplied complete
dump for repeated investigation. The usual resource commands retain their JSON
shapes, field selection, filters, search and bounded list pages:

```sh
zscalerctl --format json --from-dump ./scratch-live-dump \
  --fields id,name --limit 20 zia locations list
zscalerctl --format json --from-dump ./scratch-live-dump \
  --fields id,name zia locations get 12345
zscalerctl --format json --from-dump ./scratch-live-dump \
  zia advanced-settings show
```

These commands read local files and bypass configuration, credentials, secret
providers, SDK construction and network access. `--profile` and `--config`
cannot be combined with `--from-dump`. The saved redaction mode is the default;
an explicitly different `--redaction` is rejected. This cannot recover a field
that was omitted during collection. `--output` still has its advertised local
write effect.

Only complete, internally consistent artifacts that pass current catalog and
redaction admission are accepted. A missing resource, failed collection, or
missing ID is an error; it is not an empty inventory. A present list with zero
records is a successful empty inventory. Loading is capped at 256 MiB of
aggregate serialized resource data; decoded memory can be larger. Narrowing
flags do not reduce this admission work. Each CLI invocation reloads the
directory, so externally modifying it between calls can change the answers.
The core collection object stays immutable after loading. Keep the directory
unchanged for a repeatable investigation and record which collection was used.

## Inspect reviewed field meaning

When `introspect` advertises `schema describe`, request one resource directly:

```sh
zscalerctl --format json --redaction standard schema describe zia url-filtering-rules
```

This config-free candidate contract describes selected existing fields for
`zia/locations`, `zia/url-filtering-rules`, and `zia/rule-labels`. It includes
SDK-derived types and descriptions, confirmed reference targets, and explicit
`unknown` collection ordering. It is not a complete field catalog or a policy
evaluation engine. Check `review_status`: known resources outside the pilot
return `not_reviewed` with empty `fields`, while unknown resources fail with
exit 4. Only fields renderable in the requested mode are described; the default
is `standard`, independent of configuration. Continue using `schema list` to
validate all available field names. An omitted value never establishes that a
setting is false, empty, or unconfigured.
