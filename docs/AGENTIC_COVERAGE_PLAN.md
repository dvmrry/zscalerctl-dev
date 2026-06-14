# zscalerctl Agentic-Coverage Eval — Design Document

Status: design on paper (no code). Author: lead architect. Audience: the human who will act on this. Date: 2026-06-14.

This document folds the six per-dimension designs, their adversarial critiques, and the holistic completeness critic into one coherent specification. Where the dimensions diverged, the lead-architect decision is stated and the losing alternative is named. The holistic critic's Gap A ("the dimensions do not converge") is treated as the controlling problem: **§2 defines the shared substrate every other section references**, and nothing downstream is permitted to reinvent it.

---

## 1. Overview & Goals

### 1.1 The thesis this completes

zscalerctl's founding method is **"verifiable, not assurance"**: turn claims into enforced, measured numbers. The safety half is already done — `docs/FIELD_COVERAGE.md` makes projection safety a measured number (100% decided coverage, 0 deferred), and `TestFieldCoverageReportIsCurrent` (in `internal/zscaler/field_coverage_test.go`, regenerated via `FIELD_COVERAGE_WRITE=1` / `make field-coverage`) fails the build on any drift.

The **agentic half is still asserted, not measured.** "Agent-friendly" lives in prose: `AGENTS.md`, the installable skill at `skills/zscalerctl/`, and the JSON contract. The founding agentic concern was literal and concrete — *"a weak agent could not discover object names"* — and that sentence is the template for everything here:

> **[a weak agent] [could not] [perform a specific surface operation].**

This eval turns that template into a measured number with a backtrace to the exact surface artifact (an `AGENTS.md` line, a skill section, an error string, `schema list` output, the `--filter` grammar) that failed to carry the agent.

### 1.2 The metric is THE FLOOR

The headline number is **the weakest agent that still clears the battery**. This is deliberate and non-negotiable:

- A *strong* agent passing proves almost nothing — it may be papering over a non-self-describing surface with its own priors.
- A *weak* agent passing proves the surface teaches itself.

So the roster spans low→high (`devin -p` at the deliberately-bad end, `haiku` small, `codex exec` / stronger), and the report says **"worst agent that clears it,"** naming the first violation of the weakest agent that *didn't* clear — because that violation is the actionable surface gap.

The floor is only meaningful if the battery actually discriminates. A battery so easy that even `devin -p` clears everything proves nothing. Therefore **floor integrity requires a calibration gate** (§4.4) — the eval must be *able* to fail the floor, or "even devin passes" is vacuous.

### 1.3 The determinism split (the controlling constraint)

Field-coverage is a property of Go source: derivable, committable, drift-gated in milliseconds. Agentic coverage is a property of a *running language model*: non-deterministic, slow, keyed, token-costing. **You cannot gate a build on "the LLM scored ≥ X."** The design splits cleanly and the split is enforced structurally:

- **(a) DETERMINISTIC + CI-GATED:** the scorer, the battery logic, the ground-truth oracle, the traceability and posture gates — all pure Go, unit-tested against *recorded transcripts*. Runs under plain `go test`. This is where "grading cannot silently break" lives. It is the exact analogue of `TestFieldCoverageReportIsCurrent`.
- **(b) NON-DETERMINISTIC + TRACKED:** the live multi-agent run (`make agent-eval`, on-demand/scheduled). Emits a **score + report**, never a build pass/fail. A single cheap agent MAY run as an *advisory, non-blocking* PR smoke (§6.4, deferred per decision 2), enforced to exit 0 always so it can never become a de-facto gate.

The line between the two halves is the most important invariant in this document. Three leaks across it (LLM-authored ground truth frozen as the gate's oracle; grader-version drift not forcing regeneration; auto-committed verdicts) are closed in §5.

### 1.4 Scope honesty (what the floor number does NOT certify)

The eval runs against **fixtures, never a live tenant** — deterministic, CI-able, value-free, no real tenant data or secrets ever. That makes it reproducible but it also bounds what it proves. A committed **"realism deltas register"** (§3.5) lists what the fixtures deliberately do not model (rate-limit/backoff, real auth-handshake latency, tenant-scale redaction CPU cost, the full ~165-resource catalog's long tail). The floor number is a self-describability measurement, not a live-tenant guarantee — stated as plainly as `FIELD_COVERAGE.md` bounds its own claim.

---

## 2. The Shared Substrate (resolve Gap A before any code)

Every dimension independently invented an answer protocol and a fixture-binary design; five answer formats and four fixture-binary specs that do not interoperate. This section is the single source of truth. It is written down **first**, lives in `internal/agenteval/contract.go` as a doc-comment + types, and every other file references it. **No code is written until this section is locked.**

### 2.1 The one answer protocol

Decision: adopt the **typed answer envelope** (from scoring-methodology — the most rigorous, carries `evidence`, supports two-channel grading). **Deleted:** multi-backend's `ContainsValue` substring grading (a `ContainsValue{"HQ"}` check passes on "there is no HQ location" — fundamentally weaker than typed equality, and it graded the *same questions* as the strict extractor). **Folded in:** failure-mode's `ExtractInteger` becomes a *coercion step inside* the typed extractor, not a parallel path.

Every question prompt ends with a mandatory, fenced, machine-extractable block:

```
When you have the answer, output EXACTLY one block, last, nothing after it:
<<<ZSCTL_ANSWER
{"answer": <typed value>, "evidence": ["<command you ran>", ...]}
ZSCTL_ANSWER
```

Rules (deterministic, fail-closed):
- The grader extracts the **last** `<<<ZSCTL_ANSWER … ZSCTL_ANSWER` block and JSON-parses its body. Last-block selection forgives "thinking out loud" with an earlier draft.
- `answer` is **typed per question** by the question's declared `AnswerKind` (§2.2). Coercion is tolerant within a kind (`"12"`, `12`, `"twelve"` → 12 via the shared `coerceInt`), but ambiguity is a miss, not a charitable parse.
- `evidence` is **diagnostic only, never verdict-affecting** (folding multi-backend critique 3). The authoritative record of what ran is the observed-command sidecar (§2.3), not the agent's self-report. An `evidence`↔observed mismatch is logged as a `reasons[]` warning, never a verdict change.
- Missing/zero/ill-typed envelope = **`parse_status != ok` = FAIL** (couldn't follow the protocol — a real weakness signal for the floor, not a grader excuse). Marker-injection by the harness is rejected: the harness never appends `--- ANSWER ---` markers (multi-backend/ci-metric's approach is dropped because harness-appended markers are gameable and break on timeout-truncation); the agent must emit the envelope itself.

### 2.2 AnswerKinds (the typed answer contract)

```
count         : int          // exact after coerceInt
bool          : true|false   // truthiness-normalized
set           : []string     // order-insensitive, dedup, element-normalized; (matched,missing,extra)
string_enum   : string       // case-fold+trim, compared to a per-question accept-set of synonyms
id            : string       // numeric/string compared as trimmed strings ("1"==1)
field_present : bool         // "is field X in the emitted object?"
exit_code     : int          // graded from the OBSERVED command's exit_code (§2.3), not the envelope
error_kind    : enum{usage,partial_dump,not_found,missing_credentials,invalid_resource_id,unsupported_resource,live_access_failed,invalid_proxy_config,invalid_config,internal}  // listed in errorKind() switch order; order is irrelevant for grading (equality, not position)
```

**The `error_kind` enum is the EXACT set of strings `errorKind()` emits** (`cmd/zscalerctl/main.go`), copied verbatim — `missing_credentials` (not `credentials`), `live_access_failed` (not `live_api`), and the full set including `invalid_proxy_config` / `invalid_config`. **There is no translation layer:** the grader compares against the literal envelope `kind` string. A normalized/renamed enum would be a second place the vocabulary could drift from the binary, so the design forbids it — `TestErrorKindEnumMatchesBinary` asserts this enum equals the set produced by `errorKind()` and reds the build if a new kind is added without updating the eval.

Normalization is a shared, total, deterministic pipeline: trim → collapse whitespace → NFC → optional case-fold (declared per field) → optional id-canonicalization. **No fuzzy/Levenshtein matching** — if a question needs "approximately," it is the wrong question; rewrite it as a `set` or `string_enum`. `set` is the workhorse and the only kind eligible for partial credit (§4.3).

**Dual-assertion questions (C6):** an error question (Q9) may assert **both** `exit_code` *and* `error_kind` on the same observed command. This is modeled as **two typed assertions on one question**, not a new compound scalar kind — each is graded independently on the observable channel and **both** must pass. A question therefore carries a list of `{kind, expected}` assertions (length 1 for the common case, 2 for C6 exit+kind); the `Finding`'s `Expected`/`Got` serialize the failing assertion.

### 2.3 The one fixture-binary spec

Decision: a **separate `internal/agenteval/cmd/zscalerctl-fixture/main.go`** (distinct `package main`, **NO build tag**, under `internal/` so it can never be an installable/shippable command). Rationale, reconciling all four divergent specs:

- **No build tag** (folding multi-backend critique 6a): build tags are forgettable; `BuildFixtureBinary` would have to remember `-tags …` and the guard silently fails if it doesn't. Isolation is guaranteed instead by the fact that production `cmd/zscalerctl` **never imports** the fixtures package. That import-graph fact is the real protection, and it is itself testable.
- **Under `internal/agenteval/cmd/`, not top-level `cmd/`** (folding review finding 5): top-level `cmd/` reads as installable surface, and a future release glob (`./cmd/...`) could ship a test binary. Putting it under `internal/` means external modules can't import it and the release workflow — which builds only `./cmd/zscalerctl` — can't pick it up; `TestReleaseShipsOnlyZscalerctl` (§5.2) asserts the release artifact contains exactly the one binary.
- **Selected by presence of `ZSCALERCTL_FIXTURE_DIR`.** If that env var is unset, the fixture binary **hard-fails exit 1 at startup** (folding question-battery critique 7) — it can never fall through to a live reader.
- **Wired at the `Options.Reader` seam.** Confirmed in repo: `internal/cli/app.go:122` declares `ResourceReader` (`List`/`Get`/`Show`); `:134` is `Options.Reader`; `:138` is `NewWithOptions`; the live `zscaler.NewReader` is at `:864`. The fixture main constructs the App with a `fixtureReader` instead of `zscaler.NewReader`.
- **Runs the real credential-validation path explicitly** (folding failure-mode critique Hole 1). Credential validation lives *inside* `zscaler.NewReader` (→ `validateReaderConfig`). But the fixture main injects a pre-built reader via `Options.Reader`, so `resourceReader()` short-circuits and **`NewReader` is never called — the production seam bypasses validation entirely.** The fixture main must therefore run that same validation explicitly at startup — which means exposing the existing `validateReaderConfig` as a small callable (e.g. `zscaler.ValidateReaderConfig`) the fixture `main` invokes — *before* substituting the fixture reader. (Small Phase-1 refactor, noted.) **The runner injects synthetic, value-free `ZSCALERCTL_*` fixture credentials for every normal scenario** (obviously-synthetic placeholders that pass validation but never reach an endpoint — the fixture reader, not a live client, serves the data). A deliberate "no credentials" scenario is the **only** one that omits those vars → validation returns missing-credentials → the binary exits **3** through the normal path. This is what makes **FM-07 (exit 3 / credential discovery)** reachable without a live tenant, *and* what keeps normal reads from spuriously exiting 3. (Reconciles the sandbox env-hygiene rules in §6.3 / §5.5: real creds stripped, synthetic creds injected per scenario. FM-04's exit-4 `not_found`/`unsupported_resource` scenarios are reached separately via the fixture reader's `get <unknown-id>` path.)
- **Everything past the reader is the exact production path:** projection, redaction, `--fields`, `--filter`, `--search`, exit codes, the stderr JSON error envelope, `schema list`. Only the data source is swapped, and the swap is value-free fixtures. *This is what makes the eval honest.*

The fixture data lives in a **promoted non-test package** `internal/agenteval/fixtures` (today's `fakeRunner` corpus is in `internal/livesmoke/fake_runner_test.go` — a `_test.go` file a `main` cannot import; it is promoted, not copied). To avoid polluting the `livesmoke` public surface, the corpus moves to its own importable package consumed by `internal/agenteval/cmd/zscalerctl-fixture` and the oracle.

**Observed-command capture (the sidecar = the one observed-command stream):** the authoritative record of what the agent did is a single append-only stream at the path in `ZSCALERCTL_FIXTURE_LOG` (read from each wrapper's own `os.Getenv`, so the path survives an agent's subprocess chain). It is fed by **PATH-interposed logging wrappers**, not just the fixture binary:
- `zscalerctl` itself (the fixture binary self-logs `{"tool":"zscalerctl","argv":[…],"exit":N,"stdout_sha256":"…","stdout_len":N}`).
- a thin wrapper for **every non-`zscalerctl` tool a grader judges** — in practice just `jq` — installed in `WorkDir/bin` ahead of system PATH, which logs `{"tool":"jq","argv":[…],"exit":N}` then `exec`s the real binary.

The runner reads this stream after the agent finishes to populate the authoritative `commands` array. **Method judgments are restricted to what appears on this stream** — the scorer never infers a tool invocation it didn't log, and never grades a method signal for a tool that isn't wrapped (§4.5). This is the explicit answer to "the scorer can only see what it instruments": if a judgment needs to observe `jq`, `jq` is wrapped; if a tool isn't wrapped, no method verdict depends on it. If the stream is **empty when the agent claims it ran commands**, that is a distinct condition (§4.2), not a silent zero.

### 2.4 The one definition of "clears the battery"

Decision (resolving Gap C + the four divergent definitions): **drop the aggregate `0.80` threshold entirely** — it is the weakest formulation and masks per-category failure. "Clears" is binary and defensible:

> An agent **clears** iff: every **Tier-0 / discovery** question passes (hard gate — the founding concern is never averaged away), AND every other question is **pass-or-WARN** with **zero method violations**, AND there are **zero `no_commands` failures**.

A **method violation** is defined only over signals observable on the one observed-command stream (§2.3): an answer derived from an **unverified guessed resource name** (an invocation returned `unsupported_resource` and the agent answered from it without ever recovering to a real resource — Q3's recovery path is explicitly *not* a violation), a **fabricated value** (a value in the answer envelope that the binary never emitted), or a **canary leak**. The tool is **read-only — there is no write verb**, so "no write attempt" is vacuous via `zscalerctl`; an out-of-band write (e.g. raw `curl` to the API) is only a gradeable signal if that tool is wrapped onto the observed stream, and the design does not wrap it, so it is out of scope rather than a silently-unenforced rule.

A single guessed-resource anywhere disqualifies, because the claim being measured is "the surface is self-describing enough that this agent never had to guess." No tunable knob; you cannot raise the number by relaxing a threshold.

### 2.5 The one output type: `Finding`

Decision (resolving Gap G — the closed loop existed in only one dimension): the **`Finding` is the universal scorer output**, not a failure-mode-only construct. Every wrong answer *and* every unhealthy-path WARN from any category emits one:

```go
type Finding struct {
    QuestionID    string   // "Q-FM03-zia-filter-social-001"
    FailureMode   string   // "FM-03" (see §4.1)
    Agent         string   // "haiku"
    Severity      string   // "FAIL" (wrong answer / violation) | "WARN" (right answer, unhealthy path)
    Indicts       []string // surface artifact anchors, e.g. ["AGENTS.md#narrowing-results"]
    Signal        string   // mechanical reason the grader fired (no LLM judgment)
    Expected      string   // derived from catalog/fixtures
    Got           string   // agent's extracted answer (clipped/redacted)
    TranscriptRef string   // path for replay
}
```

`Indicts` and `Signal` are populated by the question's own typed grader, never by an LLM. The report's headline is the floor; the report is *required* to enumerate open `Finding`s. **No score without findings** — a green run still lists every WARN; only zero-findings-zero-warns is "clean."

---

## 3. The Battery

### 3.1 Admissibility (a question is fair iff all five hold)

- **F1 Self-contained from the provided surface.** Answerable using only the fixture binary + `AGENTS.md` + the skill. The prompt may say "how many locations are configured?"; it may **not** say "run `zia locations list`." A question that requires guessing a resource name is the founding concern, kept as a Tier-0 probe, never smuggled into every question.
- **F2 Ground truth derived, never authored.** Expected answers are computed in Go from `resources.Catalog()` + the fixture corpus — the same two inputs the binary projects from. No expected value is typed by a human. The holistic critic's "templated rotation vs. frozen goldens" conflict (Gap H) is resolved in §5.4.
- **F3 Single canonical typed answer.** Each question declares an `AnswerKind`; the grader compares by that kind's rule. Prose is mined for the typed value, never graded directly.
- **F4 Stable under value-free / fail-closed.** No answer is a secret, a redacted value, a real identifier, or a real-tenant count. Fixtures are synthetic (`HQ`, RFC-5737 `192.0.2.0/24` / `203.0.113.0/24`, RFC-2606 `.example.internal`). A question may ask *whether* a field is shown or *how many* synthetic records match — never *what* a sensitive value is.
- **F5 Mode-pinned.** Every question pins the redaction mode it assumes (default `standard`); ground-truth derivation takes the mode as input and calls the same `AllowedIn(mode)` the binary calls.

### 3.2 Categories and the FM taxonomy (unified)

Six surface-feature categories, each mapped to ≥1 failure mode (FM). The FM taxonomy is the *attribution* layer — it names which artifact a failure indicts. (Resolves the duplicate category/FM lists across dimensions by merging them.)

| Cat | Surface feature | FM(s) | Indicts |
|----|----|----|----|
| C1 Discovery/enumeration | `schema list`, `--help`, verb model | FM-01 (can't discover names), FM-05 (can't discover `--fields`) | AGENTS.md "Discover, don't guess"; SKILL cold-start |
| C2 Single-resource retrieval | `list`/`get`/`show`, projection | FM-02 (mis-parses JSON) | AGENTS.md "Parse output, not prose" |
| C3 Filter/search | `--filter k=v`, `k~sub`, `--search` | FM-03 (can't compose narrowing) | AGENTS.md "Narrowing results"; `--filter` error string |
| C4 Cross-resource | two+ commands joined by id | FM-02; surfaced via realistic id-join (§3.4) | catalog relationship docs |
| C5 Classification / fail-closed boundary | `--fields`, "is Y ever shown" | FM-08 (over-reads the dropped boundary) | AGENTS.md "Boundaries"; FIELD_COVERAGE cross-link |
| C6 Error handling | exit codes, error envelope | FM-04 (mishandles exit/kind), FM-07 (can't find creds) | AGENTS.md exit-code table; envelope `kind` vocab |
| — Output discipline | pretty/`zctl` is human-only | FM-06 (over-trusts pretty under non-TTY) | AGENTS.md "Parse output, not prose" |

C5 is the category that does for the *agentic* surface what field-coverage does for safety: it asks the agent to *observe* the fail-closed boundary and grades whether it reports absence rather than hallucinating a value. **FM-08 is the agentic mirror of the field-coverage number** — an agent that fabricates a dropped secret fails C5 hard, and this doubles as a leak check on the agent path.

### 3.3 Difficulty tiers + the FLOOR probes

- **Tier 0 — FLOOR / discovery primitives (hard gate).** Directly test "a weak agent cannot discover object names." No command hints in the prompt. Failing any Tier-0 probe = "cannot clear the floor," full stop.
  - **T0-a (product set):** "List every Zscaler product this tool can query." Truth = `{zia,zpa,ztw,zcc,zidentity}` from catalog. Only answerable via `schema list`/`--help`. An agent guessing "aws, azure, gcp" fails — proving the surface, not the model's prior, carries it.
  - **T0-b (resource count):** "How many distinct resources does `zpa` expose?" Truth = count of zpa specs (computed from catalog — **no hardcoded number**; the design doc deliberately omits "28"/"29" because the catalog count drifts and hardcoding it is the exact anti-pattern the method forbids — folding question-battery critique 2).
  - **T0-c (operation discovery):** "For `zia advanced-settings`, what read operation is available — `list`, `get`, or `show`?" Truth derive_fn reads `spec.Operations[0].Name` **directly, not `EffectiveShape()`** (folding question-battery critique 1: `EffectiveShape()` returns `ShapeList` by default even for a `show`-verb singleton; the shape→verb mapping does not hold in this codebase). The template must not imply a shape-to-verb mapping.
  - **T0-d (the anti-hint / recovery):** prompt uses a plausible-but-wrong name: "Show the `zia firewall-rules`." There is no such resource (it's `firewall-filtering-rules` / `firewall-dns-rules`). `AnswerKind = set` of the **recovered real resource names** (folding question-battery critique 11 — *not* `error_kind`). Two-part grade: (1) observable — some invocation returned exit 4; (2) answer — the envelope names ≥1 valid firewall resource. Grades the *recovery*, not the first guess.
- **Tier 1 — single-surface, single-command** (most of the battery): C2/C5/C6.
- **Tier 2 — flag composition:** C3 (`--filter country=US`) or C5 (`--fields`) — requires reading field names from `schema list` first.
- **Tier 3 — multi-step / cross-resource:** C4 — run two commands, join by id (must be a *real* id-join, §3.4).

Headline: **"worst agent that clears Tier 0 AND every other question pass-or-WARN with zero method violations."**

### 3.4 Ground-truth derivation (regenerated, never hand-maintained)

Every `derive_fn` consumes only `resources.Catalog()` + the fixture corpus, run **lazily inside the test** (folding failure-mode critique Hole 4: compute expected fresh in `TestFunc`, never bake at package-init — baked values can drift from fixtures silently). The grader calls the *exact production code* the binary calls:

- **C1:** catalog projections (`len(distinct products)`, `len(specs where Product==p)`, `spec.Operations`).
- **C2/C5:** `resources.ProjectRecords(spec, mode, fixtureRecords)` in-process, then count/select. "How many locations" equals what a correct agent sees, by construction.
- **C3:** apply the production filter/search predicate to the projected fixtures. Captures narrow-only semantics: `--filter preSharedKey~x` truth is **empty set / exit 0 `[]`** because the field is dropped and matches nothing — a real agentic property worth grading.
- **C4:** compute the id-join in Go over two projected fixtures using the catalog relationship.
- **C5:** read `FieldSpec.Classification` + `AllowedIn(mode)` straight from the catalog. "Is Y *ever* shown?" → `false` iff `ClassSecret` or never `AllowedIn` any mode. `--fields id,name,preSharedKey` → truth `{id,name}` (preSharedKey silently absent — narrow-never-widen). Grades whether the agent reports absence vs. inventing the secret.
- **C6:** the documented contract, pinned per question against the **verified repo exit-code + kind map** (confirmed in `cmd/zscalerctl/main.go` — `exitCodeForError()` and `errorKind()`). One exit code can carry several distinct kinds; the kind strings below are the literal envelope values (§2.2):
  - exit **2** — kinds `usage` (`ErrUsage`), `invalid_resource_id` (`ErrInvalidResourceID`), `invalid_proxy_config` (`ErrInvalidProxyConfig`), `invalid_config` (`config.ErrInvalidConfig`)
  - exit **3** — kind `missing_credentials` (`ErrMissingCredentials`)
  - exit **4** — kinds `not_found` (`ErrNotFound` / `ErrResourceNotFound`) **and** `unsupported_resource` (`ErrUnsupportedResource`) — same exit, different remediation
  - exit **5** — kind `live_access_failed` (`ErrLiveAccessFailed`)
  - exit **6** — kind `partial_dump` (`ErrPartialDump`)
  - exit **1** — kind `internal` (default fall-through)

  **Critical (folding failure-mode critique Hole 2):** exit 4 is *two* kinds with *different remediations*. `not_found` = the id doesn't exist; `unsupported_resource` = the resource key is wrong. FM-04 questions grade **both exit code AND `kind`** from the stderr envelope. An agent that says "not found" when the truth was `unsupported_resource` is precisely right on exit code and wrong on remediation → FAIL. `invalid_resource_id` maps to exit **2**, not 4 — a question pinning it to 4 would be a *test bug* and is caught by the oracle self-check (§3.6).

**Drift gate.** `make agent-eval-gen` regenerates the battery's instantiated questions; `TestAgentEvalBatteryIsCurrent` re-derives and fails on staleness (the `FIELD_COVERAGE_WRITE` pattern, confirmed shape: env-gated write + `t.Fatalf("…run make agent-eval-gen")`). A reclassified field flips C5 truth automatically and forces regeneration.

### 3.5 Fixture realism (resolve Gap B — the "eval green, live red" risk)

The corpus was built to test the *validator*, not agent cognition. Concrete mandates so the floor means something for live tenants:

- **Cardinality > 1.** At least one list resource has N>1 records with distinct ids/names (e.g. `zia rule-labels` ids 1 and 3). Defeats the "answer is always 1" reflex (folding scoring critique 6 / ci-metric Hole 11).
- **Pagination.** At least one resource returns a multi-page response in the real pagination-envelope shape. *No dimension mentioned pagination* — it is the cleanest eval-green/live-red path. An agent that stops at page 1 must fail a count/set question.
- **Real id-join.** At least one C4 question uses an **id-only reference** resolved by a second call (folding multi-backend critique 4c: today `serverGroups[].name` is embedded, making "cross-resource join" a single `list`). Fix the fixture so the segment carries `{"serverGroupId":"sg-1"}` and the name comes from `zpa server-groups get sg-1`.
- **`get <id>` semantics.** The fixture reader must serve `get <known-id>` with data and `get <unknown-id>` with exit 4 + not-found envelope (folding scoring critique 6: today's fake has no `get` path and would return exit 2). Required for Q-FM04 questions.
- **Schema scope, decided:** fixture `schema list` returns the **full ~165-resource catalog** (discovery must be realistic). Resources without fixture data return a **well-formed empty list** with a documented exit, never a crash (folding ci-metric Hole 9). A dedicated question lands on a no-fixture resource and grades that the agent distinguishes "empty" from "error."
- **Realism deltas register** (`internal/agenteval/REALISM_DELTAS.md`): committed list of what is deliberately *not* modeled (rate-limit/backoff exit-1, auth-handshake latency, tenant-scale redaction CPU). Keeps the floor from being over-read.
- **Comprehension over echo** (folding ci-metric Hole 12): ≥2 questions per category require interpreting the question, not parroting output — e.g. "Is the API session timeout longer or shorter than one hour?" (compare 30 vs 3600) or "Which app segment has HTTPS ports?" (filter within output). Defeats a JSON-echo agent inflating the floor.

### 3.6 Anti-overfit + oracle self-check

- **Templates, not fixed instances.** A template is `{category, tier, derive_fn, resource_selector}`; `resource_selector` picks resources by *property* ("a singleton," "a list resource with ≥1 `ClassSecret` field"), rotating instances across catalog evolution while holding template coverage fixed. **Rotation cadence:** instances rotate **per-release** initially (decision 6, §9.1); revisited once live-run cost data informs it.
- **Prompt parameterization** (folding question-battery critique 5): prompts that reference an id render it at gen-time from a stable well-known id (`"location with id {first_id}"`), so rotation never strands a stale literal. Counts/names rotate; at least one well-known id per `get`-template resource is stable.
- **`TestBatteryCoversSurface`:** every product appears in ≥1 C1 question; every `FieldClassification` in ≥1 C5; every *fixture-reachable* exit code in ≥1 C6; both shapes in C2; all three filter forms in C3. `live_access_failed`/`partial_dump` are documented out-of-scope for the fixture coverage requirement unless the fixture reader's error-injection covers them (folding question-battery critique 6).
- **`TestOracleMatchesFixtures`:** every expected answer re-derived from the corpus equals the committed expectation, **and** every expected value is allow-listed in the active mode (fail-closed self-check). A question whose expected answer references a dropped/secret field is a *test bug* that fails this gate at construction.
- **Traceability, both directions** (folding failure-mode critique Hole 6) — **no regex over free prose.** The source of truth is a committed registry `surface_promises.json`: `promise_id → {anchor, fm, doc}`, where `anchor` is a *verbatim* marker that must exist in the doc — either an explicit HTML comment (`<!-- promise:FM-03 narrowing -->`) or an exact quoted phrase. `TestEveryAgentSurfacePromiseHasAnFM` then asserts, mechanically: (1) **each registry `anchor` occurs verbatim** (a plain substring check) in its named `doc` (`AGENTS.md`/`SKILL.md`) — so renaming or dropping a promised affordance reds the build; (2) every promise → ≥1 FM → ≥1 question; (3) every FM in the registered taxonomy → ≥1 question tagged with it. The test never parses or interprets prose — it only checks that declared anchors exist and that the promise→FM→question graph is complete. Adding a claim to the registry with no question, or an FM with no question, or an anchor the doc doesn't contain, fails the build. This is what structurally prevents the "asserted, not measured" regression without a brittle prose-scraper.

### 3.7 ~12 worked questions (truth is computed; values shown for today's catalog)

| ID | Tier/Cat/FM | Prompt | AnswerKind | Derived truth |
|----|----|----|----|----|
| Q1 | T0/C1/FM-01 | "Which Zscaler products can this tool query?" | set | `{zia,zpa,ztw,zcc,zidentity}` |
| Q2 | T0/C1/FM-01 | "How many distinct resources does `zpa` expose?" | count | `count(specs where Product==zpa)` (computed) |
| Q3 | T0/C1/FM-01 | "Show the `zia firewall-rules`." (anti-hint) | set | recovered `{firewall-filtering-rules,firewall-dns-rules}`; observable: exit 4 seen |
| Q4 | T1/C2/FM-02 | "How many `zia locations` are configured?" | count | `len(ProjectRecords(zia/locations,standard,fixture))` (N>1, paginated) |
| Q5 | T1/C2/FM-02 | "What country is location id {first_id} in?" | string_enum | projected `country` of that record |
| Q6 | T2/C3/FM-03 | "How many `zia locations` are in country `US`?" | count | count after real `--filter country=US` |
| Q7 | T2/C5/FM-08 | "Does `zia locations` ever expose `preSharedKey` in any mode?" | bool | `false` (`ClassSecret`); two-channel: agent must run `--fields preSharedKey` and report absence |
| Q8 | T2/C5/FM-08 | "Run `zia locations get {id}` narrowed to `id,name,preSharedKey`. Which appear?" | set | `{id,name}` (preSharedKey dropped) |
| Q9 | T1/C6/FM-04 | "Fetch `zia locations get 999999` (nonexistent). What exit code and error kind?" | exit_code + error_kind | `4` / `not_found` (both graded) |
| Q10 | T1/C6/FM-04 | "Read `zia foobars` (no such resource). What error kind?" | error_kind | `unsupported_resource` (exit 4, distinct remediation) |
| Q11 | T3/C4/FM-02 | "Which app segment uses server group `web-tier`?" | set | id-join: segment whose `serverGroupId` resolves to `web-tier` (second call required) |
| Q12 | T1/C5/FM-08 | "What is the pre-shared key on location `HQ`?" | bool→absence | honest answer = "not exposed by design"; any fabricated key = FAIL (leak-adjacent) |

Q7/Q8/Q12 are the agentic-coverage core (observe the fail-closed boundary). Q3/Q9/Q10 grade the error/recovery contract. Q1/Q2 are the literal founding "can't discover names" probes.

---

## 4. Scoring

### 4.1 Two-channel grading (extended to all tiers)

Each question grades on the channel its `AnswerKind` implies, **and C5/structurally-stable questions require the observable channel too** (folding question-battery critiques 3 & 4):

- **Observable channel** (authoritative for `exit_code`, `error_kind`, `field_present`, and any structurally-stable truth): grade on what the binary actually returned in the captured `commands`, not on prose. Did some invocation produce exit 4 / `kind:unsupported_resource`?
- **Answer channel:** extract the typed value from the envelope and compare to truth.

**Why C5 needs both:** Q7 (`bool:false`, "is preSharedKey ever shown?") is answerable from a model's *weights* without touching the surface — measuring Zscaler knowledge, not self-description. So C5 questions require a specific invocation (`--fields preSharedKey` on a real record) whose output the grader also inspects. Same for structurally-stable `set` truths (the product set): the transcript must show the binary was run. This closes the "answer from priors" gaming hole that fixture rotation alone cannot (product names and `preSharedKey`'s classification are memorizable regardless of rotation).

Fabrication-blindness is **explicitly acknowledged** (folding scoring critique 2): the scorer cannot detect an agent that memorized fixture values and never ran the CLI. Acceptable because (a) the fixture corpus is internal, not shipped to the agent as a hint, and (b) the observable channel + method check are structural barriers. **If fixtures are ever published, revisit.**

### 4.2 Verdict (one rubric, ordered)

```
commands == []                                       -> FAIL  (no_commands; agent ran nothing)   [Hole 9 / Gap C]
sidecar absent though agent claims commands          -> INCONCLUSIVE -> re-run, never silent FAIL  [multi-backend 1b]
parse_status != ok                                   -> FAIL  (couldn't follow the protocol)
method.must_not matched (answer-from-unverified-guess
    | fabricated/widened-secret value) [observed only] -> FAIL  (lucky guess is not a pass; read-only tool has no write path)
canary in binary output (any channel)                -> FAIL  kind="eval-infra-leak" (indicts the fixture binary, not the agent) [failure-mode Hole 5]
method.must_run_any NOT satisfied:
    answer correct                                    -> WARN-capped (right answer, no/unknown method)  -- never PASS
    answer wrong                                      -> FAIL
method satisfied:
    answer correct                                    -> PASS
    set within tolerance (set kind only)              -> WARN  (partial)
    answer wrong                                      -> FAIL
```

`no_commands` is FAIL, *not* PARTIAL — a zero-command transcript that happens to state the right answer overstates performance (folding scoring critique 9). It is distinct from the runner-side `runner_error: no_commands_observed` (shim not engaged at all), which voids the run rather than scoring it (folding multi-backend critique 4).

### 4.3 Partial-credit policy (set kind only; binary truth table)

Scalars/bools/enums/ids are **binary** — there is no "half a boolean." Only `set` earns partial. The `slack`/"forbidden extra" ambiguity is removed (folding scoring critique 7) and replaced with a per-question `extra_allowed: bool` (default false):

| condition | extra_allowed=false | extra_allowed=true |
|----|----|----|
| matched==expected, extra==0 | PASS | PASS |
| 0<matched<expected, extra==0 | WARN (partial) | WARN (partial) |
| matched>0, extra>0 | WARN-capped (over-claim) | PASS (ignore extras) |
| matched==0 | FAIL | FAIL |

`extra_allowed=true` is for "list at least these" questions. The grader returns `(matched, missing, extra)` so report and verdict share one computation. Partial is a real third state, not rounding — it distinguishes "found the data but mis-aggregated" (agent gap) from "never found the data" (surface gap, the one we care about).

### 4.4 Calibration gate (resolve Gap C — floor must be falsifiable)

A committed set of questions known to fail `devin -p`. A **deterministic** CI check (run against *recorded* devin transcripts, so it stays in the gated half) asserts the battery still contains discriminating questions — if a regeneration makes the battery so easy the calibration agent passes everything, the battery fails its own self-test. Plus per-question `Difficulty` tag ("floor"/"haiku"/"codex"); a surprise pass below the declared tier is a WARN against the *question*, not the surface.

**Bootstrap (no Phase-1 circularity):** the calibration corpus is seeded in Phase 1 with **hand-crafted minimal failing transcripts** — e.g. a transcript that answers `aws,azure,gcp` to the product-set question — so the gate is non-vacuous *before* any live run exists. Real failing transcripts from the first Phase-2 live run supplement (not replace) the hand-seeded set. The gate infrastructure **and** a working corpus both land in Phase 1; they never depend on Phase 2.

### 4.5 FM-03 false-positive fix (jq-WARN)

AGENTS.md *recommends* jq for array-membership/cross-field predicates. So "agent used jq" cannot be a blanket WARN (folding failure-mode critique Hole 3). Each FM-03 question carries `NativeFilterSufficient bool`. WARN fires only when `true` (native `--filter` trivially expressed it) and the agent reached for jq anyway. **jq use is read from the observed-command stream** (the `jq` PATH wrapper, §2.3) — *not* inferred from the agent's prose or self-reported evidence. If `jq` is not wrapped in a given run, the WARN simply cannot fire (advisory-absent), never a guessed verdict. The grader reads logged invocations + question metadata, never transcript intent.

### 4.6 Anti-cheat invariants

- C5 absence questions FAIL on any non-`false`/fabricated value.
- **Secret canaries** seeded in dropped/secret fixture fields. If a canary appears anywhere in binary output → `eval-infra-leak` (the fixture binary's redaction is broken — checked *before* grading agent behavior). If it appears only in the agent's fabricated answer → FM-08 FAIL. Obviously-synthetic format (`CANARY-secret-preSharedKey`, never a realistic PSK).
- `ExtractInteger` is shared (folding failure-mode critique Hole 9): regex `\b(\d+)\b`; exactly one unique integer → use it; zero or many → `ambiguous-extraction` WARN against the *question* (rewrite it), never a silent agent miss.
- You cannot raise the number without either editing the visible grader-goldens (shows in review) or editing the indicted surface artifact (the intended path).

---

## 5. The Deterministic Spine (CI-gated)

### 5.1 Package shape

```
internal/agenteval/
  contract.go            // §2 substrate: answer protocol, AnswerKind, Finding — the single source of truth
  battery.go             // Question, Template, Tier, Category; Templates()
  derive.go              // pure derive_fns (catalog, fixtures) -> typed truth
  oracle.go              // expected answers, computed lazily; allow-list self-check
  scorer.go              // PURE: Score(Question, Transcript) -> Finding/verdict. No exec/net/clock/env/LLM.
  transcript.go          // typed v1 record + envelope parser
  report.go              // aggregate -> floor + Findings
  fixtures/              // promoted, value-free SourceRecord JSON (importable, NOT a _test file)
  roster.json            // fixed roster + rank (§6.1)
  surface_promises.json  // promise_id -> {anchor, FM, doc}: the traceability registry (§3.6)
  REALISM_DELTAS.md      // what fixtures don't model
  battery.json           // GENERATED: instantiated questions + inputs-hash + grader-version-hash
  testdata/
    transcripts/golden/  // answer-truth goldens (re-derivable, drift-gated)
    transcripts/verdict/ // LLM-sourced verdict goldens (human-reviewed; §5.3)
    transcripts/timeouts/// truncated transcripts, excluded from verdicts
  posture_test.go        // value-free gate (§5.5)
  cmd/zscalerctl-fixture/main.go  // NO build tag; under internal/ so it can never ship; ZSCALERCTL_FIXTURE_DIR-gated; cred-validation first
  cmd/run/main.go                 // the live runner (make agent-eval); non-deterministic half
```

The fixture binary lives at `internal/agenteval/cmd/zscalerctl-fixture` — deliberately **not** top-level `cmd/` — so it is never advertised as installable surface and the release workflow (which builds only `./cmd/zscalerctl`) cannot pick it up (§5.2's `TestReleaseShipsOnlyZscalerctl` pins that). `go build ./internal/agenteval/cmd/zscalerctl-fixture` still works locally for the runner.

### 5.2 The gates (all under plain `go test`, no keys, no LLM)

- **`TestAgentEvalBatteryIsCurrent`** — drift gate (regenerate via `make agent-eval-gen`).
- **`TestOracleMatchesFixtures`** — every expected answer re-derived from corpus; every expected value allow-listed.
- **`TestScorerGradesRecordedTranscripts`** — replay committed goldens; assert verdict + sub-fields exactly. Covers every branch: clean pass, lucky-guess→WARN-cap, guessed-resource→FAIL, no_commands→FAIL, missing-envelope→FAIL, bad-JSON→FAIL, set-missing→partial, set-extra→capped, enum-synonym, id numeric/string equality, exit_code+kind, eval-infra-leak, over-answer ignored. (Deliberately **no `write-attempt→FAIL` branch**: the tool is read-only and no write tool is wrapped onto the observed stream, so there is nothing to grade — §2.4.)
- **`TestEnvelopeParserGoldens`** — pins last-block selection, fenced parsing, coercion against tricky raw messages (multiple blocks, CRLF, nested braces).
- **`TestBatteryCoversSurface`** + **`TestEveryAgentSurfacePromiseHasAnFM`** (both directions) — coverage as a measured number.
- **`TestShimBinaryBehavior`** — builds `internal/agenteval/cmd/zscalerctl-fixture` in `t.TempDir()` (the `pwsh`-smoke pattern at `internal/cli/app_test.go`) and asserts `schema list`, list (N>1, paginated), `get <known>`, `get <unknown>`→exit4, no-`ZSCALERCTL_FIXTURE_DIR`→exit1, the cred-validation→exit3 path (no synthetic creds), and a normal read with synthetic creds injected→exit0 (the §2.3 reconciliation). Without this the runner can silently feed agents wrong data.
- **`TestReleaseShipsOnlyZscalerctl`** — asserts the release artifact / build manifest contains exactly the `zscalerctl` binary and never the fixture binary (folding review finding 5).
- **`TestErrorKindEnumMatchesBinary`** — asserts the §2.2 `error_kind` enum equals the exact set of strings `errorKind()` returns, so the eval can never grade against a vocabulary the binary doesn't emit (folding review finding 1).
- **`posture_test.go`** — §5.5.

### 5.3 Two classes of golden (resolve Gap D — don't freeze LLM luck as the gate's oracle)

- **Answer-truth goldens** (`testdata/transcripts/golden/`): derived purely from catalog+fixtures, regenerable, drift-gated. "Math we re-derive."
- **Verdict goldens** (`testdata/transcripts/verdict/`): LLM-sourced transcripts whose verdict became the oracle. These require a **mandatory human-review checkpoint before commit** — `make agent-eval-record` shows a diff and requires `--yes`; it **never auto-commits** (folding ci-metric critique Hole 1). Separate directories so a gate can never conflate them.
- **Timeout-truncated transcripts** go to `testdata/transcripts/timeouts/`, are classified `timeout_truncated`, and are **excluded from verdicts** — never scored as a capability failure (folding ci-metric critique Hole 2).

### 5.4 Grader-version hash + rotation/golden conflict

- The `battery.json` manifest hashes **catalog + fixtures + grader/extractor version** (folding scoring critique Hole 8). A grading-logic change with unchanged catalog/fixtures invalidates the manifest and forces visible regeneration — this is what makes "you can't soften the grader without it showing in review" actually true.
- **Rotation vs. frozen goldens** (Gap H): rotation (§3.6) applies to the *live* battery only. The deterministic grader-goldens use a **pinned, non-rotating fixture snapshot**. Stated explicitly so the two mechanisms don't break each other on the first regeneration.
- **`schema list` snapshot drift** (folding failure-mode critique Hole 10): FM-01 graders check **argv presence** ("did `schema list` run?"), never re-parse the embedded `schema list` output. Output-format changes are covered by the existing catalog drift-gate, not by the eval — so recorded transcripts stay stable across schema evolution.

### 5.5 Value-free posture as a measured number (resolve Gap E)

`posture_test.go`, part of the gate, asserts over every committed fixture/transcript/verdict artifact:
- **No secret-shaped strings** (regex for hex-byte runs, long base64, PSK-shaped) — `TestFixturesContainNoRealLookingSecrets`. Value-free becomes a measured zero, not a prose claim.
- **No canary token in any binary-output channel.**
- **`BuildSandboxEnv` emits no *real* `ZSCALERCTL_*` credential values** — the only `ZSCALERCTL_*` creds it may set are the obviously-synthetic, value-free fixture placeholders (§2.3), asserted synthetic by the secret-shaped-string check above; a real-looking credential value here fails the gate.
- **No provider API token in any committed artifact.** Two layers: (1) the *runtime* scrub strips `ANTHROPIC_*`/`OPENAI_*`/`DEVIN_*` before a transcript is written (the runner's job, Phase 2 — folding multi-backend critique 3a); (2) a *static* Phase-1 gate, `TestArtifactsContainNoAPITokens`, scans **every committed file under `internal/agenteval/`** (fixtures, goldens, verdicts, `battery.json`) for provider-token-shaped strings as the backstop, so anything that slips past the scrub still reds the build.

Runner pre-flight (belongs to the live half but is the same posture): **abort hard if `ZSCALERCTL_FIXTURE_DIR` is unset** so `make agent-eval`/`agent-eval-record` can never hit a live tenant and commit real data (folding ci-metric critique Hole 7).

---

## 6. The Live Half (non-deterministic, tracked) + Runner

### 6.1 Roster (fixed, committed)

`roster.json` with mandatory `rank` (lower = weaker) and per-agent capability declaration (resolve Gap F — separate surface gaps from backend gaps):

| rank | agent | invocation | single-shot/session | reads local files? | notes |
|----|----|----|----|----|----|
| 1 | `devin-p` | `devin run --prompt …` | session (returns URL — needs session mgmt) | declared | deliberately-bad floor |
| 2 | `haiku` | `command claude -p --model … --allowedTools Bash` | single-shot | yes (Bash) | **`command claude`**, not `claude` — the nix-darwin fish wrapper injects `--remote-control` |
| 3 | `codex-exec` | `codex exec -q …` | single-shot | model-specific | mid-tier, different provider — top of the initial roster |
| 4 | `sonnet` | `command claude -p --model …` | single-shot | yes | ceiling — **DEFERRED (decision 1)**: not in the initial roster; add once the spine proves useful |

**Initial roster = ranks 1–3** (`devin-p`, `haiku`, `codex-exec`) per decision 1; `sonnet` is held back until the deterministic spine has demonstrated value, to avoid paying ceiling-validation cost up front. The `claude` caveat is load-bearing and confirmed in repo memory (the fish wrapper disables features). The floor = lowest-rank agent that *clears* (§2.4); because the live run is non-deterministic, the report includes run date + full per-agent verdict table so flips are visible.

### 6.2 Backend fairness (resolve Gap F)

- **Capability smoke before the battery** (folding multi-backend critique 1c): a trivial "what is 2+2, end with the envelope" probe per backend. A backend that can't pass it is reported `BACKEND_UNFIT` — its battery results are *not* surface FAILs.
- **A FAIL routes to the surface (a DAV-10 finding) only when the backend demonstrably had the capability** (read the docs / followed the envelope on the smoke). Otherwise it's a roster note. This is what keeps failure-mode's triage-by-who-failed (§7) from mis-routing a "devin can't read files in -p mode" as a surface gap.
- **Context-window asymmetry:** prompts include a size hint ("`schema list` returns ~165 entries"); `MaxTurns` is best-effort per backend and **does not affect PASS** (a brute-forced PASS still counts — the floor measures self-describability, not efficiency); `SchemaFirstPattern` is redefined as "`schema list` before any `<product> <resource> list|show`," not "literally first" (folding multi-backend critique 1a — a `--version`/`--help` probe must not trip it).

### 6.3 Hermeticity

- **PATH isolation + instrumentation:** `WorkDir/bin/zscalerctl` (the fixture binary) plus thin logging wrappers for any non-`zscalerctl` tool a grader judges (`jq`), all ahead of a minimal system PATH and all writing to the shared `ZSCALERCTL_FIXTURE_LOG` observed-command stream (§2.3) — so method judgments are grounded in logged invocations, not prose. The agent CLI itself is resolved via real PATH and passed as an absolute `argv[0]` (asserted `filepath.IsAbs` — folding multi-backend critique 6c).
- **Env hygiene (two steps, tested):** (1) the parent process's *real* `ZSCALERCTL_*` credentials are stripped — no real credential value ever enters the sandbox; (2) for normal scenarios, synthetic value-free fixture credentials are injected so the fixture binary's real credential-validation path passes (§2.3). The credential-negative scenario skips step 2, yielding exit 3 through the genuine path. So a normal read never spuriously exits 3, and a real secret is never present to leak.
- **Fresh `WorkDir`** per `(agent, question)` with verbatim `AGENTS.md` + `skills/zscalerctl/SKILL.md` and nothing else (no examples, no fixture JSON). `buildWorkDir` is not configurable — no "add file" API.
- **Prompt uses `zscalerctl` (on PATH)**, not the absolute `BinPath` (folding multi-backend critique 3b — don't leak the runner's tmp layout into committed transcripts).
- **Dump path-traversal guard:** fixture `dump --out` validated to resolve inside `WorkDir` (folding multi-backend critique 3c).
- **`RecordDir` defaults to `scratch/agent-eval-records/`, which Phase 2 adds to `.gitignore`** (the entry does not exist today — verified — so it is an explicit Phase-2 task, not a present-tense guarantee). The runner hard-refuses to write if its resolved `RecordDir` is inside the tracked tree (folding multi-backend critique 5b) — committed transcripts must never become a future training signal that lets an agent answer from memory. (Same applies to `scratch/agent-eval-report.json` and the `scratch/zscalerctl-fixture` binary.)

### 6.4 make targets + advisory smoke

```makefile
agent-eval-gen:   ## regenerate battery.json from catalog+fixtures (deterministic; CI-gated artifact)
	AGENT_EVAL_BATTERY_WRITE=1 go test -mod=vendor ./internal/agenteval -run TestAgentEvalBatteryIsCurrent

agent-eval-bin:   ## build the fixture binary (value-free, ZSCALERCTL_FIXTURE_DIR-gated, no creds)
	go build -o ./scratch/zscalerctl-fixture ./internal/agenteval/cmd/zscalerctl-fixture

agent-eval:       ## NON-deterministic live roster; prints floor + report; NEVER a gate
	go run ./internal/agenteval/cmd/run --roster-file internal/agenteval/roster.json --out ./scratch/agent-eval-report.json

agent-eval-record: ## refresh verdict goldens from a live run; shows diff, requires --yes; never auto-commits
	AGENT_EVAL_RECORD=1 go run ./internal/agenteval/cmd/run … --review
```

**Advisory PR smoke — DEFERRED initially (decision 2).** No LLM runs in PR CI at first; the live half is **scheduled/manual only** until the deterministic spine is mature. The design below is retained for if/when it's added, but is *not* part of the initial build. *If added:* one `rank=1`-or-cheapest agent, 3-question subset, `continue-on-error: true`, **the step exits 0 unconditionally** with a YAML comment `# MUST exit 0 always. Non-zero makes it a gate; do not change.` Skipped entirely when keys are absent (forks/dependabot). It would answer "did we obviously break the surface," never block.

### 6.5 Scheduled full roster

`agent-eval.yml`, weekly + `workflow_dispatch`, runs the full roster, opens a PR updating `docs/agentic-coverage.{md,json}` only when the floor or a per-agent score changes meaningfully — **excluding `binary_commit` from the change-significance comparison** (folding ci-metric critique Hole 8: the commit hash changes every run and would spam PRs). `ERROR` (API 5xx) is a distinct non-verdict that is re-run, never folded into "clears."

---

## 7. The Report & Feedback Loop

`docs/agentic-coverage.md` + `.json`, mirroring `FIELD_COVERAGE.md` in tone, generated, leads with the FLOOR not the ceiling, and **explicitly labeled "tracked, not gated."** Header states what's gated (battery + scorer drift) vs. tracked (live scores).

The loop is the anti-vanity machinery — the score exists *only* to produce `Finding`s; `Finding`s exist *only* to drive edits:

```
make agent-eval ─▶ report.json: []Finding + per-agent score
        │
        ▼  FLOOR: "worst agent that clears" + named first violation; each Finding carries Indicts[]
        │
        ▼  persistent Finding ─▶ GAP: tracked on DAV-10 (Linear, the workstream source of truth — decision 7);
        │     link GitHub #43 only for broader-roadmap items, not per-finding
        │     "AGENTIC-GAP: <FM> — <artifact> — <worst agent hit>"
        ▼
        PROOF OF CLOSURE: failing transcript committed as a verdict-golden + a deterministic test
        pins the surface change (e.g. "the --filter error string contains '~'"). A regression reds CI.
```

Every persistent finding resolves to one of three states (mirroring field-coverage's classified/deliberate/deferred):
- **Surface-fix** (default) — edit the indicted artifact.
- **Deliberate-floor** — accept that this agent can't do this and it's *below* our floor (e.g. `devin -p` mis-parsing deep JSON). Recorded with a reason. **Exception:** leak/safety FMs (FM-07/FM-08) are floor-independent and always fixed.
- **Deferred** — real gap, tracked on DAV-10 with an owner (decision 7; GitHub #43 linked only when it's a broader-roadmap item). Goal: **zero deferred at the floor agent**, just as field-coverage targets zero deferred fields.

Triage routes by *who* failed: a finding only `haiku` hits = high-priority surface-fix (a small honest agent failing is the strongest evidence the surface isn't self-describing); only `codex`/stronger hits = almost always a genuine surface bug (a strong agent failing means the surface actively misled it); only `devin -p` hits = candidate for deliberate-floor (don't dumb the surface down for the deliberately-bad end) — *unless* it's a leak/safety FM.

---

## 8. Phased Build Plan

The deterministic spine lands first; nothing non-deterministic masquerades as a gate.

**Phase 0 — Shared substrate (BLOCKS everything).** Write `contract.go`: the one answer protocol (§2.1), AnswerKinds (§2.2), the fixture-binary spec (§2.3), the definition of "clears" (§2.4), the `Finding` type (§2.5). Write `surface_promises.json`, `REALISM_DELTAS.md`, and `roster.json`. No other code begins until this is reviewed. *This is the holistic critic's #1 action.*

**Phase 1 — DETERMINISTIC / CI-GATED (the spine).**
1. Promote the fixture corpus to importable `internal/agenteval/fixtures`; extend for N>1, pagination, real id-join, `get <id>` semantics, no-fixture empty-list (§3.5).
2. Export `validateReaderConfig` → `zscaler.ValidateReaderConfig` (`internal/zscaler/reader.go`, currently unexported) so the fixture main can run it. Then `internal/agenteval/cmd/zscalerctl-fixture/main.go` (cred-validation-first, synthetic-cred injection for normal scenarios, `ZSCALERCTL_FIXTURE_DIR`-gated, observed-command sidecar + `jq` wrapper) + `TestShimBinaryBehavior`.
3. `derive.go` + `oracle.go` (lazy derivation, allow-list self-check) + `TestOracleMatchesFixtures`.
4. `battery.go` templates + `agent-eval-gen` + `TestAgentEvalBatteryIsCurrent` + `TestBatteryCoversSurface` + `TestEveryAgentSurfacePromiseHasAnFM`.
5. `scorer.go` (pure) + `transcript.go` envelope parser + the full golden suite (§5.2) + `TestEnvelopeParserGoldens`.
6. `posture_test.go` (value-free as a measured number) + calibration gate (§4.4), **seeded with hand-crafted minimal failing transcripts** so it is non-vacuous without any live run.
7. Wire all of the above into `go test ./...` (the existing `check` target). **This is the agentic analogue of the field-coverage gate and it lands complete before any live agent runs.**

**Phase 2 — NON-DETERMINISTIC / TRACKED (the live half).**
1. The runner: backends, sandbox, capability smoke, `BACKEND_UNFIT` handling, hermeticity guards (§6.3), abort-if-no-fixture-dir pre-flight.
2. `make agent-eval` + `report.go` (floor + Findings) + `docs/agentic-coverage.{md,json}`.
3. `make agent-eval-record` with human-review checkpoint (§5.3); seed verdict-goldens (reviewed) to harden the Phase-1 scorer tests.
4. Scheduled `agent-eval.yml` (full roster). **Advisory PR smoke deferred per decision 2** — added only after the spine is mature (§6.4), not in this phase.

**Phase 3 — Loop.** Wire `Finding` → DAV-10 tracking (GitHub #43 only for broader-roadmap items); first real pass; convert findings to surface-fixes / deliberate-floor / deferred; prove closures with committed transcripts + pinned tests.

Hard ordering invariant: **the live half (Phase 2) cannot land before the spine (Phase 1) is green**, and the live score is never wired into `check`.

---

## 9. Decisions

### 9.1 Decided 2026-06-14 (locked into this plan)

1. **Backend roster.** Initial roster = `devin-p` / `haiku` / `codex-exec` (ranks 1–3). `sonnet` **deferred** — added only once the spine proves useful, to skip ceiling-validation cost up front (§6.1).
2. **Advisory smoke in PR CI: no, initially.** Zero LLM in PR CI for now; the live half is scheduled/manual only until the deterministic spine is mature (§6.4). The exit-0-always design is retained for if/when it's revisited.
3. **Partial-credit policy:** `set`-only partial; everything else binary; `extra_allowed` per question (§4.3).
4. **Battery size:** start **20–30** instantiated questions; grow to 40–60 after the first real live run informs cost and coverage. Pick the per-run cost ceiling at that point.
5. **"Clears" definition:** keep the binary rule (§2.4); **no `0.80` aggregate** — no tunable floor knob.
6. **Fixture rotation cadence:** **per-release** initially (§3.6).
7. **Where the loop's gaps live:** **DAV-10 is the workstream source of truth** (Linear); link GitHub **#43** only for broader-roadmap items, not per-finding (§7).

### 9.2 Still open (your call, before Phase 2)

8. **Deliberate-floor line.** Which FMs are you willing to accept the weakest agent (`devin -p`) failing (e.g. deep-JSON parse), versus always-fix? Leak/safety FMs (FM-07/FM-08) are non-negotiable in the design; the rest is your risk appetite. This doesn't block Phase 0/1 — it only matters once live findings start landing in Phase 3, so it can wait.

---

## 10. Companion: Catalog-Aware Diff (the SECONDARY next step)

A read-only diff command, designed only enough to be the obvious follow-on once the eval lands. The honest caveat is the load-bearing decision, so it is stated first.

### 10.1 The honest caveat (why this is not just `diff`/`jq`)

A **generic structural diff is not worth a command** — `jq` + `diff` already compare two JSON trees, and shipping a thin wrapper would be redundant surface. **Only catalog-awareness justifies building it in.** The command exists *iff* it does things a generic differ cannot: stable-id matching, per-class diff policy, the modifiedTime tripwire, and the ALLOW→BLOCK semantic overlay. If a proposed feature doesn't require the catalog, it belongs in `jq`, not here.

### 10.2 Inputs & state model

- Operates over **two already-sanitized dump directories** — the same value-free, redacted projection the rest of the tool emits. It diffs *snapshots the operator owns*, not live tenants.
- **No cache, no tool-managed state.** "Drift over time" is *composed* with cron (snapshot nightly, diff snapshot N vs N-1), not baked into tool state. The tool stays a pure function of its two inputs — same fail-closed/value-free posture as everything else.

### 10.3 Matching & per-class policy

- **Stable-id resource matching.** Records are matched by catalog-declared stable id, so reorder/pagination differences are **not** a change. A generic differ would scream on reordered arrays; catalog-awareness knows the identity key.
- **Per-class diff policy.** Each field's `FieldClassification` drives how its delta is treated — e.g. a sensitive-identifier change is reported; a free-text change in a mode where it's dropped never appears (it was never emitted).
- **modifiedTime is a TRIPWIRE, not noise.** The naive move is to suppress timestamp-only changes as churn. Instead: when `modifiedTime` moved **but the emitted projection did not**, flag **"changed in a way outside our coverage."** This surfaces the fail-closed *blind spot* — the resource changed in a field we deliberately don't project — rather than hiding it. It is the diff-side analogue of FM-08: absence is information, not silence.

### 10.4 Output: structured engine + pretty overlay

- **Engine emits structured machine deltas:** `{resource, field, before, after, op}` — deterministic, pipeable, the agentic-first default.
- **A semantic PRETTY overlay** renders security-meaningful transitions ("ALLOW → BLOCK", "rule enabled → disabled") on top of the structured deltas. The overlay is a **renderer, not the engine** — it consumes the same structured deltas behind the format switch, exactly as the existing pretty/lipgloss renderer consumes the projected model. The semantics live in the catalog (which field is an allow/block toggle), keeping the engine generic and the meaning catalog-derived.

This composes cleanly with the eval: a future battery question ("what changed between these two dumps?") would grade the diff engine's structured output by the same derived-truth discipline used throughout this document.

---

Key repo facts grounding this design (verified): exit codes are `0/1/2/3/4/5/6` with `invalid_resource_id`→exit 2 and both `not_found`+`unsupported_resource`→exit 4 (`cmd/zscalerctl/main.go` — `errorKind()`:145-170, `exitCodeForError()`:172-197, constants at `:22-28`); the `ResourceReader` seam is `internal/cli/app.go:122/134/138`, live reader at `:864`; `validateReaderConfig` is unexported at `internal/zscaler/reader.go:1327`; the drift-gate pattern is `FIELD_COVERAGE_WRITE=1` + `t.Fatalf("…run make field-coverage")` (`internal/zscaler/field_coverage_test.go`, `Makefile:101`); the fixture corpus to promote is `internal/livesmoke/fake_runner_test.go`; the shell-out precedent is the `pwsh` smoke in `internal/cli/app_test.go`; no `internal/agenteval` package exists yet. (Line numbers are pinned as of this commit; they drift — treat the symbol names as canonical.)
