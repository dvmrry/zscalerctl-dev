# CLI Architecture Baseline

This document records the post-Cobra baseline for `zscalerctl`. It is meant to
keep cleanup and future UI work from erasing intentional command-boundary
contracts.

## Dispatch

`App.Run` remains the entry point for the CLI package. It parses global flags
with `parseGlobal`, applies the safety gates that must run before command
execution, and routes recognized commands through the Cobra command tree.

Cobra owns recognized command dispatch, help for recognized commands, local
flag parsing, shell completion, generated CLI docs, and introspection metadata.
Unknown top-level commands are intentionally kept out of Cobra's generic
unknown-command path so `unknownCommandMessage` can emit project-specific
resource hints.

## Global Flags

`parseGlobal` is the canonical parser for global flags. Cobra mirrors global
flags as persistent flags so help, completion, generated docs, and introspection
show the same surface, but `App.Run` parses and applies their values before
calling Cobra.

This preserves existing behavior for:

- `--format auto` resolution.
- `--output` buffering and owner-only file writes.
- narrowing flags such as `--fields`, `--filter`, and `--search`.
- machine-readable error formatting.

## Intentional Bespoke Seams

Some code remains outside "pure Cobra" by design:

- `unknownCommandMessage` keeps the resource-name hint for cases like
  `zscalerctl locations` or `zscalerctl --fields zia locations list`, where a
  value-taking global flag may have consumed the product token.
- `__complete` and `__completeNoDesc` bypass normal global parsing so shell
  completion stays config-free and never loads credentials.
- `shouldReinsertTerminator` preserves `--` behavior after globals are parsed
  outside Cobra but local flags are parsed inside Cobra.
- `writeUsageForHumans` prevents human usage text from being prepended to JSON
  error envelopes on stderr.
- `App.catalog` injection keeps command-tree generation, completion,
  introspection, tests, and runtime resource behavior on one catalog source of
  truth.
- Virtual resource docs remain generated from the catalog because resources are
  positional arguments under product commands, not individual Cobra leaf
  commands.

These seams should be deleted only when the owning contract is deliberately
replaced and the surface goldens are updated with a manifest entry.

## Help And Missing Commands

Bare interactive invocation renders Cobra root help. Bare non-interactive
invocation returns the machine-first missing-command error, with usage text
suppressed for JSON-compatible stderr.

Recognized commands with `--help` route to Cobra before narrowing and format
validation. Unknown non-help commands keep the resource-aware
`unknownCommandMessage` path. Unknown commands with `--help` preserve the
current help-precedes-dispatch behavior and render the global usage block
instead of Cobra's generic unknown-command error.

## Completion And Introspection

Help, completion, generated CLI docs, and `introspect` must remain config-free.
They may read the static catalog and command metadata, but they must not load
operator config, resolve secret refs, construct a live reader, or contact
Zscaler.

## Resource Read Runtime

Catalog resource `list`, `show`, and `get <id>` commands route through
`internal/machine.Executor` after CLI parsing, config loading, and reader
construction have already happened. The executor only sees a narrow projected
loader capability and returns projected machine response records; it does not
own config, credentials, SDK clients, Cobra commands, or renderers.

`internal/machineio` provides the small JSON request/response adapter helpers
for stdio-style machine consumers. It decodes `machine.Request`, calls a
machine executor, and encodes `machine.Response`; it does not own CLI rendering,
stderr envelopes, exit codes, config, credentials, or SDK clients.

`internal/browser` is the shared projected resource-loading seam below the
machine executor. It may use an injected reader to call list/show/get, but raw
`resources.SourceRecord` values must be projected and verified inside that
seam before machine responses or renderers see them.

Future overlays should reuse the same machine/browser/resources seams. They
must not import `internal/cli`, `internal/output`, `internal/config`,
`internal/credentials`, `internal/secretref`, `internal/secret`, or
`internal/zscaler` to bypass CLI/runtime ownership.

## Surface Changes

CLI surface changes are gated by the golden tests under
`cmd/zscalerctl/testdata/surface/`. Any intentional change to command output,
help text, exit behavior, or machine-readable envelopes must be recorded in
`cmd/zscalerctl/testdata/surface/surface_changes.md`.

The current baseline is surface-preserving. Pretty formatting and color policy
changes should remain deliberate because they affect TTY detection,
stdout/stderr boundaries, redaction, and golden output. Interactive UI work
belongs outside the normal CLI binary unless a future change deliberately
introduces a UI-agnostic core boundary first.
