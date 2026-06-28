# stdio machine adapter experiment

This nested module is an unsupported experiment. It is not part of the
`zscalerctl` CLI surface, the root module, release artifacts, or default CI
gates.

The prototype reads one JSON `machine.Request` from stdin, decodes it through
strict `internal/machineio` helpers, constructs the trusted read-only
`internal/runtime` machine facade, executes the request, and writes one
newline-delimited JSON `machine.Response` to stdout.

It intentionally does not import CLI rendering, output helpers, raw config,
credentials, secret handling, SDK clients, UI frameworks, MCP servers, daemon
loops, sockets, Wails, TUI packages, Fang, or React. Config, credential,
secret-reference, SDK reader, browser, and machine assembly stay behind
`internal/runtime`; the experiment consumes that facade instead of duplicating
runtime setup.
