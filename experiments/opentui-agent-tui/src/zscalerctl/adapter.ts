import {
  EngineCanceledError,
  EngineClientError,
  EngineOperationError,
  WireNumber,
  spawnEngine,
  type CatalogResource,
  type CatalogResponse,
  type DiffResponse,
  type EngineClient,
  type OperationOptions,
  type ProjectedRecordItem,
  type ResourceGetInput,
  type ResourceListInput,
  type ResourceReadResponse,
  type ResourceShowInput,
  type URLLookupInput,
  type WireValue
} from "../../../../clients/typescript/src/index.ts";

import type {ExperimentOptions} from "../options.ts";
import type {ContextState, Tone} from "../model.ts";
import {
  WorkspaceCommandError,
  type WorkspaceAdapter,
  type WorkspaceExecutionContext,
  type WorkspacePicker,
  type WorkspaceResult
} from "../workspace.ts";
import {
  CommandSyntaxError,
  parseCommand,
  ZSCALERCTL_COMMANDS,
  type ParsedCommand
} from "./commands.ts";
import {
  summarizeAuth,
  summarizeCatalog,
  summarizeConfig,
  summarizeDiff,
  summarizeDoctor,
  summarizeLookup,
  summarizeManifest,
  summarizeRead
} from "./summaries.ts";

export type EnginePort = Pick<EngineClient,
  "ready" | "manifest" | "catalog" | "doctor" | "authStatus" | "configStatus" |
  "lookup" | "list" | "get" | "show" | "diff" | "close">;

export type SpawnEnginePort = (options: Parameters<typeof spawnEngine>[0]) => Promise<EnginePort>;

interface LinkedSignal {
  readonly signal: AbortSignal;
  dispose(): void;
}

function linkedSignal(...signals: readonly AbortSignal[]): LinkedSignal {
  const controller = new AbortController();
  const abort = () => controller.abort();
  for (const signal of signals) {
    if (signal.aborted) controller.abort();
    else signal.addEventListener("abort", abort, {once: true});
  }
  return {
    signal: controller.signal,
    dispose() {
      for (const signal of signals) signal.removeEventListener("abort", abort);
    }
  };
}

function wireValue(value: unknown): WireValue {
  if (value === null || typeof value === "string" || typeof value === "boolean" || value instanceof WireNumber) return value;
  if (typeof value === "number") {
    if (!Number.isSafeInteger(value)) throw new WorkspaceCommandError({title: "Invalid engine data", message: "The engine returned a number outside the exact wire-value contract."});
    return new WireNumber(String(value));
  }
  if (Array.isArray(value)) return value.map(item => wireValue(item));
  if (typeof value === "object") {
    const output: Record<string, WireValue> = Object.create(null) as Record<string, WireValue>;
    for (const [name, item] of Object.entries(value)) {
      if (item !== undefined) output[name] = wireValue(item);
    }
    return output;
  }
  throw new WorkspaceCommandError({title: "Invalid engine data", message: "The engine returned a value outside the structured wire contract."});
}

function contextState(options: {
  readonly scope: string;
  readonly records: number;
  readonly countLabel?: string;
  readonly effects: string;
  readonly label: string;
  readonly tone?: "complete" | "error";
}): ContextState {
  return {
    connection: options.tone === "error" ? "error" : "connected",
    transport: "local child-process stdio v1",
    authority: "tenant read-only",
    scope: options.scope,
    records: options.records,
    ...(options.countLabel === undefined ? {} : {countLabel: options.countLabel}),
    effects: options.effects,
    operation: {status: options.tone ?? "complete", label: options.label}
  };
}

function operationOptions(context: WorkspaceExecutionContext): OperationOptions {
  return {
    signal: context.signal,
    onEvent: event => {
      if (event.type === "progress") {
        context.emit({
          kind: "progress",
          completed: event.current - 1,
          total: event.total,
          message: `${event.product}/${event.resource}`
        });
      }
    }
  };
}

function quoteToken(value: string): string {
  return `'${value.replace(/\\/gu, "\\\\").replace(/'/gu, "\\'")}'`;
}

function catalogFieldNames(fields: CatalogResource["fields"]): readonly string[] {
  const output: string[] = [];
  const visit = (items: CatalogResource["fields"]): void => {
    for (const field of items) {
      output.push(field.name);
      visit(field.fields);
    }
  };
  visit(fields);
  return output;
}

function catalogMatches(resource: CatalogResource, command: Extract<ParsedCommand, {kind: "catalog"}>): boolean {
  if (command.product !== undefined && resource.product !== command.product) return false;
  if (command.query.length === 0) return true;
  const haystack = `${resource.product} ${resource.name} ${resource.shape} ${resource.operations.join(" ")} ${catalogFieldNames(resource.fields).join(" ")}`.toLowerCase();
  return command.query.split(/\s+/u).filter(Boolean).every(term => haystack.includes(term));
}

const PRODUCT_PRESENTATION: Readonly<Record<CatalogResource["product"], {
  readonly short: string;
  readonly name: string;
  readonly order: number;
}>> = Object.freeze({
  zia: {short: "ZIA", name: "Internet Access", order: 0},
  zpa: {short: "ZPA", name: "Private Access", order: 1},
  zcc: {short: "ZCC", name: "Client Connector", order: 2},
  ztw: {short: "ZTW", name: "Workload", order: 3},
  zidentity: {short: "ZIDENTITY", name: "Identity", order: 4}
});

function catalogPicker(resources: readonly CatalogResource[], initialQuery = ""): WorkspacePicker {
  const readable = resources
    .filter(resource => resource.operations.includes("show") || resource.operations.includes("list"))
    .map((resource, index) => ({resource, index}))
    .sort((left, right) => {
      const byProduct = PRODUCT_PRESENTATION[left.resource.product].order - PRODUCT_PRESENTATION[right.resource.product].order;
      return byProduct === 0 ? left.index - right.index : byProduct;
    })
    .map(entry => entry.resource);
  const counts = new Map<CatalogResource["product"], number>();
  for (const resource of readable) counts.set(resource.product, (counts.get(resource.product) ?? 0) + 1);
  const scopes = Object.entries(PRODUCT_PRESENTATION)
    .map(([id, presentation]) => ({id: id as CatalogResource["product"], ...presentation}))
    .sort((left, right) => left.order - right.order)
    .flatMap(scope => {
      const count = counts.get(scope.id) ?? 0;
      return count === 0 ? [] : [{id: scope.id, label: scope.short, count}];
    });
  const onlyScope = scopes.length === 1 ? scopes[0] : undefined;
  return {
    title: onlyScope === undefined ? "Zscaler resource map" : `${onlyScope.label} resource map`,
    placeholder: "Search resources, operations, or projected fields…",
    instruction: "Choose a read surface to inspect sanitized tenant data.",
    emptyMessage: "No readable resources in this product match the search.",
    scopeLabel: "Product",
    scopes,
    initialQuery,
    items: readable.map(resource => {
      const fields = catalogFieldNames(resource.fields);
      const presentation = PRODUCT_PRESENTATION[resource.product];
      return {
        id: `${resource.product}/${resource.name}`,
        title: resource.name,
        description: `${resource.operations.join(" · ")} · ${fields.length} projected field${fields.length === 1 ? "" : "s"}`,
        searchText: fields.join(" "),
        category: `${presentation.short} · ${presentation.name}`,
        scopeId: resource.product,
        badge: resource.shape,
        command: resource.operations.includes("show")
          ? `/show ${resource.product} ${quoteToken(resource.name)}`
          : `/list ${resource.product} ${quoteToken(resource.name)}`
      };
    })
  };
}

function catalogData(resources: readonly CatalogResource[]): WireValue {
  return wireValue({resources});
}

function catalogResult(response: CatalogResponse, command: Extract<ParsedCommand, {kind: "catalog"}>): WorkspaceResult {
  const productResources = response.items.map(item => item.value).filter(resource => command.product === undefined || resource.product === command.product);
  const matched = productResources.filter(resource => catalogMatches(resource, command));
  return {
    announcement: {
      title: "Resource catalog",
      body: [
        `${matched.length} of ${response.result.resources} projected resources match.`,
        "Choose a row to run its default read, or keep typing to narrow the catalog."
      ],
      tone: "info",
      summary: summarizeCatalog(matched, response.result.resources)
    },
    data: catalogData(matched),
    context: contextState({scope: command.product === undefined ? "catalog" : `catalog/${command.product}`, records: matched.length, countLabel: "Resources", effects: "none", label: `${matched.length} resources discovered`}),
    picker: catalogPicker(productResources, command.query)
  };
}

function readResult(
  response: ResourceReadResponse,
  product: string,
  resource: string,
  summary: WorkspaceResult["announcement"]["summary"]
): WorkspaceResult {
  const records = response.items.map(item => item.value.record);
  return {
    announcement: {
      title: `Read ${product}/${resource}`,
      body: [`${response.result.records} projected record${response.result.records === 1 ? "" : "s"} returned.`],
      tone: "success",
      summary
    },
    data: wireValue({records}),
    context: contextState({
      scope: `${product}/${resource}`,
      records: response.result.records,
      countLabel: "Records",
      effects: "network access; configuration-dependent local reads/process execution",
      label: `${response.result.records} records projected`
    })
  };
}

function diffData(response: DiffResponse): WireValue {
  return wireValue({summary: response.result, changes: response.items.map(item => ({kind: item.kind, ...item.value}))});
}

function diffResult(response: DiffResponse): WorkspaceResult {
  const summary = response.result.summary;
  const partial = response.result.old.partial || response.result.new.partial;
  return {
    announcement: {
      title: partial
        ? response.result.has_drift ? "Partial comparison: drift detected" : "Partial comparison complete"
        : response.result.has_drift ? "Drift detected" : "No drift detected",
      body: [
        `${summary.resources_compared} resources compared · ${summary.records_added} added · ${summary.records_removed} removed · ${summary.records_changed} changed`,
        ...(partial ? ["At least one dump was partial; uncollected resources were not compared."] : [])
      ],
      tone: partial || response.result.has_drift ? "warning" : "success",
      summary: summarizeDiff(response.result)
    },
    data: diffData(response),
    context: contextState({scope: "local dump diff", records: response.items.length, countLabel: "Diff items", effects: "local filesystem read", label: `${summary.resources_compared} resources compared`})
  };
}

function simpleResult(options: {
  readonly title: string;
  readonly body: readonly string[];
  readonly tone: Tone;
  readonly data: unknown;
  readonly scope: string;
  readonly records?: number;
  readonly countLabel?: string;
  readonly effects: string;
  readonly summary?: WorkspaceResult["announcement"]["summary"];
}): WorkspaceResult {
  return {
    announcement: {
      title: options.title,
      body: options.body,
      tone: options.tone,
      ...(options.summary === undefined ? {} : {summary: options.summary})
    },
    data: wireValue(options.data),
    context: contextState({scope: options.scope, records: options.records ?? 1, countLabel: options.countLabel, effects: options.effects, label: options.title.toLowerCase()})
  };
}

function syntaxFailure(error: CommandSyntaxError): WorkspaceCommandError {
  return new WorkspaceCommandError({
    title: "Invalid command",
    message: error.message,
    tone: "warning",
    details: error.usage === undefined ? [] : [`Usage: ${error.usage}`]
  });
}

const OPERATION_FAILURES: Readonly<Record<string, {readonly title: string; readonly message: string}>> = {
  usage: {title: "Invalid request", message: "The engine rejected the request shape."},
  unsupported_capability: {title: "Capability unavailable", message: "This engine does not support that capability."},
  unsupported_operation: {title: "Operation unavailable", message: "This engine does not support that operation."},
  unknown_resource: {title: "Unknown resource", message: "The requested resource is not in the engine catalog."},
  not_found: {title: "Record not found", message: "The requested record does not exist or is not visible."},
  invalid_resource_id: {title: "Invalid resource ID", message: "The supplied record identifier is invalid."},
  live_access_failed: {title: "Live access failed", message: "The engine could not complete the Zscaler read."},
  deadline_exceeded: {title: "Request timed out", message: "The configured request deadline was exceeded."},
  invalid_config: {title: "Invalid configuration", message: "The selected engine configuration is invalid."},
  invalid_proxy_config: {title: "Invalid proxy configuration", message: "The selected proxy configuration is invalid."},
  unsupported_resource: {title: "Resource unavailable", message: "The tenant or SDK does not support this resource."},
  response_too_large: {title: "Response too large", message: "The engine refused a response beyond its safety limits."},
  internal: {title: "Engine failure", message: "The engine encountered an internal error."}
};

export function normalizedFailure(error: unknown, signal: AbortSignal): WorkspaceCommandError {
  if (error instanceof WorkspaceCommandError) return error;
  if (error instanceof CommandSyntaxError) return syntaxFailure(error);
  if (error instanceof EngineCanceledError) {
    return new WorkspaceCommandError({title: "Operation canceled", message: "The engine acknowledged cancellation.", tone: "warning", canceled: true});
  }
  if (error instanceof EngineOperationError) {
    if (error.failure.kind === "missing_credentials") {
      const missing = (error.failure.missing ?? []).filter(name => /^ZSCALERCTL_[A-Z0-9_]{1,96}$/u.test(name));
      return new WorkspaceCommandError({
        title: "Credentials required",
        message: "Set the required ZSCALERCTL_* variables in the engine environment and retry.",
        tone: "warning",
        details: missing.map(name => `Missing: ${name}`)
      });
    }
    const detail = OPERATION_FAILURES[error.failure.kind] ?? {title: "Engine failure", message: "The engine operation failed."};
    return new WorkspaceCommandError({title: detail.title, message: detail.message});
  }
  if (error instanceof EngineClientError) {
    const details: Readonly<Record<string, {readonly title: string; readonly message: string}>> = {
      unsupported_protocol: {title: "Protocol mismatch", message: "The engine and client do not share a supported protocol version."},
      protocol: {title: "Protocol failure", message: "The engine violated the negotiated stdio contract."},
      transport: {title: "Engine unavailable", message: "The local engine process could not be reached or exited unexpectedly."},
      request: {title: "Invalid request", message: "The local client rejected the request."},
      capability_unavailable: {title: "Capability unavailable", message: "The connected engine did not advertise this operation."},
      callback: {title: "Consumer failure", message: "The local event consumer rejected an engine event."},
      closed: {title: "Engine closed", message: "The local engine session is closed."},
      canceled: {title: "Operation canceled", message: "The engine acknowledged cancellation."},
      operation: {title: "Engine failure", message: "The engine operation failed."}
    };
    const detail = details[error.kind] ?? {title: "Engine failure", message: "The engine client failed."};
    return new WorkspaceCommandError({title: detail.title, message: detail.message, tone: error.kind === "canceled" ? "warning" : "danger", canceled: error.kind === "canceled"});
  }
  if (signal.aborted) {
    return new WorkspaceCommandError({title: "Operation canceled", message: "The local operation was canceled.", tone: "warning", canceled: true});
  }
  return new WorkspaceCommandError({title: "Operation failed", message: "The local operation failed unexpectedly."});
}

export function createZscalerctlWorkspace(
  options: ExperimentOptions & {readonly engine: string},
  spawn: SpawnEnginePort = spawnEngine
): WorkspaceAdapter {
  const lifecycle = new AbortController();
  let client: EnginePort | undefined;
  let catalog: CatalogResponse | undefined;
  let closed = false;

  const initial: WorkspaceAdapter["initial"] = {
    data: {status: "connecting", transport: "stdio"},
    context: {
      connection: "connecting",
      transport: "local child-process stdio",
      authority: "tenant read-only",
      scope: "engine bootstrap",
      records: 0,
      effects: "local process execution",
      operation: {status: "running", label: "negotiating stdio v1"}
    },
    announcement: {
      title: "Connecting to zscalerctl engine",
      body: ["Negotiating the immutable stdio protocol, then loading the config-free resource catalog."],
      tone: "info"
    }
  };

  async function connect(context: WorkspaceExecutionContext): Promise<WorkspaceResult> {
    if (closed) throw new WorkspaceCommandError({title: "Engine closed", message: "The local engine session is closed."});
    if (client !== undefined && catalog !== undefined) {
      return simpleResult({
        title: "Engine connected",
        body: ["The existing local stdio session is ready."],
        tone: "success",
        data: {status: "ready"},
        scope: "catalog",
        records: catalog.result.resources,
        countLabel: "Resources",
        effects: "none",
        summary: summarizeCatalog(catalog.items.map(item => item.value), catalog.result.resources, client.ready.engine.capabilities.length)
      });
    }
    const linked = linkedSignal(context.signal, lifecycle.signal);
    let created: EnginePort | undefined;
    try {
      created = await spawn({
        executable: options.engine,
        signal: linked.signal,
        ...(options.profile === undefined ? {} : {profile: options.profile}),
        ...(options.config === undefined ? {} : {config: options.config}),
        ...(options.timeout === undefined ? {} : {timeout: options.timeout}),
        ...(options.redaction === undefined ? {} : {redaction: options.redaction}),
        ...(options.noCache === undefined ? {} : {noCache: options.noCache})
      });
      if (closed || linked.signal.aborted) {
        await created.close();
        throw new EngineCanceledError();
      }
      client = created;
      catalog = await created.catalog({signal: linked.signal});
      return {
        announcement: {
          title: "Engine connected",
          body: [
            `${created.ready.server.name} ${created.ready.server.version} · stdio v${created.ready.version} · ${created.ready.engine.capabilities.length} capabilities`,
            `${catalog.result.resources} projected resources discovered without loading tenant credentials.`
          ],
          tone: "success",
          summary: summarizeCatalog(
            catalog.items.map(item => item.value),
            catalog.result.resources,
            created.ready.engine.capabilities.length
          )
        },
        data: catalogData(catalog.items.map(item => item.value)),
        context: contextState({scope: "catalog", records: catalog.result.resources, countLabel: "Resources", effects: "none", label: `${catalog.result.resources} resources discovered`})
      };
    } catch (error) {
      if (created !== undefined && client === created) client = undefined;
      catalog = undefined;
      if (created !== undefined) {
        try { await created.close(); } catch { /* Preserve the authoritative connection failure. */ }
      }
      throw normalizedFailure(error, linked.signal);
    } finally {
      linked.dispose();
    }
  }

  async function execute(input: string, context: WorkspaceExecutionContext): Promise<WorkspaceResult> {
    const connected = client;
    if (connected === undefined || closed) throw new WorkspaceCommandError({title: "Engine unavailable", message: "Connect the local engine before issuing commands."});
    const linked = linkedSignal(context.signal, lifecycle.signal);
    try {
      if (linked.signal.aborted) throw new EngineCanceledError();
      const command = parseCommand(input);
      const operationContext = {...context, signal: linked.signal};
      const requestOptions = operationOptions(operationContext);
      switch (command.kind) {
        case "manifest": {
          const response = await connected.manifest(requestOptions);
          return simpleResult({
            title: "Engine capabilities",
            body: [`${response.result.manifest.capabilities.length} tenant-read-only capabilities advertised.`],
            tone: "info",
            data: response.result.manifest,
            scope: "engine manifest",
            records: response.result.manifest.capabilities.length,
            countLabel: "Capabilities",
            effects: "none",
            summary: summarizeManifest(response.result.manifest)
          });
        }
        case "catalog": {
          catalog ??= await connected.catalog(requestOptions);
          return catalogResult(catalog, command);
        }
        case "doctor": {
          const response = await connected.doctor(requestOptions);
          return simpleResult({title: "Operational readiness", body: [`Status: ${response.result.status.status}`], tone: response.result.status.status.toLowerCase() === "ok" ? "success" : "warning", data: response.result.status, scope: "status/doctor", effects: "configuration-dependent local reads/process execution", summary: summarizeDoctor(response.result.status)});
        }
        case "auth": {
          const response = await connected.authStatus(requestOptions);
          return simpleResult({title: "Authentication readiness", body: [`Credentials: ${response.result.status.credentials}`], tone: response.result.status.credentials === "configured" ? "success" : "warning", data: response.result.status, scope: "status/auth", effects: "configuration-dependent local reads/process execution", summary: summarizeAuth(response.result.status)});
        }
        case "config": {
          const response = await connected.configStatus(requestOptions);
          return simpleResult({title: "Configuration metadata", body: [`Source: ${response.result.status.source}`], tone: "info", data: response.result.status, scope: "status/config", effects: "configuration-dependent local reads/process execution", summary: summarizeConfig(response.result.status)});
        }
        case "lookup": {
          const request: URLLookupInput = {urls: command.urls};
          const response = await connected.lookup(request, requestOptions);
          return simpleResult({title: "URL classification", body: [`${response.result.classifications} classification${response.result.classifications === 1 ? "" : "s"} returned.`], tone: "success", data: {classifications: response.items.map(item => item.value)}, scope: "zia/url-lookup", records: response.result.classifications, countLabel: "Classifications", effects: "network access; configuration-dependent local reads/process execution", summary: summarizeLookup(command.urls.length)});
        }
        case "list": {
          const request: ResourceListInput = {product: command.product, resource: command.resource, fields: command.fields, filters: command.filters, search: command.search};
          return readResult(
            await connected.list(request, requestOptions as OperationOptions<ProjectedRecordItem>),
            command.product,
            command.resource,
            summarizeRead({operation: "list", fields: command.fields, filterCount: command.filters.length, hasSearch: command.search.length > 0})
          );
        }
        case "get": {
          const request: ResourceGetInput = {product: command.product, resource: command.resource, record_id: command.recordID, fields: command.fields};
          return readResult(
            await connected.get(request, requestOptions as OperationOptions<ProjectedRecordItem>),
            command.product,
            command.resource,
            summarizeRead({operation: "get", fields: command.fields})
          );
        }
        case "show": {
          const request: ResourceShowInput = {product: command.product, resource: command.resource, fields: command.fields};
          return readResult(
            await connected.show(request, requestOptions as OperationOptions<ProjectedRecordItem>),
            command.product,
            command.resource,
            summarizeRead({operation: "show", fields: command.fields})
          );
        }
        case "diff": return diffResult(await connected.diff({
          old_dir: command.oldDirectory,
          new_dir: command.newDirectory,
          products: command.products,
          resources: command.resources,
          ignore_operational: command.ignoreOperational,
          allow_partial: command.allowPartial
        }, requestOptions));
      }
    } catch (error) {
      throw normalizedFailure(error, linked.signal);
    } finally {
      linked.dispose();
    }
  }

  return {
    id: "zscalerctl-stdio-v1",
    initial,
    commands: ZSCALERCTL_COMMANDS,
    connect,
    execute,
    async close(): Promise<void> {
      if (closed) return;
      closed = true;
      lifecycle.abort();
      const current = client;
      client = undefined;
      catalog = undefined;
      if (current !== undefined) await current.close();
    }
  };
}
