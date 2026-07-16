import {isWireNumber, type WireValue} from "../../../clients/typescript/src/index.ts";
import {safeInlineText} from "./text.ts";

export type PathSegment = string | number;
export type JsonPath = readonly PathSegment[];
export type TreeKind = "object" | "array" | "string" | "number" | "boolean" | "null";
export type ArrayOrder = "index" | "name";

export interface TreeRow {
  readonly id: string;
  readonly path: JsonPath;
  readonly depth: number;
  readonly label: string;
  readonly kind: TreeKind;
  readonly preview: string;
  readonly expandable: boolean;
  readonly expanded: boolean;
  readonly value: WireValue;
  readonly sourceIndex?: number;
}

export interface FlattenTreeOptions {
  readonly rootLabel?: string;
  readonly maximumRows?: number;
  readonly arrayOrder?: ArrayOrder;
}

export interface TreeSearchMatch {
  readonly id: string;
  readonly groupId: string;
  readonly path: JsonPath;
  readonly label: string;
  readonly kind: TreeKind;
  readonly preview: string;
  readonly title: string;
  readonly context: string;
  readonly detail: string;
  readonly reason: TreeSearchMatchReason;
  readonly copyText?: string;
}

export type TreeSearchMatchReason = "identity" | "key" | "value" | "key-value";

export interface TreeSearchResult {
  readonly matches: readonly TreeSearchMatch[];
  readonly visitedNodes: number;
  readonly truncated: boolean;
}

export interface TreeSearchOptions {
  readonly rootLabel?: string;
  readonly arrayOrder?: ArrayOrder;
  readonly maximumNodes?: number;
  readonly maximumMatches?: number;
}

export function pathKey(path: JsonPath): string {
  return JSON.stringify(path);
}

export function formatPath(path: JsonPath): string {
  if (path.length === 0) return "$";
  return path.reduce<string>((output, segment) => {
    if (typeof segment === "number") return `${output}[${segment}]`;
    if (/^[A-Za-z_$][A-Za-z0-9_$]*$/u.test(segment)) return `${output}.${segment}`;
    return `${output}[${JSON.stringify(segment)}]`;
  }, "$");
}

export function formatBreadcrumb(rows: readonly TreeRow[], row: TreeRow): string {
  const labels: string[] = [];
  for (let depth = 0; depth <= row.path.length; depth += 1) {
    const ancestor = rows.find(candidate => candidate.id === pathKey(row.path.slice(0, depth)));
    if (ancestor !== undefined) labels.push(ancestor.label);
  }
  if (labels.length <= 1) return labels[0] ?? "result";
  return labels.slice(1).join(" › ");
}

export function valueKind(value: WireValue): TreeKind {
  if (value === null) return "null";
  if (isWireNumber(value)) return "number";
  if (Array.isArray(value)) return "array";
  if (typeof value === "object") return "object";
  if (typeof value === "string") return "string";
  return "boolean";
}

function preview(value: WireValue): string {
  const kind = valueKind(value);
  switch (kind) {
    case "null": return "null";
    case "number": return isWireNumber(value) ? value.lexeme : String(value);
    case "boolean": return String(value);
    case "string": return JSON.stringify(safeInlineText(value as string, 72));
    case "array": return `[${(value as readonly WireValue[]).length}]`;
    case "object": return `{${Object.keys(value as Readonly<Record<string, WireValue>>).length}}`;
  }
}

interface TreeChild {
  readonly label: string;
  readonly segment: PathSegment;
  readonly value: WireValue;
  readonly sourceIndex?: number;
  readonly sortName?: string;
}

interface ArrayItemIdentity {
  readonly field: string;
  readonly label: string;
  readonly value: string;
}

const IDENTITY_FIELDS = ["name", "display_name", "displayName", "title", "label", "id"] as const;
const DISPLAY_WORDS: Readonly<Record<string, string>> = Object.freeze({
  api: "API",
  gbps: "Gbps",
  id: "ID",
  ip: "IP",
  ips: "IPs",
  mbps: "Mbps",
  url: "URL",
  urls: "URLs",
  uuid: "UUID",
  zcc: "ZCC",
  zia: "ZIA",
  zpa: "ZPA",
  ztw: "ZTW"
});

function identityValue(value: WireValue): string | undefined {
  if (typeof value === "string") {
    const safe = safeInlineText(value, 64).trim();
    return safe.length === 0 ? undefined : safe;
  }
  if (isWireNumber(value)) return value.lexeme;
  return undefined;
}

function arrayItemIdentity(value: WireValue): ArrayItemIdentity | undefined {
  if (value === null || Array.isArray(value) || typeof value !== "object" || isWireNumber(value)) return undefined;
  for (const field of IDENTITY_FIELDS) {
    const rendered = identityValue(value[field]);
    if (rendered === undefined) continue;
    return {
      field,
      label: field === "id" ? `ID ${rendered}` : rendered,
      value: rendered
    };
  }
  return undefined;
}

function humanizeKey(value: string): string {
  const words = value
    .replace(/([A-Z]+)([A-Z][a-z])/gu, "$1 $2")
    .replace(/([a-z0-9])([A-Z])/gu, "$1 $2")
    .replace(/[_-]+/gu, " ")
    .trim()
    .split(/\s+/u)
    .filter(Boolean);
  return words.map((word, index) => {
    const normalized = word.toLowerCase();
    const known = DISPLAY_WORDS[normalized];
    if (known !== undefined) return known;
    return index === 0 ? `${normalized.slice(0, 1).toUpperCase()}${normalized.slice(1)}` : normalized;
  }).join(" ");
}

function presentationLabel(label: string, segment: PathSegment): string {
  if (typeof segment === "number") return /^\[\d+\]$/u.test(label) ? `Item ${segment + 1}` : label;
  return humanizeKey(label);
}

function describeValue(value: WireValue): string {
  const kind = valueKind(value);
  switch (kind) {
    case "null": return "null";
    case "number": return isWireNumber(value) ? value.lexeme : String(value);
    case "boolean": return String(value);
    case "string": return safeInlineText(value as string, 72) || "empty string";
    case "array": {
      const count = (value as readonly WireValue[]).length;
      return `${count} ${count === 1 ? "item" : "items"}`;
    }
    case "object": {
      const count = Object.keys(value as Readonly<Record<string, WireValue>>).length;
      return `${count} ${count === 1 ? "field" : "fields"}`;
    }
  }
}

function children(value: WireValue, arrayOrder: ArrayOrder): readonly TreeChild[] {
  if (Array.isArray(value)) {
    const items = value.map((child, index): TreeChild => {
      const identity = arrayItemIdentity(child);
      return {
        label: identity?.label ?? `[${index}]`,
        segment: index,
        value: child,
        sourceIndex: index,
        ...(identity === undefined ? {} : {sortName: identity.label.toLowerCase()})
      };
    });
    if (arrayOrder !== "name" || items.length < 2 || items.some(item => item.sortName === undefined)) return items;
    return [...items].sort((left, right) => {
      if (left.sortName! < right.sortName!) return -1;
      if (left.sortName! > right.sortName!) return 1;
      return left.sourceIndex! - right.sourceIndex!;
    });
  }
  if (value !== null && typeof value === "object" && !isWireNumber(value)) {
    return Object.entries(value).map(([label, child]) => ({label, segment: label, value: child}));
  }
  return [];
}

export function flattenTree(
  value: WireValue,
  expandedPaths: ReadonlySet<string>,
  options: FlattenTreeOptions = {}
): readonly TreeRow[] {
  const maximumRows = options.maximumRows ?? 800;
  const arrayOrder = options.arrayOrder ?? "index";
  const output: TreeRow[] = [];

  const visit = (current: WireValue, path: JsonPath, label: string, depth: number, sourceIndex?: number): void => {
    if (output.length >= maximumRows) return;
    const descendants = children(current, arrayOrder);
    const id = pathKey(path);
    const expandable = descendants.length > 0;
    const expanded = expandable && expandedPaths.has(id);
    const valuePreview = preview(current);
    output.push({
      id,
      path,
      depth,
      label: safeInlineText(label, 80),
      kind: valueKind(current),
      preview: sourceIndex === undefined ? valuePreview : `${valuePreview} · index ${sourceIndex}`,
      expandable,
      expanded,
      value: current,
      ...(sourceIndex === undefined ? {} : {sourceIndex})
    });
    if (!expanded) return;
    for (const child of descendants) {
      if (output.length >= maximumRows) return;
      visit(child.value, [...path, child.segment], child.label, depth + 1, child.sourceIndex);
    }
  };

  visit(value, [], options.rootLabel ?? "result", 0);
  return output;
}

export function initialExpansion(value: WireValue): ReadonlySet<string> {
  const paths = new Set<string>([pathKey([])]);
  if (value !== null && typeof value === "object" && !isWireNumber(value)) {
    if (!Array.isArray(value) && Array.isArray(value.records)) {
      paths.add(pathKey(["records"]));
      if (value.records.length > 0) paths.add(pathKey(["records", 0]));
    }
  }
  return paths;
}

export function toggleExpansion(expanded: ReadonlySet<string>, row: TreeRow): ReadonlySet<string> {
  if (!row.expandable) return expanded;
  const next = new Set(expanded);
  if (next.has(row.id)) next.delete(row.id);
  else next.add(row.id);
  return next;
}

export function parentPath(path: JsonPath): JsonPath {
  return path.slice(0, -1);
}

export function expandAncestors(expanded: ReadonlySet<string>, path: JsonPath): ReadonlySet<string> {
  const next = new Set(expanded);
  for (let depth = 0; depth < path.length; depth += 1) {
    next.add(pathKey(path.slice(0, depth)));
  }
  return next;
}

interface RankedText {
  readonly score: number;
}

function normalizedSearchText(value: string): string {
  return safeInlineText(value, 240).trim().toLowerCase().replace(/\s+/gu, " ");
}

function fuzzySubsequenceScore(candidate: string, query: string): number | undefined {
  const compactCandidate = [...candidate.replace(/[^\p{L}\p{N}]+/gu, "")];
  const compactQuery = [...query.replace(/[^\p{L}\p{N}]+/gu, "")];
  if (compactQuery.length < 2 || compactQuery.length > compactCandidate.length) return undefined;
  let candidateIndex = 0;
  let firstIndex = -1;
  let previousIndex = -1;
  let gaps = 0;
  for (const character of compactQuery) {
    while (candidateIndex < compactCandidate.length && compactCandidate[candidateIndex] !== character) {
      candidateIndex += 1;
    }
    if (candidateIndex >= compactCandidate.length) return undefined;
    if (firstIndex < 0) firstIndex = candidateIndex;
    if (previousIndex >= 0) gaps += candidateIndex - previousIndex - 1;
    previousIndex = candidateIndex;
    candidateIndex += 1;
  }
  return firstIndex + gaps;
}

function rankText(value: string, query: string, base: number, allowFuzzy: boolean): RankedText | undefined {
  const candidate = normalizedSearchText(value);
  if (candidate.length === 0) return undefined;
  if (candidate === query) return {score: base};
  if (candidate.startsWith(query)) return {score: base + 10 + Math.min(20, candidate.length - query.length)};
  const substringIndex = candidate.indexOf(query);
  if (substringIndex >= 0) return {score: base + 40 + Math.min(40, substringIndex)};
  if (!allowFuzzy) return undefined;
  const fuzzy = fuzzySubsequenceScore(candidate, query);
  return fuzzy === undefined ? undefined : {score: base + 80 + Math.min(80, fuzzy)};
}

function bestRank(...ranks: readonly (RankedText | undefined)[]): RankedText | undefined {
  let best: RankedText | undefined;
  for (const rank of ranks) {
    if (rank !== undefined && (best === undefined || rank.score < best.score)) best = rank;
  }
  return best;
}

function scalarCopyText(value: WireValue): string | undefined {
  const kind = valueKind(value);
  switch (kind) {
    case "null": return "null";
    case "number": return isWireNumber(value) ? value.lexeme : String(value);
    case "boolean": return String(value);
    case "string": return safeInlineText(value as string, 4_096);
    case "array":
    case "object":
      return undefined;
  }
}

export function searchTree(value: WireValue, query: string, options: TreeSearchOptions = {}): TreeSearchResult {
  const normalizedQuery = normalizedSearchText(safeInlineText(query, 120));
  if (normalizedQuery.length === 0) return {matches: [], visitedNodes: 0, truncated: false};

  const arrayOrder = options.arrayOrder ?? "index";
  const maximumNodes = Math.max(0, options.maximumNodes ?? 5_000);
  const maximumMatches = Math.max(0, options.maximumMatches ?? 200);
  interface RankedMatch {
    readonly match: TreeSearchMatch;
    readonly rank: number;
    readonly ordinal: number;
  }
  const candidates: RankedMatch[] = [];
  let visitedNodes = 0;
  let nodeLimitReached = false;
  let ordinal = 0;

  interface RecordContext {
    readonly id: string;
    readonly title: string;
    readonly trailStart: number;
  }

  const visit = (
    current: WireValue,
    path: JsonPath,
    label: string,
    sourceIndex?: number,
    suppressPreview = false,
    parentTrail: readonly string[] = [],
    recordContext?: RecordContext
  ): void => {
    if (visitedNodes >= maximumNodes) {
      nodeLimitReached = true;
      return;
    }
    visitedNodes += 1;
    const safeLabel = safeInlineText(label, 80);
    const kind = valueKind(current);
    const valuePreview = preview(current);
    const identity = sourceIndex === undefined ? undefined : arrayItemIdentity(current);
    const segment = path.at(-1);
    const displayLabel = segment === undefined ? humanizeKey(safeLabel) : presentationLabel(safeLabel, segment);
    const trail = path.length === 0 || segment === undefined
      ? parentTrail
      : [...parentTrail, displayLabel];
    const recordBoundary = sourceIndex !== undefined
      && ((path.length === 1 && typeof path[0] === "number")
        || (path.length === 2 && path[0] === "records" && typeof path[1] === "number"));
    const effectiveRecord = recordBoundary
      ? {id: pathKey(path), title: identity?.label ?? `Record ${sourceIndex + 1}`, trailStart: trail.length}
      : recordContext;
    const title = effectiveRecord?.title ?? trail[0] ?? displayLabel;
    const groupId = effectiveRecord?.id ?? pathKey(path.length === 0 ? [] : path.slice(0, 1));
    const contextLabels = effectiveRecord === undefined
      ? trail.slice(1)
      : trail.slice(effectiveRecord.trailStart);
    const context = contextLabels.length > 0
      ? contextLabels.join(" › ")
      : effectiveRecord === undefined ? "" : "Record";
    const identityRank = identity === undefined
      ? undefined
      : bestRank(
        rankText(safeLabel, normalizedQuery, 0, true),
        rankText(displayLabel, normalizedQuery, 0, true),
        rankText(identity.value, normalizedQuery, 0, true)
      );
    const labelRank = recordBoundary
      ? undefined
      : bestRank(
        rankText(safeLabel, normalizedQuery, 300, true),
        rankText(displayLabel, normalizedQuery, 300, true)
      );
    const valueRank = kind !== "array"
      && kind !== "object"
      && !suppressPreview
      ? rankText(describeValue(current), normalizedQuery, 600, false)
      : undefined;
    const rank = bestRank(identityRank, labelRank, valueRank);
    if (rank !== undefined) {
      const reason: TreeSearchMatchReason = identityRank !== undefined
        ? "identity"
        : labelRank !== undefined && valueRank !== undefined
          ? "key-value"
          : labelRank !== undefined ? "key" : "value";
      const copyText = scalarCopyText(current);
      candidates.push({
        rank: rank.score,
        ordinal: ordinal++,
        match: {
          id: pathKey(path),
          groupId,
          path,
          label: safeLabel,
          kind,
          preview: valuePreview,
          title,
          context,
          detail: describeValue(current),
          reason,
          ...(copyText === undefined ? {} : {copyText})
        }
      });
    }
    for (const child of children(current, arrayOrder)) {
      const duplicatesIdentity = identity !== undefined
        && child.segment === identity.field
        && identityValue(child.value) === identity.value;
      visit(
        child.value,
        [...path, child.segment],
        child.label,
        child.sourceIndex,
        duplicatesIdentity,
        trail,
        effectiveRecord
      );
      if (nodeLimitReached) return;
    }
  };

  visit(value, [], options.rootLabel ?? "result");
  interface MatchGroup {
    readonly id: string;
    readonly firstOrdinal: number;
    readonly matches: RankedMatch[];
    bestRank: number;
  }
  const groups = new Map<string, MatchGroup>();
  for (const candidate of candidates) {
    const current = groups.get(candidate.match.groupId);
    if (current === undefined) {
      groups.set(candidate.match.groupId, {
        id: candidate.match.groupId,
        bestRank: candidate.rank,
        firstOrdinal: candidate.ordinal,
        matches: [candidate]
      });
      continue;
    }
    current.bestRank = Math.min(current.bestRank, candidate.rank);
    current.matches.push(candidate);
  }
  const ordered = [...groups.values()]
    .sort((left, right) => left.bestRank - right.bestRank
      || left.firstOrdinal - right.firstOrdinal
      || left.id.localeCompare(right.id))
    .flatMap(group => group.matches.sort((left, right) => left.rank - right.rank
      || left.ordinal - right.ordinal
      || left.match.id.localeCompare(right.match.id)))
    .map(candidate => candidate.match);
  return {
    matches: ordered.slice(0, maximumMatches),
    visitedNodes,
    truncated: nodeLimitReached || ordered.length > maximumMatches
  };
}
