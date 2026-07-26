import {isUnicodeFormatCodePoint} from "../../../clients/typescript/src/index.ts";

const CONTROL = /[\u0000-\u001f\u007f-\u009f]/gu;
const GRAPHEME_SEGMENTER = new Intl.Segmenter(undefined, {granularity: "grapheme"});

export function containsUnsafeFormatCharacter(value: string): boolean {
  for (const character of value) {
    const codePoint = character.codePointAt(0);
    if (codePoint !== undefined && isUnicodeFormatCodePoint(codePoint)) return true;
  }
  return false;
}

function replaceUnsafeFormatCharacters(value: string): string {
  let output = "";
  for (const character of value) {
    const codePoint = character.codePointAt(0);
    output += codePoint !== undefined && isUnicodeFormatCodePoint(codePoint) ? "�" : character;
  }
  return output;
}

export function safeInlineText(value: string, maximumCharacters = 120): string {
  const safe = replaceUnsafeFormatCharacters(value)
    .replace(CONTROL, character => character === "\t" ? " " : "�");
  const characters = [...safe];
  if (characters.length <= maximumCharacters) return safe;
  return `${characters.slice(0, Math.max(0, maximumCharacters - 1)).join("")}…`;
}

export function fitCellText(value: string, maximumWidth: number): string {
  const width = Math.max(1, Math.floor(maximumWidth));
  if (Bun.stringWidth(value) <= width) return value;
  if (width === 1) return "…";
  let output = "";
  let used = 0;
  for (const {segment} of GRAPHEME_SEGMENTER.segment(value)) {
    const segmentWidth = Bun.stringWidth(segment);
    if (used + segmentWidth + 1 > width) break;
    output += segment;
    used += segmentWidth;
  }
  return `${output}…`;
}
