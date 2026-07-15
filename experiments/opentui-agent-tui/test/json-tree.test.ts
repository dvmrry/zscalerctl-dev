import {afterEach, describe, expect, test} from "bun:test";
import {testRender} from "@opentui/react/test-utils";
import {act, createElement} from "react";

import {JsonTree} from "../src/components/JsonTree.tsx";
import {paletteFor} from "../src/theme.ts";
import {flattenTree, initialExpansion} from "../src/tree.ts";

const renderers: Array<{destroy(): void}> = [];

afterEach(async () => {
  await act(async () => {
    for (const renderer of renderers.splice(0)) renderer.destroy();
  });
});

describe("JSON tree presentation", () => {
  test("uses neutral keys and distinct type colors for value previews", async () => {
    const data = {name: "Raleigh", enabled: true, missing: null};
    const rows = flattenTree(data, initialExpansion(data));
    const setup = await testRender(createElement(JsonTree, {
      colors: paletteFor("tokyonight", "dark"),
      rows,
      selectedId: "not-selected",
      focus: "tree",
      onFocus: () => undefined,
      onSelect: () => undefined,
      onToggle: () => undefined
    }), {width: 48, height: 6});
    renderers.push(setup.renderer);
    await setup.flush();

    const spans = setup.captureSpans().lines.flatMap(line => line.spans);
    const colorOf = (text: string): string => {
      const span = spans.find(candidate => candidate.text === text);
      expect(span).toBeDefined();
      return JSON.stringify(span!.fg);
    };

    const keyColors = ["result", "name", "enabled"].map(colorOf);
    const valueColors = ["{3}", "\"Raleigh\"", "true"].map(colorOf);
    expect(new Set(keyColors).size).toBe(1);
    expect(new Set(valueColors).size).toBe(3);
    expect(valueColors).not.toContain(keyColors[0]);
  });
});
