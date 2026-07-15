import {describe, expect, test} from "bun:test";

import {
  filterWorkspacePicker,
  normalizeWorkspacePicker,
  type WorkspacePickerItem
} from "../src/workspace.ts";

const ITEMS: readonly WorkspacePickerItem[] = [
  {id: "zia/locations", title: "locations", description: "list, get · 2 fields", searchText: "name country", category: "ZIA", badge: "list", command: "/list zia locations"},
  {id: "zpa/app-segments", title: "app-segments", description: "list, get · name domain", category: "ZPA", badge: "list", command: "/list zpa app-segments"},
  {id: "ztw/advanced-settings", title: "advanced-settings", description: "show · enabled", category: "ZTW", badge: "singleton", command: "/show ztw advanced-settings"}
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

  test("normalizes every adapter-owned picker presentation field", () => {
    const hostile = "safe\u001b[31m\u202e";
    const normalized = normalizeWorkspacePicker({
      title: hostile,
      placeholder: hostile,
      instruction: hostile,
      emptyMessage: hostile,
      initialQuery: hostile,
      items: [{
        id: "stable",
        title: hostile,
        description: hostile,
        category: hostile,
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
  });
});
