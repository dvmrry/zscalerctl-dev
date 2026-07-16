import {afterEach, describe, expect, test} from "bun:test";
import type {TextRenderable} from "@opentui/core";
import {testRender} from "@opentui/react/test-utils";
import {act, createElement} from "react";

import {
  OperationScene,
  operationTrack
} from "../src/components/OperationScene.tsx";
import type {ContextState} from "../src/model.ts";
import type {MotionMode} from "../src/motion.ts";
import {paletteFor} from "../src/theme.ts";
import {MotionProvider} from "../src/useMotion.ts";

const renderers: Array<{destroy(): void}> = [];
const colors = paletteFor("tokyonight", "dark");

function context(overrides: Partial<ContextState["operation"]> = {}): ContextState {
  return {
    connection: "connected",
    transport: "stdio v1",
    authority: "tenant read-only",
    scope: "zia/locations",
    records: 0,
    effects: "network read",
    operation: {
      status: "running",
      label: "zia/locations",
      completed: 0,
      total: 3,
      ...overrides
    }
  };
}

async function captureScene(mode: MotionMode, frame: number, value = context(), width = 72): Promise<string> {
  const setup = await testRender(createElement(
    MotionProvider,
    {spinner: "hangul", mode, active: false},
    createElement(OperationScene, {
      colors,
      context: value,
      availableWidth: width,
      compact: width < 52,
      motionFrame: frame
    })
  ), {width, height: 7});
  renderers.push(setup.renderer);
  await setup.flush();
  const rendered = setup.captureCharFrame();
  await act(async () => setup.renderer.destroy());
  renderers.pop();
  return rendered;
}

async function captureCompactScene(
  mode: MotionMode,
  frame: number,
  value: ContextState,
  width: number
): Promise<{
  readonly frame: string;
  readonly sceneHeight: number;
  readonly textHeight: number;
  readonly textPlain: string;
  readonly textWidth: number;
}> {
  const setup = await testRender(createElement(
    MotionProvider,
    {spinner: "hangul", mode, active: false},
    createElement(OperationScene, {
      colors,
      context: value,
      availableWidth: width,
      compact: true,
      motionFrame: frame
    })
  ), {width, height: 7});
  renderers.push(setup.renderer);
  await setup.flush();
  try {
    const scene = setup.renderer.root.findDescendantById("operation-scene-compact");
    const text = setup.renderer.root.findDescendantById("operation-scene-compact-text") as TextRenderable | undefined;
    if (scene === undefined || text === undefined) throw new Error("compact operation scene renderables are missing");
    return {
      frame: setup.captureCharFrame(),
      sceneHeight: scene.height,
      textHeight: text.height,
      textPlain: text.plainText,
      textWidth: text.width
    };
  } finally {
    await act(async () => setup.renderer.destroy());
    renderers.pop();
  }
}

afterEach(async () => {
  await act(async () => {
    for (const renderer of renderers.splice(0)) renderer.destroy();
  });
});

describe("data-reactive operation scene", () => {
  test("keeps every route fixed-width and limits decorative movement to full motion", () => {
    for (const width of [1, 2, 7, 30]) {
      for (const mode of ["full", "reduced", "off"] as const) {
        const frames = Array.from({length: 12}, (_, frame) => operationTrack(width, frame, mode));
        expect(frames.every(value => Bun.stringWidth(value) === Math.max(1, Math.floor(width)))).toBeTrue();
        expect(frames.every(value => [...value].filter(cell => cell === "◆").length === 1)).toBeTrue();
        expect(new Set(frames).size).toBe(mode === "full" && width > 1 ? Math.min(12, width) : 1);
      }
    }
  });

  test("renders only real operation metadata and truthful completed-work progress", async () => {
    const frame = await captureScene("full", 2);
    expect(frame).toContain("ACTIVE OPERATION · tenant read-only");
    expect(frame).toContain("[ stdio v1 ]");
    expect(frame).toContain("zia/locations");
    expect(frame).toContain("0/3");
    expect(frame).not.toContain("1/3");

    const malformed = await captureScene("off", 0, context({completed: 4, total: 3}));
    expect(malformed).not.toContain("4/3");
  });

  test("holds reduced and off artwork still while full motion advances", async () => {
    const fullStart = await captureScene("full", 0);
    const fullNext = await captureScene("full", 4);
    expect(fullStart).not.toBe(fullNext);

    const reducedStart = await captureScene("reduced", 0);
    const reducedNext = await captureScene("reduced", 4);
    expect(reducedStart).toBe(reducedNext);

    const offStart = await captureScene("off", 0);
    const offNext = await captureScene("off", 4);
    expect(offStart).toBe(offNext);
  });

  test("sanitizes adapter-owned labels in both wide and narrow variants", async () => {
    const unsafe: ContextState = {
      ...context(),
      transport: "stdio\u001b[31m",
      authority: "tenant\u202e read-only",
      operation: {...context().operation, label: "zia/locations\u001b[2J"}
    };
    const wide = await captureScene("off", 0, unsafe);
    const narrow = await captureScene("off", 0, unsafe, 40);
    for (const [frame, width] of [[wide, 72], [narrow, 40]] as const) {
      expect(frame).not.toContain("\u001b");
      expect(frame).not.toContain("\u202e");
      expect(frame).toContain("�");
      expect(frame.split("\n").every(line => Bun.stringWidth(line.trimEnd()) <= width)).toBeTrue();
    }
  });

  test("keeps compact progress on one stable row across valid counter sizes", async () => {
    const progressCases = [
      {completed: 1, total: 3},
      {completed: 99_999, total: 100_000},
      {completed: 1_000_000, total: 1_000_000},
      {completed: Number.MAX_SAFE_INTEGER, total: Number.MAX_SAFE_INTEGER}
    ] as const;

    for (const mode of ["full", "off"] as const) {
      for (const width of [12, 20, 40, 51]) {
        let renderedWidth: number | undefined;
        for (const progress of progressCases) {
          const captured = await captureCompactScene(mode, 3, context({
            ...progress,
            label: "locations"
          }), width);
          const nonblank = captured.frame.split("\n").filter(line => line.trim().length > 0);
          expect(nonblank).toHaveLength(1);
          expect(captured.textPlain).toMatch(/ (?:\d|…)+\/(?:\d|…)+$/u);
          expect(Bun.stringWidth(captured.textPlain)).toBe(width - 1);
          expect(captured.textWidth).toBe(width - 1);
          expect(captured.sceneHeight).toBe(1);
          expect(captured.textHeight).toBe(1);
          const lineWidth = Bun.stringWidth(nonblank[0]!.trimEnd());
          expect(lineWidth).toBeLessThanOrEqual(width);
          renderedWidth ??= lineWidth;
          expect(lineWidth).toBe(renderedWidth);
        }
      }
    }
  });
});
