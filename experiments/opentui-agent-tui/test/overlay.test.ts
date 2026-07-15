import {describe, expect, test} from "bun:test";

import {OVERLAY_Z_INDEX, placeFloatingWindow} from "../src/overlay.ts";

describe("overlay layout", () => {
  test("places a top-centered window one quarter into the available vertical space", () => {
    expect(placeFloatingWindow({
      viewportWidth: 100,
      viewportHeight: 40,
      preferredWidth: 60,
      preferredHeight: 12
    })).toEqual({left: 20, top: 7, width: 60, height: 12});
  });

  test("supports true centering for modal context windows", () => {
    expect(placeFloatingWindow({
      viewportWidth: 101,
      viewportHeight: 41,
      preferredWidth: 61,
      preferredHeight: 13,
      placement: "center"
    })).toEqual({left: 20, top: 14, width: 61, height: 13});
  });

  test("clamps both dimensions inside tiny terminals", () => {
    expect(placeFloatingWindow({
      viewportWidth: 20,
      viewportHeight: 6,
      preferredWidth: 80,
      preferredHeight: 20
    })).toEqual({left: 1, top: 1, width: 18, height: 4});
  });

  test("keeps popovers, drawers, dialogs, utility windows, and toasts in explicit order", () => {
    expect(OVERLAY_Z_INDEX.popover).toBeLessThan(OVERLAY_Z_INDEX.drawer);
    expect(OVERLAY_Z_INDEX.drawer).toBeLessThan(OVERLAY_Z_INDEX.dialog);
    expect(OVERLAY_Z_INDEX.dialog).toBeLessThan(OVERLAY_Z_INDEX.utility);
    expect(OVERLAY_Z_INDEX.utility).toBeLessThan(OVERLAY_Z_INDEX.toast);
  });
});
