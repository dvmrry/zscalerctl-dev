# Builder Handoff

## Intent

feat(cli): name the implicitly chosen profile on stderr after a successful read

A downstream report (2026-10-13) ran a query against the default profile
when another tenant was intended, and nothing in the output showed it.
Config loading now records how the profile was chosen (flag,
`ZSCALERCTL_PROFILE`, the file's `default_profile`, or the built-in
`default`) and how many profiles the file defines. When a live reader is
built from a profile chosen by the file's default while the file defines
several, the runtime writes
`zscalerctl: using profile "<name>" (...); pass --profile to choose` to its
`Notices` writer. The CLI collects notices for resource reads, dump,
url-lookup and admin-audit-logs and `App.Run` writes them through the
redacted stderr writer only when the whole run succeeds, so a failure's
stderr stays the single JSON error envelope. The machine stdio host leaves
`Notices` unset. Stdout, error envelopes and exit codes are unchanged; only
the profile name is printed.

## Base / Head

- Base: `integration/2026-10-09` at `e542978`.
- Reviewed heads: `2e1a894` (round 1), `0d35a3c` (round 2), `b1ec5a9`
  (round 3), each an amend of one commit.
- Final head: `b1ec5a9`. This artifact is added afterwards and changes no
  reviewed code.

## Files Changed

- `.agents/skills/zscalerctl/SKILL.md`
- `docs/INSTALL.md`
- `internal/cli/admin_audit_logs.go`
- `internal/cli/app.go`
- `internal/cli/app_internal_test.go`
- `internal/cli/dump_diff.go`
- `internal/cli/url_lookup.go`
- `internal/config/config.go`
- `internal/config/load.go`
- `internal/config/load_test.go`
- `internal/config/profile.go`
- `internal/runtime/profile_notice_test.go`
- `internal/runtime/runtime.go`
- `man/zscalerctl.1`
- `skills/zscalerctl/SKILL.md`

## Tests Run

- `go test ./...`, `go vet` on changed packages, `make docs-check` (Go
  1.26.9): pass.
- Throwaway two-profile config whose client secret is an unset env ref (so
  every command fails before any network call): `zia locations list`,
  `zia url-lookup`, `zia admin-audit-logs status` and `dump` exit 3 with
  stderr that parses as one JSON document and holds no notice. The success
  path is covered by `TestLiveReadNoticeIsWrittenOnlyWhenTheCommandSucceeds`
  and the runtime notice test, not by a live run.

## Review History

Reviewed with no repository access by a fresh-context local model given the
diff, surrounding source and call sites, and this repository's reviewer
prompt and template: Strata 0.1.43.1 serving Qwen3.8-Flash-Next (IQ3_S) on
gm01. The reviewer could not run commands; the builder reproduced each
blocking finding as a failing test before fixing it.

- Round 1, request changes: the notice was written straight to stderr
  before credential resolution, so a failed read put a text line before the
  JSON error envelope; it also bypassed the stderr redactor. Fixed by
  buffering notices in the CLI and writing them through the redactor.
- Round 2, request changes: the flush ran when Cobra succeeded, but `Run`
  can still fail afterwards (`--output` publication, redactor close). Fixed
  by moving the flush to `App.Run`'s own result; the CLI test gained an
  `--output` failure case that fails on the round-2 code.
- Round 3, approve with nits (below). Its open question about `applyOptions`
  is settled: it changes only redaction, no-cache and timeout, never the
  profile.

Accepted and not changed: per-command wiring tests for dump, url-lookup and
admin-audit-logs (each passes `Notices: &a.notices`; the shared buffering
contract is tested once); best-effort notice writes; `App` is not used
concurrently; a panic after a notice is buffered could print it before the
process dies, and `main` does not recover panics into envelopes.

# Adversarial Review

Fresh-context reviewer: Strata 0.1.43.1 (Qwen3.8-Flash-Next IQ3_S, gm01), read-only, no command execution, 2026-10-13 (round 3)

Baseline confirmed from the review inputs: base `e542978` on `integration/2026-10-09`, head `b1ec5a9`, single commit, review round 3. Process-doc baseline is `docs/adversarial-review-template.md`. I did not run commands; the review is based on the shown diff, source artifacts, and tests.

## Blocking Findings

No blocking findings.

The round-1 and round-2 blocking issues are fixed by the shown code, for normal CLI return paths.

### Round-1 issue: notice before JSON error envelope

The notice is no longer written directly to stderr at reader-construction time.

- `internal/runtime/runtime.go` writes the notice only into `Options.Notices`:
  ```go
  writeProfileNotice(opts.Notices, *cfg)
  ```
- The CLI passes a buffer:
  ```go
  Notices: &a.notices
  ```
  in the live-read paths:
  - `internal/cli/app.go` `defaultMachineRuntime`
  - `internal/cli/admin_audit_logs.go`
  - `internal/cli/dump_diff.go` live dump engine
  - `internal/cli/url_lookup.go`
- `App.Run` buffers notices and writes them only when the whole run succeeds:
  ```go
  func (a *App) Run(ctx context.Context, args []string) (err error) {
      ...
      a.notices.Reset()
      defer func() {
          if err == nil {
              a.writeNotices()
          }
      }()
      ...
  }
  ```
- `writeNotices` writes through a fresh redacted stderr writer:
  ```go
  w := redact.NewWriter(a.err, redact.ModeStandard)
  _, _ = w.Write(a.notices.Bytes())
  _ = w.Close()
  ```
- `cmd/zscalerctl/main.go` writes the error envelope only after `App.Run` returns an error. Because `Run` writes notices only when its final named `err` is nil, a failed live read does not put a notice before the envelope.

Traced return paths in `Run`:

- `__complete` / `__completeNoDesc` path returns `execCobra`. If it returns nil, notices are empty because completion bypasses config/live loading. If it returns an error, notices are suppressed.
- `parseGlobal` error returns before any live reader is built.
- `--output` with `dump` usage error returns before live reads.
- `--output` branch:
  ```go
  err := a.runParsed(ctx, opts, rest)
  ...
  if err != nil && !errors.Is(err, ErrDriftDetected) {
      return err
  }
  if writeErr := writeOutputFile(opts.output, buffered.Bytes()); writeErr != nil {
      return writeErr
  }
  return err
  ```
  If `runParsed` fails, `Run` returns non-nil and notices are suppressed. If `runParsed` succeeds but `--output` publication fails, `Run` returns `writeErr` and notices are suppressed. If publication succeeds and `err` is nil, notices are written. If `err` is `ErrDriftDetected`, `Run` returns non-nil and notices are suppressed, which is consistent with the failure-envelope contract.
- Non-`--output` path returns `runParsed` directly. Same defer behavior applies.

### Round-2 issue: notices flushed before post-Cobra failures

`executeRoot` no longer touches notices. It only closes stdout/stderr redactors and may convert a successful Cobra execution into an error if either redactor `Close` fails:

```go
defer func() {
    cerrOut := outW.Close()
    cerrErr := errW.Close()
    if err == nil {
        if cerrOut != nil {
            err = cerrOut
        } else if cerrErr != nil {
            err = cerrErr
        }
    }
}()
```

Because the final flush decision is in `App.Run`, any `executeRoot` `Close` error becomes a non-nil `Run` error and suppresses notices. The new CLI test adds the `--output` publication failure case, which directly addresses the round-2 class:

```go
{name: "output failure", fail: true, output: true},
```

That case would have exposed the round-2 bug: the read succeeds, notices are buffered, then `--output` publication fails, and stderr must remain empty from `Run`.

### Answer to focus (0)

For the shown normal CLI paths, there is no path where `App.Run` returns a non-nil error after notices have been written to stderr.

Caveat: if a panic occurs after notices are buffered and a caller recovers and converts that panic into an error, the `Run` defer can write notices because the named `err` is still nil when the panic unwinds. That is not a normal returned-error path, but it is worth noting as a non-blocking hardening risk.

## Non-Blocking Risks

### 1. CLI wiring tests cover only the product-read machine runtime path, not every live command

- File/path:
  - `internal/cli/admin_audit_logs.go`
  - `internal/cli/dump_diff.go`
  - `internal/cli/url_lookup.go`
  - `internal/cli/app.go`
  - `internal/cli/app_internal_test.go`
- Concrete issue:
  - The new CLI test proves the buffering contract through `machineRuntimeFactory`, which exercises the product-read path via `defaultMachineRuntime`/machine runtime.
  - It does not prove that `zia admin-audit-logs status`, `zia dump`, and `zia url-lookup` still pass `Notices: &a.notices` into their runtime options.
  - A future edit that removes one of those `Notices` fields would silently stop emitting the notice for that command while the existing test suite could still pass.
- Why it matters:
  - The whole point of the change is to make an implicit multi-profile read visible for live tenant reads. The reported downstream incident involves a live read against the wrong tenant. If one live command path drops the wiring, the protection disappears for that command.
- How to reproduce or verify:
  - Remove `Notices: &a.notices` from `internal/cli/admin_audit_logs.go`, `internal/cli/dump_diff.go`, or `internal/cli/url_lookup.go`, then run the shown CLI test. The current test should still pass because it does not exercise those command paths.
- Test that would catch it:
  - Add a table-driven CLI test with cases for:
    - product read, e.g. `zia locations list`
    - `zia url-lookup example.com`
    - `zia admin-audit-logs status`
    - `zia dump`
  - Each case should use a fake runtime/engine that writes a notice into the passed `Options.Notices`, then assert:
    - success: notice appears once on stderr and not stdout;
    - failure: stderr from `Run` is empty;
    - `--output` failure where applicable: stderr from `Run` is empty.

### 2. Some runtime public paths are not fully shown, so coverage cannot be fully proven from artifacts

- File/path:
  - `internal/runtime/runtime.go`
  - `internal/runtime/engine.go`
  - `internal/runtime/dump.go`
  - `internal/runtime/url_lookup.go`
  - `internal/runtime/admin_audit_logs.go`
- Concrete issue:
  - The shown artifacts prove that `newReaderFromConfig` writes the notice and that several CLI options pass `Notices`.
  - The body of `NewReaderFromConfig` is not fully shown, and `applyOptions` is not shown.
  - If `applyOptions` mutates `cfg.Profile` without updating `ProfileSource` or `ProfileCount`, the notice could be wrong or missing in paths where options override a loaded config.
- Why it matters:
  - The notice depends on `Config.ImplicitProfileAmongMany()`, which depends on `ProfileSource` and `ProfileCount`. If a later option mutation changes the selected profile without updating those fields, the notice can lie or disappear.
- How to reproduce or verify:
  - Add a test through `runtime.NewReaderFromConfig` or `runtime.NewMachineFromConfig` with a config whose `ProfileSource`/`ProfileCount` are set, plus `Options.Profile` or other options that `applyOptions` may apply.
  - Alternatively, show `applyOptions` and prove it does not change profile selection without updating selection metadata.
- Test that would catch it:
  - A runtime test that calls `NewReaderFromConfig` with:
    ```go
    config.Config{Profile: "prod", ProfileSource: config.ProfileFromDefaultProfile, ProfileCount: 2}
    Options{Notices: &notices, ...}
    ```
    and asserts the expected notice.
  - A separate test where `Options.Profile` is non-empty and asserts no implicit notice if the option represents an explicit profile selection, or asserts how option overrides map to `ProfileSource`.

### 3. Successful CLI stderr is intentionally no longer empty for implicit multi-profile reads

- File/path:
  - `docs/INSTALL.md`
  - `man/zscalerctl.1`
  - `skills/zscalerctl/SKILL.md`
  - `internal/runtime/runtime.go`
  - `internal/cli/app.go`
- Concrete issue:
  - Before this change, successful CLI commands were expected to have empty stderr. Now, when the config file defines multiple profiles and the profile came from the file default, a successful live read emits one non-JSON stderr line.
  - This is intentional and documented, and the machine stdio host leaves `Notices` nil via `OptionsFromExecutionSettings`.
- Why it matters:
  - Scripts that merge stderr into stdout, or that always parse stderr as JSON even when exit code is zero, can break.
  - The change is still defensible because stdout and error envelopes are unchanged, and the new docs explicitly describe the success notice.
- How to reproduce or verify:
  - Run `zscalerctl --format json zia locations list` with a two-profile config whose default profile is used, and pipe stderr into `jq`.
  - `jq` on stdout remains valid JSON, but `jq` on stderr or merged `2>&1` may fail because stderr contains a plain-text notice.
- Test that would catch it:
  - Add a CLI golden or contract test asserting that `--format json` success stdout remains parseable JSON while stderr contains the notice.
  - If the project requires machine consumers to have empty stderr, add a machine-safe mode or an opt-out such as `--quiet`/`--no-notices`, and test that mode.

### 4. `writeNotices` ignores stderr write and redactor close errors

- File/path:
  - `internal/cli/app.go`
- Concrete issue:
  ```go
  _, _ = w.Write(a.notices.Bytes())
  _ = w.Close()
  ```
  If stderr is closed, partially failed, or the redactor close fails, the notice can be dropped and `Run` still returns nil.
- Why it matters:
  - The notice is the safety signal for an implicit profile selection. Silently dropping it can recreate the original confusion, although it does not corrupt stdout or error envelopes.
- How to reproduce or verify:
  - Simulate a failing stderr writer or redactor close and assert that a successful live read either surfaces a failure or has an explicit documented best-effort notice contract.
- Test that would catch it:
  - A CLI test using a failing `io.Writer` for stderr and asserting either:
    - `Run` returns an error, or
    - the contract explicitly allows best-effort notice loss and documents it.

### 5. `App.notices` is a `bytes.Buffer`, which is not concurrency-safe

- File/path:
  - `internal/cli/app.go`
  - `internal/runtime/runtime.go`
- Concrete issue:
  - `App.notices` is a `bytes.Buffer`. If a runtime path ever writes notices concurrently, or if `App.Run` is ever called concurrently on the same `App`, this is a data race.
- Why it matters:
  - The current CLI appears single-command/single-run, so this is not currently blocking. But `Options.Notices` is an `io.Writer`, and future runtime concurrency could make the buffer unsafe.
- How to reproduce or verify:
  - Run the race detector with a fake runtime that writes notices from multiple goroutines, or call `App.Run` concurrently on the same `App`.
- Test that would catch it:
  - A concurrency test with `go test -race` that exercises multiple notice writes through the same runtime options.
  - Alternatively, replace the buffer with a mutex-protected writer if concurrency is allowed.

### 6. Panic recovery could still interleave notices with a recovered error envelope

- File/path:
  - `internal/cli/app.go`
- Concrete issue:
  - If a panic occurs after notices are buffered, the `Run` defer sees `err == nil` and writes notices before the panic continues unwinding. If a host recovers the panic and then writes an error envelope, stderr could contain the notice before the envelope.
- Why it matters:
  - This is not a normal return path, and the shown `cmd/zscalerctl/main.go` does not show panic recovery. But it is a machine-contract hardening issue if the process ever recovers panics.
- How to reproduce or verify:
  - Add a test where a command panics after writing a notice, a host recovers, and the host writes an error envelope.
- Test that would catch it:
  - A panic/recovery CLI test asserting that recovered failures still have only the envelope on stderr, or that notices are discarded on panic.

### 7. Generated agent-skill copy is excluded from the shown diff

- File/path:
  - `skills/zscalerctl/SKILL.md`
  - generated `.agents` skill copy, excluded from the shown commit diff
- Concrete issue:
  - The source skill doc is updated, but the generated copy is not shown.
- Why it matters:
  - If docs-check enforces generated-copy synchronization, the commit must include the regenerated copy. The builder reported `make docs-check`, but that is not proof.
- How to reproduce or verify:
  - Confirm the generated `.agents` copy is present in the actual commit and matches `skills/zscalerctl/SKILL.md` according to the docs-check rule.
- Test that would catch it:
  - `make docs-check` or an equivalent generated-artifact sync test in CI.

## Machine Contract Review

### Stdout

Stdout is unchanged.

The CLI test asserts stdout does not contain the notice:

```go
if strings.Contains(out.String(), "using profile") {
    t.Errorf("%s: stdout = %q, want no notice", tc.name, out.String())
}
```

The notice is written only to `a.err` through `writeNotices`.

### Stderr on failure

For normal CLI failures, stderr from `App.Run` is empty because notices are written only when `Run`'s final error is nil.

The `cmd/zscalerctl/main.go` error writer then emits the single JSON envelope for machine formats:

```go
envelope := errorEnvelope{Error: errorDetails(err)}
_ = output.NewRenderer(redact.New(redact.ModeStandard)).WriteJSON(w, envelope)
```

The new CLI test pins that failure stderr from `Run` is empty:

```go
{name: "failure", fail: true},
{name: "output failure", fail: true, output: true},
```

### Stderr on success

Success stderr is intentionally changed for implicit multi-profile live CLI reads.

- The notice is one line.
- It is redacted through `redact.NewWriter(a.err, redact.ModeStandard)`.
- It is not part of stdout or the JSON error envelope.
- The machine stdio host remains unaffected because `OptionsFromExecutionSettings` leaves `Notices` nil:
  ```go
  func OptionsFromExecutionSettings(settings machine.ExecutionSettings) Options {
      return Options{
          Profile:      settings.Profile,
          ConfigPath:   settings.ConfigPath,
          Timeout:      settings.Timeout,
          Redaction:    settings.Redaction,
          RedactionSet: settings.RedactionSet,
          NoCache:      settings.NoCache,
      }
  }
  ```

This is a contract delta, but it is documented in:

- `docs/INSTALL.md`
- `man/zscalerctl.1`
- `skills/zscalerctl/SKILL.md`

### Exit codes

Exit-code behavior is unchanged by the shown diff.

- Notices are written only when `Run` returns nil.
- Failures still return errors to `main.go`, which maps them to the existing exit-code behavior.
- The `--output` publication failure path now returns a non-nil `Run` error and therefore preserves the failure envelope and failure exit behavior.

### JSON/NDJSON

stdout JSON/NDJSON is not changed.

The notice is not written into a JSON envelope. It is written to stderr only after the whole run succeeds. For successful machine-style CLI output, stdout remains machine data and stderr contains only the new human/operator notice.

For failed machine-style CLI output, stderr remains the single JSON envelope because `Run` suppresses notices on failure.

### Resource catalog / live reader coverage

Shown live CLI paths pass `Notices`:

- `internal/cli/app.go` `defaultMachineRuntime`
- `internal/cli/admin_audit_logs.go`
- `internal/cli/dump_diff.go` live dump engine
- `internal/cli/url_lookup.go`

Shown non-live paths do not pass `Notices`:

- `internal/cli/status.go` uses `machineruntime.Options{Timeout: opts.timeout}` without `Notices`.
- `internal/cli/dump_diff.go` diff-from-dump engine uses only `Catalog` and no `Notices`.
- `cmd/zscalerctl-engine` machine stdio host uses `OptionsFromExecutionSettings`, which leaves `Notices` nil.

The runtime writes the notice only inside `newReaderFromConfig`:

```go
writeProfileNotice(opts.Notices, *cfg)
```

So from-dump diff and machine stdio host do not emit the notice.

I cannot prove from the shown artifacts that every possible live path in the repository goes through `newReaderFromConfig`, but the shown live CLI paths do, and the shown non-live paths do not.

## Safety Review

### Redaction

The notice is written through a redacted stderr writer:

```go
w := redact.NewWriter(a.err, redact.ModeStandard)
```

This is consistent with the CLI’s stderr redaction approach.

The runtime test also verifies that a nil `Notices` writer does not panic and writes nothing:

```go
_, _ = newReaderFromConfig(context.Background(), &cfg, Options{
    newReader: func(zscaler.ReaderConfig) (browser.RecordReader, error) { return nil, nil },
})
```

### Profile-name disclosure

The notice prints only the profile name and profile count:

```go
fmt.Fprintf(
    w,
    "zscalerctl: using profile %q (%s; %d profiles configured); pass --profile to choose\n",
    cfg.Profile,
    how,
    cfg.ProfileCount,
)
```

It does not print:

- vanity domain,
- client secret,
- tenant IDs,
- credentials,
- environment variable values.

Using `%q` escapes quotes and control characters. For example, a profile name containing a newline is rendered as an escaped string rather than a raw newline.

This is consistent with the stated existing safe-view behavior: profile names may be shown, but vanity domains and IDs are not.

### Profile selection correctness

The selection logic is traced through `internal/config/load.go` and `internal/config/profile.go`.

`LoadConfig` records:

- `ProfileFromFlag` when `opts.Profile` is non-empty.
- `ProfileFromEnv` when `ZSCALERCTL_PROFILE` is non-empty and no flag profile is present.
- `ProfileFromDefaultProfile` when no flag/env profile is present and the config file has `default_profile`.
- `ProfileFromBuiltinDefault` when no flag/env profile is present and the file has no `default_profile` but has a profile named `default`.

`loadProfileFile` records:

- `count: len(model.Profiles)`
- `defaultSource: ProfileFromDefaultProfile` or `ProfileFromBuiltinDefault`

`ImplicitProfileAmongMany` emits only when:

```go
implicit := c.ProfileSource == ProfileFromDefaultProfile || c.ProfileSource == ProfileFromBuiltinDefault
return implicit && c.ProfileCount > 1
```

The config tests cover:

- explicit flag;
- explicit env;
- `default_profile`;
- built-in `default`;
- single profile.

The runtime notice tests cover:

- `default_profile` among multiple profiles;
- built-in default among multiple profiles;
- flag among multiple profiles, no notice;
- env among multiple profiles, no notice;
- single profile, no notice;
- env-only config, no notice;
- nil `Notices`, no panic.

The shown logic appears correct for the cases in the artifacts. The remaining uncertainty is `applyOptions`, which is not shown.

## Generated Artifact Review

- `docs/INSTALL.md`, `man/zscalerctl.1`, and `skills/zscalerctl/SKILL.md` are updated consistently with the behavior.
- The commit message says the generated `.agents` skill copy is excluded from the shown diff.
- I cannot verify from the shown artifacts that the generated copy is present and synchronized.
- Builder-reported `make docs-check` is not proof for this review. This should be confirmed in the actual commit or CI output.

## Coverage Summary Against Focus Areas

### (0) Prior blocking findings

Fixed for normal CLI return paths. No shown path where `App.Run` returns non-nil after notices have been written to stderr.

### (a) Coverage

Shown live CLI paths pass `Notices`. Shown non-live paths do not. The main gap is test coverage, not a proven code gap.

### (b) Machine contracts

- stdout unchanged.
- failure stderr remains a single envelope.
- success stderr intentionally gains one redacted operator notice for implicit multi-profile live CLI reads.
- machine stdio host leaves `Notices` nil and is unaffected.
- no notice inside JSON envelopes.
- no notice on stdout.

### (c) Disclosure

Profile name and count are printed. This is consistent with existing safe views and is redacted/escaped. No vanity domain or IDs are printed.

### (d) Selection correctness

Flag/env/default/builtin-default behavior is traced and tested. Edge cases are handled by `%q`. The unshown `applyOptions` behavior is the only meaningful uncertainty.

### (e) Test strength

The new tests are strong for the core buffering contract and runtime notice selection, but they do not fully pin every live CLI command wiring. The most important remaining test gap is command-path coverage for `admin-audit-logs status`, `dump`, and `url-lookup`.

## Verdict

Verdict: approve with nits
