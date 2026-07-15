import {describe, expect, test} from "bun:test";

import {
  filterWorkspacePicker,
  normalizeWorkspacePicker,
  type WorkspacePickerItem
} from "../src/workspace.ts";

const ITEMS: readonly WorkspacePickerItem[] = [
  {id: "zia/locations", title: "locations", description: "list, get · 2 fields", searchText: "name country", category: "ZIA", scopeId: "zia", badge: "list", command: "/list zia locations"},
  {id: "zpa/app-segments", title: "app-segments", description: "list, get · name domain", category: "ZPA", scopeId: "zpa", badge: "list", command: "/list zpa app-segments"},
  {id: "ztw/advanced-settings", title: "advanced-settings", description: "show · enabled", category: "ZTW", scopeId: "ztw", badge: "singleton", command: "/show ztw advanced-settings"}
];

describe("workspace resource picker", () => {
  test("matches every query term across resource metadata", () => {
    expect(filterWorkspacePicker(ITEMS, "zia country").items.map(item => item.id)).toEqual(["zia/locations"]);
    expect(filterWorkspacePicker(ITEMS, "singleton enabled").items.map(item => item.id)).toEqual(["ztw/advanced-settings"]);
  });

  test("preserves catalog order and reports bounded truncation", () => {
    const filtered = filterWorkspacePicker(ITEMS, "", 2);
    expect(filtered.items.map(item => item.id)).toEqual(["zia/locations", "zpa/app-segments"]);
    expect(filtered.truncated).toBe(true);
  });

  test("searches only within the selected product scope", () => {
    expect(filterWorkspacePicker(ITEMS, "", {scopeId: "zpa"}).items.map(item => item.id)).toEqual(["zpa/app-segments"]);
    expect(filterWorkspacePicker(ITEMS, "locations", {scopeId: "zpa"}).items).toEqual([]);
    expect(filterWorkspacePicker(ITEMS, "locations").items.map(item => item.id)).toEqual(["zia/locations"]);
  });

  test("normalizes every adapter-owned picker presentation field", () => {
    const hostile = "safe\u001b[31m\u202e";
    const normalized = normalizeWorkspacePicker({
      title: hostile,
      placeholder: hostile,
      instruction: hostile,
      emptyMessage: hostile,
      initialQuery: hostile,
      scopeLabel: hostile,
      scopes: [{id: "zia", label: hostile, count: Number.NaN}],
      items: [{
        id: "stable",
        title: hostile,
        description: hostile,
        category: hostile,
        scopeId: "zia",
        badge: hostile,
        searchText: hostile,
        command: "/safe"
      }]
    });
    const presentation = [
      normalized.title,
      normalized.placeholder,
      normalized.instruction,
      normalized.emptyMessage,
      normalized.initialQuery,
      normalized.scopeLabel,
      normalized.scopes?.[0]?.label,
      normalized.items[0]?.title,
      normalized.items[0]?.description,
      normalized.items[0]?.category,
      normalized.items[0]?.badge,
      normalized.items[0]?.searchText
    ].join("|");
    expect(presentation).not.toContain("\u001b");
    expect(presentation).not.toContain("\u202e");
    expect(presentation).toContain("�");
    expect(normalized.items[0]?.id).toBe("stable");
    expect(normalized.items[0]?.command).toBe("/safe");
    expect(normalized.scopes?.[0]?.id).toBe("zia");
    expect(normalized.scopes?.[0]?.count).toBe(1);
  });

  test("preserves opaque scope IDs while deduplicating and deriving truthful counts", () => {
    const normalized = normalizeWorkspacePicker({
      title: "Resources",
      placeholder: "Search",
      instruction: "Choose",
      emptyMessage: "Empty",
      scopes: [
        {id: "__all__", label: "First", count: 99},
        {id: "dup", label: "Kept", count: -1},
        {id: "dup", label: "Dropped", count: 50},
        {id: "stale", label: "Stale", count: 25}
      ],
      items: [
        {id: "first", title: "first", description: "first", category: "First", scopeId: "__all__", command: "/first"},
        {id: "second", title: "second", description: "second", category: "Second", scopeId: "dup", command: "/second"},
        {id: "third", title: "third", description: "third", category: "Second", scopeId: "dup", command: "/third"}
      ]
    });
    expect(normalized.scopes).toEqual([
      {id: "__all__", label: "First", count: 1},
      {id: "dup", label: "Kept", count: 2}
    ]);
  });
});
