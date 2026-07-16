import {afterEach, describe, expect, test} from "bun:test";
import {testRender} from "@opentui/react/test-utils";
import {act, createElement} from "react";

import {
  OperationIndicator,
  normalizeOperationProgress,
  progressBarSegments
} from "../src/components/OperationIndicator.tsx";
import {spinnerFrame} from "../src/spinner.ts";
import {paletteFor} from "../src/theme.ts";

const renderers: Array<{destroy(): void}> = [];

afterEach(async () => {
  await act(async () => {
    for (const renderer of renderers.splice(0)) renderer.destroy();
  });
});

describe("operation progress presentation", () => {
  test("accepts only truthful completed-work counters", () => {
    expect(normalizeOperationProgress(0, 3)).toEqual({completed: 0, total: 3, ratio: 0});
    expect(normalizeOperationProgress(2, 3)).toEqual({completed: 2, total: 3, ratio: 2 / 3});
    for (const [completed, total] of [
      [undefined, 3], [0, undefined], [-1, 3], [4, 3], [0, 0], [1.5, 3], [1, Number.POSITIVE_INFINITY]
    ] as const) {
      expect(normalizeOperationProgress(completed, total)).toBeUndefined();
    }
  });

  test("renders an exact-width bar without inventing elapsed progress", () => {
    const empty = progressBarSegments(normalizeOperationProgress(0, 4)!, 8);
    expect(empty).toEqual({complete: "", head: "", remaining: "········"});

    const halfway = progressBarSegments(normalizeOperationProgress(2, 4)!, 8);
    expect(halfway).toEqual({complete: "━━━", head: "╺", remaining: "····"});
    expect(Bun.stringWidth(`${halfway.complete}${halfway.head}${halfway.remaining}`)).toBe(8);

    const complete = progressBarSegments(normalizeOperationProgress(4, 4)!, 8);
    expect(complete).toEqual({complete: "━━━━━━━━", head: "", remaining: ""});
  });

  test("degrades by available terminal cells without overflowing", async () => {
    for (const width of [44, 12, 8, 6, 2, 1]) {
      const setup = await testRender(createElement(OperationIndicator, {
        colors: paletteFor("tokyonight", "dark"),
        operation: {status: "running", label: "긴 작업", completed: 2, total: 3},
        detail: "network + config",
        availableWidth: width,
        activityFrame: spinnerFrame("hangul", 0)
      }), {width, height: 2});
      renderers.push(setup.renderer);
      await setup.flush();
      const lines = setup.captureCharFrame().split("\n");
      expect(lines.every(line => Bun.stringWidth(line) <= width)).toBe(true);
      const visible = lines.join("\n").trim();
      if (width === 44) expect(visible).toContain("━");
      if (width === 12) expect(visible).not.toContain("━");
      if (width === 6) expect(visible).toContain("2/3");
      if (width === 2) expect(visible).toMatch(/[삼이일]/u);
      if (width === 1) expect(visible).toBe("·");
      await act(async () => { setup.renderer.destroy(); });
      renderers.pop();
    }

    for (const width of [6, 2, 1]) {
      const setup = await testRender(createElement(OperationIndicator, {
        colors: paletteFor("tokyonight", "dark"),
        operation: {status: "running", label: "indeterminate"},
        detail: "network + config",
        availableWidth: width,
        activityFrame: spinnerFrame("hangul", 0)
      }), {width, height: 2});
      renderers.push(setup.renderer);
      await setup.flush();
      expect(setup.captureCharFrame().split("\n").every(line => Bun.stringWidth(line) <= width)).toBe(true);
      await act(async () => { setup.renderer.destroy(); });
      renderers.pop();
    }
  });

  test("renders a supplied Braille liveness frame", async () => {
    const width = 2;
    const setup = await testRender(createElement(OperationIndicator, {
      colors: paletteFor("tokyonight", "dark"),
      operation: {status: "running", label: "긴 작업", completed: 2, total: 3},
      detail: "network + config",
      availableWidth: width,
      activityFrame: spinnerFrame("braille", 0)
    }), {width, height: 2});
    renderers.push(setup.renderer);
    await setup.flush();
    const visible = setup.captureCharFrame().trim();
    expect(visible).toMatch(/[⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏]/u);
    await act(async () => { setup.renderer.destroy(); });
    renderers.pop();
  });
});
