interface RGBAColor {
  readonly red: number;
  readonly green: number;
  readonly blue: number;
  readonly alpha: number;
}

export const MINIMUM_TERMINAL_TEXT_CONTRAST = 3;

function parseHexColor(value: string): RGBAColor | undefined {
  const match = /^#([\da-f]{6})([\da-f]{2})?$/iu.exec(value);
  if (match === null) return undefined;
  const body = match[1]!;
  return {
    red: Number.parseInt(body.slice(0, 2), 16) / 255,
    green: Number.parseInt(body.slice(2, 4), 16) / 255,
    blue: Number.parseInt(body.slice(4, 6), 16) / 255,
    alpha: match[2] === undefined ? 1 : Number.parseInt(match[2], 16) / 255
  };
}

function composite(foreground: RGBAColor, background: RGBAColor): RGBAColor {
  const alpha = foreground.alpha + background.alpha * (1 - foreground.alpha);
  if (alpha === 0) return {red: 0, green: 0, blue: 0, alpha: 0};
  const channel = (front: number, back: number) => (
    front * foreground.alpha + back * background.alpha * (1 - foreground.alpha)
  ) / alpha;
  return {
    red: channel(foreground.red, background.red),
    green: channel(foreground.green, background.green),
    blue: channel(foreground.blue, background.blue),
    alpha
  };
}

function effectiveBackground(layers: readonly string[]): RGBAColor | undefined {
  let result: RGBAColor | undefined;
  for (let index = layers.length - 1; index >= 0; index -= 1) {
    const layer = parseHexColor(layers[index]!);
    if (layer === undefined) return undefined;
    result = result === undefined ? layer : composite(layer, result);
  }
  return result?.alpha === 1 ? result : undefined;
}

function relativeLuminance(color: RGBAColor): number {
  const linear = (channel: number) => channel <= 0.04045
    ? channel / 12.92
    : ((channel + 0.055) / 1.055) ** 2.4;
  return 0.2126 * linear(color.red)
    + 0.7152 * linear(color.green)
    + 0.0722 * linear(color.blue);
}

export function contrastRatioOver(foreground: string, backgroundLayers: readonly string[]): number | undefined {
  const parsedForeground = parseHexColor(foreground);
  const background = effectiveBackground(backgroundLayers);
  if (parsedForeground === undefined || background === undefined) return undefined;
  const foregroundLuminance = relativeLuminance(composite(parsedForeground, background));
  const backgroundLuminance = relativeLuminance(background);
  const lighter = Math.max(foregroundLuminance, backgroundLuminance);
  const darker = Math.min(foregroundLuminance, backgroundLuminance);
  return (lighter + 0.05) / (darker + 0.05);
}

export function readableColor(
  candidate: string,
  fallback: string,
  backgroundLayers: readonly string[],
  minimumContrast = MINIMUM_TERMINAL_TEXT_CONTRAST
): string {
  const ratio = contrastRatioOver(candidate, backgroundLayers);
  return ratio !== undefined && ratio >= minimumContrast ? candidate : fallback;
}
