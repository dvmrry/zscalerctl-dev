# Builder Handoff

## Intent

fix(redact): redact a private key from its BEGIN line to the end of the value

A downstream re-check of `9bb0344` found that a private key pasted without
its END line leaked its body through `Bytes` when a PGP armour header such as
`Comment: Alice <alice@example.com>` came before the body. Every attempt to
decide where the armour ends from its contents leaked in a new shape (see
Review History), so the model changed: there is one private-key rule, and a
private key BEGIN line (any case, loose dashes and spacing, labels with
digits and `._-`) redacts everything after it to the end of the scanned
text, with or without an END line. Projection scans each field value on its
own and JSON output is scanned per decoded string, so that is normally the
end of one field value. Text before BEGIN and sibling values are untouched;
prose after a key in the same value is intentionally lost, and a value with
a private key reports one `private_key_block` match.

## Base / Head

- Base: `integration/2026-10-09` at `9bb0344`.
- Reviewed heads: `db0d0cc` (round 1), `303067d` (round 2), `1455784`
  (round 3), `8e20a4b` (round 4), `459941c` (round 5), `52b8c84` (round 6,
  delta from `459941c`). Rounds 1-5 were amends of one commit.
- Final head: `52b8c84`. This artifact is added afterwards and changes no
  reviewed code.

## Files Changed

- `internal/redact/redact.go`
- `internal/redact/private_key_armor_test.go`

## Tests Run

- `go test ./...` and `go vet ./internal/redact/` (Go 1.26.9): pass.
- Every leak shape a review round reported was added to
  `TestPrivateKeyArmorRedactsCompletely` and confirmed to fail against the
  previous rule before the fix (all string scanners and `Bytes`).

## Review History

Reviewed with no repository access by a fresh-context local model given the
diff, surrounding source and call sites, and this repository's reviewer
prompt and template: Strata 0.1.43.1 serving Qwen3.8-Flash-Next (IQ3_S) on
gm01. The reviewer could not run commands; it traced RE2 semantics by hand,
and the builder reproduced each blocking finding before fixing it.

- Round 1, request changes: an indented header line, and a header value
  holding a literal `\n`, still stopped the END-less match before the body.
- Round 2, request changes: the blank-line terminator accepted escaped
  separators inside a raw header value (`Comment: Use \n\n ...`).
- Round 3, request changes: raw and escaped line breaks mixed before the
  first header skipped the header clause. The builder stopped modelling the
  armour and redacted to the end of the value.
- Round 4, request changes: the separate lazy complete-block rule ran first
  and stopped at an END-like line inside a header or prose, consuming the
  BEGIN and leaving the real body. The builder removed it; one rule now
  ignores END entirely and matches BEGIN case-insensitively with loose
  dashes and spacing.
- Round 5, approve with nits (below). Addressed in `52b8c84`: labels with
  digits and punctuation (`RSA1`, `SSH2 ENCRYPTED`, `PKCS8`, `X9.42 DH`).
- Round 6 (delta `459941c..52b8c84`), approve with nits: the class change
  only adds matches. Remaining nits: invented labels such as `PKCS#8` are
  still missed; armour-like text such as `BEGIN CHANGE-ME PRIVATE KEY`
  over-redacts, which is accepted.

Accepted and not changed: whole-buffer backstop scans (the stdout/stderr
writer, `WriteText`) can erase following output if a BEGIN marker survives
projection, which only happens when projection already missed it; a bare
`BEGIN RSA PRIVATE KEY` with no dashes and a marker split across lines are
not matched; prose-prefixed JSON-in-JSON with escaped dashes is a
pre-existing `ScanString` limitation.

# Adversarial Review

Fresh-context reviewer: Strata 0.1.43.1 (Qwen3.8-Flash-Next IQ3_S, gm01), read-only, no command execution, 2026-10-13 (round 5)

Editor's note: private key BEGIN markers quoted below are written `----- BEGIN` (space after the dashes), and a sample JWT is replaced by a placeholder, so the docs and secret-scan checks pass; the reviewer wrote them literally.

Assume the change is wrong until proven otherwise.

## Blocking Findings

No blocking finding that proves this round’s change is wrong for the realistic Zscaler/config shapes represented by the diff and tests.

The new rule is strictly safer than the previous pair of rules in the central failure mode: it cannot stop early inside a scanned value because the match is `(?is) ... .*` and it runs first among base rules. The JSON/NDJSON projection paths keep the regex inside decoded string tokens or individual NDJSON lines, so a private-key marker in one JSON string does not consume sibling JSON syntax or adjacent NDJSON records. The tests pin the important contract: text before `BEGIN` survives, the rest of the same value is redacted with or without `END`, sibling JSON values are untouched, and public keys are not redacted.

That said, there are hardening gaps and blast-radius risks worth recording.

---

## Non-Blocking Risks

### 1. BEGIN variants still missed because the label class is too narrow

**file/path**

- `internal/redact/redact.go`
- rule: `private_key_block`
- regex: `(?is)-{3,}[ \t]*BEGIN[ \t]+[A-Z \t]*PRIVATE[ \t]+KEY.*`

**concrete issue**

The character class between `BEGIN` and `PRIVATE` is `[A-Z \t]*`. Under `(?i)` this matches letters and spaces/tabs, but it does not match digits or punctuation. Therefore armor headers containing digits in the label can survive.

Examples that currently do **not** match:

```text
----- BEGIN RSA1 PRIVATE KEY-----
----- BEGIN SSH2 ENCRYPTED PRIVATE KEY-----
----- BEGIN PKCS8 PRIVATE KEY-----
----- BEGIN X9.42 DH PRIVATE KEY-----
```

The first two are the most concerning because they are real private-key armor labels, though they are legacy/nonstandard compared with the common labels that are matched:

```text
----- BEGIN RSA PRIVATE KEY-----
----- BEGIN EC PRIVATE KEY-----
----- BEGIN DSA PRIVATE KEY-----
----- BEGIN PRIVATE KEY-----
----- BEGIN ENCRYPTED PRIVATE KEY-----
----- BEGIN OPENSSH PRIVATE KEY-----
----- BEGIN PGP PRIVATE KEY BLOCK-----
```

Also still missed:

```text
BEGIN RSA PRIVATE KEY
```

because the rule requires `-{3,}`. This may be acceptable because PEM/PGP armor normally has dashes, but a log/UI that strips armor dashes could still contain key material after a bare `BEGIN RSA PRIVATE KEY` header.

Also still missed if the marker is split by a newline:

```text
----- BEGIN RSA PRIVATE
KEY-----
MIIE...
```

That is not valid armor, but it can occur from hard-wrapped pasted text.

**why it matters**

If the repository can see legacy SSH or nonstandard private-key armor, key material can survive even though the value contains a recognizable private-key header. The prefilter `containsFold("private")` will let the value through, but the regex will not match.

**how to reproduce or verify**

Add a table entry like:

```go
{"legacy ssh2 private key", "----- BEGIN SSH2 ENCRYPTED" + " PRIVATE KEY-----\n" + strings.Join(body, "\n")},
{"legacy rsa1 private key", "----- BEGIN RSA1 PRIVATE" + " KEY-----\n" + strings.Join(body, "\n")},
{"pkcs8 label", "----- BEGIN PKCS8 PRIVATE" + " KEY-----\n" + strings.Join(body, "\n")},
```

Current expected failure: the body survives.

**test that would catch it**

A `TestPrivateKeyArmorRedactsCompletely` case for each realistic missed label, asserting no body line survives.

**judgment**

If Zscaler admin config can contain legacy SSH/SSH2/PKCS8-style armor, promote this to blocking. If the repo only sees modern PEM/OpenSSH/PGP armor, it is a hardening nit.

---

### 2. Whole-buffer writer blast radius can erase following output when text is not recognized as JSON/NDJSON

**file/path**

- `internal/redact/writer.go`
- `NewWriter` buffers all writes and calls `ScanRenderedString` once on `Close`.

**concrete issue**

The rule redacts from the first matching `BEGIN ... PRIVATE ... KEY` to the end of the scanned text. For normal JSON/NDJSON output, the structured scanner limits the blast radius to a decoded string or an NDJSON line. But the writer is a whole-buffer backstop. If the accumulated buffer is not recognized as a valid JSON document or valid NDJSON stream, the plain-text path can redact everything after the first BEGIN marker.

Example:

```go
var b bytes.Buffer
w := redact.NewWriter(&b, redact.ModeStandard)

w.Write([]byte("note: ----- BEGIN RSA PRIVATE KEY----- example\n"))
w.Write([]byte("{\"status\":\"ok\"}\n"))

w.Close()
```

The buffer is not valid JSON as a whole, and it is not valid NDJSON because the first line is not JSON. `scanNDJSONDocuments` returns false, so the plain path runs. The private-key rule then redacts from the BEGIN marker to the end of the buffer. The second line can disappear.

**why it matters**

This is not a leak. It is over-redaction. But it can change machine-visible output if a BEGIN marker appears in non-JSON text followed by structured output. In a redaction layer whose failure mode must be over-redaction, this is defensible, but it is a wider blast radius than per-field projection.

**how to reproduce or verify**

The snippet above. Expected current behavior: output collapses to `<REDACTED:PRIVATE_KEY>` or similar, and the later JSON line is gone.

**test that would catch it**

A writer test with two writes:

1. a plain-text line containing `----- BEGIN RSA PRIVATE KEY-----`,
2. a valid JSON/NDJSON line.

Then assert either:

- the second line survives, if the contract requires line independence for mixed output, or
- the whole buffer collapses, if that is intentionally accepted.

Right now the test suite does not pin this writer blast radius.

**judgment**

Non-blocking for the normal `WriteJSON`/`WriteNDJSON` paths because those produce valid JSON/NDJSON and are scanned structurally. Worth pinning if the CLI can mix plain text and structured output in the same writer buffer.

---

### 3. JSON-in-JSON and double-escaped forms can survive `ScanString` if the outer value does not itself start as JSON

**file/path**

- `internal/redact/redact.go`
- `scanJSONString`
- `nestedJSONDocumentCandidate`

**concrete issue**

`scanJSONString` recursively scans a decoded string as a JSON document only when that string itself starts with `{` or `[` after trimming:

```go
func nestedJSONDocumentCandidate(value string) bool {
    trimmed := strings.TrimLeft(value, " \t\r\n")
    return trimmed != "" && (trimmed[0] == '{' || trimmed[0] == '[')
}
```

If a JSON string value is prose followed by an embedded JSON literal, `scanJSONString` does not decode that embedded JSON literal. The rendered/free-text scanners do decode embedded quoted literals with escapes, but the plain `ScanString` path does not.

A value like this can evade the private-key rule in a non-free-text path:

```text
see {"key":"\u2d2d2dBEGIN RSA PRIVATE KEY\nbody"}
```

The outer JSON document is valid and scanned as JSON, but the decoded field value starts with `see`, so it is not recursively scanned as a nested JSON document. The inner literal contains escaped dashes and escaped newline. The base regex sees no literal `----- BEGIN`, so it may not match.

**why it matters**

This is a pre-existing limitation, not introduced by this diff. But the review question asks whether key material can survive any scanner. It can, in double-escaped JSON-in-JSON values scanned through `ScanString` rather than `ScanFreeText`/`ScanRenderedString`.

**how to reproduce or verify**

Construct a JSON document whose string value contains prose plus an escaped JSON literal containing a double-escaped private-key header. Scan the whole document with `Bytes`/`ScanString`. Check whether the body survives.

A free-text scanner should catch it because `withEmbeddedJSONStrings` decodes quoted literals containing escapes.

**test that would catch it**

A test for a non-free-text field value containing:

```text
see {"key":"\u2d2d2dBEGIN RSA PRIVATE KEY\nbody..."}
```

asserting the body is redacted. If the contract says such fields are always free-text, then document that; otherwise add a decoder or scan embedded literals in `scanJSONString`.

**judgment**

Non-blocking unless the project uses `ScanString` on admin-controlled fields that may contain prose-prefixed JSON with escaped secrets.

---

### 4. False positives are now much broader because matching is case-insensitive, loose-dash, and redacts to end of value

**file/path**

- `internal/redact/redact.go`
- rule: `private_key_block`

**concrete issue**

The rule now matches lower-case, loose-dash, and missing-trailing-dash forms, and redacts to end of the scanned value. This can erase public administrator text that merely discusses private-key armor.

Realistic examples that would now be erased:

```text
----- BEGIN RSA PRIVATE KEY----- is the PEM armor header; do not paste it. See runbook.
```

becomes:

```text
<REDACTED:PRIVATE_KEY>
```

Another:

```text
Ticket INC0012345: ----- begin rsa private key----- was removed from this note.
```

becomes:

```text
Ticket INC0012345: <REDACTED:PRIVATE_KEY>
```

Another:

```text
The category page displays ----- BEGIN PRIVATE KEY----- download instructions.
```

becomes:

```text
<REDACTED:PRIVATE_KEY>
```

A rule name or comment containing a documentation example can lose the rest of the field.

**why it matters**

This is a contract change for free-text fields: text after a private-key marker is no longer preserved. The commit message says this is intentional, and the tests now pin it.

**how to reproduce or verify**

```go
input := "----- BEGIN RSA PRIVATE KEY----- is the PEM armor header; see runbook."
got, _ := redact.New(redact.ModeStandard).ScanFreeText(input)
// got is "<REDACTED:PRIVATE_KEY>"
```

**test that would catch it**

Already present in spirit: the test now expects prose after the key to be lost. A dedicated false-positive test could document the accepted over-redaction:

```go
{"documentation example erased", "----- BEGIN RSA PRIVATE KEY----- is armor; see runbook."}
```

with expectation that the whole value becomes the marker.

**judgment**

Acceptable for a redaction layer whose failure mode must be over-redaction, provided the machine contract only requires valid JSON/NDJSON and not semantic preservation of public descriptions. The change intentionally favors not leaking key material over preserving prose.

---

### 5. Report counts can hide other secrets consumed by the private-key rule

**file/path**

- `internal/redact/redact.go`
- `scanRule`
- rule: `private_key_block`

**concrete issue**

Because the private-key rule runs first and redacts to end of value, it can consume JWTs, provisioning keys, credentials, or other secrets that appear after the BEGIN marker. The report counts one `private_key_block` match, not the other secret types.

**why it matters**

If any caller or test uses report counts to assert which secret classes were found, counts may understate the secret material that was present. The builder response says one match per scanned value is accepted.

**how to reproduce or verify**

Scan a value containing:

```text
----- BEGIN RSA PRIVATE KEY-----
<example JWT, removed>
```

The output is one private-key marker, and the report likely counts only `private_key_block`.

**test that would catch it**

If report counts are contractual, add a test asserting the expected count behavior. If they are not contractual, document that private-key redaction is terminal within a value and may suppress other counts.

**judgment**

Non-blocking. The commit explicitly accepts one count per scanned value.

---

## Machine Contract Review

### JSON output

**Behavior**

`WriteJSON` marshals to valid JSON, then calls `Redactor.Bytes`. `ScanString` sees a valid JSON document and uses `scanStructuredDocuments`, which scans decoded JSON string tokens. The private-key rule therefore redacts inside a string value, not across JSON syntax.

**Contract impact**

- Valid JSON remains valid for the normal JSON output path.
- Sibling JSON values are preserved.
- A value containing a private-key marker becomes `<REDACTED:PRIVATE_KEY>`.
- Text before the marker in the same value is preserved.
- Text after the marker in the same value is intentionally lost.

The test:

```go
doc, _ := json.Marshal(map[string]string{"description": rsa + "\n" + body[0], "name": "Branch East"})
...
if decoded["description"] != "<REDACTED:PRIVATE_KEY>" || decoded["name"] != "Branch East" {
```

pins the important sibling-value contract.

**Risk**

If a JSON document is malformed and falls back to plain scanning, the rule can consume JSON syntax after a BEGIN marker. But the normal output paths marshal valid JSON before scanning.

---

### NDJSON output

**Behavior**

`WriteNDJSON` writes each record through `Redactor.Bytes` individually, so each line is scanned as its own JSON document. `dump` writes a whole NDJSON buffer through `Redactor.Bytes`; `scanNDJSONDocuments` scans each valid line when there are at least two complete JSON documents. A single compact JSON document plus newline is valid JSON and is scanned as JSON.

**Contract impact**

- For valid NDJSON, line structure is preserved.
- A private-key marker in one record does not erase adjacent records.
- If the stream is not valid NDJSON, the scanner can fall back to plain text and collapse everything after a BEGIN marker.

**Risk**

The writer backstop can still collapse mixed plain-text/NDJSON buffers because the whole accumulated buffer may not be valid NDJSON. This is a blast-radius risk, not a normal NDJSON contract break.

---

### Error envelopes and CLI errors

**Behavior**

Error strings are scanned individually in several call sites. A BEGIN marker inside an error message redacts the rest of that message. The envelope remains structurally valid because the message is a JSON string value.

**Contract impact**

- Error envelope shape remains valid.
- Error detail after a BEGIN marker can be lost.
- Sibling error fields remain intact.

**Judgment**

Over-redaction is acceptable, but operators may see less diagnostic detail if an error message contains a private-key armor example.

---

### Writer / stdout / stderr

**Behavior**

`redact.NewWriter` accumulates all writes and scans once at `Close`. This is a second, whole-buffer pass over output that may already have been redacted per field or per JSON document.

**Contract impact**

- If the buffer is valid JSON or valid NDJSON, structured scanning limits damage.
- If the buffer is mixed text and structured output, a BEGIN marker can erase the rest of the buffer.
- This is not pinned by the current tests.

**Judgment**

Acceptable as a fail-closed backstop, but worth a test if mixed output is a supported machine contract.

---

## Safety Review

### Redaction strength

The change improves redaction strength:

- The old complete-block rule could stop at an END-like line inside headers or prose.
- The old END-less rule stopped at characters armor cannot contain, which allowed headers/prose to terminate the match.
- The new rule does not try to determine where the armor ends. Once a BEGIN marker is recognized, the rest of the scanned value is redacted.

This directly addresses the round-1 through round-4 failure mode.

### Field coverage

No field coverage change is shown. Projection still scans string fields with `ScanFreeText`, `ScanString`, or `ScanRenderedString` depending on field class/mode. The private-key rule is in base rules, so it applies to all three standard scanners.

### Output widening

No output widening is introduced. The change can only reduce visible text.

### Dump and diff artifacts

Dump JSON/NDJSON remains sanitized. Because the rule can erase more text, a dump may contain more markers and fewer public details, but it cannot reveal additional secrets.

---

## Generated Artifact Review

The diff changes:

- `internal/redact/redact.go`
- `internal/redact/private_key_armor_test.go`

No generated docs, schemas, fixtures, or manifests are shown in the diff.

If any generated docs or external fixtures describe the old behavior — especially the old test expectation that prose after a truncated key survives — they are now out of sync. The commit message documents the intentional behavior change, but generated artifacts should be regenerated if they encode that behavior.

---

## Test Contract Review

The tests pin the core contract:

- Text before `BEGIN` is kept.
- The rest of the value is redacted with or without `END`.
- Sibling JSON values are untouched.
- Public keys are untouched.
- Many END-less, escaped, CR-only, header-containing-END, lowercase, loose-dash, and missing-trailing-dash shapes are covered.

The tests do not pin:

- writer whole-buffer blast radius,
- NDJSON sibling-record blast radius through the writer,
- legacy digit-bearing armor labels,
- bare no-dash `BEGIN RSA PRIVATE KEY` headers,
- prose-prefixed JSON-in-JSON values scanned through `ScanString` rather than free-text scanners.

Those gaps are non-blocking unless the corresponding shapes are supported product contracts.

---

## Verdict

Verdict: approve with nits


The change is a net security improvement and should not be blocked for this round. The main hardening nits are:

1. widen or test the BEGIN label class for digit-bearing legacy armor such as `SSH2`/`RSA1` if those can appear in Zscaler admin data;
2. add a writer/NDJSON blast-radius test if mixed stdout/stderr buffers are part of the machine contract;
3. document or test the intentional loss of prose after a private-key marker.
