import {describe, expect, test} from "bun:test";

import {WireNumber, type WireValue} from "../../../clients/typescript/src/index.ts";
import {DEMO_DATA} from "../src/fixture.ts";
import type {ContextState} from "../src/model.ts";
import {
  evidenceAtPath,
  resultTranscriptBlocks,
  snapshotWireValue,
  textTranscriptBlocks,
  valueAtPath
} from "../src/transcript.ts";

const CONTEXT: ContextState = {
  connection: "fixture",
  transport: "in-memory fixture",
  authority: "tenant read-only",
  scope: "zia/locations",
  records: 2,
  countLabel: "Records",
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

  test("sanitizes fallback collection labels at the transcript metric leaf", () => {
    const blocks = resultTranscriptBlocks([], {"unsafe\u200bCollection": []} as WireValue);
    const metrics = blocks.find(block => block.kind === "metrics");

    expect(metrics?.items).toEqual([{label: "Unsafe�collection", value: "0"}]);
  });

  test("keeps plain text entries as typed blocks", () => {
    expect(textTranscriptBlocks(["hello", "world"])).toEqual([
      {id: "content", kind: "text", lines: ["hello", "world"]}
    ]);
    expect(textTranscriptBlocks([])).toEqual([]);
  });

  test("commits an immutable wire snapshot without losing exact numbers", () => {
    const source: WireValue = {
      records: [{id: new WireNumber("900719925474099312345"), name: "Original"}]
    };
    const snapshot = snapshotWireValue(source);
    const sourceRecords = (source as {records: Array<{name: string}>}).records;
    sourceRecords[0]!.name = "Mutated";
    sourceRecords.reverse();

    expect(valueAtPath(snapshot, ["records", 0, "name"])).toBe("Original");
    expect(valueAtPath(snapshot, ["records", 0, "id"])).toEqual(new WireNumber("900719925474099312345"));
    expect(Object.isFrozen(snapshot)).toBe(true);
    expect(Object.isFrozen(valueAtPath(snapshot, ["records"]) as object)).toBe(true);
    expect(Object.isFrozen(valueAtPath(snapshot, ["records", 0]) as object)).toBe(true);

    const cyclic: {self?: unknown} = {};
    cyclic.self = cyclic;
    expect(() => snapshotWireValue(cyclic as WireValue)).toThrow("Workspace data contains a cycle.");
  });

  test("uses an explicit semantic count label instead of assuming records", () => {
    const blocks = resultTranscriptBlocks([], {resources: [{}, {}, {}]}, {
      ...CONTEXT,
      scope: "catalog",
      records: 3,
      countLabel: "Resources"
    });
    const metrics = blocks.find(block => block.kind === "metrics");
    expect(metrics?.items.map(item => [item.label, item.value])).toEqual([
      ["Scope", "catalog"],
      ["Resources", "3"]
    ]);
    expect(metrics?.items.some(item => item.label === "Records")).toBe(false);
  });
});
