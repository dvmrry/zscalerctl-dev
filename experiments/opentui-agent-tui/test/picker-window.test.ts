import {afterEach, describe, expect, test} from "bun:test";
import {testRender} from "@opentui/react/test-utils";
import {act, createElement} from "react";

import {
  PickerWindow,
  pickerScopeBarVisible,
  pickerScopeRowCount,
  pickerScopeText,
  type PickerScope
} from "../src/components/PickerWindow.tsx";
import {paletteFor} from "../src/theme.ts";

const SCOPES: readonly PickerScope[] = [
  {label: "ALL", count: 165},
  {id: "zia", label: "ZIA", count: 102},
  {id: "zpa", label: "ZPA", count: 28},
  {id: "zcc", label: "ZCC", count: 11},
  {id: "ztw", label: "ZTW", count: 21},
  {id: "zidentity", label: "ZIDENTITY", count: 3}
];
const renderers: Array<{destroy(): void}> = [];

afterEach(async () => {
  await act(async () => {
    for (const renderer of renderers.splice(0)) renderer.destroy();
  });
});

async function renderPicker(width: number, height: number, options: {
  readonly scopes?: readonly PickerScope[];
  readonly onScopeChange?: (id: string | undefined) => void;
} = {}) {
  const setup = await testRender(createElement(PickerWindow, {
    colors: paletteFor("tokyonight", "dark"),
    viewportWidth: width,
    viewportHeight: height,
    preferredWidth: 86,
    title: "Zscaler resource map",
    query: "",
    placeholder: "Search resources",
    focused: true,
    items: [{
      id: "zia/locations",
      value: null,
      title: "locations",
      description: "list · get · 73 projected fields",
      category: "ZIA · Internet Access",
      categoryId: "zia",
      badge: "list"
    }],
    selectedId: "zia/locations",
    instruction: "Choose a resource",
    emptyMessage: "No resources",
    inputMethod: "keyboard",
    showItemsWithoutQuery: true,
    scopeLabel: "Product",
    scopes: options.scopes ?? SCOPES,
    onInput: () => undefined,
    onFocus: () => undefined,
    onInputMethodChange: () => undefined,
    onMove: () => undefined,
    onSelect: () => undefined,
    onScopeChange: options.onScopeChange ?? (() => undefined),
    onCancel: () => undefined
  }), {width, height});
  renderers.push(setup.renderer);
  await setup.flush();
  return setup;
}

describe("picker scope layout", () => {
  test("keeps compact product scopes on one row and wraps deterministically", () => {
    expect(pickerScopeRowCount(SCOPES.slice(0, 3), "PRODUCT", 80)).toBe(1);
    expect(pickerScopeRowCount(SCOPES.slice(0, 3), "PRODUCT", 20)).toBe(2);
  });

  test("uses terminal cell width for Hangul labels", () => {
    expect(pickerScopeRowCount([{id: "product", label: "제품", count: 1}], "범위", 9)).toBe(2);
  });

  test("fits an individually overwide pill without wrapping into resource rows", async () => {
    expect(pickerScopeText(SCOPES.at(-1)!, 10)).toBe("ZIDENTI… 3");
    expect(pickerScopeRowCount(SCOPES, "PRODUCT", 10)).toBe(7);
    const setup = await renderPicker(16, 30);
    const frame = setup.captureCharFrame();
    expect(frame).toContain("ZIDENTI… 3");
    expect(frame).toContain("ZIA");
    expect(frame).not.toContain("3ZIA");
  });

  test("hides scope chrome when the clamped window cannot contain it", async () => {
    for (const height of [8, 10, 12, 14]) {
      expect(pickerScopeBarVisible({
        scopes: SCOPES,
        label: "PRODUCT",
        viewportWidth: 30,
        viewportHeight: height,
        preferredWidth: 86
      })).toBe(false);
      const setup = await renderPicker(30, height);
      const frame = setup.captureCharFrame();
      const lines = frame.split("\n");
      const top = lines.findIndex(line => line.includes("╭"));
      const bottom = lines.findIndex(line => line.includes("╰"));
      expect(top).toBeGreaterThanOrEqual(0);
      expect(bottom).toBeGreaterThan(top);
      expect(frame).not.toContain("PRODUCT");
      expect(frame).not.toContain("ZIDENTITY");
      expect(lines[bottom]).not.toContain("locations");
      expect(lines.slice(bottom + 1).join("\n")).not.toContain("Esc");
      expect(lines.slice(bottom + 1).join("\n")).not.toContain("locations");
      await act(async () => { setup.renderer.destroy(); });
      renderers.pop();
    }

    expect(pickerScopeBarVisible({
      scopes: SCOPES,
      label: "PRODUCT",
      viewportWidth: 30,
      viewportHeight: 16,
      preferredWidth: 86
    })).toBe(true);
    const boundary = await renderPicker(30, 16);
    const frame = boundary.captureCharFrame();
    const bottom = frame.split("\n").find(line => line.includes("╰"));
    expect(frame).toContain("ZIDENTITY 3");
    expect(frame).not.toContain("3ZIA");
    expect(bottom).not.toContain("locations");
    await act(async () => { boundary.renderer.destroy(); });
    renderers.pop();
  });

  test("keeps the synthetic all scope distinct from an opaque __all__ ID", async () => {
    const selected: Array<string | undefined> = [];
    const setup = await renderPicker(80, 24, {
      scopes: [
        {label: "ALL", count: 2},
        {id: "__all__", label: "FIRST", count: 1},
        {id: "other", label: "SECOND", count: 1}
      ],
      onScopeChange: id => { selected.push(id); }
    });
    const first = setup.renderer.root.findDescendantById("picker-scope-1");
    const second = setup.renderer.root.findDescendantById("picker-scope-2");
    expect(first).toBeDefined();
    expect(second).toBeDefined();
    await act(async () => {
      setup.mockMouse.click(first!.screenX + 1, first!.screenY);
      await setup.flush();
    });
    expect(selected).toEqual(["__all__"]);
    expect(setup.captureCharFrame()).toContain("FIRST 1");
    expect(setup.captureCharFrame()).toContain("SECOND 1");
  });
});
