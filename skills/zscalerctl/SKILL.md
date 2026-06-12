---
name: zscalerctl
description: Use when asked about Zscaler tenant configuration, inventory, or audit — ZIA/ZPA/ZTW/ZCC/Zidentity locations, rules, policies, app segments, connectors, groups — or to export Zscaler config, and the zscalerctl CLI is installed.
---

# zscalerctl

Read-only CLI for Zscaler tenant configuration. Safe to explore: no command
can modify tenant state.

## Cold start

1. **Never guess resource names.** Enumerate first:
   `zscalerctl --format json schema list` (full catalog: products, resources,
   operations, fields) or `zscalerctl <product> --help` (one product's list).
2. **Credentials:** `zscalerctl doctor` reports which `ZSCALERCTL_*`
   variables are set or missing without contacting Zscaler. If any are
   missing, ask the operator to set them — values are environment-specific;
   do not invent them or hunt through shell config.
3. **Read:** `zscalerctl <product> <resource> list | get <id> | show`
   e.g. `zscalerctl zia locations list`

## Contract

- Piped output is deterministic JSON; failures emit a JSON error envelope on
  stderr with `kind`, `product`, `resource`.
- Exit codes: 0 ok, 2 usage, 3 credentials missing, 4 not found/unsupported,
  5 live API failure (possibly entitlement), 6 partial dump.
- `--fields a,b,c` narrows output; `zscalerctl dump --products zia --out DIR`
  writes a sanitized export.
- Absent fields are deliberately excluded by a fail-closed allow-list — do
  not try to recover them.

## Narrowing results

Output is deterministic JSON, so filter with `jq` — field names come from
`schema list`:

```sh
# rules whose name matches a pattern (case-insensitive)
zscalerctl zia url-filtering-rules list | jq '[.[] | select(.name | test("(?i)social"))]'
# rules that reference a URL category
zscalerctl zia url-filtering-rules list | jq '[.[] | select(.urlCategories // [] | index("SOCIAL_NETWORKING"))]'
# fuzzy-ish: match a term anywhere in any field
zscalerctl zia locations list | jq --arg q "branch" '[.[] | select(tostring | test($q; "i"))]'
```

For policy questions ("would this URL be blocked for this user?"), do not
guess evaluation semantics: fetch the relevant rules with this tool, then
apply the zscaler-skill (policy precedence, wildcard and SSL-inspection
semantics) if it is installed.

Full guide: `AGENTS.md` in the zscalerctl repository.
