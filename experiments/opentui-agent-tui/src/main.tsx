import {createCliRenderer} from "@opentui/core";
import {createRoot} from "@opentui/react";

import {App} from "./App.tsx";
import {OptionError, parseOptions, USAGE} from "./options.ts";
import {modeFromBackground, type ThemeMode} from "./theme.ts";
import {FIXTURE_WORKSPACE_ADAPTER} from "./workspace.ts";
import {createZscalerctlWorkspace} from "./zscalerctl/adapter.ts";

async function main(): Promise<void> {
  let parsed;
  try {
    parsed = parseOptions(process.argv.slice(2));
  } catch (error) {
    const message = error instanceof OptionError ? error.message : "Invalid experiment options.";
    process.stderr.write(`${message}\n\n${USAGE}\n`);
    process.exitCode = 2;
    return;
  }
  if (parsed.kind === "help") {
    process.stdout.write(`${USAGE}\n`);
    return;
  }
  if (process.stdin.isTTY !== true || process.stdout.isTTY !== true) {
    process.stderr.write("The OpenTUI experiment requires an interactive terminal on stdin and stdout.\n");
    process.exitCode = 2;
    return;
  }
  const workspace = parsed.options.engine === undefined
    ? FIXTURE_WORKSPACE_ADAPTER
    : createZscalerctlWorkspace({...parsed.options, engine: parsed.options.engine});
  let didDestroy!: () => void;
  const destroyed = new Promise<void>(resolve => { didDestroy = resolve; });
  const renderer = await createCliRenderer({
    clearOnShutdown: true,
    enableMouseMovement: true,
    exitOnCtrlC: false,
    maxFps: 60,
    onDestroy: didDestroy,
    screenMode: "alternate-screen",
    targetFps: 30,
    useMouse: true
  });
  let initialMode: ThemeMode = parsed.options.themeMode === "auto" ? "dark" : parsed.options.themeMode;
  if (parsed.options.themeMode === "auto") {
    try {
      const terminal = await renderer.getPalette({size: 16, timeout: 120});
      initialMode = modeFromBackground(terminal.defaultBackground) ?? "dark";
    } catch {
      initialMode = "dark";
    }
  }
  createRoot(renderer).render(
    <App
      initialMode={initialMode}
      initialTheme={parsed.options.theme}
      initialMotion={parsed.options.motion}
      spinner={parsed.options.spinner}
      workspace={workspace}
    />
  );
  await destroyed;
  try {
    await workspace.close();
  } catch {
    process.exitCode = 1;
  }
}

await main();
