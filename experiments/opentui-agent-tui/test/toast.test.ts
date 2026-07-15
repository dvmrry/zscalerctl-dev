import {describe, expect, test} from "bun:test";

import {
  LatestToastController,
  TOAST_DURATION_MS,
  toastColorRole,
  toastDurationMs,
  toastMarker,
  type ToastState,
  type ToastTimerDriver,
  type ToastTone
} from "../src/toast.ts";

describe("toast presentation policy", () => {
  test("uses tone-aware lifetimes without delaying routine success", () => {
    expect(TOAST_DURATION_MS).toEqual({success: 2_200, info: 3_200, warning: 4_500});
    expect(toastDurationMs("success")).toBeLessThan(toastDurationMs("info"));
    expect(toastDurationMs("info")).toBeLessThan(toastDurationMs("warning"));
  });

  test("maps every tone to a distinct marker and theme role", () => {
    const tones: readonly ToastTone[] = ["success", "info", "warning"];
    expect(tones.map(tone => [toastMarker(tone), toastColorRole(tone)])).toEqual([
      ["✓", "success"],
      ["i", "accent"],
      ["!", "warning"]
    ]);
  });

  test("keeps only the latest toast and ignores stale timer callbacks", () => {
    let nextHandle = 1;
    const callbacks = new Map<number, () => void>();
    const cleared: number[] = [];
    const timers: ToastTimerDriver = {
      setTimeout(callback) {
        const handle = nextHandle++;
        callbacks.set(handle, callback);
        return handle;
      },
      clearTimeout(handle) {
        cleared.push(handle as number);
      }
    };
    const published: Array<ToastState | undefined> = [];
    const controller = new LatestToastController(toast => published.push(toast), timers);

    controller.show("Copied", "success");
    const stale = callbacks.get(1)!;
    controller.show("Clipboard unavailable", "warning");
    expect(cleared).toEqual([1]);
    expect(published.at(-1)).toMatchObject({id: 2, message: "Clipboard unavailable", tone: "warning"});

    stale();
    expect(published.at(-1)).toMatchObject({id: 2});
    callbacks.get(2)!();
    expect(published.at(-1)).toBeUndefined();
  });

  test("dismisses only the identified active toast", () => {
    let nextHandle = 1;
    const callbacks = new Map<number, () => void>();
    const timers: ToastTimerDriver = {
      setTimeout(callback) {
        const handle = nextHandle++;
        callbacks.set(handle, callback);
        return handle;
      },
      clearTimeout() {}
    };
    const published: Array<ToastState | undefined> = [];
    const controller = new LatestToastController(toast => published.push(toast), timers);

    const cancellation = controller.show("Waiting", "info");
    const newer = controller.show("Copied", "success");
    expect(controller.dismiss(cancellation)).toBe(false);
    expect(published.at(-1)).toMatchObject({id: newer, message: "Copied"});
    expect(controller.dismiss(newer)).toBe(true);
    expect(published.at(-1)).toBeUndefined();
  });

  test("restarts duplicate deadlines and invalidates callbacks on disposal", () => {
    let nextHandle = 1;
    const callbacks = new Map<number, () => void>();
    const cleared: number[] = [];
    const timers: ToastTimerDriver = {
      setTimeout(callback) {
        const handle = nextHandle++;
        callbacks.set(handle, callback);
        return handle;
      },
      clearTimeout(handle) {
        cleared.push(handle as number);
      }
    };
    const published: Array<ToastState | undefined> = [];
    const controller = new LatestToastController(toast => published.push(toast), timers);

    controller.show("No operation", "info");
    const first = callbacks.get(1)!;
    controller.show("No operation", "info");
    expect(cleared).toEqual([1]);
    first();
    expect(published.at(-1)).toMatchObject({id: 2});

    const active = callbacks.get(2)!;
    controller.dispose();
    expect(cleared).toEqual([1, 2]);
    active();
    expect(published.at(-1)).toMatchObject({id: 2});
  });
});
