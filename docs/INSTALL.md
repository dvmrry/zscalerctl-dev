# Installation

`zscalerctl` ships as a single Go CLI binary. The canonical command name is
`zscalerctl`; short local aliases such as `zctl` are intentionally left to the
operator's shell.

## Supported Platforms

Release artifacts are published for macOS and Linux on amd64 and arm64, and for
Windows on amd64.

File-backed secrets are supported when the secret file is owner-only. On macOS
and Linux, that means no group/world access. On Windows, `zscalerctl` validates
the file DACL and accepts access for the current user, the file owner, `SYSTEM`,
and `Administrators`; broad principals such as `Everyone`, `Users`,
`Authenticated Users`, and `Domain Users` are rejected.

Relatedly, `dump` creates its output directory and files with owner-only
permissions on macOS and Linux, but that enforcement does not apply on Windows:
the underlying `os.Chmod` has no ACL effect there, so the mode bits are not
honored. On Windows, write dumps into a directory that is already restricted to
your account (for example under your user profile), since the dump's own
permission tightening is a no-op.

## Verify Release Artifacts

GitHub releases include platform archives, per-target CycloneDX SBOMs,
`SHA256SUMS`, a keyless cosign bundle for `SHA256SUMS`
(`SHA256SUMS.bundle`), and GitHub provenance attestations for the subjects
listed in the checksum file.

After downloading release assets, verify the checksums from the directory that
contains the archives:

```sh
shasum -a 256 -c SHA256SUMS
```

Verify the cosign signature over `SHA256SUMS` (Sigstore keyless — no key to
distribute; the certificate identity is the release workflow):

```sh
cosign verify-blob \
  --bundle SHA256SUMS.bundle \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  --certificate-identity-regexp '^https://github.com/dvmrry/zscalerctl/\.github/workflows/release\.yml@' \
  SHA256SUMS
```

A valid signature over `SHA256SUMS`, combined with `shasum -c`, transitively
authenticates every archive and SBOM listed in it.

Verify the provenance attestation for the archive you plan to run:

```sh
gh attestation verify ./zscalerctl_<version>_<goos>_<goarch>.tar.gz \
  --repo dvmrry/zscalerctl \
  --signer-workflow github.com/dvmrry/zscalerctl/.github/workflows/release.yml
```

## Build From A Checkout

```sh
go install ./cmd/zscalerctl
zscalerctl doctor
zscalerctl version
```

## Configure Credentials

The CLI reads only `ZSCALERCTL_*` environment variables. It does not read the
Zscaler SDK's own environment variables or SDK config file.

A copy-and-edit template covering the variables below ships at
[`examples/zscalerctl.env.example`](../examples/zscalerctl.env.example). Copy
it to a local untracked file, keep that file owner-readable only (`chmod
600`), and source it into your shell.

OneAPI is the default auth mode. Prefer an owner-only secret file for the
client secret:

```sh
export ZSCALERCTL_CLIENT_ID=<client-id>
export ZSCALERCTL_CLIENT_SECRET_FILE=/path/to/owner-only/secret-file
export ZSCALERCTL_VANITY_DOMAIN=<vanity-domain>
export ZSCALERCTL_CLOUD=PRODUCTION
export ZSCALERCTL_ZPA_CUSTOMER_ID=<zpa-customer-id> # required for ZPA resources
export ZSCALERCTL_ZPA_MICROTENANT_ID=<zpa-microtenant-id> # optional
```

The secret file must be readable only by the current user. Inline
`ZSCALERCTL_CLIENT_SECRET` is supported for automation systems that already
provide protected environment variables, but file-based secret delivery is safer
for interactive shells. `ZSCALERCTL_ZPA_CUSTOMER_ID` is required only when
reading ZPA resources; ZIA, ZTW, and Zidentity resources use the standard
OneAPI credential set without an extra product customer ID.

Product API access is granted per product on the OneAPI OAuth client, not by
extra environment variables. ZCC and ZTW resources use the same
`ZSCALERCTL_CLIENT_ID`/`ZSCALERCTL_CLIENT_SECRET` credentials, but a Zscaler
administrator must grant the matching API resource to the OAuth client in the
Zidentity admin portal: Client Connector access for `zcc` resources, and Cloud
& Branch Connector access (labeled `CLOUD_CONNECTOR` in the portal's API
resource list) for `ztw` resources. Without the grant, commands for that
product fail at the API; no local configuration change can fix it.

### Switching Tenants

`zscalerctl` supports optional owner-only YAML profiles selected with
`--profile` and `--config`, but `ZSCALERCTL_*` environment variables remain the
highest-precedence path and the Zscaler SDK's own config lookup is never used.
To switch tenants in CI or an agent container, switching the `ZSCALERCTL_*`
environment block is still the simplest and most explicit option.

On macOS and Linux, keep one untracked env file per tenant and source the one
you need:

```sh
chmod 600 ~/.config/zscalerctl/prod.env ~/.config/zscalerctl/preprod.env
set -a
. ~/.config/zscalerctl/prod.env
set +a
zscalerctl doctor
```

For local operator workflows, a profile file can hold non-secret tenant
metadata plus secret references such as `env:NAME` or `file:/path`; the file is
rejected unless it is owner-only. Secret values themselves do not belong in the
profile file.

The fastest way to create a valid owner-only profile file is `zscalerctl config
init`, which writes a commented starter `config.yaml` to the default location
with permissions the loader accepts, then prints the path and next steps. The
default location is `$XDG_CONFIG_HOME/zscalerctl/config.yaml` (falling back to
`~/.config/zscalerctl/config.yaml`) on macOS and Linux, and
`%LOCALAPPDATA%\zscalerctl\config.yaml` on Windows. `%LOCALAPPDATA%` is used on
Windows instead of `%APPDATA%` (Roaming) because Roaming is frequently
fold-redirected to a UNC home on managed AD images, and `zscalerctl` only reads
config and secret files from a local fixed NTFS/ReFS volume. Pass `--force` to
overwrite an existing file, or `--config <path>` to write somewhere else.

Profiles can also reference a local secret command. `cmd` refs use a structured
argv array and are executed directly — no shell, no quoting or expansion — with
a 10 second timeout unless the ref sets a shorter or longer positive duration:

```yaml
profiles:
  prod:
    client_id: <client-id>
    vanity_domain: <vanity-domain>
    cloud: PRODUCTION
    client_secret_ref:
      cmd:
        argv: ["/usr/local/bin/zscaler-secret", "prod", "client-secret"]
        timeout: 5s
```

Use an absolute executable path when practical; otherwise `argv[0]` is resolved
through the operator's `PATH`. If a workflow needs shell features, put them in a
reviewed wrapper script and point `argv` at that script. Set
`ZSCALERCTL_DISALLOW_CMD=true` to reject `cmd` refs fleet-wide.

### Secret Providers

Profile secret references can use `env:NAME`, `file:/path/to/secret`,
structured `cmd:`, or `keyring:<service>/<key>`. `keyring:` is intended for
local operator desktops; agents and CI should continue to use protected
environment variables or owner-only secret files.

Store a macOS Keychain item with service and account matching the reference:

```sh
security add-generic-password -s zscalerctl -a prod-client-secret -w '<secret>'
```

Non-ASCII secret values round-trip correctly: `security -w` may emit such values
as hex, which the reader transparently decodes while preserving a literal
hex-looking secret unchanged.

Then reference it as:

```yaml
client_secret_ref: keyring:zscalerctl/prod-client-secret
```

On Linux, install `secret-tool` (`libsecret-tools` on Debian/Ubuntu) and store
the item with the `service` and `account` attributes:

```sh
secret-tool store --label="zscalerctl: zscalerctl/prod-client-secret" \
  service zscalerctl account prod-client-secret
```

On Windows, store a generic credential whose target is `<service>/<key>`:

```powershell
cmdkey /generic:zscalerctl/prod-client-secret /user:zscalerctl/prod-client-secret /pass:<secret>
```

You can also use Credential Manager: Windows Credentials -> Add a generic
credential, with "Internet or network address" set to `<service>/<key>`.
Use Keychain Access.app or Credential Manager instead of the CLI examples when
you do not want the secret value to appear in shell history.

For CI, use the CI platform's protected environment or secret store and set the
same variable names per job/environment. Do not commit env files, dump
directories, or live-smoke artifacts.

On Windows, inline environment variables and file-backed secrets are both
supported. If you use `*_FILE`, store the secret in a file whose DACL is limited
to your account plus administrative principals:

```powershell
$env:ZSCALERCTL_CLIENT_ID = '<client-id>'
$env:ZSCALERCTL_CLIENT_SECRET_FILE = "$env:USERPROFILE\.config\zscalerctl\client-secret.txt"
$env:ZSCALERCTL_VANITY_DOMAIN = '<vanity-domain>'
$env:ZSCALERCTL_CLOUD = 'PRODUCTION'
$env:ZSCALERCTL_ZPA_CUSTOMER_ID = '<zpa-customer-id>' # only for ZPA resources
zscalerctl doctor
```

ZIA legacy auth is available for read-only ZIA resources when OneAPI
credentials are not available. Use only `ZSCALERCTL_ZIA_*` variables; raw SDK
names such as `ZIA_USERNAME`, `ZIA_PASSWORD`, and `ZIA_API_KEY` are ignored.

```sh
export ZSCALERCTL_AUTH_MODE=zia-legacy
export ZSCALERCTL_ZIA_USERNAME=<zia-username>
export ZSCALERCTL_ZIA_PASSWORD_FILE=/path/to/owner-only/password-file
export ZSCALERCTL_ZIA_API_KEY_FILE=/path/to/owner-only/api-key-file
export ZSCALERCTL_ZIA_CLOUD=<zia-cloud-name>
```

Inline `ZSCALERCTL_ZIA_PASSWORD` and `ZSCALERCTL_ZIA_API_KEY` are supported for
short-lived local smoke tests, but file-based secret delivery is preferred.

## Drift Checks

`zscalerctl` does not run a daemon or scheduler. A drift workflow is simply:
collect one dump, collect a later dump, then compare the two directories.

```sh
today="$(date -u +%Y%m%dT%H%M%SZ)"
zscalerctl dump --products zia,zpa,ztw,zcc,zidentity --out "$HOME/zscaler-dumps/$today"
zscalerctl --format json diff "$HOME/zscaler-dumps/previous" "$HOME/zscaler-dumps/$today" \
  --fail-on-drift --output "$HOME/zscaler-dumps/$today.diff.json"
```

Use cron, launchd, systemd timers, GitHub Actions, or your scheduler of choice
to run those commands on the cadence you need. For GitHub Actions, keep the
dump directories as encrypted/restricted artifacts or write them to a protected
storage location; sanitized dumps and diff reports are still confidential tenant
inventory.

```yaml
on:
  schedule:
    - cron: "0 6 * * *"
jobs:
  drift:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v6
      - run: zscalerctl dump --products zia --out ./current
      - run: zscalerctl --format json diff ./baseline ./current --fail-on-drift --output ./drift.json
```

## Configure A Proxy

By default, live reads use a direct transport and ignore ambient proxy
variables. This keeps credentialed traffic from being redirected by unrelated
shell state.

If your office network requires the standard Go proxy environment, opt in
explicitly:

```sh
export ZSCALERCTL_PROXY_FROM_ENV=true
export HTTPS_PROXY=http://proxy.example.invalid:8080
export NO_PROXY=localhost,127.0.0.1
```

If you prefer not to depend on ambient `HTTPS_PROXY`/`HTTP_PROXY`, set a
dedicated proxy URL instead:

```sh
export ZSCALERCTL_PROXY_URL=http://proxy.example.invalid:8080
```

`ZSCALERCTL_PROXY_URL` and `ZSCALERCTL_PROXY_FROM_ENV` are mutually exclusive.
Status commands report only whether a proxy is configured, never the proxy URL.

## Shell Completions

Completion scripts are static helper output. Generating them does not contact
Zscaler, construct a live reader, or read credential files.

### Bash

```sh
mkdir -p ~/.local/share/bash-completion/completions
zscalerctl completion bash > ~/.local/share/bash-completion/completions/zscalerctl
```

### Zsh

```sh
mkdir -p ~/.zfunc
zscalerctl completion zsh > ~/.zfunc/_zscalerctl
```

Add this once to your shell startup file if `~/.zfunc` is not already in
`fpath`:

```sh
fpath=(~/.zfunc $fpath)
autoload -Uz compinit
compinit
```

### Fish

```sh
mkdir -p ~/.config/fish/completions
zscalerctl completion fish > ~/.config/fish/completions/zscalerctl.fish
```

### PowerShell

For the current session:

```powershell
zscalerctl completion powershell | Invoke-Expression
```

To load completions in future sessions, append the generated script to your
PowerShell profile:

```powershell
New-Item -ItemType Directory -Force -Path (Split-Path -Parent $PROFILE)
zscalerctl completion powershell | Out-File -FilePath $PROFILE -Append -Encoding utf8
```

## Man Page

A man page ships in the repository at `man/zscalerctl.1`. Install it into a
`man1` directory on your `MANPATH`, for example:

```sh
mkdir -p ~/.local/share/man/man1
cp man/zscalerctl.1 ~/.local/share/man/man1/
man zscalerctl
```

## Local Alias

```sh
alias zctl=zscalerctl
```
