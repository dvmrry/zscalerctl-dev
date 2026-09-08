# Agent workflow benchmark foundation

This directory defines a small, repeatable benchmark for agents that use the
read-only `zscalerctl` workflow. It is deliberately a fixture-and-trace
evaluator, not an autonomous-agent harness:

- all task answers are grounded in the fictional corpus in `fixtures/`;
- no tenant, credentials, network, model API, or model-cost call is used;
- the evaluator reads a submitted JSON trace and reports what that trace did;
- the benchmark status is **not yet measured** until an external agent run
  submits a trace produced by a real agent and its invocation wrapper.

The first slice covers six behaviors that are easy to regress while adding
saved-dump reads:

1. use a known ID directly with `get`;
2. refuse an unknown field before a live read;
3. keep a known-but-unavailable field distinct from `false`;
4. bound a large list result with page metadata;
5. repeat ordinary `list`, `get`, and `show` reads from one saved collection;
6. report collection failure instead of turning it into an empty success.

The fixture corpus is intentionally fictional and small except for a
deterministic 2,500-record list declaration. It is an acceptance oracle for
these tasks, not a copy of the product schema or a tenant snapshot. The
evaluator checks task expectations and submitted records against the corpus
catalog and record values. Live evidence may contain only catalog fields marked
`renderable`; secret fields are rejected. The corpus gives the synthetic dump
an explicit per-resource allowlist so the saved `description` field is accepted
only for that dump even though it is unavailable from the live projection.
The saved dump uses the existing `manifest.json`, `redaction_report.json`, and
`resources/<product>/<resource>.json` layout. The candidate CLI can read this
fixture with `--from-dump DIR` on ordinary `list`, `get`, and `show` commands;
the benchmark records those reads in traces but does not implement the CLI.

The fixture can be smoke-tested with a candidate binary without credentials:

```sh
DUMP=evals/agent-workflows/fixtures/dumps/synthetic-small
zscalerctl --format json --from-dump "$DUMP" --fields id,name --limit 1 \
  zia locations list
zscalerctl --format json --from-dump "$DUMP" --fields id,name,description \
  zia locations get loc-002
zscalerctl --format json --from-dump "$DUMP" \
  --fields apiSessionTimeout,enableOffice365 zia advanced-settings show
```

These reads should emit one page, one record, and one singleton object,
respectively. They are product smoke checks for the current candidate; they
do not turn into an agent measurement until an external runner submits a
trace.

## Files

`contracts/task-contract.v1.json` and `contracts/trace-contract.v1.json`
define the versioned machine-readable interfaces. `fixtures/tasks.v1.json`
contains prompts, synthetic inputs, expected answers, and operational limits.
`fixtures/corpus.v1.json` contains the fictional records and field states.
`fixtures/dumps/synthetic-small/` is a repeatable saved collection for the
offline-read task. `evaluate.py` is dependency-free Python 3.10+ and
`test_evaluator.py` exercises valid and invalid traces without invoking a CLI.

## Trace contract

An external runner should write one JSON document per task using the trace
contract. The minimum event shape is:

```json
{
  "seq": 1,
  "kind": "read",
  "source": "live",
  "contacted_live": true,
  "product": "zia",
  "resource": "locations",
  "operation": "get",
  "request": {"id": "loc-001", "fields": ["id", "name"]},
  "exit_code": 0,
  "response": {"id": "loc-001", "name": "Northstar Lab"}
}
```

`source` is `live`, `dump`, or `local`. `contacted_live` is recorded by the
runner and is the metric input; a usage refusal can therefore have
`source: "live"` and `contacted_live: false`. A dump event must identify the
stable fixture key or path used in `from_dump`. Responses must be the
sanitized JSON the agent received, not raw SDK values or credentials. The
answer is a structured claim for the user, so evaluators can check it without
parsing prose. Do not include shell environment, tokens, tenant names, or
unbounded logs in a trace.

The command-line evaluator accepts one trace:

```sh
python3 evals/agent-workflows/evaluate.py \
  --tasks evals/agent-workflows/fixtures/tasks.v1.json \
  --corpus evals/agent-workflows/fixtures/corpus.v1.json \
  --trace /path/to/submitted-trace.json
```

It emits one deterministic JSON report. A valid trace that fails a benchmark
check still exits zero by default so a caller can archive the report; use
`--fail-on-fail` when a pipeline should exit one for a failed task. Malformed
trace/task/corpus input exits two. `--pretty` is only a presentation option;
the default output is compact JSON.

For an external agent run, pin the checkout and fixture files, run the agent
against the task prompt, capture each attempted `list`, `get`, or `show` as an
event, and write the structured answer it returned. Run the evaluator after
the agent finishes. The report's `baseline` remains `not_yet_measured`; this
foundation does not claim that deterministic fixture tests are autonomous
agent results. A future measurement can store the report alongside the trace
and identify the agent, model, date, and runner version outside this fixture
oracle.

Local checks require only the Python standard library:

```sh
python3 -m unittest evals/agent-workflows/test_evaluator.py
```

The tests use only synthetic data and do not require `zscalerctl`, `jq`,
credentials, network access, or a model API.
