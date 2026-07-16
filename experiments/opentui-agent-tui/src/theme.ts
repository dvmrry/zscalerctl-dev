import {THEME_CATALOG, THEME_NAMES, type ThemeName} from "./theme/catalog.ts";
import {
  isThemeMode,
  isThemeModePreference,
  modeFromBackground,
  ThemeRegistry,
  THEME_MODE_PREFERENCES,
  THEME_MODES,
  type ThemeMode,
  type ThemeModePreference
} from "./theme/engine.ts";

export {
  THEME_MODE_PREFERENCES,
  THEME_MODES,
  THEME_NAMES,
  isThemeMode,
  isThemeModePreference,
  modeFromBackground
};
export type {ThemeMode, ThemeModePreference, ThemeName};

export interface Palette {
  readonly name: ThemeName;
  readonly mode: ThemeMode;
  readonly background: string;
  readonly panel: string;
  readonly panelRaised: string;
  readonly surface: string;
  readonly surfaceFocus: string;
  readonly border: string;
  readonly borderActive: string;
  readonly accent: string;
  readonly accentSecondary: string;
  readonly text: string;
  readonly textMuted: string;
  readonly success: string;
  readonly warning: string;
  readonly danger: string;
  readonly selection: string;
  readonly selectionText: string;
}

const registry = new ThemeRegistry(THEME_CATALOG);

function requiredColor(colors: Readonly<Record<string, string>>, name: string): string {
  const value = colors[name];
  if (value === undefined) throw new Error(`Theme is missing required color ${JSON.stringify(name)}.`);
  return value;
}

export function paletteFor(name: ThemeName, mode: ThemeMode): Palette {
  const resolved = registry.resolve(name, mode);
  const colors = resolved.colors;
  const text = requiredColor(colors, "text");
  const backgroundElement = requiredColor(colors, "backgroundElement");
  return Object.freeze({
    name,
    mode,
    background: requiredColor(colors, "background"),
    panel: requiredColor(colors, "backgroundPanel"),
    panelRaised: backgroundElement,
    surface: requiredColor(colors, "backgroundPanel"),
    surfaceFocus: colors.backgroundMenu ?? backgroundElement,
    border: requiredColor(colors, "border"),
    borderActive: requiredColor(colors, "borderActive"),
    accent: requiredColor(colors, "primary"),
    accentSecondary: requiredColor(colors, "secondary"),
    text,
    textMuted: requiredColor(colors, "textMuted"),
    success: requiredColor(colors, "success"),
    warning: requiredColor(colors, "warning"),
    danger: requiredColor(colors, "error"),
    selection: colors.backgroundMenu ?? backgroundElement,
    selectionText: resolved.hasSelectedListItemText
      ? requiredColor(colors, "selectedListItemText")
      : text
  });
}

export function isThemeName(value: string): value is ThemeName {
  return registry.has(value);
}

export function nextTheme(name: ThemeName): ThemeName {
  const index = THEME_NAMES.indexOf(name);
  return THEME_NAMES[(index + 1) % THEME_NAMES.length];
}

export function nextThemeMode(mode: ThemeMode): ThemeMode {
  return mode === "dark" ? "light" : "dark";
}
