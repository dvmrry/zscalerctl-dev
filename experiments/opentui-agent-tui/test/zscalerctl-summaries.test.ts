import {describe, expect, test} from "bun:test";

import type {
  CatalogResource,
  DiffSummary,
  EngineManifest
} from "../../../clients/typescript/src/index.ts";
import {
  summarizeCatalog,
  summarizeDiff,
  summarizeManifest,
  summarizeRead
} from "../src/zscalerctl/summaries.ts";

const RESOURCES: readonly CatalogResource[] = [
  {
    product: "zpa",
    name: "do-not-surface-resource-name",
    shape: "singleton",
    operations: ["show"],
    fields: []
  },
  {
    product: "zia",
    name: "locations",
    shape: "list",
    operations: ["list", "get"],
    get_key: "id",
    fields: []
  },
  {
    product: "zia",
    name: "rules",
    shape: "list",
    operations: ["list"],
    fields: []
  }
];

describe("zscalerctl command summaries", () => {
  test("summarizes catalog composition in fixed semantic order without resource data", () => {
    const expected = {
      metrics: [{label: "Available", value: "4"}],
      facets: [
        {label: "Products", values: [{label: "ZIA", count: 2}, {label: "ZPA", count: 1}]},
        {label: "Shapes", values: [{label: "List", count: 2}, {label: "Singleton", count: 1}]},
        {label: "Operations", values: [{label: "List", count: 2}, {label: "Get", count: 1}, {label: "Show", count: 1}]}
      ]
    };
    expect(summarizeCatalog(RESOURCES, 4)).toEqual(expected);
    expect(summarizeCatalog([...RESOURCES].reverse(), 4)).toEqual(expected);
    expect(JSON.stringify(expected)).not.toContain("do-not-surface");
  });

  test("summarizes negotiated operations and declared effects", () => {
    const manifest: EngineManifest = {
      version: "engine.v1",
      tenant_read_only: true,
      capabilities: [
        {
          name: "resources.read",
          operations: ["list", "get"],
          input: "resource_read",
          result: "resource_read_summary",
          tenant_read_only: true,
          effects: [
            {kind: "network_access", when: "always"},
            {kind: "process_execution", when: "configuration_dependent"}
          ]
        },
        {
          name: "diff.compare",
          operations: ["diff"],
          input: "diff",
          result: "diff_summary",
          tenant_read_only: true,
          effects: [{kind: "local_filesystem_read", when: "always"}]
        }
      ]
    };
    expect(summarizeManifest(manifest)).toEqual({
      metrics: [
        {label: "Version", value: "engine.v1"},
        {label: "Tenant read only", value: "yes", tone: "success"}
      ],
      facets: [
        {label: "Operations", values: [{label: "List", count: 1}, {label: "Get", count: 1}, {label: "Diff", count: 1}]},
        {label: "Declared effects", values: [
          {label: "Local Filesystem Read", count: 1},
          {label: "Network Access", count: 1},
          {label: "Process Execution", count: 1}
        ]}
      ]
    });
  });

  test("reports read modifiers without copying query text or identifiers", () => {
    const summary = summarizeRead({
      operation: "list",
      fields: ["id", "name"],
      filterCount: 2,
      hasSearch: true
    });
    expect(summary).toEqual({metrics: [
      {label: "Operation", value: "list"},
      {label: "Fields", value: "2"},
      {label: "Filters", value: "2"},
      {label: "Search", value: "applied"}
    ]});
    expect(JSON.stringify(summary)).not.toContain("id");
    expect(JSON.stringify(summary)).not.toContain("name");
  });

  test("separates diff resource totals, record changes, and partial-state notes", () => {
    const result: DiffSummary = {
      kind: "diff_summary",
      schema: "zscalerctl.diff.v1",
      old: {side: "old", manifest_schema: "zscalerctl.dump.v1", redaction: "standard", status: "complete", partial: false},
      new: {side: "new", manifest_schema: "zscalerctl.dump.v1", redaction: "share", status: "partial", partial: true},
      summary: {resources_compared: 9, resources_with_drift: 2, records_added: 3, records_removed: 4, records_changed: 5},
      has_drift: true,
      stream_items_emitted: 12
    };
    expect(summarizeDiff(result)).toEqual({
      metrics: [
        {label: "Resources", value: "9"},
        {label: "With drift", value: "2", tone: "warning"}
      ],
      facets: [{label: "Record changes", values: [
        {label: "Added", count: 3},
        {label: "Removed", count: 4},
        {label: "Changed", count: 5}
      ]}],
      notes: [
        "Old: complete · standard redaction; New: partial · share redaction",
        "Partial comparison: uncollected resources were not compared."
      ]
    });
  });
});
