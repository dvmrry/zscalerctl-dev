---
name: zscalerctl
description: Use when asked about Zscaler tenant configuration, inventory, or audit — ZIA/ZPA/ZTW/ZCC/Zidentity locations, rules, policies, app segments, connectors, groups — or to export Zscaler config, when `zscalerctl` is available or should be checked.
---

# zscalerctl

Tenant-read-only CLI for Zscaler configuration: no command can modify tenant
state. Local and process effects are separate. Inspect `zscalerctl --format
json introspect` `effects` before delegation. Config-loading commands may read
local files; live reads may execute configured `cmd:` or keyring helpers;
`config init`, `dump --out`, and global `--output` write locally, and
force/overwrite modes can replace existing local data.

## Cold start

1. **CLI missing?** If `zscalerctl` is not on `PATH`, ask the operator to
   install it — do not fall back to raw Zscaler APIs or SDK environment
   variables.
2. **Never guess resource names.** Discover them with the config-free machine
   capability manifest — see [Discovery ladder](#discovery-ladder) below for
   the exact commands, their output sizes, and the things not to try. No rung
   resolves credentials, constructs an SDK client, or contacts Zscaler, but
   they differ on config: `machine manifest` and `introspect` are config-free,
   while `schema list` loads config when one is configured.
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
   Treat `configuration_dependent` effects as possible unless the effective
   config, environment, provider, and platform are reviewed and pinned.
4. **Read:** `zscalerctl --format json <product> <resource> list | get <id> | show`,
   e.g. `zscalerctl --format json zia locations list`. Pass `--format json`
   explicitly rather than relying on piped auto-JSON; use `--format ndjson`
   for streaming resource `list`/`get`/`show` reads when useful.

## Discovery ladder

Five products — `zia`, `zpa`, `ztw`, `zcc`, `zidentity` — and, at the time of
writing, 165 resources. Treat every count and size below as approximate; the
catalog grows. Climb only as far as the task needs: the raw discovery
documents are large, so do not read one whole when a lower rung answers the
question.

No rung resolves credentials, constructs an SDK client, or contacts Zscaler.
They differ on config: rungs 1, 2, and 4 are config-free, but rung 3
(`schema list`) declares `local_filesystem_read@configuration_dependent` and
loads config when one is configured — with `--config` or a profile pointing at
a missing file it exits 2 with `invalid_config` rather than returning schema.

**Rung 1 — what exists (~5 KB).** The whole catalogue, one line per resource,
reduced from the 92 KB manifest:

```sh
zscalerctl --format json machine manifest \
  | jq -r '.capabilities[] | select(.name == "resources.read")
           | "\(.meta.product)/\(.meta.resource)\t\(.operations | join(","))"'
```

Without `jq`, use a real JSON parser so a failed read is not mistaken for an
empty catalog. Do not substitute `grep`/`sed` over the pretty-printed text: it
silently yields nothing on compact JSON and reports success when the command
upstream of it failed.

```sh
set -o pipefail
zscalerctl --format json machine manifest | python3 -c '
import json, sys
for c in json.load(sys.stdin)["capabilities"]:
    if c["name"] == "resources.read":
        print(c["meta"]["product"] + "/" + c["meta"]["resource"])'
```

**Rung 2 — one product.** Filter rung 1, e.g. `... | grep "^zia/"` (102 `zia`
resources).

**Rung 3 — fields for one resource (~1 KB).** `schema list` is 732 KB whole;
select the one resource instead of reading it. This prints top-level field
names only:

```sh
zscalerctl --format json schema list \
  | jq -r '.[] | select(.product == "zia" and .name == "locations")
           | .fields[].name'
```

Records nest, and nested shape is where agents guess most. Flatten the whole
field tree to dotted paths with the redaction modes each survives — for a
deeply nested resource this is ~2 KB against ~12 KB for the raw object:

```sh
zscalerctl --format json schema list | jq -r '
  def fieldpaths($prefix):
    .[] as $f
    | ($prefix + $f.name) as $p
    | "\($p)\t\($f.allowed_modes // [] | join(","))",
      ($f.fields // [] | fieldpaths($p + "."));
  .[] | select(.product == "zia" and .name == "ssl-inspection-rules")
  | .fields | fieldpaths("")'
```

```
action.decryptSubActions	standard,share
action.decryptSubActions.serverCertificates	standard,share
```

Those dotted paths describe shape and redaction; they are **not** `--fields`
arguments. `--fields` matches top-level names only and rejects a dotted path as
an unknown field. To narrow to something nested, select its top-level parent
and extract with `jq`. A field with no mode listed never renders — do not try
to recover it.

**Which resources support `get <id>`, and by which key:**

```sh
zscalerctl --format json schema list | jq -r '
  .[] | select(.product == "zia")
  | "\(.name)\t\([.operations[].name] | join(","))\t\(.get_key // "-")"'
```

A `-` in the third column means no ID key: the resource is `list`- or
`show`-only. Note `operations` is an array of objects here but an array of
strings in `machine manifest`; the two discovery surfaces differ.

Some resources are parent/child and the catalog does not link them. The known
case: `zia locations list` returns **parent locations only** — child locations
are the separate `zia/sublocations` resource, correlated by its `parentId`
field. If a location you expect is missing, check `sublocations` before
concluding it is absent.

**Rung 4 — full CLI surface (474 KB).** `zscalerctl --format json introspect`
carries commands, flags, `effects`, and exit codes. Never a first move, and
select the one command rather than reading the document — this is the check to
run before delegating a command whose effects matter:

```sh
zscalerctl --format json introspect \
  | jq '.commands[] | select(.path == "dump") | {path, effects}'
```

Do not:

- Use `<product> --help` to find resources. It lists subcommands only —
  `zia --help` shows `url-lookup`, not the 102 `zia` resources.
- Pass `--fields`, `--filter`, or `--search` to `machine manifest`,
  `schema list`, or `introspect`. They apply to resource reads only and exit 2
  with a usage envelope.
- Request `--format table|pretty|ndjson` from `machine manifest`; its output is
  JSON only. (`schema list` and `introspect` do accept `table` and `pretty`,
  but machine consumers should stay on JSON.)

## Contract

- Machine consumers use JSON or NDJSON, not `pretty` or `table`. Failures emit
  a JSON error envelope on stderr with `kind`, `product`, `resource`.
- Exit codes: 0 ok, 2 usage, 3 credentials missing, 4 not found/unsupported,
  5 live API failure (possibly entitlement), 6 partial dump, 7 drift detected
  when `diff --fail-on-drift` is used.
- `--fields a,b,c` narrows output; `zscalerctl dump --products zia --out DIR`
  writes a sanitized export. A long dump is silent by default; add
  `--log-level info` for start, per-resource, and completion progress on stderr.
- Prefer stdout for agent reads. `--output PATH` creates or atomically replaces
  a restricted regular file and should be used only with explicit local-write
  authorization; it is not valid with `dump`.
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
