import { createHash, type Hash } from "node:crypto";

import {
  BOOTSTRAP_FRAME_BYTES,
  FRAGMENT_CHUNK_BYTES,
  MAX_SAFE_INTEGER,
  PROTOCOL,
  V1_FRAME_BYTES,
  V1_VERSION,
} from "./constants.ts";
import {
  decodeBootstrapServerFrame,
  decodeCanonicalBase64,
  decodeItemPayload,
  decodeServerFrame,
  encodeBootstrapClientFrame,
  encodeClientFrame,
} from "./codec.ts";
import { errorKind } from "./errors.ts";
import { NdjsonStreamReader } from "./framing.ts";
import type {
  AuthStatusResult,
  BootstrapClientFrame,
  Capability,
  CatalogResource,
  CatalogSummary,
  ClientFrame,
  ClientRequest,
  Completed,
  CompletionResult,
  ConfigStatusResult,
  DiffFieldChange,
  DiffInput,
  DiffRecordRef,
  DiffResource,
  DiffSummary,
  DoctorStatusResult,
  DumpFailure,
  DumpInput,
  DumpSummary,
  EngineManifest,
  EngineManifestResult,
  Filter,
  ItemKind,
  ItemValue,
  Operation,
  OperationFailure,
  Progress,
  ProjectedRecord,
  Ready,
  ResourceGetInput,
  ResourceListInput,
  ResourceReadSummary,
  ResourceSelector,
  ResourceShowInput,
  ServerFrame,
  Started,
  URLLookupInput,
  URLLookupSummary,
  URLClassification,
  Warning,
} from "./types.ts";

export interface EngineTransportExit {
  readonly code: number | null;
  readonly signal: string | null;
}

export interface EngineTransport {
  readonly output: AsyncIterable<Uint8Array>;
  readonly completion?: Promise<EngineTransportExit>;
  write(data: Uint8Array): Promise<void>;
  closeInput(): Promise<void>;
  abort(): void;
}

export interface EngineClientOptions {
  readonly startupTimeoutMs?: number;
  readonly signal?: AbortSignal;
  readonly cancelTimeoutMs?: number;
  readonly closeTimeoutMs?: number;
}

const DEFAULT_STARTUP_TIMEOUT_MS = 10_000;
const DEFAULT_CANCEL_TIMEOUT_MS = 7_000;
const DEFAULT_CLOSE_TIMEOUT_MS = 8_000;

export type EngineClientErrorKind =
  | "unsupported_protocol"
  | "protocol"
  | "transport"
  | "request"
  | "capability_unavailable"
  | "operation"
  | "canceled"
  | "callback"
  | "closed";

export class EngineClientError extends Error {
  readonly kind: EngineClientErrorKind;

  constructor(kind: EngineClientErrorKind, message: string) {
    super(message);
    this.name = "EngineClientError";
    this.kind = kind;
  }
}

export class EngineOperationError extends EngineClientError {
  readonly id: number;
  readonly failure: OperationFailure;

  constructor(id: number, failure: OperationFailure) {
    super("operation", "the engine operation failed");
    this.name = "EngineOperationError";
    this.id = id;
    this.failure = copyFailure(failure);
  }
}

export class EngineCanceledError extends EngineClientError {
  readonly id: number | undefined;

  constructor(id?: number) {
    super("canceled", "the engine operation was canceled");
    this.name = "EngineCanceledError";
    this.id = id;
  }
}

export interface SemanticItem<K extends ItemKind = ItemKind, V extends ItemValue = ItemValue> {
  readonly type: "semantic_item";
  readonly kind: K;
  readonly value: V;
}

export type CatalogItem = SemanticItem<"catalog_resource", CatalogResource>;
export type URLClassificationItem = SemanticItem<"url_classification", URLClassification>;
export type ProjectedRecordItem = SemanticItem<"projected_record", ProjectedRecord>;
export type DiffItem =
  | SemanticItem<"diff_resource", DiffResource>
  | SemanticItem<"diff_added", DiffRecordRef>
  | SemanticItem<"diff_removed", DiffRecordRef>
  | SemanticItem<"diff_field_change", DiffFieldChange>;

export type OperationEvent<I extends SemanticItem = SemanticItem> = Started | I | Progress | Warning;

export interface OperationOptions<I extends SemanticItem = SemanticItem> {
  readonly signal?: AbortSignal;
  // Callbacks are deliberately synchronous. Returning a promise is treated as
  // a callback failure so a slow consumer cannot create an untracked task.
  readonly onEvent?: (event: OperationEvent<I>) => void;
}

export interface OperationResponse<I extends SemanticItem, R extends CompletionResult> {
  readonly id: number;
  readonly items: readonly I[];
  readonly progress: readonly Progress[];
  readonly warnings: readonly DumpFailure[];
  readonly result: R;
}

export type ManifestResponse = OperationResponse<never, EngineManifestResult>;
export type CatalogResponse = OperationResponse<CatalogItem, CatalogSummary>;
export type DoctorResponse = OperationResponse<never, DoctorStatusResult>;
export type AuthStatusResponse = OperationResponse<never, AuthStatusResult>;
export type ConfigStatusResponse = OperationResponse<never, ConfigStatusResult>;
export type URLLookupResponse = OperationResponse<URLClassificationItem, URLLookupSummary>;
export type ResourceReadResponse = OperationResponse<ProjectedRecordItem, ResourceReadSummary>;
export type DumpResponse = OperationResponse<never, DumpSummary>;
export type DiffResponse = OperationResponse<DiffItem, DiffSummary>;

interface OperationDescriptor {
  readonly capability: Capability;
  readonly operation: Operation;
  readonly itemKinds: ReadonlySet<ItemKind>;
  readonly resultKind: CompletionResult["kind"];
  readonly progress: boolean;
  readonly warnings: boolean;
  readonly input: boolean;
}

const EMPTY_ITEM_KINDS = new Set<ItemKind>();

const DESCRIPTORS = {
  manifest: descriptor("engine.manifest", "manifest", [], "engine_manifest", false, false, false),
  catalog: descriptor("catalog.schema", "list", ["catalog_resource"], "catalog_summary", false, false, false),
  doctor: descriptor("status.inspect", "doctor", [], "doctor_status", false, false, false),
  authStatus: descriptor("status.inspect", "auth_status", [], "auth_status", false, false, false),
  configStatus: descriptor("status.inspect", "config_status", [], "config_status", false, false, false),
  lookup: descriptor("zia.url_lookup", "lookup", ["url_classification"], "url_lookup_summary", false, false, true),
  list: descriptor("resources.read", "list", ["projected_record"], "resource_read_summary", false, false, true),
  get: descriptor("resources.read", "get", ["projected_record"], "resource_read_summary", false, false, true),
  show: descriptor("resources.read", "show", ["projected_record"], "resource_read_summary", false, false, true),
  dump: descriptor("dump.write", "dump", [], "dump_summary", true, true, true),
  diff: descriptor(
    "diff.compare",
    "diff",
    ["diff_resource", "diff_added", "diff_removed", "diff_field_change"],
    "diff_summary",
    true,
    false,
    true,
  ),
} as const;

interface QueuedOperation {
  readonly descriptor: OperationDescriptor;
  readonly input: unknown;
  readonly options: OperationOptions;
  readonly resolve: (response: OperationResponse<SemanticItem, CompletionResult>) => void;
  readonly reject: (error: unknown) => void;
}

interface FragmentState {
  readonly itemID: number;
  readonly kind: ItemKind;
  readonly bytes: number;
  readonly expectedChunks: number;
  readonly payload: Uint8Array;
  readonly hash: Hash;
  chunkIndex: number;
  offset: number;
}

interface ActiveOperation {
  readonly id: number;
  readonly descriptor: OperationDescriptor;
  readonly requestInput: unknown;
  readonly options: OperationOptions;
  readonly items: SemanticItem[];
  readonly progress: Progress[];
  readonly warnings: DumpFailure[];
  started: boolean;
  nextSequence: number;
  cancelSent: boolean;
  successCommitted: boolean;
  fragment?: FragmentState;
  callbackError?: EngineClientError;
  cancelTimer?: ReturnType<typeof setTimeout>;
}

function descriptor(
  capability: Capability,
  operation: Operation,
  itemKinds: readonly ItemKind[],
  resultKind: CompletionResult["kind"],
  progress: boolean,
  warnings: boolean,
  input: boolean,
): OperationDescriptor {
  return {
    capability,
    operation,
    itemKinds: itemKinds.length === 0 ? EMPTY_ITEM_KINDS : new Set(itemKinds),
    resultKind,
    progress,
    warnings,
    input,
  };
}

function copyFailure(failure: OperationFailure): OperationFailure {
  if (failure.kind === "missing_credentials" && failure.missing !== undefined) {
    return Object.freeze({ kind: "missing_credentials", missing: Object.freeze([...failure.missing]) });
  }
  return Object.freeze({ kind: failure.kind });
}

function deepFreeze<T>(value: T): T {
  if (typeof value !== "object" || value === null || Object.isFrozen(value)) {
    return value;
  }
  for (const child of Object.values(value as Record<string, unknown>)) {
    deepFreeze(child);
  }
  return Object.freeze(value);
}

function line(data: Uint8Array): Uint8Array {
  const output = new Uint8Array(data.length + 1);
  output.set(data);
  output[data.length] = 0x0a;
  return output;
}

function isThenable(value: unknown): value is PromiseLike<unknown> {
  return typeof value === "object" && value !== null && "then" in value &&
    typeof (value as { then?: unknown }).then === "function";
}

function copySelectors(values: readonly ResourceSelector[]): ResourceSelector[] {
  return values.map((value) => ({ product: value.product, resource: value.resource }));
}

function copyFilters(values: readonly Filter[]): Filter[] {
  return values.map((value) => ({ field: value.field, operator: value.operator, value: value.value }));
}

function snapshotRequestInput<T>(build: () => T): T {
  try {
    return build();
  } catch {
    throw new EngineClientError("request", "engine request input is invalid");
  }
}

function boundedClientTimeout(value: unknown, fallback: number): number {
  if (value === undefined) return fallback;
  if (typeof value !== "number" || !Number.isSafeInteger(value) || value < 1 || value > 300_000) {
    throw new EngineClientError("request", "engine client timeout is invalid");
  }
  return value;
}

function checkedAbortSignal(value: unknown): AbortSignal | undefined {
  if (value === undefined) return undefined;
  if (value === null || typeof value !== "object" ||
      typeof (value as AbortSignal).aborted !== "boolean" ||
      typeof (value as AbortSignal).addEventListener !== "function" ||
      typeof (value as AbortSignal).removeEventListener !== "function") {
    throw new EngineClientError("request", "engine client signal is invalid");
  }
  return value as AbortSignal;
}

class StartupGate {
  private readonly stopped: Promise<never>;
  private readonly signal?: AbortSignal;
  private readonly abortListener?: () => void;
  private readonly timer: ReturnType<typeof setTimeout>;

  constructor(timeoutMs: number, signal?: AbortSignal) {
    this.signal = signal;
    let rejectStopped!: (error: EngineClientError) => void;
    this.stopped = new Promise((_, reject) => {
      rejectStopped = reject;
    });
    this.timer = setTimeout(() => {
      rejectStopped(new EngineClientError("transport", "engine bootstrap timed out"));
    }, timeoutMs);
    if (signal !== undefined) {
      this.abortListener = (): void => {
        rejectStopped(new EngineCanceledError());
      };
      signal.addEventListener("abort", this.abortListener, { once: true });
      if (signal.aborted) this.abortListener();
    }
  }

  async wait<T>(operation: Promise<T>): Promise<T> {
    return await Promise.race([operation, this.stopped]);
  }

  close(): void {
    clearTimeout(this.timer);
    if (this.signal !== undefined && this.abortListener !== undefined) {
      this.signal.removeEventListener("abort", this.abortListener);
    }
  }
}

function warningEqual(left: DumpFailure, right: DumpFailure): boolean {
  return left.product === right.product && left.resource === right.resource &&
    left.phase === right.phase && left.kind === right.kind;
}

function manifestsEqual(left: EngineManifest, right: EngineManifest): boolean {
  if (left.version !== right.version || left.tenant_read_only !== right.tenant_read_only ||
      left.capabilities.length !== right.capabilities.length) {
    return false;
  }
  for (let index = 0; index < left.capabilities.length; index += 1) {
    const a = left.capabilities[index];
    const b = right.capabilities[index];
    if (a.name !== b.name || a.input !== b.input || a.result !== b.result ||
        a.tenant_read_only !== b.tenant_read_only || a.operations.length !== b.operations.length ||
        a.effects.length !== b.effects.length) {
      return false;
    }
    for (let operation = 0; operation < a.operations.length; operation += 1) {
      if (a.operations[operation] !== b.operations[operation]) return false;
    }
    for (let effect = 0; effect < a.effects.length; effect += 1) {
      if (a.effects[effect].kind !== b.effects[effect].kind || a.effects[effect].when !== b.effects[effect].when) {
        return false;
      }
    }
  }
  return true;
}

export class EngineClient {
  readonly ready: Ready;

  private readonly transport: EngineTransport;
  private readonly reader: NdjsonStreamReader;
  private readonly advertised = new Set<string>();
  private readonly queue: QueuedOperation[] = [];
  private readonly cancelTimeoutMs: number;
  private readonly closeTimeoutMs: number;
  private writeTail: Promise<void> = Promise.resolve();
  private pumpPromise: Promise<void> = Promise.resolve();
  private pumping = false;
  private nextID = 1;
  private active?: ActiveOperation;
  private closing = false;
  private closed = false;
  private dead?: EngineClientError;
  private closePromise?: Promise<void>;

  private constructor(
    transport: EngineTransport,
    reader: NdjsonStreamReader,
    ready: Ready,
    cancelTimeoutMs: number,
    closeTimeoutMs: number,
  ) {
    this.transport = transport;
    this.reader = reader;
    this.cancelTimeoutMs = cancelTimeoutMs;
    this.closeTimeoutMs = closeTimeoutMs;
    this.ready = deepFreeze(ready);
    for (const capability of ready.engine.capabilities) {
      for (const operation of capability.operations) {
        this.advertised.add(`${capability.name}\u0000${operation}`);
      }
    }
  }

  static async connect(transport: EngineTransport, options: EngineClientOptions = {}): Promise<EngineClient> {
    if (transport === null || typeof transport !== "object" ||
        typeof transport.write !== "function" || typeof transport.closeInput !== "function" ||
        typeof transport.abort !== "function") {
      throw new EngineClientError("transport", "invalid engine transport");
    }
    let startupGate: StartupGate | undefined;
    try {
      if (options === null || typeof options !== "object") {
        throw new EngineClientError("request", "engine client options are invalid");
      }
      const startupTimeoutMs = boundedClientTimeout(options.startupTimeoutMs, DEFAULT_STARTUP_TIMEOUT_MS);
      const signal = checkedAbortSignal(options.signal);
      if (signal?.aborted === true) {
        throw new EngineCanceledError();
      }
      const cancelTimeoutMs = boundedClientTimeout(options.cancelTimeoutMs, DEFAULT_CANCEL_TIMEOUT_MS);
      const closeTimeoutMs = boundedClientTimeout(options.closeTimeoutMs, DEFAULT_CLOSE_TIMEOUT_MS);
      startupGate = new StartupGate(startupTimeoutMs, signal);
      const reader = new NdjsonStreamReader(transport.output);
      const helloData = await startupGate.wait(reader.readFrame(BOOTSTRAP_FRAME_BYTES));
      if (helloData === null) {
        throw new EngineClientError("transport", "engine output ended before hello");
      }
      const hello = decodeBootstrapServerFrame(helloData);
      if (hello.type === "protocol_error") {
        throw new EngineClientError("protocol", "engine rejected bootstrap before negotiation");
      }
      if (!hello.versions.includes(V1_VERSION)) {
        await startupGate.wait(transport.write(line(encodeBootstrapClientFrame({
          type: "reject",
          protocol: PROTOCOL,
          reason: "unsupported_protocol",
        }))));
        const rejectionData = await startupGate.wait(reader.readFrame(BOOTSTRAP_FRAME_BYTES));
        if (rejectionData === null) {
          throw new EngineClientError("transport", "engine output ended before protocol rejection");
        }
        const rejection = decodeBootstrapServerFrame(rejectionData);
        if (rejection.type !== "protocol_error" || rejection.error.kind !== "unsupported_protocol") {
          throw new EngineClientError("protocol", "engine sent an invalid protocol rejection");
        }
        throw new EngineClientError("unsupported_protocol", "engine and client have no common protocol version");
      }

      const initialize: BootstrapClientFrame = {
        type: "initialize",
        protocol: PROTOCOL,
        version: V1_VERSION,
      };
      await startupGate.wait(transport.write(line(encodeBootstrapClientFrame(initialize))));
      const readyData = await startupGate.wait(reader.readFrame(V1_FRAME_BYTES));
      if (readyData === null) {
        throw new EngineClientError("transport", "engine output ended before ready");
      }
      const ready = decodeServerFrame(readyData);
      if (ready.type === "protocol_error") {
        throw new EngineClientError("protocol", "engine rejected protocol initialization");
      }
      if (ready.type !== "ready") {
        throw new EngineClientError("protocol", "engine did not send ready after initialization");
      }
      return new EngineClient(transport, reader, ready, cancelTimeoutMs, closeTimeoutMs);
    } catch (error) {
      try {
        transport.abort();
      } catch {
        // Preserve the normalized bootstrap failure below.
      }
      if (error instanceof EngineClientError) {
        throw error;
      }
      if (errorKind(error) !== undefined) {
        throw new EngineClientError("protocol", "engine bootstrap output violated the protocol");
      }
      throw new EngineClientError("transport", "engine bootstrap transport failed");
    } finally {
      startupGate?.close();
    }
  }

  async manifest(options: OperationOptions<never> = {}): Promise<ManifestResponse> {
    return this.enqueue(DESCRIPTORS.manifest, undefined, options);
  }

  async catalog(options: OperationOptions<CatalogItem> = {}): Promise<CatalogResponse> {
    return this.enqueue(DESCRIPTORS.catalog, undefined, options);
  }

  async doctor(options: OperationOptions<never> = {}): Promise<DoctorResponse> {
    return this.enqueue(DESCRIPTORS.doctor, undefined, options);
  }

  async authStatus(options: OperationOptions<never> = {}): Promise<AuthStatusResponse> {
    return this.enqueue(DESCRIPTORS.authStatus, undefined, options);
  }

  async configStatus(options: OperationOptions<never> = {}): Promise<ConfigStatusResponse> {
    return this.enqueue(DESCRIPTORS.configStatus, undefined, options);
  }

  async lookup(input: URLLookupInput, options: OperationOptions<URLClassificationItem> = {}): Promise<URLLookupResponse> {
    const copied = snapshotRequestInput<URLLookupInput>(() => ({ urls: [...input.urls] }));
    return this.enqueue(DESCRIPTORS.lookup, copied, options);
  }

  async list(input: ResourceListInput, options: OperationOptions<ProjectedRecordItem> = {}): Promise<ResourceReadResponse> {
    const copied = snapshotRequestInput<ResourceListInput>(() => ({
      product: input.product,
      resource: input.resource,
      fields: [...input.fields],
      filters: copyFilters(input.filters),
      search: input.search,
    }));
    return this.enqueue(DESCRIPTORS.list, copied, options);
  }

  async get(input: ResourceGetInput, options: OperationOptions<ProjectedRecordItem> = {}): Promise<ResourceReadResponse> {
    const copied = snapshotRequestInput<ResourceGetInput>(() => ({
      product: input.product,
      resource: input.resource,
      record_id: input.record_id,
      fields: [...input.fields],
    }));
    return this.enqueue(DESCRIPTORS.get, copied, options);
  }

  async show(input: ResourceShowInput, options: OperationOptions<ProjectedRecordItem> = {}): Promise<ResourceReadResponse> {
    const copied = snapshotRequestInput<ResourceShowInput>(() => ({
      product: input.product,
      resource: input.resource,
      fields: [...input.fields],
    }));
    return this.enqueue(DESCRIPTORS.show, copied, options);
  }

  async dump(input: DumpInput, options: OperationOptions<never> = {}): Promise<DumpResponse> {
    const copied = snapshotRequestInput<DumpInput>(() => ({
      output_dir: input.output_dir,
      products: [...input.products],
      resources: copySelectors(input.resources),
      continue_on_error: input.continue_on_error,
      force: input.force,
    }));
    return this.enqueue(DESCRIPTORS.dump, copied, options);
  }

  async diff(input: DiffInput, options: OperationOptions<DiffItem> = {}): Promise<DiffResponse> {
    const copied = snapshotRequestInput<DiffInput>(() => ({
      old_dir: input.old_dir,
      new_dir: input.new_dir,
      products: [...input.products],
      resources: copySelectors(input.resources),
      ignore_operational: input.ignore_operational,
      allow_partial: input.allow_partial,
    }));
    return this.enqueue(DESCRIPTORS.diff, copied, options);
  }

  close(): Promise<void> {
    if (this.closePromise !== undefined) {
      return this.closePromise;
    }
    this.closing = true;
    const closedError = new EngineClientError("closed", "engine client is closing");
    this.rejectQueued(closedError);
    if (this.active !== undefined) {
      this.requestCancellation(this.active);
    }
    this.closePromise = this.finishClose();
    return this.closePromise;
  }

  private enqueue<I extends SemanticItem, R extends CompletionResult>(
    descriptorValue: OperationDescriptor,
    input: unknown,
    options: OperationOptions<I>,
  ): Promise<OperationResponse<I, R>> {
    if (options === null || typeof options !== "object") {
      return Promise.reject(new EngineClientError("request", "engine operation options are invalid"));
    }
    if (this.dead !== undefined) {
      return Promise.reject(this.dead);
    }
    if (this.closing || this.closed) {
      return Promise.reject(new EngineClientError("closed", "engine client is closed"));
    }
    if (options.signal?.aborted === true) {
      return Promise.reject(new EngineCanceledError());
    }
    const advertised = this.advertised.has(`${descriptorValue.capability}\u0000${descriptorValue.operation}`);
    if (!advertised) {
      return Promise.reject(new EngineClientError(
        "capability_unavailable",
        "the engine did not advertise the requested capability and operation",
      ));
    }

    const promise = new Promise<OperationResponse<SemanticItem, CompletionResult>>((resolve, reject) => {
      this.queue.push({
        descriptor: descriptorValue,
        input,
        options: { signal: options.signal, onEvent: options.onEvent as OperationOptions["onEvent"] },
        resolve,
        reject,
      });
    });
    this.startPump();
    return promise as Promise<OperationResponse<I, R>>;
  }

  private startPump(): void {
    if (this.pumping) return;
    this.pumping = true;
    this.pumpPromise = this.pump();
    void this.pumpPromise;
  }

  private async pump(): Promise<void> {
    try {
      while (this.queue.length > 0 && this.dead === undefined) {
        const queued = this.queue.shift();
        if (queued === undefined) break;
        if (this.closing) {
          queued.reject(new EngineClientError("closed", "engine client is closing"));
          continue;
        }
        if (queued.options.signal?.aborted === true) {
          queued.reject(new EngineCanceledError());
          continue;
        }
        try {
          queued.resolve(await this.runOperation(queued));
        } catch (error) {
          queued.reject(error);
        }
      }
    } finally {
      this.pumping = false;
      if (this.queue.length > 0 && this.dead === undefined && !this.closing) {
        this.startPump();
      }
    }
  }

  private async runOperation(queued: QueuedOperation): Promise<OperationResponse<SemanticItem, CompletionResult>> {
    if (this.nextID > MAX_SAFE_INTEGER) {
      throw new EngineClientError("closed", "engine request ID space is exhausted");
    }
    const id = this.nextID;
    let request: ClientRequest;
    let encoded: Uint8Array;
    try {
      request = this.buildRequest(id, queued.descriptor, queued.input);
      encoded = encodeClientFrame(request);
    } catch (error) {
      if (errorKind(error) !== undefined) {
        throw new EngineClientError("request", "engine request input is invalid");
      }
      throw error;
    }
    this.nextID += 1;

    const state: ActiveOperation = {
      id,
      descriptor: queued.descriptor,
      requestInput: queued.input,
      options: queued.options,
      items: [],
      progress: [],
      warnings: [],
      started: false,
      nextSequence: 1,
      cancelSent: false,
      successCommitted: false,
    };
    this.active = state;
    const requestWrite = this.sendBytes(encoded);
    const abort = (): void => this.requestCancellation(state);
    queued.options.signal?.addEventListener("abort", abort, { once: true });
    if (queued.options.signal?.aborted === true) abort();

    try {
      await requestWrite;
      while (true) {
        const frame = await this.readServerFrame();
        const terminal = this.acceptFrame(state, frame);
        if (terminal !== undefined) {
          return terminal;
        }
      }
    } finally {
      queued.options.signal?.removeEventListener("abort", abort);
      if (state.cancelTimer !== undefined) {
        clearTimeout(state.cancelTimer);
      }
      if (this.active === state) {
        this.active = undefined;
      }
    }
  }

  private buildRequest(id: number, descriptorValue: OperationDescriptor, input: unknown): ClientRequest {
    const common = {
      type: "request" as const,
      id,
      capability: descriptorValue.capability,
      operation: descriptorValue.operation,
    };
    if (!descriptorValue.input) {
      return common as ClientRequest;
    }
    return { ...common, input } as ClientRequest;
  }

  private async readServerFrame(): Promise<ServerFrame> {
    if (this.dead !== undefined) throw this.dead;
    let data: Uint8Array | null;
    try {
      data = await this.reader.readFrame(V1_FRAME_BYTES);
    } catch (error) {
      if (this.dead !== undefined) throw this.dead;
      if (errorKind(error) !== undefined) {
        throw this.failSession(new EngineClientError("protocol", "engine output framing violated the protocol"));
      }
      throw this.failSession(new EngineClientError("transport", "engine output transport failed"));
    }
    if (data === null) {
      throw this.failSession(new EngineClientError("transport", "engine output ended before a request terminal"));
    }
    try {
      return decodeServerFrame(data);
    } catch {
      throw this.failSession(new EngineClientError("protocol", "engine output frame violated the protocol"));
    }
  }

  private acceptFrame(
    state: ActiveOperation,
    frame: ServerFrame,
  ): OperationResponse<SemanticItem, CompletionResult> | undefined {
    if (frame.type === "protocol_error") {
      throw this.failSession(new EngineClientError("protocol", "engine reported a fatal protocol error"));
    }
    if (frame.type === "ready") {
      throw this.failSession(new EngineClientError("protocol", "engine sent ready more than once"));
    }
    if (frame.type === "request_rejected") {
      throw this.failSession(new EngineClientError(
        "protocol",
        state.started || frame.id !== state.id
          ? "engine sent an invalid busy rejection"
          : "engine rejected a serialized request as busy",
      ));
    }
    if (frame.id !== state.id || frame.seq !== state.nextSequence) {
      throw this.failSession(new EngineClientError("protocol", "engine request identity or sequence is invalid"));
    }
    state.nextSequence += 1;

    if (frame.type === "started") {
      if (state.started || frame.seq !== 1 || frame.capability !== state.descriptor.capability ||
          frame.operation !== state.descriptor.operation) {
        throw this.failSession(new EngineClientError("protocol", "engine started the wrong request lifecycle"));
      }
      state.started = true;
      this.notify(state, deepFreeze(frame));
      return undefined;
    }
    if (!state.started) {
      throw this.failSession(new EngineClientError("protocol", "engine emitted a request frame before started"));
    }
    if (state.fragment !== undefined && frame.type !== "item_chunk" && frame.type !== "item_end") {
      throw this.failSession(new EngineClientError("protocol", "engine interleaved a fragmented item"));
    }

    switch (frame.type) {
      case "item":
        this.acceptItem(state, frame.kind, frame.item);
        return undefined;
      case "item_begin":
        this.beginFragment(state, frame.item_id, frame.kind, frame.bytes);
        return undefined;
      case "item_chunk":
        this.acceptChunk(state, frame.item_id, frame.index, frame.data);
        return undefined;
      case "item_end":
        this.endFragment(state, frame.item_id, frame.chunks, frame.sha256);
        return undefined;
      case "progress":
        this.acceptProgress(state, frame);
        return undefined;
      case "warning":
        this.acceptWarning(state, frame);
        return undefined;
      case "completed":
        return this.acceptCompleted(state, frame);
      case "failed":
        this.acceptNonSuccessTerminal(state);
        if (state.callbackError !== undefined) throw state.callbackError;
        throw new EngineOperationError(state.id, frame.error);
      case "canceled":
        this.acceptNonSuccessTerminal(state);
        if (state.callbackError !== undefined) throw state.callbackError;
        throw new EngineCanceledError(state.id);
      default:
        throw this.failSession(new EngineClientError("protocol", "engine emitted an unknown lifecycle frame"));
    }
  }

  private acceptItem(state: ActiveOperation, kind: ItemKind, value: ItemValue): void {
    if (state.fragment !== undefined || !state.descriptor.itemKinds.has(kind)) {
      throw this.failSession(new EngineClientError("protocol", "engine emitted an invalid semantic item kind"));
    }
    this.markSuccessCommitted(state);
    const item = deepFreeze<SemanticItem>({ type: "semantic_item", kind, value });
    this.validateResourceScope(state, item);
    state.items.push(item);
    this.notify(state, item);
  }

  private beginFragment(state: ActiveOperation, itemID: number, kind: ItemKind, bytes: number): void {
    if (state.fragment !== undefined || !state.descriptor.itemKinds.has(kind) || itemID !== state.items.length + 1) {
      throw this.failSession(new EngineClientError("protocol", "engine began an invalid fragmented item"));
    }
    const expectedChunks = Math.ceil(bytes / FRAGMENT_CHUNK_BYTES);
    if (expectedChunks < 1 || expectedChunks > 128) {
      throw this.failSession(new EngineClientError("protocol", "engine fragmented item count is invalid"));
    }
    this.markSuccessCommitted(state);
    state.fragment = {
      itemID,
      kind,
      bytes,
      expectedChunks,
      payload: new Uint8Array(bytes),
      hash: createHash("sha256"),
      chunkIndex: 0,
      offset: 0,
    };
  }

  private acceptChunk(state: ActiveOperation, itemID: number, index: number, data: string): void {
    const fragment = state.fragment;
    if (fragment === undefined || itemID !== fragment.itemID || index !== fragment.chunkIndex) {
      throw this.failSession(new EngineClientError("protocol", "engine fragment chunk order is invalid"));
    }
    let bytes: Uint8Array;
    try {
      bytes = decodeCanonicalBase64(data);
    } catch {
      throw this.failSession(new EngineClientError("protocol", "engine fragment base64 is invalid"));
    }
    const final = index === fragment.expectedChunks - 1;
    const expectedLength = final
      ? fragment.bytes - (fragment.expectedChunks - 1) * FRAGMENT_CHUNK_BYTES
      : FRAGMENT_CHUNK_BYTES;
    if (index >= fragment.expectedChunks || bytes.length !== expectedLength ||
        fragment.offset + bytes.length > fragment.payload.length) {
      throw this.failSession(new EngineClientError("protocol", "engine fragment chunk size is invalid"));
    }
    fragment.payload.set(bytes, fragment.offset);
    fragment.hash.update(bytes);
    fragment.offset += bytes.length;
    fragment.chunkIndex += 1;
  }

  private endFragment(state: ActiveOperation, itemID: number, chunks: number, digest: string): void {
    const fragment = state.fragment;
    if (fragment === undefined || itemID !== fragment.itemID || chunks !== fragment.expectedChunks ||
        fragment.chunkIndex !== fragment.expectedChunks || fragment.offset !== fragment.bytes) {
      throw this.failSession(new EngineClientError("protocol", "engine fragment terminal counters are invalid"));
    }
    const actualDigest = fragment.hash.digest("hex");
    if (actualDigest !== digest) {
      throw this.failSession(new EngineClientError("protocol", "engine fragment digest is invalid"));
    }
    let value: ItemValue;
    try {
      value = decodeItemPayload(fragment.kind, fragment.payload);
    } catch {
      throw this.failSession(new EngineClientError("protocol", "engine fragmented item payload is invalid"));
    }
    state.fragment = undefined;
    this.acceptItem(state, fragment.kind, value);
  }

  private acceptProgress(state: ActiveOperation, progress: Progress): void {
    if (!state.descriptor.progress) {
      throw this.failSession(new EngineClientError("protocol", "engine emitted progress for the wrong operation"));
    }
    const previous = state.progress.at(-1);
    if ((previous === undefined && progress.current !== 1) ||
        (previous !== undefined && (progress.current !== previous.current + 1 || progress.total !== previous.total))) {
      throw this.failSession(new EngineClientError("protocol", "engine progress sequence is invalid"));
    }
    const safeProgress = deepFreeze(progress);
    state.progress.push(safeProgress);
    this.notify(state, safeProgress);
  }

  private acceptWarning(state: ActiveOperation, warning: Warning): void {
    if (!state.descriptor.warnings) {
      throw this.failSession(new EngineClientError("protocol", "engine emitted a warning for the wrong operation"));
    }
    const safeWarning = deepFreeze(warning);
    state.warnings.push(safeWarning.warning);
    this.notify(state, safeWarning);
  }

  private acceptNonSuccessTerminal(state: ActiveOperation): void {
    if (state.fragment !== undefined || state.items.length !== 0) {
      throw this.failSession(new EngineClientError("protocol", "engine failed or canceled after semantic item delivery"));
    }
  }

  private acceptCompleted(
    state: ActiveOperation,
    completed: Completed,
  ): OperationResponse<SemanticItem, CompletionResult> {
    if (state.fragment !== undefined || completed.result.kind !== state.descriptor.resultKind) {
      throw this.failSession(new EngineClientError("protocol", "engine completed with the wrong result kind"));
    }
    this.validateCompletion(state, completed.result);
    if (state.callbackError !== undefined) throw state.callbackError;
    return Object.freeze({
      id: state.id,
      items: Object.freeze([...state.items]),
      progress: Object.freeze([...state.progress]),
      warnings: Object.freeze([...state.warnings]),
      result: deepFreeze(completed.result),
    });
  }

  private validateCompletion(state: ActiveOperation, result: CompletionResult): void {
    if ("stream_items_emitted" in result && result.stream_items_emitted !== state.items.length) {
      throw this.failSession(new EngineClientError("protocol", "engine item count does not match its completion"));
    }
    if (state.descriptor.progress) {
      const last = state.progress.at(-1);
      if (last !== undefined && (last.current !== last.total || state.progress.length !== last.total)) {
        throw this.failSession(new EngineClientError("protocol", "engine successful progress did not complete"));
      }
    } else if (state.progress.length !== 0) {
      throw this.failSession(new EngineClientError("protocol", "engine emitted unexpected progress"));
    }
    if (!state.descriptor.warnings && state.warnings.length !== 0) {
      throw this.failSession(new EngineClientError("protocol", "engine emitted unexpected warnings"));
    }

    if (result.kind === "engine_manifest" && !manifestsEqual(result.manifest, this.ready.engine)) {
      throw this.failSession(new EngineClientError("protocol", "engine manifest changed during the session"));
    }
    if (result.kind === "dump_summary") {
      if (result.failures.length !== state.warnings.length || result.warning_count !== state.warnings.length) {
        throw this.failSession(new EngineClientError("protocol", "engine dump warnings do not match completion"));
      }
      for (let index = 0; index < result.failures.length; index += 1) {
        if (!warningEqual(result.failures[index], state.warnings[index])) {
          throw this.failSession(new EngineClientError("protocol", "engine dump warning order does not match completion"));
        }
      }
      const total = state.progress.at(-1)?.total ?? 0;
      if (state.warnings.length > total || result.resources_written !== total - state.warnings.length) {
        throw this.failSession(new EngineClientError("protocol", "engine dump counters do not match progress"));
      }
    }
    if (result.kind === "diff_summary") {
      this.validateDiffItems(state, result);
      const total = state.progress.at(-1)?.total ?? 0;
      if (result.summary.resources_compared > total) {
        throw this.failSession(new EngineClientError("protocol", "engine diff progress does not match completion"));
      }
    }
  }

  private validateResourceScope(state: ActiveOperation, item: SemanticItem): void {
    if (item.kind !== "projected_record") return;
    const input = state.requestInput as ResourceListInput | ResourceGetInput | ResourceShowInput;
    const value = item.value as ProjectedRecord;
    if (value.product !== input.product || value.resource !== input.resource) {
      throw this.failSession(new EngineClientError("protocol", "engine resource item escaped the request scope"));
    }
  }

  private validateDiffItems(state: ActiveOperation, result: DiffSummary): void {
    let index = 0;
    let resourcesCompared = 0;
    let resourcesWithDrift = 0;
    let recordsAdded = 0;
    let recordsRemoved = 0;
    const changedRecords = new Set<string>();
    const changedFields = new Set<string>();

    while (index < state.items.length) {
      const resourceItem = state.items[index];
      if (resourceItem.kind !== "diff_resource") {
        throw this.failSession(new EngineClientError("protocol", "engine diff item order is invalid"));
      }
      const header = resourceItem.value as DiffResource;
      index += 1;
      resourcesCompared += 1;
      if (header.added > 0 || header.removed > 0 || header.changed_fields > 0) resourcesWithDrift += 1;

      let remaining = state.items.length - index;
      if (header.added > remaining) {
        throw this.failSession(new EngineClientError("protocol", "engine diff added count exceeds remaining items"));
      }
      remaining -= header.added;
      if (header.removed > remaining) {
        throw this.failSession(new EngineClientError("protocol", "engine diff removed count exceeds remaining items"));
      }
      remaining -= header.removed;
      if (header.changed_fields > remaining) {
        throw this.failSession(new EngineClientError("protocol", "engine diff field count exceeds remaining items"));
      }

      for (let added = 0; added < header.added; added += 1) {
        const item = state.items[index];
        if (item?.kind !== "diff_added" || !this.diffRecordMatches(item.value as DiffRecordRef, header)) {
          throw this.failSession(new EngineClientError("protocol", "engine diff added items are invalid"));
        }
        index += 1;
        recordsAdded += 1;
      }
      for (let removed = 0; removed < header.removed; removed += 1) {
        const item = state.items[index];
        if (item?.kind !== "diff_removed" || !this.diffRecordMatches(item.value as DiffRecordRef, header)) {
          throw this.failSession(new EngineClientError("protocol", "engine diff removed items are invalid"));
        }
        index += 1;
        recordsRemoved += 1;
      }
      for (let changed = 0; changed < header.changed_fields; changed += 1) {
        const item = state.items[index];
        if (item?.kind !== "diff_field_change" || !this.diffFieldMatches(item.value as DiffFieldChange, header)) {
          throw this.failSession(new EngineClientError("protocol", "engine diff field items are invalid"));
        }
        const value = item.value as DiffFieldChange;
        const recordIdentity = `${header.product}\u0000${header.resource}\u0000${value.key}`;
        const fieldIdentity = `${recordIdentity}\u0000${value.field}`;
        if (changedFields.has(fieldIdentity)) {
          throw this.failSession(new EngineClientError("protocol", "engine diff field item is duplicated"));
        }
        changedFields.add(fieldIdentity);
        changedRecords.add(recordIdentity);
        index += 1;
      }
    }

    const summary = result.summary;
    if (resourcesCompared !== summary.resources_compared || resourcesWithDrift !== summary.resources_with_drift ||
        recordsAdded !== summary.records_added || recordsRemoved !== summary.records_removed ||
        changedRecords.size !== summary.records_changed || state.items.length !== result.stream_items_emitted) {
      throw this.failSession(new EngineClientError("protocol", "engine diff items do not reconcile with completion"));
    }
  }

  private diffRecordMatches(value: DiffRecordRef, header: DiffResource): boolean {
    if (value.product !== header.product || value.resource !== header.resource) return false;
    if (header.identity.mode === "get_key") return "key" in value;
    if (header.identity.mode === "singleton") return "key" in value && value.key === "singleton";
    return "hash" in value;
  }

  private diffFieldMatches(value: DiffFieldChange, header: DiffResource): boolean {
    if (value.product !== header.product || value.resource !== header.resource) return false;
    if (header.identity.mode === "get_key") return value.key !== "";
    if (header.identity.mode === "singleton") return value.key === "singleton";
    return false;
  }

  private notify(state: ActiveOperation, event: OperationEvent): void {
    if (state.callbackError !== undefined || state.options.onEvent === undefined) return;
    try {
      const returned = state.options.onEvent(event);
      if (isThenable(returned)) {
        void Promise.resolve(returned).catch(() => undefined);
        throw new Error("asynchronous callback");
      }
    } catch {
      state.callbackError = new EngineClientError("callback", "engine event callback failed");
      this.requestCancellation(state);
    }
  }

  private requestCancellation(state: ActiveOperation): void {
    if (state.cancelSent || state.successCommitted || this.dead !== undefined || this.closed) return;
    state.cancelSent = true;
    const cancel: ClientFrame = { type: "cancel", id: state.id };
    let encoded: Uint8Array;
    try {
      encoded = encodeClientFrame(cancel);
    } catch {
      this.failSession(new EngineClientError("protocol", "client could not encode cancellation"));
      return;
    }
    void this.sendBytes(encoded).catch(() => undefined);
    // Protocol v1 has no wire-visible filesystem commit marker. For dump, a
    // late cancel may race after atomic publication while the host is clearing
    // confidential replacement data. The official host bounds pre-commit
    // cancellation itself; the client must not SIGKILL an indeterminate
    // post-commit operation merely because the ordinary watchdog elapsed.
    if (state.descriptor.capability === "dump.write") return;
    state.cancelTimer = setTimeout(() => {
      this.failSession(new EngineClientError("transport", "engine did not terminate a canceled request"));
    }, this.cancelTimeoutMs);
  }

  private markSuccessCommitted(state: ActiveOperation): void {
    if (state.successCommitted) return;
    state.successCommitted = true;
    if (state.cancelTimer !== undefined) {
      clearTimeout(state.cancelTimer);
      state.cancelTimer = undefined;
    }
  }

  private async sendBytes(data: Uint8Array): Promise<void> {
    const payload = line(data);
    const write = this.writeTail.then(async () => {
      if (this.dead !== undefined) throw this.dead;
      await this.transport.write(payload);
    });
    this.writeTail = write.catch(() => undefined);
    try {
      await write;
    } catch (error) {
      if (this.dead !== undefined) throw this.dead;
      if (error instanceof EngineClientError) throw error;
      throw this.failSession(new EngineClientError("transport", "engine input transport failed"));
    }
  }

  private failSession(error: EngineClientError): EngineClientError {
    if (this.dead !== undefined) return this.dead;
    this.dead = error;
    this.rejectQueued(error);
    try {
      this.transport.abort();
    } catch {
      // The original closed error remains authoritative.
    }
    return error;
  }

  private rejectQueued(error: EngineClientError): void {
    for (const queued of this.queue.splice(0)) {
      queued.reject(error);
    }
  }

  private async finishClose(): Promise<void> {
    if (this.active?.descriptor.capability === "dump.write") {
      await this.finishCloseWork();
      return;
    }
    let timer: ReturnType<typeof setTimeout> | undefined;
    const timeout = new Promise<void>((_resolve, reject) => {
      timer = setTimeout(() => {
        this.closed = true;
        reject(this.failSession(new EngineClientError("transport", "engine session did not close in time")));
      }, this.closeTimeoutMs);
    });
    try {
      await Promise.race([this.finishCloseWork(), timeout]);
    } finally {
      if (timer !== undefined) clearTimeout(timer);
    }
  }

  private async finishCloseWork(): Promise<void> {
    await this.writeTail;
    if (this.dead === undefined) {
      try {
        await this.transport.closeInput();
      } catch {
        this.failSession(new EngineClientError("transport", "engine input could not be closed"));
      }
    }
    await this.pumpPromise;
    if (this.dead !== undefined) {
      this.closed = true;
      throw this.dead;
    }

    let extra: Uint8Array | null;
    try {
      extra = await this.reader.readFrame(V1_FRAME_BYTES);
    } catch {
      throw this.failSession(new EngineClientError("transport", "engine output did not close cleanly"));
    }
    if (extra !== null) {
      throw this.failSession(new EngineClientError("protocol", "engine emitted a frame after session shutdown"));
    }
    if (this.transport.completion !== undefined) {
      let exit: EngineTransportExit;
      try {
        exit = await this.transport.completion;
      } catch {
        throw this.failSession(new EngineClientError("transport", "engine process completion failed"));
      }
      if (exit.code !== 0 || exit.signal !== null) {
        throw this.failSession(new EngineClientError("transport", "engine process exited unsuccessfully"));
      }
    }
    this.closed = true;
  }
}
