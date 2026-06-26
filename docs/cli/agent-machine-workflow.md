# Agent Machine Workflow

This guide is for agents, scripts, and CI jobs that drive `zscalerctl`.
The machine path is the contract to use for discovery and reads; human help,
pretty output, and tables are fallback ergonomics for operators.

## Start With The Manifest

Begin every agent read workflow with the config-free machine manifest:

```sh
zscalerctl --format json machine manifest
```

`machine manifest` is derived from the same resource catalog used by resource
execution. It never loads tenant config, resolves credentials, constructs SDK
clients, or contacts Zscaler. Use it to discover the machine-readable read
surface before making live calls:

- `version`, currently `machine.v1`
- `capabilities[].name`, currently `resources.read`
- `capabilities[].input.product` and `input.resource`
- supported `operations`, such as `list`, `get`, and `show`
- projected response schema refs, currently `projected-records` version `1`
- `meta.read_only`, `meta.shape`, and `meta.get_key` when an ID read has a
  catalog key

Do not guess product or resource names. Use `zscalerctl introspect` when you
need the complete Cobra command and flag surface, and use
`zscalerctl --format json schema list` when you need catalog field metadata.
Use `--help` only as a syntax fallback for a command you already identified
from the machine or schema surfaces.

## Execute Reads As Machine Output

Agents should request deterministic output explicitly:

```sh
zscalerctl --format json zia locations list
zscalerctl --format json zia locations get 12345
zscalerctl --format json zia advanced-settings show
```

For streaming resource reads, use NDJSON where it is supported:

```sh
zscalerctl --format ndjson zia locations list
```

`--format ndjson` applies to resource `list`, `get`, and `show` commands.
`machine manifest` itself is JSON-only. Do not parse `pretty` or `table`
output in automation; those are human presentation formats and may depend on
terminal conventions.

## Parse Contracts

Treat stdout as data and stderr as diagnostics. On failures in JSON mode,
stderr carries a machine-readable envelope:

```json
{"error":{"kind":"...","message":"...","product":"...","resource":"..."}}
```

Parse `kind`, `product`, `resource`, and `missing` fields instead of scraping
human prose. Exit codes are part of the contract:

| Code | Meaning |
| --- | --- |
| `0` | Success |
| `1` | Internal error |
| `2` | Usage error |
| `3` | Missing or invalid credentials |
| `4` | Not found or unsupported, including a `get` of a nonexistent ID |
| `5` | Live API failure |
| `6` | Partial dump |
| `7` | Drift detected by `diff --fail-on-drift` |

## Use Schemas And Field Metadata

Use the manifest, `schema list`, committed JSON Schemas, error envelopes, and
exit codes as the durable automation surface. The published schemas in
`docs/schema/` cover stable file and stream artifacts such as stderr error
envelopes, dump manifests, dump error records, redaction reports, and diff
reports. Resource records are governed by the catalog and by the
`projected-records` schema ref advertised by the machine manifest.

Rendered resource fields are projected and redacted before machine output.
`--fields` can only narrow the projected field set; it cannot request dropped
or secret fields. If a field is absent from the projected output, treat that as
intentional and do not try to recover it through raw SDK, config, credential,
or secret layers.

## Boundary Expectations

For in-process overlays, consume `internal/machine`, `internal/browser`, and
already-projected `internal/resources` values. Do not import CLI renderers,
terminal UI packages, config, credential, secret-reference, or raw SDK adapter
packages to build an alternate runtime. If an agent, UI, or workflow needs a
new capability, add a narrow projected seam and keep raw source records inside
the trusted runtime.

See [machine-contract.md](machine-contract.md) and
[core-boundary.md](core-boundary.md) for the package boundary model.
