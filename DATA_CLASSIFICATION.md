# zscalerctl Data Classification

This document classifies data handled by `zscalerctl` and defines default
handling rules. The classification applies to stdout, stderr, logs, config
files, test fixtures, dumps, diffs, CI artifacts, and documentation examples.

## Classes

### Class 0: Public Project Data

Examples:

- Source code.
- Documentation.
- Command help text.
- Synthetic examples that contain no tenant-like data.

Handling:

- May be committed.
- May appear in logs and docs.

### Class 1: Operational Metadata

Examples:

- Product name such as ZIA or ZPA.
- Resource type names.
- Counts of resources.
- Command duration.
- HTTP status classes without request or response bodies.
- Redaction rule names and counts.

Handling:

- May be printed and logged.
- Should avoid tenant identifiers unless explicitly classified otherwise.

### Class 2: Tenant Configuration Data

Examples:

- Policy names.
- Application segment names.
- Location names.
- Firewall, access, routing, and forwarding rules.
- Group names.
- Cloud names and tenant identifiers.
- Sanitized dump contents.

Handling:

- Sensitive by default.
- May be written only through approved output paths.
- Must pass through allow-list projection.
- Must be redacted according to the selected redaction mode.
- Must not be used in public examples unless synthetic.

### Class 3: Sensitive Identifiers And Free Text

Examples:

- Usernames.
- Email addresses.
- IP addresses.
- Domains and hostnames.
- Tenant, customer, org, or cloud identifiers.
- Descriptions, notes, labels, comments, and other administrator-controlled
  free-text fields.

Handling:

- Excluded, masked, or tokenized in `share` and `paranoid` modes.
- Free-text fields are high risk because administrators may paste credentials,
  ticket data, incident details, or internal hostnames into them.
- If emitted in `standard` mode, they must be explicitly allowed per resource
  and scanned before output, including a free-text-only high-entropy token
  backstop for bare unlabeled secret material.

### Class 4: Secrets And Credential Material

Examples:

- API keys.
- Client IDs when paired with secret material.
- Client secrets.
- Bearer tokens.
- Refresh tokens.
- Cookies.
- Session IDs.
- Authorization headers.
- Private keys.
- PEM blocks.
- Passwords.
- Credential-bearing URLs.
- Sandbox API tokens.
- Webhook tokens.
- SCIM bearer tokens.
- One-time passwords and temporary passwords.
- VPN or XAUTH pre-shared keys.
- Certificate upload blobs that contain private-key material.
- Encrypted private/session keys returned by enrollment certificate APIs.
- Provisioning keys, including Zscaler App Connector, Private Service Edge, or
  Network Connector provisioning keys.

Handling:

- Never printed.
- Never logged.
- Never written to dumps.
- Never committed.
- Never included in examples.
- Never included in redaction reports.
- Must be represented in code using secret-safe types where practical.

## Output Rules

The default output rule is fail closed:

- Unknown fields are not emitted.
- New API fields are not emitted until classified.
- Raw SDK responses are not rendered directly.
- Resource-specific view structs define the fields eligible for rendering.
- Non-secret fields must explicitly declare the redaction modes in which they
  are eligible for rendering.
- Secret-class fields are never eligible for rendering.
- Nested maps and lists of maps are never eligible merely because the top-level
  field is eligible. Nested fields must have their own explicit classifications,
  or the whole nested value is dropped.
- Allowed string values are still scanned before rendering because secrets can
  be pasted into names, descriptions, labels, comments, or other allowed fields.
- Per-value scanning catches self-describing secret shapes. It does not have
  enough context to guarantee that an arbitrary unlabeled string is safe, so
  field classification remains the primary control.
- Free-text fields receive an additional high-entropy token scan because they
  are the most likely place for arbitrary unlabeled credential material to be
  pasted by an administrator.
- Free-text fields may be emitted only in `standard` mode, and every such field
  must carry a `standard_free_text_reason` in the catalog. They are rejected in
  `share` and `paranoid` unless a future tokenization design explicitly changes
  the policy.
- The high-entropy scan preserves canonical UUIDs and contextual git commit
  SHAs, but may redact other long hashes or thumbprints. It will not catch a
  UUID-shaped secret pasted without a labeling keyword. It is a backstop, not
  proof that every unlabeled secret is detectable.
- Context-sensitive generic field names, such as `value`, must be classified per
  resource. They must not be allowed merely because the name is generic.

See [ZSCALER_SENSITIVE_DATA.md](ZSCALER_SENSITIVE_DATA.md) for the current
Zscaler-specific source-backed inventory.

All renderers must serialize the same safe view model. Table, JSON, YAML, and
NDJSON formats must not each invent their own data path.

## Redaction Markers

Rendered redactions use typed markers so callers can distinguish an intentionally
obscured value from missing data.

Examples:

- `<REDACTED:SECRET>`
- `<REDACTED:PRIVATE_KEY>`
- `<REDACTED:JWT>`
- `<REDACTED:PROVISIONING_KEY>`
- `<REDACTED:EMAIL>`
- `<REDACTED:IP>`

Markers may appear inside otherwise allowed fields when the field itself is safe
to render but a substring looks secret-like.

## Redaction Report Rules

`redaction_report.json` may include:

- Rule names.
- Counts.
- Field paths.
- Resource names.
- Output file paths.
- Fields whose allowed values were modified by secret scanning.

It must not include:

- Original secret values.
- Original sensitive identifiers.
- Before/after examples from real tenant data.
- Raw snippets containing redacted content.

## Test Fixture Rules

Tests may use synthetic canary secrets, but they must be obviously fake and kept
inside test-only files.

Security tests should prove:

- Known canary values do not reach stdout, stderr, logs, or files.
- Rendered output is a subset of declared allow-listed fields.
- Free-text fields are handled according to the active redaction mode.
- Bare high-entropy canaries in emitted free-text fields are redacted.
- Secret-safe types do not reveal values through string formatting, JSON
  marshaling, or structured logging.

## Dump Sharing Guidance

Even after redaction, dumps are not public artifacts. They can reveal tenant
architecture and security posture.

Before sharing a dump:

- Use `share` or `paranoid` mode.
- Review the manifest and redaction report.
- Confirm the recipient is authorized to see tenant configuration.
- Treat the file as confidential operational data.
