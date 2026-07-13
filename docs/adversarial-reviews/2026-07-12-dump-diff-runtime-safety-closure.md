# Builder Handoff

## Intent

Remediate the eight independently validated release blockers at local checkpoint
`78903f9`: selective-diff correctness, complete dump-artifact reconciliation,
destructive `--force` authorization, interrupted/concurrent publication,
missing-credential and HTTP-timeout taxonomy, and macOS ACL enforcement.

## Base / Head

- Base commit: `78903f9d10a62621d9f5584fa684ae58934b9141`
- Head: reviewed uncommitted working tree on `feature/stdio-engine-api`
- Process baseline: `origin/main` at
  `b0597dfb8e673a06d99995e6e1360cfcc709f0a8`

## Files Changed

- Dump validation/publication and platform primitives under `internal/dump/`
- Diff collection-state and artifact admission under `internal/diff/`
- Runtime/machine/CLI error mapping under `internal/runtime/`,
  `internal/machine/`, `internal/cli/`, and `cmd/zscalerctl/`
- Darwin ACL validation under `internal/fileperm/`
- Windows native-test selection in `.github/workflows/ci.yml`
- Focused fixtures/goldens and the categorized surface-change record
- Architecture, resource, threat-model, installation, versioning, schema, and
  machine-contract documentation

## Expected Delta

- Diff distinguishes successful, failed, and not-selected collection state;
  selected-scope mismatch is a permanent usage failure and no absent/failed
  state becomes fabricated empty success.
- Manifest, redaction report, errors, paths, counts, catalog shape, and file
  inventory are reconciled before diff or destructive authorization.
- Dump is built and validated in private staging, then published at the
  directory boundary. Existing-directory replacement requires atomic exchange;
  unsupported platforms/filesystems fail closed with typed exit 4.
- Force accepts only an empty tree or a complete current-catalog artifact and
  never a partial dump, prefix spoof, foreign file, unsafe path, or symlink.
- Old-root and failed-staging cleanup use whole-root private quarantine,
  identity revalidation, ACL/immutable/append-only preflight, and restore/rollback
  before irreversible deletion. Public staging/quarantine pathnames are never
  unlinked after identity checks.
- Product-specific missing credentials retain safe variable metadata and exit 3.
- Configured HTTP-client timeouts retain `deadline_exceeded` identity and exit 5.
- macOS owner-only secret files reject any extended ACL despite mode `0600`.

## Generated Artifacts

- Added the missing-ZPA-customer stdout/stderr golden and categorized it in
  `surface_changes.md`.
- Added redaction-report companions and canonical ordinary-list shape encoding
  to the two existing diff fixture directories.
- Clarified the existing v2 manifest schema description without changing its
  schema id or incompatible structure.
- No CLI reference, field-coverage artifact, introspection schema, machine
  manifest fixture, or agent skill changed.

## Verification Before Review Closure

- Full uncached Go suite: pass
- Full/affected race suites: pass
- Vet, staticcheck, semgrep, formatting, and diff checks: pass
- Vulnerability and secret scans: pass
- Docs, CLI docs, machine contract, surface manifest, core/experiment boundary,
  action pin, credential-workflow, release, script, and skill-sync gates: pass
- Linux amd64/arm64, Windows amd64, and Darwin cross-build/test binaries: pass
- Native Darwin immutable-file, delete-deny ACL, extended secret-file ACL,
  post-validation insertion, same-name substitution, and staging/discard
  substitution regressions: pass
- Original missing-ZPA-customer and selective-scope reproductions now return the
  intended structured errors and exits with empty stdout.

## Known Platform Boundary

Native Windows publication tests are wired into the required `windows-latest`
job. They cross-compile locally; native execution remains the CI proof boundary
because no Windows runtime is available on the review Mac. Windows publishes a
new destination but intentionally fails closed for existing-directory exchange.

# Adversarial Review

Fresh-context reviewer: Heisenberg (`019f58cc-2fb1-7da2-b90d-a5fa5a4b91fb`, Sol xhigh) and Curie (`019f58cc-31b6-7e21-bb6f-557a8e7c35a5`, Luna max) for the initial implementation; Carver (`019f5974-b932-7252-a6a4-c714af046a00`, Sol xhigh) and Poincare (`019f5974-bba8-7300-822b-7640be91f6c4`, Luna max) for the post-CI correction

Both reviewers inspected the actual working tree read-only and did not share
the builder's implementation context. Each began with `request changes`. The
builder fixed the union, mapped every finding to root cause, regression, and
verification, and returned only the changed surface for repeated narrow
re-review. Neither reviewer modified files.

## Findings Resolved

1. Post-validation insertion into an unchanged target inode could be
   recursively deleted. Post-exchange full revalidation, exact inventory, and
   whole-root quarantine now preserve and roll back foreign entries.
2. Windows/unsupported exchange used a two-rename backup gap. The fallback was
   removed; unsupported exchange performs no move and returns typed
   `unsupported_operation`/exit 4.
3. Stable unsupported-operation exit behavior contradicted introspection.
   Machine-kind exits are centralized; unsupported capability/operation map to
   permanent discovery exit 4, with every stable kind covered by tests.
4. Arbitrary/mismatched manifest resource shapes were accepted. Runtime
   vocabulary, diff catalog, and force current-catalog shape checks now agree.
5. Windows `os.OpenRoot` could block `MoveFile`. Windows closes the validated
   staging root immediately before new-destination exclusive rename; native
   dump tests are selected by Windows CI.
6. Valid partial dumps authorized force deletion. Force is complete-only.
7. Cleanup could fail after commit on mode, immutable, append-only, or ACL
   constraints. Preflight rejects predictable failures before exchange and
   native Darwin regressions prove immutable and delete-deny cases.
8. Path-only cleanup allowed same-name substitutions. Cleanup plans bind inode
   identities, move the whole root atomically into private quarantine, validate
   the moved root and every entry, and restore/rollback on mismatch.
9. Failed staging cleanup still unlinked a public staging path. It now relocates
   the entire staging root before deletion; the full deferred-failure regression
   preserves a substituted staging sentinel.
10. Deleting the outer quarantine repeated the same parent-entry race. Public
    discard/cleanup outer directories are never unlinked and remain empty 0700
    directories after normal cleanup.
11. Staging-clear failures could silently retain confidential data under
    discard. Full staging preflight runs before/after hooks, clear failures
    attempt whole-root restoration, and documentation marks the irreducible
    simultaneous cleanup-and-restore failure as confidential.

## Post-CI Correction Review

The published PR then exposed platform-specific flaws that the first local
review did not reproduce: Linux could immediately reuse an inode after a
same-name substitution, and Windows path-derived identity/mode assumptions did
not match native filesystem behavior. Carver and Poincare independently
reviewed the corrective working tree read-only. Both began with `request
changes`; the builder mapped and fixed their union before narrow re-review.

### Follow-up Findings Resolved

1. Saved path-derived identity was not stable across same-name replacement on
   every platform. Artifact and root identities are now captured from opened
   handles; repeated Linux substitution regressions no longer depend on inode
   non-reuse.
2. Per-entry deletion and the removed `PrepareOutputDir` path could not sustain
   a safe lease across validation and deletion. The dead API was deleted.
   Publication now relocates the validated old root as one directory into a
   private quarantine, fully revalidates it through the still-open root, and
   clears only through that handle.
3. Initial exchange, rollback, quarantine relocation, and failed-staging
   cleanup remained pathname operations in a potentially shared namespace.
   POSIX publication now requires an operator-owned, non-group/world-writable
   immediate parent; resolved ancestors must be current-user or root owned, and
   writable ancestors must use sticky protection for the next operator-owned
   component. Later operations use the resolved parent, and a deterministic
   symlink-swap regression proves the lexical path cannot redirect them.
4. macOS extended ACL permits could grant namespace mutation while mode bits
   remained `0700`. Every resolved ancestry component is now identity-bound to
   an open handle and its `ATTR_CMN_EXTENDED_SECURITY` data is parsed with
   bounds checks. Permit, unknown, and malformed ACEs fail closed; deny-only
   ACLs remain accepted. Native Darwin tests cover both immediate-parent and
   ancestor permits plus the deny-only case.
5. Computing `Dir` before `Clean` rewrote `export/` as `export/export`.
   Destination cleaning now precedes force checks and parent selection;
   regressions cover a trailing separator and final `.`/`..` components.
6. Windows tests asserted synthetic POSIX mode bits and did not prove the
   documented DACL inheritance model. Mode assertions are platform split, and
   a native test restricts the parent DACL then validates the root, nested
   directories, resources, manifest, report, and partial-error file. Existing
   directory replacement remains unsupported and fails closed.

### Follow-up Verification

- Focused namespace, ACL, path-cleaning, substitution, and rollback regressions:
  pass once and for 100 repeated iterations where applicable.
- Full dump package and race detector: pass.
- Darwin amd64/arm64, Linux amd64, and Windows amd64 dump-test builds: pass.
- Full uncached `env -u GOFLAGS make check`: pass on the final implementation
  tree, including all Go/race/static/security/contract/docs/release gates.
- Both follow-up reviewers confirmed the blocking findings fixed. Each approved
  with only documentation/test-strengthening nits; neither identified a new
  blocking issue.

## Post-Push CI Follow-up

Native CI confirmed the Windows dump, restricted-parent DACL, file-permission,
and secret-reference tests all pass. The Windows job failed later on two stale
CLI assertions: one still expected existing-directory force replacement to
succeed, and one compared the canonical long error path to the runner's short
temporary path. The CLI regressions now preserve the Windows fail-closed
contract, assert the old manifest remains unchanged, and derive expected error
text through the same `EvalSymlinks` canonicalization as production.

GitHub code scanning also imported 19 gosec annotations from the larger PR. The
builder triaged every annotation instead of weakening the scanner:

- Fragment, completion, dump-warning, and diff counts now use checked
  nonnegative `SafeInteger` conversions with the JavaScript maximum enforced.
- Signal exit codes are produced as `int32` directly, and POSIX UID comparisons
  widen rather than narrow values.
- Diff failure paths preserve their primary error and check every file-close
  result.
- The two caller-selected `os.Open` sites have narrow audited suppressions: the
  handles read no content and exist only to bind already-checked identity and
  inspect ACL/removal metadata.
- Darwin syscall-pointer suppressions document the synchronous ABI lifetime,
  and ACL offset/length values remain `int64` until bounded by the 64 KiB
  response buffer.

Exact gosec v2.26.1 now reports zero findings. Affected unit and race suites,
vet, staticcheck, Windows CLI/engine cross-compilation, and the full repository
gate pass. Carver and Poincare re-reviewed this post-CI delta; Poincare approved
and Carver approved with one non-blocking pre-existing nit: unsupported 32-bit
builds still encounter an unrelated untyped `MaxSafeInteger` formatting
constant, while every released target is 64-bit.

## Final Review

The initial and post-CI review loops independently verified the frozen cleanup,
namespace, ACL, path-normalization, and platform-test boundaries. The remaining
nits do not weaken the implementation contract: architecture/threat-model docs
carry the detailed handle-binding and ownership rationale, while focused tests
already fail on the reproduced pre-clean destination bug and cover the complete
ACL authorization boundary.

Verdict: approve with nits
