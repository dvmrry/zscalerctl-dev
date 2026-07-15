export const THEME_MODES = ["dark", "light"] as const;
export const THEME_MODE_PREFERENCES = ["auto", ...THEME_MODES] as const;

export type ThemeMode = (typeof THEME_MODES)[number];
export type ThemeModePreference = (typeof THEME_MODE_PREFERENCES)[number];

export interface ThemeColorVariant {
  readonly dark: ThemeColorValue;
  readonly light: ThemeColorValue;
}

export type ThemeColorValue = string | number | ThemeColorVariant;

export interface ThemeDefinition {
  readonly $schema?: string;
  readonly defs?: Readonly<Record<string, ThemeColorValue>>;
  readonly theme: Readonly<Record<string, ThemeColorValue | undefined>> & {
    readonly thinkingOpacity?: number;
  };
}

export interface ResolvedTheme {
  readonly colors: Readonly<Record<string, string>>;
  readonly thinkingOpacity: number;
  readonly hasSelectedListItemText: boolean;
}

const ANSI_COLORS = [
  "#000000", "#800000", "#008000", "#808000",
  "#000080", "#800080", "#008080", "#c0c0c0",
  "#808080", "#ff0000", "#00ff00", "#ffff00",
  "#0000ff", "#ff00ff", "#00ffff", "#ffffff"
] as const;

function ansiToHex(code: number): string {
  if (!Number.isInteger(code) || code < 0 || code > 255) {
    throw new Error(`ANSI color must be an integer from 0 to 255; received ${String(code)}.`);
  }
  if (code < 16) return ANSI_COLORS[code]!;
  if (code < 232) {
    const index = code - 16;
    const blue = index % 6;
    const green = Math.floor(index / 6) % 6;
    const red = Math.floor(index / 36);
    const channel = (value: number) => value === 0 ? 0 : value * 40 + 55;
    return `#${[channel(red), channel(green), channel(blue)]
      .map(value => value.toString(16).padStart(2, "0"))
      .join("")}`;
  }
  const gray = (code - 232) * 10 + 8;
  const channel = gray.toString(16).padStart(2, "0");
  return `#${channel}${channel}${channel}`;
}

function normalizeHex(value: string): string | undefined {
  const match = /^#([\da-f]{3}|[\da-f]{4}|[\da-f]{6}|[\da-f]{8})$/iu.exec(value);
  if (match === null) return undefined;
  const body = match[1]!.toLowerCase();
  if (body.length === 3 || body.length === 4) {
    return `#${[...body].map(character => `${character}${character}`).join("")}`;
  }
  return `#${body}`;
}

function isRecord(value: unknown): value is Readonly<Record<string, unknown>> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

export function isThemeDefinition(value: unknown): value is ThemeDefinition {
  return isRecord(value) && isRecord(value.theme);
}

export function resolveTheme(definition: ThemeDefinition, mode: ThemeMode): ResolvedTheme {
  if (!isThemeDefinition(definition)) throw new Error("Theme must contain a theme object.");
  const defs = definition.defs ?? {};

  const resolveColor = (value: ThemeColorValue, chain: readonly string[] = []): string => {
    if (typeof value === "number") return ansiToHex(value);
    if (typeof value === "string") {
      if (value === "transparent" || value === "none") return "#00000000";
      const hex = normalizeHex(value);
      if (hex !== undefined) return hex;
      if (chain.includes(value)) {
        throw new Error(`Circular color reference: ${[...chain, value].join(" -> ")}`);
      }
      const referenced = defs[value] ?? definition.theme[value];
      if (referenced === undefined || value === "thinkingOpacity") {
        throw new Error(`Color reference ${JSON.stringify(value)} was not found.`);
      }
      return resolveColor(referenced, [...chain, value]);
    }
    if (!isRecord(value) || !(mode in value)) {
      throw new Error(`Theme color is missing its ${mode} variant.`);
    }
    return resolveColor(value[mode] as ThemeColorValue, chain);
  };

  const colors: Record<string, string> = {};
  for (const [name, value] of Object.entries(definition.theme)) {
    if (name === "thinkingOpacity" || value === undefined) continue;
    colors[name] = resolveColor(value);
  }

  const hasSelectedListItemText = definition.theme.selectedListItemText !== undefined;
  if (!hasSelectedListItemText && colors.background !== undefined) {
    colors.selectedListItemText = colors.background;
  }
  if (colors.backgroundMenu === undefined && colors.backgroundElement !== undefined) {
    colors.backgroundMenu = colors.backgroundElement;
  }

  const thinkingOpacity = definition.theme.thinkingOpacity ?? 0.6;
  if (!Number.isFinite(thinkingOpacity) || thinkingOpacity < 0 || thinkingOpacity > 1) {
    throw new Error("thinkingOpacity must be a number from 0 to 1.");
  }

  return Object.freeze({
    colors: Object.freeze(colors),
    thinkingOpacity,
    hasSelectedListItemText
  });
}

export class ThemeRegistry {
  readonly #themes = new Map<string, ThemeDefinition>();

  constructor(themes: Readonly<Record<string, ThemeDefinition>> = {}) {
    for (const [name, definition] of Object.entries(themes)) this.add(name, definition);
  }

  add(name: string, definition: unknown, options: {readonly replace?: boolean} = {}): boolean {
    if (name.trim().length === 0 || !isThemeDefinition(definition)) return false;
    if (this.#themes.has(name) && options.replace !== true) return false;
    this.#themes.set(name, definition);
    return true;
  }

  has(name: string): boolean {
    return this.#themes.has(name);
  }

  names(): readonly string[] {
    return Object.freeze([...this.#themes.keys()]);
  }

  resolve(name: string, mode: ThemeMode): ResolvedTheme {
    const definition = this.#themes.get(name);
    if (definition === undefined) throw new Error(`Unknown theme ${JSON.stringify(name)}.`);
    return resolveTheme(definition, mode);
  }
}

export function isThemeMode(value: string): value is ThemeMode {
  return (THEME_MODES as readonly string[]).includes(value);
}

export function isThemeModePreference(value: string): value is ThemeModePreference {
  return (THEME_MODE_PREFERENCES as readonly string[]).includes(value);
}

export function modeFromBackground(background: string | null | undefined): ThemeMode | undefined {
  if (background === null || background === undefined) return undefined;
  const hex = normalizeHex(background);
  if (hex === undefined || hex.length < 7) return undefined;
  const red = Number.parseInt(hex.slice(1, 3), 16);
  const green = Number.parseInt(hex.slice(3, 5), 16);
  const blue = Number.parseInt(hex.slice(5, 7), 16);
  const luminance = (0.299 * red + 0.587 * green + 0.114 * blue) / 255;
  return luminance > 0.5 ? "light" : "dark";
}
