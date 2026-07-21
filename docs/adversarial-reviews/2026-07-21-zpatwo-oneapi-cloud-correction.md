# Builder Handoff

## Intent

Correct the earlier identity-only ZPATWO workaround. Keep `ZPATWO` as an
accepted compatibility input while normalizing it to `PRODUCTION` at the
OneAPI SDK-construction boundary so OAuth, product, and ZIdentity requests all
use production endpoints.

## Base / Head Reviewed

- Base: `2be8061bd7406f72146823999d22b3b3f11720f8`
- Reviewed head: `ba76aa198cbd45a4f97fdd82f50537d5646178e1`
- Process baseline: `origin/main` at the base commit
- Reviewer context: fresh read-only subagent with no builder conversation
  history

The prior artifact
`docs/adversarial-reviews/2026-07-21-zpatwo-oneapi-auth-url.md` remains an
unchanged historical record of the earlier architecture and approval.

# Adversarial Review

Fresh-context reviewer: Huygens (`019f865b-c547-76c0-87a2-320b1174214f`)

## Blocking Findings

None.

## Non-Blocking Risks

None identified.

## Machine Contract Review

- `ZPATWO` is normalized to `PRODUCTION` at the sole OneAPI SDK-construction
  boundary.
- Vendored SDK source and regression tests confirm production routing for:
  - OAuth: `https://<vanity>.zslogin.net/oauth2/v1/token`
  - generic product/ZPA: `https://api.zsapi.net/...`
  - ZIdentity: `https://<vanity>-admin.zslogin.net/admin/api/v1/...`
- Other OneAPI cloud values remain unchanged; `BETA` is covered explicitly.
- Legacy ZIA uses its separate cloud configuration and constructor and is not
  remapped.
- JSON, NDJSON, error envelopes, exit codes, manifest, introspection, dump, and
  diff contracts are unchanged.
- README, installation guide, config schema description, and man page
  distinguish configured compatibility input from effective production SDK
  routing.

## Safety Review

No redaction, projection, field coverage, narrowing, credential handling, or
tenant-data behavior changed. The obsolete request-rewriting transport was
removed; the standard configured transport remains in use for every SDK HTTP
client.

## Generated Artifact Review

No generated artifacts changed. Generated CLI documentation and repository
documentation drift checks passed. No prior adversarial-review artifact was
modified.

## Independent Verification

The reviewer reported these checks passing against the reviewed head:

- focused routing and configuration tests;
- focused race tests;
- `go test ./internal/... -count=1`;
- documentation and generated CLI documentation checks;
- SDK and core boundary checks;
- machine-contract verification;
- `git diff --check`.

No credentials or live systems were used.

Verdict: approve
