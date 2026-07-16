import {describe, expect, test} from "bun:test";

import {commandSuggestions} from "../src/model.ts";
import {OptionError, parseOptions, USAGE} from "../src/options.ts";

describe("experiment options", () => {
  test("uses Tokyo Night auto appearance by default and accepts explicit values", () => {
    expect(parseOptions([], {})).toEqual({kind: "run", options: {theme: "tokyonight", themeMode: "auto", motion: "full", spinner: "hangul"}});
    expect(parseOptions(["--theme", "tokyonight", "--theme-mode", "dark"], {})).toEqual({
      kind: "run",
      options: {theme: "tokyonight", themeMode: "dark", motion: "full", spinner: "hangul"}
    });
    expect(parseOptions(["--theme=cyberpunk", "--theme-mode=light"], {})).toEqual({
      kind: "run",
      options: {theme: "cyberpunk", themeMode: "light", motion: "full", spinner: "hangul"}
    });
    expect(parseOptions(["--spinner", "braille"], {})).toEqual({
      kind: "run",
      options: {theme: "tokyonight", themeMode: "auto", motion: "full", spinner: "braille"}
    });
    expect(parseOptions(["--spinner=pipe"], {})).toEqual({
      kind: "run",
      options: {theme: "tokyonight", themeMode: "auto", motion: "full", spinner: "pipe"}
    });
    expect(parseOptions(["--spinner=dots"], {})).toEqual({
      kind: "run",
      options: {theme: "tokyonight", themeMode: "auto", motion: "full", spinner: "dots"}
    });
    expect(parseOptions(["--motion", "reduced"], {})).toEqual({
      kind: "run",
      options: {theme: "tokyonight", themeMode: "auto", motion: "reduced", spinner: "hangul"}
    });
    expect(parseOptions(["--motion=off"], {})).toEqual({
      kind: "run",
      options: {theme: "tokyonight", themeMode: "auto", motion: "off", spinner: "hangul"}
    });
    expect(USAGE).toContain("--motion MODE      full, reduced, or off (default: full)");
    expect(USAGE).toContain("--spinner TYPE     braille, hangul, pipe, or dots (default: hangul)");
  });

  test("accepts explicit or environment-selected absolute engines and safe process policy", () => {
    expect(parseOptions([
      "--engine", "/tmp/zscalerctl-engine",
      "--profile", "lab",
      "--config=/tmp/config.yaml",
      "--timeout", "15s",
      "--redaction", "paranoid",
      "--no-cache"
    ], {})).toEqual({
      kind: "run",
      options: {
        theme: "tokyonight",
        themeMode: "auto",
        motion: "full",
        spinner: "hangul",
        engine: "/tmp/zscalerctl-engine",
        profile: "lab",
        config: "/tmp/config.yaml",
        timeout: "15s",
        redaction: "paranoid",
        noCache: true
      }
    });
    expect(parseOptions([], {ZSCALERCTL_ENGINE_PATH: "/opt/zscalerctl-engine"})).toEqual({
      kind: "run",
      options: {theme: "tokyonight", themeMode: "auto", motion: "full", spinner: "hangul", engine: "/opt/zscalerctl-engine"}
    });
    expect(parseOptions(["--fixture"], {ZSCALERCTL_ENGINE_PATH: "/opt/zscalerctl-engine"})).toEqual({
      kind: "run",
      options: {theme: "tokyonight", themeMode: "auto", motion: "full", spinner: "hangul"}
    });
  });

  test("rejects unknown options and themes", () => {
    expect(() => parseOptions(["--theme", "ultraviolet"], {})).toThrow(OptionError);
    expect(() => parseOptions(["--theme-mode", "sepia"], {})).toThrow(OptionError);
    expect(() => parseOptions(["--spinner", "rotator"], {})).toThrow(OptionError);
    expect(() => parseOptions(["--motion", "maximum"], {})).toThrow(OptionError);
    expect(() => parseOptions(["--engine", "relative-engine"], {})).toThrow(OptionError);
    expect(() => parseOptions(["--engine", "/one", "--engine", "/two"], {})).toThrow(OptionError);
    expect(() => parseOptions(["--fixture", "--engine", "/engine"], {})).toThrow(OptionError);
    expect(() => parseOptions(["--profile", "lab"], {})).toThrow(OptionError);
    expect(() => parseOptions(["--profile", "bad\nname"], {ZSCALERCTL_ENGINE_PATH: "/engine"})).toThrow(OptionError);
    expect(() => parseOptions(["--wat"], {})).toThrow(OptionError);
    try {
      parseOptions(["--client-secret=do-not-echo"], {});
      throw new Error("expected an unknown option");
    } catch (error) {
      expect(error).toBeInstanceOf(OptionError);
      expect((error as Error).message).not.toContain("do-not-echo");
    }
  });
});

describe("slash suggestions", () => {
  test("opens for slash input and narrows deterministically", () => {
    expect(commandSuggestions("/").length).toBeGreaterThan(3);
    expect(commandSuggestions("/ins").map(item => item.command)).toEqual(["/inspect"]);
    expect(commandSuggestions("/mot").map(item => item.command)).toEqual(["/motion"]);
    expect(commandSuggestions("plain text")).toEqual([]);
  });
});
