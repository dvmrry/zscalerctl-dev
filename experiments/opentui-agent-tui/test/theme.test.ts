import {describe, expect, test} from "bun:test";

import {
  isThemeName,
  modeFromBackground,
  paletteFor,
  THEME_NAMES
} from "../src/theme.ts";
import {resolveTheme, ThemeRegistry} from "../src/theme/engine.ts";

describe("theme catalog", () => {
  test("includes the OpenCode catalog and local compatibility themes", () => {
    expect(THEME_NAMES.length).toBe(37);
    expect(isThemeName("opencode")).toBe(true);
    expect(isThemeName("tokyonight")).toBe(true);
    expect(isThemeName("tron")).toBe(true);
    expect(isThemeName("ultraviolet")).toBe(false);
  });

  test("resolves every built-in in both appearance modes", () => {
    for (const name of THEME_NAMES) {
      for (const mode of ["dark", "light"] as const) {
        const palette = paletteFor(name, mode);
        expect(palette.name).toBe(name);
        expect(palette.mode).toBe(mode);
        expect(palette.background.startsWith("#")).toBe(true);
        expect(palette.text.startsWith("#")).toBe(true);
      }
    }
  });

  test("uses distinct OpenCode dark and light variants", () => {
    expect(paletteFor("opencode", "dark").background).toBe("#0a0a0a");
    expect(paletteFor("opencode", "light").background).toBe("#ffffff");
    expect(paletteFor("opencode", "dark").accent).toBe("#fab283");
    expect(paletteFor("opencode", "light").accent).toBe("#3b7dd8");
  });
});

describe("theme resolver", () => {
  test("resolves references, ANSI colors, variants, and optional fallbacks", () => {
    const definition = {
      defs: {
        ink: {dark: "#abc", light: 4}
      },
      theme: {
        primary: "ink",
        background: {dark: "#000000", light: "#ffffff"},
        backgroundElement: "#123456",
        selectedListItemText: "none"
      }
    } as const;

    const dark = resolveTheme(definition, "dark");
    const light = resolveTheme(definition, "light");
    expect(dark.colors.primary).toBe("#aabbcc");
    expect(light.colors.primary).toBe("#000080");
    expect(dark.colors.backgroundMenu).toBe("#123456");
    expect(dark.colors.selectedListItemText).toBe("#00000000");
  });

  test("rejects circular references", () => {
    expect(() => resolveTheme({defs: {a: "b", b: "a"}, theme: {primary: "a"}}, "dark"))
      .toThrow("Circular color reference");
  });

  test("supports collision-safe registry extension", () => {
    const registry = new ThemeRegistry({base: {theme: {primary: "#123456"}}});
    expect(registry.add("base", {theme: {primary: "#ffffff"}})).toBe(false);
    expect(registry.add("custom", {theme: {primary: "#abcdef"}})).toBe(true);
    expect(registry.resolve("custom", "dark").colors.primary).toBe("#abcdef");
  });

  test("detects terminal appearance from its background", () => {
    expect(modeFromBackground("#101010")).toBe("dark");
    expect(modeFromBackground("#f8f8f8")).toBe("light");
    expect(modeFromBackground(null)).toBeUndefined();
  });
});
