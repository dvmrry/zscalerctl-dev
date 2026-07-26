import {describe, expect, test} from "bun:test";

import {
  containsUnsafeFormatCharacter,
  fitCellText,
  safeInlineText,
  type SafeString
} from "../src/text.ts";
import type {TranscriptBlock} from "../src/model.ts";
import type {ToastState} from "../src/toast.ts";
import type {TreeRow} from "../src/tree.ts";
import type {SafeWorkspacePickerItem} from "../src/workspace.ts";

type ExpectFalse<T extends false> = T;
type PlainStringIsSafe = ExpectFalse<string extends SafeString ? true : false>;
type PlainStringIsTranscriptText = ExpectFalse<
  string extends Extract<TranscriptBlock, {readonly kind: "text"}>["lines"][number] ? true : false
>;
type PlainStringIsTreeLabel = ExpectFalse<string extends TreeRow["label"] ? true : false>;
type PlainStringIsPickerTitle = ExpectFalse<string extends SafeWorkspacePickerItem["title"] ? true : false>;
type PlainStringIsToastMessage = ExpectFalse<string extends ToastState["message"] ? true : false>;

const TYPE_CONTRACT: readonly [
  PlainStringIsSafe,
  PlainStringIsTranscriptText,
  PlainStringIsTreeLabel,
  PlainStringIsPickerTitle,
  PlainStringIsToastMessage
] = [
  false,
  false,
  false,
  false,
  false
];

describe("terminal text safety", () => {
  test("recognizes previously omitted client-aligned Unicode format points", () => {
    for (const codePoint of [0x110bd, 0x110cd, 0x13430, 0x1343f]) {
      expect(containsUnsafeFormatCharacter(String.fromCodePoint(codePoint))).toBe(true);
    }
  });

  test("replaces terminal controls and unsafe formats before display", () => {
    expect(String(safeInlineText("safe\u001b[31m\u202eevil\nnext", 80))).toBe("safe�[31m�evil�next");
    expect(String(safeInlineText("ordinary 한글", 80))).toBe("ordinary 한글");
    expect(TYPE_CONTRACT).toEqual([false, false, false, false, false]);
  });

  test("replaces every Unicode 15 format rune before display", () => {
    let formatRunes = 0;
    for (let codePoint = 0; codePoint <= 0x10ffff; codePoint += 1) {
      const character = String.fromCodePoint(codePoint);
      if (!/\p{Cf}/u.test(character)) continue;
      formatRunes += 1;
      expect(String(safeInlineText(character, 2))).toBe("�");
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
