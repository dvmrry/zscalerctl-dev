import {describe, expect, test} from "bun:test";

import {
  CommandSyntaxError,
  parseCommand,
  tokenizeCommand
} from "../src/zscalerctl/commands.ts";

describe("zscalerctl command grammar", () => {
  test("splits syntax without evaluating shell expressions", () => {
    expect(tokenizeCommand("/lookup 'https://example.test/a b' $(whoami) `id` $HOME")).toEqual([
      "/lookup",
      "https://example.test/a b",
      "$(whoami)",
      "`id`",
      "$HOME"
    ]);
  });

  test("supports bounded quoting while rejecting controls and format characters", () => {
    expect(tokenizeCommand("/catalog zia url\\ filtering")).toEqual(["/catalog", "zia", "url filtering"]);
    expect(() => tokenizeCommand("/catalog 'unterminated")).toThrow(CommandSyntaxError);
    expect(() => tokenizeCommand("/catalog trailing\\")).toThrow(CommandSyntaxError);
    expect(() => tokenizeCommand("/catalog\nzia")).toThrow(CommandSyntaxError);
    expect(() => tokenizeCommand("/catalog \u202eevil")).toThrow(CommandSyntaxError);
  });

  test("maps typed read and diff options exactly", () => {
    expect(parseCommand("/list ZIA locations --fields id,name,id --filter 'name=foo~bar' --search branch")).toEqual({
      kind: "list",
      product: "zia",
      resource: "locations",
      fields: ["id", "name"],
      filters: [{field: "name", operator: "exact", value: "foo~bar"}],
      search: "branch"
    });
    expect(parseCommand("/diff '/tmp/old dump' /tmp/new --product zia --resource zpa/app-segments --resource zpa/app-segments --ignore-operational --allow-partial")).toEqual({
      kind: "diff",
      oldDirectory: "/tmp/old dump",
      newDirectory: "/tmp/new",
      products: ["zia"],
      resources: [{product: "zpa", resource: "app-segments"}],
      ignoreOperational: true,
      allowPartial: true
    });
  });

  test("rejects shell, dump, malformed resources, and unsupported read options", () => {
    expect(() => parseCommand("/shell whoami")).toThrow(/Unknown command/);
    expect(() => parseCommand("/dump --out /tmp/x")).toThrow(/Unknown command/);
    expect(() => parseCommand("/list nope locations")).toThrow(/Unknown product/);
    expect(() => parseCommand("/get zia locations 1 --filter name=x")).toThrow(/Unknown option/);
    expect(() => parseCommand("/diff /tmp/a /tmp/b --resource malformed")).toThrow(/product\/resource/);
  });
});
