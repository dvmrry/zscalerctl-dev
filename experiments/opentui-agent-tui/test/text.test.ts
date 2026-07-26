import {describe, expect, test} from "bun:test";

import {
  containsUnsafeFormatCharacter,
  fitCellText,
  safeInlineText
} from "../src/text.ts";

describe("terminal text safety", () => {
  test("recognizes previously omitted client-aligned Unicode format points", () => {
    for (const codePoint of [0x110bd, 0x110cd, 0x13430, 0x1343f]) {
      expect(containsUnsafeFormatCharacter(String.fromCodePoint(codePoint))).toBe(true);
    }
  });

  test("replaces terminal controls and unsafe formats before display", () => {
    expect(safeInlineText("safe\u001b[31m\u202eevil\nnext", 80)).toBe("safe�[31m�evil�next");
    expect(safeInlineText("ordinary 한글", 80)).toBe("ordinary 한글");
  });

  test("replaces every Unicode 15 format rune before display", () => {
    let formatRunes = 0;
    for (let codePoint = 0; codePoint <= 0x10ffff; codePoint += 1) {
      const character = String.fromCodePoint(codePoint);
      if (!/\p{Cf}/u.test(character)) continue;
      formatRunes += 1;
      expect(safeInlineText(character, 2)).toBe("�");
    }
    expect(formatRunes).toBe(170);
  });

  test("fits whole graphemes to terminal cells", () => {
    const fitted = fitCellText("삼 combining e\u0301 한글", 12);
    expect(Bun.stringWidth(fitted)).toBeLessThanOrEqual(12);
    expect(fitted).toEndWith("…");
    expect(fitted).not.toContain("e…\u0301");
  });
});
