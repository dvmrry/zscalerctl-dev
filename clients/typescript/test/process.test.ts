import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { test } from "node:test";
import assert from "node:assert/strict";

import { EngineOperationError, spawnEngine } from "../src/index.ts";

const executable = process.env.ZSCALERCTL_ENGINE_TEST_BINARY;

test("real Go process negotiates, serves config-free reads, and closes cleanly", {
  skip: executable === undefined ? "ZSCALERCTL_ENGINE_TEST_BINARY is not set" : false,
}, async () => {
  assert.notEqual(executable, undefined);
  const home = await mkdtemp(join(tmpdir(), "zscalerctl-ts-client-"));
  const env: NodeJS.ProcessEnv = {
    PATH: process.env.PATH,
    HOME: home,
    XDG_CONFIG_HOME: join(home, "xdg"),
    LANG: "C",
  };
  const client = await spawnEngine({ executable: executable as string, env });
  try {
    const manifest = await client.manifest();
    assert.equal(manifest.result.kind, "engine_manifest");
    assert.equal(manifest.result.manifest.tenant_read_only, true);

    const catalog = await client.catalog();
    assert.ok(catalog.items.length > 0);
    assert.equal(catalog.result.resources, catalog.items.length);

    const config = await client.configStatus();
    assert.equal(config.result.kind, "config_status");
    assert.equal(config.result.status.config_file_set, false);

    await assert.rejects(
      client.list({ product: "zia", resource: "locations", fields: [], filters: [], search: "" }),
      (error: unknown) => error instanceof EngineOperationError && error.failure.kind === "missing_credentials",
    );
  } finally {
    await client.close();
    await rm(home, { recursive: true, force: true });
  }
});
