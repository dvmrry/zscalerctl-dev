# Builder Handoff

## Intent

fix(dump): never block on special files during admission or --force

Dump admission (diff, LoadCollection) and `dump --force` replacement opened
directories and files with plain blocking opens after validating them, so a
FIFO substituted at the right moment could hang the CLI forever.

- Entries are opened through OpenRootEntry: Lstat, a nonblocking open
  (O_NONBLOCK, plus O_DIRECTORY for directories on Unix), then an fstat type
  and identity check. Absolute cleanup paths use the same nonblocking,
  identity-checked open.
- Artifact inventory, --force inspection and staging cleanup enumerate through
  the validated handles with path re-checks around each listing, instead of
  fs.WalkDir/filepath.WalkDir re-opening directories by path. Each walk closes
  a directory before descending, so descriptor use stays bounded by depth.
- --force cleanup re-validates every entry before removal and fails closed
  with ErrUnsafePath; ancestry binding no longer uses a blocking os.Open.
- Diff comparison budgets bound oversized collections.
- In-process test hooks (test-hook builds only) substitute FIFOs and symlinks
  at each window; a child-process test walks a 96-level tree under a
  64-descriptor limit.

The --force cleanup changes started from the strongest entry of a worker
bake-off (DeepSeek V4.1 Flash) and were reviewed and extended here.


## Base / Head

- Base: `main` at `50135e4`.
- Head reviewed: `9df4500` on `pr/dump-admission` (one squashed commit). This artifact
  is added to that commit afterwards and changes no reviewed code.

## Files Changed

- `Makefile`
- `docs/ARCHITECTURE.md`
- `internal/diff/admission_posix_test.go`
- `internal/diff/comparison_budget_test.go`
- `internal/diff/diff.go`
- `internal/diff/inventory_admission_testhook_test.go`
- `internal/diff/inventory_replacement_testhook_test.go`
- `internal/dump/artifact.go`
- `internal/dump/artifact_test.go`
- `internal/dump/descriptor_depth_unix_test.go`
- `internal/dump/dump.go`
- `internal/dump/force.go`
- `internal/dump/parent_namespace_posix.go`
- `internal/dump/parent_namespace_unsafe_open_unix_test.go`
- `internal/dump/publication_testhook_default.go`
- `internal/dump/publication_testhook_enabled.go`
- `internal/dump/root_entry.go`
- `internal/dump/root_entry_other.go`
- `internal/dump/root_entry_unix.go`
- `internal/dump/root_entry_unix_test.go`
- `internal/dump/staging_cleanup_unsafe_unix_test.go`
- `internal/dump/unsafe_open_unix_test.go`
- `internal/dump/unsafe_walk_testhook_test.go`

## Tests Run

- `make check` (gofmt, vet, `make test` and `make race` including the
  zscalerctl_engine_testhooks runs, docs, surface manifest, machine contract and
  the other repository gates); `make vuln`: no reachable vulnerabilities.
- gitleaks with `.gitleaks.toml` over `main..HEAD`: no leaks.

## Review History

The change was built on per-area branches, each reviewed by fresh-context
reviewers (GPT-6.1 Sol, then GPT-6 Astra) until approved, and then reviewed
again as this single squashed commit against main. The review below is that
final review.

# Adversarial Review

Fresh-context reviewer: gpt-6-astra (Codex CLI), read-only, 2026-10-08

## Blocking Findings

None found.

**Prior blocking finding 1: fixed by source inspection.** At `9df4500:internal/dump/artifact.go:767`, inventory enumerates the directory. Line 798 now retains `entry.Info()` rather than replacing it with a fresh `Lstat`. Lines 805–818 open the current entry and reject a different identity with `ErrInvalidArtifact`.

For the supplied reproduction—atomically replacing `manifest.json` with another regular file after enumeration—Go 1.26.6’s root-backed directory entries retain the original metadata. The replacement handle therefore fails `os.SameFile`. Artifact validation returns the error; `loadDumpWithBudgetOptions` wraps it as `ErrInvalidDump` for both `CompareContext` and `LoadCollection`.

`internal/diff/inventory_replacement_testhook_test.go` exercises this exact substitution through all three entry points. Its execution remains unverified here.

## Non-Blocking Risks

- **Pre-existing initial-root blocking gap remains.** At `9df4500:internal/diff/diff.go:647–660`, replacing the checked directory with a FIFO before `os.OpenRoot(dir)` can still block. The installed Go 1.26.6 source opens that path without `O_NONBLOCK` or `O_DIRECTORY`. The same sequence exists on `main`; it is non-blocking under the supplied severity bar. The handoff’s “never block” wording should be narrowed accordingly.
- **Runtime verification remains outstanding.** No Go builds or tests ran in this environment. In particular, the post-enumeration regression test requires `zscalerctl_engine_testhooks`; plain `go test ./...` does not exercise it. The updated Makefile includes tagged runs in both `test` and `race`, but the builder’s reported commands do not establish that those runs occurred.
- Retain the prior review’s recommendation to include same-type, different-inode substitutions after enumeration in future review handoffs. The new regression test now covers that case.

## Machine Contract Review

The intentional contract change is a **256 MiB aggregate serialized-resource budget across both selected diff inputs**, documented in `docs/ARCHITECTURE.md:514`. `loadComparisonDumps` subtracts the first input’s consumption before admitting the second. Tests cover exact-budget success, exhausted budget, and aggregate overflow.

Oversized comparisons retain `ErrInvalidDump` classification alongside `ErrCollectionTooLarge`; the runtime maps invalid dumps to usage errors. Admission diagnostics can change because unsafe entries fail earlier.

No JSON/NDJSON shapes, dump schema, CLI signatures, manifest, introspection, or resource-catalog definitions changed.

## Safety Review

Unix entry opens now use nonblocking flags, directory opens add `O_DIRECTORY`, and opened handles undergo type and identity checks. Inventory preserves enumerated identities, revalidates directory paths around enumeration, and closes parent handles before descending.

Metadata reads now enforce their byte limits while reading. Redaction, projection, field coverage, and sensitive-data classifications are unchanged. No output-widening path was introduced; dump and diff data retain their existing sanitization and confidentiality requirements.

## Generated Artifact Review

No generated docs, schemas, goldens, resource inventories, or agent-skill copies changed. There are no generated count or ordering deltas to reconcile.

The architecture edit explains the comparison-budget change. `git diff --check main...9df4500` passed. Generation and drift checks were not run.

The three process documents were verified from the explicitly requested `main` baseline; change evidence came from Git objects, not the unrelated checkout.

## Verdict

Verdict: approve with nits