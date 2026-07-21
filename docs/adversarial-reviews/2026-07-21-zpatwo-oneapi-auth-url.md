# Builder Handoff

## Intent

Keep `ZSCALERCTL_CLOUD=ZPATWO` available to the Zscaler SDK for product API
routing while correcting the SDK-generated OneAPI token and ZIdentity admin
hosts from the nonexistent `zsloginzpatwo.net` domain to `zslogin.net`.
Reject malformed vanity and cloud values before the SDK can construct a URL.

## Base / Head

Base branch or commit: `origin/main` at `38653f53b152c3b86ed6ebd8c3aa011aedb9d896`

Head branch or commit: `feature/fix-zpatwo-auth-url` working tree

## Files Changed

- `internal/zscaler/reader.go`
- `internal/zscaler/reader_test.go`
- `docs/adversarial-reviews/2026-07-21-zpatwo-oneapi-auth-url.md`

## Source Inputs Consulted

- Vendored `zscaler-sdk-go` v3.8.38 `zscaler/oneapiclient.go` authentication
  and per-product HTTP-client selection.
- Vendored `zscaler-sdk-go` v3.8.38 `zscaler/oneapiconfig.go` product API URL
  construction.
- Upstream `zscaler-sdk-go` v3.8.41 `zscaler/oneapiclient.go`; its refactored
  `getAuthURL` still appends `zpatwo` to the `zslogin` hostname.
- Operator confirmation that the same credentials authenticate when cloud is
  set to `PRODUCTION`.

## Generated Artifacts

None.

## Expected Delta

- OneAPI authentication with cloud `ZPATWO` sends the OAuth token request to
  `https://<vanity>.zslogin.net/oauth2/v1/token`.
- ZIdentity reads with cloud `ZPATWO` use
  `https://<vanity>-admin.zslogin.net/admin/api/v1/...`.
- The SDK configuration retains cloud `ZPATWO`; product API routing is
  unchanged.
- Other clouds, hosts, schemes, and paths are unchanged.
- Vanity and cloud values that are not single DNS labels fail before network
  access.
- CLI and machine-readable surfaces are unchanged.

## Invariants Claimed

- The adapter does not mutate the SDK request supplied to the transport.
- Rewriting requires HTTPS, the configured vanity host, and the exact OAuth
  token path or ZIdentity admin path prefix.
- ZPA and other product clients retain the ordinary transport and the original
  `ZPATWO` cloud value.
- The vanity and cloud inputs used by SDK URL construction are validated as
  ASCII DNS labels before client construction.
- Proxy selection, request timeout, credentials, redaction, output, error
  taxonomy, and tenant-read-only behavior are unchanged.

## Tests Run

- Focused ZPATWO identity-routing and malformed-input tests — pass.
- The same focused tests under `go test -race` — pass.
- `go test ./internal/zscaler -count=1` — pass.
- `make check` — all substantive tests, race tests, vet, vulnerability scans,
  static analysis, license/docs/security/boundary gates passed; stopped only at
  `verify-adversarial-review` pending this artifact's independent verdict.

## Known Deferrals

- No live `ZPATWO` tenant smoke test was run; the regression captures the real
  SDK authentication request before network access.
- The upstream SDK remains unfixed through v3.8.41. This change is a narrowly
  scoped adapter workaround rather than a vendored patch or SDK upgrade.

## Review Focus

- Attack whether any non-authentication request can be redirected.
- Verify token renewal uses the same corrected transport.
- Verify the original request and product transports are not mutated.
- Verify preserving `ZPATWO` continues to control product API routing.
- Look for proxy, timeout, connection-pooling, concurrency, or credential
  exposure regressions caused by splitting the auth HTTP client.

## Review Resolution

Finding: Malformed vanity domains could cause the SDK to parse and send
credentials to an attacker-controlled host.

Root cause: OneAPI configuration checked only that the vanity domain was
non-empty before interpolating it into SDK URLs. The cloud value had the same
URL-construction risk.

Fix: Validate both values as single ASCII DNS labels before any SDK client is
constructed. Invalid values wrap the existing invalid-credentials sentinel and
do not include credential values.

Regression test: `TestNewReaderRejectsInvalidVanityDomain` and
`TestNewReaderRejectsInvalidCloud` cover separators, userinfo, schemes, ports,
multi-label values, overlong labels, edge hyphens, and non-ASCII input.

Verification: Focused ordinary and race tests pass.

Finding: ZIdentity requests retained the invalid
`<vanity>-admin.zsloginzpatwo.net` host and the negative test used a hostname
the SDK does not emit.

Root cause: The initial workaround handled only the token host even though the
SDK's ZIdentity URL builder applies the same cloud suffix.

Fix: The identity transport now rewrites the exact SDK OAuth host/path and the
exact SDK ZIdentity admin host/path prefix. Other traffic passes through.

Regression test: `TestNewSDKConfigurationZPATWORoutesZidentityToProductionHost`
authenticates through the real vendored client and performs a real vendored
ZIdentity user read; the transport-scope table tests the actual admin hostname
and pass-through boundaries.

Verification: Focused ordinary and race tests pass; the original request is
also asserted unchanged.

# Adversarial Review

Fresh-context reviewer: Codex reviewer agent `019f84e0-f023-7051-b1ce-1f283555b144`

## Blocking Findings

None. Both previously blocking findings are resolved.

## Resolution Recheck

Malformed vanity and cloud values are rejected before SDK client construction.
The errors wrap `ErrMissingCredentials`, preserving the documented
`missing_credentials` classification and exit code 3. Allowed values remain
compatible with documented vanity and cloud examples.

The ZPATWO identity transport now matches the SDK's actual OAuth and ZIdentity
admin hosts. Rewriting requires HTTPS, the exact configured host, and either
the exact OAuth token path or the ZIdentity `/admin/api/v1` path boundary. The
integration test authenticates with the real vendored client and performs a
real vendored ZIdentity user read while preserving `Cloud=ZPATWO`.

## Non-Blocking Risks

`validDNSLabel` permits source labels up to 63 bytes, while the SDK later forms
compound labels such as `<vanity>-admin` and `zslogin<cloud>`. Excessively long
but syntactically valid inputs can therefore fail DNS resolution. They cannot
redirect credentials, and documented configuration values are substantially
shorter. Derived-host length validation may improve diagnostics later.

## Machine Contract Review

No JSON, NDJSON, error-envelope schema, exit-code, dump, diff, manifest,
introspection, or CLI-surface changes were found. Malformed authentication
metadata fails earlier under the existing credential-error classification.

## Safety Review

No redaction, projection, field-coverage, narrowing, or tenant-data handling
changed. Malformed routing input is rejected before network access. Request
cloning and the shared underlying `http.Transport` preserve request
immutability, proxy behavior, pooling, and concurrency safety.

## Generated Artifact Review

No generated artifacts changed. Focused ordinary and race tests, the full
`internal/zscaler` package, `go vet ./...`, formatting, and diff checks passed.
The reviewer confirmed vendored SDK v3.8.38 resolution and made no workspace
changes.

## Verdict

Verdict: approve with nits
