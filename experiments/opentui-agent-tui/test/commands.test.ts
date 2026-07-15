import {describe, expect, test} from "bun:test";

import {
  activeInteractionMode,
  resolveInteractionCommand
} from "../src/commands.ts";

describe("interaction command routing", () => {
  test("uses an explicit overlay priority", () => {
    expect(activeInteractionMode({search: false, inspector: false, drawer: false})).toBe("base");
    expect(activeInteractionMode({search: false, inspector: false, drawer: true})).toBe("drawer");
    expect(activeInteractionMode({search: false, inspector: true, drawer: true})).toBe("inspector");
    expect(activeInteractionMode({search: true, inspector: true, drawer: true})).toBe("search");
    expect(activeInteractionMode({search: true, picker: true, inspector: true, drawer: true})).toBe("picker");
  });

  test("scopes picker commands without stealing ordinary query text", () => {
    expect(resolveInteractionCommand("search", {name: "return"})).toBe("search.commit");
    expect(resolveInteractionCommand("search", {name: "pagedown"})).toBe("search.page-next");
    expect(resolveInteractionCommand("search", {name: "o", ctrl: true})).toBe("search.inspect");
    expect(resolveInteractionCommand("search", {name: "c"})).toBeUndefined();
    expect(resolveInteractionCommand("search", {name: "c", shift: true})).toBe("search.copy-value");
    expect(resolveInteractionCommand("search", {name: "p", shift: true})).toBe("search.copy-path");
  });

  test("keeps global and base commands deterministic", () => {
    expect(resolveInteractionCommand("picker", {name: "c", ctrl: true})).toBe("app.interrupt");
    expect(resolveInteractionCommand("search", {name: "f", ctrl: true})).toBe("search.toggle");
    expect(resolveInteractionCommand("base", {name: "b", ctrl: true})).toBe("sidebar.toggle");
    expect(resolveInteractionCommand("inspector", {name: "escape"})).toBe("overlay.close");
    expect(resolveInteractionCommand("base", {name: "tab", shift: true})).toBe("focus.previous");
  });

  test("routes resource picker navigation ahead of the base shell", () => {
    expect(resolveInteractionCommand("picker", {name: "return"})).toBe("picker.commit");
    expect(resolveInteractionCommand("picker", {name: "down"})).toBe("picker.next");
    expect(resolveInteractionCommand("picker", {name: "pagedown"})).toBe("picker.page-next");
    expect(resolveInteractionCommand("picker", {name: "escape"})).toBe("picker.cancel");
  });
});
