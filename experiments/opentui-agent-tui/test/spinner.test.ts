import {afterEach, describe, expect, test} from "bun:test";
import {testRender} from "@opentui/react/test-utils";
import {act, createElement} from "react";

import {Composer} from "../src/components/Composer.tsx";
import {
  DEFAULT_SPINNER,
  SPINNER_INTERVAL_MS,
  SPINNERS,
  SPINNER_TYPES,
  spinnerFrame
} from "../src/spinner.ts";
import {paletteFor} from "../src/theme.ts";
import {SpinnerFrameProvider, useSpinnerFrame} from "../src/useSpinnerFrame.ts";

const renderers: Array<{destroy(): void}> = [];

function SpinnerProbe() {
  const activityFrame = useSpinnerFrame();
  return createElement("text", null, activityFrame);
}

function PassiveProbe(props: {readonly onRender: () => void}) {
  props.onRender();
  return createElement("text", null, "steady");
}

function ObservedSpinnerProbe(props: {readonly onRender: (frame: string) => void}) {
  const activityFrame = useSpinnerFrame();
  props.onRender(activityFrame);
  return createElement("text", null, activityFrame);
}

afterEach(async () => {
  await act(async () => {
    for (const renderer of renderers.splice(0)) renderer.destroy();
  });
});

describe("activity spinner presentation", () => {
  test("keeps every frame nonblank and fixed to its declared cell width", () => {
    expect(DEFAULT_SPINNER).toBe("hangul");

    for (const spinner of SPINNER_TYPES) {
      const definition = SPINNERS[spinner];
      expect(definition.frames.length).toBeGreaterThan(1);
      for (const [index, frame] of definition.frames.entries()) {
        expect(frame.trim().length).toBeGreaterThan(0);
        expect(Bun.stringWidth(frame)).toBe(definition.cellWidth);
        expect(spinnerFrame(spinner, index)).toBe(frame);
      }
    }
  });

  test("keeps the composer status geometry stable through every frame", async () => {
    for (const spinner of SPINNER_TYPES) {
      const workingColumns: number[] = [];
      for (const activityFrame of SPINNERS[spinner].frames) {
        const setup = await testRender(createElement(Composer, {
          colors: paletteFor("tokyonight", "dark"),
          focus: "composer",
          busy: true,
          commands: [],
          workspaceLabel: "fixture",
          availableWidth: 120,
          roomy: true,
          activityFrame,
          onFocus: () => undefined,
          onFocusNext: () => undefined,
          onSubmit: () => undefined
        }), {width: 120, height: 8});
        renderers.push(setup.renderer);
        await setup.flush();

        const statusLine = setup.captureCharFrame().split("\n").find(line => line.includes("Working"));
        expect(statusLine).toBeDefined();
        const workingIndex = statusLine!.indexOf("Working");
        expect(workingIndex).toBeGreaterThanOrEqual(0);
        expect(statusLine).not.toContain("undefined");
        workingColumns.push(Bun.stringWidth(statusLine!.slice(0, workingIndex)));

        await act(async () => setup.renderer.destroy());
        renderers.pop();
      }
      expect(new Set(workingColumns).size).toBe(1);
    }
  });

  test("advances and wraps the shared animation clock", async () => {
    const setup = await testRender(createElement(
      SpinnerFrameProvider,
      {spinner: "hangul", active: true},
      createElement(SpinnerProbe)
    ), {width: 4, height: 1});
    renderers.push(setup.renderer);
    await setup.flush();

    const observed = [setup.captureCharFrame().trim()];
    for (let index = 0; index < 3; index += 1) {
      await act(async () => {
        await Bun.sleep(SPINNER_INTERVAL_MS + 20);
        await setup.flush();
      });
      observed.push(setup.captureCharFrame().trim());
    }
    expect(observed).toEqual(["삼", "이", "일", "삼"]);
  });

  test("does not rerender non-consumers on animation ticks", async () => {
    let passiveRenders = 0;
    const activeFrames: string[] = [];
    const setup = await testRender(createElement(
      SpinnerFrameProvider,
      {spinner: "hangul", active: true},
      createElement("box", null,
        createElement(ObservedSpinnerProbe, {onRender: frame => { activeFrames.push(frame); }}),
        createElement(PassiveProbe, {onRender: () => { passiveRenders += 1; }})
      )
    ), {width: 12, height: 2});
    renderers.push(setup.renderer);
    await setup.flush();

    await act(async () => {
      await Bun.sleep(SPINNER_INTERVAL_MS + 40);
      await setup.flush();
    });
    expect(new Set(activeFrames).size).toBeGreaterThan(1);
    expect(passiveRenders).toBe(1);
  });
});
