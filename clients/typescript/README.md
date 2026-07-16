# TypeScript engine-stdio client (candidate)

This directory is a candidate-only, zero-runtime-dependency TypeScript
reference client for the local `zscalerctl` engine stdio protocol v1. It is
not a supported package or included in release artifacts.

The client targets and is verified on Node `>=24.12.0`, using Node's built-in
TypeScript type stripping. Bun compatibility is not claimed until Bun runs the
same codec, lifecycle, and real-process suite. There are no runtime
dependencies, package installs, lockfiles, or code generation steps.

The public entry point is [src/index.ts](src/index.ts). It provides:

- strict bounded UTF-8 JSON parsing and encoding;
- immutable `WireNumber` values that preserve dynamic number lexemes;
- incremental LF/CRLF NDJSON splitting with bounded storage;
- closed bootstrap and v1 root-frame codecs;
- one-active-request queuing and typed methods for all 11 operation pairs;
- request-relative sequence, item, progress, warning, result, and diff
  reconciliation;
- exact fragment base64, size, order, SHA-256, and payload validation;
- `AbortSignal` cancellation with bounded bootstrap, operation-cancellation,
  and shutdown watchdogs; and
- a no-shell Node child-process adapter that requires an absolute engine path,
  passes only the host's five policy flags, drains and discards stderr, and
  exposes no credential wire API.

Minimal local use:

```ts
import { spawnEngine } from "./src/index.ts";

const engine = await spawnEngine({ executable: "/absolute/path/to/zscalerctl-engine" });
try {
  const result = await engine.list({
    product: "zia",
    resource: "locations",
    fields: [],
    filters: [],
    search: "",
  });
  console.log(result.items);
} finally {
  await engine.close();
}
```

The process inherits the caller-supplied environment (or `process.env` by
default), so credential lifetime and secret-provider behavior remain owned by
the Go runtime. Event callbacks are synchronous and receive deeply frozen
values; use them for quick state updates rather than blocking work.

The client waits at most ten seconds for bootstrap, seven seconds for an
ordinary canceled request terminal, and eight seconds for ordinary session
shutdown before aborting the child. Consumers may set `startupTimeoutMs`,
`cancelTimeoutMs`, and `closeTimeoutMs` on `spawnEngine` (or
`EngineClient.connect`) from 1 through 300,000 milliseconds when their
transport needs different bounds. A bootstrap `signal` can terminate startup
earlier; process-backed startup waits for the direct child to exit before
rejecting, so the caller is never handed an unreachable live engine process.

Dump cancellation is intentionally different because protocol v1 has no
wire-visible filesystem commit marker. Once a dump is active, the reference
client sends cancellation but does not apply its own cancel or close kill
watchdog; the host remains responsible for promptly terminating pre-commit
work and for staying alive through any committed confidential-artifact
cleanup. This trades a bounded wait against an untrusted engine for avoiding a
client-forced partial cleanup of a local effect.

Stderr is not a protocol or diagnostic API: the process adapter continuously
drains and discards it without retaining bytes, and neither stderr volume nor a
read-side stderr error can terminate the child. This keeps memory bounded while
preventing a non-authoritative side channel from killing committed dump
cleanup. Protocol/stdin failures remain transport failures.

V1 is response-atomic and has a 64 MiB per-item limit but no total-response
limit. The client therefore retains validated semantic items until completion,
and a fragmented item is reconstructed in one bounded buffer. Consumers should
expect memory to scale with the admitted response and should use fields,
filters, search, and narrow resource selections where available.

Run the corpus and focused tests with:

```sh
node --test test/*.test.ts
```

The tests load the Go-owned fixtures directly from
`internal/enginewire/testdata/conformance/`. The repository verifier also
builds the actual Go host and runs credential-free process integration.
