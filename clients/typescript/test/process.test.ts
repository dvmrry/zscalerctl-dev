import { mkdtemp, readFile, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import { test } from "node:test";
import assert from "node:assert/strict";

import { EngineCanceledError, EngineClientError, EngineOperationError, spawnEngine } from "../src/index.ts";

const executable = process.env.ZSCALERCTL_ENGINE_TEST_BINARY;
const bootstrapHelper = fileURLToPath(new URL("./fixtures/bootstrap-helper.cjs", import.meta.url));
const stderrFloodHelper = fileURLToPath(new URL("./fixtures/stderr-flood-engine.cjs", import.meta.url));

async function waitForHelperPID(path: string): Promise<number> {
  for (let attempt = 0; attempt < 100; attempt += 1) {
    try {
      const value = Number.parseInt(await readFile(path, "utf8"), 10);
      if (Number.isSafeInteger(value) && value > 0) return value;
    } catch {
      // The helper writes its PID after the process starts.
    }
    await new Promise((resolve) => setTimeout(resolve, 10));
  }
  throw new Error("bootstrap helper did not write its PID");
}

function assertProcessExited(pid: number): void {
  assert.throws(
    () => process.kill(pid, 0),
    (error: unknown) => typeof error === "object" && error !== null && "code" in error && error.code === "ESRCH",
  );
}

async function bootstrapHelperOptions(mode: "silent" | "hello"): Promise<{
  readonly directory: string;
  readonly pidFile: string;
  readonly options: Parameters<typeof spawnEngine>[0];
}> {
  const directory = await mkdtemp(join(tmpdir(), "zscalerctl-bootstrap-helper-"));
  const pidFile = join(directory, "pid");
  return {
    directory,
    pidFile,
    options: {
      executable: process.execPath,
      startupTimeoutMs: 500,
      env: {
        PATH: process.env.PATH,
        HOME: directory,
        LANG: "C",
        NODE_OPTIONS: `--require=${bootstrapHelper}`,
        ZSCALERCTL_BOOTSTRAP_HELPER_MODE: mode,
        ZSCALERCTL_BOOTSTRAP_HELPER_PID_FILE: pidFile,
      },
    },
  };
}

for (const mode of ["silent", "hello"] as const) {
  test(`bootstrap timeout terminates helper in ${mode} mode`, async () => {
    const helper = await bootstrapHelperOptions(mode);
    try {
      const connection = spawnEngine(helper.options);
      const rejected = assert.rejects(
        connection,
        (error: unknown) => error instanceof EngineClientError && error.kind === "transport" &&
          error.message === "engine bootstrap timed out",
      );
      const pid = await waitForHelperPID(helper.pidFile);
      await rejected;
      assertProcessExited(pid);
    } finally {
      await rm(helper.directory, { recursive: true, force: true });
    }
  });
}

test("bootstrap AbortSignal terminates the hidden helper process", async () => {
  const helper = await bootstrapHelperOptions("silent");
  const controller = new AbortController();
  try {
    const connection = spawnEngine({ ...helper.options, startupTimeoutMs: 5_000, signal: controller.signal });
    const rejected = assert.rejects(connection, (error: unknown) => error instanceof EngineCanceledError);
    const pid = await waitForHelperPID(helper.pidFile);
    controller.abort();
    await rejected;
    assertProcessExited(pid);
  } finally {
    await rm(helper.directory, { recursive: true, force: true });
  }
});

test("stderr flood cannot kill an active dump", {
  skip: process.platform === "win32" ? "POSIX executable fixture" : false,
}, async () => {
  const directory = await mkdtemp(join(tmpdir(), "zscalerctl-stderr-helper-"));
  try {
    const client = await spawnEngine({
      executable: stderrFloodHelper,
      startupTimeoutMs: 2_000,
      env: { PATH: process.env.PATH, HOME: directory, LANG: "C" },
    });
    const response = await client.dump({
      output_dir: join(directory, "dump"),
      products: [],
      resources: [],
      continue_on_error: false,
      force: false,
    });
    assert.equal(response.result.kind, "dump_summary");
    assert.equal(response.result.resources_written, 0);
    await client.close();
  } finally {
    await rm(directory, { recursive: true, force: true });
  }
});

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
