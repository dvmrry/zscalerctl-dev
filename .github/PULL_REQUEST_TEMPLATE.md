## Summary

## Validation

## Baseline safety

- [ ] Does not change dispatch.
- [ ] Does not change `parseGlobal`.
- [ ] Does not change completion protocol.
- [ ] Does not change `introspect` schema/output.
- [ ] Does not change JSON/NDJSON output.
- [ ] Does not change stderr machine error envelopes.
- [ ] Any golden change has a `surface_changes.md` entry.
- [ ] For UX/TUI changes, compared against `cobra-baseline-2026-06-24` and the latest UX release tag.

## Dependency / tooling guardrails

- [ ] No workflow `curl | sh`, ad hoc installer, or `@latest`.
- [ ] Any Go tool dependency is pinned through `go.mod` / `go.sum` or a fixed version.
- [ ] Any GitHub Action is SHA-pinned.
- [ ] `go mod tidy` / `go mod vendor` produce reviewed changes only.
- [ ] `go mod verify` passes.
- [ ] `make verify-licenses` passes.
- [ ] `make vuln` passes.
- [ ] `make check` passes.
