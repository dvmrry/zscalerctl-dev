import {describe, expect, test} from "bun:test";

import {commandSuggestions} from "../src/model.ts";
import {OptionError, parseOptions} from "../src/options.ts";

describe("experiment options", () => {
  test("uses Tokyo Night auto appearance by default and accepts explicit values", () => {
    expect(parseOptions([], {})).toEqual({kind: "run", options: {theme: "tokyonight", themeMode: "auto"}});
    expect(parseOptions(["--theme", "tokyonight", "--theme-mode", "dark"], {})).toEqual({
      kind: "run",
      options: {theme: "tokyonight", themeMode: "dark"}
    });
    expect(parseOptions(["--theme=cyberpunk", "--theme-mode=light"], {})).toEqual({
      kind: "run",
      options: {theme: "cyberpunk", themeMode: "light"}
    });
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
      options: {theme: "tokyonight", themeMode: "auto", engine: "/opt/zscalerctl-engine"}
    });
    expect(parseOptions(["--fixture"], {ZSCALERCTL_ENGINE_PATH: "/opt/zscalerctl-engine"})).toEqual({
      kind: "run",
      options: {theme: "tokyonight", themeMode: "auto"}
    });
  });

  test("rejects unknown options and themes", () => {
    expect(() => parseOptions(["--theme", "ultraviolet"], {})).toThrow(OptionError);
    expect(() => parseOptions(["--theme-mode", "sepia"], {})).toThrow(OptionError);
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
    expect(commandSuggestions("plain text")).toEqual([]);
  });
});
