# Builder Handoff

## Intent

fix(cli): pin --output and config init parents without lexical cleaning

Both writers split the destination with filepath.Split and open the parent
with os.OpenRoot, so the kernel resolves paths such as link/../file to the
file the OS would actually write, and the temp-file/rename sequence stays
inside that pinned directory. A destination without a file name (trailing
separator, "." or "..") is a usage error before anything is created. The
dump-specific ownership, ancestry and ACL admission rules do not apply to
these writers, so destinations main accepted (for example /tmp/x.json) still
work, and temp names keep os.CreateTemp's length budget so long file names
fit NAME_MAX.


## Base / Head

- Base: `main` at `50135e4`.
- Head reviewed: `bd556f0` on `pr/output-parent` (one squashed commit). This artifact
  is added to that commit afterwards and changes no reviewed code.

## Files Changed

- `cmd/zscalerctl/config_init_windows_test.go`
- `internal/cli/app.go`
- `internal/cli/commands_config_schema_auth.go`
- `internal/cli/config_init_test.go`
- `internal/cli/destination_inspection_test.go`
- `internal/cli/filesystem_race_posix_test.go`
- `internal/cli/output_file_posix_test.go`
- `internal/cli/output_file_test.go`
- `internal/cli/output_file_windows_test.go`
- `internal/fileperm/fileperm.go`
- `internal/fileperm/fileperm_root_posix.go`
- `internal/fileperm/fileperm_root_windows.go`

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

None identified.

**Prior blocking finding — fixed by source trace.** In `bd556f0:internal/cli/commands_config_schema_auth.go:135–144`, any `root.Lstat` error other than not-exist now triggers `os.Lstat(path)`. Consequently, `ERROR_SHARING_VIOLATION` reaches pathname attribute inspection; an existing config without `--force` returns the existing-file `UsageError`. The unchanged CLI error mapping produces kind `usage`, exit **2**, before removal or writing.

`cmd/zscalerctl/config_init_windows_test.go` covers the exact exclusive-handle sequence: successful `os.Lstat`, sharing violation from `root.Lstat`, existing-config guidance, `ErrUsage`, JSON error kind, exit 2, and unchanged contents. This test was inspected, not executed.

## Non-Blocking Risks

- Runtime verification was unavailable under the supplied environment limits. Builder-reported builds, tests, race checks, and platform checks remain unverified. `git diff --check main...bd556f0` passed.
- Parent pinning is conditional: permission-denied `OpenRoot` calls retain pathname operations, and failed root metadata inspection can fall back to pathname inspection. These retain race exposure present on `main`.
- `config init --force` still removes the existing config before creating its replacement. A subsequent failure can lose the old config; this is pre-existing.

## Machine Contract Review

Confirmed the process-document baseline on `main` (`50135e4e9af4c62169f9290a05bf6325631a895f`) and reviewed `main...bd556f0` through Git.

JSON/NDJSON payloads, schemas, manifest, introspection, and dump/diff formats are unchanged. The intentional destination-validation change rejects trailing separators and terminal `.` or `..` as usage errors. Ordinary successful output and config-init path reporting retain their shapes.

Source and tests cover raw symlink/`..` resolution, ancestor substitution, write/search-only directories, sticky shared directories, destination inspection failures, and long output filenames. CLI declarations remain unchanged.

## Safety Review

Redaction, projection, field coverage, and sensitive-data classification are unchanged; field narrowing cannot gain additional data through this change.

When the parent opens successfully, output creation, cleanup, and publication use that pinned directory. Config creation remains exclusive and validates owner-only permissions on the opened file. Windows output ACL limitations remain documented. Dump/diff confidentiality requirements are unchanged.

## Generated Artifact Review

No generated documentation, schemas, fixtures, catalogs, or agent-skill copies changed. The diff contains writer implementation changes and added tests, with no removed tests or weakened assertions. No generated-surface discrepancy was identified through source inspection; generation checks were not executable here.

## Verdict

Verdict: approve