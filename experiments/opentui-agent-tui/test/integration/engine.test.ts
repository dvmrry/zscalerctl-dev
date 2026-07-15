import {expect, test} from "bun:test";

import {createZscalerctlWorkspace} from "../../src/zscalerctl/adapter.ts";
import {parseCommand} from "../../src/zscalerctl/commands.ts";

const engine = process.env.ZSCALERCTL_ENGINE_TEST_BINARY;
const context = {signal: new AbortController().signal, emit: () => undefined};

test.skipIf(engine === undefined)("real engine supports config-free discovery through the OpenTUI adapter", async () => {
  expect(engine).toBeDefined();
  const workspace = createZscalerctlWorkspace({
    theme: "tokyonight",
    themeMode: "dark",
    engine: engine!
  });
  try {
    const connected = await workspace.connect!(context);
    expect(connected.announcement.title).toBe("Engine connected");
    expect(connected.context?.records).toBeGreaterThan(0);
    const manifest = await workspace.execute!("/manifest", context);
    expect(manifest.announcement.title).toBe("Engine capabilities");
    const catalog = await workspace.execute!("/catalog zia location", context);
    expect(catalog.announcement.title).toBe("Resource catalog");
    expect(catalog.picker?.items.length).toBeGreaterThan(0);
    for (const item of catalog.picker?.items ?? []) {
      const command = parseCommand(item.command);
      expect(command.kind === "list" || command.kind === "show").toBe(true);
    }
  } finally {
    await workspace.close();
  }
});
