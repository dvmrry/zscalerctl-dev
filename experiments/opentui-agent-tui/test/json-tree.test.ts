import {afterEach, describe, expect, test} from "bun:test";
import {testRender} from "@opentui/react/test-utils";
import {act, createElement} from "react";
import {WireNumber} from "../../../clients/typescript/src/index.ts";

import {MINIMUM_TERMINAL_TEXT_CONTRAST, contrastRatioOver} from "../src/color.ts";
import {JsonTree, resolveJsonTreeColors} from "../src/components/JsonTree.tsx";
import {paletteFor, THEME_NAMES} from "../src/theme.ts";
import {flattenTree, initialExpansion, type TreeKind} from "../src/tree.ts";

const renderers: Array<{destroy(): void}> = [];

function rgbaBytes(color: string): number[] {
  return [
    Number.parseInt(color.slice(1, 3), 16),
    Number.parseInt(color.slice(3, 5), 16),
    Number.parseInt(color.slice(5, 7), 16),
    color.length >= 9 ? Number.parseInt(color.slice(7, 9), 16) : 255
  ];
}

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

  test("keeps primary row text readable across every resolvable theme surface", () => {
    const kinds: readonly TreeKind[] = ["string", "number", "boolean", "null", "array", "object"];
    for (const name of THEME_NAMES) {
      for (const mode of ["dark", "light"] as const) {
        const palette = paletteFor(name, mode);
        const resolved = resolveJsonTreeColors(palette);
        const normalBackgrounds = [palette.panel, palette.background];
        const activeBackgrounds = [palette.selection, ...normalBackgrounds];
        const assertReadableOrFallback = (
          color: string,
          fallback: string,
          backgrounds: readonly string[]
        ) => {
          const ratio = contrastRatioOver(color, backgrounds);
          if (ratio === undefined) {
            expect(color).toBe(fallback);
          } else {
            expect(ratio).toBeGreaterThanOrEqual(MINIMUM_TERMINAL_TEXT_CONTRAST);
          }
        };

        for (const color of [resolved.normalMatchedLabel, ...kinds.map(kind => resolved.normalValues[kind])]) {
          assertReadableOrFallback(color, palette.text, normalBackgrounds);
        }
        for (const color of [
          resolved.activeForeground,
          resolved.activeMatchedLabel,
          resolved.activeMatchLabel,
          ...kinds.map(kind => resolved.activeValues[kind])
        ]) {
          assertReadableOrFallback(color, resolved.activeForeground, activeBackgrounds);
        }
      }
    }
  });

  test("renders a readable fallback for Matrix light numeric values", async () => {
    const data = {answer: new WireNumber("42")};
    const rows = flattenTree(data, initialExpansion(data));
    const colors = paletteFor("matrix", "light");
    const resolved = resolveJsonTreeColors(colors);
    expect(resolved.normalValues.number).toBe(colors.text);

    const setup = await testRender(createElement(JsonTree, {
      colors,
      rows,
      selectedId: "not-selected",
      focus: "tree",
      onFocus: () => undefined,
      onSelect: () => undefined,
      onToggle: () => undefined
    }), {width: 32, height: 4});
    renderers.push(setup.renderer);
    await setup.flush();

    const valueSpan = setup.captureSpans().lines.flatMap(line => line.spans)
      .find(span => span.text === "42");
    expect(valueSpan).toBeDefined();
    expect(Array.from(valueSpan!.fg.buffer)).toEqual(rgbaBytes(colors.text));
  });

  test("preserves normal, active-match, and selected value precedence", async () => {
    const data = {answer: new WireNumber("42")};
    const rows = flattenTree(data, initialExpansion(data));
    const answerId = rows.find(row => row.label === "answer")!.id;
    const colors = paletteFor("cobalt2", "dark");
    const resolved = resolveJsonTreeColors(colors);
    const cases = [
      {selectedId: "not-selected", expected: resolved.normalValues.number},
      {selectedId: "not-selected", activeMatchId: answerId, expected: resolved.activeValues.number},
      {
        selectedId: answerId,
        activeMatchId: answerId,
        matchedIds: new Set([answerId]),
        expected: resolved.activeForeground
      }
    ] as const;

    for (const testCase of cases) {
      const setup = await testRender(createElement(JsonTree, {
        colors,
        rows,
        selectedId: testCase.selectedId,
        focus: "tree",
        matchedIds: "matchedIds" in testCase ? testCase.matchedIds : undefined,
        activeMatchId: "activeMatchId" in testCase ? testCase.activeMatchId : undefined,
        onFocus: () => undefined,
        onSelect: () => undefined,
        onToggle: () => undefined
      }), {width: 32, height: 4});
      renderers.push(setup.renderer);
      await setup.flush();

      const valueSpan = setup.captureSpans().lines.flatMap(line => line.spans)
        .find(span => span.text.includes("42"));
      expect(valueSpan).toBeDefined();
      expect(Array.from(valueSpan!.fg.buffer)).toEqual(rgbaBytes(testCase.expected));
      await act(async () => setup.renderer.destroy());
      renderers.pop();
    }
  });
});
