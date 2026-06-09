# Published JSON Schemas

These are the stable, versioned JSON Schemas for the artifacts written by
`zscalerctl dump`. Downstream automation can validate dump output against them.

| Artifact | File in the dump | Schema | `schema` id |
| --- | --- | --- | --- |
| Manifest | `manifest.json` | [manifest.schema.json](manifest.schema.json) | `zscalerctl.dump.manifest.v1` |
| Redaction report | `redaction_report.json` | [redaction-report.schema.json](redaction-report.schema.json) | `zscalerctl.redaction_report.v1` |
| Error record | each line of `errors.ndjson` | [dump-error.schema.json](dump-error.schema.json) | `zscalerctl.dump.error.v1` |

Each artifact carries its `schema` id as a field, so consumers can route on it.
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
artifact structs appears in the matching schema's `properties` and vice versa,
so the structs and these files cannot drift apart silently.
