import {afterEach, describe, expect, test} from "bun:test";
import {testRender} from "@opentui/react/test-utils";
import {act, createElement} from "react";

import {App} from "../src/App.tsx";
import {
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
    await interact(() => setup.mockInput.pressKey("f", {ctrl: true}), setup.flush);
    await interact(() => setup.mockInput.typeText("500"), setup.flush);

    const frame = setup.captureCharFrame();
    expect(frame).toContain("Find in structured data");
    expect(frame).toContain("Raleigh Headquarters");
    expect(frame.split("\n").every(line => [...line].length <= 50)).toBe(true);
  });

  test("commits a search result into the inspector from the keyboard", async () => {
    const setup = await testRender(createElement(App, {initialMode: "dark", initialTheme: "tokyonight"}), {
      width: 120,
      height: 36
    });
    renderers.push(setup.renderer);
    await setup.flush();

    await interact(() => setup.mockInput.pressKey("f", {ctrl: true}), setup.flush);
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
    await interact(() => setup.mockInput.pressKey("f", {ctrl: true}), setup.flush);
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
              items: [{
                id: "zia/locations",
                title: "locations",
                description: "list, get · 12 fields",
                category: "ZIA",
                badge: "list\u001b\u202e",
                command: "/list zia locations"
              }]
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
    expect(pickerFrame).not.toContain("\u001b");
    expect(pickerFrame).not.toContain("\u202e");
    expect(pickerFrame).toContain("�");

    await interact(() => setup.mockInput.pressEnter(), setup.flush);
    expect(setup.captureCharFrame()).toContain("Read zia/locations");
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
  });
});
