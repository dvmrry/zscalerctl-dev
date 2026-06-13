# Agent Guide

`zscalerctl` is a read-only CLI for querying Zscaler tenant configuration.
It is safe to explore: there are no write commands, so no invocation can
change tenant state. Worst case is a usage error with a helpful message.

## Discover, don't guess

Resource names are not guessable. Enumerate them first:

```sh
zscalerctl --format json schema list   # every product, resource, operation, and field
zscalerctl zia --help                  # one product's resources, human-readable
```

Then read with `list`, `get <id>`, or `show` (singletons):

```sh
zscalerctl --format json zia locations list
zscalerctl --format json zia locations get 12345
zscalerctl --format json zia advanced-settings show
zscalerctl dump --products zia --out ./scratch-live-dump   # sanitized whole-product export
```

## Credentials

Configuration is environment-only, via `ZSCALERCTL_*` variables — never
files, never the Zscaler SDK's own variables. Run `zscalerctl doctor`: it
reports exactly which variables are set or missing without contacting
Zscaler. The canonical set is in the [README](README.md#authentication)
(`ZSCALERCTL_CLIENT_ID`, `ZSCALERCTL_CLIENT_SECRET` or `..._FILE`,
`ZSCALERCTL_VANITY_DOMAIN`, `ZSCALERCTL_CLOUD`, plus
`ZSCALERCTL_ZPA_CUSTOMER_ID` for ZPA). **Values are operator- and
environment-specific: if doctor reports variables missing, ask your operator
to set them rather than inventing values or hunting through shell config.**

## Parse output, not prose

- Piped/redirected output is always deterministic JSON (`--format auto` is
  the default; force with `--format json`).
- Failures emit a JSON envelope on stderr:
  `{"error": {"kind": "...", "message": "...", "product": "...", "resource": "..."}}`
- Exit codes are a stable contract: `0` ok, `1` internal, `2` usage,
  `3` credentials missing/invalid, `4` not found/unsupported (including a
  `get` of a nonexistent id), `5` live API failure, `6` partial dump.
- Narrow output with `--fields a,b,c` (can only narrow, never widen).

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
mode. Do not try to recover dropped fields — absence is deliberate
([docs/FIELD_COVERAGE.md](docs/FIELD_COVERAGE.md)). Resources failing with
exit `4`/`5` on a live tenant may be entitlement-gated, not broken.
Dump directories contain sanitized but still confidential tenant inventory; use
ignored scratch paths and do not paste dump payloads into tickets or chats.
