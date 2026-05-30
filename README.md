# zscalerctl

`zscalerctl` is an unofficial, security-first Go CLI for authorized Zscaler
administrators. The initial release is read-only by design and focuses on safe
configuration query, inventory, and controlled sanitized exports.

This project is not affiliated with, endorsed by, or sponsored by Zscaler.

The canonical binary is `zscalerctl`. If you prefer a short command locally,
use a shell alias:

```sh
alias zctl=zscalerctl
```

The primary use case is CLI and agentic automation: one reviewed command that
can replace duplicated Python snippets across pipelines and workflows. Human
tables should be readable, but machine output should stay explicit,
deterministic, and script-friendly.

Human output should feel polished in modern 256-color terminals, but color must
be optional and never required to understand results.

Color policy is explicit:

```sh
zscalerctl --color auto doctor
zscalerctl --color always doctor
zscalerctl --no-color doctor
```

`NO_COLOR` and `TERM=dumb` are respected when color mode is `auto`.

## Current Status

This project is in early scaffold mode. The code intentionally starts with the
safety rails before adding Zscaler API resources:

- Secret-safe value type.
- Environment config loading with `ZSCALERCTL_*` names.
- Read-only operation markers.
- Output redaction backstop.
- Type-enforced projected resource output.
- Restricted dump writer with `manifest.json` and value-free
  `redaction_report.json`.
- CLI skeleton for `doctor`, `auth status`, `config show`, `schema list`,
  `completion bash|zsh|fish`, `zia locations list|get`, and
  ZIA resource `list|get` commands.

The initial live reader supports a small read-only ZIA resource set through the
official Go SDK. It requires explicit `ZSCALERCTL_*` configuration and does not
consume the SDK's own environment variable names, local SDK config files, SDK log
flags, or ambient proxy variables unless `ZSCALERCTL_PROXY_FROM_ENV=true` is set.
SDK response caching is disabled.

OneAPI credentials are the default:

```sh
export ZSCALERCTL_CLIENT_ID=...
export ZSCALERCTL_CLIENT_SECRET_FILE=/path/to/owner-only/secret-file
export ZSCALERCTL_VANITY_DOMAIN=...
export ZSCALERCTL_CLOUD=PRODUCTION # optional
```

ZIA legacy credentials are supported for read-only ZIA resources through
explicit `ZSCALERCTL_ZIA_*` variables. Raw SDK names such as `ZIA_USERNAME` are
intentionally ignored.

```sh
export ZSCALERCTL_AUTH_MODE=zia-legacy
export ZSCALERCTL_ZIA_USERNAME=...
export ZSCALERCTL_ZIA_PASSWORD_FILE=/path/to/owner-only/password-file
export ZSCALERCTL_ZIA_API_KEY_FILE=/path/to/owner-only/api-key-file
export ZSCALERCTL_ZIA_CLOUD=zscalerthree
```

Corporate proxy settings are opt-in. To use standard `HTTPS_PROXY`,
`HTTP_PROXY`, and `NO_PROXY` values, set:

```sh
export ZSCALERCTL_PROXY_FROM_ENV=true
```

To avoid ambient proxy discovery while still using a proxy, set an explicit
proxy URL:

```sh
export ZSCALERCTL_PROXY_URL=http://proxy.example.invalid:8080
```

```sh
zscalerctl zia locations list
zscalerctl zia location-groups list
zscalerctl zia rule-labels list
zscalerctl zia static-ips list
zscalerctl zia gre-tunnels list
zscalerctl completion zsh
zscalerctl version
zscalerctl dump --products zia --out ./dump
zscalerctl dump --products zia --resources locations,static-ips --out ./dump-subset
zscalerctl dump --products zia --continue-on-error --out ./partial-dump
make live-smoke
```

`make live-smoke` validates every current ZIA live-smoke resource and writes
artifacts to a secure temporary directory. Set `LIVE_SMOKE_OUT=./scratch-live-smoke`
only when you want a predictable artifact path.

Key design docs:

- [THREAT_MODEL.md](THREAT_MODEL.md)
- [DATA_CLASSIFICATION.md](DATA_CLASSIFICATION.md)
- [ZSCALER_SENSITIVE_DATA.md](ZSCALER_SENSITIVE_DATA.md)
- [ARCHITECTURE.md](ARCHITECTURE.md)
- [docs/INSTALL.md](docs/INSTALL.md)
- [docs/RESOURCES.md](docs/RESOURCES.md)
- [docs/RESOURCE_QUEUE.md](docs/RESOURCE_QUEUE.md)
- [docs/VERSIONING.md](docs/VERSIONING.md)
- [docs/DEPENDENCY_POLICY.md](docs/DEPENDENCY_POLICY.md)
- [docs/RELEASE_CHECKLIST.md](docs/RELEASE_CHECKLIST.md)

## Security Posture

This is defensive administration software. It is not an exploitation,
credential discovery, bypass, traffic interception, or attack-path tool.

The primary leak-prevention model is allow-list projection into safe view
structs. Output redaction and secret scanning are defense-in-depth, not an
excuse to render raw API responses.

Administrator-controlled free-text fields are standard-only catalog exceptions:
each one must be justified in the schema, scanner-backed, and excluded from
`share` and `paranoid` output.

Version 1 must not include write commands or a generic raw API executor.

Table output is best-effort for quick human inspection. JSON and dump output are
the primary automation surfaces.

Dump commands fail closed by default: if a selected resource fails, no dump is
written. `--continue-on-error` is opt-in and writes a clearly marked partial
dump with `manifest.json` status `partial` and value-free `errors.ndjson`.

## Development

```sh
make fmt-check
go test ./...
go test -race ./...
go vet ./...
govulncheck ./...
bash scripts/verify-sdk-boundary.sh
bash scripts/test-verify-sdk-boundary.sh
```

Optional local checks once installed:

```sh
go run honnef.co/go/tools/cmd/staticcheck@v0.7.0 ./...
gitleaks dir .
gitleaks git .
```
