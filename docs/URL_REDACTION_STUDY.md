# URL Redaction Investigation

Decision: retain the existing runtime redactor in this increment. The reported
URL overreach is reproducible, but the global component-scanning prototype did
not preserve the required protection for opaque values. No runtime URL
redaction change is included in `feature/config-investigation-foundation`.

## Evidence

These examples are synthetic. The baseline is commit
`2b350c94680b160d4059af886de1fdce147d6887`. The broad entropy candidate pattern
accepts dots and slashes. It can combine a hostname and path into one long
candidate; the ordinary versioned URL below is consequently redacted by
`ScanRenderedString`, `ScanFreeText`, and the final writer. The explicit-pattern
`ScanString` pass alone leaves it intact.

| Synthetic input | Baseline rendered scan | Interpretation |
| --- | --- | --- |
| `https://updates.example.test/products/client/2026/release-notes` | `https://<REDACTED:SECRET>` | Confirmed ordinary URL over-redaction |
| `https://example.invalid/object/550e8400-e29b-41d4-a716-446655440000/status` | `https://<REDACTED:SECRET>` | Joining the hostname/path defeats the existing standalone UUID exemption |
| `https://example.invalid/path/abcdefghijklmnop/QRSTUVWX12345678/status` | `https://<REDACTED:SECRET>` | A component scanner left this opaque split value intact; a new blind spot |

The prototype preserved the first two inputs by scanning host labels, path
segments, query names/values and fragment components separately. It also
exercised explicit assignments, credentials in userinfo, percent encoding,
JSON escapes, writer boundaries, and keys pasted into descriptions.

Independent review found that splitting a previously contiguous entropy
candidate could make each piece fall below the scanner's length threshold.
Joining short opaque pieces again protected selected cases, but reintroduced
false positives for ordinary deployment/version components and still missed
pure-letter or pure-digit pieces. Trailing punctuation exposed another case
where only part of an opaque value was hidden. These were compared against an
actual build of the baseline scanner, rather than inferred from the regex alone.

Tokens distributed across separate query parameters remain an existing
heuristic limit: the baseline pattern already treats query delimiters as
boundaries. That limit is distinct from the new dot/slash blind spots introduced
by the rejected prototype.

## Follow-up boundary

Keep the existing field allow-list and value scanners. Upstream masking does
not establish that descriptions or other rendered fields cannot contain pasted
credentials. A syntactically valid URL is not evidence that it contains no key.

A future change should start with representative operator-provided structures
using fictional values, identify the exact affected catalog fields, and test
ordinary URLs and opaque credentials through the complete projection and writer
path. Prefer field-specific evidence about URL meaning and identity components
to a broad exemption for all URL-shaped prose. Any accepted refinement needs
both preservation tests and differential no-leak tests against the prior scanner;
matching a few selected benign examples is insufficient evidence of safety.

The investigation does not claim that the baseline detects every unlabeled
secret. Its existing heuristic limits remain documented in
[DATA_CLASSIFICATION.md](DATA_CLASSIFICATION.md). The decision here is to avoid
shipping additional demonstrated blind spots while addressing the broader
saved-investigation and discovery workflow.
