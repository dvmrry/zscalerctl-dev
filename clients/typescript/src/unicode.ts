// Keep this table aligned with Go 1.26's unicode.Cf table (Unicode 15.0.0).
// The TypeScript client test exhaustively compares every code point with the
// JavaScript runtime's Unicode property table so omissions fail closed.
export const UNICODE_FORMAT_VERSION = "15.0.0" as const;

const UNICODE_FORMAT_RANGES: readonly (readonly [number, number])[] = [
  [0x00ad, 0x00ad],
  [0x0600, 0x0605],
  [0x061c, 0x061c],
  [0x06dd, 0x06dd],
  [0x070f, 0x070f],
  [0x0890, 0x0891],
  [0x08e2, 0x08e2],
  [0x180e, 0x180e],
  [0x200b, 0x200f],
  [0x202a, 0x202e],
  [0x2060, 0x2064],
  [0x2066, 0x206f],
  [0xfeff, 0xfeff],
  [0xfff9, 0xfffb],
  [0x110bd, 0x110bd],
  [0x110cd, 0x110cd],
  [0x13430, 0x1343f],
  [0x1bca0, 0x1bca3],
  [0x1d173, 0x1d17a],
  [0xe0001, 0xe0001],
  [0xe0020, 0xe007f],
];

export function isUnicodeFormatCodePoint(codePoint: number): boolean {
  for (const [start, end] of UNICODE_FORMAT_RANGES) {
    if (codePoint >= start && codePoint <= end) return true;
  }
  return false;
}
