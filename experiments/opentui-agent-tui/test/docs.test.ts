import {expect, test} from "bun:test";

test("README documents interaction controls and spinner choices", async () => {
  const readme = await Bun.file(new URL("../README.md", import.meta.url)).text();
  expect(readme).toContain("Ctrl+C | Cancel the active engine operation; exit and restore the terminal when idle");
  expect(readme).toContain("/quit` | Close the engine, exit, and restore the terminal");
  expect(readme).toContain("Ctrl+B / Ctrl+F in a text input | Move the editing cursor backward / forward");
  expect(readme).toContain("Tab | Accept the selected autocomplete suggestion; when none exists, move focus forward");
  expect(readme).toContain("P while tree is focused | Pin the selected JSON value to the working set");
  expect(readme).toContain("`/unpin all` clears the working set without adding command noise");
  expect(readme).toContain("Transcript summaries are deterministic presentation metadata, not model\n  context");
  expect(readme).toContain("--spinner braille|hangul|pipe|dots");
  expect(readme).toContain("Hangul sequence by\ndefault");
});
