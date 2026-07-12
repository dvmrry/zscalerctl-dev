import { createHash } from "node:crypto";
import { test } from "node:test";
import assert from "node:assert/strict";

import {
  AGGREGATE_ITEM_BYTES,
  EngineCanceledError,
  EngineClient,
  EngineClientError,
  FRAGMENT_CHUNK_BYTES,
  PROTOCOL,
  V1_FRAME_BYTES,
  V1_JSON_DEPTH,
  V1_SCHEMA_ID,
  V1_SCHEMA_SHA256,
  V1_VERSION,
  decodeBootstrapClientFrame,
  decodeClientFrame,
  encodeBootstrapServerFrame,
  encodeServerFrame,
  type ClientFrame,
  type ClientRequest,
  type EngineManifest,
  type EngineTransport,
  type EngineTransportExit,
  type Ready,
  type ServerFrame,
} from "../src/index.ts";

const encoder = new TextEncoder();

class AsyncByteQueue implements AsyncIterable<Uint8Array> {
  private readonly values: Uint8Array[] = [];
  private readonly waiters: Array<(result: IteratorResult<Uint8Array>) => void> = [];
  private ended = false;

  push(value: Uint8Array): void {
    assert.equal(this.ended, false);
    const waiter = this.waiters.shift();
    if (waiter !== undefined) waiter({ value, done: false });
    else this.values.push(value);
  }

  close(): void {
    if (this.ended) return;
    this.ended = true;
    for (const waiter of this.waiters.splice(0)) waiter({ value: undefined, done: true });
  }

  [Symbol.asyncIterator](): AsyncIterator<Uint8Array> {
    return {
      next: async (): Promise<IteratorResult<Uint8Array>> => {
        const value = this.values.shift();
        if (value !== undefined) return { value, done: false };
        if (this.ended) return { value: undefined, done: true };
        return await new Promise((resolve) => this.waiters.push(resolve));
      },
    };
  }
}

type RequestHandler = (transport: ReactiveTransport, frame: ClientFrame) => void;

class ReactiveTransport implements EngineTransport {
  readonly output: AsyncIterable<Uint8Array>;
  readonly completion: Promise<EngineTransportExit>;
  readonly requestIDs: number[] = [];
  aborted = false;

  private readonly queue = new AsyncByteQueue();
  private readonly handler: RequestHandler;
  private resolveCompletion!: (exit: EngineTransportExit) => void;
  private initialized = false;
  private ended = false;
  private readonly hangOnClose: boolean;

  constructor(handler: RequestHandler = completeRequest, options: { readonly hangOnClose?: boolean } = {}) {
    this.handler = handler;
    this.hangOnClose = options.hangOnClose === true;
    this.output = this.queue;
    this.completion = new Promise((resolve) => {
      this.resolveCompletion = resolve;
    });
    this.sendBootstrap({
      type: "hello",
      protocol: PROTOCOL,
      versions: [V1_VERSION],
      bootstrap: { frame_bytes: 65_536, json_depth: 8 },
    });
  }

  async write(data: Uint8Array): Promise<void> {
    assert.equal(this.ended, false);
    assert.equal(data.at(-1), 0x0a);
    const body = data.slice(0, -1);
    if (!this.initialized) {
      const frame = decodeBootstrapClientFrame(body);
      assert.equal(frame.type, "initialize");
      this.initialized = true;
      this.send(READY);
      return;
    }
    const frame = decodeClientFrame(body);
    if (frame.type === "request") this.requestIDs.push(frame.id);
    this.handler(this, frame);
  }

  async closeInput(): Promise<void> {
    if (this.hangOnClose) return;
    this.finish({ code: 0, signal: null });
  }

  abort(): void {
    this.aborted = true;
    this.finish({ code: 1, signal: null });
  }

  send(frame: ServerFrame, split = false): void {
    const body = encodeServerFrame(frame);
    const data = new Uint8Array(body.length + 1);
    data.set(body);
    data[body.length] = 0x0a;
    if (!split || data.length < 4) {
      this.queue.push(data);
      return;
    }
    const first = Math.floor(data.length / 3);
    const second = Math.floor(data.length * 2 / 3);
    this.queue.push(data.slice(0, first));
    this.queue.push(data.slice(first, second));
    this.queue.push(data.slice(second));
  }

  private sendBootstrap(frame: Parameters<typeof encodeBootstrapServerFrame>[0]): void {
    const body = encodeBootstrapServerFrame(frame);
    const data = new Uint8Array(body.length + 1);
    data.set(body);
    data[body.length] = 0x0a;
    this.queue.push(data);
  }

  private finish(exit: EngineTransportExit): void {
    if (this.ended) return;
    this.ended = true;
    this.queue.close();
    this.resolveCompletion(exit);
  }
}

const ENGINE_MANIFEST = {
  version: "engine.v1",
  tenant_read_only: true,
  capabilities: [
    { name: "engine.manifest", operations: ["manifest"], input: "none", result: "engine_manifest", tenant_read_only: true, effects: [] },
    { name: "catalog.schema", operations: ["list"], input: "none", result: "resource_catalog", tenant_read_only: true, effects: [] },
    { name: "status.inspect", operations: ["doctor", "auth_status", "config_status"], input: "status", result: "status", tenant_read_only: true, effects: [{ kind: "local_filesystem_read", when: "configuration_dependent" }] },
    { name: "zia.url_lookup", operations: ["lookup"], input: "url_lookup", result: "url_classifications", tenant_read_only: true, effects: [{ kind: "local_filesystem_read", when: "configuration_dependent" }, { kind: "network_access", when: "always" }, { kind: "process_execution", when: "configuration_dependent" }] },
    { name: "resources.read", operations: ["list", "get", "show"], input: "resource_read", result: "projected_records", tenant_read_only: true, effects: [{ kind: "local_filesystem_read", when: "configuration_dependent" }, { kind: "network_access", when: "always" }, { kind: "process_execution", when: "configuration_dependent" }] },
    { name: "dump.write", operations: ["dump"], input: "dump", result: "dump_summary", tenant_read_only: true, effects: [{ kind: "local_filesystem_read", when: "always" }, { kind: "local_filesystem_write", when: "always" }, { kind: "local_filesystem_delete", when: "request_dependent" }, { kind: "network_access", when: "always" }, { kind: "process_execution", when: "configuration_dependent" }] },
    { name: "diff.compare", operations: ["diff"], input: "diff", result: "diff_report", tenant_read_only: true, effects: [{ kind: "local_filesystem_read", when: "always" }] },
  ],
} as const satisfies EngineManifest;

const READY = {
  type: "ready",
  protocol: PROTOCOL,
  version: V1_VERSION,
  schema: { id: V1_SCHEMA_ID, sha256: V1_SCHEMA_SHA256 },
  server: { name: "zscalerctl-engine", version: "test" },
  limits: {
    client_frame_bytes: V1_FRAME_BYTES,
    server_frame_bytes: V1_FRAME_BYTES,
    json_depth: V1_JSON_DEPTH,
    aggregate_item_bytes: AGGREGATE_ITEM_BYTES,
    fragment_chunk_bytes: FRAGMENT_CHUNK_BYTES,
    url_count: 1024,
    read_field_count: 1024,
    read_filter_count: 1024,
    product_selector_count: 16,
    resource_selector_count: 4096,
    path_bytes: 32_768,
    control_string_bytes: 8192,
  },
  engine: ENGINE_MANIFEST,
} as const satisfies Ready;

function started(request: ClientRequest): ServerFrame {
  return {
    type: "started",
    id: request.id,
    seq: 1,
    capability: request.capability,
    operation: request.operation,
  };
}

function completeRequest(transport: ReactiveTransport, frame: ClientFrame): void {
  if (frame.type === "cancel") return;
  transport.send(started(frame), true);
  const id = frame.id;
  switch (`${frame.capability}/${frame.operation}`) {
    case "engine.manifest/manifest":
      transport.send({ type: "completed", id, seq: 2, result: { kind: "engine_manifest", manifest: ENGINE_MANIFEST } });
      return;
    case "catalog.schema/list":
      transport.send({ type: "item", id, seq: 2, kind: "catalog_resource", item: {
        product: "zia", name: "locations", shape: "list", operations: ["list", "get"], get_key: "id", fields: [],
      } });
      transport.send({ type: "completed", id, seq: 3, result: { kind: "catalog_summary", resources: 1, stream_items_emitted: 1 } });
      return;
    case "status.inspect/doctor":
      transport.send({ type: "completed", id, seq: 2, result: { kind: "doctor_status", status: {
        status: "ok", mode: "env", profile: "", config: "", auth_mode: "oneapi", redaction: "standard",
        timeout: "30s", cache: "enabled", proxy: "unset", credentials: "missing", live_api: "not_checked",
      } } });
      return;
    case "status.inspect/auth_status":
      transport.send({ type: "completed", id, seq: 2, result: { kind: "auth_status", status: {
        credentials: "missing", credential_exchange: "not_attempted", live_api: "not_checked",
      } } });
      return;
    case "status.inspect/config_status":
      transport.send({ type: "completed", id, seq: 2, result: { kind: "config_status", status: {
        source: "environment", config_file_set: false, profile: "", auth_mode: "oneapi", vanity_domain_set: false,
        credentials: { client_id_set: false, client_secret_set: false, client_secret_file_set: false },
        zpa: { customer_id_set: false, microtenant_id_set: false },
        zia_legacy: { username_set: false, password_set: false, password_file_set: false, api_key_set: false, api_key_file_set: false, cloud_set: false },
        proxy: { url_set: false, from_environment: false }, defaults: { redaction: "standard", no_cache: false },
      } } });
      return;
    case "zia.url_lookup/lookup":
      transport.send({ type: "item", id, seq: 2, kind: "url_classification", item: {
        url: "example.com", classifications: ["BUSINESS_USE"], security_alert_classifications: [], application: "",
      } });
      transport.send({ type: "completed", id, seq: 3, result: { kind: "url_lookup_summary", classifications: 1, stream_items_emitted: 1 } });
      return;
    case "resources.read/list":
    case "resources.read/get":
    case "resources.read/show": {
      const input = (frame as ClientRequest & { readonly input: { readonly product: "zia" | "zpa" | "ztw" | "zcc" | "zidentity"; readonly resource: string } }).input;
      transport.send({ type: "item", id, seq: 2, kind: "projected_record", item: {
        product: input.product, resource: input.resource, record: { id: "1", name: "HQ" },
      } });
      transport.send({ type: "completed", id, seq: 3, result: { kind: "resource_read_summary", records: 1, stream_items_emitted: 1 } });
      return;
    }
    case "dump.write/dump": {
      const failure = { product: "zia", resource: "locations", phase: "list", kind: "list_failed" } as const;
      transport.send({ type: "progress", id, seq: 2, phase: "resource_started", current: 1, total: 1, product: "zia", resource: "locations" });
      transport.send({ type: "warning", id, seq: 3, warning: failure });
      transport.send({ type: "completed", id, seq: 4, result: {
        kind: "dump_summary", records_written: 0, resources_written: 0, warning_count: 1, partial: true,
        redaction: "standard", failures: [failure], stream_items_emitted: 0,
      } });
      return;
    }
    case "diff.compare/diff":
      transport.send({ type: "progress", id, seq: 2, phase: "resource_started", current: 1, total: 1, product: "zia", resource: "locations" });
      transport.send({ type: "item", id, seq: 3, kind: "diff_resource", item: {
        product: "zia", resource: "locations", identity: { mode: "get_key", field: "id" }, added: 1, removed: 1, changed_fields: 1,
      } });
      transport.send({ type: "item", id, seq: 4, kind: "diff_added", item: { product: "zia", resource: "locations", key: "1", record: { id: "1" } } });
      transport.send({ type: "item", id, seq: 5, kind: "diff_removed", item: { product: "zia", resource: "locations", key: "2", record: { id: "2" } } });
      transport.send({ type: "item", id, seq: 6, kind: "diff_field_change", item: { product: "zia", resource: "locations", key: "3", field: "name", old: "old", new: "new" } });
      transport.send({ type: "completed", id, seq: 7, result: {
        kind: "diff_summary", schema: "zscalerctl.diff.v1",
        old: { side: "old", manifest_schema: "zscalerctl.dump.v1", redaction: "standard", status: "complete", partial: false },
        new: { side: "new", manifest_schema: "zscalerctl.dump.v1", redaction: "standard", status: "complete", partial: false },
        summary: { resources_compared: 1, resources_with_drift: 1, records_added: 1, records_removed: 1, records_changed: 1 },
        has_drift: true, stream_items_emitted: 4,
      } });
      return;
    default:
      assert.fail(`unhandled request ${frame.capability}/${frame.operation}`);
  }
}

test("queues and validates all eleven typed operations", async () => {
  const transport = new ReactiveTransport();
  const client = await EngineClient.connect(transport);
  assert.equal(Object.isFrozen(client.ready), true);
  assert.equal(Object.isFrozen(client.ready.engine.capabilities), true);

  const results = await Promise.all([
    client.manifest(),
    client.catalog(),
    client.doctor(),
    client.authStatus(),
    client.configStatus(),
    client.lookup({ urls: ["example.com"] }),
    client.list({ product: "zia", resource: "locations", fields: [], filters: [], search: "" }),
    client.get({ product: "zia", resource: "locations", record_id: "1", fields: [] }),
    client.show({ product: "zia", resource: "advanced-settings", fields: [] }),
    client.dump({ output_dir: "/tmp/dump", products: ["zia"], resources: [], continue_on_error: true, force: false }),
    client.diff({ old_dir: "/tmp/old", new_dir: "/tmp/new", products: [], resources: [], ignore_operational: false, allow_partial: false }),
  ]);

  assert.deepEqual(results.map((result) => result.id), [1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11]);
  assert.deepEqual(transport.requestIDs, [1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11]);
  assert.equal(results[1].items[0].kind, "catalog_resource");
  assert.equal(results[9].warnings.length, 1);
  assert.equal(results[10].items.length, 4);
  await client.close();
});

test("reconstructs and validates a fragmented semantic item", async () => {
  const payload = encoder.encode('{"product":"zia","resource":"locations","record":{"id":"☃"}}');
  const digest = createHash("sha256").update(payload).digest("hex");
  const transport = new ReactiveTransport((current, frame) => {
    if (frame.type === "cancel") return;
    current.send(started(frame));
    current.send({ type: "item_begin", id: frame.id, seq: 2, item_id: 1, kind: "projected_record", encoding: "json", bytes: payload.length });
    current.send({ type: "item_chunk", id: frame.id, seq: 3, item_id: 1, index: 0, data: Buffer.from(payload).toString("base64") }, true);
    current.send({ type: "item_end", id: frame.id, seq: 4, item_id: 1, chunks: 1, sha256: digest });
    current.send({ type: "completed", id: frame.id, seq: 5, result: { kind: "resource_read_summary", records: 1, stream_items_emitted: 1 } });
  });
  const client = await EngineClient.connect(transport);
  const result = await client.list({ product: "zia", resource: "locations", fields: [], filters: [], search: "" });
  assert.equal(result.items.length, 1);
  assert.equal(result.items[0].value.record.id, "☃");
  await client.close();
});

test("AbortSignal cancellation drains the terminal and preserves the session", async () => {
  let awaitingCancel = true;
  const transport = new ReactiveTransport((current, frame) => {
    if (frame.type === "request" && awaitingCancel) {
      current.send(started(frame));
    } else if (frame.type === "cancel") {
      current.send({ type: "canceled", id: frame.id, seq: 2, error: { kind: "canceled" } });
      awaitingCancel = false;
    } else {
      completeRequest(current, frame);
    }
  });
  const client = await EngineClient.connect(transport);
  const controller = new AbortController();
  await assert.rejects(
    client.list(
      { product: "zia", resource: "locations", fields: [], filters: [], search: "" },
      { signal: controller.signal, onEvent: (event) => {
        if (event.type === "started") controller.abort();
      } },
    ),
    (error: unknown) => error instanceof EngineCanceledError && error.id === 1,
  );
  const manifest = await client.manifest();
  assert.equal(manifest.id, 2);
  await client.close();
});

test("success after a late abort remains successful", async () => {
  const transport = new ReactiveTransport((current, frame) => {
    if (frame.type === "cancel") return;
    current.send(started(frame));
    current.send({ type: "item", id: frame.id, seq: 2, kind: "projected_record", item: {
      product: "zia", resource: "locations", record: { id: "1", name: "HQ" },
    } });
    setTimeout(() => current.send({
      type: "completed", id: frame.id, seq: 3,
      result: { kind: "resource_read_summary", records: 1, stream_items_emitted: 1 },
    }), 60);
  });
  const client = await EngineClient.connect(transport, { cancelTimeoutMs: 25, closeTimeoutMs: 100 });
  const controller = new AbortController();
  const result = await client.list(
    { product: "zia", resource: "locations", fields: [], filters: [], search: "" },
    { signal: controller.signal, onEvent: (event) => {
      if (event.type === "semantic_item") controller.abort();
    } },
  );
  assert.equal(result.result.kind, "resource_read_summary");
  assert.equal(transport.aborted, false);
  await client.close();
});

test("a fragment digest mismatch kills the session", async () => {
  const payload = encoder.encode('{"product":"zia","resource":"locations","record":{}}');
  const transport = new ReactiveTransport((current, frame) => {
    if (frame.type === "cancel") return;
    current.send(started(frame));
    current.send({ type: "item_begin", id: frame.id, seq: 2, item_id: 1, kind: "projected_record", encoding: "json", bytes: payload.length });
    current.send({ type: "item_chunk", id: frame.id, seq: 3, item_id: 1, index: 0, data: Buffer.from(payload).toString("base64") });
    current.send({ type: "item_end", id: frame.id, seq: 4, item_id: 1, chunks: 1, sha256: "0".repeat(64) });
  });
  const client = await EngineClient.connect(transport);
  await assert.rejects(
    client.list({ product: "zia", resource: "locations", fields: [], filters: [], search: "" }),
    (error: unknown) => error instanceof EngineClientError && error.kind === "protocol",
  );
});

test("maliciously large diff counters fail without counter-driven work", async () => {
  const transport = new ReactiveTransport((current, frame) => {
    if (frame.type === "cancel") return;
    current.send(started(frame));
    current.send({ type: "progress", id: frame.id, seq: 2, phase: "resource_started", current: 1, total: 1, product: "zia", resource: "locations" });
    current.send({ type: "item", id: frame.id, seq: 3, kind: "diff_resource", item: {
      product: "zia", resource: "locations", identity: { mode: "get_key", field: "id" },
      added: Number.MAX_SAFE_INTEGER, removed: 0, changed_fields: 0,
    } });
    current.send({ type: "completed", id: frame.id, seq: 4, result: {
      kind: "diff_summary", schema: "zscalerctl.diff.v1",
      old: { side: "old", manifest_schema: "zscalerctl.dump.v1", redaction: "standard", status: "complete", partial: false },
      new: { side: "new", manifest_schema: "zscalerctl.dump.v1", redaction: "standard", status: "complete", partial: false },
      summary: { resources_compared: 1, resources_with_drift: 1, records_added: Number.MAX_SAFE_INTEGER, records_removed: 0, records_changed: 0 },
      has_drift: true, stream_items_emitted: 1,
    } });
  });
  const client = await EngineClient.connect(transport);
  await assert.rejects(
    client.diff({ old_dir: "/tmp/old", new_dir: "/tmp/new", products: [], resources: [], ignore_operational: false, allow_partial: false }),
    (error: unknown) => error instanceof EngineClientError && error.kind === "protocol",
  );
});

test("event values are immutable before consumer callbacks run", async () => {
  const transport = new ReactiveTransport();
  const client = await EngineClient.connect(transport);
  await assert.rejects(
    client.list(
      { product: "zia", resource: "locations", fields: [], filters: [], search: "" },
      { onEvent: (event) => {
        if (event.type === "semantic_item") {
          (event.value.record as { name?: string }).name = "mutated";
        }
      } },
    ),
    (error: unknown) => error instanceof EngineClientError && error.kind === "callback",
  );
  assert.equal((await client.manifest()).id, 2);
  await client.close();
});

test("accepts selected diff progress for a resource empty on both sides", async () => {
  const transport = new ReactiveTransport((current, frame) => {
    if (frame.type === "cancel") return;
    current.send(started(frame));
    current.send({ type: "progress", id: frame.id, seq: 2, phase: "resource_started", current: 1, total: 1, product: "zia", resource: "locations" });
    current.send({ type: "completed", id: frame.id, seq: 3, result: {
      kind: "diff_summary", schema: "zscalerctl.diff.v1",
      old: { side: "old", manifest_schema: "zscalerctl.dump.v1", redaction: "standard", status: "complete", partial: false },
      new: { side: "new", manifest_schema: "zscalerctl.dump.v1", redaction: "standard", status: "complete", partial: false },
      summary: { resources_compared: 0, resources_with_drift: 0, records_added: 0, records_removed: 0, records_changed: 0 },
      has_drift: false, stream_items_emitted: 0,
    } });
  });
  const client = await EngineClient.connect(transport);
  const result = await client.diff({
    old_dir: "/tmp/old", new_dir: "/tmp/new", products: [], resources: [],
    ignore_operational: false, allow_partial: false,
  });
  assert.equal(result.progress.length, 1);
  assert.equal(result.result.summary.resources_compared, 0);
  await client.close();
});

test("rejects dump completions that do not conserve selected resources", async (context) => {
  const cases = [
    { name: "unaccounted", resources: 1, warnings: 0 },
    { name: "too many successes", resources: 3, warnings: 0 },
    { name: "successes plus warnings exceed total", resources: 1, warnings: 2 },
  ] as const;
  for (const testCase of cases) {
    await context.test(testCase.name, async () => {
      const failures = Array.from({ length: testCase.warnings }, () => ({
        product: "zia" as const, resource: "locations", phase: "list" as const, kind: "list_failed" as const,
      }));
      const transport = new ReactiveTransport((current, frame) => {
        if (frame.type === "cancel") return;
        current.send(started(frame));
        current.send({ type: "progress", id: frame.id, seq: 2, phase: "resource_started", current: 1, total: 2, product: "zia", resource: "locations" });
        current.send({ type: "progress", id: frame.id, seq: 3, phase: "resource_started", current: 2, total: 2, product: "zia", resource: "url-filtering-rules" });
        let sequence = 4;
        for (const failure of failures) {
          current.send({ type: "warning", id: frame.id, seq: sequence, warning: failure });
          sequence += 1;
        }
        current.send({ type: "completed", id: frame.id, seq: sequence, result: {
          kind: "dump_summary", records_written: 0, resources_written: testCase.resources,
          warning_count: failures.length, partial: failures.length > 0, redaction: "standard",
          failures, stream_items_emitted: 0,
        } });
      });
      const client = await EngineClient.connect(transport);
      await assert.rejects(
        client.dump({ output_dir: "/tmp/dump", products: ["zia"], resources: [], continue_on_error: true, force: false }),
        (error: unknown) => error instanceof EngineClientError && error.kind === "protocol",
      );
    });
  }
});

test("rejects duplicate diff field changes", async () => {
  const transport = new ReactiveTransport((current, frame) => {
    if (frame.type === "cancel") return;
    current.send(started(frame));
    current.send({ type: "progress", id: frame.id, seq: 2, phase: "resource_started", current: 1, total: 1, product: "zia", resource: "locations" });
    current.send({ type: "item", id: frame.id, seq: 3, kind: "diff_resource", item: {
      product: "zia", resource: "locations", identity: { mode: "get_key", field: "id" },
      added: 0, removed: 0, changed_fields: 2,
    } });
    const change = { product: "zia" as const, resource: "locations", key: "1", field: "name", old: "old", new: "new" };
    current.send({ type: "item", id: frame.id, seq: 4, kind: "diff_field_change", item: change });
    current.send({ type: "item", id: frame.id, seq: 5, kind: "diff_field_change", item: change });
    current.send({ type: "completed", id: frame.id, seq: 6, result: {
      kind: "diff_summary", schema: "zscalerctl.diff.v1",
      old: { side: "old", manifest_schema: "zscalerctl.dump.v1", redaction: "standard", status: "complete", partial: false },
      new: { side: "new", manifest_schema: "zscalerctl.dump.v1", redaction: "standard", status: "complete", partial: false },
      summary: { resources_compared: 1, resources_with_drift: 1, records_added: 0, records_removed: 0, records_changed: 1 },
      has_drift: true, stream_items_emitted: 3,
    } });
  });
  const client = await EngineClient.connect(transport);
  await assert.rejects(
    client.diff({ old_dir: "/tmp/old", new_dir: "/tmp/new", products: [], resources: [], ignore_operational: false, allow_partial: false }),
    (error: unknown) => error instanceof EngineClientError && error.kind === "protocol",
  );
});

test("invalid method input returns a normalized rejected promise without consuming an ID", async () => {
  const transport = new ReactiveTransport();
  const client = await EngineClient.connect(transport);
  const invalid = client.list(null as never);
  assert.ok(invalid instanceof Promise);
  await assert.rejects(
    invalid,
    (error: unknown) => error instanceof EngineClientError && error.kind === "request",
  );
  assert.equal((await client.manifest()).id, 1);
  await client.close();
});

test("invalid transport output is normalized and aborted", async () => {
  let aborted = false;
  const transport = {
    output: null as unknown as AsyncIterable<Uint8Array>,
    async write(): Promise<void> {},
    async closeInput(): Promise<void> {},
    abort(): void { aborted = true; },
  } satisfies EngineTransport;
  await assert.rejects(
    EngineClient.connect(transport),
    (error: unknown) => error instanceof EngineClientError && error.kind === "protocol",
  );
  assert.equal(aborted, true);
});

test("busy rejection is fatal for a serialized client", async () => {
  const transport = new ReactiveTransport((current, frame) => {
    if (frame.type === "request") {
      current.send({ type: "request_rejected", id: frame.id, reason: "busy" });
    }
  });
  const client = await EngineClient.connect(transport);
  const first = client.manifest();
  const second = client.catalog();
  const results = await Promise.allSettled([first, second]);
  assert.equal(results.every((result) => result.status === "rejected" && result.reason instanceof EngineClientError && result.reason.kind === "protocol"), true);
  assert.deepEqual(transport.requestIDs, [1]);
  assert.equal(transport.aborted, true);
});

test("canceled request watchdog aborts a child that withholds its terminal", async () => {
  const transport = new ReactiveTransport((current, frame) => {
    if (frame.type === "request") current.send(started(frame));
  });
  const client = await EngineClient.connect(transport, { cancelTimeoutMs: 25, closeTimeoutMs: 100 });
  const controller = new AbortController();
  await assert.rejects(
    client.list(
      { product: "zia", resource: "locations", fields: [], filters: [], search: "" },
      { signal: controller.signal, onEvent: (event) => {
        if (event.type === "started") controller.abort();
      } },
    ),
    (error: unknown) => error instanceof EngineClientError && error.kind === "transport",
  );
  assert.equal(transport.aborted, true);
});

test("close watchdog aborts a child that keeps stdout open", async () => {
  const transport = new ReactiveTransport(completeRequest, { hangOnClose: true });
  const client = await EngineClient.connect(transport, { closeTimeoutMs: 25 });
  const startedAt = performance.now();
  await assert.rejects(
    client.close(),
    (error: unknown) => error instanceof EngineClientError && error.kind === "transport",
  );
  assert.ok(performance.now() - startedAt < 500);
  assert.equal(transport.aborted, true);
});
