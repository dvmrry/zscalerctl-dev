# stdio machine adapter experiment

This nested module is an unsupported experiment. It is not part of the
`zscalerctl` CLI surface, the root module, release artifacts, or default CI
gates.

The prototype reads one JSON `machine.Request` from stdin, executes it through
`internal/machineio.ExecuteJSON` with a static fake executor, and writes one
newline-delimited JSON `machine.Response` to stdout.

It intentionally does not import CLI rendering, config, credentials, secret
handling, SDK clients, UI frameworks, MCP servers, daemon loops, sockets, Wails,
TUI packages, Fang, or React. A future promoted adapter must replace the static
executor with a reviewed runtime seam rather than duplicating CLI/runtime
assembly.
