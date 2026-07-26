import {isWireNumber, type WireValue} from "../../../clients/typescript/src/index.ts";

import type {
  ContextState,
  TranscriptBlock,
  TranscriptEvidence,
  TranscriptFacet,
  TranscriptMetric
} from "./model.ts";
import {safeInlineText, type SafeString} from "./text.ts";
import {type JsonPath, type PathSegment, valueKind} from "./tree.ts";
import type {WorkspaceSummary} from "./workspace.ts";

const MAXIMUM_FACETS = 3;
const MAXIMUM_FACET_VALUES = 6;
const MAXIMUM_EVIDENCE_ITEMS = 3;
const IDENTITY_FIELDS = ["name", "display_name", "displayName", "title", "label", "id"] as const;
const COLLECTION_FIELDS = ["records", "resources", "changes", "capabilities", "items", "results"] as const;

type WireObject = Readonly<Record<string, WireValue>>;

interface CollectionView {
  readonly label: string;
  readonly path: JsonPath;
  readonly items: readonly WireValue[];
}

function wireObject(value: WireValue): WireObject | undefined {
  if (value === null || Array.isArray(value) || isWireNumber(value) || typeof value !== "object") return undefined;
  return value;
}

function normalizedField(value: string): string {
  return value.replace(/[^A-Za-z0-9]/gu, "").toLowerCase();
}

function humanizeField(value: string): string {
  const words = value
    .replace(/([A-Z]+)([A-Z][a-z])/gu, "$1 $2")
    .replace(/([a-z0-9])([A-Z])/gu, "$1 $2")
    .replace(/[_-]+/gu, " ")
    .trim()
    .split(/\s+/u)
    .filter(Boolean);
  return words.map((word, index) => index === 0
    ? `${word.slice(0, 1).toUpperCase()}${word.slice(1).toLowerCase()}`
    : word.toLowerCase()).join(" ");
}

function scalarText(value: WireValue, maximumCharacters = 72): SafeString | undefined {
  if (typeof value === "string") return safeInlineText(value, maximumCharacters);
  if (typeof value === "boolean") return safeInlineText(String(value));
  if (isWireNumber(value)) return safeInlineText(value.lexeme, maximumCharacters);
  if (value === null) return safeInlineText("null");
  return undefined;
}

function valuePreview(value: WireValue): SafeString {
  if (Array.isArray(value)) return safeInlineText(`[${value.length}]`);
  const object = wireObject(value);
  if (object !== undefined) return safeInlineText(`{${Object.keys(object).length}}`);
  return scalarText(value, 72) ?? safeInlineText("value");
}

function collectionView(value: WireValue): CollectionView | undefined {
  if (Array.isArray(value)) return {label: "items", path: [], items: value};
  const object = wireObject(value);
  if (object === undefined) return undefined;
  for (const field of COLLECTION_FIELDS) {
    const items = object[field];
    if (Array.isArray(items)) return {label: field, path: [field], items};
  }
  for (const [field, items] of Object.entries(object)) {
    if (Array.isArray(items)) return {label: field, path: [field], items};
  }
  return undefined;
}

function identityFor(value: WireValue, fallback: string): string {
  const object = wireObject(value);
  if (object !== undefined) {
    for (const field of IDENTITY_FIELDS) {
      const identity = scalarText(object[field]);
      if (identity === undefined || identity.trim().length === 0) continue;
      return field === "id" ? `ID ${identity}` : identity;
    }
  }
  return fallback;
}

function evidenceDetail(value: WireValue): SafeString {
  const object = wireObject(value);
  if (object !== undefined) {
    const fields = Object.keys(object).length;
    return safeInlineText(`${fields} ${fields === 1 ? "field" : "fields"}`);
  }
  if (Array.isArray(value)) {
    return safeInlineText(`${value.length} ${value.length === 1 ? "item" : "items"}`);
  }
  return valuePreview(value);
}

function evidenceFor(value: WireValue, path: JsonPath, fallback: string): TranscriptEvidence {
  return {
    path,
    label: safeInlineText(identityFor(value, fallback), 80),
    detail: evidenceDetail(value),
    kind: valueKind(value),
    preview: valuePreview(value)
  };
}

function evidenceItems(value: WireValue, collection: CollectionView | undefined): readonly TranscriptEvidence[] {
  if (collection !== undefined) {
    return collection.items.slice(0, MAXIMUM_EVIDENCE_ITEMS).map((item, index) => evidenceFor(
      item,
      [...collection.path, index],
      `Item ${index + 1}`
    ));
  }
  const object = wireObject(value);
  if (object === undefined) return [evidenceFor(value, [], "Result")];
  return Object.entries(object).slice(0, MAXIMUM_EVIDENCE_ITEMS).map(([field, child]) => evidenceFor(
    child,
    [field],
    humanizeField(field)
  ));
}

function summaryFacets(summary: WorkspaceSummary | undefined): readonly TranscriptFacet[] {
  return (summary?.facets ?? []).slice(0, MAXIMUM_FACETS).flatMap(facet => {
    const values = facet.values.slice(0, MAXIMUM_FACET_VALUES).flatMap(value => {
      if (!Number.isSafeInteger(value.count) || value.count < 0) return [];
      return [{label: safeInlineText(value.label, 40), count: value.count}];
    });
    return values.length === 0 ? [] : [{label: safeInlineText(facet.label, 40), values}];
  });
}

function metricsFor(
  value: WireValue,
  context: ContextState | undefined,
  collection: CollectionView | undefined,
  summary: WorkspaceSummary | undefined
): readonly TranscriptMetric[] {
  const metrics: TranscriptMetric[] = [];
  if (context !== undefined) {
    metrics.push({label: safeInlineText("Scope"), value: safeInlineText(context.scope, 80), tone: "info"});
    if (context.countLabel !== undefined) {
      metrics.push({
        label: safeInlineText(context.countLabel, 40),
        value: safeInlineText(String(context.records)),
        tone: context.records === 0 ? "neutral" : "success"
      });
    }
  }
  if (collection !== undefined && (context?.countLabel === undefined || collection.items.length !== context.records)) {
    metrics.push({label: safeInlineText(humanizeField(collection.label), 40), value: safeInlineText(String(collection.items.length))});
  } else if (context === undefined) {
    const object = wireObject(value);
    if (object !== undefined) metrics.push({label: safeInlineText("Fields"), value: safeInlineText(String(Object.keys(object).length))});
  }
  for (const metric of summary?.metrics ?? []) {
    if (metrics.some(existing => normalizedField(existing.label) === normalizedField(metric.label))) continue;
    metrics.push({
      label: safeInlineText(metric.label, 40),
      value: safeInlineText(metric.value, 120),
      ...(metric.tone === undefined ? {} : {tone: metric.tone})
    });
  }
  return metrics.slice(0, 6);
}

export function textTranscriptBlocks(lines: readonly string[]): readonly TranscriptBlock[] {
  if (lines.length === 0) return [];
  return [{id: "content", kind: "text", lines: lines.map(line => safeInlineText(line, 4_096))}];
}

export function resultTranscriptBlocks(
  lines: readonly string[],
  data: WireValue | undefined,
  context?: ContextState,
  summary?: WorkspaceSummary
): readonly TranscriptBlock[] {
  const blocks: TranscriptBlock[] = [...textTranscriptBlocks(lines)];
  if (data === undefined) return blocks;
  const collection = collectionView(data);
  const metrics = metricsFor(data, context, collection, summary);
  const facets = summaryFacets(summary);
  const notes = (summary?.notes ?? []).slice(0, 2).map(note => safeInlineText(note, 240));
  const evidence = evidenceItems(data, collection);
  if (metrics.length > 0) blocks.push({id: "metrics", kind: "metrics", items: metrics});
  if (facets.length > 0) blocks.push({id: "facets", kind: "facets", items: facets});
  if (notes.length > 0) {
    blocks.push({
      id: "notes",
      kind: "text",
      lines: notes
    });
  }
  if (evidence.length > 0) blocks.push({id: "evidence", kind: "evidence", items: evidence});
  blocks.push({
    id: "actions",
    kind: "actions",
    items: [
      {id: "inspect", label: safeInlineText("Inspect"), hint: safeInlineText("Ctrl+O")},
      {id: "find", label: safeInlineText("Find"), hint: safeInlineText("/find")}
    ]
  });
  return blocks;
}

export function snapshotWireValue(value: WireValue): WireValue {
  const memo = new WeakMap<object, WireValue>();
  const active = new WeakSet<object>();

  const snapshot = (candidate: unknown, depth: number): WireValue => {
    if (depth > 128) throw new TypeError("Workspace data exceeds the snapshot depth limit.");
    if (candidate === null || typeof candidate === "string" || typeof candidate === "boolean") return candidate;
    if (isWireNumber(candidate)) return candidate;
    if (typeof candidate !== "object") throw new TypeError("Workspace data is not a valid wire value.");
    if (active.has(candidate)) throw new TypeError("Workspace data contains a cycle.");
    const existing = memo.get(candidate);
    if (existing !== undefined) return existing;

    active.add(candidate);
    if (Array.isArray(candidate)) {
      const copy: WireValue[] = [];
      memo.set(candidate, copy);
      for (const item of candidate) copy.push(snapshot(item, depth + 1));
      active.delete(candidate);
      return Object.freeze(copy) as unknown as WireValue;
    }

    const copy: Record<string, WireValue> = Object.create(null) as Record<string, WireValue>;
    memo.set(candidate, copy);
    for (const key of Object.keys(candidate)) {
      copy[key] = snapshot((candidate as Readonly<Record<string, unknown>>)[key], depth + 1);
    }
    active.delete(candidate);
    return Object.freeze(copy) as WireValue;
  };

  return snapshot(value, 1);
}

export function valueAtPath(value: WireValue, path: JsonPath): WireValue | undefined {
  let current: WireValue = value;
  for (const segment of path) {
    if (typeof segment === "number") {
      if (!Array.isArray(current) || segment < 0 || segment >= current.length) return undefined;
      current = current[segment]!;
      continue;
    }
    const object = wireObject(current);
    if (object === undefined || !Object.hasOwn(object, segment)) return undefined;
    current = object[segment]!;
  }
  return current;
}

export function evidenceAtPath(value: WireValue, path: JsonPath, fallback?: string): TranscriptEvidence | undefined {
  const selected = valueAtPath(value, path);
  if (selected === undefined) return undefined;
  const segment: PathSegment | undefined = path.at(-1);
  const label = fallback ?? (typeof segment === "string"
    ? humanizeField(segment)
    : typeof segment === "number" ? `Item ${segment + 1}` : "Result");
  return evidenceFor(selected, path, label);
}
