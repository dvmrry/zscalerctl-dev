# Experimental standalone TUI binary: `cmd/zscalerctl-tui`

`zscalerctl-tui` is an experimental, separate binary for the interactive TUI
browser. It lives in `cmd/zscalerctl-tui/` and is allowed to import
`github.com/charmbracelet/bubbletea` and `internal/tui/tea`. The normal
`cmd/zscalerctl` binary and `internal/cli` package remain Bubble Tea-free.

## Why a separate binary?

Bubble Tea v1.x runs package-initialization terminal probing (via Lip Gloss
background detection) that can emit OSC/DSR sequences before `main()`. If Bubble
Tea were linked into the normal `zscalerctl` binary, every invocation — even
`zscalerctl version --format json` on a non-TTY — could be affected. Keeping
the TUI runtime in a separate binary preserves the safety of the main CLI.

## Boundary

| Package / binary | May import Bubble Tea |
| --- | --- |
| `cmd/zscalerctl` | No |
| `internal/cli` | No |
| `internal/tui` | No |
| `internal/tui/data` | No |
| `internal/tui/browserdata` | No |
| `internal/tui/launcher` | No |
| `internal/tui/tea` | Yes |
| `cmd/zscalerctl-tui` | Yes |
| `scripts/tui-demo.go` | Yes |
| `scripts/tui-browser-demo.go` | Yes |

The boundary is enforced by `scripts/verify-tui-import-boundary.sh`, which runs
`go list -deps` on `./cmd/zscalerctl`, `./internal/cli`, `./internal/tui`,
`./internal/tui/data`, `./internal/tui/browserdata`, and `./internal/tui/launcher`.

## Vendor patch

`cmd/zscalerctl-tui` intentionally imports `github.com/charmbracelet/bubbletea`.
Bubble Tea v1.x runs `lipgloss.HasDarkBackground()` in package `init()`, which
emits OSC/DSR terminal probes before `main()` and can hang failure paths such as
`zscalerctl-tui --live --profile <invalid>`. The vendored
`vendor/github.com/charmbracelet/bubbletea/tea_init.go` is patched to remove
that call; the patched `init()` does nothing.

This is acceptable because:

- `cmd/zscalerctl` (the normal binary) still never imports Bubble Tea, so the
  patch has no effect on normal CLI output.
- `zscalerctl-tui` does not rely on Bubble Tea's startup background-color
  detection; color is decided by the existing `output.ShouldColor` gate after
  `main()` runs.
- The patch is guarded by `scripts/verify-bubbletea-vendor-patch.sh`, which
  fails if `go mod vendor` reintroduces the probe.
- A PTY regression verifier, `scripts/verify-zscalerctl-tui-live-failure.sh`,
  proves the patched failure path returns promptly with zero `ESC` bytes and a
  config/profile error.

## Modes

### Fixture modes (no credentials)

```sh
# Run the fake-reader-backed collector fixture (default). The fixture intentionally
# omits some resources, so the collector runs with --continue-on-error behavior.
go run ./cmd/zscalerctl-tui --collector-fixture

# Use the hard-coded static fixture.
go run ./cmd/zscalerctl-tui --fixture
```

### Live mode

```sh
# Load config, resolve credentials, build a real reader, and collect live tenant data.
go run ./cmd/zscalerctl-tui --live

# Filter to specific products and resources.
go run ./cmd/zscalerctl-tui --live --products zia --resources locations,url-filtering-rules

# Continue browsing resources that succeed even if some fail.
go run ./cmd/zscalerctl-tui --live --continue-on-error

# Use a specific profile or config file.
go run ./cmd/zscalerctl-tui --live --profile prod --config /path/to/config.yaml
```

Live mode requires Zscaler credentials for the selected auth mode only:

- OneAPI (default): resolves `client_secret`.
- ZIA legacy (`auth_mode: zia-legacy`): resolves `zia_password` and `zia_api_key`.

Unused credentials are not resolved, so a OneAPI profile does not need legacy
ZIA secrets and a legacy ZIA profile does not need a client secret.

The credential discovery order is the same as the normal `zscalerctl` CLI:
`ZSCALERCTL_*` environment variables take precedence, then a profile from the
selected config file. See `docs/INSTALL.md` for details.

## Flags

| Flag | Default | Description |
| --- | --- | --- |
| `--live` | `false` | Load config, resolve credentials, and collect live tenant data. |
| `--collector-fixture` | `true` (default when no mode is selected) | Use the fake-reader-backed collector fixture. |
| `--fixture` | `false` | Use the hard-coded static fixture. |
| `--products` | `""` | Comma-separated list of products to include. |
| `--resources` | `""` | Comma-separated list of resources to include. |
| `--continue-on-error` | `false` | Continue collecting after a resource error. In fixture mode this is always true. |
| `--profile` | `""` | Config profile name (live mode only). |
| `--config` | `""` | Config file path (live mode only). |
| `--color` | `auto` | Color mode: `auto`, `always`, `never`. |
| `--format` | `auto` | Output format gate (used by the TUI eligibility check). |

`--live`, `--fixture`, and `--collector-fixture` are mutually exclusive.

## Gate behavior

The binary uses the same `internal/tui` gate as the removed `browse --tui`
path. It refuses to start unless stdin, stdout, and stderr are all TTYs, the
format is not `json`/`ndjson`, no `--output` path is set, and styling is not
disabled by `--color never`, `NO_COLOR`, or `TERM=dumb`.

## Failure ordering

Live mode is designed to fail before Bubble Tea starts:

1. TUI gate check.
2. Config load.
3. Credential resolution (`client_secret`, ZIA legacy password/API key).
4. Reader creation (Zscaler SDK client).
5. Collector execution (unless `--continue-on-error` is set, in which case
   per-resource errors become error nodes inside the BrowserData and the TUI
   still launches).

## Live smoke checklist

Run these against a scratch tenant before declaring live mode ready:

- [ ] `go run ./cmd/zscalerctl-tui --live` launches and shows products/resources.
- [ ] `go run ./cmd/zscalerctl-tui --live --products zia` filters to ZIA only.
- [ ] `go run ./cmd/zscalerctl-tui --live --products zia --resources locations` collects only the `locations` resource.
- [ ] `go run ./cmd/zscalerctl-tui --live --continue-on-error` launches when one resource is entitlement-gated or unsupported.
- [ ] Missing credentials exit with an error before the TUI opens.
- [ ] Invalid config exits with an error before the TUI opens.
- [ ] Non-TTY invocation exits with a gate error before any config or credential work.
- [ ] `--format json` exits with a gate error before any config or credential work.
- [ ] No secrets appear in the TUI rendering or in any logged output.
- [ ] `scripts/verify-pty-escape-clean.sh` still passes (normal `zscalerctl` JSON output is clean).
- [ ] `go list -deps ./cmd/zscalerctl | grep -E 'bubbletea|internal/tui/tea'` still produces no matches.

## Future work

- Add a `--timeout` flag for live collection.
- Add `--log-level` to surface SDK diagnostics when debugging live reads.
- Decide whether the main `zscalerctl` binary should ever `exec` this separate
  binary.
