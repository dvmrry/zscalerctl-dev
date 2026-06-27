# Development And Public Surface Model

This project keeps a strict line between the supported product surface,
candidate seams being prepared for promotion, and experiments. The goal is to
make presentation and UI exploration possible without accidentally changing the
released CLI, root dependency graph, or machine contract.

A surface is anything another person or tool can depend on: commands, flags,
output formats, JSON shapes, schemas, exit codes, docs, package seams, build
targets, release artifacts, dependency graphs, or repository guidance for
agents.

## Surface Classes

| Class | Meaning | Default Gates | Compatibility |
| --- | --- | --- | --- |
| Supported | Shipped or documented behavior that users, agents, CI, and release automation may rely on. | Included in the normal root-module `make check`, CI, release checks, and public docs. | Changes need tests, docs, surface review, and semver treatment. |
| Candidate | A reviewed dev-repo seam or internal behavior being shaped for a possible supported surface. | Included in default gates only when it is part of the normal product runtime and has no experiment-only dependencies. | Not promised as a public API until promoted, but must not weaken supported behavior. |
| Experimental | A prototype, spike, alternate presentation layer, or uncommitted product idea. | Excluded from default root-module build/check paths unless deliberately promoted. | No compatibility promise. Experiments must be easy to remove. |

Current supported surfaces include:

- the `zscalerctl` CLI command, flags, help, completion, generated CLI docs, and
  introspection output
- JSON and NDJSON resource output, stderr error envelopes, and exit-code
  mapping
- committed JSON Schemas for dump, diff, redaction, config, and error artifacts
- `machine manifest`, `schema list`, and the manifest-first agent workflow
- the installable `skills/zscalerctl/` guidance and generated `.agents` copy
- release artifacts, checksums, provenance, and documented install behavior

Current candidate seams include:

- `internal/machine` request, response, manifest, executor, and error types
- `internal/machineio` JSON request/response helpers over the machine contract
- `internal/browser` projected catalog/resource loading interfaces
- `internal/resources` projected-record containers and catalog metadata
- `internal/runtime` trusted read-only machine runtime assembly for adapters
- package-boundary checks that keep overlays away from raw runtime packages

These are candidate seams for in-repo overlays and future promotion. They are
not a public Go module API: the `internal/` layout remains intentional.

Experimental surfaces include future Fang help/error experiments, Lip Gloss v2
renderer changes, TUI prototypes, Wails or desktop clients, React frontends,
MCP sidecars, or alternate command dispatchers until they are explicitly
promoted.

## Development Repo And Public Repo

The development repo may carry candidate seams on `main` when they strengthen
the supported CLI and pass the default gates. Experimental work should live on
feature branches, in separate repositories, or in clearly isolated nested
modules until it has a promotion decision.

The public/release surface should contain only supported behavior plus any
candidate seams that are needed by that behavior. Do not promote an experiment
to the public repo merely because it builds locally. Promotion requires an
explicit compatibility, dependency, and security review.

If a branch carries experimental code for review, the PR description must say
which class it belongs to and whether it changes any supported or candidate
surface. An experiment PR should not be labeled or described as supportable
product work until it has passed the promotion checklist below.

## Default Build And Check Rule

Default root-module commands validate supported surfaces and accepted candidate
seams only:

```sh
go build -mod=vendor ./cmd/zscalerctl
go test -mod=vendor ./...
go vet -mod=vendor ./...
go run honnef.co/go/tools/cmd/staticcheck@v0.7.0 ./...
make check
```

Experiments must not be pulled into those commands by default. That means:

- no experiment-only package may be required by `cmd/zscalerctl` or normal
  `internal/...` packages
- no experiment-only dependency may be added to the root `go.mod`, `go.sum`, or
  `vendor/` tree before promotion
- no experiment-only generated assets may be required by default docs, checks,
  release artifacts, or the normal binary
- no hidden command, renderer, or adapter may enter the supported command tree
  just to make an experiment reachable

Preferred isolation options are:

- a separate branch or repository
- a nested module under a path such as `experiments/<name>/` with its own
  `go.mod`, no committed root `go.work`, and no root-module imports from the
  experiment
- a separate executable or harness outside the default product binary
- an explicit opt-in target such as `make check-experiment-<name>`, not a
  prerequisite of `make check`

Build tags are acceptable only for very small spikes and only when the default
tag set leaves the root-module build, tests, docs, and dependency graph
unchanged. A build tag must not be the long-term boundary for a product-sized
UI or alternate runtime.

## Experiments Consume Core Seams

Experiments may consume the supported CLI JSON contract or the candidate core
seams. They must not define the core seam by reaching around it.

Allowed experiment inputs:

- `zscalerctl --format json` and `--format ndjson`
- `zscalerctl --format json machine manifest`
- `internal/machine`, `internal/machineio`, `internal/browser`, and
  already-projected `internal/resources` values when the experiment lives in
  this repo
- `internal/runtime` when an in-repo experiment needs the trusted live
  read-only machine runtime rather than a static fixture

Forbidden experiment shortcuts:

- importing `internal/cli` or `internal/output` to reuse command/rendering
  internals
- importing `internal/config`, `internal/credentials`, `internal/secretref`,
  `internal/secret`, or `internal/zscaler` to construct a parallel runtime
- receiving raw SDK clients, tokens, secret refs, or raw source records
- widening projection allow-lists or applying redaction after raw records have
  crossed into an overlay
- changing JSON, NDJSON, error envelopes, exit codes, introspection, completion,
  or generated CLI docs as a side effect of UI experimentation

If an experiment needs a capability that the core seam does not expose, add a
separate candidate-seam PR first. The experiment should then consume that seam.
Safe seams such as `internal/resources`, `internal/browser`, `internal/machine`,
and `internal/machineio` must not import the trusted runtime facade.

## Promotion Rules

Promoting an experimental surface to candidate requires:

- a short design note or PR description that names the target surface class
- proof that the experiment consumes projected seams or the JSON contract
- no raw config, credential, secret, SDK, or source-record path crossing into
  the overlay
- experiment-only dependencies isolated from the root module unless dependency
  promotion is part of the same reviewed change
- a rollback plan that can remove the experiment without changing supported
  behavior

Promoting candidate work to supported requires:

- documentation in the relevant public docs and generated references
- tests for behavior, errors, and security boundaries
- golden or schema coverage for any machine-readable surface
- dependency review for any new root-module package or tool
- `make check`, `make verify-experiment-boundaries`, `git diff --check`, and
  any targeted boundary checks
- semver labeling based on the supported surface impact
- removal of prototype markers such as hidden-only command exposure,
  experiment-only target names, or unsupported docs language

Once promoted, the surface belongs to the normal product. It must stay in the
default build/check path and cannot be treated as optional cleanup.

## Review Questions

Before merging a presentation, UI, or alternate-runtime change, answer these:

- Is this supported, candidate, or experimental?
- Does it change the root binary, root module dependencies, generated docs, or
  machine-readable output?
- Does default `make check` include only supported/candidate work?
- Does the change consume projected records and narrow capabilities?
- Could a future public repo promotion carry this safely, or should it stay in
  the dev repo or a separate module?
