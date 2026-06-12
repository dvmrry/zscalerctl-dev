# Security Policy

## Supported Versions

Before the first public release, security fixes apply to the current default
branch. After release, only the latest released minor version is supported unless
a release note says otherwise.

## Reporting a Vulnerability

Do not open a public issue for a suspected vulnerability, credential exposure, or
unsafe output behavior. Email `github@mrry.io` or use GitHub private
vulnerability reporting if it is enabled for the repository.

Please include:

- the affected `zscalerctl` version or commit;
- the command and output mode involved;
- whether the issue affects stdout, stderr, dump files, reports, logs, or CI;
- redacted sample output or a minimal synthetic reproduction.

Do not send real Zscaler credentials, tenant dumps, API tokens, private keys, or
unsanitized production output.

## Response

Reports are acknowledged within 7 days, best effort, and receive an initial
assessment with an estimated fix or risk decision within 14 days. This is a
single-maintainer project, so timelines may occasionally vary; every report is
read. Fixed vulnerabilities are disclosed publicly through GitHub Security
Advisories and identified in the release notes of the fixing release.

## Project Roles

This is a single-maintainer project: the repository owner is the only person
with privileged repository access and holds all roles — development, review
(enforced through the required CI gate rather than a second reviewer), release
approval, and security response via the contact above.

## Authorized Use

`zscalerctl` is intended for administrators and agents working with tenants they
are authorized to inspect. Reports about abuse potential, unsafe defaults, or
misleading output are welcome, but this project does not accept requests to add
reconnaissance features or unaudited raw API output paths.
