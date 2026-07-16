export const SPINNER_TYPES = ["braille", "hangul", "pipe", "dots"] as const;

export type SpinnerType = (typeof SPINNER_TYPES)[number];

export interface SpinnerDefinition {
  readonly frames: readonly string[];
  readonly cellWidth: number;
}

export const DEFAULT_SPINNER: SpinnerType = "hangul";
export const SPINNER_INTERVAL_MS = 120;

export const SPINNERS: Readonly<Record<SpinnerType, SpinnerDefinition>> = {
  braille: {
    frames: ["⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"],
    cellWidth: 1
  },
  hangul: {
    frames: ["삼", "이", "일"],
    cellWidth: 2
  },
  pipe: {
    frames: ["|", "/", "-", "\\"],
    cellWidth: 1
  },
  dots: {
    frames: [".  ", ".. ", "...", " ..", "  .", " ..", "...", ".. "],
    cellWidth: 3
  }
};

export function isSpinnerType(value: string): value is SpinnerType {
  return (SPINNER_TYPES as readonly string[]).includes(value);
}

export function spinnerFrame(spinner: SpinnerType, index: number): string {
  const frames = SPINNERS[spinner].frames;
  const normalizedIndex = Number.isSafeInteger(index) && index >= 0 ? index % frames.length : 0;
  return frames[normalizedIndex]!;
}
