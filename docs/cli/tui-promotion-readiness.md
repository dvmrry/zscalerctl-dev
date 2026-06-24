# TUI Promotion Readiness Audit

Release-candidate audit for the `feature/tui` integration branch. This document
records the architecture, command surface, dependency changes, validation
evidence, and remaining risks before `feature/tui` can be promoted to `main`.

> **Historical audit:** this readback predates the Charm v2 spike. Current TUI
dependency and import-boundary policy is maintained in
`docs/DEPENDENCY_POLICY.md`, `docs/cli/tui-import-boundary.md`, and
`docs/cli/tui-separate-binary.md`.

> **Post-blocker-fix update:** the hidden `zscalerctl browse --tui` command
described in this audit has been removed because it transitively linked Bubble
Tea into the normal `zscalerctl` binary via `internal/cli -> internal/tui/launcher
-> internal/tui/tea`. Bubble Tea v1.x package-init probing can emit OSC/DSR
sequences before `main()`, corrupting interactive JSON output. The gate/collector
path remains, but the TUI runtime is now restricted to isolated entrypoints such
as `scripts/tui-browser-demo.go` or a future `cmd/zscalerctl-tui`. See
`docs/cli/tui-import-boundary.md` for the updated boundary.

**Scope:** docs and verification only. No new features or behavior changes were
introduced during this audit.

**Branch:** `feature/tui-rc-audit` -> `feature/tui`

---

## 1. Architecture invariants

The complete TUI command path is:

```
zscalerctl browse --tui
  -> gate           (launcher.CheckGate)
  -> config         (config.LoadConfig)
  -> credentials     (resourceReader construction)
  -> reader          (ResourceReader / RecordReader)
  -> collector      (internal/tui/browserdata.Collector)
  -> BrowserData     (internal/tui/tea.BrowserData)
  -> launcher        (internal/tui/launcher.LaunchBrowser)
  -> Bubble Tea      (internal/tui/tea)
```

Protections that remain intact:

- The gate runs **before** any config, credential, or reader work.
- Missing credentials stop the command **before** Bubble Tea is constructed.
- Invalid config stops the command **before** Bubble Tea is constructed.
- Collector fail-fast (`--continue-on-error=false`, the default) stops the command
  before Bubble Tea is constructed.
- `--continue-on-error=true` converts resource errors into `BrowserData` error
  states and still reaches the launch step.
- Bubble Tea imports are isolated under `internal/tui/tea`. No normal CLI
  startup package (`cmd/`, `internal/cli/`, `internal/tui`) imports Bubble Tea.
- No Fang integration was added.
- No Cobra execution wrapper was added.
- No live API credentials were required to validate this branch.

---

## 2. Dependency / security posture

`go.mod` changes (relative to `main`):

| Dependency | Change | Note |
| --- | --- | --- |
| `github.com/charmbracelet/bubbletea` | added `v1.3.10` | Direct TUI runtime dependency. Confined to `internal/tui/tea` and the demo scripts. |
| `github.com/charmbracelet/x/ansi` | `v0.8.0` -> `v0.10.1` | Transitive upgrade pulled by Bubble Tea v1.3.10. Also used by existing `lipgloss` path. |
| `github.com/erikgeiser/coninput` | added | Bubble Tea input handling (Windows). |
| `github.com/mattn/go-localereader` | added | Bubble Tea input reader. |
| `github.com/muesli/ansi` | added | Bubble Tea / Lip Gloss ANSI utilities. |
| `github.com/muesli/cancelreader` | added | Bubble Tea cancellation reader. |
| `golang.org/x/text` | added | Bubble Tea width/transform utilities. |

Vendor footprint:

- Total delta: ~14,444 insertions, ~1,435 deletions across 132 files.
- The overwhelming majority is the Bubble Tea dependency tree and the
  `x/ansi` upgrade.
- No new dependencies are referenced from normal CLI startup paths.

Security notes:

- Bubble Tea v1.3.10 is the current stable release.
- `govulncheck ./...` reports no vulnerabilities.
- `gitleaks` reports no leaks.
- The TUI dependencies are only reachable from `internal/tui/tea` and the demo
  scripts (`scripts/tui-demo.go`, `scripts/tui-browser-demo.go`).

---

## 3. Import-boundary proof

`scripts/verify-tui-import-boundary.sh` passes. It checks that
`github.com/charmbracelet/bubbletea` is not directly imported by:

- `cmd/`
- `internal/cli/`
- `internal/tui/` (excluding `internal/tui/tea`)

Result:

```sh
bash scripts/verify-tui-import-boundary.sh
# exit 0
```

The only direct Bubble Tea imports are in:

- `internal/tui/tea/*.go`
- `scripts/tui-demo.go`
- `scripts/tui-browser-demo.go`

---

## 4. Command-surface changes

`feature/tui` adds one hidden top-level command: `browse`.

Introspect golden changes (recorded in `surface_changes.md`):

- `introspect` and `introspect-pretty` now include the hidden `browse` command.
- `browse` local flags: `--tui`, `--products`, `--resources`, `--continue-on-error`.
- No existing command output changed.
- No non-hidden commands were added.
- `TestCommandTreeInventory` (which skips hidden commands) is unchanged because
  `browse` is `Hidden: true`.

Introspect snippet:

```text
browse                                    experimental TUI browser [hidden]
    --continue-on-error (bool, default "false")
    --products (string)
    --resources (string)
    --tui (bool, default "false")
```

---

## 5. Hidden `browse` command behavior

`browse` is hidden (`cobra.Command.Hidden = true`) and does nothing unless
`--tui` is provided. It is not advertised in normal help, but it is exposed in
`introspect` because the existing introspect schema already exposes hidden
commands with a `hidden: true` marker. This is intentional and documented.

Command help is still accessible via `zscalerctl browse --help` for developers.

---

## 6. Machine output unaffected

Normal CLI paths were verified to contain no `ESC` (0x1B) bytes and no TUI
sequences (cursor hide/show, bracketed paste, mouse, OSC, DSR).

```sh
/tmp/zscalerctl-tui version --format json
# ESC bytes: 0

/tmp/zscalerctl-tui version --format pretty --color never
# ESC bytes: 0

/tmp/zscalerctl-tui introspect --format json
# ESC bytes: 0

/tmp/zscalerctl-tui --format json browse --tui 2>&1
# ESC bytes: 0
```

The `browse --tui` non-TTY rejection returns a JSON envelope on stderr and no
stdout output, matching the machine-first contract.

---

## 7. TUI rejection matrix

All rejection cases were verified. The gate runs before config/reader work;
credential/config errors occur after the gate but before Bubble Tea.

| Invocation | Expected result | Verified |
| --- | --- | --- |
| `browse --tui --format json` (PTY) | usage error: `machine output format requested` | yes |
| `browse --tui --format ndjson` (PTY) | usage error: `machine output format requested` | yes |
| `browse --tui --output out.txt` (PTY) | usage error: `output path is not supported for TUI` | yes |
| `browse --tui --color never` (PTY) | usage error: `terminal styling disabled` | yes |
| `browse --tui` (non-TTY pipe) | usage error: `stdin is not interactive` | yes |
| `browse --tui` with bad config (PTY) | config error: `invalid configuration: config file not found` | yes |
| `browse --tui` with missing creds (PTY) | credential error: `missing zscaler API credentials: ...` | yes |

No rejection path produced TUI escape sequences; all exited before Bubble Tea
started.

---

## 8. Missing credentials / bad config behavior

Verified with a PTY so the TTY gate passes and the real config/credential path is
exercised.

Bad config (missing file):

```text
zscalerctl: invalid configuration: config file not found
```

Missing credentials (no env, no config):

```text
zscalerctl: missing zscaler API credentials: ZSCALERCTL_CLIENT_ID,
ZSCALERCTL_CLIENT_SECRET, ZSCALERCTL_VANITY_DOMAIN required
```

Both errors return before `internal/tui/launcher.LaunchBrowser` constructs the
Bubble Tea program. The `browse --tui` command returns the normal CLI error
envelope, not a TUI-screen error.

---

## 9. PTY readback evidence

### Fixture demo (`scripts/tui-browser-demo.go --collector-fixture`)

The fixture demo exercises the collector path with a fake reader and requires
no credentials. It is the primary success-path evidence available on this
machine.

#### 80x24 — initial frame

```text
┌────────────────────────┐┌────────────────────────────────────────────────────┐
│ Products / Resources   ││ zia                                                │
│                        ││                                                    │
│ zia                    ││ Product: zia                                       │
│   locations            ││ Resources: 3                                       │
│   url-filtering-rules  ││                                                    │
│   forwarding-rules     ││                                                    │
│ zpa                    ││                                                    │
│   application-segments ││                                                    │
│   app-connectors       ││                                                    │
│ zcc                    ││                                                    │
│   devices              ││                                                    │
│                        ││                                                    │
│                        ││                                                    │
│                        ││                                                    │
│                        ││                                                    │
│                        ││                                                    │
│                        ││                                                    │
│                        ││                                                    │
│                        ││                                                    │
└────────────────────────┘└────────────────────────────────────────────────────┘
zia · 1/9
↑/↓ move · tab switch · enter select · ? help · esc/q quit
```

#### 60x16 — initial frame

```text
│ zpa                                                      │
│   application-segments                                   │
│   app-connectors                                         │
│ zcc                                                      │
│   devices                                                │
└──────────────────────────────────────────────────────────┘
┌──────────────────────────────────────────────────────────┐
│ zia                                                      │
│                                                          │
│ Product: zia                                             │
│ Resources: 3                                             │
│                                                          │
└──────────────────────────────────────────────────────────┘
zia · 1/9
↑/↓ move · tab switch · enter select · ? help · esc/q quit
```

The TUI exits cleanly on `q`, `esc`, and `ctrl+c`.

---

## 10. Mainline diff classification

`git diff --stat main...feature/tui`:

| Category | Files / size | Assessment |
| --- | --- | --- |
| Expected dependency/vendor delta | Bubble Tea + transitive deps in `vendor/`, `go.mod`, `go.sum` | Expected. Confined to TUI runtime. |
| Expected TUI package delta | `internal/tui/`, `internal/tui/browserdata/`, `internal/tui/launcher/`, `internal/tui/tea/` | Expected. New TUI subsystem. |
| Expected hidden browse command delta | `internal/cli/app.go`, `internal/cli/cobra_browse.go`, `internal/cli/cobra_browse_test.go`, `cmd/zscalerctl/testdata/surface/*` | Expected. Hidden command surface. |
| Expected docs/scripts delta | `docs/cli/tui-*.md`, `docs/DEPENDENCY_POLICY.md`, `docs/README.md`, `docs/SCRIPTS.md`, `scripts/tui-*.go`, `scripts/verify-tui-import-boundary.sh`, `scripts/test-verify-tui-import-boundary.sh` | Expected. TUI documentation and demo tooling. |
| Makefile delta | `Makefile` | Expected. Added import-boundary verifier to `check`. |
| Unexpected baseline movement | None found | No unrelated changes detected. |

No unexpected changes were observed in normal CLI code paths.

---

## 11. Validation evidence

All validation commands pass on `feature/tui`:

```sh
bash scripts/verify-tui-import-boundary.sh
# ok

go test -mod=vendor ./cmd/zscalerctl/... -run TestGoldenSurface
# ok

go test -mod=vendor ./internal/cli -run TestBrowse
# ok

go test -mod=vendor ./internal/tui/...
# ok

make check
# ok
```

Specific test coverage includes:

- Gate rejects `--format json/ndjson` before config/reader work.
- Gate rejects `--output` before config/reader work.
- Non-TTY `browse --tui` is rejected at the gate.
- Missing credentials prevent Bubble Tea launch.
- Bad config prevents Bubble Tea launch.
- Fake reader success collects `BrowserData` and reaches the launch hook.
- Fake reader fail-fast returns an error before launch.
- Fake reader continue-on-error produces `BrowserData` error states.

---

## 12. Remaining blockers before `feature/tui -> main`

1. **Live credential smoke test.**
   The success path is currently proven with fake readers and the fixture demo.
   A single `zscalerctl browse --tui` run against a scratch tenant with real
   credentials would confirm that the real reader/collector/BrowserData path
   works outside the test harness. This is **not** required on this machine, but
   it is a reasonable prerequisite before exposing the feature to mainline users.

2. **Adversarial / multi-model review.**
   The branch is now substantial enough to warrant an external review focused on:
   - import-boundary violations
   - Bubble Tea startup side effects on non-TUI invocations
   - hidden command / introspect exposure
   - missing-credential behavior
   - dependency/vendor risk
   - whether a hidden experimental command should land in `main` without live smoke

3. **Decision on promotion timing.**
   The command is hidden, so mainline users will not discover it accidentally.
   However, the project should decide whether hidden-but-introspectable is
   acceptable before a live smoke test, or whether the command should remain on
   `feature/tui` until a live tenant readback is captured.

4. **No known code blockers.**
   The architecture is complete, the gate ordering is correct, the import
   boundary is enforced, and all automated validation passes.

---

## 13. Recommendation

**Recommendation: keep incubating until an isolated TUI entrypoint is designed
and approved.**

The `feature/tui` branch is architecturally sound and is now promotable to
`main` as a **foundation-only** merge. The blocker fix removed the hidden
`browse --tui` command, and an isolated experimental TUI binary `cmd/zscalerctl-tui`
has been added. The normal `zscalerctl` binary (`cmd/zscalerctl`) and `internal/cli`
remain Bubble Tea-free, while the TUI runtime (Bubble Tea and `internal/tui/tea`)
lives only inside the separate binary and the development scripts. This keeps the
normal CLI safe from Bubble Tea v1.x package-init probing while preserving the
reusable TUI components for future interactive work.

This promotion does **not** ship a user-facing TUI command or live-reader path. It
ships:

- Bubble-free TUI gate/data/browserdata/launcher packages.
- Isolated Bubble Tea model (`internal/tui/tea`).
- Isolated experimental binary `cmd/zscalerctl-tui` with fixture-only modes.
- Transitive import-boundary verifier and PTY escape-clean regression verifier.
- Documentation and readback evidence.

Before a user-facing TUI feature is declared ready:

- Capture at least one live-reader-backed readback from `cmd/zscalerctl-tui` on a
  scratch tenant with no secrets in output.
- Add config, credential, and live reader support to `cmd/zscalerctl-tui`.
- Decide whether the main `zscalerctl` binary should ever launch the separate
  TUI binary (e.g. via `exec`).
- Update the promotion audit to reflect the final live-reader design.

This audit no longer discovers a technical blocker: the TUI runtime is
isolated from the normal `zscalerctl` binary.

---

## 14. Open questions before promotion

- How will `cmd/zscalerctl-tui` load config and credentials without pulling
  Bubble Tea into the normal CLI binary? (It is already a separate binary, so
  it can import the normal config/credential packages directly.)
- Should the main `zscalerctl` binary ever exec `cmd/zscalerctl-tui`, or should
  the TUI binary always be invoked directly?
- Is the transitive `x/ansi` version bump acceptable for the existing `lipgloss`
  usage, or should it be pinned separately?
