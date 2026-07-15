import {describe, expect, test} from "bun:test";

import {
  containsUnsafeFormatCharacter,
  safeInlineText,
  UNSAFE_FORMAT_RANGES
} from "../src/text.ts";

describe("terminal text safety", () => {
  test("recognizes every boundary in the client-aligned Unicode format table", () => {
    for (const [start, end] of UNSAFE_FORMAT_RANGES) {
      expect(containsUnsafeFormatCharacter(String.fromCodePoint(start))).toBe(true);
      expect(containsUnsafeFormatCharacter(String.fromCodePoint(end))).toBe(true);
    }
  });

  test("replaces terminal controls and unsafe formats before display", () => {
    expect(safeInlineText("safe\u001b[31m\u202eevil\nnext", 80)).toBe("safe�[31m�evil�next");
    expect(safeInlineText("ordinary 한글", 80)).toBe("ordinary 한글");
  });
});
