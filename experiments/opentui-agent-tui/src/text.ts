const CONTROL = /[\u0000-\u001f\u007f-\u009f]/gu;
const GRAPHEME_SEGMENTER = new Intl.Segmenter(undefined, {granularity: "grapheme"});

export const UNSAFE_FORMAT_RANGES: readonly (readonly [number, number])[] = [
  [0x00ad, 0x00ad], [0x0600, 0x0605], [0x061c, 0x061c], [0x06dd, 0x06dd],
  [0x070f, 0x070f], [0x0890, 0x0891], [0x08e2, 0x08e2], [0x180e, 0x180e],
  [0x200b, 0x200f], [0x202a, 0x202e], [0x2060, 0x206f],
  [0xfff9, 0xfffb], [0xfeff, 0xfeff], [0x1bca0, 0x1bca3], [0x1d173, 0x1d17a],
  [0xe0001, 0xe0001], [0xe0020, 0xe007f]
];

export function isUnsafeFormatCodePoint(codePoint: number): boolean {
  return UNSAFE_FORMAT_RANGES.some(([start, end]) => codePoint >= start && codePoint <= end);
}

export function containsUnsafeFormatCharacter(value: string): boolean {
  for (const character of value) {
    const codePoint = character.codePointAt(0);
    if (codePoint !== undefined && isUnsafeFormatCodePoint(codePoint)) return true;
  }
  return false;
}

function replaceUnsafeFormatCharacters(value: string): string {
  let output = "";
  for (const character of value) {
    const codePoint = character.codePointAt(0);
    output += codePoint !== undefined && isUnsafeFormatCodePoint(codePoint) ? "�" : character;
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
