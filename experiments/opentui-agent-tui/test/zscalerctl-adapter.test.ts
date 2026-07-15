import {describe, expect, test} from "bun:test";

import {
  EngineOperationError,
  WireNumber,
  type CatalogResponse,
  type ResourceListInput,
  type ResourceReadResponse
} from "../../../clients/typescript/src/index.ts";
import {WorkspaceCommandError, type WorkspaceProgressEvent} from "../src/workspace.ts";
import {createZscalerctlWorkspace} from "../src/zscalerctl/adapter.ts";
import {CATALOG_RESPONSE, fakeEngine} from "./helpers.ts";

const context = (signal = new AbortController().signal) => ({signal, emit: () => undefined});
const OPTIONS = {theme: "tokyonight" as const, themeMode: "dark" as const, engine: "/absolute/zscalerctl-engine"};

const CATALOG: CatalogResponse = {
  ...CATALOG_RESPONSE,
  items: [
    {
      type: "semantic_item",
      kind: "catalog_resource",
      value: {
        product: "zia",
        name: "locations",
        shape: "list",
        operations: ["list", "get"],
        get_key: "id",
        fields: [{name: "name", classification: "tenant_configuration", allowed_modes: ["standard", "share", "paranoid"], fields: []}]
      }
    },
    {
      type: "semantic_item",
      kind: "catalog_resource",
      value: {product: "ztw", name: "advanced-settings", shape: "singleton", operations: ["show"], fields: []}
    },
    {
      type: "semantic_item",
      kind: "catalog_resource",
      value: {product: "zia", name: "auth-settings", shape: "singleton", operations: ["list"], fields: []}
    }
  ],
  result: {kind: "catalog_summary", resources: 3, stream_items_emitted: 3}
};

describe("zscalerctl workspace adapter", () => {
  test("passes only explicit process policy and bootstraps with config-free catalog discovery", async () => {
    let processOptions: unknown;
    const workspace = createZscalerctlWorkspace(
      {...OPTIONS, profile: "lab", timeout: "15s", redaction: "share", noCache: true},
      async options => {
        processOptions = options;
        return fakeEngine({catalog: async () => CATALOG});
      }
    );
    const result = await workspace.connect!(context());
    expect(processOptions).toMatchObject({
      executable: OPTIONS.engine,
      profile: "lab",
      timeout: "15s",
      redaction: "share",
      noCache: true
    });
    expect(result.announcement.title).toBe("Engine connected");
    expect(result.context?.records).toBe(3);
    expect(result.context?.effects).toBe("none");
    await workspace.close();
  });

  test("turns the catalog into safe list/show picker commands", async () => {
    const workspace = createZscalerctlWorkspace(OPTIONS, async () => fakeEngine({catalog: async () => CATALOG}));
    await workspace.connect!(context());
    const result = await workspace.execute!("/catalog", context());
    expect(result.picker?.items.map(item => ({id: item.id, command: item.command}))).toEqual([
      {id: "zia/locations", command: "/list zia 'locations'"},
      {id: "zia/auth-settings", command: "/list zia 'auth-settings'"},
      {id: "ztw/advanced-settings", command: "/show ztw 'advanced-settings'"}
    ]);
    expect(result.picker?.title).toBe("Zscaler resource map");
    expect(result.picker?.scopes).toEqual([
      {id: "zia", label: "ZIA", count: 2},
      {id: "ztw", label: "ZTW", count: 1}
    ]);
    expect(result.picker?.items.map(item => item.scopeId)).toEqual(["zia", "zia", "ztw"]);
    expect(result.context?.records).toBe(3);
    await workspace.close();
  });

  test("maps typed reads and preserves exact wire numbers", async () => {
    let request: ResourceListInput | undefined;
    const progress: WorkspaceProgressEvent[] = [];
    const response: ResourceReadResponse = {
      id: 2,
      items: [{
        type: "semantic_item",
        kind: "projected_record",
        value: {product: "zia", resource: "locations", record: {id: new WireNumber("900719925474099312345"), name: "HQ"}}
      }],
      progress: [],
      warnings: [],
      result: {kind: "resource_read_summary", records: 1, stream_items_emitted: 1}
    };
    const workspace = createZscalerctlWorkspace(OPTIONS, async () => fakeEngine({
      list: async (input, options) => {
        request = input;
        options?.onEvent?.({
          type: "progress",
          id: 2,
          seq: 2,
          phase: "resource_started",
          current: 1,
          total: 3,
          product: "zia",
          resource: "locations"
        });
        options?.onEvent?.({
          type: "progress",
          id: 2,
          seq: 3,
          phase: "resource_started",
          current: 3,
          total: 3,
          product: "zpa",
          resource: "app-segments"
        });
        return response;
      }
    }));
    await workspace.connect!(context());
    const result = await workspace.execute!("/list zia locations --fields id,name --filter 'name=foo~bar' --search hq", {
      signal: new AbortController().signal,
      emit: event => progress.push(event)
    });
    expect(request).toEqual({
      product: "zia",
      resource: "locations",
      fields: ["id", "name"],
      filters: [{field: "name", operator: "exact", value: "foo~bar"}],
      search: "hq"
    });
    const data = result.data as {records: Array<{id: WireNumber}>};
    expect(data.records[0]?.id).toBeInstanceOf(WireNumber);
    expect(data.records[0]?.id.lexeme).toBe("900719925474099312345");
    expect(progress).toEqual([
      {kind: "progress", completed: 0, total: 3, message: "zia/locations"},
      {kind: "progress", completed: 2, total: 3, message: "zpa/app-segments"}
    ]);
    await workspace.close();
  });

  test("exposes canonical missing variable names but never backend prose", async () => {
    const workspace = createZscalerctlWorkspace(OPTIONS, async () => fakeEngine({
      list: async () => {
        throw new EngineOperationError(1, {
          kind: "missing_credentials",
          missing: ["ZSCALERCTL_CLIENT_ID", "unsafe\u001b[31m"]
        });
      }
    }));
    await workspace.connect!(context());
    try {
      await workspace.execute!("/list zia locations", context());
      throw new Error("expected missing credentials");
    } catch (error) {
      expect(error).toBeInstanceOf(WorkspaceCommandError);
      const failure = error as WorkspaceCommandError;
      expect(failure.title).toBe("Credentials required");
      expect(failure.details).toEqual(["Missing: ZSCALERCTL_CLIENT_ID"]);
      expect(failure.message).not.toContain("unsafe");
    }
    await workspace.close();
  });

  test("is cancellation-aware and closes the child exactly once", async () => {
    let closes = 0;
    const workspace = createZscalerctlWorkspace(OPTIONS, async () => fakeEngine({close: async () => { closes += 1; }}));
    await workspace.connect!(context());
    const controller = new AbortController();
    controller.abort();
    expect(workspace.execute!("/manifest", context(controller.signal))).rejects.toMatchObject({
      title: "Operation canceled",
      canceled: true
    });
    await workspace.close();
    await workspace.close();
    expect(closes).toBe(1);
  });
});
