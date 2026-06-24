# Live TUI smoke checklist

This checklist is for manual validation of `cmd/zscalerctl-tui --live` against a
scratch Zscaler tenant. It is not required to complete the PR that wires live
config/credential/reader support into the standalone binary; it documents the
steps a human operator should run before declaring live mode ready.

## Prerequisites

- A scratch Zscaler tenant with no production data.
- Credentials set via `ZSCALERCTL_*` environment variables or a config profile.
- A terminal that is at least 80x24.
- A working `zscalerctl-tui` binary built from the branch under test:

```sh
go build -mod=vendor -o zscalerctl-tui ./cmd/zscalerctl-tui
```

## Checks

### Fixture baseline

Verify the fixture-only modes still work so the standalone binary is not broken.

```sh
./zscalerctl-tui --fixture
./zscalerctl-tui --collector-fixture
```

Expected: both launch the TUI and exit cleanly with `q`/`esc`/`ctrl+c`.

### Live gate

```sh
./zscalerctl-tui --live --verbose
```

Expected: the TUI launches and shows at least one product/resource tree. The
verbose output should print milestones through "launching TUI" with no
secret values.

### Live filtering

```sh
./zscalerctl-tui --live --products zia
./zscalerctl-tui --live --products zia --resources locations
./zscalerctl-tui --live --products zia,zpa --resources locations,application-segments
```

Expected: only the selected products and resources appear in the left pane.

### Live continue-on-error

```sh
./zscalerctl-tui --live --continue-on-error
```

Expected: the TUI launches even if some resources fail (e.g. entitlement-gated
or unsupported). Failing resources should show an error node instead of records.

### Failure paths

For each of the following, confirm the program exits with a non-zero status and
**does not open a TUI**:

```sh
# Non-TTY (pipe stdout)
./zscalerctl-tui --live | cat

# Machine output format
./zscalerctl-tui --live --format json

# Disabled color
./zscalerctl-tui --live --color never

# Missing credentials (unset ZSCALERCTL_*)
env -u ZSCALERCTL_CLIENT_SECRET -u ZSCALERCTL_CLIENT_ID ./zscalerctl-tui --live

# Collection timeout
./zscalerctl-tui --live --timeout 1s --profile prod
```

### Boundary safety

After live-smoke runs, confirm that the normal `zscalerctl` binary still has no
Bubble Tea in its dependency graph:

```sh
go list -deps ./cmd/zscalerctl | grep -E 'github.com/charmbracelet/bubbletea|internal/tui/tea'
# should produce no output
```

And confirm normal JSON output is still ESC-clean in a PTY:

```sh
bash scripts/verify-pty-escape-clean.sh
```

### Secret safety

While running live mode, verify:

- No raw client secret, password, API key, or tenant identifiers appear in the TUI
  rendering.
- No credentials are written to the terminal scrollback or to any log file.
- The scratch tenant has no production data, so an accidental screenshot is safe.

## Sign-off

- [ ] Fixture modes still work.
- [ ] Live mode launches on a scratch tenant.
- [ ] Product/resource filters work.
- [ ] Continue-on-error launches with error nodes.
- [ ] Non-TTY, machine format, missing-credential, and timeout failures exit before the TUI.
- [ ] `--verbose` live output shows milestones without secret values.
- [ ] Normal `zscalerctl` remains Bubble Tea-free.
- [ ] Normal JSON output remains ESC-clean in a PTY.
- [ ] No secrets appear in the TUI output.
