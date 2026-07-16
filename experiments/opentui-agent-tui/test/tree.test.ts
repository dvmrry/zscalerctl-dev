import {describe, expect, test} from "bun:test";

import {WireNumber, type WireValue} from "../../../clients/typescript/src/index.ts";
import {DEMO_DATA} from "../src/fixture.ts";
import {
  expandAncestors,
  flattenTree,
  formatBreadcrumb,
  formatPath,
  initialExpansion,
  pathKey,
  searchTree,
  toggleExpansion
} from "../src/tree.ts";

describe("JSON tree model", () => {
  test("flattens only expanded branches and preserves exact wire numbers", () => {
    const rows = flattenTree(DEMO_DATA, initialExpansion(DEMO_DATA));
    expect(rows.some(row => formatPath(row.path) === "$.records[0].address")).toBe(true);
    expect(rows.some(row => formatPath(row.path) === "$.records[1].address")).toBe(false);

    const id = rows.find(row => formatPath(row.path) === "$.records[0].id");
    const firstRecord = rows.find(row => formatPath(row.path) === "$.records[0]");
    expect(id?.kind).toBe("number");
    expect(id?.preview).toBe("9007199254740993");
    expect(firstRecord?.label).toBe("Raleigh Headquarters");
    expect(firstRecord?.preview).toBe("{7} · index 0");
  });

  test("orders named array objects without changing their source paths", () => {
    const value: WireValue = {
      items: [
        {name: "Zulu", id: new WireNumber("1")},
        {name: "Alpha", id: new WireNumber("2")}
      ]
    };
    const expanded = new Set([pathKey([]), pathKey(["items"])]);
    const labels = (order: "index" | "name") => flattenTree(value, expanded, {arrayOrder: order})
      .filter(row => row.path.length === 2)
      .map(row => ({label: row.label, path: row.path, sourceIndex: row.sourceIndex}));

    expect(labels("index").map(row => row.label)).toEqual(["Zulu", "Alpha"]);
    expect(labels("name").map(row => row.label)).toEqual(["Alpha", "Zulu"]);
    expect(labels("name")[0]).toEqual({label: "Alpha", path: ["items", 1], sourceIndex: 1});
  });

  test("toggle expansion returns new state and leaves scalar rows unchanged", () => {
    const expanded = initialExpansion(DEMO_DATA);
    const rows = flattenTree(DEMO_DATA, expanded);
    const record = rows.find(row => row.id === pathKey(["records", 0]));
    const scalar = rows.find(row => row.id === pathKey(["records", 0, "name"]));
    expect(record).toBeDefined();
    expect(scalar).toBeDefined();

    const collapsed = toggleExpansion(expanded, record!);
    expect(collapsed).not.toBe(expanded);
    expect(collapsed.has(record!.id)).toBe(false);
    expect(toggleExpansion(collapsed, scalar!)).toBe(collapsed);
  });

  test("bounds visible output", () => {
    const value: WireValue = {items: Array.from({length: 100}, (_, index) => new WireNumber(String(index)))};
    const expanded = new Set([pathKey([]), pathKey(["items"])]);
    expect(flattenTree(value, expanded, {maximumRows: 12})).toHaveLength(12);
  });

  test("formats paths without making ambiguous dotted keys", () => {
    expect(formatPath(["ordinary", 2, "hyphen-name"])).toBe('$.ordinary[2]["hyphen-name"]');
    expect(formatPath(["space name"])).toBe('$["space name"]');
  });

  test("formats a readable breadcrumb from visible tree labels", () => {
    const rows = flattenTree(DEMO_DATA, initialExpansion(DEMO_DATA));
    const address = rows.find(row => row.id === pathKey(["records", 0, "address"]));
    expect(address).toBeDefined();
    expect(formatBreadcrumb(rows, address!)).toBe("records › Raleigh Headquarters › address");
  });

  test("searches friendly labels and scalar values inside collapsed branches", () => {
    const byName = searchTree(DEMO_DATA, "seoul");
    const byValue = searchTree(DEMO_DATA, "서울");
    const byExactNumber = searchTree(DEMO_DATA, "9007199254740995");
    const byHumanizedKey = searchTree(DEMO_DATA, "download mbps");

    expect(byName.matches).toHaveLength(1);
    expect(byName.matches[0]).toMatchObject({
      id: pathKey(["records", 1]),
      title: "Seoul Branch",
      context: "Record",
      detail: "7 fields",
      reason: "identity"
    });
    expect(byValue.matches[0]).toMatchObject({
      id: pathKey(["records", 1, "address", "city"]),
      title: "Seoul Branch",
      context: "Address › City",
      detail: "서울",
      reason: "value"
    });
    expect(byExactNumber.matches[0]).toMatchObject({
      id: pathKey(["records", 1, "id"]),
      title: "Seoul Branch",
      context: "ID",
      detail: "9007199254740995",
      reason: "value"
    });
    expect(formatPath(byExactNumber.matches[0]!.path)).toBe("$.records[1].id");
    expect(byHumanizedKey.matches).toHaveLength(2);
    expect(byHumanizedKey.matches[1]).toMatchObject({
      title: "Seoul Branch",
      context: "Network › Bandwidth › Download Mbps",
      detail: "500",
      reason: "key"
    });
  });

  test("uses a stable ID identity when a record has no friendly name", () => {
    const value: WireValue = {records: [{id: new WireNumber("42"), status: "ready"}]};
    const result = searchTree(value, "42");

    expect(result.matches).toHaveLength(1);
    expect(result.matches[0]).toMatchObject({
      id: pathKey(["records", 0]),
      title: "ID 42",
      context: "Record",
      detail: "2 fields",
      reason: "identity"
    });
  });

  test("ranks record names before fields and values while limiting fuzzy matching to labels", () => {
    const fuzzyName = searchTree(DEMO_DATA, "seol brnch");
    const fuzzyField = searchTree(DEMO_DATA, "dwnld mbps");
    const fuzzyValue = searchTree(DEMO_DATA, "prodction");

    expect(fuzzyName.matches[0]).toMatchObject({title: "Seoul Branch", context: "Record", reason: "identity"});
    expect(fuzzyField.matches.map(match => match.context)).toEqual([
      "Network › Bandwidth › Download Mbps",
      "Network › Bandwidth › Download Mbps"
    ]);
    expect(fuzzyValue.matches).toEqual([]);

    const ranked: WireValue = {
      records: [
        {name: "needle"},
        {name: "field record", needle: "other"},
        {name: "value record", status: "needle"}
      ]
    };
    expect(searchTree(ranked, "needle").matches.map(match => match.reason)).toEqual([
      "identity",
      "key",
      "value"
    ]);
  });

  test("keeps matches from the same logical record contiguous and exposes safe copy text", () => {
    const grouped = searchTree(DEMO_DATA, "500");
    expect(grouped.matches.map(match => match.title)).toEqual([
      "Raleigh Headquarters",
      "Seoul Branch",
      "Seoul Branch"
    ]);
    expect(grouped.matches[1]?.groupId).toBe(grouped.matches[2]?.groupId);
    expect(grouped.matches[0]?.copyText).toBe("500");

    const exactNumber = searchTree(DEMO_DATA, "9007199254740993").matches[0];
    const record = searchTree(DEMO_DATA, "Raleigh Headquarters").matches[0];
    expect(exactNumber?.copyText).toBe("9007199254740993");
    expect(record?.copyText).toBeUndefined();
  });

  test("bounds search work and expands only a selected match's ancestors", () => {
    const value: WireValue = {items: Array.from({length: 20}, (_, index) => `match-${index}`)};
    const result = searchTree(value, "match", {maximumMatches: 3});
    expect(result.matches).toHaveLength(3);
    expect(result.truncated).toBe(true);

    const expanded = expandAncestors(new Set(), ["items", 12]);
    expect([...expanded]).toEqual([pathKey([]), pathKey(["items"])]);
  });
});
