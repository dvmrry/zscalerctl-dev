# Builder Handoff

## Intent

chore(ci): bump semgrep to 1.179.0 and refresh its pinned deps

The 22 open Dependabot alerts on `main` are all in the CI-only lockfile
`.github/requirements/semgrep.txt` (PyJWT, urllib3, anyio, cryptography,
mcp). semgrep 1.166.0 pinned `mcp==1.23.3` and `pyjwt~=2.13.0`, so the
fixes needed a semgrep bump: 1.179.0 pins `mcp==1.29.0` and
`pyjwt>=2.15.0`. The lockfile is regenerated with `--upgrade
--exclude-newer 2026-10-03T00:00:00Z`, so every package is at least two
weeks old: anyio 4.15.1, cryptography 50.0.2, mcp 1.29.0, pyjwt 2.15.1,
urllib3 2.8.0. `Makefile` and `scripts/verify-semgrep.sh` pin the same
version, and the regenerate comment in `semgrep.in` gives the full command.
Supersedes Dependabot PRs #129, #133 and #134.

## Base / Head

- Base: `integration/2026-10-09` at `9311c77`.
- Reviewed head: `4ba579a`.
- Final head: round-1 nit fixes (cutoff tightened to midnight, dropping
  zipp 4.1.1 for 4.1.0; `semgrep.in` comment) and this artifact.

## Files Changed

- `.github/requirements/semgrep.in`, `.github/requirements/semgrep.txt`
- `Makefile`, `scripts/verify-semgrep.sh`

## Tests Run

- `SEMGREP_VERSION=1.179.0 bash scripts/verify-semgrep.sh`: pass.
- `verify-actions-pinned`, `verify-release-automation`,
  `verify-node-toolchain`, `verify-go-toolchain`, `verify-script-registry`,
  `scripts/test-verify-node-toolchain.sh`: pass.
- `uvx pip-audit --require-hashes -r .github/requirements/semgrep.txt`:
  only CVE-2026-59951 (below).

## Known Gaps

- CVE-2026-59951 / GHSA-84m7-p3x7-pcfv (medium): unbounded session
  accumulation in mcp's StreamableHTTP server, fixed in 1.30.0. Every semgrep
  release through 1.181.0 pins `mcp==1.29.0`. `semgrep scan` imports the
  mcp server modules but never builds or starts a server, so CI cannot reach
  the bug. Forcing 1.30.0 would need `--no-deps` in the policed install
  command and an untested combination. Dismiss the alert as "vulnerable code
  not used" and recheck when semgrep moves off 1.29.0.

## Review History

Strata on gm01 remained stopped, so the review ran in a fresh-context Claude
subagent with no conversation history and read-only repository access.

- Round 1, approve with nits (below). Acted on: cutoff tightened from the
  date to midnight UTC so no pin is under 14 days old to the hour; the
  `semgrep.in` regenerate comment now includes `--upgrade` and
  `--exclude-newer`.

# Adversarial Review

Fresh-context reviewer: Claude subagent (no conversation history, read-only repository access), 2026-10-17 (round 1)

## Blocking Findings

None. The header command reproduces `semgrep.txt` byte for byte; `uv` and
`pip 26.2.1` installs with `--require-hashes` succeed on Python 3.12. Every
pin's PyPI upload time is on or before 2026-10-03. All 22 advisories'
vulnerable ranges exclude the new versions. No `1.166` reference remains,
and the CI and release install commands are unchanged. On a synthetic Go
fixture, semgrep 1.166.0 and 1.179.0 give the same 6 findings for the repo
rules.

## Non-Blocking Risks

1. minor: the mcp CVE has an advisory, so Dependabot will open one alert.
   Not reachable from `semgrep scan`; recorded in Known Gaps.
2. nit: zipp 4.1.1 was under 14 days old to the hour. Fixed (4.1.0).
3. nit: the `semgrep.in` regenerate comment omitted `--upgrade` and
   `--exclude-newer`. Fixed.
4. nit: `verify-semgrep.sh` bad-fixture checks test only the exit code, not
   which rule fired. Predates this change.

## Verdict

Verdict: approve with nits
