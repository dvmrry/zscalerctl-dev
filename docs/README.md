# zscalerctl Documentation

This directory holds the project documentation beyond the root README and
GitHub security policy.

## Start Here

- [INSTALL.md](INSTALL.md) — install, verify releases, configure credentials,
  proxy behavior, and completions.
- [RESOURCES.md](RESOURCES.md) — currently enabled resource catalog and field
  classification notes.
- [RESOURCE_QUEUE.md](RESOURCE_QUEUE.md) — queued, deferred, and future resource
  work.

## Design And Security

- [ARCHITECTURE.md](ARCHITECTURE.md)
- [THREAT_MODEL.md](THREAT_MODEL.md)
- [DATA_CLASSIFICATION.md](DATA_CLASSIFICATION.md)
- [FIELD_COVERAGE.md](FIELD_COVERAGE.md) — generated per-resource classified vs
  ignored-with-reason field counts (`make field-coverage`).
- [ZSCALER_SENSITIVE_DATA.md](ZSCALER_SENSITIVE_DATA.md)

## Maintenance

- [SDK_SURFACE_INVENTORY.md](SDK_SURFACE_INVENTORY.md)
- [DEFERRED_RESOURCE_RECHECK.md](DEFERRED_RESOURCE_RECHECK.md) — pinned-SDK
  source review for resources removed after live-smoke failures.
- [ZSCALER_PRODUCT_SCOPE_PLAN.md](ZSCALER_PRODUCT_SCOPE_PLAN.md)
- [ZDX_SCOPE_PLAN.md](ZDX_SCOPE_PLAN.md)
- [VERSIONING.md](VERSIONING.md)
- [DEPENDENCY_POLICY.md](DEPENDENCY_POLICY.md)
- [cli/tui-import-boundary.md](cli/tui-import-boundary.md) — why the TUI gate
  package and Bubble Tea runtime are separated.
- [cli/tui-demo-readback.md](cli/tui-demo-readback.md) — integration-branch
  readback for the isolated TUI demo harness.
- [RELEASE_CHECKLIST.md](RELEASE_CHECKLIST.md)
- [schema/](schema/) — published JSON Schemas for machine-readable artifacts.
- [SCRIPTS.md](SCRIPTS.md) — registry of `scripts/` ownership and validation.
- [releases/](releases/) — curated release notes (most releases use generated
  notes; this holds the hand-written ones).
