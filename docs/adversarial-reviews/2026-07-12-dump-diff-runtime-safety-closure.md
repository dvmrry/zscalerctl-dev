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

Fresh-context reviewer: Heisenberg (`019f58cc-2fb1-7da2-b90d-a5fa5a4b91fb`, Sol xhigh) and Curie (`019f58cc-31b6-7e21-bb6f-557a8e7c35a5`, Luna max)

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

## Final Review

Both reviewers independently verified the frozen final cleanup boundary and its
complete failure-path regressions. Heisenberg reported no findings and approved.
Curie reported no findings and approved. The only earlier nit—redundant harmless
preflight calls—was non-blocking and does not weaken the contract.

Verdict: approve
