import {expect, test} from "bun:test";

test("README distinguishes active Ctrl+C cancellation from idle exit", async () => {
  const readme = await Bun.file(new URL("../README.md", import.meta.url)).text();
  expect(readme).toContain("Ctrl+C | Cancel the active engine operation; exit and restore the terminal when idle");
  expect(readme).toContain("/quit` | Close the engine, exit, and restore the terminal");
});
