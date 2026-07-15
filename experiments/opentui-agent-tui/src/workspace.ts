import type {WireValue} from "../../../clients/typescript/src/index.ts";

import {DEMO_DATA} from "./fixture.ts";
import type {CommandDescriptor, ContextState, Tone, TranscriptFacet, TranscriptMetric} from "./model.ts";
import {safeInlineText} from "./text.ts";

export interface WorkspaceSummary {
  readonly metrics?: readonly TranscriptMetric[];
  readonly facets?: readonly TranscriptFacet[];
  readonly notes?: readonly string[];
}

export interface WorkspaceAnnouncement {
  readonly title: string;
  readonly body: readonly string[];
  readonly tone: Tone;
  readonly summary?: WorkspaceSummary;
}

export interface WorkspaceSnapshot {
  readonly data: WireValue;
  readonly context: ContextState;
  readonly announcement: WorkspaceAnnouncement;
}

export interface WorkspaceProgressEvent {
  readonly kind: "progress";
  /** Work units fully completed; the in-flight unit is never included. */
  readonly completed: number;
  readonly total: number;
  readonly message: string;
}

export interface WorkspaceExecutionContext {
  readonly signal: AbortSignal;
  readonly emit: (event: WorkspaceProgressEvent) => void;
}

export interface WorkspacePickerItem {
  readonly id: string;
  readonly title: string;
  readonly description: string;
  readonly searchText?: string;
  readonly category: string;
  readonly scopeId?: string;
  readonly badge?: string;
  readonly command: string;
}

export interface WorkspacePickerScope {
  readonly id: string;
  readonly label: string;
  readonly count: number;
}

export interface WorkspacePicker {
  readonly title: string;
  readonly placeholder: string;
  readonly instruction: string;
  readonly emptyMessage: string;
  readonly items: readonly WorkspacePickerItem[];
  readonly scopes?: readonly WorkspacePickerScope[];
  readonly scopeLabel?: string;
  readonly initialQuery?: string;
}

export interface WorkspaceResult {
  readonly announcement: WorkspaceAnnouncement;
  readonly data?: WireValue;
  readonly context?: ContextState;
  readonly picker?: WorkspacePicker;
}

export interface WorkspaceAdapter {
  readonly id: string;
  readonly initial: WorkspaceSnapshot;
  readonly commands?: readonly CommandDescriptor[];
  connect?(context: WorkspaceExecutionContext): Promise<WorkspaceResult>;
  execute?(input: string, context: WorkspaceExecutionContext): Promise<WorkspaceResult>;
  reload?(context: WorkspaceExecutionContext): Promise<WorkspaceResult>;
  close(): Promise<void>;
}

export class WorkspaceCommandError extends Error {
  readonly title: string;
  readonly tone: Tone;
  readonly details: readonly string[];
  readonly canceled: boolean;

  constructor(options: {
    readonly title: string;
    readonly message: string;
    readonly tone?: Tone;
    readonly details?: readonly string[];
    readonly canceled?: boolean;
  }) {
    super(options.message);
    this.name = "WorkspaceCommandError";
    this.title = options.title;
    this.tone = options.tone ?? "danger";
    this.details = options.details ?? [];
    this.canceled = options.canceled ?? false;
  }
}

export interface FilteredWorkspacePicker {
  readonly items: readonly WorkspacePickerItem[];
  readonly truncated: boolean;
}

export function normalizeWorkspacePicker(picker: WorkspacePicker): WorkspacePicker {
  const items = picker.items.map(item => ({
    ...item,
    title: safeInlineText(item.title, 240),
    description: safeInlineText(item.description, 500),
    category: safeInlineText(item.category, 120),
    ...(item.badge === undefined ? {} : {badge: safeInlineText(item.badge, 80)}),
    ...(item.searchText === undefined ? {} : {searchText: safeInlineText(item.searchText, 4_096)})
  }));
  const scopeCounts = new Map<string, number>();
  for (const item of items) {
    if (item.scopeId !== undefined) scopeCounts.set(item.scopeId, (scopeCounts.get(item.scopeId) ?? 0) + 1);
  }
  const seenScopeIds = new Set<string>();
  const scopes = picker.scopes?.flatMap(scope => {
    if (seenScopeIds.has(scope.id)) return [];
    seenScopeIds.add(scope.id);
    const count = scopeCounts.get(scope.id) ?? 0;
    return count === 0 ? [] : [{
      id: scope.id,
      label: safeInlineText(scope.label, 80),
      count
    }];
  });
  return {
    title: safeInlineText(picker.title, 180),
    placeholder: safeInlineText(picker.placeholder, 180),
    instruction: safeInlineText(picker.instruction, 500),
    emptyMessage: safeInlineText(picker.emptyMessage, 500),
    ...(picker.scopeLabel === undefined ? {} : {scopeLabel: safeInlineText(picker.scopeLabel, 80)}),
    ...(scopes === undefined ? {} : {scopes}),
    items,
    ...(picker.initialQuery === undefined ? {} : {initialQuery: safeInlineText(picker.initialQuery, 120)})
  };
}

export function filterWorkspacePicker(
  items: readonly WorkspacePickerItem[],
  query: string,
  options: number | {readonly limit?: number; readonly scopeId?: string} = {}
): FilteredWorkspacePicker {
  const limit = typeof options === "number" ? options : options.limit ?? 80;
  const scopeId = typeof options === "number" ? undefined : options.scopeId;
  const scoped = scopeId === undefined ? items : items.filter(item => item.scopeId === scopeId);
  const terms = query.trim().toLowerCase().split(/\s+/u).filter(Boolean);
  const matched = terms.length === 0
    ? scoped
    : scoped.filter(item => {
        const text = `${item.category} ${item.title} ${item.description} ${item.searchText ?? ""} ${item.badge ?? ""}`.toLowerCase();
        return terms.every(term => text.includes(term));
      });
  return {items: matched.slice(0, limit), truncated: matched.length > limit};
}

const FIXTURE_CONTEXT: ContextState = {
  connection: "fixture",
  transport: "in-memory fixture",
  authority: "tenant read-only",
  scope: "zia/locations",
  records: 2,
  effects: "none",
  operation: {status: "idle", label: "ready"}
};

const INITIAL_FIXTURE: WorkspaceSnapshot = {
  data: DEMO_DATA,
  context: FIXTURE_CONTEXT,
  announcement: {
    title: "Structured fixture ready",
    body: [
      "A nested, sanitized location result is loaded without credentials.",
      "The large numeric identifier remains an exact wire lexeme rather than a JavaScript float."
    ],
    tone: "success"
  }
};

function delay(milliseconds: number, signal: AbortSignal): Promise<void> {
  return new Promise((resolve, reject) => {
    if (signal.aborted) {
      reject(new WorkspaceCommandError({title: "Operation canceled", message: "The local operation was canceled.", tone: "warning", canceled: true}));
      return;
    }
    const timer = setTimeout(() => {
      signal.removeEventListener("abort", canceled);
      resolve();
    }, milliseconds);
    const canceled = () => {
      clearTimeout(timer);
      reject(new WorkspaceCommandError({title: "Operation canceled", message: "The local operation was canceled.", tone: "warning", canceled: true}));
    };
    signal.addEventListener("abort", canceled, {once: true});
  });
}

export const FIXTURE_WORKSPACE_ADAPTER: WorkspaceAdapter = Object.freeze({
  id: "fixture",
  initial: INITIAL_FIXTURE,
  async reload(context: WorkspaceExecutionContext): Promise<WorkspaceResult> {
    await delay(480, context.signal);
    return {
      data: DEMO_DATA,
      context: {...FIXTURE_CONTEXT, operation: {status: "complete", label: "2 records projected"}},
      announcement: {
        title: "Fixture reloaded",
        body: ["The context tree has been reset to its initial expansion state."],
        tone: "success"
      }
    };
  },
  async close(): Promise<void> {
    // The fixture owns no external resources.
  }
});
