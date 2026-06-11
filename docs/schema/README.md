# Published JSON Schemas

These are the stable, versioned JSON Schemas for the machine-readable output
`zscalerctl` emits. Downstream automation can validate output against them.

| Artifact | Where | Schema | `schema` id |
| --- | --- | --- | --- |
| Manifest | `manifest.json` in the dump | [manifest.schema.json](manifest.schema.json) | `zscalerctl.dump.manifest.v2` |
| Redaction report | `redaction_report.json` in the dump | [redaction-report.schema.json](redaction-report.schema.json) | `zscalerctl.redaction_report.v1` |
| Error record | each line of `errors.ndjson` in the dump | [dump-error.schema.json](dump-error.schema.json) | `zscalerctl.dump.error.v1` |
| Error envelope | stderr, on a failing command with JSON output | [error.schema.json](error.schema.json) | `zscalerctl.error.v1` |

The dump artifacts each carry their `schema` id as a field, so consumers can
route on it. The stderr error envelope carries no in-payload `schema` field
(its `$id` lives in the schema file); it is written when a command fails and the
resolved output format is JSON — an explicit `--format json`, or the default
`auto` on a non-terminal stdout.
The schemas are JSON Schema draft 2020-12 and use `additionalProperties: false`,
so a new field is a breaking change — bump the `schema` id (and these files) when
the artifact shape changes, per [VERSIONING.md](../VERSIONING.md).

## Resource files

Per-resource files (`resources/<product>/<name>.json`) are **not** schematized
here: their shape is the projected, redaction-filtered field set for that
resource, which varies by resource and redaction mode and is governed by the
resource catalog rather than a fixed envelope. Each file is a JSON array of
objects (or, for singleton resources, a single object). Use `zscalerctl schema
list` for the catalog-level field model.

## Drift guard

`internal/dump/published_schema_test.go` asserts that every JSON field on the
dump artifact structs appears in the matching schema's `properties` and vice
versa (and that emitted `status` values stay within the schema enums), so the
structs and these files cannot drift apart silently. The stderr error envelope
has the same guard in `cmd/zscalerctl/main_test.go`.
