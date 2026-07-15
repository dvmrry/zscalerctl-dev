import {afterEach, describe, expect, test} from "bun:test";
import {testRender} from "@opentui/react/test-utils";
import {act, createElement} from "react";

import {App} from "../src/App.tsx";
import {
  FIXTURE_WORKSPACE_ADAPTER,
  WorkspaceCommandError,
  type WorkspaceAdapter,
  type WorkspaceResult,
  type WorkspaceSnapshot
} from "../src/workspace.ts";

const renderers: Array<{destroy(): void}> = [];

afterEach(async () => {
  await act(async () => {
    for (const renderer of renderers.splice(0)) renderer.destroy();
  });
});

async function interact(callback: () => void | Promise<void>, flush: () => Promise<void>): Promise<void> {
  await act(async () => {
    await callback();
    await new Promise(resolve => setTimeout(resolve, 100));
    await flush();
  });
}

describe("OpenTUI shell interactions", () => {
  test("previews grouped search results, commits with Enter, and cancels with Escape", async () => {
    const setup = await testRender(createElement(App, {initialMode: "dark", initialTheme: "tokyonight"}), {
      width: 120,
      height: 36
    });
    renderers.push(setup.renderer);
    await setup.flush();

    await interact(() => setup.mockInput.pressTab({shift: true}), setup.flush);
    await interact(() => setup.mockInput.pressKey("f", {ctrl: true}), setup.flush);
    expect(setup.captureCharFrame()).toContain("Find in structured data");

    await interact(() => setup.mockInput.typeText("서울"), setup.flush);
    const searchFrame = setup.captureCharFrame();
    expect(searchFrame).toContain("Seoul Branch");
    expect(searchFrame).toContain("Address › City");
    expect(searchFrame).toContain("Enter reveal");

    await interact(() => setup.mockInput.pressEscape(), setup.flush);
    const canceledFrame = setup.captureCharFrame();
    expect(canceledFrame).not.toContain("Find in structured data");
    expect(canceledFrame).not.toContain("records › Seoul Branch › address › city");

    await interact(() => setup.mockInput.pressKey("f", {ctrl: true}), setup.flush);
    await interact(() => setup.mockInput.pressEnter(), setup.flush);
    expect(setup.captureCharFrame()).not.toContain("Find in structured data");
  });

  test("keeps the picker usable after a narrow resize", async () => {
    const setup = await testRender(createElement(App, {initialMode: "dark", initialTheme: "tokyonight"}), {
      width: 80,
      height: 24
    });
    renderers.push(setup.renderer);
    await setup.flush();
    await interact(() => setup.resize(50, 12), setup.flush);
    await interact(() => setup.mockInput.typeText("/find"), setup.flush);
    await interact(() => setup.mockInput.pressEnter(), setup.flush);
    await interact(() => setup.mockInput.typeText("500"), setup.flush);

    const frame = setup.captureCharFrame();
    expect(frame).toContain("Find in structured data");
    expect(frame).toContain("Raleigh Headquarters");
    expect(frame.split("\n").every(line => [...line].length <= 50)).toBe(true);
  });

  test("keeps the composer roomy when the context rail narrows the conversation", async () => {
    const setup = await testRender(createElement(App, {initialMode: "dark", initialTheme: "tokyonight"}), {
      width: 121,
      height: 30
    });
    renderers.push(setup.renderer);
    await setup.flush();

    const lines = setup.captureCharFrame().split("\n");
    const promptRow = lines.findIndex(line => line.includes("Ask about tenant configuration"));
    const statusRow = lines.findIndex(line => line.includes("Explore"));
    expect(promptRow).toBeGreaterThanOrEqual(0);
    expect(statusRow - promptRow).toBeGreaterThanOrEqual(3);
    expect(lines[statusRow]).toContain("Explore · fixture");
    expect(lines[statusRow]).toContain("Enter send");
    expect(lines[statusRow]).not.toContain("tenant read-only");
    expect(lines[statusRow]).not.toContain("/ commands");
  });

  test("opens the searchable theme picker for bare theme commands and applies a selection", async () => {
    const workspaceCommands: string[] = [];
    const workspace: WorkspaceAdapter = {
      ...FIXTURE_WORKSPACE_ADAPTER,
      execute: async input => {
        workspaceCommands.push(input);
        return {announcement: {title: "Unexpected dispatch", body: [input], tone: "danger"}};
      }
    };
    const setup = await testRender(createElement(App, {
      initialMode: "dark",
      initialTheme: "tokyonight",
      workspace
    }), {
      width: 120,
      height: 36
    });
    renderers.push(setup.renderer);
    await setup.flush();

    await interact(() => setup.mockInput.typeText("/theme"), setup.flush);
    await interact(() => setup.mockInput.pressEnter(), setup.flush);
    const pickerFrame = setup.captureCharFrame();
    expect(pickerFrame).toContain("Choose theme");
    expect(pickerFrame).toContain("Curated themes");
    expect(pickerFrame).toContain("tokyonight");
    expect(pickerFrame).toContain("current");
    expect(pickerFrame).not.toContain("/theme");

    await interact(() => setup.mockInput.pressEnter(), setup.flush);
    const unchangedFrame = setup.captureCharFrame();
    expect(unchangedFrame).toContain("Now using tokyonight · dark.");
    expect(unchangedFrame).not.toContain("Now using opencode · dark.");
    expect(unchangedFrame).not.toContain("/theme");

    await interact(() => setup.mockInput.typeText("/clear"), setup.flush);
    await interact(() => setup.mockInput.pressEnter(), setup.flush);
    await interact(() => setup.mockInput.typeText("/theme"), setup.flush);
    await interact(() => setup.mockInput.pressEnter(), setup.flush);

    await interact(() => setup.mockInput.pressEscape(), setup.flush);
    expect(setup.captureCharFrame()).not.toContain("/theme");
    await interact(() => setup.mockInput.typeText("/theme list"), setup.flush);
    await interact(() => setup.mockInput.pressEnter(), setup.flush);
    expect(setup.captureCharFrame()).toContain("Choose theme");

    await interact(() => setup.mockInput.typeText("tron"), setup.flush);
    const filteredFrame = setup.captureCharFrame();
    expect(filteredFrame).toContain("Experimental themes");
    expect(filteredFrame).toContain("tron");
    expect(filteredFrame).not.toContain("tokyonight");

    await interact(() => setup.mockInput.pressEnter(), setup.flush);
    const appliedFrame = setup.captureCharFrame();
    expect(appliedFrame).not.toContain("Choose theme");
    expect(appliedFrame).toContain("Theme changed");
    expect(appliedFrame).toContain("Now using tron · dark.");
    expect(appliedFrame).not.toContain("/theme list");
    expect(appliedFrame).not.toContain("/theme tron");

    await interact(() => setup.mockInput.typeText("/theme next"), setup.flush);
    await interact(() => setup.mockInput.pressEnter(), setup.flush);
    const directFrame = setup.captureCharFrame();
    expect(directFrame).toContain("/theme next");
    expect(directFrame).toContain("Now using cyberpunk · dark.");
    expect(workspaceCommands).toEqual([]);
  });

  test("commits a search result into the inspector from the keyboard", async () => {
    const setup = await testRender(createElement(App, {initialMode: "dark", initialTheme: "tokyonight"}), {
      width: 120,
      height: 36
    });
    renderers.push(setup.renderer);
    await setup.flush();

    await interact(() => setup.mockInput.typeText("/find"), setup.flush);
    await interact(() => setup.mockInput.pressEnter(), setup.flush);
    expect(setup.captureCharFrame()).toContain("Find in structured data");
    await interact(() => setup.mockInput.typeText("서울"), setup.flush);
    await interact(() => setup.mockInput.pressKey("o", {ctrl: true}), setup.flush);
    expect(setup.captureCharFrame()).toContain("JSON Inspector");
    expect(setup.captureCharFrame()).not.toContain("Find in structured data");
  });

  test("commits a selected picker row with the mouse", async () => {
    const setup = await testRender(createElement(App, {initialMode: "dark", initialTheme: "tokyonight"}), {
      width: 120,
      height: 36
    });
    renderers.push(setup.renderer);
    await setup.flush();
    await interact(() => setup.mockInput.typeText("/find"), setup.flush);
    await interact(() => setup.mockInput.pressEnter(), setup.flush);
    await interact(() => setup.mockInput.typeText("500"), setup.flush);
    const secondItem = setup.renderer.root.findDescendantById("picker-item-1");
    expect(secondItem).toBeDefined();
    await interact(
      () => setup.mockMouse.click(secondItem!.screenX + 2, secondItem!.screenY + 1),
      setup.flush
    );
    const frame = setup.captureCharFrame();
    expect(frame).not.toContain("Find in structured data");
    expect(frame).toContain("records › Seoul Branch");
  });

  test("preserves composer editing keys and uses Tab only after autocomplete declines it", async () => {
    const setup = await testRender(createElement(App, {initialMode: "dark", initialTheme: "tokyonight"}), {
      width: 100,
      height: 28
    });
    renderers.push(setup.renderer);
    await setup.flush();

    await interact(() => setup.mockInput.typeText("ac"), setup.flush);
    await interact(() => setup.mockInput.pressKey("b", {ctrl: true}), setup.flush);
    await interact(() => setup.mockInput.typeText("b"), setup.flush);
    await interact(() => setup.mockInput.pressKey("f", {ctrl: true}), setup.flush);
    await interact(() => setup.mockInput.typeText("d"), setup.flush);
    const edited = setup.captureCharFrame();
    expect(edited).toContain("abcd");
    expect(edited).not.toContain("Find in structured data");
    expect(edited).not.toContain("Tenant workspace");

    await interact(() => setup.mockInput.pressTab(), setup.flush);
    await interact(() => setup.mockInput.typeText("x"), setup.flush);
    expect(setup.captureCharFrame()).not.toContain("abcdx");
    await interact(() => setup.mockInput.pressTab({shift: true}), setup.flush);
    await interact(() => setup.mockInput.typeText("x"), setup.flush);
    expect(setup.captureCharFrame()).toContain("abcdx");

    await interact(() => setup.mockInput.pressTab({shift: true}), setup.flush);
    expect(setup.captureCharFrame()).toContain("Tenant workspace");
    await interact(() => setup.mockInput.pressTab(), setup.flush);
    await interact(() => setup.mockInput.typeText("y"), setup.flush);
    expect(setup.captureCharFrame()).toContain("abcdxy");
    expect(setup.captureCharFrame()).not.toContain("Tenant workspace");
  });

  test("keeps autocomplete ahead of focus movement and opens search with slash from the tree", async () => {
    const setup = await testRender(createElement(App, {initialMode: "dark", initialTheme: "tokyonight"}), {
      width: 121,
      height: 30
    });
    renderers.push(setup.renderer);
    await setup.flush();

    await interact(() => setup.mockInput.typeText("/th"), setup.flush);
    await interact(() => setup.mockInput.pressTab(), setup.flush);
    expect(setup.captureCharFrame()).toContain("/theme");

    await act(async () => { setup.renderer.destroy(); });
    renderers.pop();

    const treeSetup = await testRender(createElement(App, {initialMode: "dark", initialTheme: "tokyonight"}), {
      width: 121,
      height: 30
    });
    renderers.push(treeSetup.renderer);
    await treeSetup.flush();
    await interact(() => treeSetup.mockInput.pressTab({shift: true}), treeSetup.flush);
    await interact(() => treeSetup.mockInput.pressKey("/"), treeSetup.flush);
    expect(treeSetup.captureCharFrame()).toContain("Find in structured data");
  });

  test("accepts an injected workspace adapter without coupling the shell to fixtures", async () => {
    const snapshot: WorkspaceSnapshot = {
      data: {records: []},
      context: {
        connection: "connected",
        transport: "test adapter",
        authority: "tenant read-only",
        scope: "catalog",
        records: 0,
        effects: "none",
        operation: {status: "idle", label: "ready"}
      },
      announcement: {
        title: "Injected workspace ready",
        body: ["The shell rendered adapter-owned state."],
        tone: "success"
      }
    };
    const workspace: WorkspaceAdapter = {
      id: "test",
      initial: snapshot,
      reload: async () => snapshot,
      close: async () => undefined
    };
    const setup = await testRender(createElement(App, {
      initialMode: "dark",
      initialTheme: "tokyonight",
      workspace
    }), {width: 100, height: 28, exitOnCtrlC: false});
    renderers.push(setup.renderer);
    await setup.flush();

    expect(setup.captureCharFrame()).toContain("Injected workspace ready");
  });

  test("shows completed-work progress and reconciles context-free success", async () => {
    let resolveOperation!: (result: WorkspaceResult) => void;
    let emitLate!: (event: {kind: "progress"; completed: number; total: number; message: string}) => void;
    const workspace: WorkspaceAdapter = {
      ...FIXTURE_WORKSPACE_ADAPTER,
      commands: [{command: "/work", usage: "/work", summary: "Run test work"}],
      execute: async (_input, context) => new Promise(resolve => {
        resolveOperation = resolve;
        emitLate = context.emit;
        context.emit({kind: "progress", completed: 0, total: 3, message: "zia/locations"});
      })
    };
    const setup = await testRender(createElement(App, {
      initialMode: "dark",
      initialTheme: "tokyonight",
      workspace
    }), {width: 121, height: 30});
    renderers.push(setup.renderer);
    await setup.flush();

    await interact(() => setup.mockInput.typeText("/work"), setup.flush);
    await interact(() => setup.mockInput.pressEnter(), setup.flush);
    const running = setup.captureCharFrame();
    expect(running).toContain("zia/locations");
    expect(running).toContain("0/3");
    expect(running).not.toContain("3/3");

    await interact(() => resolveOperation({
      announcement: {title: "Work complete", body: ["The adapter returned no replacement context."], tone: "success"}
    }), setup.flush);
    const completed = setup.captureCharFrame();
    expect(completed).toContain("Work complete");
    expect(completed).not.toContain("0/3");

    await interact(() => emitLate({kind: "progress", completed: 2, total: 3, message: "late event"}), setup.flush);
    expect(setup.captureCharFrame()).not.toContain("late event");
    expect(setup.captureCharFrame()).not.toContain("2/3");
  });

  test("connects an injected engine workspace and executes catalog picker choices", async () => {
    const initial: WorkspaceSnapshot = {
      data: {status: "connecting"},
      context: {
        connection: "connecting",
        transport: "test stdio",
        authority: "tenant read-only",
        scope: "bootstrap",
        records: 0,
        effects: "process execution",
        operation: {status: "running", label: "connecting"}
      },
      announcement: {title: "Connecting", body: ["Test engine bootstrap."], tone: "info"}
    };
    const connected: WorkspaceResult = {
      data: {resources: []},
      context: {...initial.context, connection: "connected", scope: "catalog", effects: "none", operation: {status: "complete", label: "ready"}},
      announcement: {title: "Engine connected", body: ["Config-free catalog loaded."], tone: "success"}
    };
    const calls: string[] = [];
    let resolveConnect!: (result: WorkspaceResult) => void;
    const connectResult = new Promise<WorkspaceResult>(resolve => { resolveConnect = resolve; });
    const workspace: WorkspaceAdapter = {
      id: "test-engine",
      initial,
      commands: [{command: "/catalog", usage: "/catalog", summary: "Browse resources"}],
      connect: async () => connectResult,
      execute: async input => {
        calls.push(input);
        if (input === "/catalog") {
          return {
            ...connected,
            announcement: {title: "Resource catalog", body: ["Choose a resource."], tone: "info"},
            picker: {
              title: "Resource catalog\u001b[31m\u202e",
              placeholder: "Filter resources…\u001b\u202e",
              instruction: "Choose a resource.\u001b\u202e",
              emptyMessage: "No resources.\u001b\u202e",
              scopeLabel: "Product\u001b\u202e",
              scopes: [
                {id: "__all__", label: "ZIA", count: 1},
                {id: "zpa", label: "ZPA", count: 1}
              ],
              items: [
                {
                  id: "zia/locations",
                  title: "locations",
                  description: "list, get · 12 fields",
                  category: "ZIA · Internet Access",
                  scopeId: "__all__",
                  badge: "list\u001b\u202e",
                  command: "/list zia locations"
                },
                {
                  id: "zpa/app-segments",
                  title: "app-segments",
                  description: "list, get · 8 fields",
                  category: "ZPA · Private Access",
                  scopeId: "zpa",
                  badge: "list",
                  command: "/list zpa app-segments"
                }
              ]
            }
          };
        }
        return {
          data: {records: [{name: "Headquarters"}]},
          context: {...connected.context!, scope: "zia/locations", records: 1},
          announcement: {title: "Read zia/locations", body: ["1 projected record returned."], tone: "success"}
        };
      },
      close: async () => undefined
    };
    const setup = await testRender(createElement(App, {
      initialMode: "dark",
      initialTheme: "tokyonight",
      workspace
    }), {width: 110, height: 32});
    renderers.push(setup.renderer);
    await interact(() => resolveConnect(connected), setup.flush);
    expect(setup.captureCharFrame()).toContain("Engine connected");

    await interact(() => setup.mockInput.typeText("/catalog"), setup.flush);
    await interact(() => setup.mockInput.pressEnter(), setup.flush);
    const pickerFrame = setup.captureCharFrame();
    expect(pickerFrame).toContain("Resource catalog");
    expect(pickerFrame).toContain("locations");
    expect(pickerFrame).toContain("PRODUCT");
    expect(pickerFrame).toContain("ALL 2");
    expect(pickerFrame).toContain("ZIA 1");
    expect(pickerFrame).toContain("ZPA 1");
    expect(pickerFrame).not.toContain("\u001b");
    expect(pickerFrame).not.toContain("\u202e");
    expect(pickerFrame).toContain("�");

    await interact(() => setup.mockInput.pressTab(), setup.flush);
    expect(setup.captureCharFrame()).not.toContain("app-segments");
    await interact(() => setup.mockInput.pressTab(), setup.flush);
    const zpaFrame = setup.captureCharFrame();
    expect(zpaFrame).toContain("ZPA · 1/1");
    expect(zpaFrame).toContain("app-segments");
    expect(zpaFrame).not.toContain("locations");

    await interact(() => setup.mockInput.pressEnter(), setup.flush);
    expect(setup.captureCharFrame()).toContain("Read zia/locations");
    expect(calls).toEqual(["/catalog", "/list zpa app-segments"]);
  });

  test("resets a selected product scope when resize hides its controls", async () => {
    const calls: string[] = [];
    const workspace: WorkspaceAdapter = {
      ...FIXTURE_WORKSPACE_ADAPTER,
      commands: [{command: "/catalog", usage: "/catalog", summary: "Browse resources"}],
      execute: async input => {
        calls.push(input);
        if (input === "/catalog") {
          return {
            announcement: {title: "Resource catalog", body: ["Choose a resource."], tone: "info"},
            picker: {
              title: "Zscaler resource map",
              placeholder: "Search resources",
              instruction: "Choose a resource",
              emptyMessage: "No resources",
              scopeLabel: "Product",
              scopes: [
                {id: "zia", label: "ZIA", count: 1},
                {id: "zpa", label: "ZPA", count: 1},
                {id: "zcc", label: "ZCC", count: 1},
                {id: "ztw", label: "ZTW", count: 1},
                {id: "zidentity", label: "ZIDENTITY", count: 1}
              ],
              items: [
                {id: "zia/locations", title: "locations", description: "list", category: "ZIA", scopeId: "zia", command: "/list zia locations"},
                {id: "zpa/app-segments", title: "app-segments", description: "list", category: "ZPA", scopeId: "zpa", command: "/list zpa app-segments"},
                {id: "zcc/devices", title: "devices", description: "list", category: "ZCC", scopeId: "zcc", command: "/list zcc devices"},
                {id: "ztw/workloads", title: "workloads", description: "list", category: "ZTW", scopeId: "ztw", command: "/list ztw workloads"},
                {id: "zidentity/groups", title: "groups", description: "list", category: "ZIDENTITY", scopeId: "zidentity", command: "/list zidentity groups"}
              ]
            }
          };
        }
        return {announcement: {title: "Read complete", body: [input], tone: "success"}};
      }
    };
    const setup = await testRender(createElement(App, {
      initialMode: "dark",
      initialTheme: "tokyonight",
      workspace
    }), {width: 30, height: 16});
    renderers.push(setup.renderer);
    await setup.flush();

    await interact(() => setup.mockInput.typeText("/catalog"), setup.flush);
    await interact(() => setup.mockInput.pressEnter(), setup.flush);
    expect(setup.captureCharFrame()).toContain("PRODUCT");
    await interact(() => setup.mockInput.pressTab(), setup.flush);
    await interact(() => setup.mockInput.pressTab(), setup.flush);
    expect(setup.captureCharFrame()).toContain("ZPA · 1/1");
    expect(setup.captureCharFrame()).toContain("app-seg");

    await interact(() => setup.resize(30, 14), setup.flush);
    const compactFrame = setup.captureCharFrame();
    expect(compactFrame).not.toContain("PRODUCT");
    expect(compactFrame).not.toContain("ZPA · 1/1");
    expect(compactFrame).toContain("locations");
    await interact(() => setup.mockInput.pressTab(), setup.flush);
    await interact(() => setup.mockInput.pressEnter(), setup.flush);
    expect(calls).toEqual(["/catalog", "/list zia locations"]);
  });

  test("routes Ctrl+C to active engine cancellation without destroying the shell", async () => {
    const initial: WorkspaceSnapshot = {
      data: {records: []},
      context: {
        connection: "connected",
        transport: "test stdio",
        authority: "tenant read-only",
        scope: "catalog",
        records: 0,
        effects: "none",
        operation: {status: "idle", label: "ready"}
      },
      announcement: {title: "Engine ready", body: ["Test session."], tone: "success"}
    };
    const workspace: WorkspaceAdapter = {
      id: "test-engine",
      initial,
      commands: [{command: "/list", usage: "/list zia locations", summary: "Read locations"}],
      execute: async (_input, context) => new Promise((_resolve, reject) => {
        const canceled = () => reject(new WorkspaceCommandError({
          title: "Operation canceled",
          message: "The engine acknowledged cancellation.",
          tone: "warning",
          canceled: true
        }));
        if (context.signal.aborted) canceled();
        else context.signal.addEventListener("abort", canceled, {once: true});
      }),
      close: async () => undefined
    };
    const setup = await testRender(createElement(App, {
      initialMode: "dark",
      initialTheme: "tokyonight",
      workspace
    }), {width: 100, height: 28, exitOnCtrlC: false});
    renderers.push(setup.renderer);
    await setup.flush();
    await interact(() => setup.mockInput.typeText("/list zia locations"), setup.flush);
    await interact(() => setup.mockInput.pressEnter(), setup.flush);
    expect(setup.captureCharFrame()).toContain("Working");
    await interact(() => setup.mockInput.pressKey("c", {ctrl: true}), setup.flush);
    const frame = setup.captureCharFrame();
    expect(frame).toContain("Operation canceled");
    expect(frame).toContain("Explore");
    expect(frame).not.toContain("waiting for engine acknowledgment");
  });

  test("clears cancellation feedback when abort ends in a terminal failure", async () => {
    const workspace: WorkspaceAdapter = {
      ...FIXTURE_WORKSPACE_ADAPTER,
      commands: [{command: "/work", usage: "/work", summary: "Run test work"}],
      execute: async (_input, context) => new Promise((_resolve, reject) => {
        const failed = () => reject(new WorkspaceCommandError({
          title: "Cancellation failed",
          message: "The operation ended in a terminal failure.",
          tone: "warning"
        }));
        if (context.signal.aborted) failed();
        else context.signal.addEventListener("abort", failed, {once: true});
      })
    };
    const setup = await testRender(createElement(App, {
      initialMode: "dark",
      initialTheme: "tokyonight",
      workspace
    }), {width: 100, height: 28, exitOnCtrlC: false});
    renderers.push(setup.renderer);
    await setup.flush();

    await interact(() => setup.mockInput.typeText("/work"), setup.flush);
    await interact(() => setup.mockInput.pressEnter(), setup.flush);
    await interact(() => setup.mockInput.pressKey("c", {ctrl: true}), setup.flush);
    const frame = setup.captureCharFrame();
    expect(frame).toContain("Cancellation failed");
    expect(frame).not.toContain("waiting for engine acknowledgment");
  });

  test("renders inactive cancellation as informational transient feedback", async () => {
    const setup = await testRender(createElement(App, {initialMode: "dark", initialTheme: "tokyonight"}), {
      width: 100,
      height: 28
    });
    renderers.push(setup.renderer);
    await setup.flush();

    await interact(() => setup.mockInput.typeText("/cancel"), setup.flush);
    await interact(() => setup.mockInput.pressEnter(), setup.flush);
    const frame = setup.captureCharFrame();
    expect(frame).toContain("No engine operation is active.");
    expect(frame).toContain("i No engine operation");
  });
});
