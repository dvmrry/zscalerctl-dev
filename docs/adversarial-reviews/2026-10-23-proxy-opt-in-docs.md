# Builder Handoff

## Intent

docs: say that the proxy is opt-in and what the ignored-proxy hint means

AGENTS.md and the skill said nothing about proxies, and the skill tells agents
to retry `timeout`, which loops forever on a network that needs a proxy the
CLI is ignoring. ARCHITECTURE.md still said proxy support "must be added",
though `ZSCALERCTL_PROXY_FROM_ENV` and `ZSCALERCTL_PROXY_URL` exist
(INSTALL.md documents both).

Docs only. The hint condition is `config.EnvironmentProxyIgnored`:
`HTTPS_PROXY`/`https_proxy` set, `ZSCALERCTL_PROXY_FROM_ENV` not true and
`ZSCALERCTL_PROXY_URL` empty; `withProxyHint` appends it to `timeout` and
`transport` failures only.

## Base / Head

- Base: `integration/2026-10-09` at `de5820b`.
- Reviewed head: `docs/proxy-opt-in` (staged).

## Files Changed

- `AGENTS.md`, `skills/zscalerctl/SKILL.md`, `.agents/skills/zscalerctl/SKILL.md` (generated), `docs/ARCHITECTURE.md`

## Tests Run

- `make verify-agents-skill docs-check`: pass.

## Review History

- Round 1 (fresh-context Claude subagent), request changes. Blocking: the
  skill said the message "ends with" a phrase that the emitted hint does not
  end with (it ends `...to use it)`). Fixed: the skill now says "contains",
  states the proxy is likely (not certainly) needed, points at `doctor`'s
  `Proxy` line (`direct (HTTPS_PROXY is set but not used; ...)`), and says
  dump `errors.ndjson` records do not carry the hint. All other claims were
  verified against the code.

# Adversarial Review

Fresh-context reviewer: Claude subagent (no conversation history, read-only repository access), 2026-10-23 (round 1)

## Blocking Findings

- The skill's "ends with `HTTPS_PROXY is set but not used`" never matches
  the emitted message. Fixed after review ("contains").

## Non-Blocking Risks

- The hint proves only that `HTTPS_PROXY` is set and unused, not that the
  network needs it. Fixed after review ("likely", check `doctor`).
- The hint is on the top-level error only, not dump `errors.ndjson` records.
  Fixed after review (stated in the skill).

## Checked, Sound

- Hint text matches `config.IgnoredProxyHint`; it reaches both the JSON
  envelope `message` and text errors.
- Applied only to `timeout` and `transport`; unwraps to the original, so
  kind, reason and exit code are unchanged.
- Condition: `HTTPS_PROXY`/`https_proxy` set, `ZSCALERCTL_PROXY_FROM_ENV`
  not true, `ZSCALERCTL_PROXY_URL` empty. `ZSCALERCTL_PROXY_URL` is used via
  `http.ProxyURL`; setting both is rejected; an invalid
  `ZSCALERCTL_PROXY_FROM_ENV` is `invalid_config` before any read.
- Proxy settings are env-only. ARCHITECTURE.md and AGENTS.md are accurate;
  the generated `.agents` skill copy matches.

## Tests Run

- `make -s verify-agents-skill docs-check`: exit 0.

Verdict: approve with nits
