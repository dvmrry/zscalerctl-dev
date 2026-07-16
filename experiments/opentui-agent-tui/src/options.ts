import {isAbsolute} from "node:path";

import type {Redaction} from "../../../clients/typescript/src/index.ts";
import {DEFAULT_MOTION_MODE, isMotionMode, type MotionMode} from "./motion.ts";
import {DEFAULT_SPINNER, isSpinnerType, type SpinnerType} from "./spinner.ts";
import {containsUnsafeFormatCharacter} from "./text.ts";
import {
  isThemeModePreference,
  isThemeName,
  type ThemeModePreference,
  type ThemeName
} from "./theme.ts";

const MAXIMUM_OPTION_CHARACTERS = 8_192;

export interface ExperimentOptions {
  readonly theme: ThemeName;
  readonly themeMode: ThemeModePreference;
  readonly motion?: MotionMode;
  readonly spinner?: SpinnerType;
  readonly engine?: string;
  readonly profile?: string;
  readonly config?: string;
  readonly timeout?: string;
  readonly redaction?: Redaction;
  readonly noCache?: boolean;
}

export type ParsedOptions = {readonly kind: "help"} | {readonly kind: "run"; readonly options: ExperimentOptions};

export class OptionError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "OptionError";
  }
}

export const USAGE = `Usage: bun start -- [options]

Unsupported OpenTUI agent-shell experiment.

Interface options:
  --theme NAME       Built-in theme name (default: tokyonight)
  --theme-mode MODE  auto, dark, or light (default: auto)
  --motion MODE      full, reduced, or off (default: ${DEFAULT_MOTION_MODE})
  --spinner TYPE     braille, hangul, pipe, or dots (default: ${DEFAULT_SPINNER})
  --fixture          Ignore ZSCALERCTL_ENGINE_PATH and use sanitized fixture data

Engine options:
  --engine PATH       Absolute zscalerctl-engine executable path
  --profile NAME      Select an existing zscalerctl profile
  --config PATH       Select an existing zscalerctl config file
  --timeout DURATION  Set the engine HTTP-request timeout
  --redaction MODE    standard, share, or paranoid
  --no-cache          Disable the engine cache
  -h, --help          Show this help

ZSCALERCTL_ENGINE_PATH may supply --engine. Credentials remain in inherited
ZSCALERCTL_* environment variables consumed by the Go engine; this experiment
has no credential arguments or credential wire API.`;

function checkedValue(name: string, value: string | undefined): string {
  if (value === undefined || value.length === 0 || value.length > MAXIMUM_OPTION_CHARACTERS
      || /[\u0000-\u001f\u007f-\u009f]/u.test(value) || containsUnsafeFormatCharacter(value)) {
    throw new OptionError(`${name} requires a bounded value without control or format characters.`);
  }
  return value;
}

function assignOnce(current: string | undefined, name: string, value: string): string {
  if (current !== undefined) throw new OptionError(`${name} may be supplied only once.`);
  return value;
}

function optionAssignment(argument: string, name: string): string | undefined {
  const prefix = `${name}=`;
  return argument.startsWith(prefix) ? argument.slice(prefix.length) : undefined;
}

function optionForError(argument: string): string {
  const separator = argument.indexOf("=");
  return separator > 0 ? argument.slice(0, separator) : argument;
}

export function parseOptions(
  args: readonly string[],
  environment: Readonly<NodeJS.ProcessEnv> = process.env
): ParsedOptions {
  let theme: ThemeName = "tokyonight";
  let themeMode: ThemeModePreference = "auto";
  let motion: MotionMode = DEFAULT_MOTION_MODE;
  let spinner: SpinnerType = DEFAULT_SPINNER;
  let engine: string | undefined;
  let profile: string | undefined;
  let config: string | undefined;
  let timeout: string | undefined;
  let redaction: Redaction | undefined;
  let noCache = false;
  let fixture = false;

  for (let index = 0; index < args.length; index += 1) {
    const argument = args[index]!;
    if (argument === "-h" || argument === "--help") {
      if (args.length !== 1) throw new OptionError("--help cannot be combined with other options.");
      return {kind: "help"};
    }
    if (argument === "--fixture") {
      if (fixture) throw new OptionError("--fixture may be supplied only once.");
      fixture = true;
      continue;
    }
    if (argument === "--no-cache") {
      if (noCache) throw new OptionError("--no-cache may be supplied only once.");
      noCache = true;
      continue;
    }

    const valueOption = (name: string): string | undefined => {
      const assigned = optionAssignment(argument, name);
      if (assigned !== undefined) return checkedValue(name, assigned);
      if (argument !== name) return undefined;
      const value = checkedValue(name, args[index + 1]);
      index += 1;
      return value;
    };

    const themeValue = valueOption("--theme");
    if (themeValue !== undefined) {
      if (!isThemeName(themeValue)) throw new OptionError(`Unknown theme ${JSON.stringify(themeValue)}.`);
      theme = themeValue;
      continue;
    }
    const modeValue = valueOption("--theme-mode");
    if (modeValue !== undefined) {
      if (!isThemeModePreference(modeValue)) throw new OptionError(`Unknown theme mode ${JSON.stringify(modeValue)}.`);
      themeMode = modeValue;
      continue;
    }
    const spinnerValue = valueOption("--spinner");
    if (spinnerValue !== undefined) {
      if (!isSpinnerType(spinnerValue)) {
        throw new OptionError("--spinner must be braille, hangul, pipe, or dots.");
      }
      spinner = spinnerValue;
      continue;
    }
    const motionValue = valueOption("--motion");
    if (motionValue !== undefined) {
      if (!isMotionMode(motionValue)) {
        throw new OptionError("--motion must be full, reduced, or off.");
      }
      motion = motionValue;
      continue;
    }
    const engineValue = valueOption("--engine");
    if (engineValue !== undefined) {
      engine = assignOnce(engine, "--engine", engineValue);
      continue;
    }
    const profileValue = valueOption("--profile");
    if (profileValue !== undefined) {
      profile = assignOnce(profile, "--profile", profileValue);
      continue;
    }
    const configValue = valueOption("--config");
    if (configValue !== undefined) {
      config = assignOnce(config, "--config", configValue);
      continue;
    }
    const timeoutValue = valueOption("--timeout");
    if (timeoutValue !== undefined) {
      timeout = assignOnce(timeout, "--timeout", timeoutValue);
      continue;
    }
    const redactionValue = valueOption("--redaction");
    if (redactionValue !== undefined) {
      if (redaction !== undefined) throw new OptionError("--redaction may be supplied only once.");
      if (redactionValue !== "standard" && redactionValue !== "share" && redactionValue !== "paranoid") {
        throw new OptionError("--redaction must be standard, share, or paranoid.");
      }
      redaction = redactionValue;
      continue;
    }
    throw new OptionError(`Unknown option ${JSON.stringify(optionForError(argument))}.`);
  }

  const selectedEngine = fixture ? undefined : engine ?? environment.ZSCALERCTL_ENGINE_PATH;
  if (fixture && engine !== undefined) throw new OptionError("--fixture cannot be combined with --engine.");
  const hasEnginePolicy = profile !== undefined || config !== undefined || timeout !== undefined || redaction !== undefined || noCache;
  if (selectedEngine === undefined && hasEnginePolicy) {
    throw new OptionError("Engine policy options require --engine or ZSCALERCTL_ENGINE_PATH.");
  }
  if (selectedEngine !== undefined) {
    const checkedEngine = checkedValue("--engine or ZSCALERCTL_ENGINE_PATH", selectedEngine);
    if (!isAbsolute(checkedEngine)) throw new OptionError("The engine executable path must be absolute.");
    engine = checkedEngine;
  } else {
    engine = undefined;
  }

  return {
    kind: "run",
    options: {
      theme,
      themeMode,
      motion,
      spinner,
      ...(engine === undefined ? {} : {engine}),
      ...(profile === undefined ? {} : {profile}),
      ...(config === undefined ? {} : {config}),
      ...(timeout === undefined ? {} : {timeout}),
      ...(redaction === undefined ? {} : {redaction}),
      ...(noCache ? {noCache: true} : {})
    }
  };
}
