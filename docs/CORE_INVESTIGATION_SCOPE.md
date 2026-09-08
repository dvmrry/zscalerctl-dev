# Core Configuration Investigation — Scope and Experiments

Status: first increment authorized for implementation on 2026-09-08.
Baseline: `2b350c94680b160d4059af886de1fdce147d6887`, after PR #131.
Working branch: `feature/config-investigation-foundation`.

## Product objective

Make a configuration investigation repeatable: acquire sanitized configuration,
ask several precise questions of it, and retain enough evidence to distinguish
an observed answer from unavailable information. Preserve the tenant-read-only
boundary and the existing live CLI and machine contracts.

The first increment delivers saved-collection reads, a small semantic-catalog
pilot, and an agent-task evaluation foundation. It also investigates reported
URL over-redaction. These are bounded improvements to the common core and CLI;
public Go API promotion, policy simulation, and new frontends are later decisions.

## First increment

| Workstream | Deliverable | Acceptance evidence |
| --- | --- | --- |
| Saved collection core | Immutable admitted collection loaded from an existing dump; list/get/show through projected-loader interfaces | Repeated reads without config, credentials, providers, SDK clients, network, or writes; malformed/incomplete artifacts fail closed; returned values cannot mutate collection state |
| Saved collection CLI | `--from-dump DIR` on ordinary resource list/get/show with existing filtering, fields, formats, and bounded list output | Real CLI tests for list/get/show/filter/page/NDJSON, invalid scope before filesystem access, poisoned live config never loaded, exact stored-mode behavior |
| Catalog semantics pilot | Explicit reviewed descriptions for selected fields of ZIA locations, URL filtering rules, and rule labels; targeted config-free discovery | Every described field and reference resolves to the catalog; unreviewed semantics remain unknown; no expansion of rendered fields or sensitivity permissions |
| Agent-task evaluation | Synthetic tasks, trace/result contract, evaluator and positive/negative examples | Known-ID task, unknown field, unavailable data, large list, local repeated reads, and failed collection scenarios; no model run claimed without an actual submitted run |
| URL redaction investigation | Reproducible diagnosis and reviewed prototype outcome; runtime correction deferred | Baseline and prototype compared on benign and opaque URL structures; retain existing sanitization after new blind spots were demonstrated |

## Saved collection contract

The existing dump artifact is the input. Reuse its strict admission machinery:
metadata/file reconciliation, path containment, regular-file and size checks,
record counts, current catalog compatibility, and projected/redacted-record
validation. Do not implement an unrelated loose JSON importer or convert an
untrusted input map directly into an output-safe record.

The first version requires a complete artifact. A missing resource, a failed
collection, an unsupported operation, and an empty successfully collected
resource are different outcomes. Get-by-ID must reject ambiguity and return
not-found for an absent ID; list order remains the admitted collection order.

The stored redaction mode is the default. An explicit different mode is rejected
in this increment. Saved data cannot recover a dropped field, and a caller must
not mistake a request for weaker redaction as permission to widen the artifact.
The live credential/config path is never entered for saved reads. Validation of
command scope, arguments, and requested fields precedes artifact access.

The CLI retains existing output shapes. `--limit` and `--offset` remain output
views over a completely loaded and filtered collection. The source flag has an
explicit local-read effect; output files remain separate intentional writes.
A library collection owns a stable in-memory view after loading. Separate CLI
calls reopen and validate the selected directory, so external modification can
change later answers. This is not an atomic upstream tenant snapshot.

Admission limits total serialized resource data to 256 MiB; decoded objects and
projection copies can use more memory. Cancellation is checked around loading
and projection, but an individual decode or projection step can delay its
observation. This increment does not promise a hard memory or execution-time
budget.

Keep current dump schema compatibility. The source manifest retains tool version
and collection time. The collection API returns only validated schema, mode,
status and counts; it does not promote free-form manifest strings into trusted
provenance. A future artifact version can add acquisition intervals, SDK/catalog identity and content
fingerprints after their exact compatibility semantics are reviewed. Do not
fabricate metadata absent from an older artifact.

## Semantic catalog pilot

Start with a handful of fields supported by concrete catalog and SDK evidence.
Candidate metadata can describe JSON value shape, textual meaning, identity,
known relationships and collection ordering. Unknown ordering, undocumented enum
sets, and null/omission semantics stay explicitly unspecified. SDK zero values
are not evidence that the upstream response supplied a field.

Expose the pilot separately from the frozen broad schema/machine manifest.
Targeted CLI discovery may present a versioned description of one resource;
unreviewed resources must be identifiable as outside the pilot. No new filter
operators or implicit value coercions are part of this increment.

## URL redaction intent

The operator reports over-redaction of URLs, but does not yet have a reproducible
example or know whether the affected portion is the host, path, or query. The
original protection goal includes keys pasted into descriptions and exposed
configuration fields. Some upstream responses may already hide secrets; that
observation does not establish a universal upstream guarantee.

Investigate field exclusion, structured-value scanning, free-text scanning and
final-byte scanning separately. A syntactically valid URL is not proof that it
contains no secret. Preserve secret-field exclusion and protection for explicit
key assignments, bearer tokens, JWTs, PEM material, userinfo and credential-like
URL components. Do not whitelist all URL-shaped strings or depend on Zscaler
having performed redaction first. If no narrow safe behavior change is supported,
ship the diagnostic corpus and report the remaining uncertainty.

## Agent-task evaluation

Use synthetic fixtures only. Evaluate observable correctness, source choice,
unnecessary live reads, needless list calls before a known-ID get, and excessive
record exposure. Record incomplete or unavailable evidence as such. A scripted
fixture run proves the evaluator and CLI contract; it does not measure an
autonomous agent. Initial real-agent baseline is explicitly unmeasured.

Keep task fixtures, expected answers, trace validation and scoring independently
inspectable. No automatic model calls or external uploads are required. A future
operator-run comparison can evaluate discovery alone versus discovery plus the
matching skill against the same tasks.

## Follow-on exploration sequence

1. Extend reviewed catalog semantics only where the pilot supports useful
   queries; add declared reference navigation with resolved/unresolved/not-
   observable results, preserving redaction boundaries.
2. Add opt-in semantic diff policies for confirmed set-like collections,
   reference renames and catalog-version visibility changes. Preserve ordered
   policy rules and the exact underlying field changes.
3. Add evidence-backed configuration checks with pass/finding/unknown/not-
   applicable outcomes and explicit data prerequisites. Operator policy decides
   whether a broad configuration is intentional.
4. Unify config-free request preparation across core entry points, then add
   whole-operation deadlines and work budgets distinct from output limits.
5. Prove a small SDK-independent library facade with an external consumer before
   freezing a public Go package or transport expansion.

## Experiments and stop criteria

| Experiment | Useful evidence | Stop or defer when |
| --- | --- | --- |
| Policy explanation | Relevant rules plus a trace of observed criteria and missing inputs | Missing enforcement context makes most conclusions speculative |
| Offline what-if | Re-evaluate explicit relationships/checks over a hypothetical local copy | Users mistake it for an authoritative traffic decision simulator |
| Incremental API output | Measurable time-to-first-result or memory benefit with explicit terminal semantics | Consumers cannot reliably distinguish partial output from complete evidence |
| Parallel acquisition | Lower wall time without increased throttling or incomplete collections | Retry/rate-limit behavior erases the benefit |
| SQL/database index | Repeated operator queries materially simpler or faster | Small in-memory indexes cover the proven workflows |

## Delivery and review

Builders use isolated file ownership in the shared worktree. No live keys or
payloads are required for implementation. Run focused tests during development,
then the repository aggregate checks, generated-contract checks and skill sync.
No unmeasured tenant/agent behavior is reported as verified.

Before completion, provide a builder handoff and independent fresh-context Luna
max adversarial reviews for artifact admission/core behavior and CLI/catalog/
redaction/contracts. Fix concrete findings, request narrow independent rechecks,
record the actual verdicts, and pass `make verify-adversarial-review` and
`make check`. Promotion and release remain separate from this implementation.

## URL investigation outcome

The global HTTP(S) component-scanning prototype is deferred. It reproduced and
reduced ordinary URL over-redaction but introduced new blind spots for opaque
credentials split across components. The final increment keeps runtime
redaction identical to the base commit. The diagnosis, adversarial examples,
and a more targeted follow-up are recorded in
[URL_REDACTION_STUDY.md](URL_REDACTION_STUDY.md). Preserving the existing safety
boundary takes precedence over shipping a heuristic correction that introduces
demonstrated blind spots.
