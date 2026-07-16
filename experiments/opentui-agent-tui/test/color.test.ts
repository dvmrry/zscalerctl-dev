import {describe, expect, test} from "bun:test";

import {
  MINIMUM_TERMINAL_TEXT_CONTRAST,
  contrastRatioOver,
  readableColor
} from "../src/color.ts";

describe("terminal color contrast", () => {
  test("computes opaque and layered contrast", () => {
    expect(contrastRatioOver("#000000", ["#ffffff"])).toBeCloseTo(21, 8);
    expect(contrastRatioOver("#ffffff", ["#ffffff80", "#000000"])).toBeGreaterThan(3);
    expect(contrastRatioOver("#ffffff", ["#00000000"])).toBeUndefined();
  });

  test("falls back when a role is unreadable or the terminal underlay is unknown", () => {
    expect(readableColor("#eeeeee", "#111111", ["#ffffff"])).toBe("#111111");
    expect(readableColor("#111111", "#eeeeee", ["#ffffff"])).toBe("#111111");
    expect(readableColor("#111111", "#eeeeee", ["#00000000"])).toBe("#eeeeee");
    expect(MINIMUM_TERMINAL_TEXT_CONTRAST).toBe(3);
  });
});
