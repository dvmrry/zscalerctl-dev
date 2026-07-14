const { writeFileSync, writeSync } = require("node:fs");

const pidFile = process.env.ZSCALERCTL_BOOTSTRAP_HELPER_PID_FILE;
if (pidFile === undefined || pidFile === "") {
  process.exit(2);
}
writeFileSync(pidFile, String(process.pid), { encoding: "utf8", mode: 0o600 });
if (process.env.ZSCALERCTL_BOOTSTRAP_HELPER_MODE === "hello") {
  writeSync(1, `${JSON.stringify({
    type: "hello",
    protocol: "zscalerctl.engine.stdio",
    versions: ["1"],
    bootstrap: { frame_bytes: 65_536, json_depth: 8 },
  })}\n`);
}
Atomics.wait(new Int32Array(new SharedArrayBuffer(4)), 0, 0);
