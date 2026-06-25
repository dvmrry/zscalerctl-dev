# Experimental standalone TUI binary: `cmd/zscalerctl-tui`

`zscalerctl-tui` is an experimental, separate binary for the interactive TUI
browser. It lives in `cmd/zscalerctl-tui/` and is allowed to import
`charm.land/bubbletea/v2`, `charm.land/bubbles/v2`, and `internal/tui/tea`.
The normal `cmd/zscalerctl` binary and `internal/cli` package remain Bubble
Tea-free.

## Why a separate binary?

Keeping the TUI runtime in a separate binary preserves the safety of the main
CLI. Normal `zscalerctl` JSON/NDJSON, completion, introspection, and machine
error paths must not depend on Bubble Tea or Bubbles, even transitively.

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

## Startup-probe guard

`cmd/zscalerctl-tui` intentionally imports `charm.land/bubbletea/v2`. Bubble Tea
v2 does not vendor the old v1 `tea_init.go` background-detection probe, so there
is no local patch to restore after `go mod vendor`.

This is acceptable because:

- `cmd/zscalerctl` (the normal binary) still never imports Bubble Tea, so the
  standalone TUI stack has no effect on normal CLI output.
- `zscalerctl-tui` does not rely on Bubble Tea's startup background-color
  detection; color is decided by the existing `output.ShouldColor` gate after
  `main()` runs.
- `scripts/verify-bubbletea-vendor-patch.sh` fails if the vendored Bubble Tea
  v2 tree gains package `init()` functions or `HasDarkBackground` references.
- A PTY regression verifier, `scripts/verify-zscalerctl-tui-live-failure.sh`,
  proves the failure path returns promptly with zero `ESC` bytes and a
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
# Load config, resolve credentials, build a real reader, and open an unloaded catalog.
# Records load lazily when a resource is selected in the TUI.
go run ./cmd/zscalerctl-tui --live

# Filter to specific products and resources. The visible catalog is narrowed,
# but records are still loaded lazily.
go run ./cmd/zscalerctl-tui --live --products zia --resources locations,url-filtering-rules

# Keep the live lazy error handling path enabled. Resource failures render as
# per-resource error states after selection instead of blocking first paint.
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

## Live diagnostics and timeout

Live mode builds config, resolves credentials, constructs a reader, and opens
the TUI with product/resource catalog entries in an unloaded state. It does not
collect all resource records before first paint. Use `--verbose` to print
pre-launch milestones to stderr:

```sh
./zscalerctl-tui --live --verbose --profile prod
```

Milestones are intentionally secret-safe: they report the auth mode and selected
catalog scope, but never emit the client secret, password, API key, tenant URL,
or other credentials.

Use `--timeout` to cap each selected live resource load. The default is `30s`:

```sh
./zscalerctl-tui --live --timeout 10s --profile prod
```

If the timeout fires after the TUI is open, the selected resource becomes an
error state with a sanitized `context deadline exceeded` message. Config,
credential, and reader setup failures still exit before Bubble Tea starts.

## Flags

| Flag | Default | Description |
| --- | --- | --- |
| `--live` | `false` | Load config, resolve credentials, build a reader, and lazily browse live tenant data. |
| `--collector-fixture` | `true` (default when no mode is selected) | Use the fake-reader-backed collector fixture. |
| `--fixture` | `false` | Use the hard-coded static fixture. |
| `--products` | `""` | Comma-separated list of products to include. |
| `--resources` | `""` | Comma-separated list of resources to include. |
| `--continue-on-error` | `false` | Keep per-resource live errors as TUI error states. In fixture mode this is always true. |
| `--profile` | `""` | Config profile name (live mode only). |
| `--config` | `""` | Config file path (live mode only). |
| `--timeout` | `30s` | Timeout for each live resource load (e.g. `30s`, `2m`). |
| `--verbose` | `false` | Print pre-launch diagnostics to stderr. |
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
5. Catalog construction in unloaded state.

Resource collection is intentionally not part of prelaunch. Pressing enter on a
resource starts one per-resource load; success updates that resource, and
failure renders a sanitized error state inside the TUI.

## Live smoke checklist

Run these against a scratch tenant before declaring live mode ready:

- [ ] `go run ./cmd/zscalerctl-tui --live` launches quickly and shows unloaded products/resources.
- [ ] `go run ./cmd/zscalerctl-tui --live --products zia` filters to ZIA only without preloading every ZIA resource.
- [ ] `go run ./cmd/zscalerctl-tui --live --products zia --resources locations` shows only the targeted resource and loads it on selection.
- [ ] A selected entitlement-gated or unsupported resource becomes a sanitized TUI error state.
- [ ] Missing credentials exit with an error before the TUI opens.
- [ ] Invalid config exits with an error before the TUI opens.
- [ ] Non-TTY invocation exits with a gate error before any config or credential work.
- [ ] `--format json` exits with a gate error before any config or credential work.
- [ ] No secrets appear in the TUI rendering or in any logged output.
- [ ] `scripts/verify-pty-escape-clean.sh` still passes (normal `zscalerctl` JSON output is clean).
- [ ] `go list -deps ./cmd/zscalerctl | grep -E 'bubbletea|internal/tui/tea'` still produces no matches.

## Future work

- Add `--log-level` to surface SDK diagnostics when debugging live reads.
- Decide whether the main `zscalerctl` binary should ever `exec` this separate
  binary.
