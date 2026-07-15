import {describe, expect, test} from "bun:test";

import {WireNumber, type WireValue} from "../../../clients/typescript/src/index.ts";
import {DEMO_DATA} from "../src/fixture.ts";
import type {ContextState} from "../src/model.ts";
import {
  evidenceAtPath,
  resultTranscriptBlocks,
  textTranscriptBlocks,
  valueAtPath
} from "../src/transcript.ts";

const CONTEXT: ContextState = {
  connection: "fixture",
  transport: "in-memory fixture",
  authority: "tenant read-only",
  scope: "zia/locations",
  records: 2,
  effects: "none",
  operation: {status: "complete", label: "ready"}
};

describe("transcript presentation model", () => {
  test("builds bounded metrics, facets, evidence, and actions from structured data", () => {
    const blocks = resultTranscriptBlocks(["Two records returned."], DEMO_DATA, CONTEXT, {
      facets: [{label: "Active", values: [{label: "true", count: 2}]}]
    });
    const metrics = blocks.find(block => block.kind === "metrics");
    const facets = blocks.find(block => block.kind === "facets");
    const evidence = blocks.find(block => block.kind === "evidence");
    const actions = blocks.find(block => block.kind === "actions");

    expect(metrics).toEqual({
      id: "metrics",
      kind: "metrics",
      items: [
        {label: "Scope", value: "zia/locations", tone: "info"},
        {label: "Records", value: "2", tone: "success"}
      ]
    });
    expect(facets).toEqual({
      id: "facets",
      kind: "facets",
      items: [{label: "Active", values: [{label: "true", count: 2}]}]
    });
    expect(evidence?.items).toHaveLength(2);
    expect(evidence?.items[0]).toMatchObject({
      path: ["records", 0],
      label: "Raleigh Headquarters",
      detail: "7 fields",
      kind: "object",
      preview: "{7}"
    });
    expect(actions?.items.map(item => item.id)).toEqual(["inspect", "find"]);
  });

  test("preserves exact wire numbers and rejects missing or inherited paths", () => {
    const exact = evidenceAtPath(DEMO_DATA, ["records", 0, "id"]);
    expect(exact).toMatchObject({kind: "number", preview: "9007199254740993"});
    expect(valueAtPath(DEMO_DATA, ["records", 0, "id"])).toEqual(new WireNumber("9007199254740993"));
    expect(valueAtPath(DEMO_DATA, ["records", 99])).toBeUndefined();
    expect(valueAtPath({record: {}}, ["record", "toString"])).toBeUndefined();
  });

  test("sanitizes presentation strings and bounds large collection summaries", () => {
    const records = Array.from({length: 501}, (_, index) => ({
      name: index === 0 ? "unsafe\u001b[31m\u202e" : `record-${index}`,
      status: index % 2 === 0 ? "enabled" : "disabled"
    }));
    const data: WireValue = {records};
    const blocks = resultTranscriptBlocks(["line\u001b\u202e"], data);
    const text = blocks.find(block => block.kind === "text");
    const evidence = blocks.find(block => block.kind === "evidence");

    expect(text?.lines[0]).toBe("line��");
    expect(blocks.some(block => block.kind === "facets")).toBe(false);
    expect(evidence?.items).toHaveLength(3);
    expect(evidence?.items[0]?.label).toBe("unsafe�[31m�");
  });

  test("keeps plain text entries as typed blocks", () => {
    expect(textTranscriptBlocks(["hello", "world"])).toEqual([
      {id: "content", kind: "text", lines: ["hello", "world"]}
    ]);
    expect(textTranscriptBlocks([])).toEqual([]);
  });
});
