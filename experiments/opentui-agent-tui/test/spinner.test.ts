import {afterEach, describe, expect, test} from "bun:test";
import {testRender} from "@opentui/react/test-utils";
import {act, createElement, useState} from "react";

import {Composer} from "../src/components/Composer.tsx";
import {
  DEFAULT_SPINNER,
  SPINNERS,
  SPINNER_TYPES,
  spinnerFrame
} from "../src/spinner.ts";
import {paletteFor} from "../src/theme.ts";
import {
  FULL_MOTION_INTERVAL_MS,
  REDUCED_MOTION_INTERVAL_MS,
  type MotionTimerDriver,
  type MotionMode
} from "../src/motion.ts";
import {
  MotionProvider,
  useSpinnerFrame
} from "../src/useMotion.ts";

const renderers: Array<{destroy(): void}> = [];

class ManualMotionTimers implements MotionTimerDriver {
  #nextHandle = 1;
  readonly delays = new Map<number, number>();
  readonly callbacks = new Map<number, () => void>();
  readonly active = new Set<number>();

  setInterval(callback: () => void, delayMs: number): number {
    const handle = this.#nextHandle++;
    this.delays.set(handle, delayMs);
    this.callbacks.set(handle, callback);
    this.active.add(handle);
    return handle;
  }

  clearInterval(handle: unknown): void {
    this.active.delete(handle as number);
  }

  setTimeout(callback: () => void, delayMs: number): number {
    return this.setInterval(callback, delayMs);
  }

  clearTimeout(handle: unknown): void {
    this.clearInterval(handle);
  }

  tick(): void {
    for (const handle of [...this.active]) this.callbacks.get(handle)?.();
  }

  fireStale(handle: number): void {
    this.callbacks.get(handle)?.();
  }

  latestHandle(): number {
    return Math.max(...this.callbacks.keys());
  }
}

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

interface MotionControls {
  readonly setActive: (active: boolean) => void;
  readonly setMode: (mode: MotionMode) => void;
  readonly setSpinner: (spinner: (typeof SPINNER_TYPES)[number]) => void;
}

function ControlledMotionProbe(props: {
  readonly timers: MotionTimerDriver;
  readonly onControls: (controls: MotionControls) => void;
}) {
  const [active, setActive] = useState(true);
  const [mode, setMode] = useState<MotionMode>("full");
  const [spinner, setSpinner] = useState<(typeof SPINNER_TYPES)[number]>("hangul");
  props.onControls({setActive, setMode, setSpinner});
  return createElement(
    MotionProvider,
    {spinner, mode, active, timers: props.timers},
    createElement(SpinnerProbe)
  );
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
    const timers = new ManualMotionTimers();
    const setup = await testRender(createElement(
      MotionProvider,
      {spinner: "hangul", mode: "full", active: true, timers},
      createElement(SpinnerProbe)
    ), {width: 4, height: 1});
    renderers.push(setup.renderer);
    await setup.flush();

    const observed = [setup.captureCharFrame().trim()];
    for (let index = 0; index < 3; index += 1) {
      await act(async () => timers.tick());
      await setup.flush();
      observed.push(setup.captureCharFrame().trim());
    }
    expect(observed).toEqual(["삼", "이", "일", "삼"]);
    expect(timers.delays.get(timers.latestHandle())).toBe(FULL_MOTION_INTERVAL_MS);
    expect(setup.renderer.liveRequestCount).toBe(0);
  });

  test("does not rerender non-consumers on animation ticks", async () => {
    let passiveRenders = 0;
    const activeFrames: string[] = [];
    const timers = new ManualMotionTimers();
    const setup = await testRender(createElement(
      MotionProvider,
      {spinner: "hangul", mode: "full", active: true, timers},
      createElement("box", null,
        createElement(ObservedSpinnerProbe, {onRender: frame => { activeFrames.push(frame); }}),
        createElement(PassiveProbe, {onRender: () => { passiveRenders += 1; }})
      )
    ), {width: 12, height: 2});
    renderers.push(setup.renderer);
    await setup.flush();

    await act(async () => timers.tick());
    await setup.flush();
    expect(new Set(activeFrames).size).toBeGreaterThan(1);
    expect(passiveRenders).toBe(1);
  });

  test("stops cleanly, ignores stale callbacks, and restarts at the reduced cadence", async () => {
    const timers = new ManualMotionTimers();
    let controls!: MotionControls;
    const setup = await testRender(createElement(ControlledMotionProbe, {
      timers,
      onControls: value => { controls = value; }
    }), {width: 4, height: 1});
    renderers.push(setup.renderer);
    await setup.flush();

    const fullHandle = timers.latestHandle();
    await act(async () => timers.tick());
    await setup.flush();
    expect(setup.captureCharFrame().trim()).toBe("이");

    await act(async () => controls.setMode("off"));
    await setup.flush();
    expect(timers.active.size).toBe(0);
    expect(setup.captureCharFrame().trim()).toBe("삼");
    await act(async () => timers.fireStale(fullHandle));
    await setup.flush();
    expect(setup.captureCharFrame().trim()).toBe("삼");

    await act(async () => controls.setMode("reduced"));
    await setup.flush();
    const reducedHandle = timers.latestHandle();
    expect(timers.delays.get(reducedHandle)).toBe(REDUCED_MOTION_INTERVAL_MS);
    await act(async () => timers.tick());
    await setup.flush();
    expect(setup.captureCharFrame().trim()).toBe("이");

    await act(async () => controls.setSpinner("braille"));
    await setup.flush();
    expect(timers.active.has(reducedHandle)).toBeFalse();
    expect(setup.captureCharFrame().trim()).toBe("⠋");
    await act(async () => timers.fireStale(reducedHandle));
    await setup.flush();
    expect(setup.captureCharFrame().trim()).toBe("⠋");

    await act(async () => controls.setActive(false));
    await setup.flush();
    expect(timers.active.size).toBe(0);
  });
});
