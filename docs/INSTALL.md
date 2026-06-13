# Installation

`zscalerctl` ships as a single Go CLI binary. The canonical command name is
`zscalerctl`; short local aliases such as `zctl` are intentionally left to the
operator's shell.

## Supported Platforms

Release artifacts are published for macOS and Linux on amd64 and arm64, and for
Windows on amd64.

On macOS and Linux, file-backed secrets are supported when the secret file is
owner-only. On Windows, file-backed secrets are not supported yet because the
permission model needs an explicit ACL design and test coverage. Windows users
must use protected inline environment variables such as
`ZSCALERCTL_CLIENT_SECRET`, `ZSCALERCTL_ZIA_PASSWORD`, and
`ZSCALERCTL_ZIA_API_KEY`; `*_FILE` variables fail closed on Windows until ACL
support lands.

## Verify Release Artifacts

GitHub releases include platform archives, per-target CycloneDX SBOMs,
`SHA256SUMS`, and GitHub provenance attestations for the subjects listed in the
checksum file.

After downloading release assets, verify the checksums from the directory that
contains the archives:

```sh
shasum -a 256 -c SHA256SUMS
```

Verify the provenance attestation for the archive you plan to run:

```sh
gh attestation verify ./zscalerctl_<version>_<goos>_<goarch>.tar.gz --repo dvmrry/zscalerctl
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
