---
name: zscalerctl
description: Use when asked about Zscaler tenant configuration, inventory, or audit — ZIA/ZPA/ZTW/ZCC/Zidentity locations, rules, policies, app segments, connectors, groups — or to export Zscaler config, when `zscalerctl` is available or should be checked.
---

# zscalerctl

Read-only CLI for Zscaler tenant configuration. Safe to explore: no command
can modify tenant state.

## Cold start

1. **CLI missing?** If `zscalerctl` is not on `PATH`, ask the operator to
   install it — do not fall back to raw Zscaler APIs or SDK environment
   variables.
2. **Never guess resource names.** Start with the config-free machine
   capability manifest: `zscalerctl --format json machine manifest`. It lists
   the `resources.read` product/resource pairs, supported `list`/`get`/`show`
   operations, projected-record schema refs, and read-only metadata without
   loading config, resolving credentials, constructing SDK clients, or
   contacting Zscaler. Use `zscalerctl introspect` when you need the full CLI
   command/flag surface, `zscalerctl --format json schema list` when you need
   catalog field metadata, and `zscalerctl <product> --help` only as a syntax
   fallback after discovery.
3. **Credentials:** Use `ZSCALERCTL_*` environment variables — not profiles.
   Profiles and secret providers (`env:`, `file:`, `keyring:`, `cmd:`) are
   operator ergonomics for interactive local workflows; env vars are the right
   agent path and always take precedence. `zscalerctl doctor`
   reports which env values or profile-backed secret refs are set or missing
   without contacting Zscaler. Profile secret refs can use `env:`, `file:`, `keyring:`, or
   structured `cmd:` providers; `cmd:` executes an operator-specified argv with
   no shell and can be disabled with `ZSCALERCTL_DISALLOW_CMD=true`. If any are
   missing, ask the operator to set them — values and provider commands are
   environment-specific; do not invent them or hunt through shell config.
4. **Read:** `zscalerctl --format json <product> <resource> list | get <id> | show`,
   e.g. `zscalerctl --format json zia locations list`. Pass `--format json`
   explicitly rather than relying on piped auto-JSON; use `--format ndjson`
   for streaming resource `list`/`get`/`show` reads when useful.

## Contract

- Machine consumers use JSON or NDJSON, not `pretty` or `table`. Failures emit
  a JSON error envelope on stderr with `kind`, `product`, `resource`.
- Exit codes: 0 ok, 2 usage, 3 credentials missing, 4 not found/unsupported,
  5 live API failure (possibly entitlement), 6 partial dump, 7 drift detected
  when `diff --fail-on-drift` is used.
- `--fields a,b,c` narrows output; `zscalerctl dump --products zia --out DIR`
  writes a sanitized export. A long dump is silent by default; add
  `--log-level info` for start, per-resource, and completion progress on stderr.
- `zscalerctl --format json diff OLD_DUMP_DIR NEW_DUMP_DIR` compares two
  existing dumps. It does not schedule collection or contact Zscaler; use cron,
  CI, or another scheduler to create dumps on a cadence.
- Absent fields are deliberately excluded by a fail-closed allow-list — do
  not try to recover them.

## Narrowing results

`list` narrows in-process — no `jq` needed. Field names come from
`schema list`; both flags run after redaction (narrow only, never widen — a
dropped or secret field matches nothing), and an empty match is exit 0 with
`[]`:

```sh
# substring match on a field, case-insensitive
zscalerctl --format json zia url-filtering-rules list --filter name~social
# exact match, AND-ed; repeat --filter to add conditions
zscalerctl --format json zia locations list --filter country=US --filter name~hq
# --search matches a term in any rendered field
zscalerctl --format json zia locations list --search branch
```

For richer predicates (array membership, cross-field logic) that the native
flags can't express, pipe the JSON to `jq`:

```sh
zscalerctl --format json zia url-filtering-rules list | jq '[.[] | select(.urlCategories // [] | index("SOCIAL_NETWORKING"))]'
```

For policy questions ("would this URL be blocked for this user?"), do not
guess evaluation semantics: fetch the relevant rules with this tool, then
apply the zscaler-skill (policy precedence, wildcard and SSL-inspection
semantics) if it is installed.

Full guide: `AGENTS.md` in the repo checkout, or
https://github.com/dvmrry/zscalerctl/blob/main/AGENTS.md.
Agent machine workflow:
`docs/cli/agent-machine-workflow.md` in the repo checkout, or
https://github.com/dvmrry/zscalerctl/blob/main/docs/cli/agent-machine-workflow.md.
CLI reference (commands, flags, defaults):
`docs/cli/zscalerctl.md` in the repo checkout, or
https://github.com/dvmrry/zscalerctl/blob/main/docs/cli/zscalerctl.md.
