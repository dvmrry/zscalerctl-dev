#!/usr/bin/env node

const readline = require("node:readline");

const protocol = "zscalerctl.engine.stdio";

function send(frame) {
  process.stdout.write(`${JSON.stringify(frame)}\n`);
}

send({
  type: "hello",
  protocol,
  versions: ["1"],
  bootstrap: { frame_bytes: 65_536, json_depth: 8 },
});

const input = readline.createInterface({ input: process.stdin, crlfDelay: Infinity });
let initialized = false;
let requestFinished = false;

input.on("line", (line) => {
  const frame = JSON.parse(line);
  if (!initialized) {
    if (frame.type !== "initialize") process.exit(2);
    initialized = true;
    send({
      type: "ready",
      protocol,
      version: "1",
      schema: {
        id: "urn:zscalerctl:engine-stdio:protocol:1",
        sha256: "6cba5a8170e538bd6eacde38c84526873f691421d6dc5f57cacfbd5f9438c522",
      },
      server: { name: "zscalerctl-engine", version: "test" },
      limits: {
        client_frame_bytes: 1_048_576,
        server_frame_bytes: 1_048_576,
        json_depth: 64,
        aggregate_item_bytes: 67_108_864,
        fragment_chunk_bytes: 524_288,
        url_count: 1024,
        read_field_count: 1024,
        read_filter_count: 1024,
        product_selector_count: 16,
        resource_selector_count: 4096,
        path_bytes: 32_768,
        control_string_bytes: 8192,
      },
      engine: {
        version: "engine.v1",
        tenant_read_only: true,
        capabilities: [{
          name: "dump.write",
          operations: ["dump"],
          input: "dump",
          result: "dump_summary",
          tenant_read_only: true,
          effects: [
            { kind: "local_filesystem_read", when: "always" },
            { kind: "local_filesystem_write", when: "always" },
            { kind: "local_filesystem_delete", when: "request_dependent" },
            { kind: "network_access", when: "always" },
            { kind: "process_execution", when: "configuration_dependent" },
          ],
        }],
      },
    });
    return;
  }
  if (frame.type === "cancel") return;
  if (frame.type !== "request" || frame.capability !== "dump.write" || frame.operation !== "dump") {
    process.exit(2);
  }
  send({
    type: "started",
    id: frame.id,
    seq: 1,
    capability: "dump.write",
    operation: "dump",
  });
  process.stderr.write(Buffer.alloc(70 * 1024, 0x78));
  setTimeout(() => {
    send({
      type: "completed",
      id: frame.id,
      seq: 2,
      result: {
        kind: "dump_summary",
        records_written: 0,
        resources_written: 0,
        warning_count: 0,
        partial: false,
        redaction: "standard",
        failures: [],
        stream_items_emitted: 0,
      },
    });
    requestFinished = true;
  }, 75);
});

input.on("close", () => {
  if (!requestFinished) process.exit(2);
  process.exit(0);
});
